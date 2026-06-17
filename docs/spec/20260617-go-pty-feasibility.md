# Go PTY 技术可行性结论
最后修改时间: 2026-06-17 14:23:48

Review status: Accepted

日期: 2026-06-17

## 结论摘要

当前验证结论：**TermBridge-go 可以继续采用 Go + `github.com/aymanbagabas/go-pty` 作为 Windows Agent Runtime 的 PTY 技术方向**。

这不是最终生产级承诺，而是进入业务开发阶段的技术准入结论。已验证 Windows ConPTY 的核心链路可用：

```text
TermBridge Agent
  ↓
go-pty
  ↓
Windows ConPTY
  ↓
cmd / PowerShell / pwsh / Claude Code / Codex
```

后续业务开发应遵守以下约束：

1. 正式代码必须通过 TermBridge 自己的 `PTY` abstraction 使用底层库，不要让业务层直接依赖 spike 代码或 `go-pty` 细节。
2. 多 attach 应采用 **单 PTY reader + Agent 层 fan-out**，不要多个 goroutine 同时读取同一个 PTY。
3. `interrupt`、`close workspace`、`kill process tree` 必须是不同协议语义，不应简单混为写入 Ctrl+C。
4. MVP 阶段不应承诺 Agent 崩溃后 Workspace 继续运行；应按 `failed/stopped` 状态恢复处理。
5. `cmd/pty-spike` 与 `cmd/pty-testprogram` 当前是 spike 验证工具，后续可归入测试用例或测试工具目录，但不作为正式 runtime 架构实现。

## 整合来源

本文整合以下材料：

- 前期 Windows PTY Spike 文件级实施计划。
- 前期 Windows PTY Spike 实测结果。
- 本轮实际执行的测试、问题修正和结论。
- 本地 clone 的 `go-pty` 实现检查。

## 背景

TermBridge-go 的目标不是复刻原 TermBridge 的 `ttyd + tmux` 架构，而是让 TermBridge 自主管理运行时：

```text
Workspace
  ↓
PTY
  ↓
Process
```

Windows-first 是当前关键约束。Windows 上现代 PTY 基础能力依赖 ConPTY / Pseudoconsole，因此必须先证明 Go 生态中可用的 PTY 类库能满足 TermBridge Agent 的最低运行要求。

本次候选库为：

```text
github.com/aymanbagabas/go-pty
```

本地 clone 路径：

```text
D:\SourceCodes\mywork\TermBridge-go\go-pty
```

项目根目录 `go.mod` 使用本地 replace 验证：

```text
replace github.com/aymanbagabas/go-pty => ./go-pty
```

## 为什么选择 go-pty 作为首轮验证对象

`go-pty` README 明确声明支持：

```text
Unix PTYs and Windows through ConPty
```

并说明 Windows 不能仅依赖 `os/exec`，因为启动 ConPTY 子进程需要特殊 process attribute。

源码检查确认其 Windows 实现包含关键 ConPTY API 调用：

- `CreatePseudoConsole`
- `ResizePseudoConsole`
- `ClosePseudoConsole`
- `PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE`
- `CreateProcess` + `EXTENDED_STARTUPINFO_PRESENT`

这说明它不是简单 shell wrapper，而是直接接入 Windows ConPTY。

不选择 `creack/pty` 作为 Windows-first 基座的原因：其主线定位仍偏 Unix PTY，Windows ConPTY 支持不是当前可依赖的稳定主路径。

## 测试环境

```text
OS: Windows 11 Pro 10.0.26100
Go: go1.25.8 windows/amd64
Claude Code: 2.1.172
Codex CLI: 0.139.0
```

可用命令：

```text
cmd.exe
powershell.exe
pwsh.exe
claude
codex
```

## 验证程序

本次新增两个 spike 程序：

```text
cmd/pty-spike/main.go
cmd/pty-testprogram/main.go
```

用途：

- `cmd/pty-spike`：运行 Windows PTY 场景验证。
- `cmd/pty-testprogram`：制造 unicode、ANSI、long-output 等可控输出。

