# M2 PTY Command Runner MVP 验证
最后修改时间: 2026-06-18 08:15:03

Review status: Accepted

## Requirement alignment

验证对象：

```text
docs/requirement/20260617-m2-pty-command-runner-mvp.md
```

M2 目标是实现最小可用 PTY Command Runner MVP，让用户可以通过：

```text
termbridge [options] -- <command...>
```

在 TermBridge 管理的 PTY runtime 中运行用户命令，并获得可预测的输入、输出、退出码和停止行为。

本次实现对齐以下需求：

- 将 M1 placeholder `termbridge command runner is not implemented yet` 替换为正式 runtime 路径。
- 保持 `termbridge [options] -- <command...>` 为唯一规范入口。
- `--` 后 command / args 原样传给用户命令。
- 支持 `--cwd` 并传递给底层 PTY process。
- 新增 `ProcessSpec`，表达 command、args、cwd、env、initial terminal size。
- 新增 `CommandRunner`，负责从 spec 到 runtime execution。
- 新增 TermBridge-owned `PTYSession` / `PTYManager` abstraction。
- `go-pty` 只出现在 `internal/pty/gopty` adapter 内，未泄漏到 CLI/app 层。
- 启动前通过 `exec.LookPath` resolve executable。
- 实现 stdin relay、stdout relay、resize relay、Ctrl+C escalation、exit code propagation。
- 用户命令非零退出码透传为 `termbridge` exit code，不再被折叠成 TermBridge runtime error。
- TermBridge 自身 usage/config/internal/runtime error 仍使用保留 exit code。

仍未完全覆盖或需后续人工验证：

- Claude Code / Codex 真实 TUI 长时间交互尚未验证，只完成 `--version` smoke。
- Ctrl+C 真实交互尚未在人工 terminal 中验证；代码层已实现 soft interrupt / close / kill escalation。
- process tree cleanup 当前是基础 fallback，复杂 Agent 子孙进程 cleanup 仍应进入 M5 hardening。

## Spec alignment

standard / 标准模式本功能没有单独 spec 文档；按 requirement 和 plan 验证。

架构对齐：

```text
cmd/termbridge
  ↓
internal/cli
  ↓
internal/app
  ↓
internal/runner
  ↓
internal/pty
  ↓
internal/pty/gopty
  ↓
github.com/aymanbagabas/go-pty
```

当前实现没有引入 GUI、Web、Gateway、Workspace daemon、detach 或 reattach，符合 M2 非目标。

## Plan alignment

验证对象：

```text
docs/plan/20260617-m2-pty-command-runner-mvp.md
```

计划中的主要步骤完成情况：

- [x] CLI / app IO and exit-code contract。
- [x] 新增 `internal/process`：`ProcessSpec`、terminal size、resolve、exit classification。
- [x] 新增 `internal/pty` abstraction。
- [x] 新增 `internal/pty/gopty` adapter。
- [x] 新增 `internal/runner` lifecycle、relay、interrupt、terminal resize handling。
- [x] `internal/app` 从 placeholder 切到正式 runner wiring。
- [x] 更新 app / cli / process / runner 测试。
- [x] 运行自动化检查、build 和 smoke。

计划中“真实 adapter 最小集成测试”以 CLI smoke + `internal/cli` command-exit test 形式覆盖；尚未为 `internal/pty/gopty` 单独添加 adapter package test 文件，这是一个可后续补强点。

## Actual diff summary

本次 M2 实现涉及：

```text
cmd/termbridge/main.go
go.mod
internal/app/app.go
internal/app/app_test.go
internal/cli/cli.go
internal/cli/cli_test.go
internal/process/exit.go
internal/process/exit_test.go
internal/process/resolve.go
internal/process/resolve_test.go
internal/process/spec.go
internal/process/spec_test.go
internal/pty/pty.go
internal/pty/gopty/manager.go
internal/pty/gopty/manager_windows.go
internal/runner/interrupt.go
internal/runner/relay.go
internal/runner/relay_test.go
internal/runner/runner.go
internal/runner/runner_test.go
internal/runner/terminal.go
docs/requirement/20260617-m2-pty-command-runner-mvp.md
docs/plan/20260617-m2-pty-command-runner-mvp.md
docs/verification/20260617-m2-pty-command-runner-mvp.md
```

