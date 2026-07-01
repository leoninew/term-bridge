# API Inventory Requirement
最后修改时间: 2026-07-01 14:50:45

## Review status

Accepted

## Flow mode / Stage

轻量模式 / light；需求 / Requirement 已接受，用户已显式要求进入规格 / Spec。

## Background

用户提出新任务：需要对当前 TermBridge-go 项目已有后端 API 做一次摸底，明确目前有哪些 API，以及每个 API 的请求方法、路径、请求体和响应体。

该任务以代码事实为准，目标是形成当前实现的后端 API 清单，便于后续评审、文档补齐、前后端对齐或接口治理。

## Goal

1. 梳理当前项目后端暴露的 HTTP API 和 WebSocket API。
2. 对每个 API 记录：
   - 请求方法 / method。
   - 请求路径 / path。
   - 请求体 / request body；没有请求体时明确标注无。
   - 响应体 / response body；记录响应体名称和定义，不展开所有具体字段。
3. 标注 API 所属模块或用途，例如 auth、sessions、terminal、cloud binding、gateway 等。
4. 尽量关联前端调用代码，辅助确认接口是否实际被前端使用。
5. 摸底结果直接落实在本文件中。
6. 标记接口是否遵从显式 `Req` / `Resp` 请求响应类型命名；对匿名 struct、map/dict、数组、文本、空响应和 WebSocket frame 等非标准形态明确标注。
7. 补充前端 API client、封装函数、类型定义来源和前后端契约差异说明。

## Non-goal

1. 本任务不修改 API 行为。
2. 本任务不新增、删除或重命名接口。
3. 本任务不重构后端 route、handler、DTO 或前端 API client。
4. 本任务不生成 OpenAPI/Swagger 代码，除非用户后续明确要求。
5. 本任务不执行 git 写操作。
6. 本任务不把前端路由或静态资源文件清单作为业务 API 摸底对象。

## Acceptance

- [x] 找到当前后端注册 API route 的主要入口。
- [x] 覆盖普通 HTTP API 与 WebSocket/tunnel 类接口。
- [x] 每个接口至少记录 method、path、request body、response body。
- [x] 对无法从代码静态确认的字段或响应明确标注“不确定”或“需运行验证”，不臆造。
- [x] 区分成功响应与通用错误响应模型。
- [x] 尽量指出对应的前端调用位置或未发现前端调用。
- [x] 输出结果可供后续补 API 文档或 OpenAPI 规格使用。
- [x] 标记未遵从显式 `Req` / `Resp` 类型命名的接口和响应形态。
- [x] 补充前端 API 封装、类型定义和调用说明。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- 用户已明确这是“新任务”，因此创建新的 requirement 文档，不沿用已有配置或打包任务文档。
- 该任务当前按轻量模式 / light 处理。
- 用户确认摸底结果直接落实在 `docs/requirement/20260701-api-inventory.md`。
- “API”范围限定为后端接口；不把前端路由或静态资源文件清单作为业务 API 摸底对象。
- 响应体记录响应体名称和定义，不需要展开所有具体字段。
- 用户已明确要求“直接实现”，视为 Requirement / 需求已接受并进入 Implementation / 实现。
- 用户补充要求：部分接口没有遵从 `Req` / `Resp` 形式，请在清单中标记；数组、字典/map、文本、空响应等响应形态也需要标记。
- 用户补充要求：需要添加前端相关说明，包括 API client、封装函数、类型定义来源和前后端契约差异。
- 用户显式要求进入 Spec / 规格阶段，并要求列出后续待修改接口列表，作为实现 checklist。

## Risk

1. 如果 handler 复用通用响应包装或错误处理中间件，仅靠 route 注册点可能无法完整识别响应体，需要继续追踪 handler、DTO、前端调用和测试。
2. WebSocket 接口的 request/response 可能是帧协议而不是 JSON body，需要单独描述握手参数和消息格式。
3. 部分接口只在 cloud/local mode 下启用，摸底时需要标注运行模式或配置依赖。
4. 当前工作区已有其他未提交改动，摸底应避免修改产品代码或覆盖用户已有工作。
5. 本次为静态代码摸底，未启动服务逐个请求验证实际响应。

## API inventory

### Scope and sources

本清单基于静态代码摸底，主要来源：

- Route 注册入口：`internal/transport/http/gatewayapi/server.go` 的 `Handler.Handler()`。
- HTTP server 挂载入口：`internal/transport/http/server/server.go`，仅 `/api` 与 `/api/` 挂载到 gateway API handler；静态资源 handler 不纳入业务 API 清单。
- HTTP 错误响应：`internal/transport/http/gatewayapi/errors.go`。
- Browser terminal WebSocket 协议：`internal/protocol/terminal/protocol.go`。
- Agent tunnel WebSocket 协议：`internal/protocol/tunnel/frame.go`。
- Agent relay method 到本地 runtime 的映射：`internal/application/agent/client.go`。
- Terminal runtime request / response DTO：`internal/application/terminal/registry.go`。
- 前端调用参考：`web/src/features/gateway/api.ts`、`web/src/features/workspaces/api.ts`、`web/src/features/sessions/api.ts`、`web/src/features/api/client.ts`、`web/src/protocol/terminal.ts`。

