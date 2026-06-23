# 停止态会话重新运行验证
最后修改时间: 2026-06-23 19:15:00

Review status: Accepted

## Verification scope

本验证对应严格模式 / strict 的 Verification / 验证阶段。

依据文档：

- Requirement: `docs/requirement/20260623-session-rerun.md`
- Spec: `docs/spec/20260623-session-rerun.md`
- Plan: `docs/plan/20260623-session-rerun.md`

验证过程中仅更新本文档；不继续修改产品代码。

## Requirement alignment

### 状态模型验收

| Requirement goal | 结果 | 说明 |
|---|---|---|
| stopped / failed 会话支持"重新运行" | 通过 | `Registry.RerunSession` 实现，校验终态后才允许启动 |
| 重新运行复用同一个 session / session.id | 通过 | rerun 响应返回原 session id，不创建新 record |
| 重新运行复用原会话名称、工作目录和命令 | 通过 | `commandFromSession` 从 `workspace.json` 读取原始 command record |
| 重新运行后的 terminal 不复用旧 history 输出 | 通过 | 启动前调用 `archiveCurrentHistory` 重命名旧 `history.log`，新 history 为空 |
| `workspace.json` 作为 workspace + sessions 当前事实的聚合存储 | 通过 | `SaveState` / `SaveProcess` / `SaveExit` 全部写入 `workspace.json` session node |
| 合并收拾 session.json / state.json / process.json / exit.json | 通过 | 新代码不再读写这些文件；测试显式断言旧碎片文件不被读取 |
| 当前 session 元数据集中保存在 workspace.json | 通过 | `SessionNode` 扩展了 `History` / `State` / `CurrentRun` / `ArchivedRuns` |
| history.log 仍作为大体积追加日志单独保留 | 通过 | `HistoryPath` 仍返回 `sessions/<session_id>/history.log` |
| 重新运行前旧 history.log 按时间戳重命名 | 通过 | `archiveCurrentHistory` 重命名为 `history.<archive_id>.log` |
| 旧 run 的小型 metadata 归入 workspace.json archived_runs | 通过 | `ArchiveCurrentRun` 将 state/process/exit 写入 `ArchivedRuns` |
| 不向后兼容旧碎片文件 | 通过 | 测试 `TestStoreIgnoresLegacySessionFragments` 显式覆盖 |
| 归档仅本地 runtime / registry 行为 | 通过 | 无归档 API，无 gateway/agent 通知 |
| 状态集合只有 running / stopped / failed | 通过 | `StateStarting` / `StateStopping` 已从 valid states 移除 |
| create failure 固定 failed | 通过 | `startSessionRuntime` 失败时调用 `saveSessionFailed` |
| rerun failure 固定 failed | 通过 | `RerunSession` 失败时调用 `saveSessionFailed` |
| 启动、关闭、删除 session 都是同步操作 | 通过 | close 同步等待 `done`；rerun 同步返回 running/failed |
| 新 live terminal 不回放旧输出 | 通过 | 旧 history 已重命名，新 history writer 从空文件开始 |

### 状态机验收

| 转换 | 结果 | 说明 |
|---|---|---|
| create ok → running | 通过 | `startSessionRuntime` 成功后保存 running |
| create failure → failed | 通过 | 失败路径调用 `saveSessionFailed` |
| running → stopped | 通过 | `waitLoop` 中正常退出保存 stopped |
| running → failed | 通过 | `waitLoop` 中 `result.Err != nil` 且非 stopped/closed 时保存 failed |
| stopped → running | 通过 | `RerunSession` 成功后保存 running |
| failed → running | 通过 | `RerunSession` 成功后保存 running |
| stopped → failed | 通过 | rerun 启动失败时保存 failed |
| failed → failed | 通过 | rerun 启动失败时保存 failed |

## Spec alignment

