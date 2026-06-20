# 前端 workspace/session tree 操作入口完善
最后修改时间: 2026-06-20 11:55:45

Review status: Accepted

## Background

当前前端 workspace/session tree 已展示 workspace 与 session，并已接入部分 reka-ui 模态窗和后端 API。用户希望继续完善前端操作入口，让用户能直接在 tree 节点上修改/删除 session、删除 workspace，并减少操作后的全量刷新。

## Goal

1. 在 session 节点上增加操作入口：
   - 修改 icon：打开 reka-ui 模态窗修改 session name。
   - 删除 icon：仅停止/终态 session 可见，打开 reka-ui 确认删除模态窗。
2. 在 workspace 节点上增加删除 icon：
   - hover 时可见。
   - 点击后打开 reka-ui 确认删除模态窗。
3. 对接已有后端接口：
   - `PATCH /api/sessions/{session_id}` 修改会话。
   - `DELETE /api/sessions/{session_id}` 删除会话。
   - `DELETE /api/workspaces/{workspace_id}` 删除 workspace 记录。
4. 调用成功后不再刷新 `/api/workspaces/tree`，改为在前端本地更新 `workspaceTree` / `sessions` / 打开 tab 等相关状态。
5. 继续使用已有错误处理和 toast 展示失败信息。

## Non-goal

1. 不新增后端接口。
2. 不实现真实文件系统目录删除。
3. 不改变后端删除约束：running/starting/stopping session 不应被删除，含运行中 session 的 workspace 仍由后端拒绝。
4. 不引入新的 UI 组件库；模态窗继续使用 reka-ui。
5. 不要求本次新增 workspace 重命名、session command/cwd 修改或 session 拖拽排序。

## User scenarios

1. 用户 hover 一个 session 节点时，能看到修改入口并打开重命名模态窗。
2. 用户 hover 一个 stopped/failed session 节点时，能看到删除入口并打开删除确认模态窗。
3. 用户 hover 一个 workspace 节点时，能看到删除入口并打开 workspace 删除确认模态窗。
4. 用户修改 session name 成功后，tree 和已打开 tab 标题立即反映新名称，不重新请求 workspace tree。
5. 用户删除 session 成功后，该 session 从所属 workspace children、本地 sessions 列表和相关打开 tab 中移除，不重新请求 workspace tree。
6. 用户删除 workspace 成功后，该 workspace 从 tree、本地 workspaces 和相关 session/tab 状态中移除，不重新请求 workspace tree。

## Acceptance

- [ ] session 节点 hover 时显示修改 icon。
- [ ] stopped/failed session 节点 hover 时显示删除 icon；starting/running/stopping session 不显示删除 icon。
- [ ] workspace 节点 hover 时显示删除 icon。
- [ ] 修改、删除 session 和删除 workspace 均通过 reka-ui 模态窗确认/提交。
- [ ] 成功调用 `PATCH /api/sessions/{session_id}` 后，本地状态更新，不调用 `refresh()` 或 `listWorkspaceTree()`。
- [ ] 成功调用 `DELETE /api/sessions/{session_id}` 后，本地状态更新，不调用 `refresh()` 或 `listWorkspaceTree()`。
- [ ] 成功调用 `DELETE /api/workspaces/{workspace_id}` 后，本地状态更新，不调用 `refresh()` 或 `listWorkspaceTree()`。
- [ ] API 失败时保留当前状态并展示 toast error。
- [ ] `yarn --cwd web typecheck` 通过。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- 使用轻量模式 / light。
- 本次仅完善前端已有接口入口和本地状态更新。
- “目录删除接口”按当前后端能力理解为删除 workspace record 的 `DELETE /api/workspaces/{workspace_id}`，不删除真实文件系统目录。
- 操作入口采用 hover 可见 icon，避免 tree 默认状态过于拥挤。

## Risk

1. 不刷新 workspace tree 意味着前端必须正确维护本地派生状态；如果后端返回状态与本地预期不同，需要以接口返回值或明确删除对象为准。
2. Workspace 删除若被后端因运行中 session 拒绝，前端应保持原状并提示错误。
3. 当前 workspace 排序、折叠状态、搜索过滤同时存在，删除后需要避免留下无效 active tab 或 selected item。
