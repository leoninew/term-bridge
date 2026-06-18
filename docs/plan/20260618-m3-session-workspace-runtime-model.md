# M3 Session / Workspace Runtime Model 计划
最后修改时间: 2026-06-18 11:14:38

Review status: Accepted

## Context

当前流程为 strict / 严格模式，当前阶段为 Plan / 计划。

前置文档：

```text
docs/requirement/20260618-m3-session-workspace-runtime-model.md
docs/spec/20260618-m3-session-workspace-runtime-model.md
```

前置状态：

- Requirement 已 Accepted。
- Spec 已在进入 Plan 时标记为 Accepted。

M3 要在 M2 PTY Command Runner MVP 之上引入 Workspace / Session runtime model，把当前“运行一个命令”的 CLI-first 链路扩展为：

```text
termbridge [options] exec [exec options] -- <command...>
  ↓
Workspace
  ↓
Session
  ↓
PTY
  ↓
Process
```

同时新增：

- `.termbridge/<workspace>/<session>` 文件系统持久化。
- Workspace / Session metadata。
- Session state machine。
- process / exit record。
- bounded history。
- startup/list recovery。
- `termbridge workspace` / `termbridge session` 查询命令。

M3 不实现 daemon、detach / reattach、multi-attach、GUI、Gateway、SQLite、完整 terminal replay 或 M5 级别 process-tree hardening。

## Implementation strategy

推荐按“先领域模型和存储，后 CLI 切换，最后接入 exec runtime”的顺序实现，避免一开始就把 CLI、PTY、history 和 recovery 混在一起。

实施原则：

1. `internal/runner` 继续只负责 PTY lifecycle，不承载 Workspace / Session 持久化。
2. `internal/app` 作为编排层，负责把 CLI command 分发到 exec / workspace / session。
3. Workspace / Session / History / State 独立成包，避免 app 层散落文件路径和 JSON schema。
4. 旧入口 `termbridge [options] -- <command...>` 直接变为 usage error，不做兼容分支。
5. 先用单元测试锁定领域模型、状态机、storage 和 CLI parser，再做端到端 smoke。
6. `.termbridge` 是 runtime state；实现阶段需要把 `.termbridge/` 加入项目根 `.gitignore`，避免本地验证污染工作树。

## Implementation steps

### Step 1: 建立 runtime domain 基础类型

目标：先把 Workspace / Session / State / Command / Process / Exit 的 Go 类型立住，不接 PTY。

新增或调整：

```text
internal/runtime/types.go         // 如决定建 umbrella package
internal/workspace/workspace.go
internal/session/session.go
internal/session/state.go
internal/session/record.go
```

建议类型：

```go
type Workspace struct {
    ID        string
    Key       string
    Name      string
    Path      string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Session struct {
    ID           string
    WorkspaceID  string
    WorkspaceKey string
    LaunchCwd    string
    Command      CommandRecord
    State        State
    LogPath      string
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type State string

const (
    StateStarting State = "starting"
    StateRunning  State = "running"
    StateStopping State = "stopping"
    StateStopped  State = "stopped"
    StateFailed   State = "failed"
)
```

关键要求：

- State 不能使用任意字符串。
- transition 通过函数校验，例如 `CanTransition(from, to State) bool`。
- 非零用户 exit code 不进入 `failed`，而是 `stopped` + exit code。

测试：

```text
internal/session/state_test.go
```

覆盖：

- 所有允许 transition。
- 禁止 transition。
- 非法 state parse/validate。

### Step 2: 实现 ULID 生成边界

目标：明确 Workspace ID / Session ID 的生成方式。

计划采用成熟小依赖：

```text
github.com/oklog/ulid/v2
```

理由：

- ULID 格式成熟，排序属性明确。
- 避免自己实现编码和随机熵处理带来的隐藏错误。
- 依赖体量小，职责单一。

新增：

```text
internal/identity/ulid.go
internal/identity/ulid_test.go
```

建议接口：

