# M2.5 Web Terminal Technical Spike 验证
最后修改时间: 2026-06-18 15:13:36

Review status: Accepted

## Verification basis

本验证基于：

- `docs/requirement/20260618-m2-5-web-terminal-technical-spike.md`
- `docs/spec/20260618-m2-5-web-terminal-technical-spike.md`
- `docs/plan/20260618-m2-5-web-terminal-technical-spike.md`
- 当前工作区实际 diff
- 用户在实现后追加的工程化要求：参考 `D:\SourceCodes\mywork\TermBridge\web` 对齐前端依赖版本、Tailwind 配置、ESLint / Prettier、Yarn、`justfile`，并要求 Vue SFC 中 `template` 在上、`script` 在下且内容缩进

当前流程：严格模式 / strict，验证 / Verification。

用户此前明确要求无须继续 `compound-engineering` review；本验证不调用 compound-engineering agent。

## Requirement alignment

| Requirement acceptance item | Result | Notes |
| --- | --- | --- |
| `termbridge web` 子命令启动 Web terminal 原型 | Pass | 已在 CLI/app 增加 `web` command 与 server dispatch。 |
| `just web` 开发期入口 | Pass | 已保留 `web-backend`、`web-frontend`、`web` 开发入口。 |
| 前端位于当前仓库 `web/`，使用 Vite/Vue/TypeScript/reka-ui/Tailwind | Pass | 已新增 `web/` 工程，并按参考项目补齐 Tailwind config、ESLint、Prettier、Yarn lock。 |
| 使用官方 `@xterm/*` packages | Pass | `web/package.json` 使用 `@xterm/xterm`、`@xterm/addon-fit`、`@xterm/addon-web-links`，并固定版本。 |
| xterm.js 封装为 Vue component | Pass | 已新增 `TerminalView.vue` 和 `useXterm.ts`。 |
| WebSocket 连接后端并桥接 PTY stdout/input | Partial | 后端使用 WebSocket binary frames；前端建立 socket 并把 xterm input 写入 WebSocket。真实浏览器交互由用户自行验证。 |
| browser resize 传递到 PTY resize，前端 throttle 后端去重 | Pass | 前端 `ResizeObserver` + 100ms throttle；后端 validate/deduplicate 后调用 PTY resize。 |
| 可新建会话和打开现有会话 | Partial | 新建 live session 和 attach live session 已实现；stopped/failed history display 已提供基础 API/UI。跨进程 reattach 不承诺。 |
| Web 会话走 Workspace / Session / History / State 模型 | Pass | 后端 `webterminal.Registry` 复用 `workspace.Resolver`、`session.Manager`、`state.Store`、`history.Writer`。 |
| 页面左右分离 | Pass | 左侧 Workspace/Session sidebar，右侧 terminal/history pane。 |
| detach / reattach 状态语义 | Partial | 同进程 detach/reattach 用内存 registry 与 attachment state 实现；未持久化 attachment state。 |
| 慢客户端不阻塞 PTY reader | Partial | 后端采用 per-client bounded queue，queue full detach client；history writer 仍为同步路径，符合 M2.5 已知风险。 |
| 不破坏现有 CLI 行为 | Pass | `just check` 通过，覆盖 Go fmt/vet/golangci-lint/test 与前端 typecheck/lint/format/build。 |
| 前端工程化参考既有 `TermBridge/web` 技术栈和经验 | Pass | 已对齐固定依赖版本、Tailwind v4 配置、ESLint flat config、Prettier、Yarn、Vue SFC block order / indentation。 |

## Spec alignment

### 已对齐

- 新增 sibling packages：
  - `internal/terminalproto`
  - `internal/webterminal`
  - `internal/webserver`
- WebSocket protocol 采用：
  - text JSON control frames
  - binary terminal byte frames
  - `termbridge.terminal.v1` subprotocol
- `close` 与 `detach` 是不同 control messages。
- backend 默认 host 为 `127.0.0.1`，port 默认 `0`。
- dev mode 保留 Vite proxy 方向，不实现完整 production embed。
- Web terminal 没有直接复用 `runner.CommandRunner`，而是复用更底层的 PTY/session/state/history/domain package。
- Attachment state 未污染 `session.State` lifecycle enum。
- 前端目录为 `web/`，采用 Vite + Vue 3 + TypeScript + Tailwind CSS + reka-ui dependency。
- Vue SFC 已按用户要求调整为 `template` 在上、`script` 在下，并通过 Prettier `vueIndentScriptAndStyle` 与 ESLint 规则约束。

