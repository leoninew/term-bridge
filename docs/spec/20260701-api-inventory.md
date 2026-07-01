# API Inventory Spec
最后修改时间: 2026-07-01 16:45:47

## Review status

Accepted

## Flow mode / Stage

轻量模式 / light；用户已显式要求从规格 / Spec 进入实现 / Implementation，因此本 Spec 视为已接受。当前实现阶段按本 checklist 和后续用户补充规则推进，不创建 Plan / 计划文档。

## Requirement basis

基于已接受需求文档：`docs/requirement/20260701-api-inventory.md`。

已确认事实：

1. 当前后端 API 已完成静态摸底。
2. 当前存在多类非统一契约形态：
   - handler 匿名 request struct。
   - `map[string]...` / `map[string]any` 响应。
   - 顶层数组响应。
   - `text/plain` 响应。
   - `204 No Content` 空响应。
   - tunnel relay 返回 `null`。
   - WebSocket frame 协议。
3. 前端目前手写 TypeScript 类型，并不总是和后端 Go DTO 同名或同源。
4. 用户要求本 Spec 明确列出“待修改接口列表”，供后续实现阶段作为 checklist。

## Overview

后续目标不是改变业务行为，而是规范 API 契约表达，使后端和前端更容易长期维护、生成文档和减少类型漂移。

本 Spec 将待修改项分为三类：

1. **需要引入显式 Req/Resp DTO 的 HTTP JSON API**：优先消除匿名 request struct 和 map response。
2. **需要决策是否包装的非对象响应**：数组、文本、NoContent、null relay result 是否保持原样，还是引入统一 envelope。
3. **需要保持协议形态但补充命名文档/类型的 WebSocket API**：WebSocket 不强行改成 HTTP Req/Resp，但应明确 frame/message 类型边界。

## Design decisions

1. **不在 Spec 阶段修改代码**：本文件只定义后续待修改范围和 checklist。
2. **优先保持行为兼容**：后续实现默认不改变 HTTP method、path 和 JSON 字段名；但用户在 Implementation 阶段明确要求调整部分成功响应状态码和列表响应形态。
3. **后端 DTO 优先成为契约源头**：后续实现应优先在 Go 后端定义显式 request/response 类型，再同步前端 TypeScript 类型或至少对齐命名。
4. **列表响应统一对象包装**：用户确认列表类型使用 `{ items: []T }`，顶层数组响应需要同步后端 relay、offline cache 与前端读取逻辑。
5. **成功空响应统一 `204 No Content`**：有响应内容使用 `XxxxResp`；无响应内容不再使用 `OkResp`，统一 `204`。
6. **分页响应保留泛型**：`PaginatedResp[T]` 继续作为通用分页 response。
7. **错误响应使用泛型契约**：后端导出 `APIErrorResp[T]`，前端同步 `ApiErrorResp<TDetails = unknown>`。
8. **DTO 定义从业务逻辑抽离**：该抽取 DTO 的抽取到独立 `dto.go`；只有 DTO 自身逻辑（例如自定义 JSON 解析）保留在 DTO 文件中。
9. **避免把 WebSocket 伪装成 HTTP JSON API**：Browser terminal WS 和 Agent tunnel WS 继续按 message/frame protocol 管理。
10. **relay 接口需要兼顾 gateway 和 agent runtime**：修改 `/api/devices/{deviceId}/...` 相关契约时，需要同时考虑 gateway handler、agent tunnel method、local runtime DTO 和前端 API 封装。

## Affected components

后续实现预计会涉及：

- 后端 HTTP gateway：`internal/transport/http/gatewayapi/server.go`
- 后端错误响应：`internal/transport/http/gatewayapi/errors.go`
- 后端 terminal DTO：`internal/application/terminal/registry.go`
- Agent runtime relay：`internal/application/agent/client.go`
- Browser terminal protocol：`internal/protocol/terminal/protocol.go`
- Agent tunnel protocol：`internal/protocol/tunnel/frame.go`
- 前端 API client：`web/src/features/api/client.ts`
- 前端 gateway API：`web/src/features/gateway/api.ts`
- 前端 workspace API：`web/src/features/workspaces/api.ts`
- 前端 sessions API：`web/src/features/sessions/api.ts`
- 前端 terminal protocol types：`web/src/protocol/terminal.ts`
- 相关测试：`internal/transport/http/gatewayapi/*_test.go`、`web/src/**/*.test.ts`

