# RuntimeEndpoint 与 local direct runtime path 计划

最后修改时间: 2026-07-02 13:13:31

Review status: Accepted

## Requirement / Spec basis

本计划基于：

```text
docs/requirement/20260701-runtime-endpoint-direct-local.md
docs/spec/20260701-runtime-endpoint-direct-local.md
```

前置阶段状态：

```text
Requirement: Accepted
Spec: Accepted
```

已确认的核心方向：

```text
1. local 使用 direct runtime API：/api/workspaces... 与 /api/sessions...
2. cloud 保留 device-scoped API：/api/devices/{device_id}/...
3. RuntimeEndpoint 先放在 gatewayapi 包内。
4. 前端使用 RuntimeTarget = {mode:'local'} | {mode:'cloud'; deviceId}。
5. RuntimeEndpoint 必须沿用现有 tunnel RequestReq/ResponseResp 方法与 DTO 规约，不另造一套 Req/Resp contract。
6. 不拆仓库，不拆二进制。
7. cloud connector 使用 `cloud.gate_url`，真实 cloud tunnel 行为保留。
```

## Implementation steps

### Step 1：提取 RuntimeAccess request 映射

目标：避免 local direct path 和 tunnel path 各维护一份 method switch。

修改：

```text
internal/application/agent/client.go
```

新增导出 helper：

```go
func HandleRuntimeRequest(ctx context.Context, runtime RuntimeAccess, request tunnel.RequestReq) (any, error)
```

行为：

```text
1. 将现有 Client.handleRuntimeRequest 中的 switch 移入该 helper。
2. Client.handleRuntimeRequest 改为调用 helper。
3. 保持所有 method 名称、参数 DTO、响应 DTO 不变。
```

需要覆盖的 method：

```text
workspaces
workspace_tree
workspace_order
delete_workspace
workspace_sessions
session_order
create_session
rerun_session
get_session
update_session
delete_session
close_session
history
```

### Step 2：新增 RuntimeEndpoint adapter

目标：让 Gateway workspace/session handler 不再只认识 `*agentRoute`。

新增文件：

```text
internal/transport/http/gatewayapi/runtime_endpoint.go
```

实现：

```go
type runtimeEndpoint interface {
    JSON(ctx context.Context, method string, params any, requestId string) (json.RawMessage, error)
    History(ctx context.Context, workspaceId string, sessionId string, requestId string) (text string, offline bool, err error)
    Attach(w http.ResponseWriter, r *http.Request, workspaceId string, sessionId string) error
    Available() bool
}
```

实现类型：

```text
localRuntimeEndpoint
- 持有 agentapp.RuntimeAccess
- JSON 调用 agentapp.HandleRuntimeRequest 后 marshal result
- History 调用 RuntimeAccess.ReadHistory
- Attach 直接 bridge RuntimeAccess.Attach 得到的 TerminalStream
- 不返回 offline cache

tunnelRuntimeEndpoint
- 持有 *agentRoute + deviceId
- JSON 调用 route.request
- History 调用 route.request("history", ...)，保留 offline cache 入口由 Handler 控制
- Attach 复用当前 tunnel terminal relay path
```

### Step 3：拆出 terminal websocket bridge helper

目标：避免 local direct terminal websocket 复用 `terminalRelay` 这个 tunnel 语义类型。

修改：

```text
internal/transport/http/gatewayapi/server.go
internal/transport/http/gatewayapi/runtime_endpoint.go
```

新增 local direct bridge helper：

```text
bridgeTerminalStream 或 handleLocalTerminalWS
```

行为要求：

```text
1. websocket.Accept 使用现有 terminal subprotocol 与 origin patterns。
2. 解析 cols/rows query，复用 terminalAttachSizeFromQuery。
3. RuntimeAccess.Attach(workspaceId, sessionId) 获取 TerminalStream。
4. 初始 size 存在时调用 stream.Resize。
5. browser binary -> stream.WriteInput。
6. browser text resize -> stream.Resize。
7. browser text detach -> stream.Detach 并关闭。
8. browser ping -> pong。
9. stream.Outbound binary/text -> browser。
10. stream closed -> browser websocket close。
```

