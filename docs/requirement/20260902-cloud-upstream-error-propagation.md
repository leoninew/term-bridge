# Cloud 上游错误传播需求 / Cloud Upstream Error Propagation Requirement
最后修改时间: 2026-09-02 16:47:10

Review status: Accepted

## Background

本机 Agent 通过 `CloudApi` 调用 Cloud HTTP API，当前出站操作包括设备上报、身份探测、设备列表和 OAuth 授权码换 token。Cloud 已使用结构化 `ErrorResp` 返回 HTTP status、`code`、安全 `error` 文本和 request ID；但 Agent Cloud client 对非成功响应多数只构造字符串错误，Agent HTTP handler 又将其归为 `502 upstream_error`。

这会使本机 `GET /api/cloud/devices` 把 Cloud 的 `401 unauthorized` 错误表示为本地 `502`，前端无法按凭据失效处理。相同问题还可能影响设备上报的 `409 conflict`、Cloud 的其他已定义业务状态，以及未来新增的 Agent-to-Cloud HTTP 调用。

Cloud 用户 access token 的有效期由 `cloud.jwt.ttl` 配置控制，当前默认值为 `24h`。系统当前不提供 refresh token；用户 token 到期后应重新登录。

当前工作区存在此前为本问题产生的未提交局部补丁。该补丁不是本需求的已接受实现，也不能作为后续实现范围或验收依据；Implementation 阶段必须以本需求和 Plan 为准，明确替换或撤回其不完整设计。

## Goal

为 Agent 到 Cloud 的全部 HTTP 出站操作建立统一的远程错误模型，并保证 Cloud 已返回的 HTTP status 在对应本地 API 响应中原样继承。

当 Cloud 返回结构化错误时，Agent 本地应产生可由调用方和 handler 识别的 typed error，至少包含：

- `operation`：稳定的 Cloud 操作类型，而不是仅依赖 URL 或字符串描述；
- `upstreamStatus`：Cloud 实际返回的 HTTP status；
- `cloudError`：完整保留 Cloud 的结构化错误字段，其中包含 `code`、`error`、`request_id` 和已有 `details`；`cloudCode` 可作为其 `code` 的便捷访问字段。

本地 handler 必须使用该 typed error 输出既有结构化本地错误：顶层继续为当前 `ErrorResp{code,error,request_id,details?}`，HTTP status、`code`、安全 `error` 文案和已有 `details` 都沿用 Cloud 的结构化响应，不再嵌套或重复这些字段。本机入口取得的 `X-Request-ID` 必须写入当前调用链 context，并由任意 Agent-to-Cloud HTTP request 无条件读取和透传，因此 Cloud response 与本地 response 使用同一个 `request_id`。`operation` 仅用于 Agent 内部 typed error、日志与测试定位，不作为浏览器 error body 的重复字段。

对任何已收到的、非 `2xx` Cloud HTTP response，Agent 使用同一条通用传播逻辑，不为具体 operation 或具体 status 编写重写/映射分支。不得再把 Cloud 已定义业务错误泛化为 `502 upstream_error`。

## Non-goal

- 不新增 refresh token、token rotation、登录态续期或修改 `cloud.jwt.ttl` 的产品策略。
- 不改变 Cloud 直连 API 的既有认证、授权和业务状态定义。
- 不修改 Agent tunnel runtime 的 `tunnel.RemoteError` 传播模型；本需求仅覆盖 Agent 调用 Cloud HTTP API 的出站边界。
- 不引入新的成功响应 envelope，也不将 HTTP status 复制到成功响应或通用 JSON body 顶层。
- 不将 Cloud 的原始响应 body、认证头、token、client secret 或未审查的内部错误文本暴露给浏览器。

## User scenarios

### 1. 本机已登录用户的 token 过期后查看设备

