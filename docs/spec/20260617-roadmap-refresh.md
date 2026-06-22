# Roadmap 刷新规格
最后修改时间: 2026-06-17 15:05:21

Review status: Accepted

## Requirement basis

本规格基于：

- `docs/requirement/20260617-roadmap-refresh.md`
- `docs/spec/20260617-go-pty-feasibility.md`
- `README.md`
- 用户关于本地 CLI、GUI 容器和 Gateway Web 的最新决策

当前关键决策：

```text
1. Go + github.com/aymanbagabas/go-pty 是 Windows PTY 技术方向。
2. 本地阶段 termbridge CLI 是唯一一等使用入口。
3. 标准形态是 termbridge [options] -- <command...>。
4. GUI 是 CLI 的容器，不拥有 runtime。
5. Web terminal 技术风险提前验证，但 Gateway 阶段才作为正式 Web 产品入口。
```

## Overview

路线从“本地 Agent + Web terminal / Workspace UI”进一步收敛为：

```text
Local CLI first
  ↓
PTY Command Runner
  ↓
Session / Workspace Runtime Model
  ↓
CLI-first Local Product Surface
  ↓
Gateway Web Terminal
```

本地阶段不把 local Web 作为正式交付形态，也不要求用户理解常驻 daemon、workspace create/attach/stop 等复杂操作。用户只需要进入目录后运行：

```text
termbridge -- <command...>
```

或显式指定目录：

```text
termbridge --cwd <directory> -- <command...>
```

GUI 可以存在，但必须作为 CLI/runtime 的容器。它可以选择目录、选择命令、展示 terminal surface、展示 exit code 和 logs；但不能自己直接创建 PTY、启动进程、维护 Workspace lifecycle 或 History。

Web terminal 仍然必须提前验证。否则 Gateway 阶段会同时叠加 browser/xterm.js、WebSocket backpressure、Gate relay、auth、device routing 和 reconnect 等风险。

## Design decisions

### 1. CLI 是本地唯一一等入口

本地阶段的用户入口收敛为：

```text
termbridge [options] -- <command...>
```

示例：

```text
termbridge -- claude
termbridge -- codex
termbridge -- pwsh
termbridge --cwd D:\project -- claude
termbridge --cwd D:\project -- npm run dev
```

可以后续提供语法糖：

```text
termbridge claude
termbridge codex
```

但语义上仍应归一到 command runner，而不是另起一套 workspace 操作心智。

### 2. CLI runtime 内部拥有 PTY / Process lifecycle

本地阶段不应要求用户先启动：

```text
termbridge serve
termbridge workspace create
termbridge attach <id>
```

这些概念可以作为内部 runtime / 后续 Gateway 映射存在，但不是本地一等用户操作。

本地 MVP 的目标是：

```text
termbridge invocation
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

### 3. Ctrl+C 表示停止当前命令

CLI 用户按下 `Ctrl+C` 的意图是停止当前命令，而不是 detach 到后台。

建议策略：

```text
first Ctrl+C:
  soft interrupt foreground process

grace period:
  wait for process exit

second Ctrl+C or timeout:
  close PTY / kill process tree
```

正式实现必须区分：

```text
interrupt foreground process
close session
kill process tree
```

不能把这些都写成单一 Ctrl+C 字节。

### 4. GUI 是 CLI 容器

GUI 的边界：

可以做：

```text
select cwd
select command
spawn / supervise termbridge CLI
host terminal surface
show exit code
show logs
store UI preferences
```

不能做：

```text
directly create PTY
directly own Process lifecycle
directly maintain Workspace state
directly implement History buffer
directly implement interrupt / kill strategy
```

如果 GUI 实现了上述 runtime 能力，就等于另做一套 Agent，应避免。

### 5. Web terminal 是提前技术验证，不是本地正式产品面

本地阶段可以有 dev-only Web terminal technical spike：

```text
dev-only web page
  ↓
xterm.js
  ↓
local WebSocket
  ↓
TermBridge attach / command runtime
  ↓
PTY
```

目标是验证：

```text
xterm.js rendering
browser input / shortcut / paste / IME
resize
large output
slow client backpressure
WebSocket protocol shape
reconnect / detach semantics
```

但正式本地交付仍以 CLI 为核心。

### 6. Gateway 阶段 Web 才成为正式入口

Gateway 阶段目标形态：

```text
Browser
  ↓
