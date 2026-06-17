# M1 Go CLI Skeleton 验证
最后修改时间: 2026-06-17 16:54:42

Review status: Accepted

## Requirement alignment

验证对象：

```text
docs/requirement/20260617-m1-go-cli-skeleton.md
```

M1 目标是建立 `termbridge` CLI 的稳定工程骨架，为 M2 PTY Command Runner 提供边界。

本次实现对齐以下需求：

- `cmd/termbridge` 作为唯一正式 CLI entrypoint。
- CLI 合同为 `termbridge [options] -- <command...>`。
- 暴露 `--cwd`、`--help`、`--version`。
- 不暴露 `--config`、`--log-level`、`--log-format`、`--log-dir` 等细粒度配置 flags。
- 使用 `viper` 建立 `.termbridge.yaml` 配置边界。
- 配置发现顺序为 effective cwd -> home -> defaults。
- 配置文件存在但读取、解析、未知 key、非法值失败时 fail fast。
- 使用 `log/slog` 建立日志边界。
- 建立错误分类和 exit code 约定。
- 建立 `internal/app` 薄层，为未来 GUI / Gateway 复用预留空间。
- 提供 `configs/termbridge.example.yaml`。
- 提供 `justfile`，建立主流 Go fmt / lint / test / build 入口。
- M1 不实现 PTY Command Runner。

## Spec alignment

本功能没有独立 M1 spec 文档；按轻量模式 / light，依据 M1 requirement 和 `docs/plan/20260617-roadmap-refresh.md` 验证。

对齐路线原则：

```text
CLI first
GUI optional container
Web technical spike before Gateway
Gateway Web as formal remote product surface
```

当前实现没有引入 GUI、Web 或 Gateway，只预留了 `internal/app` 应用编排薄层。

## Plan alignment

对齐 `docs/plan/20260617-roadmap-refresh.md` 的 M1：

```text
M1: Go CLI Skeleton
```

当前完成项：

- `termbridge` binary entrypoint。
- command-line parser。
- `--help`。
- `--version`。
- `--cwd`。
- `--` separator parsing。
- viper config boundary。
- slog logging boundary。
- basic error model。
- test command。
- just-based fmt / lint / test / build 入口。
- binary 输出到 `bin/termbridge.exe`。

未进入 M2：

- 未实现 PTY Command Runner。
- 未接入 `go-pty` 到正式 runtime。
- 未实现 Ctrl+C interrupt / close / kill process tree。

## Actual diff summary

本次 M1 实现涉及：

```text
cmd/termbridge/main.go
internal/app/app.go
internal/app/app_test.go
internal/cli/cli.go
internal/cli/cli_test.go
internal/config/config.go
internal/config/config_test.go
internal/errors/errors.go
internal/errors/errors_test.go
internal/logging/logging.go
internal/logging/logging_test.go
internal/version/version.go
configs/termbridge.example.yaml
justfile
docs/requirement/20260617-m1-go-cli-skeleton.md
docs/verification/20260617-m1-go-cli-skeleton.md
go.mod
go.sum
```

说明：

- `go.mod` / `go.sum` 因引入 `github.com/spf13/viper` 更新。
- `justfile` 参考 `D:\SourceCodes\mywork\pomelo-orbit\justfile` 的 Go 部分，提供 `fmt`、`lint`、`test`、`check`、`build` 等入口。
- `build` 输出位置已按要求调整为 `bin/termbridge.exe`。
- `cmd/pty-spike` / `cmd/pty-testprogram` 未作为正式 runtime 修改。
- `docs/roadmap/project-milestones.md` 未恢复。

## Expected vs actual changed files

### Expected

```text
cmd/termbridge
internal/cli
internal/app
internal/config
internal/logging
internal/errors
internal/version
configs/termbridge.example.yaml
justfile
docs/requirement/20260617-m1-go-cli-skeleton.md
docs/verification/20260617-m1-go-cli-skeleton.md
go.mod
go.sum
```

