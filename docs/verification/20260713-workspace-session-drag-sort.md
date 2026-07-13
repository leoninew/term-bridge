# 工作区会话拖拽排序体验优化验证
最后修改时间: 2026-07-13 16:13:09

Review status: Accepted

## Requirement alignment

按 `docs/requirement/20260713-workspace-session-drag-sort.md` 核对：

- 工作区会话排序保持按单个工作区发送 `workspaceId` 和完整 `sessionIds`，未修改后端 API、协议或数据模型。
- 侧边栏已删除 `watch(treeItems, ...)` 和 `draggableTreeItems` 本地排序副本，渲染顺序仅由 `props.workspaceTree` 派生的 `treeItems` 驱动。
- 工作区与会话 Sortable 均改为 `:model-value` / `@update:model-value` 受控同步；排序请求只从 update 事件产生，`@end` 不再重复发起排序保存。
- 每个工作区会话列表继续使用独立 group，并设置 `pull: false`、`put: false`，禁止跨目录移动。
- ghost、chosen、dragging 样式、搜索禁用、按钮过滤和行内交互保护仍保留。

## Spec alignment

不适用：light / 轻量模式未创建独立 Spec。

## Plan alignment

按 Requirement 与已接受的拖拽同步重构计划核对：

- 已删除排序镜像状态和 watch。
- 已采用 props 单一来源的受控 Sortable 更新。
- 自动展开改为基于运行中会话的默认规则与用户显式展开/折叠覆盖值，不使用 watch。
- 已增加同一工作区连续两次排序请求不同顺序的 store 回归测试。
- 未引入后端修改、跨目录拖拽、队列/互斥机制或组件测试基础设施。

## Actual diff summary

- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
  - 以 `treeItems` 和 `workspace.children` 作为 Sortable 的只读 model 输入。
  - 使用 `@update:model-value` 将新的排序数组转换为既有 `reorderWorkspaces` / `reorderSessions` 事件。
  - 删除局部排序副本与同步 watcher，避免连续拖拽复用旧顺序。
  - 保持独立工作区 group、交互控件过滤、搜索禁用、拖拽视觉状态、释放后点击抑制和键盘边界保护。
- `web/src/store/workspaceSessions.test.ts`
  - 扩展两个均包含会话的工作区 fixture。
  - 覆盖跨工作区隔离、服务端规范顺序、失败回滚、无运行时目标，以及同一工作区连续两次不同排序请求。
- `docs/requirement/20260713-workspace-session-drag-sort.md`
  - 记录连续拖拽缺陷和单一来源重构决策。

## Expected versus actual changed files

| Expected file | Actual status | Notes |
|---|---|---|
| `docs/requirement/20260713-workspace-session-drag-sort.md` | Changed | Requirement 及修复记录已更新。 |
| `web/src/components/workspace/WorkspaceSessionSidebar.vue` | Changed | 完成受控 Sortable 同步重构。 |
| `web/src/store/workspaceSessions.test.ts` | Changed | 补充连续排序回归覆盖。 |
| `web/src/store/workspaceSessions.ts` | Unchanged | 既有乐观更新、服务端规范回填和回滚逻辑复用。 |
| `web/src/components/session/SessionsPageShell.vue` | Unchanged | 既有事件转发链路复用。 |

## Acceptance checklist

- [x] 每个工作区目录的会话使用独立 Sortable group，且禁止跨目录移动。
- [x] 排序请求携带目标工作区及该目录会话完整 ID 顺序。
- [x] 删除排序副本与 watch；UI 顺序仅从 `workspaceTree` 派生。
- [x] 连续两次已完成排序在 store 边界产生两次不同的请求顺序。
- [x] 服务端规范响应只替换目标工作区，会话兄弟目录保持不变。
- [x] 请求失败恢复原有工作区排序。
- [x] 搜索状态保持排序禁用，交互按钮仍被过滤为非拖拽起点。
- [x] ghost、chosen、dragging 状态样式保留并使用主题变量。
- [ ] 在可访问真实工作区 API 的浏览器环境中，完成同一目录两次连续拖拽的端到端观察。

## Test results

| Command | Result |
|---|---|
| `yarn --cwd web test src/store/workspaceSessions.test.ts` | Passed: 1 file, 9 tests. |
| `yarn --cwd web typecheck` | Passed. |
| `yarn --cwd web lint` | Passed. |
| `yarn --cwd web test` | Passed: 14 files, 80 tests. |
| `git diff --cached --check` and `git diff --check` | Passed: no whitespace errors reported. |

## Scope deviations

无实现范围扩张。工作树的其他未暂存文件属于已有并行改动，未纳入本次拖拽排序功能的暂存范围或验证结论。

## Risks

- 当前开发服务器曾因工作区 API 返回 HTTP 404 而无法加载真实工作区数据，故未能以该环境取得连续两次真实浏览器拖拽的请求与最终 UI 截图证据。
- 浏览器端仍应在连接真实本地或云端 runtime API 的环境中验收：同一工作区连续两次不同拖拽必须产生两组不同 `session_ids`，并在服务端响应后保持第二次顺序。

## Incomplete items

- 真实 API 环境下的浏览器端到端连续拖拽观察尚未完成，原因是当前验证环境的工作区列表请求返回 404。

## Conclusion

静态检查、定向 store 回归测试和完整前端测试均通过。代码层面已消除导致连续拖拽重复请求的 watch 与本地排序镜像，并以单一 props/store 来源同步排序；但真实工作区 API 环境中的浏览器端到端验收仍待完成。
