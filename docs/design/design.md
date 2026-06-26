# TermBridge-go 产品设计

最后修改时间: 2026-06-26 14:05:34

## 1. 项目定位

TermBridge-go 是面向本地与远程工作流的 **Workspace Runtime Platform**。

它要解决的问题是：用户的 CLI agent、shell、构建任务和调试命令通常运行在某一台具体设备上，但用户希望用统一的 Browser workbench 查看、管理、继续这些工作，而不被本地窗口、单次终端或设备边界限制。

TermBridge-go 的产品承诺是：

- 用户可以在本地快速启动一个可记录、可恢复、可管理的 terminal session。
- 用户可以在 Browser 中看到自己的 device、workspace、session 和 terminal。
- 用户可以在未来从其他设备安全访问本地 runtime。
- 命令真实运行的位置始终清晰，Browser 负责管理和交互，不改变 runtime ownership。

---

## 2. 目标用户与使用场景

### 本地开发者

用户在自己的开发机上运行 Claude Code、Codex、Shell、构建命令或调试命令，希望：

- 用 CLI 快速启动任务。
- 在 Browser 中查看正在运行和已经结束的 session。
- 查看历史输出。
- 重新 attach terminal。
- 对 session 做 close、delete、rerun 等管理操作。

### 多设备用户

用户拥有多台设备，例如 Home-PC、Laptop、VPS，希望：

- 在一个 Browser workbench 中区分这些设备。
- 明确知道当前正在操作哪台 device。
- 从一台设备访问另一台设备上的 workspace/session。
- 不需要给本地设备开放入站端口。

### 远程访问用户

用户把 Gate 发布到远端后，希望：

- 本地 Agent 主动连接远端 Gate。
- Browser 通过远端 Gate 访问本地 runtime。
- 远程 terminal 行为与本地 self-connected 行为保持一致。
- 身份、凭据、授权和敏感数据边界是清晰的。

---

## 3. 产品模型

TermBridge-go 使用统一资源模型组织产品体验：

```text
User
  -> Device
    -> Workspace
      -> Session
        -> Terminal
```

### User

访问产品的人，也是后续授权、设备绑定和审计的主体。

### Device

运行 Agent 和 local runtime 的机器。Device 是 Browser workbench 的第一层选择对象。

### Workspace

一个项目目录或工作目录。Workspace 用来组织同一目录下的 sessions。

### Session

一次命令运行记录，也是用户在 Browser 中管理和继续工作的基本单位。

### Terminal

用户与 session 背后的 PTY / Process 交互的界面，负责输入、输出、resize、attach、detach 和历史回放体验。

---

## 4. 产品入口

### CLI

CLI 是本地启动任务的入口。

典型使用：

```text
termbridge exec -- claude
termbridge exec -- codex
termbridge exec -- pwsh
termbridge --cwd D:\project exec -- npm run dev
```

CLI 的产品价值是：让一次命令运行天然进入 TermBridge 的 workspace/session 管理体系。

### Serve

`termbridge serve` 是本地统一工作台入口。

它让本机同时具备：

- Browser workbench 服务。
- 本地 device 连接。
- workspace/session/terminal runtime。

用户启动后打开 `/sessions`，即可进入统一工作台。

### Browser Workbench

`/sessions` 是核心产品表面。

它承载：

- device 选择。
- workspace tree。
- session list。
- session create / edit / close / delete / rerun。
- history。
- terminal attach。

---

## 5. 核心体验原则

### Device 优先

用户进入 Browser workbench 后，首先要知道当前选中哪台 device，以及这台 device 是否可操作。

单 device 场景应尽量直达工作台；多 device 场景由用户明确选择，不在产品上猜测用户意图。

### 状态可信

workspace、session、tab、terminal 都必须属于当前 selected device。切换 device 时，旧 device 的状态不应污染新的工作台视图。

### Runtime ownership 清晰

用户命令真实运行在 Agent 所在设备的 local runtime 中。Gate 和 Browser 负责访问、管理和交互，但不拥有用户进程生命周期。

### 本地与远程体验一致

本地 self-connected 和远端 Gate 接入应共享同一套产品模型。部署位置可以变化，但 User / Device / Workspace / Session / Terminal 的理解方式不变。

### 失败可理解

当操作失败时，用户需要能区分：

- device offline。
- route unavailable。
- auth error。
- runtime error。
- 输入或操作不合法。

---

## 6. 当前产品能力

当前已经具备的产品能力：

- 本地 CLI command runner。
- workspace/session/history 基础模型。
- `/sessions` Browser workbench。
- `termbridge serve` 统一启动本地 Gate、Agent 和 runtime。
- device-scoped Browser API。
- Gateway / Agent relay 基础链路。
- 基础 Browser auth。
- workspace/session mutation。
- terminal attach、history 和 resize hardening。

当前仍需要收口的能力：

- `/sessions` device selector 状态表达。
- device 切换隔离的验证闭环。
- 真实 TUI、interrupt、backpressure、long-running 场景补证。
- Remote Gate / Local Agent 的部署、身份、凭据、授权和安全边界。

---

## 7. 本地与远端产品形态

### 本地 self-connected

```text
Browser
  -> Local Gate
  -> Local Agent
  -> Local Runtime
  -> PTY / Process
```

这是当前主要产品形态。用户在本机启动 `termbridge serve`，Browser 访问本机 `/sessions`，本机 device 出现在工作台中。

### 远端 Gate 接入

```text
Browser
  -> Remote Gate
  -> Local Agent
  -> Local Runtime
  -> PTY / Process
```

这是 M7 的目标形态。Gate 可以部署到远端，本地 Agent 主动连接远端 Gate，Browser 从其他设备访问本地 runtime。

远端形态的产品重点不是改变 terminal 的使用方式，而是补齐：

- secure pairing。
- device credential。
- token rotation。
- terminal websocket authorization。
- disconnect / reconnect UX。
- sensitive data boundary。

---

## 8. 产品质量标准

TermBridge-go 的产品质量不只看“能不能跑”，还要看用户路径是否稳定可信。

进入下一阶段前，至少需要满足：

1. 用户能清楚知道自己正在操作哪个 device。
2. workspace/session/terminal 状态不会跨 device 污染。
3. terminal attach、history、resize、interrupt 等关键交互有真实场景验证。
4. 失败状态能被用户理解，并能被开发者定位。
5. 远程访问能力必须先完成身份、凭据、授权和安全边界设计。
6. 每个 milestone 都有可复查的验证结论。

---

## 9. 与路线图的关系

本文描述 TermBridge-go 是什么、服务谁、提供什么产品体验，以及当前能力边界。

阶段优先级、进入条件、当前主线和后续规划以 `docs/design/roadmap.md` 为准。
