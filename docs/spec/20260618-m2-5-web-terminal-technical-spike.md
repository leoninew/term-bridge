# M2.5 Web Terminal Technical Spike 规格
最后修改时间: 2026-06-18 13:48:29

Review status: Accepted

## Requirement basis

本规格基于：

- `docs/requirement/20260618-m2-5-web-terminal-technical-spike.md`
- `docs/design.md`
- `docs/plan/20260617-roadmap-refresh.md`
- `docs/requirement/20260618-m3-session-workspace-runtime-model.md`
- 当前代码中的 M2/M3 runtime、Workspace / Session、state、history、PTY abstraction 和 CLI command model
- 用户关于 M2.5 的最新决策：`termbridge web`、`just web`、`TermBridge-go/web`、Vite + Vue 3 + TypeScript + reka-ui + Tailwind CSS、`@xterm/*`、左右分离 UI、Web 会话走 Workspace / Session 领域模型、detach / reattach 状态语义进入设计

M2.5 的定位是：

```text
Gate Web Terminal 的技术前身 / 可演进原型
```

它不是一次性 demo，也不是完整 Gateway 产品。M2.5 要交付可运行的本地 Web terminal 能力，同时把前端组件、WebSocket protocol、后端 relay abstraction 和 Workspace / Session 接入设计成未来 Gate 可继承、迁移或扩展的形态。

## Overview

M2.5 目标架构：

```text
Browser
  ↓
Vite + Vue 3 + TypeScript
  ↓
Terminal view based on @xterm/xterm
  ↓
WebSocket protocol
  ↓
termbridge web local server
  ↓
Web terminal relay / session registry
  ↓
Workspace / Session / History / State store
  ↓
PTY abstraction
  ↓
go-pty / Windows ConPTY
  ↓
user command
```

M2.5 不把 WebSocket 逻辑塞进现有 CLI runner。当前 `internal/runner.CommandRunner` 偏向本地 terminal：raw mode、OS signal、stdin reader、stdout writer、local terminal resize polling。Web terminal 需要显式的 browser input、resize、close、attach、detach、reconnect、client fan-out 和 backpressure 语义。因此 M2.5 设计新增 sibling relay 层，复用底层 runtime/domain 能力，而不是复用 CLI runner 的本地 terminal 假设。

核心分层：

```text
CLI command parsing
  ↓
app dispatch
  ↓
web server / HTTP API / WebSocket upgrade
  ↓
web terminal relay service
  ↓
PTY session runtime + Workspace/Session records
```

## Design decisions

### 1. `termbridge web` 是本地 Web terminal 子命令

M2.5 新增根命令：

```text
termbridge [options] web [web options]
```

初步语义：

```text
termbridge web
termbridge --cwd D:\project web
termbridge web --host 127.0.0.1 --port 0
termbridge web --open
```

根级 `--cwd` 仍表示 effective cwd，并决定默认 Workspace / state dir / config discovery。`web` 子命令只启动本地 Web terminal server，不直接等同于运行一个 command。命令运行由 Web UI 发起：新建 session 或打开现有 session。

`termbridge web` 与现有命令的关系：

| Command | 语义 |
| --- | --- |
| `termbridge exec -- <command...>` | 本地 CLI terminal 直接运行命令 |
| `termbridge workspace` | 列出 Workspace metadata |
| `termbridge session` | 列出 Session metadata |
| `termbridge web` | 启动本地 Web terminal server / Gate 前身原型 |

`termbridge web` 不替代 CLI-first；它是 Gate Web Terminal 的本地前身。

### 2. `just web` 是开发期便捷入口

新增开发入口：

```text
just web
```

Spec 建议 `just web` 编排：

```text
1. 启动 Go backend：go run cmd/termbridge/main.go web --dev
2. 启动 Vite dev server：web/package.json scripts.dev
3. Vite 通过 proxy 转发 /api 和 /ws 到 Go backend
```

如果 implementation 阶段为了简化只能先启动 Go backend 或只启动 Vite，需要在 Plan 中明确取舍；但目标设计是前后端开发体验一条命令启动。

