# xterm 从其他页面进入 /sessions 布局错位修复验证

最后修改时间: 2026-07-14 17:39:23

Review status: Accepted

## Flow mode / Stage

轻量模式 / light；验证 / Verification。

## Requirement alignment / 需求对齐

- Requirement 文档：`docs/requirement/20260714-xterm-sessions-entry-layout-race.md`
- Requirement status：`Accepted`

对齐结论：实现覆盖需求核心目标——拒绝 Splitter 未稳定时的错误窄尺寸 fit、可信尺寸前不 attach WebSocket、open 后 settle/invalid 重试/`fonts.ready` 再 fit、通过既有 `onResize` 同步后端。用户手测确认“有效”。额外增加 boot overlay，改善等待期 UI 反馈，未超出 non-goal 边界（无协议/架构重写）。

## Spec alignment / 规格对齐

不适用。轻量模式 / light 未创建单独 Spec 文档；按 Requirement / 需求核对。

## Plan alignment / 计划对齐

不适用。轻量模式 / light 未创建 Plan 文档；按 Requirement / 需求默认决策实施。

## Actual diff summary / 实际差异摘要

- `web/src/components/terminal/useXterm.ts`
  - open 后双 rAF + settle 延迟再 fit；invalid-size 退避重试
  - `describeUnsettledLayout`：窄宽/矮高/过小 cols/rows 视为未稳定，不应用、不回调 onResize
  - `document.fonts.ready` 后再 fit
  - ResizeObserver + 几何诊断字段（默认不打 console）
- `web/src/components/terminal/TerminalView.vue`
  - 仅在首次可信尺寸后 attach；attach URL 使用可信 cols/rows
  - boot overlay：准备布局 / 连接 / 接入三阶段文案 + spinner
  - `started`/错误后关闭 overlay
- `web/src/components/terminal/diagnostics.ts`
  - 统一入口 `terminalDebug`，`termbridge.terminalDebug=1` 控制
- `web/src/features/sessions/useTerminalSocket.ts`、`useTerminalSize.ts`、`SessionsPageShell.vue`
  - 诊断调用迁移到 `terminalDebug`
- `web/src/i18n.ts`
  - 新增 `workbench.preparingTerminal` / `connectingTerminal` / `attachingTerminal` 中英文案
- `docs/requirement/20260714-xterm-sessions-entry-layout-race.md`
  - 需求记录

## Expected vs actual changed files / 预期与实际修改文件对比

| 预期 | 实际 |
|------|------|
| `useXterm.ts` open/fit 加固 | 已改 |
| 必要时薄改 `TerminalView.vue` | 已改（可信尺寸 attach + boot overlay） |
| HistoryTerminalView 若共用 open 则受益 | 共用 `createXterm().open`，未单独改文件 |
| 诊断日志可排查 | 已统一 `terminalDebug`，默认关闭 |
| requirement 文档 | 已新增 |
| 不改协议/后端/无关 UI | 满足；未改设置菜单/主题子菜单 |
| i18n 加载文案 | 额外新增，服务 overlay |

说明：`SessionsPageShell.vue` / `i18n.ts` / `useTerminalSize.ts` 在忽略空白时实质改动很小（诊断 rename 与 3 条文案）；全量 diff 含换行风格噪声。

## Acceptance criteria checklist / 验收标准检查清单

- [x] 从其他页面 SPA 进入 `/sessions` 打开 live terminal 时，不应依赖 F12 才能恢复几何布局  
  - 用户确认“有效”；日志曾证明根因是 73px→6×53 attach，现已拦截
- [x] `useXterm.open()` 具备布局 settle 后再 fit（rAF + 延迟）与 invalid/unsettled 重试
- [x] invalid/unsettled 不再“只打日志永久放弃”
- [x] 容器尺寸变化时 ResizeObserver 仍工作；可信尺寸通过 onResize → resize/attach
- [x] History 路径共用 open 加固（未单独回归 History 手测）
- [x] 变更集中在 terminal fit/attach 前端路径与诊断；无协议大改
- [x] 相关前端测试通过；typecheck 失败来自无关 shortcut WIP，非本任务文件

## Test / command results / 测试 / 命令结果

工具链：Vue + TypeScript + Vite（`web/package.json`）。

| 命令 | 结果 |
|------|------|
| `npm run typecheck`（web） | 失败：未暂存的 shortcut WIP（`Shortcut.tags` / `shortcutTags` 等），与本任务文件无关 |
| `npm test`（web） | 通过：14 files / 80 tests |
| `npm run lint`（web） | 通过 |
| 浏览器手测 | 用户确认修复有效；日志分析验证 attach 不再使用 6×53 |

## Missed or expanded scope / 遗漏或扩大的范围

- **扩大（合理）**：boot loading overlay + i18n 三阶段文案；诊断 API 收敛为 `terminalDebug`
- **未做**：HistoryTerminalView 单独手测；E2E 自动化；window/visualViewport 全局 resize 兜底；后端/协议层 attach 尺寸语义重构
- **工作区噪声**：staging 中 `SessionsPageShell.vue`/`i18n.ts` 含换行风格大 diff，实质内容改动小；仓库另有 shortcut 未暂存改动干扰 typecheck

## Risks / 风险

1. 可信尺寸阈值（宽≥160、cols≥20 等）在极端窄窗可能延迟 attach；真实 workbench 最小宽度通常更大。
2. boot overlay 依赖 `started` 关闭；若服务端迟迟不发 started，会较久显示加载态（错误路径会关闭）。
3. 默认关闭 console 后，线上需手动 `localStorage.termbridge.terminalDebug=1` 才能看到诊断。
4. 提交时若包含 `SessionsPageShell`/`i18n` 的换行噪声，会放大 diff；建议 commit 前确认 diff 可读性。

## Incomplete items / 未完成事项

1. 未单独手测 History terminal 视图。
2. 未写针对 unsettled/defer-connect 的自动化单测（xterm DOM 依赖重）。
3. 全仓 `vue-tsc` 当前被无关 shortcut 改动挡住，需在提交 shortcut 或清理 WIP 后重跑绿。

## Conclusion / 结论

轻量模式验证通过。实现满足需求主路径：布局 settle 前拒绝错误 fit、可信尺寸后再 attach、加载反馈完整；用户已确认现象修复。可交付，建议仅提交本任务相关已暂存文件，并注意换行噪声与无关 WIP 分离。
