# 会话 Tab 溢出与清理验证
最后修改时间: 2026-07-13 17:57:19

Review status: Accepted

## Requirement alignment

按 `docs/requirement/20260713-session-tab-overflow-management.md` 核对：

- Tab 条右侧管理菜单与完整已打开 Tab 溢出列表已实现；菜单作为可横向滚动、可拖拽 Tab 容器的 sibling 保持固定。
- “关闭已停止标签页”只移除已打开且生命周期为 `stopped` 或 `failed` 的前端 Tab，不调用后端 Session API。
- “关闭运行中会话”使用 Reka Drawer，按工作区 tree 展示运行中会话；未打开的运行中会话默认勾选，已打开的运行中会话可由用户手动勾选。
- Drawer 只提交当前仍为 `running` 的已选 identity；关闭请求严格串行，每次成功立即写回 store，首次失败停止后续请求，并在 `finally` 统一刷新。
- Drawer 的会话行显示会话名称及快捷方式快照名或启动命令，不显示原始 ID；快捷方式不显示其底层命令。
- `SessionSourceIcon` 统一在会话树、Drawer、状态栏、Tab 条和溢出列表中渲染来源图标：快捷方式使用 `Keyboard`，其他来源使用 `Command`，生命周期颜色由共享映射决定。每个位置只保留一个会话来源图标。
- 状态栏回归修复后同时展示生命周期文本、一个来源图标和快捷方式名称或启动命令。
- Tab 横向滚动条隐藏，滚动与拖拽行为保持。

## Spec alignment

不适用：light / 轻量模式未创建独立 Spec。

## Plan alignment

不适用：light / 轻量模式未创建独立 Plan；按已接受 Requirement 与实施过程核对。

## Actual diff summary

- `web/src/components/session/SessionWorkbench.vue`、`SessionsPageShell.vue`、`TerminalPane.vue`、`web/src/store/workbench.ts`：增加 Tab 溢出管理、批量关闭停止 Tab 的原子状态变更，以及 `sessionId` 对齐约束，避免 Reka Tabs trigger/content 值失配。
- `web/src/components/session/CloseBackgroundSessionsDrawer.vue`、`web/src/styles.css`：以右侧 Drawer 承载运行中会话关闭候选，隐藏原始 ID，提供默认勾选、单行来源信息、680px 宽度和无滚动条 Tab strip 相关样式。
- `web/src/features/sessions/tabManagement.ts` 与测试：提供规范化 workspace tree、默认选择、当前 tree 目标解析、停止 Tab 筛选和严格串行关闭 helper。
- `web/src/components/session/SessionSourceIcon.vue`、`SessionStatusBar.vue`、`WorkspaceSessionSidebar.vue`：共享来源图标和生命周期色，状态栏保留 lifecycle 文本。
- `web/src/i18n.ts`、`web/package.json`、`web/yarn.lock`：更新文案，并升级 `reka-ui` 至 `2.10.0` 以使用真实 Drawer primitives。
- `docs/requirement/20260713-session-tab-overflow-management.md`：记录当前运行中会话候选范围与验收要求。

## Expected versus actual changed files

| Expected file / area | Actual status | Notes |
|---|---|---|
| `docs/requirement/20260713-session-tab-overflow-management.md` | Changed | 需求、决策与当前 Verification 阶段已记录。 |
| `docs/verification/20260713-session-tab-overflow-management.md` | Added | 本次验证记录。 |
| `web/src/components/session/*` | Changed / Added | Tab 菜单、Drawer、共享来源图标、状态栏和 Tab 渲染实现。 |
| `web/src/components/workspace/WorkspaceSessionSidebar.vue` | Changed | 使用共享会话来源图标，树缩进为 24px。 |
| `web/src/features/sessions/tabManagement.ts` 与测试 | Added | 纯 helper 和串行关闭行为覆盖。 |
| `web/src/store/workbench.ts` 与测试 | Changed | Tab 批量关闭及 sessionId 不变量保护。 |
| `web/src/styles.css`、`web/src/i18n.ts` | Changed | Drawer、隐藏滚动条与文案支持。 |
| `web/package.json`、`web/yarn.lock` | Changed | Reka Drawer 所需版本更新。 |

## Acceptance checklist

- [x] 管理菜单固定在 Tab 条右侧，不参与横向滚动或拖拽。
- [x] 溢出列表按当前打开 Tab 排序展示，并可走既有激活路径。
- [x] 批量关闭仅移除已打开的 `stopped` 与 `failed` Tab，不调用后端停止 API。
- [x] Drawer 只展示 `running` 会话；未打开的运行中会话默认勾选，已打开运行中会话默认不勾选但可选择。
- [x] Drawer 以 `(workspaceId, sessionId)` 解析选择，关闭请求串行执行并在首次失败停止后续请求。
- [x] Drawer 只显示会话名称及快捷方式名称或启动命令；快捷方式不暴露底层命令。
- [x] 会话树、Drawer、状态栏、Tab 条与溢出列表使用单一共享来源图标，生命周期颜色一致，来源 tooltip 可用。
- [x] 状态栏显示 lifecycle 文本、来源图标和快捷方式名称或启动命令。
- [x] Tab strip 隐藏浏览器原生横向滚动条，同时保留横向滚动与拖拽。
- [x] Helper 测试、类型检查、lint、格式检查、生产构建和 diff 空白检查通过。
- [ ] 连接真实 runtime API 的浏览器中完成 Drawer、串行关闭、Tab 溢出激活及来源 tooltip 的人工端到端验收。

## Test results

| Command | Result |
|---|---|
| `yarn --cwd web test` | Passed: 14 files, 80 tests. |
| `yarn --cwd web typecheck` | Passed. |
| `yarn --cwd web lint` | Passed. |
| `yarn --cwd web prettier … --check --ignore-unknown` | Passed for本次范围内的新增/修改 session、helper、store、样式、i18n 与 requirement 文件。 |
| `yarn --cwd web build:cloud` | Passed. 仅报告既有 `@vueuse/core` PURE annotation 与大于 500 kB chunk 警告。 |
| `git diff --check` | Passed: no whitespace errors reported. |

## Scope deviations

- `WorkspaceSessionSidebar.vue` 的完整 Prettier 检查仍因先前已存在的格式漂移失败；未执行 `--write`，避免改写无关用户工作区内容。新增会话来源图标代码通过类型检查与 lint。
- 未启动开发服务器，按用户要求由用户自行进行浏览器验证。

## Risks

- Drawer、来源 tooltip、选中已打开会话后的运行时状态刷新，以及关闭请求失败后保留 Drawer，仍依赖真实 runtime API 环境进行最终交互验收。
- 生产构建存在依赖包 PURE annotation 警告和 500 kB chunk 警告；构建成功，本次未改变依赖包注解或 bundle 拆分策略。

## Incomplete items

- 尚未在真实浏览器 / runtime API 环境手工验证本功能的所有交互路径。

## Conclusion

需求范围内的纯逻辑、类型、lint、格式、生产构建与 diff 检查均通过。实现满足 Tab 溢出管理、运行中会话 Drawer 串行关闭、来源信息和单一共享来源图标的要求；真实 runtime API 环境下的人工交互验收仍待用户完成。
