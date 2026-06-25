# 统一 Gate / Device 模型设计整理

日期：2026-06-23  
更新：2026-06-25

## 背景

TermBridge 长期目标不是区分“本地模式”和“Gateway 模式”，而是收敛为一套统一模型：

```text
Browser -> Gate -> Agent -> Runtime -> PTY / Process
```

本地使用时，Agent 连接的 Gate 配置成自己：

```text
Browser -> Local Gate -> Local Agent -> Local Runtime
```

接入云端时，Agent 连接的 Gate 配置成远程云端地址：

```text
Browser -> Cloud Gate -> Local Agent -> Local Runtime
```

因此，Gate 不是“远端模式”的专属组件，而是统一控制面。本地单机只是默认存在一个本机 device，且该 device 连接到本机 Gate；云端模式只是 Agent 连接到云端 Gate。

## 当前后端事实

当前 `termbridge serve` 启动后，backend / gateway / agent connector 已经是一体的：

```text
termbridge serve
  ├─ HTTP server
  │   └─ gatewayapi
  │       ├─ Browser API     /api/devices/:deviceId/...
  │       ├─ Auth API        /api/login, /api/logout, /api/me
  │       └─ Agent tunnel    /api/agent/tunnel
  └─ agent client
      └─ connects to configured Gate URL
```

关键事实：

1. `serve` 创建同一个 web terminal registry / runtime。
2. HTTP server 只挂载统一 API handler，不再挂载 localapi direct path。
3. Browser 产品 API 使用 device-scoped 路径：`/api/devices/:deviceId/...`。
4. Agent connector 通过 `/api/agent/tunnel` 连接目标 Gate。
5. Gateway API 通过已注册的 agent route 调用本地 runtime adapter。
6. 如果 `agent.connect_url` 指向当前 serve 自己，本地就形成 self-connected Gate。
7. 如果 `agent.connect_url` 指向云端，当前本地进程就作为本地 Agent 接入云端 Gate。

因此当前后端已经完成从 local direct path 到统一 Gate / Device 模型的收口。历史上的 Browser -> localapi -> registry 直连路径不再作为运行时兼容层存在。

## 当前唯一产品路径

当前 Browser workbench 路径为：

```text
Browser
  -> /api/devices/:deviceId/...
  -> gatewayapi
  -> agent route / tunnel request
  -> agent client
  -> runtime adapter
  -> registry
  -> PTY / Process
```

本地 self-connected 时，gatewayapi 和 agent client 可以在同一个 `serve` 进程内；云端接入时，gatewayapi 在云端，agent client 在用户本地。

当前产品路径特点：

- 有显式 Device 概念。
- 本地和云端共用同一业务流程。
- Browser API 不再暴露 `/api/workspaces/...` 或 `/api/sessions/...` 直连路径。
- Browser API 不再使用 `/api/gateway/devices/...` 作为产品级路由。
- `/api/agent/tunnel` 是 Agent connector 使用的内部传输入口，不是 Browser workbench API。

## 核心判断

长期不应存在两个产品模式：

```text
/sessions = 本地模式
/gateway  = Gateway 模式
```

长期只有一套资源模型：

```text
User
  -> Device
    -> Workspace
      -> Session
        -> Terminal
```

本地使用：

```text
agent.connect_url = self
```

云端使用：

```text
agent.connect_url = cloud
```

业务流程不应该因为 Gate URL 不同而分裂。

## 当前与历史 local direct path 的区别

历史 direct local path 是：

```text
Browser
  -> localapi
  -> registry
  -> PTY / Process
```

当前统一 Gate / Device path 是：

```text
Browser
  -> /api/devices/:deviceId/...
  -> gatewayapi
  -> agent tunnel
  -> runtime adapter
  -> registry
  -> PTY / Process
```

| 维度 | 历史 direct local path | 当前统一 Gate / Device path |
|---|---|---|
| 前端入口 | `/api/workspaces`, `/api/sessions` | `/api/devices/:deviceId/...` |
| 资源模型 | 隐式本机 | 显式 Device |
| 控制面 | localapi 直接调用 registry | gatewayapi 通过 agent tunnel 调用 runtime adapter |
| Terminal | Browser WS 直连 localapi | Browser WS 经 gateway relay / tunnel 到 agent |
| 启动依赖 | HTTP server ready 即可 | HTTP server ready + Agent registered |
| 云端一致性 | 差 | 好 |
| 当前状态 | 已移除 | 当前产品路径 |

## 当前 Gateway mutation 能力

当前 Gate / Agent relay 已覆盖基础 workspace/session mutation，不再是 attach-only：

- list workspaces
- workspace tree
- update workspace order
- delete workspace
- create session
- list workspace sessions
- get session
- update session
- close session
- delete session
- rerun session
- read history
- terminal attach

因此 `/sessions` workbench 可以基于 device-scoped API 承载主要 workspace/session/terminal 操作。

## 仍需处理的成本与风险

### 1. 本地也依赖 Agent 注册成功

统一 Gate path 下，本地也需要：

```text
server ready -> agent connects -> device registered -> workspace/session available
```

需要继续处理：