| Spec decision / interface | 结果 | 说明 |
|---|---|---|
| Rerun 是显式动作，不是自动恢复 | 通过 | 只有 `RerunSession` 触发终态→running；refresh/recover/attach 不触发 |
| workspace.json 是 session 当前事实聚合根 | 通过 | 所有 state/process/exit 读写都通过 `updateSessionJson` |
| 旧运行输出本地重命名归档 | 通过 | `archiveCurrentHistory` 使用 `os.Rename` |
| 后端复用原始 command record | 通过 | `commandFromSession` 读取 `sess.Command.Command` / `Args` |
| 同步操作语义 | 通过 | create/rerun 同步返回；close 同步等待 |
| 归档不走云端 / API | 通过 | 无归档 route/agent method |
| Agent tunnel method: rerun_session | 通过 | `client.go` 新增 `rerun_session` dispatch |
| Gateway API: POST /api/devices/:deviceId/sessions/:sessionId/rerun | 通过 | `server.go` 新增 `rerun` case |
| 前端 LifecycleState 收敛为 running/stopped/failed | 通过 | `terminal.ts` 类型定义已更新 |
| 前端 rerun API | 通过 | `api.ts` 新增 `rerunSession` |
| 前端 rerun 按钮 | 通过 | `WorkspaceSessionSidebar.vue` 新增 rerun 按钮 |
| 前端 rerun handler | 通过 | `SessionsView.vue` 新增 `rerunSessionFromSidebar` |

## Plan alignment

| Plan step | 结果 | 说明 |
|---|---|---|
| Step 1. 重组 workspace session storage model | 通过 | `SessionNode` 扩展 / mapping helper / `SaveSession` 只写 `workspace.json` |
| Step 2. 移除 legacy fragment 依赖 | 通过 | 删除所有 `session.json` / `state.json` / `process.json` / `exit.json` 读写 |
| Step 3. 收敛 session 状态集合和状态机 | 通过 | 移除 `StateStarting` / `StateStopping`；`CanTransition` 更新 |
| Step 4. create failure 为明确 failed 状态 | 通过 | `saveSessionFailed` + `startSessionRuntime` 失败路径 |
| Step 5. 取消 close 路径写入 stopping | 通过 | `runtime.go` 移除 `OnStopping` 和 close 时的 `StateStopping` |
| Step 6. rerun 路径归档当前 run | 通过 | `archiveCurrentHistory` + `ArchiveCurrentRun` |
| Step 7. 抽取同步启动 runtime helper | 通过 | `startSessionRuntime` / `startClaimedSessionRuntime` |
| Step 8. Registry 增加 RerunSession | 通过 | `RerunSession` 实现，含 claimSessionStart 防并发 |
| Step 9. Agent runtime 接口和 tunnel method | 通过 | `RuntimeAccess` 扩展 + `client.go` dispatch |
| Step 10. Gateway API route | 通过 | `server.go` 新增 `rerun` case |
| Step 11. 前端协议和 API | 通过 | `RerunSessionRequest` 类型 + `rerunSession` API |
| Step 12. 前端状态判断收敛 | 通过 | `canRerunSession` / `isActiveSession` 只检查 running/stopped/failed |
| Step 13. Sidebar 增加"重新运行"入口 | 通过 | `RotateCcw` icon + `rerunSession` emit |
| Step 14. SessionsView 接入 rerun | 通过 | `rerunSessionFromSidebar` handler |
| Step 15. i18n 文案 | 通过 | 中英文 `rerunSessionAria` / `sessionRerun` / `rerunSessionFailed` |

## Actual diff summary

### Backend

主要变更：

- `internal/domain/session/state.go`
  - 移除 `StateStarting` / `StateStopping`。
  - `Valid()` 只接受 running/stopped/failed。
  - `CanTransition` 允许 stopped→running / failed→running。

- `internal/domain/session/manager.go`
  - `Store` 接口移除 `SaveState`。
  - `Create` 不再写 starting。

- `internal/domain/workspace/workspace.go`
  - `SchemaVersion` 提升到 2。
  - `SessionNode` 扩展 `History` / `State` / `CurrentRun` / `ArchivedRuns`。
  - 新增 `HistoryRecord` / `StateRecord` / `ProcessRecord` / `ExitRecord` / `RunRecord` / `ArchivedRun` 类型。

