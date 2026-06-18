# M2 PTY Command Runner MVP 计划
最后修改时间: 2026-06-17 19:42:48

Review status: Accepted

## Requirement basis

本计划基于：

- `docs/requirement/20260617-m2-pty-command-runner-mvp.md`
- `docs/plan/20260617-roadmap-refresh.md`
- `docs/spec/20260617-go-pty-feasibility.md`
- `docs/verification/20260617-m1-go-cli-skeleton.md`

当前 M1 状态：`cmd/termbridge/main.go` → `internal/cli.Run` → `internal/app.Run` 路径已建立，但 `internal/app.Run` 仍返回 `termbridge command runner is not implemented yet`。

M2 目标是将该 placeholder 替换为真实 PTY Command Runner MVP，同时保持：

```text
termbridge [options] -- <command...>
```

作为唯一规范入口。

## Implementation approach

采用以下分层：

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

原则：

1. `internal/cli` 只负责 CLI contract、help/version、错误输出和最终 exit code。
2. `internal/app` 只做 config、logging、ProcessSpec 构造和 runner wiring。
3. `internal/runner` 负责 command lifecycle、relay、interrupt、cleanup、exit code。
4. `internal/pty` 定义 TermBridge 自己的 PTY abstraction。
5. `internal/pty/gopty` 是唯一直接依赖 `go-pty` 的正式 runtime adapter。
6. `cmd/pty-spike` 和 `cmd/pty-testprogram` 只作为经验和验证 helper，不作为正式 runtime 依赖。

## Files to change

### Existing files

```text
cmd/termbridge/main.go
internal/cli/cli.go
internal/cli/cli_test.go
internal/app/app.go
internal/app/app_test.go
```

### New packages / files

```text
internal/process/spec.go
internal/process/resolve.go
internal/process/exit.go
internal/process/*_test.go

internal/pty/pty.go
internal/pty/gopty/manager.go
internal/pty/gopty/manager_windows.go
internal/pty/gopty/*_test.go

internal/runner/runner.go
internal/runner/relay.go
internal/runner/interrupt.go
internal/runner/terminal.go
internal/runner/*_test.go
```

具体文件可在实现中合并或拆分，但必须保留上述边界。

## Implementation steps

### Step 1: CLI / app IO and exit-code contract

目标：让 runtime 能接收 stdin，并让用户命令 exit code 作为最终 CLI exit code。

实现：

1. 将 `cmd/termbridge/main.go` 改为向 `cli.Run` 传入 `os.Stdin`、`os.Stdout`、`os.Stderr`。
2. 扩展 `internal/cli.Run` 签名。
3. 扩展 `internal/app.Options`，加入 stdin/stdout/stderr 或等价 IO 对象。
4. 扩展 `internal/app.Result`，加入用户命令 `ExitCode`。
5. 保持 help/version 不触发 app/config/logging。
6. 保持 usage/config/internal/runtime 错误仍走 `internal/errors` 的保留 exit code。
7. 用户命令非零退出不打印 TermBridge `error:` 前缀。

### Step 2: ProcessSpec and executable resolve

目标：把用户命令启动前语义集中到 `internal/process`。

实现：

1. 定义 `ProcessSpec`：command、args、cwd、env、initial size。
2. 定义 `TerminalSize`。
3. 定义 spec validation。
4. 使用 `exec.LookPath` 在启动前 resolve executable path。
5. 保留 env 字段，M2 默认继承 `os.Environ()`，暂不新增 env 配置。
6. 定义 exit classification，区分 normal exit、stopped、closed cleanup、runtime failure。

### Step 3: PTY abstraction

目标：隔离 `go-pty` concrete type。

实现：

1. 在 `internal/pty` 定义 `Manager` / `Session` interface。
2. `Session` 至少表达：`Read`、`Write`、`Resize`、`Interrupt`、`Close`、`KillTree`、`Wait`。
3. `Interrupt`、`Close`、`KillTree` 必须是独立语义。
4. 为 runner 测试准备 fake session / fake manager。

### Step 4: Runner lifecycle

目标：实现 PTY Command Runner 编排。

实现：

