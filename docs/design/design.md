# TermBridge-go 架构设计

最后修改时间: 2026-06-26 12:07:02

## 1. 产品定位

TermBridge-go 是面向本地与远程工作流的 **Workspace Runtime Platform**。

它提供一套统一能力：

- 在本地工作目录中启动 Claude Code、Codex、Shell 或其他 CLI agent。
- 将运行中的 workspace / session / terminal 暴露给 Browser workbench。
- 通过 Gate / Agent 模型支持本地 self-connected 与后续云端接入。
- 保持命令运行、终端流、历史记录和设备连接的统一资源模型。

核心用户价值：

1. 用户可以用 CLI 快速启动工作 session。
2. 用户可以在 Browser 中管理 workspace、session 和 terminal。
3. 用户可以在多设备场景中访问同一套 runtime 能力。
4. Runtime ownership 清晰，Gateway 只做控制面和 relay，不直接运行用户命令。

---

## 2. 核心资源模型

```text
User
  -> Device
    -> Workspace
      -> Session
        -> Terminal
```

### User

用户身份与访问授权主体。当前用于 Browser auth，后续承载多设备绑定、权限和审计。

### Device

运行 Agent 与 local runtime 的机器，例如 Home-PC、Laptop、VPS。

Device 是 Browser workbench 的第一层选择对象。所有 workspace、session、terminal 操作都必须落到具体 device。

### Workspace

一个项目目录或工作目录。Workspace 组织同一目录下的 session，并承载排序、展示和后续恢复能力。

### Session

一次命令运行记录和 attach 单元。Session 保存命令、cwd、状态、退出码、历史记录等 metadata。

Session 不拥有独立于 runtime 的进程生命周期；真实 PTY / Process lifecycle 归 runtime 管理。

### Terminal

Browser 与 PTY 之间的交互通道。Terminal 负责输入、输出、resize、attach、detach 和 bounded history replay。

---

## 3. 目标运行架构

```text
Browser /sessions
  -> Gateway Browser API
  -> Device route
  -> Agent tunnel
  -> Runtime adapter
  -> Workspace / Session / Terminal runtime
  -> PTY / Process
```

本地使用和云端接入共享同一条产品路径。

### 本地 self-connected 形态

```text
Browser
  -> Local Gate
  -> Local Agent
  -> Local Runtime
  -> PTY / Process
```

本地开发和日常使用由 `termbridge serve` 启动统一后端。Agent 连接本机 Gate，Browser 通过 `/sessions` 使用同一套 device-scoped API。

### 云端接入形态

```text
Browser
  -> Cloud Gate
  -> Local Agent
  -> Local Runtime
  -> PTY / Process
```

云端接入时，Gate 部署位置变化，但 Browser workbench、Device、Workspace、Session 和 Terminal 模型不变。

---

## 4. 入口与产品表面

### CLI 入口

```text
termbridge [options] exec -- <command...>
```

CLI 是本地命令运行的一等入口。

示例：

```text
termbridge exec -- claude
termbridge exec -- codex
termbridge exec -- pwsh
termbridge --cwd D:\project exec -- npm run dev
```

### Service 入口

```text
termbridge serve
```

`serve` 启动：

```text
Gateway service
Agent connector
Runtime registry / adapter
Browser API
Agent tunnel endpoint
```

### Browser 入口

```text
/sessions
/settings
/help
```

`/sessions` 是统一工作台，承载：

- device 选择
- workspace tree
- session list
- create / edit / close / delete / rerun
- reorder
- history
- terminal attach

---

## 5. Browser API 结构

Browser 产品 API 使用 device-scoped 路径：

