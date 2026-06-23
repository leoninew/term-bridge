# 统一 Gate / Device / Auth 工作台规格
最后修改时间: 2026-06-23 13:26:28

Review status: Accepted

## Requirement basis

依据：`docs/requirement/20260623-unified-gate-device-auth.md`

当前 Requirement 已接受，流程模式为严格模式 / strict。本规格只定义统一 Gate / Device / Auth 工作台的技术方案边界，不进入实施计划或产品代码修改。

已确认的关键需求：

1. 长期移除 `/gateway` 作为独立产品路由，`/sessions` 成为统一会话工作台。
2. 本地使用不是绕过 Gate，而是 Agent 连接本机 Gate；云端接入只是 Agent 的 Gate URL 指向远程 Gate。
3. 页面打开后必须先登录，再选择设备，然后进入 `/sessions`。
4. `/sessions` 主体布局、tab、workspace/session tree、terminal 操作方式保持基本不变。
5. `/sessions` 左面板底部展示当前设备名。
6. 后端引入 Device 概念，Device 成为 workspace / session 持久化数据的一部分。
7. runtime state 目录结构按 Device 重新设计，本次不要求向后兼容旧 state 目录结构或旧 workspace/session 数据。
8. 启动时检查 `.termbridge.yaml` 中的本地设备名、用户账号和 Gate 连接凭据；缺失时自动生成并写入。
9. 自动生成账号使用当前操作系统用户名，密码随机生成并满足常规强度要求。
10. 首次生成登录凭据后，在控制台打印提示信息。
11. Agent device credential 和 Browser user credential 暂时使用同一套凭据，后续权限系统再细分。
12. Gate path 必须补齐当前 localapi 已具备的 mutation 能力。
13. Device 作用域 API 需要在路径式与 header 式之间做方案评估。
14. 设备离线时只读历史模式若实现简单则列入计划，否则作为后续能力。
15. 需要系统性设计 serve 启动、agent 注册、登录、设备列表、设备选择、工作台加载和 terminal attach 的加载/错误/空状态。

相关设计整理：`docs/design/20260623-unified-gate-device-model.md`

## Overview

本变更将 TermBridge 的 Web 工作台收敛到一条统一访问链路：

```text
Browser -> Gate -> Device Agent -> Runtime -> Workspace -> Session -> Terminal
```

本地单机和云端接入不再是两套产品模式：

```text
本地使用：agent.server_url = self Gate
云端使用：agent.server_url = cloud Gate
```

两者差异只体现在部署位置和配置的 Gate URL，不体现在前端页面、资源模型或用户操作流程上。

当前代码已经具备该模型的雏形：`termbridge serve` 同时启动 HTTP server、localapi、gatewayapi 和 agent client。现有差异主要是：前端仍以 local backend / gateway backend 分支区分路径，localapi 具备完整 mutation，gateway path 只覆盖 list/history/attach 等读或连接能力。

本规格建议采用分层收敛：

1. 产品路由收敛：移除 `/gateway` 页面概念，统一到 `/sessions`。
2. API 模型收敛：新增或暴露 device-scoped Gate API，前端围绕 selected device 加载数据。
3. 能力收敛：补齐 Gate -> Agent tunnel 的 mutation 能力。
4. 认证收敛：Browser 和 Agent 都使用 `.termbridge.yaml` 中生成/读取的同一套凭据。
5. 状态模型收敛：workspace/session 持久化数据进入 Device 作用域。
6. 兼容收口：localapi 暂时保留，但不作为前端长期主路径。

## Design decisions

### 1. `/sessions` is the only workbench route

前端长期页面结构：

```text
/
  认证入口或登录后跳转

/sessions
  统一会话工作台
  selected device -> workspace -> session -> terminal

/settings
  设置页，后续承载用户设置、设备配置、Gate 配置

/help
  帮助页
```

`/gateway` 不再作为长期产品路由暴露。Gateway 是内部控制面能力，不是用户需要切换的页面模式。

实施含义：

- router 中删除 `/gateway` route，或在过渡期只保留 redirect，不再承载独立组件语义。
- `SessionsView.vue` 不再接收 `gatewayRoute` 这种产品模式 prop。
- 当前 `activeBackend` 的 local/gateway 二分应逐步替换为 selected-device-driven client。
- 原 gateway 登录/设备选择能力不消失，而是前置为进入 `/sessions` 前的统一流程。

