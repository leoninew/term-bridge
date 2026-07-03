# internal agent/cloud 拆分需求

最后修改时间: 2026-07-02 17:56:28

Review status: Accepted

## Background

当前 TermBridge 同一套 `internal` 代码同时承载本地 agent/runtime/provider 和 cloud portal/gate/relay 两类角色。现有核心链路已经明确：

```text
Browser -> HTTP/WebSocket -> RuntimeEndpoint -> RuntimeAccess -> terminal.Registry -> RuntimeStore / PTY / History
```

local mode 下 Browser 直接访问本机 runtime；cloud mode 下 Browser 通过 cloud 访问某个 device 的 tunnel route，再由远端 agent 调用本机 runtime。

随着本地 agent 和云端 cloud 的职责继续扩展，当前混合包带来以下问题：

1. `internal/app` 同时装配 local runtime、PTY、auth、cloud connector、cloud portal 相关依赖。
2. `internal/transport/http/gatewayapi` 同时管理 local API、cloud API、cloud OAuth、device binding、agent tunnel 和 local runtime endpoint。
3. `internal/application/agent` 同时包含本地 device identity、tunnel client、RuntimeAccess、runtime request dispatch 等不同边界职责。
4. local/agent HTTP API 和 cloud HTTP API 从 route shape 开始就不同，不适合继续由同一个 API 包通过 mode 分支管理。
5. 需要引入 Protocol Buffers 作为 IDL / Schema / Contract 来源，但不把它限定为 gRPC 协议；REST / WebSocket 仍可使用 JSON 编码。
6. 现有协议和 API DTO 已有 `Req` / `Resp` 命名规约，新增 proto message 和生成模型需要继续遵守该规约。

本需求的交付边界是在同一个 repo 的 `internal` 下拆分角色边界。

## Goal

1. 在 `internal` 下清晰拆分 agent/local 和 cloud 两类角色代码。
2. `internal/agent` 管理本地 server、PTY CLI、本地 runtime、cloud connector 和 agent 侧 HTTP API。
3. `internal/cloud` 管理云端 user auth、device binding、cloud OAuth server、tunnel ingress、device runtime proxy 和 cloud 侧 HTTP API。
4. local/agent HTTP API 和 cloud HTTP API 各自拥有 route、middleware、codec、mapper 和 error policy。
5. 引入 `.proto` 作为跨 agent/cloud 的 IDL / Schema / Contract 来源，生成共享 wire model。
6. Go generated proto model 固定生成到 `internal/gen/proto/...`。
7. 第一阶段交付 Go model；前端 TypeScript model 和前端 proto 工具链留作后续阶段评估。
8. REST / WebSocket 中需要 JSON 编解码 protobuf message 的部分使用 `google.golang.org/protobuf/encoding/protojson`，并保持 snake_case JSON 字段。
9. 第一阶段 proto 定义 messages；service / HTTP annotation 留作后续阶段评估。
10. REST / WebSocket 传输继续可以使用 JSON；proto 主要用于模型定义、兼容性约束和代码生成。
11. tunnel WebSocket 当前 JSON envelope/control 部分迁移为 proto message；当前 binary 数据通道继续保留其二进制语义。
12. runtime proto 保持 device-neutral：`device_id` 是 cloud route context，runtime command 使用 workspace/session 等 runtime 标识。
13. tunnel proto 负责 agent/cloud 之间远端 runtime 操作请求/响应和 terminal frame 的传输，直接承载 typed runtime message。
14. `protocol/tunnel.Frame` 迁移到 generated `tunnel.v1.TunnelFrame` 语义。
15. API error response 同步 proto 化，作为边界 contract 的一部分。
16. 数据库 migration 随本次拆分同步拆为 role-specific migration，agent/cloud 使用各自 migration set。
17. agent/cloud 使用 role-specific config / composition 和 `termbridge agent` / `termbridge cloud` 入口。
18. 拆分后 agent 和 cloud 通过 proto-generated model、协议包和真正中立的底层工具包协作。
19. 新增或迁移的 request / response message 遵守现有 `Req` / `Resp` 命名规约，例如 `CreateSessionReq` / `CreateSessionResp`，不使用 `CreateSessionRequest` / `CreateSessionResponse`。

## Scope boundary