### Step 4：gatewayapi.Config 注入 LocalRuntime

修改：

```text
internal/transport/http/gatewayapi/server.go
internal/app/app.go
```

`gatewayapi.Config` 新增：

```go
LocalRuntime agentapp.RuntimeAccess
```

`runServe` 中：

```text
非 cloud mode 已创建 runtimeAccess。
构造 gatewayapi.Config 时传入 LocalRuntime: runtimeAccess。
cloud mode 下 LocalRuntime 保持 nil。
```

`gatewayapi.New` 中：

```text
当 ServerMode == local 且 LocalRuntime != nil，构造 localRuntimeEndpoint。
```

### Step 5：新增 local direct runtime routes

修改：

```text
internal/transport/http/gatewayapi/server.go
```

在 `Handler()` 注册：

```text
/api/workspaces
/api/workspaces/
/api/sessions
```

新增 handler：

```text
handleLocalWorkspaceRoot
handleLocalWorkspaceRoute
handleLocalWorkspaceSessionRoute
handleLocalSessionRoot
```

或使用现有 workspace/session route handler 的泛化版本。

路由要求：

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

行为：

```text
1. local mode 下可用。
2. endpoint 不存在时返回 service unavailable 或 not found，按现有 error shape。
3. JSON response shape 与 device-scoped API 一致。
4. history response 为 text/plain。
5. 不设置 X-TermBridge-Offline。
```

### Step 6：将 cloud device-scoped handler 改为使用 TunnelRuntimeEndpoint

修改：

```text
internal/transport/http/gatewayapi/server.go
internal/transport/http/gatewayapi/route.go
```

当前函数签名类似：

```go
handleWorkspaceRoute(w, r, route *agentRoute, deviceId string, parts []string)
handleWorkspaceSessionRoute(w, r, route *agentRoute, deviceId string, workspaceId string, parts []string)
handleJSONRelay(... route *agentRoute ...)
handleHistoryRelay(... route *agentRoute ...)
handleTerminalWS(... route *agentRoute ...)
```

计划改为：

```text
handleWorkspaceRoute(... endpoint runtimeEndpoint, deviceId string, parts []string)
handleWorkspaceSessionRoute(... endpoint runtimeEndpoint, deviceId string, workspaceId string, parts []string)
handleJSONRuntime(... endpoint runtimeEndpoint ...)
handleHistoryRuntime(... endpoint runtimeEndpoint ...)
```

cloud device-scoped route 解析：

```text
route := s.routeFor(deviceId)
endpoint := newTunnelRuntimeEndpoint(deviceId, route, ...)
```

保留现有：

```text
route == nil 时 workspace_tree/history 可读 offline cache。
其他操作返回 device_offline。
```

### Step 7：前端引入 RuntimeTarget

修改：

```text
web/src/features/workspaces/api.ts
web/src/features/sessions/api.ts
web/src/store/gateway.ts
web/src/store/workspaceSessions.ts
web/src/views/SessionsView.vue
```

新增类型位置建议：

```text
web/src/features/runtimeTarget.ts
```

或放在现有 feature API 层中，Plan 推荐单独文件，避免 workspaces/sessions 循环引用。

类型：

```ts
export type RuntimeTarget =
  | { mode: 'local' }
  | { mode: 'cloud'; deviceId: string }
```

helper：

```ts
export function runtimePath(target: RuntimeTarget, path: string): string {
  if (target.mode === 'local') return `/api${path}`
  return `/api/devices/${encodeURIComponent(target.deviceId)}${path}`
}
```

将 API 函数参数从 `deviceId: string` 改为 `target: RuntimeTarget`：

```text
listWorkspaces
listWorkspaceTree
updateWorkspaceOrder
deleteWorkspace
createSession
getSession
updateSession
deleteSession
readHistory
closeSession
rerunSession
updateSessionOrder
terminalWsUrl
```

### Step 8：前端 Gateway store 提供 current RuntimeTarget

修改：

```text
web/src/store/gateway.ts
```

新增 computed 或 action：

