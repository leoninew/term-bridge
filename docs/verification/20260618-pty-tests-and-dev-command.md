# PTY 测试收敛与 dev 命令验证
最后修改时间: 2026-06-18 08:48:59

Review status: Accepted

## Requirement alignment

验证对象：

```text
docs/requirement/20260618-pty-tests-and-dev-command.md
```

本次变更对齐用户要求：

```text
原来的测试 cmd/pty-testprogram cmd/pty-spike 要都整合成针对本项目 pty 包的测试，添加 just dev 以运行 go run cmd/termbridge/main.go --
```

以及后续补充约束：

```text
选择性地把适合的测试搬入单元测试，那些没有明显副作用的、明显耗时的。just dev 后续参数合法性不在考虑，我们只负责拼接
```

对齐情况：

- `cmd/pty-spike/main.go` 已删除，不再作为正式测试入口。
- `cmd/pty-testprogram/main.go` 已删除，不再作为正式 helper 二进制。
- 新增 `internal/pty/gopty/manager_test.go`，把适合自动化、耗时可控、无明显副作用的 PTY adapter 场景迁移到正式包测试。
- 新增 `just dev`，拼接为 `go run cmd/termbridge/main.go -- {{args}}`。
- 未迁移 owner-crash、descendant-cleanup、multi-attach、Agent TUI 长交互、真实 Ctrl+C 差异等重副作用或脆弱场景，符合“选择性迁移”的要求。

## Spec alignment

light / 轻量模式没有单独 spec 文档；按 requirement / 需求核对。

## Plan alignment

light / 轻量模式没有单独 plan 文档；按 requirement / 需求核对。

## Actual diff summary

本次实际变更：

```text
D  cmd/pty-spike/main.go
D  cmd/pty-testprogram/main.go
A  docs/requirement/20260618-pty-tests-and-dev-command.md
A  docs/verification/20260618-pty-tests-and-dev-command.md
A  internal/pty/gopty/manager_test.go
M  justfile
```

核心实现摘要：

- 删除早期 PTY spike 命令和测试 helper 命令。
- 新增 Windows-only `internal/pty/gopty` integration tests，真实启动 PowerShell / pwsh 并通过 TermBridge-owned `gopty.Manager` 运行。
- 测试覆盖 unicode / ANSI 输出、cwd、env、长输出、用户 exit code、resize。
- `justfile` 新增可变参数 recipe：

```just
dev *args:
    go run cmd/termbridge/main.go -- {{args}}
```

## Expected vs actual changed files

### Expected

```text
cmd/pty-spike/main.go
cmd/pty-testprogram/main.go
internal/pty/gopty/*_test.go
justfile
docs/requirement/20260618-pty-tests-and-dev-command.md
docs/verification/20260618-pty-tests-and-dev-command.md
```

### Actual

实际变更与预期一致。

## Acceptance criteria checklist

- [x] `cmd/pty-spike` 不再作为正式测试验证入口保留；其仍有价值的场景已迁移或明确取舍。
- [x] `cmd/pty-testprogram` 不再作为正式测试 helper 二进制保留；长输出、unicode、exit-code 等 helper 能力已进入测试代码。
- [x] 新增测试覆盖 `internal/pty/gopty` 的最小真实 adapter 路径。
- [x] 测试覆盖基础命令输出、cwd、env、resize、长输出、exit code 中适合自动化的部分。
- [x] 难以稳定自动化的 Ctrl+C / Agent TUI / 复杂进程树清理没有被误标为已完全自动化；本验证记录了取舍。
- [x] `just dev` 存在，并运行 `go run cmd/termbridge/main.go --`。
- [x] `just dev` 支持把 recipe 后续参数拼接为用户命令传入。
- [x] `just check` 通过。
- [x] 不新增正式 runtime 对 `cmd/pty-spike` 或 `cmd/pty-testprogram` 的依赖。

## Command results

### go test ./internal/pty/gopty

命令：

```powershell
go test ./internal/pty/gopty
```

结果：通过。

输出：

```text
ok  	termbridge-go/internal/pty/gopty	(cached)
```

### go test ./...

命令：

```powershell
go test ./...
```

结果：通过。

输出摘要：

