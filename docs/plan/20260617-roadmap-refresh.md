# Roadmap 里程碑计划
最后修改时间: 2026-06-20 17:23:23

Review status: Accepted

## Plan basis

本计划基于：

- `docs/requirement/20260617-roadmap-refresh.md`
- `docs/spec/20260617-roadmap-refresh.md`
- `docs/spec/20260617-go-pty-feasibility.md`
- `README.md`

当前已确认技术方向：

```text
Windows PTY Runtime: Go
PTY 技术方向: github.com/aymanbagabas/go-pty
底层机制: Windows ConPTY / Pseudoconsole
```

当前已确认本地交付方向：

```text
CLI first
GUI optional container
Web technical spike before Gateway
Gateway Web as formal remote product surface
```

核心运行时原则：

```text
termbridge CLI invocation
  ↓
TermBridge runtime
  ↓
PTY abstraction
  ↓
go-pty
  ↓
Windows ConPTY
  ↓
user command
```

中长期资源模型仍以 README 的架构为目标：

```text
Workspace
  ↓
PTY
  ↓
Process
```

但本地阶段不要求用户先理解或操作常驻 Agent / Workspace create / attach 流程。本地一等入口收敛为：

```text
termbridge [options] exec -- <command...>
```

## Plan goal

本计划作为后续业务开发的阶段性里程碑计划，替代已经删除的独立 roadmap 文档。

目标是回答：

1. TermBridge-go 后续按什么顺序交付。
2. 如何从 Go + go-pty 可行性进入 CLI-first 本地产品。
3. 每个 milestone 的目标、交付物、验收标准和风险门是什么。
4. GUI、Web、Gateway 分别在什么时候进入，边界是什么。
5. 哪些事项必须先验证，哪些事项不能提前扩展。

## Planning principles

### 原则一：CLI 是本地唯一一等使用入口

本地阶段标准使用方式：

```text
termbridge [options] exec -- <command...>
```

示例：

```text
termbridge exec -- claude
termbridge exec -- codex
termbridge exec -- pwsh
termbridge --cwd D:\project exec -- claude
termbridge --cwd D:\project exec -- npm run dev
```

默认 cwd 为当前目录；也可以通过 `--cwd` 指定目录。

用户愿意运行什么命令，TermBridge 就在 PTY runtime 中直接运行该命令。

### 原则二：GUI 是 CLI 容器，不是另一套 Agent

GUI 可以做：

```text
选择 cwd
选择 command
spawn / supervise termbridge CLI
承载 terminal surface
展示 exit code / logs
保存 UI preference
```

GUI 不能做：

```text
直接创建 PTY
直接拥有 Process lifecycle
直接维护 Workspace state
直接实现 History buffer
直接实现 interrupt / kill strategy
```

如果 GUI 直接实现这些能力，就等于另做一套 Agent，应避免。

### 原则三：Web 技术验证前置，正式 Web 放到 Gateway

本地不把 Web 作为正式交付产品面，但必须在 Gateway 前验证：

```text
xterm.js rendering
browser input / shortcut / paste / IME
resize
large output
slow client backpressure
WebSocket protocol
reconnect / detach semantics
```

因此需要 M2.5 Web Terminal Technical Spike。

Gateway 阶段再把 Web 作为正式跨设备产品入口。

### 原则四：Go + go-pty 是当前方向，但必须被 abstraction 隔离

Go + go-pty 是当前技术准入结论，不是所有生产场景已验证完成。

正式代码必须定义 TermBridge 自己的边界：

```text
CommandRunner
ProcessSpec
PTYSession
PTYManager
InterruptStrategy
SessionRecord
HistoryBuffer
```

业务层不直接依赖：

```text
go-pty.Pty
cmd/pty-spike
cmd/pty-testprogram
```

### 原则五：Ctrl+C 是停止当前命令，不是 detach

CLI 用户按下 `Ctrl+C` 的意图是停止当前命令。

建议策略：

```text
first Ctrl+C:
  soft interrupt foreground process

grace period:
  wait for process exit

second Ctrl+C or timeout:
  close PTY / kill process tree
```

正式实现必须区分：

```text
interrupt foreground process
close session
kill process tree
```

### 原则六：先 CLI Runtime，后本地产品面，再 Gateway

交付顺序固定为：

