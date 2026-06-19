# Web Terminal xterm.js 集成治理需求
最后修改时间: 2026-06-19 19:08:00

Review status: Accepted

## Background

当前项目已经具备本地 Web terminal 原型：前端使用 Vue + xterm.js，通过 WebSocket 与 Go 后端 relay 通信；后端通过 `webterminal.Registry` / `SessionRuntime` 管理 PTY session、WebSocket client 和 bounded history。

近期代码审视显示，当前 Web terminal 已能跑通基本链路，但整体仍停留在 prototype 阶段。问题集中在以下方面：

1. xterm.js 组件只是完成基础初始化和输入输出桥接，尚未形成稳定的 terminal 产品语义。
2. WebSocket 协议区分 text control frame 与 binary PTY frame，但缺少可靠重连、回放、seq、ack 和明确 close 语义。
3. 后端 outbound queue、WebSocket writer、history writer 与 PTY read loop 的关系仍较粗糙，大输出和慢客户端场景容易暴露断开、丢输出或性能问题。
4. 安全边界偏弱，尤其是 WebSocket Origin 校验、REST/WS 鉴权、任意 command/cwd 执行面和环境变量继承。
5. 前端 UX 还没有覆盖真实 terminal 场景所需的 attach/reconnect、history replay、错误展示、close/detach 确认、xterm write 背压和输入编码验证。

本需求的目标不是继续把 Web terminal 当作临时 spike 修补，而是把已暴露的问题转换成可审查、可设计、可分阶段实现的治理需求，为后续 Spec / Plan / Implementation 提供边界。

## Goal

本次严格模式需求目标是治理当前 Web terminal / xterm.js 集成，使其从“可跑通原型”演进为“本地可用、边界清晰、可向未来 Gateway 复用”的 terminal 基础能力。

具体目标：

1. 明确 Web terminal 的安全边界，避免本地 terminal API 在无认证、无 Origin 限制或非 loopback 暴露时形成远程命令执行面。
2. 明确 WebSocket terminal 协议的生命周期语义，包括 attach、hello、resize、detach、close、error、exited、disconnect 和 reconnect。
3. 修复或重新设计 WebSocket 单连接写入模型，避免多 goroutine 并发写同一个 WebSocket connection。
4. 为 running session 的 attach/reconnect 提供明确策略：要么实现 bounded history replay / seq replay，要么显式声明不支持可靠续传并移除误导性协议字段。
5. 改善大量输出与慢客户端处理，避免无提示 detach、无界内存增长、不可恢复丢输出或 history writer 严重拖慢实时输出。
6. 改善 xterm.js 前端集成，包括初始化尺寸、resize、输入编码、paste/IME/快捷键行为、输出 write queue、错误展示和 stopped session history replay。
7. 建立可验证的验收标准和测试方向，覆盖安全、协议、输入输出、resize、大输出、慢客户端、重连和 history replay。

## Non-goal

本次治理不包括：

1. 不实现完整 Gateway 产品化。
2. 不实现公网访问、反向隧道、跨设备路由、Device Registry 或 User model。
3. 不引入完整多用户权限系统。
4. 不把当前 Web UI 提升为正式云端产品入口。
5. 不承诺完成所有 M5 级长期压力、复杂进程树 cleanup 或生产级 terminal multiplexing。
6. 不重选前端技术栈；默认继续使用当前 Vue + TypeScript + xterm.js 路线。
7. 不在 Requirement 阶段决定所有实现细节；具体协议格式、buffer 策略、token 机制和测试命令在 Spec / Plan 阶段设计。
8. 不执行 `git add`、`git commit`、`git push` 或其他 git 写操作。

## User scenarios

### 场景一：维护者本地启动 Web terminal

作为维护者，我通过本地命令启动 Web terminal，并在浏览器中打开页面。

期望：

