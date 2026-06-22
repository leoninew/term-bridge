# M6.1 Serve 统一入口与文档收口验证
最后修改时间: 2026-06-22 17:09:09

Review status: Accepted

## Requirement alignment

对照 `docs/requirement/20260622-m6-serve-entry-consolidation.md`：

- 已将正式入口收口为 `termbridge serve`；CLI help 只展示 `serve`，不再展示 `gateway` / `agent` command。
- 已将开发入口收口为 `just serve`；`just --list` 只保留 `install`、`clean`、`check`、`test`、`build`、`exec`、`web`、`serve`。
- 已删除长期 `just gateway` / `just agent` / `just m6-*` 入口。
- Gateway/Agent 运行参数已进入配置：`.termbridge.default.yaml` 包含 `gateway.*` 和 `agent.*`，当前默认后端端口为 `9010`。
- 前端保持单一项目 `web/`，开发服务监听 `localhost:9011`，通过 `web/.env` 中的 `VITE_TERMBRIDGE_BACKEND=http://localhost:9010` 指向统一后端。
- README 已说明本地 CLI、统一后端服务、Gateway/Agent 配置、单前端访问面和 Gateway MVP 支持/不支持能力。
- README 未公开临时 `admin/admin`。
- M6 verification 已更新为 `Accepted`，并保留 Agent reconnect、20+ sessions、真实 Claude/Codex TUI、Windows 真机验证等未完成项。
- `docs/todo.md` 已按 M5/M6 事实重新整理，区分已处理/仍需人工验证/后续关注项。

## Spec alignment

对照 `docs/spec/20260622-m6-serve-entry-consolidation.md`：

- CLI 入口符合设计：`termbridge serve` 是 Gateway service 和 Agent connector 的统一入口。
- 删除的 Gateway/Agent 参数 flag 不再作为 `serve` command options；`termbridge serve --help` 只展示 `--help`，并说明 Gateway listen、Agent upstream URL、device name 来自配置。
- 配置模型符合设计：`gateway.host`、`gateway.port`、`gateway.open`、`gateway.dev`、`agent.gateway_url`、`agent.device_name` 已在默认配置和 config loader/test 中覆盖。
- App runtime 符合设计：`runServe` 使用同一个 registry 构造 local Web API、Gateway service 和 Agent connector；Gateway service 不直接拥有 PTY/process lifecycle，Agent connector 通过 `webterminal.Registry` adapter 访问 runtime。
- 前端入口符合设计：未新增第二套 frontend 或 Gateway 专用构建，当前通过同一前端承载 local/Gateway 访问面。
- 当前默认端口已按最新用户决策修正为前端 `localhost:9011`、后端 `localhost:9010`。

## Plan alignment

对照 `docs/plan/20260622-m6-serve-entry-consolidation.md`：

- Step 1 已完成：CLI command shape 收口为 `serve`，历史 `gateway` / `agent` command 被拒绝。
- Step 2 已完成：Gateway/Agent 配置结构、默认配置、覆盖和校验测试已加入。
- Step 3 已完成：app 层通过 `serve` 同时启动 Gateway service 和 Agent connector，并覆盖基本生命周期测试。
- Step 4 已完成：justfile 长期入口收口；`just serve` 直接运行 `go run cmd/termbridge/main.go serve`；组合命令按“前端在前、后端在后”组织。
- Step 5 已完成文档核对：README 与设计文档表述为同一前端承载 local/Gateway 访问面。
- Step 6 已完成：README 更新为当前使用路径和配置说明。
- Step 7 已完成：`docs/design.md` 已补充 serve 链路和 Gateway/Agent 能力边界。
- Step 8 已完成：roadmap 已补充 M6/M6.1 状态。
- Step 9 已完成：`docs/todo.md` 已整理过期 TODO。
- Step 10 已完成：M6 verification 已标记 Accepted 并保留未完成项。
- Step 11 为本文档。

## Actual diff summary

主要改动范围：

- `internal/cli/cli.go`、`internal/cli/cli_test.go`
  - 移除 `gateway` / `agent` command shape 和 Gateway/Agent runtime flags。
  - 增加/保留 `serve` usage，并说明运行参数来自配置。
- `internal/config/config.go`、`internal/config/config_test.go`、`.termbridge.default.yaml`
  - 增加 Gateway/Agent 配置结构、默认值、unknown-key allowlist、校验与测试。
  - 当前默认后端端口为 `9010`，Agent 默认连接 `http://127.0.0.1:9010`。
- `internal/app/app.go`、`internal/app/app_test.go`
  - `serve` 同时构造 local Web API、Gateway service 和 Agent connector。
  - 删除 seed-session 入口行为。
- `internal/gateway/server.go`、`internal/webserver/server.go`
  - Gateway 可挂载 local Web API handler；webserver/gateway 暴露 handler 能力。
  - Gateway fallback 端口同步为 `9010`。
- `justfile`
  - 删除旧 `web-backend`、`web-frontend`、`gateway`、`agent`、`m6-*` 和 Go 专用 `fmt` / `vet` 子入口。
  - 保留 `web`、`serve`、`test`、`check`、`build` 等统一入口。
