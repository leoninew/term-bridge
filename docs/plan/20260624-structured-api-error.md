# Structured API Error Plan
最后修改时间: 2026-06-24 18:14:36

## Review status

Accepted

## Flow mode / Stage

标准模式 / standard；计划 / Plan 已接受，当前阶段：实现 / Implementation。

## Requirement basis

- Requirement: `docs/requirement/20260624-structured-api-error.md`
- Requirement status: `Accepted`

本计划遵守以下已接受约束：

1. 使用 best-practices 结构：错误响应采用 `{ code, error, requestId, details? }`，不沿用也不向后兼容历史 `{ code, message, error }`。
2. `details` 本轮可以先不实现；如果保留类型，也必须是 optional。
3. 前端引入 axios：request interceptor 生成/注入 request id，response interceptor 统一处理结构化 API error。
4. `requestId` 必须接入服务端日志，不能只出现在响应 body；后端收到 request id 必须沿用并输出到下游和响应，没有则自行生成。
5. `error` 字段是否赋值由配置控制；关闭时不透出内部错误，开启时仍避免敏感信息泄露。
6. 成功响应保持自然 DTO 字段语义，不增加统一 envelope。
7. 前后端 API contract 层请求 DTO 使用 `XxxxReq` 命名，响应 DTO 使用 `XxxxResp` 命名；嵌套结构视具体需求，不强制要求。
8. 代码标识符中的 ID 缩写使用 `Id`：`AbcdID` → `AbcdId`，`abcdID` → `abcdId`；JSON 字段名和外部 wire format 不因此改动。
9. 404 兜底为服务前端路由和 API 调用的一致体验，需结合现有 HTTP server / frontend serving 方式自行评估，不能破坏 SPA 服务。
9. WebSocket 升级后的 terminal protocol 不强行改造；普通 HTTP API 与 agent relay/terminal attach 等下游 tunnel 交互按可兼容方式携带 request id。
10. 不执行 git 写操作。

## Current code findings

1. `internal/transport/http/gatewayapi/server.go` 目前集中承载 gateway API，但大量失败路径直接使用 `http.Error`：
   - login unauthorized。
   - device offline。
   - agent relay / `route.request` 失败。
   - history JSON decode 失败。
   - terminal ws 升级前 conflict / offline。
   - agent tunnel unauthorized。
   - `methodNotAllowed()` / `decodeJSONRequest()` helper 也返回纯文本。
2. `internal/transport/http/gatewayapi/auth/auth.go` 的 `Middleware` 直接 `http.Error(w, "unauthorized", 401)`，当前无法写结构化 JSON，也拿不到 gateway handler 的 config/logger。
3. `internal/transport/http/middleware/requestlog/logging.go` 已有请求日志 middleware：
   - 使用 header `X-Request-ID`。
   - 未提供 context helper，但日志里已有 `request_id` 字段。
   - 现有 header 常量拼写为 `X-Request-ID`，而参考示例为 `X-Request-Id`；HTTP header case-insensitive，计划统一沿用现有 `X-Request-ID` 避免无谓 churn。
4. `internal/transport/http/server/server.go` 目前只挂载 `/api` 和 `/api/` 到 gateway handler，没有发现当前 Go 层 serve 前端静态资源或 SPA fallback。
5. `internal/app/app.go` 已把 `cfg.Web.Error.Debug` 传入 `gatewayapi.Config.DebugErrors`，该配置目前可直接作为 `error` 字段赋值开关。
6. `web/package.json` 当前未引入 axios，所有 feature API 直接使用 `fetch`。
7. `web/src/protocol/terminal.ts` 中 `ApiErrorResponse` 当前为 `{ code, message, error }`，需要改为 best-practices 结构；不保留旧结构兼容。
8. `web/src/features/sessions/api.ts` 的 `responseError()` 当前只识别 `parsed.code` + `parsed.message`，需要被 axios response interceptor 取代；不保留旧结构 fallback。
9. `internal/protocol/tunnel/frame.go` 的 `RequestPayload` / `ResponsePayload`、`TerminalAttachPayload` 属于 gateway-agent protocol DTO，当前命名不符合 `XxxxReq` / `XxxxResp` 规则；如果本轮纳入 protocol DTO 命名，需要重命名并同步调用点。
10. `internal/protocol/tunnel/frame.go` 的 request / terminal attach DTO 当前没有 request id 字段；如果要求向 agent 下游传递 request id，应以 optional JSON 字段追加，避免破坏现有 frame JSON 字段语义。
11. `internal/application/terminal/registry.go` 和 `web/src/protocol/terminal.ts` 中存在 `CreateSessionRequest`、`RerunSessionRequest`、`UpdateSessionRequest`、`CreateSessionResponse` 等前后端 API contract DTO，需要按 `Req` / `Resp` 命名收敛。

