# internal agent/cloud 拆分规格

最后修改时间: 2026-07-02 18:01:55

Review status: Accepted

## Requirement basis

本规格基于已接受的需求文档：

```text
docs/requirement/20260702-agent-cloud-internal-split.md
```

需求阶段已确认的关键约束：

1. 只在同一个 repo 的 `internal` 下拆分 agent 和 cloud，不拆成两个项目。
2. agent/local HTTP API 与 cloud HTTP API 从 route shape 开始分离，各自管理 API 包。
3. 两侧只共享 proto generated wire model，不共享 HTTP route adapter / handler implementation。
4. Protocol Buffers 作为 IDL / Schema / Contract 使用，不等同于必须采用 gRPC。
5. Go generated proto model 输出到 `internal/gen/proto/...`。
6. 第一阶段交付 Go generated proto model；TypeScript client model 和前端 proto 工具链留作后续阶段评估。
7. 第一阶段 proto 定义 messages；proto service / HTTP annotation 留作后续阶段评估。
8. Go 侧 protobuf JSON codec 使用 `google.golang.org/protobuf/encoding/protojson`。
9. tunnel 当前 JSON envelope/control 部分可以切到 proto；binary 数据通道保留二进制语义。
10. `protocol/tunnel.Frame` 迁移到 generated `tunnel.v1.TunnelFrame`。
11. 数据库 migration 本次同步拆为 role-specific migrations。
12. 运行角色由 `termbridge agent` / `termbridge cloud` 表达。
13. API error response 同步 proto 化。
14. 协议和边界模型遵守现有 `Req` / `Resp` 命名规约。
15. 原 `internal/application` 和 `internal/infrastructure` 下的能力需要由 `internal/agent` 或 `internal/cloud` 各自认领；真正共享的部分保持少量并进入 `internal/shared`。
16. runtime session 模型只保留一种 `SessionSummary`；所有 session 都归属 workspace，列表场景也携带 `workspace_id`。
17. 不单独引入 `RuntimeReq` / `RuntimeResp` 这类 runtime command envelope；远端 runtime 操作由 `TunnelFrame` 直接承载具体 runtime `Req` / `Resp` message。

## Overview

本次拆分采用“角色包 + generated Go wire model + 少量 shared primitives”的架构：

```text
proto/termbridge/*/v1/*.proto
  -> internal/gen/proto/termbridge/*/v1      Go wire model

internal/agent/...
  -> 本地 server / PTY CLI / 本地 runtime / tunnel client / agent-owned application 与 infrastructure

internal/cloud/...
  -> cloud portal / auth / device binding / tunnel ingress / runtime proxy / cloud-owned application 与 infrastructure

internal/shared/
  validation/   # 示例，按真实需求保留或调整
  auth/         # 示例，按真实需求保留或调整
  errors/       # 示例，按真实需求保留或调整
  -> 只放真正中立、低层、无角色语义的共享能力；具体目录以实施中真实共享需求为准
```

设计目标是把 protobuf 固定在跨边界的 wire contract：

```text
HTTP JSON request/response schema
agent <-> cloud tunnel frame schema
runtime operation Req/Resp schema
API error response schema
```

内部 domain、application service、repository、runtime store 和 PTY 继续使用角色内手写 Go 类型。agent/cloud API 层和 tunnel 层负责在 generated proto model 与各自 service/domain model 之间做 mapper。

`internal/application` 与 `internal/infrastructure` 的能力按角色认领：agent 需要的 terminal/runtime/state/pty/device identity 等能力进入 `internal/agent/...`，cloud 需要的 user auth/device binding/oauth/email/tunnel server 等能力进入 `internal/cloud/...`。两侧共享能力限定为中立 primitive：无角色语义、无具体 repository/runtime 实现依赖。

## Design decisions

### 1. 包边界

目标包结构：

```text
internal/
  agent/
    app/
    api/
    application/
      runtime/
      terminal/
      device/
      tunnelclient/
    infrastructure/
      config/
      database/
      repository/
        state/
      pty/
    migrations/

  cloud/
    app/
    api/
    application/
      auth/
      device/
      oauth/
      tunnelserver/
    infrastructure/
      config/
      database/
      repository/
        auth/
        device/
      email/
    migrations/

  shared/
    validation/   # 示例，只有真实中立 validation 需求时保留
    auth/         # 示例，只有真实中立 auth primitive 需求时保留
    errors/       # 示例，只有真实中立 error primitive 需求时保留

  gen/
    proto/
      termbridge/
        common/v1/
        runtime/v1/
        cloud/v1/
        tunnel/v1/
```

允许依赖方向：

