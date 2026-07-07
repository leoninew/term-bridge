# 后端 API 请求响应体 proto DTO 对齐

最后修改时间: 2026-07-06 13:52:40

Review status: Accepted

## Background

前端已经完成 DTO 来源向 proto 生成类型收敛：HTTP/WebSocket 业务请求和响应尽量从 `web/src/gen/proto/...` 引用生成类型，而不是继续维护手写 DTO。后端当前虽然已经把 proto 作为前后端契约来源之一，但 API 层仍存在多处手写请求/响应体 struct。

本次只读调查覆盖了：

- `internal/agent/api/handler`
- `internal/cloud/api/handler`
- `internal/agent/application/task/terminal/dto.go`
- `proto/termbridge/*/v1/*.proto`
- `internal/gen/proto/termbridge/*/v1/*.go`

调查结论：错误响应、runtime/tunnel、terminal 控制消息已经大量使用生成 proto 类型；但常规 HTTP JSON 请求/响应体，尤其是 auth、device、health、cloud connect 和部分 runtime 请求体，仍主要是手写 Go struct。

用户已明确决策：**这些 API 请求/响应体都要迁移到 proto 生成类型；遇到问题就解决问题，不通过适配层、兼容层保留旧手写 DTO。** 当前很多 proto 定义已经就绪，问题主要是业务代码里存在冗余手写定义和重复转换。

## Goal

1. 记录后端 API DTO 调查结果，明确哪些请求/响应体已经 proto 化，哪些仍是手写 DTO。
2. 推进后端 API 层和前端一致：wire 层请求/响应体必须使用 `internal/gen/proto/termbridge/*/v1` 生成类型，不继续维护同 shape 手写 DTO。
3. 迁移所有已被 proto 覆盖的 API 请求/响应体，包括 auth、health、device、cloud session、cloud connect、runtime request、terminal control message 等相关类型。
4. 删除业务代码中与 proto message 重复的冗余 DTO；如果某个 proto 定义已经就绪，直接引用生成类型。
5. 迁移过程中遇到 JSON 行为、timestamp、nil/null、optional/presence、unknown field 等问题时，记录问题并正面修复，不引入兼容层或适配层来维持两套 DTO。
6. 不把问题掩盖为“暂缓不迁移”；确实需要分步骤实施时，也必须以最终完全迁移为目标，并在阶段内消除对应手写 DTO。

## Non-goal

1. 本需求阶段不修改产品代码，不运行验证命令。
2. 不修改 proto 契约本身；本次优先基于现有 `proto/termbridge/*/v1/*.proto` 进行后端引用对齐。
3. 不保留同 shape 手写 DTO 作为兼容层、适配层或过渡层。
4. 不把 handler config、service interface、registry、route、relay 等非请求/响应体类型 proto 化。
5. 不为了迁移而保留“旧 DTO + 新 proto DTO”两套 wire 模型长期并存。
6. 如果发现 proto 真的缺失必要消息或字段，不在业务层补一个永久手写 DTO 绕过；应记录缺口，并按 proto 契约补齐或复用已有等价消息。

## Investigation summary

### 已经使用 proto 的部分

#### 错误响应

位置：

- `internal/agent/api/handler/errors.go`
- `internal/cloud/api/handler/errors.go`

当前已使用：

- `commonv1.ErrorResp`
- `codec.MarshalProtoJSON`

结论：live 错误响应已经基本 proto 化。`APIErrorResp[T]`、`ApiErrorResp[T]`、`errorResponse` 更像残留或测试辅助类型，应清理或改成测试局部解析结构。

#### Runtime / tunnel

位置：

- `internal/agent/api/handler/runtime_endpoint.go`
- `internal/cloud/api/handler/runtime_endpoint.go`
- `internal/shared/dto/protocol/tunnel/frame.go`

当前已使用：

- `runtimev1.*`
- `tunnelv1.*`
- `commonv1.ErrorResp`

结论：runtime 响应侧已经大量通过 tunnel response 输出 proto JSON；但请求体入口仍有手写 DTO 再转换为 `runtimev1` 的情况。该冗余需要消除。

#### Terminal control

位置：

- `internal/shared/dto/protocol/terminal/protocol.go`

当前通过 alias 使用：