### 2. Login and device selection are required before the workbench

页面打开后的统一前端状态流：

```text
boot
  -> checking-auth
  -> unauthenticated
  -> logging-in
  -> loading-devices
  -> no-device | selecting-device
  -> loading-workbench
  -> workbench-ready
```

关键行为：

- 未登录时不能直接展示工作台主体。
- 登录成功后必须加载设备列表。
- 没有设备时显示明确空状态，而不是进入空白工作台。
- 有设备但未选择时显示设备选择状态。
- 选择设备后进入 `/sessions` 并加载 workspace/session tree。
- 左面板底部展示当前设备名。
- terminal attach 失败时只影响当前 session pane，不应使整个工作台崩溃。

推荐前端状态划分：

```ts
type AuthState = 'checking' | 'anonymous' | 'authenticated'
type DeviceState = 'idle' | 'loading' | 'empty' | 'ready' | 'offline' | 'error'
type WorkbenchState = 'idle' | 'loading' | 'ready' | 'empty' | 'error'
type TerminalState = 'idle' | 'connecting' | 'attached' | 'detached' | 'failed'
```

这些类型不一定按原样实现，但需要在代码中体现同等的状态边界，避免依赖零散布尔值组合出不可解释状态。

### 3. Device-scoped API should use path scope, not header scope

Requirement 中列出的两个候选方案：

1. 路径式：`/api/devices/:deviceId/sessions`
2. Header 式：`/api/sessions` + header 指定 device，例如 `X-TermBridge-Device-ID: <deviceId>`

评估结论：推荐路径式 API。

#### Path-scoped API

建议形态：

```text
GET    /api/me
POST   /api/login
POST   /api/logout
GET    /api/devices
GET    /api/devices/:deviceId
GET    /api/devices/:deviceId/workspaces/tree
PATCH  /api/devices/:deviceId/workspaces/order
DELETE /api/devices/:deviceId/workspaces/:workspaceId
GET    /api/devices/:deviceId/sessions
POST   /api/devices/:deviceId/sessions
GET    /api/devices/:deviceId/sessions/:sessionId
PATCH  /api/devices/:deviceId/sessions/:sessionId
POST   /api/devices/:deviceId/sessions/:sessionId/close
DELETE /api/devices/:deviceId/sessions/:sessionId
GET    /api/devices/:deviceId/sessions/:sessionId/history
GET    /api/devices/:deviceId/sessions/:sessionId/ws
```

优点：

- URL 本身表达资源归属，日志、浏览器 devtools、代理日志和错误报告更清晰。
- WebSocket attach 可以直接通过 URL 携带 device scope，不依赖浏览器 WebSocket header 限制。
- 权限和 route matching 更直接，middleware 可以从 path 中得到 device id。
- 便于未来链接到特定 device/session 页面或诊断某个 device 请求。
- 与当前 gateway path `/api/gateway/devices/:deviceId/...` 的思路接近，迁移成本低。
- 缓存和观察性更自然，不需要每个工具都理解自定义 header。

缺点：

- device id 会出现在 URL 和日志中，需要确认 device id 不作为 secret 使用。
- URL 较长。
- 若未来存在“当前选中 device”语义，前端仍需要自己保存 selected device。

#### Header-scoped API

候选形态：

```text
GET    /api/sessions
POST   /api/sessions
GET    /api/sessions/:sessionId
PATCH  /api/sessions/:sessionId
DELETE /api/sessions/:sessionId
GET    /api/workspaces/tree
```

请求 header：

```text
X-TermBridge-Device-ID: <deviceId>
```

优点：

- URL 简短，资源路径看起来更像“当前上下文中的 sessions”。
- local direct path 的前端调用点迁移可能更少。
- 如果未来产品语义强依赖“当前选中 device”，header 可以看作上下文传递。

缺点：

- 浏览器原生 WebSocket 构造函数不能设置任意 header，terminal attach 仍需要 query/cookie/path/subprotocol 等额外机制，导致 HTTP API 和 WS API 不一致。
- 日志和错误报告中不一定显式看到 device id，排障成本高。
- 中间件、反向代理、缓存、录制工具需要额外保留和显示自定义 header。
- REST 资源边界不清晰：同一个 `/api/sessions/:id` 在不同 header 下代表不同设备资源。
- 前端遗漏 header 时可能访问错误上下文，问题更隐蔽。

