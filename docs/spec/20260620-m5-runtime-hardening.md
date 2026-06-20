# M5 Runtime Hardening 规格
最后修改时间: 2026-06-20 17:42:38

Review status: Accepted

## Requirement basis

依据：`docs/requirement/20260620-m5-runtime-hardening.md`

当前 Requirement 已接受，M5 采用严格模式 / strict。M5 聚焦 runtime hardening，不进入 Gateway/M6，不改变正式 CLI 入口 `termbridge exec -- <command...>`，不实现显式 `recent` UX。

用户补充决策：

1. Windows process tree cleanup 作为 M5 必做；当前开发环境是 macOS，因此先按计划实现 Windows 和 macOS 能力，后续在 Windows 环境核验。
2. Web terminal backpressure 作为 M5 实现项，不仅做压测和风险记录。
3. Claude Code / Codex 当前开发机环境已有，可纳入真实交互验证。

## Overview

M5 将现有 CLI runner、PTY adapter、Session / Workspace runtime、local Web/workbench 从“功能可用”推进到“可靠可验证”。规格分为四条主线：

1. **Process lifecycle hardening**：明确 interrupt、close、kill tree 的状态机和平台能力；对 macOS / Unix 与 Windows 分别实现 process tree cleanup；失败时可观察。
2. **Runtime consistency hardening**：确保 CLI 和 Web session 在 start、stop、error、exit、history、process record、state record 上一致。
3. **Web terminal output hardening**：实现 backpressure 策略，避免 slow client 拖死 PTY reader 或无说明丢失；降低大量输出下的 history I/O 和 allocation 压力。
4. **Verification hardening**：增加自动化测试、压力/行为验证命令和人工验证清单，覆盖 Claude Code / Codex 真实 TUI、Ctrl+C、长时间运行、多 session、resize 和大输出。

## Design decisions

### 1. Process tree cleanup is part of M5

M5 不只记录 process tree cleanup 风险，而是实现平台能力：

- macOS / Unix：通过进程组或平台等价机制终止子进程树，避免只 kill shell 父进程。
- Windows：通过 Job Object、进程树枚举或 go-pty 可行 adapter fallback 实现可验证 cleanup；若当前 macOS 环境无法本地运行 Windows 验证，仍需保留 Windows-specific 实现和测试边界，并在 verification 中记录 Windows 环境待核验项。
- `PTYSession.KillTree()` 保持作为 runtime 边界；平台细节留在 `internal/pty/gopty` 或其平台文件中，不泄漏到 CLI/Web 业务层。

### 2. Interrupt escalation remains explicit

现有 `CommandRunner.Run` 已有状态机：第一次 Ctrl+C soft interrupt，超时 close PTY，再超时 kill process。M5 继续沿用这个语义，但需要：

- 在 tests 和 manual checklist 中明确每一步的 observable result。
- 对 `OnStopping`、exit record、state record 和日志补齐验证。
- 确保 CLI signal、stdin Ctrl+C、Web close/session close 的语义不混淆。

### 3. Web backpressure is implemented in M5

M5 选择实现 backpressure，而不是只压测记录。目标不是让 slow client 无损接收无限输出，而是让策略明确且不拖垮全局 runtime：

- PTY reader 不因单个慢客户端永久阻塞。
- 每个 client 有明确 queue 限制和 queued bytes 限制。
- 当 client 追不上时，优先保证 runtime 和 history 可继续工作；client 的 detach、drop、coalesce 或 truncate 必须有明确 reason/status。
- 需要避免无说明的 client 丢失；如果 detach，必须通过 state/reason/log 可观察。

### 4. History write strategy remains bounded and batched

当前 `history.Writer` 已具备 max lines、max bytes、max line bytes、pending bytes 和 flush interval。M5 继续强化该方向：

- 保持 bounded history，不引入无限内存增长。
- 大输出下避免每个 chunk 全量刷盘。
- attach/history API 调用需要能 flush 当前 buffer，但不能破坏 runtime throughput。
- 需要测试 truncation、flush、close 和 replay 行为。

### 5. Allocation reduction is local to webterminal output path

M5 处理 `copyBytes` 高频分配风险，但不引入复杂 buffer ownership 模型。优先采用局部、可测试的优化：

- 减少每个 client 重复 copy。
- 明确 outbound binary payload 的不可变约定。
- 保持 race-safe，不让 PTY read buffer 被异步发送复用污染。

### 6. Real TUI verification is required but not fully automated

Claude Code / Codex 真实交互依赖本机安装、账号和交互状态。当前环境已有，因此 M5 verification 要记录真实验证：

- 启动。
- 输入。
- 输出。
- 正常退出。
- Ctrl+C interrupt。
- close / forced stop 行为。

可自动化部分写测试；不可稳定自动化的真实 TUI 操作写 manual verification checklist。

## Affected components

### CLI runner

- `internal/runner/runner.go`
- `internal/runner/interrupt.go`
- `internal/runner/runner_test.go`
- `internal/runner/relay.go`

关注点：interrupt state machine、close/kill escalation、output relay finish、exit interpretation、hooks observable behavior。

### PTY abstraction and go-pty adapter

- `internal/pty/pty.go`
- `internal/pty/gopty/manager.go`
- `internal/pty/gopty/manager_windows.go`
- potential platform-specific files such as `manager_unix.go` or equivalent tests

关注点：`KillTree()` 的平台实现、process group / Job Object / fallback、ProcessReporter、Wait/Close/Kill idempotency。

