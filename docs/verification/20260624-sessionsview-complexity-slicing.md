# SessionsView Complexity Decomposition Verification
最后修改时间: 2026-06-24 12:58:36

## Review status

Draft

## Flow mode / Stage

严格模式 / strict；验证 / Verification。

## User clarification during verification

用户在验证阶段确认：两个 workspace 下不会出现相同 `session.id`。

因此本验证按以下约束重新解释 session identity：

- API 调用、workspace/session store mutation、history read、delete/edit/stop/rerun 等 domain/API 路径仍应显式携带 `workspace_id + session.id`，以满足 workspace-scoped backend contract。
- UI presentation/internal tab active key、pending id、sidebar active id 可以使用裸 `session.id`，前提是该 id 在当前 device/workspace tree 内全局唯一。
- 之前将“重复 session id 跨 workspace”视为 blocker 的判断不再成立；该项降级为文档假设和测试覆盖说明。

## Requirement alignment

按 `docs/requirement/20260624-sessionsview-complexity-slicing.md` 核对，并结合验证阶段用户补充约束：

- `SessionsView.vue` 已明显从巨型 application controller 收敛为 composition shell，主要组合 sidebar、workbench、dialogs、toast，并保留顶层 orchestration。
- Gateway/Auth/Device state 已迁入 `web/src/store/gateway.ts`，toast queue 已迁入 `web/src/store/notifications.ts` 与 `ToastHost.vue`。
- Workspace/session domain state 已迁入 `web/src/store/workspaceSessions.ts`，并以 `workspaceTree` 作为唯一 mutable collection，`workspaces` / `sessions` 为 computed projection。
- Create session draft、terminal size、dialog state 已迁入 `web/src/composable/*` 与 `web/src/components/session/*`。
- Terminal pane 与 workbench UI 已拆分到 `TerminalPane.vue` 与 `SessionWorkbench.vue`，未重写 `TerminalView` / `HistoryTerminalView`。
- Domain/API 级 session 操作均保留 workspace-scoped 参数：`getSession`、`createSession`、`updateSession`、`deleteSession`、`closeSession`、`rerunSession`、`readHistory` 等路径使用 device id、workspace id、session id。
- Workbench/sidebar UI 仍使用裸 `session.id` 作为 active/tab/pending presentation key；在“session id 全局唯一”的已确认约束下，不构成交付 blocker。

## Spec alignment

按 `docs/spec/20260624-sessionsview-complexity-slicing.md` 核对，并结合验证阶段用户补充约束：

- 文件结构基本符合 Spec：新增 store、composable、session components，并拆分 `SessionsView.vue`。
- Store/composable/component 边界大体符合：domain store 不直接 toast；dialog component 只负责展示和 emit；TerminalPane 只转发 terminal event。
- Workspace/session store 符合 `workspaceTree` 单一事实源方向，`sessionById(workspaceId, sessionId)` 使用 workspace 约束查找。
- Workbench store 没有实现 Spec 建议的内部 tab key，`activeSessionId` 仍是裸 session id；由于用户确认 session id 不会跨 workspace 重复，此处属于实现简化而非功能错误。
- `WorkspaceSessionSidebar` 的 active 判断仍只接收/比较 `activeSessionId`；在 session id 全局唯一前提下可接受，但建议后续文档补充该系统不变量，避免需求文档与实现解释不一致。

## Plan alignment

按 `docs/plan/20260624-sessionsview-complexity-slicing.md` 核对，并结合验证阶段用户补充约束：

- Step 1、Step 2、Step 3、Step 4、Step 5、Step 6、Step 7 大体完成。
- Step 3 中“tab active/close/lookup 支持同名 session id 跨 workspace”的风险前提被用户否定；当前实现使用裸 session id 作为 UI/internal active key，在系统不变量下可接受。
- Step 8 部分完成：新增了 store/composable Vitest 测试。测试覆盖了主要 store/composable 行为，但未把“session id 全局唯一”作为显式测试/文档约束，也缺少 workspace reorder 成功路径断言。
- Verification plan 中四个 frontend 命令已执行并通过。

## Actual diff summary

本次实际 diff 包括：

- 新增 SpecFlow 过程文档：requirement、spec、plan、verification。
- 新增 session UI 组件：`CreateSessionPanel.vue`、`DeleteSessionDialog.vue`、`EditSessionDialog.vue`、`LoginPanel.vue`、`RemoveWorkspaceDialog.vue`、`SessionWorkbench.vue`、`TerminalPane.vue`、`ToastHost.vue`。
- 新增 composable：`useCreateSessionDraft.ts`、`useSessionDialogs.ts`、`useTerminalSize.ts`。
- 新增 store：`gateway.ts`、`notifications.ts`、`workbench.ts`、`workspaceSessions.ts`。
- 新增测试：`useCreateSessionDraft.test.ts`、`workbench.test.ts`、`workspaceSessions.test.ts`。
- 修改 `SessionsView.vue`，迁移状态和模板到 store/composable/component 后保留顶层 orchestration。
- 修改 `WorkspaceSessionSidebar.vue`。
- 新增 `LoginView.vue` 并修改 `router/index.ts` 增加 `/login` 路由和 `/gateway` redirect。

## Expected vs actual changed files

### Expected new files from plan

均已出现：

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

### Expected modified files from plan

均已出现：