### 3. 前端放入当前仓库 `web/`

M2.5 不复用 `D:\SourceCodes\mywork\TermBridge\web` 的实现，只复用其技术栈和经验。前端放入当前仓库：

```text
TermBridge-go/
  web/
    package.json
    vite.config.ts
    tsconfig.json
    src/
      main.ts
      App.vue
      components/
      features/
      protocol/
```

主流 Go + Vite monorepo 实践在 M2.5 中采用：

```text
Go root owns runtime and server
web/ owns Vite app
Vite dev server proxies API/WS to Go
production/prototype build emits web/dist
Go can later embed web/dist via go:embed
```

M2.5 初期可以先使用 Vite dev server + Go backend proxy，不要求立刻完成 production embed；但 Spec 保留未来 Gate 可继承路径。

### 4. 前端技术栈固定

前端技术栈：

```text
Vite
Vue 3
TypeScript
reka-ui
Tailwind CSS
```

xterm.js 使用官方 scoped packages：

```text
@xterm/xterm
@xterm/addon-fit
@xterm/addon-web-links
```

不使用旧的 unscoped `xterm` / `xterm-addon-*` 包。

Vue 封装建议：

```text
web/src/components/terminal/TerminalView.vue
web/src/components/terminal/useXterm.ts
web/src/features/sessions/useTerminalSocket.ts
web/src/protocol/terminal.ts
```

`TerminalView.vue` 负责 xterm lifecycle：

```text
onMounted:
  create Terminal
  load FitAddon
  load WebLinksAddon
  open(container)
  fit()
  bind onData / onBinary
  observe resize

onBeforeUnmount:
  dispose websocket subscriptions
  dispose ResizeObserver
  dispose xterm disposables
  dispose Terminal
```

要求：

- `Terminal` instance 不进入深层 Vue reactive object；使用 local variable 或 `shallowRef`。
- `ResizeObserver` + `FitAddon` 计算 cols/rows。
- 前端 resize throttle，并且只在 cols/rows 改变时发送。
- `onData` 处理普通输入，`onBinary` 预留 raw bytes 输入场景。
- PTY output 优先写入 `Uint8Array`，避免把终端 byte stream 过早文本化。

### 5. UI 采用左右分离结构

M2.5 页面不是完整 Workspace UI，但要验证未来 Gate 的核心信息架构。

布局：

```text
┌───────────────────────────────┬────────────────────────────────────┐
│ Workspace / Session sidebar    │ Terminal pane                      │
│                               │                                    │
│ Workspace A                    │ session status bar                 │
│   Session 1                    │ ─────────────────────────────────  │
│   Session 2                    │ xterm.js terminal                  │
│ Workspace B                    │                                    │
└───────────────────────────────┴────────────────────────────────────┘
```

左侧至少展示：

- Workspace name / path / key 或 id 的短显示。
- Workspace 下的 Session 列表。
- Session command。
- Session state。
- updated time 或 created time。

右侧至少展示：

- terminal。
- 当前 session id 短显示。
- cwd。
- command。
- state。
- exit code（如果已退出）。
- disconnect / detached / reconnecting / error 状态提示。

M2.5 不做完整 workspace dashboard，不做权限、设备、用户、Gate routing 信息。

### 6. 后端新增 Web terminal relay 层

新增后端包建议：

```text
internal/webserver       HTTP server, static serving, API routing, WebSocket upgrade
internal/webterminal     session registry, relay service, attach/detach lifecycle
internal/terminalproto   protocol message types and validation
```

命名可在 Plan 阶段微调，但职责边界必须保持：

- `webserver` 不直接读写 PTY。
- `webterminal` 不处理 CLI parsing。
- `terminalproto` 不依赖 HTTP framework。
- PTY 仍通过 `internal/pty` abstraction。
- Workspace / Session 仍通过 `internal/workspace`、`internal/session`、`internal/state`。
- History 仍通过 `internal/history` 或等价 bounded history abstraction。

后端不得绕过 M3 runtime model 自己另建 session 概念。

### 7. Web 会话必须走 Workspace / Session 领域模型

