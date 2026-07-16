# 工作区/目录视图左栏切换入口

最后修改时间: 2026-07-16 21:30:38

Review status: Draft

## Flow mode / Stage

轻量模式 / light；阶段：Verification / 验证。

## Requirement alignment

对照 docs/requirement/20260716-left-panel-view-switch.md：

1. 工作区视图左 panel 右下角提供目录视图切换 icon。
2. 目录视图左 panel 右下角提供工作区视图切换 icon。
3. 目标 workspace 以当前活动 tab 所属工作区为准。
4. 无活动 tab 时切换按钮 disabled。
5. 继续路由切换，不实现跨路由 tab 状态保存。
6. local/cloud 复用既有 openWorkspaceFiles / goBackToSessions 路由导航。

## Spec / Plan alignment

不适用（light 模式按 requirement 核验）。

## Actual diff summary

本任务相关产品改动：

- WorkspaceSessionSidebar：footer 右侧新增目录视图切换按钮；接收 ctiveWorkspace；无活动 workspace 时 disabled。
- SessionsPageShell：由当前活动 session tab 计算 ctiveWorkspaceForViewSwitch 并下传。
- WorkspaceFileTree：status bar 右侧新增工作区视图切换按钮；依赖 ctiveDocument 判定可用。
- i18n.ts：新增中英文切换文案。
- WorkspaceFileTree.test.ts：补充无活动文件 tab disabled / 有活动 tab 可切换用例。
- docs/requirement/20260716-left-panel-view-switch.md：需求记录。

## Expected vs actual changed files

| 预期 | 实际 |
|------|------|
| 左 panel 切换入口组件 | 已改 WorkspaceSessionSidebar / WorkspaceFileTree |
| sessions 壳层目标 workspace | 已改 SessionsPageShell |
| i18n | 已改 |
| 相关单测 | 已改 WorkspaceFileTree.test.ts |
| 需求文档 | 已新增 |

说明：当前工作区还存在与“页面 loading/error 处理”相关的其他未提交改动（如 PageStatus、useAsyncAction 等），**不属于本任务范围**；暂存时应只纳入本任务文件/片段。

## Acceptance criteria checklist

- [x] 工作区视图左 panel 右下角有目录视图切换 icon
- [x] 目录视图左 panel 右下角有工作区视图切换 icon
- [x] 目标 workspace 来自当前活动 tab 所属工作区
- [x] 工作区视图无活动 session tab 时目录切换按钮 disabled；目录视图返回工作区按钮始终可用
- [x] 点击后走现有 files/sessions 路由（local/cloud 既有路径）
- [x] 未实现跨路由 tab 状态保存/恢复
- [x] 保留 a11y 名称/title；未移除既有 per-workspace openFiles / header back

## Test results

- yarn test src/components/workspace/WorkspaceFileTree.test.ts：4/4 通过
- yarn typecheck：通过
- 未做浏览器手工端到端点击验收（需本地 dev server 人工确认视觉与路由）

## Missed or expanded scope

- 无本任务主动扩 scope。
- 工作树中混有其他 feature 的 loading/error 处理改动，验证与暂存时已识别并隔离。

## Risks

- 目录视图启用条件严格依赖 ctiveDocument；仅进入 files 路由但未打开文件时按钮保持 disabled，符合当前需求，但可能被误解为按钮失效。
- 工作区视图若活动 tab 的 workspace 已不存在于 tree，则按钮 disabled。
- 混合工作树若误 stage 其他任务文件，会污染提交边界。

## Incomplete items

- 浏览器内 local/cloud 路径的手工点击验收未执行。
- 未提交 git commit（仅按用户要求准备验证并暂存本任务相关变更）。

## Conclusion

本任务实现与 requirement 对齐；自动化检查通过。可暂存本任务相关变更，但提交前应避免混入页面 loading/error 处理相关文件。