xterm.js
  ↓
Gate
  ↓
Agent tunnel
  ↓
TermBridge runtime
  ↓
PTY / Process
```

到该阶段时，Web terminal 的基础风险应已通过 M2.5 / M5 提前验证，M6 主要处理 Gate relay、auth、device routing、tunnel reconnect 和 remote attach。

## Affected documents

将更新：

```text
docs/requirement/20260617-roadmap-refresh.md
docs/spec/20260617-roadmap-refresh.md
docs/plan/20260617-roadmap-refresh.md
docs/spec/20260617-go-pty-feasibility.md
```

不会恢复：

```text
docs/roadmap/project-milestones.md
```

不会删除：

```text
docs/spec/20260617-go-pty-feasibility.md
```

## Milestone target shape

更新后的里程碑建议：

```text
M0: Runtime 技术决策（已阶段性完成）
M1: Go CLI Skeleton
M2: PTY Command Runner MVP
M2.5: Web Terminal Technical Spike
M3: Session / Workspace Runtime Model
M4: Local Product Surface: CLI first, GUI optional container
M5: Runtime Hardening
M6: Gateway Web Terminal MVP
M7: Multi-device Beta
M8: Security / Packaging / Release
```

### M1-M2 的变化

从“本地 Agent skeleton + terminal API”改为“Go CLI skeleton + PTY command runner”。

核心是先跑通：

```text
termbridge [options] -- <command...>
```

而不是先做 local WebSocket terminal API。

### M2.5 的新增

增加 Web Terminal Technical Spike，提前验证 Gateway 将依赖的 browser terminal 风险。

该阶段不是正式本地 Web 产品。

### M3 的变化

Session / Workspace Model 从 CLI invocation 逐步抽象出来。Workspace 可以作为内部模型、history / metadata / Gateway 映射基础，但本地一等入口仍是 CLI。

### M4 的变化

Local Product Surface 必选 CLI polish，可选 GUI container。GUI 不能拥有 runtime。

### M6 的变化

Gateway 阶段正式引入 Web terminal 产品入口。

## Technical questions

后续仍需决策：

1. `termbridge <命令>` 的语法糖支持范围。
2. `Ctrl+C` 的 second interrupt / timeout escalation 交互细节。
3. GUI 容器是 spawn CLI、调用 local IPC，还是共享 Go runtime package。
4. Web terminal spike 使用临时 dev server 还是复用未来 Gateway UI 组件。
5. Session / Workspace 在本地 CLI 阶段是否默认持久化，还是先只作为 runtime 内部概念。

这些问题不阻塞更新里程碑 plan。

## Risks

### 1. CLI 入口和 Workspace 模型冲突

如果过早暴露 workspace 子命令，会让本地用户心智复杂化。应先保证 command runner 模型成立，再逐步引入 session/workspace metadata。

### 2. GUI runtime 分裂

GUI 如果绕过 CLI/runtime 直接管理 PTY，会导致 CLI、GUI、Gateway 三套 runtime。必须在文档和实现边界上禁止。

### 3. Web 技术验证过晚

Gateway 阶段才验证 xterm.js / browser input / backpressure 会导致风险叠加，因此必须前置 technical spike。

### 4. Ctrl+C 行为不一致

Windows shell、Claude Code、Codex 对 interrupt 的行为可能不同。`Ctrl+C` 策略必须分层、可观测、可升级，不能无限等待。

## Alternatives

### Alternative A：本地 Web 产品优先

不采用。它会提前引入 xterm.js、browser UI、local auth、localhost 安全和前端工程复杂度，偏离本地 CLI-first 决策。

### Alternative B：本地常驻 Agent + attach CLI 优先

暂不作为一等用户模型。常驻 Agent / attach 可以作为后续 Gateway 或后台 session 能力，但本地首要体验应是 `termbridge [options] -- <command...>`。

### Alternative C：GUI 直接实现 runtime

不采用。GUI 应作为 CLI 容器，否则等于另做一套 Agent。

## User review notes

用户明确修正：本地阶段应该只有一种一等使用方式，即进入目录后运行 `termbridge <命令>`，或者 `termbridge [options] -- <command...>`；GUI 也应该是 CLI 的容器，否则等于做 Agent。