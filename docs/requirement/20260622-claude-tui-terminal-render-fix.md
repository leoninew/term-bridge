# Claude TUI terminal render fix 需求
最后修改时间: 2026-06-23 08:26:00

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

## Verification plan

本修复的验证分层执行，避免把真实 Claude Code / Codex TUI、浏览器截图和压力场景放入默认 `just check`，同时尽量用自动化覆盖可稳定断言的边界。

### 单元测试 / Unit tests

适合覆盖可稳定复现的尺寸状态和 socket 时序：

1. Go 单元测试覆盖 `SessionRuntime` 创建时的 `current` terminal size 与 PTY initial size 一致。
2. Go 单元测试覆盖 attach / resize 状态不会从默认 `80x25` 重新开始。
3. 前端 Vitest 覆盖 WebSocket `OPEN` 前产生的 resize 会被缓存，并在连接建立后按 `hello` 后补发。
4. 前端 Vitest 覆盖多次 pending resize 只保留最后一次，避免把过期尺寸补发给后端。

不适合用单元测试直接断言真实 xterm / FitAddon 的浏览器布局测量结果；这部分依赖 DOM layout、字体指标和浏览器环境，应由浏览器自动化或人工验证覆盖。

### Pomelo PW / 浏览器自动化

适合覆盖真实浏览器、xterm DOM、截图和 Gateway/local Web attach 行为：

1. Local Web flow：创建 terminal session，确认页面可加载、session 可创建、history 可读、resize 后页面仍可截图。
2. Claude TUI 手动辅助 flow：在 `just serve` + `just web` 已运行时创建 `claude` session，保留截图或诊断日志，确认初始 TUI 尺寸稳定。
3. Gateway running session flow：当存在 running session 时，通过 Gateway attach、发送输入 marker、resize、detach，并截图记录。
4. Pomelo PW 不作为真实 Claude/Codex TUI 视觉正确性的唯一验收来源；真实全屏 TUI 仍需要人工确认。

### Go 集成测试 / Integration tests

适合覆盖不依赖真实浏览器和真实 Claude 账号的端到端逻辑：

1. 使用 fake TUI 或受控长运行命令模拟全屏程序，验证 session create、attach、resize、detach 和 history。
2. Gateway terminal relay 集成测试覆盖 Browser WS control message 不被误当作 terminal input。
3. Gateway terminal relay 集成测试覆盖 binary input/output、single writer、detach 和 missing session error。
4. Agent/Gateway reconnect 与 stale route cleanup 应作为 M6/M7 后续专项集成测试，不阻塞本 Claude TUI 修复收口。

### Bash 脚本 / Shell orchestration

适合作为验证流程胶水，而不是复杂断言主体：

1. 检查 `just serve` / `just web` 是否已运行或提示用户启动。
2. 创建一个 guaranteed running session 作为 Pomelo PW Gateway attach 前置条件。
3. 串联执行 targeted Go tests、frontend Vitest、Pomelo PW flow。
4. 不建议用 Bash 承载复杂 JSON 状态判断、并发压力统计或 WebSocket 断言。
5. 不修改 `justfile`；如为验证临时使用 just 命令或临时脚本，只作为本地执行入口，不把不适合进入主线的验证胶水提交到仓库。

### Python uv 集成测试 / Black-box scripts

适合后续压力和黑盒验证，不作为本修复收口的硬性前置。用户已确认可以接受使用 Python `uv` 进行集成测试。

1. 20+ sessions 压力：批量创建 session、轮询 state/history、统计失败项。
2. Gateway API 黑盒验证：device/session/history relay、running session attach 前置准备。
3. Windows process cleanup 黑盒验证：stop/close 后结合系统进程查询确认无残留。
4. 如果需要 WebSocket black-box attach，Python `uv` 脚本可以显式声明临时依赖；不需要进入主线的脚本、虚拟环境、输出文件或测试业务代码必须在验证后清理。
5. 如果某个验证能稳定沉淀为长期检查，再单独评估是否进入 `tools/`、`.pomelo-pw/` 或 Go / frontend test；默认不改 `justfile`。

### 临时验证代码清理 / Cleanup policy

1. 为验证临时修改业务代码、加入调试 hook、插入测试命令、创建临时脚本或生成测试产物时，如果这些内容不适合进入主线，验证完成后必须还原或删除。
2. 不使用 `git checkout` / `git reset` 等 git 写操作清理；清理应通过明确的文件编辑、删除临时文件或恢复原内容完成。
3. 临时测试产物、Pomelo 输出目录、Python `uv` 缓存/虚拟环境、日志和截图如果不作为 verification evidence 保留，应在完成后清理。
4. 如果发现某个临时测试能力值得长期保留，应先说明理由和主线归属，再按正式实现处理，而不是以临时验证名义混入。

### 人工验证 / Manual verification

真实 Claude Code TUI 的最终体验仍需要人工确认：

1. 在当前浏览器中通过 Web UI 创建 `claude` session。
2. 确认安全提示、主 TUI、输入框、横向分隔线、右侧提示和光标位置稳定。
3. 改变浏览器尺寸，确认 resize 后布局仍可用。
4. detach / reattach 后确认输出和 history 正常。
5. 本轮已由用户在 2026-06-23 确认 Claude TUI 当前可以工作，因此本修复可收口；Codex TUI 和 Gateway attach running session 继续作为后续验证项。

## Open questions

当前无阻塞本修复收口的问题。

已关闭问题：

- 创建 session 前的 `cols/rows` 已改为优先使用同结构 xterm / FitAddon 测量结果，不再只依赖 viewport 估算。
- 用户已在 2026-06-23 确认 Claude TUI 当前可以工作，本修复可以进入轻量验证收口。

仍需作为后续 M6/M7 验证跟踪的问题：

- Gateway attach running session 分支仍需专项自动化或人工验证。
- Codex TUI、20+ sessions、Agent reconnect 和 Windows process cleanup 不属于本修复收口范围。

## Decisions

- 本次按轻量模式 / light 处理，先记录问题和最小修复。
- 需求阶段随用户“记录问题，实施修复”要求视为接受，进入实现阶段。
- 用户已确认 Claude TUI 已经可以工作，本修复按文档收口处理。
- 用户明确要求不删除 Air / `.air.toml`；当前 `just serve` 不依赖 Air，但 Air 作为历史/可选资产保留。
- 用户确认 `just serve` + `just web` 已在运行中；本次收口不接管、不重启运行中的服务。
- 验证计划采用分层策略：单元测试覆盖可稳定断言的时序/状态，Pomelo PW 覆盖真实浏览器路径，Go 集成测试覆盖 relay 和 fake TUI，Bash/Python uv 作为 targeted verification 胶水和压力脚本，真实 Claude TUI 体验保留人工确认。
- 不修改 `justfile`；为了验证临时使用的脚本、业务代码改动、输出文件或 Python `uv` 环境，如果不适合进入主线，使用完必须清理。

## Risk

- 关闭 `convertEol` 可能改变少数纯 `\n` 输出的显示行为；但对于真实 PTY/xterm 桥接，这是更接近终端原始行为的配置。
- Claude TUI 可工作结论来自用户当前人工确认；本次文档收口不新增截图、日志或自动化验证。
- Gateway terminal attach、Codex TUI、20+ sessions、Agent reconnect 和 Windows process cleanup 仍是后续验证项，不应被本次 Claude TUI 收口覆盖。
