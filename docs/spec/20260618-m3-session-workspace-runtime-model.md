# M3 Session / Workspace Runtime Model 规格
最后修改时间: 2026-06-18 11:07:45

Review status: Accepted

## Requirement basis

本规格基于已接受的需求文档：

```text
docs/requirement/20260618-m3-session-workspace-runtime-model.md
```

当前流程为 strict / 严格模式，当前阶段为 Spec / 规格。

M3 的核心目标不是把 TermBridge 做成 daemon、GUI、Web terminal 或 Gateway，而是在 M2 PTY Command Runner MVP 之上建立可复用的 Workspace / Session runtime model：

```text
Workspace
  ↓
Session
  ↓
PTY
  ↓
Process
```

其中：

- Workspace 是未来 Gate / GUI 中展示的个人工作区对象。
- Session 是一次具体运行会话，归属于一个 Workspace。
- Session 记录发起目录、命令、进程、状态、history、退出信息和日志位置。
- M3 仍保持 CLI-first，不引入常驻 daemon，不实现 detach / reattach 产品化。

## Overview

M3 在现有 M2 链路：

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
```

外层增加 Workspace / Session / History / Persistence 编排，但不改变 PTY adapter 的职责边界。

新的运行链路为：

```text
termbridge [root options] exec [exec options] -- <command...>
  ↓
internal/cli parses root command + subcommand
  ↓
internal/app resolves config and launch cwd
  ↓
WorkspaceResolver derives or creates Workspace from launch cwd
  ↓
SessionManager creates Session under Workspace
  ↓
SessionStore persists metadata / process / state / history
  ↓
runner.CommandRunner starts PTY and user process
  ↓
Session lifecycle updates state, process, history and exit record
```

查询链路为：

```text
termbridge workspace
  ↓
load .termbridge workspace records
  ↓
print workspace list

termbridge session
  ↓
load .termbridge workspace/session records
  ↓
refresh stale running records by process match
  ↓
print session list
```

M3 的重点是把“运行一次命令”变成“创建、记录、维护和查询一次 Session”，同时让多个 Session 被稳定归入由 launch cwd 推导出的 Workspace。

## Design decisions

### 1. Workspace 与 Session 是显式领域概念

M3 采用用户确认后的模型：

```text
Workspace 1 ── * Session 1 ── 1 PTY 1 ── 1 Process
```

说明：

- Workspace 不是 PTY，也不是 Process。
- Session 不是 Workspace 的别名。
- 一个 Workspace 可以有多个历史或当前 Session。
- 一个 Session 在 M3 中只拥有一个 user command process path。
- M3 不实现一个 Session 内多个 PTY、multi-attach 或 daemon ownership。

`docs/design.md` 中早期“Workspace 是唯一状态对象 / 不存在额外 Session 层”的表述是 M2 前架构取舍；本 M3 规格以已接受需求为准，将本地 runtime 领域模型更新为 Workspace → Session → PTY → Process。后续如需同步设计总文档，应在单独文档更新中处理。

### 2. Workspace 由 launch cwd 稳定映射

Session 的 launch cwd 来源：

1. 用户未传 `--cwd`：使用当前进程工作目录解析后的绝对路径。
2. 用户传根级 `--cwd <path>`：使用该路径解析后的绝对路径。

Workspace 由规范化后的 launch cwd 稳定映射生成。

M3 使用两个标识：

| 字段 | 用途 | 稳定性 |
| --- | --- | --- |
| `workspace_key` | 文件系统目录名，来自 launch cwd 的稳定 hash-like mapping | 同一路径稳定复用 |
| `workspace_id` | 领域 ID，ULID | Workspace 第一次创建时生成，之后持久化复用 |

这样满足两个约束：

- `.termbridge/<workspace>/<session>` 中 `<workspace>` 是目录映射，适合从路径稳定定位。
- workspace id 仍是 ULID，适合未来 Gate / SQLite / API 领域建模。

#### Workspace key 生成规则

M3 推荐规则：

```text
workspace_key = base32lower(sha256(normalized_launch_cwd))[0:26]
```

约束：

- 只使用小写字母和数字，避免 Windows 路径非法字符。
- 不把完整路径直接放入目录名，避免冒号、反斜杠、空格、过长路径和隐私暴露。
- `workspace.json` 中记录原始绝对路径和展示名，用于列表展示。

#### 路径规范化规则

M3 至少要求：

- 将 cwd 解析为绝对路径。
- 清理 `.` / `..`。
- 在 Windows 上使用 volume + path 的稳定表示。
- 不要求解析 symlink / junction 到真实路径；如未来需要，可作为迁移策略处理。
- hash 输入应记录 schema version，避免未来规范化策略改变时无法迁移。

### 3. Session id 使用 ULID

`session_id` 使用 ULID，并作为 `.termbridge/<workspace>/<session>` 中 `<session>` 目录名。

约束：

- 每次 `exec` 创建一个新的 Session。
- M3 不暴露 `--session-id` 作为用户可传参数。
- Session id 是内部持久化和领域建模机制，可用于 `termbridge session` 输出和后续 GUI/Gate 关联。

### 4. 文件系统 persistence 是有 schema 的领域存储

M3 不把 `.termbridge` 当作随意文件堆叠，而是明确 schema、文件名和写入规则，为未来迁移 SQLite 留边界。

开发模式持久化根目录：

```text
<effective cwd>/.termbridge
```

M3 记录布局：

```text
.termbridge/
  workspace-index.json
  <workspace_key>/
    workspace.json
    <session_id>/
      session.json
      process.json
      state.json
      history.log
      exit.json