```go
type Generator interface {
    NewID() (string, error)
}
```

实现要求：

- production 使用 crypto/rand 或 ulid 推荐熵源。
- 测试可注入 deterministic generator，避免测试依赖随机值。
- 不在业务包直接调用第三方 ULID API，统一经 `internal/identity`。

测试覆盖：

- 生成值能被 ULID parser 接受。
- deterministic generator 方便 Workspace / Session 测试。

### Step 3: 实现 Workspace key / cwd normalization

目标：由 launch cwd 稳定推导 workspace key 和默认 name。

新增：

```text
internal/workspace/key.go
internal/workspace/key_test.go
```

实现规则：

```text
workspace_key = lower(base32-no-padding(sha256(normalized_launch_cwd)))[0:26]
```

注意：Go 标准 `base32.StdEncoding` 默认输出大写并带 `=`，实现需转小写并去 padding。

Normalization 最小规则：

- `filepath.Abs`
- `filepath.Clean`
- Windows volume/path 保持稳定。
- 不解析 symlink/junction。

测试覆盖：

- 同一路径重复生成同一 key。
- 不同路径生成不同 key。
- key 只包含安全字符。
- default workspace name 来自路径最后一段。
- 根路径 fallback 使用 key 前缀。

### Step 4: 实现文件系统 state store

目标：封装 `.termbridge` 布局、JSON schema、原子写和扫描。

新增：

```text
internal/state/store.go
internal/state/layout.go
internal/state/json.go
internal/state/store_test.go
```

核心职责：

- 计算路径：
  - state root: `<effective cwd>/.termbridge`
  - workspace dir: `.termbridge/<workspace_key>`
  - session dir: `.termbridge/<workspace_key>/<session_id>`
- 原子写 JSON：临时文件 + rename。
- 读写：
  - `workspace.json`
  - `session.json`
  - `process.json`
  - `state.json`
  - `exit.json`
- 扫描 workspace/session。
- 对损坏 JSON 做 best-effort 跳过并返回 warning。

建议接口：

```go
type Store struct {
    Root string
}

func (s Store) SaveWorkspace(workspace.Workspace) error
func (s Store) LoadWorkspace(key string) (workspace.Workspace, error)
func (s Store) SaveSession(session.Session) error
func (s Store) SaveState(sessionID string, state session.StateRecord) error
func (s Store) SaveProcess(sessionID string, process process.Record) error
func (s Store) SaveExit(sessionID string, exit process.ExitRecord) error
func (s Store) ListWorkspaces() ([]workspace.Workspace, []Warning, error)
func (s Store) ListSessions() ([]session.SessionView, []Warning, error)
```

具体签名可在实现时调整，但必须保持 store 统一管理文件布局。

测试覆盖：

- 保存并读取 workspace。
- 保存并读取 session。
- 保存 state / process / exit。
- 扫描多个 workspace 下多个 session。
- JSON 文件写入后包含 `schema_version: 1`。
- 损坏 JSON 不导致整个列表失败。

### Step 5: 实现 Workspace resolver / Session manager

目标：把“给定 effective cwd，创建或复用 Workspace，并创建新的 Session”封装起来。

新增：

```text
internal/workspace/resolver.go
internal/workspace/resolver_test.go
internal/session/manager.go
internal/session/manager_test.go
```

Workspace resolver 行为：

1. 输入 launch cwd。
2. 生成 workspace key。
3. 如果 `.termbridge/<workspace_key>/workspace.json` 存在，则读取并复用 workspace id。
4. 如果不存在，则生成 workspace ULID，写入 `workspace.json`。
5. 更新 `updated_at`。

Session manager 行为：

1. 输入 Workspace、command record、log path、history config。
2. 生成 session ULID。
3. 创建 session dir。
4. 写入 `session.json`。
5. 写入 `state.json` = `starting`。

测试覆盖：

- 同 cwd 两次 resolver 复用同 Workspace ID。
- 同 Workspace 下多次创建不同 Session ID。
- session 初始状态是 `starting`。
- log path 被写入 metadata。

