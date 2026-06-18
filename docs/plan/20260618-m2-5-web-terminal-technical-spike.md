# M2.5 Web Terminal Technical Spike 计划
最后修改时间: 2026-06-18 14:00:24

Review status: Accepted

## Plan basis

本计划基于：

- `docs/requirement/20260618-m2-5-web-terminal-technical-spike.md`（Accepted）
- `docs/spec/20260618-m2-5-web-terminal-technical-spike.md`（Accepted）
- 现有 M2/M3 代码边界：`internal/cli`、`internal/app`、`internal/runner`、`internal/pty`、`internal/workspace`、`internal/session`、`internal/state`、`internal/history`

M2.5 目标是交付 Gate Web Terminal 的本地前身：`termbridge web` + `just web`、`TermBridge-go/web` 前端、WebSocket relay、Workspace/Session 接入、可新建/打开会话、同进程 detach/reattach、bounded fan-out。

## Implementation strategy

按“先立后端领域边界，再接 CLI，再建前端，再连通端到端”的顺序实施，避免先写 UI 后补 runtime 导致第二套 Agent。

## Implementation steps

### Step 1: 引入 Web command parsing

修改：

```text
internal/cli/cli.go
internal/cli/cli_test.go
internal/app/app.go
internal/app/app_test.go
```

目标：

- 新增 `web` 根命令。
- 支持 web options：

  ```text
  --host <host> default 127.0.0.1
  --port <port> default 0
  --open
  --dev
  ```

- 保持旧入口 `termbridge -- <command>` 继续拒绝。
- `exec` / `workspace` / `session` 行为不变。

### Step 2: 新增 terminal protocol package

新增：

```text
internal/terminalproto
```

内容：

- JSON control message structs。
- message type constants：`hello`、`resize`、`detach`、`close`、`ping`、`started`、`state`、`exited`、`error`、`pong`。
- validation：message size、cols/rows range、known type、close/detach 语义。
- subprotocol constant：`termbridge.terminal.v1`。

决策：

- terminal bytes 使用 binary WebSocket frames。
- JSON control 使用 text frames。
- v1 不加 binary opcode prefix，按 direction 区分 stdin/output；在文档中保留 future Gate multiplex 升级路径。

### Step 3: 新增 Web terminal runtime / relay 层

新增：

```text
internal/webterminal
```

职责：

- live session registry。
- 新建 Web session：复用 Workspace / Session / State / History / PTY abstraction。
- 打开 existing session：
  - live handle 存在：attach。
  - stopped/failed：metadata + history only。
  - metadata running 但无 live handle：不可 attach，触发 recovery/错误说明。
- attachment state：`unattached`、`attached`、`detached`、`reattaching`。
- close/detach 区分：
  - detach：只断开 client，process 继续。
  - close：关闭/终止 session，进入 stopping。

关键结构：

```text
Registry
SessionRuntime
Client
Hub
AttachmentState
```

### Step 4: 实现 PTY output fan-out

在 `internal/webterminal` 中实现：

```text
PTY reader
  -> history writer
  -> hub publish
  -> per-client bounded queue
  -> client writer
```

要求：

- PTY reader 不直接写 WebSocket。
- 每个 client 一个 writer goroutine。
- queue bounded。
- queue 满时 detach/close client，不阻塞 PTY reader，不静默丢 terminal bytes。
- history 写入失败要进入 error/failed 路径或记录 warning；M2.5 不做完整 async persistence hardening。

### Step 5: 新增 Web server package

新增：

```text
internal/webserver
```

职责：

- HTTP server lifecycle。
- API routes：

  ```text
  GET  /api/health
  GET  /api/workspaces
  GET  /api/sessions
  POST /api/sessions
  GET  /api/sessions/{session_id}
  GET  /api/sessions/{session_id}/history
  GET  /api/sessions/{session_id}/ws
  ```

- WebSocket upgrade。
- 默认 bind `127.0.0.1`。
- dev mode 支持 Vite proxy。
- built static serving 可先不实现，若不实现需在 Verification 记录。

WebSocket library 在实现阶段优先选择维护活跃、API 简洁的库；候选：

```text
nhooyr.io/websocket
github.com/gorilla/websocket
```

Plan 建议偏向 `nhooyr.io/websocket`（context-first API），但实施前需确认 Go 1.25 兼容性和维护状态。

### Step 6: app dispatch 接入 web server

修改：

```text
internal/app/app.go
```

新增：

```text
CommandWeb
WebCommand options
runWeb(...)
```

`runWeb` 负责：

- 加载 config。
- 初始化 logger。
- 构建 state store。
- 构建 webterminal registry/service。
- 启动 webserver。
- 输出 local URL。
- 根据 `--open` 决定是否打开浏览器。

### Step 7: 前端工程初始化

新增：

```text
web/package.json
web/vite.config.ts
web/tsconfig.json
web/src/main.ts
web/src/App.vue
web/src/styles.css
```

依赖：

```text
vue
vite
typescript
@vitejs/plugin-vue
@tailwindcss/vite
tailwindcss
reka-ui
@xterm/xterm
@xterm/addon-fit
@xterm/addon-web-links
vue-tsc
eslint/prettier（按项目可用性添加）
```

Vite dev proxy：

