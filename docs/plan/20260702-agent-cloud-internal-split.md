# internal agent/cloud 拆分计划

最后修改时间: 2026-07-02 21:16:30

Review status: Accepted

## Requirement / Spec basis

本计划基于已接受的需求和规格文档：

```text
docs/requirement/20260702-agent-cloud-internal-split.md
docs/spec/20260702-agent-cloud-internal-split.md
```

当前流程：严格模式 / strict，计划 / Plan。

已确认的关键决策：

1. 拆分限定在同一个 repo 的 `internal` 下。
2. `internal/agent` 和 `internal/cloud` 是角色边界；跨角色通信通过 proto/tunnel/auth contract 完成。
3. `internal/application` 和 `internal/infrastructure` 的能力由 agent/cloud 各自认领。
4. `internal/shared` 只放真实中立、低层、无角色语义的共享能力；`validation` / `auth` / `errors` 只是示例。
5. local/agent HTTP API 与 cloud HTTP API 各自管理 route、middleware、codec、mapper、error policy。
6. Protocol Buffers 作为 IDL / Schema / Contract；第一阶段定义 messages，Go-only proto 工具链采用 `buf + protoc-gen-go`。
7. Go model 生成到 `internal/gen/proto/...`。
8. Go 侧 protobuf JSON codec 使用 `google.golang.org/protobuf/encoding/protojson`，统一 `UseProtoNames: true`、`DiscardUnknown: false`。
9. runtime session 模型使用携带 `workspace_id` 的 `SessionSummary`。
10. tunnel 使用 typed proto payload 表达 runtime 操作，避免 `method string + raw JSON` 和重复 envelope。
11. API error response proto 化，接受从 `requestId` 迁移为 `request_id`。
12. `protocol/tunnel.Frame` 迁移到 generated `tunnel.v1.TunnelFrame` 语义。
13. migration 本次同步拆成 role-specific migrations。
14. `termbridge agent` 和 `termbridge cloud` 是 agent/cloud 业务入口。

## Implementation strategy

采用“先建立 contract，再按角色纵向迁移，最后删除混合入口”的顺序，避免先搬目录导致 import cycle 或业务语义漂移。

实施期间允许待迁移包作为短期过渡来源；每个阶段都要把 `gatewayapi`、`application`、`infrastructure` 中的角色能力迁入目标角色包。最终状态以 `internal/agent`、`internal/cloud`、`internal/shared`、`internal/gen/proto` 为边界。

优先保证每个阶段后 Go 代码可编译、关键测试可运行。若某阶段发现现实代码与 plan 冲突，先更新 requirement/spec/plan 或向用户报告，不静默改变边界。

## Implementation steps

### Step 1: 建立 proto 工程和 Go-only 生成链路

目标：先建立跨边界 contract，不改变运行时行为。

文件 / 目录：

```text
proto/termbridge/common/v1/common.proto
proto/termbridge/runtime/v1/runtime.proto
proto/termbridge/cloud/v1/cloud.proto
proto/termbridge/tunnel/v1/tunnel.proto
buf.yaml
buf.gen.yaml
go.mod
go.sum
internal/gen/proto/termbridge/*/v1/*.pb.go
```

动作：

1. 新增 `buf.yaml`，启用标准 lint 和 breaking check 基础配置。
2. 新增 `buf.gen.yaml`，只配置 `protoc-gen-go`，输出到 `internal/gen/proto`。
3. 在 proto 文件中固定：
   - `package termbridge.<area>.v1`
   - `option go_package = "termbridge-go/internal/gen/proto/termbridge/<area>/v1;<areav1>"`
4. 在 `common.v1` 中定义：
   - `ErrorResp`
   - `HealthResp`
5. 在 `runtime.v1` 中定义：
   - `Workspace`
   - `WorkspaceTreeNode`
   - `SessionSummary`
   - runtime operation `Req` / `Resp`
   - 不定义 `WorkspaceSessionSummary`
   - 不定义 `RuntimeReq` / `RuntimeResp`
6. 在 `cloud.v1` 中定义 cloud auth / device / cloud-oauth 边界 message。
7. 在 `tunnel.v1` 中定义 `TunnelFrame`、control frame、terminal frame、typed runtime operation payload。
8. 运行 `buf generate` 生成 Go 文件。
9. 引入 Go protobuf runtime 依赖：
   - `google.golang.org/protobuf`