### DTO naming and payload shape markers

本清单使用以下标记描述请求/响应体在代码中的形态：

- `Explicit Req/Resp`：后端存在明确命名的 Go 类型，且名称本身接近 `*Req` / `*Resp`。
- `Anonymous request`：后端 handler 内部使用匿名 struct 解码请求体，没有独立 `Req` 类型。
- `Map response`：后端直接返回 `map[string]...` / `map[string]any`，没有独立 `Resp` 类型。
- `Array response`：成功响应体直接是数组，例如 `DeviceSummary[]` 或 `WorkspaceSummary[]`。
- `Plain text response`：成功响应体是 `text/plain`，不是 JSON object。
- `NoContent`：成功响应为 `204 No Content`，没有响应体。
- `Null relay result`：经 tunnel relay 返回的 JSON 结果为 `null`，前端通常按 `void` 处理。
- `WebSocket frame`：不是传统 HTTP JSON request/response，而是 WebSocket upgrade 后的消息协议。
- `Frontend-only type`：前端定义了 TypeScript 类型名称，但后端没有同名 Go DTO；该名称只能代表前端契约视角。

### DTO shape exceptions and non-Req/Resp endpoints

以下接口或协议没有完全遵从显式 `Req` / `Resp` 命名形式，或响应不是 JSON object：

| Area | Method / Path | Request shape | Response shape | 标记 |
|---|---|---|---|---|
| Health | `GET /api/health` | 无 | `map[string]string` | `Map response`，无后端 `HealthResp` 类型 |
| Auth | `POST /api/auth/login` | handler 匿名 struct | `map[string]string` | `Anonymous request` + `Map response`；`TokenResp` 仅是前端/文档名称 |
| Auth | `POST /api/auth/logout` | 无 | `map[string]bool` | `Map response`；无后端 `LogoutResp` 类型 |
| Auth | `GET /api/auth/me` | 无 | `map[string]any`，嵌入 `authapp.UserView` / `authapp.Capabilities` / `CloudSessionSummary` | `Map response`；前端有 `AuthMeResp` |
| Auth | `POST /api/auth/register` | handler 匿名 struct | `map[string]bool` | `Anonymous request` + `Map response` |
| Auth | `POST /api/auth/email/verify` | handler 匿名 struct | `map[string]bool` | `Anonymous request` + `Map response` |
| Auth | `POST /api/auth/email/verification/resend` | handler 匿名 struct | `map[string]bool` | `Anonymous request` + `Map response` |
| Auth | `POST /api/auth/password/change` | handler 匿名 struct | `map[string]bool` | `Anonymous request` + `Map response` |
| Auth | `POST /api/auth/password-reset/request` | handler 匿名 struct | `map[string]bool` | `Anonymous request` + `Map response` |
| Auth | `POST /api/auth/password-reset/confirm` | handler 匿名 struct | `map[string]bool` | `Anonymous request` + `Map response` |
| Auth | `GET /api/auth/google` | 无 | `map[string]string` | `Map response`；无后端 `GoogleAuthURLResp` 类型 |
| Auth | `POST /api/auth/google/callback` | handler 匿名 struct | `map[string]string` | `Anonymous request` + `Map response`；前端用 `TokenResp` |
| Cloud OAuth | `GET /api/cloud-oauth/start` | query 参数 | `map[string]string` | `Map response`；无后端 `CloudOAuthStartResp` 类型 |
| Cloud OAuth | `POST /api/cloud-oauth/callback` | handler 匿名 struct | `map[string]any`，嵌入 `CloudSessionSummary` | `Anonymous request` + `Map response`；前端有 `CloudOAuthCallbackResp` |
| Cloud OAuth | `GET /api/cloud-oauth/authorize` | query 参数 | `map[string]string` | `Map response`；无后端 `CloudOAuthAuthorizeResp` 类型 |
| Cloud OAuth | `POST /api/cloud-oauth/exchange` | handler 匿名 struct | `map[string]string` | `Anonymous request` + `Map response`；前端未直接调用 |
| Devices | `GET /api/devices` | 无 | `[]DeviceSummary` | `Array response` |
| Devices | `POST /api/devices/current` | handler 匿名 struct | `map[string]any`，嵌入 `DeviceSummary` | `Anonymous request` + `Map response` |
| Devices | `DELETE /api/devices/{deviceId}` | 无 | 空 | `NoContent` |
| Workspaces | `GET /api/devices/{deviceId}/workspaces` | 无 | `[]terminal.WorkspaceSummary` | `Array response` |
| Workspaces | `GET /api/devices/{deviceId}/workspaces/tree` | 无 | `[]terminal.WorkspaceTreeNode` | `Array response` |
| Workspaces | `PATCH /api/devices/{deviceId}/workspaces/order` | handler 匿名 struct，结构同 `terminal.UpdateWorkspaceOrderReq` | `[]terminal.WorkspaceSummary` | `Anonymous request` + `Array response` |
| Workspaces | `DELETE /api/devices/{deviceId}/workspaces/{workspaceId}` | 无 | tunnel result 为 `null` | `Null relay result`；前端按 `void` 使用 |
| Sessions | `GET /api/devices/{deviceId}/workspaces/{workspaceId}/sessions` | 无 | `[]terminal.WorkspaceSessionSummary` | `Array response` |
| Sessions | `PATCH /api/devices/{deviceId}/workspaces/{workspaceId}/sessions/order` | handler 匿名 struct，结构同 `terminal.UpdateSessionOrderReq` | `[]terminal.WorkspaceSessionSummary` | `Anonymous request` + `Array response` |
| Sessions | `DELETE /api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}` | 无 | 空 | `NoContent` |
| Sessions | `GET /api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/history` | 无 | `text/plain` | `Plain text response` |
| Browser terminal WS | `GET /api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/ws` | WebSocket text/binary message | WebSocket text/binary message | `WebSocket frame`，不适用 HTTP `Req` / `Resp` |
| Agent tunnel WS | `GET /api/agent/tunnel` | WebSocket `tunnel.Frame` | WebSocket `tunnel.Frame` | `WebSocket frame`；内部 payload 有 `RequestReq` / `ResponseResp` |