```text
/api/me
/api/login
/api/logout
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

Agent 内部传输入口：

```text
/api/agent/tunnel
```

---

## 6. 组件职责

| 组件 | 职责 |
| --- | --- |
| CLI | 解析命令，启动本地 command runner，处理 cwd/env/stdin/stdout/exit code。 |
| Runtime | 管理 workspace、session、history、terminal stream、PTY 和 process lifecycle。 |
| PTY abstraction | 隔离具体 PTY backend，向业务层提供 start/read/write/resize/interrupt/close 能力。 |
| Agent connector | 主动连接 Gate，注册 device，接收 tunnel request，调用 local runtime adapter。 |
| Gateway service | 提供 Browser API、auth、device registry、routing、pending request、terminal relay。 |
| Browser workbench | 提供 `/sessions` UI，管理 device/workspace/session/terminal 用户操作。 |
| Config | 描述 Gate listen URL、Agent connect URL、device identity、auth、runtime state dir。 |

---

## 7. Runtime ownership

PTY / Process lifecycle 的唯一 owner 是 local runtime。

```text
Runtime
  -> PTY
  -> Process
```

Gateway 只维护：

```text
Device registry
Route table
Tunnel connection
Pending request
Terminal relay
```

Agent connector 是 Gateway 与 local runtime 之间的 adapter：

```text
Gateway request
  -> Agent tunnel frame
  -> Runtime adapter method
  -> Runtime / PTY / Process
```

---

## 8. Terminal 链路

```text
Browser xterm.js
  -> terminal websocket
  -> Gateway terminal relay
  -> tunnel frame
  -> Agent terminal stream
  -> Runtime terminal session
  -> PTY
```

Terminal 链路必须支持：

- attach
- input
- output
- resize
- bounded history replay
- close / detach
- device disconnect / reconnect
- error propagation
- browser-side diagnostics

当前 resize 策略：

- Browser 测量 xterm 可用尺寸。
- 前端使用 one-cell safety margin 避免边界抖动。
- Browser local grid 与后端 PTY 使用同一个安全尺寸。
- Gateway / Agent attach 时传递初始 cols / rows。
- PTY resize 失败不推进成功状态，允许后续重试。

---

## 9. State 与 lifecycle

### Device state

```text
online
offline
last_seen
connected_at
```

Device state 用于 Browser workbench 的 device selector、route availability 和 reconnect 反馈。

### Session lifecycle

```text
running
stopped
failed
```

Session lifecycle 用于：

- terminal attach 判断
- close / delete / rerun 操作
- history 展示
- tab 状态同步

### Attachment state

```text
unattached
attached
detached
reattaching
```

Attachment state 用于表达 Browser terminal 与 runtime terminal stream 之间的连接关系。

---

## 10. 配置模型

关键配置：

```yaml
runtime:
  state_dir: .termbridge

gate:
  listen_url: http://127.0.0.1:9010
  browser:
    allowed_origins:
      - http://localhost:9011
  api:
    expose_errors: false

agent:
  connect_url: ""
  device_id: ""
  device_name: ""
```

配置语义：

- `gate.listen_url`：Gate 监听地址。
- `agent.connect_url`：Agent 连接目标；为空时使用 `gate.listen_url`。
- `agent.device_id` / `agent.device_name`：当前设备身份。
- `runtime.state_dir`：workspace/session/history/device state 存储根目录。

---

## 11. 安全边界

### Browser auth

Browser API 使用登录态 / Bearer token 保护。

### Device identity

Device 是 Agent 连接 Gate 后注册出来的运行主体。后续产品化需要正式 credential、pairing 和 rotation。

### Terminal authorization

Terminal websocket 必须与 Browser auth、device route、workspace/session ownership 保持一致。

### Sensitive data

命令、cwd、history、terminal output 可能包含敏感信息。后续 audit、日志和 remote access 设计必须包含脱敏与访问边界。

---

## 12. 架构质量要求

1. API 必须 device-scoped，避免跨 device 状态污染。
2. Gateway relay 失败必须可观测，并向 Browser 返回结构化错误。
3. Runtime 状态只在真实操作成功后推进。
4. Resize、close、delete、rerun 等状态变更必须可重试或明确失败。
5. Browser workbench 切换 device 时必须隔离 workspace/session/tab/terminal 状态。
6. Terminal output 必须有 backpressure 或 bounded queue 策略。
7. History 必须 bounded，截断行为可观测。
8. Auth、device identity、terminal websocket 必须形成一致边界。
9. 每个进入下一阶段的 milestone 必须有验证记录。