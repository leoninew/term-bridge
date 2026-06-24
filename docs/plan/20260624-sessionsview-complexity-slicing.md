# SessionsView Complexity Decomposition Plan
最后修改时间: 2026-06-24 13:03:36

## Review status

Accepted

## Flow mode / Stage

严格模式 / strict；计划 / Plan 已接受，当前进入实现 / Implementation。

## Requirement / Spec basis

- Requirement: `docs/requirement/20260624-sessionsview-complexity-slicing.md`，状态 `Accepted`。
- Spec: `docs/spec/20260624-sessionsview-complexity-slicing.md`，状态 `Accepted`。

本计划遵守以下已确认约束：

1. 不发明 `SessionRef` / `SessionKey` 等新领域术语；会话 API/domain 定位直接使用 `SessionSummary.workspace_id + SessionSummary.id`。用户确认两个 workspace 下不会出现相同 `session.id`，因此 UI/internal active key、tab value、pending id 可使用裸 `session.id`。
2. `workspaceTree` 是 workspace/session domain state 的单一事实源；`workspaces`、`sessions` 如存在只能是 computed projection。
3. `web/src/store` 和 `web/src/composable` 按职责分工。
4. 通过组件拆分建立 terminal pane coordination boundary，不无谓重写 `TerminalView` / xterm internals。
5. 不改变后端 API contract，不改变用户可见行为，不执行 git 写操作。

## Implementation steps

### Step 0: 预检与基线确认

1. 确认当前 working tree 状态，避免覆盖用户未预期改动。
2. 读取当前 `SessionsView.vue`、`WorkspaceSessionSidebar.vue`、相关 API、terminal components、i18n 文案。
3. 保持现有业务行为清单，实施期间逐项对照：
   - auth check / login；
   - device list / selected device；
   - workspace tree refresh/reorder/remove；
   - session create/edit/delete/stop/rerun；
   - tab open/close/activate/reorder；
   - running terminal attach；
   - stopped/failed history loading；
   - toast/error presentation。

### Step 1: 建立目录和基础 store/composable

新增目录：

- `web/src/store/`
- `web/src/composable/`
- `web/src/components/session/`

新增基础模块：

1. `web/src/store/notifications.ts`
   - 迁移 toast state：`toasts`、`toastId`、`pushToast`、`notifyError`、`dismissToast`、`errorMessage`。
   - 保持 toast payload 类型：`kind`、`title`、`description`。

2. `web/src/components/session/ToastHost.vue`
   - 迁移当前 ToastProvider/ToastRoot/ToastViewport 相关 UI。
   - 接收 notification store 或 props/emits；Plan 选择：组件内使用 notification store，因 toast 是 presentation state。

3. `web/src/store/gateway.ts`
   - 迁移 auth/device state：`authInitialized`、`authenticated`、`loggingIn`、`usernameInput`、`passwordInput`、`devices`、`selectedDeviceId`。
   - 迁移 `initializeAuth()`、`login()`、`loadDevices()`、`selectDeviceId()`。
   - 不直接 toast；错误向调用层抛出。
   - 不直接 reset workspace/workbench。

验收点：

- `SessionsView.vue` 不再直接持有 toast/auth/device state。
- auth/device 行为保持一致。

### Step 2: workspace/session domain store

新增 `web/src/store/workspaceSessions.ts`。

迁移/重写：

1. state：
   - `workspaceTree` 作为唯一 mutable state。
   - `loading` 可放入此 store，表示 workspace tree refresh loading。

2. computed projection：
   - `workspaces` 从 `workspaceTree` 派生。
   - `sessions` 从 `workspaceTree.children` 派生，并在派生时补齐 `workspace_id`。
   - `workspaceById(workspaceId)`。
   - `sessionById(workspaceId, sessionId)`。
   - `sessionTitle(session)` 或 `sessionTitle(workspaceId, sessionId)`。

3. actions：
   - `reset()`。
   - `refresh(deviceId)` 调用 `listWorkspaceTree(deviceId)` 并 `applyWorkspaceTree(response.data)`。
   - `applyWorkspaceTree(nextWorkspaceTree)`。
   - `upsertSession(updated)`：只修改 `workspaceTree`。
   - `updateSession(updated)`：只在已有 session 存在时更新。
   - `removeSession(workspaceId, sessionId)`。
   - `removeWorkspace(workspaceId)`：删除 workspace，并返回被删除 session 列表用于 workbench 清理。
   - `reorderWorkspaces(deviceId, workspaceIds)`：乐观更新 `workspaceTree`，失败恢复。