仍然相对接近显式 `Req` / `Resp` 的接口主要是 terminal runtime relay 相关类型：

- `terminal.CreateSessionReq` / `terminal.CreateSessionResp`
- `terminal.RerunSessionReq` / `terminal.CreateSessionResp`
- `terminal.UpdateSessionReq` / `terminal.SessionSummary`
- `terminal.UpdateWorkspaceOrderReq` / `terminal.WorkspaceSummary[]`，但 HTTP handler 当前用匿名 struct 解码。
- `terminal.UpdateSessionOrderReq` / `terminal.WorkspaceSessionSummary[]`，但 HTTP handler 当前用匿名 struct 解码。
- `tunnel.RequestReq` / `tunnel.ResponseResp`

### Common response and auth model

#### Common error response

- 名称：`gatewayapi.errorResponse` / frontend `ApiErrorResp`
- 定义：所有 gateway JSON API 错误的通用响应体，包含错误码、面向客户端的错误文案、请求 ID 和可选详情。
- 后端定义：`internal/transport/http/gatewayapi/errors.go`
- 前端定义：`web/src/protocol/terminal.ts`
- 常见状态码：`400`、`401`、`403`、`404`、`405`、`409`、`429`、`500`、`502`、`503`。

#### Request ID

- 请求头：`X-Request-ID`
- 定义：客户端可传入；缺失时后端生成并写回响应头。错误响应体中的 `requestId` 与响应头保持同一语义。
- 前端行为：`web/src/features/api/client.ts` 自动补充 `X-Request-ID`。

#### Browser auth

- 主要形式：`Authorization: Bearer <token>`。
- 兼容形式：`token` query 参数可被 token 提取逻辑识别，当前主要用于 terminal WebSocket URL。
- local mode：部分 `authMiddleware` 保护接口会绕过登录态；但个别 cloud/account 相关接口仍要求 cloud mode 或有效 claims。

#### Agent tunnel auth

- 路径：`GET /api/agent/tunnel`
- 若配置了 device public key 或 device repository：使用设备签名请求头。
  - `X-TermBridge-Device-ID`
  - `X-TermBridge-Device-Timestamp`
  - `X-TermBridge-Device-Nonce`
  - `X-TermBridge-Device-Signature`
- 否则：使用 HTTP Basic auth。

## Response body definitions

以下只记录响应体名称和定义，不展开全部字段。

### Gateway / auth response bodies

