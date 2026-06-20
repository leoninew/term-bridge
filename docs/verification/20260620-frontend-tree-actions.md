# 前端 workspace/session tree 操作入口完善验证
最后修改时间: 2026-06-20 13:33:49

Review status: Accepted

## Requirement alignment

基于 `docs/requirement/20260620-frontend-tree-actions.md`，Review status: Accepted。

| Requirement | Result | Notes |
|---|---|---|
| session 节点 hover 显示修改 icon | Pass | `WorkspaceSessionSidebar.vue` 中 session 行添加 `Pencil` 操作按钮，默认 `opacity-0`，hover 可见。 |
| stopped/failed session 节点 hover 显示删除 icon，active lifecycle 不显示 | Pass | `canDeleteSession()` 仅允许 `stopped` / `failed`，按钮使用 `v-if` 控制。 |
| workspace 节点 hover 显示删除 icon | Pass | workspace 行添加 `Trash2` 操作按钮，默认 `opacity-0`，hover 可见。 |
| 修改、删除 session 和删除 workspace 均通过 reka-ui 模态窗 | Pass | icon 入口复用 App 中已有 `DialogRoot` / `AlertDialogRoot` 模态窗事件链。 |
| `PATCH /api/sessions/{session_id}` 成功后本地更新，不刷新 tree | Pass | `renameSelectedSession()` 改为 `updateSessionInState(updated)`，成功路径不调用 `refresh()`。 |
| `DELETE /api/sessions/{session_id}` 成功后本地更新，不刷新 tree | Pass | `deleteSelectedSession()` 改为 `removeSessionFromState(session.id)`，成功路径不调用 `refresh()`。 |
| `DELETE /api/workspaces/{workspace_id}` 成功后本地更新，不刷新 tree | Pass | `removeSelectedWorkspace()` 改为 `removeWorkspaceFromState(workspace.id)`，成功路径不调用 `refresh()`。 |
| API 失败时保留当前状态并展示 toast error | Pass | 三个操作均只在 API await 成功后变更本地状态；catch 路径保留原 toast error。 |
| `yarn --cwd web typecheck` 通过 | Pass | 实际命令使用绝对路径 `yarn --cwd /Volumes/media/SourceCodes/mywork/TermBridge-go/web typecheck`，结果通过。 |

## Spec alignment

不适用。轻量模式 / light 未创建独立 spec，按 requirement 核对。

## Plan alignment

不适用。轻量模式 / light 未创建独立 plan，按 requirement 核对。

## Actual diff summary

本次验证范围内的实际改动：

- `docs/requirement/20260620-frontend-tree-actions.md`
  - 新增轻量模式需求文档，并标记 `Accepted`。
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
  - 将 tree 渲染调整为显式 workspace/session 两层结构，并接入 `VueDraggable` 顶层 workspace 排序。
  - workspace 行使用展开/折叠文件夹图标，并增加 hover 可见删除按钮。
  - session 行增加 hover 可见 rename 按钮；stopped/failed session 增加 hover 可见 delete 按钮。
  - 节点行使用 `div role="button"`，避免 action button 嵌套在 button 内。
- `web/src/App.vue`
  - 接入 workspace 排序持久化。
  - session rename/delete 和 workspace delete 成功后改为本地状态更新。
  - 新增 `updateSessionInState()`、`removeSessionFromState()`、`removeWorkspaceFromState()`、`orderWorkspaceTree()`。
- `web/src/features/workspaces/api.ts`
  - 增加现有后端接口 `PATCH /api/workspaces/order` 的前端调用函数 `updateWorkspaceOrder()`。

## Expected vs actual changed files

| Expected | Actual | Notes |
|---|---|---|
| `docs/requirement/20260620-frontend-tree-actions.md` | Changed | 符合 SpecFlow light requirement 记录。 |
| `web/src/components/workspace/WorkspaceSessionSidebar.vue` | Changed | 符合 tree 节点操作入口需求；同时包含本轮上下文中的 workspace 排序和 tree 展示调整。 |
| `web/src/App.vue` | Changed | 符合本地状态更新和事件处理需求；同时包含 workspace 排序持久化。 |
| `web/src/features/sessions/api.ts` | Not changed | 已有 `updateSession()` / `deleteSession()` 可复用，无需修改。 |
| `web/src/features/workspaces/api.ts` | Changed | 为已存在后端 workspace order 接口补前端调用；workspace delete 已存在。 |

## Acceptance criteria checklist

- [x] session 节点 hover 时显示修改 icon。
- [x] stopped/failed session 节点 hover 时显示删除 icon；starting/running/stopping session 不显示删除 icon。
- [x] workspace 节点 hover 时显示删除 icon。
- [x] 修改、删除 session 和删除 workspace 均通过 reka-ui 模态窗确认/提交。
- [x] 成功调用 `PATCH /api/sessions/{session_id}` 后，本地状态更新，不调用 `refresh()` 或 `listWorkspaceTree()`。
- [x] 成功调用 `DELETE /api/sessions/{session_id}` 后，本地状态更新，不调用 `refresh()` 或 `listWorkspaceTree()`。
- [x] 成功调用 `DELETE /api/workspaces/{workspace_id}` 后，本地状态更新，不调用 `refresh()` 或 `listWorkspaceTree()`。
- [x] API 失败时保留当前状态并展示 toast error。
- [x] `yarn --cwd web typecheck` 通过。

## Test results

### Web typecheck

Command:

```sh
yarn --cwd /Volumes/media/SourceCodes/mywork/TermBridge-go/web typecheck
```

Result: Pass。

Output summary:

```text
$ vue-tsc --noEmit
Done in 9.43s.
```

### Web lint

Command:

```sh
yarn --cwd /Volumes/media/SourceCodes/mywork/TermBridge-go/web lint
```

Result: Pass。

Output summary:

```text
$ eslint .
Done in 7.04s.
```

## Missed or expanded scope

Expanded scope:

- 当前实际 diff 中包含此前同一工作树内的 workspace 排序前端接入：`updateWorkspaceOrder()`、`reorderWorkspaces()` 和 `VueDraggable` 顶层 workspace 排序。这是本次对 tree 操作入口完善前已经进行的相邻 UI 工作，提交前应作为同一前端 tree 改进边界确认，或按需拆分。

Missed scope:

- 未执行浏览器端人工交互验证；当前验证基于代码核对、typecheck 和 lint。

## Risks

1. 本地状态更新替代刷新后，若后端未来返回更多 session/workspace 派生字段，前端更新逻辑需要同步维护。
2. Workspace 删除被后端拒绝时前端会保留状态并 toast error；需要人工确认错误提示体验是否符合预期。
3. 当前 diff 有 staged 与 unstaged 混合状态：`web/src/App.vue` lint 修复仍是 unstaged；提交前需要重新确认 staging 边界。
4. 操作按钮为 hover 可见，触屏或无 hover 环境下入口可发现性较弱。

## Incomplete items

- 未做浏览器手工验收。
- 未提交 git；也未执行 `git add` 调整当前 staged/unstaged 状态。

## Conclusion

验证通过。实现与轻量 Requirement 对齐，前端 typecheck 与 lint 均通过。需要注意当前工作树包含 workspace 排序相邻改动，以及 `web/src/App.vue` 存在未暂存 lint 修复；提交前应确认是否一并纳入。