### 实现中调整

- Plan 原本建议优先评估 `nhooyr.io/websocket`。实现初期使用该库后，`golangci-lint` 暴露维护方迁移提示；当前已切换到维护方 `github.com/coder/websocket v1.8.15`，避免引入 deprecated import。
- Plan 中前端 install/check 初稿仍写 npm 或 `web-check` 方向；根据用户反馈，当前已统一为 Yarn，并把前后端依赖安装合并进 root `install`，把前后端 lint/format/test/build 收敛进 root `check`。
- 因 `web/node_modules` 中存在第三方 Go package，root Go checks 不再使用 `go test ./...`，而显式限定 `./cmd/... ./internal/...`，避免误扫前端依赖目录。

### 部分对齐 / 保留风险

- `hello.last_seq` 暂未用于 replay 或 ack；M2.5 只保留字段形态。
- stopped/failed session history API 为基础文本读取，不是完整 terminal replay。
- 多浏览器 attach 未做产品化约束；当前 registry 允许多个 client attach 同一 runtime。
- built mode / `go:embed web/dist` 未实现。
- 前端 API/WS response runtime validation 较薄，主要依赖后端协议与 TypeScript 类型。
- command input 目前是简单 whitespace split，复杂 quoted argv 仍可能需要后续 UI 改进。

## Plan alignment

| Plan step | Result | Notes |
| --- | --- | --- |
| Step 1 Web command parsing | Pass | CLI/app 增加 `web`、`--host`、`--port`、`--open`、`--dev`。 |
| Step 2 `internal/terminalproto` | Pass | 已新增 protocol constants、message structs、validation、tests。 |
| Step 3 `internal/webterminal` | Pass | 已新增 registry/runtime/client/fan-out/attachment state。 |
| Step 4 PTY output fan-out | Pass with risk | PTY reader → history writer → client queue；history write 同步风险保留。 |
| Step 5 `internal/webserver` | Pass | 已新增 HTTP routes 和 WebSocket handling；WebSocket library 已由 `nhooyr` 调整为维护方 `coder/websocket`。 |
| Step 6 app dispatch | Pass | `runWeb` 构建 registry/server 并输出 URL。 |
| Step 7 frontend project | Pass | 已新增 Vite/Vue/TS/Tailwind 工程文件，并补齐 Tailwind config、ESLint、Prettier、Yarn lock。 |
| Step 8 terminal component | Pass | 已新增 xterm component、socket composable、protocol TS types。 |
| Step 9 Workspace/Session UI | Pass | 已新增 sidebar、新建 session form、history panel。 |
| Step 10 justfile | Pass | 已收敛为 root `install` / `check` 加开发入口 `web-backend`、`web-frontend`、`web`。 |
| Step 11 tests/docs | Partial | Go checks、frontend typecheck/lint/format/build 已通过；真实交互验证由用户执行。 |

## Actual diff summary

### Existing files changed

- `.gitignore`
  - 忽略 frontend generated artifacts，包括 `web/node_modules/` 与 `web/dist/`。
- `docs/plan/20260618-m2-5-web-terminal-technical-spike.md`
  - Plan 已标记 `Accepted`。
- `go.mod` / `go.sum`
  - 新增 WebSocket dependency；当前使用 `github.com/coder/websocket v1.8.15`。
- `internal/cli/cli.go` / `internal/cli/cli_test.go`
  - 新增 `web` command parsing、help、tests。
- `internal/app/app.go` / `internal/app/app_test.go`
  - 新增 web dispatch、server runner injection、tests。
- `internal/session/recovery_windows.go`
  - 修复 `windows.CloseHandle` 返回值未处理的 lint 问题。
- `justfile`
  - `install` 统一处理 Go 与 frontend dependencies。
  - `check` 统一处理 backend fmt/vet/golangci-lint/test 和 frontend format/typecheck/lint/build。
  - 保留 `web-backend`、`web-frontend`、`web` 开发入口。

### New backend files

- `internal/terminalproto/protocol.go`
- `internal/terminalproto/protocol_test.go`
- `internal/webterminal/registry.go`
- `internal/webterminal/runtime.go`
- `internal/webterminal/registry_test.go`
- `internal/webserver/server.go`
- `internal/webserver/server_test.go`