- `HealthResp`：健康检查响应，表示后端 API 可达。
- `TokenResp`：登录、Google callback、cloud oauth exchange 成功后的 bearer token 响应。
- `OkResp`：通用确认响应，表示请求已接收或操作成功。
- `AuthMeResp`：当前认证状态、用户信息、认证能力和本机 cloud session 摘要。
- `authapp.UserView`：已认证用户展示信息。
- `authapp.Capabilities`：当前认证模式、可用 provider、密码重置/邮箱验证/账号认证/cloud oauth 能力。
- `GoogleAuthURLResp`：Google OAuth 授权 URL 响应。
- `CloudOAuthStartResp`：本机发起 cloud OAuth 绑定时返回的云端授权 URL。
- `CloudOAuthAuthorizeResp`：云端授权本机绑定后返回的本机 callback URL。
- `CloudOAuthCallbackResp`：本机完成 cloud OAuth callback 后返回 cloud session 和后续跳转路径。
- `CloudSessionSummary`：本机连接到 cloud gate 的摘要。
- `CurrentDeviceResp`：cloud gate 接收本机设备上报后的确认响应和设备摘要。
- `DeviceSummary`：设备展示摘要，含在线状态和最近连接/更新时间。
- `NoContent`：`204 No Content`，响应体为空。

### Terminal / workspace response bodies

- `terminal.WorkspaceSummary`：workspace 列表项摘要。
- `terminal.WorkspaceTreeNode`：workspace 树节点，包含 workspace 摘要和其 session 子节点。
- `terminal.WorkspaceSessionSummary`：workspace tree 或 workspace session 列表中的 session 摘要。
- `terminal.SessionSummary`：单个 session 的展示摘要。
- `terminal.CreateSessionResp`：创建或 rerun session 后返回 session id、workspace id 和初始状态。
- `HistoryText`：session history 的 `text/plain` 文本响应。

### WebSocket protocol bodies

- `terminalproto.ClientMessage`：浏览器 terminal WebSocket 文本控制消息。
- `terminalproto.ServerMessage`：浏览器 terminal WebSocket 文本控制响应消息。
- `Binary terminal data`：浏览器 terminal WebSocket 二进制输入/输出数据。
- `tunnel.Frame`：agent tunnel WebSocket 顶层帧。
- `tunnel.HelloPayload`：agent 建立 tunnel 后发送的 hello payload。
- `tunnel.HelloAckPayload`：gateway 对 agent hello 的确认 payload。
- `tunnel.RequestReq`：gateway 经 tunnel 请求 agent runtime 的 RPC 请求 payload。
- `tunnel.ResponseResp`：agent 经 tunnel 返回 gateway 的 RPC 响应 payload。
- `tunnel.TerminalAttachReq`：gateway 经 tunnel 请求 agent 附着 terminal session 的 payload。
- `tunnel.TerminalDataPayload`：terminal 输入/输出二进制数据经 tunnel 封装后的 payload。
- `tunnel.TerminalResizePayload`：terminal resize 经 tunnel 封装后的 payload。
- `tunnel.ErrorPayload`：agent tunnel 错误 payload。

## HTTP API list

### Health

| Method | Path | Request body | Success response | 前端调用 |
|---|---|---|---|---|
| `GET` | `/api/health` | 无 | `HealthResp`：健康状态响应 | 未发现前端调用 |

### Auth / account

| Method | Path | Request body | Success response | 条件 / 备注 | 前端调用 |
|---|---|---|---|---|---|
| `POST` | `/api/auth/login` | `AuthLoginReq`：邮箱或用户名 + 密码登录请求 | `TokenResp` | cloud web mode；local mode 返回 `404` | `authLogin()` |
| `POST` | `/api/auth/logout` | 无 | `OkResp` | `authMiddleware`；local mode 下中间件放行 | `authLogout()` |
| `GET` | `/api/auth/me` | 无 | `AuthMeResp` | local/cloud 都存在；cloud 下无 token 时返回未认证状态 | `authMe()` |
| `POST` | `/api/auth/register` | `AuthRegisterReq`：邮箱 + 密码注册请求 | `OkResp`，状态码 `202` | cloud web mode；需要 `AuthService` | `authRegister()` |
| `POST` | `/api/auth/email/verify` | `AuthVerifyEmailReq`：邮箱 + 验证码 | `OkResp` | cloud web mode；需要 `AuthService` | `authVerifyEmail()` |
| `POST` | `/api/auth/email/verification/resend` | `AuthResendVerificationReq`：邮箱 | `OkResp`，状态码 `202` | cloud web mode；需要 `AuthService` | `authResendVerification()` |
| `POST` | `/api/auth/password/change` | `AuthChangePasswordReq`：当前密码 + 新密码 | `OkResp` | cloud web mode；需要有效 bearer token | `authChangePassword()` |
| `POST` | `/api/auth/password-reset/request` | `AuthPasswordResetRequestReq`：邮箱 | `OkResp`，状态码 `202` | cloud web mode；即使邮箱不存在也返回 accepted | `authPasswordResetRequest()` |
| `POST` | `/api/auth/password-reset/confirm` | `AuthPasswordResetConfirmReq`：邮箱 + 验证码 + 新密码 | `OkResp` | cloud web mode | `authPasswordResetConfirm()` |
| `GET` | `/api/auth/google` | 无 | `GoogleAuthURLResp` | cloud web mode；需要 Google OAuth 已配置 | `authGoogleURL()` |
| `POST` | `/api/auth/google/callback` | `AuthGoogleCallbackReq`：OAuth code + state | `TokenResp` | cloud web mode；需要 Google OAuth 已配置 | `authGoogleCallback()` |