## Error response contract

### Body

本轮统一 HTTP API error body：

```json
{
  "code": "device_offline",
  "error": "Device is offline.",
  "requestId": "req_123"
}
```

可选扩展：

```json
{
  "code": "validation.failed",
  "error": "Request validation failed.",
  "requestId": "req_123",
  "details": {
    "fields": []
  }
}
```

本轮不实现 `details.fields` 的具体使用；类型和 writer 可以保留 optional `details any` 或先不定义 details helper。

### Header

- 前端 axios request interceptor 为每个 HTTP API 请求生成或补齐 request id，并写入 `X-Request-ID`。
- 后端请求已携带 `X-Request-ID` 时复用该值。
- 后端请求未携带时生成 request id。
- 响应回写同一个 `X-Request-ID`。
- JSON body 的 `requestId` 与 header 一致。
- 后端向 agent 下游发起 tunnel request / terminal attach 时携带同一个 request id。

### `error` 字段

`error` 字段是响应中唯一的人可读错误摘要字段，但是否包含底层诊断信息由配置控制：

- `DebugErrors == false`：`error` 只放安全摘要，例如 `Device is offline.` / `Upstream request failed.`。
- `DebugErrors == true`：允许在安全摘要基础上追加或替换为有限底层错误信息，用于本地开发诊断；仍不得输出 token、cookie、credential、连接串等敏感信息。

实现上不新增 `debug`、`exceptionType`、`stackTrace`、`traceback`、`message` 等响应字段。

## Error code and status mapping

| Scenario | HTTP status | code | Safe error |
| --- | --- | --- | --- |
| malformed JSON / invalid request body | 400 | `bad_request` | `Request body is invalid.` |
| unauthorized browser API | 401 | `unauthorized` | `Authentication is required.` |
| unauthorized agent tunnel | 401 | `unauthorized` | `Authentication is required.` |
| unknown API route / invalid nested route | 404 | `not_found` | `Resource was not found.` |
| unsupported method | 405 | `method_not_allowed` | `Method is not allowed.` |
| device not connected and no cache | 503 | `device_offline` | `Device is offline.` |
| agent relay request failed | 502 | `upstream_error` | `Upstream request failed.` |
| history relay result is not valid JSON string | 502 | `upstream_error` | `Upstream response is invalid.` |
| terminal writer conflict before websocket upgrade | 409 | `conflict` | `Session already has an active writer.` |
| unexpected gateway error | 500 | `internal_error` | `An unexpected error occurred.` |

## Implementation steps

### Step 1: Add frontend axios API client and request id interceptor

新增文件：`web/src/features/api/client.ts` 或同等集中位置。

实现内容：

1. 在 `web/package.json` 引入 `axios` 依赖。
2. 创建单例 axios client，base path 使用相对路径，保持现有同源 API 行为。
3. request interceptor：
   - 为每个请求生成 request id，例如 `req_${crypto.randomUUID()}`；如果调用方已显式设置 `X-Request-ID`，则沿用。
   - 写入 `X-Request-ID` header。
4. response interceptor：
   - 对成功响应直接返回 response，不包装 data。
   - 对失败响应只识别 `{ code, error, requestId, details? }`，构造统一 `ApiClientError` 或等价 Error 类型。
   - 不兼容旧 `{ code, message, error }`；协议不匹配时抛出明确的 API error contract mismatch。
5. 不让 Vue component 直接 import axios；feature API 只 import 集中 client。

验收点：前端所有 HTTP API 请求统一经过 axios client，不再新增裸 `fetch` 调用。

### Step 2: Add structured API error helper in gatewayapi

新增文件：`internal/transport/http/gatewayapi/errors.go`。

实现内容：

1. 常量：
   - `requestIdHeader = "X-Request-ID"`。
   - error code constants：`bad_request`、`unauthorized`、`not_found`、`method_not_allowed`、`device_offline`、`upstream_error`、`conflict`、`internal_error`。
2. 类型：
   ```go
   type errorResponse struct {
       Code    string `json:"code"`
       Error   string `json:"error"`
       RequestId string `json:"requestId"`
       Details any    `json:"details,omitempty"`
   }
   ```
