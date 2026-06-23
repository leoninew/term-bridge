# 停止态会话重新运行计划
最后修改时间: 2026-06-23 18:44:58

Review status: Accepted

## Requirement / Spec basis

基于：

- `docs/requirement/20260623-session-rerun.md`，Review status: Draft
- `docs/spec/20260623-session-rerun.md`，Review status: Draft

当前流程：严格模式 / strict，计划 / Plan。

本版 Plan 已纳入新增要求：`workspace.json` 内部已有 session 大部分信息，应合并收拾 `process.json` / `session.json` / `state.json` / `exit.json`，重新组织结构并在 Plan 阶段 review；不继续维持碎片化当前事实存储。

本版同时纳入最新确认：不向后兼容，不迁移已有文件，`exit.json` 也要收拾。

核心约束：

1. stopped / failed session 支持“重新运行”。
2. 重新运行复用同一个 session / `session.id`，不创建新 session record。
3. 重新运行复用原 session 的名称、工作目录和命令。
4. `workspace.json` 是 workspace + sessions 当前事实的聚合存储。
5. 当前 session metadata、state、process、exit、history 配置集中保存到 `workspace.json` 对应 session node。
6. `session.json` / `state.json` / `process.json` / `exit.json` 不再作为当前 session 事实来源。
7. 不读取旧碎片文件，不迁移旧碎片文件，不提供 fallback，不做双写。
8. `history.log` 仍作为大体积 terminal 输出单独保存。
9. 重新运行前只重命名旧 `history.log`；旧 run 的小型 metadata 收进 `workspace.json.archived_runs`，不再制造 `process.<archive_id>.json` / `exit.<archive_id>.json` / `state.<archive_id>.json`。
10. 归档是 runtime / registry 本地文件与 `workspace.json` 整理，不新增归档接口，不让云端 / gateway / agent API 感知。
11. 状态集合只有 `running` / `stopped` / `failed`，不进行 `starting` / `stopping` 历史兼容。
12. 状态机扩展终态到 running：`stopped -> running`、`failed -> running`。
13. create failure 固定 failed，并返回明确状态供前端组织 UI。
14. rerun failure 固定 failed。
15. 启动、关闭、删除 session 都是同步操作。
16. 新 live terminal 不回放旧输出。
17. 文案 B 当前按 `重新运行` 实施。

## Target state model

目标用户可见 / 持久化 lifecycle state：

- `running`：有 live PTY runtime，可 attach、输入、停止。
- `stopped`：进程正常结束或用户同步停止后的终态，可查看当前 history、删除、重新运行。
- `failed`：创建、运行、重新运行或等待过程失败后的终态，可查看当前 history / 错误、删除、重新运行。

不保留：

- `starting`
- `stopping`

目标状态机：

```text
create ok ------------------------------> running
create failure -------------------------> failed
running -- natural exit ok / close ok --> stopped
running -- runtime/wait failure --------> failed
stopped -- rerun ok --------------------> running
failed  -- rerun ok --------------------> running
stopped -- rerun failure ---------------> failed
failed  -- rerun failure ---------------> failed
stopped -- delete ----------------------> deleted
failed  -- delete ----------------------> deleted
```

`deleted` 不是 lifecycle state，只表示记录被同步删除。

## Target storage model

### 文件布局

目标布局：

```text
.termbridge/
  workspaces/
    <workspace_id>/
      workspace.json
      sessions/
        <session_id>/
          history.log
          history.<archive_id>.log
```

不再作为当前事实来源：

```text
sessions/<session_id>/session.json
sessions/<session_id>/state.json
sessions/<session_id>/process.json
sessions/<session_id>/exit.json
```

`exit.json` 与 process/state 一样属于当前 run 的小型结构化 metadata；本轮一起收进 `workspace.json`，避免留下新的碎片事实源。

### schema boundary

计划将 workspace 存储 schema 提升到新版本，例如：

```go
const SchemaVersion = 2
```

原则：

