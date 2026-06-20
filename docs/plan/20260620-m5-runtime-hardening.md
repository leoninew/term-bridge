# M5 Runtime Hardening 计划
最后修改时间: 2026-06-20 17:43:45

Review status: Accepted

## Basis

- Requirement: `docs/requirement/20260620-m5-runtime-hardening.md`，Review status: Accepted
- Spec: `docs/spec/20260620-m5-runtime-hardening.md`，Review status: Accepted
- Roadmap: `docs/plan/20260617-roadmap-refresh.md`
- Known risks: `docs/todo.md`

M5 采用严格模式 / strict。本计划只覆盖 Runtime Hardening，不进入 Gateway/M6，不改变 CLI 入口 `termbridge exec -- <command...>`。

## Implementation steps

### Step 1. 建立 M5 hardening 测试基线

目标：先用自动化测试锁定当前 runtime 行为，避免后续 hardening 改动破坏 CLI/Web 基础能力。

计划：

1. 扩展 runner 测试，覆盖 interrupt → close → kill escalation 的状态和 hook reason。
2. 扩展 gopty adapter 或测试替身，覆盖 Close / KillTree idempotency 和 Wait 结果解释。
3. 扩展 webterminal registry/runtime 测试，覆盖 attach、detach、close、exit、history replay、queue full reason。
4. 扩展 history writer 测试，覆盖 pending flush、interval flush、close flush、truncate flag、large output。

### Step 2. 实现 macOS / Unix process tree cleanup

目标：在 macOS 当前环境可实现并验证 process tree cleanup，不只 kill 直接父进程。

计划：

1. 在 go-pty adapter 的 Unix 平台文件中为进程设置独立 process group 或等价机制。
2. 让 `KillTree()` 对进程组发送终止信号，必要时 fallback 到 `Process.Kill()`。
3. 保持 `internal/pty.Session` 接口不变，平台逻辑封装在 `internal/pty/gopty`。
4. 增加 macOS / Unix 可运行测试或验证命令，覆盖 shell 启动子进程后 kill tree 的行为。

### Step 3. 实现 Windows process tree cleanup 能力

目标：M5 必做 Windows cleanup 能力；当前 macOS 环境先完成代码路径和可编译/可审查实现，Windows 环境后续核验。

计划：

1. 在 Windows 平台文件中实现 `KillTree()` 的 Windows-specific 策略。
2. 优先评估 Job Object；若 go-pty / command lifecycle 限制明显，则实现进程树枚举或 adapter fallback。
3. 为 Windows-specific 代码增加可编译边界和可读测试替身；不能在 macOS 本地完成真实核验的部分写入 verification incomplete items。
4. 在 manual verification checklist 中列出 Windows 必跑场景：shell 子进程、Claude/Codex、Ctrl+C escalation、close timeout、kill timeout。

### Step 4. 加固 CLI interrupt / close / kill 可观察性

目标：Ctrl+C、context cancel、close PTY、kill tree 的行为和状态记录明确。

计划：

1. 检查 `CommandRunner.Run` 当前状态机，补齐测试覆盖第一次 Ctrl+C、第二次 Ctrl+C、interrupt timeout、close timeout。
2. 验证 stderr 提示、`OnStopping` reason、`process.ExitResult`、exit code、forced/closed/stopped 标记一致。
3. 确保 `waitOutput` 不导致 runtime 卡死；必要时调整日志或超时策略。
4. 保持用户语义：Ctrl+C 是停止当前命令，不是 detach。

### Step 5. 实现 Web terminal backpressure 策略

目标：slow client 不拖死全局 PTY reader；大量输出不会造成无限内存增长或无说明的 client 丢失。

计划：

1. 明确并实现最小策略：每 client bounded queue + queued bytes；超过限制时 detach 或发送明确 state/reason。
2. 避免 publish path 对每个 client 做不必要 copy；建立 outbound binary payload 不可变约定。
3. 让 detach reason、日志和 attachment state 可观察。
4. 增加测试覆盖：slow client queue full、大 binary 输出、多 client 一快一慢、detach 后 runtime/history 继续工作。
5. 如 frontend 需要展示 reason，仅做最小协议/显示调整，不扩大为 Gateway 功能。

### Step 6. 加固 history 写盘和 replay

目标：大输出下 history 有上限、写盘有批处理策略、attach/replay 可用。

计划：

1. 保持并测试 bounded history：max lines、max bytes、max line bytes。
2. 检查当前 pending bytes 和 flush interval 策略是否满足大输出；必要时优化 `flush()` 构造和写盘频率。
3. 确保 `Close()` 必定 flush，`History()` 和 attach replay 可读到合理最新内容。
4. 在 session metadata 中正确反映 history truncated。

### Step 7. 验证 session/workspace metadata 与 runtime 状态一致

目标：进程正常退出、异常退出、interrupt、close、kill 后，session/workspace metadata、state、process、exit、history 和 logs 一致。

计划：

1. 扩展 app/session/state/store 测试，覆盖 failed/stopped/stopping/running 状态转换。
2. 覆盖 `process.Record` 的 PID、OwnerPID、Executable、CommandLine、Cwd、StartedAt。
3. 覆盖 `process.ExitRecord` 的 exit code、forced、closed、wait error、started/ended time。
4. 覆盖 workspace/session list 输出对 running/stopped/failed 的反映。