### Cloud OAuth / device binding

| Method | Path | Request body | Success response | 条件 / 备注 | 前端调用 |
|---|---|---|---|---|---|
| `GET` | `/api/cloud-oauth/start` | 无；可带 query `redirect` | `CloudOAuthStartResp` | local web mode 且 cloud OAuth enabled | `cloudOAuthStart()` |
| `POST` | `/api/cloud-oauth/callback` | `CloudOAuthCallbackReq`：OAuth code + state | `CloudOAuthCallbackResp` | local web mode 且 cloud OAuth enabled；内部会调用 cloud gate exchange 和 device report | `cloudOAuthCallback()` |
| `GET` | `/api/cloud-oauth/authorize` | 无；query `client_id`、`redirect_uri`、`state` | `CloudOAuthAuthorizeResp` | cloud web mode；需要有效非 local-admin bearer token | `cloudOAuthAuthorize()` |
| `POST` | `/api/cloud-oauth/exchange` | `CloudOAuthExchangeReq`：binding code | `TokenResp` | cloud web mode；本机 callback 内部调用，前端未直接调用 | 未发现前端直接调用 |

### Devices

| Method | Path | Request body | Success response | 条件 / 备注 | 前端调用 |
|---|---|---|---|---|---|
| `GET` | `/api/devices` | 无 | `DeviceSummary[]` | `authMiddleware`；local mode 可返回本机设备或 registry 设备 | `listDevices()` |
| `POST` | `/api/devices/current` | `CurrentDeviceReq`：当前设备 id/name/public key 上报 | `CurrentDeviceResp` | 需要 `DeviceRepository` 和有效 bearer token；本机 cloud OAuth callback 内部调用 | 未发现前端直接调用 |
| `DELETE` | `/api/devices/{deviceId}` | 无 | `NoContent` | 需要有效非 local-admin bearer token 且用户拥有该设备 | `deleteDevice()` |

### Workspaces via device relay

这些接口由 gateway 接收 HTTP 请求，再通过 agent tunnel 转发到对应 device 的 local runtime。device 离线时，部分接口可返回缓存，并在响应头写入 `X-TermBridge-Offline: true`。

| Method | Path | Request body | Tunnel method | Success response | Offline cache | 前端调用 |
|---|---|---|---|---|---|---|
| `GET` | `/api/devices/{deviceId}/workspaces` | 无 | `workspaces` | `terminal.WorkspaceSummary[]` | 无 | `listWorkspaces()` |
| `GET` | `/api/devices/{deviceId}/workspaces/tree` | 无 | `workspace_tree` | `terminal.WorkspaceTreeNode[]` | 有 | `listWorkspaceTree()` |
| `PATCH` | `/api/devices/{deviceId}/workspaces/order` | `terminal.UpdateWorkspaceOrderReq`：workspace id 排序列表 | `workspace_order` | `terminal.WorkspaceSummary[]` | 无 | `updateWorkspaceOrder()` |
| `DELETE` | `/api/devices/{deviceId}/workspaces/{workspaceId}` | 无 | `delete_workspace` | 当前代码经 `handleJSONRelay` 返回 agent result；runtime 删除成功 result 为 `null` | 无 | `deleteWorkspace()` |

### Sessions via device relay

| Method | Path | Request body | Tunnel method | Success response | 前端调用 |
|---|---|---|---|---|---|
| `POST` | `/api/devices/{deviceId}/sessions` | `terminal.CreateSessionReq` | `create_session` | `terminal.CreateSessionResp`，状态码 `201` | `createSession()` 在无 workspaceId 时调用 |
| `GET` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions` | 无 | `workspace_sessions` | `terminal.WorkspaceSessionSummary[]` | 未发现前端直接调用；前端当前主要用 tree |
| `POST` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions` | `terminal.CreateSessionReq`；gateway 会补充 path 中的 `workspace_id` | `create_session` | `terminal.CreateSessionResp`，状态码 `201` | `createSession()` 在有 workspaceId 时调用 |
| `PATCH` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/order` | `terminal.UpdateSessionOrderReq`：session id 排序列表 | `session_order` | `terminal.WorkspaceSessionSummary[]` | `updateSessionOrder()` |
| `GET` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}` | 无 | `get_session` | `terminal.SessionSummary` | `getSession()` |
| `PATCH` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}` | `terminal.UpdateSessionReq` | `update_session` | `terminal.SessionSummary` | `updateSession()` |
| `DELETE` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}` | 无 | `delete_session` | `NoContent` | `deleteSession()` |
| `POST` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/close` | 无 | `close_session` | `terminal.SessionSummary` | `closeSession()` |
| `POST` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/rerun` | `terminal.RerunSessionReq` | `rerun_session` | `terminal.CreateSessionResp` | `rerunSession()` |
| `GET` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/history` | 无 | `history` | `HistoryText`，`Content-Type: text/plain; charset=utf-8` | `readHistory()` |

## WebSocket API list

### Browser terminal WebSocket

| Method | Path | Request body | Response body | 前端调用 |
|---|---|---|---|---|
| `GET` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/ws` | HTTP upgrade；query 可带 `token`、`cols`、`rows`；WebSocket subprotocol `termbridge.terminal.v1` | WebSocket 文本消息为 `terminalproto.ServerMessage`，二进制消息为 terminal output bytes | `terminalWsUrl()` + `useTerminalSocket()` |