```text
CLI Command Runner
  ↓
Session / Workspace Runtime Model
  ↓
CLI-first Local Product Surface
  ↓
Gateway Web Terminal
```

不要在 CLI runtime 稳定前提前建设复杂 Web / GUI / Gateway。

## Milestone overview

```text
M0: Runtime 技术决策（已阶段性完成）
M1: Go CLI Skeleton（已完成）
M2: PTY Command Runner MVP（核心已完成）
M2.5: Web Terminal Technical Spike（已实现，风险进入 M5）
M3: Session / Workspace Runtime Model（主体已完成）
M4: Local Product Surface: CLI first, GUI optional container（收口中）
M5: Runtime Hardening
M6: Gateway Web Terminal MVP
M7: Multi-device Beta
M8: Security / Packaging / Release
```

建议优先完成 M1-M3；M2.5 必须在 M6 前完成；只有 M5 通过后再进入 Gateway 正式产品化。

---

## M0：Runtime 技术决策（已阶段性完成）

### Objective

确认 Windows PTY 技术方向和本地交付主入口。

### Decision

当前阶段性决策：

```text
Runtime language: Go
PTY library: github.com/aymanbagabas/go-pty
Windows PTY backend: ConPTY / Pseudoconsole
Local first-class entry: termbridge CLI
```

### Evidence

Go PTY feasibility 已验证：

```text
start / read / write
cwd / env
resize
unicode / ANSI / long-output
cleanup
Claude Code --version smoke
Codex --version smoke
多 Workspace 短生命周期并发
单 PTY reader + 多 attach fan-out 模拟
owner crash cleanup 基础场景
descendant cleanup 基础场景
```

### Known limitations

```text
cmd.exe Ctrl+C interrupt 场景未通过，返回 0xc000013a。
Claude Code / Codex 真实 TUI 交互尚未验证。
多 Workspace 只验证了短生命周期基础并发。
慢客户端 backpressure 尚未验证。
Web terminal 尚未验证。
Agent 崩溃后不承诺 Workspace 可恢复 attach。
```

### Exit criteria

已满足：

```text
1. Go + go-pty 可作为进入业务开发的技术准入。
2. 本地交付主入口收敛为 termbridge CLI。
3. 不再保留 ttyd / tmux 作为正式 runtime 依赖。
4. 后续风险已经进入 M2 / M2.5 / M3 / M5 的 gate。
```

### Gate decision

```text
Continue with Go + go-pty and CLI-first local delivery.
```

---

## M1：Go CLI Skeleton

状态：已完成。

当前实现已经具备 CLI entrypoint、命令解析、`--help`、`--version`、`--cwd`、`exec --` 命令分隔、config loading、structured logging、基础错误模型和测试覆盖。

### Objective

建立 `termbridge` CLI 的最小骨架。

该阶段不实现完整 PTY command runner，只先确定 CLI 入口、参数解析、错误模型、日志和版本信息。

### Deliverables

```text
termbridge binary
main entrypoint
command-line parser
--help
--version
--cwd option skeleton
exec -- separator parsing skeleton
config loading skeleton
structured logging
basic error model
test command
```

### Suggested components

```text
cmd/termbridge
internal/cli
internal/config
internal/logging
internal/version
internal/errors
```

目录命名可以在实现前微调，但必须保持 CLI-first 边界清晰。

### CLI shape

```text
termbridge --help
termbridge --version
termbridge exec -- <command...>
termbridge --cwd <directory> exec -- <command...>
termbridge exec --help
termbridge web --dev
```

命令执行使用显式 `exec --` 分隔 TermBridge 选项与用户命令参数。

### Acceptance criteria

```text
1. termbridge 可构建并从命令行启动。
2. --help 明确展示 `termbridge [options] exec -- <command...>` 模型。
3. --version 返回版本和 runtime 信息。
4. --cwd 能被解析并做基础校验。
5. 未提供 command 时 fail fast，输出明确错误。
6. 参数错误不静默降级。
7. 本地测试命令明确。
```

### Risks

```text
过早设计复杂 workspace 子命令会破坏 CLI-first command runner 心智。
参数解析如果不严格，会导致命令透传和 TermBridge 自身选项混淆。
```

### Non-goals

```text
不实现 PTY run。
不实现 GUI。
不实现 Web。
不实现 Gateway。
不暴露常驻 Agent daemon。
```

### Gate

进入 M2 前必须确认：