约束：

1. 第一阶段使用 Go-only proto 工具链。
2. 第一阶段 proto 内容为 messages。
3. generated proto struct 用作边界 wire model。

### Step 2: 设计 tunnel typed payload 的合理方式

目标：用 typed runtime payload 替代 `method string + raw JSON`，保持 tunnel frame 只承担请求关联和 payload 承载。

采用方式：`TunnelFrame` 顶层提供请求关联字段，payload oneof 直接承载具体 typed runtime `Req` / `Resp`。

建议结构原则：

```proto
message TunnelFrame {
  string stream_id = 1;
  string request_id = 2;

  oneof payload {
    // control: 10-99
    Hello hello = 10;
    HelloAck hello_ack = 11;
    Ping ping = 12;
    Pong pong = 13;

    // runtime requests: 100-199
    termbridge.runtime.v1.ListWorkspacesReq list_workspaces_req = 100;
    termbridge.runtime.v1.CreateSessionReq create_session_req = 101;

    // runtime responses: 200-299
    termbridge.runtime.v1.ListWorkspacesResp list_workspaces_resp = 200;
    termbridge.runtime.v1.CreateSessionResp create_session_resp = 201;

    // terminal: 300-399
    TerminalAttachReq terminal_attach = 300;
    TerminalInput terminal_input = 301;
    TerminalOutput terminal_output = 302;

    // terminal / error / close: 900+
    termbridge.common.v1.ErrorResp error = 900;
    Close close = 901;
  }
}
```

Plan 阶段的具体取舍：

1. `request_id` 放在 `TunnelFrame` 顶层，用于 runtime request/response 和需要应答的 tunnel 操作。
2. `stream_id` 继续表示 tunnel stream / terminal stream；control frame 可以使用固定 control stream。
3. runtime response 不直接使用实体作为 response payload；补齐明确 `Resp` message，例如：
   - `GetSessionResp { SessionSummary session = 1; }`
   - `UpdateSessionResp { SessionSummary session = 1; }`
   - `CloseSessionResp { SessionSummary session = 1; }`
4. `ErrorResp` 使用同一个 `TunnelFrame.request_id` 与请求对应，不额外包一层 response envelope。
5. 新增 runtime operation 时只追加 oneof 字段和字段号，不复用、不重命名、不删除已发布字段。
6. 使用字段号区间管理演进，降低 `TunnelFrame` 随 runtime API 扩展时的混乱程度。
7. 不使用 `google.protobuf.Any`，避免把类型安全退回运行时解析。
8. 不使用 `operation enum + bytes/json payload`，避免重新发明弱类型 command envelope。

需要调整规格中示例时，按本计划的字段区间和 explicit Resp wrapper 落地。

### Step 3: 建立 protojson codec 与边界错误模型

目标：统一 protobuf JSON 行为，落实 error response `request_id`。

候选文件 / 目录：

```text
internal/shared/codec/protojson.go
internal/shared/errors 或 internal/shared/apierrors
internal/agent/api/codec.go
internal/cloud/api/codec.go
internal/agent/api/errors.go
internal/cloud/api/errors.go
web/src/features/gateway/api.ts
```

动作：

1. 仅在确实被 agent/cloud 双方复用时新增 `internal/shared/codec`，封装：
   - `protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: false}`
   - `protojson.UnmarshalOptions{DiscardUnknown: false}`
2. API handler 层使用 generated `commonv1.ErrorResp` 输出错误。
3. `requestId` 迁移为 `request_id`，前端错误解析同步改造。
4. agent/cloud 可以各自保留 response writer 和 status code policy，不共享完整 HTTP error writer。
5. 如果两侧错误 code 常量完全一致，再抽 `internal/shared/errors`；否则先保留在各自 API 包。

约束：

1. `internal/shared` 不包含 HTTP handler。
2. `internal/shared` 不包含 cloud user auth / OAuth service。
3. shared 子目录按真实调用方需求创建。

### Step 4: 拆 role-specific migrations 与 database 入口

目标：agent migration set 只包含 runtime schema，cloud migration set 只包含 auth/device schema。

目标文件 / 目录：

