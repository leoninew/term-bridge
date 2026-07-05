# 前端 Proto DTO 清理需求 / Frontend Proto DTO Cleanup Requirement

最后修改时间: 2026-07-05 21:47:53

Review status: Accepted / 接受

## Background

当前项目已经通过 `task proto` 使用 buf 生成两端 proto 类型：

- Go 生成物：`internal/shared/dto/proto/...`
- 前端 TypeScript 生成物：`web/src/gen/proto/...`

但前端仍存在一批 API 级别手写 DTO、wrapper type、`Omit<ProtoX>` 派生类型以及 WebSocket control JSON union。这导致 API contract 同时存在 proto 生成物和前端手写定义两条来源，容易出现协议漂移。

用户已明确决策：

- 所有 proto 只负责定义请求和响应体，不使用 gRPC。
- 不存在“path 参数导致不能用 proto”的问题；如果 HTTP body 不包含 path 参数，就应让 proto message 表达 body contract。
- 前端不要出现 API 级别接口定义；API DTO 必须来自 `web/src/gen/proto/`。
- 不做 adapter / wrapper / view-model 转换来掩盖 contract 不一致。
- 枚举、时间字段本轮该替换就替换；业务处理可以留在业务逻辑处，但 DTO 类型不另起一套。
- WebSocket JSON control protocol 也需要 proto 化。
- 异常 details 保持灵活，具体业务具体使用。
- 协议层面 `null` / `undefined` 都表达字段未设置，不应因此手写 DTO。
- 后端当前手写 JSON struct 后续也要转向 proto 生成定义。

## Goal

1. 清理前端 API 级别 DTO 重复定义，使前端请求/响应类型以 `web/src/gen/proto/` 生成物为唯一来源。
2. 补齐缺失的 proto request / response body message。
3. 调整 runtime proto request message，使其表达 HTTP request body，而不是 URL path 参数和 body 的混合结构。
4. 将 terminal WebSocket JSON control protocol 纳入 proto 定义，并让前端/后端使用生成类型或由生成类型约束的结构。
5. 删除或收敛 `web/src/features/types.ts`、`web/src/protocol/terminal.ts` 中 API contract 级别的手写类型。
6. 保留纯业务工具函数、terminal size clamp、WebSocket 编解码校验逻辑等非 DTO 逻辑。
7. 使用现有工具链生成代码；工具链已就绪，不增加条件分支或猜测式 fallback。

## Non-goal

- 不引入 gRPC。
- 不新增 adapter 层、DTO wrapper、前端 view-model 包装来绕过 generated proto type。
- 不用手写 TypeScript interface 替代 proto 缺失项；缺失项必须补 proto。
- 不在本轮完整迁移所有后端 handler 手写 JSON struct 到 proto 生成类型；但不得新增与 proto 冲突的新手写 contract。
- 不改变业务功能语义，只统一 contract 来源。
- 不处理与 proto DTO 清理无关的 OAuth/device/transaction 业务逻辑。

## Current findings

### Existing generated proto outputs

前端生成物当前位于：

- `web/src/gen/proto/termbridge/cloud/v1/cloud.ts`
- `web/src/gen/proto/termbridge/runtime/v1/runtime.ts`
- `web/src/gen/proto/termbridge/common/v1/common.ts`
- `web/src/gen/proto/termbridge/tunnel/v1/tunnel.ts`

Go 生成物当前位于：

- `internal/shared/dto/proto/termbridge/cloud/v1/cloud.pb.go`
- `internal/shared/dto/proto/termbridge/runtime/v1/runtime.pb.go`
- `internal/shared/dto/proto/termbridge/common/v1/common.pb.go`
- `internal/shared/dto/proto/termbridge/tunnel/v1/tunnel.pb.go`

### Frontend duplicated DTO locations

需要清理的主要前端文件：

- `web/src/features/types.ts`
  - 当前存在 `DeviceSummary`、`CloudSessionSummary`、`UserInfo`、`AuthMeResp`、`CloudOAuthCallbackResp`、`CloudConnectResp`、`ListDevicesResp`、`GoogleAuthUrlResp` 等 API DTO 级定义。
- `web/src/protocol/terminal.ts`
  - 当前存在 `ApiErrorResp`、`ListResp`、`PaginatedResp`、`WorkspaceSummary`、`SessionSummary`、`WorkspaceTreeSession`、`WorkspaceTreeSummary`、`CreateSessionReq`、`UpdateSessionReq`、`RerunSessionReq`、`CreateSessionResp` 等 API DTO 级定义。
  - 同时存在 `ClientControlMessage` / `ServerControlMessage` WebSocket JSON union，需要 proto 化。
- `web/src/features/agent/api.ts`
- `web/src/features/cloud/api.ts`
  - 当前 import 上述手写 DTO，并在 API client 泛型中使用。

### Backend protocol locations relevant to WebSocket control

当前 WebSocket control protocol 后端定义位于：

- `internal/shared/dto/protocol/terminal/protocol.go`

当前后端手写：

- `ClientMessage`
- `ServerMessage`
- `DecodeClient`
- `ValidateClient`
- `EncodeServer`
- `validateServer`

前端对应：

- `web/src/protocol/terminal.ts`
- `web/src/features/sessions/useTerminalSocket.ts`

## Requirement details

### R1. Cloud HTTP DTO must come from generated proto

`web/src/features/types.ts` 不应继续定义 cloud API DTO wrapper。缺失的 cloud request / response body 必须补到 `proto/termbridge/cloud/v1/cloud.proto`。

已知需要补齐或核对的消息包括：