同时，当前 working tree 还包含本次 M2 之前已有或并行存在的文档/README 变更：

```text
README.md
docs/design.md
```

这些不属于 M2 runtime implementation 的核心代码范围，提交前建议确认是否与 M2 同 commit 保持一致，或拆分提交。

## Expected vs actual changed files

### Expected

```text
cmd/termbridge/main.go
internal/cli/*
internal/app/*
internal/process/*
internal/pty/*
internal/runner/*
go.mod
docs/requirement/20260617-m2-pty-command-runner-mvp.md
docs/plan/20260617-m2-pty-command-runner-mvp.md
docs/verification/20260617-m2-pty-command-runner-mvp.md
```

### Actual

实际 M2 变更覆盖上述范围。

额外存在：

```text
README.md
docs/design.md
```

这两个文件属于当前 working tree 的其他文档变更，需要提交前由用户确认是否同批提交。

## Acceptance criteria checklist

- [x] 存在 M2 requirement 文档，明确 PTY Command Runner 的目标、边界、验收标准、风险和待定事项。
- [x] `termbridge -- <command...>` 可以通过正式 runtime 路径运行用户命令，不再只返回 M1 的 runner not implemented。
- [x] `--cwd` 指定目录对底层用户进程生效；通过 `pwsh -Command "(Get-Location).Path"` smoke 验证。
- [x] 未传 `--cwd` 时使用当前目录；由 `config.Load` 既有行为和 app wiring 继承。
- [x] `--` 后的 command 与 args 原样传递给用户命令；由 CLI parse tests 和 app spec tests 覆盖。
- [x] 启动进程前进行 executable path resolve；由 `internal/process.ResolveExecutable` 和 runner 调用覆盖。
- [x] 定义 TermBridge 自己的 PTY abstraction，业务层不直接依赖 `go-pty.Pty`。
- [x] 定义并使用 `ProcessSpec`，覆盖 command、args、cwd、env、initial size。
- [x] 定义并使用 `CommandRunner`，负责从 spec 到 runtime execution。
- [x] stdin relay 已实现；自动化测试覆盖 Ctrl+C byte 拦截与普通输入转发，完整交互需人工 terminal 验证。
- [x] stdout relay 可用；smoke 输出 `TERM_BRIDGE_SMOKE_OK` 和 long-output marker。
- [x] TermBridge 自身日志仍写入 log file，不走用户 stdout。
- [x] terminal resize relay 已实现为 polling watcher；未做人工 resize 验证。
- [x] Ctrl+C escalation 代码已实现；真实 Ctrl+C 人工验证未执行。
- [x] interrupt、close PTY、kill process tree 在代码边界上是独立方法。
- [x] 用户命令退出码可传递为 `termbridge` exit code；`cmd.exe /C exit /b 7` 验证通过。
- [x] TermBridge 自身 usage/config/internal/runtime error 仍使用保留 exit code。
- [ ] close / interrupt 后不留下基础 shell 子进程：本轮未做进程残留人工检查。
- [x] 长输出不会导致 read loop 死锁；20000 行 long-output smoke 验证通过。
- [x] `cmd.exe Ctrl+C` 未被误判为通用成功；当前实现只提供 escalation 语义，不把 cmd.exe interrupt 作为成功样例。
- [x] `go test ./...` 通过。
- [x] 提供并运行一组可重复执行的本地检查命令。
- [x] Claude Code / Codex 完成 `--version` smoke；真实 TUI 交互仍待人工验证。

## Command results

### just check

命令：

```powershell
just check
```

结果：通过。

输出摘要：

```text
go fmt ./...
go vet ./...
go test ./...
?    termbridge-go/cmd/pty-spike        [no test files]
?    termbridge-go/cmd/pty-testprogram  [no test files]
?    termbridge-go/cmd/termbridge       [no test files]
ok   termbridge-go/internal/app         (cached)
ok   termbridge-go/internal/cli         (cached)
ok   termbridge-go/internal/config      (cached)
ok   termbridge-go/internal/errors      (cached)
ok   termbridge-go/internal/logging     (cached)
ok   termbridge-go/internal/process     (cached)
?    termbridge-go/internal/pty         [no test files]
?    termbridge-go/internal/pty/gopty   [no test files]
ok   termbridge-go/internal/runner      (cached)
?    termbridge-go/internal/version     [no test files]
```