```

其中主要求中的 `.termbridge/<workspace>/<session>` 对应：

```text
.termbridge/<workspace_key>/<session_id>
```

`workspace-index.json` 是可选但推荐的索引文件，用于快速列出 Workspace；真实数据仍以各 `workspace.json` 为准，索引损坏时可通过扫描 `<workspace_key>/workspace.json` 重建。

### 5. 日志位置继续使用 effective cwd 下 logs

日志不迁入 session 目录。M3 继续沿用当前配置逻辑：

```text
<effective cwd>/logs/termbridge.log
```

Session metadata 必须记录该日志路径：

```json
{
  "log_path": "D:\\project\\logs\\termbridge.log"
}
```

理由：

- 保持 M2 日志行为稳定。
- 避免用户命令输出、history 和 TermBridge 自身日志混为一谈。
- 通过 metadata 建立 session record 与 log file 的可追踪关联。

### 6. History 使用 line-first then bytes 限制

M3 history 是“最近输出缓存”，不是完整 terminal replay store。

默认值：

| 配置项 | 默认值 | 说明 |
| --- | ---: | --- |
| `history.max_lines` | `10000` | 类似主流 terminal scrollback 的较保守默认，明显大于一屏 |
| `history.max_bytes` | `5242880` | 5 MiB，防止长时间输出无限增长 |
| `history.max_line_bytes` | `65536` | 64 KiB，防止单行极长导致展示和存储问题 |

裁剪顺序：

1. 写入时按 PTY 输出流累计为 history entry；遇到换行形成 logical line，未换行部分作为 partial line。
2. 单条 logical line 超过 `max_line_bytes` 时截断中间或尾部，并标记 `truncated=true`。
3. 总行数超过 `max_lines` 时，从最旧 line 开始丢弃。
4. 行数裁剪后，如果总 bytes 仍超过 `max_bytes`，继续从最旧 line 开始丢弃，直到满足 bytes 限制。

`history.log` 在 M3 采用追加文本格式即可，但必须由 history component 统一写入，避免业务层散落文件操作。

推荐格式：

```text
<raw PTY bytes decoded as UTF-8 with replacement>
```

同时在 `session.json` 或 `state.json` 中记录 history limits 和是否发生过 truncation。M3 不要求实现 ANSI 解析、屏幕状态重建或 full replay。

### 7. 状态机使用枚举和受控 transition

Session state 至少包含：

```text
starting
running
stopping
stopped
failed
```

状态语义：

| State | 语义 |
| --- | --- |
| `starting` | Session 已创建，正在启动 PTY/process，process 可能尚未成功 start |
| `running` | user process 已启动，TermBridge 正在 relay IO 并维护状态 |
| `stopping` | TermBridge 已收到 interrupt/close/cleanup 路径，正在停止 process |
| `stopped` | user process 已结束，已有 exit record |
| `failed` | TermBridge runtime 在启动或维护时失败，不能视为正常 user command exit |

允许 transition：

```text
starting -> running
starting -> failed
running  -> stopping
running  -> stopped
running  -> failed
stopping -> stopped
stopping -> failed
failed   -> stopped   // 仅恢复扫描确认已有 exit record 时允许修正
```

禁止：

- 任意字符串状态散落在代码中。
- 查询命令直接把 stale `running` 当作真实 running 输出。
- 正常用户命令非零 exit code 进入 `failed`；它应是 `stopped` + exit code 非零。

### 8. 启动恢复只识别状态，不承诺 reattach

M3 recovery scope：

- 扫描 `.termbridge/<workspace>/<session>`。
- 读取 `workspace.json`、`session.json`、`process.json`、`state.json`、`exit.json`。
- 对 `starting` / `running` / `stopping` 等非终态记录执行 alive check。
- 如果进程不存在或不匹配，则将记录更新为终态或失败识别状态，避免 stale running 永久残留。

M3 不做：

- 不恢复 PTY handle。
- 不 reattach 到旧进程。
- 不接管 owner-crash 后遗留 process tree。
- 不实现 daemon 级别 process supervisor。

#### Alive check 匹配字段

`process.json` 至少记录：

- `pid`
- `executable`
- `command`
- `args`
- `cwd`
- `started_at`
- `owner_pid`

Windows 上 M3 alive check 推荐：

1. 检查 pid 是否存在。
2. 如果可读取 process executable/path 或 command line，则与记录的 `executable` / `command` / `cwd` 做 best-effort match。
3. 如果 pid 存在但无法确认匹配，保守视为 `unknown/stale`，在 M3 状态集中落到 `failed` 并记录 reason，而不是继续显示 `running`。
4. 如果 pid 不存在，将 state 更新为 `failed` 或 `stopped`：
   - 有 `exit.json`：`stopped`。
   - 无 `exit.json`：`failed`，reason=`stale_process_missing`。

由于 M3 state 集合不包含 `unknown`，恢复无法确认时使用 `failed` 并在 `state.reason` 中写明 `stale_*`。后续如需要更细状态，可在后续阶段扩展。

### 9. CLI 入口切换为 subcommand grammar

M3 直接移除旧入口，不保留兼容期。

新的语法：

```text
termbridge [root options] <command> [command options]