1. 新写入的 `workspace.json` 使用新 schema。
2. 新 schema 的 session node 必须包含当前 state/history/current run 结构。
3. 旧 schema 或缺少必需 session 字段的数据不通过读取旧碎片文件修复。
4. 不写 schema migration。
5. 不在运行时把旧 `session.json` / `state.json` / `process.json` / `exit.json` 折叠进 `workspace.json`。

### workspace.json session node 结构

在 `internal/domain/workspace/workspace.go` 中扩展 `SessionNode`，让它承载当前 session 事实。

建议结构方向：

```go
type SessionNode struct {
    Id           string        `json:"session_id"`
    Name         string        `json:"name"`
    LaunchCwd    string        `json:"launch_cwd"`
    Command      CommandRecord `json:"command"`
    History      HistoryRecord `json:"history"`
    State        StateRecord   `json:"state"`
    CurrentRun   RunRecord     `json:"current_run"`
    ArchivedRuns []ArchivedRun `json:"archived_runs,omitempty"`
    CreatedAt    time.Time     `json:"created_at"`
    UpdatedAt    time.Time     `json:"updated_at"`
}
```

其中：

```go
type HistoryRecord struct {
    Path         string `json:"path"`
    MaxLines     int    `json:"max_lines"`
    MaxBytes     int64  `json:"max_bytes"`
    MaxLineBytes int    `json:"max_line_bytes"`
    Truncated    bool   `json:"truncated"`
}

type StateRecord struct {
    SchemaVersion int       `json:"schema_version"`
    State         string    `json:"state"`
    Reason        string    `json:"reason"`
    UpdatedAt     time.Time `json:"updated_at"`
}

type ProcessRecord struct {
    SchemaVersion int       `json:"schema_version"`
    Pid           int       `json:"pid"`
    OwnerPid      int       `json:"owner_pid"`
    Executable    string    `json:"executable"`
    CommandLine   string    `json:"command_line"`
    Cwd           string    `json:"cwd"`
    StartedAt     time.Time `json:"started_at"`
}

type ExitRecord struct {
    SchemaVersion int       `json:"schema_version"`
    ExitCode      int       `json:"exit_code"`
    Reason        string    `json:"reason"`
    Forced        bool      `json:"forced"`
    Closed        bool      `json:"closed"`
    StartedAt     time.Time `json:"started_at"`
    EndedAt       time.Time `json:"ended_at"`
    WaitError     string    `json:"wait_error"`
}

type RunRecord struct {
    Process *ProcessRecord `json:"process,omitempty"`
    Exit    *ExitRecord    `json:"exit,omitempty"`
}

type ArchivedRun struct {
    ArchiveId   string         `json:"archive_id"`
    HistoryPath string         `json:"history_path"`
    State       StateRecord    `json:"state"`
    Process     *ProcessRecord `json:"process,omitempty"`
    Exit        *ExitRecord    `json:"exit,omitempty"`
    ArchivedAt  time.Time      `json:"archived_at"`
}
```

注意：

1. `workspace` package 不直接 import `session` package，避免 `session -> workspace -> session` import cycle。
2. repository 层负责 `workspace.StateRecord` / `workspace.ProcessRecord` / `workspace.ExitRecord` 与 `session.StateRecord` / `process.Record` / `process.ExitRecord` 互转。
3. 不为了这次重组引入泛化持久化框架；集中在 workspace session node 的显式字段和显式 mapping helper。

### legacy file handling

明确不做 legacy handling：

1. 不读取旧 `session.json`。
2. 不读取旧 `state.json`。
3. 不读取旧 `process.json`。
4. 不读取旧 `exit.json`。
5. 不从旧碎片文件迁移任何字段。
6. 不提供旧文件 fallback。
7. 不为旧文件写清理逻辑。
8. 新代码只停止创建这些碎片文件。

旧文件如果仍在磁盘上，只是未使用遗留文件；本功能不主动删除它们，避免把“无迁移”变成隐式数据清理任务。