Web 新建会话时必须：

```text
resolve effective cwd
resolve/create Workspace
create Session record
write state=starting
start PTY
write process.json
write state=running
write history.log while output flows
write exit.json on process exit
write state=stopped/failed according to result
```

打开现有会话时分三类：

| Session 类型 | 行为 |
| --- | --- |
| 当前 web server 进程持有 live PTY handle | 可 attach / reattach |
| 已停止或失败的历史 session | 展示 metadata + bounded history，不可交互 |
| 持久化记录显示 running 但当前进程没有 PTY handle | 通过 recovery 标记或展示为不可 attach，不能假装已恢复 PTY |

这点很关键：M3 的文件系统记录能恢复 metadata 和进程状态识别，但不能恢复旧进程的 ConPTY handle。M2.5 可以设计未来 Gate / daemon 的 reattach 语义，但 implementation 不能虚假承诺跨进程 PTY reattach。

### 8. 新建会话与打开现有会话 API

HTTP API 初稿：

```text
GET  /api/workspaces
GET  /api/sessions
POST /api/sessions
GET  /api/sessions/{session_id}
GET  /api/sessions/{session_id}/history
GET  /api/sessions/{session_id}/ws
```

`POST /api/sessions` request：

```json
{
  "cwd": "D:\\project",
  "command": ["pwsh", "-NoLogo"],
  "cols": 120,
  "rows": 32
}
```

response：

```json
{
  "session_id": "01J...",
  "workspace_id": "01J...",
  "workspace_key": "...",
  "state": "starting",
  "ws_url": "/api/sessions/01J.../ws"
}
```

`GET /api/sessions/{session_id}/ws`：

- 若 session live 且 attachable，升级 WebSocket。
- 若 session stopped/failed，返回不可 attach 的错误，同时 UI 可展示 history。
- 若 session metadata running 但不可 attach，返回明确错误并触发 recovery / refresh。

### 9. WebSocket protocol 使用 JSON control + binary PTY stream

协议名称建议：

```text
Sec-WebSocket-Protocol: termbridge.terminal.v1
```

消息策略：

- PTY bytes 使用 binary frames。
- control / state / error 使用 JSON text frames。
- 不使用 base64 包裹 PTY output，除非后续发现框架限制。

JSON control message 初稿：

Client → Server：

```json
{ "type": "hello", "last_seq": 123 }
{ "type": "resize", "cols": 120, "rows": 32 }
{ "type": "detach" }
{ "type": "close" }
{ "type": "ping", "nonce": "..." }
```

Server → Client：

```json
{ "type": "started", "session_id": "01J...", "workspace_id": "01J...", "state": "running" }
{ "type": "state", "state": "detached", "reason": "client_disconnected" }
{ "type": "exited", "exit_code": 0, "state": "stopped" }
{ "type": "error", "code": "session_not_attachable", "message": "..." }
{ "type": "pong", "nonce": "..." }
```

Binary frames：

```text
Client -> Server: stdin bytes
Server -> Client: PTY output bytes
```

M2.5 可以在 Spec/Plan 中选择“纯 binary payload 无 opcode”，因为 WebSocket direction 已能区分 input/output；若未来 Gate 需要 multiplex，可升级为：

```text
[1 byte opcode][payload bytes]
```

但 M2.5 必须在协议文档中说明兼容策略。

### 10. WebSocket validation

后端必须限制：

- JSON message size。
- `cols` / `rows` 合理范围，例如 cols 1..500，rows 1..200。
- unknown `type` 拒绝或明确 error。
- `close` 表示关闭/终止 session，不提供“关闭但 detach”的模式。
- `detach` 是独立显式消息，或由浏览器断连 / 刷新触发的 attachment lifecycle 语义。
- binary input size 单帧上限。

M2.5 不做 auth，但仍应避免本地开发 server 被任意远程访问：默认 bind `127.0.0.1`。

### 11. PTY output fan-out 与慢客户端策略

后端 output path：