```text
internal/agent/infrastructure/database
internal/agent/migrations/sqlite/*.sql
internal/agent/migrations/mysql/*.sql
internal/cloud/infrastructure/database
internal/cloud/migrations/sqlite/*.sql
internal/cloud/migrations/mysql/*.sql
internal/infrastructure/database/*                 过渡后删除或降级为中立 DB primitive
internal/infrastructure/database/migrations/*      迁移来源
```

当前 migration 归属：

```text
202606270001_auth_schema.sql        cloud-owned
202606280001_device_bindings.sql    cloud-owned
202606290001_runtime_state.sql      agent-owned runtime + device public_key 混合
```

拆分策略：

1. cloud schema：
   - `users`
   - `user_identities`
   - `auth_codes`
   - `email_delivery_logs`
   - `oauth_states`
   - `devices`
   - `user_devices`
   - `device_binding_codes`
   - `devices.public_key`
2. agent schema：
   - local device identity persistence table，如确需 DB 中保存则使用 `local_devices` 或 role-local `devices`
   - `workspaces`
   - `sessions`
   - `session_runs`
3. agent baseline 只声明 runtime 所需表，不复用包含 cloud/device 依赖的混合 SQL。
4. cloud baseline 只声明 identity/device binding 所需表，不复用包含 runtime workspace/session 的混合 SQL。
5. 新 role-specific baseline migration 使用新的版本号，避免同一 goose version 对应不同 SQL 内容。
6. 对已有 DB 做 schema state detection：
   - 如果目标表已完整存在，跳过 role baseline 或执行只读一致性检查。
   - 如果发现部分 schema 状态，直接报错并提示手工迁移/备份。
7. 新 DB 只执行对应 role migration set。
8. 不在本次删除 DB 中多余表；清理作为后续数据维护任务。

验证矩阵：

1. 新 agent SQLite DB：只出现 agent runtime 所需表。
2. 新 cloud SQLite DB：只出现 cloud auth/device 所需表。
3. 已有统一 SQLite DB 作为 agent 启动：runtime 表完整且迁移幂等。
4. 已有统一 SQLite DB 作为 cloud 启动：auth/device 表完整且迁移幂等。
5. MySQL migration 文件与 SQLite 文件保持角色边界一致。

### Step 5: 拆 tunnel auth 与 device identity

目标：cloud 验签依赖中立 tunnel auth primitive；agent 本地 device identity 保持 agent-owned。

目标文件 / 目录：

```text
internal/application/agent/signature.go              -> internal/shared/auth 或更具体 shared tunnel auth primitive
internal/application/agent/device.go                 -> internal/agent/application/device 或 internal/agent/infrastructure/deviceidentity
internal/infrastructure/repository/device/*          -> cloud-owned repository；如 agent 确需本地 device repo，拆 agent-local repo
internal/cloud/application/device
internal/agent/application/device
```

动作：

1. 把签名 header 名称、canonical string、sign/verify primitive 抽到真实共享包，例如 `internal/shared/auth`。
2. 把 `LoadOrCreateDevice`、`LoadDevicePrivateKey`、`LoadDevicePublicKey` 留在 agent-owned package。
3. cloud tunnel ingress 只依赖：
   - cloud-owned device public key repository
   - shared signing verification primitive
4. 移除 cloud tunnel ingress 的 BasicAuth fallback 生产路径；测试使用显式 fake 或 test-only config。
5. 补充签名验证测试：timestamp、nonce、method/path/audience/device_id、public key mismatch。

### Step 6: 拆 agent role application / infrastructure

目标：agent 拥有本地 runtime、PTY、state repository、tunnel client、agent API。

目标文件 / 目录：

```text
internal/agent/app
internal/agent/api
internal/agent/application/runtime
internal/agent/application/terminal
internal/agent/application/tunnelclient
internal/agent/application/device
internal/agent/infrastructure/config
internal/agent/infrastructure/database
internal/agent/infrastructure/repository/state
internal/agent/infrastructure/pty
```

迁移来源：

```text
internal/application/terminal/*
internal/application/agent/runtime_access.go
internal/application/agent/client.go
internal/application/agent/device.go
internal/infrastructure/repository/state/*
internal/infrastructure/pty/*
internal/infrastructure/history/*
```

动作：