- `web/vite.config.ts`、`web/.env`
  - 前端 dev server 固定 `localhost:9011`。
  - 后端 proxy 通过 Vite env `VITE_TERMBRIDGE_BACKEND=http://localhost:9010` 配置。
- `web/package.json`、`web/yarn.lock`
  - 增加 Vitest 与 `yarn test`，用于统一 `just test` / `just check`。
- `README.md`、`docs/design.md`、roadmap、M6 plan/spec/requirement/verification、`docs/todo.md`
  - 同步统一入口、配置驱动、单前端、当前限制和 M6/M6.1 状态。
- `internal/pty/gopty/manager_test.go`、`internal/state/*`、`internal/webterminal/*`
  - 修复验证过程中暴露的 Windows PTY output race、state file replacement 和 webterminal runtime persistence/logging 相关问题。

## Expected vs actual changed files

### Expected

计划预期会修改：

- README、`.termbridge.default.yaml`、`justfile`。
- CLI/config/app/gateway/agent 相关实现和测试。
- 设计文档、roadmap、todo、M6 verification。
- M6.1 requirement/spec/plan/verification 文档。
- 必要时少量 frontend 配置或代码。

### Actual

实际改动与预期基本一致，并额外包含验证过程中发现并修复的稳定性问题：

- Windows PTY `cmd.exe` 输出读取 race 修复。
- Windows 下 state file overwrite/retry 修复。
- Webterminal runtime persistence failure logging 和 Windows home env 测试修复。
- Frontend Vitest 入口和格式化修复。
- Air 配置仍保留为历史/可选资产，但当前 `just serve` 不再依赖 Air。

这些额外改动都来自实现和验证过程中的真实失败，不属于无关功能扩张。

## Acceptance checklist

- [x] 正式入口为 `termbridge serve`。
- [x] 开发入口为 `just serve`。
- [x] `just gateway` / `just agent` / `just m6-*` 已删除。
- [x] Gateway/Agent runtime 参数不再通过 CLI flags 传递。
- [x] Gateway/Agent runtime 参数进入 Viper 配置和 `.termbridge.default.yaml`。
- [x] 默认前端为 `localhost:9011`。
- [x] 默认后端为 `localhost:9010`。
- [x] `web/.env` 使用 `VITE_TERMBRIDGE_BACKEND=http://localhost:9010`。
- [x] README 覆盖 `termbridge serve`、`just serve`、配置示例、前端访问面和 Gateway MVP 限制。
- [x] README 不出现 `admin/admin`。
- [x] CLI help 不出现 `termbridge gateway` / `termbridge agent` 或 Gateway/Agent runtime flags。
- [x] M6 verification 已标记 Accepted，并保留未完成项。
- [x] `docs/todo.md` 已整理为与 M5/M6 事实一致。
- [x] 未新增第二套 frontend。
- [x] Gateway service 不拥有 PTY/process lifecycle 的边界在代码和文档中保持。

## Command results

### `just --list`

结果：通过。

当前可用 recipe：

```text
build      # Build web frontend and termbridge binary
check      # Run frontend and backend checks
clean      # Remove build outputs
exec *args # Run local command through termbridge exec
install    # Install web and Go dependencies
serve      # Start unified backend: local Web API, Gateway service, and Agent connector
test       # Run frontend and backend unit tests
web        # Start web frontend on localhost:9011, proxying to serve backend
```

确认：无 `gateway`、`agent`、`m6-*` 平级 just target。

### `go test ./internal/config ./internal/cli ./internal/app ./internal/agent`

结果：通过。

```text
ok termbridge-go/internal/config
ok termbridge-go/internal/cli
ok termbridge-go/internal/app
ok termbridge-go/internal/agent
```

### `go test ./internal/tunnel ./internal/gatewayauth ./internal/gateway ./internal/agent ./internal/cli ./internal/app`

结果：通过。

```text
ok termbridge-go/internal/tunnel
ok termbridge-go/internal/gatewayauth
ok termbridge-go/internal/gateway
ok termbridge-go/internal/agent
ok termbridge-go/internal/cli
ok termbridge-go/internal/app
```

### `just check`

结果：通过。

覆盖：

```text
cd web && yarn typecheck
cd web && yarn lint
cd web && yarn format:check
cd web && yarn test
go fmt ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
go test ./cmd/... ./internal/...
```

其中 frontend Vitest 当前没有测试文件，使用 `--passWithNoTests`，命令按预期返回 0。

### `just build`

结果：通过。

覆盖：

```text
cd web && yarn build
mkdir -p bin
go build -o bin/termbridge ./cmd/termbridge
```

警告：Vite/Rolldown 对 `node_modules/@vueuse/core` 的 `/* #__PURE__ */` annotation 输出 warning，并提示 bundle chunk 超过 500 kB。这些 warning 未导致构建失败，且来自依赖 annotation / bundle size 提示。

### CLI help

`go run cmd/termbridge/main.go --help` 结果：通过。

关键输出：