3. helper：
   - `requestIdFromRequest(r *http.Request) string`：优先读 `X-Request-ID`，否则生成 `req_...`。
   - `writeAPIError(w, r, status, code, safeError string, cause error)`：写 header、JSON body、日志。
   - `debugError(config.DebugErrors, safeError, cause)`：控制 `error` 字段是否使用底层 cause。
4. 日志策略：
   - `writeAPIError` 使用 `Handler.config.Logger` 记录 `gateway api error`。
   - attrs 至少包含：`request_id`、`method`、`path`、`status`、`code`。
   - 若 `cause != nil`，日志记录 `cause.Error()`，但响应是否包含 cause 由 `DebugErrors` 决定。

注意：不要在 helper 里 panic；logger 已由 `normalizeConfig()` 保证非 nil。

### Step 3: Propagate request id through tunnel requests

修改：

- `internal/protocol/tunnel/frame.go`
- `internal/transport/http/gatewayapi/route.go`
- `internal/application/agent/client.go`

计划：

1. 按命名规则重命名 gateway-agent protocol DTO：
   - `tunnel.RequestPayload` → `tunnel.RequestReq`。
   - `tunnel.ResponsePayload` → `tunnel.ResponseResp`。
   - `tunnel.TerminalAttachPayload` → `tunnel.TerminalAttachReq`。
   - `tunnel.TerminalResizePayload` / `TerminalDataPayload` 这类 terminal 嵌套/事件 payload 是否重命名由实现时按语义判断；本轮不强制所有嵌套 payload。
2. `tunnel.RequestReq` 增加 optional 字段：
   ```go
   RequestId string `json:"request_id,omitempty"`
   ```
3. `tunnel.TerminalAttachReq` 增加 optional 字段：
   ```go
   RequestId string `json:"request_id,omitempty"`
   ```
4. `agentRoute.request` 增加 request id 参数，构造 tunnel request 时带上 request id。
5. gateway handler 调用 `route.request(...)` 时从当前 HTTP request 取 request id 并传入。
6. `handleTerminalWS` 在 terminal attach frame 中带 request id。
7. agent client 收到 request 后，可先不改变业务处理签名；至少在错误日志或未来扩展中可读取 `request.RequestId`。如果当前 agent 侧没有合适日志点，保留字段透传能力即可，不为了 request id 大改 runtime interface。

验收点：gateway 向下游 agent 发出的普通 tunnel request 和 terminal attach frame 都包含同一 request id；相关 protocol DTO 命名符合 `Req` / `Resp` 规则，JSON 字段语义保持不变。

### Step 4: Make auth middleware write structured errors

修改：`internal/transport/http/gatewayapi/auth/auth.go`。

目标：避免 auth package 直接写纯文本 `http.Error`。

推荐最小改法：

1. 保留现有 `Middleware(next http.Handler) http.Handler` 以降低影响。
2. 新增：
   ```go
   func (m *Manager) MiddlewareWithUnauthorized(next http.Handler, unauthorized http.HandlerFunc) http.Handler
   ```
   或等价命名。
3. `gatewayapi.Handler.Handler()` 中对 browser API 使用新 middleware，传入 `h.writeUnauthorized`。
4. 保留旧 middleware 给 auth package tests 或外部调用；必要时测试新增覆盖新方法。

验收点：`/api/devices` 未登录时返回结构化 JSON，而不是纯文本。

### Step 5: Replace gatewayapi http.Error paths

修改：`internal/transport/http/gatewayapi/server.go`。

替换策略：

1. `methodNotAllowed` 改为 `Handler` 方法，或接收 `*Handler` 参数，使其能调用 `writeAPIError`。
2. `decodeJSONRequest` 改为 `Handler` 方法，或接收 error writer；JSON decode 失败返回 `bad_request`。
3. 对已知路径替换：
   - login bad credentials → `unauthorized`。
   - device offline → `device_offline`。
   - relay failure → `upstream_error`，cause 记录日志，响应按 DebugErrors 控制。
   - invalid upstream history payload → `upstream_error`。
   - terminal writer conflict → `conflict`。
   - agent tunnel unauthorized → `unauthorized`。
4. 对 `http.NotFound(w, r)` 路径改为 `h.writeNotFound(w, r)`。
5. WebSocket upgrade 已成功后的 control messages 不纳入 HTTP JSON 化；升级前的 accept 失败仍由 websocket 库处理，不强行包装。

