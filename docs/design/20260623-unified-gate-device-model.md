# 统一 Gate / Device 模型设计整理

日期：2026-06-23

## 背景

当前讨论目标不是继续区分“本地模式”和“Gateway 模式”，而是将 TermBridge 收敛为一套统一模型：

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

因此，Gate 不是“远端模式”的专属组件，而是统一控制面。所谓“本地模式”如果继续存在，本质上只是绕过 Gate 的 direct path。

## 当前后端事实

`termbridge serve` 启动后，backend / gateway / agent connector 已经是一体的。

当前 serve 进程包含：

```text
termbridge serve
  ├─ HTTP server
  │   ├─ localapi      /api/...
  │   └─ gatewayapi    /api/gateway/...
  └─ agent client
      └─ connects to configured Gate URL
```

关键事实：

1. `serve` 创建同一个 web terminal registry / runtime。
2. `localapi` 直接访问这个 registry。
3. `agent client` 也通过 runtime adapter 访问同一个 registry。
4. `gatewayapi` 通过 tunnel route 调用已连接的 agent。
5. 如果 `agent.server_url` 指向当前 serve 自己，本地就形成 self-connected Gate。
6. 如果 `agent.server_url` 指向云端，当前本地进程就作为本地 Agent 接入云端 Gate。

因此当前后端已经具备统一 Gate 模型的雏形，只是前端和 API 仍保留了 local direct path 与 gateway path 两套入口。

## 两条现存路径

### Direct local path

当前所谓“本地模式”走：

```text
Browser
  -> /api/workspaces/tree
  -> /api/sessions/:sessionId/ws
  -> localapi
  -> registry
  -> PTY / Process
```

特点：

- 路径短。
- 功能完整。
- 不依赖 agent tunnel 已连接。
- 没有显式 device 概念。
- 与未来云端 Gate 路径不一致。

### Gate path

当前 Gateway 路径走：

```text
Browser
  -> /api/gateway/devices/:deviceId/...
  -> gatewayapi
  -> tunnel route
  -> agent client
  -> runtime adapter
  -> registry
  -> PTY / Process
```

本地 self-connected 时，gatewayapi 和 agent client 可以在同一个 serve 进程内；云端接入时，gatewayapi 在云端，agent client 在用户本地。

特点：

- 有显式 device 概念。
- 本地和云端可共用同一业务流程。
- 多一层 tunnel / relay / device route。
- 当前功能未完全覆盖 localapi，尤其 mutation 能力不足。

## 核心判断

长期不应存在两个产品模式：

```text
/sessions = 本地模式
/gateway  = Gateway 模式
```

长期应该只有一套资源模型：

```text
User
  -> Device
    -> Workspace
      -> Session
        -> Terminal
```

本地单机只是默认存在一个本机 device，且该 device 连接到本机 Gate。云端模式只是 Agent 连接到云端 Gate。

换句话说：

```text
本地使用：agent.server_url = self
云端使用：agent.server_url = cloud
```

业务流程不应该因为 Gate URL 不同而分裂。

## 与当前“本地模式”的真实区别

如果移除 direct local path，改成本地也走 Gate，则最终执行资源仍然是同一个 registry / PTY / Process，但请求路径不同。

| 维度 | 当前 direct local path | 统一 Gate path，本地 self-connected |
|---|---|---|
| 前端入口 | `/api/workspaces`, `/api/sessions` | device-scoped Gate API |
| 资源模型 | 隐式本机 | 显式 Device |
| 控制面 | localapi 直接调用 registry | gatewayapi 通过 agent tunnel 调用 registry |
| Terminal | Browser WS 直连 localapi | Browser WS 经 gateway relay / tunnel 到 agent |
| 启动依赖 | HTTP server ready 即可 | HTTP server ready + Agent registered |
| 云端一致性 | 差 | 好 |
| 当前功能完整度 | 完整 | 还缺 mutation |

## 成本差异

### 1. 补齐 Gate 控制面能力

这是最大成本。

当前 Gate path 主要支持：

- workspace tree
- session list
- history read
- terminal attach

要替代 localapi，还需要补齐：

- create session
- get session
- update session / rename
- close / stop session
- delete session
- delete workspace
- update workspace order
- 统一错误响应
- 权限 / capability 判断

否则前端全走 Gate 后会退化成 attach-only。

### 2. 本地也依赖 Agent 注册成功