Commands:
  exec       run a command through TermBridge PTY runtime
  workspace  list workspace records
  session    list session records
```

执行命令：

```text
termbridge [root options] exec [exec options] -- <command...>
```

示例：

```text
termbridge exec -- claude
termbridge exec -- pwsh -NoLogo
termbridge --cwd D:\project exec -- npm run dev
```

根级 options：

| Option | Scope | 说明 |
| --- | --- | --- |
| `--cwd <path>` | root | TermBridge effective cwd / session launch cwd |
| `--help` | root | 根命令帮助 |
| `--version` | root | 版本 |

exec 级 options：

M3 可先不引入 exec-specific options，但 parser 必须预留边界：

```text
termbridge [root options] exec [exec options] -- <command...>
```

若 M3 没有 exec option，`exec --help` 输出 exec 帮助即可。

旧入口行为：

```text
termbridge -- pwsh
```

应返回 usage error，并提示使用：

```text
termbridge exec -- pwsh
```

不要默默兼容或自动 rewrite。

### 10. workspace / session 输出先用稳定文本表格

M3 不引入 JSON output flag，避免扩大范围；但输出字段应稳定，后续可添加 `--json`。

`termbridge workspace` 输出字段：

```text
WORKSPACE ID                 KEY                         NAME        PATH              SESSIONS  UPDATED
01J...                       3f7k...                     project-a   D:\project-a      4         2026-06-18T10:00:00Z
```

至少包含：

- Workspace id。
- Workspace key。
- human-readable name。
- launch/root path。
- session count。
- updated time。

`termbridge session` 输出字段：

```text
SESSION ID                  WORKSPACE ID                STATE    EXIT  COMMAND          CWD            UPDATED
01J...                      01J...                      stopped  0     pwsh -NoLogo     D:\project     2026-06-18T10:00:00Z
```

至少包含：

- Session id。
- Workspace id 或 workspace name/key。
- state。
- exit code / exit reason。
- command / args 摘要。
- cwd。
- log path 可在宽输出不足时省略，但必须可通过 session metadata 找到；M3 文本输出可追加 `LOG` 列或下一行详情。

M3 查询命令默认扫描当前 effective cwd 下 `.termbridge`。跨项目全局 registry 不在 M3 范围。

## Affected components

### `internal/cli`

现状：

- 当前 parser 直接寻找根级 `--`。
- 当前 usage 是 `termbridge [options] -- <command> [args...]`。

M3 需要：

- 引入 command/subcommand parse model。
- 根级 options 在 subcommand 前解析。
- `exec` subcommand 自己解析 `-- <command...>`。
- `workspace` / `session` 不要求用户命令 separator。
- 旧 `termbridge -- <command...>` 返回 usage error。

建议结构：

```go
type CommandKind string