1. 浏览器携带已过期 Cloud bearer 调用本机 `GET /api/cloud/devices`。
2. Agent 调用 Cloud `GET /api/devices`，Cloud 返回 `401` 和 `code: "unauthorized"`。
3. Agent 建立 `operation=list_devices`、`upstreamStatus=401`、`cloudCode=unauthorized` 的 typed error。
4. 本机 API 返回 HTTP `401` 和 `code: "unauthorized"`，而不是 `502 upstream_error`。
5. 前端将该请求识别为使用用户 Cloud 凭据的请求，清理已失效凭据并进入既有登录恢复流程。

### 2. 本机连接设备时 Cloud 拒绝设备身份冲突

1. 浏览器携带有效 Cloud bearer 调用本机 `POST /api/cloud/connect`。
2. Agent 上报当前设备，Cloud 返回 `409` 和 `code: "conflict"`。
3. 本地 API 返回 HTTP `409` 与 `code: "conflict"`，供界面展示既有业务错误；不得转换为 `502`。
4. 前端不得因为非 `401` 的 Cloud 业务错误清除用户 token。

### 3. 本机启动时探测已保存的 Cloud 凭据

1. 浏览器调用本机 `GET /api/cloud/auth/me` 验证本地保存的 Cloud token。
2. Cloud 当前对无效或过期 token 的正常语义是 `200` 与 `authenticated:false`；Agent 将其作为成功响应原样返回，不改写为 HTTP error。
3. 如 Cloud 在该操作返回异常非 `2xx` 响应，Agent 与其他 Cloud 操作一样按统一 typed error 传播其 status 和结构化错误字段。
4. 网络失败、超时、无结构化响应或成功响应解码失败仍与“已收到 Cloud 业务错误”区分处理。

### 4. OAuth 授权码换 token 失败

1. 浏览器调用本机 OAuth 换码接口，Agent 使用 OAuth client 配置调用 Cloud token endpoint。
2. Cloud 对过期/无效授权码返回业务 `4xx` 时，本机 API 继承该 status 和 Cloud error code。
3. Agent 不为 OAuth 换码错误创建 status/code 的专用重写逻辑；调用方根据请求自身的凭据语义处理后续 UI 行为。

### 5. Cloud 无法连接或违反错误契约

1. Agent 无法建立 Cloud HTTP 连接、请求超时、读取 body 失败，或收到无法解析为结构化错误的异常响应。
2. Agent 仍创建可诊断的 typed error，但不得伪造 Cloud error code。
3. 对已收到的非 `2xx` 异常响应，即使 body 不符合 `ErrorResp`，本地 API 仍沿用该 HTTP status，使用本地 `upstream_error` 表示上游协议失败。
4. 对不存在可继承上游错误 status 的传输故障，以及 Cloud `2xx` 成功响应无法按成功 DTO 解码的协议故障，本地 API 保留 `502 upstream_error`；日志保留安全诊断上下文。

## Acceptance