## Interfaces checklist

### Legend

- `DTO`：需要引入或调整显式后端 Go request/response 类型。
- `FE`：需要同步或核对前端 TypeScript 类型/封装。
- `Behavior`：可能影响响应状态码、响应体形态或兼容性，需要单独确认。
- `Keep`：建议暂时保持现状，仅文档化或补类型说明。

### P0: 建议优先规范的 HTTP JSON API

这些接口当前以匿名 request struct 或 `map` response 为主，最适合作为第一批规范化对象。

| Checklist | Method | Path | Current request | Current response | 建议修改方向 | Scope |
|---|---|---|---|---|---|---|
| [x] | `GET` | `/api/health` | 无 | `map[string]string` | 已新增 `HealthResp` 后端 DTO | DTO |
| [x] | `POST` | `/api/auth/login` | 匿名 struct | `map[string]string` | 已新增 `AuthLoginReq`、`TokenResp` 后端 DTO；前端继续使用 `TokenResp` | DTO + FE |
| [x] | `POST` | `/api/auth/logout` | 无 | `map[string]bool` | 已改为 `204 No Content`；前端继续 `Promise<void>` | DTO + FE |
| [x] | `GET` | `/api/auth/me` | 无 | `map[string]any` | 已新增 `AuthMeResp` 后端 DTO，显式引用 `UserView`、`Capabilities`、`CloudSessionSummary` | DTO + FE |
| [x] | `POST` | `/api/auth/register` | 匿名 struct | `map[string]bool` | 已新增 `AuthRegisterReq`，成功响应改为 `204 No Content` | DTO + FE |
| [x] | `POST` | `/api/auth/email/verify` | 匿名 struct | `map[string]bool` | 已新增 `AuthVerifyEmailReq`，成功响应改为 `204 No Content` | DTO + FE |
| [x] | `POST` | `/api/auth/email/verification/resend` | 匿名 struct | `map[string]bool` | 已新增 `AuthResendVerificationReq`，成功响应改为 `204 No Content` | DTO + FE |
| [x] | `POST` | `/api/auth/password/change` | 匿名 struct | `map[string]bool` | 已新增 `AuthChangePasswordReq`，成功响应改为 `204 No Content` | DTO + FE |
| [x] | `POST` | `/api/auth/password-reset/request` | 匿名 struct | `map[string]bool` | 已新增 `AuthPasswordResetRequestReq`，成功响应改为 `204 No Content` | DTO + FE |
| [x] | `POST` | `/api/auth/password-reset/confirm` | 匿名 struct | `map[string]bool` | 已新增 `AuthPasswordResetConfirmReq`，成功响应改为 `204 No Content` | DTO + FE |
| [x] | `GET` | `/api/auth/google` | 无 | `map[string]string` | 已新增 `GoogleAuthURLResp` 后端 DTO；前端同步命名类型 | DTO + FE |
| [x] | `POST` | `/api/auth/google/callback` | 匿名 struct | `map[string]string` | 已新增 `AuthGoogleCallbackReq`、复用 `TokenResp` | DTO + FE |
| [x] | `GET` | `/api/cloud-oauth/start` | query 参数 | `map[string]string` | 已新增 `CloudOAuthStartResp`；前端同步命名类型 | DTO + FE |
| [x] | `POST` | `/api/cloud-oauth/callback` | 匿名 struct | `map[string]any` | 已新增 `CloudOAuthCallbackReq`、`CloudOAuthCallbackResp` | DTO + FE |
| [x] | `GET` | `/api/cloud-oauth/authorize` | query 参数 | `map[string]string` | 已新增 `CloudOAuthAuthorizeResp`；前端同步命名类型 | DTO + FE |
| [x] | `POST` | `/api/cloud-oauth/exchange` | 匿名 struct | `map[string]string` | 已新增 `CloudOAuthExchangeReq`、复用 `TokenResp` | DTO |
| [x] | `POST` | `/api/devices/current` | 匿名 struct | `map[string]any` | 已新增 `CurrentDeviceReq`、`CurrentDeviceResp` | DTO |