### Step 6: 扩展 config 支持 history / runtime 默认值

目标：让 history limits 和 state dir 有配置边界。

修改：

```text
internal/config/config.go
internal/config/config_test.go
```

新增配置：

```go
type HistoryConfig struct {
    MaxLines     int
    MaxBytes     int64
    MaxLineBytes int
}

type RuntimeConfig struct {
    StateDir string
}
```

默认值：

```text
history.max_lines = 10000
history.max_bytes = 5242880
history.max_line_bytes = 65536
runtime.state_dir = .termbridge
```

配置文件示例：

```yaml
history:
  max_lines: 10000
  max_bytes: 5242880
  max_line_bytes: 65536
runtime:
  state_dir: .termbridge
```

约束：

- `runtime.state_dir` 相对 effective cwd 解析。
- M3 不新增 CLI flag 调整 history limits。
- 配置值必须校验为正数。

测试覆盖：

- 默认配置包含 history / runtime。
- local config 能覆盖 history limits。
- 非正 history limits 返回 config error。
- unknown config key 仍然被拒绝。

### Step 7: 实现 bounded history writer

目标：把 PTY 输出写入 `history.log`，并按 line-first then bytes 限制。

新增：

```text
internal/history/writer.go
internal/history/writer_test.go
```

实现策略：

- 提供 `io.Writer` 实现，供 app 层通过 `io.MultiWriter(stdout, historyWriter)` 接入 runner。
- 内部维护最近 lines，写入后执行：
  1. `max_line_bytes` 单行限制。
  2. `max_lines` 总行数限制。
  3. `max_bytes` 总 bytes 限制。
- 每次写入后可重写 `history.log` 当前 bounded 内容；M3 数据量默认 5 MiB，可接受。若性能有风险，后续优化为 append + compaction。
- 记录 truncation metadata，供 session metadata 或 state reason 更新。

测试覆盖：

- 小输出完整保留。
- 超过 max_lines 后丢弃旧行。
- 超过 max_bytes 后继续丢弃旧行。
- 单行超过 max_line_bytes 被截断。
- partial line 不丢失。
- writer error 不破坏 stdout 写入的路径在 app 测试中覆盖。

### Step 8: 扩展 process record / PTY process info

目标：让 Session 能写入 `process.json`。

修改：

```text
internal/process/spec.go
internal/process/exit.go 或新增 internal/process/record.go
internal/pty/pty.go
internal/pty/gopty/manager.go
internal/pty/gopty/manager_test.go
```

新增记录：

```go
type Record struct {
    PID         int
    OwnerPID    int
    Executable  string
    CommandLine string
    Cwd         string
    StartedAt   time.Time
}
```

推荐接口：

```go
type ProcessReporter interface {
    ProcessInfo() process.Record
}
```

避免强制修改所有 fake session。app/runner 可在拿到 session 后通过 type assertion 读取 process info。

但当前 `runner.CommandRunner.Run` 内部不暴露 session。为让 app 能写 process record，有两种实现路径：

#### 推荐路径：扩展 runner.Result

在 runner 内部启动 session 后读取 process info，并返回：

```go
type Result struct {
    ExitCode int
    Exit     process.ExitResult
    Process  process.Record
}
```

限制：process record 在 runner 返回时才到 app，无法在 running 期间立刻写 `process.json`。

#### 更符合 Spec 的路径：增加 lifecycle hooks

给 runner 增加可选 hooks：

```go
type Hooks struct {
    OnStarted func(process.Record)
    OnStopping func(reason string)
}
```

`CommandRunner` 启动 session 后立即调用 `OnStarted`，app 在 hook 中写 `process.json` 和 state=`running`。

Plan 采用 hooks 路径，因为 M3 要在 lifecycle 中持续维护 state。

测试覆盖：

- fake session 实现 `ProcessInfo()` 后，runner 调用 `OnStarted`。
- gopty session 返回非零 pid。
- process record 包含 cwd、command line、owner pid。

