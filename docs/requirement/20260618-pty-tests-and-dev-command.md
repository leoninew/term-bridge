# PTY 测试收敛与 dev 命令需求
最后修改时间: 2026-06-18 08:22:07

Review status: Accepted

## Background

M2 已经把 `termbridge [options] -- <command...>` 从 M1 placeholder 切换到正式 PTY runtime，并在验证文档中记录了一个未完成补强点：`internal/pty/gopty` 目前缺少独立 adapter integration test。

当前仓库里还保留了两个早期验证用命令：

```text
cmd/pty-spike/main.go
cmd/pty-testprogram/main.go
```

它们曾用于探索 `github.com/aymanbagabas/go-pty`、Windows ConPTY、长输出、cwd、env、resize、interrupt、cleanup、agent version smoke 等行为。但这些命令现在不应该继续作为正式 runtime 之外的孤立 spike 工具存在；相关验证能力应收敛到本项目自己的 PTY 包测试中。

同时，开发过程中需要一个轻量 dev 入口，用于直接通过源码运行 TermBridge CLI：

```text
go run cmd/termbridge/main.go -- <command...>
```

## Goal

本变更目标：把早期 PTY spike / testprogram 的有价值验证能力整合到项目正式测试边界，并补充 `just dev` 开发入口。

具体目标：

1. 将 `cmd/pty-spike` 中可自动化、仍属于 M2 PTY runtime 的验证场景，迁移或改写为针对本项目 `internal/pty` / `internal/pty/gopty` / `internal/runner` 边界的测试。
2. 将 `cmd/pty-testprogram` 的测试 helper 能力迁移到测试代码内部，避免为了测试再依赖一个正式 `cmd/` 下的辅助二进制入口。
3. 保留有价值的自动化覆盖：基础命令、cwd、env、unicode/ANSI 输出、长输出、resize、exit code、基础 cleanup 或 close 语义。
4. 对难以稳定自动化的真实交互项，例如 Claude Code / Codex 长时间 TUI、复杂 process tree cleanup、不同 shell 的 Ctrl+C 差异，继续记录为人工验证或后续 hardening 风险，不强行写成脆弱测试。
5. 添加 `just dev`，用于运行：

```text
go run cmd/termbridge/main.go --
```

并允许调用者在 `just dev` 后继续传入用户命令参数。

## Non-goal

本变更不包括：

1. 不改变 `termbridge [options] -- <command...>` 作为本地一等入口的 CLI contract。
2. 不引入 `termbridge <command...>` 语法糖。
3. 不把 spike 中所有实验场景无差别搬进自动化测试，避免测试过慢、过脆或依赖本机 Agent 登录状态。
4. 不在本阶段完成 M5 级别 process tree hardening。
5. 不要求真实 Claude Code / Codex TUI 长时间交互自动化。
6. 不执行 git 写操作。

## User scenarios

### 场景一：维护者运行项目测试

作为维护者，我希望 `go test ./...` 覆盖正式 PTY adapter 的基本行为，而不是依赖 `cmd/pty-spike` 人工运行结果。

### 场景二：维护者验证长输出

作为维护者，我希望长输出验证位于测试代码内，能被 `go test` 或项目约定检查命令调用，而不是先构建 `.tmp/pty-testprogram.exe` 再手动 smoke。

### 场景三：维护者使用 dev 入口快速运行 CLI

作为维护者，我希望运行类似：

```powershell
just dev pwsh -NoLogo -NoProfile
```

实际执行：

```powershell
go run cmd/termbridge/main.go -- pwsh -NoLogo -NoProfile
```

以便在不先 `just build` 的情况下调试 TermBridge CLI。

## Acceptance

- [ ] `cmd/pty-spike` 不再作为正式测试验证入口保留；其仍有价值的场景已迁移或明确取舍。
- [ ] `cmd/pty-testprogram` 不再作为正式测试 helper 二进制保留；长输出、unicode、exit-code 等 helper 能力已进入测试代码或测试专用 helper。
- [ ] 新增或调整测试覆盖 `internal/pty/gopty` 的最小真实 adapter 路径。
- [ ] 测试覆盖基础命令输出、cwd、env 或等价环境继承行为、resize、长输出、exit code 中适合自动化的部分。
- [ ] 难以稳定自动化的 Ctrl+C / Agent TUI / 复杂进程树清理不被误标为已完全自动化；需要在 verification 中记录取舍。
- [ ] `just dev` 存在，并运行 `go run cmd/termbridge/main.go --`。
- [ ] `just dev` 支持把 recipe 后续参数作为用户命令传入。
- [ ] `just check` 通过。
- [ ] 不新增正式 runtime 对 `cmd/pty-spike` 或 `cmd/pty-testprogram` 的依赖。

## Open questions

暂无必须阻塞推进的问题。

需要用户关注的非阻塞事项：

1. `just dev` 在 justfile 中可实现为可变参数 recipe；不同 shell / just 版本对参数转义有差异，实施时需用当前项目环境验证。
2. `internal/pty/gopty` integration test 会真实启动 shell / process，可能比纯 unit test 慢；需要控制测试规模，避免 `go test ./...` 变得不稳定。
3. spike 中的 owner-crash、descendant-cleanup、multi-attach 等场景更接近后续 Workspace / M5 hardening，不建议全部纳入 M2 自动化。

## Decisions

当前已明确：

1. 这是一项 M2 后续测试收敛与开发入口补强，不改变 runtime 产品范围。
2. 当前流程采用 light / 轻量模式即可：需求 / Requirement → 实现 / Implementation → 验证 / Verification。
3. 迁移方向是“测试围绕正式包”，不是继续保留 `cmd/` 下的 spike 程序。
4. `just dev` 的目标命令是 `go run cmd/termbridge/main.go --`。

## Risk

1. PTY integration tests 真实启动 shell，若依赖本机 shell 可用性或 terminal 行为，可能在不同 Windows 环境表现不一致。
2. 过度迁移 spike 场景会让测试变慢、变脆，反而降低开发体验。
3. 删除 `cmd/pty-testprogram` 前必须确保长输出等关键 marker 覆盖已经有替代测试。
4. `just dev` 参数转发如果处理不严谨，可能破坏带空格或特殊字符的用户命令。

## User review notes

用户要求：原来的测试 `cmd/pty-testprogram`、`cmd/pty-spike` 都整合成针对本项目 pty 包的测试，并添加 `just dev` 以运行 `go run cmd/termbridge/main.go --`。

用户补充：选择性迁移适合进入单元 / 集成测试的场景；优先选择没有明显副作用、不会明显耗时的测试。`just dev` 后续参数合法性不纳入本次考虑，只负责拼接传递。
