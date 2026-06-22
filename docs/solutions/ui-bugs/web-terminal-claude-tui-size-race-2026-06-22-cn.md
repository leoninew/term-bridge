---
title: Web terminal 中 Claude TUI 初始尺寸不一致
date: 2026-06-22
category: ui-bugs
module: web terminal
problem_type: ui_bug
component: tooling
symptoms:
  - Claude Code TUI 的横向分隔线没有填满终端宽度
  - 确认后主界面布局错乱，但 session 状态仍为 Running
  - 前端 xterm 实测为 207x33，而早期 PTY 创建时使用 173x24
  - 第一次 xterm resize 发生在 WebSocket OPEN 之前，未能到达后端
root_cause: async_timing
resolution_type: code_fix
severity: medium
tags: [web-terminal, xterm, tui, pty, websocket, claude-code]
---

# Web terminal 中 Claude TUI 初始尺寸不一致

## 问题

在 TermBridge 的 Web terminal 中运行 `claude` 时，Claude Code 的 TUI 会出现布局错乱：初始黄色分隔线没有填满可见终端宽度，进入主界面后输入框、分隔线和光标位置都按错误宽度排列。session 本身仍保持 `running`，浏览器端也能发送输入事件，但 Claude Code 的第一帧 TUI 已经基于错误的 PTY 尺寸完成布局。

## 现象

- Claude Code 安全确认界面能显示，但确认后主 TUI 错位。
- 初始横向分隔线只填充了可见终端的一部分宽度。
- 前端诊断显示 live xterm 实际尺寸为 `207x33`，而旧的创建请求使用 `173x24`。
- xterm 在 WebSocket 打开前就产生了 resize：

  ```text
  xterm.resize ... cols: 207, rows: 33
  socket.control.deferred {type: 'resize', cols: 207, rows: 33, readyState: null}
  ```

- 修复前，后端没有收到这次 deferred resize，因此日志里没有对应的 `terminal resize received` / `terminal pty resize`。

## 没有解决问题的尝试

- 将后端 runtime 的 current size 初始化为 PTY initial size，解决了 attach 日志仍显示 `80x25` 的问题，但不能单独修复 TUI 错乱。
- 移除 xterm 的 `convertEol: true` 能让 PTY 输出更接近原始终端行为，但不能修复错误的初始 terminal geometry。
- 只从后端 resize 去重角度排查会漏掉前端 race：xterm 已经测出了正确尺寸，但首次 resize 在 WebSocket 未打开时发出并被丢弃。
- 在 terminal 挂载前用 `window.innerWidth` / `window.innerHeight` 估算尺寸不够准确。xterm 的真实列数依赖实际容器、padding、scrollbar、字体指标和 FitAddon 的 cell 测量结果。

## 解决方案

最终修复目标是让 session 创建、PTY 启动、WebSocket attach 和 xterm resize 使用同一套真实尺寸。

### 创建 PTY 前使用 xterm/FitAddon 实测初始尺寸

`web/src/App.vue` 在发送 `POST /api/sessions` 之前，先在当前创建表单所在 workbench 区域中临时挂载一个隐藏的 terminal shell/container，并用 xterm `FitAddon` 测量真实 `cols/rows`。旧的 viewport 估算仅作为 fallback 保留。

```ts
function measureInitialTerminalSize(): { cols: number; rows: number } {
  const measured = measureCreateSessionWorkbench()
  if (measured) {
    return measured
  }
  const cols = Math.max(80, Math.min(10000, Math.floor((window.innerWidth - 360) / 9)))
  const rows = Math.max(24, Math.min(10000, Math.floor((window.innerHeight - 180) / 18)))
  logTerminalDiagnostic('xterm.measure.fallback', { cols, rows })
  return { cols, rows }
}
```

隐藏测量容器要尽量模拟真实 tab 内容区和 terminal shell：

```ts
function measureCreateSessionWorkbench(): { cols: number; rows: number } | null {
  const workbench = createSessionWorkbench.value
  if (!workbench) {
    logTerminalDiagnostic('xterm.measure.missing-workbench')
    return null
  }

  const wrapper = document.createElement('section')
  wrapper.style.position = 'fixed'
  wrapper.style.left = '-10000px'
  wrapper.style.width = `${workbench.clientWidth}px`
  wrapper.style.height = `${workbench.clientHeight}px`
  wrapper.style.display = 'flex'
  wrapper.style.flexDirection = 'column'
  wrapper.style.padding = '8px'
  wrapper.style.boxSizing = 'border-box'
  wrapper.style.visibility = 'hidden'
  wrapper.style.pointerEvents = 'none'

  const shell = document.createElement('section')
  shell.className = 'terminal-shell'
  shell.style.flex = '1'

  const container = document.createElement('div')
  container.className = 'terminal-container'
  shell.appendChild(container)
  wrapper.appendChild(shell)
  document.body.appendChild(wrapper)

  try {
    return measureXtermSize(container)
  } finally {
    wrapper.remove()
  }
}
```

`web/src/components/terminal/useXterm.ts` 提供可复用的测量函数，并与 live terminal 使用同一套 xterm options：

