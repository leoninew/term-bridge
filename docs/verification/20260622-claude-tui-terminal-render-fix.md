# Claude TUI terminal render fix 验证
最后修改时间: 2026-06-23 08:57:54

Review status: Accepted

## Flow mode

轻量模式 / light。

本次收口基于：

- `docs/requirement/20260622-claude-tui-terminal-render-fix.md`
- 用户在 2026-06-23 明确确认：Claude TUI 已经可以工作。
- 用户明确决策：不删除 Air / `.air.toml`。
- 用户明确当前环境：`just serve` + `just web` 已在运行中。
- 用户明确要求：聚焦 Claude TUI 和 Gateway attach；可沉淀为 E2E 的验证直接写入主线测试代码，不用临时文件。

## Requirement alignment / 需求对齐

对照 requirement，本次收口结论如下：

- [x] 后端 `SessionRuntime` 的 current terminal size 已按 PTY initial size 初始化，避免 attach 阶段回到默认 `80x25`。
- [x] 前端 xterm 不再依赖会干扰全屏 TUI 的额外 EOL 转换作为修复路径。
- [x] 创建 session 前的 `cols/rows` 已通过同结构 xterm / FitAddon 测量获得，避免只用 viewport 估算。
- [x] WebSocket `OPEN` 前产生的 resize 已通过 pending resize 缓存，并在连接建立后补发。
- [x] 用户已人工确认 Claude TUI 当前可以工作，本修复可收口。
- [x] pending resize 行为已沉淀为前端 Vitest 主线测试。
- [x] Gateway attach running session 的可自动化 relay 路径已沉淀为 Go E2E 测试，覆盖 Gateway service、真实 Agent client、fake RuntimeAccess、Browser WebSocket、terminal input/output、resize 和 detach。
- [x] 本次没有修改 `justfile`，也没有删除或修改 Air / `.air.toml`。

## Spec alignment / 规格对齐

不适用。该任务按 SpecFlow 轻量模式 / light 执行，没有单独 spec 文档。

## Plan alignment / 计划对齐

不适用。该任务按 SpecFlow 轻量模式 / light 执行，没有单独 plan 文档；按 requirement 中的分层 verification plan 执行。

## Actual diff summary / 实际改动摘要

本次收口包含文档和主线测试代码：

- 更新 `docs/requirement/20260622-claude-tui-terminal-render-fix.md`
  - 补充分层验证计划。
  - 记录不修改 `justfile`、临时验证代码用后清理、允许 Python `uv` 集成测试。
  - 记录用户已确认 Claude TUI 可工作、不删除 Air、`just serve` / `just web` 正在运行等决策。
- 新增 `docs/verification/20260622-claude-tui-terminal-render-fix.md`
  - 记录本轻量验证结果、命令、风险和剩余后续项。
- 删除 `docs/todo.md`
  - 按用户要求移除过期集中 TODO 文档，避免继续维护与 milestone verification 重叠的状态源。
  - Codex、Agent reconnect、20+ sessions、Windows cleanup、真实浏览器 Gateway attach 等后续项保留在本 verification 的 incomplete items 中。
- 新增 `web/src/features/sessions/useTerminalSocket.test.ts`
  - 覆盖 WebSocket `OPEN` 前 resize 缓存、open 后 hello + pending resize 补发、只保留最新 pending resize、已发送 resize 不在重连后重复发送。
- 新增 `internal/transport/http/gatewayapi/terminal_e2e_test.go`
  - 覆盖 Gateway service + Agent client + Browser terminal WebSocket 的 running session attach E2E。
  - 使用 fake RuntimeAccess / fake TerminalStream 模拟可控 TUI runtime，不依赖真实 Claude 账号或浏览器。
  - 验证输出 marker、binary input、resize relay、detach reason。

未修改 `justfile`，未删除 `.air.toml`。

## Expected vs actual changed files / 预期与实际改动对比