## Implementation steps

### 1. 先重组 workspace session storage model

文件：

- `internal/domain/workspace/workspace.go`
- `internal/infrastructure/repository/state/store.go`
- `internal/infrastructure/repository/state/store_test.go`

实施：

1. 将 `workspace.SchemaVersion` 提升到新版本。
2. 扩展 `workspace.SessionNode`，增加：
   - `History`
   - `State`
   - `CurrentRun`
   - `ArchivedRuns`
3. 在 `workspace` package 内定义 workspace-local storage DTO，避免 import cycle。
4. 在 `state.Store` 内补 mapping helper：
   - `sessionNodeFromSession`
   - `sessionFromWorkspaceNode`
   - `workspaceStateFromSessionState`
   - `sessionStateFromWorkspaceState`
   - `workspaceProcessFromProcess`
   - `processFromWorkspaceProcess`
   - `workspaceExitFromProcessExit`
   - `processExitFromWorkspaceExit`
5. `SaveSession` 只 upsert `workspace.json` children，不再写 `session.json`。
6. `LoadSession` 只从 `workspace.json` children 构造 `session.Session`。
7. `SaveState` / `LoadState` 改为读写 `workspace.json` session node 的 `State`。
8. `SaveProcess` / `LoadProcess` 改为读写 `workspace.json` session node 的 `CurrentRun.Process`。
9. `SaveExit` / `LoadExit` 改为读写 `workspace.json` session node 的 `CurrentRun.Exit`。
10. `ListSessionsByWorkspaceId` / `ListSessions` 不再逐 session 读取 `state.json` / `exit.json`。
11. 旧 schema 或缺少必需 state 的 session node 按无效当前数据处理；不读取旧碎片文件补齐。
12. `DeleteSession` 仍删除 session node，并删除 `sessions/<session_id>` 目录下的 history / archive history 文件。
13. `HistoryPath` 保持返回 `sessions/<session_id>/history.log`。
14. 所有 workspace 写入继续使用临时文件 + replace，不能半写坏 `workspace.json`。

测试：

- SaveSession 不创建 `session.json`。
- SaveState 不创建 `state.json`，并能从 workspace node LoadState。
- SaveProcess 不创建 `process.json`，并能从 workspace node LoadProcess。
- SaveExit 不创建 `exit.json`，并能从 workspace node LoadExit。
- ListSessions 不依赖 session dir 下碎片文件。
- 已存在的旧 `state.json` / `process.json` / `exit.json` 不会被读取或迁移。
- DeleteSession 删除 workspace children 和 session history 目录。

### 2. 移除 legacy fragment 依赖，不实现迁移

文件：

- `internal/infrastructure/repository/state/store.go`
- `internal/infrastructure/repository/state/store_test.go`
- 相关调用方测试

实施：

1. 删除或停止使用所有面向以下路径的读写：
   - `session.json`
   - `state.json`
   - `process.json`
   - `exit.json`
2. 不增加 `foldLegacySessionFragments` / migration helper。
3. 不增加 fallback 读取逻辑。
4. 不增加旧文件删除逻辑。
5. 旧文件存在时，新代码行为不受影响；事实来源仍只看 `workspace.json`。
6. 测试显式断言：即使 session dir 下存在旧碎片文件，`LoadState` / `LoadProcess` / `LoadExit` 也不会从这些文件取值。

### 3. 收敛 session 状态集合和状态机

文件：`internal/domain/session/state.go`

实施：

1. 移除 `StateStarting`、`StateStopping` 作为合法状态。
2. `Valid()` 只接受：
   - `StateRunning`
   - `StateStopped`
   - `StateFailed`
3. `CanTransition` 只允许：
   - `running -> stopped`
   - `running -> failed`
   - `stopped -> running`
   - `stopped -> failed`
   - `failed -> running`
   - `failed -> stopped`
4. `Terminal(StateStopped/StateFailed)` 保持不变。
5. 搜索并移除所有新旧代码对 starting/stopping 的依赖，包括 `internal/app/app.go`。