- `internal/infrastructure/repository/state/store.go`
  - `SaveSession` 只写 `workspace.json`，不再写 `session.json`。
  - `LoadSession` 只从 `workspace.json` children 构造。
  - `SaveState` / `LoadState` 读写 `workspace.json` session node。
  - `SaveProcess` / `LoadProcess` 读写 `CurrentRun.Process`。
  - `SaveExit` / `LoadExit` 读写 `CurrentRun.Exit`。
  - 新增 `ArchiveCurrentRun`。
  - 新增 mapping helper：`sessionNodeFromSession` / `sessionFromWorkspaceNode` / `workspaceStateFromSessionState` / `sessionStateFromWorkspaceState` / `workspaceProcessFromProcess` / `processFromWorkspaceProcess` / `workspaceExitFromProcessExit` / `processExitFromWorkspaceExit`。
  - `listSessionsInWorkspace` 不再逐 session 读取碎片文件。

- `internal/application/terminal/registry.go`
  - 新增 `RerunSessionRequest` 类型。
  - 新增 `RerunSession` 方法。
  - 抽取 `startSessionRuntime` / `startClaimedSessionRuntime`。
  - 新增 `claimSessionStart` / `releaseSessionStart` 防并发。
  - 新增 `saveSessionFailed` / `startFailureReason` / `commandFromSession` / `validateSessionCwd` / `archiveCurrentHistory` / `archiveIdForTime` / `formatProcessCommand`。
  - `CreateSession` 使用新 helper，失败时调用 `saveSessionFailed`。

- `internal/application/terminal/runtime.go`
  - `lifecycleState()` 移除 stopping 判断。
  - `closeSession` 移除 stopping 写入和 WebSocket 消息。
  - `waitLoop` 根据 `result.Err` 判断最终状态。

- `internal/application/agent/client.go`
  - 新增 `rerun_session` dispatch。

- `internal/application/agent/runtime_access.go`
  - `RuntimeAccess` 接口新增 `RerunSession`。
  - `WebTerminalAccess` 实现 `RerunSession`。

- `internal/transport/http/gatewayapi/server.go`
  - 新增 `rerun` case，relay 到 `rerun_session`。

- `internal/app/app.go`
  - 移除 `OnStopping` hook。
  - `runExec` 最终状态判断与 `waitLoop` 一致。

### Frontend

主要变更：

- `web/src/protocol/terminal.ts`
  - `LifecycleState` 收敛为 `'running' | 'stopped' | 'failed'`。
  - 新增 `RerunSessionRequest` 类型。

- `web/src/features/sessions/api.ts`
  - 新增 `rerunSession` API。

- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
  - 新增 `RotateCcw` icon。
  - 新增 rerun 按钮，仅对 stopped/failed 展示。
  - 新增 `rerunSession` emit。
  - `canRerunSession` / `isActiveSession` 收敛状态判断。

- `web/src/views/SessionsView.vue`
  - 新增 `rerunSessionFromSidebar` handler。
  - 新增 `canRerunLifecycle` / `resetTabHistory` / `refreshSessionAfterRerunFailure`。
  - create 失败时调用 `refresh()` 以拉取 failed 状态。

- `web/src/i18n.ts`
  - 新增 `sidebar.rerunSessionAria` / `toast.rerunSessionFailed` / `toast.sessionRerun` 中英文文案。

### Tests

- `internal/domain/session/state_test.go`
  - 更新 `TestCanTransitionAllowsExpectedTransitions` / `TestCanTransitionRejectsInvalidTransitions` / `TestStateValid`。

- `internal/infrastructure/repository/state/store_test.go`
  - 新增 `TestStoreIgnoresLegacySessionFragments`。
  - 新增 `TestArchiveCurrentRunMovesCurrentMetadataIntoWorkspaceArchive`。
  - 更新 `TestStoreSavesAndListsRecordsFromWorkspaceAggregate`。
  - 新增 `saveWorkspaceSession` / `assertNotExists` helper。

- `internal/application/terminal/registry_test.go`
  - 新增 `TestRerunSessionArchivesHistoryAndReusesSessionId`。
  - 新增 `TestRerunSessionRejectsRunningSession`。
  - 新增 `TestRerunSessionFailureMarksFailed`。
  - 新增 `waitHistoryContains` / `assertNoArchiveFragments` helper。
  - 更新 `fakeManager` 支持多 session。