### P1: Relay JSON API，已有部分 terminal DTO 但 handler 仍需规范

这些接口的业务 DTO 多数存在于 `internal/application/terminal/registry.go`，但 gateway handler 仍有匿名 struct、数组响应或 relay result 形态不统一。

| Checklist | Method | Path | Current request | Current response | 建议修改方向 | Scope |
|---|---|---|---|---|---|---|
| [x] | `POST` | `/api/devices/{deviceId}/sessions` | `terminal.CreateSessionReq` raw relay | `terminal.CreateSessionResp` | gateway 层已显式解码 `terminal.CreateSessionReq` | DTO |
| [x] | `GET` | `/api/devices/{deviceId}/workspaces` | 无 | `[]terminal.WorkspaceSummary` | 已改为 `{ items: []terminal.WorkspaceSummary }`；前端读取 `items` | Behavior + FE |
| [x] | `GET` | `/api/devices/{deviceId}/workspaces/tree` | 无 | `[]terminal.WorkspaceTreeNode` | 已改为 `{ items: []terminal.WorkspaceTreeNode }`；offline cache 保留原 header 语义 | Behavior + FE |
| [x] | `PATCH` | `/api/devices/{deviceId}/workspaces/order` | 匿名 struct，结构同 `terminal.UpdateWorkspaceOrderReq` | `[]terminal.WorkspaceSummary` | gateway handler 已改用 `terminal.UpdateWorkspaceOrderReq`；响应改为 `{ items: []terminal.WorkspaceSummary }` | DTO + Behavior |
| [x] | `DELETE` | `/api/devices/{deviceId}/workspaces/{workspaceId}` | 无 | relay `null` | 已改为 `204 No Content`；前端继续 `void` | Behavior + FE |
| [x] | `GET` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions` | 无 | `[]terminal.WorkspaceSessionSummary` | relay 参数已改为 `terminal.WorkspaceSessionsReq`；响应改为 `{ items: []terminal.WorkspaceSessionSummary }` | DTO + Behavior |
| [x] | `POST` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions` | `terminal.CreateSessionReq` raw relay，gateway 补 workspace id | `terminal.CreateSessionResp` | gateway 层已显式解码 `terminal.CreateSessionReq`，并以 path workspace id 覆盖 body workspace id | DTO + Behavior |
| [x] | `PATCH` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/order` | 匿名 struct，结构同 `terminal.UpdateSessionOrderReq` | `[]terminal.WorkspaceSessionSummary` | gateway handler 已显式解码 `terminal.UpdateSessionOrderReq`；relay 参数使用 `terminal.WorkspaceSessionOrderReq`；响应改为 `{ items: []terminal.WorkspaceSessionSummary }` | DTO + Behavior |
| [x] | `GET` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}` | 无 | `terminal.SessionSummary` | relay 参数已改为 `terminal.WorkspaceSessionReq`；前端字段保持一致 | DTO + FE |
| [x] | `PATCH` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}` | `terminal.UpdateSessionReq` raw relay | `terminal.SessionSummary` | gateway handler 已显式解码 `terminal.UpdateSessionReq` 后再 relay | DTO |
| [x] | `DELETE` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}` | 无 | `204 No Content` | 保持 `204 No Content`；relay 参数改为 `terminal.WorkspaceSessionReq` | Keep |
| [x] | `POST` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/close` | 无 | `terminal.SessionSummary` | relay 参数已改为 `terminal.WorkspaceSessionReq`；前端字段保持一致 | DTO + FE |
| [x] | `POST` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/rerun` | `terminal.RerunSessionReq` raw relay | `terminal.CreateSessionResp` | gateway handler 已显式解码 `terminal.RerunSessionReq` 后再 relay | DTO |
| [x] | `GET` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/history` | 无 | `text/plain` | 保持文本响应；relay 参数改为 `terminal.WorkspaceSessionReq`；前端继续 `responseType: text` | Keep / FE |

### P2: Devices and ownership APIs

| Checklist | Method | Path | Current request | Current response | 建议修改方向 | Scope |
|---|---|---|---|---|---|---|
| [x] | `GET` | `/api/devices` | 无 | `[]DeviceSummary` | 已改为 `ListDevicesResp` / `{ items: []DeviceSummary }`；前端读取 `items` | Behavior + FE |
| [x] | `DELETE` | `/api/devices/{deviceId}` | 无 | `204 No Content` | 保持 `204 No Content`；前端继续 `void` | Keep / FE |

### P3: WebSocket protocol APIs

WebSocket 不建议改造成 HTTP `Req` / `Resp`，但应明确协议类型、前端类型与后端类型的对应关系。

| Checklist | Method | Path | Current request | Current response | 建议修改方向 | Scope |
|---|---|---|---|---|---|---|
| [ ] | `GET` | `/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/ws` | HTTP upgrade + query `token` / `cols` / `rows`；WS 文本 `terminalproto.ClientMessage`；WS binary terminal input | WS 文本 `terminalproto.ServerMessage`；WS binary terminal output | 保持 WebSocket message protocol；补充前端/后端协议对照测试或文档 | Keep / FE |
| [ ] | `GET` | `/api/agent/tunnel` | HTTP upgrade + device signature headers 或 Basic auth；WS `tunnel.Frame` | WS `tunnel.Frame` | 保持 tunnel frame protocol；补充内部 RPC method 与 payload 对照文档 | Keep |

### P4: Common contract infrastructure

| Checklist | Item | Current state | 建议修改方向 | Scope |
|---|---|---|---|---|
| [x] | 通用错误响应 | 后端 `gatewayapi.errorResponse` 非导出；前端 `ApiErrorResp` 手写 | 已导出后端 `APIErrorResp[T]`，并保留 `errorResponse` 测试别名；前端改为 `ApiErrorResp<TDetails = unknown>` | DTO + FE |
| [x] | `OkResp` | 多处直接返回 `map[string]bool{"ok": true}` | 用户确认无内容成功响应使用 `204 No Content`，不新增 `OkResp` | DTO |
| [x] | `TokenResp` | 多处直接返回 `map[string]string{"access_token": ..., "token_type": "bearer"}` | 已新增统一后端 `TokenResp`，复用 login/google/cloud exchange；前端保持 `TokenResp` | DTO + FE |
| [x] | Query 参数 DTO | GET cloud oauth authorize/start 直接读 query | 本轮仅保持 query 直接读取；响应 DTO 已命名，未引入未使用运行时代码类型 | Decision |
| [x] | 前端类型来源 | 前端手写 TS 类型 | 本轮继续手写 TS 类型，并对齐命名与 `{ items }` 列表契约；生成链路留作后续 | Alternative |

## Proposed DTO additions

后续实现可优先新增这些后端 DTO，具体包位置待 Plan 决定：

### Gateway common

- `HealthResp`
- `ListResp[T]`
- `PaginatedResp[T]`
- `TokenResp`
- `APIErrorResp[T]`（保留 `errorResponse` 测试兼容别名）

### Auth / account

- `AuthLoginReq`
- `AuthRegisterReq`
- `AuthVerifyEmailReq`
- `AuthResendVerificationReq`
- `AuthChangePasswordReq`
- `AuthPasswordResetRequestReq`
- `AuthPasswordResetConfirmReq`
- `AuthGoogleCallbackReq`
- `AuthMeResp`
- `GoogleAuthURLResp`

### Cloud OAuth / device binding

- `CloudOAuthStartResp`
- `CloudOAuthCallbackReq`
- `CloudOAuthCallbackResp`
- `CloudOAuthAuthorizeResp`
- `CloudOAuthExchangeReq`
- `CurrentDeviceReq`
- `CurrentDeviceResp`

### Relay / terminal gateway

- 复用或引入 gateway 层类型以替代 handler 匿名 struct：
  - `terminal.UpdateWorkspaceOrderReq`
  - `terminal.UpdateSessionOrderReq`
  - `terminal.CreateSessionReq`
  - `terminal.UpdateSessionReq`
  - `terminal.RerunSessionReq`
- 列表响应已统一包装为 `{ items: []T }`：
  - gateway 层新增 `ListDevicesResp`
  - terminal 层新增 `ListWorkspacesResp`
  - terminal 层新增 `WorkspaceTreeResp`
  - terminal 层新增 `WorkspaceSessionsResp`
  - terminal 层新增 `UpdateWorkspaceOrderResp`
  - terminal 层新增 `UpdateSessionOrderResp`

## Technical questions

1. 是否要求所有 HTTP 成功响应都统一为 object envelope？
   - 如果是，顶层数组响应需要破坏性调整，前端调用也要同步。
   - 如果不是，可只规范命名 DTO，保留数组/text/204 语义。
2. `DELETE /api/devices/{deviceId}/workspaces/{workspaceId}` 当前成功返回 tunnel `null`。后续是否改为 `204 No Content`？
   - 这会改变状态码和 body，但更符合 delete 语义。
3. gateway handler 是否应该在 relay 前显式 decode request DTO？
   - 好处：gateway 层提前校验契约。
   - 风险：可能与 agent runtime 的实际参数解析重复，需要保证错误语义一致。
4. 前端类型是否继续手写，还是后续引入 OpenAPI / schema 生成？
   - 本 Spec 不要求引入生成链路，只为后续预留方向。
5. 是否导出后端 `errorResponse` 类型？
   - 导出有利于测试和文档，但也会扩大包级 API 表面。

## Risks

1. 统一响应体可能造成前端兼容性变更，尤其是当前返回数组、文本、204、null 的接口。
2. 过度引入 DTO 可能让简单 handler 变重；需要避免只为命名而增加无意义层次。
3. Relay API 横跨 gateway、agent tunnel、local runtime、frontend，修改任一层都可能造成契约不一致。
4. Auth/cloud OAuth 部分存在 local/cloud mode 条件差异，规范 DTO 时不能误改可用性和错误码。
5. WebSocket 协议不能简单套用 HTTP Req/Resp，必须保持 message/frame 语义。

## Alternatives

1. **只文档化，不改代码**：成本最低，但不能消除匿名 struct / map response 与前端手写类型漂移。
2. **只为 auth/cloud 新增 DTO，不动 relay**：收益高、风险较低；relay 保持当前行为。
3. **全量统一为 envelope 响应**：长期一致性最好，但会影响前端和兼容性，适合另起破坏性版本化任务。
4. **引入 OpenAPI 生成前端类型**：长期方向明确，但本轮范围更大，需要先把后端 DTO 和响应形态稳定下来。

## Implementation notes

- 用户要求：列表类型统一为 `{ items: []T }`。
- 用户要求：有响应内容使用 `XxxxResp`；无响应内容使用 `204 No Content`。
- 用户要求：`PaginatedResp[T]` 继续作为通用分页 response。
- 用户要求：错误响应也使用泛型契约。
- 用户要求：DTO 定义抽取到独立 `dto.go`，不要和业务逻辑混在一起；DTO 自身逻辑例外。
- 当前 Implementation 已运行：`go test ./internal/transport/http/gatewayapi ./internal/application/agent ./internal/application/terminal`。
- 当前 Implementation 已运行：`npm --prefix web run typecheck`。
- 曾尝试 `npm --prefix web run type-check`，项目无该脚本，已改用实际存在的 `typecheck` 脚本。

## User review notes

- 用户要求：`进入 spec，这一步要列出待修改接口列表，后面实现用作 checklist`。
- 用户随后要求：`开始Implementation`，因此本 Spec 标记为 `Accepted` 并进入实现阶段。
- 本 Spec 因此将重点放在 `Interfaces checklist`，供后续 Plan / Implementation 逐项勾选。