```text
termbridge [options] exec -- <command...> 的 CLI 入口和错误边界成立。
```

---

## M2：PTY Command Runner MVP

状态：核心已完成；可靠性验证和 hardening 风险进入 M5。

当前实现已经具备 PTY abstraction、CommandRunner、ProcessSpec、stdin/stdout relay、resize relay、Ctrl+C 基础处理、exit code 传递、cwd/env 支持和基本 cleanup。进程树清理、Claude Code / Codex 真实交互、长时间运行和复杂 interrupt 行为进入 M5。

### Objective

实现最小可用的 PTY command runner：用户通过 `termbridge [options] exec -- <command...>` 在 PTY runtime 中运行任意命令。

目标链路：

```text
User terminal
  ↓
termbridge CLI
  ↓
CommandRunner
  ↓
TermBridge PTY abstraction
  ↓
go-pty
  ↓
Windows ConPTY
  ↓
user command
```

### Deliverables

```text
CommandRunner
ProcessSpec
PTY abstraction
PTY Manager
Process launcher
stdin relay
stdout/stderr relay
terminal resize relay
Ctrl+C handling
exit code propagation
cwd support
env support
basic cleanup
```

### Suggested components

```text
internal/runtime/command
internal/runtime/pty
internal/runtime/process
internal/runtime/interrupt
internal/terminal
```

### Required abstraction

至少定义以下概念，具体命名可在实现阶段调整：

```text
ProcessSpec
  command
  args
  cwd
  env
  initialSize

CommandRunner
  run(ProcessSpec) -> ExitResult

PTYSession
  write(input)
  resize(cols, rows)
  interrupt()
  close()
  wait()

InterruptStrategy
  softInterrupt()
  closePTY()
  killProcessTree()
```

### CLI behavior

基础行为：

```text
termbridge exec -- claude
termbridge --cwd D:\project exec -- codex
termbridge --cwd D:\project exec -- pwsh
termbridge --cwd D:\project exec -- npm run dev
```

规则：

```text
1. 未传 --cwd 时使用当前目录。
2. --cwd 必须存在且是目录。
3. `exec --` 后的内容原样作为用户命令和参数。
4. 命令退出后 termbridge 返回对应 exit code。
5. TermBridge 自身错误使用明确的非零 exit code。
```

### Ctrl+C behavior

MVP 策略：

```text
1. 捕获 Ctrl+C。
2. 对 foreground process 尝试 soft interrupt。
3. 等待短暂 grace period。
4. 如果用户再次 Ctrl+C 或 grace period 后仍未退出，则 close PTY / kill process tree。
5. 输出明确状态，不无限等待。
```

注意：

```text
Ctrl+C 不是 detach。
```

### Windows-specific requirements

必须吸收 feasibility 结论：

```text
1. 启动进程前先 resolve executable path。
2. 设置 cwd 时避免 Windows CreateProcess 路径解析差异。
3. Windows terminal command submit 按 CRLF 语义处理。
4. ClosePseudoConsole 后 0xc000013a 在 close/cleanup 语义下可以接受。
5. cmd.exe Ctrl+C 失败不能被误判为通用 interrupt 成功。
6. interrupt / close / kill process tree 不能混成一个操作。
```

### Acceptance criteria

```text
1. termbridge exec -- <command...> 可以运行用户命令。
2. --cwd 指定目录生效。
3. stdin/stdout relay 可用，交互式命令可操作。
4. terminal resize 能传递到底层 PTY。
5. Ctrl+C 能停止常见前台命令，失败时有升级策略。
6. 子进程退出码能传递为 termbridge exit code。
7. close / interrupt 后不留下基础 shell 子进程。
8. 长输出不会导致 read loop 死锁。
```

### Tests / checks to prepare

```text
unit: CLI argument parsing
unit: ProcessSpec validation
unit: executable path resolve
integration: termbridge exec -- pwsh -NoLogo
integration: cwd / env
integration: long output
manual: Claude Code real interaction
manual: Codex real interaction
manual: Ctrl+C interrupt behavior
```

### Risks

```text
Windows Ctrl+C 行为在 cmd / PowerShell / pwsh / Claude Code / Codex 中不一致。
命令透传如果解析不严谨，会误吞用户参数。
进程树 cleanup 如果只依赖 PTY close，复杂 Agent CLI 可能残留子进程。
```

### Gate

进入 M2.5 / M3 前必须确认：