- `agent.connect_url` 配错。
- agent tunnel 连接失败。
- Gate 已启动但本机 device 尚未出现。
- device offline / reconnect。
- UI 中的 device loading / empty / offline 状态。

### 2. Terminal 链路复杂度

当前 terminal 链路为：

```text
Browser WS -> gatewayapi terminal relay -> tunnel frames -> agent -> registry stream -> PTY
```

本机 loopback 下性能成本预计可接受，但复杂度仍需要持续 hardening：

- frame encode/decode
- stream id 管理
- pending request map
- relay backpressure
- writer conflict
- tunnel close / reconnect
- terminal detach / close 语义映射

### 3. Auth / Device credential 仍需产品化

当前已有 Browser auth 边界，但正式生产级能力仍需继续设计：

- Browser 访问 Gate 的正式用户系统。
- Agent 连接 Gate 的 device credential。
- device pairing flow。
- token rotation。
- WebSocket auth 策略。
- local self-connected Gate 首次运行体验。

### 4. 配置成为核心路径

本地使用和云端接入的差异主要由 Gate 监听地址和 Agent 连接地址决定。`agent.connect_url` 为空时默认回退到 `gate.listen_url`，形成本地 self-connected Gate：

```yaml
gate:
  listen_url: http://127.0.0.1:9010

agent:
  connect_url: ""
```

云端接入时，Agent 显式连接云端 Gate：

```yaml
agent:
  connect_url: https://gate.example.com
```

因此默认配置必须可靠，错误配置需要清晰错误和 UI 状态。

## 收益

统一 Gate / Device 模型带来的收益：

1. 前端不再区分 local backend 与 gateway backend。
2. `/gateway` 不再作为产品页面存在。
3. `/sessions` 成为统一会话工作台。
4. 本地与云端业务流程一致。
5. 接入云端时只改变 Gate URL，不改变前端架构。
6. Device / Workspace / Session 模型为多设备、权限、审计、协作预留空间。
7. 删除 local direct path 后，减少未鉴权兼容路径形成绕过的风险。

## 推荐产品信息架构

长期页面应收敛为：

```text
/
  认证入口或登录后跳转

/sessions
  统一会话工作台
  device -> workspace -> session

/settings
  用户设置 / 设备配置 / Gate 配置

/help
  帮助文档
```

不建议长期保留：

```text
/gateway
```

因为 Gateway 是统一控制面，不是独立业务页面。

## 当前 API 方向

当前 Browser API 是 device-scoped API：

```text
/api/me
/api/devices
/api/devices/:deviceId/workspaces
/api/devices/:deviceId/workspaces/tree
/api/devices/:deviceId/workspaces/order
/api/devices/:deviceId/workspaces/:workspaceId
/api/devices/:deviceId/workspaces/:workspaceId/sessions
/api/devices/:deviceId/workspaces/:workspaceId/sessions/:sessionId
/api/devices/:deviceId/workspaces/:workspaceId/sessions/:sessionId/history
/api/devices/:deviceId/workspaces/:workspaceId/sessions/:sessionId/ws
```

Agent connector 内部传输入口是：

```text
/api/agent/tunnel
```

`gateway` 可以继续作为内部 package / transport 名称，但不应暴露为产品级 Browser 路由概念。

## 后续迁移与收口方向

### Phase 1：前端移除“本地模式”概念

状态：已完成。

- `/sessions` 是统一 session workbench。
- Browser workbench 使用 device-scoped API。
- `/gateway` 不作为产品页面入口。

### Phase 2：补齐 Gateway mutation

状态：基础能力已完成。

已覆盖：

- create session
- get session
- update session
- close session
- delete session
- rerun session
- delete workspace
- workspace reorder
- history
- terminal attach

后续重点是补齐真实场景 verification 与边界行为，而不是恢复 local direct path。

### Phase 3：统一认证与设备凭证

仍需继续设计和实现：

- Browser auth 上提到统一 Gate 层。
- Agent 使用正式 device credential / pairing flow。
- 移除固定 admin/admin 类 MVP 认证假设。
- WebSocket 纳入一致鉴权边界。

### Phase 4：localapi 收口

状态：已完成运行时收口。

- HTTP server 不再挂载 localapi。
- Browser API 不再绕过 Gate / Device 模型。
- localapi 不作为兼容层保留。

## 当前需要避免的误区

1. 不要把 Gateway 理解成“远端模式”。本地也应该连接 Gate。
2. 不要让 `/gateway` 成为长期产品入口。
3. 不要恢复 Browser -> localapi -> registry 的 direct path。
4. 不要把 `/api/agent/tunnel` 当作 Browser workbench API；它只是 Agent connector 内部传输入口。
5. 不要让本地使用强依赖云端；本地 self-connected Gate 应该是 first-class。

## 结论

当前 `termbridge serve` 是一体化入口。Browser 产品 API 已收口到：

```text
/api/devices/:deviceId/...
```

Agent connector 内部传输入口已收口到：

```text
/api/agent/tunnel
```

统一运行路径为：

```text
Browser -> Gate -> Device Agent -> Workspace -> Session -> Terminal
```

本地使用时，Agent 连接自己的 Gate；云端使用时，Agent 连接云端 Gate。两者业务流程一致，只是 Gate URL 和部署位置不同。
