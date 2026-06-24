# SessionsView Complexity Decomposition Spec
最后修改时间: 2026-06-24 13:03:36

## Review status

Accepted

## Flow mode / Stage

严格模式 / strict；规格 / Spec 已接受，当前进入计划 / Plan。

## Requirement basis

基于已接受的需求文档：

- `docs/requirement/20260624-sessionsview-complexity-slicing.md`

核心要求：

1. `SessionsView.vue` 从 application controller 降级为 composition shell。
2. `workspaceTree` 作为 workspace/session domain state 的单一事实源。
3. 会话 API/domain 定位直接使用已有 `SessionSummary.workspace_id` 和 `SessionSummary.id`；device 由当前 selected device 页面上下文提供。用户确认两个 workspace 下不会出现相同 `session.id`，因此 UI/internal active key 可使用裸 `session.id`。不引入 `SessionRef` / `SessionKey` 这类新领域术语。
4. `web/src/store` 与 `web/src/composable` 各司其职：store 管共享应用/domain 状态，composable 管可复用交互逻辑、派生逻辑或局部副作用。
5. Terminal pane coordination 通过拆分组件建立边界，不无谓重写 `TerminalView` / xterm internals。
6. 不引入新产品行为，不改变后端 API contract，不执行 git 写操作。

## Overview

本规格采用“store + composable + component boundary”的分解方式，而不是在 `SessionsView.vue` 内继续移动函数位置。

目标结构：

```text
web/src/store/
  gateway.ts
  workspaceSessions.ts
  workbench.ts
  notifications.ts

web/src/composable/
  useCreateSessionDraft.ts
  useSessionDialogs.ts
  useTerminalSize.ts

web/src/components/session/
  LoginPanel.vue
  SessionWorkbench.vue
  TerminalPane.vue
  CreateSessionPanel.vue
  EditSessionDialog.vue
  DeleteSessionDialog.vue
  RemoveWorkspaceDialog.vue
  ToastHost.vue
```

说明：

- 具体文件名可在 Plan 阶段微调，但职责边界必须保持。
- `store` 使用现有 Pinia，不视为引入新框架。
- `composable` 不持有跨页面长期 domain state，主要承载表单 draft、DOM measurement、dialog target 管理等可复用逻辑。
- `components/session` 用于从 `SessionsView.vue` 拆出 workbench、terminal pane、dialogs、login panel 等 UI 组合。
- 不新增所谓 `SessionRef` / `SessionKey` 模块。API/domain 路径需要定位会话时使用现有 `SessionSummary`，或直接传 `workspaceId + sessionId` 两个参数；UI/internal active key、tab value、pending id 可使用裸 `session.id`，前提是维护 session id 全局唯一不变量。

## Design decisions

### 1. 会话定位规则

不发明新的领域类型。现有数据已经足够表达会话 API/domain 定位：

- `SessionSummary.workspace_id`
- `SessionSummary.id`

补充约束：用户确认两个 workspace 下不会出现相同 `session.id`；即 session id 在当前 device/workspace tree 内全局唯一。

设计约束：

- `selectedDeviceId` 是页面级上下文，由 gateway store 提供。
- 需要调用 API、读 history、更新 tree、删除 session 等 workspace-scoped domain/API 操作时，使用 `workspaceId + sessionId`，其中 workspaceId 来自 `session.workspace_id` 或调用点所在 workspace。
- 组件事件可以继续传 `SessionSummary`，因为它已经包含 session id、workspace id、name、command、cwd、lifecycle state 等 UI 和操作所需信息。
- UI/internal active key、tab value、pending id 可以使用裸 `session.id`，但该用法依赖 session id 全局唯一不变量。
- 如果未来 session id 不再全局唯一，tab list / history cache / map lookup 必须改为基于 `workspaceId + sessionId` 的内部 helper，例如 `${workspaceId}:${sessionId}`；这只能是实现细节，不成为架构名词，不在 UI/API 层传播。

### 2. Gateway store

新增 `web/src/store/gateway.ts`。