1. 定义 agent role composition root：`internal/agent/app`。
2. 将本地 runtime access/service 迁入 agent-owned application。
3. 将 terminal registry / runtime / DTO mapper 与 generated `runtimev1` 隔离。
4. 将 tunnel client 迁入 agent-owned application，改用 `tunnelv1.TunnelFrame`。
5. 将 PTY manager 与 runtime state repository 迁入 agent-owned infrastructure。
6. 创建 `internal/agent/api`，只处理 local route：
   - `/api/health`
   - `/api/auth/login`、`/api/auth/logout`、`/api/auth/me`，如本地 admin 仍保留
   - `/api/workspaces`
   - `/api/workspaces/...`
   - `/api/sessions`
   - `/api/devices`，仅作为本地 device/cloud connection summary
   - `/api/cloud-oauth/start`
   - `/api/cloud-oauth/callback`
7. agent API request/response 使用 generated Go proto model 做边界 schema，通过 mapper 转换到 agent service model。

### Step 7: 拆 cloud role application / infrastructure

目标：cloud 拥有 auth、device binding、OAuth server、tunnel ingress、device runtime proxy、cloud API。

目标文件 / 目录：

```text
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
```

迁移来源：

```text
internal/application/auth/*
internal/infrastructure/repository/auth/*
internal/infrastructure/repository/device/*
internal/infrastructure/email/*
internal/transport/http/gatewayapi cloud-only handlers
internal/transport/http/gatewayapi tunnel ingress handlers
```

动作：

1. 定义 cloud role composition root：`internal/cloud/app`。
2. 将 user auth / Google OAuth / email code / cloud OAuth authorize/exchange 迁入 cloud-owned application。
3. 将 auth repository、device binding repository、email sender 迁入 cloud-owned infrastructure。
4. 创建 `internal/cloud/application/tunnelserver`，维护 online device tunnel registry 和 request routing。
5. 创建 `internal/cloud/api`，处理 cloud route：
   - `/api/health`
   - `/api/auth/*` cloud user auth
   - `/api/cloud-oauth/authorize`
   - `/api/cloud-oauth/exchange`
   - `/api/devices`
   - `/api/devices/current`
   - `/api/devices/{device_id}`
   - `/api/devices/{device_id}/workspaces`
   - `/api/devices/{device_id}/sessions`
   - `/api/agent/tunnel`
6. cloud device-scoped runtime API 从 path 中获取 `device_id`，只用于 authz / route lookup / tunnel selection，不写入 runtime proto payload。
7. cloud API 使用 generated proto model 做边界 schema，通过 mapper 转换到 cloud service model 或 tunnel request。

### Step 8: 拆 CLI、config 和 composition 入口

目标：移除 `server.mode` 和 `termbridge serve` 分流，提供明确 agent/cloud 入口。

目标文件：

```text
internal/transport/cli/cli.go
internal/transport/cli/cli_test.go
internal/app/app.go
internal/app/app_test.go
internal/infrastructure/config/config.go
internal/infrastructure/config/config_test.go
configs/config.yaml
configs/config.<env>.yaml
```

动作：

1. CLI command set 调整：
   - 新增 `termbridge agent`
   - 新增 `termbridge cloud`
   - 保留 agent-role CLI：`exec` / `workspace` / `session`
   - `migrate agent` / `migrate cloud` 用于显式 migration
2. `termbridge serve` 处理方式：
   - 完全移除时输出明确错误：`termbridge serve has been removed; use termbridge agent or termbridge cloud`
   - 短期 alias 行为指向明确 role 入口
3. `app.Run` 拆成 agent/cloud composition：
   - `agentapp.Run(ctx, cfg, options)`
   - `cloudapp.Run(ctx, cfg, options)`
4. config 结构拆成 role-specific：
   - `agent.server.*`
   - `agent.runtime.*`
   - `agent.cloud.gate_url`
   - `agent.cloud.oauth.*`
   - `agent.database.*`
   - `agent.history.*`
   - `agent.auth.local_admin.*`
   - `cloud.server.*`
   - `cloud.database.*`
   - `cloud.auth.*`
   - `cloud.oauth.*`
   - `cloud.tunnel.*`
   - `cloud.resend.*`