```text
PTY reader goroutine
  ↓
append to bounded history writer / session output journal
  ↓
publish immutable output chunks to session hub
  ↓
per-client bounded queue
  ↓
client writer goroutine
  ↓
WebSocket binary frames
```

原则：

1. PTY reader 不直接写 WebSocket。
2. PTY reader 不被某个慢客户端阻塞。
3. 每个 client 只有一个 WebSocket writer goroutine。
4. 每个 client outbound queue 有上限。
5. queue 满时优先 detach / disconnect 慢客户端，而不是阻塞 PTY reader 或静默丢 terminal bytes。
6. 终端字节流不能随意 drop，否则前端 screen state 会损坏。
7. 若必须丢弃，应发送 reset/error，让客户端知道 terminal state 不可信。

M2.5 初始策略：

```text
client outbound queue full
  -> mark client slow
  -> detach client
  -> close websocket with policy reason
  -> session continues running if process still alive
```

### 12. History 接入

Web terminal output 必须写入 M3 bounded history。

现有 history writer 是同步 file-backed writer。Web relay 不能简单使用：

```go
io.MultiWriter(historyWriter, websocketWriter)
```

否则慢 WebSocket 会影响 PTY reader。正确方向是：

```text
PTY output chunk
  -> history write path
  -> session hub publish path
```

如果 history write 本身成为阻塞点，M2.5 先记录风险；M5 再做更强 backpressure / async persistence hardening。

### 13. Detach / reattach 状态语义

当前 M3 状态：

```text
starting
running
stopping
stopped
failed
```

M2.5 需要表达 client attachment lifecycle。直接把 `detached` 加入 process lifecycle state 会混淆“进程是否运行”和“有没有客户端 attached”。因此 Spec 建议：

```text
Process/session lifecycle state:
  starting | running | stopping | stopped | failed

Attachment lifecycle state:
  unattached | attached | detached | reattaching
```

其中：

- `running + attached`：进程运行，至少一个 Web client attached。
- `running + detached`：进程运行，但没有 Web client attached。
- `running + reattaching`：客户端正在重新建立连接。
- `stopped + detached` 不作为有效 active state；stopped session 只能查看 history/metadata。

M2.5 可以把 attachment state 先放在 Web terminal runtime registry 中，同时在 session metadata 或 state extension 中记录 last attach/detach 时间。是否扩展持久化 schema 在 Plan 阶段决定。

### 14. Disconnect / close / reconnect 语义

WebSocket 断开分三类：

| 事件 | 语义 | 结果 |
| --- | --- | --- |
| 浏览器刷新 / 网络断开 | detach | session process 继续运行，attachment state = detached |
| UI 显式点击 detach | detach | session process 继续运行，attachment state = detached |
| UI 点击关闭 terminal | close / terminate | soft interrupt / close / kill 策略进入 stopping |

关闭 terminal 在 UI 语义上就是关闭/终止，不再提供“关闭但 detach”的二义性选项，避免用户认知负担。detach 必须作为独立动作展示，或作为浏览器断连/刷新后的连接状态结果。

M2.5 不承诺跨进程 reattach，但同一 `termbridge web` server 进程内必须支持：

```text
client disconnect
  ↓
session remains running
  ↓
client opens existing session
  ↓
reattach to in-memory PTY session
```

如果 `termbridge web` server 自身退出，旧 ConPTY handle 不可恢复；下一次启动只能读取 metadata/history，并通过 recovery 标记状态。这一点必须在 UI 和文档里明确。

### 15. Resize 策略

前端：

```text
ResizeObserver detects container change
  ↓
FitAddon.fit()
  ↓
read terminal.cols / terminal.rows
  ↓
throttle 100ms
  ↓
send resize message only if cols/rows changed
```

后端：

```text
receive resize
  ↓
validate cols/rows
  ↓
ignore duplicate size
  ↓
session.Resize(cols, rows)
```

M2.5 不要求持久化每个 resize event；只需要当前 live PTY 正确 resize。

### 16. Frontend state model

前端主要状态：