验收点：`grep` 不应再在 `gatewayapi/server.go` 和 `gatewayapi/auth/auth.go` 中发现业务路径直接 `http.Error`，除非是明确无法包装且有注释说明的 websocket library path。

### Step 6: Evaluate and implement 404 / frontend fallback boundary

修改：`internal/transport/http/server/server.go`，仅在评估后需要时修改。

当前发现：Go server 只挂 `/api` 和 `/api/`，没有 Go 层 serve frontend 静态资源；web dev 可能由 Vite 负责前端服务。因此计划如下：

1. API 路径：
   - `/api`、`/api/` 继续进入 gateway handler。
   - gateway handler 内所有 unknown API route 返回 structured `not_found`。
2. 非 API 路径：
   - 不在本轮新增静态文件服务。
   - 不把所有非 `/api` 404 改成 JSON，避免未来 SPA fallback 被破坏。
3. 如果实现阶段发现已有隐藏 frontend serving 或将要加静态服务：
   - API 404 使用 JSON。
   - 非 API 404 / SPA fallback 由 frontend serving 规则处理，不强制结构化 API error。

验收点：API not found 有结构化 JSON；非 API 路径不因本任务破坏前端服务策略。

### Step 7: Rename API contract DTOs to Req / Resp

修改：

- `web/src/protocol/terminal.ts`
- `web/src/features/*/api.ts`
- `internal/application/terminal/registry.go`
- `internal/application/agent/runtime_access.go`
- `internal/application/agent/client.go`
- 相关 tests

计划：

1. 前端 API request/response 类型重命名：
   - `CreateSessionRequest` → `CreateSessionReq`。
   - `UpdateSessionRequest` → `UpdateSessionReq`。
   - `RerunSessionRequest` → `RerunSessionReq`。
   - `CreateSessionResponse` → `CreateSessionResp`。
   - `ApiErrorResponse` → `ApiErrorResp`。
2. 后端 application/gateway contract 类型重命名：
   - `CreateSessionRequest` → `CreateSessionReq`。
   - `RerunSessionRequest` → `RerunSessionReq`。
   - `UpdateSessionRequest` → `UpdateSessionReq`。
   - `UpdateWorkspaceOrderRequest` → `UpdateWorkspaceOrderReq`。
   - `CreateSessionResponse` → `CreateSessionResp`。
3. 只重命名 API contract DTO；`SessionSummary`、`WorkspaceSummary`、`DeviceSummary` 这类 summary/display/domain projection 不强制改为 `Resp`，除非它们在实现中被明确作为某个 endpoint 的响应 wrapper。
4. 嵌套结构按需命名，不强制要求，例如 details field item、terminal event payload 可保留表达清晰的名称。
5. 重命名不得改变 JSON 字段名和业务语义。
6. 同步应用 ID 缩写规则：Go/TypeScript 标识符使用 `Id` / `requestId` / `deviceId` / `WorkspaceId` 等形式，不新增 `ID` 结尾标识符；既有触达的 API contract 类型和相关调用点按该规则收敛。

验收点：公开的前后端请求 DTO 以 `Req` 结尾，响应 DTO 以 `Resp` 结尾；触达标识符不使用 `ID` 结尾；嵌套结构不做机械改名。

### Step 8: Migrate feature APIs from fetch to axios client

修改：

- `web/src/protocol/terminal.ts`
- `web/src/features/sessions/api.ts`
- `web/src/features/gateway/api.ts`
- `web/src/features/workspaces/api.ts`
- 新增的 axios client 文件

计划：

1. `ApiErrorResp` 使用 best-practices 结构：
   ```ts
   export type ApiErrorResp = {
     code: string
     error: string
     requestId: string
     details?: unknown
   }
   ```
2. 移除或停止导出旧 `responseError(prefix, response)` 作为主要错误解析入口。
3. 所有 feature API 使用集中 axios client：
   - JSON 成功响应读取 `response.data`。
   - history text 成功响应要求 axios 使用 `responseType: 'text'` 或等价配置。
   - no-content 响应只等待请求完成。
4. axios response interceptor 只接受 best-practices error body；旧 `{ code, message, error }` 或纯文本错误不作为兼容路径处理，而是抛出明确的 API error contract mismatch。
5. UI Error message 由统一 `ApiClientError` 或等价 Error 格式化，包含 HTTP status、code、requestId 和 error 摘要。
6. 不让组件层直接读取 `details`。

### Step 9: Tests

新增或更新 Go tests：

