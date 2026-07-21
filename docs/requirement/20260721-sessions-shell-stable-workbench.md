# 会话壳布局切换时保持终端 Workbench 实例

最后修改时间: 2026-07-21 21:27:14

Review status: Accepted

## Flow mode / Stage

轻量模式 / light；需求 / Requirement（Accepted）；用户要求开始实现

## Background

`SessionsPageShell` 在 `isNarrow`（`max-width: 768px`）下使用移动 overlay 侧栏布局，宽屏使用 `SplitterGroup` 双栏布局。两套模板各自挂载 `SessionWorkbench`（`v-if` / `v-else`）。

手机横竖屏旋转时，视口宽度常跨过 768 断点，导致：

1. 当前 `SessionWorkbench` 整树销毁，终端 WebSocket 关闭；
2. 新布局再挂载新的 `SessionWorkbench` 并重新 attach；
3. 多端控制模型下，旧 controller 断开会把控制权 **自动** 交给仍在线的其它 attach（例如仍开着的电脑浏览器）；手机重连变为 observer，再次提示「接管会话」。

多设备控制需求（`20260720-multi-device-terminal-control`）已明确：observer 因旋转、分屏等触发的 **本地 layout resize** 不应弹接管；当前现象来自 **布局壳 remount 导致重连**，而非角色协议本身要求旋转后重鉴权。

## Goal

1. 窄屏 / 宽屏布局切换时，**同一** `SessionWorkbench` 实例保持挂载，不因 `isNarrow` 翻转而销毁重建。
2. 布局切换本身 **不** 主动关闭或重建终端 WebSocket；controller 在连接未断时保持控制权。
3. 旋转或断点变化后，仅允许本地 xterm fit / controller 的 PTY `resize` 路径；不得因壳切换再次出现 observer「接管」提示（在手机连接未真正断开、且用户仍为该连接 controller 的前提下）。
4. 宽屏侧栏拖拽、窄屏 overlay 抽屉与现有交互行为不回退。

## Non-goal

1. 不改后端 multi-attach / controller 选举、`take_control` 协议或 tunnel 语义。
2. 不做 sticky controller / 同设备断线自动认领控制权（重连后的产品策略另案）。
3. 不调整 768px 断点数值，不做断点滞回（可后续增强）。
4. 不改 `TerminalView` 输入门禁、keep-alive 配额、touch scroll / FAB 等无关前端逻辑。
5. 不改 Code / Files / Git workbench 布局。

## User scenarios

1. 用户在手机上 attach 并已接管某 session 的控制权；仅旋转屏幕（宽度跨过或不跨过 768），终端连接保持，**不再**提示需要接管；可继续输入。
2. 同一 session 在电脑上仍以 observer 打开时：手机旋转导致的 **纯布局切换** 不使手机断开，因此电脑 **不会** 因手机 detatch 被 auto-grant 为 controller。
3. 用户在窄屏打开侧栏抽屉、选择 session 后抽屉关闭；行为与现网一致。
4. 用户在宽屏拖拽侧栏宽度、折叠/展开侧栏；行为与现网一致。
5. 窄屏 ↔ 宽屏切换时侧栏形态可从 overlay 变为 in-flow（或反之），仅侧栏 chrome 变化，终端 workbench 不重建。

## Acceptance

- [ ] `SessionsPageShell` 中 `SessionWorkbench` 在 DOM 树中仅有 **一处** 稳定挂载点；`isNarrow` 变化不触发该组件卸载/重挂载。
- [ ] 布局切换后，已连接的 live terminal WebSocket **不** 因壳切换而 close + 新建（可通过前端诊断日志或 DevTools 验证）。
- [ ] 手机作为 controller 时，跨 768 的横竖屏旋转 **不** 再显示 observer 只读/接管提示（连接未因其它原因断开时）。
- [ ] 窄屏：overlay 侧栏、backdrop、选 session 后收起侧栏仍可用。
- [ ] 宽屏：Splitter 侧栏、resize handle、折叠侧栏后 workbench 全宽仍可用。
- [ ] 桌面主路径与移动主路径无功能回退；无后端协议变更。

## Open questions

不适用。暂无需要用户确认的未决事项。

## Decisions

1. 采用「稳定 Workbench 实例」方案：侧栏随窄/宽切换呈现方式，终端 workbench 不随分支销毁。
2. 不在本需求引入 sticky controller 或断点滞回。
3. 继续沿用 `max-width: 768px` 与现有 `useSessionsLayoutMode`。
4. 实现范围优先落在 `web/src/components/session/SessionsPageShell.vue`（必要时微调相关样式类）；不扩散到 agent/cloud 控制权状态机。
5. **真正 WebSocket 断开后** 走既有 multi-attach 控制权重选（含 auto-grant）；这是正确行为，本需求不改变。本需求只消除「布局壳 remount 误杀连接」这一路径。
6. 实现细节（实现者自行兜底，不需产品二选一）：
   - 优先：单一 `SessionWorkbench` 放在稳定宿主（可仍在终端侧 `SplitterPanel` 内）。
   - 若实测发现 reka-ui 在侧栏 panel 增减时仍会把终端 panel 整棵拆掉重建，则把 workbench **提到 Splitter 外**，宽屏侧栏单独用 splitter/固定宽。成功标准只看「实例不销毁、WS 不因壳切换重连」。

## Risk

1. reka-ui Splitter 在 panel 增减时的内部行为需实现时实测；有 Decision 6 兜底。
2. 宽/窄切换瞬间侧栏与 workbench 的 flex 尺寸抖动可能导致一次 xterm fit/resize；这是预期内的尺寸同步，不应伴随 WS 重连。
3. 旋转时若系统/浏览器本身断开 WebSocket，会触发控制权重选；用户确认可接受，不在本需求修复范围。

## User review notes

- 用户反馈：手机接管会话控制权后，仅旋转屏幕又提示需要接管。
- 根因分析：`isNarrow` 双分支各挂 `SessionWorkbench`，旋转跨断点 remount → WS 重连 → 其它端 auto-grant / 本端变 observer。
- 用户确认方向 1 合理：布局切换保持同一 workbench 实例。
- 用户要求：`/specflow light` 记录本任务（本文件）。
- 用户澄清（2026-07-21）：原「事项 1」不是产品选择，而是实现兜底；用户此前未理解。已改写为 Decision 6。
- 用户确认（2026-07-21）：WS 真正断开后重选 controller **没问题**；写入 Decision 5。
- 用户要求开始实现（2026-07-21）；Requirement → Accepted。
- 用户要求开始验证（2026-07-21）。