### Actual

实际变更符合上述范围。

## Acceptance criteria checklist

- [x] 存在 M1 requirement 文档，明确 CLI、配置、日志、错误、exit code、测试边界。
- [x] `cmd/termbridge` 是唯一正式 CLI entrypoint。
- [x] `termbridge --help` 输出 CLI 合同、选项和示例。
- [x] `termbridge --version` 输出版本与 runtime 信息。
- [x] `termbridge [options] -- <command...>` 能解析 command 与 args，且不吞掉用户参数。
- [x] `--cwd` 默认使用当前目录；显式传入时必须校验目录存在。
- [x] 缺少 `--`、缺少 command、非法 cwd、未知 TermBridge 选项时 fail fast。
- [x] 使用 `viper` 建立配置边界：明确配置来源、优先级、`.termbridge.yaml` 自动发现和 fail-fast 规则；不使用 `--config <path>`。
- [x] 使用 `log/slog` 建立日志边界：支持主流日志级别/格式配置，开发阶段写入 `logs/` 目录。
- [x] 建立错误模型：区分 usage error、config error、internal error、future runtime error。
- [x] 建立 exit code 约定。
- [x] 建立 package 边界，覆盖 `internal/cli`、`internal/app`、`internal/config`、`internal/logging`、`internal/version`、`internal/errors`。
- [x] 提供 `configs/termbridge.example.yaml`，明确 M1 支持的配置 schema。
- [x] 有最小单元测试覆盖 CLI parsing、cwd 校验、usage error、version/help 行为、Run exit code、unknown config key、日志不污染 stdout 并写入 log file。
- [x] 提供主流 Go 工具入口：`go fmt`、`go vet`、`go test`、`go build`，并通过 `justfile` 暴露。
- [x] M1 范围内的 targeted tests 通过。
- [x] `just check` 通过。
- [x] `just build` 通过，二进制输出到 `bin/termbridge.exe`。

## Command results

### just --list

命令：

```powershell
just --list
```

结果：通过。

```text
Available recipes:
    build
    check
    clean
    fmt
    install
    lint
    lint-extra
    test
```

### just check

命令：

```powershell
just check
```

结果：通过。

输出：

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
?    termbridge-go/internal/version     [no test files]
```

说明：

```text
go-pty 已从本地 replace 切换为 Go module 依赖 github.com/aymanbagabas/go-pty v0.2.3，因此全量 go vet ./... / go test ./... 不再被缺失 ./go-pty 目录阻塞。
```

### Targeted M1 tests

命令：

```powershell
go test ./internal/... ./cmd/termbridge
```

结果：通过。

```text
ok   termbridge-go/internal/app      (cached)
ok   termbridge-go/internal/cli      (cached)
ok   termbridge-go/internal/config   (cached)
ok   termbridge-go/internal/errors   (cached)
ok   termbridge-go/internal/logging  (cached)
?    termbridge-go/internal/version  [no test files]
?    termbridge-go/cmd/termbridge    [no test files]
```

### just build

命令：

```powershell
just build
```

结果：通过。

```text
mkdir -p bin
go build -o bin/termbridge.exe ./cmd/termbridge
```

生成：

```text
bin/termbridge.exe
```

### Built binary smoke: version

命令：

```powershell
.\bin\termbridge.exe --version
```

结果：通过。

```text
termbridge dev windows/amd64 go=go1.25.8 commit=unknown built=unknown
```

### Built binary smoke: command placeholder

命令：

```powershell
.\bin\termbridge.exe --cwd . -- pwsh
```

结果：符合 M1 预期，返回 exit code 1。

```text
error: termbridge command runner is not implemented yet

cwd: D:\SourceCodes\mywork\TermBridge-go
command: pwsh
```

### CLI smoke: help

命令：

```powershell
go run .\cmd\termbridge --help
```

结果：通过。

输出包含：

```text
Usage:
  termbridge [options] -- <command> [args...]