- `GoogleAuthUrlResp`
- `CloudConnectReq`
- `CloudConnectResp`
- `AuthRegisterReq`
- `AuthVerifyEmailReq`
- `AuthResendVerificationReq`
- `AuthChangePasswordReq`
- `AuthPasswordResetRequestReq`
- `AuthPasswordResetConfirmReq`
- `AuthGoogleCallbackReq`
- `OAuthTokenResp` 或统一现有 `TokenResp` 的使用边界
- `CloudOAuthDeviceReportReq`

如果后端 HTTP response 中包含 `User.provider`，则 `User` proto 必须补字段；如果不应包含，则前端不得依赖该字段。

### R2. Runtime HTTP DTO must come from generated proto

`web/src/protocol/terminal.ts` 不应继续定义 runtime HTTP DTO wrapper。runtime HTTP API 请求/响应必须使用 `web/src/gen/proto/termbridge/runtime/v1/runtime.ts` 中的生成类型。

runtime proto request body 必须表达 HTTP body contract：

- URL path 参数不应出现在 body-only request message 中。
- 如果前端实际 body 只发送 `{ name }`，则对应 `UpdateSessionReq` 不应要求 `workspace_id` / `session_id`。
- 如果前端实际 body 只发送 `{ cols, rows }`，则对应 `RerunSessionReq` 不应要求 path 中已有的 `workspace_id` / `session_id`。
- 如果前端实际 body 只发送 `{ session_ids }`，则对应 `UpdateSessionOrderReq` 不应要求 path 中已有的 `workspace_id`。

### R3. Terminal WebSocket control protocol must be proto-defined

新增或扩展 proto 定义 terminal WebSocket control message。

推荐新增：

- `proto/termbridge/terminal/v1/terminal.proto`

必须覆盖当前 JSON control message 字段：

Client side:

- `type`
- `cols`
- `rows`
- `nonce`

Server side:

- `type`
- `session_id`
- `workspace_id`
- `state`
- `lifecycle_state`
- `attachment_state`
- `reason`
- `exit_code`
- `code`
- `message`
- `error`
- `nonce`
- `truncated`

当前 wire format 是 JSON over WebSocket，并使用 string `type`，本轮应保持现有 wire format，不引入 gRPC，也不把 WebSocket 改成 binary protobuf。

### R4. Enums and time fields must not cause DTO wrappers

枚举和时间字段如果需要业务约束，应在业务逻辑或 proto enum 中处理，但不得通过前端 DTO wrapper 改写 generated proto type。

- `lifecycle_state` / `attachment_state` 若需要枚举约束，应改 proto enum 或业务处校验。
- timestamp 字段按 generated proto type 使用。
- 不允许为了把 `string | undefined` 改成 `string` 新增 `Omit<ProtoX>` wrapper。

### R5. Error details remain flexible

`ApiErrorResp<TDetails>` 这类前端泛型 contract 不应作为 API DTO 主定义继续扩散。协议层以 `common.proto` 的 `ErrorResp` 为准；具体业务对 `details` 的读取可在业务处做类型收窄。

### R6. Backend hand-written JSON structs are migration debt

本轮重点是前端不再手写 API DTO。后端当前 `internal/.../api/handler/dto.go` 中手写 JSON struct 后续要迁移到 proto 生成类型。

本轮实现不得新增新的后端手写 DTO 与 proto 冲突；如为了对齐前端必须同步后端字段，应优先补 proto 并使用生成代码验证 contract。

## Acceptance

1. `proto/termbridge/cloud/v1/cloud.proto` 覆盖前端 cloud API 使用的请求/响应体。
2. `proto/termbridge/runtime/v1/runtime.proto` 的 request message 表达 HTTP body contract，不混入 path 参数。
3. Terminal WebSocket control message 被 proto 定义覆盖，并生成 Go / TypeScript 类型。
4. 执行现有 proto 生成入口成功：`task proto`。
5. 前端 API client 不再引用 `web/src/features/types.ts` 中的手写 API DTO。
6. `web/src/protocol/terminal.ts` 不再定义 runtime HTTP API DTO；只保留非 DTO 逻辑，例如 terminal size 常量/函数、WebSocket 编解码/校验逻辑。
7. 前端 API client 泛型直接使用 `web/src/gen/proto/...` generated types。
8. 不再出现 `Omit<Proto...>` 用于构造 API DTO wrapper。
9. 不新增 adapter / wrapper / view-model 转换层来掩盖 proto 与实际 HTTP contract 的差异。
10. 相关前端类型检查和 Go 测试通过，至少覆盖：
    - proto generation
    - frontend typecheck / build check
    - Go packages affected by proto type changes

## Risks / Assumptions

1. 修改 proto request body 会影响 Go generated types，需要同步后端引用点。
2. WebSocket control protocol 目前使用 JSON string `type`；本轮保持 wire format，只统一类型来源。
3. 后端手写 JSON struct 尚未全部迁移，可能短期仍存在 Go handler struct 与 proto generated struct 并行；该问题记录为后续迁移债务，但前端不再因此手写 DTO。
4. 当前工作区已有较多 staged / unstaged 改动；实现时必须限制在本需求相关文件，避免混入 transaction 或 device binding 业务逻辑。

## User review notes

- 用户明确拒绝 adapter / wrapper 方案：“所有适配都是扯淡”。
- 用户明确要求前端 API 级别不要出现手写接口定义。
- 用户明确要求工具链已就绪，不要猜测 fallback。
- 用户明确指出 proto 只负责请求和响应体，不使用 gRPC，也不存在 path 参数作为阻碍。
