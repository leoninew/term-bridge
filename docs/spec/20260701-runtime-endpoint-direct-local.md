# RuntimeEndpoint 与 local direct runtime path 规格

最后修改时间: 2026-07-02 13:13:31

Review status: Accepted

## Requirement basis

本规格基于：

```text
docs/requirement/20260701-runtime-endpoint-direct-local.md
```

该 Requirement 已接受，核心决策为：

```text
1. 不拆仓库。
2. 不拆二进制。
3. 通过不同 internal 模块分清职责并保持代码共用。
4. 模型统一建立在 RuntimeAccess / RuntimeEndpoint contract 上。
5. local 使用 direct runtime path。
6. cloud 保留 tunnel runtime path。
7. local 使用直接 runtime API，cloud 使用 device-scoped API。
```

## Overview

当前 Gateway workspace/session API 的实现以 `agentRoute` 为核心：

```text
/api/devices/{device_id}/...
↓
routeFor(device_id)
↓
agentRoute.request / terminalRelay
↓
tunnel.Frame
↓
agent.Client
↓
RuntimeAccess
↓
terminal.Registry
```

任务 1 的目标是把统一点上移：

```text
统一 RuntimeEndpoint / RuntimeAccess contract
```

调整后的两条路径为：

```text
local:
Browser -> Gateway Handler -> LocalRuntimeEndpoint -> RuntimeAccess -> terminal.Registry

cloud:
Browser -> Cloud Gateway Handler -> TunnelRuntimeEndpoint -> agentRoute.request / terminalRelay -> tunnel.Frame -> Agent Client -> RuntimeAccess -> terminal.Registry
```

同时 API 语义按用户指定分开：

```text
local: direct runtime API
cloud: device-scoped API
```

UI 继续使用统一的 workspace/session/terminal 模型，但 API client 层根据 `capabilities.mode` 或调用上下文选择 local/cloud endpoint。

## Current implementation facts

### Backend facts

当前已存在：

```text
agent.RuntimeAccess
agent.WebTerminalAccess
terminal.Registry
agent.Client.handleRuntimeRequest
gatewayapi.agentRoute
gatewayapi.terminalRelay
gatewayapi.handleWorkspaceRoute
gatewayapi.handleWorkspaceSessionRoute
gatewayapi.handleJSONRelay
gatewayapi.handleHistoryRelay
gatewayapi.handleTerminalWS
```

当前 `agent.RuntimeAccess` 已覆盖目标能力：

```text
ListWorkspaces
WorkspaceTree
UpdateWorkspaceOrder
DeleteWorkspace
ListSessionsByWorkspaceId
UpdateSessionOrder
CreateSession
RerunSession
GetSession
UpdateSession
DeleteSession
CloseSession
ReadHistory
Attach
```

因此 `RuntimeEndpoint` 作为 Gateway 到 RuntimeAccess/tunnel route 的 transport-facing adapter。

### Frontend facts

当前前端 workspace/session API 完全 device-scoped：

```text
web/src/features/workspaces/api.ts
- /api/devices/{device_id}/workspaces
- /api/devices/{device_id}/workspaces/tree
- /api/devices/{device_id}/workspaces/order

web/src/features/sessions/api.ts
- /api/devices/{device_id}/sessions
- /api/devices/{device_id}/workspaces/{workspace_id}/sessions
- /api/devices/{device_id}/workspaces/{workspace_id}/sessions/{session_id}
- /api/devices/{device_id}/workspaces/{workspace_id}/sessions/{session_id}/history
- /api/devices/{device_id}/workspaces/{workspace_id}/sessions/{session_id}/ws
```

任务 1 保持 UI Store 的 workspace/session 形态，并让 API client 层支持 local direct runtime API。

## Design decisions

### D1. RuntimeEndpoint 是 Gateway 侧 transport adapter，不替代 RuntimeAccess

新增抽象建议放在 Gateway 相关内部模块中。

首选位置：

```text
internal/transport/http/gatewayapi/runtime_endpoint.go
```

原因：

```text
1. RuntimeEndpoint 需要处理 HTTP response shape、status code、history text、terminal websocket bridge 等 transport 细节。
2. 它依赖 gatewayapi 当前错误处理、request id、cache、websocket、terminal protocol。
3. 过早放入 internal/application 会把 HTTP/WebSocket 细节上移。
```

