# Cloud 上游错误传播计划 / Cloud Upstream Error Propagation Plan
最后修改时间: 2026-09-02 16:47:10

Review status: Accepted

## Flow mode / Stage

标准模式 / standard；计划 / Plan 已接受，当前阶段：实现 / Implementation。

## Requirement basis

- Requirement: [`docs/requirement/20260902-cloud-upstream-error-propagation.md`](../requirement/20260902-cloud-upstream-error-propagation.md)
- Requirement status: `Accepted`

本计划遵守以下已确认约束：

1. Agent 到 Cloud 的远程错误必须使用包含操作、上游 HTTP status 和 Cloud `ErrorResp` 的 typed error。
2. Cloud 已返回的非 `2xx` status、`code`、安全 `error` 文本、request ID 与已有 `details` 在 Agent 本地 API 中原样继承。
3. 本机入口的 `X-Request-ID` 必须透传到 Cloud，Cloud response 与本地 response 使用同一个 request ID；本地 API 顶层仍使用既有 `ErrorResp`，不为 Cloud 错误增加 wrapper 或重复字段。
4. 不按操作或具体 status（包括 `401`、`409`）建立改写逻辑。
5. Cloud `/api/auth/me` 的正常无效凭据语义是 `200` 与 `authenticated:false`，继续是成功响应；只有异常非 `2xx` 才进入错误传播。
6. 不新增 refresh token、登录续期或 TTL 策略；JWT 在 `exp` 时刻及之后无效。

## Current findings

Agent 当前通过 `internal/agent/infrastructure/cloudapi/client.go` 发起四类 Cloud HTTP 调用：

| Operation | Cloud request | Agent local route |
| --- | --- | --- |
| `register_current_device` | `POST /devices/current` | `POST /api/cloud/connect` |
| `auth_me` | `GET /auth/me` | `GET /api/cloud/auth/me` |
| `list_devices` | `GET /devices` | `GET /api/cloud/devices` |
| `exchange_oauth_code` | `POST /oauth2/token` | `POST /api/cloud/oauth/exchange` |

当前 client 对前 3 个普通 HTTP 请求大多只返回格式化字符串错误；OAuth 依赖 `oauth2.Config.Exchange` 返回的 `*oauth2.RetrieveError`。Agent handler 将这些错误分别转换为 `502 upstream_error`，`auth/me` 还把所有错误转换为 `200 authenticated:false`。当前 Agent-to-Cloud 请求也没有转发入口 `X-Request-ID`，会导致 Cloud 生成另一条关联 ID。工作树中的未提交补丁仅为连接和设备列表增加 `401` sentinel，并让前端按 `/cloud/` URL 前缀清 token；该设计不覆盖全部调用点，也不满足统一传播约束，实施时必须移除并替换。

Cloud 已使用 `shared.ErrorResp{code,error,request_id,details}` 输出其正常 API 错误；Cloud OAuth token endpoint 也使用同一结构化 writer。`oauth2.RetrieveError` 同时保存 `Response` 和已读取的 `Body`，可进入同一解析路径。现有 Agent `writeAPIErrorWithDetails` 可保持顶层 envelope，但在 debug 模式会尝试用 `cause` 覆盖 safe message，因此 Cloud 透传路径不能直接复用其默认消息选择行为。

## Target error contract

在 `internal/agent/application/user` 定义一个可由 infrastructure 返回、可由 HTTP handler 使用 `errors.As` 识别的统一类型，例如 `CloudUpstreamError`。该包已定义 `CloudApi` 接口，且不依赖 infrastructure，避免产生依赖环。

类型至少包含：

```go
type CloudOperation string

type CloudUpstreamError struct {
    Operation      CloudOperation
    UpstreamStatus int // 收到 HTTP response 时为实际 status；无 response 时为零
    CloudError     *shared.ErrorResp
    Cause          error
}
```

- 操作常量固定为 `register_current_device`、`auth_me`、`list_devices`、`exchange_oauth_code`。
- `CloudError` 非空时保存完整 Cloud `ErrorResp`，其 `Code` 是 typed error 的 Cloud error code；不复制或改写字段。
- 类型实现 `Error()` 和 `Unwrap()`，用于日志及 `errors.As`，但不把 `Cause` 的原始文本暴露到浏览器。
- Cloud 无 response、成功 payload 解码失败、请求创建失败等 Cloud client 边界失败仍包装为该类型，使 operation 可诊断；没有 Cloud code 时不伪造 code。