构建命令：

```powershell
go build -o .tmp\pty-testprogram.exe .\cmd\pty-testprogram
go build -o .tmp\pty-spike.exe .\cmd\pty-spike
```

后续处理建议：

```text
当前 cmd/pty-* 是 spike 工具；后续进入业务开发后，应将其中有价值的验证逻辑归入测试用例或测试工具，而不是直接作为正式 Agent Runtime 实现。
```

## 基础能力验证

执行命令：

```powershell
.\.tmp\pty-spike.exe --scenario all --shell all --timeout 15s
```

结果：

```text
[PASS] basic cmd (65ms)
[PASS] basic powershell (275ms)
[PASS] basic pwsh (406ms)
[PASS] cwd cmd (59ms)
[PASS] cwd powershell (269ms)
[PASS] cwd pwsh (388ms)
[PASS] env cmd (59ms)
[PASS] env powershell (274ms)
[PASS] env pwsh (407ms)
[PASS] resize cmd (1.676s)
[PASS] resize powershell (429ms)
[PASS] resize pwsh (404ms)
[PASS] unicode (61ms)
[PASS] long-output (421ms)
[PASS] cleanup cmd (64ms)
[PASS] cleanup powershell (273ms)
[PASS] cleanup pwsh (400ms)
[PASS] agent claude version (134ms)
[PASS] agent codex version (118ms)

summary: total=19 failed=0 duration=6.183s
```

### 已验证能力

| 能力 | 结果 | 说明 |
| --- | --- | --- |
| 启动 shell | 通过 | `cmd` / `powershell` / `pwsh` 均可启动。 |
| 写入 stdin | 通过 | 可向 PTY 写入命令。 |
| 读取 stdout | 通过 | 可持续读取 PTY output。 |
| 正常 exit | 通过 | shell 正常退出可被 `Wait` 观测。 |
| cwd | 通过 | 指定工作目录后 shell 内可观测。 |
| env | 通过 | 自定义环境变量可传入。 |
| resize | 通过 | `cmd` / `powershell` / `pwsh` 均能观测到 120x40。 |
| unicode | 通过 | 中文、ASCII、ANSI 输出通过。 |
| long-output | 通过 | 20000 行输出无死锁。 |
| cleanup | 通过 | `ClosePseudoConsole` 后 shell 退出可观测。 |
| Claude Code smoke | 通过 | `claude --version` 可通过 PTY 输出。 |
| Codex smoke | 通过 | `codex --version` 可通过 PTY 输出。 |

## 重点风险验证

用户指定重点验证项：

```text
3. Ctrl+C / interrupt
5. 多 Workspace 并发
6. 多 attach 同一 PTY
8. Agent 崩溃后的进程/状态恢复策略
9. Close 后是否存在子孙进程残留
```

执行命令：

```powershell
.\.tmp\pty-spike.exe --scenario focused --shell all --timeout 15s
```

结果：

```text
[FAIL] interrupt cmd (613ms)
       exit status 0xc000013a
[PASS] interrupt powershell (831ms)
[PASS] interrupt pwsh (957ms)
[PASS] concurrent sessions (466ms)
[PASS] multi-attach cmd (30ms)
[PASS] multi-attach powershell (283ms)
[PASS] multi-attach pwsh (424ms)
[PASS] owner crash cleanup (1.744s)
[PASS] descendant cleanup cmd (1.628s)
[PASS] descendant cleanup powershell (1.778s)
[PASS] descendant cleanup pwsh (1.908s)

summary: total=11 failed=1 duration=10.662s
```

### Ctrl+C / interrupt

PowerShell 与 pwsh 通过：

```text
写入 Ctrl+C 字节 0x03 后，长时间阻塞命令被中断，shell 仍可继续接收命令并正常 exit。
```

cmd.exe 未通过：

```text
cmd.exe interrupt 场景返回 exit status 0xc000013a。
```

判断：

```text
cmd.exe 的 Ctrl+C 语义不能代表所有 runtime。正式协议不能把 “interrupt” 简化为统一写入 0x03。
```