#### Decision

采用路径式 device scope：

```text
/api/devices/:deviceId/...
```

Header 可以作为后续辅助能力，例如请求跟踪、capability hints 或内部调试，但不作为 Device 作用域的主表达。

为降低迁移成本，后端可以短期保留现有 `/api/gateway/devices/:deviceId/...`，同时引入新路径作为同一 handler 的正式入口：

```text
/api/gateway/devices/:deviceId/...  # transitional compatibility
/api/devices/:deviceId/...          # canonical product API
```

前端新代码只调用 canonical path。

### 4. Gate mutation coverage must match current localapi

当前 localapi 已具备的能力需要映射到 Gate -> Agent tunnel，否则 `/sessions` 全走 Gate 后会退化。

需要补齐的最小 mutation/API：

| 能力 | Canonical API | Tunnel method | 说明 |
|---|---|---|---|
| workspace tree | `GET /api/devices/:deviceId/workspaces/tree` | `workspace_tree` | 已有能力，迁移路径命名 |
| workspace reorder | `PATCH /api/devices/:deviceId/workspaces/order` | `workspace_reorder` | 对齐 localapi workspace order |
| delete workspace | `DELETE /api/devices/:deviceId/workspaces/:workspaceId` | `workspace_delete` | 需处理有 active session 时的拒绝语义 |
| list sessions | `GET /api/devices/:deviceId/sessions` | `sessions` | 已有能力 |
| create session | `POST /api/devices/:deviceId/sessions` | `session_create` | 请求体沿用 localapi create session DTO |
| get session | `GET /api/devices/:deviceId/sessions/:sessionId` | `session_get` | 供刷新/详情使用 |
| update session | `PATCH /api/devices/:deviceId/sessions/:sessionId` | `session_update` | rename / metadata update |
| close session | `POST /api/devices/:deviceId/sessions/:sessionId/close` | `session_close` | stop running session，返回 close summary |
| delete session | `DELETE /api/devices/:deviceId/sessions/:sessionId` | `session_delete` | 对齐 localapi delete |
| read history | `GET /api/devices/:deviceId/sessions/:sessionId/history` | `history` | 已有能力 |
| terminal attach | `GET /api/devices/:deviceId/sessions/:sessionId/ws` | `terminal_attach` | 已有能力，纳入 canonical path |

Tunnel request/response 继续使用 JSON payload，但应统一错误结构：

```json
{
  "error": {
    "code": "session_not_found",
    "message": "session not found"
  }
}
```

Gate handler 不应直接拼接 runtime 错误文本作为 HTTP 响应语义，应将 agent/runtime 错误映射为稳定 HTTP status 和 error code。

### 5. Agent RuntimeAccess expands from read/attach to full control surface

当前 Agent runtime access 只有 workspace tree、session list、history read、terminal attach。为支撑统一 `/sessions`，需要扩展为覆盖 localapi 的工作台能力。

建议边界：

```go
type RuntimeAccess interface {
    WorkspaceTree(ctx context.Context) ([]terminalapp.WorkspaceTreeNode, error)
    UpdateWorkspaceOrder(ctx context.Context, workspaceIDs []string) error
    DeleteWorkspace(ctx context.Context, workspaceID string) error

    ListSessions(ctx context.Context) ([]terminalapp.SessionSummary, error)
    CreateSession(ctx context.Context, request terminalapp.CreateSessionRequest) (terminalapp.SessionSummary, error)
    GetSession(ctx context.Context, sessionID string) (terminalapp.SessionSummary, error)
    UpdateSession(ctx context.Context, sessionID string, request terminalapp.UpdateSessionRequest) (terminalapp.SessionSummary, error)
    CloseSession(ctx context.Context, sessionID string) (terminalapp.CloseSessionResult, error)
    DeleteSession(ctx context.Context, sessionID string) error

    ReadHistory(ctx context.Context, sessionID string) ([]byte, error)
    Attach(ctx context.Context, sessionID string) (TerminalStream, error)
}
```

实际类型名应以当前 `internal/application/terminalapp` 和 localapi DTO 为准，不要求完全按上述命名新建重复结构。原则是：Gate path 不定义另一套业务语义，而是复用 localapi/runtime 已经存在的 application 层命令。

