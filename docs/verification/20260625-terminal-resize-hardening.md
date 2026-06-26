# Terminal resize hardening 验证
最后修改时间: 2026-06-26 11:17:40

Review status: Draft

Flow mode: light / 轻量模式
Stage: Verification / 验证

## Requirement alignment

依据：`docs/requirement/20260625-terminal-resize-hardening.md`，状态为 `Accepted`。

| Requirement / Acceptance | 验证结论 |
| --- | --- |
| `SessionRuntime.resize()` 不应在 PTY resize 失败后把 `current` 永久更新为失败尺寸；相同尺寸后续 resize 应可重试。 | ✅ 已实现。`current` 只在 `r.pty.Resize(size)` 成功后更新，失败会记录 warning 并返回错误；新增 `TestRuntimeRetriesResizeAfterFailure` 覆盖同尺寸重试。 |
| CLI runner 的 `watchResize()` 不应在 `session.Resize(next)` 失败后推进 `last`。 | ✅ 已实现。抽出 `resizeIfChanged`，失败返回旧 `last`；新增 `TestResizeIfChangedRetriesAfterFailure` 覆盖失败后重试。 |
| `gopty.Manager.Start()` 对启动前初始 PTY resize 错误不再静默忽略。 | ✅ 已实现。初始 resize 失败会关闭 PTY 并返回 wrapped error；新增 `manager_initial_resize_test.go` 覆盖错误、关闭和不返回 session。 |
| Gateway / Agent 中 resize 转发或执行失败不应完全静默；前端浏览器诊断日志、Gateway 日志和 Agent 日志应覆盖异常路径。 | ✅ 已实现。Gateway/Agent 多个 decode/build/write/resize 失败路径增加 warning；前端 socket/control error 使用 `console.error` 级诊断日志。 |
| reattach 初始尺寸必须本轮完成，保持旧客户端兼容。 | ✅ 已实现。Browser terminal WS attach query 支持 `cols/rows`；Gateway 将尺寸写入 `TerminalAttachReq`；Agent attach 后先 resize；缺省 query 时保持旧客户端兼容。E2E 覆盖 attach resize。 |
| resize 上限必须本轮收紧；协议常量、前端 fallback clamp 和测试保持一致。 | ✅ 已实现。后端 `MaxCols/MaxRows` 从 `10000` 收紧到 `1000`，前端 `clampTerminalSize` 同步为 `1000`，协议测试覆盖 `1000` 接受、`1001` 拒绝。 |
| 新增或更新测试覆盖关键路径，现有 pending resize 和协议边界测试不得回退。 | ✅ 已覆盖。新增/更新 runtime、runner、gopty initial resize、gateway attach resize、attach resize error、协议边界测试；web socket pending resize 逻辑未回退。 |
| 变更范围集中在 terminal resize 相关文件和对应测试，不引入无关 UI 或配置变更。 | ✅ 基本满足。`web/eslint.config.js` 仅为新增 `URLSearchParams` 全局支持；其余变更均与 terminal resize / diagnostics / tests 直接相关。 |
| 用户后续反馈：实际 size 合适但受输入法和滚动条影响会向左弹动，采纳 `cols - 1 / rows - 1` safety margin。 | ✅ 已补充。前端测量/fit 使用 `fitSafeTerminalSize`，并对本地 xterm grid 调用 `terminal.resize(safeCols, safeRows)`，确保浏览器 grid 与后端 PTY 使用同一安全尺寸。 |

## Spec alignment

不适用。当前任务采用 light / 轻量模式，仅依据已接受的 Requirement / 需求进行实现与验证。

## Plan alignment

不适用。当前任务采用 light / 轻量模式，未创建独立 Plan / 计划文档；本验证按 Requirement / 需求中的目标、非目标、验收标准和风险核对。

## Actual diff summary

### Backend / Go

| 文件 | 变更摘要 |
| --- | --- |
| `internal/application/terminal/runtime.go` | 串行化 resize；失败时不推进 `current`；成功后才更新当前尺寸；失败路径记录 warning。 |
| `internal/application/runner/terminal.go` | 抽出 `resizeIfChanged`，resize 失败时保留旧 `last` 并允许后续 tick 重试。 |
| `internal/infrastructure/pty/gopty/manager.go` | 初始 PTY resize 失败时关闭 PTY 并返回错误；增加 `newPTY` 注入点便于测试。 |
| `internal/protocol/terminal/protocol.go` | `MaxCols/MaxRows` 收紧到 `1000`。 |
| `internal/application/agent/client.go` | Agent terminal attach/resize/input 失败路径增加校验、日志和必要的 tunnel error 返回。 |
| `internal/transport/http/gatewayapi/server.go` | Terminal WS attach query 解析 `cols/rows`；attach frame 写入初始尺寸；关键 write/decode/build 失败路径记录日志。 |
| `internal/transport/http/gatewayapi/route.go` | Gateway terminal relay 将 tunnel `FrameError` 转为 browser terminal control error，并记录 output/error 失败路径。 |