```ts
type WorkspaceView = {
  id: string
  key: string
  name: string
  path: string
  sessions: SessionSummary[]
}

type SessionSummary = {
  id: string
  workspaceId: string
  command: string
  cwd: string
  lifecycleState: 'starting' | 'running' | 'stopping' | 'stopped' | 'failed'
  attachmentState?: 'unattached' | 'attached' | 'detached' | 'reattaching'
  exitCode?: number
  updatedAt: string
}
```

前端最小交互：

- 选择 Workspace。
- 查看 Workspace 下 Session。
- 新建 Session。
- 打开 attachable Session。
- 查看 stopped Session metadata/history。
- terminal pane 显示当前 session。
- detach / close 操作明确区分；关闭就是关闭/终止，不承载 detach 语义。

### 17. Config 设计

M2.5 初期建议使用 CLI flags 而不是立即扩展 `.termbridge.yaml`，避免 config schema 过早膨胀。

可选 web flags：

```text
termbridge web --host 127.0.0.1 --port 0 --open
```

默认值：

```text
host: 127.0.0.1
port: 0 or 8080（Plan 阶段决定）
open: false
```

如果 Plan 阶段决定加入 config keys，例如：

```yaml
web:
  host: 127.0.0.1
  port: 8080
  open: false
```

则必须同步：

- `internal/config/termbridge.default.yaml`
- `configs/termbridge.default.yaml`
- `internal/config/config.go` unknown key allowlist
- config tests

### 18. Static assets and dev/prototype serving

M2.5 建议分两种模式：

#### dev mode

```text
Vite dev server: http://localhost:5173
Go backend:       http://127.0.0.1:<port>
Vite proxy:
  /api -> Go backend
  /ws  -> Go backend
```

#### built mode（可选，Plan 阶段确定是否实现）

```text
web/dist
  ↓
go:embed
  ↓
termbridge web serves static assets and API/WS from one origin
```

M2.5 至少需要 dev mode。若 built mode 不实现，必须在 Plan / Verification 中记录为未完成项。

## Affected components

### Existing components to update

```text
cmd/termbridge/main.go
internal/cli/cli.go
internal/cli/cli_test.go
internal/app/app.go
internal/app/app_test.go
internal/config/config.go（仅当新增 web config keys）
internal/session/state.go（若扩展持久状态）
internal/state/store.go（若新增 attachment metadata 持久文件）
justfile
```

### Existing components to reuse

```text
internal/process
internal/pty
internal/pty/gopty
internal/workspace
internal/session
internal/state
internal/history
internal/logging
internal/config
```

### New backend components

```text
internal/webserver
internal/webterminal
internal/terminalproto
```

### New frontend components

```text
web/package.json
web/vite.config.ts
web/tsconfig.json
web/src/main.ts
web/src/App.vue
web/src/components/terminal/TerminalView.vue
web/src/components/workspace/WorkspaceSessionSidebar.vue
web/src/features/sessions/*
web/src/protocol/terminal.ts
```

## Interfaces

### CLI options draft

```text
termbridge [root options] web [web options]

Root options:
  --cwd <dir>

Web options:
  --host <host>      default 127.0.0.1
  --port <port>      default 0 or 8080
  --open             open browser after server starts
  --dev              enable dev-server friendly behavior
```

### HTTP API draft

```text
GET  /api/health
GET  /api/workspaces
GET  /api/sessions
POST /api/sessions
GET  /api/sessions/{session_id}
GET  /api/sessions/{session_id}/history
GET  /api/sessions/{session_id}/ws
```

### WebSocket control message draft

```ts
type ClientControlMessage =
  | { type: 'hello'; lastSeq?: number }
  | { type: 'resize'; cols: number; rows: number }
  | { type: 'detach' }
  | { type: 'close' }
  | { type: 'ping'; nonce: string }

type ServerControlMessage =
  | { type: 'started'; sessionId: string; workspaceId: string; state: string }
  | { type: 'state'; lifecycleState: string; attachmentState?: string; reason?: string }
  | { type: 'exited'; exitCode: number; state: 'stopped' | 'failed' }
  | { type: 'error'; code: string; message: string }
  | { type: 'pong'; nonce: string }
```

Binary frames carry terminal bytes.