- [ ] Agent `CloudApi` 的所有 HTTP 出站操作对 Cloud 非成功响应使用统一 typed error；typed error 至少可读取 `operation`、`upstreamStatus` 和 `cloudCode`。
- [ ] Cloud 返回结构化 `ErrorResp` 时，本地 HTTP handler 返回完全相同的 HTTP status、`ErrorResp.code`、安全 `ErrorResp.error` 与 `ErrorResp.details`；不新增 ErrorResp 顶层字段，也不对 Cloud error 再套一层 details。
- [ ] 本机入口生成或接收的 `X-Request-ID` 会写入当前调用链 context，并由所有 Agent-to-Cloud HTTP request 无条件透传至 Cloud；Cloud error 的 `request_id` 与本地 response 的 `request_id` 相同。
- [ ] `GET /api/cloud/devices` 的 Cloud `401 unauthorized` 在本地仍是 `401 unauthorized`，不会成为 `502 upstream_error`。
- [ ] `POST /api/cloud/connect` 的 Cloud `409 conflict` 在本地仍是 `409 conflict`。
- [ ] OAuth 换码操作的 Cloud `4xx` 与其他 Cloud HTTP response 使用相同的传播逻辑继承 status/code/error/details，不存在专用的 Agent status/code 改写分支。
- [ ] Cloud 身份探测的正常 `200 + authenticated:false` 保持成功响应；异常非 `2xx` 与其他 Cloud HTTP response 使用相同的传播逻辑。
- [ ] 前端可依据原样继承的 HTTP status、错误字段和请求自身的凭据语义处理状态；Agent 不通过 URL 前缀、operation 或具体 status 增加业务改写。
- [ ] 没有上游 HTTP response 的传输故障，以及 `2xx` 成功响应的解码失败，保留本地 `502 upstream_error`；已收到但不符合 `ErrorResp` 的非 `2xx` 响应沿用上游 status。typed error 与日志保留 operation 和安全失败类别，浏览器 response 不伪造或嵌套 Cloud details。
- [ ] JWT 在 `exp` 所表示的到期时刻及之后不再有效；不改变配置的 token TTL。
- [ ] 测试覆盖四个 Agent-to-Cloud HTTP 操作的成功路径、结构化上游错误传播、传输/协议失败 fallback，以及前端 token 清理边界。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- Agent 到 Cloud 的错误类型以“操作”而非“接口路径”标识，避免路由改名影响错误分类、日志与调用方分支。
- 对已收到且可解析的 Cloud 结构化错误，status、`code`、`error`、request ID 和已有 details 是 Cloud 错误契约的一部分，必须由 typed error 保留，并由本地 response 直接沿用。
- 本地顶层 `ErrorResp` 结构保持不变；本机入口 request ID 会透传到 Cloud，因此不存在需要在 local/cloud 间选择或嵌套两个 request ID 的情况。
- 对所有已收到的非 `2xx` Cloud HTTP response，Agent 仅执行通用“解析 typed error → 原样输出 status/code/error/request_id/details”逻辑；不按 operation、`401`、`409` 或任何其他具体 status 写分支改写。
- Cloud 无响应、或 `2xx` 成功响应无法按成功 DTO 解码时不存在可继承的上游错误，本地保留 `502 upstream_error` fallback；已收到但无法解析为 `ErrorResp` 的非 `2xx` 仍继承该上游 status。
- 本需求不接受仅为 `devices` 或 `connect` 增加独立 sentinel error 的局部实现；统一模型必须覆盖 Agent 当前全部 Cloud HTTP 出站操作，并成为未来操作的复用入口。
- Request ID 透传以调用链 context 和 Cloud HTTP transport 为唯一机制；不得在 handler、service 或 operation 方法签名中重复传递 request ID，也不得按 Cloud operation 建立透传分支。

## Risk

- 直接透传 Cloud status/code 会扩大本地 API 对 Cloud 错误契约的依赖；Cloud 错误结构需要继续保持稳定并有测试保障。
- 调用方若未根据自身凭据语义处理原样继承的 HTTP error，可能造成误登出或掩盖真实配置错误；该风险不应由 Agent 通过 status 重写掩盖。
- 现有未提交局部补丁若未在 Implementation 前明确替换或撤回，会与统一 typed error 方案重复或留下 URL 前缀判断等错误边界。
- 错误类型、handler 映射和前端响应拦截器跨 Agent、Cloud 和 Web 三层；Plan 必须列出所有调用点和每类错误的期望 status/code。

## User review notes

- 用户明确要求：远程错误在本地以“操作类型 + 上游状态 + Cloud 错误码”的 typed error 反馈。
- 用户明确要求：远程错误的 HTTP status 在本地沿用上游状态码。
- 用户确认：typed error 沿用 Cloud 字段和状态码；本地既有错误结构不变，Cloud 原有 `details` 直接沿用。
- 用户确认：不为具体 operation 或具体 HTTP status 编写错误重写逻辑；没有区分的错误保持统一传播。
- 用户补充确认：HTTP status、`code`、`error`、`request_id` 和 Cloud 原有 `details` 已由本地 response 直接沿用，不重复嵌套；请求 ID 必须从本机入口透传至 Cloud，`operation` 只留在内部 typed error 和日志。
- 用户补充确认：任何 Agent-to-Cloud 请求都直接携带当前调用链的 `X-Request-ID`；不按 operation 或调用场景区分。