### Step 8. 建立 M5 自动化验证命令和人工验证清单

目标：M5 完成后可以重复判断是否进入 M6。

计划：

1. 自动化命令至少包括相关 Go 单元测试。
2. 增加可重复运行的 shell 压测命令或测试入口，用于大输出、20+ session、resize、long-running command。
3. 人工验证清单覆盖 Claude Code / Codex：启动、输入、输出、退出、Ctrl+C、close、force stop。
4. Windows 核验清单单独列出，标记当前 macOS 环境无法完成的项目。

## Files to change

预计会修改或新增：

1. `internal/runner/runner.go`
2. `internal/runner/interrupt.go`
3. `internal/runner/runner_test.go`
4. `internal/pty/pty.go`（仅在确需接口注释或测试辅助时修改）
5. `internal/pty/gopty/manager.go`
6. `internal/pty/gopty/manager_windows.go`
7. `internal/pty/gopty/manager_test.go`
8. 可能新增 `internal/pty/gopty/manager_unix.go` 或等价平台文件
9. `internal/webterminal/runtime.go`
10. `internal/webterminal/registry.go`
11. `internal/webterminal/registry_test.go`
12. `internal/history/writer.go`
13. `internal/history/writer_test.go`
14. `internal/session/*_test.go`
15. `internal/state/store_test.go`
16. `internal/app/app_test.go`
17. `docs/verification/20260620-m5-runtime-hardening.md`

如 Plan 执行时发现 frontend 必须展示 backpressure/closed reason，才最小修改 Web frontend 文件。

## Verification plan

### Automated tests

优先运行：

```text
go test ./internal/runner ./internal/pty/gopty ./internal/webterminal ./internal/history ./internal/session ./internal/state ./internal/app
```

最终回归：

```text
go test ./...
```

如增加 race-sensitive concurrent tests，可补充：

```text
go test -race ./internal/webterminal ./internal/runner
```

### Manual verification checklist

macOS 当前环境验证：

1. `termbridge exec -- <long-running shell command>` 可启动、输出、退出。
2. CLI Ctrl+C 第一次 soft interrupt，未退出时升级 close，再升级 kill tree。
3. shell 子进程树在 force stop 后不遗留。
4. `termbridge exec -- claude` 真实启动、输入、输出、退出、中断。
5. `termbridge exec -- codex` 真实启动、输入、输出、退出、中断。
6. local Web/workbench 创建 session、attach、输入、输出、resize、detach、close。
7. 大输出命令下 slow client 不拖死 runtime，detach/drop reason 可观察。
8. 高频 resize 不出现 panic、协议错误或 runtime 卡死。
9. history replay 在 attach 后可用，truncated 状态可解释。
10. workspace/session list 与真实 state/exit/process/history 一致。

Windows 后续环境核验：

1. shell 子进程树 force stop 后不遗留。
2. Ctrl+C / close / kill escalation 行为符合记录。
3. Claude Code / Codex 真实 TUI 能启动、交互、退出、中断。
4. Web session close 后进程树清理。
5. process/exit/state/history 与真实进程状态一致。

## Blockers

当前无阻塞实现的用户决策项。已确认：

1. Windows cleanup 是 M5 必做。
2. Web backpressure 是 M5 实现项。
3. Claude Code / Codex 当前环境已有，可做真实验证。

环境限制：Windows 真实核验不在当前 macOS 环境完成，需要在 verification 中单独标记。

## Assumptions

1. 不改变 `termbridge exec -- <command...>` CLI contract。
2. 不引入 Gateway/M6 概念或远程多用户能力。
3. Web/workbench 仍是 local auxiliary surface，不拥有 PTY/Process/Workspace runtime。
4. process tree cleanup 的平台细节封装在 go-pty adapter 层。
5. 自动化测试覆盖稳定 runtime 行为；真实 TUI 和 Windows 环境核验允许作为 manual verification 记录。

## Risks

1. Windows cleanup 代码可能需要在 Windows 环境迭代，macOS 上只能完成静态边界和非真实运行测试。
2. Backpressure 策略如果选择不当，可能造成用户可见输出丢失或频繁 detach；必须保证 reason 可观察。
3. Process group / Job Object 行为可能与 go-pty 内部实现冲突，需要小步实现和验证。
4. 大输出 + history + WebSocket 可能暴露 frontend rendering 瓶颈，M5 不应扩大到 Gateway 级重构。
5. Claude/Codex 真实验证受账号和外部 CLI 状态影响，verification 需要记录环境条件。

## Rollback

1. Process cleanup 改动应局限在 `internal/pty/gopty` 平台文件和 runner 状态机测试中；若平台实现异常，可回退到 previous `Process.Kill()` fallback，但必须保留风险记录。
2. Web backpressure 改动应局限在 registry/runtime/client queue；若策略不稳定，可回退为现有 queue full detach 行为，同时保留可观察 reason 和测试。
3. History writer 优化应保持 public method 不变；如出现 replay 兼容问题，可回退内部 flush 策略。
4. 文档 verification 可记录未完成平台核验，不阻塞代码回滚。

## User review notes

本 Plan 基于用户已确认的 M5 决策创建：严格模式 / strict、Windows/macOS cleanup 必做、Web backpressure 实现、Claude Code / Codex 当前环境可验证。

等待用户 review Plan。接受后进入 Implementation / 实现阶段。