- `terminalv1.ClientControlMessage`
- `terminalv1.ServerControlMessage`

结论：类型已经来自 proto，但 encode/decode 仍使用 `encoding/json`，不是 `protojson`。该问题也要迁移处理，而不是作为长期例外。

### 仍然手写的主要 DTO

#### Agent API handler

文件：`internal/agent/api/handler/dto.go`

手写类型包括：

- `DeviceSummary`
- `CloudSessionSummary`
- `APIErrorResp[T]`
- `HealthResp`
- `PaginatedResp[T]`
- `ListResp[T]`
- `ListDevicesResp`
- `TokenResp`
- `AuthLoginReq`
- `AuthRegisterReq`
- `AuthVerifyEmailReq`
- `AuthResendVerificationReq`
- `AuthChangePasswordReq`
- `AuthPasswordResetRequestReq`
- `AuthPasswordResetConfirmReq`
- `GoogleAuthURLResp`
- `AuthGoogleCallbackReq`
- `AuthMeResp`
- `CloudConnectReq`
- `CloudConnectResp`
- `CurrentDeviceReq`
- `CurrentDeviceResp`
- `CloudDeviceReportReq`

大部分已被 `cloudv1` 或 `commonv1` 覆盖。其中 `CloudDeviceReportReq` 没有同名 proto，但字段等价于 `cloudv1.CurrentDeviceReq`，应直接复用该生成类型。

文件：`internal/agent/api/handler/auth_types.go`

手写类型：

- `UserView`
- `AuthResult`

`UserView` 的 wire 表达可对应 `cloudv1.User`。`AuthResult` 是内部返回值，不是直接 HTTP body；若它只服务 wire 层，也应随迁移消除或改为内部非 wire 语义明确的类型。

#### Cloud API handler

文件：`internal/cloud/api/handler/dto.go`

手写类型包括：

- `DeviceSummary`
- `CloudSessionSummary`
- `ApiErrorResp[T]`
- `errorResponse`
- `HealthResp`
- `PaginatedResp[T]`
- `ListResp[T]`
- `ListDevicesResp`
- `TokenResp`
- `AuthLoginReq`
- `AuthRegisterReq`
- `AuthVerifyEmailReq`
- `AuthResendVerificationReq`
- `AuthChangePasswordReq`
- `AuthPasswordResetRequestReq`
- `AuthPasswordResetConfirmReq`
- `GoogleAuthURLResp`
- `AuthGoogleCallbackReq`
- `AuthMeResp`
- `CurrentDeviceReq`
- `CurrentDeviceResp`
- `CloudDeviceReportReq`

其中 cloud auth、device、cloud session 等请求/响应多数已被 `cloudv1` 覆盖，应迁移到生成类型。

文件：`internal/cloud/api/handler/runtime_dto.go`

手写类型包括：

- `CreateSessionReq`
- `RerunSessionReq`
- `UpdateSessionReq`
- `UpdateWorkspaceOrderReq`
- `UpdateSessionOrderReq`
- `DeleteWorkspaceReq`
- `WorkspaceSessionsReq`
- `WorkspaceSessionOrderReq`
- `WorkspaceSessionReq`
- `UpdateWorkspaceSessionReq`
- `RerunWorkspaceSessionReq`

这些已经有 `runtimev1.*` 对应消息，应迁移，不能作为长期手写边界。

#### Agent application runtime DTO

文件：`internal/agent/application/task/terminal/dto.go`

这里也存在与 `runtimev1.*` 对应的手写请求 DTO。它们当前处于 application 层，但如果这些类型实际承担 API wire 请求/响应语义，就应消除冗余。迁移时应直接面对 application 层是否仍需要独立业务命令模型的问题；如果只是 proto 的重复定义，应删除并引用 `runtimev1.*`。

## User scenarios

1. 开发者查看 API 请求/响应体定义时，能从 proto 生成类型定位 wire 契约，不需要同时维护 Go 手写 DTO 和 proto 消息。
2. 前端使用 proto 生成 TS 类型，后端 API handler 使用 proto 生成 Go 类型，双方字段命名、可选性、响应结构更容易保持一致。
3. 当 proto 新增或修改字段后，后端 handler 和前端都能通过生成类型暴露编译期不一致，而不是继续依赖手写 struct 默默漂移。
4. 后端 API 层不再存在“proto 已经定义，但业务里又手写一份同 shape struct”的重复实现。
5. 如果迁移暴露出 JSON 行为或 proto 契约问题，开发者直接修正契约或业务实现，而不是通过兼容层让旧新两套模型并存。