```text
termbridge CLI command runner 能稳定运行真实交互命令，并能可靠退出。
```

---

## M2.5：Web Terminal Technical Spike

状态：已实现；large output、slow client backpressure、history flush 和高频内存分配风险进入 M5。

当前实现已经具备本地 Web terminal/workbench、xterm.js 前端、WebSocket relay、stdin/stdout bridge、resize bridge、session create/attach/detach/close 和 history replay。该本地 Web/workbench 是 CLI-first 本地产品面的辅助入口，不拥有 runtime。

### Objective

在 Gateway 前提前验证 Web terminal 技术风险，并为本地 workbench 提供辅助入口；正式远程 Web 产品入口仍放到 Gateway 阶段。

### Target shape

```text
dev-only web page
  ↓
xterm.js
  ↓
local WebSocket / dev attach endpoint
  ↓
TermBridge command/session runtime
  ↓
PTY
  ↓
user command
```

### Deliverables

```text
dev-only web terminal harness
minimal xterm.js page
local websocket relay prototype
stdin/stdout bridge
resize bridge
large output check
slow client / backpressure check
browser input notes
```

### Non-goals

```text
不做 auth / Gate / device routing。
不替代 CLI。
不让 Web/workbench 拥有 PTY / Process / Workspace runtime。
```

### Acceptance criteria

```text
1. xterm.js 能显示基础 shell / Claude Code / Codex 输出。
2. 浏览器输入、复制粘贴、常见快捷键行为有记录。
3. resize 能从 browser 传到底层 PTY。
4. 大量输出不会明显卡死 Agent runtime。
5. 慢客户端不会拖死全局 PTY reader。
6. WebSocket protocol 对未来 Gate relay 没有明显阻塞。
```

### Risks

```text
如果该 spike 太晚，Gateway 阶段会同时暴露 Web terminal 和 tunnel 问题。
如果该 spike 被产品化，会偏离 CLI-first 本地交付。
```

### Gate

M6 前必须完成该 spike 或同等验证。

---

## M3：Session / Workspace Runtime Model

状态：主体已完成。

当前实现已经具备 session/workspace metadata、runtime state、bounded history、local session record、process/exit/log record、workspace/session listing。M4 的基础 recent 能力由 session metadata、session listing 和 history 承接，不新增显式 `recent` 命令或 UX。

### Objective

在 CLI command runner 成立后，引入 Session / Workspace runtime model，用于 history、metadata、restart、GUI container 和未来 Gateway 映射。

本阶段不是要求用户必须使用 workspace 子命令；本地一等入口仍是 CLI command runner。

### Conceptual model

```text
termbridge CLI invocation
  ↓
Session / Workspace record
  ↓
PTY
  ↓
Process
```

### Deliverables

```text
Session / Workspace metadata model
runtime state model
in-memory history buffer
optional local session record
restart metadata skeleton
logs location
basic list support; session metadata/listing/history provide recent command visibility
```

### Suggested components

```text
internal/session
internal/workspace
internal/runtime/history
internal/storage 或 internal/state
```

### State model

建议内部状态：

```text
starting
running
stopping
stopped
failed
```

如果未来支持后台 detach / reattach，再扩展：

```text
detached
reattaching
```

M3 不应过早要求用户理解复杂 workspace lifecycle。

### History design

MVP 可以先使用 bounded in-memory buffer：

```text
按字节或行数限制
支持 GUI/Web spike 回放近期输出
支持输出过多时截断
截断行为可观测
```

如果 CLI invocation 结束即退出，可以暂不持久化完整 terminal history。

### Acceptance criteria

```text
1. 每次 termbridge run 有清晰 session metadata。
2. run 状态能表达 running / stopped / failed。
3. history buffer 有明确上限。
4. exit reason / exit code 可记录。
5. logs 可定位。
6. 该模型可被 GUI container 和 Gateway 后续复用。
```

### Risks

```text
过早引入后台 workspace 会复杂化本地 CLI 心智。
Session metadata 如果没有边界，会演变成未完成的 daemon。
```

### Gate

进入 M4 前必须确认：

```text
Session / Workspace model 支撑产品化，但没有破坏 CLI-first 使用方式。
```

---

## M4：Local Product Surface: CLI first, GUI optional container

状态：收口中。