1. `internal/transport/http/gatewayapi/server_test.go`
   - 未认证 `/api/devices` 返回 401 JSON：`code=unauthorized`、`requestId` 存在、无 `message` 字段。
   - login bad password 返回 401 JSON。
   - invalid JSON login 返回 400 JSON。
   - method not allowed 返回 405 JSON。
   - unknown API path 返回 404 JSON。
2. `internal/transport/http/gatewayapi/terminal_test.go`
   - device offline 返回 503 JSON `device_offline`，替换旧纯文本断言。
   - second terminal writer conflict 在 websocket dial 失败 response body 如可读取，则断言 `conflict`；若 websocket client 不暴露 body，可至少保证 status 不变，并通过 handler-level request 覆盖 conflict helper。
3. `internal/transport/http/gatewayapi/relay_test.go`
   - fake agent 返回 `OK:false` 时 gateway 返回 502 JSON `upstream_error`，默认配置不泄露 agent error body。
   - `DebugErrors: true` 时 `error` 字段包含可诊断底层 cause；`DebugErrors: false` 时不包含。
4. `internal/transport/http/middleware/requestlog/logging_test.go`
   - 现有 request id 日志测试应继续通过；如需要，补一条确保 `X-Request-ID` 回写和日志字段一致。

前端 tests：

- 新增轻量 Vitest test 覆盖 axios client：
  - request interceptor 自动写入 `X-Request-ID`。
  - 已存在 `X-Request-ID` 时不覆盖。
  - response interceptor 识别 `{ code, error, requestId, details? }` 并抛出统一错误。
  - 收到旧 `{ code, message, error }` 或纯文本错误时抛出 contract mismatch，不做向后兼容。
- 覆盖 history text 成功响应不被 JSON 解析破坏。
- 若 axios interceptor 测试需要 mock adapter，优先使用 Vitest mock axios adapter 能力或直接测试 helper；不要为了测试引入过重依赖。

## Files to change

### Expected new files

- `internal/transport/http/gatewayapi/errors.go`
- `web/src/features/api/client.ts` 或同等集中 axios client 文件。
- `web/src/features/api/client.test.ts` 或同等 axios client 单元测试。

### Expected modified files

- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/server_test.go`
- `internal/transport/http/gatewayapi/relay_test.go`
- `internal/transport/http/gatewayapi/terminal_test.go`
- `internal/transport/http/gatewayapi/auth/auth.go`
- `internal/transport/http/gatewayapi/auth/auth_test.go`，如新增 auth middleware 测试。
- `internal/transport/http/server/server.go`，仅当实现 API 404 兜底或 wrapper 必需时修改。
- `internal/protocol/tunnel/frame.go`
- `internal/application/terminal/registry.go`
- `internal/application/agent/runtime_access.go`
- `internal/application/agent/client.go`
- 相关 Go tests 中引用旧 `Request` / `Response` DTO 名称的位置。
- `web/package.json`
- 对应 lockfile（如依赖安装会更新）。
- `web/src/protocol/terminal.ts`
- `web/src/features/sessions/api.ts`
- `web/src/features/gateway/api.ts`
- `web/src/features/workspaces/api.ts`

### Not expected to change

- `internal/application/*` domain/application logic。
- `internal/protocol/tunnel/*` 既有 JSON 字段语义；允许为符合 `Req` / `Resp` 命名重命名 Go 类型，并追加 optional request id 字段，不改变现有 frame 行为。
- `internal/protocol/terminal/*` websocket protocol contract，除非实现阶段发现升级前错误必须补最小适配。
- 成功响应 DTO 字段语义和 API 路由结构；类型名会按 `Resp` 规则调整。
- `justfile` 或项目工具入口。
- git metadata 或 branch state。

## Verification plan

优先使用项目显式入口，但不修改 `justfile`。

### Backend

1. `go test ./internal/transport/http/gatewayapi/...`
2. `go test ./internal/transport/http/server/...`
3. `go test ./internal/transport/http/middleware/requestlog/...`
4. 如改动影响更广：`go test ./cmd/... ./internal/...`
5. `go vet ./cmd/... ./internal/...`

### Frontend

1. `yarn --cwd web typecheck`
2. `yarn --cwd web lint`
3. `yarn --cwd web test`
4. 如有格式风险：`yarn --cwd web format:check`

### Static/manual checks

- 搜索 `gatewayapi` 和 `gatewayapi/auth` 中剩余 `http.Error`。
- 静态检查错误响应不包含 `message`、`debug`、`stackTrace`、`traceback` 等字段。
- 手动或 test 确认：`X-Request-ID` header、body `requestId`、request log `request_id` 三者一致。
- 静态检查前后端 API contract DTO：request 类型以 `Req` 结尾，response 类型以 `Resp` 结尾；代码标识符使用 `Id` 而不是 `ID`；嵌套结构不做机械要求。
- 确认 success responses 未被 envelope 包装。

## Assumptions

1. `cfg.Web.Error.Debug` / `gatewayapi.Config.DebugErrors` 可直接作为 `error` 字段调试信息开关。
2. 现有 requestlog middleware 已经为每个请求生成并记录 request id；gatewayapi 只要读取同一 header 即可共享 request id。
3. 当前 Go server 未 serve frontend 静态资源；本轮只保证 API 404 结构化，不新增 frontend static fallback。
4. WebSocket upgrade 后的 terminal control protocol 继续保持现状，不纳入本轮结构化 HTTP error body。
5. 前端 UI 目前通过 Error message 展示失败；集中 axios client 抛出的统一错误需要保持现有 catch/toast 调用点可消费。

## Risks

1. `X-Request-ID` 由 requestlog middleware 生成并写响应 header，但 gatewayapi 单元测试直接调用 `gateway.ServeHTTP` 时可能绕过 requestlog；helper 需要能自行生成 request id，避免测试和直接调用无 request id。
2. requestlog 在 handler 前设置 header；gatewayapi helper 也会回写 header。二者必须使用同一 header 名，避免 body requestId 和日志 request_id 不一致。
3. `DebugErrors: true` 输出 cause 仍可能泄露敏感数据；实现中只能提供有限支持，不能承诺自动脱敏所有第三方错误文本。
4. `http.NotFound` 替换为 JSON 只覆盖 gateway handler 内部路径；Go ServeMux 对完全未挂载路径的默认 404 不一定进入 gateway handler。非 API 路径为 frontend serving 保留，不强行 JSON 化。
5. WebSocket client 对握手失败 body 的可读性有限，部分测试可能只能断言 status；HTTP helper 的 body 需要通过普通 handler path 补充覆盖。
6. axios client 如果格式化 request id 过长或过 noisy，可能影响 toast 文案可读性；但本轮优先保证可定位。
7. 结构化错误是 API contract change，且用户明确不向后兼容；历史测试和前端类型必须同步更新，不能只改后端。
8. 引入 axios 会更新依赖与 lockfile；实现前需要确认当前项目使用的包管理器和 lockfile 状态，不能手写依赖版本导致安装状态不一致。
9. `XxxxReq` / `XxxxResp` 命名会造成较多引用点重命名，尤其是 Go tests 和 TypeScript imports；实现时必须保持 JSON 字段不变，避免把类型名重构误变成 API wire format 变更。
10. `ID` → `Id` 是代码标识符规则，不是协议字段规则；实现时必须避免误改 `json:"..._id"`、URL 参数、localStorage key 或 tunnel JSON 字段。

## Rollback

如实施中出现难以收敛的问题：

1. 保留 `errors.go` helper 和新增 tests，先仅切换最小路径：unauthorized、bad request、method not allowed、device offline。
2. 对 relay / history / terminal ws pre-upgrade 路径逐个回退到旧行为时，必须同步更新 Plan/Verification 记录未完成项。
3. 不使用 git reset/stash/checkout 等写操作；回退通过 Edit/Write 精确修改。
4. 若发现 frontend serving 已在隐藏路径中依赖默认 404，优先缩小 API 404 结构化范围，不破坏 SPA fallback。

## User review notes

- 用户要求进入 Plan / 计划阶段；Requirement 已标记为 `Accepted`。
- 用户确认采用 best-practices 结构、requestId 接入日志、`error` 字段由配置控制、404 兜底服务前端且由 agent 自行评估。
- 用户补充要求：前端引入 axios，用 interceptor 生成/注入 request id 并统一处理异常；后端收到 request id 沿用并输出到下游和响应，没有则自行生成。
- 用户补充确认：不向后兼容旧错误结构，前后端一次性切换到 best-practices 结构。
- 用户补充要求：所有前后端请求结构使用 `XxxxReq`，所有前后端响应结构使用 `XxxxResp`；嵌套结构视具体需求，不强制要求。
- 用户补充要求：所有代码标识符中的 `AbcdID` 改为 `AbcdId`，`abcdID` 改为 `abcdId`；该规则不改变 JSON 字段名和外部协议字段。
- 本计划选择不新增 Spec 阶段，符合 standard 模式；实现前应先由用户 review Plan。
