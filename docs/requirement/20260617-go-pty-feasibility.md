# Go PTY 技术可行性文档需求
最后修改时间: 2026-06-17 14:20:57

Review status: Accepted

## Background

TermBridge-go 的目标是面向 Windows 优先实现本地 Agent Runtime，核心链路为：

```text
Browser / Client
  ↓
TermBridge Agent
  ↓
PTY
  ↓
Process
```

当前 README 已明确新架构不再依赖 `ttyd` 和 `tmux`，而是由 TermBridge 自主管理：

```text
Workspace
  ↓
PTY
  ↓
Process
```

因此，在进入正式业务开发前，需要形成一份 Go PTY 技术可行性文档，整合前期 Spike 计划、实测过程、问题修正和结论。

这份文档用于作为后续 Agent Runtime 技术路线的依据，而不是单纯保存测试日志。形成整合文档后，临时 Spike 计划和结果草稿可以清理，避免文档来源分散。

## Goal

形成一份面向项目决策的 Go PTY 技术可行性文档，至少回答：

1. Windows-first 场景下，Go + `aymanbagabas/go-pty` 是否可作为 TermBridge Agent 的 PTY 基座。
2. 已验证了哪些能力，包括 start/read/write/cwd/env/resize/cleanup/并发/multi-attach/owner crash/descendant cleanup。
3. 哪些能力仍然存在风险或需要后续补测。
4. 对业务开发阶段的约束是什么，例如 PTY abstraction、单 reader fan-out、多 attach、Agent 崩溃恢复策略、interrupt 策略。
5. 是否建议继续以 Go + go-pty 进入业务开发。

文档应从工程决策角度整合材料，而不是简单拼接现有 spike plan 和 spike result。

## Non-goal

本需求不包括：

1. 不继续扩展 PTY Spike 测试代码。
2. 不继续运行新的 PTY 验证命令。
3. 不实现 Agent Runtime、Workspace Manager、WebSocket terminal 或前端 UI。
4. 不创建正式 ADR，除非后续用户明确要求。
5. 不修改 `go-pty` clone 源码。
6. 不做 git commit、push 或分支操作。

## User scenarios

### 场景一：技术路线决策

作为项目负责人，我希望看到一份清晰的 Go PTY 可行性结论，从而决定是否继续使用 Go 进入 Agent Runtime 业务开发。

### 场景二：后续开发约束输入

作为后续 Agent Runtime 的实现者，我希望文档明确指出 go-pty 的使用边界，例如：

- 业务层必须通过 PTY interface 使用底层库。
- 多 attach 应由 Agent 层单 reader fan-out，不应多个 goroutine 直接读同一个 PTY。
- Agent 崩溃后不应承诺 Workspace 继续运行。
- `cmd.exe` 的 Ctrl+C 语义不能代表所有 runtime。

### 场景三：历史验证可追溯

作为后续维护者，我希望文档保留关键测试命令、测试结果和已知问题，能追溯为什么选择继续 Go + go-pty。

## Acceptance

文档完成后应满足：

- [ ] 明确说明本文整合了前期 Spike 计划、实测结果、问题修正和最终结论。
- [ ] 明确说明测试环境，包括 Windows、Go、Claude Code、Codex CLI。
- [ ] 明确说明 `go-pty` 的 Windows ConPTY 实现依据和本地 clone / replace 使用方式。
- [ ] 整理基础能力验证结果：basic、cwd、env、resize、unicode、long-output、cleanup、agent version smoke。
- [ ] 整理重点验证结果：Ctrl+C / interrupt、多 Workspace 并发、多 attach、Agent owner crash、descendant cleanup。
- [ ] 明确解释 `cmd.exe` Ctrl+C 返回 `0xc000013a` 的风险含义。
- [ ] 明确说明 Close 后 `0xc000013a` 在 cleanup 语义下可接受。
- [ ] 明确区分“Agent 崩溃后清理进程”和“Agent 崩溃后 Workspace 可恢复”不是同一件事。
- [ ] 给出阶段性结论：当前支持继续 Go + go-pty 进入业务开发。
- [ ] 给出后续业务开发约束和补测清单。
- [ ] 文档语言以中文为主，路径、命令、库名和协议术语保留英文。

## Open questions

当前无必须阻塞文档形成的问题。

后续可能需要单独决策的问题：

1. 是否将本次技术可行性结论进一步固化为 `docs/adr/0001-agent-runtime-language.md`。
2. 是否保留 `cmd/pty-spike` 作为项目内长期工具，还是后续迁移到 `tools/` 或 `internal/spike/`。
3. MVP 阶段是否正式声明：Agent 崩溃后 Workspace 不保证继续运行，只做 failed/stopped 状态恢复。

这些问题不阻塞当前技术可行性文档形成。

## Decisions

已确认的阶段性决策：

1. Go PTY 方案首选验证对象为 `github.com/aymanbagabas/go-pty`。
2. 不使用 `creack/pty` 作为 Windows-first PTY 基座。
3. 当前测试结论支持继续以 Go + go-pty 作为 Agent Runtime 的技术方向。
4. 正式业务代码不得直接依赖 spike 代码结构，应设计独立 PTY abstraction。
5. 多 attach 采用 Agent 层 fan-out 模型。
6. interrupt / close / kill process tree 在正式协议中应区分，不应混为单一操作。

## Risk

### 风险一：真实 Claude Code / Codex 交互尚未完整验证

当前只验证了：

```text
claude --version
codex --version
```

尚未验证真实 TUI/交互会话、长时间运行、动态输出、登录态、工具调用等行为。

### 风险二：cmd.exe Ctrl+C 语义不稳定

`cmd.exe` interrupt 场景返回 `0xc000013a`，说明不能把写入字节 `0x03` 作为统一 interrupt 语义。

### 风险三：Agent 崩溃后不能承诺 Workspace 继续运行

owner crash 测试未观察到残留进程，这支持清理语义，但不支持“崩溃后继续运行并可恢复 attach”的产品承诺。

### 风险四：高并发和慢客户端尚未验证

当前多 Workspace 并发只验证了 6 个短生命周期 PTY session；WebSocket backpressure、慢客户端、长期运行和大量 resize 尚未验证。

### 风险五：Spike 代码不是正式架构

当前 `cmd/pty-spike` 是验证代码，不能直接作为 Agent Runtime 架构落地。正式实现应重新设计分层、错误处理、生命周期和测试结构。

## User review notes

待用户 review。