- `internal/transport/http/gatewayapi/terminal_e2e_test.go`
  - `fakeRuntimeAccess` 实现 `RerunSession`。

- `internal/app/app_test.go`
  - 更新断言：`exit.json` 不应创建；`workspace.json` 应包含 exit 聚合。
  - `state.json` 不应创建；`workspace.json` 应包含 failed state。

- `internal/transport/cli/cli_test.go`
  - 更新断言：`exit.json` 不应创建；`workspace.json` 应存在。

## Expected vs actual changed files

### 符合预期的变更范围

- 所有 Plan 中列出的 17 个文件均已修改。
- 测试文件覆盖 workspace 聚合存储、rerun 成功/失败/拒绝、状态机、legacy 忽略。
- 前端类型、API、组件、i18n 均已更新。

### 超出或偏离预期但有原因的变更

- `internal/domain/session/manager.go` 的 `Store` 接口移除了 `SaveState`：Plan 未明确标注接口变更，但 `SaveState` 实现仍在 `state store` 中，只是不再被 `Manager.Create` 调用；这是状态收敛的自然结果。
- `internal/app/app.go` 的最终状态判断逻辑与 `waitLoop` 重复：两者都根据 `result.Err` 判断最终状态；`app.go` 的 `runExec` 路径需要独立判断是因为 CLI 路径不经过 `waitLoop`。

### 未纳入本轮的内容

- 没有新增归档列表/读取 API。
- 没有实现云端归档通知。
- 没有实现前端归档 UI。
- 没有迁移旧碎片文件。
- 没有主动删除旧碎片文件。

## Acceptance checklist

- [x] stopped / failed 会话支持"重新运行"。
- [x] 重新运行复用同一个 session id。
- [x] 重新运行复用原会话名称、工作目录和命令。
- [x] 重新运行后的 terminal 不复用旧 history 输出。
- [x] `workspace.json` 作为聚合存储。
- [x] 合并收拾 session.json / state.json / process.json / exit.json。
- [x] 不向后兼容旧碎片文件。
- [x] 归档仅本地行为。
- [x] 状态集合只有 running / stopped / failed。
- [x] create failure 固定 failed。
- [x] rerun failure 固定 failed。
- [x] 启动、关闭、删除 session 都是同步操作。
- [x] 新 live terminal 不回放旧输出。
- [x] 前端 rerun 按钮仅对 stopped / failed 展示。
- [x] 前端 stop 按钮仅对 running 展示。
- [x] 前端状态判断不依赖 starting/stopping。
- [x] Gateway API `POST /rerun` route 接入。
- [x] Agent tunnel `rerun_session` method 接入。
- [x] 运行相关 Go 测试。
- [x] 不触碰 git 写操作。

## Command results

### Backend

```text
go vet ./cmd/... ./internal/...
```

结果：通过（无输出）。

```text
go test ./internal/domain/session/... ./internal/domain/workspace/... ./internal/infrastructure/repository/state/... ./internal/application/terminal/... ./internal/application/agent/... ./internal/transport/http/gatewayapi/... ./internal/transport/cli/... ./internal/app/...
```

结果：通过。

```text
ok  	termbridge-go/internal/domain/session	(cached)
ok  	termbridge-go/internal/domain/workspace	(cached)
ok  	termbridge-go/internal/infrastructure/repository/state	(cached)
ok  	termbridge-go/internal/application/terminal	1.884s
ok  	termbridge-go/internal/application/agent	(cached)
ok  	termbridge-go/internal/transport/http/gatewayapi	(cached)
ok  	termbridge-go/internal/transport/http/gatewayapi/auth	(cached)
ok  	termbridge-go/internal/transport/cli	1.349s
ok  	termbridge-go/internal/app	(cached)
```

## Static code inspection

- `internal/domain/session/state.go`
  - `StateStarting` / `StateStopping` 已移除。
  - `CanTransition` 允许 stopped→running / failed→running。
  - `Valid()` 只接受 running/stopped/failed。

