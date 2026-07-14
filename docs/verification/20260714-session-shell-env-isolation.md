# Session 交互式 Shell 环境隔离验证

最后修改时间: 2026-07-14 13:17:55

Review status: Accepted

## Flow mode / Stage

轻量模式 / light；验证 / Verification。

## Requirement alignment / 需求对齐

- Requirement 文档：`docs/requirement/20260714-session-shell-env-isolation.md`
- Requirement status：`Accepted`

对齐结论：实现按需求默认决策修复了父 shell option 环境变量污染交互式 bash 的问题。PTY 用户进程启动时剥离 `SHELLOPTS` / `BASHOPTS`；失败命令不再因继承 `errexit` 直接杀死交互 shell；根进程真正退出与一次性命令结束的 stop 语义保持不变。

## Spec alignment / 规格对齐

不适用。轻量模式 / light 未创建单独 Spec 文档；按 Requirement / 需求核对。

## Plan alignment / 计划对齐

不适用。轻量模式 / light 未创建 Plan 文档；按 Requirement / 需求实施。

## Actual diff summary / 实际差异摘要

- `internal/agent/model/task/process/env.go`：新增 `SanitizeLaunchEnv`，大小写不敏感剥离 `SHELLOPTS` / `BASHOPTS`，保留 `nil` 与空 slice 语义。
- `internal/agent/model/task/process/env_test.go`：覆盖剥离、nil、空 slice。
- `internal/agent/model/task/process/spec.go`：`NewSpec` 默认环境走 `SanitizeLaunchEnv(os.Environ())`。
- `internal/agent/model/task/process/spec_test.go`：验证 `NewSpec` 不保留 shell option 变量并保留其它 env。
- `internal/agent/infrastructure/pty/gopty/manager.go`：PTY 启动前统一再过滤一次 env，覆盖 create / rerun / CLI exec 与手工 `ProcessSpec`。
- `internal/agent/application/task/terminal/registry.go`：删除 `spec.Env = os.Environ()` 覆盖，避免绕过 `NewSpec` 过滤。
- `docs/requirement/20260714-session-shell-env-isolation.md`：需求记录与 Accepted 状态。

## Expected vs actual changed files / 预期与实际修改文件对比

| 预期 | 实际 |
|------|------|
| process 层 env 过滤与测试 | 已实现 `env.go` / `env_test.go` / `spec.go` / `spec_test.go` |
| PTY 启动统一过滤 | 已实现 `gopty/manager.go` |
| create/rerun 不重新注入污染 env | 已删除 registry 中 `os.Environ()` 覆盖 |
| 不改前端 / 不改用户 shell rc | 未改前端与 bashrc |
| 过程文档 | requirement + verification |

未发现范围外业务 API、前端或生命周期状态机改动。

## Acceptance criteria checklist / 验收标准检查清单

- [x] Session 启动路径（create / rerun）在构造/应用 `ProcessSpec.Env` 时剥离 `SHELLOPTS` 与 `BASHOPTS`。
- [x] 在注入 `SHELLOPTS=...errexit...` 时，交互式 bash 执行 `false` 后仍存活并继续输出。
- [x] 根进程 `exit` 后仍正常结束（冒烟中显式 `exit` 得到 code=0）。
- [x] 一次性命令 `bash -c "false"` 仍按非 0 退出（code=1），stop 语义未破坏。
- [x] 自动化测试覆盖环境过滤与 `NewSpec` 行为。
- [x] 相关 Go 包测试通过：`process`、`gopty`、`terminal`、`runner`。

## Test / command results / 测试 / 命令结果

通过：

```text
go test ./internal/agent/model/task/process/ -count=1
  → ok

go test ./internal/agent/application/task/runner/ -count=1
  → ok

go test ./internal/agent/application/task/terminal/ -count=1
  → ok

go test ./internal/agent/infrastructure/pty/gopty/ -count=1
  → ok

go test ./internal/agent/model/task/process/ -run 'Sanitize|NewSpecSanitizes' -v -count=1
  → TestSanitizeLaunchEnvStripsShellOptionVars PASS
  → TestSanitizeLaunchEnvPreservesNil PASS
  → TestSanitizeLaunchEnvPreservesEmpty PASS
  → TestNewSpecSanitizesLaunchEnv PASS
```

行为冒烟（手工 `go run`，未入库）：

```text
注入 SHELLOPTS/BASHOPTS 后启动 bash
  false; echo STILL_HERE_AFTER_FALSE
  → PASS still alive after false

注入 SHELLOPTS 后启动 bash -c "false"
  → one-shot exit code 1
```

未完成 / 跳过：

```text
task check / golangci-lint run on changed packages
  → 本轮因命令超时未拿到完整 lint 结果；功能测试与行为冒烟已通过
```

未执行完整 `task test`（含前端 yarn test）——本变更无前端改动，按相关后端包测试验收。

## Missed or expanded scope / 遗漏或扩大的范围

- 无扩大范围：未做全量 env 净化，未改前端与 session 状态机。
- 无遗漏关键路径：Manager 层统一过滤覆盖 CLI exec；registry 不再二次注入未过滤 env。
- 未在 CI 中新增真实 bash 交互集成测试；采用纯函数测试 + 本地 PTY 冒烟，与 requirement 风险说明一致。

## Risks / 风险

1. 依赖继承父进程 `SHELLOPTS`/`BASHOPTS` 的脚本启动语义会变化；交互终端场景通常更期望干净 shell。
2. 用户 `.bashrc` 主动 `set -e` 仍会导致失败命令退出 shell；本修复只隔离父环境污染。
3. 全量 `task check` / lint 本轮未完整跑通，交付前如需可再补一次。
4. Browser workbench 端到端（重启 agent 后在 UI 新建 bash 会话）需用户本地确认。

## Incomplete items / 未完成事项

1. 未在真实 Browser UI 中由用户完成端到端验收。
2. 未完整执行 `task check` / 全仓 `go test ./cmd/... ./internal/...`。

## Conclusion / 结论

需求目标已达成：父 shell option 不再污染 PTY 用户进程；交互 bash 在失败命令后可继续运行；一次性失败命令仍正常退出。建议在用户本地重启 agent 后做一次 UI 冒烟，然后提交本 diff。