## Technical questions for Plan

1. `web` command flags 是否先只做 host/port/open/dev，还是需要默认 command/cwd 参数？
2. backend WebSocket library 选择标准库 + `nhooyr.io/websocket`、`gorilla/websocket`，还是其他？需要在 Plan 阶段结合维护状态和 API 选择。
3. 是否在 M2.5 实现 built mode `go:embed web/dist`，还是仅 dev mode？
4. Attachment state 是否持久化到文件，还是只存在内存 registry？
5. stopped session history API 是否直接读取 `history.log`，还是通过 history package 暴露？
6. 是否支持多 client attach 同一个 live session？若不支持，第二个 client 是拒绝、抢占，还是只读 viewer？
7. slow client queue 的字节上限、消息上限和 write deadline 具体取值。
8. WebSocket binary frame 是否需要 opcode prefix，还是 v1 先按方向区分。
9. 是否需要前后端共享 schema 生成，还是先手写 Go/TS 两份类型并靠测试保持一致。
10. 是否要扩展 `session.State`，还是单独定义 `AttachmentState`。

## Risks

### 风险一：把 Web terminal 写成独立 runtime

如果 Web terminal 直接创建 PTY、写自己的 session/history/state，将破坏 M3 领域模型，也会让未来 Gate 继承困难。后端必须复用现有 Workspace / Session / State / History / PTY abstraction。

### 风险二：误承诺跨进程 reattach

当前文件系统 state 不能恢复 ConPTY handle。M2.5 可以支持同一 `termbridge web` server 进程内 reattach，但不能承诺 server 退出后仍能重新 attach 到旧 PTY。

### 风险三：慢客户端阻塞 PTY reader

终端输出必须先 drain PTY，再 fan-out。任何直接从 PTY reader 写 WebSocket 的实现都可能在慢客户端下卡死 session。

### 风险四：terminal bytes 被错误文本化

PTY 是 byte stream。若全部塞进 JSON UTF-8 string，可能遇到控制序列、编码、分片、多字节边界问题。Spec 建议 binary frame 承载 terminal bytes。

### 风险五：状态模型混淆

`running` 表示进程运行，不等于 browser attached。M2.5 必须区分 lifecycle state 和 attachment state，否则 detach/reattach 会污染 process lifecycle。

### 风险六：Scope 过大

左右分离 UI、新建/打开 session、attach/detach、history、WebSocket、xterm、前端工程化都进入 M2.5，Plan 阶段需要切出可交付的最小实现顺序，避免一次性铺太大。

## Alternatives

### Alternative A: 只做静态 HTML + WebSocket demo

拒绝。用户已明确 M2.5 是 Gate 前身，不是一次性 spike；前端应基于 Vite + Vue 3 + TypeScript + reka-ui + Tailwind CSS，并按 Gate 可复用方式设计。

### Alternative B: 复用旧 `D:\SourceCodes\mywork\TermBridge\web` 实现

拒绝。用户明确不复用旧实现，只复用技术栈和经验。新前端应放入当前仓库并面向 Go runtime / Gate 演进。

### Alternative C: 直接用 `runner.CommandRunner` 包一层 WebSocket

拒绝作为主方案。`CommandRunner` 有本地 terminal assumptions，不适合 browser input/resize/attach/detach。可以复用其思路和底层 packages，但 Web relay 应作为 sibling runtime orchestration。

### Alternative D: WebSocket 全部使用 JSON + base64 output

不推荐。base64 增加开销，且终端输出本质是 byte stream。M2.5 使用 binary frames for terminal bytes、JSON for control。

### Alternative E: disconnect 默认 terminate process

拒绝作为目标语义。用户已明确 detach / reattach 总归要实现。M2.5 应至少在同一 web server 进程内支持 detach 后重新打开 existing session。

## User review notes

用户接受其他设计点，并纠正 close/detach 交互语义：UI 点击关闭 terminal 就是关闭/终止，不再提供“关闭但 detach”的选项；detach 应作为独立显式动作，或作为浏览器断连/刷新后的 attachment lifecycle 语义。
