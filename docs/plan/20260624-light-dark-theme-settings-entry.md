# 浅暗色主题与左下角设置入口计划
最后修改时间: 2026-06-24 21:41:19

Review status: Accepted

## Requirement basis

- Requirement: `docs/requirement/20260624-light-dark-theme-settings-entry.md`
- Requirement status: `Accepted`
- 流程模式：标准模式 / standard

已确认约束：

1. 只支持 `light` / `dark`，不支持 `system`。
2. 默认主题为 `dark`。
3. 在 `web/src/store` 添加 storage 封装，使用 `localStorage` 缓存主题设置。
4. 主题切换参考现有语言切换。
5. 左下角设置入口承载主题切换。
6. 终端区域跟随主题切换，包括 xterm 内部主题色。
7. 前端技术栈为 `reka-ui` + `tailwind css v4`。

## Current code observations

1. 语言切换当前集中在 `web/src/i18n.ts`：
   - `locales` / `localeLabels` 定义可选项与标签。
   - 初始化时从 `window.localStorage.getItem('termbridge.locale')` 读取。
   - `setLocale()` 更新 `i18n.global.locale.value` 并写回 localStorage。
2. 左下角设置入口已存在于 `web/src/components/workspace/WorkspaceSessionSidebar.vue` footer，使用 `reka-ui` 的 `DropdownMenuRoot` / `DropdownMenuSub` / `DropdownMenuRadioGroup` 实现语言切换。
3. 当前大量页面颜色使用 Tailwind arbitrary color 或 `slate-*` class，整体是暗色硬编码。
4. `web/tailwind.config.ts` 已配置 `darkMode: 'class'`。
5. 全局样式在 `web/src/styles.css`，目前 `:root` 默认是暗色背景与文字。
6. xterm 主题在 `web/src/components/terminal/useXterm.ts` 的 `terminalOptions()` 中硬编码暗色：`background`、`foreground`、`cursor`、`selectionBackground`。
7. live/history terminal 分别由 `TerminalView.vue` 与 `HistoryTerminalView.vue` 创建 xterm；两者当前没有主题输入，也没有主题变更监听。

## Implementation steps

### 1. 增加通用 localStorage 封装

在 `web/src/store/storage.ts` 新增一个小型封装，避免业务代码直接重复访问 `window.localStorage`：

- 暴露读取字符串的方法，例如 `readStorageValue(key): string | null`。
- 暴露写入字符串的方法，例如 `writeStorageValue(key, value): void`。
- 可选提供删除方法，例如 `removeStorageValue(key): void`。
- 对 `window.localStorage` 访问失败保持安全降级：捕获异常并返回默认空值 / 静默跳过写入，避免隐私模式或存储不可用导致 UI 初始化失败。

实现时保持封装简单，不引入复杂序列化层；本次主题只需要字符串值。

### 2. 抽取主题 store

在 `web/src/store/theme.ts` 新增 Pinia store：

- 定义 `themes = ['light', 'dark'] as const`。
- 定义 `AppTheme` 类型。
- 定义 `themeLabels: Record<AppTheme, ...>` 或在组件侧通过 i18n 文案展示。
- 使用 storage 封装读取 `termbridge.theme`。
- 初始主题规则：缓存值合法则使用缓存，否则使用 `dark`。
- 暴露当前 `theme` 状态。
- 暴露 `setTheme(theme: AppTheme)`：
  - 校验只接受 `light` / `dark`。
  - 更新 Pinia 状态。
  - 写入 `localStorage`。
  - 同步更新 `document.documentElement` 上的主题标记。
- 暴露 `applyTheme(theme)` 或内部函数：
  - 对 `document.documentElement` 设置 / 移除 `dark` class，使现有 Tailwind `darkMode: 'class'` 可用。
  - 设置 `data-theme="light|dark"`，方便 CSS token 和非 Tailwind 区域读取。
  - 默认 dark 时也应明确设置 `dark` class，避免首屏与默认状态不一致。

### 3. 应用启动时初始化主题

在 `web/src/main.ts` 创建应用前或挂载前初始化主题：

- 创建 Pinia 实例后安装到 app。
- 在 mount 前获取 `useThemeStore()` 并调用初始化 / apply 方法。
- 目标是尽量减少首屏从默认样式闪到用户缓存主题的时间。

如果 Pinia store 在 app.use(pinia) 前不方便调用，则将纯函数 `resolveInitialTheme()` / `applyDocumentTheme()` 与 store 解耦，先在模块加载阶段应用初始主题，再由 store 接管状态。

### 4. 将语言 localStorage 访问迁移到 storage 封装

调整 `web/src/i18n.ts`：

- 将直接调用 `window.localStorage.getItem('termbridge.locale')` 改为 storage 封装读取。
- 将 `setLocale()` 中直接 `setItem` 改为 storage 封装写入。
- 保持现有语言切换行为不变。