该 type 只在 Agent 内部传递。对于可解析的 Cloud error，浏览器 response 直接使用该 `CloudError` 的既有字段；不增加 `cloud_error` namespace，不嵌套 `response`，不重复 HTTP status、`code`、`error`、`request_id` 或 Cloud `details`。

```json
{
  "code": "unauthorized",
  "error": "Authentication is required.",
  "request_id": "req_8683c3772eddd2ed76f94b13c8851e0c",
  "details": { "...": "Cloud 原有 details" }
}
```

Agent HTTP 入口必须先复用或生成本机 request ID，并将其写入请求 context；Cloud client 的统一 HTTP transport 对任意出站 request 从 context 读取并设置 `X-Request-ID`，不按 operation、认证模式或 OAuth 路径分支。Cloud 结构化 error 返回该 ID 后，本地 response 不生成第二个 ID。非 `2xx` 但无法解析为 `ErrorResp` 时，本地以同一个 request ID 输出 `upstream_error`；operation、上游 status 和稳定失败类别仅进入 typed error 与安全日志。无 response 或成功 payload 解码失败也不记录 raw body、Authorization、token、client secret 或底层错误文本。

| Cloud client 结果 | 本地 HTTP status | 顶层 code / error | details |
| --- | --- | --- | --- |
| 非 `2xx` + 合法 `ErrorResp` | 原样继承 | Cloud `code` / `error` | Cloud 原有 `details` 直接沿用 |
| 非 `2xx` + 无效或非结构化 body | 原样继承 | `upstream_error` / `Upstream response is invalid.` | 不伪造 details |
| 无 HTTP response | `502` | `upstream_error` / `Upstream request failed.` | 不伪造 details |
| `2xx` + 成功 DTO 无法解码 | `502` | `upstream_error` / `Upstream response is invalid.` | 不伪造 details |
| `200` AuthMe `{authenticated:false}` | `200` | 不适用 | 正常 `AuthMeResp` |

该表只按是否收到 response、status 区间和是否满足错误契约区分，不根据 operation 或具体 HTTP status 重写结果。

## Implementation steps

### Step 1: Define the Agent Cloud upstream error boundary

Modify `internal/agent/application/user/cloud_service.go`.

1. 删除仅表达 `401` 的 `ErrCloudUnauthorized` sentinel。
2. 新增 `CloudOperation` 常量和 `CloudUpstreamError`，使其保留 operation、非零 upstream status、完整 `*shared.ErrorResp` 和 wrapped cause。
3. 为 handler 提供稳定的 Cloud code 读取方式；当 `CloudError` 为空时返回空值而非本地伪造 code。
4. 保持 `CloudApi` 与 `CloudService` 的业务方法签名不含 request ID；调用链 context 作为唯一传递媒介，业务成功语义不变。

### Step 2: Unify Cloud HTTP and OAuth response parsing

Modify `internal/agent/infrastructure/cloudapi/client.go` and extend `internal/agent/infrastructure/cloudapi/client_test.go`.

1. 抽取通用的 Cloud failure constructor：输入 operation、可选 `*http.Response`、可选已读取 body 和 cause，输出 `*agentapp.CloudUpstreamError`。
2. `RegisterCurrentDevice`、`AuthMe`、`ListDevices` 在每个非 `2xx` response 上读取 body，调用该通用 constructor；删除所有 `401` special case。
3. 在 `Client.New` 中用统一 RoundTripper 包装 HTTP client；该 transport 对每个普通 HTTP request 和 OAuth token request 都从 request context 写入 `X-Request-ID`，不在 operation 方法内单独处理。`Do`、请求构造、读取 body 和成功 DTO 解码失败都带上对应 operation；成功结果语义不变。
4. `AuthMe` 继续把 Cloud `200 authenticated:false` 解码并返回为成功，不再把异常响应或网络失败软转换为 `authenticated:false`。
5. `ExchangeOAuthCode` 用 `errors.As` 提取 `*oauth2.RetrieveError`，从其 `Response.StatusCode` 和 `Body` 调用同一个 failure constructor；为 OAuth client 构造保留原有 client 配置、但注入同一 `X-Request-ID` 的 HTTP transport，使 token endpoint 也使用该关联 ID；无 `RetrieveError.Response` 的 error 仍作为无 response typed error 返回。
6. 对非 `2xx` body，使用现有 protobuf JSON codec 解码 `shared.ErrorResp`，并要求至少存在 Cloud `code`；解码失败只记录稳定协议失败类别。不得暴露 raw response body，也不得回退使用 OAuth RFC `error` 文本伪装为 Cloud `ErrorResp.code`。

