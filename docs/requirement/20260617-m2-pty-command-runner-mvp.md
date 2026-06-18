# M2 PTY Command Runner MVP 需求
最后修改时间: 2026-06-17 19:42:48

Review status: Accepted

## Background

TermBridge-go 的路线已经确认：本地阶段以 `termbridge` CLI 作为唯一一等入口，标准使用方式为：

```text
termbridge [options] -- <command...>
```

M1 已用于建立 Go CLI Skeleton，包括 CLI contract、配置、日志、错误模型、版本信息和最小测试边界。M2 在此基础上进入第一个真正的 runtime vertical slice：通过 TermBridge 自己的 PTY abstraction 运行用户命令。

当前已确认技术方向：

```text
Windows PTY Runtime: Go
PTY 技术方向: github.com/aymanbagabas/go-pty
底层机制: Windows ConPTY / Pseudoconsole
```

M2 的目标不是完整 Workspace 平台，而是先让以下链路成立：

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

同时，M2 必须吸收 feasibility 与 roadmap 中已经暴露的 Windows 风险：路径解析、cwd 行为、terminal input 语义、Ctrl+C 行为不一致、PTY close 后 exit 状态、子进程清理和长输出 read loop。

## Goal

M2 的目标是实现最小可用的 PTY Command Runner MVP：用户可以通过 `termbridge [options] -- <command...>` 在 PTY runtime 中运行任意命令，并获得可预测的输入、输出、退出码和停止行为。

M2 完成后应明确并实现：

1. `ProcessSpec`：表达 command、args、cwd、env、initial terminal size 等运行参数。
2. `CommandRunner`：把 CLI 解析出的用户命令转换为 PTY runtime 执行。
3. TermBridge 自有 `PTYSession` / `PTYManager` abstraction，业务层不直接依赖 `go-pty.Pty`。
4. Process launcher：启动用户命令前 resolve executable path，避免 Windows `CreateProcess` 路径解析差异。
5. stdin relay：把用户终端输入转发到底层 PTY。
6. stdout/stderr relay：把 PTY 输出转发到用户终端，且不被 TermBridge 自身日志污染。
7. terminal resize relay：把用户 terminal resize 传递到底层 PTY。
8. Ctrl+C handling：区分 soft interrupt、close PTY、kill process tree，不把 Ctrl+C 当作 detach。
9. exit code propagation：用户命令退出后，尽可能把对应 exit code 作为 `termbridge` exit code。
10. cwd / env support：支持 `--cwd`，并为后续 env 扩展建立边界。
11. basic cleanup：命令结束、interrupt、close 或异常退出后，不留下基础 shell 子进程。
12. long output safety：长输出不会导致 read loop 死锁或无限内存增长。
13. 最小测试与人工验证入口：至少覆盖 CLI 到 runner 的核心路径、ProcessSpec 校验、executable resolve、cwd、长输出和基础交互命令。

## Non-goal

本需求不包括：

1. 不实现完整 Session / Workspace Runtime Model；M3 再引入 metadata、history、recent/list 等模型。
2. 不实现后台 daemon、detach、reattach 或持久运行的 Workspace。
3. 不实现 GUI。
4. 不实现正式 Web terminal 或 Gateway。
5. 不把 local Web 作为本地正式产品入口。
6. 不实现多设备访问、Device Registry、Workspace Registry 或 Agent tunnel。
7. 不把 `cmd/pty-spike` 或 `cmd/pty-testprogram` 作为正式 runtime 依赖。
8. 不保证 M2 已覆盖所有 Claude Code / Codex 真实 TUI 长期稳定性；M2 需要基础人工验证，完整 hardening 留到 M5。
9. 不在未授权情况下执行 git 写操作。
10. 不创建 verification 文档，除非后续明确进入 Verification / 验证阶段。

## User scenarios

### 场景一：运行普通命令

作为用户，我希望运行：

```text
termbridge -- pwsh -NoLogo
```

TermBridge 应在 PTY runtime 中启动 `pwsh`，把我的输入转发给 shell，并把 shell 输出显示到当前终端。

### 场景二：指定 cwd 运行命令

作为用户，我希望运行：

```text
termbridge --cwd D:\project -- pwsh -NoLogo
```

底层进程应在 `D:\project` 下启动。若目录不存在或不是目录，TermBridge 应在启动 PTY 前 fail fast。

### 场景三：运行带参数的用户命令

作为用户，我希望运行：

```text
termbridge --cwd D:\project -- npm run dev
```

`--` 后的内容必须原样作为用户命令及参数，不应被 TermBridge 继续解析或吞掉。

### 场景四：运行 Agent CLI

作为用户，我希望运行：

```text
termbridge -- claude
termbridge -- codex
```