### 6. Device identity and persistence become first-class

当前 device identity 存在于 state dir 的 `device.json`，字段包括 id、name、created_at。新模型需要把 Device 作用域提升到 workspace/session 持久化布局。

设备生成规则：

```text
device_id: stable random id, generated once
hostname: OS hostname
name: <hostname>
```

如果用户在配置中显式设置设备名，则可覆盖 display name；自动生成规则默认使用 hostname。由于 device id 已作为独立字段存在，设备显示名不再追加 id 前缀或后缀。

推荐 runtime state 布局：

```text
<state_dir>/
  devices/
    <device_id>/
      device.json
      workspaces/
        <workspace_key>/
          workspace.json
          order.json or metadata.json
          sessions/
            <session_id>/
              session.json
              history.cast or history.log
              runtime.json
```

设计要点：

- Device 是 workspace/session 的持久化父级。
- 同一 Gate 可管理多个 Device，状态目录不会混杂。
- 本次不要求兼容旧目录结构，旧数据可以不迁移。
- `device.json` 中保存稳定 id、display name、created_at、updated_at，必要时保存 hostname。
- session/workspace DTO 中可以显式带上 device_id，便于日志、调试和未来同步。
- workspace key 的生成规则应继续避免路径穿越和跨平台非法字符。

本次不要求完整设备管理 UI，也不要求支持在 UI 中修改设备名；但后端模型应允许后续 rename。

### 7. Bootstrap writes missing credentials to `.termbridge.yaml` and viper reads them

启动时需要在配置加载阶段处理本地凭据缺失问题。

建议配置结构：

```yaml
agent:
  server_url: http://127.0.0.1:9010
  device_id: "..."
  device_name: "hostname-ab12cd34"

user:
  username: "<os username>"
  password: "<generated random password>"
```

也可以将凭据放在更明确的命名下：

```yaml
auth:
  username: "<os username>"
  password: "<generated random password>"
```

推荐采用 `auth.username` / `auth.password`，避免把 Browser user credential 混入 `agent` 命名空间。Agent 暂时复用同一套凭据连接 Gate，但这是策略选择，不意味着配置字段归属于 agent。

配置处理顺序：

```text
load defaults
  -> load .termbridge.yaml via viper
  -> inspect missing auth/device fields
  -> generate missing values
  -> write .termbridge.yaml if changed
  -> reload or merge generated config into runtime Config
  -> start HTTP server / Gate
  -> start Agent connector using Config.Agent + Config.Auth
```

生成规则：

- username：当前 OS 用户名；获取失败时退回可解释默认值，例如 `termbridge`，并记录 warning。
- password：使用 cryptographic random，建议至少 24 字符，包含足够 entropy；不要求人为记忆友好。
- device_id：stable random id，生成一次后持久化。
- device_name：默认使用本机 hostname；不再追加 device id 片段。

控制台首次提示：

```text
TermBridge generated local credentials in .termbridge.yaml.
Username: <username>
Password: <password>
Open: http://127.0.0.1:<port>
Keep this file private.
```

安全边界：

- 本阶段允许明文写入本地 `.termbridge.yaml`，但必须在文档和日志中明确它是本地 secret 文件。
- 不在普通启动日志重复打印已存在密码；只在首次生成时打印。
- 后续可加入文件权限检查、password hashing、token 分离、pairing flow。

### 8. Browser auth and Agent auth share credentials for this phase

当前 gateway auth 使用 hardcoded `admin/admin`。本阶段需要替换为配置驱动凭据。

Browser 登录：

```text
POST /api/login
body: { username, password }
-> HttpOnly session cookie
```

兼容期也可以继续暴露：

```text
POST /api/gateway/login
POST /api/gateway/logout
GET  /api/gateway/me
```

但前端新代码应调用 canonical API：

```text
POST /api/login
POST /api/logout
GET  /api/me
```

Agent 连接 Gate：

- 移除 hardcoded `basicAuthHeader("admin", "admin")`。
- 使用 `.termbridge.yaml` / viper 读取到的 `auth.username` + `auth.password`。
- Gate 的 agent tunnel endpoint 验证同一套凭据。
- Agent hello payload 继续包含 device_id、device_name、protocol_version；可增加 hostname 或 capability，但不要把 password 放进 hello JSON 中。