测试：

- 覆盖目标转换允许 / 不允许。
- starting/stopping 不再 valid。

### 4. 调整 create failure 为明确 failed 状态

文件：

- `internal/domain/session/manager.go`
- `internal/application/terminal/registry.go`
- `internal/transport/http/gatewayapi/server.go`（如响应 shape 需要调整）
- `web/src/features/sessions/api.ts` / `web/src/views/SessionsView.vue`（如前端错误处理需要拉取 failed summary）

实施：

1. 创建 session record 后不写 starting。
2. 调整 manager / registry 边界：create 流程生成 session id 和 session metadata 后，同步启动 runtime，再以最终结果写入 `workspace.json` session node。
3. 成功：保存 running state 和 process record 到 `workspace.json`，返回 `CreateSessionResponse{state: running}`。
4. 失败：保存 failed state 到 `workspace.json`，reason 使用明确错误原因，返回让前端能组织 UI 的错误 / 状态。
5. 如果 create 失败时已经有 session id，前端必须能拿到该 failed session：
   - 优先方案：扩展错误响应或 create response，包含 `session_id` 和 `state: failed`。
   - 备选方案：create 返回错误后立即 refresh workspace tree，后端必须能列出 failed session。
6. 不允许 create failure 出现“没有状态、前端无法组织 UI”的空洞。

测试：

- create 成功：running。
- history writer / spec / executable / PTY 启动失败：failed 且可查询。
- create 失败响应或刷新路径能让前端识别 failed。
- 不出现 starting。
- create failure 后不产生 `session.json` / `state.json` / `process.json` / `exit.json`。

### 5. 取消 close 路径写入 stopping

文件：