```text
internal/agent/...  -> internal/gen/proto/...
internal/agent/...  -> internal/shared/...
internal/agent/...  -> internal/agent/application/...
internal/agent/...  -> internal/agent/infrastructure/...

internal/cloud/...  -> internal/gen/proto/...
internal/cloud/...  -> internal/shared/...
internal/cloud/...  -> internal/cloud/application/...
internal/cloud/...  -> internal/cloud/infrastructure/...

internal/shared/... -> stdlib 或明确中立的第三方库
```

禁止依赖方向：

```text
internal/agent/...  -> internal/cloud/...
internal/cloud/...  -> internal/agent/...
internal/cloud/...  -> internal/agent/infrastructure/pty
internal/cloud/...  -> internal/agent/infrastructure/repository/state
internal/cloud/...  -> terminal.Registry / PTY runtime implementation
internal/agent/...  -> cloud-only auth repository / cloud-only device binding service
internal/shared/... -> internal/agent/...
internal/shared/... -> internal/cloud/...
```

迁移期间按文件列出 `internal/application/*` 和 `internal/infrastructure/*` 的角色认领路径，避免形成新的“第三个共享 application/infrastructure 层”。

### 2. shared 目录边界

`internal/shared` 只承载真正中立的低层能力。建议第一阶段只允许以下方向：

```text
internal/shared/validation
  - ID/name/path/basic input validation helpers
  - 只依赖基础类型和中立 helper

internal/shared/auth
  - tunnel signed request 的 header 名称、签名 canonical string、verify/sign primitive
  - 通用 token/key parsing primitive
  - 不包含 cloud user auth service、OAuth flow、device binding workflow

internal/shared/errors
  - 边界错误 code 常量
  - error classification / mapping primitive
  - 不包含某个 API handler 的完整 response writer
```

本地 device identity 文件管理属于 agent 角色能力：

```text
internal/agent/application/device 或 internal/agent/infrastructure/deviceidentity
  - LoadOrCreateDevice
  - LoadDevicePrivateKey
  - LoadDevicePublicKey
```

cloud 保存和读取 device public key，并使用 `internal/shared/auth` 的验签 primitive；agent 的 device identity 实现留在 agent 角色内。

### 3. API 包不共享 route implementation

agent/local API 和 cloud API 各自拥有：

```text
route parser
middleware
codec
mapper
error policy
handler implementation
```

agent/cloud 各自拥有 HTTP runtime route adapter。两侧共享 generated proto messages 与少量 shared primitives。

agent API 负责本机 runtime：

```text
GET  /api/workspaces
POST /api/sessions
GET  /api/workspaces/{workspace_id}/sessions
WS   /api/workspaces/{workspace_id}/sessions/{session_id}/terminal
```

cloud API 负责 device-scoped runtime proxy：

```text
GET  /api/devices/{device_id}/workspaces
POST /api/devices/{device_id}/sessions
GET  /api/devices/{device_id}/workspaces/{workspace_id}/sessions
WS   /api/devices/{device_id}/workspaces/{workspace_id}/sessions/{session_id}/terminal
```

`device_id` 是 cloud route context，只用于 device ownership check、online route lookup 和 tunnel route selection，不进入 runtime operation proto payload。

### 4. proto source layout

新增 proto source root：

```text
proto/
  termbridge/
    common/v1/common.proto
    runtime/v1/runtime.proto
    cloud/v1/cloud.proto
    tunnel/v1/tunnel.proto
```

第一阶段只定义 messages，不定义 service，不使用 HTTP annotation。

Go package 输出：

```text
internal/gen/proto/termbridge/common/v1
internal/gen/proto/termbridge/runtime/v1
internal/gen/proto/termbridge/cloud/v1
internal/gen/proto/termbridge/tunnel/v1
```

第一阶段交付 Go package；TypeScript package、`web/src/gen/proto/...`、前端 proto runtime/generator 留作后续阶段评估。

### 5. proto 生成工具

规格建议第一阶段采用 Go-only 工具链：

```text
buf             统一 lint / generate / breaking check 入口
protoc-gen-go   生成 Go model
```

后续阶段可评估：

```text
@bufbuild/protobuf
protoc-gen-es
ts-proto
Connect/gRPC server
grpc-gateway
HTTP annotations
```

Go side JSON codec 使用：

```go
google.golang.org/protobuf/encoding/protojson
```

统一 codec 策略：

```go
protojson.MarshalOptions{
    UseProtoNames:   true,
    EmitUnpopulated: false,
}

protojson.UnmarshalOptions{
    DiscardUnknown: false,
}
```

### 6. Req/Resp 命名规约

所有 request / response message 使用现有 `Req` / `Resp` 命名规约。

采用：

```proto
message CreateSessionReq {}
message CreateSessionResp {}
message ListWorkspacesReq {}
message ListWorkspacesResp {}
message ErrorResp {}
message DeleteSessionResp {}
```