## Acceptance

### 调查记录

- 本文档记录已完成的只读调查结果。
- 明确区分：
  - 已经使用 proto 的 API 部分。
  - proto 已覆盖但仍手写的 DTO。
  - proto 未覆盖或不建议 proto 化的非 wire 类型。
  - 迁移中需要正面处理的问题和风险。

### 总体迁移要求

- 所有 API 请求/响应体，只要 proto 已覆盖，就必须迁移到生成类型。
- 不保留同 shape 手写 DTO 作为兼容层、适配层或长期过渡层。
- 如果现有 proto message 已经能表达当前请求/响应，业务代码直接引用生成类型。
- 如果没有同名 proto 但存在字段等价消息，例如 `CloudDeviceReportReq` 与 `cloudv1.CurrentDeviceReq`，应复用已有生成类型。
- 如果 proto 缺失必要契约，应记录为 proto 缺口并补契约，而不是在 API handler 层新增永久手写 DTO。

### 必须迁移的已覆盖 DTO

以下类型都在迁移目标内，不再标记为“暂缓不迁移”：

- `HealthResp -> commonv1.HealthResp`
- `TokenResp -> cloudv1.TokenResp`
- `GoogleAuthURLResp -> cloudv1.GoogleAuthUrlResp`
- `AuthLoginReq -> cloudv1.AuthLoginReq`
- `AuthRegisterReq -> cloudv1.AuthRegisterReq`
- `AuthVerifyEmailReq -> cloudv1.AuthVerifyEmailReq`
- `AuthResendVerificationReq -> cloudv1.AuthResendVerificationReq`
- `AuthChangePasswordReq -> cloudv1.AuthChangePasswordReq`
- `AuthPasswordResetRequestReq -> cloudv1.AuthPasswordResetRequestReq`
- `AuthPasswordResetConfirmReq -> cloudv1.AuthPasswordResetConfirmReq`
- `AuthGoogleCallbackReq -> cloudv1.AuthGoogleCallbackReq`
- `DeviceSummary -> cloudv1.DeviceSummary`
- `CloudSessionSummary -> cloudv1.CloudSessionSummary`
- `ListDevicesResp -> cloudv1.ListDevicesResp`
- `CurrentDeviceReq -> cloudv1.CurrentDeviceReq`
- `CurrentDeviceResp -> cloudv1.CurrentDeviceResp`
- `CloudDeviceReportReq -> cloudv1.CurrentDeviceReq`
- `CloudConnectReq -> cloudv1.CloudConnectReq`
- `CloudConnectResp -> cloudv1.CloudConnectResp`
- `AuthMeResp -> cloudv1.AuthMeResp`
- `UserView -> cloudv1.User`（wire 输出场景）
- `internal/cloud/api/handler/runtime_dto.go` 中与 `runtimev1.*` 重复的请求 DTO
- `internal/agent/application/task/terminal/dto.go` 中仅重复 `runtimev1.*` wire 语义的请求 DTO
- `internal/shared/dto/protocol/terminal/protocol.go` 中 terminal control message 的 JSON encode/decode 行为，应与 proto 类型的序列化策略对齐

### 冗余清理要求

- 删除或停止使用 live 路径中的 `APIErrorResp[T]` / `ApiErrorResp[T]` / `errorResponse` 残留。
- 删除已无 live 引用的手写 auth/device/runtime DTO。
- 测试不应继续依赖旧业务 DTO 解析响应；必要时使用生成 proto 或测试局部结构。
- 不新增新的 hand-written DTO 来绕过 proto 已覆盖的请求/响应体。

### 行为约束

- 不改变公开 API 路径。
- 不改变前端已依赖的 JSON 字段名，除非 proto 契约明确要求并同步修正前后端。
- 不静默改变错误响应结构。
- 不引入兼容层或适配层保留旧 DTO；迁移暴露的问题要直接修正。
- 对 `encoding/json` 与 `protojson` 的行为差异必须显式处理，尤其是未知字段、空字段、nil message、timestamp 和 optional/presence。

