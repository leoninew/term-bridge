# xterm 从其他页面进入 /sessions 布局错位修复

最后修改时间: 2026-07-14 17:39:23

Review status: Accepted

## Flow mode / Stage

轻量模式 / light；需求 / Requirement 已接受；实现 / Implementation 与验证 / Verification 已完成

## Background / 背景

从其他页面（如 dashboard）SPA 跳转到 `/sessions` 时，xterm.js 终端主界面经常出现布局错位（字符网格与容器不对齐、内容区域偏小/偏大、全屏 TUI 首帧错位等）。用户打开再关闭 F12（DevTools）后布局恢复正常。

排查结论：

1. `TerminalView` 在 `onMounted` 中调用 `createXterm(...).open(container)`。
2. `useXterm.open()` 在 `terminal.open(element)` 后 **同步** 调用 `emitResize()`，再挂 `ResizeObserver`。
3. `/sessions` 布局依赖 reka-ui `SplitterGroup` + 多层 flex（sidebar / workbench / tabs / status bar / terminal-shell）。从其他路由切入时，终端容器在首帧可能仍是中间尺寸（0、偏小或分栏未结算完）。
4. `emitResize()` 在 `proposeDimensions()` 无效时仅记录 `xterm.fit.invalid-size` 并 return，**没有主动重试**。
5. 若首次 fit 得到“非零但错误”的 cols/rows，xterm 内部 canvas 与可能发给后端的 PTY resize 都会按错误尺寸固定；后续若尺寸变化不大，可能不会被纠正。
6. F12 开关会改变 viewport，触发 reflow 与 `ResizeObserver` → `scheduleResize` → 重新 `emitResize`，因此“碰巧修好”。
7. 字体 `Cascadia Mono` 未就绪时 cell 指标也可能偏差，字体加载后若无再 fit 也会错位。

本问题与既有 `docs/solutions/ui-bugs/web-terminal-claude-tui-size-race-2026-06-22*.md` 同属 terminal geometry / 时序 race 家族，但场景聚焦 **路由进入时的容器布局 settle race**，不是 PTY 创建前尺寸测量或 WS OPEN 前 resize 丢失本身。

## Goal / 目标

1. 从其他页面跳入 `/sessions`（及本页内切换到 live terminal tab）时，xterm 在容器布局稳定后能得到正确 `cols/rows`，终端主界面不再长期错位。
2. 首次 `fit` 失败（invalid size）或疑似过早测量时，应主动重试，而不是只依赖偶发的窗口 resize / F12。
3. 布局稳定后的正确尺寸仍通过既有路径同步到后端 PTY（resize control / attach size），避免仅浏览器侧看起来正常而 TUI 仍按旧尺寸布局。
4. 尽量不引入新依赖；优先加固 `web/src/components/terminal/useXterm.ts` 与 `TerminalView.vue` / `HistoryTerminalView.vue` 的 open/fit 生命周期。
5. 保留或增强现有 terminal diagnostic 日志，便于确认 open 时尺寸、invalid-size 与后续纠正 resize。

## Non-goal / 非目标

1. 不重写 Web terminal、WebSocket 协议、Gateway/Agent tunnel 或 PTY 管理架构。
2. 不在本轮解决所有历史 TUI/PTY 尺寸 race（如 create-session 前测量、attach 初始尺寸协议），除非实现时发现与本次 open/fit 加固直接耦合且改动极小。
3. 不改设置菜单、主题/语言子菜单或其他无关 UI。
4. 不引入新的 UI 组件库或替换 xterm.js / FitAddon。
5. 不自动 `git commit` / `push`。
6. 不要求本轮补齐完整 E2E 浏览器自动化（若项目已有可低成本挂接的单测/诊断测可加；否则以代码加固 + 手动验收为主）。

## User scenarios / 用户场景

1. 用户从 dashboard/home 等页面点击进入 `/sessions`，若已有 running session 自动打开 terminal，终端区域在 1 秒内应显示正确布局，无需开关 F12。
2. 用户在 `/sessions` 内从空 workbench 打开 running session tab，终端 open 后布局正确。
3. 用户拖动 sidebar 分栏或调整浏览器窗口后，终端仍能通过既有 `ResizeObserver` 路径正确自适应。
4. 用户运行全屏 TUI（如 Claude Code）时，进入页面后的首屏布局不应因“过早 fit 粘住错误尺寸”而长期错位；若 PTY 已按错误尺寸画过，至少浏览器侧几何纠正后应发出正确 resize。
5. 维护者在浏览器诊断日志中能看到 open 时的 container 尺寸、invalid-size（若发生）以及纠正后的 `xterm.resize`。