### Step 9: 扩展 runner hooks 但不引入 Workspace 依赖

目标：让 app 能知道 started / stopping / exit 事件，同时 runner 不依赖 session/workspace 包。

修改：

```text
internal/runner/runner.go
internal/runner/runner_test.go
internal/runner/relay_test.go 如需要
```

建议结构：

```go
type Hooks struct {
    OnStarted  func(process.Record)
    OnStopping func(process.StopMode, string)
}

type CommandRunner struct {
    Manager        termpty.Manager
    Logger         Logger
    InterruptGrace time.Duration
    Hooks          Hooks
}
```

事件：

- `OnStarted`：PTY/process start 成功后触发。
- `OnStopping`：收到 interrupt、context cancel、close escalation、kill escalation 时触发。

约束：

- hook error 不应直接破坏 runner 主路径；如果需要处理错误，app 可在 hook 内记录并 logger error。
- runner 不 import `internal/session` 或 `internal/state`。

测试覆盖：

- 启动成功触发 OnStarted。
- Ctrl+C / context cancellation 触发 OnStopping。
- 用户正常退出不触发 stopping。
- 现有 exit code passthrough 测试仍通过。

### Step 10: 重构 CLI parser 为 subcommand grammar

目标：切换 CLI contract。

修改：

```text
internal/cli/cli.go
internal/cli/cli_test.go
```

新 parser 行为：

- `termbridge --help`：root help。
- `termbridge --version`：version。
- `termbridge exec -- pwsh`：exec command。
- `termbridge --cwd D:\project exec -- pwsh`：root cwd + exec command。
- `termbridge exec --help`：exec help。
- `termbridge workspace`：workspace command。
- `termbridge session`：session command。
- `termbridge -- pwsh`：usage error，提示使用 `termbridge exec -- pwsh`。

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
    Kind        CommandKind
    Exec        ExecOptions
    ShowHelp    bool
    ShowVersion bool
}
```

测试覆盖：

- root help 文案。
- exec help 文案。
- parse exec command。
- parse root cwd + exec。
- parse workspace / session。
- reject missing subcommand。
- reject old grammar。
- reject exec missing `--`。
- reject exec missing command after `--`。
- reject unknown root option。
- reject unknown command。

### Step 11: 重构 app command dispatch

目标：让 app 编排 exec / workspace / session。

修改：

```text
internal/app/app.go
internal/app/app_test.go
```

新 app 行为：

#### exec

1. `config.Load` 解析 effective cwd / log config / history config / state dir。
2. Workspace resolver 创建或复用 workspace。
3. Session manager 创建 session，写 `session.json` 和 `state=starting`。
4. 创建 history writer。
5. 构造 `ProcessSpec`。
6. 构造 runner，注入 hooks：
   - `OnStarted`: 写 `process.json`，state=`running`。
   - `OnStopping`: state=`stopping`。
7. 用 `io.MultiWriter(stdout, historyWriter)` 接入 runner。
8. runner 返回后写 `exit.json`，state=`stopped`。
9. runner 返回 runtime error 时写 state=`failed`。
10. 返回用户 command exit code。

#### workspace

1. `config.Load` 解析 effective cwd / state dir。
2. store 扫描 workspaces。
3. 输出文本表格。

#### session

1. `config.Load` 解析 effective cwd / state dir。
2. store 扫描 sessions。
3. 对非终态 session 执行 recovery alive check。
4. 输出文本表格。

测试覆盖：

- exec 创建 `.termbridge/<workspace_key>/<session_id>`。
- exec 写入 workspace/session/state/exit/history。
- 用户 exit code 7 仍返回 7。
- workspace 命令列出 workspace。
- session 命令列出 session。
- runtime start error 写 failed state。
- history writer 接收到 stdout。

### Step 12: 实现 recovery / alive checker

目标：查询时避免 stale running 永久显示 running。

新增：

```text
internal/session/recovery.go
internal/session/recovery_windows.go
internal/session/recovery_other.go
internal/session/recovery_test.go
```

M3 最小实现：

- 通用：检查 pid 是否存在。
- Windows：使用 `os.FindProcess` 只能返回 handle，不代表进程存在；需要用 `golang.org/x/sys/windows` 或 `tasklist` 不可取。优先使用 Windows API 查询 process exit code 或打开进程。
- 非 Windows：可用 signal 0 或平台实现。

由于项目当前主要目标是 Windows，Windows 实现优先。

规则：

- state in `stopped` / `failed`：不改。
- state in `starting` / `running` / `stopping`：检查 process。
- pid missing：
  - 有 exit record：state=`stopped`, reason=`recovered_exit_record`。
  - 无 exit record：state=`failed`, reason=`stale_process_missing`。
- pid exists 但无法匹配：state=`failed`, reason=`stale_process_unverified`。
- pid exists 且匹配：保持 `running`。

测试策略：

- 单元测试使用 fake alive checker，不依赖真实 OS。
- Windows integration 可选：启动短进程/长进程验证 pid exists/missing。

### Step 13: 更新 justfile 与项目忽略规则

目标：同步 CLI contract，避免 `.termbridge` 污染。

修改：

```text
justfile
.gitignore
```

`just dev` 从：

```just
dev *args:
    go run cmd/termbridge/main.go -- {{args}}