| 文件 | 预期 | 实际 | 结论 |
| --- | --- | --- | --- |
| `docs/requirement/20260622-claude-tui-terminal-render-fix.md` | 更新验证计划和约束 | 已更新 | Pass |
| `docs/verification/20260622-claude-tui-terminal-render-fix.md` | 创建/更新轻量 verification | 已更新 | Pass |
| `docs/todo.md` | 按用户要求删除过期集中 TODO | 已删除 | Pass |
| `web/src/features/sessions/useTerminalSocket.test.ts` | 主线单元测试覆盖 pending resize | 已新增 | Pass |
| `internal/transport/http/gatewayapi/terminal_e2e_test.go` | 主线 E2E 覆盖 Gateway attach relay | 已新增 | Pass |
| `justfile` | 不修改 | 未修改 | Pass |
| `.air.toml` | 不删除、不修改 | 未修改 | Pass |

## Acceptance checklist / 验收清单

- [x] Claude TUI 已由用户确认可以工作。
- [x] 本次收口不删除 Air / `.air.toml`。
- [x] 本次收口不重启或接管正在运行的 `just serve` / `just web`。
- [x] 需求文档记录当前决策和关闭状态。
- [x] `docs/todo.md` 已按用户要求删除，不再维护过期集中 TODO 状态源。
- [x] 前端 pending resize 可自动化边界已有主线单元测试。
- [x] Gateway attach running session 的 relay 可自动化边界已有主线 Go E2E 测试。
- [x] 仍保留 Codex TUI、真实浏览器 Gateway attach、Agent reconnect、20+ sessions、Windows cleanup 等真实未完成验证项。
- [x] 没有修改 `justfile`。

## Command results / 命令结果

### `git status --short`

收口开始前运行，结果：通过，无输出，表示工作区在本次文档收口前为 clean。

### `just serve` / `just web`

未由本次收口重新启动。用户已确认两者正在运行中，本次文档收口不接管或重启运行中的服务，避免引入无关状态变化。

### Frontend targeted test

命令：

```text
yarn --cwd web test useTerminalSocket
```

最终结果：通过。

```text
Test Files  1 passed (1)
Tests  4 passed (4)
```

说明：第一次在 repo 根目录误运行 `yarn test useTerminalSocket --runInBand`，因根目录没有 `package.json` 失败；随后使用正确命令 `yarn --cwd web test useTerminalSocket` 通过。

### Frontend type/lint targeted checks

命令：

```text
yarn --cwd web typecheck
yarn --cwd web lint
yarn --cwd web prettier --check src/features/sessions/useTerminalSocket.test.ts
```

结果：通过。

说明：全量 `yarn --cwd web format:check` 失败，但失败文件是既有格式问题：

```text
src/components/workspace/WorkspaceSessionSidebar.vue
src/features/sessions/useTerminalSocket.ts
src/i18n.ts
```

新增测试文件自身通过 Prettier 检查。未运行 `prettier --write`，避免引入无关格式化改动。

### Go targeted runtime size test

命令：

```text
go test ./internal/application/terminal -run TestRuntimeInitialSizeMatchesCreatedPTYSize
```

结果：通过。

```text
ok  termbridge-go/internal/application/terminal
```

### Gateway attach E2E test

命令：

```text
go test ./internal/transport/http/gatewayapi -run TestGatewayAgentTerminalAttachE2E -count=1
```

结果：通过。

```text
ok  termbridge-go/internal/transport/http/gatewayapi
```

覆盖：

- Gateway service route registration。
- Agent client 通过 tunnel 连接 Gateway。
- Browser terminal WebSocket attach running session。
- fake terminal stream 输出 `POMELO_M6_ATTACH_READY` marker。
- Browser binary input relay 到 Agent/Runtime。
- Browser resize control relay 到 Runtime。
- Browser detach control 触发 `gateway_detached`。

