# 浅暗色主题与左下角设置入口验证
最后修改时间: 2026-06-24 22:09:28

Review status: Draft

## Requirement alignment

- Requirement 文档：`docs/requirement/20260624-light-dark-theme-settings-entry.md`
- Requirement status：`Accepted`

对齐结论：自动化与静态核对通过，主要实现与需求一致。

需求验收点核对：

1. 左下角存在设置入口：已实现。复用 `web/src/components/workspace/WorkspaceSessionSidebar.vue` footer 中现有设置入口。
2. 设置入口可打开设置 UI，并包含浅色 / 暗色主题切换项：已实现。设置菜单中新增 Theme 子菜单，使用 `DropdownMenuRadioGroup` 提供 `light` / `dark`。
3. 主题切换后主要 UI 视觉元素适配：已实现主要区域 token 化，包括全局基础样式、sidebar、workbench、login、help、settings、dialog、toast、terminal shell 等。
4. 不破坏现有路由、会话列表、终端区域和主要交互流程：自动化检查通过；未执行浏览器端手动交互验证。
5. 使用现有前端栈：已使用 `reka-ui` 现有 dropdown/radio menu 结构与 `tailwind css v4` arbitrary value + CSS variables；未引入新 UI 框架。
6. `web/src/store` 增加 storage 封装并使用 `localStorage` 保存主题：已实现 `web/src/store/storage.ts` 与 `web/src/store/theme.ts`。
7. 默认主题为 `dark`，无缓存或非法缓存回退 `dark`：已实现并由 `web/src/store/theme.test.ts` 覆盖。
8. 只提供 `light` / `dark`，不提供 `system`：已实现。
9. 终端区域跟随主题切换：已实现 xterm theme 参数化和 live/history terminal watch 当前主题。
10. 不出现明显低对比或主题错位：静态 token 核对通过；仍需真实浏览器视觉验收确认。

## Spec alignment

不适用。标准模式 / standard 未创建单独 Spec 文档。

## Plan alignment

- Plan 文档：`docs/plan/20260624-light-dark-theme-settings-entry.md`
- Plan status：`Accepted`

计划项对齐：

1. 新增 storage 封装：已完成 `web/src/store/storage.ts`。
2. 抽取主题 store：已完成 `web/src/store/theme.ts`。
3. 应用启动时初始化主题：已修改 `web/src/main.ts`，mount 前调用 `applyDocumentTheme(resolveInitialTheme())`，并在安装 Pinia 后让 store 接管。
4. 语言 localStorage 迁移到 storage 封装：已修改 `web/src/i18n.ts`。
5. 左下角设置菜单增加主题切换：已修改 `WorkspaceSessionSidebar.vue`。
6. 建立主题 token 与全局基础样式：已修改 `web/src/styles.css`。
7. 适配主要布局与工作台颜色：已覆盖 `SessionsView.vue`、`LoginView.vue`、`HelpView.vue`、`SettingsView.vue`、`LoginPanel.vue`、`CreateSessionPanel.vue`、`SessionWorkbench.vue`、`TerminalPane.vue`、`WorkspaceSessionSidebar.vue`。
8. xterm 跟随主题：已修改 `useXterm.ts`、`TerminalView.vue`、`HistoryTerminalView.vue`。
9. 测试与类型覆盖：已新增 `storage.test.ts` 与 `theme.test.ts`。

## Actual diff summary

主题功能相关变更：

- 新增 `web/src/store/storage.ts`：本地 storage 安全读写封装。
- 新增 `web/src/store/theme.ts`：主题类型、默认值、缓存恢复、document marker 和 Pinia store。
- 新增 `web/src/store/storage.test.ts`、`web/src/store/theme.test.ts`：覆盖 storage 与 theme store 行为。
- 修改 `web/src/main.ts`：启动阶段应用主题。
- 修改 `web/src/i18n.ts`：语言缓存改用 storage 封装，并增加主题文案。
- 修改 `web/src/styles.css`：新增 light/dark CSS variables，并迁移全局样式到 token。
- 修改 `web/src/components/workspace/WorkspaceSessionSidebar.vue`：设置菜单增加主题切换，菜单和 sidebar 样式适配主题。
- 修改 `web/src/components/terminal/useXterm.ts`、`TerminalView.vue`、`HistoryTerminalView.vue`：xterm 创建与运行时主题更新。
- 修改主界面、登录、帮助、设置、工作台、终端 pane、新建会话表单等组件样式以使用主题 token。

过程文档变更：

- 新增 `docs/requirement/20260624-light-dark-theme-settings-entry.md`。
- 新增 `docs/plan/20260624-light-dark-theme-settings-entry.md`。
- 新增本 verification 文档。

额外工作区变更：