业务影响：

- Claude Code / Codex / PowerShell 是更重要目标，当前风险可接受。
- 如果后续正式支持 `cmd.exe` runtime，需要为 cmd 单独定义 interrupt 策略。
- 协议层应拆分：
  - `interrupt foreground process`
  - `close workspace`
  - `kill process tree`

### 多 Workspace 并发

结果：通过。

验证方式：同时启动 6 个 PTY session：

```text
cmd, powershell, pwsh, cmd, powershell, pwsh
```

每个 session 独立写入 marker、读取 output、正常退出。

结论：

```text
go-pty 可以同时创建多个 Windows ConPTY session，基础并发可用。
```

限制：

```text
当前只是 6 个短生命周期 session，不等价于长期高并发压力测试。
```

### 多 attach 同一 PTY

结果：通过。

验证方式：在同一个 PTY read loop 上模拟两个 subscriber：

```text
PTY output reader
  ↓
subscriber A
subscriber B
```

两个 subscriber 均收到同一 marker。

关键结论：

```text
多 attach 是 Agent 层 fan-out 能力，不应依赖 go-pty 原生多 reader。
```

正式实现约束：

```text
一个 PTY 只能有一个 reader goroutine；Agent 将 output 广播给多个 attach connection。
```

### Agent owner crash

结果：通过。

验证方式：

```text
1. 父进程启动内部 child runner。
2. child runner 通过 go-pty 启动 cmd。
3. cmd 中启动带唯一 marker 的长时间 PowerShell 子进程。
4. child runner 用 os.Exit(77) 模拟 Agent abrupt exit。
5. 父进程等待 child runner 退出。
6. 查询是否仍存在携带 marker 的子孙进程。
```

结果未观察到 marker 子孙进程残留。

工程判断：

```text
当前行为支持“Agent 崩溃后进程倾向于被清理”，但不支持“Agent 崩溃后 Workspace 可继续运行并恢复 attach”。
```

MVP 建议：

```text
Agent 崩溃后 Workspace 不保证继续运行；Agent 重启时将不确定 Workspace 标记为 failed/stopped，并提示用户重新启动。
```

如果未来要实现崩溃后继续运行，需要额外设计：

```text
supervisor / child runtime host / external process ownership model
```

这不属于当前 Go PTY 可行性验证范围。

### Close 后子孙进程残留

结果：通过。

验证方式：对 `cmd` / `powershell` / `pwsh` 分别执行：

```text
1. 启动 shell。
2. 在 shell 内启动带唯一 marker 的长时间 PowerShell 子进程。
3. 确认子进程可被系统查询到。
4. ClosePseudoConsole。
5. Wait shell exit。
6. 再次查询 marker 子进程。
```

结果：未观察到 marker 子孙进程残留。

结论：

```text
ClosePseudoConsole 在当前基础场景下可以清理 shell 及其子孙进程。
```

仍需补测：

```text
真实 Claude Code / Codex 运行期间的子进程树。
```

## 测试过程中发现的问题

### PowerShell / pwsh 命令提交需要 CRLF

初版 spike 对非 cmd 使用 `\n`，导致 PowerShell / pwsh 可输出 marker，但 `exit` 等待超时。

修正：

```text
所有 shell 写入命令统一使用 \r\n。
```

正式实现约束：

```text
Windows terminal input 应按 CRLF 语义处理命令提交，不能只按 Unix LF 假设。
```

### 指定 cwd 时应先解析命令绝对路径

初版 cwd 场景直接启动 `cmd.exe`，go-pty Windows 实现会按 cwd 拼接相对命令，导致：

```text
exec: "D:\SourceCodes\mywork\cmd.exe": file does not exist
```

修正：

```text
启动 shell 前使用 exec.LookPath 解析绝对路径。
```

正式实现约束：

```text
Process launcher 在设置 cwd 前，应先 resolve executable path，避免 Windows CreateProcess 路径解析差异。
```

### Close 后退出码 0xc000013a