后续若 Gateway 应用层进一步拆分，可以再迁移到：

```text
internal/application/gateway
```

但任务 1 不强制创建新 application package。

### D2. RuntimeEndpoint 是薄 transport adapter

`RuntimeEndpoint` 只表达 Gateway 对 runtime target 的调用方式。

建议最小接口形态：

```go
type runtimeEndpoint interface {
    json(ctx context.Context, method string, params any, requestId string) (json.RawMessage, error)
    history(ctx context.Context, workspaceId string, sessionId string, requestId string) (string, bool, error)
    attach(w http.ResponseWriter, r *http.Request, workspaceId string, sessionId string) error
    available() bool
}
```

其中：

```text
json: 覆盖 workspace/session JSON 操作。
history: 覆盖 history text 操作，并可返回 offline 标记。
attach: 覆盖 terminal websocket attach。
available: 表达 endpoint 是否可用。
```

实现时可以根据 Go 代码可读性微调命名，接口保持 transport adapter 粒度。

### D3. LocalRuntimeEndpoint 直接调用 RuntimeAccess

`LocalRuntimeEndpoint` 持有：

```go
agentapp.RuntimeAccess
```

调用行为：

```text
method="workspaces"          -> RuntimeAccess.ListWorkspaces
method="workspace_tree"      -> RuntimeAccess.WorkspaceTree
method="workspace_order"     -> RuntimeAccess.UpdateWorkspaceOrder
method="delete_workspace"    -> RuntimeAccess.DeleteWorkspace
method="workspace_sessions"  -> RuntimeAccess.ListSessionsByWorkspaceId
method="session_order"       -> RuntimeAccess.UpdateSessionOrder
method="create_session"      -> RuntimeAccess.CreateSession
method="rerun_session"       -> RuntimeAccess.RerunSession
method="get_session"         -> RuntimeAccess.GetSession
method="update_session"      -> RuntimeAccess.UpdateSession
method="delete_session"      -> RuntimeAccess.DeleteSession
method="close_session"       -> RuntimeAccess.CloseSession
method="history"             -> RuntimeAccess.ReadHistory
```

为避免重复 `agent.Client.handleRuntimeRequest` 的 switch，可以抽一个共享 helper：

```text
agent.HandleRuntimeRequest(runtime RuntimeAccess, request tunnel.RequestReq)
```

或在 `agent` 包内提取非导出 helper 并提供导出包装。

推荐方向：

```go
func HandleRuntimeRequest(ctx context.Context, runtime RuntimeAccess, request tunnel.RequestReq) (any, error)
```

然后：

```text
agent.Client.handleRuntimeRequest -> 调用共享 helper
LocalRuntimeEndpoint.json -> 构造 tunnel.RequestReq 并调用共享 helper，再 marshal 成 json.RawMessage
```

这样 direct path 和 tunnel path 的 method/params/result contract 共用同一份映射。

### D4. TunnelRuntimeEndpoint 包裹现有 agentRoute

`TunnelRuntimeEndpoint` 持有：

```go
*agentRoute
deviceId
cache access hooks if needed
```

行为保持当前：

```text
json -> agentRoute.request
history -> agentRoute.request("history", ...)
attach -> 当前 terminalRelay/tunnel frame path
```

`agentRoute` 作为 cloud tunnel route，`runtimeEndpoint` 作为 Gateway workspace/session handler 的目标类型。

### D5. local API 使用 direct runtime routes

任务 1 采用 local direct runtime API。

建议新增 local runtime HTTP routes：

```text
GET    /api/workspaces
GET    /api/workspaces/tree
PATCH  /api/workspaces/order
DELETE /api/workspaces/{workspace_id}

POST   /api/sessions

GET    /api/workspaces/{workspace_id}/sessions
POST   /api/workspaces/{workspace_id}/sessions
PATCH  /api/workspaces/{workspace_id}/sessions/order

GET    /api/workspaces/{workspace_id}/sessions/{session_id}
PATCH  /api/workspaces/{workspace_id}/sessions/{session_id}
DELETE /api/workspaces/{workspace_id}/sessions/{session_id}
POST   /api/workspaces/{workspace_id}/sessions/{session_id}/close
POST   /api/workspaces/{workspace_id}/sessions/{session_id}/rerun
GET    /api/workspaces/{workspace_id}/sessions/{session_id}/history
GET    /api/workspaces/{workspace_id}/sessions/{session_id}/ws
```