- `go.mod` 存在 diff：`go.yaml.in/yaml/v3` 从 direct require 移至 indirect require。该改动与本次主题功能无关，并且在本次会话开始时已显示为 modified。

## Expected vs actual changed files

预期主题相关文件：

- `web/src/store/storage.ts`
- `web/src/store/theme.ts`
- `web/src/main.ts`
- `web/src/i18n.ts`
- `web/src/styles.css`
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
- `web/src/views/SessionsView.vue`
- `web/src/views/LoginView.vue`
- `web/src/views/HelpView.vue`
- `web/src/views/SettingsView.vue`
- `web/src/components/session/LoginPanel.vue`
- `web/src/components/session/CreateSessionPanel.vue`
- `web/src/components/session/SessionWorkbench.vue`
- `web/src/components/session/TerminalPane.vue`
- `web/src/components/terminal/useXterm.ts`
- `web/src/components/terminal/TerminalView.vue`
- `web/src/components/terminal/HistoryTerminalView.vue`
- `web/src/store/storage.test.ts`
- `web/src/store/theme.test.ts`

预期过程文档：

- `docs/requirement/20260624-light-dark-theme-settings-entry.md`
- `docs/plan/20260624-light-dark-theme-settings-entry.md`
- `docs/verification/20260624-light-dark-theme-settings-entry.md`

实际额外文件：

- `go.mod`：与主题功能无关，建议提交前拆分或确认是否一并包含。

## Acceptance criteria checklist

- [x] 左下角设置入口存在。
- [x] 设置入口中可切换 `light` / `dark`。
- [x] 没有 `system` 选项。
- [x] 默认主题为 `dark`。
- [x] 非法缓存值回退到 `dark`。
- [x] 主题设置通过 `localStorage` 缓存。
- [x] storage 封装位于 `web/src/store`。
- [x] 语言缓存已迁移到同一 storage 封装。
- [x] xterm live/history 终端接入主题参数和运行时更新。
- [x] 使用 `reka-ui` 与 `tailwind css v4`，未引入新 UI 框架。
- [ ] 浏览器视觉验收未执行。
- [ ] 真实终端会话运行时切换未执行。

## Test results

执行时间：2026-06-24 22:10 左右。

通过：

```bash
npm --prefix web run typecheck
```

结果：`vue-tsc --noEmit` 通过。

通过：

```bash
npm --prefix web run lint
```

结果：`eslint .` 通过。

通过：

```bash
npm --prefix web run test
```

结果：

```text
Test Files  7 passed (7)
Tests       32 passed (32)
```

通过：

```bash
npm --prefix web run build
```

结果：构建成功。

构建 warning：

1. `node_modules/@vueuse/core/dist/index.js` 中 `/* #__PURE__ */` annotation 位置无法被 Rolldown 解释，comment ignored。
2. `SessionsView` chunk 超过 500 kB，Vite/Rolldown 提示可考虑 code splitting 或调整 chunk size warning limit。

这些 warning 未导致构建失败，且来源为依赖 / chunk size 提示，本次未处理。

## Missed or expanded scope

Missed / 未完成：

1. 未执行真实浏览器视觉验收。
2. 未执行真实终端连接和运行时主题切换验收。

Expanded / 范围扩展：

1. 除主 sessions 工作台外，也适配了 `HelpView.vue`、`SettingsView.vue`、`LoginView.vue` 与 `LoginPanel.vue`，属于为避免明显主题错位而进行的合理样式覆盖。
2. 语言缓存迁移到 storage 封装，属于 Plan 明确项。

Unrelated / 无关变更：

1. `go.mod` 当前存在与本次主题功能无关的 diff。

## Risks

1. 虽然自动化验证通过，但浅色主题的最终视觉对比度仍需浏览器人工验收。
2. xterm 运行时主题更新通过 `terminal.options.theme = ...` 实现，类型检查和构建通过；仍建议在真实 live/history terminal 中确认即时生效。
3. 当前工作区包含无关 `go.mod` diff，若直接提交可能扩大提交边界。
4. build warning 未处理；当前不阻塞交付，但应避免误解为本次主题实现导致的失败。

## Incomplete items

1. 浏览器实际视觉检查未运行。
2. 真实终端连接 / 历史回放场景下的主题切换未运行。
3. `go.mod` 无关 diff 未拆分。

## Conclusion

自动化验证结论：通过。

- typecheck：通过。
- lint：通过。
- unit tests：通过，7 个 test files / 32 个 tests 全部通过。
- build：通过，有非阻塞 warning。

交付结论：本次实现满足 requirement / plan 的主要功能与自动化验证要求；建议用户在合入或提交前完成浏览器视觉验收，并处理或拆分无关 `go.mod` diff。