```text
?   	termbridge-go/cmd/termbridge	[no test files]
ok  	termbridge-go/internal/app	(cached)
ok  	termbridge-go/internal/cli	(cached)
ok  	termbridge-go/internal/config	(cached)
ok  	termbridge-go/internal/errors	(cached)
ok  	termbridge-go/internal/logging	(cached)
ok  	termbridge-go/internal/process	(cached)
?   	termbridge-go/internal/pty	[no test files]
ok  	termbridge-go/internal/pty/gopty	(cached)
ok  	termbridge-go/internal/runner	(cached)
?   	termbridge-go/internal/version	[no test files]
```

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
?   	termbridge-go/cmd/termbridge	[no test files]
ok  	termbridge-go/internal/app	(cached)
ok  	termbridge-go/internal/cli	(cached)
ok  	termbridge-go/internal/config	(cached)
ok  	termbridge-go/internal/errors	(cached)
ok  	termbridge-go/internal/logging	(cached)
ok  	termbridge-go/internal/process	(cached)
?   	termbridge-go/internal/pty	[no test files]
ok  	termbridge-go/internal/pty/gopty	(cached)
ok  	termbridge-go/internal/runner	(cached)
?   	termbridge-go/internal/version	[no test files]
```

### just dev dry-run

命令：

```powershell
just --dry-run dev pwsh -NoLogo -NoProfile
```

结果：通过。

输出：

```text
go run cmd/termbridge/main.go -- pwsh -NoLogo -NoProfile
```

### just dev command smoke

命令：

```powershell
just dev pwsh -NoLogo -NoProfile -Command "Write-Output TERM_BRIDGE_DEV_OK"
```

结果：通过。

输出包含：

```text
TERM_BRIDGE_DEV_OK
```

说明：ConPTY 输出中包含 terminal control sequences，核心 marker 正确。

### just dev exit-code observation

命令：

```powershell
just dev cmd.exe /C exit /b 7; code=$?; printf 'exit=%s\n' "$code"
```

结果：命令按预期拼接并执行，但 `go run` 对被运行程序的非零 exit status 会包装为自身 exit code `1`。

输出包含：

```text
go run cmd/termbridge/main.go -- cmd.exe /C exit /b 7
exit status 7
error: recipe `dev` failed on line 29 with exit code 1
exit=1
```

这不违反本次需求，因为用户已明确 `just dev` 后续参数合法性和高级语义不纳入考虑，本次只负责拼接。真实 exit code passthrough 仍应使用 built binary 验证。

## Missed or expanded scope

### Missed / intentionally not migrated

以下场景未迁移到自动化测试，属于有意取舍：

- Claude Code / Codex 真实 TUI 长时间交互。
- 不同 shell / Agent CLI 的 Ctrl+C 行为差异。
- owner-crash cleanup。
- descendant-cleanup。
- multi-attach。
- 复杂 process tree cleanup。

原因：这些场景具有明显环境依赖、耗时或副作用，更适合后续 M5 / Workspace hardening 或人工验证记录。

### Expanded

- 新增了独立需求文档 `docs/requirement/20260618-pty-tests-and-dev-command.md`，用于记录本次 light / 轻量模式边界。
- 新增了本验证文档。

## Risks

1. `internal/pty/gopty/manager_test.go` 是 Windows-only 测试，使用 `//go:build windows`。
2. `internal/pty/gopty` 测试真实启动 `pwsh.exe` 或 `powershell.exe`；如果机器缺少 PowerShell，会跳过相关测试。
3. PTY 输出包含 terminal control sequences，测试只断言核心 marker。
4. `just dev` 只做参数拼接，不处理复杂参数合法性、转义策略或 `go run` 非零 exit code 包装问题。
5. 删除 `cmd/pty-spike` 后，历史 feasibility 文档中的手动命令记录仍指向旧 spike；这些是历史记录，不代表当前验证入口。

## Incomplete items

无阻塞未完成项。

后续可选补强：

- 若需要保留重型人工验证能力，可单独设计 M5 / hardening 工具，而不是恢复 `cmd/pty-spike`。
- 若需要 `just dev` 保真透传用户命令 exit code，应改用 built binary 或额外脚本；当前需求未要求。

## Conclusion

本次 PTY 测试收敛与 `just dev` 开发入口补强已完成：旧 spike/helper 命令被移除，适合自动化的 PTY adapter 场景已进入 `internal/pty/gopty` 测试，`just dev` 能按要求拼接 `go run cmd/termbridge/main.go -- <command...>`，并且 `go test ./...` 与 `just check` 均通过。