const (
    CommandExec      CommandKind = "exec"
    CommandWorkspace CommandKind = "workspace"
    CommandSession   CommandKind = "session"
)

type Options struct {
    Cwd         string
    CommandKind CommandKind
    Exec        ExecOptions
    ShowHelp    bool
    ShowVersion bool
}

type ExecOptions struct {
    Command []string
}
```

### `internal/app`

现状：

- `app.Options` 只有 `Cwd`、`Command` 和 IO。
- `app.Run` 直接创建 `ProcessSpec` 并调用 `runner`。

M3 需要：

- 按 command kind 分发：exec / workspace / session。
- exec path 在调用 runner 前创建 Workspace / Session record。
- runner 输出同时写用户 stdout 和 history sink。
- runner exit 后更新 state / exit record。
- workspace/session path 调用 storage/recovery/listing，不启动 PTY。

建议结构：

```go
type Options struct {
    Cwd     string
    Command Command
    Stdin   io.Reader
    Stdout  io.Writer
    Stderr  io.Writer
}

type Command struct {
    Kind CommandKind
    Exec ExecCommand
}
```

### `internal/config`

现状：

- 解析 cwd、command、log level、log format、log dir。
- log dir 相对 effective cwd。

M3 需要：

- 保持 log location 行为不变。
- 增加 history limits 配置。
- 增加 state root 配置的内部默认值，但 M3 不需要暴露 CLI flag。

建议配置：

```go
type HistoryConfig struct {
    MaxLines     int
    MaxBytes     int64
    MaxLineBytes int
}