职责：

- `authInitialized`
- `authenticated`
- `usernameInput`
- `passwordInput`
- `loggingIn`
- `devices`
- `selectedDeviceId`
- `initializeAuth()`
- `login()`
- `loadDevices()`
- `selectDeviceId(deviceId)`

边界：

- Gateway store 可以调用 `authMe`、`authLogin`、`listDevices`。
- Gateway store 不直接管理 workspace tree、tabs、history。
- Device switch 后 workspace/workbench reset 由 `SessionsView.vue` 顶层 wiring 或后续 application orchestrator 调用，不由 gateway store 隐式修改其他 store。
- Gateway store action 抛出错误或返回结果，toast 展示由调用层/notification 边界处理，避免 gateway store 强绑定 toast/i18n。

### 3. Workspace/session store

新增 `web/src/store/workspaceSessions.ts`。

职责：

- 持有 `workspaceTree: WorkspaceTreeSummary[]` 作为单一事实源。
- 提供 computed projections：
  - `workspaces: WorkspaceSummary[]`
  - `sessions: SessionSummary[]`
  - `sessionById(workspaceId, sessionId)`
  - `workspaceById(workspaceId)`
  - `sessionTitle(session)` 或 `sessionTitle(workspaceId, sessionId)`
- 提供 domain mutations/actions：
  - `reset()`
  - `refresh(deviceId)`
  - `applyWorkspaceTree(tree)`
  - `upsertSession(session)`
  - `updateSession(session)`
  - `removeSession(workspaceId, sessionId)`
  - `removeWorkspace(workspaceId)`
  - `reorderWorkspaces(deviceId, workspaceIds)`

设计约束：

- `workspaceTree` 是唯一 mutable collection source-of-truth。
- `workspaces`、`sessions` 只能是 computed projection，不能作为独立 ref 手工同步。
- `WorkspaceTreeSession` 进入 store 后，在 projection 时补齐 `workspace_id`，不直接改写 API 返回对象。
- `upsertSession` 必须按 `session.workspace_id` 定位 workspace children。
- `removeSession` 使用 `workspaceId + sessionId`，只从对应 workspace children 删除，避免全局裸 id 删除。
- `removeWorkspace` 返回被移除的 session 列表，至少包含每个 session 的 `workspace_id` 和 `id`，供 workbench 清理 tabs/history。
- `reorderWorkspaces` 乐观更新时保留 rollback 输入，失败后调用方可恢复或 store 内部恢复，但只能修改 `workspaceTree`。

### 4. Workbench store

新增 `web/src/store/workbench.ts`。

职责：

- `openedTabs`
- active tab
- tab order
- per-tab history state
- source/device change reset
- open/close/activate tab
- ensure history loaded
- reset history after rerun
- close tabs for removed session/workspace

建议数据结构：

```ts
type OpenSessionTab = {
  workspaceId: string
  sessionId: string
  historyText: string
  historyLoaded: boolean
  historyLoading: boolean
  historyError: string | null
}
```

说明：

- 用户确认 session id 在当前 device/workspace tree 内全局唯一，因此 active tab、Vue key、draggable item key、history cache lookup 可以直接使用 `sessionId`。
- 如果未来 session id 不再全局唯一，再增加内部 `key` 字段或 helper，例如 `${workspaceId}:${sessionId}`；该 key 仍只作为实现细节，不作为产品/领域术语，不暴露到 API，也不替代 API/domain 路径的 `workspaceId + sessionId`。

主要 actions：

```ts
openSession(session: SessionSummary): Promise<void>
activateSession(workspaceId: string, sessionId: string): Promise<void>
closeTab(workspaceId: string, sessionId: string): void
resetForSourceChange(): void
ensureHistoryLoaded(deviceId: string, session: SessionSummary): Promise<void>
resetTabHistory(workspaceId: string, sessionId: string): void
closeRemovedSessions(sessions: Array<{ workspace_id: string; id: string }>): void
```

边界：