- `internal/application/terminal/registry.go`
  - `RerunSession` 校验终态、无 live runtime、cwd 存在、size 合法。
  - `claimSessionStart` 防止并发 create/rerun。
  - `archiveCurrentHistory` 重命名旧 history，冲突时追加序号。
  - `startSessionRuntime` / `startClaimedSessionRuntime` 抽取启动逻辑。
  - 失败路径统一调用 `saveSessionFailed`。

- `internal/infrastructure/repository/state/store.go`
  - `SaveSession` / `LoadSession` 只操作 `workspace.json`。
  - `SaveState` / `SaveProcess` / `SaveExit` 通过 `updateSessionNode` 写入 session node。
  - `ArchiveCurrentRun` 将当前 run 移入 archive 并清空 `CurrentRun`。
  - `listSessionsInWorkspace` 从 `workspace.json.children` 读取 state/exit。

- `internal/application/terminal/runtime.go`
  - `lifecycleState()` 只返回 running。
  - `closeSession` 不写 stopping。
  - `waitLoop` 根据 `result.Err` 判断最终状态。

- `web/src/protocol/terminal.ts`
  - `LifecycleState` 类型收敛。

- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
  - `canRerunSession` / `isActiveSession` 状态判断收敛。

- `web/src/views/SessionsView.vue`
  - `rerunSessionFromSidebar` handler 完整：校验 → 测量 size → 调用 API → 更新状态 → 打开 tab。
  - create 失败时 `refresh()` 拉取 failed 状态。

## Missed or expanded scope

### Expanded scope

- `claimSessionStart` / `releaseSessionStart` 防并发机制：Plan 未明确标注，但是 `RerunSession` 和 `CreateSession` 共享启动 helper 后的自然需求。
- `archiveCurrentHistory` 冲突时追加序号：Plan 提到"冲突时追加序号或更高精度"，实现使用 `.1` / `.2` 后缀。
- `formatProcessCommand` helper：Plan 未明确标注，但 `ProcessRecord.CommandLine` 回填需要。

### Missed / incomplete scope

无。所有 Requirement / Spec / Plan 目标均已覆盖。

## Risks

1. **CLI 与 serve 路径的最终状态判断重复**
   - `app.go` 的 `runExec` 和 `runtime.go` 的 `waitLoop` 都包含最终状态判断逻辑。
   - 当前两者逻辑一致，但未来修改时需要同步更新两处。

2. **workspace.json 写入频率提高**
   - 每次 state/process/exit 变更都写入整个 `workspace.json`。
   - 当前使用临时文件 + replace 保证原子性，但高频写入可能成为瓶颈。

3. **归档后 rerun 失败**
   - 旧 history 已归档，当前 state 为 failed。
   - 用户需要明确的错误提示；当前 `saveSessionFailed` 会记录 reason，但前端错误展示依赖 `refreshSessionAfterRerunFailure`。

4. **旧碎片文件残留**
   - 不迁移、不清理旧文件；磁盘空间可能浪费。
   - 这是用户明确接受的边界。

5. **create failure 响应**
   - 当前 create 失败时返回错误，前端通过 `refresh()` 拉取 failed 状态。
   - 如果 `refresh()` 失败，用户可能看不到 failed session。

## Conclusion

当前实现满足 Plan 阶段确定的停止态会话重新运行目标，并完整覆盖了存储结构收敛要求：

- stopped / failed 会话支持"重新运行"，复用原 session id。
- `workspace.json` 作为 workspace + sessions 当前事实的聚合存储。
- `session.json` / `state.json` / `process.json` / `exit.json` 不再作为当前事实来源。
- 旧碎片文件不读取、不迁移、不兼容。
- 状态集合收敛为 running / stopped / failed。
- create / rerun failure 固定 failed。
- 前端 rerun 按钮、handler、i18n 文案完整。
- Gateway API 和 Agent tunnel 已扩展。
- 自动化验证命令全部通过。

但交付前仍需明确以下已知风险：

1. CLI 与 serve 路径的最终状态判断逻辑重复，未来修改需同步。
2. workspace.json 写入频率提高，需保持原子替换。
3. 归档后 rerun 失败的用户提示依赖前端 refresh 路径。

总体结论：验证通过，但带上述已知风险和后续修正项。