主动 `ClosePseudoConsole` 后，shell 可能返回：

```text
0xc000013a
```

判断：

```text
在 cleanup / close workspace 语义下，该退出状态可接受，表示控制台进程被关闭或中断。
```

但在 interrupt 场景中，`cmd.exe` 返回同样状态意味着 shell 会话被终止，不能视为成功的 foreground interrupt。

## 对业务开发的架构约束

### PTY abstraction

正式代码应定义 TermBridge 自己的接口，例如：

```text
PTYSession
PTYManager
ProcessSpec
```

业务层依赖 TermBridge 接口，而不是直接依赖 `go-pty.Pty`。

原因：

- 保留替换底层 PTY 库的能力。
- 隔离 Windows 特殊语义。
- 便于测试 fake PTY / deterministic output。

### 单 reader fan-out

正式实现应采用：

```text
PTY read goroutine
  ↓
History buffer
  ↓
Attach connection fan-out
```

禁止：

```text
多个 attach connection 直接并发 Read 同一个 PTY。
```

### 生命周期语义拆分

正式协议中应区分：

```text
interrupt foreground process
close workspace
kill process tree
restart workspace
```

不要将这些行为都映射为 Ctrl+C 或 ClosePseudoConsole。

### Agent 崩溃恢复策略

MVP 阶段建议：

```text
Agent 崩溃后 Workspace 不保证继续运行。
Agent 重启后，无法确认的 Workspace 标记为 failed/stopped。
用户显式 restart 后重新创建 PTY / Process。
```

如果未来希望 Agent 崩溃后进程继续运行，需要额外 runtime host 或 supervisor 架构。

### Spike 代码归宿

当前：

```text
cmd/pty-spike
cmd/pty-testprogram
```

后续建议：

```text
将有效场景沉淀为自动化测试或测试工具，不直接并入生产 runtime。
```

## 仍需后续补测

进入业务开发后，需要继续补齐：

1. Claude Code 真实交互，而不只是 `--version`。
2. Codex 真实交互，而不只是 `--version`。
3. WebSocket 慢客户端 backpressure。
4. 20+ Workspace 长时间并发。
5. 大量 output + 多 attach 组合场景。
6. 高频 resize。
7. Claude Code / Codex 运行期间的子孙进程清理。
8. Agent 重启后状态 reconciler。
9. Ctrl+C / interrupt 在 Claude Code / Codex 中的真实行为。

## 阶段性决策

当前阶段建议采用：

```text
Go + github.com/aymanbagabas/go-pty
```

作为 TermBridge Agent Runtime 的 Windows PTY 技术方向。

该决策的含义是：

```text
可以进入业务开发流程，开始设计 Agent Runtime / Workspace Manager / WebSocket terminal。
```

该决策不意味着：

```text
1. go-pty 已经被证明满足所有生产场景。
2. cmd.exe Ctrl+C 已经有统一解决方案。
3. Agent 崩溃后 Workspace 可恢复 attach。
4. spike 代码可以直接作为生产代码。
```

## 最终结论

基于当前测试，Go PTY 方案满足 TermBridge-go 进入下一阶段业务开发的最低条件。

建议下一步围绕以下路线推进：

```text
1. 以 docs/plan/20260617-roadmap-refresh.md 承接后续里程碑计划。
2. 将本地一等入口收敛为 termbridge CLI：termbridge [options] -- <command...>。
3. 设计正式 PTY abstraction / CommandRunner / ProcessSpec / InterruptStrategy。
4. 优先启动 Go CLI Skeleton 和 PTY Command Runner MVP。
5. 将 GUI 定位为 CLI 容器，不让 GUI 拥有 PTY / Process / Workspace runtime。
6. 在 Gateway 前提前做 Web Terminal Technical Spike，验证 xterm.js / WebSocket / backpressure 风险。
7. 将 cmd/pty-* 中有价值的场景逐步迁移为自动化测试或测试工具。
8. 在 Runtime Hardening 前补测 Claude Code / Codex 真实交互。
```