5. 删除 `ModeLocal`、`ModeCloud`、`ServerConfig.Mode`、`IsLocalMode`、`IsCloudMode`、`Mode(cfg)`、`validateModeRequirements` 对 mode 的依赖。
6. `configs/config.yaml` 移除 `server.mode` 示例，更新注释为 agent/cloud role config。
7. 更新配置测试，覆盖：
   - no `server.mode`
   - agent config validation
   - cloud config validation
   - env override path

### Step 9: 替换 gatewayapi 并删除混合协议包

目标：agent/cloud API 使用各自 Handler，tunnel 使用 generated `tunnelv1.TunnelFrame`。

目标文件 / 目录：

```text
internal/transport/http/gatewayapi/*      删除、拆走或只保留迁移期间不可导出的测试辅助
internal/protocol/tunnel/*                删除或替换为 generated tunnel proto codec helper
internal/protocol/terminal/*              保留浏览器 terminal WS 协议，除非实现中确认需要迁移
```

动作：

1. 将 gatewayapi 中 local handlers 迁移到 `internal/agent/api`。
2. 将 gatewayapi 中 cloud/auth/device/tunnel handlers 迁移到 `internal/cloud/api` 和 `internal/cloud/application/tunnelserver`。
3. 将 `runtime_endpoint.go` 的职责拆成：
   - agent local runtime API mapper
   - cloud tunnel runtime proxy
4. 将 tunnel encode/decode 改为 generated `tunnelv1.TunnelFrame` + protojson codec。
5. 删除 `tunnel.RequestReq` / `tunnel.ResponseResp` / `FrameType request/response` 依赖。
6. 保留浏览器 terminal WebSocket 现有 binary PTY payload 语义。
7. 更新全部相关测试。

### Step 10: import 边界检查与清理

目标：防止拆分完成后留下隐性耦合。

动作：

1. 增加或运行 import boundary 检查，按允许依赖集合验证：
   - `internal/agent/...` -> `internal/agent/...`、`internal/shared/...`、`internal/gen/proto/...`
   - `internal/cloud/...` -> `internal/cloud/...`、`internal/shared/...`、`internal/gen/proto/...`
   - `internal/shared/...` -> stdlib、明确中立的第三方库
   - `cmd/termbridge/...` -> `cmd/termbridge/app`、`cmd/termbridge/cli`
2. 删除空目录、死代码和 DTO duplicate。
3. 确认 generated proto model 只在边界 mapper / codec / tunnel 层使用。
4. 整理 docs/config comments，说明新入口是 `termbridge agent` / `termbridge cloud`。

## Expected files to change

### Process docs

```text
docs/requirement/20260702-agent-cloud-internal-split.md
docs/spec/20260702-agent-cloud-internal-split.md
docs/plan/20260702-agent-cloud-internal-split.md
```

### Proto / generated

```text
proto/termbridge/common/v1/common.proto
proto/termbridge/runtime/v1/runtime.proto
proto/termbridge/cloud/v1/cloud.proto
proto/termbridge/tunnel/v1/tunnel.proto
buf.yaml
buf.gen.yaml
internal/gen/proto/termbridge/common/v1/*.pb.go
internal/gen/proto/termbridge/runtime/v1/*.pb.go
internal/gen/proto/termbridge/cloud/v1/*.pb.go
internal/gen/proto/termbridge/tunnel/v1/*.pb.go
go.mod
go.sum
```

### Agent target

```text
internal/agent/app/*
internal/agent/api/*
internal/agent/application/runtime/*
internal/agent/application/terminal/*
internal/agent/application/device/*
internal/agent/application/tunnelclient/*
internal/agent/infrastructure/config/*
internal/agent/infrastructure/database/*
internal/agent/infrastructure/repository/state/*
internal/agent/infrastructure/pty/*
internal/agent/migrations/sqlite/*.sql
internal/agent/migrations/mysql/*.sql
```

### Cloud target

```text
internal/cloud/app/*
internal/cloud/api/*
internal/cloud/application/auth/*
internal/cloud/application/device/*
internal/cloud/application/oauth/*
internal/cloud/application/tunnelserver/*
internal/cloud/infrastructure/config/*
internal/cloud/infrastructure/database/*
internal/cloud/infrastructure/repository/auth/*
internal/cloud/infrastructure/repository/device/*
internal/cloud/infrastructure/email/*
internal/cloud/migrations/sqlite/*.sql
internal/cloud/migrations/mysql/*.sql
```

### Shared target, only when needed