```ts
const runtimeTarget = computed<RuntimeTarget | null>(() => {
  if (capabilities.value?.mode === 'local') return { mode: 'local' }
  if (selectedDeviceId.value) return { mode: 'cloud', deviceId: selectedDeviceId.value }
  return null
})
```

实际字段名可按项目风格调整。

要求：

```text
1. local mode 使用 {mode:'local'} 拼 API。
2. cloud mode 有 selectedDeviceId 时使用 {mode:'cloud', deviceId}。
3. UI store 仍维护统一 workspace/session 模型。
```

### Step 9：更新调用点

修改：

```text
web/src/store/workspaceSessions.ts
web/src/views/SessionsView.vue
```

将：

```ts
workspaceSessions.refresh(gateway.selectedDeviceId)
createSession(gateway.selectedDeviceId, ...)
terminalWsUrl(gateway.selectedDeviceId, ...)
```

改为：

```ts
const target = gateway.runtimeTarget
if (!target) return / show unavailable
workspaceSessions.refresh(target)
createSession(target, ...)
terminalWsUrl(target, ...)
```

注意修复潜在问题：

```text
workspaceTree 必须始终保持数组，避免出现 u.value.some is not a function 这类状态形态错误。
```

该问题不是任务 1 主目标，但改调用层时应避免引入非数组赋值。

### Step 10：整理 cloud connector 与当前阶段配置

实现要求：

```text
1. `cloudConnector` 使用 `cloud.gate_url`。
2. Cloud OAuth callback 后启动 `cloudConnector`。
3. local workspace/session/terminal 操作走 local direct RuntimeEndpoint。
4. /api/agent/tunnel 保留给真实 Agent 连接 Cloud Gate。
5. 当前进程 HTTP/API server 配置统一迁移到 `server` 节点：`server.listen_url`、`server.mode`、`server.static_dir`、`server.public_url`。
6. 环境变量示例统一使用 `TERMBRIDGE_SERVER__LISTEN_URL`、`TERMBRIDGE_SERVER__MODE`、`TERMBRIDGE_SERVER__STATIC_DIR`、`TERMBRIDGE_SERVER__PUBLIC_URL`。
7. 设备身份相关配置入口从 viper 配置模型、默认配置、环境变量示例和测试配置中清理。
8. 设备身份通过 `<runtime.state_dir>/agent.json` 生成和读取。
9. Agent Client 连接目标不暴露配置 key；远程 connector 由应用装配传入 `cloud.gate_url`。
```

## Files to change

### Backend expected files

```text
internal/application/agent/client.go
internal/transport/http/gatewayapi/server.go
internal/transport/http/gatewayapi/route.go
internal/transport/http/gatewayapi/runtime_endpoint.go
internal/app/app.go
```

### Backend possible test files

```text
internal/application/agent/client_test.go
internal/transport/http/gatewayapi/relay_test.go
internal/transport/http/gatewayapi/terminal_test.go
internal/transport/http/gatewayapi/server_test.go
```

### Frontend expected files

```text
web/src/features/runtimeTarget.ts
web/src/features/workspaces/api.ts
web/src/features/sessions/api.ts
web/src/store/gateway.ts
web/src/store/workspaceSessions.ts
web/src/views/SessionsView.vue
```

### Frontend possible test files

```text
web/src/store/gateway.test.ts
web/src/store/workspaceSessions.test.ts
web/src/features/workspaces/api.test.ts
web/src/features/sessions/api.test.ts
```

如果现有测试结构不包含 feature API tests，可在 Plan 执行时优先扩展 store tests，避免过度新增测试文件。

## Verification plan

### Static checks

根据项目实际工具链选择：

```text
go test ./...
前端类型检查 / test 命令，以 package scripts 或 justfile 为准
```

执行前在 Implementation 阶段读取项目显式入口：

```text
package.json
justfile / Makefile 如存在
CI 配置如需要
```

### Backend behavior tests

至少覆盖：