### 验证要求

实现后至少需要：

- `go test ./cmd/... ./internal/...`
- 如涉及前端契约或生成物，再运行相关前端 typecheck / 项目 check。
- 对关键接口进行行为核对：
  - `/agent-api/health`
  - `/cloud-api/health`
  - cloud auth login/register/email/password/google callback 相关接口
  - agent/cloud connect 或 device report 相关接口
  - device list / current device 相关接口
  - runtime request 相关接口
  - terminal WebSocket control message 相关交互

## Open questions

1. 请求 decode 是否统一切换到 `codec.UnmarshalProtoJSON`？
   - 需要正面决策；不能因为 unknown field 行为差异而保留手写 DTO。
   - 如果行为变化是正确的，应记录并通过测试固定。

2. proto response 是否统一通过 `codec.MarshalProtoJSON` 输出？
   - 默认建议：是。不要直接用 `encoding/json` 输出 proto message。

3. `time.Time` 到 `google.protobuf.Timestamp` 的 zero value 行为如何处理？
   - 必须在迁移 `DeviceSummary`、`CloudSessionSummary` 时明确。
   - 不允许以此为理由长期保留重复手写 DTO。

4. `AuthMeResp.cloud_session` 的 nil 输出从 `null` 变为省略字段时是否接受？
   - 如果 protojson 行为为准，应同步前端处理。
   - 不通过兼容层维持旧输出。

5. application 层是否接受 `runtimev1.*`，还是需要定义真正的内部业务命令模型？
   - 如果当前 `internal/agent/application/task/terminal/dto.go` 只是 proto 的重复定义，应删除并引用生成类型。
   - 如果需要内部命令模型，必须与 wire DTO 明确区分，不得继续作为 API 请求/响应体冗余存在。

## Decisions

- 使用轻量模式 / light 记录调查结果并推进处理。
- 本文档为新任务，不沿用历史 frontend proto DTO 文档。
- 本阶段只记录 Requirement / 需求和调查结论，不修改产品代码。
- 用户已明确：所有 API 请求/响应体都要迁移到生成 proto 类型。
- 用户已明确：遇到问题解决问题，不做适配层、兼容层。
- 用户判断：很多 proto 定义已经就绪，当前主要问题是业务代码里存在冗余手写定义。
- 后续实现可以分步骤推进，但每一步都应删除对应范围内的冗余手写 DTO，而不是保留两套 wire 类型。

## Risk

1. **JSON 行为差异风险**：`encoding/json` 和 `protojson` 在未知字段、空字段、nil message、默认值输出上不完全一致，可能导致客户端观察到行为变化。该问题要通过明确契约和测试解决，不通过兼容层绕过。
2. **timestamp 风险**：`time.Time` 到 `google.protobuf.Timestamp` 需要明确 zero time 和输出格式。迁移 device/cloud session 时必须正面处理。
3. **nil/null 风险**：`AuthMeResp.cloud_session` 等 nested message 的 nil 输出可能从 `null` 变成省略字段。若 protojson 是目标契约，应同步修正前端处理。
4. **optional/presence 风险**：runtime request 中的 `optional` 字段可能改变“未传字段”和“传空值”的区分方式。迁移时应固定期望语义。
5. **内部模型污染风险**：如果直接把 generated proto message 传入 application/domain 层，可能让 wire 契约反向影响业务模型。若确需内部模型，应定义真正业务语义，而不是保留 API DTO 的重复手写版本。
6. **测试耦合风险**：部分手写 DTO 可能只剩测试解析使用，删除时需要同步测试结构或改用生成类型。
7. **范围较大风险**：全部迁移会触及 auth、device、runtime、terminal 多条链路。可以分步骤做，但最终目标不变：API 请求/响应体不再维护同 shape 手写 DTO。

## User review notes

- 用户要求：记录后端 API DTO 调查结果，并准备推进处理。
- 用户补充：都要迁移，遇到问题就解决问题，但不做适配层、兼容层。
- 用户判断：很多定义已经就绪，只是业务里冗余了。
- 本文档基于已完成的只读扫描结果和用户补充决策整理。
