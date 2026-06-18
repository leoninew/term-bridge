# TermBridge-go 设计文档

最后修改时间: 2026-06-17

## 背景

### 历史架构问题

原 TermBridge（目录 ../TermBridge）使用 Python 实现，基于 `ttyd + tmux` 架构：

```text
Browser
    ↓
ttyd
    ↓
tmux Session
    ↓
Process
```

这种架构存在以下问题：

1. **状态分层过多**：Workspace → tmux Session → PTY → Process，双层状态模型导致状态同步复杂
2. **依赖外部组件**：ttyd 和 tmux 是外部进程，TermBridge 无法完全控制其生命周期
3. **扩展性受限**：tmux 模型限制了云端跨设备访问的能力
4. **平台绑定**：tmux 在 Windows 上需要 Cygwin，增加了部署复杂度

### 架构决策

TermBridge-go 选择移除 ttyd 和 tmux，由 TermBridge 自主管理运行时：

**核心对象从：**

```text
tmux Session
```

**变为：**

```text
Workspace
```

**技术原则：**

1. **TermBridge 自主管理运行时**：不依赖 ttyd、tmux 等外部会话管理组件，所有运行时状态由 TermBridge 管理
2. **Workspace 是唯一状态对象**：不存在 Workspace → tmux Session 的双层状态模型，统一为 Workspace → PTY → Process
3. **Agent 主动连接 Gate**：支持 NAT 穿透、家庭宽带、企业网络

---

## 愿景

### 目标架构

```text
┌─────────────────────┐
│ Browser             │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ TermBridge Gate     │
│                     │
│ Auth                │
│ Device Registry     │
│ Workspace Registry  │
│ Routing             │
└─────────┬───────────┘
          │ WSS
          ▼
┌─────────────────────┐
│ TermBridge Agent    │
│                     │
│ Workspace Manager   │
│ PTY Manager         │
│ Process Manager     │
│ History Manager     │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ Workspace           │
│                     │
│ Claude Code         │
│ Codex               │
│ Shell               │
└─────────────────────┘
```

### 核心运行时链路

```text
termbridge CLI invocation
  ↓
TermBridge runtime
  ↓
PTY abstraction
  ↓
go-pty
  ↓
Windows ConPTY
  ↓
user command
```

### 三级资源模型

```text
User
    ↓
Device (MacBook / Home-PC / VPS)
    ↓
Workspace (Claude / Codex / Shell)
```

### 技术路线

#### Phase 1：Runtime 重构（核心阶段）

目标：同时移除 ttyd 和 tmux

架构：

```text
Browser
    ↓
xterm.js
    ↓
WebSocket
    ↓
TermBridge Agent
    ↓
PTY
    ↓
Claude Code / Codex
```

交付内容：

- **PTY Manager**：PTY 创建、销毁、Resize、Attach、Detach
- **Process Manager**：Claude Code、Codex、Shell 生命周期管理
- **Workspace Manager**：Workspace 创建、销毁、恢复、状态同步
- **Terminal History**：替代 tmux buffer，支持历史缓存、滚动回放、断线恢复

#### Phase 2：Workspace 模型完善

架构：

```text
Workspace
    ↓
PTY
    ↓
Process
```

交付内容：

- **Workspace Metadata**：

```json
{
  "id": "workspace-001",
  "name": "project-a",
  "runtime": "claude",
  "status": "running"
}
```

- **Workspace Recovery**：

```text
Browser关闭
    ↓
Workspace继续运行
    ↓
重新打开
    ↓
恢复连接
```

- **Multi Attach**：支持 PC、Laptop、Phone 同时查看同一个 Workspace

#### Phase 3：Gate 架构

目标：实现跨设备访问

架构：

```text
Browser
    ↓
Gate
    ↓
Agent
    ↓
Workspace
```

交付内容：

- **User**：用户
- **Device**：MacBook、Home-PC、VPS
- **Workspace**：Claude、Codex、Shell 三级资源模型
- **Device Registry**：设备注册中心
- **Workspace Registry**：Workspace 路由中心
- **Reverse Tunnel**：Agent ──WSS──► Gate 长连接

### 核心收益

#### 性能

移除 iframe、ttyd、tmux，终端链路缩短为：

```text
Browser
    ↓
WebSocket
    ↓
PTY
```

接近 VSCode Terminal 架构。

#### 架构

统一状态模型：

```text
Workspace
    ↓
PTY
    ↓
Process
```

不存在额外 Session 层。

#### 云端扩展

天然支持：

```text
用户
    ↓
设备
    ↓
Workspace
```

资源模型。

#### 产品定位

TermBridge 从第一天开始就是：

```text
Workspace Runtime Platform
```

而不是：

```text
ttyd + tmux 的封装工具
```

这样后续无论接 Claude Code、Codex、Gemini CLI 还是自定义 Agent，都不会受到 tmux 模型的限制。

---

## 技术决策记录

### 当前已确认决策

1. **Windows PTY Runtime**: Go
2. **PTY 技术方向**: github.com/aymanbagabas/go-pty
3. **底层机制**: Windows ConPTY / Pseudoconsole
4. **本地交付方向**: CLI first
5. **GUI 定位**: CLI 容器，不拥有 runtime
6. **Web 策略**: Gateway 前技术验证，Gateway 后正式产品入口

### 设计原则

1. **CLI 是本地唯一一等入口**：本地阶段标准使用方式为 `termbridge [options] -- <command...>`
2. **GUI 是 CLI 容器，不是另一套 Agent**：GUI 不能直接创建 PTY、拥有 Process lifecycle、维护 Workspace state
3. **Web 技术验证前置，正式 Web 放到 Gateway**：避免 Gateway 阶段同时暴露多个风险
4. **Go + go-pty 必须被 abstraction 隔离**：业务层不直接依赖 go-pty.Pty
5. **Ctrl+C 是停止当前命令，不是 detach**：必须区分 interrupt foreground process / close session / kill process tree
6. **先 CLI Runtime，后本地产品面，再 Gateway**：不在 CLI runtime 稳定前提前建设复杂 Web / GUI / Gateway

### 后续仍需决策

1. `termbridge <命令>` 是否作为 `termbridge -- <命令>` 的语法糖支持
2. `Ctrl+C` 升级策略的具体交互：是否第一次 soft interrupt，第二次 hard stop，或使用超时自动升级
3. GUI 容器调用 CLI 的具体方式：spawn CLI、local IPC，还是复用同一 Go runtime package
4. Web terminal spike 放在 M2.5 独立阶段，还是并入 M3/M5 gate
5. Session / Workspace 在本地 CLI 阶段是否默认持久化，还是先只作为 runtime 内部概念