### Step 3: Write one generic Agent HTTP propagation path

Modify `internal/agent/api/handler/server.go` and `internal/agent/api/handler/errors.go`.

1. Agent HTTP handler 在统一入口取得 `requestIdFor(w, r)` 并写入 request context；所有四个 Cloud relay handler 直接传递该 context 给 CloudService，失败路径统一调用一个 `writeCloudError`，包括 `handleCloudConnect`、`handleCloudAuthMe`、`handleCloudDevices` 和 `handleExchangeOAuthCode`。
2. `writeCloudError` 仅通过 `errors.As` 检查 `*agentapp.CloudUpstreamError`，不按 operation、`401`、`409` 或任何其他 status 写分支。
3. 对结构化 Cloud error，使用上游 status 并直接写出 Cloud `ErrorResp`：`code`、`error`、`request_id` 与 `details` 都不复制、不包装；断言该 request ID 与本地 handler 已取得的 ID 相同。
4. 增加一个内部 response writer 路径，使 Cloud 透传的 `error` 字段在 `DebugErrors` 打开时也不被本地 `cause.Error()` 替换；日志记录 operation、cause 和同一个 request ID。
5. 对 typed transport/protocol error 按上表输出 fallback，不伪造 details；对不是 `CloudUpstreamError` 的现有本地错误保持既有 `502 upstream_error` 行为。
6. 删除局部的 `ErrCloudUnauthorized` 映射和 AuthMe 的 `200 authenticated:false` error fallback。

### Step 4: Give local Cloud relay requests an explicit frontend identity

Modify `web/src/features/api/client.ts`、`web/src/features/api/client.test.ts` 和 `web/src/features/local/api.ts`.

1. 新建明确的 `localCloudApiClient`，其 base URL 与 `localApiClient` 相同，但其 response interceptor 的 `401` 语义是“本次请求使用 Cloud bearer，清除 Cloud credential 并走既有登录恢复”。
2. `fetchCloudIdentityViaLocalApi`、`listCloudDevicesViaLocalApi` 与 `connectCloudWithToken` 改用 `localCloudApiClient`，并继续在请求上显式传入当前 Cloud token。
3. `exchangeOAuthCode`、`disconnectCloud` 和所有普通 local API 请求保留 `localApiClient`；OAuth 换码不因为本地 API 的 `401` 自动清除已有 Cloud credential。
4. 删除 `isCloudCredentialRequest()` 及任何 `/cloud/` URL 前缀判断。直接 Cloud API 的 `cloudApiClient` 现有凭据行为保持不变。

### Step 5: Keep the JWT expiry boundary correction

Retain and review the current targeted change in `internal/shared/common/auth/jwt.go` and its new unit test.

1. `Verify` 在 `exp <= now` 时拒绝 token，符合 JWT expiration 的边界语义。
2. 保持 `cloud.jwt.ttl` 配置和默认 `24h` 不变，不引入 refresh token。
3. 测试只覆盖配置 TTL 和刚好到期时拒绝，避免扩展为认证策略重构。

## Files to change

### Expected modified files

- `internal/agent/application/user/cloud_service.go`
- `internal/agent/infrastructure/cloudapi/client.go`
- `internal/agent/infrastructure/cloudapi/client_test.go`
- `internal/agent/api/handler/errors.go`
- `internal/agent/api/handler/server.go`
- `internal/agent/api/handler/cloud_binding_test.go`
- `internal/shared/common/auth/jwt.go`
- `web/src/features/api/client.ts`
- `web/src/features/api/client.test.ts`
- `web/src/features/local/api.ts`
- `docs/requirement/20260902-cloud-upstream-error-propagation.md`
- `docs/plan/20260902-cloud-upstream-error-propagation.md`

### Expected new files

- `internal/shared/common/auth/jwt_test.go`（当前工作树已存在，实施中保留并纳入本次范围。）
- `internal/shared/common/requestid/context.go`

### Not expected to change

- Cloud handler 的认证/授权和业务 status 定义。
- `shared.ErrorResp` protobuf schema、成功 response envelope 或本地 API route。
- Agent tunnel 的 `tunnel.RemoteError` 模型。
- Cloud token TTL、refresh/rotation、浏览器 token 存储格式。
- 开发服务器、迁移文件和 Git metadata。