Options:
  --cwd <dir>     working directory for the command; defaults to current directory
  --version       show version
  --help          show help

Config files:
  TermBridge reads .termbridge.yaml from --cwd/current directory, then from the user home directory.
```

确认未暴露：

```text
--config
--log-level
--log-format
--log-dir
```

### CLI smoke: version via go run

命令：

```powershell
go run .\cmd\termbridge --version
```

结果：通过。

```text
termbridge dev windows/amd64 go=go1.25.8 commit=unknown built=unknown
```

### CLI smoke: command placeholder via go run

命令：

```powershell
go run .\cmd\termbridge --cwd . -- pwsh
```

结果：符合 M1 预期，返回 exit code 1。

```text
error: termbridge command runner is not implemented yet

cwd: D:\SourceCodes\mywork\TermBridge-go
command: pwsh
exit status 1
```

### CLI smoke: usage error via go run

命令：

```powershell
go run .\cmd\termbridge claude
```

结果：符合 M1 预期，CLI 内部返回 usage exit code 2；`go run` 外层显示 exit code 1 属于 Go tool 包装行为。

```text
error: missing -- before command
...
exit status 2
```

## Missed or expanded scope

### 未完成但符合 M1 非目标

- 未实现 PTY Command Runner。
- 未接入 `go-pty` 到正式 runtime。
- 未实现 Ctrl+C interrupt / close / kill process tree。
- 未实现 Session / Workspace 模型。
- 未实现 GUI。
- 未实现 Web terminal spike。
- 未实现 Gateway。

### 范围扩展

相比最小 CLI skeleton，本次额外加入：

```text
internal/app 薄层
configs/termbridge.example.yaml
justfile
build info 注入点
```

这些扩展已由用户明确采纳，用于为未来 GUI / Gateway 复用预留空间，并确保本阶段具备主流 Go lint/format/test/build 入口。

## Risks

### 风险一：M1 command placeholder 仍返回 general failure

`termbridge -- <command...>` 当前返回 runtime placeholder，exit code 为 1。这是 M1 预期行为；M2 需要替换为真实 PTY Command Runner。

### 风险二：配置自动发现未做 merge

M1 只选择第一个存在的配置文件：local 优先，home 其次，不做 merge。这符合当前决策；后续如需用户级默认 + 项目覆盖，需要单独设计。

### 风险三：日志目录相对 effective cwd

默认 `logs/` 解析到 effective cwd 下。M2 运行用户命令时需保持 TermBridge 自身日志目录语义稳定，避免被用户命令工作目录变化影响。

### 风险四：`go run` 包装 exit code

CLI 内部 usage error 映射为 exit code 2，但通过 `go run` 执行时外层工具显示 exit code 1 并打印 `exit status 2`。真实 binary 验证时应直接运行编译产物。

### 风险五：go-pty 依赖版本需要后续跟踪

当前已移除本地 replace，改为使用 Go module 依赖：

```text
github.com/aymanbagabas/go-pty v0.2.3
```

该版本解除项目级 `just check` 的本地目录阻塞。后续进入 M2 PTY Command Runner 时，需要确认该发布版本与前期 spike 使用的源码行为保持一致，尤其是 Windows ConPTY、cwd、resize、close 和 process cleanup 行为。

## Incomplete items

M1 CLI skeleton 自身无阻塞 incomplete items。

进入 M2 前仍需明确：

```text
PTY Command Runner 的 ProcessSpec / PTYSession / InterruptStrategy 设计。
```

## Conclusion

M1 Go CLI Skeleton 的目标实现已通过验证；`termbridge` CLI、配置发现、日志、错误模型、build info、`internal/app` 薄层、just check 和 just build 入口均可用。

本次验证结论为：

```text
M1 verification: pass
Project-wide just check: pass
```

可以确认 M1 交付。