- Workbench store 可调用 `readHistory`，因为 history state 是 workbench domain。
- Workbench store 不直接显示 toast；history load error 存入 tab state，并向调用方抛出或返回 error，调用方决定是否 notify。
- Workbench store 需要查询 session 时，通过参数接收 `SessionSummary` 或 resolver 函数，避免 workspace/session store 与 workbench store 强耦合成巨型 controller。

### 5. Notifications store/component

新增 `web/src/store/notifications.ts` 和 `ToastHost.vue`。

职责：

- 持有 toast queue。
- 提供 `pushToast(kind, title, description?)`。
- 提供 `notifyError(title, err)`。
- 提供 `dismissToast(id)`。

设计：

- notification store 可以是 Pinia store，因为 toast queue 是跨多个组件/动作共享的 presentation state。
- domain store 不应在内部随意 import notification store 后直接 toast；优先由 `SessionsView.vue` 或拆分后的 component/orchestrator 在 action catch 中调用。
- 如果 Plan 阶段采用在 component 中调用 store action 并 catch 的模式，则 notification store 只作为 presentation adapter。

### 6. Create session draft composable/component

新增 `web/src/composable/useCreateSessionDraft.ts` 和 `CreateSessionPanel.vue`。

职责：

- 管理 draft：`workspace`、`name`、`cwd`、`commandText`。
- 生成默认值：
  - command：按 UA 推断 `cmd` / `zsh` / `bash`。
  - cwd：workspace path 或 `~`。
  - name：由调用方传入 i18n 文案，例如 `t('dialog.defaultSessionName')`。
- validation：
  - name required
  - cwd required
  - command required
  - command text parse 使用现有 `commandFromText()`。

边界：

- composable 不直接调用 `createSession` API。
- `CreateSessionPanel.vue` 负责 UI 和 submit event。
- submit orchestration 由 `SessionsView.vue` wiring 或后续 session action 处理：measure terminal size → create session → get session → workspace store upsert/refresh → workbench open tab → notify。

### 7. Terminal size composable

新增 `web/src/composable/useTerminalSize.ts`。

职责：

- 包装当前 `measureInitialTerminalSize()` 和 `measureCreateSessionWorkbench()`。
- 保留 DOM measurement 行为。
- 保留 fallback cols/rows 规则。
- 保留 diagnostic logging。

边界：

- 不改 `measureXtermSize()`。
- 不改 xterm internals。
- `CreateSessionPanel` 或上层 workbench 提供 workbench DOM ref。

### 8. Dialog components/composable

新增组件：

- `EditSessionDialog.vue`
- `DeleteSessionDialog.vue`
- `RemoveWorkspaceDialog.vue`

可配合 `web/src/composable/useSessionDialogs.ts` 管理 selected target / open state。

边界：

- Dialog component 负责展示和 emit confirm/cancel。
- 是否 pending、是否 disabled 由 props 控制。
- API 调用和 store mutation 不放进 dialog 组件内部。
- edit dialog 内部可以持有局部 input draft，打开时从 props/session 初始化。

### 9. Terminal pane component

新增 `web/src/components/session/TerminalPane.vue`。

职责：

- 接收 active session、active tab history state、terminal ws url 或生成 URL 所需参数。
- 根据 `session.lifecycle_state` 选择渲染：
  - running → `TerminalView`
  - non-running + loading/error/history/no-history → `HistoryTerminalView` 或状态提示
- 处理 `TerminalView` 的 `state` / `terminal-error` 事件，向上 emit：
  - `terminal-state`
  - `terminal-error`
  - `refresh-session-requested`

边界：

- 不改 `TerminalView` 和 `HistoryTerminalView` 内部行为。
- `TerminalPane` 不直接 mutate workspace/session store；它只转发事件。
- terminal ws URL 可以由父层传入，或由 `TerminalPane` 接收 `deviceId + session` 后调用 API helper。为减少组件了解 API path，推荐父层传入 `wsUrl`。

### 10. Session workbench component

新增 `web/src/components/session/SessionWorkbench.vue`。

职责：