当前 CLI 产品面已经具备 clear help、clear errors、config file support、log path exposure、workspace/session listing 和 local Web/workbench 辅助入口。M4 收口重点是文档同步、可自动化检查的单元测试补齐，以及进入 M5 前的人工验证项边界。

### Objective

把本地 CLI 产品面打磨到可日常使用；如需要 GUI，GUI 只作为 CLI 容器。

### Required deliverables: CLI product

```text
clear help
clear error messages
config file support
session metadata/listing/history provide recent command visibility
log path exposure
shell completion（可选）
common runtime shortcuts（可选）
automated tests for stable checks
manual verification items for interactive/runtime scenarios
```

### Optional deliverables: GUI container

```text
select cwd
select command
spawn termbridge CLI
host terminal surface
show exit code
show logs
store UI preferences
```

### GUI boundary

GUI 必须依赖：

```text
termbridge CLI
或 shared CLI runtime package
或 local IPC backed by the same CLI/runtime layer
```

GUI 不允许独立实现：

```text
PTY lifecycle
Process lifecycle
History lifecycle
Interrupt / kill strategy
Workspace ownership
```

### Acceptance criteria

```text
1. 用户可以只通过 CLI 完成本地主要使用。
2. termbridge [options] exec -- <command...> 的帮助和错误信息清晰。
3. Ctrl+C / exit code / cleanup 行为可预测。
4. GUI 如果存在，只调用或容器化 CLI/runtime。
5. 本地产品不依赖 local Web UI 才能完成核心 CLI 使用；local Web/workbench 作为辅助产品面存在，不拥有 runtime。
```

### Risks

```text
GUI 如果为了体验绕过 CLI runtime，会产生第二套 Agent。
CLI polish 不足会导致用户依赖未成型的 GUI/Web。
```

### M4 closeout checklist

可自动化检查应由单元测试覆盖：

```text
1. CLI help 展示 exec / workspace / session / web 和当前命令示例。
2. CLI 参数解析支持 `termbridge exec -- <command...>`。
3. CLI 参数解析支持 `termbridge --cwd <dir> exec -- <command...>`。
4. CLI 参数错误、缺少 command、缺少 `--`、缺少 `--` 后 command 时返回 usage error。
5. workspace/session list 能输出 workspace、session、command、cwd、log path 等基础 product surface 信息。
```

保留为人工验证项：

```text
1. Claude Code / Codex 真实 TUI 交互。
2. Ctrl+C 在 Claude Code / Codex / shell 中的真实 interrupt 行为。
3. 大量输出、慢客户端 backpressure、高频 resize。
4. 进程树清理和长时间运行稳定性。
5. local Web/workbench 的输入、输出、resize、attach/detach、history replay。
```

### Gate

进入 M5 前必须确认：

```text
CLI 作为本地一等产品面可用；GUI/Web 可选且不拥有 runtime；可自动化的 M4 product-surface 检查已有单元测试或明确测试入口。
```

### Closeout decision

M4 可在完成文档同步和单元测试补齐后关闭；上述人工验证项进入 M5 Runtime Hardening。

---

## M5：Runtime Hardening

### Objective

把 CLI command runner 和 Session / Workspace runtime 从“能用”提升到“可靠”，并补齐 Go PTY feasibility 中尚未完成的风险验证。

### Deliverables

```text
process cleanup hardening
process tree cleanup verification
bounded history limits
backpressure handling
structured logs refinement
panic / crash handling
config validation hardening
API / internal error contract
integration tests
manual verification checklist
performance / stress checks
```

### Required risk verification

必须补齐：

```text
1. Claude Code 真实交互，而不只是 --version。
2. Codex 真实交互，而不只是 --version。
3. Ctrl+C / interrupt 在 Claude Code / Codex 中的真实行为。
4. 20+ command/session 长时间运行。
5. 大量 output 场景。
6. 高频 resize。
7. Claude Code / Codex 运行期间的子孙进程清理。
8. Web Terminal Technical Spike 的 backpressure 风险。
9. Session / Workspace metadata 与真实进程状态一致性。
```

### Spike code migration

当前工具：

```text
cmd/pty-spike
cmd/pty-testprogram
```

后续处理：

```text
1. 将可自动化场景迁移为 integration tests。
2. 将人工验证场景保留为 tools 或 manual verification helper。
3. 删除或隔离不再需要的 spike-only 逻辑。
4. 不把 spike 程序作为正式 runtime 依赖。
```

