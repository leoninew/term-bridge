我同意。如果已经明确目标是：

```text
TermBridge Agent
    ↔
TermBridge Gate
```

那么继续保留 tmux 实际上是在给未来埋技术债。

之前保留 tmux 的建议，本质上是站在 **低风险渐进改造** 角度。但如果现在还处于架构设计阶段，而不是已经有数千用户在线运行，那么更合理的方案是：

> 删除 ttyd 和 tmux 一起进行。

因为未来架构中的核心对象已经不是：

```text
tmux Session
```

而是：

```text
Workspace
```

---

# 修订后的技术路线

## 技术原则

### 原则一：TermBridge 自主管理运行时

不依赖：

* ttyd
* tmux

等外部会话管理组件。

所有运行时状态由 TermBridge 管理。

---

### 原则二：Workspace 是唯一状态对象

系统内部不存在：

```text
Workspace
    ↓
tmux Session
```

双层状态模型。

统一为：

```text
Workspace
    ↓
PTY
    ↓
Process
```

---

### 原则三：Agent 主动连接 Gate

```text
Agent
    ──WSS──► Gate
```

支持：

* NAT 穿透
* 家庭宽带
* 企业网络

---

# 修订后的阶段规划

## Phase 1：Runtime 重构（核心阶段）

目标：

同时移除 ttyd 和 tmux。

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

### PTY Manager

负责：

```text
PTY 创建
PTY 销毁
Resize
Attach
Detach
```

---

### Process Manager

负责：

```text
Claude Code
Codex
Shell
```

生命周期管理。

---

### Workspace Manager

负责：

```text
Workspace 创建
Workspace 销毁
Workspace 恢复
Workspace 状态同步
```

---

### Terminal History

替代 tmux buffer。

支持：

```text
历史缓存
滚动回放
断线恢复
```

---

## Phase 2：Workspace 模型完善

架构：

```text
Workspace
    ↓
PTY
    ↓
Process
```

交付内容：

### Workspace Metadata

例如：

```json
{
  "id": "workspace-001",
  "name": "project-a",
  "runtime": "claude",
  "status": "running"
}
```

---

### Workspace Recovery

实现：

```text
Browser关闭
↓
Workspace继续运行

重新打开
↓
恢复连接
```

---

### Multi Attach

支持：

```text
PC
Laptop
Phone
```

同时查看同一个 Workspace。

---

## Phase 3：Gate 架构

目标：

实现跨设备访问。

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

### User

```text
用户
```

### Device

```text
MacBook
Home-PC
VPS
```

### Workspace

```text
Claude
Codex
Shell
```

三级资源模型。

---

### Device Registry

设备注册中心。

---

### Workspace Registry

Workspace 路由中心。

---

### Reverse Tunnel

```text
Agent
    ──WSS──► Gate
```

长连接。

---

# 最终目标架构

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

# 核心收益

### 性能

移除：

```text
iframe
ttyd
tmux
```

终端链路缩短为：

```text
Browser
    ↓
WebSocket
    ↓
PTY
```

接近 VSCode Terminal 架构。

---

### 架构

统一状态模型：

```text
Workspace
    ↓
PTY
    ↓
Process
```

不存在额外 Session 层。

---

### 云端扩展

天然支持：

```text
用户
    ↓
设备
    ↓
Workspace
```

资源模型。

---

### 产品定位

TermBridge 从第一天开始就是：

```text
Workspace Runtime Platform
```

而不是：

```text
ttyd + tmux 的封装工具
```

这样后续无论接 Claude Code、Codex、Gemini CLI 还是自定义 Agent，都不会受到 tmux 模型的限制。