- 默认只暴露在安全本地边界内。
- 如果配置为非 loopback 监听，系统应有明确安全保护或拒绝不安全启动。
- REST 与 WebSocket 不应在无认证情况下被任意网页或局域网客户端滥用。
- 页面可以创建 session、attach terminal、输入命令、查看输出并关闭 session。

### 场景二：用户通过 xterm.js 交互 shell/TUI

作为用户，我在浏览器 terminal 中运行 shell、Claude Code、Codex 或其他 TUI。

期望：

- ANSI、颜色、box drawing、光标移动、清屏和常见 TUI 布局能被 xterm.js 正确展示。
- 普通键盘输入、Enter、Backspace、Tab、方向键、Ctrl+C、复制粘贴和初步 IME 行为可用或有明确限制记录。
- 浏览器 resize 后，PTY size 与 TUI 布局能及时更新。
- 初始 PTY size 不应长期依赖固定常量导致启动时布局错误。

### 场景三：用户刷新页面或重新 attach running session

作为用户，我刷新浏览器、切换 session 或在 WebSocket 断开后重新 attach 一个仍在运行的 session。

期望：

- 系统明确支持何种程度的恢复：history snapshot、bounded replay、seq replay 或仅实时 attach。
- 如果支持 replay，用户应看到断开前后的合理上下文。
- 如果不支持可靠续传，协议和 UI 不应暗示支持 `last_seq` 或完整恢复。
- reconnect 过程中不应误杀 PTY，也不应静默丢失关键状态而无提示。

### 场景四：用户运行大量输出命令

作为用户，我运行会持续输出大量文本的命令，例如构建、测试、日志 tail 或生成大段文本。

期望：

- 后端 PTY reader 不应被单个慢客户端无限拖住。
- WebSocket outbound queue 不应无提示满后直接让用户失去上下文。
- 前端 xterm.js 不应因为未限制 write queue 而无限积压。
- history 持久化不应每个 chunk 都高成本重写完整文件并显著影响实时输出。
- 如果发生截断、丢弃或 detach，用户应看到明确原因和可恢复路径。

### 场景五：用户点击 detach 或 close session

作为用户，我希望 detach 与 close 有明确且可靠的语义。

期望：

- detach 表示断开当前 WebSocket client，但不关闭 PTY session。
- close 表示请求关闭 session / PTY，并应可靠到达服务端。
- 前端不应在发送 close control 后立即关闭 socket 而导致 close 消息可能丢失。
- 服务端应给出 ack、state、exited 或可靠 REST close 语义。

### 场景六：维护者审查协议和安全边界

作为维护者，我希望 Web terminal 协议和 API 边界清晰、可测试、可向未来 Gateway 演进。

期望：

- control message schema 有明确字段校验。
- binary frame size 在读取层受到限制，而不是读入内存后才拒绝。
- WebSocket 所有写操作串行化。
- 错误消息不会通过 terminal escape sequence 混淆用户界面。
- session id、attach token、API token、Origin allowlist 等安全机制在 Spec 阶段明确取舍。

## Acceptance

Requirement / Spec / Plan 完成后应满足：

- [ ] 存在本需求文档，且 Review status 在用户接受后更新为 `Accepted`。
- [ ] 本需求明确当前流程为 strict / 严格模式，并遵守 Requirement → Spec → Plan → Implementation → Verification。
- [ ] 需求明确安全边界是第一优先级，包括 Origin 校验、REST/WS 鉴权、非 loopback 暴露、command/cwd 执行面和环境变量继承。
- [ ] 需求明确 WebSocket terminal 协议需要定义 attach、hello、resize、detach、close、error、exited、disconnect、reconnect 和 replay 语义。
- [ ] 需求明确需要修复 WebSocket 并发写风险。
- [ ] 需求明确 running session attach/reconnect 必须选择并实现或明确放弃 history/seq replay 语义。
- [ ] 需求明确大量输出、慢客户端、queue full、history flush 和 xterm write backlog 是同一类端到端背压问题，需要统一设计。
- [ ] 需求明确 xterm.js 集成需要覆盖输入编码、paste/IME、resize、初始尺寸、错误展示和 stopped session history replay。
- [ ] 需求明确哪些事项进入 Spec 阶段设计，哪些可留作后续 M5/Gateway hardening。