1. 交付边界限定在同一个 repository 的 `internal` 目录内。
2. 本阶段保持现有前端产品功能和 UI 行为。
3. HTTP API 继续以 REST / WebSocket 形态演进；proto 用作 contract model。
4. domain model、repository model 和 application service 内部模型继续使用角色内 Go 类型。
5. agent/cloud 的 HTTP route adapter 各自实现。
6. `protocol/tunnel.Frame` 迁移到 generated `tunnel.v1.TunnelFrame`。
7. 运行角色由 `termbridge agent` / `termbridge cloud` 表达。
8. 需求阶段只产出需求文档。

## User scenarios

### Scenario 1: 本地用户使用 agent/local server

用户在本机运行 TermBridge，本地 server 提供本机 runtime 的 workspace/session/terminal 能力。

期望：

```text
Browser -> internal/agent/api -> local runtime -> PTY/session/history
```

agent/local API 使用本地 route shape，例如：

```text
GET  /api/workspaces
POST /api/sessions
GET  /api/workspaces/{workspace_id}/sessions
WS   /api/workspaces/{workspace_id}/sessions/{session_id}/terminal
```

### Scenario 2: 云端用户通过 cloud 访问某个 device runtime

用户登录 cloud portal 后，选择某个在线 device，通过 cloud tunnel proxy 访问该 device 的 runtime。

期望：

```text
Browser -> internal/cloud/api -> tunnel route(device_id) -> agent tunnel client -> local runtime
```

cloud API 使用 device-scoped route shape，例如：

```text
GET  /api/devices/{device_id}/workspaces
POST /api/devices/{device_id}/sessions
GET  /api/devices/{device_id}/workspaces/{workspace_id}/sessions
WS   /api/devices/{device_id}/workspaces/{workspace_id}/sessions/{session_id}/terminal
```

### Scenario 3: agent 连接 cloud

本地 agent 通过 cloud OAuth / device report 绑定到 cloud 后，使用本地 device identity 签名连接 cloud tunnel ingress。

期望：

```text
agent tunnel client -> signed /api/agent/tunnel -> cloud tunnel server
```

cloud 验签依赖中立的 tunnel auth / device identity contract。

### Scenario 4: 开发者维护协议模型

开发者通过 `.proto` 修改跨 agent/cloud 的 wire schema，生成 Go 模型，并通过 breaking change 检查减少协议漂移。

期望：

```text
proto IDL -> generated model -> agent/api and cloud/api each map to own service/domain model
```

### Scenario 5: 开发者维护 tunnel 协议

开发者更新 tunnel envelope 时，直接修改 `tunnel.v1` proto message，并让 agent tunnel client 与 cloud tunnel server 共同使用 generated model。

期望：

```text
tunnel.v1.TunnelFrame -> agent/tunnelclient and cloud/tunnelserver
```

协议演进目标是 `tunnel.v1.TunnelFrame`。

## Acceptance

1. 代码结构上存在明确的 agent/cloud 角色边界，至少具备以下方向：

   ```text
   internal/agent/...
   internal/cloud/...
   internal/gen/proto/...
   ```

2. `internal/agent` 的允许依赖集合为 agent 内部包、`internal/shared` 和 `internal/gen/proto`。

3. `internal/cloud` 的允许依赖集合为 cloud 内部包、`internal/shared` 和 `internal/gen/proto`。

4. `internal/shared` 保持中立底层能力边界。

5. local/agent HTTP API 和 cloud HTTP API 由各自包管理：

   ```text
   internal/agent/api
   internal/cloud/api
   ```

6. local/agent route 由 `internal/agent/api` 管理，cloud route 由 `internal/cloud/api` 管理。

7. `.proto` 作为 IDL / Schema / Contract 引入，至少覆盖 runtime 操作 request/response、tunnel frame 和 API error response 的核心模型。

8. proto generated struct 只作为 wire model / contract model，不直接成为 domain model、repository model 或 application service 内部模型。

9. Go generated proto model 输出到：

   ```text
   internal/gen/proto/...
   ```

10. 第一阶段交付 Go generated proto model；TypeScript client model 和前端 proto 工具链留作后续阶段评估。

11. 第一阶段 proto 定义 messages；proto service / HTTP annotation 留作后续阶段评估。