```text
internal/shared/codec/*
internal/shared/auth/*
internal/shared/errors/*
```

不预创建：

```text
internal/shared/validation/*
```

除非实现中出现真实的、跨 agent/cloud 复用的中立 validation primitive。

### Current packages to migrate away from

```text
internal/app/app.go
internal/transport/cli/cli.go
internal/transport/http/gatewayapi/*
internal/application/agent/*
internal/application/auth/*
internal/application/terminal/*
internal/infrastructure/config/*
internal/infrastructure/database/*
internal/infrastructure/repository/auth/*
internal/infrastructure/repository/device/*
internal/infrastructure/repository/state/*
internal/infrastructure/email/*
internal/infrastructure/pty/*
internal/protocol/tunnel/*
```

### Frontend / config / docs

```text
configs/config.yaml
web/src/features/gateway/api.ts
web/src/features/runtimeTarget.ts
web/src/store/*
README 或其他引用 serve/mode 的文档
```

Frontend 只做 error response 和 route target 必要适配；TS proto generation 留作后续阶段评估。

## Verification plan

### Proto verification

1. `buf lint`
2. `buf generate`
3. 检查 generated Go 文件只输出到 `internal/gen/proto/...`
4. 检查没有生成 `web/src/gen/proto/...`
5. 检查 proto message 命名遵守 `Req` / `Resp`
6. 检查没有定义 proto service / HTTP annotation
7. 检查没有 `RuntimeReq` / `RuntimeResp`
8. 检查没有 `WorkspaceSessionSummary`

### Go unit / integration tests

优先运行：

```text
go test ./...
```

如果全量测试耗时或失败，需要至少分组运行并记录结果：

```text
go test ./internal/transport/cli ./internal/agent/... ./internal/cloud/...
go test ./internal/shared/... ./internal/gen/proto/...
go test ./internal/... -run Test.*Tunnel
go test ./internal/... -run Test.*Config
go test ./internal/... -run Test.*Migration
go test ./internal/... -run Test.*Gateway
go test ./internal/... -run Test.*Terminal
```

实际命令以实现后的包路径为准。

### Boundary checks

需要新增或运行脚本/测试检查 import 方向，验证依赖只落在允许集合内：

```text
internal/agent/...  -> internal/agent/...、internal/shared/...、internal/gen/proto/...
internal/cloud/...  -> internal/cloud/...、internal/shared/...、internal/gen/proto/...
internal/shared/... -> stdlib、明确中立的第三方库
cmd/termbridge/...  -> cmd/termbridge/app、cmd/termbridge/cli
```

### Runtime / API behavior checks

1. local agent API path 保持：
   - `GET /api/workspaces`
   - `POST /api/sessions`
   - `GET /api/workspaces/{workspace_id}/sessions`
   - terminal WS path
2. cloud API path 保持 device-scoped：
   - `GET /api/devices/{device_id}/workspaces`
   - `POST /api/devices/{device_id}/sessions`
   - `GET /api/devices/{device_id}/workspaces/{workspace_id}/sessions`
3. cloud path `device_id` 不进入 runtime proto payload。
4. `WorkspaceSessionsResp.items` 使用 `SessionSummary` 且包含 `workspace_id`。
5. error response JSON 使用 `request_id`。
6. tunnel frame JSON 使用 `protojson` snake_case 字段。
7. terminal binary payload 语义不变。

### Migration checks

1. 新 agent SQLite DB 只执行 agent migrations。
2. 新 cloud SQLite DB 只执行 cloud migrations。
3. 已有统一 DB 作为 agent DB 可识别，migration 幂等。
4. 已有统一 DB 作为 cloud DB 可识别，migration 幂等。
5. MySQL migration 文件存在且角色边界与 SQLite 一致。
6. 对部分 schema 状态断言失败并给出明确错误。

### CLI / config checks

1. `termbridge agent` 可解析为 agent business entry。
2. `termbridge cloud` 可解析为 cloud business entry。
3. `termbridge serve` 的迁移提示错误文案明确。
4. `server.mode` 从 config struct、validation 和 config YAML 中移除。
5. `termbridge exec -- <command>`、`workspace`、`session` 的 agent CLI 行为不被误改。

## Blockers / assumptions