4. 内部 helper：
   - `workspaceSummaryFromTree()`。
   - `sessionSummaryForWorkspaceTree()`。
   - `sessionsForWorkspace()`。
   - `makeTabKey(workspaceId, sessionId)` 如果需要，可以放在 workbench store 内部，避免成为领域术语。

验收点：

- `SessionsView.vue` 不再直接维护 `workspaces`、`sessions` mutable refs。
- API/domain session lookup 不丢失 workspace 维度；需要调用 API、读 history、更新 workspace tree、删除 session 时使用 workspace id + session id。UI/internal active key 可使用裸 `session.id`，因为用户确认 session id 全局唯一。

### Step 3: workbench store

新增 `web/src/store/workbench.ts`。

迁移/重写：

1. state：
   - `openedTabs: OpenSessionTab[]`。
   - `activeTabKey: string | null`。
   - `createSessionFormOpen` 可暂放 workbench store，因为它决定 workbench 区域显示 create form 还是 terminal pane。
   - `createSessionWorkspace` 是否进入 workbench store需谨慎：若只服务 create form，可放在 create draft composable；建议放入 create draft composable。

2. `OpenSessionTab`：
   - `workspaceId`
   - `sessionId`
   - history state：`historyText`、`historyLoaded`、`historyLoading`、`historyError`。
   - 不强制新增 `key` 字段；基于用户确认的 session id 全局唯一约束，tab active/value/drag key 可使用 `sessionId`。如果未来该约束变化，再改为内部 key，例如 `${workspaceId}:${sessionId}`。

3. actions：
   - `openSession(session)`：关闭 create form，确保 tab 存在，激活 tab，触发 history load。
   - `activateSession(workspaceId, sessionId)`。
   - `closeTab(workspaceId, sessionId)`。
   - `resetForSourceChange()`。
   - `ensureHistoryLoaded(deviceId, session)`：running session 不读 history；stopped/failed 才读。
   - `ensureActiveHistoryLoaded(deviceId, sessionResolver)`。
   - `resetTabHistory(workspaceId, sessionId)`。
   - `closeRemovedSessions(removedSessions)`。

4. 错误处理：
   - history error 写入 tab state。
   - 是否 toast 由调用层决定。

验收点：

- `SessionsView.vue` 不再直接维护 `openedTabs`、`activeTabId`、history loading state。
- tab active/close/lookup 基于用户确认的 session id 全局唯一约束正确工作；API/domain 路径仍使用 workspace id + session id。

### Step 4: create session draft 与 terminal size composable

新增：

1. `web/src/composable/useCreateSessionDraft.ts`
   - state：`workspace`、`sessionName`、`cwd`、`commandText`。
   - `reset(workspace, defaultName)`。
   - `cancel()` 可只清空或由上层关闭。
   - `validate()` 返回 `{ name, cwd, command, workspaceId }` 或 validation error code。
   - `defaultCommand()`。

2. `web/src/composable/useTerminalSize.ts`
   - 迁移 `measureInitialTerminalSize()`。
   - 迁移 `measureCreateSessionWorkbench()`。
   - 保留 fallback 规则和 `logTerminalDiagnostic()`。

验收点：

- `SessionsView.vue` 不再包含 UA command 判断、create form required validation、terminal initial size DOM measurement 细节。

### Step 5: dialog state 与 dialog components

新增：

- `web/src/composable/useSessionDialogs.ts`
- `web/src/components/session/EditSessionDialog.vue`
- `web/src/components/session/DeleteSessionDialog.vue`
- `web/src/components/session/RemoveWorkspaceDialog.vue`

实现策略：

1. `useSessionDialogs.ts` 管理：
   - `editDialogOpen`、`deleteSessionDialogOpen`、`removeWorkspaceDialogOpen`。
   - `selectedSession`、`selectedWorkspace`。
   - `openEditDialog(session)`、`openDeleteSessionDialog(session)`、`openRemoveWorkspaceDialog(workspace)`。
   - `clearSelectedSession()`、`clearSelectedWorkspace()`。