### Frontend / Web

| 文件 | 变更摘要 |
| --- | --- |
| `web/src/protocol/terminal.ts` | 新增 terminal size 常量、`clampTerminalSize` 和 `fitSafeTerminalSize`；fit safety margin 固定为 1 cell。 |
| `web/src/components/terminal/useXterm.ts` | xterm 测量与 resize 使用 `proposeDimensions()` 获取最大拟合尺寸，再减 1 cell 并调用 `terminal.resize()`，确保本地 grid 和发往后端的尺寸一致。 |
| `web/src/composable/useTerminalSize.ts` | fallback 初始尺寸使用同一 `fitSafeTerminalSize`，日志记录 measured 与 safe 尺寸。 |
| `web/src/components/terminal/TerminalView.vue` | 保存最近 terminal size，连接/重连时通过 query 传递 attach size；server error 记录 error 级诊断并写入 terminal。 |
| `web/src/features/sessions/api.ts` | `terminalWsUrl` 支持 token 与 optional size 共同组装 query。 |
| `web/src/features/sessions/useTerminalSocket.ts` | websocket/control decode error 使用 error 级诊断日志。 |
| `web/src/components/terminal/diagnostics.ts` | 新增 `logTerminalDiagnosticError`。 |
| `web/eslint.config.js` | 增加 `URLSearchParams` readonly global。 |

### Tests / Docs

| 文件 | 变更摘要 |
| --- | --- |
| `docs/requirement/20260625-terminal-resize-hardening.md` | 新增已接受需求文档。 |
| `internal/application/runner/runner_test.go` | 新增 runner resize 失败后重试测试。 |
| `internal/application/terminal/registry_test.go` | 新增 runtime resize 失败后重试测试。 |
| `internal/infrastructure/pty/gopty/manager_initial_resize_test.go` | 新增 initial PTY resize 失败测试。 |
| `internal/protocol/terminal/protocol_test.go` | 更新 resize 上限测试并新增 rows 越界测试。 |
| `internal/transport/http/gatewayapi/terminal_e2e_test.go` | 更新 attach E2E 以验证 attach 初始尺寸；新增 attach resize 失败返回 control error 测试。 |
| `web/src/protocol/terminal.test.ts` | 新增 `fitSafeTerminalSize` safety margin 与最小值保护测试。 |
| `docs/verification/20260625-terminal-resize-hardening.md` | 新增并更新本验证记录。 |

## Expected vs actual changed files

| 预期范围 | 实际文件 | 结论 |
| --- | --- | --- |
| terminal resize runtime / runner 状态一致性 | `internal/application/terminal/runtime.go`, `internal/application/runner/terminal.go`, 对应测试 | ✅ 符合 |
| PTY initial resize 错误处理 | `internal/infrastructure/pty/gopty/manager.go`, `manager_initial_resize_test.go` | ✅ 符合 |
| terminal protocol resize 上限与前端安全尺寸 | `internal/protocol/terminal/protocol.go`, `protocol_test.go`, `web/src/protocol/terminal.ts`, `web/src/protocol/terminal.test.ts` | ✅ 符合 |
| Gateway / Agent resize attach 和错误可观测性 | `internal/application/agent/client.go`, `internal/transport/http/gatewayapi/server.go`, `route.go`, `terminal_e2e_test.go` | ✅ 符合 |
| Frontend xterm / socket attach size 和 diagnostics | `TerminalView.vue`, `useXterm.ts`, `useTerminalSize.ts`, `api.ts`, `useTerminalSocket.ts`, `diagnostics.ts`, `eslint.config.js` | ✅ 符合 |
| 过程文档 | requirement 与 verification 文档 | ✅ 符合 |

当前工作区状态注意事项：

- 暂存区已有 terminal resize hardening 变更。
- `docs/verification/20260625-terminal-resize-hardening.md`、`internal/infrastructure/pty/gopty/manager_initial_resize_test.go`、`web/src/protocol/terminal.test.ts` 是新增文件。
- 格式修复和后续 safety margin 修改导致部分已暂存文件存在未暂存差异，提交前需要统一检查并暂存。
- 当前工作区还显示 `web/src/components/workspace/WorkspaceSessionSidebar.vue` 与 `web/src/views/SessionsView.vue` 修改；本验证未将其作为 resize hardening 核心变更评估，提交前建议人工确认是否同属本次范围。

## Acceptance criteria checklist