认证失败行为：

- Browser：返回 401 和稳定 error code，前端展示登录失败。
- Agent：tunnel dial 或 hello_ack 返回明确错误，日志中说明认证失败但不打印 password。
- Device list：未登录返回 401，不返回设备数据。
- Terminal WS：未登录或无 device 权限时拒绝握手。

### 9. Gate registry remains routing state, not runtime owner

Gate 维护 Device registry：

```text
DeviceSummary {
  id
  name
  online
  connected_at
  last_seen
  capabilities
}
```

Gate 可以持有 shallow snapshot：

- 最近一次 workspace tree
- 最近一次 session list
- device online/offline 状态
- tunnel route metadata

但 runtime authoritative state 仍在 Device Agent 所在机器上：

- PTY/process lifecycle
- session current state
- history writes
- workspace/session mutations

这条边界必须保留，避免 Gate 在云端误成为 runtime owner。

### 10. Offline readonly history is deferred unless shallow cache is already enough

Requirement 指出：设备离线时只读历史模式若易于实现则列入计划。

本规格判断：本阶段不把 offline readonly history 作为必交付能力，只保留一个低成本可选增强。

原因：

- 当前 history authoritative data 在 Agent/local runtime/state dir。
- Gate 当前只有 shallow registry/routing state，不保证持久保存完整 history。
- 若为了 offline history 引入 Gate 持久缓存，会扩大数据一致性、安全和隐私范围。

可选低成本方案：

- 如果 Gateway 已经在用户在线时读取过某个 session history，并且内存中有该 history 的显示缓存，可在 device offline 时展示“last viewed history snapshot”。
- UI 必须明确标注 snapshot/stale，不能暗示数据完整。
- 不要求刷新、不要求跨 Gate 重启持久化、不要求覆盖所有 session。

计划阶段应默认将 offline readonly history 放入后续能力，除非实现时发现现有缓存即可无额外风险支持。

## Affected components

### Frontend

- `web/src/router/index.ts`
  - 删除或重定向 `/gateway`。
  - `/` 保留为认证入口或登录后跳转到 `/sessions`。

- `web/src/App.vue`
  - 仍可作为 thin router shell。
  - 后续如果引入 auth shell，应保持全局状态边界清晰，不把 workbench 状态搬回 App。

- `web/src/views/SessionsView.vue`
  - 移除 `gatewayRoute` prop 和 local/gateway 产品分支。
  - 引入 selected device 后的数据加载。
  - 保留当前 layout、tabs、dialogs、terminal/history 组件用法。

- `web/src/features/gateway/api.ts`
  - 迁移或重命名为 device/gate API client。
  - 新增 canonical `/api/devices/:deviceId/...` mutation 方法。

- `web/src/features/sessions/api.ts`
  - 当前 local direct API client 可保留为 compat/internal。
  - 不再作为 `/sessions` 的长期主路径。

- `web/src/features/workspaces/api.ts`
  - 同上，workspace mutations 应迁移到 device-scoped client。

- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
  - 左面板底部显示当前设备名。
  - 设置/语言菜单不应被设备状态逻辑污染。

- Pinia store candidates
  - `auth`：me/login/logout/checking state。
  - `devices`：device list、selected device、online/offline/error。
  - `workbench`：workspace tree、selected session、loading/error。
  - 本阶段不要求一次性把所有 terminal 局部状态迁入 store。

### Backend app/config

- `internal/app/app.go`
  - `runServe` 启动顺序需要接收 bootstrap 后的 config。
  - localapi/gatewayapi/agent client 都应使用同一 Auth/Device config。

- `internal/infrastructure/config/config.go`
  - 增加 AuthConfig 或等价结构。
  - 增加 device_id/device_name 配置读取。
  - 增加 allowed keys。
  - viper 加载后支持写回缺失字段。

- config bootstrap helper
  - 建议新增独立 bootstrap/ensure 函数，避免把文件写回逻辑塞进普通 config normalization。
  - 该函数负责检测缺失、生成、写回、返回是否首次生成。

### Backend gateway

- `internal/transport/http/gatewayapi/server.go`
  - 增加 canonical `/api/...` routes 或提取 device-scoped handler 供 `/api/gateway/...` 和 `/api/devices/...` 复用。
  - 补齐 mutation handlers。
  - terminal WS canonical path 鉴权。