- 承载 tabs header、draggable tabs、empty state、create session panel slot/branch、terminal pane。
- 通过 props 接收 opened tabs、active tab、active session、session title resolver、create form open state。
- 通过 emits 转发：activate tab、close tab、open create form、submit create form、terminal events。

边界：

- Workbench component 主要是 presentation + event forwarding。
- Workbench state 和 history loading 不放在组件内部，而在 workbench store。

### 11. Login panel component

新增 `web/src/components/session/LoginPanel.vue` 或 `web/src/components/gateway/LoginPanel.vue`。

职责：

- 登录表单 UI。
- 接收 username/password/loggingIn props 或 v-model。
- emit submit。

边界：

- 不调用 auth API。
- 不直接持有 authenticated state。

### 12. SessionsView composition shell

改造后的 `SessionsView.vue` 保留：

- auth gate branch：checking / login / authenticated shell。
- store 初始化和顶层 wiring。
- sidebar、workbench、dialogs、toast host 组合。
- action orchestration：
  - login catch notify；
  - device switch 后 reset workspace/workbench 并 refresh；
  - create session 流程；
  - edit/delete/stop/rerun/remove/reorder 调用 API/store 并 notify。

限制：

- 不再直接维护 domain collection mutation。
- 不再直接维护 openedTabs/history state。
- 不再包含大段 dialog markup。
- 不再包含 terminal running/history pane markup。
- 不再包含 create session defaults/validation/terminal size measurement 细节。

## Affected components

### Must change