### Gateway and terminal targeted Go tests

命令：

```text
go test ./internal/transport/http/gatewayapi ./internal/application/terminal
```

结果：通过。

```text
ok  termbridge-go/internal/transport/http/gatewayapi
ok  termbridge-go/internal/application/terminal
```

### Frontend typecheck and lint

命令：

```text
yarn --cwd web typecheck
yarn --cwd web lint
```

结果：通过。

```text
vue-tsc --noEmit
Done
eslint .
Done
```

### New frontend test formatting

命令：

```text
yarn --cwd web prettier --check src/features/sessions/useTerminalSocket.test.ts
```

结果：通过。

```text
All matched files use Prettier code style!
```

说明：未运行 `prettier --write`，避免引入无关格式化改动。

### Diff whitespace checks

命令：

```text
git diff --check
git diff --cached --check
```

结果：通过，无 whitespace error。

## Missed or expanded scope / 范围偏差

### 未进入范围

- 未删除或调整 `.air.toml`。
- 未修改 `justfile`。
- 未修复本机 `pomelo-pw` 安装问题。
- 未重新运行 Pomelo PW browser flow。
- 未验证真实 Gateway 页面中 Claude TUI 的浏览器截图路径。
- 未验证 Codex TUI。
- 未验证 Agent disconnect / reconnect。
- 未执行 20+ sessions 压测。
- 未做 Windows process tree cleanup 黑盒验证。

### 实际扩展

- 按用户要求，原本只记录为后续项的 Gateway attach running session 可自动化部分已沉淀为主线 Go E2E 测试。
- 按 verification plan，pending resize 可自动化部分已沉淀为主线 Vitest 测试。

## Risks / 风险

1. Claude TUI 可工作结论来自用户当前人工确认；本次未新增 Claude 真实 TUI 浏览器截图或自动化复现记录。
2. Gateway attach E2E 使用 fake RuntimeAccess / fake TerminalStream，证明 relay、input/output、resize、detach 链路，但不替代真实 Claude/Codex TUI 视觉验证。
3. Pomelo PW 当前本机入口损坏，无法作为浏览器 flow evidence；后续可修复 Pomelo PW 或继续使用 Python `uv` + Playwright 做浏览器端验证。
4. 全量 frontend format check 仍受既有文件格式问题影响；本次新增测试文件自身格式通过。
5. Air / `.air.toml` 按用户决策保留；当前 `just serve` 是否使用 Air 取决于当前 justfile，后续如直接使用 Air 遇到 Windows `.exe` 输出问题，需要单独处理。

## Incomplete items / 未完成项

以下事项不阻塞 Claude TUI 修复和 Gateway attach relay E2E 收口，但仍应作为 M6/M7 后续验证项保留：

1. 真实浏览器中通过 Gateway attach running Claude session 的截图或自动化记录。
2. Codex TUI 真实交互验证。
3. Agent disconnect / reconnect 行为和 stale route cleanup。
4. 20+ sessions 或同等级压力场景。
5. Windows 真机上的 Agent/Gateway 连接、ConPTY 和 process tree cleanup。
6. Pomelo PW 本机工具修复，或以 Python `uv` + Playwright 替代形成长期浏览器 E2E 入口。

## Conclusion / 结论

Claude TUI terminal render fix 按轻量模式 / light 收口：用户已确认 Claude TUI 当前可以工作；本次保留 Air / `.air.toml`，不接管正在运行的 `just serve` / `just web`，不修改 `justfile`。

本轮已将可稳定自动化的两条核心验证沉淀为主线测试：

1. 前端 pending resize 单元测试，防止 WebSocket OPEN 前 resize 被丢弃导致 TUI 初始尺寸错误。
2. Gateway attach E2E 测试，覆盖 Gateway service、Agent tunnel、Browser terminal WebSocket、input/output、resize、detach 的 running session relay 链路。

本验证结论为 Accepted。