### New frontend files

- `web/index.html`
- `web/package.json`
- `web/yarn.lock`
- `web/tsconfig.json`
- `web/tsconfig.node.json`
- `web/vite.config.ts`
- `web/tailwind.config.ts`
- `web/eslint.config.js`
- `web/.prettierrc.json`
- `web/.prettierignore`
- `web/src/main.ts`
- `web/src/App.vue`
- `web/src/styles.css`
- `web/src/env.d.ts`
- `web/src/protocol/terminal.ts`
- `web/src/components/terminal/TerminalView.vue`
- `web/src/components/terminal/useXterm.ts`
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
- `web/src/features/sessions/api.ts`
- `web/src/features/sessions/useTerminalSocket.ts`
- `web/src/features/workspaces/api.ts`

### Existing staged docs from previous phases

当前 git status 中还包含本次 SpecFlow 阶段文档：

- `docs/requirement/20260618-m2-5-web-terminal-technical-spike.md`
- `docs/spec/20260618-m2-5-web-terminal-technical-spike.md`
- `docs/plan/20260618-m2-5-web-terminal-technical-spike.md`
- `docs/verification/20260618-m2-5-web-terminal-technical-spike.md`
- `docs/requirement/20260618-m4-local-product-surface.md`

其中 M4 requirement 是此前暂停的草稿文档，不属于 M2.5 runtime 实现本身；如果用户希望 commit 边界更干净，建议提交前单独处理或拆分。

## Expected vs actual changed files

### Expected by Plan

Plan 预计触达：

- CLI/app files
- `go.mod` / `go.sum`
- `justfile`
- `internal/terminalproto/*`
- `internal/webterminal/*`
- `internal/webserver/*`
- `web/*`

### Actual

实际改动与 Plan 主体一致。额外/需注意项：

1. `.gitignore` 被更新，用于排除前端 generated artifacts，属于实现过程中必要工程化补充。
2. `web/yarn.lock` 由 Yarn 生成，取代早期 npm lockfile，符合用户“使用 yarn”的要求。
3. `web/dist/` 和 `web/node_modules/` 已通过 `.gitignore` 排除，不进入 git status。
4. `docs/requirement/20260618-m4-local-product-surface.md` 仍出现在工作区，属于前置阶段遗留文档，不属于 M2.5 代码实现。
5. `internal/session/recovery_windows.go` 的 lint 修复由 root `check` 纳入 `golangci-lint` 后暴露，属于为了让项目约定检查通过的工程质量修复。
6. WebSocket library 从 `nhooyr.io/websocket` 调整为 `github.com/coder/websocket`，属于维护状态驱动的依赖修正。

## Acceptance criteria checklist

- [x] `termbridge web` command exists.
- [x] `termbridge web --host --port --open --dev` flags parse.
- [x] `just web` exists.
- [x] Frontend is under `web/`.
- [x] Frontend stack uses Vite, Vue 3, TypeScript, Tailwind CSS, reka-ui dependency.
- [x] Frontend dependency versions are pinned instead of `latest`.
- [x] Frontend uses Yarn lockfile.
- [x] Tailwind config exists and targets `index.html` plus `src/**/*.{vue,ts}`.
- [x] ESLint and Prettier configs exist.
- [x] Vue SFC block order is linted as `template` before `script`.
- [x] Vue SFC script/style indentation is formatted through Prettier.
- [x] xterm uses official scoped packages.
- [x] terminal is wrapped in Vue component.
- [x] WebSocket protocol has documented Go/TS shape.
- [x] PTY bytes use binary WebSocket frames.
- [x] Control messages use JSON text frames.
- [x] Backend validates resize ranges and message types.
- [x] Web sessions reuse Workspace / Session / State / History.
- [x] Detach and close are distinct in protocol and UI.
- [x] Slow client queue is bounded and queue full detaches client.
- [x] Root `install` handles backend and frontend dependencies.
- [x] Root `check` handles backend lint/format/test and frontend format/typecheck/lint/build.
- [x] Existing CLI checks pass.
- [ ] Real browser interaction with `pwsh`, `claude`, `codex` verified by user.
- [ ] Built mode / `go:embed web/dist` implemented.
- [ ] Cross-process reattach implemented.
- [ ] Full M5-level runtime hardening implemented.

## Test results

### Consolidated project check

Command:

```powershell
just check
```

Result: Pass.

Observed command sequence:

```text
go fmt ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
golangci-lint run ./cmd/... ./internal/...  # when available
go test ./cmd/... ./internal/...
cd web && yarn format
cd web && yarn typecheck
cd web && yarn lint
cd web && yarn build
```

Observed Go test packages included:

```text
ok termbridge-go/internal/app
ok termbridge-go/internal/cli
ok termbridge-go/internal/terminalproto
ok termbridge-go/internal/webserver
ok termbridge-go/internal/webterminal
...
```

Observed frontend results:

```text
yarn format: PASS
yarn typecheck: PASS
yarn lint: PASS
yarn build: PASS
```

Build output was generated under `web/dist/`, which is ignored by git.

### Dependency install model

Current root install command:

```powershell
just install
```

Expected behavior:

```text
go mod download
go mod tidy
cd web && yarn install
```

The frontend lockfile is now `web/yarn.lock`. The earlier npm lockfile was removed to avoid mixed package-manager state.

## Missed or expanded scope

### Missed / deferred

- `go:embed web/dist` built mode is not implemented.
- Real interactive browser smoke for `pwsh`, `claude`, `codex` was not run by assistant; requirement says user will self-verify.
- Cross-process reattach is intentionally not implemented.
- Attachment state is in-memory only.
- Full session replay / sequence ack is not implemented.
- Full command argv parser for quoted command text is not implemented in frontend.
- Frontend network response validation is minimal.

### Expanded but justified

- `.gitignore` updated for frontend generated directories.
- `web/yarn.lock` added for deterministic frontend dependency resolution.
- `web/tailwind.config.ts`, `web/eslint.config.js`, `web/.prettierrc.json`, `web/.prettierignore`, and `web/tsconfig.node.json` added after comparison with the reference frontend project.
- `internal/webserver` and `internal/webterminal` tests added beyond the original minimal implementation list.
- `internal/session/recovery_windows.go` lint cleanup added because root `check` now includes `golangci-lint` when available.
- WebSocket dependency switched to `github.com/coder/websocket` after lint surfaced the previous import as deprecated.

### Out-of-scope item present in diff

- `docs/requirement/20260618-m4-local-product-surface.md` is present as a Draft requirement from the earlier paused M4 analysis. It is not part of the M2.5 Web Terminal implementation scope. Treat it as a separate documentation artifact if preparing a clean commit.

## Risks

1. Web terminal browser behavior is only compile/build/lint verified here; real TUI behavior still needs manual validation.
2. `webterminal` writes history synchronously before publishing to clients. This matches M2.5 risk acceptance but can still block PTY reader on slow filesystem.
3. Multi-client attach behavior is not productized. Current implementation does not enforce single-owner attach semantics.
4. WebSocket error handling is sufficient for prototype but not full Gateway-grade reliability.
5. Frontend command input uses whitespace splitting; commands with quoted args or spaces in paths may be split incorrectly.
6. Frontend API response validation is minimal; future Gate should harden protocol decoding.
7. `just web` uses shell backgrounding under the current bash-based justfile. Windows process lifecycle may need refinement after hands-on use.
8. `check` intentionally scopes Go package patterns to `./cmd/... ./internal/...` because frontend `node_modules` contains third-party Go source that `go test ./...` would otherwise discover.

## Incomplete items

- Manual browser verification with:
  - `pwsh -NoLogo`
  - `claude`
  - `codex`
  - detach / reattach
  - close
  - resize
  - large output
- Optional static serving / built mode.
- Optional persisted attachment metadata.
- Optional frontend runtime validation tests.
- Optional better argv editor/parser for command creation.

## Conclusion

M2.5 implementation is code-complete for the planned local Web terminal prototype shape and passes the consolidated backend/frontend project check available in this environment.

The implementation satisfies the primary M2.5 goals: `termbridge web`, `just web`, Vite/Vue/xterm frontend under `web/`, JSON + binary WebSocket protocol, Workspace/Session/History/State reuse, detach/close separation, and bounded client fan-out.

The follow-up frontend tooling refinements are also complete: dependency versions are pinned, Yarn is used, Tailwind config is present, ESLint/Prettier are configured, Vue SFC block order and indentation are enforced, and root `install` / `check` now cover backend and frontend together.

Delivery remains a technical spike / Gate precursor, not a complete Gateway product. Manual browser/TUI validation remains required before treating the Web terminal behavior as proven for daily use.