- `internal/transport/http/gatewayapi/route.go`
  - request method 增加 mutation 方法。
  - response error mapping 稳定化。

- `internal/transport/http/gatewayapi/auth/auth.go`
  - 移除 hardcoded `admin/admin`。
  - 使用 config-driven credential validator。
  - canonical `/api/login` / `/api/logout` / `/api/me` 可复用同一 auth manager。

### Backend agent/tunnel

- `internal/application/agent/client.go`
  - tunnel dial 使用配置凭据。
  - hello payload 上报 device id/name/protocol/capabilities。
  - request method switch 补齐 mutation。

- `internal/application/agent/runtime_access.go`
  - 扩展 RuntimeAccess 到完整工作台控制面。

- `internal/protocol/tunnel/frame.go`
  - 若 request/response payload 结构兼容，可以不立即 bump protocol version。
  - 若新增 capability negotiation、structured error 或 hello 字段并要求双方强约束，建议 bump protocol version。

### Backend state/repository

- `internal/infrastructure/repository/state/store.go`
  - state root 按 `devices/<device_id>/...` 组织。
  - WorkspaceDir/SessionDir 等方法接收 device id 或持有 device-scoped store。

- runtime registry construction
  - `newWebTerminalRegistry` 需要使用当前 local device 的 device-scoped state store。
  - 本地 self-connected 情况下，Agent 访问的 runtime 与 local state device id 一致。

## Interfaces

### Canonical Browser API

```text
GET    /api/me
POST   /api/login
POST   /api/logout
GET    /api/devices
GET    /api/devices/:deviceId/workspaces/tree
PATCH  /api/devices/:deviceId/workspaces/order
DELETE /api/devices/:deviceId/workspaces/:workspaceId
GET    /api/devices/:deviceId/sessions
POST   /api/devices/:deviceId/sessions
GET    /api/devices/:deviceId/sessions/:sessionId
PATCH  /api/devices/:deviceId/sessions/:sessionId
POST   /api/devices/:deviceId/sessions/:sessionId/close
DELETE /api/devices/:deviceId/sessions/:sessionId
GET    /api/devices/:deviceId/sessions/:sessionId/history
GET    /api/devices/:deviceId/sessions/:sessionId/ws
```

### Device summary DTO

```json
{
  "id": "device_...",
  "name": "host-ab12cd34",
  "online": true,
  "connected_at": "2026-06-23T10:00:00Z",
  "last_seen": "2026-06-23T10:00:10Z",
  "capabilities": ["workspace_tree", "session_create", "terminal_attach"]
}
```

### Auth DTOs

```json
POST /api/login
{
  "username": "wangm25",
  "password": "generated-secret"
}
```

```json
GET /api/me
{
  "authenticated": true,
  "username": "wangm25"
}
```

### Tunnel request method examples

```json
{
  "method": "session_create",
  "params": {
    "workspace_key": "...",
    "command": "...",
    "cwd": "..."
  }
}
```

```json
{
  "method": "workspace_reorder",
  "params": {
    "workspace_ids": ["..."]
  }
}
```

### Structured error shape

```json
{
  "error": {
    "code": "device_offline",
    "message": "device is offline"
  }
}
```

HTTP mapping examples：

| code | status | meaning |
|---|---:|---|
| `unauthorized` | 401 | 未登录或凭据错误 |
| `device_not_found` | 404 | device id 不存在 |
| `device_offline` | 409 或 503 | device 存在但无可用 tunnel |
| `session_not_found` | 404 | session 不存在 |
| `workspace_not_found` | 404 | workspace 不存在 |
| `conflict_active_session` | 409 | workspace/session 正被运行态占用 |
| `validation_error` | 400 | 请求体非法 |
| `agent_timeout` | 504 | Gate 等待 Agent 响应超时 |

## Loading and state design

### Serve bootstrap

```text
start termbridge serve
  -> load config defaults + .termbridge.yaml
  -> ensure auth/device fields
  -> write generated fields if missing
  -> print one-time credential hint if generated
  -> create device-scoped state store
  -> start HTTP server / Gate
  -> start Agent connector
  -> Agent dials configured Gate
  -> Gate authenticates Agent
  -> Agent sends hello(device_id, device_name, capabilities)
  -> Gate marks device online
```