避免：

```proto
message CreateSessionRequest {}
message CreateSessionResponse {}
```

非 request/response 的实体模型不强制加 `Req` / `Resp`，例如：

```proto
message Workspace {}
message SessionSummary {}
message TunnelFrame {}
message DeviceSummary {}
```

### 7. common proto

`common.v1` 承载通用边界模型。

建议核心 messages：

```proto
message ErrorResp {
  string code = 1;
  string error = 2;
  string request_id = 3;
  google.protobuf.Struct details = 4;
}

message HealthResp {
  string status = 1;
}
```

说明：

1. error response 同步 proto 化。
2. 字段名使用 snake_case proto name，JSON 使用 `UseProtoNames: true`。
3. 现有 `requestId` camelCase 会迁移为 `request_id`；这是 error response 层面的 breaking change，需要在 Plan 阶段同步处理前端 error handling 或明确兼容策略。

### 8. runtime proto

`runtime.v1` 是 agent/cloud runtime 能力共享 contract，但保持 device-neutral。

所有 runtime session 都归属 workspace，因此只保留一种 session summary：

```proto
message Workspace {
  string id = 1;
  string name = 2;
  string path = 3;
  google.protobuf.Timestamp updated_at = 4;
}

message WorkspaceTreeNode {
  string id = 1;
  string name = 2;
  string path = 3;
  google.protobuf.Timestamp updated_at = 4;
  repeated SessionSummary children = 5;
}

message SessionSummary {
  string id = 1;
  string workspace_id = 2;
  string name = 3;
  string command = 4;
  string cwd = 5;
  string lifecycle_state = 6;
  string attachment_state = 7;
  optional int32 exit_code = 8;
  google.protobuf.Timestamp updated_at = 9;
}
```

统一使用 `SessionSummary`。即使在 `/api/workspaces/{workspace_id}/sessions` 这类 route 下，response item 也保留 `workspace_id`，避免同一业务实体因为查询上下文不同分裂成两个几乎相同的 DTO。

建议 request / response messages：

```proto
message ListWorkspacesReq {}
message ListWorkspacesResp { repeated Workspace items = 1; }

message WorkspaceTreeReq {}
message WorkspaceTreeResp { repeated WorkspaceTreeNode items = 1; }

message UpdateWorkspaceOrderReq { repeated string workspace_ids = 1; }
message UpdateWorkspaceOrderResp { repeated Workspace items = 1; }

message DeleteWorkspaceReq { string workspace_id = 1; }
message DeleteWorkspaceResp {}

message CreateSessionReq {
  string workspace_id = 1;
  string name = 2;
  string cwd = 3;
  repeated string command = 4;
  int32 cols = 5;
  int32 rows = 6;
}

message CreateSessionResp {
  string session_id = 1;
  string workspace_id = 2;
  string state = 3;
}

message WorkspaceSessionsReq { string workspace_id = 1; }
message WorkspaceSessionsResp { repeated SessionSummary items = 1; }

message WorkspaceSessionReq {
  string workspace_id = 1;
  string session_id = 2;
}

message RerunSessionReq {
  string workspace_id = 1;
  string session_id = 2;
  int32 cols = 3;
  int32 rows = 4;
}

message UpdateSessionReq {
  string workspace_id = 1;
  string session_id = 2;
  optional string name = 3;
}

message DeleteSessionResp {}

message CloseSessionResp { SessionSummary session = 1; }

message UpdateSessionOrderReq {
  string workspace_id = 1;
  repeated string session_ids = 2;
}
message UpdateSessionOrderResp { repeated SessionSummary items = 1; }

message ReadHistoryReq {
  string workspace_id = 1;
  string session_id = 2;
}
message ReadHistoryResp { string text = 1; }
```

Update 类请求使用 `optional` 或后续 `FieldMask` 以保留 field presence。

### 9. 远端 runtime 操作使用 typed payload

当前 tunnel 的 runtime 操作表达方式是 `method string + raw JSON params`。目标是使用 typed runtime message、frame-level `request_id` 和 `oneof payload` 表达请求响应关联。

本规格移除独立 runtime command envelope，采用以下原则：

1. `runtime.v1` 定义具体操作的 typed `Req` / `Resp` message。
2. HTTP API 直接使用这些 `Req` / `Resp` 作为 request/response schema。
3. tunnel 远端调用时，由 `tunnel.v1.TunnelFrame` 的 frame-level `request_id` 做关联，并由 `oneof payload` 直接承载具体 runtime `Req` / `Resp`。
4. 不定义 `RuntimeReq` / `RuntimeResp`，避免出现“HTTP route 已经表达操作 + tunnel payload 再表达 command”的重复抽象。