```text
Commands:
  exec       run a command through a PTY
  workspace  list workspaces
  session    list sessions
  web        start local Web terminal server
  serve      start unified Gateway service and Agent connector
```

`go run cmd/termbridge/main.go serve --help` 结果：通过。

关键输出：

```text
Serve starts the unified Gateway service and Agent connector.
Gateway listen settings, Agent upstream URL, and device name are read from config.

Options:
  --help          show serve help
```

`go run cmd/termbridge/main.go gateway` / `agent` 结果：按预期被拒绝，退出码非 0，并输出 `unknown command`。

### 文档一致性核对

结果：通过。

- `README.md` 不出现 `admin/admin`。
- `README.md` 不出现 `termbridge gateway` / `termbridge agent` / `--gateway-url` / `--device-name` / `--seed-session-command`。
- `justfile` 不出现平级 `gateway:` / `agent:` / `m6-*:` target。

### `go test ./internal/pty/gopty -run TestManagerRunsCmdExe -count=20`

结果：通过。

```text
ok termbridge-go/internal/pty/gopty
```

用于确认 Windows `cmd.exe` PTY output race 修复稳定。

### `git diff --check` / `git diff --cached --check`

结果：通过，无 whitespace error。

### Pomelo PW validation asset check

命令：

```text
pomelo-pw validate .pomelo-pw/m6-gateway-web-terminal.yaml
```

结果：未通过，原因是当前本机 `pomelo-pw` 命令入口存在 Python package 安装问题：

```text
ModuleNotFoundError: No module named 'pomelo_pw'
```

说明：M6.1 plan 的必跑验证项不依赖 Pomelo PW；本次尝试用于确认历史 M6 flow 资产可用性，但本机工具安装损坏导致无法执行。M6 历史 verification 中已记录该 flow 曾通过；本次不把 Pomelo PW 结果作为 M6.1 入口收口的通过条件。

## Missed or expanded scope

### Expanded scope

本次实现过程中扩展处理了几类直接影响验证通过和 Windows 稳定性的失败：

1. Windows PTY reader race：`TestManagerRunsCmdExe` 需要先等待输出 marker，再等待 process result。
2. Windows state overwrite：`state.json` overwrite 需要 remove/retry，避免 `exit.json` 已写但 `state.json` 仍为 running。
3. Webterminal persistence failure logging：保存 exit/state 失败时不再静默。
4. Frontend Vitest 入口：为统一 `just test` 增加 frontend unit test entry。
5. `just serve` 从 Air 改为直接 `go run`：避免 Windows 下 Air build output 无 `.exe` 导致系统弹“选择应用打开”。
6. 默认端口按用户最新要求统一为 frontend `9011`、backend `9010`。

### Not completed in this scope

- 未执行完整 Pomelo PW browser flow；当前本机 `pomelo-pw` 安装入口损坏。
- 未做 Agent reconnect 压测。
- 未做 20+ sessions 压测。
- 未做真实 Claude/Codex TUI Gateway attach 人工验证。
- 未做真实多设备/远端 Gateway 部署验证。

## Risks

1. 当前 README 不公开临时 `admin/admin`，符合用户决策；但在正式 auth 文档补齐前，新用户可能知道如何启动服务但不知道如何登录 Gateway MVP。
2. `just check` 包含 `go fmt`，属于会写文件的检查命令；本次运行后 `git status --short` 未显示因 format 产生的新额外变化。
3. `.air.toml` 仍在仓库中作为历史/可选资产，但当前 `just serve` 不依赖 Air。若后续有人直接运行 Air，需要另行决定是删除 `.air.toml` 还是修复为 Windows `.exe` 输出。
4. M6 verification 已 Accepted，但仍保留深度人工验证未完成项；后续推进 M7 前仍应处理或重新评估这些风险。
5. `web/.env` 被加入仓库用于开发默认值，当前只包含非 secret 的本地 backend URL；后续如加入敏感值必须重新调整 ignore/配置策略。

## Incomplete items

1. 修复或重装本机 `pomelo-pw`，再决定是否重新跑 M6 browser flow。
2. 在存在 running session 的环境中重新覆盖 Gateway terminal attach browser 自动化或人工验证。
3. 记录 Agent disconnect/reconnect 行为。
4. 记录真实 Claude/Codex TUI 通过 Gateway attach 的人工验证。
5. 记录 20+ sessions 或同等压力验证。
6. 决定 `.air.toml` 的最终处置：删除、修复为可选热加载资产，或明确标为历史文件。

## Conclusion

M6.1 Serve 统一入口与文档收口已完成并通过当前核心自动化验证：CLI/just 入口已收口，Gateway/Agent 参数已进入配置，默认开发端口已统一为前端 `localhost:9011` / 后端 `localhost:9010`，README/design/roadmap/todo/M6 verification 已同步，M6 专用长期入口已删除。

本次 verification 结论为 Accepted，但保留 Pomelo PW 本机工具损坏、Gateway terminal attach 自动化、Agent reconnect、20+ sessions、真实 TUI 和 `.air.toml` 最终处置等后续事项。