2. Dialog component：
   - 只负责展示和 emit confirm/cancel。
   - pending/disabled 从 props 传入。
   - edit input draft 可以在 `EditSessionDialog.vue` 内部维护，确认时 emit trimmed name。

验收点：

- `SessionsView.vue` 不再包含大段 dialog markup。
- API action 仍在 shell/wiring 层或 store action 中触发，不藏入 dialog component。

### Step 6: terminal pane 与 workbench components

新增：

- `web/src/components/session/CreateSessionPanel.vue`
- `web/src/components/session/TerminalPane.vue`
- `web/src/components/session/SessionWorkbench.vue`
- `web/src/components/session/LoginPanel.vue`

实施顺序：

1. `LoginPanel.vue`
   - 先抽最简单 UI，降低 `SessionsView.vue` template 噪声。

2. `CreateSessionPanel.vue`
   - 接收 draft state / props 或 v-model。
   - emit submit/cancel。
   - 暴露 workbench DOM ref 给 terminal size measurement，或由父层包裹提供 ref。

3. `TerminalPane.vue`
   - 接收 `session`、`tab`、`wsUrl`。
   - 渲染 running terminal 或 history 状态。
   - emit terminal state/error/refresh request。

4. `SessionWorkbench.vue`
   - 承载 tabs header、draggable tabs、empty state、create panel、terminal pane。
   - 仅做 presentation + event forwarding。

验收点：

- `SessionsView.vue` template 从完整 UI 实现变成组件组合。
- `TerminalView` / `HistoryTerminalView` / `useTerminalSocket` 不被无谓修改。

### Step 7: 收敛 SessionsView wiring

改造 `web/src/views/SessionsView.vue`：

1. 使用 stores/composables：
   - `useGatewayStore()`。
   - `useWorkspaceSessionsStore()`。
   - `useWorkbenchStore()`。
   - `useNotificationsStore()`。
   - `useCreateSessionDraft()`。
   - `useTerminalSize()`。
   - `useSessionDialogs()`。

2. 保留顶层 orchestration：
   - initialize auth。
   - login。
   - select device → reset workspace/workbench → refresh。
   - refresh tree。
   - create session。
   - edit/delete/stop/rerun session。
   - remove/reorder workspace。
   - terminal state/error handling。

3. 每个 orchestration 函数保持短小，只串联 API/store/composable/component event。

4. 移除已迁出的局部 refs、computed、helpers、markup。

验收点：

- `SessionsView.vue` 不再是巨型 application controller。
- 仍可从 shell 中清楚看到用户流程，但细节在对应模块内。

### Step 8: 测试补充

优先补以下 Vitest 单元测试：

1. `workspaceSessions` store：
   - `workspaceTree` projection 补齐 `workspace_id`。
   - `upsertSession` 只更新目标 workspace。
   - `removeSession(workspaceId, sessionId)` 不误删其他 workspace 同 id session。
   - `removeWorkspace` 返回被移除 session 列表并更新 tree。
   - `reorderWorkspaces` 排序逻辑。

2. `workbench` store：
   - open/activate/close tab 基于 session id 全局唯一约束正确维护 active tab。
   - close tab 后 active fallback 正确。
   - resetForSourceChange 清空 tabs/history。
   - resetTabHistory 只清目标 tab。
   - 可选补充测试记录：该 store 使用裸 `sessionId` 作为 UI/internal key 的前提是 session id 全局唯一。

3. `useCreateSessionDraft`：
   - Windows/macOS/Linux UA default command。
   - workspace path → cwd；无 workspace → `~`。
   - name/cwd/command required validation。

4. 保留现有 `useTerminalSocket` tests 不改，除非导入路径受影响。

## Files to change

### Expected new files

- `web/src/store/gateway.ts`
- `web/src/store/workspaceSessions.ts`
- `web/src/store/workbench.ts`
- `web/src/store/notifications.ts`
- `web/src/composable/useCreateSessionDraft.ts`
- `web/src/composable/useTerminalSize.ts`
- `web/src/composable/useSessionDialogs.ts`
- `web/src/components/session/LoginPanel.vue`
- `web/src/components/session/CreateSessionPanel.vue`
- `web/src/components/session/TerminalPane.vue`
- `web/src/components/session/SessionWorkbench.vue`
- `web/src/components/session/EditSessionDialog.vue`
- `web/src/components/session/DeleteSessionDialog.vue`
- `web/src/components/session/RemoveWorkspaceDialog.vue`
- `web/src/components/session/ToastHost.vue`
- `web/src/store/workspaceSessions.test.ts`
- `web/src/store/workbench.test.ts`
- `web/src/composable/useCreateSessionDraft.test.ts`