1. `buf` CLI 可能未安装；实现或验证时如缺失，需要先向用户说明安装要求或使用项目约定方式安装，不静默跳过 proto verification。
2. 生成 `*.pb.go` 需要本地可用 `protoc-gen-go`；若缺失，需要通过明确命令安装或记录为 blocker。
3. 当前工作区已有未提交改动，实施时必须避免覆盖用户已有 docs/config/code 修改。
4. 本计划不包含前端 TypeScript proto generation；如果后续要引入，需要新需求或新阶段文档。
5. 本计划接受 error response `request_id` breaking change；前端必须同步适配。
6. `tunnelv1.TunnelFrame` 切换要求 agent/cloud 同步升级。
7. role-specific migration 先断言 schema 状态，再执行迁移。

## Risks

1. 拆分范围大，直接移动目录可能造成 import cycle；必须按 contract -> role service -> API/composition -> cleanup 的顺序推进。
2. `gatewayapi` 拆分时容易改变 status code、CORS、auth middleware、error details，需要测试锁定关键行为。
3. role-specific migration 如果版本号或 schema state detection 设计不严谨，可能导致新 DB 缺表或已有 DB 重复建表。
4. `TunnelFrame` oneof 随 runtime API 增长会变长，需要字段号区间和 buf breaking check 控制演进。
5. shared 包边界容易膨胀；只能抽真正中立 primitive，不抽 service、repository、handler。
6. 生成 proto struct 如果直接进入 domain/repository，会形成业务模型污染。
7. `server.mode` 移除会影响 config fixture、文档、用户现有配置和包构建脚本。

## Rollback / mitigation

1. 每个阶段保持可编译，必要时用短期 adapter 保持调用点连续，adapter 在 cleanup step 删除。
2. proto/schema 变更先生成并测试，再迁移 handler。
3. migration 变更先在临时新 DB 和复制的已有 DB 上验证。
4. 若 tunnel proto migration 风险过高，先实现 generated proto 与当前 tunnel frame 的内部测试对照。
5. 若 CLI/config 拆分影响过大，先让 `termbridge agent` / `termbridge cloud` 入口并存于 app 层测试，再删除 `serve` 和 `server.mode`。

## User review notes

- 用户已接受 Spec 并要求进入 Plan。
- 用户确认 Go-only proto 工具链采用 `buf + protoc-gen-go`。
- 用户确认 API error response 接受从 `requestId` 迁移为 `request_id`。
- 用户确认 `termbridge agent` 和 `termbridge cloud` 是 agent/cloud 业务入口。
- 用户确认 `internal/shared` 示例不是固定清单，具体内容按真实中立需求决定。
- 用户要求 TunnelFrame concrete runtime Req/Resp 采用合理方式；本计划采用 frame-level `request_id`、oneof typed payload、字段号区间、explicit Resp wrapper。

## Implementation follow-up: shared boundary tightening

2026-07-03 检查 `internal/shared` 后继续按本 RSP 推进 shared 收口。当前主干拆分已经落地到 `internal/agent`、`internal/cloud`、`internal/shared`、`internal/gen/proto`，并通过 import boundary test 约束 agent/cloud/shared 方向。

本轮继续工作重点：

1. `internal/shared/auth` 只保留 token / bearer / credential 这类中立认证 primitive：`Claims`、`TokenService`、`ExtractBearerToken`、`Credentials`、`Auther`。
2. agent-local auth contract 归属 `internal/agent/api`，包括 local admin user view、capabilities、auth result 和 local credential error。
3. cloud account auth contract 归属 `internal/cloud/auth`，由 `internal/cloud/api` 做薄 alias 以保持 handler/error mapping 调用面稳定。
4. `internal/shared/config` 暂作为 loader / compatibility layer 保留；role-specific projection 已在 `cmd/termbridge/app` 输出到 `internal/agent/app.Config` 和 `internal/cloud/app.Config`。完整 config namespace 收口另行处理。
5. `internal/shared/tunnelwire` 暂保留 wire helper；后续如继续收紧，可把 runtime response JSON mapping 下沉到 role API adapter。

实施约束：

- 不新增 RSP 文件，继续更新本组 accepted RSP 文档。
- 不把 cloud account / email / code / OAuth 语义放回 `internal/shared/auth`。
- 迁移 Go 类型所有权时保持 auth API JSON 字段和错误映射不变。