### Acceptance criteria

```text
1. CLI command runner 长时间运行可用。
2. Ctrl+C soft interrupt 和升级策略行为明确。
3. 大输出不会导致内存无限增长。
4. 进程异常退出能正确反映状态。
5. close/stop/kill 能清理进程树，或明确报告清理失败。
6. Claude Code / Codex 真实交互有验证记录。
7. Web terminal spike 风险已记录并给出 M6 前处理方案。
8. 有可重复运行的测试或检查命令。
```

### Gate

进入 M6 前必须确认：

```text
本地 CLI runtime 足够稳定，可以承受 Gateway 引入的远程 relay 和 Web UI 复杂度。
```

---

## M6：Gateway Web Terminal MVP

### Objective

引入 Gateway，并把 Web 作为正式跨设备产品入口。

Gateway 不拥有进程，不运行用户命令，只做：

```text
auth
device registry
session / workspace registry
routing
tunnel relay
```

### Target architecture

```text
Browser
  ↓
xterm.js
  ↓
Gateway
  ↓
Agent tunnel
  ↓
TermBridge runtime
  ↓
PTY / Process
```

### Deliverables

```text
Gateway service
Agent outbound tunnel
Web terminal UI
terminal websocket relay
device registration
session / workspace listing relay
single-user token auth
connection status
```

### Minimal model

```text
Device
SessionRoute / WorkspaceRoute
TunnelConnection
RemoteAttach
```

### Acceptance criteria

```text
1. Agent 主动连接 Gateway。
2. Gateway 能看到在线 device。
3. Browser 通过 Gateway 能看到可 attach 的 session/workspace。
4. Browser 通过 xterm.js attach terminal。
5. Agent 断线后 Gateway 清理路由。
6. Agent reconnect 后 device online 状态恢复。
7. Gateway 不直接运行用户进程。
8. Web terminal 基础风险已通过 M2.5/M5 承接，不在 M6 首次暴露。
```

### Risks

```text
如果 M5 未通过，Gateway 会放大 Runtime 问题。
如果 Gateway 误持有 runtime 状态，会破坏 CLI/runtime ownership 边界。
如果 Web terminal spike 不充分，M6 会同时暴露 UI、relay、auth、routing 问题。
```

### Gate

进入 M7 前必须确认：

```text
Gateway relay 只负责连接和路由，不改变 TermBridge runtime ownership。
```

---

## M7：Multi-device Beta

### Objective

让跨设备访问成为可测试产品形态。

### Deliverables

```text
device registry
session / workspace registry
multi attach over Gateway
connection status
basic audit log
token rotation
agent pairing flow
remote access UX
```

### User flow

```text
1. 用户在本地通过 termbridge CLI 运行命令或启动可远程 attach 的 session。
2. Agent / runtime 与 Gateway 建立连接。
3. 用户在 Gateway 页面绑定 device。
4. Gateway 显示 device sessions / workspaces。
5. 用户远程 attach。
6. 多设备可以查看同一个 runtime output。
```

### Acceptance criteria

```text
1. PC / Laptop / Phone 至少两个设备可访问。
2. NAT 环境下 Agent 无需入站端口。
3. 远程断线不影响本地 runtime。
4. 多 attach over Gateway 可用。
5. Gateway 不直接运行用户进程。
6. 权限边界清晰。
```

### Risks

```text
Pairing token 和 long-lived token 的存储安全。
移动端弱网导致 backpressure 和 reconnect 问题。
Audit log 可能泄漏命令或路径信息，需要脱敏策略。
```

---

## M8：Security / Packaging / Release

### Objective

准备真实发布。

### Deliverables

```text
security model
threat model
release packages
installer / archive
upgrade strategy
config migration
docs
minimal CI checks
```

### Security focus

```text
1. Gateway auth。
2. Agent / runtime pairing。
3. token storage。
4. local CLI trust boundary。
5. terminal access authorization。
6. command execution trust boundary。
7. log secret redaction。
8. public network exposure warning。
9. local config permissions。
```

### Packaging direction

当前技术方向下优先考虑：

```text
Go CLI binary
```

可选发布形态：

```text
zip archive
installer
portable binary
GUI container package（可选）
service / tunnel mode（Gateway 阶段后续）
```

### Acceptance criteria