## Verification plan

### Focused backend tests

1. `internal/agent/infrastructure/cloudapi/client_test.go`：四个 operation 的成功路径；结构化 Cloud error 的 operation/status/code/error/request ID/details 保留；非 `2xx` 非结构化 body；transport failure；成功 DTO 解码失败；OAuth `RetrieveError` 的 structured body。
2. `internal/agent/api/handler/cloud_binding_test.go`：connect、异常 AuthMe、devices、OAuth exchange 分别断言 Cloud 收到的 `X-Request-ID`、Cloud error body 与本地 response 使用同一个 request ID；status/code/error/details 直接一致，不出现 wrapper 或重复嵌套。
3. 断言 Cloud `200 authenticated:false` 经 `/api/cloud/auth/me` 仍返回成功，不再把 Cloud 不可用软转换成未认证。
4. `internal/shared/common/auth/jwt_test.go`：配置 TTL 与 `exp == now` 的拒绝边界。

### Frontend tests

1. `localCloudApiClient` 收到结构化 `401` 时清 Cloud token 并保持既有登录跳转。
2. `localApiClient` 对普通 local request 和 OAuth exchange 的 `401` 不执行该清理。
3. 直接 Cloud client 的既有 bearer 注入与清理逻辑继续通过。
4. `ApiClientError` 继续暴露继承的 HTTP status、code、同一个 request ID 和 details，不新增 URL 推断断言。

### Required checks after implementation

```text
yarn --cwd web lint:fix
yarn --cwd web typecheck
task check
go test ./cmd/... ./internal/...
```

实现阶段还应运行受影响 Go package 与 Vitest 的聚焦测试，以便在全量检查前定位失败。按仓库约定，不启动、停止或重启开发服务器。

## Assumptions

1. Cloud 的非 `2xx` 业务错误继续使用 `shared.ErrorResp`；该响应含有效 `code` 时才作为可继承的 Cloud error code。
2. Cloud `ErrorResp.details` 已是浏览器可见的安全数据；Agent 直接沿用，不解析、拼接、嵌套或扩张其语义。
3. Cloud OAuth token endpoint 的错误仍由 Cloud 结构化 error writer 输出；`oauth2.RetrieveError.Body` 是读取该 body 的唯一输入，不再额外读取 response body。
4. 本地 Cloud relay 的调用身份由显式 client 表达，避免路由命名改变引入凭据清理行为。

## Risks

1. 透传 Cloud `code` 和 status 使本地 API 对 Cloud error contract 更直接依赖；Cloud 端结构化错误测试必须继续覆盖稳定性。
2. `DebugErrors` 目前可显示本地 cause；若 Cloud 路径没有独立 message writer，会破坏 Cloud 文案继承或泄露细节，实施时必须单独验证。
3. OAuth library 已消费 response body；重复读取或只依赖 `RetrieveError.ErrorCode` 会丢失 Cloud `ErrorResp`，且 OAuth transport 必须保留同一 request ID，需用测试约束。
4. 工作区已有局部补丁且包含 CRLF/尾随空白问题；实施时应以本计划重写相关部分，不能保留 sentinel 或 URL-prefix 判断。

## Rollback

若统一实现出现问题，优先保留 typed error、Cloud response parser 和覆盖四个操作的测试，再精确回退 handler 或 frontend client 的未完成接入。不得恢复按 `401`/operation 的 sentinel 分支，也不得用全局 URL 前缀重新引入凭据清理判断。任何范围调整应先更新本计划和后续 Verification 记录。

## User review notes

- 用户要求先完整思考 Cloud 与本地的调用和异常传递，再实施，不接受只修两个接口的局部补丁。
- 用户要求 typed error 沿用 Cloud 字段与上游 status；本地保持既有 `ErrorResp`，直接沿用 Cloud `details`，不重复包装 Cloud 错误。
- 用户要求不为任意具体 HTTP status 或 operation 编写改写逻辑；无法区分的错误保持统一传播。
- 用户确认：本机入口和 Cloud response 的 `request_id` 应相同，必须转发 `X-Request-ID`；operation 仅用于内部 typed error 和日志。
- 用户明确要求开始 Implementation；Plan 已于 2026-09-02 接受。
- 本计划选择不创建 Spec 文档，符合 standard 模式；请 review 此 Plan 后再进入 Implementation。