#### Browser terminal WebSocket client messages

- 文本消息：`terminalproto.ClientMessage`
  - 定义：浏览器发给 gateway 的 terminal 控制消息，包括 hello、resize、detach、ping。
- 二进制消息：`Binary terminal data`
  - 定义：浏览器发给 terminal 的原始输入 bytes。

#### Browser terminal WebSocket server messages

- 文本消息：`terminalproto.ServerMessage`
  - 定义：gateway 发给浏览器的 terminal 控制消息，包括 started、replay_started、replay_finished、state、exited、error、pong。
- 二进制消息：`Binary terminal data`
  - 定义：terminal 输出 bytes，浏览器写入 xterm。

### Agent tunnel WebSocket

| Method | Path | Request body | Response body | 前端调用 |
|---|---|---|---|---|
| `GET` | `/api/agent/tunnel` | HTTP upgrade；认证使用 device signature headers 或 Basic auth；升级后 agent 首帧必须是 `tunnel.Frame` + `tunnel.HelloPayload` | WebSocket frame 均为 `tunnel.Frame`；hello 成功后 gateway 返回 `tunnel.HelloAckPayload` | 后端 agent client 调用，前端不直接调用 |

#### Agent tunnel frame protocol

- 顶层帧：`tunnel.Frame`
  - 定义：所有 agent tunnel WebSocket 文本消息的统一 envelope。
- 初始握手：
  - agent -> gateway：`FrameHello` + `tunnel.HelloPayload`
  - gateway -> agent：`FrameHelloAck` + `tunnel.HelloAckPayload`
- Gateway RPC 到 agent runtime：
  - gateway -> agent：`FrameRequest` + `tunnel.RequestReq`
  - agent -> gateway：`FrameResponse` + `tunnel.ResponseResp`
- Gateway terminal attach 到 agent runtime：
  - gateway -> agent：`FrameTerminalAttach` + `tunnel.TerminalAttachReq`
  - gateway -> agent：`FrameTerminalInput` + `tunnel.TerminalDataPayload`
  - gateway -> agent：`FrameTerminalResize` + `tunnel.TerminalResizePayload`
  - agent -> gateway：`FrameTerminalOutput` + `tunnel.TerminalDataPayload`
  - agent -> gateway：`FrameTerminalClosed` 或 `FrameError` + `tunnel.ErrorPayload`
- Keepalive / close：
  - `FramePing`、`FramePong`、`FrameClose`

## Request body definitions

以下记录请求体名称和定义来源，不展开所有字段细节。

### Auth / account request bodies

- `AuthLoginReq`：匿名 handler struct，登录凭据请求；定义在 `handleAuthLogin`。
- `AuthRegisterReq`：匿名 handler struct，注册邮箱账号请求；定义在 `handleRegister`。
- `AuthVerifyEmailReq`：匿名 handler struct，邮箱验证码验证请求；定义在 `handleVerifyEmail`。
- `AuthResendVerificationReq`：匿名 handler struct，重发邮箱验证码请求；定义在 `handleResendVerification`。
- `AuthChangePasswordReq`：匿名 handler struct，修改密码请求；定义在 `handleChangePassword`。
- `AuthPasswordResetRequestReq`：匿名 handler struct，请求密码重置验证码；定义在 `handlePasswordResetRequest`。
- `AuthPasswordResetConfirmReq`：匿名 handler struct，确认密码重置；定义在 `handlePasswordResetConfirm`。
- `AuthGoogleCallbackReq`：匿名 handler struct，Google OAuth callback code/state；定义在 `handleGoogleCallback`。

### Cloud OAuth / device binding request bodies