这些 routes 只在 local mode 下使用 direct endpoint。

cloud mode 继续使用：

```text
/api/devices/{device_id}/...
```

### D6. local direct API 返回真实 runtime 数据

local direct path 直接访问 `RuntimeAccess`。

因此：

```text
local direct API 成功即返回真实 runtime 数据；
local direct API 失败即返回真实错误；
不使用 X-TermBridge-Offline；
不读取 workspaceTreeCache/historyCache。
```

离线 cache 语义留在任务 2 处理，当前仅要求不污染 local direct path。

### D7. 前端 API 层负责 local/cloud endpoint 选择

UI Store 继续维护统一模型：

```text
workspaceTree
workspaces
sessions
selected/opened sessions
terminal tabs
```

但 API client 层需要支持：

```text
local mode -> /api/workspaces... /api/sessions...
cloud mode -> /api/devices/{device_id}...
```

推荐方式：

```ts
function runtimePath(deviceId: string | null, mode: 'local' | 'cloud', path: string): string
```

或定义更明确的 runtime target：

```ts
type RuntimeTarget =
  | { mode: 'local' }
  | { mode: 'cloud'; deviceId: string }
```

Spec 建议采用 `RuntimeTarget`，因为它能避免 local mode 下传入空 deviceId 后继续拼出错误 device-scoped URL。

前端现有函数可以逐步从：

```ts
listWorkspaceTree(deviceId: string)
createSession(deviceId: string, workspaceId, request)
terminalWsUrl(deviceId, workspaceId, sessionId, ...)
```

调整为：

```ts
listWorkspaceTree(target: RuntimeTarget)
createSession(target: RuntimeTarget, workspaceId, request)
terminalWsUrl(target: RuntimeTarget, workspaceId, sessionId, ...)
```

### D8. gatewayapi.Handler 配置需要注入 local RuntimeAccess

当前 `runServe` 已创建：

```go
runtimeAccess = agent.WebTerminalAccess{Registry: newWebTerminalRegistry(...)}
```

但 `gatewayapi.Config` 目前没有传入 `RuntimeAccess`。

需要新增配置字段，例如：

```go
type Config struct {
    LocalRuntime agentapp.RuntimeAccess
    LocalDevice agentapp.Device
    ServerMode string
    ...
}
```

`runServe` 在非 cloud mode 下传入 `runtimeAccess`。

`gatewayapi.New` 根据 `ServerMode` 与 `LocalRuntime` 构造 local direct endpoint。

### D9. cloud connector 使用 cloud gate URL

Cloud OAuth callback 完成后，cloud connector 使用 `cloud.gate_url` 作为真实 Cloud Gate 连接目标。

### D10. 当前阶段配置归属

配置组织在本阶段处理，按使用方归属而不是历史 `agent` / `gate` 名称归属：

```text
server.listen_url
- 当前进程 HTTP/API server 监听地址。
- 本地开发、发布包、Docker/runtime env 都使用 TERMBRIDGE_SERVER__LISTEN_URL。

server.mode
- 当前进程运行模式。
- 本地开发、发布包、Docker/runtime env 都使用 TERMBRIDGE_SERVER__MODE。

server.static_dir
- 当前进程提供的 Web 静态资源目录。
- 本地开发、发布包、Docker/runtime env 都使用 TERMBRIDGE_SERVER__STATIC_DIR。

server.public_url
- 浏览器访问当前 server 前端/setup 的地址。
- 本地开发、发布包、Docker/runtime env 都使用 TERMBRIDGE_SERVER__PUBLIC_URL。
- 不表达 Cloud Gate 目标。

cloud.gate_url
- Cloud Gate 目标地址。
- OAuth 完成后的远程 connector 使用该值。

<runtime.state_dir>/agent.json
- 设备身份 state 文件。
- 不通过 viper key 或 env key 配置 device id/name。
```

Agent Client 的 `ConnectUrl` 仅作为应用装配时传入的内部运行参数；本阶段不暴露对应配置 key。

## Affected components

### Backend