```ts
export function measureXtermSize(element: HTMLElement): { cols: number; rows: number } | null {
  const terminal = new Terminal(terminalOptions())
  const fitAddon = new FitAddon()
  terminal.loadAddon(fitAddon)
  try {
    terminal.open(element)
    fitAddon.fit()
    if (terminal.cols < 1 || terminal.rows < 1) {
      logTerminalDiagnostic('xterm.measure.invalid-size', {
        source: 'measure',
        clientWidth: element.clientWidth,
        clientHeight: element.clientHeight,
      })
      return null
    }
    return { cols: terminal.cols, rows: terminal.rows }
  } finally {
    terminal.dispose()
  }
}
```

### 保留并补发 WebSocket OPEN 前产生的 resize

`web/src/features/sessions/useTerminalSocket.ts` 现在会缓存最近一次 resize。如果 resize 发生时 socket 还未打开，就在 socket `OPEN` 后、发送 `hello` 后补发。

```ts
let pendingResize: Extract<ClientControlMessage, { type: 'resize' }> | null = null

next.onopen = () => {
  status.value = 'connected'
  logTerminalDiagnostic('socket.open', { path: diagnosticWebSocketPath(wsUrl) })
  sendControl({ type: 'hello' })
  if (pendingResize) {
    sendControl(pendingResize)
  }
}

function sendControl(message: ClientControlMessage) {
  if (message.type === 'resize') {
    pendingResize = message
  }
  if (socket.value?.readyState !== WebSocket.OPEN) {
    logTerminalDiagnostic('socket.control.deferred', {
      type: message.type,
      cols: 'cols' in message ? message.cols : undefined,
      rows: 'rows' in message ? message.rows : undefined,
      readyState: socket.value?.readyState ?? null,
    })
    return
  }
  socket.value.send(encodeControl(message))
  if (message.type === 'resize' && pendingResize === message) {
    pendingResize = null
  }
}
```

关键点：不要在 `connect()` 开始时清空 `pendingResize`。live xterm 的第一次 resize 可能发生在 `connect()` 调用之前，或发生在 WebSocket 进入 `OPEN` 之前。

### 后端 runtime size 与 PTY initial size 对齐

后端创建 runtime 时使用启动 PTY 的同一个 `size`，避免 attach 阶段仍从默认 `80x25` 开始。

```go
runtime := newSessionRuntime(r, sess, ptySession, historyWriter, size)
```

```go
func newSessionRuntime(..., initialSize process.TerminalSize) *SessionRuntime {
	return &SessionRuntime{
		// ...
		current: initialSize.OrDefault(),
	}
}
```

### 将高频 binary 输出日志降为 debug

`terminal websocket binary sent` 是高频输出路径遥测，保留价值但不适合默认 info 日志，因此在 `internal/transport/http/localapi/server.go` 中降为 debug：

```go
s.logger.Debug("terminal websocket binary sent", "session_id", client.SessionID(), ...)
```

## 为什么有效

Claude Code 这类全屏 TUI 会根据当前 PTY 的 rows/cols 绘制第一屏。如果进程启动时 PTY 是 `173x24`，但浏览器 xterm 实际是 `207x33`，TUI 会先按错误网格绘制分隔线、输入框、提示信息和光标位置。后续 resize 即使到达，也可能无法完全恢复第一帧已经错误布局的状态。

修复后，各层尺寸一致：

```text
xterm.measure ... cols=207 rows=33
session.create.request ... cols=207 rows=33
terminal pty start ... cols=207 rows=33
terminal attach start ... current_cols=207 current_rows=33
socket.control.send {type: 'resize', cols: 207, rows: 33}
terminal resize received ... cols=207 rows=33
```

这同时消除了两个问题：

1. 进程启动时就拿到与可见 xterm 一致的尺寸。
2. 即使 xterm 在 WebSocket OPEN 前产生 resize，也会被缓存并补发，不再静默丢弃。

## 预防

- terminal 尺寸应被视为“测量状态”，不要只用 viewport 估算。xterm.js 场景下优先用 `FitAddon` 对真实或同结构 DOM 容器测量。
- resize 事件依赖 WebSocket 状态时，应缓存最近一次 resize，等连接打开后补发。丢掉首次 resize 就足以破坏 TUI 应用。
- 保留跨层诊断日志，方便对齐 terminal size pipeline：

  ```text
  xterm.measure
  session.create.request
  terminal pty start
  terminal attach start
  socket.control.send resize
  terminal resize received
  terminal pty resize
  ```

- 后端应有回归测试覆盖 runtime 初始 size 与 PTY initial size 一致，避免 attach 阶段重新回到默认 `80x25`。
- 高频 binary 输出日志放在 debug；info 日志保留 lifecycle 和 control-plane 事件。

## 相关

- 英文版：`docs/solutions/ui-bugs/web-terminal-claude-tui-size-race-2026-06-22.md`
- Requirement 记录：`docs/requirement/20260622-claude-tui-terminal-render-fix.md`
- 关键文件：
  - `web/src/App.vue`
  - `web/src/components/terminal/useXterm.ts`
  - `web/src/features/sessions/useTerminalSocket.ts`
  - `web/src/components/terminal/TerminalView.vue`
  - `internal/application/terminal/runtime.go`
  - `internal/transport/http/localapi/server.go`