- [x] Runtime resize 失败不推进成功尺寸状态，允许同尺寸重试。
- [x] CLI runner resize 失败不推进 `last`，允许后续 tick 重试。
- [x] PTY 启动前 initial resize 失败不再静默忽略，并关闭 PTY。
- [x] Gateway resize frame build/write、attach size parse、control decode、terminal relay error 等异常路径有日志。
- [x] Agent attach resize / inbound resize / input decode/write 等异常路径有日志，attach resize 失败返回 tunnel error。
- [x] Browser 侧 websocket/control error 通过 error 级 diagnostic log 输出。
- [x] Reattach/attach 初始尺寸通过 `cols/rows` query 传递到 Gateway，再进入 `TerminalAttachReq` 和 Agent stream resize。
- [x] 旧客户端不传 `cols/rows` 时保持兼容。
- [x] resize 上限后端协议和前端 clamp 同步为 `1000x1000`。
- [x] 前端 xterm fit 使用 `cols - 1 / rows - 1` safety margin，避免精确贴边时受输入法/滚动条影响出现横向弹动；本地 xterm grid 与发送给后端的尺寸一致。
- [x] 针对 runtime、runner、gopty initial resize、Gateway/Agent attach resize、协议边界、前端 safety margin 的测试已补充或更新。

## Test results

### 初次验证发现并修复的格式问题

| 命令 | 初次结果 | 处理 |
| --- | --- | --- |
| `gofmt -l cmd internal` | ❌ 输出 `internal/application/runner/runner_test.go`、`internal/transport/http/gatewayapi/terminal_e2e_test.go` | 已运行 `gofmt -w internal/application/runner/runner_test.go internal/transport/http/gatewayapi/terminal_e2e_test.go` |
| `yarn --cwd web format:check` | ❌ Prettier 报 `src/components/terminal/TerminalView.vue` | 已运行 `yarn --cwd web prettier --write src/components/terminal/TerminalView.vue` |

### 最终验证命令

| 命令 | 结果 |
| --- | --- |
| `gofmt -l cmd internal` | ✅ 通过，无输出 |
| `yarn --cwd web format:check` | ✅ 通过，`All matched files use Prettier code style!` |
| `yarn --cwd web typecheck` | ✅ 通过，`vue-tsc --noEmit` |
| `yarn --cwd web lint` | ✅ 通过，`eslint .` |
| `yarn --cwd web test` | ✅ 通过，8 个 test files、34 个 tests passed |
| `go vet ./cmd/... ./internal/...` | ✅ 通过，无输出 |
| `go test ./cmd/... ./internal/...` | ✅ 通过，所有 cmd/internal package 测试通过或无测试文件 |

补充说明：没有直接运行 `just check`。原因是 `justfile` 中该目标包含会修改文件的 `cd web && yarn format:check --write` 和 `go fmt ...`；本轮改为显式运行等价的 typecheck、lint、format check、web test、go vet、go test，并在发现格式问题后单独执行格式化修复再重跑最终检查。

## Missed or expanded scope

- 无产品功能缺口。
- 范围轻微扩展：新增 `web/eslint.config.js` 中 `URLSearchParams` global，只服务于 `terminalWsUrl` query 组装，不引入独立功能。
- 范围轻微扩展：新增 `manager_initial_resize_test.go`，用于补齐 requirement 中的 initial resize 失败测试覆盖。
- 范围轻微扩展：根据用户后续反馈新增前端 fit safety margin，解决尺寸刚好但输入法/滚动条导致横向弹动的问题；该变更限定在前端 xterm 尺寸计算与测试，不修改后端协议上限。
- 验证阶段执行了格式化修复，导致部分已暂存文件出现未暂存格式化差异；提交前需要统一暂存。

## Risks

1. 未执行浏览器端真实手动运行验证；reattach 初始尺寸通过 Gateway/Agent E2E fake runtime 和前端代码路径验证，safety margin 通过前端单元测试和类型/格式/lint 验证。
2. initial PTY resize 失败改为 `Start()` 返回错误，可能让某些原本以错误尺寸启动的环境暴露为创建失败；这与 requirement 中“优先选择不启动错误尺寸进程”的决策一致。
3. resize 上限收紧到 `1000x1000` 可能拒绝极端尺寸请求；当前后端协议测试和前端 clamp 已同步，错误路径可诊断。
4. 前端 safety margin 会固定少用右侧 1 列和底部 1 行，换取避免精确贴边导致的滚动条/IME 抖动；这是用户确认采纳的取舍。
5. 当前工作区包含新增测试、verification 文档、格式化差异和后续 safety margin 修改；提交前需要人工检查并暂存。

## Incomplete items

- 产品实现层面：无已知未完成项。
- 验证层面：未做真实浏览器手动 reattach 与 IME 输入观察。
- 版本控制层面：未执行 `git add` / `git commit` / `git push`；新增文件和未暂存差异仍需提交前处理。

## Conclusion

Verification / 验证通过。实现与 light / 轻量模式 Requirement / 需求对齐；格式问题已修复，前端 safety margin 已按用户反馈补充，最终 Go 与 Web 检查均通过。提交前需将未暂存的新测试、verification 文档、格式化差异和 safety margin 变更纳入暂存，或按需要拆分提交。