这使 agent/cloud 仍能获得 typed contract，同时减少一层 dispatcher 抽象。

### 10. tunnel proto

`tunnel.v1` 是 tunnel wire contract。

建议核心结构：

```proto
message TunnelFrame {
  string stream_id = 1;
  string request_id = 2;

  oneof payload {
    Hello hello = 10;
    HelloAck hello_ack = 11;
    Ping ping = 12;
    Pong pong = 13;

    termbridge.runtime.v1.ListWorkspacesReq list_workspaces_req = 20;
    termbridge.runtime.v1.ListWorkspacesResp list_workspaces_resp = 21;
    termbridge.runtime.v1.WorkspaceTreeReq workspace_tree_req = 22;
    termbridge.runtime.v1.WorkspaceTreeResp workspace_tree_resp = 23;
    termbridge.runtime.v1.WorkspaceSessionsReq workspace_sessions_req = 24;
    termbridge.runtime.v1.WorkspaceSessionsResp workspace_sessions_resp = 25;
    termbridge.runtime.v1.CreateSessionReq create_session_req = 26;
    termbridge.runtime.v1.CreateSessionResp create_session_resp = 27;
    termbridge.runtime.v1.WorkspaceSessionReq get_session_req = 28;
    termbridge.runtime.v1.SessionSummary get_session_resp = 29;
    termbridge.runtime.v1.RerunSessionReq rerun_session_req = 30;
    termbridge.runtime.v1.CreateSessionResp rerun_session_resp = 31;
    termbridge.runtime.v1.UpdateSessionReq update_session_req = 32;
    termbridge.runtime.v1.SessionSummary update_session_resp = 33;
    termbridge.runtime.v1.WorkspaceSessionReq close_session_req = 34;
    termbridge.runtime.v1.CloseSessionResp close_session_resp = 35;
    termbridge.runtime.v1.WorkspaceSessionReq delete_session_req = 36;
    termbridge.runtime.v1.DeleteSessionResp delete_session_resp = 37;
    termbridge.runtime.v1.ReadHistoryReq read_history_req = 38;
    termbridge.runtime.v1.ReadHistoryResp read_history_resp = 39;
    termbridge.runtime.v1.UpdateWorkspaceOrderReq update_workspace_order_req = 40;
    termbridge.runtime.v1.UpdateWorkspaceOrderResp update_workspace_order_resp = 41;
    termbridge.runtime.v1.UpdateSessionOrderReq update_session_order_req = 42;
    termbridge.runtime.v1.UpdateSessionOrderResp update_session_order_resp = 43;
    termbridge.runtime.v1.DeleteWorkspaceReq delete_workspace_req = 44;
    termbridge.runtime.v1.DeleteWorkspaceResp delete_workspace_resp = 45;

    TerminalAttachReq terminal_attach = 60;
    TerminalInput terminal_input = 61;
    TerminalOutput terminal_output = 62;
    TerminalResize terminal_resize = 63;
    TerminalClosed terminal_closed = 64;

    termbridge.common.v1.ErrorResp error = 90;
    Close close = 91;
  }
}

message Hello {
  string device_id = 1;
  string device_name = 2;
  int32 protocol_version = 3;
}

message HelloAck {
  int32 protocol_version = 1;
}

message Ping { string nonce = 1; }
message Pong { string nonce = 1; }

message TerminalAttachReq {
  string workspace_id = 1;
  string session_id = 2;
  int32 cols = 3;
  int32 rows = 4;
}

message TerminalInput { bytes data = 1; }
message TerminalOutput { bytes data = 1; }

message TerminalResize {
  int32 cols = 1;
  int32 rows = 2;
}

message TerminalClosed { string reason = 1; }
message Close { string reason = 1; }
```

编码策略：

1. tunnel WebSocket 的 JSON envelope/control 部分使用 `protojson(TunnelFrame)`。
2. `request_id` 放在 `TunnelFrame` 顶层，避免每个 tunnel-only message 重复携带；runtime HTTP message 内部仍可按自身 schema 保持必要字段。
3. terminal data 在 tunnel frame 中仍可使用 `bytes`，JSON 表达会是 base64；如性能或体积不可接受，后续可进一步把 data frame 切为 websocket binary frame。
4. 浏览器 terminal WebSocket 的现有 binary PTY data 语义不因 tunnel proto 迁移而改变。

### 11. cloud proto

`cloud.v1` 承载 cloud auth/device/cloud-oauth 边界模型。

建议核心 messages：

