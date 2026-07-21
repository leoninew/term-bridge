# 手机端 xterm 历史触屏滚动

最后修改时间: 2026-07-21 13:04:59

Review status: Accepted

## Mode

light / 轻量模式

## Background

会话终端使用 xterm.js 渲染 live / history 输出，`scrollback` 最大约 5000 行。桌面端可用滚轮或拖动滚动条查看历史；手机端在历史较长时，用户反馈**无法用手指上下滑动**查看 scrollback。

当前仓库已有相关提交（均在 `develop`）：

1. `d8d6453`：移动端视口 `100svh` / safe-area，避免浏览器 chrome 裁切工作区（含 tab 半高类问题）。
2. `4d0c619`：终端触屏辅助
   - CSS：`.xterm` / `.xterm-viewport` 设置 `touch-action: pan-y` 与 `-webkit-overflow-scrolling: touch`
   - UI：`TerminalScrollFabs` 按滚动边缘显示「到顶 / 到底」按钮
   - API：`useXterm` 暴露 `scrollToTop` / `scrollToBottom` 与 scroll edge 监听

上述能力**不能可靠替代**中间历史的自由指滑：xterm 使用 canvas + 自管 viewport，触点常被屏幕层/焦点层吞掉，仅靠 `pan-y` 在多数移动浏览器上仍滑不动。FAB 只解决跳到顶/底，不覆盖「逐段浏览」场景。

本任务聚焦：**手机端长历史下的可滑动 scrollback**，前端范围。

## Goal

1. 在手机（窄屏 / 触屏）上，对 **live 终端** 与 **history 终端**，用户可用手指纵向滑动浏览 xterm scrollback 缓冲区。
2. 滑动应映射到 xterm 缓冲区滚动（如 `scrollLines` / viewport 偏移），而不是仅依赖不可靠的原生 DOM 滚动。
3. 保留现有边缘 FAB（到顶 / 到底）作为补充，不作为唯一手段。
4. 滑动过程中不应误触发大量无关输入（例如把竖直滑动当成选区或把触点写成键盘输入）；水平轻移不应用来抢占纵向滚动。
5. 改动范围限制在前端终端组件 / 样式（如 `useXterm`、`TerminalView`、`HistoryTerminalView`、必要时 `styles.css`），不改 agent 历史协议或后端。

## Non-goal

1. 不改 `scrollback` 容量策略（仍为现有上限，例如 5000）。
2. 不引入完整触摸键盘/手势系统（双指缩放、选择文本增强等）。
3. 不重做 PC 鼠标滚轮/滚动条行为（除非为统一入口必须共享少量抽象）。
4. 不修改会话关闭、tab 生命周期、keep-alive 或 WebSocket 回放逻辑。
5. 不在本任务处理「半高 tab」以外的布局问题（视口修复已合入，若仍复现另开任务）。

## User scenarios

### Scenario 1：Live 会话长输出

用户在手机上打开进行中会话，终端已刷出多屏输出。用手指在终端区域上滑 / 下滑，可连续查看 scrollback 中更早 / 更新的内容，不必只能点 FAB。

### Scenario 2：已停止会话历史

用户打开 stopped/failed 会话的 history 终端，同样可用指滑浏览历史文本；只读终端不得因触屏滚动意外进入「可输入」态。

### Scenario 3：与 FAB 共存

当不在顶部时显示「到顶」，不在底部时显示「到底」；指滑改变位置后边缘状态正确更新；FAB 点击仍可跳到顶/底。

### Scenario 4：桌面

使用鼠标滚轮 / 触控板时行为与现状一致；不因触屏逻辑破坏桌面交互。

## Acceptance

1. 在真实或模拟手机触屏环境下，live 与 history 终端均能用单指纵向滑动浏览长 scrollback。
2. 实现优先使用 xterm 缓冲区滚动 API（或等价可靠路径），不把「仅 CSS `touch-action: pan-y`」当作完成标准。
3. 现有 `TerminalScrollFabs` 到顶 / 到底与 edge 检测在滑动后仍正确。
4. 指滑不应向 live 会话发送可感知的错误输入数据（不以滑动产生 stdin 字符为准）。
5. History（只读）终端可滑动，且仍 `disableStdin` / 无光标输入。
6. 桌面滚轮滚动回归通过（手工或既有自动化能力范围内）。
7. 改动文件集中在前端终端相关模块；无后端 / proto 变更。
8. 目标浏览器：iOS Safari、Android Chrome、Android Edge（真机或等价模拟）；不要求覆盖其他浏览器，但实现不得故意只绑定单一引擎私有 API。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

1. 流程模式：light / 轻量模式（Requirement → Implementation → Verification）。
2. 问题定性：既有 `pan-y` + FAB **不充分**；本任务补齐自由指滑 scrollback。
3. 范围：前端 only；live + history 一并覆盖。
4. 与 `6f348740` 无关（该提交无 tab/xterm 触屏实质改动）；相关基线为 `4d0c619` 与 `d8d6453`。
5. **启用条件**（用户采纳）：在 coarse pointer 或具备 touch 能力的设备上启用自定义触屏滚动；桌面精细指针不挂该路径，避免影响 trackpad/鼠标语义。
6. **文本选择**（用户采纳）：本轮滚动优先；长按选择维持浏览器/xterm 默认，若与拖动冲突则以滚动为准。
7. **验收浏览器**（用户确认）：支持并验收 **iOS Safari、Android Chrome、Android Edge**；不强制扩展至其他浏览器。

## Risk

1. xterm 版本与内部 DOM 结构变化可能导致挂载点（viewport vs screen/canvas）选择失效，需集中封装并在 open 后绑定。
2. 触屏滚动与 focus、selection、链接点击（WebLinksAddon）可能冲突，需控制 touch 捕获阈值与阈值。
3. CI 难以完整模拟真机触屏手感；验证依赖 iOS Safari / Android Chrome / Android Edge 真机或模拟步骤，并在 verification 中写明。
4. 若仅修 CSS 再次「看起来像能滚」但 canvas 仍吞事件，会重复失败；验收必须以实际指滑浏览中间历史为准。
5. Android Edge 与 Chrome 同内核（Chromium）时行为通常接近，但 UA/手势细节仍可能不同，验收时至少各走一遍指滑路径。