12. request / response proto message 遵守 `Req` / `Resp` 命名规约。

13. REST JSON 编解码使用 protobuf JSON 语义时必须有统一规则，例如：

   ```text
   UseProtoNames: true
   DiscardUnknown: false
   ```

14. Go 侧 protobuf JSON 编解码使用：

   ```text
   google.golang.org/protobuf/encoding/protojson
   ```

15. runtime proto 不包含 `device_id`；cloud path 中的 `device_id` 只用于 route lookup / tunnel route selection。

16. agent/cloud 之间的远端 runtime 操作通过 tunnel frame 直接承载 typed runtime `Req` / `Resp` message，避免额外 runtime command envelope。

17. tunnel 中现有 JSON envelope/control 部分迁移为 proto message；binary payload 的传输语义按 terminal/runtime 需要保留。

18. `protocol/tunnel.Frame` 迁移到 generated `tunnel.v1.TunnelFrame`。

19. 数据库 migration 拆为 agent/cloud role-specific migration；agent migration set 覆盖 runtime schema，cloud migration set 覆盖 auth/device schema。

20. agent/cloud 有明确的 role-specific composition 和 `termbridge agent` / `termbridge cloud` 入口。

21. API error response proto 化，并纳入生成模型和边界 codec。

22. 现有外部 API path shape 在未明确声明 breaking change 前保持兼容；如果某处因拆分必须改变，应在 spec / plan 中显式列为 breaking change。

23. 拆分过程中有测试或静态检查覆盖关键边界：

   ```text
   agent/cloud import direction
   runtime proto JSON shape
   tunnel request/response compatibility
   local route behavior
   cloud device-scoped route behavior
   role-specific migration selection
   error response proto JSON shape
   Req/Resp naming convention
   ```

## Open questions

暂无阻塞 Requirement / 需求接受的未决事项。

以下事项进入 Spec / 规格阶段继续技术化，但不阻塞当前需求确认：

1. Go proto 生成的具体命令、插件组合和工程脚本如何组织。
2. role-specific migration 如何兼容已有本地 `.termbridge` DB 和已有 cloud DB。
3. `server.mode` 移除后的 CLI/config 迁移路径和错误提示文案。
4. binary terminal payload 在 proto tunnel envelope 中的最终编码边界。
5. 前端 TypeScript proto model 是否在后续独立需求中引入。

## Decisions

1. 本次拆分限定在同一 repo 的 `internal` 目录内，不拆成两个项目。
2. agent/local HTTP API 与 cloud HTTP API 从 route 开始分离，各自管理 API 包。
3. 两侧共享 proto 生成的 wire model，但不共享 HTTP route adapter / handler implementation。
4. Protocol Buffers 作为 IDL / Schema / Contract 使用，不等同于必须采用 gRPC。
5. proto generated struct 是边界模型，不是业务模型。
6. runtime command schema 保持 device-neutral。
7. `device_id` 属于 cloud routing context，不属于 runtime request payload。
8. agent 和 cloud 之间通过 tunnel proto / runtime proto / device identity / tunnel auth contract 交互，而不是互相 import application package。
9. Go generated proto model 路径采用 `internal/gen/proto/...`。
10. 第一阶段交付 Go generated proto model；TypeScript client model 和前端 proto 工具链留作后续阶段评估。
11. 第一阶段 proto 定义 messages；proto service / HTTP annotation 留作后续阶段评估。
12. Go 侧 protobuf JSON codec 使用 `google.golang.org/protobuf/encoding/protojson`。
13. tunnel 当前 JSON 部分可以切到 proto；binary 数据通道保留二进制语义。
14. `protocol/tunnel.Frame` 迁移到 generated `tunnel.v1.TunnelFrame`。
15. 数据库 migration 本次同步拆为 role-specific migrations。
16. 运行角色由 `termbridge agent` / `termbridge cloud` 表达。
17. API error response 同步 proto 化。
18. 协议和边界模型遵守现有 `Req` / `Resp` 命名规约。
19. 原 `internal/application` 和 `internal/infrastructure` 下的能力需要由 `internal/agent` 或 `internal/cloud` 各自认领；真正共享的部分保持少量并进入 `internal/shared`，例如 `validation`、`auth`、`errors`。
20. runtime session 模型使用一种携带 `workspace_id` 的 `SessionSummary`；所有 Session 都归属 Workspace，列表场景也携带 `workspace_id`。
21. 移除单独的 runtime command envelope 设计；tunnel 中需要远端 runtime 操作时，采用合理的 typed proto 传输方式，避免回到 string method + raw JSON，也避免重复 envelope。
22. Go-only proto 工具链采用 `buf + protoc-gen-go`。
23. API error response 字段接受从现有 `requestId` 迁移为 `request_id`。
24. CLI 使用 `termbridge agent` 和 `termbridge cloud` 作为不同业务入口。
25. `internal/shared` 的具体内容按实施中真实中立需求决定，`validation` / `auth` / `errors` 是示例而非固定清单。