- `web/src/views/SessionsView.vue`
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`：基于用户确认的 session id 全局唯一不变量，可以继续用 `activeSessionId` 判断 active；如果未来 session id 不再全局唯一，应改为同时比较 workspace id 和 session id，或传入内部 tab key。

### New files expected

- `web/src/store/gateway.ts`
- `web/src/store/workspaceSessions.ts`
- `web/src/store/workbench.ts`
- `web/src/store/notifications.ts`
- `web/src/composable/useCreateSessionDraft.ts`
- `web/src/composable/useTerminalSize.ts`
- `web/src/composable/useSessionDialogs.ts`
- `web/src/components/session/LoginPanel.vue`
- `web/src/components/session/SessionWorkbench.vue`
- `web/src/components/session/TerminalPane.vue`
- `web/src/components/session/CreateSessionPanel.vue`
- `web/src/components/session/EditSessionDialog.vue`
- `web/src/components/session/DeleteSessionDialog.vue`
- `web/src/components/session/RemoveWorkspaceDialog.vue`
- `web/src/components/session/ToastHost.vue`

### Should not change unless Plan proves necessary

- `web/src/components/terminal/TerminalView.vue`
- `web/src/components/terminal/HistoryTerminalView.vue`
- `web/src/features/sessions/useTerminalSocket.ts`
- backend API/server code

## Interfaces

### Store usage sketch

```ts
const gateway = useGatewayStore()
const workspaceSessions = useWorkspaceSessionsStore()
const workbench = useWorkbenchStore()
const notifications = useNotificationsStore()
```

### Device switch orchestration

```ts
async function selectDeviceId(deviceId: string) {
  await gateway.selectDeviceId(deviceId)
  workspaceSessions.reset()
  workbench.resetForSourceChange()
  await refreshWorkspaceTree()
}
```

### Refresh orchestration

```ts
async function refreshWorkspaceTree() {
  if (!gateway.selectedDeviceId) return
  await workspaceSessions.refresh(gateway.selectedDeviceId)
  await workbench.ensureActiveHistoryLoaded(gateway.selectedDeviceId, (workspaceId, sessionId) =>
    workspaceSessions.sessionById(workspaceId, sessionId),
  )
}
```

### Create session orchestration

```ts
async function createSessionFromDraft(draft: ValidCreateSessionDraft) {
  const size = measureInitialTerminalSize()
  const created = await createSession(gateway.selectedDeviceId, draft.workspaceId, request)
  const session = await getSession(gateway.selectedDeviceId, created.workspace_id, created.session_id)

  if (!workspaceSessions.upsertSession(session)) {
    await workspaceSessions.refresh(gateway.selectedDeviceId)
  }

  await workbench.openSession(session)
}
```

### Terminal event orchestration

```ts
async function handleTerminalState(message: ServerControlMessage) {
  if (message.type === 'error') {
    notifications.pushToast('error', t('toast.terminalError', { code: message.code }), message.message)
    return
  }
  if (message.type === 'state' || message.type === 'exited') {
    await refreshActiveSession()
  }
}
```

## Technical questions

暂无需要用户决策的问题。以下技术细节进入 Plan 阶段细化：

1. Store actions 是直接抛错还是返回 `{ ok, error }` result；需保持调用层 toast 边界清晰。
2. Dialog target state 放 `useSessionDialogs` 还是由各 dialog 组件局部持有；Plan 阶段按模板简洁度决定。
3. 是否新增 pure function tests / store tests；Plan 阶段结合 Vitest 现状决定。

已收敛技术约束：用户确认两个 workspace 下不会出现相同 `session.id`，因此 tab/sidebar active key 可使用裸 `session.id`。

## Risks

1. 拆分过程中可能形成 store 之间互相 import、互相调用的隐性巨型 controller。Plan 阶段必须规定依赖方向。
2. 会话 API/domain 定位不含 deviceId，要求 device switch reset 必须彻底，否则可能出现旧 tab/history 引用新 device 上同名 workspace/session 的风险；UI/internal active key 使用裸 `session.id` 的前提是 session id 在当前 device/workspace tree 内全局唯一。
3. `workspaceTree` 作为单一事实源时，所有 mutation 都必须精确更新 tree；如果局部仍保留 mutable flat sessions，会重新引入双写风险。
4. TerminalPane 拆分若边界不清，可能把 terminal event handling 直接搬进组件，导致组件承担 API/store orchestration。组件应只 emit 或调用明确传入的 handler。
5. Toast/i18n 如果进入 domain store，会降低可测试性；如果全部留在 shell，又可能让 shell 保留过多 orchestration。Plan 阶段需要平衡。
6. 大量组件拆分可能带来 props/emits 噪声；需要以职责边界为准，不为“拆文件”而拆文件。
7. 现有测试仅发现 `useTerminalSocket` tests；新 store/composable 若无测试，复杂重构回归风险较高。

## Alternatives considered

### Alternative A: 只抽 auth/device store

拒绝。它只能减少局部代码，不能解决会话定位、`workspaceTree` 单一事实源、workbench/history、terminal pane、dialogs/toasts 的架构耦合。

### Alternative B: 建立一个巨型 `sessionsStore`

拒绝。会把 `SessionsView.vue` 的 application controller 平移到 store，违背“store/composable 每司其职”的要求。

### Alternative C: normalized workspace/session store

拒绝作为当前方向。用户已确认 `workspaceTree` 已有足够信息展示工作区和会话，因此采用 `workspaceTree` 单一事实源，projection 用 computed 派生即可。

### Alternative D: 为会话定位发明 `SessionRef` / `SessionKey`

拒绝。会话 API/domain 定位直接使用已有 `SessionSummary.workspace_id` 和 `SessionSummary.id` 即可；用户确认 session id 全局唯一，因此 UI/internal active key 可使用裸 `session.id`。如未来需要内部 tab/map key，也只是实现细节，不上升为领域抽象。

## User review notes

- 用户要求进入 Spec / 规格阶段；Requirement 已标记为 `Accepted`。
- 本规格按用户反馈固定：`workspaceTree` 单一事实源，两个 workspace 下不会出现相同 `session.id`，API/domain 会话定位使用已有 `workspace_id + session.id`，UI/internal active key 可使用裸 `session.id`，`web/src/store` 与 `web/src/composable` 分工，terminal coordination 拆组件，测试由后续阶段自行评估。
- 用户纠正：不要发明 `SessionRef` / `SessionKey` 等过多术语；规格已移除这些新概念。