```text
internal/app/app.go
- 将 runtimeAccess 传入 gatewayapi.Config。
- 保持 cloudConnector 行为不变。

internal/transport/http/gatewayapi/server.go
- Handler 增加 local runtime endpoint 字段。
- 新增 local direct runtime routes。
- 将 workspace/session handling 从 *agentRoute 参数迁移到 runtimeEndpoint 参数。
- 保持 /api/devices/{device_id}/... cloud device-scoped routes。

internal/transport/http/gatewayapi/route.go
- agentRoute 保留，作为 TunnelRuntimeEndpoint 内部实现。
- terminalRelay 逻辑可保留或被 TunnelRuntimeEndpoint 包裹。

internal/transport/http/gatewayapi/runtime_endpoint.go
- 新增 runtimeEndpoint / LocalRuntimeEndpoint / TunnelRuntimeEndpoint。

internal/application/agent/client.go
- 提取 handleRuntimeRequest 的共享 helper，供 tunnel client 与 local endpoint 共同使用。

internal/application/agent/runtime_access.go
- 维持 RuntimeAccess / TerminalStream contract。
```

### Frontend

```text
web/src/features/workspaces/api.ts
- 从 deviceId 参数迁移到 RuntimeTarget 或等价 adapter。
- local mode 拼 /api/workspaces...。
- cloud mode 拼 /api/devices/{device_id}/workspaces...。

web/src/features/sessions/api.ts
- 从 deviceId 参数迁移到 RuntimeTarget 或等价 adapter。
- local mode 拼 /api/sessions 与 /api/workspaces/{workspace_id}/sessions...。
- cloud mode 保持 device-scoped URL。
- terminalWsUrl 支持 local/cloud 两种路径。

web/src/store/workspaceSessions.ts
web/src/views/SessionsView.vue
- 调用 API 时传 RuntimeTarget。
- UI store 仍维护统一 workspace/session 模型。

web/src/store/gateway.ts
- 提供或辅助计算当前 RuntimeTarget。
```

### Tests

```text
internal/transport/http/gatewayapi/*_test.go
- local direct runtime routes。
- cloud device-scoped relay routes。
- terminal ws direct/tunnel path。

internal/application/agent/*_test.go
- shared HandleRuntimeRequest mapping。

web/src/features/workspaces/*.test.ts 或现有 store/view tests
- local/cloud URL 拼接。
- RuntimeTarget selection。
```

## Interfaces

### Go: RuntimeEndpoint draft

```go
type runtimeEndpoint interface {
    JSON(ctx context.Context, method string, params any, requestId string) (json.RawMessage, error)
    History(ctx context.Context, workspaceId string, sessionId string, requestId string) (text string, offline bool, err error)
    Attach(w http.ResponseWriter, r *http.Request, workspaceId string, sessionId string) error
    Available() bool
}
```

命名可在实现阶段按 Go 风格调整，保持上述语义。

### Go: LocalRuntimeEndpoint draft

```go
type localRuntimeEndpoint struct {
    runtime agentapp.RuntimeAccess
}
```

行为：

```text
JSON 调用 agent.HandleRuntimeRequest。
History 调用 RuntimeAccess.ReadHistory。
Attach 接受 browser websocket，并桥接 TerminalStream。
Available 返回 runtime != nil。
```

### Go: TunnelRuntimeEndpoint draft

```go
type tunnelRuntimeEndpoint struct {
    route *agentRoute
    deviceId string
}
```

行为：

```text
JSON 调用 route.request。
History 调用 route.request("history", ...)。
Attach 使用当前 terminalRelay + tunnel frame 机制。
Available 返回 route != nil。
```

### TypeScript: RuntimeTarget draft

```ts
export type RuntimeTarget =
  | { mode: 'local' }
  | { mode: 'cloud'; deviceId: string }
```

Path builder：

```ts
function runtimePath(target: RuntimeTarget, path: string): string {
  if (target.mode === 'local') return `/api${path}`
  return `/api/devices/${encodeURIComponent(target.deviceId)}${path}`
}
```

示例：

```text
runtimePath({mode:'local'}, '/workspaces/tree')
=> /api/workspaces/tree

runtimePath({mode:'cloud', deviceId:'dev-1'}, '/workspaces/tree')
=> /api/devices/dev-1/workspaces/tree
```

## Route specification

### Local direct runtime API