```proto
message User {
  string id = 1;
  string email = 2;
  string display_name = 3;
  bool email_verified = 4;
}

message DeviceSummary {
  string id = 1;
  string name = 2;
  bool online = 3;
  string status = 4;
  google.protobuf.Timestamp connected_at = 5;
  google.protobuf.Timestamp last_seen = 6;
}

message CloudSessionSummary {
  string gate_url = 1;
  string device_id = 2;
  string device_name = 3;
  google.protobuf.Timestamp connected_at = 4;
}

message AuthLoginReq {
  string email = 1;
  string username = 2;
  string password = 3;
}

message TokenResp {
  string access_token = 1;
  string token_type = 2;
}

message AuthMeResp {
  bool authenticated = 1;
  string username = 2;
  User user = 3;
  CloudSessionSummary cloud_session = 4;
}

message ListDevicesResp {
  repeated DeviceSummary items = 1;
}

message CurrentDeviceReq {
  string id = 1;
  string name = 2;
  string public_key = 3;
}

message CurrentDeviceResp {
  bool accepted = 1;
  DeviceSummary device = 2;
}

message CloudOAuthStartResp { string authorize_url = 1; }
message CloudOAuthAuthorizeResp { string redirect_url = 1; }
message CloudOAuthExchangeReq { string code = 1; }
message CloudOAuthCallbackReq { string code = 1; string state = 2; }
message CloudOAuthCallbackResp { CloudSessionSummary cloud_session = 1; string redirect = 2; }
```

`CloudSessionSummary` 表示本地 agent 与 cloud 的连接/授权会话摘要，不表示 runtime terminal session；runtime terminal session 统一使用 `runtime.v1.SessionSummary`。

cloud auth 相关 message 第一阶段以现有 API DTO 为准迁移；具体覆盖范围在 Plan 阶段按 handler 拆分顺序落地。

### 12. agent app / API 设计

`internal/agent/app` 是本地角色 composition root。

职责：

```text
1. 加载 agent role config。
2. 初始化本地 runtime state dir。
3. 加载或创建本地 device identity。
4. 初始化 agent role database 和 agent migrations。
5. 创建 RuntimeStore / terminal.Registry / runtime access service。
6. 创建 internal/agent/api Handler。
7. 按 cloud gate 配置创建 tunnel client。
8. 运行本地 server / PTY CLI / workspace/session CLI。
```

`internal/agent/api` 只处理本地 API：

```text
/api/health
/api/auth/login       可选本地 admin
/api/auth/logout
/api/auth/me
/api/workspaces
/api/workspaces/...
/api/sessions
/api/devices          本地 device/cloud connection summary
/api/cloud-oauth/start
/api/cloud-oauth/callback
```

不得处理：

```text
/api/auth/register
/api/auth/google
/api/auth/password-reset/*
/api/cloud-oauth/authorize
/api/cloud-oauth/exchange
/api/devices/current
/api/devices/{device_id}/...
/api/agent/tunnel
```

agent-owned application/infrastructure 认领方向：

```text
internal/application/terminal/*              -> internal/agent/application/terminal 或 runtime
internal/application/agent/runtime_access.go -> internal/agent/application/runtime
internal/application/agent/client.go         -> internal/agent/application/tunnelclient
internal/application/agent/device.go         -> internal/agent/application/device 或 infrastructure/deviceidentity
internal/infrastructure/pty/*                -> internal/agent/infrastructure/pty
internal/infrastructure/repository/state/*   -> internal/agent/infrastructure/repository/state
agent runtime migrations                     -> internal/agent/migrations
```

### 13. cloud app / API 设计

`internal/cloud/app` 是云端角色 composition root。

职责：

```text
1. 加载 cloud role config。
2. 初始化 cloud database 和 cloud migrations。
3. 创建 auth service / device service / cloud OAuth service。
4. 创建 tunnel registry / tunnel server。
5. 创建 internal/cloud/api Handler。
6. 运行 cloud HTTP server。
```