实现完成后应满足的初步验收方向：

- [ ] WebSocket Accept 不再默认跳过 Origin 校验，或仅在明确安全条件下允许。
- [ ] REST 与 WebSocket 至少具备本地 token / 同源保护 / session attach token 中的一种明确保护机制。
- [ ] 非 loopback 监听不会在无保护情况下暴露任意 command/cwd 执行能力。
- [ ] WebSocket connection 写入路径为单 writer 或有明确同步保护。
- [ ] close session 不再依赖前端 `send()` 后立即 `close()` 的不可靠行为。
- [ ] running session attach 后的 terminal 上下文策略明确，并在 UI/协议中表现一致。
- [ ] 如果保留 `last_seq`，则后端实现对应 seq/replay；如果不实现，则移除或隐藏相关协议字段。
- [ ] client queue full 时用户能获得明确错误或重连恢复路径，而不是静默失去输出上下文。
- [ ] 大输出场景不会造成明显无界内存增长或 history 每 chunk 全量刷盘造成的严重退化。
- [ ] xterm.js 输入、resize、输出写入和 stopped history 展示有对应测试或手工验证记录。
- [ ] 相关 Go 测试、前端类型检查/构建和项目约定检查通过。
- [ ] 不执行 git 写操作。

## Open questions

暂无需要用户确认的未决事项。所有已知取舍已收敛到 Decisions，并将在 Spec / Plan 中转化为实施步骤。

## Decisions

- 当前流程采用 strict / 严格模式。
- Requirement 已接受，进入 Spec / 规格阶段。
- 本需求优先处理 Web terminal/xterm.js 集成的基础语义、安全边界和稳定性问题，而不是扩展产品功能。
- 当前前端技术栈默认沿用 Vue + TypeScript + xterm.js，不重新选型。
- 安全边界作为本轮治理的第一优先级。
- reconnect/replay 纳入本轮核心治理范围，但短期优先实现 attach-time bounded history replay，不要求一次性实现完整 seq replay。
- `last_seq` 短期移除或隐藏，避免协议误导；未来真正实现 seq replay 时再恢复。
- stopped session history 应增加 readonly xterm replay；raw text 可作为辅助视图或诊断视图保留。
- 本轮移除临时 token/bootstrap/auth 机制；后续认证交给正式用户体系实现。
- 本轮保留 Origin allowlist 作为浏览器来源边界。
- cwd 默认限制在启动 cwd 或配置 allowlist 内。
- command 在本地 Origin/cwd 边界下保留工具所需灵活性；非 loopback 场景后续必须由正式用户体系和部署策略收敛。
- env policy 进入 Spec 阶段设计，至少考虑可配置 denylist。

## Risk

1. 如果先修 UX 而不修安全边界，后续一旦服务监听非 loopback，风险会快速放大。
2. 如果不先统一 WebSocket writer 模型，后续新增 ack/replay/error control 可能继续扩大并发写风险。
3. 如果继续保留未实现的 `last_seq`，协议会误导后续实现和测试。
4. 如果只扩大 queue 而不设计 replay/backpressure，大输出问题会从断开转化为内存积压。
5. 如果 stopped history 继续用 `<pre>` 展示，无法真实验证 ANSI/TUI 行为。
6. 如果 close/detach 语义不可靠，用户可能误以为 session 已关闭，但后台 PTY 仍在运行。
7. 如果 command/cwd/env policy 未明确，本地 Web terminal 会长期背负安全债务。

## User review notes

用户要求不要保留尾巴。Requirement 已接受，关键取舍已收敛到 Decisions，暂无需要用户确认的未决事项。