```text
GET    /api/workspaces
GET    /api/workspaces/tree
PATCH  /api/workspaces/order
DELETE /api/workspaces/{workspace_id}
POST   /api/sessions
GET    /api/workspaces/{workspace_id}/sessions
POST   /api/workspaces/{workspace_id}/sessions
PATCH  /api/workspaces/{workspace_id}/sessions/order
GET    /api/workspaces/{workspace_id}/sessions/{session_id}
PATCH  /api/workspaces/{workspace_id}/sessions/{session_id}
DELETE /api/workspaces/{workspace_id}/sessions/{session_id}
POST   /api/workspaces/{workspace_id}/sessions/{session_id}/close
POST   /api/workspaces/{workspace_id}/sessions/{session_id}/rerun
GET    /api/workspaces/{workspace_id}/sessions/{session_id}/history
GET    /api/workspaces/{workspace_id}/sessions/{session_id}/ws
```

这些 routes 由 `LocalRuntimeEndpoint` 处理，返回真实 runtime 数据。

### Cloud device-scoped API

保留现有：

```text
/api/devices/{device_id}/workspaces...
/api/devices/{device_id}/sessions...
/api/devices/{device_id}/workspaces/{workspace_id}/sessions...
```

这些 routes：

```text
1. cloud mode 继续使用。
2. 通过 TunnelRuntimeEndpoint。
3. 保留 agentRoute / tunnel frame 机制。
4. 保留现有 offline cache 行为，后续任务 2 再定义边界。
```

## Technical questions

1. `HandleRuntimeRequest` 是否应放在 `agent` 包？
   - 建议放在 `internal/application/agent`，因为它映射 tunnel request method 到 `RuntimeAccess`，且现有 `agent.Client` 已经承担该映射。

2. local route 是否需要 auth middleware？
   - 应沿用当前 local mode 的访问控制策略，不在任务 1 改变 auth 行为。具体哪些 routes 需要 middleware，以现有 session page/API 行为为准。

3. local direct API 与 device-scoped API response shape 是否一致？
   - 必须一致，前端 store 才能保持统一模型。

## Alternatives considered

### Alternative A: local 也保留 device-scoped API，但 backend 解析到 LocalRuntimeEndpoint

优点：

```text
前端改动较小。
UI/API 语义更统一。
```

缺点：

```text
用户已明确选择 local 使用直接 runtime API、cloud 使用 device-scoped API。
local 单机路径仍被 device URL 包装，概念上继续混淆。
```

结论：不作为任务 1 主方案。

### Alternative C: RuntimeEndpoint 复制 RuntimeAccess 所有方法

优点：

```text
类型显式。
调用处少用 method string。
```

缺点：

```text
接口过厚。
与 RuntimeAccess 高度重复。
TunnelRuntimeEndpoint 仍需 method string 到 tunnel request 的映射。
容易形成第二套应用服务接口。
```

结论：不推荐。任务 1 应保持薄 adapter。

## Risks

1. local/cloud API 分叉会带来前端调用层变更，若 target 选择不集中，容易出现漏改。
2. `RuntimeEndpoint.JSON(method string, params any)` 继续保留 method string，类型安全有限；但它可以最大化复用现有 tunnel request contract。
3. local direct terminal websocket bridge 如果与 tunnel bridge 分叉过多，可能导致 resize/detach/ping 行为不一致。
4. cloud device-scoped API 的 offline cache 仍有语义风险，但按任务拆分留到任务 2。

## User review notes

- 用户要求进入任务 1 Spec 阶段。
- 用户已切换到正确分支，并要求查看变更后开始任务 1 Spec。
- 当前工作树显示已有未跟踪文档变更，未发现代码变更纳入本 Spec 阶段。
- 用户已接受新增 local direct API 路由形态：`/api/workspaces...` 与 `/api/sessions...`。
- 用户已接受 `RuntimeEndpoint` 暂放在 `gatewayapi` 包内，后续再视复杂度迁移到 application/gateway。
- 用户要求解释前端 `RuntimeTarget` 方案。
- 用户已接受 `RuntimeTarget = {mode:'local'} | {mode:'cloud'; deviceId}` 作为前端 API target，并要求进入 Plan 阶段。

## Pending user attention

暂无需要用户确认的 Spec 未决事项。下一阶段进入 Plan / 计划，拆具体实施步骤、文件清单和验证命令。