M2 至少应能进行基础启动和人工交互验证；如果真实 TUI 交互存在限制，必须记录实际表现和未完成风险，不能把 `--version` smoke 当作完整交互成功。

### 场景五：停止当前命令

作为用户，当我在 `termbridge -- <command...>` 运行期间按下 `Ctrl+C`，我的意图是停止当前前台命令，而不是 detach。

M2 应尝试：

```text
first Ctrl+C:
  soft interrupt foreground process

grace period:
  wait for process exit

second Ctrl+C or timeout:
  close PTY / kill process tree
```

如果某些 shell 或 Agent CLI 的 Ctrl+C 行为不一致，TermBridge 应明确输出状态并进入升级策略，不应无限等待。

### 场景六：命令退出码可预测

作为用户，我希望用户命令退出后，`termbridge` 返回对应 exit code。TermBridge 自身错误仍使用 M1 已约定的保留 exit code，不能与用户命令退出混淆。

### 场景七：长输出不会卡死

作为用户，我希望运行会产生大量输出的命令时，TermBridge 能持续读取并转发输出，不因为 read loop、buffer 或慢写入导致死锁。

## Acceptance

M2 完成后应满足：

- [ ] 存在 M2 requirement 文档，明确 PTY Command Runner 的目标、边界、验收标准、风险和待定事项。
- [ ] `termbridge -- <command...>` 可以通过正式 runtime 路径运行用户命令，不再只返回 M1 的 runner not implemented。
- [ ] `--cwd` 指定目录对底层用户进程生效。
- [ ] 未传 `--cwd` 时使用当前目录。
- [ ] `--` 后的 command 与 args 原样传递给用户命令，TermBridge 不继续解析用户参数。
- [ ] 启动进程前进行 executable path resolve，避免 Windows `CreateProcess` 路径解析差异。
- [ ] 定义 TermBridge 自己的 PTY abstraction，业务层不直接依赖 `go-pty.Pty`。
- [ ] 定义并使用 `ProcessSpec` 或等价结构，覆盖 command、args、cwd、env、initial size。
- [ ] 定义并使用 `CommandRunner` 或等价边界，负责从 spec 到 runtime execution。
- [ ] stdin relay 可用，基础交互式命令可操作。
- [ ] stdout/stderr relay 可用，TermBridge 自身日志不污染用户命令输出。
- [ ] terminal resize 能传递到底层 PTY。
- [ ] Ctrl+C 能停止常见前台命令；失败时有明确升级策略。
- [ ] interrupt、close PTY、kill process tree 在代码边界上不能混成一个操作。
- [ ] 用户命令退出码尽可能传递为 `termbridge` exit code。
- [ ] TermBridge 自身 usage/config/internal/runtime error 仍使用保留 exit code，不与用户命令退出混淆。
- [ ] close / interrupt 后不留下基础 shell 子进程；如果清理失败，应明确记录或报告。
- [ ] 长输出不会导致 read loop 死锁。
- [ ] `cmd.exe Ctrl+C` 等已知不稳定场景不能被误判为通用 interrupt 成功。
- [ ] `go test ./...` 通过。
- [ ] 至少提供或记录一组可重复执行的本地检查命令。
- [ ] 至少记录 Claude Code / Codex 的基础人工验证结果或未完成风险。

## Technical decisions

### Runtime boundary

M2 必须保持以下方向：

```text
CLI command
  ↓
internal app orchestration
  ↓
CommandRunner
  ↓
TermBridge PTY abstraction
  ↓
go-pty implementation
```

业务层和 CLI 层不直接依赖 `go-pty.Pty`。`go-pty` 应被隔离在 runtime implementation 边界内，以便未来必要时替换为 alternative PTY backend。

### CLI contract

M2 继续沿用 M1 的规范合同：

```text
termbridge [options] -- <command...>
```

M2 不引入 `termbridge <command...>` 语法糖。该语法糖可以后续评估，但不能影响当前 `--` 分隔合同。

### Ctrl+C semantics

M2 必须明确：

```text
Ctrl+C 是停止当前命令，不是 detach。
```

MVP 策略为：

```text
1. 捕获 Ctrl+C。
2. 对 foreground process 尝试 soft interrupt。
3. 等待短暂 grace period。
4. 如果用户再次 Ctrl+C 或 grace period 后仍未退出，则 close PTY / kill process tree。
5. 输出明确状态，不无限等待。
```

### Windows requirements

M2 必须吸收以下 Windows-specific requirements：

```text
1. 启动进程前先 resolve executable path。
2. 设置 cwd 时避免 Windows CreateProcess 路径解析差异。
3. Windows terminal command submit 按 CRLF 语义处理。
4. ClosePseudoConsole 后 0xc000013a 在 close/cleanup 语义下可以接受。
5. cmd.exe Ctrl+C 失败不能被误判为通用 interrupt 成功。
6. interrupt / close / kill process tree 不能混成一个操作。
```