### Expected modified files

- `web/src/views/SessionsView.vue`
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`

### Not expected to change

- backend Go code
- `web/src/features/*/api.ts` unless TypeScript typing reveals a small import/type adjustment is necessary
- `web/src/components/terminal/TerminalView.vue`
- `web/src/components/terminal/HistoryTerminalView.vue`
- `web/src/features/sessions/useTerminalSocket.ts`
- git metadata or branch state

## Verification plan

优先运行项目已有显式入口：

1. `npm --prefix web run typecheck`
2. `npm --prefix web run test`
3. `npm --prefix web run lint`
4. 如有格式风险，运行 `npm --prefix web run format:check`

如果 lint/typecheck/test 暴露与 refactor 相关问题，修复后重跑对应命令。

手动/静态验收清单：

- auth checking/login branch 与原行为一致。
- device selection 后 workspace/workbench reset，避免旧 tab/history 残留。
- workspace tree 展示、搜索、expand、reorder、remove 行为一致。
- session tab open/close/activate 行为一致。
- running session 仍渲染 `TerminalView` 并生成 workspace-scoped ws url。
- stopped/failed session 仍加载 history。
- create session 默认 name/cwd/command 与原行为一致。
- edit/delete/stop/rerun session toast 与状态更新一致。
- remove workspace 清理 tabs/history。

## Blockers

当前无需要用户决策的 blocker。

## Assumptions

1. Pinia store 可直接在 `web/src/store` 下新增并被现有 `createPinia()` 支持。
2. `workspaceTree` 已包含展示和 session projection 所需信息。
3. 当前 selected device 是页面上下文；切换 device 时必须 reset workspace/workbench。
4. 两个 workspace 下不会出现相同 `session.id`；UI/internal active key、tab value、pending id 可使用裸 `session.id`，但 API/domain 路径仍使用 workspace id + session id。
5. `TerminalView`、`HistoryTerminalView`、`useTerminalSocket` 行为保持稳定，不需要重写。
6. 新增 Vitest store/composable tests 不需要额外测试依赖。

## Risks

1. 拆分范围大，容易在中途形成半迁移状态；实施时必须按 step 顺序完成并及时 typecheck。
2. Store 之间若互相 import 调用，可能变成新的巨型 controller；实现时以 shell orchestration 和参数传递控制依赖方向。
3. `workspaceTree` 单一事实源要求所有 mutation 精确更新 tree；如果误保留 mutable flat sessions，会回到双写问题。
4. UI/internal tab key 当前可使用裸 `session.id`，前提是 session id 全局唯一；如果未来该约束变化，必须改为稳定内部 key，否则 draggable tabs 和 active fallback 会回归。
5. Dialog component 拆分可能引入 props/emits 噪声；需要保持组件职责简单，不把 API 调用藏进组件。
6. Toast 边界若处理不当，可能让 store 直接绑定 i18n/toast side effect；实现中优先由 shell/component catch 后 notify。
7. 大范围 template 拆分可能影响 Tailwind class 或 DOM 结构；需通过 UI review 和必要手动检查兜底。

## Rollback

如实施过程中出现难以收敛的回归：

1. 保留已通过测试的纯 helper/store 单元测试。
2. 按文件粒度回退最近拆出的 component/store，并恢复 `SessionsView.vue` 对应片段。
3. 不使用 git reset/stash/checkout 等写操作，除非用户明确授权；回退通过 Edit/Write 精确修改完成。
4. 若发现 spec 与代码现实冲突，先暂停实现并更新 spec/plan，不静默改变范围。

## User review notes

- 用户要求进入 Plan / 计划阶段；Spec 已标记为 `Accepted`。
- 用户明确反对过度术语化，计划中不使用 `SessionRef` / `SessionKey` 作为领域概念。
- 用户在验证阶段补充确认：两个 workspace 下不会出现相同 `session.id`；R/S/P 文档已同步该约束。
- 本计划按彻底分解执行，不采用 MVP/低风险表层切片。