## Acceptance / 验收标准

- [ ] 从其他页面 SPA 导航进入 `/sessions` 打开 live terminal 时，xterm 不应依赖 F12/手动 resize 才能恢复正常几何布局。
- [ ] `useXterm.open()` 在首帧 fit 之外，具备布局 settle 后的再 fit 机制（例如 nextTick/rAF/短延迟重试，或 invalid-size 退避重试）。
- [ ] `emitResize()` 遇到 invalid size 时不再“只打日志然后永久放弃”，应在合理次数/时限内重试。
- [ ] 容器真实尺寸变化时，既有 `ResizeObserver` 路径仍工作；纠正后的尺寸会触发既有 `onResize` → 后端 resize（live terminal）。
- [ ] History terminal 若复用同一 `open` 路径，也应受益于相同加固，避免历史回放视图同类错位。
- [ ] 不引入无关 UI/协议大改；变更集中在 terminal fit/resize 前端路径及必要测试/诊断。
- [ ] 相关前端类型检查/既有测试不因本次改动回归（按项目可用命令执行）。

## Open questions / 待定问题

1. 重试策略粒度：
   - 选项 A：仅 `invalid-size` 时退避重试
   - 选项 B：每次 `open` 后固定再 fit 1~N 次（rAF + 50/100/200ms），无论首次是否成功
   - 默认建议：**B + A 并用**——open 后至少再 settle 一次；invalid 时继续退避重试，避免“错误但非零”尺寸粘住。
2. 是否监听 `window.resize` / `visualViewport.resize` 作兜底？
   - 默认建议：本轮可不加全局 window listener，优先容器 `ResizeObserver` + open 后重试；若实现中仍不稳再补。
3. 是否等待 `document.fonts.ready`？
   - 默认建议：open 路径中 `fonts.ready.then(fit)` 作为一次额外纠正，失败忽略。
4. 是否观察 workbench/splitter 外层容器而不只是 terminal div？
   - 默认建议：仍 observe terminal container；open 后重试足以覆盖 splitter settle。若不够再扩大 observe 目标。

## Decisions / 决策

1. 流程采用轻量模式 / light：Requirement → Implementation → Verification。
2. 问题定性为 **前端 xterm open/fit 与路由进入时布局 settle 的时序 race**，不是 F12 本身的问题。
3. 修复落点优先：`web/src/components/terminal/useXterm.ts`，必要时薄改 `TerminalView.vue` / `HistoryTerminalView.vue`。
4. 默认采用 open 后 settle 再 fit + invalid-size 重试 +（可选）`fonts.ready` 再 fit；不重做测量/协议层。
5. 本需求为新建文档，不沿用 `20260625-terminal-resize-hardening` 或 Claude TUI size race 文档正文；可在 Background 引用为关联上下文。

## Risk / 假设

1. 假设：用户描述的“F12 开关即恢复”说明几何 re-measure 足以修复主路径，后端 PTY 错误首帧在多数场景可通过后续 resize 纠正或可接受短暂不正确。
2. 多次 fit/resize 可能增加短时 `resize` control 消息；应沿用现有 `lastCols/lastRows` 去重，避免刷屏。
3. 过密重试可能在容器长期为 0 时产生噪音日志；需限制次数并保留 diagnostic 而非抛错中断。
4. 不同浏览器对 `ResizeObserver` / 字体加载时序略有差异；重试策略应不依赖单一事件。
5. 假设：不需要为了本 bug 改 Splitter 组件本身。

## User review notes / 用户审查记录

- 2026-07-14：用户报告从其他页面跳入 `/sessions` 时 xterm 主界面经常错位，F12 打开再关闭后恢复。
- 2026-07-14：代码审查定位到 `useXterm.open` 同步 fit、invalid-size 无重试、Splitter/flex 首帧尺寸 race 与字体指标风险。
- 2026-07-14：用户要求用 SpecFlow light 记录并开始该任务。
- 2026-07-14：用户要求开始实现 / Implementation；接受默认决策（open 后 settle 再 fit + invalid-size 重试 + fonts.ready 再 fit）。
- 2026-07-14：用户要求开始验证 / Verification；用户手测确认布局修复有效。