- `CloudOAuthCallbackReq`：匿名 handler struct，本机 cloud OAuth callback code/state；定义在 `handleCloudOAuthCallback`。
- `CloudOAuthExchangeReq`：匿名 handler struct，cloud gate binding code 换取用户 token；定义在 `handleCloudOAuthExchange`。
- `CurrentDeviceReq`：匿名 handler struct，本机设备 id/name/public key 上报；定义在 `handleCurrentDevice`。

### Workspace / session request bodies

- `terminal.CreateSessionReq`：创建 session 请求；定义在 `internal/application/terminal/registry.go`。
- `terminal.RerunSessionReq`：rerun session 请求；定义在 `internal/application/terminal/registry.go`。
- `terminal.UpdateSessionReq`：更新 session 请求；定义在 `internal/application/terminal/registry.go`。
- `terminal.UpdateWorkspaceOrderReq`：workspace 排序请求；定义在 `internal/application/terminal/registry.go`，HTTP handler 当前使用同结构匿名 struct。
- `terminal.UpdateSessionOrderReq`：session 排序请求；定义在 `internal/application/terminal/registry.go`，HTTP handler 当前使用同结构匿名 struct。

## Frontend API notes

### Frontend API modules

前端 API 调用主要分布在以下模块：

- `web/src/features/api/client.ts`
  - 定义共享 `apiClient`。
  - 请求拦截器自动补充 `X-Request-ID`。
  - 若 gateway store 中存在 token，自动设置 `Authorization: Bearer <token>`。
  - 响应拦截器将后端 `ApiErrorResp` 转换为 `ApiClientError`；如果错误响应不符合 `ApiErrorResp`，转为 `ApiContractMismatchError`。
  - `401` 时会清除 token；非 local mode 且当前不在 login 页时跳转 login。
- `web/src/features/gateway/api.ts`
  - 封装 auth、cloud oauth、devices 相关 HTTP API。
  - 定义前端类型：`DeviceSummary`、`UserInfo`、`AuthCapabilities`、`CloudSessionSummary`、`AuthMeResp`、`TokenResp`、`CloudOAuthCallbackResp`。
- `web/src/features/workspaces/api.ts`
  - 封装 workspace 列表、workspace tree、workspace order、delete workspace。
  - 使用 `ApiResult<T>` 表达 data + offline 状态。
- `web/src/features/sessions/api.ts`
  - 封装 session create/get/update/delete/close/rerun/history/order，以及 terminal WebSocket URL 生成。
  - `readHistory()` 明确设置 `responseType: 'text'`，对应后端 `HistoryText`。
- `web/src/features/sessions/useTerminalSocket.ts`
  - 建立 browser terminal WebSocket。
  - 使用 subprotocol `termbridge.terminal.v1`。
  - 文本消息走 control protocol，二进制消息走 terminal 输入/输出。
- `web/src/protocol/terminal.ts`
  - 定义前端 terminal/control/API 类型，包括 `ApiErrorResp`、`WorkspaceSummary`、`SessionSummary`、`CreateSessionReq`、`CreateSessionResp`、`ClientControlMessage`、`ServerControlMessage`。

### Frontend and backend contract alignment notes

1. 前端存在多个 `*Resp` 类型，但后端并不总是有同名 Go DTO。
   - `TokenResp` 是前端类型；后端实际直接返回 `map[string]string`。
   - `AuthMeResp` 是前端类型；后端实际直接返回 `map[string]any`，其中嵌入 `authapp.UserView` / `authapp.Capabilities` / `CloudSessionSummary`。
   - `CloudOAuthCallbackResp` 是前端类型；后端实际直接返回 `map[string]any`。
   - `GoogleAuthURLResp`、`CloudOAuthStartResp`、`CloudOAuthAuthorizeResp`、`CurrentDeviceResp` 是本文档为描述接口而命名的响应体概念，后端没有独立同名 Go DTO。
2. 前端 workspace/session 类型主要放在 `web/src/protocol/terminal.ts`，后端对应类型主要放在 `internal/application/terminal/registry.go`。
   - 命名大体一致，例如 `CreateSessionReq`、`CreateSessionResp`、`WorkspaceSummary`、`SessionSummary`。
   - 数组响应在前端通常表示为 `T[] | null`，再用 `?? []` 规整为空数组。
3. 前端对 offline cache 有显式包装：
   - `listWorkspaces()`、`listWorkspaceTree()`、`readHistory()` 返回 `ApiResult<T>`，读取响应头 `X-TermBridge-Offline`。
   - 后端当前只有 `workspace_tree` 和 history 有缓存返回逻辑；`workspaces` 接口没有 cacheKind。
4. 前端对部分后端 `map` 响应只提取单个字段：
   - `authGoogleURL()` 提取 `auth_url`。
   - `cloudOAuthStart()` 提取 `authorize_url`。
   - `cloudOAuthAuthorize()` 提取 `redirect_url`。