direct local path 下，server ready 后前端即可访问 registry。

统一 Gate path 下，本地也需要：

```text
server ready -> agent connects -> device registered -> workspace/session available
```

需要处理：

- agent.server_url 配错。
- agent tunnel 连接失败。
- Gate 已启动但本机 device 尚未出现。
- device offline / reconnect。
- UI 中的 device loading / empty / offline 状态。

### 3. Terminal 链路更复杂

本地 direct terminal：

```text
Browser WS -> localapi -> registry client -> PTY
```

本地 self-connected Gate terminal：

```text
Browser WS -> gatewayapi terminal relay -> tunnel frames -> agent -> registry stream -> PTY
```

本机 loopback 下性能成本预计可接受，但复杂度增加：

- frame encode/decode
- stream id 管理
- pending request map
- relay backpressure
- writer conflict
- tunnel close / reconnect
- terminal detach / close 语义映射

### 4. Auth 必须统一

如果前端统一走 Gate，但 localapi 仍然裸露，就会形成绕过登录的后门。

需要长期收口：

- Browser 访问 Gate 需要统一登录态。
- Agent 连接 Gate 需要正式 device credential，不能依赖固定账号密码。
- localapi 如果保留，需要加同一层 auth，或仅作为 dev/internal/compat API。
- WebSocket 也必须纳入同一鉴权边界。

### 5. 配置成为核心路径

本地使用和云端接入的差异主要由 Gate URL 决定：

```yaml
agent:
  server_url: http://127.0.0.1:9010   # 本地 self-connected
```

或：

```yaml
agent:
  server_url: https://gate.example.com # 云端 Gate
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
7. local direct path 可以逐步降级为兼容层，而不是长期主路径。

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

## 推荐 API 方向

当前路径：

```text
/api/workspaces/tree
/api/sessions/:sessionId/ws

/api/gateway/devices/:deviceId/workspaces/tree
/api/gateway/devices/:deviceId/sessions/:sessionId/ws
```

长期应收敛为 device-scoped API：

```text
/api/me
/api/devices
/api/devices/:deviceId/workspaces/tree
/api/devices/:deviceId/sessions
/api/devices/:deviceId/sessions/:sessionId
/api/devices/:deviceId/sessions/:sessionId/history
/api/devices/:deviceId/sessions/:sessionId/ws
```

`gateway` 可以继续作为内部 package / transport 名称，但不应暴露为产品级路由概念。

## 推荐迁移路径

### Phase 1：前端移除“本地模式”概念

- 删除 `/gateway` 作为独立页面入口。
- `/sessions` 内以 Device 为第一层资源。
- 本地使用时默认选中 self-connected local device。
- 前端数据加载优先走 Gate / device-scoped API。
- localapi 后端暂时保留，作为兼容层。

### Phase 2：补齐 Gateway mutation

将 localapi 已有写操作补到 Gate / Agent tunnel：

- create session
- update session
- close session
- delete session
- delete workspace
- workspace reorder

完成后，前端不再需要调用 localapi。

### Phase 3：统一认证与设备凭证

- Browser auth 上提到统一 Gate 层。
- Agent 使用正式 device credential / pairing flow。
- 移除固定 admin/admin 类 MVP 认证假设。
- localapi 若保留，纳入同一认证边界。

### Phase 4：localapi 收口

可选策略：

1. localapi 仅 dev 模式启用；
2. localapi 变成 device API 的兼容别名；
3. localapi 加 auth 后保留给内部工具；
4. 最终删除 public localapi。

## 当前需要避免的误区

1. 不要把 Gateway 理解成“远端模式”。本地也应该连接 Gate。
2. 不要让 `/gateway` 成为长期产品入口。
3. 不要只改前端 URL，而不补齐 Gate mutation。
4. 不要保留未鉴权 localapi 作为长期绕过路径。
5. 不要让本地使用强依赖云端；本地 self-connected Gate 应该是 first-class。

## 结论

在当前后端架构中，`termbridge serve` 已经是一体化入口。所谓“本地模式”只是 Browser 直接走 localapi 绕过 Gate 的短路径。

长期应移除本地模式这个产品/前端概念，统一为：

```text
Browser -> Gate -> Device Agent -> Workspace -> Session -> Terminal
```

本地使用时，Agent 连接自己的 Gate；云端使用时，Agent 连接云端 Gate。两者业务流程一致，只是 Gate URL 和部署位置不同。
