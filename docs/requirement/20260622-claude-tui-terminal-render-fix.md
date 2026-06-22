# Claude TUI terminal render fix 需求
最后修改时间: 2026-06-22 22:47:22

## Review status

Accepted

## Background

在 Web terminal 中创建命令为 `claude` 的终端会话后，Claude Code 首次安全确认界面可以显示；确认后进入 Claude Code 主 TUI 时界面出现布局错乱。截图表现为输入框、横线、右侧提示和光标位置不稳定。日志显示创建 PTY 时使用 `cols=173 rows=24`，但 attach 日志中的 runtime current size 仍为默认 `80x25`。

相关链路：

- 前端 `POST /api/sessions` 创建 terminal session。
- 后端用请求中的 `cols/rows` 启动 PTY。
- 前端随后通过 `/api/sessions/{id}/ws` attach 到 runtime 并由 xterm.js 渲染。

## Goal

修复 Claude Code 这类全屏 TUI 在 Web terminal 中因终端尺寸状态和换行处理不一致导致的初始渲染错乱风险。

本轮目标采用最小修复：

1. 后端 runtime 的 `current` terminal size 必须与创建 PTY 时的 initial size 保持一致，避免 attach/resize 状态从默认 `80x25` 开始。
2. 前端 xterm 不应启用额外 EOL 转换，尽量原样处理 PTY 输出，降低对全屏 TUI ANSI 重绘的干扰。
3. 创建 session 前的 `cols/rows` 应尽量来自与实际 terminal view 同样结构的 xterm FitAddon 测量结果，而不是仅使用 `window.innerWidth/innerHeight` 估算。
4. 如果 xterm 首次 fit/resize 发生在 WebSocket `OPEN` 之前，前端必须缓存并在连接建立后补发 resize control，不能静默丢弃。

## Non-goal

- 不重构 session 创建流程为“先 mount xterm 再创建 PTY”；允许在创建表单阶段用隐藏 xterm 进行同结构尺寸测量。
- 不新增专门的 Claude Code 集成逻辑或命令识别分支。
- 不调整 Web terminal 整体 UI 布局、tab/sidebar 或 gateway attach-only 行为。
- 不处理历史 session 的已保存输出修复。

## User scenarios

1. 用户在 Web UI 中创建命令为 `claude` 的 session，确认安全提示后，Claude Code 主 TUI 应尽量稳定显示，不因后端 runtime 记录为 `80x25` 而跳过必要 resize。
2. 用户打开 devtools 或改变浏览器尺寸后，xterm fit 触发的 resize 应基于 runtime 已知的真实初始尺寸进行比较和发送。
3. 普通 shell 输出和 stopped history replay 不应因本次修复出现额外 CR/LF 转换副作用。

## Acceptance

- 创建 session 时，后端创建 `SessionRuntime` 使用 PTY 启动时的 initial terminal size 初始化 `current`。
- 日志中 attach 时的 `current_cols/current_rows` 不再固定为 `80/25`，而应与创建时请求尺寸一致，直到浏览器 resize 更新它。
- xterm 配置不再设置 `convertEol: true`。
- 前端创建请求日志中的 `cols/rows` 应与随后 live xterm `xterm.resize` 日志一致或接近一致；若 xterm 首次 resize 在 socket 打开前发生，应看到连接打开后补发 resize control。
- 后端应能收到首个真实 terminal resize，或创建请求本身已经使用同结构测量出的真实尺寸。
- 相关 Go / TypeScript 代码能够通过项目已有格式/类型检查或至少通过针对性测试/构建检查。

## Open questions

- 当前前端仍在创建 session 前用 viewport 估算 `cols/rows`，不是 xterm FitAddon 的真实结果；如果最小修复后仍有错乱，后续需要设计“两阶段创建”或“attach 后首个 resize 再放行 TUI”的方案。
- 需要人工在真实浏览器中再次运行 `claude` 验证 TUI 交互显示；自动化测试很难完全覆盖全屏 TUI 渲染。

## Decisions

- 本次按轻量模式 / light 处理，先记录问题和最小修复。
- 需求阶段随用户“记录问题，实施修复”要求视为接受，进入实现阶段。

## Risk

- 关闭 `convertEol` 可能改变少数纯 `\n` 输出的显示行为；但对于真实 PTY/xterm 桥接，这是更接近终端原始行为的配置。
- 后端 runtime size 修复只能解决状态一致性和 resize 去重风险，不能完全消除“创建前尺寸估算不准确”的问题。