1. 从 `ProcessSpec` 启动 PTY session。
2. relay PTY output 到 CLI stdout。
3. relay CLI stdin 到 PTY input。
4. 处理 terminal resize，失败时记录风险但不破坏主链路。
5. 处理 Ctrl+C：soft interrupt → grace period → close → kill tree。
6. 正常退出后返回用户 command exit code。
7. runtime 启动失败或内部不可恢复错误返回 `apperrors.Runtime` 或 `apperrors.Internal`。
8. cleanup failure 记录日志；如果影响最终状态，应返回 runtime error，否则不覆盖已知用户 exit code。

### Step 5: go-pty adapter

目标：把可行性验证落到正式 adapter。

实现：

1. `internal/pty/gopty` import `github.com/aymanbagabas/go-pty`。
2. 使用 resolved executable path 启动 command。
3. 设置 `cmd.Dir` 为 effective cwd。
4. 设置 `cmd.Env` 为 inherited env。
5. 实现 read/write/resize/interrupt/close/wait。
6. `Interrupt` MVP 可写入 Ctrl+C byte，但必须保留语义边界。
7. close cleanup 场景下识别 Windows `0xc000013a`。
8. process tree kill 先实现基础 fallback，复杂 Agent 子孙进程清理留到 M5 hardening。

### Step 6: app wiring

目标：移除 placeholder。

实现：

1. `internal/app.Run` 保持 config load 和 logger init。
2. 构造 `ProcessSpec`。
3. 创建默认 `runner.CommandRunner`。
4. 调用 runner 并返回 `Result.ExitCode`。
5. 只在 TermBridge 自身错误时返回 error。

### Step 7: tests

目标：先用 fake 固定语义，再补真实 adapter 检查。

实现：

1. 替换 placeholder tests。
2. 增加 `internal/process` 单测。
3. 增加 `internal/runner` fake PTY 单测。
4. 增加 `internal/pty/gopty` 最小集成测试。
5. 保留现有 `t.TempDir()`、isolated home、stdout/stderr buffer 习惯。

## Verification plan

### Automated

```powershell
go test ./...
go vet ./...
```

或：

```powershell
just check
```

### Build

```powershell
just build
```

### Manual / smoke

```powershell
.\bin\termbridge.exe -- pwsh -NoLogo -NoProfile
.\bin\termbridge.exe --cwd D:\SourceCodes\mywork\TermBridge-go -- pwsh -NoLogo -NoProfile
.\bin\termbridge.exe -- cmd.exe /C exit /b 7
```

长输出：

```powershell
go build -o .tmp\pty-testprogram.exe .\cmd\pty-testprogram
.\bin\termbridge.exe -- .\.tmp\pty-testprogram.exe --mode long-output --lines 20000
```

人工验证：

```powershell
.\bin\termbridge.exe -- claude
.\bin\termbridge.exe -- codex
```

这些 Agent CLI 真实交互如果受登录或环境限制，应在 Implementation 汇报中记录，不在本阶段静默视为通过。

## Assumptions

1. Ctrl+C grace period M2 采用固定短值，暂不暴露配置。
2. Env support M2 只继承当前进程环境。
3. Resize 以最小可用实现为主，自动化验证不足时保留人工验证项。
4. `cmd.exe` interrupt 已知不稳定，不作为通用成功样例。
5. process tree cleanup M2 做基础能力，真实 Agent 子孙进程 hardening 留到 M5。

## Risks

1. Windows shell / Agent CLI 对 Ctrl+C 行为不一致。
2. `0xc000013a` 在 close cleanup 和 interrupt 场景下语义不同，容易误判。
3. 长输出可能暴露 read/write backpressure 问题。
4. 日志如果污染用户命令 stdout，会破坏 CLI 使用体验。
5. 过早引入 Workspace / daemon / detach 会扩大 M2 范围。
6. `go-pty v0.2.3` 与 spike 时本地验证行为可能存在细节差异。

## Rollback

1. 如果 app wiring 不稳定，可临时回退 placeholder，同时保留 `internal/process`、`internal/pty`、`internal/runner` 单测。
2. 如果 `go-pty` adapter 行为异常，问题限制在 `internal/pty/gopty`。
3. 如果 resize / raw mode 不稳定，优先保留 start/read/write/exit-code 主路径，并明确记录未完成项。
4. 如果 cleanup 无法可靠确认，不把用户命令退出误报为 TermBridge internal error；记录风险进入 M5。

## User review notes

用户已接受 Requirement 并要求进入 Plan / 计划。本计划作为 standard / 标准模式的 M2 实施依据。