这样满足“storage 封装 localStorage 缓存主题设置”，同时让语言和主题使用一致的存储入口。

### 5. 在左下角设置菜单增加主题切换

修改 `web/src/components/workspace/WorkspaceSessionSidebar.vue`：

- 复用现有 settings dropdown 结构。
- 在语言切换 `DropdownMenuSub` 旁增加主题切换 `DropdownMenuSub`。
- 使用 `DropdownMenuRadioGroup` / `DropdownMenuRadioItem`，结构参考语言切换。
- 引入主题 store 中的 `themes` / `AppTheme` / `setTheme`。
- 使用图标区分主题项，例如 lucide 的 `Sun` / `Moon`，保持现有 `Languages` 一类的视觉密度。
- 新增 i18n 文案：
  - `common.theme`
  - `theme.light`
  - `theme.dark`
- 菜单本身、hover、focus、选中态需要支持浅色与暗色主题。

### 6. 建立主题 token 与全局基础样式

修改 `web/src/styles.css`：

- 在 `:root` 或 `[data-theme='dark']` / `[data-theme='light']` 定义语义化 CSS variables，例如：
  - `--color-app-bg`
  - `--color-panel-bg`
  - `--color-panel-muted`
  - `--color-border`
  - `--color-text`
  - `--color-text-muted`
  - `--color-control-bg`
  - `--color-control-hover`
  - `--color-terminal-bg`
  - `--color-terminal-fg`
  - `--color-terminal-cursor`
  - `--color-terminal-selection`
- 默认变量值使用暗色，确保无缓存 / 首屏默认 dark。
- `html` / `body` / `#app` 和现有 `.button`、`.dialog-*`、`.toast-*`、`.terminal-*` 等全局 class 改为使用变量，避免浅色主题出现硬编码暗色残留。
- 对 Tailwind class 难以集中替换的 Vue 模板，优先使用 Tailwind arbitrary value 读取变量，例如 `bg-[var(--color-app-bg)]`、`text-[var(--color-text)]`、`border-[var(--color-border)]`。

### 7. 适配主要布局与工作台颜色

按现有组件结构逐步替换硬编码暗色 class：

- `web/src/views/SessionsView.vue`
  - 认证检查态背景 / 文字。
  - 主 `SplitterGroup` 背景 / 文字。
  - resize handle 背景 / hover 颜色。
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
  - sidebar 背景、header、footer、输入框、select、dropdown、树节点、按钮、空状态、hover / focus / selected 状态。
- `web/src/components/session/SessionWorkbench.vue`
  - 工作台背景、tab bar、tab active/inactive、空状态、footer。
- `web/src/components/session/TerminalPane.vue`
  - terminal pane 外层背景、history loading/error/no-history 文案。
- `web/src/components/session/CreateSessionPanel.vue`
  - 新建会话表单背景、边框、输入框、说明文字。
- 视情况适配 `LoginView.vue`、Dialog 组件和 Toast 组件中显著的硬编码暗色。

替换原则：

1. 优先使用语义 token，减少在模板中散落两套颜色。
2. 不做大规模结构重构。
3. 保持现有交互布局、尺寸和信息层级。

### 8. 让 xterm 跟随主题切换

修改 `web/src/components/terminal/useXterm.ts`：

- 抽出 `terminalThemes` 或 `xtermThemeFor(appTheme)`：
  - dark 使用现有接近值。
  - light 使用浅色背景、深色前景、可见光标与选区。
- `terminalOptions(theme: AppTheme)` 接收主题参数。
- `measureXtermSize()` 保持可使用默认 dark 或传入当前主题；因测量只关心尺寸，可以继续使用默认 dark，除非实现更容易传入主题。
- `createXterm(..., theme)` 创建时使用当前主题。
- 在返回的 controller 中增加 `setTheme(theme: AppTheme)`，内部调用 `terminal.options.theme = xtermThemeFor(theme)` 或 xterm 支持的等价更新方式。

修改 `TerminalView.vue` 与 `HistoryTerminalView.vue`：

- 引入 `useThemeStore()`。
- 创建 xterm 时传入当前主题。
- watch 当前主题，调用 `xterm?.setTheme(theme)`。
- live terminal 与 history terminal 都要覆盖。

### 9. 测试与类型覆盖

新增或调整测试：

- `web/src/store/storage.test.ts`
  - 读取存在值。
  - 读取不存在值。
  - 写入值。
  - localStorage 抛错时不让调用方崩溃。
- `web/src/store/theme.test.ts`
  - 无缓存时默认 `dark`。
  - 合法缓存值恢复。
  - 非法缓存值回退 `dark`。
  - `setTheme('light')` / `setTheme('dark')` 会更新状态、写 storage、更新 document class 和 data-theme。
- 如 xterm 主题更新逻辑容易单元测试，可补充纯函数测试；否则依赖 typecheck 与手动验证。

