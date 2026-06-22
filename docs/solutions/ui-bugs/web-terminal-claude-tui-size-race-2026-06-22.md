---
title: Web terminal Claude TUI initial size mismatch
date: 2026-06-22
category: ui-bugs
module: web terminal
problem_type: ui_bug
component: tooling
symptoms:
  - Claude Code TUI rendered with horizontal separators that did not fill the terminal width
  - After confirmation, the main TUI layout was visually misaligned even though the session stayed Running
  - Frontend xterm measured 207x33 while the PTY was initially created with 173x24
  - The first xterm resize happened before WebSocket OPEN and was not reaching the backend
root_cause: async_timing
resolution_type: code_fix
severity: medium
tags: [web-terminal, xterm, tui, pty, websocket, claude-code]
---

# Web terminal Claude TUI initial size mismatch

## Problem

Running `claude` inside TermBridge's web terminal produced a broken Claude Code TUI: the initial yellow separator and later full-screen layout used a narrower width than the visible terminal. The session remained `running`, and browser input events were sent, but the TUI was already laid out against the wrong PTY dimensions.

## Symptoms

- The Claude Code safety screen appeared, but after confirmation the main TUI was misaligned.
- The initial horizontal divider only filled part of the visible terminal width.
- Frontend diagnostics showed the live xterm size was `207x33`, while older session creation logs used `173x24`.
- A resize was produced before the WebSocket opened:

  ```text
  xterm.resize ... cols: 207, rows: 33
  socket.control.deferred {type: 'resize', cols: 207, rows: 33, readyState: null}
  ```

- Before the fix, the backend did not receive the deferred resize, so there was no matching `terminal resize received` / `terminal pty resize` line.

## What Didn't Work

- Initializing the backend runtime's current size from the PTY initial size fixed misleading `80x25` attach state, but did not fix the TUI by itself.
- Removing xterm `convertEol: true` made PTY output handling more faithful, but did not address the wrong initial terminal geometry.
- Assuming the issue was only a backend resize deduplication problem missed the frontend race: xterm measured the correct size, but the first resize could be emitted before the WebSocket was open and then discarded.
- Using `window.innerWidth` / `window.innerHeight` estimates before mounting the terminal was not precise enough for full-screen TUIs, because xterm's real column count depends on the actual container, padding, scrollbar, font metrics, and FitAddon cell measurements.

## Solution

The working fix made the terminal size path factual from session creation through WebSocket attach.

### Measure initial size with xterm/FitAddon before creating the PTY

`web/src/App.vue` now measures a hidden terminal using the same terminal shell/container structure before sending `POST /api/sessions`. This replaces viewport estimation for the primary path and keeps the old estimate only as a fallback.

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

The hidden measurement mirrors the actual tab content padding and terminal shell before calling the shared xterm measurement helper:

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

`web/src/components/terminal/useXterm.ts` exposes the measurement helper using the same xterm options as the live terminal:

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

### Preserve and replay resize messages emitted before WebSocket OPEN

`web/src/features/sessions/useTerminalSocket.ts` now stores the most recent resize when the socket is not yet open, and replays it after `socket.open` sends `hello`.

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

The important subtlety: do not clear `pendingResize` at the start of `connect()`. The live xterm can emit its first resize before `connect()` runs or before the socket reaches `OPEN`.

### Keep runtime size aligned with PTY initial size

The backend runtime should start with the same size used to create the PTY, so attach and resize deduplication compare against reality rather than the default `80x25`.

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

### Move noisy binary output diagnostics to debug

`terminal websocket binary sent` is high-volume output-path telemetry. Keep it available, but log it at debug level in `internal/transport/http/localapi/server.go`.

```go
s.logger.Debug("terminal websocket binary sent", "session_id", client.SessionID(), ...)
```

## Why This Works

Full-screen TUIs such as Claude Code render their first screen based on the PTY's current rows and columns. If the process starts at `173x24` while the browser terminal is actually `207x33`, the TUI can draw separators, prompt boxes, and cursor positions against the wrong grid before any later resize is processed.

The corrected flow makes all stages agree:

```text
xterm.measure ... cols=207 rows=33
session.create.request ... cols=207 rows=33
terminal pty start ... cols=207 rows=33
terminal attach start ... current_cols=207 current_rows=33
socket.control.send {type: 'resize', cols: 207, rows: 33}
terminal resize received ... cols=207 rows=33
```

This removes both failure modes:

1. The process starts with the same dimensions as the visible xterm grid.
2. If xterm still emits a resize before WebSocket OPEN, that resize is buffered and replayed instead of silently discarded.

## Prevention

- Treat terminal dimensions as measured state, not a viewport estimate. For xterm.js, use `FitAddon` against a DOM structure that matches the real terminal shell.
- When terminal resize events depend on WebSocket readiness, buffer the latest resize until the transport is open. Dropping the first resize is enough to break TUI applications.
- Keep diagnostics that compare each layer of the pipeline:

  ```text
  xterm.measure
  session.create.request
  terminal pty start
  terminal attach start
  socket.control.send resize
  terminal resize received
  terminal pty resize
  ```

- Add regression coverage for backend runtime initial size so attach state starts from the PTY size rather than `80x25`.
- Keep high-volume binary output logs at debug level; info logs should surface lifecycle and control-plane events, not every output chunk.

## Related Issues

- Requirement record: `docs/requirement/20260622-claude-tui-terminal-render-fix.md`
- Key files:
  - `web/src/App.vue`
  - `web/src/components/terminal/useXterm.ts`
  - `web/src/features/sessions/useTerminalSocket.ts`
  - `web/src/components/terminal/TerminalView.vue`
  - `internal/application/terminal/runtime.go`
  - `internal/transport/http/localapi/server.go`