## Risk

1. 拆分 `gatewayapi` 时容易无意改变现有 API path、status code、error shape 或 auth 行为。
2. proto JSON 默认 lowerCamelCase，若未设置 `UseProtoNames` 可能破坏现有 snake_case API。
3. proto3 默认值无法表达 PATCH field presence，Update 类请求需要 `optional` 或 `FieldMask`。
4. `bytes` 在 proto JSON 中会以 base64 表达，terminal payload 的 JSON 兼容性需要单独确认。
5. 直接共享 mapper 或 route adapter 可能重新引入 agent/cloud 耦合。
6. migration 拆分会触及已有 `.termbridge` 本地 DB 和 cloud DB，必须谨慎处理兼容。
7. 若 cloud tunnel ingress 保留 BasicAuth fallback，拆分后可能留下不符合 cloud 生产语义的认证路径。
8. 若只移动文件不先抽中立 contract，容易形成 import cycle 或 cloud -> agent 依赖。
9. `tunnel.v1.TunnelFrame` 切换要求 agent/cloud 同步升级；实施计划需要避免半升级状态。
10. 移除 `server.mode` 会影响现有配置文件、环境变量和测试 fixture，需要明确迁移策略。
11. API error response proto 化可能影响前端 error handling，需要在 spec / plan 中列出兼容策略或同步改造范围。
12. proto message 需要严格遵守 `Req` / `Resp` 规约，保持 DTO 命名风格一致。

## User review notes

- 用户明确修正范围：是在 `internal` 下拆分 agent 和 cloud，不是拆成两个项目。
- 用户提出协议方向：引入 proto，但把 Protocol Buffers 当做 IDL / Schema，而不是只当作 gRPC 协议。
- 用户确认：local/agent HTTP 接口和 cloud HTTP 接口从路由开始就不同，因此 API 包应各自管理，只共用 proto 生成的模型定义。
- 用户确认：Go generated proto model 使用 `internal/gen/proto/...` 路径。
- 用户确认：第一阶段交付 Go generated proto model；TypeScript client model 和前端 proto 工具链留作后续阶段评估。
- 用户确认：第一阶段 proto 定义 messages；proto service / HTTP annotation 留作后续阶段评估。
- 用户确认：tunnel 当前 JSON 部分可以切到 proto；binary 数据通道保留二进制语义。
- 用户确认：`protocol/tunnel.Frame` 迁移到 generated tunnel frame。
- 用户确认：数据库 migration 本次同步拆分。
- 用户确认：运行角色由 `termbridge agent` / `termbridge cloud` 表达。
- 用户确认：API error response 同步 proto 化。
- 用户强调：协议和边界模型需要遵守现有 `Req` / `Resp` 命名规约。
- 用户确认：原 `application`、`infrastructure` 也要由 agent/cloud 各自认领，真正共用部分不多；`internal/shared/validation`、`internal/shared/auth`、`internal/shared/errors` 只是示例，具体 shared 内容按真实中立需求决定。
- 用户确认：Go-only proto 工具链接受 `buf + protoc-gen-go`。
- 用户确认：API error response 接受从现有 `requestId` 迁移为 `request_id`。
- 用户确认：`termbridge` 使用不同子命令作为 agent/cloud 业务入口，直接使用 `termbridge agent` 和 `termbridge cloud`。
- 用户确认：TunnelFrame concrete runtime Req/Resp 扩展方式采纳合理方式，Plan 阶段需设计避免重复 envelope 且能随 runtime API 演进的具体方案。