- `web/src/views/SessionsView.vue`
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`

### Additional changed files

- `web/src/router/index.ts`
- `web/src/views/LoginView.vue`

上述额外文件与 auth/login boundary 拆分相关。它不是 backend/API 行为扩展，但属于 Plan 文件列表之外的 frontend routing/UI 范围扩展，需要用户在最终审查时确认接受。

### Not expected to change

未发现 backend Go code、terminal internals、`useTerminalSocket` 被修改。

## Acceptance criteria checklist

- [x] Workspace/session domain state 被迁出 `SessionsView.vue`，`workspaceTree` 为单一 mutable source-of-truth。
- [x] Workspace/session mutation 入口集中到 `workspaceSessions` store。
- [x] Workbench tabs/history state 被迁出 `SessionsView.vue`。
- [x] Gateway/Auth/Device state 被迁出 `SessionsView.vue`。
- [x] Create session draft/defaults/validation 被迁出主视图。
- [x] Dialog state 和 dialog markup 被抽离。
- [x] Toast queue / notification state 被抽离。
- [x] Terminal pane coordination 通过组件拆分形成边界，未重写 terminal internals。
- [x] `web/src/store` 与 `web/src/composable` 职责基本清晰，未形成单一巨型 store。
- [x] `SessionsView.vue` 已明显转为 composition shell。
- [x] 会话 API/domain 操作使用 `workspace_id + session.id`；UI/internal active key 使用裸 `session.id`，基于用户确认的 session id 全局唯一约束可接受。
- [x] 现有用户可见行为未发现静态违背项；尚未做浏览器手动验证。
- [x] Frontend typecheck / lint / tests / format check 通过。
- [x] 未执行 git add / commit / push / checkout / stash / reset / rebase / merge 等 git 写操作。

## Test results

已运行命令：

1. `yarn --cwd web typecheck`
   - 结果：通过
   - 输出摘要：`vue-tsc --noEmit`，`Done in 2.17s.`
2. `yarn --cwd web test`
   - 结果：通过
   - 输出摘要：4 个 test files passed，17 个 tests passed。
3. `yarn --cwd web lint`
   - 结果：通过
   - 输出摘要：`eslint .`，无错误。
4. `yarn --cwd web format:check`
   - 结果：通过
   - 输出摘要：`All matched files use Prettier code style!`

## Findings

### Finding 1: UI/internal tab key 依赖 session id 全局唯一不变量

- 文件：`web/src/store/workbench.ts`
- 文件：`web/src/components/session/SessionWorkbench.vue`
- 文件：`web/src/components/workspace/WorkspaceSessionSidebar.vue`
- 现象：workbench active tab、SessionWorkbench tab value/key、sidebar active selection 仍使用裸 `session.id`。
- 用户确认：两个 workspace 下不会出现相同 `session.id`。
- 结论：不作为 blocker。建议后续把“session id 在当前 device/workspace tree 内全局唯一”补入 requirement/spec/plan 或协议类型注释，避免未来维护者误以为这里可以承受重复 id。

### Finding 2: 测试仍可加强系统不变量和 reorder success path

- 文件：`web/src/store/workbench.test.ts`
- 现象：测试验证了 tab open/activate/close/history 行为，但没有显式记录“session id 全局唯一”这一约束。
- 文件：`web/src/store/workspaceSessions.test.ts`
- 现象：reorder 只测失败回滚，未测成功排序。
- 结论：不阻塞当前交付，但建议后续补充测试或注释，避免未来误改破坏约束。

## Missed or expanded scope

### Missed scope

在用户确认 session id 全局唯一后，未发现阻塞交付的 missed scope。

### Expanded scope

- 新增 `LoginView.vue` 和 `/login` route。该改动与 auth/login boundary 拆分相关，但不在 Plan 的 expected changed files 中，需要用户审查是否接受。

## Risks

1. 当前实现依赖“session id 全局唯一”这一验证阶段补充约束；如果未来后端或协议允许不同 workspace 下重复 session id，则 workbench/sidebar 需要改回 `workspace_id + session.id` 组合定位。
2. `LoginView.vue` 与 `/login` route 属于计划外范围扩展，需要确认它是否作为 auth boundary 拆分的一部分被接受。
3. 当前未运行浏览器手动验证，无法确认实际 UI 在 auth/login/device/workbench flows 下完全无行为漂移。
4. 测试覆盖虽然通过，但没有显式保护 session id 全局唯一不变量，也缺少 workspace reorder success-path 测试。

## Incomplete items

无阻塞交付的 incomplete items。

建议后续改进项：

- 在 requirement/spec/plan 或协议类型注释中记录 session id 全局唯一不变量。
- 补充 workspace reorder 成功路径测试。
- 可选：补充测试说明 workbench active key 使用 session id 的前提是全局唯一。
- 用户确认或收敛 `LoginView.vue` / `router/index.ts` 的范围扩展。
- 如需更高信心，使用浏览器手动验证 auth/login/device/workbench/create/edit/delete/stop/rerun flows。

## Conclusion

验证结论：通过，带非阻塞风险说明。

本次重构完成了 requirement/spec/plan 要求的主要职责分解，并且 frontend typecheck、tests、lint、format check 均通过。验证阶段用户补充确认 session id 不会在两个 workspace 下重复，因此 workbench/sidebar 使用裸 `session.id` 作为 UI/internal active key 不构成交付 blocker；API/domain 操作仍保留 workspace-scoped 参数，符合后端 contract。

当前可以进入用户最终审查。建议用户重点确认两项：

1. 接受 `LoginView.vue` 与 `/login` route 作为 auth boundary 拆分带来的范围扩展。
2. 接受“session id 全局唯一”作为本功能后续维护约束，或要求后续补写到 requirement/spec/plan 中。