- `internal/application/terminal/runtime.go`
- `internal/application/terminal/registry.go`
- `internal/app/app.go`
- `web/src/views/SessionsView.vue`
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`

实施：

1. `closeSession` 继续同步关闭 PTY 并等待 `done`。
2. 移除 `SaveState(... StateStopping ...)`。
3. 移除或调整 WebSocket `stopping` lifecycle message。
4. `waitLoop` 最终保存 stopped / failed 到 `workspace.json`。
5. close API 返回时，`GetSession` 应看到 stopped 或 failed。
6. 前端 stop 按钮只对 running 展示。

测试：

- close running session 后返回 stopped。
- close 失败不留下 stopping。
- 前端状态判断不依赖 stopping。

### 6. 在 rerun 路径归档当前 run

文件：

- `internal/application/terminal/registry.go`
- `internal/infrastructure/repository/state/store.go`

实施：

1. 不新增 `ArchiveSessionRunFiles` 这类 store 抽象接口。
2. 在 `Registry.RerunSession` 或私有 helper 中直接基于 session dir/path 对 `history.log` 执行 `os.Rename`。
3. 归档文件：
   - `history.log` -> `history.<archive_id>.log`
4. `archive_id`：
   - UTC 时间戳，例如 `20260623T083318Z`。
   - 冲突时追加序号或更高精度。
5. 不存在 `history.log` no-op。
6. 旧 run 的 state/process/exit metadata 追加到 `workspace.json` session node 的 `ArchivedRuns`。
7. 清空或重置 `CurrentRun`，为新 run 写入新的 process/exit。
8. 不归档 `workspace.json`。
9. 不新增归档 list/read API。
10. 不通过 gateway / agent 通知云端。

测试：

- `history.log` 存在时重命名为 `history.<archive_id>.log`。
- `history.log` 不存在时不报错。
- 冲突命名不覆盖。
- 旧 state/process/exit metadata 进入 `ArchivedRuns`。
- 不生成 `process.<archive_id>.json` / `exit.<archive_id>.json` / `state.<archive_id>.json`。

### 7. 抽取同步启动 runtime helper

文件：`internal/application/terminal/registry.go`

建议 helper：

```go
func (r *Registry) startSessionRuntime(
    ctx context.Context,
    sess session.Session,
    command []string,
    size process.TerminalSize,
    failureReasonPrefix string,
) error
```

职责：

1. 创建 history writer。
2. 构建 process spec。
3. 注入过滤后的 env。
4. resolve executable。
5. start PTY。
6. 保存 process record 到 `workspace.json` current run。
7. 保存 running state 到 `workspace.json` session node。
8. 注册 runtime。
9. 启动 runtime goroutines。

失败处理：

- 不留下 runtime。
- 已打开的 history writer / PTY 必须关闭。
- 不写 starting。
- 返回错误给 create/rerun 调用方，由调用方保存 failed。

### 8. Registry 增加 RerunSession

文件：`internal/application/terminal/registry.go`

新增请求类型：

```go
type RerunSessionRequest struct {
    Cols int `json:"cols"`
    Rows int `json:"rows"`
}
```

新增方法：

```go
func (r *Registry) RerunSession(ctx context.Context, sessionId string, request RerunSessionRequest) (CreateSessionResponse, error)
```

行为：

1. 查找 session view。
2. 只允许 stopped / failed。
3. 拒绝已有 live runtime。
4. 从 `workspace.json` session node 读取原始命令：`Command + Args`。
5. 验证 cwd 仍存在且为目录。
6. 校验 terminal size。
7. 本地重命名旧 `history.log`，并把旧 run metadata 写入 `ArchivedRuns`。
8. 调用同步启动 helper。
9. 启动成功返回 `CreateSessionResponse{SessionId: 原 id, State: running}`。
10. 启动失败固定保存 failed state，reason 使用可诊断值。

失败规则：

- 校验失败发生在 history 归档前：保存 / 返回 failed 状态，不重命名旧 history。
- history 重命名失败：不启动 PTY，保存 / 返回 failed 状态。
- history 重命名后启动失败：不回滚归档，保存 / 返回 failed 状态。
- 不允许任何失败路径留下 running runtime 或 running state。

测试：

- stopped rerun 成功复用 id。
- failed rerun 成功复用 id。
- running rerun 被拒绝且状态明确。
- rerun 成功前重命名旧 history，成功后新 history 不含旧输出。
- rerun 失败固定 failed。
- rerun 使用原始 command args，不用展示字符串反解析。

### 9. Agent runtime 接口和 tunnel method

文件：

- `internal/application/agent/runtime_access.go`
- `internal/application/agent/client.go`

新增：

```go
RerunSession(ctx context.Context, sessionId string, request terminalapp.RerunSessionRequest) (terminalapp.CreateSessionResponse, error)
```

Agent method：

- `rerun_session`

不新增归档相关 method。

测试：

- method decode / dispatch 正确。

### 10. Gateway API route

文件：`internal/transport/http/gatewayapi/server.go`

新增：

```http
POST /api/devices/:deviceId/sessions/:sessionId/rerun
```

实施：

1. rerun：在线 relay，不使用缓存，成功 `200 OK`。
2. 不新增归档相关 route。

测试：

- route method/path 正确。
- rerun offline 不走缓存。

### 11. 前端协议和 API

文件：

- `web/src/protocol/terminal.ts`
- `web/src/features/sessions/api.ts`

实施：

1. `LifecycleState` 收敛为：

```ts
export type LifecycleState = 'running' | 'stopped' | 'failed'
```

2. 新增：

```ts
export type RerunSessionRequest = {
  cols: number
  rows: number
}
```

3. 新增 API：

```ts
rerunSession(...)
```

不新增 archive API。

### 12. 前端状态判断收敛

文件：

- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
- `web/src/views/SessionsView.vue`

实施：

1. stop 按钮仅对 `running` 展示。
2. rerun 按钮仅对 `stopped` / `failed` 展示。
3. delete 按钮仅对 `stopped` / `failed` 展示。
4. active/live 判断仅依赖 `running`。
5. 移除 starting/stopping 判断。

### 13. Sidebar 增加“重新运行”入口

文件：`web/src/components/workspace/WorkspaceSessionSidebar.vue`

实施：

1. 引入 icon：建议 `RotateCcw`。
2. 新增 emit：`rerunSession: [session: SessionSummary]`。
3. stopped / failed session 显示“重新运行”按钮。
4. 文案 B：`sidebar.rerunSessionAria`，中文 `重新运行 {name} 会话`。

### 14. SessionsView 接入 rerun

文件：`web/src/views/SessionsView.vue`

handler：

1. `ensureMutationAllowed()`。
2. 只允许 stopped / failed。
3. `measureInitialTerminalSize()`。
4. 调 `rerunSession(selectedDeviceId.value, session.id, { cols, rows })`。
5. `getSession()` 拉取同 id 最新 summary。
6. `updateSessionInState(updated)`。
7. `openSessionTab(updated)`。
8. success toast：`会话已重新运行`。
9. failure：notify error，并 refresh / getSession 以拿到 failed 状态。
10. 由于 id 不变，已有 opened tab 不新增重复项，只切换并展示 live terminal。

### 15. i18n 文案

文件：`web/src/i18n.ts`

中文：

- `sidebar.rerunSessionAria`: `重新运行 {name} 会话`
- `toast.rerunSessionFailed`: `重新运行会话失败`
- `toast.sessionRerun`: `会话已重新运行`

英文：

- `sidebar.rerunSessionAria`: `Rerun {name} session`
- `toast.rerunSessionFailed`: `Rerun session failed`
- `toast.sessionRerun`: `Session rerun`

## Files to change

预计修改：

1. `internal/domain/workspace/workspace.go`
2. `internal/domain/session/state.go`
3. `internal/domain/session/session.go`
4. `internal/domain/session/manager.go`
5. `internal/domain/session/recovery.go`
6. `internal/infrastructure/repository/state/store.go`
7. `internal/application/terminal/registry.go`
8. `internal/application/terminal/runtime.go`
9. `internal/application/agent/runtime_access.go`
10. `internal/application/agent/client.go`
11. `internal/transport/http/gatewayapi/server.go`
12. `internal/app/app.go`
13. `web/src/protocol/terminal.ts`
14. `web/src/features/sessions/api.ts`
15. `web/src/components/workspace/WorkspaceSessionSidebar.vue`
16. `web/src/views/SessionsView.vue`
17. `web/src/i18n.ts`

预计新增或修改测试：

1. `internal/domain/session/state_test.go` 或现有状态测试位置。
2. `internal/infrastructure/repository/state/store_test.go`：重点覆盖 workspace 聚合存储、无碎片文件、不读取旧碎片文件。
3. `internal/domain/session/manager_test.go` 或 registry 测试中覆盖 manager 不写 starting。
4. `internal/application/terminal/registry_test.go`。
5. `internal/application/agent/client_test.go` 或相关 tunnel 测试。
6. `internal/transport/http/gatewayapi/terminal_test.go` 或相关 gateway route 测试。
7. 前端若无组件测试基础，至少依赖 typecheck / lint / existing vitest。

## Verification plan

优先使用项目显式入口：`justfile` 和 package scripts。

计划运行：

1. Go 格式化：
   - `go fmt ./cmd/... ./internal/...`
2. Go 单测：
   - `go test ./cmd/... ./internal/...`
3. Go 静态检查：
   - `go vet ./cmd/... ./internal/...`
4. 前端类型检查：
   - `yarn --cwd web typecheck`
5. 前端 lint：
   - `yarn --cwd web lint`
6. 前端单测：
   - `yarn --cwd web test`

不默认运行全量 `just check`：其中包含 `format:check`，当前工作区已有用户认可的 formatter 带出变更；Verification 阶段按实际 diff 再决定是否补跑并说明。

## Blockers

无实现阻塞项。

仍需 review 的重点：

1. `workspace.json` 聚合 session 当前事实的结构是否符合预期。
2. `exit.json` 已按用户要求一并收进 `workspace.json`，不再单独读写。
3. 已移除旧碎片文件迁移 / fold / fallback 计划。
4. `文案 B` 当前按 `重新运行` 实施；如用户提供精确文案，需同步修改 i18n。
5. create failure 要让前端明确拿到 failed 状态；实现时需选择响应 shape 或 refresh 策略。

## Assumptions

1. 归档 history 与当前 session 保存在同一 session directory 下。
2. 归档旧 history 后，新 `history.log` 为空文件或由新 history writer 首次写入创建。
3. `workspace.json` 表示当前 workspace 与其 sessions 的最新事实。
4. 归档仅本地文件系统与 `workspace.json` 内部可见，不通过 Web/API 展示。
5. `CreateSessionResponse` 可复用于 rerun 响应，即使 rerun 不创建新 session。
6. create failure / rerun failure 固定写入 failed。
7. create / close / delete 均按同步操作处理。
8. 状态集合只有 running/stopped/failed，不保留 starting/stopping 历史兼容。
9. 旧碎片文件不读、不迁移、不清理。

## Risks

1. `workspace.json` 写入频率提高；必须保持原子替换，避免半写入破坏整个 workspace/session 列表。
2. 不迁移旧碎片文件会让旧数据在新代码路径下不可见；这是用户明确要求的边界。
3. 复用同一个 session id 重新运行会改变原 stopped 记录的当前状态；旧运行只能通过本地归档 history 和 `ArchivedRuns` 追溯。
4. 归档后 rerun 失败会导致旧 history 已归档且当前 state failed；这是设计边界，需要错误提示准确。
5. 抽取 create/rerun 共享启动 helper 容易影响现有 create 行为，必须用测试覆盖。
6. 取消 starting/stopping 可能触及现有 WebSocket 控制消息、前端按钮状态和测试。
7. create failure 固定 failed 可能要求改变现有错误响应，让前端能拿到 failed 状态。
8. 状态机允许终态到 running 后，必须防止 refresh/recover/attach/delete 判断自动复活终态会话。
9. 前端已有未提交格式化变更，Implementation / Verification 需区分功能 diff 和已有格式化 diff。

## Rollback

如果实现后发现风险不可接受，回滚范围：

1. 移除 `/rerun` gateway route 和 agent method。
2. 移除 `Registry.RerunSession` 及相关 helper 调整。
3. 移除本地 history 重命名归档 helper。
4. 移除 workspace session node 中 `State` / `CurrentRun` / `ArchivedRuns` 相关写入。
5. 移除前端 rerun API、按钮、handler、i18n 文案。
6. 撤回终态到 running 的状态机扩展。
7. 如果 workspace 聚合存储已实施，回滚需要恢复 `session.json` / `state.json` / `process.json` / `exit.json` 写入；这会重新引入用户已要求移除的碎片化存储，不建议作为常规回滚方向。

## User review notes

- 用户要求切换为严格模式 / strict。
- 用户要求补 Plan、更新文档。
- 用户要求状态机相关纳入文档。
- 用户确认：启动、关闭、删除会话都是同步操作。
- 用户确认：消除 `starting` 概念。
- 用户确认：不进行历史兼容。
- 用户确认：create failure 固定 failed，且要返回明确状态。
- 用户确认：归档是文件整理，不需要归档接口。
- 用户确认：归档不需要云端知晓。
- 用户采纳：rerun 失败固定 failed。
- 用户追加：`workspace.json` 内部已有 session 大部分信息，合并收拾 `process.json` / `session.json` / `state.json`，重新组织结构并在这里 review，不需要这么碎片化的存储。
- 用户强调：这个存储设计在 Plan 阶段就要规划好。
- 用户确认：不向后兼容，不迁移已有文件，`exit.json` 也要收拾。
- 待用户 review 当前 Plan。
- 用户接受 Plan 并要求开始实现后，再进入 Implementation / 实现阶段。