## Files to change

预计修改：

1. `web/src/store/storage.ts`：新增 localStorage 封装。
2. `web/src/store/theme.ts`：新增主题状态、默认值、持久化和 document 应用逻辑。
3. `web/src/main.ts`：应用启动时初始化主题。
4. `web/src/i18n.ts`：语言持久化改用 storage 封装，并补充主题相关文案。
5. `web/src/styles.css`：新增主题 token，并迁移全局 class 到 token。
6. `web/src/components/workspace/WorkspaceSessionSidebar.vue`：左下角设置菜单增加主题切换，并适配菜单主题样式。
7. `web/src/views/SessionsView.vue`：主布局背景、文字、分割条适配主题。
8. `web/src/components/session/SessionWorkbench.vue`：工作台、tab、footer 适配主题。
9. `web/src/components/session/TerminalPane.vue`：终端 pane 外围状态适配主题。
10. `web/src/components/session/CreateSessionPanel.vue`：新建会话表单适配主题。
11. `web/src/components/terminal/useXterm.ts`：xterm theme 参数化和运行时更新。
12. `web/src/components/terminal/TerminalView.vue`：live terminal 跟随主题。
13. `web/src/components/terminal/HistoryTerminalView.vue`：history terminal 跟随主题。
14. `web/src/store/storage.test.ts`：新增 storage 测试。
15. `web/src/store/theme.test.ts`：新增 theme store 测试。

可能修改：

1. `web/src/views/LoginView.vue`：如果登录页存在硬编码暗色，需要纳入主题适配。
2. `web/src/components/session/*Dialog.vue`、`ToastHost.vue`：如果组件模板中存在硬编码主题色，按验收标准适配。
3. `web/src/views/SettingsView.vue`：当前是空页面；本次主要入口在左下角菜单，除非实现阶段发现路由仍暴露明显空白且影响体验，否则不扩展完整设置页。

## Verification plan

实现完成后进入 Verification / 验证阶段时执行：

1. 静态检查与构建：
   - `cd web && npm run typecheck`
   - `cd web && npm run lint`
   - `cd web && npm run test`
   - `cd web && npm run build`
2. 手动或运行态验证：
   - 启动 Web UI 后默认主题为 dark。
   - 左下角设置入口可打开。
   - 主题菜单中只有 light / dark，没有 system。
   - 切换 light 后主布局、sidebar、菜单、dialog/toast、工作台、表单、按钮、输入框可读。
   - 切换 dark 后恢复暗色外观。
   - 刷新页面后恢复最后选择的主题。
   - localStorage 中存在 `termbridge.theme`，值为 `light` 或 `dark`。
   - localStorage 被写入非法值时，刷新后回退 dark。
   - live terminal 与 history terminal 的 xterm 背景、前景、光标、选区跟随主题切换。
   - 终端连接、历史回放、resize、输入输出行为不因主题切换中断。
3. diff 范围核对：
   - 确认没有修改后端协议、会话核心逻辑、认证流程。
   - 确认没有引入新的 UI 框架或主题库。

## Assumptions

1. `reka-ui` 当前 dropdown/select/tabs 使用方式可以继续复用，不需要新增依赖。
2. `tailwind css v4` 支持当前项目的 arbitrary value class 与 CSS variables 组合。
3. xterm 6 的 `terminal.options.theme` 运行时更新可以即时刷新终端颜色；若实际不可用，实现阶段应改用 xterm 官方支持的替代方式并记录偏差。
4. 主题持久化只需浏览器本地缓存，不需要同步到后端或设备配置。

## Risks

1. 当前模板中硬编码暗色 class 较多，若只局部替换，浅色主题可能出现低对比或暗色残留。
2. xterm 内部样式与容器样式分离，若只改 CSS 不改 xterm options，终端内部不会真正跟随主题。
3. 主题初始化如果晚于首屏渲染，可能出现短暂闪烁；实现时应尽量在 mount 前应用 document theme。
4. Dialog / Toast / Dropdown 通过 portal 渲染，必须确保 portal 内容也能读到根节点 theme token。
5. 语言持久化迁移到 storage 封装时，需要保持现有语言切换行为兼容。

## Rollback plan

1. 移除 `web/src/store/theme.ts` 与新增测试。
2. 恢复 `web/src/main.ts`、`web/src/i18n.ts`、`WorkspaceSessionSidebar.vue` 中主题相关变更。
3. 将 `styles.css` 和主要 Vue 组件中的主题 token class 恢复为原硬编码暗色 class。
4. 将 `useXterm.ts`、`TerminalView.vue`、`HistoryTerminalView.vue` 恢复为固定暗色 xterm theme。
5. 如果 storage 封装已被语言切换复用但主题回滚，可保留 storage 封装用于语言，或按实际 diff 一并恢复。

## User review notes

- 2026-06-24：用户确认 Plan，并要求开始实现。