```text
1. local direct GET /api/workspaces/tree 调用 RuntimeAccess.WorkspaceTree。
2. local direct POST /api/sessions 调用 RuntimeAccess.CreateSession。
3. local direct history 返回真实 RuntimeAccess.ReadHistory 内容。
4. cloud device-scoped workspace_tree 仍通过 agentRoute.request。
5. cloud route nil 时 workspace_tree/history 仍保留现有 offline cache 行为。
6. cloud route nil 时 create/update/delete/terminal ws 返回 device_offline。
7. shared HandleRuntimeRequest 对所有 method 保持原响应 shape。
```

### Terminal websocket tests

至少覆盖：

```text
1. local direct terminal ws attach 调用 RuntimeAccess.Attach。
2. browser binary input 转为 TerminalStream.WriteInput。
3. resize control 转为 TerminalStream.Resize。
4. detach control 调用 TerminalStream.Detach。
5. TerminalStream.Outbound 输出写回 browser websocket。
6. cloud terminal relay 现有测试仍通过。
```

若现有测试难以完整覆盖 websocket 双向流，至少保留当前 e2e/terminal 测试，并新增 local direct attach 的关键路径测试。

### Frontend tests

至少覆盖：

```text
1. RuntimeTarget local 拼出 /api/workspaces/tree。
2. RuntimeTarget cloud 拼出 /api/devices/{device_id}/workspaces/tree。
3. terminalWsUrl local 拼出 /api/workspaces/{workspace_id}/sessions/{session_id}/ws。
4. terminalWsUrl cloud 保持 /api/devices/{device_id}/.../ws。
5. gateway store local mode 生成 {mode:'local'}。
6. cloud mode 有 selectedDeviceId 时生成 {mode:'cloud', deviceId}。
```

### Manual/browser verification

因为任务涉及前端与 terminal websocket，Implementation 后应启动本地 dev/server 并浏览器验证：

```text
1. local mode 打开 Sessions 页面。
2. 创建新 session。
3. attach 后输入命令并看到输出。
4. resize/detach/close/rerun 至少走一遍主路径。
5. 确认 local direct runtime 操作不会出现 device_offline。
```

如当前环境无法浏览器验证，Verification 阶段必须明确说明未能完成 UI 人工验证。

## Rollback plan

如果实现后 local direct path 出现不可接受问题：

```text
1. 回滚前端 RuntimeTarget 调用改动，恢复 deviceId 参数。
2. 回滚新增 /api/workspaces 与 /api/sessions local routes。
3. 保留 agent.HandleRuntimeRequest helper 可单独评估；若未被使用也回滚。
4. 保留 cloud device-scoped route 原行为。
```

direct path 出现问题时，先定位 direct endpoint 或 RuntimeAccess bridge。

## Assumptions

1. 现有 `agent.RuntimeAccess` 覆盖 local direct runtime API 所需全部能力。
2. local direct API response shape 可以与 device-scoped API 保持一致。
3. terminal protocol 对 browser 侧保持 local direct 和 cloud tunnel 一致。
4. local direct API 是本机 workspace/session/terminal 的运行时路径。
5. 前端可以从 capabilities / gateway store 推导当前 runtime target。

## Risks

1. `RuntimeEndpoint` 若过度抽象，会变成第二套应用服务层。
2. 共享 `HandleRuntimeRequest` 仍使用 method string，编译期类型安全有限。
3. 前端 API 从 deviceId 改为 RuntimeTarget 影响范围较广，漏改会导致运行时 URL 错误。
4. local direct terminal websocket 与 cloud terminal relay 分叉后，需要测试防止 ping/resize/detach 行为漂移。
5. 设备列表/selectedDeviceId 相关 UI 可能暴露旧假设。

## Blockers

当前无实现前必须由用户决定的 blocker。

已接受的关键方案：

```text
local direct API 路由形态
RuntimeEndpoint 放 gatewayapi 包
RuntimeTarget 前端 target 方案
```

## User review notes

- 用户已接受 Spec 中所有关键未决点，并要求进入 Plan 阶段。
- Plan 仅覆盖任务 1；local device online 语义、offline cache 边界不在本计划实现范围。

## Pending user attention

用户已接受本 Plan 并要求开始 Implementation / 实现，且要求实现完成后连续进入 Verification / 验证。