`internal/cloud/api` 处理云端 API：

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
/api/cloud-oauth/authorize
/api/cloud-oauth/exchange
/api/devices
/api/devices/current
/api/devices/{device_id}
/api/devices/{device_id}/workspaces
/api/devices/{device_id}/sessions
/api/agent/tunnel
```

不得处理：

```text
local /api/workspaces
local /api/sessions
local /api/cloud-oauth/start
local /api/cloud-oauth/callback
local PTY/runtime registry
```

cloud-owned application/infrastructure 认领方向：

```text
internal/application/auth/*                  -> internal/cloud/application/auth
cloud OAuth attempt/store/service            -> internal/cloud/application/oauth
cloud device binding service                 -> internal/cloud/application/device
internal/infrastructure/repository/auth/*    -> internal/cloud/infrastructure/repository/auth
internal/infrastructure/repository/device/*  -> internal/cloud/infrastructure/repository/device
internal/infrastructure/email/*              -> internal/cloud/infrastructure/email
cloud auth/device migrations                 -> internal/cloud/migrations
```

### 14. tunnel auth / device identity

本地 device identity 与 tunnel signing 当前都混在 `internal/application/agent` 语义下。拆分后按职责分配：

```text
internal/agent/application/device 或 internal/agent/infrastructure/deviceidentity
  Device
  LoadOrCreateDevice
  LoadPrivateKey
  LoadPublicKey

internal/shared/auth
  SignedTunnelHeader 或更低层的 SignTunnelRequest
  VerifySignedRequest
  HeaderDeviceId
  HeaderSignature
  HeaderTimestamp
  HeaderNonce
```

agent 用 agent-owned device identity 加载本地身份，并用 `internal/shared/auth` 的签名 primitive 生成 tunnel request header。

cloud 用 cloud-owned device repository 保存 public key，并用 `internal/shared/auth` 的验签 primitive 校验 tunnel request。

cloud tunnel ingress 生产路径使用 signed tunnel auth；测试场景使用显式 test-only fake 或本地 dev 配置隔离。

### 15. role-specific migrations

migration runner 与 migration source 由角色认领，入口为 role-specific migrator。

建议入口：

```go
agentdb.Migrate(ctx, db, driver)
clouddb.Migrate(ctx, db, driver)
```

建议 migration source：

```text
internal/agent/migrations/sqlite/*.sql
internal/agent/migrations/mysql/*.sql
internal/cloud/migrations/sqlite/*.sql
internal/cloud/migrations/mysql/*.sql
```

agent schema 包含：

```text
local_devices 或 devices 中 agent-local 必需字段
workspaces
sessions
session_runs
```

cloud schema 包含：

```text
users
user_identities
auth_codes
email_delivery_logs
oauth_states
devices
user_devices
device_binding_codes
```

兼容策略建议：

1. 新 agent DB 只执行 agent migrations。
2. 新 cloud DB 只执行 cloud migrations。
3. 已有统一 DB 通过 schema state detection 识别，保留既有表。
4. split 之后的新 migration 使用 role-specific migration set。
5. 多余表清理作为后续数据维护任务。

### 16. 配置和入口

local/cloud 角色由 CLI 入口表达。

建议 CLI / composition 方向：

```text
termbridge agent                 启动 agent/local 业务入口，包括本地 HTTP API、PTY/runtime 和可选 cloud connector
termbridge cloud                 启动 cloud 业务入口，包括 cloud HTTP API、auth/device binding 和 tunnel ingress
termbridge exec -- <command>     agent role CLI
termbridge workspace             agent role CLI
termbridge session               agent role CLI
termbridge migrate agent
termbridge migrate cloud
```

长期入口为 `termbridge agent` 和 `termbridge cloud`。如需迁移提示，Plan 阶段将其列为显式提示或短期 alias，而不是实际 role selector。

配置结构建议按 role 拆分：

```text
agent.server.*
agent.runtime.*
agent.cloud.gate_url
agent.oauth.*
agent.database.*
agent.history.*
agent.auth.local_admin.*

cloud.server.*
cloud.database.*
cloud.auth.*
cloud.oauth.*
cloud.tunnel.*
cloud.resend.*
```

现有 `configs/config.yaml` 的迁移方式在 Plan 阶段细化。规格层面确认：运行角色来源为 CLI 入口。

## Affected components

### Go backend

```text
internal/app/app.go
internal/transport/cli/cli.go
internal/transport/http/gatewayapi/*
internal/application/agent/*
internal/application/auth/*
internal/application/terminal/*
internal/infrastructure/config/config.go
internal/infrastructure/database/*
internal/infrastructure/database/migrations/*
internal/infrastructure/repository/auth/*
internal/infrastructure/repository/device/*
internal/infrastructure/repository/state/*
internal/infrastructure/email/*
internal/infrastructure/pty/*
internal/protocol/tunnel/*
internal/protocol/terminal/*
```

### New / target backend packages

```text
internal/agent/app
internal/agent/api
internal/agent/application/runtime
internal/agent/application/terminal
internal/agent/application/device
internal/agent/application/tunnelclient
internal/agent/infrastructure/config
internal/agent/infrastructure/database
internal/agent/infrastructure/repository/state
internal/agent/infrastructure/pty
internal/agent/migrations

internal/cloud/app
internal/cloud/api
internal/cloud/application/auth
internal/cloud/application/device
internal/cloud/application/oauth
internal/cloud/application/tunnelserver
internal/cloud/infrastructure/config
internal/cloud/infrastructure/database
internal/cloud/infrastructure/repository/auth
internal/cloud/infrastructure/repository/device
internal/cloud/infrastructure/email
internal/cloud/migrations

internal/shared/validation
internal/shared/auth
internal/shared/errors

internal/gen/proto/termbridge/*/v1
```

### Proto and generated code

```text
proto/termbridge/common/v1/common.proto
proto/termbridge/runtime/v1/runtime.proto
proto/termbridge/cloud/v1/cloud.proto
proto/termbridge/tunnel/v1/tunnel.proto
buf.yaml
buf.gen.yaml
internal/gen/proto/...
```

不包含：

```text
web/src/gen/proto/...
frontend protobuf runtime/generator dependency
```

### Frontend

```text
web/src/features/gateway/api.ts
web/src/features/runtimeTarget.ts
web/src/store/*
```

Frontend 影响重点：

1. 适配 proto 化 error response，尤其是 `requestId` 到 `request_id` 的兼容策略。
2. 保持 local/cloud route target 规则清晰：local path 与 device-scoped cloud path 仍由 frontend runtime target 决定。
3. 第一阶段交付 Go proto model；前端仍使用现有手写类型或局部调整。

### Config / packaging

```text
configs/config.yaml
configs/config.<env>.yaml
package / portable zip config layering
README / docs references
```

## Interfaces

### Agent API interface

agent API 使用 generated Go proto messages 作为 JSON request/response schema，但 route 由 `internal/agent/api` 管理。

示例：

```text
GET /api/workspaces
  -> runtimev1.ListWorkspacesResp

POST /api/sessions
  body runtimev1.CreateSessionReq
  -> runtimev1.CreateSessionResp

GET /api/workspaces/{workspace_id}/sessions
  -> runtimev1.WorkspaceSessionsResp，其中 item 为 runtimev1.SessionSummary 且包含 workspace_id

POST /api/cloud-oauth/callback
  body cloudv1.CloudOAuthCallbackReq
  -> cloudv1.CloudOAuthCallbackResp
```

### Cloud API interface

cloud API 使用同一 runtime proto messages，但 device routing 是 cloud API 自身的 path context。

示例：

```text
GET /api/devices/{device_id}/workspaces
  -> runtimev1.ListWorkspacesResp

POST /api/devices/{device_id}/sessions
  body runtimev1.CreateSessionReq
  -> runtimev1.CreateSessionResp

GET /api/devices/{device_id}/workspaces/{workspace_id}/sessions
  -> runtimev1.WorkspaceSessionsResp，其中 item 为 runtimev1.SessionSummary 且包含 workspace_id

POST /api/devices/current
  body cloudv1.CurrentDeviceReq
  -> cloudv1.CurrentDeviceResp
```

### Tunnel interface

agent/cloud tunnel 使用 `tunnelv1.TunnelFrame`。

初始编码：

```text
WebSocket text frame = protojson(tunnelv1.TunnelFrame)
```

后续可扩展：

```text
WebSocket binary frame = proto binary(tunnelv1.TunnelFrame)
```

本阶段使用 JSON 编码的 protobuf message。

远端 runtime 操作通过 `TunnelFrame.request_id + TunnelFrame.payload(oneof concrete runtime Req/Resp)` 表达，不使用 `RuntimeReq` / `RuntimeResp` 二次 envelope。

### Mapper interface

mapper 按角色包内聚，不共享：

```text
internal/agent/api/mapper_*.go
internal/cloud/api/mapper_*.go
internal/agent/application/tunnelclient/mapper_*.go
internal/cloud/application/tunnelserver/mapper_*.go
```

示例职责：

```text
agent runtime service WorkspaceSummary -> runtimev1.Workspace
agent runtime service SessionSummary -> runtimev1.SessionSummary
runtimev1.CreateSessionReq -> agent runtime service command
runtimev1.SessionSummary -> cloud API response through tunnel proxy
cloud device repository model -> cloudv1.DeviceSummary
```

## Technical questions

以下问题已由用户在进入 Plan 前确认：

1. Go-only proto 工具链采用 `buf + protoc-gen-go`。
2. error response JSON 字段接受从现有 `requestId` 迁移为 `request_id`。
3. `termbridge` 使用 `termbridge agent` 和 `termbridge cloud` 作为 agent/cloud 业务入口。
4. `internal/shared` 的具体目录按真实中立共享需求决定，`validation` / `auth` / `errors` 只是示例。

以下事项进入 Plan 阶段细化：

1. role-specific migrations 对已有统一 DB 的 schema state detection 具体策略和测试矩阵。
2. tunnel runtime operation 如何在 `TunnelFrame` 中保持 typed contract、请求响应关联和可演进性。
3. tunnel terminal `bytes` 使用 protojson base64 是否足够；若不可接受，Plan 阶段需要增加 tunnel binary data frame 设计。

## Risks

1. agent/cloud 包移动范围大，容易产生 import cycle。
2. API handler 拆分时容易改变现有 status code、auth 行为、CORS 或 error details。
3. proto JSON 字段规则如果不统一，会导致 Go handler、前端手写 DTO 与生成模型出现 snake_case / lowerCamelCase 漂移。
4. error response proto 化会影响前端错误处理，属于明确的 API contract 变化。
5. tunnel frame 不兼容迁移要求 agent/cloud 同步升级；半升级状态不可用。
6. role-specific migration 拆分若处理不严谨，可能导致新安装缺表或已有 DB 重复执行 schema。
7. 移除 `server.mode` 会影响现有配置、环境变量和测试 fixture。
8. 直接把 generated proto struct 传入 domain/application 层会污染业务模型，必须通过 mapper 隔离。
9. 如果 `internal/shared/auth` 承载过多 cloud user auth 或 OAuth 语义，会重新形成跨角色耦合。
10. `internal/application` / `internal/infrastructure` 需要完成角色认领，避免目录移动完成后仍留下架构债务。
11. tunnel `TunnelFrame` oneof 直接列出 concrete runtime operations，会随 runtime API 增长而扩展字段；这是为了消除 runtime command envelope 的显式取舍，需要用 buf breaking/lint 控制演进。

## Alternatives

### Alternative 1: 保留 gatewayapi，继续用 mode 分支

拒绝。

原因：这会保留当前耦合，继续让 local/cloud route、auth、tunnel、runtime endpoint 混在同一个 handler 中，不满足拆分目标。

### Alternative 2: 共享一个 runtime HTTP route adapter

拒绝。

原因：agent route 与 cloud device-scoped route 从 HTTP 语义开始不同。共享 route adapter 会把 `device_id optional`、auth 差异、error policy 差异重新揉到一个包里。

### Alternative 3: 第一阶段同步生成 TypeScript client model

拒绝 / 延后。

原因：用户已明确第一阶段交付 Go generated proto model。前端 proto model 可作为后续独立需求重新评估。

### Alternative 4: 引入 gRPC / Connect 作为第一阶段传输协议

拒绝。

原因：需求明确 proto 作为 IDL / Schema / Contract，REST/WebSocket 仍是本阶段传输形态。Connect 可以作为后续演进方向。

### Alternative 5: tunnel Frame 兼容层

拒绝。

原因：需求已确认 `protocol/tunnel.Frame` 迁移到 generated `tunnel.v1.TunnelFrame`。

### Alternative 6: migration 拆分延后

拒绝。

原因：需求已确认本次同步拆 role-specific migrations。已有统一 DB 通过 schema state detection 处理，新角色执行各自 migration set。

### Alternative 7: 保留 `RuntimeReq` / `RuntimeResp` command envelope

拒绝。

原因：HTTP route 已经表达 runtime 操作，tunnel frame 也已经有 `oneof payload` 和 frame-level `request_id`。再引入 `RuntimeReq` / `RuntimeResp` 会形成重复 envelope，并把 typed operation 包进额外 dispatcher 层。

### Alternative 8: 同时保留 `SessionSummary` 和 `WorkspaceSessionSummary`

拒绝。

原因：所有 session 都归属 workspace，`WorkspaceSessionSummary` 与 `SessionSummary` 只差 `workspace_id` 会制造 DTO 分裂。列表上下文也保留 `workspace_id`，统一使用 `SessionSummary`。

## User review notes

- 用户要求进入严格模式 / strict 的 Spec / 规格阶段。
- 用户已明确 Requirement / 需求阶段的关键决策，当前规格将这些决策转化为包结构、proto schema、API ownership、migration 和配置设计。
- 用户确认：原 `application`、`infrastructure` 也要由 agent/cloud 各自认领，真正共用部分不多；`internal/shared/validation`、`internal/shared/auth`、`internal/shared/errors` 只是示例，具体 shared 内容按真实中立需求决定。
- 用户确认：第一阶段交付 Go generated proto model；前端 proto model 留作后续阶段评估。
- 用户质疑 `SessionSummary` 与 `WorkspaceSessionSummary` 的差异；本规格已消除 `WorkspaceSessionSummary`，统一 runtime session 模型为携带 `workspace_id` 的 `SessionSummary`。
- 用户希望移除 runtime command envelope；本规格已移除 `RuntimeReq` / `RuntimeResp`，改为 `TunnelFrame.request_id + oneof concrete runtime Req/Resp` 的 typed tunnel payload 方向，并在 Plan 阶段设计合理可演进方式。
- 用户确认：Go-only proto 工具链采用 `buf + protoc-gen-go`。
- 用户确认：error response proto 化接受把 `requestId` 迁移为 `request_id`。
- 用户确认：`termbridge` 使用 `termbridge agent` 和 `termbridge cloud` 作为 agent/cloud 业务入口。