```text
1. 用户能按文档安装 termbridge CLI。
2. 用户能安全运行本地命令。
3. Gateway 远程访问有明确安全边界。
4. 有配置升级 / 回滚说明。
5. 有最小自动化测试。
6. Release artifact 可重复构建。
```

---

## Recommended first six weeks

### Week 1：M0 收口与文档整理

状态：已基本完成。

```text
完成：
- Go PTY feasibility。
- Go + go-pty 技术方向确认。
- CLI-first / GUI container / Gateway Web 路线确认。
- roadmap refresh requirement / spec / plan。
```

### Week 2：M1 Go CLI Skeleton

```text
目标：termbridge CLI 可启动、可解析参数、可观测。
交付：--help、--version、--cwd、exec -- separator、logging、error model、test command。
验收：CLI 入口和错误边界成立。
```

### Week 3：M2 PTY Command Runner MVP

```text
目标：termbridge [options] exec -- <command...> 可运行真实命令。
交付：PTY abstraction、CommandRunner、stdin/stdout relay、resize、Ctrl+C、exit code。
验收：Claude/Codex/shell 基础交互可用。
```

### Week 4：M2.5 Web Terminal Technical Spike + M3 skeleton

```text
目标：提前验证 Web terminal 风险，同时开始 session/workspace 模型。
交付：dev-only xterm.js harness、WebSocket relay prototype、Session metadata skeleton。
验收：Web terminal 风险有记录，Session model 不破坏 CLI-first。
```

### Week 5：M3 Session / Workspace Runtime Model

```text
目标：为 history、GUI container、Gateway 映射建立 runtime model。
交付：state model、bounded history、logs、metadata、session listing/history 提供基础 recent 能力。
验收：CLI 使用不变，内部模型可承接后续产品化。
```

### Week 6：M4 CLI Product Surface + M5 hardening start

```text
目标：打磨本地 CLI 产品面，开始风险补测。
交付：clear help/errors、config、logs、manual checklist、Claude/Codex 真实交互补测。
验收：本地 CLI 可日常使用，进入 hardening gate。
```

## Implementation order inside each milestone

每个 milestone 内建议按以下顺序推进：

```text
1. 明确最小用户能力。
2. 定义 CLI/API/内部状态边界。
3. 实现最薄的 vertical slice。
4. 加入错误处理和日志。
5. 固化最小测试或人工验证命令。
6. 更新对应 verification / milestone note。
7. 做 Continue / Adjust / Stop 判断。
```

## Verification plan for future milestones

本计划本身不进入 Verification 阶段，但后续每个 milestone 应有独立验证记录。

建议验证维度：

```text
Requirement alignment
Actual diff summary
Expected vs actual changed files
Acceptance criteria checklist
Command results
Manual verification notes
Known risks
Incomplete items
Conclusion
```

M2 起必须优先使用项目显式检查命令；如果尚未建立检查命令，milestone 输出应包含“如何运行最小检查”。

## Rollback / adjustment strategy

### 技术方向调整

如果 M2/M3/M5 发现 Go + go-pty 无法满足关键场景，不直接重写 CLI 或上层产品模型，而是优先替换 PTY implementation：

```text
TermBridge PTY abstraction
  ↓
Alternative PTY backend
```

可重新评估：

```text
Rust portable-pty
Node node-pty
自研 Windows ConPTY wrapper
```

### Scope adjustment

如果 milestone 过大，优先缩小用户能力，不破坏架构边界。

例如：

```text
先支持 PowerShell / pwsh，暂缓 cmd.exe interrupt。
先支持 CLI command runner，暂缓后台 session。
先做 Web technical spike，暂缓 Web product。
先做 GUI container spike，禁止 GUI runtime fork。
```

### Stop condition

若出现以下情况，应暂停进入下一阶段：

```text
1. PTY read/write/close 存在不可控死锁。
2. Ctrl+C / stop 无法可靠结束常见命令。
3. stop 后高概率残留进程树。
4. 命令透传语义无法稳定表达用户参数。
5. Claude Code / Codex 真实交互不可用且无替代策略。
6. Web terminal spike 证明 xterm.js / WebSocket 基础链路不可接受且无替代方案。
```

## User review notes

本计划已根据用户最新决策更新：本地阶段以 `termbridge` CLI 为唯一一等入口；GUI 是 CLI 容器；Web 技术验证前置，但 Gateway 阶段才作为正式 Web 产品入口。

当前状态为 Draft，等待用户 review。