### just build

命令：

```powershell
just build
```

结果：通过。

输出：

```text
mkdir -p bin
go build -o bin/termbridge.exe ./cmd/termbridge
```

### pwsh cwd smoke

命令：

```powershell
.\bin\termbridge.exe -- pwsh -NoLogo -NoProfile -Command "(Get-Location).Path"
```

结果：通过。

输出包含：

```text
D:\SourceCodes\mywork\TermBridge-go
```

说明：ConPTY 输出中包含 terminal control sequences，核心 cwd 输出正确。

### exit code passthrough

命令：

```powershell
.\bin\termbridge.exe -- cmd.exe /C exit /b 7; $code=$LASTEXITCODE; "LASTEXITCODE=$code"; if ($code -ne 7) { exit 1 } exit 0
```

结果：通过。

输出包含：

```text
LASTEXITCODE=7
```

### command output smoke

命令：

```powershell
.\bin\termbridge.exe -- cmd.exe /C echo TERM_BRIDGE_SMOKE_OK
```

结果：通过。

输出包含：

```text
TERM_BRIDGE_SMOKE_OK
```

### long output smoke

命令：

```powershell
go build -o .tmp\pty-testprogram.exe .\cmd\pty-testprogram
.\bin\termbridge.exe -- .\.tmp\pty-testprogram.exe --mode long-output --lines 20000
```

结果：通过。

输出过大，完整输出保存到：

```text
C:\Users\wangm25\.claude\projects\D--SourceCodes-mywork-TermBridge-go\77e1e391-c349-4883-bd14-aa45f084edba\tool-results\bqxzfyw5o.txt
```

确认 marker：

```text
TERM_BRIDGE_LONG_OUTPUT_END
```

### Claude Code version smoke

命令：

```powershell
.\bin\termbridge.exe -- claude --version
```

结果：通过。

输出包含：

```text
2.1.172 (Claude Code)
CLAUDE_EXIT=0
```

### Codex version smoke

命令：

```powershell
.\bin\termbridge.exe -- codex --version
```

结果：通过。

输出包含：

```text
codex-cli 0.139.0
CODEX_EXIT=0
```

## Missed or expanded scope

### Missed / incomplete

- 未执行 Claude Code / Codex 真实 TUI 人工交互验证。
- 未执行 Ctrl+C 真实人工验证。
- 未执行 close / interrupt 后进程残留人工检查。
- 未添加 `internal/pty/gopty` 独立 adapter integration test 文件；当前通过 CLI smoke 间接覆盖 adapter。

### Expanded

- `go.mod` 新增 direct dependency `golang.org/x/term v0.44.0`，用于 terminal raw mode / size 获取。
- verification 期间构建了 `.tmp/pty-testprogram.exe` 和 `bin/termbridge.exe` 作为验证产物。

## Risks

1. ConPTY 输出包含 terminal control sequences。真实 terminal 中通常可接受，但未来若支持 non-TTY capture 或机器解析，需要单独处理。
2. Ctrl+C 在 `cmd.exe`、PowerShell、pwsh、Claude Code、Codex 中行为可能不同，当前仍需人工验证。
3. `KillTree` 当前主要是 `Process.Kill()` fallback，复杂 Agent 子孙进程 cleanup 仍需 M5 hardening。
4. `internal/pty/gopty` 当前缺少独立 package integration test，建议后续补齐。
5. 当前 working tree 包含 README / docs/design 等非 M2 runtime 代码变更，提交前需要确认提交边界。

## Incomplete items

- Claude Code / Codex 真实 TUI 交互验证。
- Ctrl+C interrupt / close / kill escalation 的人工验证记录。
- process tree cleanup 残留检查。
- 独立 `internal/pty/gopty` integration tests。

## Conclusion

M2 PTY Command Runner MVP 的核心目标已达成：`termbridge [options] -- <command...>` 已通过正式 PTY runtime 路径运行用户命令，支持 stdout relay、exit code passthrough、cwd smoke、long-output smoke，并通过 `just check` 与 `just build`。

本轮可以作为 M2 MVP 进入用户验收，但不应视为 M5 hardening 完成。真实 Agent TUI、Ctrl+C 行为和复杂进程树 cleanup 仍需后续人工验证与 hardening。