Failure cases:

- Config file cannot be written：fail fast with clear error; do not silently run with ephemeral password.
- Random credential generation fails：fail fast.
- Agent server_url invalid：serve may still start HTTP server, but logs and UI must show device unavailable.
- Agent auth rejected：device remains offline/unavailable; logs show auth failure without secret.

### Browser flow

```text
open /
  -> GET /api/me
  -> unauthenticated: show login
  -> POST /api/login
  -> GET /api/devices
  -> no devices: show waiting/empty state
  -> choose device
  -> navigate /sessions or render /sessions with selected device
  -> load workspace tree + sessions
  -> attach terminal on selected session
```

### Workbench loading states

- `checking auth`：首次打开页面，避免直接闪现工作台。
- `login required`：展示登录表单。
- `loading devices`：登录成功后加载设备列表。
- `no devices`：提示启动本地 `termbridge serve` 或检查 Agent Gate URL。
- `device offline`：设备曾存在但当前离线；禁用 mutation 和 terminal attach。
- `loading workspace`：选中设备后加载 tree/session。
- `workspace empty`：没有 workspace/session，但允许创建 session。
- `terminal connecting`：session pane 内部状态。
- `terminal failed`：只影响当前 terminal pane。

## Risks

1. **Gate mutation 范围大**：create/update/delete/close/reorder 都需要 tunnel、Agent runtime access、HTTP handler 和前端 client 同步扩展。
2. **认证边界容易半成品**：如果 Browser 登录已完成但 localapi 仍未鉴权且被前端/用户长期依赖，会形成绕过路径。
3. **启动时序变化**：本地 self-connected 模式从“HTTP ready 即可用”变成“HTTP ready + Agent registered 才可用”，必须有设备加载状态。
4. **明文凭据风险**：`.termbridge.yaml` 保存 password，必须避免重复打印和误提交；后续需要权限检查和更成熟 secret 管理。
5. **不兼容旧 state**：本次不迁移旧 workspace/session 数据，交付说明必须明确。
6. **API 双路径过渡风险**：`/api/gateway/devices/...` 与 `/api/devices/...` 并存期间要避免行为分叉。
7. **Device id 泄露误解**：路径式 API 会在 URL/log 中暴露 device id，因此 device id 不能被当作 secret。
8. **过度 Pinia 化风险**：若一次性迁移所有 terminal/socket 局部状态，可能引入回归；store 应先承载 auth/device/workbench 边界状态。

## Alternatives

### Alternative A: Keep `/gateway` and `/sessions` separate

拒绝。它保留了两个产品模式，和“本地也连接 Gate”的长期模型冲突。维护成本会持续扩大。

### Alternative B: Header-scoped Device API

拒绝作为主路径。主要原因是 WebSocket header 限制、可观察性差、资源边界不清晰。可作为辅助上下文或内部调试信息。

### Alternative C: Keep localapi as frontend main path for local use

拒绝作为长期方向。它会继续把本地和云端拆成两套业务路径。localapi 可以暂时作为兼容层或内部路径保留。

### Alternative D: Implement offline readonly history now via Gate persistence

暂不采纳。它会引入 Gate-side history persistence、隐私和一致性问题，超出本阶段“统一工作台路径”的核心目标。

### Alternative E: Split Agent credential and Browser credential now

暂不采纳。用户已确认本阶段先使用同一套凭据，后续权限系统再细分。规格中保留字段命名边界，避免以后难以拆分。

## User review notes

本 Spec 需要用户重点审阅：

1. API 形态推荐采用路径式 `/api/devices/:deviceId/...`，不采用 header 作为主路径。
2. Offline readonly history 默认不列为必交付，只保留低成本 snapshot 可能性。
3. `.termbridge.yaml` 推荐使用 `auth.username` / `auth.password` 保存 Browser 与 Agent 暂时共用的凭据。
4. Device state 目录推荐重建为 `devices/<device_id>/...`，且本次不向后兼容旧数据。
5. localapi 保留为过渡兼容层，但前端新主路径应是 canonical device-scoped Gate API。

暂无必须阻塞进入 Plan 的未决事项；上述选择如果被接受，可在 Plan 阶段拆为前端、auth/bootstrap、device state、gateway mutation 和兼容收口几个实施批次。