```

改为：

```just
dev *args:
    go run cmd/termbridge/main.go exec -- {{args}}
```

`.gitignore` 增加：

```text
.termbridge/
```

测试：

```powershell
just --dry-run dev pwsh -NoLogo
```

期望：

```text
go run cmd/termbridge/main.go exec -- pwsh -NoLogo
```

### Step 14: 更新文档和旧测试期望

目标：同步 M3 CLI contract，不留下 M2 旧入口断言。

可能修改：

```text
README.md                 // 若存在 CLI usage
internal/cli/cli_test.go
internal/app/app_test.go
docs/verification/...     // 不修改历史 verification，只在 M3 verification 中记录变化
```

注意：历史 M2 文档可作为历史记录保留，不主动迁移。只有当前 help、测试和开发入口需要更新。

## Critical files to change

### Existing files

```text
.gitignore
justfile
go.mod
go.sum
internal/cli/cli.go
internal/cli/cli_test.go
internal/app/app.go
internal/app/app_test.go
internal/config/config.go
internal/config/config_test.go
internal/process/spec.go
internal/process/exit.go
internal/pty/pty.go
internal/pty/gopty/manager.go
internal/pty/gopty/manager_test.go
internal/runner/runner.go
internal/runner/runner_test.go
```

### New files / packages

```text
internal/identity/ulid.go
internal/identity/ulid_test.go
internal/workspace/workspace.go
internal/workspace/key.go
internal/workspace/resolver.go
internal/workspace/*_test.go
internal/session/session.go
internal/session/state.go
internal/session/manager.go
internal/session/recovery.go
internal/session/recovery_windows.go
internal/session/recovery_other.go
internal/session/*_test.go
internal/history/writer.go
internal/history/writer_test.go
internal/state/store.go
internal/state/layout.go
internal/state/json.go
internal/state/store_test.go
```

### Stage documents

```text
docs/plan/20260618-m3-session-workspace-runtime-model.md
docs/verification/20260618-m3-session-workspace-runtime-model.md  // Verification 阶段创建
```

## Verification plan

Implementation 完成后默认停在 Implementation / 实现阶段。只有用户明确进入 Verification / 验证时，才创建 verification 文档并执行完整验收记录。

实现阶段内部可运行必要测试以确保改动正确；Verification 阶段再正式记录。

### Automated checks

优先运行：

```powershell
go test ./...
```

再运行项目约定检查：

```powershell
just check
```

如需要单独定位：

```powershell
go test ./internal/cli
go test ./internal/app
go test ./internal/workspace
go test ./internal/session
go test ./internal/history
go test ./internal/state
go test ./internal/runner
go test ./internal/pty/gopty
```

### CLI parser smoke

```powershell
go run cmd/termbridge/main.go --help
go run cmd/termbridge/main.go exec --help
go run cmd/termbridge/main.go workspace
go run cmd/termbridge/main.go session
```

期望：

- root help 显示 subcommands。
- exec help 显示 `exec -- <command>`。
- workspace/session 在无 `.termbridge` 时返回成功并显示空列表或明确 no records。

### Old grammar rejection smoke

```powershell
go run cmd/termbridge/main.go -- pwsh
```

期望：

- 返回 usage exit。
- stderr 提示旧入口不再支持，并建议 `termbridge exec -- pwsh`。

### Exec smoke

```powershell
go run cmd/termbridge/main.go --cwd <temp-dir> exec -- pwsh -NoLogo -NoProfile -Command "Write-Output TERM_BRIDGE_M3_OK"
```

期望：

- stdout 包含 `TERM_BRIDGE_M3_OK`。
- `<temp-dir>/logs/termbridge.log` 存在。
- `<temp-dir>/.termbridge/<workspace_key>/<session_id>/session.json` 存在。
- `history.log` 包含 marker。
- `state.json` 最终为 `stopped`。
- `exit.json` exit code 为 0。

### Exit code passthrough smoke

使用 built binary 更适合验证真实 exit code passthrough：

```powershell
just build
.\bin\termbridge.exe --cwd <temp-dir> exec -- cmd.exe /C exit /b 7
```

期望：

- 进程 exit code 为 7。
- session state 为 `stopped`。
- `exit.json` exit_code 为 7。
- stderr 不把用户非零 exit code 报成 TermBridge runtime error。

### Workspace/session list smoke

执行一次或多次 exec 后：

```powershell
go run cmd/termbridge/main.go --cwd <temp-dir> workspace
go run cmd/termbridge/main.go --cwd <temp-dir> session
```

期望：

- workspace 输出包含 workspace id、key、name、path、session count。
- session 输出包含 session id、workspace id、state、exit、command、cwd。

### Recovery smoke

M3 最小 recovery 可通过构造 state 文件测试，不强行制造 owner-crash。

建议自动化单元测试：

- 用 fake alive checker 构造 `running + missing pid`。
- 运行 recovery。
- 断言 state 变为 `failed` + reason=`stale_process_missing`。

可选人工验证：

- 启动长时间命令。
- 人工终止 TermBridge 或构造 stale record。
- 再运行 `termbridge session`。
- 确认不会永久显示 stale running。

## Rollback plan

M3 涉及 CLI contract、持久化和 runner hooks，rollback 应按边界拆分：

1. 如果 CLI parser 切换失败：
   - 暂时只回退 `internal/cli` / `internal/app` dispatch。
   - 保留 Workspace / Session / State / History 包的单测，不接入入口。
2. 如果 storage schema 不稳定：
   - 保留 domain types。
   - 暂停 app 写 `.termbridge`，先修 store 单测。
3. 如果 history writer 影响 stdout：
   - 在 app 层临时禁用 history writer，保持 exec 主路径。
   - 不把 history failure 变成用户 command failure。
4. 如果 runner hooks 引入回归：
   - 回退 hooks 到 no-op 或只返回 process info。
   - 确保原有 exit code passthrough 和 Ctrl+C 测试先恢复。
5. 如果 gopty process info 无法稳定取得：
   - process record 先记录 command/cwd/owner pid 和 `pid=0` 风险不满足最终验收；必须在 Verification 前补齐或明确 incomplete，不能假装完成。
6. 如果 ULID 依赖引入问题：
   - 封装在 `internal/identity`，可替换依赖而不影响 Workspace / Session 包。

## Assumptions

1. M3 允许新增一个小型 ULID 依赖 `github.com/oklog/ulid/v2`。
2. M3 的 `.termbridge` 位于 effective cwd 下，不实现全局用户级 registry。
3. M3 查询命令没有 `--json`，输出为稳定文本表格。
4. M3 history 是 bounded recent output，不是完整 terminal replay。
5. M3 recovery 只做状态识别，不 reattach，不接管旧 PTY。
6. M3 主要验证环境是 Windows；非 Windows 文件和测试应尽量可编译，但 Windows process alive check 可先平台化。
7. `just dev` 必须随 CLI contract 更新为 `exec --`。

## Risks

### 风险一：实现规模较大，容易一次性改坏主路径

缓解：按包分层提交实现顺序，先 domain/store/history 单测，再接入 app/CLI；每一步运行局部测试。

### 风险二：runner hooks 与 Ctrl+C lifecycle 交叉复杂

缓解：hooks 只传递事件，不承担控制流；runner 不 import session/state；现有 runner 测试必须保留并扩展。

### 风险三：history writer 同步落盘可能影响 TUI 输出性能

缓解：M3 默认 5 MiB 上限可接受；实现中若发现明显阻塞，应先保证 stdout，不让 history failure 破坏主路径。

### 风险四：Windows process alive check 不可靠

缓解：用 fake checker 单测 recovery 决策；Windows API 实现只做 best-effort；无法确认时不能显示 running。

### 风险五：文件系统 state 可能残留脏数据影响测试

缓解：测试全部使用 `t.TempDir()` 和隔离 HOME / USERPROFILE；`.gitignore` 加 `.termbridge/`。

### 风险六：CLI 旧入口移除会导致已有 M2 smoke 命令失效

缓解：同步更新测试、`just dev` 和 M3 verification 命令；历史文档不改，但新验证记录说明 contract 已变更。

## Blockers

当前没有必须阻塞 Plan 形成的问题。

需要在 Implementation 阶段做出的技术确认：

1. `github.com/oklog/ulid/v2` 是否能顺利加入 go.mod。
2. `github.com/aymanbagabas/go-pty` 的 `Cmd` / `Process` 是否足够暴露 pid；若不足，需要在 adapter 内保存 `cmd.Process.Pid`。
3. Windows alive check 使用 `golang.org/x/sys/windows` 的具体 API 是否已由现有依赖满足。

## User review notes

用户要求进入 Plan / 计划阶段，因此 Spec 已按 SpecFlow 规则标记为 Accepted，并基于已接受的 Requirement 与 Spec 创建本 Plan。

本 Plan 沿用用户确认的关键边界：

- Workspace 与 Session 显式区分。
- Workspace 由 launch cwd / `--cwd` 稳定映射。
- session record 持久化到 `.termbridge/<workspace>/<session>`。
- logs 继续在 effective cwd 下 `logs/termbridge.log`。
- 命令入口改为 `termbridge [options] exec [options] -- <command...>`。
- 旧入口直接移除，不做兼容。
- 先实现 `termbridge workspace` 和 `termbridge session`。
- 不进入 daemon、GUI、Gateway、detach / reattach 或 M5 hardening。

## Plan acceptance checklist

- [x] 列出 implementation steps。
- [x] 列出 critical files to change。
- [x] 列出新增 packages / files。
- [x] 明确 ULID 依赖策略。
- [x] 明确 `.gitignore` / `just dev` 同步更新。
- [x] 明确 runner hooks 与 app 编排边界。
- [x] 明确 storage / history / recovery 测试策略。
- [x] 明确 verification plan。
- [x] 明确 rollback plan。
- [x] 明确 assumptions、risks、blockers。

## Next stage notes

如果用户接受本 Plan 并要求开始 Implementation / 实现，应按以下顺序推进：

1. Domain/state/identity/workspace key。
2. Store。
3. Config history/runtime defaults。
4. History writer。
5. Process info + runner hooks。
6. CLI parser。
7. App dispatch and exec/session integration。
8. Workspace/session list and recovery。
9. justfile / .gitignore / tests。

实现完成后默认停在 Implementation / 实现阶段，汇报实际改动、已运行检查、风险和偏离范围；不自动进入 Verification，除非用户明确要求。