5. 前端对部分成功响应不读取 body：
   - `authLogout()`、`authRegister()`、`authVerifyEmail()`、`authResendVerification()`、`authPasswordResetRequest()`、`authPasswordResetConfirm()`、`authChangePassword()`、`deleteDevice()`、`deleteWorkspace()`、`deleteSession()` 返回 `Promise<void>`。
   - 其中后端实际可能返回 `OkResp`、`NoContent` 或 `null` relay result；前端统一忽略。
6. 前端 terminal WebSocket URL 会把 token 放进 query 参数：
   - `terminalWsUrl()` 支持 `token`、`cols`、`rows` query。
   - 后端 `auth.ExtractBearerToken()` 支持从 query `token` 提取 bearer token。
7. 前端没有直接调用的后端接口包括：
   - `GET /api/health`
   - `POST /api/cloud-oauth/exchange`
   - `POST /api/devices/current`
   - `GET /api/devices/{deviceId}/workspaces/{workspaceId}/sessions`
   - `GET /api/agent/tunnel`，该接口由后端 agent client 使用。
8. 前端没有生成 OpenAPI client，类型靠手写 TypeScript 与后端 Go DTO/handler 响应保持约定；当前存在后端匿名 struct / map 响应与前端命名类型之间的漂移风险。

### Frontend type coverage by endpoint

| Endpoint area | Frontend module | Frontend request type | Frontend response type | 备注 |
|---|---|---|---|---|
| Auth login / Google callback | `features/gateway/api.ts` | inline function params | `TokenResp` | 后端实际为 `map[string]string` |
| Auth me | `features/gateway/api.ts` | 无 | `AuthMeResp` | 后端实际为 `map[string]any` |
| Auth register / verify / password | `features/gateway/api.ts` | inline function params / inline object | `void` | 后端实际返回 `OkResp` map |
| Cloud OAuth start / authorize | `features/gateway/api.ts` | function params -> query | `string` | 前端只取 `authorize_url` / `redirect_url` |
| Cloud OAuth callback | `features/gateway/api.ts` | inline object | `CloudOAuthCallbackResp` | 后端实际为 `map[string]any` |
| Devices list | `features/gateway/api.ts` | 无 | `DeviceSummary[]` | 数组响应 |
| Device delete | `features/gateway/api.ts` | path param | `void` | 后端 `204 NoContent` |
| Workspace list/tree/order/delete | `features/workspaces/api.ts` | `UpdateWorkspaceOrderReq` 或 path param | `ApiResult<WorkspaceSummary[]>` / `ApiResult<WorkspaceTreeSummary[]>` / `WorkspaceSummary[]` / `void` | tree 支持 offline header |
| Session create/get/update/delete/close/rerun/order/history | `features/sessions/api.ts` | `CreateSessionReq` / `UpdateSessionReq` / `RerunSessionReq` / inline order body | `CreateSessionResp` / `SessionSummary` / `WorkspaceTreeSession[]` / `string` / `void` | history 是 text response |
| Terminal WebSocket | `features/sessions/api.ts` + `features/sessions/useTerminalSocket.ts` | `ClientControlMessage` 或 binary data | `ServerControlMessage` 或 binary data | WebSocket frame，不是 JSON HTTP response |

## Backend route map

当前后端 API route 主要由 `gatewayapi.Handler.Handler()` 注册：

```text
/api/health
/api/auth/login
/api/auth/logout
/api/auth/me
/api/auth/register
/api/auth/email/verify
/api/auth/email/verification/resend
/api/auth/password/change
/api/auth/password-reset/request
/api/auth/password-reset/confirm
/api/auth/google
/api/auth/google/callback
/api/cloud-oauth/start
/api/cloud-oauth/callback
/api/cloud-oauth/authorize
/api/cloud-oauth/exchange
/api/devices
/api/devices/current
/api/devices/
/api/agent/tunnel
```

`/api/devices/` 下还有 path 分发逻辑，形成 workspace、session、terminal WebSocket 等动态接口。

`httpserver.New()` 只把 `/api` 和 `/api/` 挂给 gateway API handler；如果配置了 `web.static_dir`，`/` 下会挂静态资源 handler，但静态资源不纳入本次业务 API 清单。

## Implementation notes

- 本次只修改文档：`docs/requirement/20260701-api-inventory.md`。
- 未修改产品代码、route、handler、DTO、测试或前端调用。
- 未运行服务逐个请求验证；接口清单来自静态代码追踪。
- 由于用户要求“响应体名称和定义，不需要具体字段”，本清单未展开所有 JSON 字段。
- 已标记后端未遵从显式 `Req` / `Resp` 类型命名的接口，包括匿名 request struct、map response、array response、plain text response、NoContent、null relay result 和 WebSocket frame。
- 已补充前端 API client、前端 API 模块、类型定义来源、调用覆盖和前后端契约差异说明。