```text
/api -> Go backend
/ws or /api/.../ws -> Go backend
```

### Step 8: 前端 terminal component

新增：

```text
web/src/components/terminal/TerminalView.vue
web/src/components/terminal/useXterm.ts
web/src/features/sessions/useTerminalSocket.ts
web/src/protocol/terminal.ts
```

实现：

- xterm lifecycle。
- FitAddon + ResizeObserver。
- 100ms resize throttle。
- `onData` / `onBinary` → WebSocket binary input。
- WebSocket binary output → `terminal.write(Uint8Array)`。
- JSON control handling：started/state/exited/error/pong。

### Step 9: 前端 Workspace / Session 两级 UI

新增：

```text
web/src/components/workspace/WorkspaceSessionSidebar.vue
web/src/features/workspaces/*
web/src/features/sessions/*
```

最小 UI：

- 左侧 Workspace list。
- Workspace 下 Session list。
- 新建 Session form：cwd、command。
- 打开 attachable session。
- stopped/failed session 展示 metadata/history。
- 右侧 terminal pane。
- detach 与 close 分开；close 就是关闭/终止。

### Step 10: justfile 接入

修改：

```text
justfile
```

新增：

```text
web:
    # 启动 Go backend + Vite dev server
```

Windows PowerShell 下可先采用两个命令说明或并行脚本；若 `just` 并发编排复杂，Plan 允许先提供：

```text
just web-backend
just web-frontend
```

再让 `just web` 调用项目脚本。

### Step 11: 测试与文档补充

测试：

- CLI parse tests：`web` command / flags。
- app dispatch tests：web command calls server runner。
- terminalproto validation tests。
- webterminal registry tests：new session、attach live、detach、close、slow client queue full。
- API handler tests：health、list sessions、create session、history。
- 前端 typecheck / build。

文档：

- 不更新 README。
- Plan/Verification 记录如何启动和验证。

## Files to change

### Existing files

```text
internal/cli/cli.go
internal/cli/cli_test.go
internal/app/app.go
internal/app/app_test.go
justfile
go.mod
go.sum
```

可能修改：

```text
internal/session/state.go
internal/state/store.go
internal/config/config.go
internal/config/termbridge.default.yaml
configs/termbridge.default.yaml
```

只有在实现决定持久化 attachment state 或添加 web config keys 时才修改这些文件。

### New backend files

```text
internal/terminalproto/*.go
internal/terminalproto/*_test.go
internal/webterminal/*.go
internal/webterminal/*_test.go
internal/webserver/*.go
internal/webserver/*_test.go
```

### New frontend files

```text
web/package.json
web/vite.config.ts
web/tsconfig.json
web/src/main.ts
web/src/App.vue
web/src/styles.css
web/src/protocol/terminal.ts
web/src/components/terminal/TerminalView.vue
web/src/components/terminal/useXterm.ts
web/src/components/workspace/WorkspaceSessionSidebar.vue
web/src/features/sessions/*
web/src/features/workspaces/*
```

## Verification plan

用户会自行做真实交互验证；实现阶段仍需提供可运行检查。

### Automated checks

```powershell
go test ./...
just check
```

如果新增前端：

```powershell
cd web
npm install
npm run typecheck
npm run build
```

或通过 root `just` 增加：

```text
just web-check
```

### Manual smoke commands for user

```powershell
go run cmd/termbridge/main.go web --dev
just web
```

浏览器验证：

- 打开 local URL。
- 新建 `pwsh -NoLogo` session。
- 新建 `claude` session。
- detach 后从左侧 session 列表重新打开。
- close 后确认 session stopped / exited。
- resize 浏览器窗口。
- 输出大量文本。

## Rollback plan

- 若 web command parsing 破坏 CLI，可回退 `CommandWeb` 相关解析，保留 backend/frontend 文件未接入。
- 若 relay 层不稳定，不改动现有 `exec` runner；Web 是 sibling path，可隔离失败。
- 若前端构建复杂度过高，先保留 Go API/WS 和最小 Vite shell，再补 UI。
- 若 attachment state 持久化设计不稳，先限制为内存 registry，并在 UI/Verification 明确 server 重启不可 reattach。

## Assumptions

- M2.5 只要求同一 `termbridge web` 进程内 reattach；server 重启后不可恢复 ConPTY handle。
- `close` 就是关闭/终止，不提供“关闭但 detach”。
- `detach` 是独立动作或浏览器断连结果。
- 默认 bind `127.0.0.1`。
- README 不更新。
- 前端使用 npm 或当前环境可用的 JS 包管理器；具体 lockfile 由实现时环境决定。

## Risks

1. Scope 较大：CLI、Go server、relay、PTY、history、frontend 都会触达，需要按步骤小步验证。
2. WebSocket 库选择会影响 API 设计和 backpressure 实现。
3. Web terminal 不能跨进程 reattach，必须在 UI 和文档中避免误导。
4. history writer 同步写入可能成为 PTY reader 阻塞点；M2.5 记录风险，M5 hardening 深化。
5. 前端依赖引入会增加仓库工具链复杂度。
6. Windows 下并发启动 Go backend + Vite dev server 的 `just web` 编排需要谨慎处理。

## User review notes

待用户 review。