### Exit code

沿用 M1 的 TermBridge 自身错误 exit code 约定，并新增 M2 行为：

```text
用户命令正常退出:
  termbridge 尽可能返回用户命令 exit code

TermBridge 自身错误:
  使用保留 exit code，不与用户命令退出混淆
```

如果 Windows PTY close / interrupt 导致特殊状态码，例如 `0xc000013a`，实现必须在 close/cleanup 语义下明确分类，不能把所有异常码都当作 TermBridge internal error。

### Test boundary

M2 测试应优先覆盖可自动化的边界：

```text
unit: ProcessSpec validation
unit: executable path resolve
unit: CLI-to-runner argument handoff
integration: termbridge -- pwsh -NoLogo 或等价 shell command
integration: cwd / env
integration: long output
manual: Claude Code real interaction
manual: Codex real interaction
manual: Ctrl+C interrupt behavior
manual: cleanup / no obvious leftover process
```

如果某些交互验证无法自动化，应在后续 Verification / 验证文档中记录实际手动步骤和结果。

## Open questions

需要用户关注的未决事项：

1. Ctrl+C grace period 的具体时长尚未确认。建议 M2 先采用短暂固定值并在 plan 中记录，例如 1-2 秒，后续根据真实交互调整。
2. M2 是否需要支持用户通过配置覆盖 interrupt timeout / grace period 尚未确认。建议 M2 暂不暴露配置，避免过早扩展。
3. M2 的 env support 最小范围尚未确认：是只继承当前进程环境，还是同时支持配置/CLI 注入额外 env。建议 M2 先继承当前环境，保留 `ProcessSpec.Env` 边界。
4. process tree cleanup 的实现方式需要在 Plan 阶段结合现有代码与 Windows API 可行性确认。
5. terminal resize 的自动化测试方式可能受当前测试环境限制，可能需要人工验证记录。
6. Claude Code / Codex 真实 TUI 交互是否能在当前本地环境完整验证，取决于本机安装、登录状态和交互授权。

上述事项不阻塞 requirement 草稿形成；如果用户要求进入 Plan / 计划，可将其作为 assumptions / risks 继续推进。

## Decisions

当前已明确：

1. 当前流程为 standard / 标准模式。
2. 当前阶段为 Requirement / 需求。
3. M2 是 `PTY Command Runner MVP`，不是 Workspace / Gateway 产品化阶段。
4. M2 继续坚持 CLI-first，本地一等入口仍是 `termbridge [options] -- <command...>`。
5. M2 必须通过 TermBridge 自己的 PTY abstraction 隔离 `go-pty`。
6. `cmd/pty-spike` 和 `cmd/pty-testprogram` 不能成为正式 runtime 依赖。
7. Ctrl+C 语义是停止当前命令，不是 detach。
8. GUI、Web、Gateway 不进入 M2。
9. M2 完成后默认停在 Implementation / 实现阶段等待用户验收；除非用户明确要求，不自动进入 Verification / 验证。

## Risk

### 风险一：PTY abstraction 不清导致后续难以替换 backend

如果业务层直接依赖 `go-pty.Pty`，未来发现 Go + go-pty 无法覆盖关键场景时，会被迫重写上层 runtime。M2 必须先建立 TermBridge 自己的 abstraction。

### 风险二：Ctrl+C 在不同 shell / Agent CLI 中行为不一致

Windows 下 `cmd.exe`、PowerShell、pwsh、Claude Code、Codex 的 interrupt 行为可能不同。M2 不能用单一成功样例推断所有命令都可靠，必须有升级策略和验证记录。

### 风险三：cleanup 不彻底导致残留进程

Agent CLI 可能产生子孙进程。只 close PTY 不一定等价于清理进程树。M2 至少要覆盖基础 cleanup，并把复杂场景纳入 M5 hardening 风险。

### 风险四：长输出和 relay 阻塞

如果 stdout/stderr relay 或 terminal write 处理不当，长输出可能导致 read loop 阻塞或内存增长。M2 应以最小方式验证大量输出场景。

### 风险五：TermBridge 自身日志污染用户输出

M2 开始真实转发用户命令输出后，TermBridge 自身日志如果写入同一输出流，会影响终端体验和自动化命令结果。日志边界必须继续遵守 M1 约定。

### 风险六：过早引入 Workspace / daemon

M2 的目标是 command runner vertical slice。若提前引入后台 session、daemon 或复杂 Workspace lifecycle，会破坏 CLI-first 心智，并把 M3/M5 风险提前混入 M2。

## User review notes

用户要求：使用 standard / 标准模式，开始任务 “M2: PTY Command Runner MVP”。

用户已要求进入 Plan / 计划阶段，本 requirement 按 SpecFlow 流转规则标记为 Accepted。