type RuntimeConfig struct {
    StateDir string // default: .termbridge under effective cwd
}
```

配置默认值：

```text
history.max_lines = 10000
history.max_bytes = 5242880
history.max_line_bytes = 65536
runtime.state_dir = .termbridge
```

### `internal/runner`

现状：

- `CommandRunner` 负责 PTY lifecycle、stdin/stdout relay、interrupt、exit code。

M3 需要：

- 不把 Workspace / Session 持久化塞进 runner。
- 允许 stdout relay 写入多个 sink：用户 stdout + history writer。
- 保持 runner 的主要职责：运行和停止 process。

如果当前 runner 只接收单一 `Stdout io.Writer`，M3 可在 app 层用 `io.MultiWriter(options.Stdout, historyWriter)` 注入，不改变 runner 抽象。

### `internal/pty` / `internal/pty/gopty`

M3 不改变 PTY abstraction 主合同。

可以在不破坏接口的前提下补充 process pid 获取方式；如果 `go-pty` adapter 暂时不能暴露 pid，则 M3 process record 至少在 app/runner 启动返回后通过 adapter result 或扩展接口写入。

推荐最小扩展：

```go
type Session interface {
    io.Reader
    io.Writer
    Resize(size process.TerminalSize) error
    Interrupt() error
    Close() error
    KillTree() error
    Wait() Result
    ProcessInfo() process.Record // 或等价只读方法
}
```

如果为避免破坏现有测试，也可新增可选接口：

```go
type ProcessReporter interface {
    ProcessInfo() process.Record
}
```

### 新增 `internal/workspace`

职责：

- 从 launch cwd 推导 workspace key。
- 创建 / 读取 / 更新 `workspace.json`。
- 维护 workspace 与 session 的关系。
- 提供 list workspace 数据给 CLI 输出。

核心类型建议：

```go
type Workspace struct {
    ID        string
    Key       string
    Name      string
    Path      string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### 新增 `internal/session`

职责：

- 创建 Session metadata。
- 管理 state transition。
- 写入 process / exit / history metadata。
- 为 list session 提供视图模型。

核心类型建议：

```go
type Session struct {
    ID          string
    WorkspaceID string
    WorkspaceKey string
    LaunchCwd   string
    Command     CommandRecord
    State       State
    LogPath     string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### 新增 `internal/history`

职责：

- 提供 bounded writer。
- 同时保证 line limit、byte limit、line byte limit。
- 统一写 `history.log`。
- 暴露 truncation metadata。

### 新增 `internal/state` 或 `internal/storage`

职责：

- 封装 `.termbridge` 文件布局。
- JSON schema version。
- 原子写入。
- 扫描 / 恢复。
- 对损坏记录做 best-effort 跳过并报告 warning。

可命名为 `internal/state`，避免过早表达成最终数据库 abstraction。

## Persistence schemas

### `workspace.json`

```json
{
  "schema_version": 1,
  "workspace_id": "01J...",
  "workspace_key": "3f7k...",
  "name": "TermBridge-go",
  "path": "D:\\SourceCodes\\mywork\\TermBridge-go",
  "path_hash_input_version": 1,
  "created_at": "2026-06-18T10:00:00Z",
  "updated_at": "2026-06-18T10:01:00Z"
}
```

### `session.json`

```json
{
  "schema_version": 1,
  "session_id": "01J...",
  "workspace_id": "01J...",
  "workspace_key": "3f7k...",
  "launch_cwd": "D:\\SourceCodes\\mywork\\TermBridge-go",
  "command": {
    "executable": "C:\\Program Files\\PowerShell\\7\\pwsh.exe",
    "command": "pwsh",
    "args": ["-NoLogo"],
    "env_strategy": "inherit",
    "env_count": 64
  },
  "history": {
    "path": "history.log",
    "max_lines": 10000,
    "max_bytes": 5242880,
    "max_line_bytes": 65536,
    "truncated": false
  },
  "log_path": "D:\\SourceCodes\\mywork\\TermBridge-go\\logs\\termbridge.log",
  "created_at": "2026-06-18T10:00:00Z",
  "updated_at": "2026-06-18T10:01:00Z"
}
```

### `process.json`

```json
{
  "schema_version": 1,
  "pid": 12345,
  "owner_pid": 6789,
  "executable": "C:\\Program Files\\PowerShell\\7\\pwsh.exe",
  "command_line": "pwsh -NoLogo",
  "cwd": "D:\\SourceCodes\\mywork\\TermBridge-go",
  "started_at": "2026-06-18T10:00:00Z"
}
```

### `state.json`

```json
{
  "schema_version": 1,
  "state": "running",
  "reason": "process_started",
  "updated_at": "2026-06-18T10:00:01Z"
}
```

### `exit.json`

```json
{
  "schema_version": 1,
  "exit_code": 0,
  "reason": "user_process_exited",
  "forced": false,
  "closed": false,
  "started_at": "2026-06-18T10:00:00Z",
  "ended_at": "2026-06-18T10:01:00Z",
  "wait_error": ""
}
```

`exit.json` 只在 process 已有确定结束结果时写入。启动失败但没有 user process 的情况进入 `failed` state，错误详情可记录到 `state.reason` 和日志。

## State transition rules

### Exec happy path

```text
create workspace if needed
create session directory
write session.json
write state=starting
start PTY/process
write process.json
write state=running
relay output to stdout + history
wait process
write exit.json
write state=stopped
```

### Start failure

```text
create session directory
write state=starting
start PTY/process fails
write state=failed reason=start_failed
return TermBridge runtime error exit code
```

### User process exits non-zero

```text
state=running
process exits with code != 0
write exit.json exit_code=<code> reason=user_process_exited
write state=stopped
return same user exit code
```

非零 exit code 不是 TermBridge failed。

### Interrupt / close escalation

```text
state=running
interrupt requested
write state=stopping reason=interrupt_requested
runner handles interrupt -> close -> kill tree as needed
write exit.json with forced/closed flags
write state=stopped
```

### Recovery stale running

```text
scan state=running
read process.json
process missing or mismatch
write state=failed reason=stale_process_missing|stale_process_mismatch
```

如果同时存在可信 `exit.json`，则修正为：

```text
write state=stopped reason=recovered_exit_record
```

## CLI behavior details

### Root help

`termbridge --help` 输出应包含：

```text
Usage:
  termbridge [options] <command> [command options]

Commands:
  exec       run a command through a PTY
  workspace  list workspaces
  session    list sessions

Options:
  --cwd <path>
  --help
  --version
```

### Exec help

`termbridge exec --help` 输出应包含：

```text
Usage:
  termbridge [options] exec [exec options] -- <command> [args...]
```

### Missing command

```text
termbridge exec
```

返回 usage error：

```text
missing -- before exec command
```

### Missing subcommand

```text
termbridge
```

返回 usage error，并提示 commands。

### Old M2 grammar

```text
termbridge -- pwsh
```

返回 usage error：

```text
old command form is no longer supported; use: termbridge exec -- pwsh
```

### Workspace/session command cwd scope

`termbridge workspace` 和 `termbridge session` 默认读取 effective cwd 下 `.termbridge`。如果用户传：

```text
termbridge --cwd D:\project workspace
termbridge --cwd D:\project session
```

则读取 `D:\project\.termbridge`。

## Technical questions resolved in this spec

### Workspace human-readable name

M3 默认使用 launch cwd 的最后一个路径段作为 `workspace.name`。

示例：

```text
D:\SourceCodes\mywork\TermBridge-go -> TermBridge-go
```

若路径根无法产生名称，则使用 `workspace_key` 的短前缀。用户自定义 workspace name 不在 M3 范围。

### Process record 粒度

M3 至少记录 pid、owner pid、executable、command line、cwd、started_at。process tree 细节留到 M5 hardening。

### History 默认限制

本规格确定 M3 默认：

```text
max_lines = 10000
max_bytes = 5 MiB
max_line_bytes = 64 KiB
```

并通过 config 支持后续调整。

### Output format

M3 采用稳定文本表格，不实现 `--json`。后续如 Gateway/GUI 需要 machine-readable 输出，应直接复用 runtime packages，而不是解析 CLI 表格。

## Alternatives considered

### Alternative A: 只使用 Workspace，不引入 Session

拒绝。

原因：用户已明确 Workspace 和 Session 必须区分，且一个 Workspace 管理多个 Session。只使用 Workspace 会把多次运行的命令、状态和 history 混在一起，未来 Gate 展示也无法自然表达。

### Alternative B: `.termbridge/<workspace_id>/<session_id>`

拒绝作为 M3 主路径。

原因：需求已明确 `<workspace>` 是 launch cwd 的稳定目录映射。若直接使用 ULID 作为 workspace 目录名，无法仅凭 cwd 稳定定位 Workspace，必须额外全局索引，开发模式体验更脆。

### Alternative C: 直接上 SQLite

拒绝。

原因：M3 目标是领域建模和 CLI-first runtime，不是最终 storage 技术选型。文件系统 schema 足够支撑当前需求，并可为未来 SQLite 迁移提供清晰对象边界。

### Alternative D: 查询 running 必须依赖 daemon

拒绝。

原因：M3 不实现 daemon。当前只要求启动和查询时扫描持久化记录并做 process alive best-effort 识别，避免 stale running 永久残留。

### Alternative E: 继续兼容旧 `termbridge -- <command...>`

拒绝。

原因：用户已明确旧入口直接改掉，不做兼容。保留兼容会让根命令空间继续模糊，削弱 `workspace` / `session` / 后续命令扩展。

## Risks

### 风险一：文件写入一致性

Session 生命周期中会多次写 `state.json`、`process.json`、`exit.json` 和 `history.log`。如果直接覆盖写入，异常退出可能留下半文件。

规格约束：JSON 文件写入应使用临时文件 + rename 的原子写策略。history 可追加写，但需要 flush/close 语义明确。

### 风险二：Windows process match 能力有限

不同权限下读取 process executable 或 command line 可能失败。

规格约束：无法确认匹配时不能继续显示 `running`，应标记 `failed` 并记录 `stale_process_unverified` 或类似 reason。

### 风险三：history 与 stdout relay 相互影响

如果 history writer 阻塞，会影响用户命令输出体验。

规格约束：M3 history writer 必须轻量、同步失败可降级为记录错误到日志；不能让 history 持久化错误吞掉用户命令输出。

### 风险四：CLI parser 重构影响现有测试

M2 测试仍以 `termbridge [options] -- <command...>` 为入口。

规格约束：M3 必须同步更新 CLI/app 测试、help 文案、`just dev` recipe 和验证命令。旧入口失败是预期行为，不是回归。

### 风险五：`.termbridge` 位于项目目录可能污染用户项目

开发模式按需求写入项目目录，但可能影响用户工作树。

规格约束：M3 只在执行 `exec` 或查询已有 state 时创建/读取 `.termbridge`；文档和 verification 中需要提示该目录是 runtime state。是否加入 `.gitignore` 不在本规格中强制，但 Plan 阶段需检查项目约定。

### 风险六：ULID 依赖选择

Go 标准库不提供 ULID。

规格约束：Plan 阶段需要决定引入成熟小依赖还是实现本地最小 ULID。Spec 只要求字段语义和排序特性，不在此阶段选库。

## Out of scope for M3

- daemon。
- detach / reattach 产品化。
- multi-attach。
- GUI。
- Web terminal / Gateway。
- Gate workspace registry。
- SQLite 迁移。
- 完整 terminal replay。
- Agent TUI 长时间稳定性验证。
- owner-crash recovery。
- descendant process tree hardening。
- 复杂 process tree cleanup。

## User review notes

本规格吸收了用户在 M3 Requirement 阶段确认的关键决策：

- Workspace 与 Session 必须区分。
- Workspace 未来展示为 Gate 上的个人工作区列表。
- 一个 Workspace 管理多个 Session。
- Session 有发起目录。
- 命令入口改为 `termbridge [options] exec [options] -- <command...>`。
- 根级 options 作用于 `termbridge`，exec 后 options 作用于 `exec`。
- 旧入口 `termbridge [options] -- <command...>` 直接移除，不做兼容。
- Workspace 由 launch cwd / `--cwd` 稳定映射生成。
- `.termbridge/<workspace>/<session>` 中 `<workspace>` 是目录映射，`<session>` 是 Session ULID。
- workspace id 与 session id 使用 ULID。
- Session record 持久化到文件系统。
- History 先按行数限制，再按 bytes 限制，并支持配置化。
- Logs 继续放在 effective cwd 下 `logs/termbridge.log`。
- 启动或查询时读取 metadata / process info 并检查进程是否存在且匹配，避免 stale running 永久残留。
- M3 先实现 `termbridge workspace` 与 `termbridge session`。

## Spec acceptance checklist

- [x] 明确 Workspace / Session 是两个显式领域概念。
- [x] 明确 Workspace 由 launch cwd 稳定映射，且 workspace id 使用 ULID。
- [x] 明确 Session id 使用 ULID，并作为 session 目录名。
- [x] 明确 `.termbridge/<workspace>/<session>` 文件布局。
- [x] 明确 metadata、process、state、history、exit record schema。
- [x] 明确 state machine 和 transition 规则。
- [x] 明确 startup/list recovery 的 alive check 策略。
- [x] 明确 history 默认限制和裁剪顺序。
- [x] 明确 logs location 继续沿用 effective cwd 下 `logs/termbridge.log`。
- [x] 明确 CLI grammar、options scope 和旧入口错误行为。
- [x] 明确 `termbridge workspace` / `termbridge session` 输出字段。
- [x] 明确 affected components 和新增 package 边界。
- [x] 明确 M3 out-of-scope 与主要风险。

## Next stage notes

如果用户接受本 Spec 并要求进入 Plan / 计划阶段，Plan 需要重点落地：

1. CLI parser 从根级 `--` 改为 subcommand grammar 的具体步骤。
2. `just dev` 从 `go run cmd/termbridge/main.go -- {{args}}` 同步为 `go run cmd/termbridge/main.go exec -- {{args}}`。
3. ULID 依赖选择。
4. 文件原子写 helper 和 JSON schema version 处理。
5. history writer 如何接入 runner stdout relay。
6. gopty session 如何暴露 process pid / process info。
7. workspace/session list 的测试策略。
8. recovery alive check 在 Windows 下的最小可测边界。