### Session / Workspace / state persistence

- `internal/session/*`
- `internal/state/store.go`
- `internal/process/*`
- `internal/app/app.go`

关注点：state、process、exit、history、log path 与真实 runtime 状态一致；异常退出和强制停止正确反映。

### Web terminal runtime

- `internal/webterminal/runtime.go`
- `internal/webterminal/registry.go`
- `internal/webterminal/registry_test.go`
- `internal/webserver/*`
- frontend workbench code if protocol/status display needs adjustment

关注点：client queue、queued bytes、detach reason、backpressure strategy、readLoop throughput、resize、attach/detach/close、history replay。

### History writer

- `internal/history/writer.go`
- `internal/history/writer_test.go`

关注点：bounded buffer、batched flush、large output、truncate flag、close flush、attach flush。

### Documentation and verification records

- `docs/plan/20260620-m5-runtime-hardening.md`
- `docs/verification/20260620-m5-runtime-hardening.md`
- possibly checklist content in verification document rather than a separate standalone checklist

关注点：自动化测试命令、人工验证清单、平台限制、M6 entry gate。

## Interfaces

### PTY Session interface

`internal/pty/pty.go` 的现有 interface 是 M5 的主要边界：

```go
type Session interface {
    io.Reader
    io.Writer
    Resize(size process.TerminalSize) error
    Interrupt() error
    Close() error
    KillTree() error
    Wait() Result
}
```

M5 原则上不扩张业务层接口。若必须增强可观察性，优先通过 existing logger、hooks、process/exit records 或 adapter-internal helper 实现。

### Runner hooks

现有 hooks：

```go
type Hooks struct {
    OnStarted  func(process.Record)
    OnStopping func(process.StopMode, string)
}
```

M5 继续使用 `OnStopping` 记录 interrupt/close/kill reason，并补测试保证状态转换可观察。

### Web outbound queue

现有 `Client` 通过 channel 发送 `Outbound`：

```go
type Outbound struct {
    Kind   OutboundKind
    Text   terminalproto.ServerMessage
    Binary []byte
}
```

M5 可调整 enqueue/backpressure 行为，但应保持 WebSocket protocol 的外部语义稳定：binary 仍代表 terminal output，text 仍代表 state/control/exited/replay messages。

### History writer

现有 `history.Writer` 接口：

```go
func (w *Writer) Write(p []byte) (int, error)
func (w *Writer) Flush() error
func (w *Writer) Close() error
func (w *Writer) Truncated() bool
```

M5 应保持该接口稳定，内部优化 flush/write 策略。

## Technical questions

1. Windows `KillTree()` 最终采用 Job Object、进程枚举，还是 go-pty / os.Process fallback 增强？Requirement 已确认 Windows 能力必做，但具体实现需在 Plan 中根据当前 go-pty 能力拆解。
2. Web slow client 策略采用 detach、drop oldest、coalesce binary chunks，还是组合策略？本 Spec 确认必须实现 backpressure，Plan 需选定最小可验证方案。
3. Web backpressure 的用户可见信号放在哪里：WebSocket state message、session attachment state、日志，还是三者组合？
4. copyBytes 优化是否只减少 per-client copy，还是引入 shared immutable payload？需要避免异步发送引用 read buffer。
5. 20+ session 压测是否全部用 shell 命令，还是混合 Claude/Codex？建议自动化使用 shell/短命令/大输出，Claude/Codex 留作人工验证。

## Risks

1. Windows process tree cleanup 当前无法在 macOS 环境完成真实运行核验；需要将实现和测试边界写清，并在 Windows 环境补跑 verification。
2. 过度复杂的 backpressure 设计可能影响 local Web/workbench 简洁性；M5 应优先选择可解释、可测试的最小策略。
3. 大输出测试可能暴露 history、WebSocket、frontend rendering 多处瓶颈，应避免把 Gateway 级优化提前引入本地 runtime。
4. Claude Code / Codex 真实 TUI 验证依赖账号、安装状态和交互环境；自动化测试不应假设外部服务稳定。
5. Process cleanup 与 PTY close 的平台行为存在差异，可能需要在 verification 中区分 macOS 已验证、Windows 待核验或 Windows 已核验。

## Alternatives

### Alternative A: 只记录 Windows cleanup 风险，不实现

不采用。用户已确认 Windows process tree cleanup 是 M5 必做。当前 macOS 环境限制只影响核验环境，不影响 M5 设计目标。

### Alternative B: Web backpressure 先压测再决定

不采用。用户已确认 Web terminal backpressure 在 M5 实现。

### Alternative C: 引入全局 ring buffer 替代现有 history writer

暂不采用。M5 优先强化现有 bounded history writer 和 client queue/backpressure；除非 Plan 阶段发现现有结构无法满足 acceptance。

### Alternative D: 将 Claude/Codex 交互全部自动化

不采用。真实 TUI 自动化受本机环境、账号和交互状态影响，M5 采用“自动化基础 runtime + 人工真实 TUI checklist”的组合。

## User review notes

用户已确认：

1. 使用严格模式 / strict 推进 M5。
2. Windows/macOS process tree cleanup 纳入 M5 必做；Windows 后续在 Windows 环境核验。
3. Web terminal backpressure 在 M5 实现。
4. 当前环境已有 Claude Code / Codex，可做真实验证。

本 Spec 仍需用户 review。若接受本 Spec，下一阶段进入 Plan / 计划，拆解具体实现步骤、文件改动和验证命令。
