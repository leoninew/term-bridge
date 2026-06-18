# M2.5 Web Terminal Technical Spike 需求
最后修改时间: 2026-06-18 13:28:11

Review status: Accepted

## Background

阅读 `docs/design.md` 与 `docs/plan/20260617-roadmap-refresh.md` 后，当前路线应优先补上 M2.5：Web Terminal Technical Spike，而不是继续推进 M4 Local Product Surface。

`docs/design.md` 已把目标运行链路描述为：

```text
Browser
  ↓
xterm.js
  ↓
WebSocket
  ↓
TermBridge Agent
  ↓
PTY
  ↓
Claude Code / Codex
```

并明确记录：

```text
Web 策略: Gateway 前技术验证，Gateway 后正式产品入口
```

`docs/plan/20260617-roadmap-refresh.md` 的里程碑顺序中，M2.5 排在 M3 / M4 / M5 / M6 之前：

```text
M2: PTY Command Runner MVP
M2.5: Web Terminal Technical Spike
M3: Session / Workspace Runtime Model
M4: Local Product Surface: CLI first, GUI optional container
M5: Runtime Hardening
M6: Gateway Web Terminal MVP
```

虽然当前实现已经推进到 M3 runtime model，但 Web terminal 的基础技术风险尚未被隔离验证。如果继续直接做 M4，本地产品面可能会缺少对未来 Gateway/Web 关键路径的早期反馈。因此 M2.5 应作为当前优先任务补齐。

M2.5 的性质需要更准确地定义为：Gate Web Terminal 的技术前身 / 可演进原型。它不是“做完就丢”的一次性 spike；本次交付的 Web terminal 前端、WebSocket 协议、browser terminal 行为和后端 relay 边界，应该尽量能被后续 Gate 继承、迁移或扩展。

同时，M2.5 仍不是完整 Gateway 产品化：本阶段不做 auth、device routing、reverse tunnel、公网访问和跨设备正式入口；但不应以“只是 dev harness”为理由写出将来需要推倒重来的 Web terminal。

用户已明确 M2.5 Web 前端应基于既有 `D:\SourceCodes\mywork\TermBridge\web` 技术栈，而不是重新选择一套前端方案。已读取该项目 `package.json`，当前技术栈基线为：

```text
Vite
Vue 3
TypeScript
reka-ui
Tailwind CSS
```

相关现有依赖包括 `vue-router`、`pinia`、`@tailwindcss/vite`、`vue-tsc`、`eslint` 和 `prettier`。M2.5 Spec 阶段应优先评估复用该技术栈和项目经验，而不是引入 React、Next.js 或其他 Web framework。

用户进一步明确：不复用原 `D:\SourceCodes\mywork\TermBridge\web` 的既有前端实现，只复用其技术栈和经验。M2.5 前端倾向放入当前仓库 `TermBridge-go/web`，但 Spec 阶段应参考 Go + Web monorepo 的主流实践，确认前端目录、构建输出和 Go 后端集成方式。

## Goal

M2.5 的目标是在 Gateway 前提前交付 Web terminal 相关能力，形成一个本地可运行、面向 Gate 演进的 Web terminal 原型，用来回答：

1. xterm.js 是否能正确显示 TermBridge PTY 输出，包括 shell、Claude Code、Codex 这类 TUI。
2. 浏览器输入是否能可靠传递到 PTY，包括普通键盘输入、Enter、Ctrl+C、复制粘贴、常见快捷键和 IME 初步行为。
3. 浏览器 resize 是否能传到底层 PTY，并触发 TUI 正确重绘。
4. WebSocket relay 的最小协议形态是否足够支撑未来 Gate relay。
5. 大量输出是否会导致 WebSocket / browser / PTY reader 明显卡死。
6. 慢客户端是否会拖死全局 PTY reader 或阻塞 command runtime。
7. disconnect / reconnect / close 的语义应该如何记录，为后续 detach / reattach / Gateway 做输入。
8. 现有 M2/M3 runtime package 是否足够支撑 Web terminal spike，还是需要调整边界。

M2.5 完成后，应产出：

- 一个位于当前仓库前端目录的 Web terminal 前端原型，技术栈为 Vite + Vue 3 + TypeScript + reka-ui + Tailwind CSS。
- 一个基于官方 `@xterm/xterm`、`@xterm/addon-fit`、`@xterm/addon-web-links` 的 xterm.js / Vue terminal view，按未来 Gate 可复用组件设计。
- 一个 local WebSocket relay prototype，其消息协议应面向未来 Gate relay 演进，而不是一次性私有 hack。
- browser input / PTY output / resize 的基础桥接。
- 通过 `termbridge web` 子命令和 `just web` 开发入口启动本地 Web terminal 原型。
- 大输出与慢客户端风险观察记录。
- 对未来 Gateway Web Terminal MVP 的协议和架构建议。
- 明确哪些代码、协议和组件预计可被 Gate 复用，哪些只是本地 spike glue。

## Non-goal

M2.5 不包括：

1. 不把当前本地 Web 原型包装成完整正式产品入口。
2. 不替代 CLI；CLI 仍是本地唯一一等入口。
3. 不做完整 Gateway Workspace UI；但本阶段 Web 页面需要提供 Workspace / Session 两级结构，以验证未来 Gate 信息架构。
4. 不做 GUI container。
5. 不做完整 Gateway 产品化。
6. 不做 auth。
7. 不做 Device Registry / Workspace Registry / User model。
8. 不做公网访问、反向隧道、跨设备访问。
9. 不承诺完整跨设备 detach / reattach 产品化；但 M2.5 需要设计本地 disconnect / detach / reattach 状态语义和状态流转，因为这是 Gate 必经能力。
10. 不承诺 multi-attach 产品化。
11. 不把本阶段 relay/API 当作最终生产稳定承诺；但协议和组件设计应面向 Gate 演进，不能故意写成一次性不可复用。
12. 不重新评估或替换已确认的前端技术栈；除非 Spec 阶段发现硬性阻塞，否则默认沿用 Vite + Vue 3 + TypeScript + reka-ui + Tailwind CSS。
13. 不解决所有 M5 hardening 问题，例如复杂进程树 cleanup、长期压力测试、完整 backpressure 设计。
14. 不执行 `git add`、`git commit`、`git push` 或其他 git 写操作。

## User scenarios

### 场景一：维护者启动 Web terminal 原型

作为维护者，我希望能用明确入口启动本地 Web terminal 原型：

```text
termbridge web
just web
```

其中 `termbridge web` 是 TermBridge 的 Web terminal 子命令，`just web` 是开发期便捷入口。

启动后，它应提示：

- 本地访问 URL。
- 当前 cwd 或可选择的 cwd。
- 默认启动命令或如何指定命令。
- 该功能是 Gate Web Terminal 的本地前身，但当前不是完整 Gateway 产品入口。

该入口不能被设计成将来无法融入 Gate 的一次性工具；Spec 阶段需要定义 `termbridge web` 与现有 `exec` / `workspace` / `session` 的关系，以及 `just web` 如何编排前端 dev server 和 Go 后端。

### 场景二：浏览器打开 xterm.js 页面并运行 shell

作为维护者，我打开本地页面后，希望能看到一个最小 terminal surface，并通过 WebSocket 连接 TermBridge runtime。

基础命令例如：

```text
pwsh -NoLogo
```

应能显示 prompt、接收输入、回显输出，并能退出。

### 场景三：验证 Claude Code / Codex TUI 显示

作为维护者，我希望通过 Web terminal harness 启动：

```text
claude
codex
```

观察：

- 初始全屏布局是否正确。
- ANSI / box drawing / color 是否可接受。
- 输入框、状态栏、菜单等 TUI 是否明显错位。
- resize 后是否能恢复正确布局。

M2.5 只要求记录技术观察和明显问题，不要求完成 M5 级长期稳定性证明。

### 场景四：验证浏览器输入行为

作为维护者，我希望验证：

- 普通字符输入。
- Enter / Backspace / Tab。
- Ctrl+C。
- 方向键。
- 复制粘贴。
- 常见快捷键冲突。
- 中文/IME 初步输入行为。

M2.5 需要记录哪些输入行为可用，哪些需要后续特殊处理。

### 场景五：验证 resize 桥接

作为维护者，我调整浏览器窗口大小或 terminal 容器大小时，希望 resize 消息能通过 WebSocket 传给 TermBridge PTY，并让底层 TUI 重新布局。

M2.5 需要确认：

- xterm.js cols/rows 如何计算。
- resize 消息何时发送。
- 后端是否正确调用 PTY resize。
- 高频 resize 是否需要节流或合并。

### 场景六：验证大量输出

作为维护者，我运行大量输出命令，希望观察：

- browser 是否明显卡死。
- WebSocket 是否堆积。
- 后端内存是否无界增长。
- PTY reader 是否被输出转发阻塞。
- history writer 与 Web relay 是否互相影响。

M2.5 只要求技术验证和风险记录；完整压力治理可以进入 M5。

### 场景七：验证慢客户端与断开连接

作为维护者，我希望模拟浏览器变慢、tab 挂起、WebSocket 断开或刷新页面。

M2.5 需要观察并记录：

- 断开连接时是否关闭 PTY。
- 是否允许短时间 reconnect。
- 如果不支持 reconnect，应明确关闭语义。
- 慢客户端是否拖住全局 PTY reader。
- 未来 Gate relay 是否需要 per-client buffer / drop policy / backpressure protocol。

## Acceptance

M2.5 Requirement / Spec / Plan 完成后应满足：

- [ ] 存在 M2.5 requirement 文档，明确目标、边界、用户场景、验收标准、风险和未决事项。
- [ ] M2.5 明确是 Web Terminal Technical Spike，同时也是未来 Gate Web Terminal 的技术前身 / 可演进原型。
- [ ] M2.5 明确 CLI 仍是本地唯一一等入口，Web terminal 原型不替代 CLI。
- [ ] M2.5 明确不进入完整 Gateway/auth/device routing/remote tunnel 产品化。
- [ ] M2.5 明确验证 xterm.js rendering、browser input、resize、WebSocket relay、large output、slow client/backpressure、disconnect/reconnect 语义。
- [ ] M2.5 明确本阶段交付的 Web terminal 前端、协议和 relay 边界应尽量可被 M6 Gateway Web Terminal MVP 继承、迁移或扩展。

M2.5 实现完成后应满足的初步验收方向：

- [ ] 可通过 `termbridge web` 子命令启动 Web terminal 原型。
- [ ] 可通过 `just web` 作为开发期便捷入口启动相关前后端能力。
- [ ] 前端基于 `D:\SourceCodes\mywork\TermBridge\web` 的技术栈：Vite、Vue 3、TypeScript、reka-ui、Tailwind CSS；但不复用原前端实现。
- [ ] 前端倾向放在当前仓库 `TermBridge-go/web`，Spec 阶段参考主流 Go + Web monorepo 实践确定目录和构建集成。
- [ ] xterm.js 使用官方 `@xterm/xterm`，并集成 `@xterm/addon-fit`、`@xterm/addon-web-links`。
- [ ] xterm.js 被封装为 Vue component，并按未来 Gate 可复用组件设计。
- [ ] WebSocket 能连接后端并桥接 PTY stdout 到 xterm.js。
- [ ] 浏览器输入能桥接到 PTY stdin。
- [ ] browser resize 能传递到 PTY resize，并采用前端 throttle、后端去重策略。
- [ ] Web terminal 既能新建会话，也能打开现有会话，协议和后端边界按未来 attach 能力可扩展设计。
- [ ] Web 会话走当前 Workspace / Session 领域模型，并复用 session/history/log/state 能力。
- [ ] 页面采用左右分离结构：左侧为 Workspace / Session 两级结构，右侧为 terminal 界面。
- [ ] detach / reattach 的状态语义和状态流转在 M2.5 设计中明确表达。
- [ ] 慢客户端不会阻塞 PTY reader；至少采用 PTY reader → bounded broadcast buffer → client writer 的基本结构。
- [ ] WebSocket protocol shape 有文档说明，指出对未来 Gate relay 的影响，并参考主流实践。
- [ ] 明确标注哪些前端组件、协议消息、后端 relay 边界预计可复用于 Gate，哪些只是本地原型 glue。
- [ ] 不破坏现有 CLI `exec` / `workspace` / `session` 行为。
- [ ] 相关 Go 测试和项目约定检查通过。
- [ ] 不执行 git 写操作。

## Open questions

这些问题需要在 Spec / 规格阶段继续细化，当前不阻塞 Requirement 草稿形成：

1. `termbridge web` 的具体参数形态是什么？例如 cwd、默认 command、host/port、是否自动打开浏览器。
2. `just web` 如何编排：只启动 Go 后端，还是同时启动 Vite dev server 与 Go WebSocket/API 后端？
3. `TermBridge-go/web` 的目录结构、构建输出和 Go 后端集成方式应采用哪种主流 Go + Web monorepo 实践？
4. 后端 attach/session relay 层如何与现有 `internal/runner`、`internal/session`、`internal/state`、`internal/history` 解耦并复用？
5. Web 新建会话和打开现有会话的 HTTP/WebSocket API 如何命名？
6. detach / reattach 状态语义如何并入或扩展当前 M3 state model？
7. WebSocket protocol 的最小 message schema 如何设计？需参考主流 terminal-over-websocket / xterm.js 实践，并保留 Gate relay 扩展字段。
8. WebSocket output 是 UTF-8 string、bytes/base64，还是二进制 frame？是否需要 sequence number、ack、ping/pong。
9. PTY stdout fan-out 的 bounded broadcast buffer、慢客户端断开策略和 drop/backpressure 语义如何定义？
10. 多浏览器 attach 是否进入 M2.5 最小交付，还是只保留协议可扩展性？
11. disconnect 后进入 detached、reattaching、stopped 还是 failed？是否有 grace timeout？
12. resize 前端 throttle 和后端去重的具体窗口是多少？是否需要记录 resize event？
13. Web terminal 如何写入 M3 history，并避免 history writer 与 Web relay 互相阻塞？
14. 左侧 Workspace / Session 两级结构展示哪些字段？是否支持选择、新建、打开、关闭 session？
15. 右侧 terminal 是否显示 session id、workspace id、cwd、command、state、exit code、log/history path？
16. 最小 message schema、前端 terminal component、后端 relay abstraction 中哪些明确作为 Gate 可继承边界？
17. 如何在文档中表明该 Web terminal 是 Gate 前身但不是完整 Gateway 产品？README 不展示，按 SpecFlow 文档推进。

如果用户要求继续推进，这些事项可进入 Spec / 规格阶段。

## Decisions

当前已明确：

1. 当前流程为 strict / 严格模式。
2. 当前阶段为 Requirement / 需求。
3. 用户已指出根据 `docs/design.md` 和 `docs/plan/20260617-roadmap-refresh.md`，应先做 M2.5，而不是继续推进 M4。
4. M4 Requirement 草稿保留为 Draft，但当前优先级切换到 M2.5。
5. M2.5 是 Web Terminal Technical Spike。
6. M2.5 同时也是未来 Gate Web Terminal 的技术前身 / 可演进原型，不是做完即丢的一次性 spike。
7. M2.5 的目的，是在 Gateway 前提前交付和验证 Web terminal 相关能力。
8. M2.5 不把 local Web 包装为完整正式产品入口。
9. CLI-first 路线不变。
10. Gateway 后正式 Web 产品入口的方向不变。
11. M2.5 需要验证 xterm.js、browser input、resize、WebSocket relay、大输出、慢客户端/backpressure 和断连语义。
12. M2.5 Web 前端基于 `D:\SourceCodes\mywork\TermBridge\web` 的技术栈：Vite、Vue 3、TypeScript、reka-ui、Tailwind CSS。
13. M2.5 交付的 Web terminal 前端、协议和 relay 边界应尽量可被后续 Gate 继承、迁移或扩展。
14. 本地启动入口确定为 `termbridge web` 子命令，并提供 `just web` 开发入口。
15. 不复用原 `D:\SourceCodes\mywork\TermBridge\web` 前端实现；只复用其 Vite、Vue 3、TypeScript、reka-ui、Tailwind CSS 技术栈和经验。
16. 前端倾向放入当前仓库 `TermBridge-go/web`，Spec 阶段参考主流 Go + Web monorepo 实践确定结构。
17. xterm.js 使用官方 `@xterm/xterm`，并集成 `@xterm/addon-fit`、`@xterm/addon-web-links`。
18. xterm.js terminal 应封装为 Vue component，并按未来 Gate 可复用方式设计。
19. 后端应新增 attach/session relay 层，复用现有 runtime/domain 能力，而不是把 WebSocket 直接塞进 CLI runner。
20. Web terminal 既要能新建会话，也要能打开现有会话；协议和后端边界按未来 attach 能力可扩展设计。
21. detach / reattach 总归要实现，M2.5 需要设计对应状态语义和状态流转。
22. 最小 WebSocket message schema 采用 output/input/resize/close/error/started/exited 等基础消息，并在 Spec 阶段参考主流实践细化。
23. 慢客户端处理采纳 PTY reader → bounded broadcast buffer → client writer 的基本结构，避免慢客户端拖死 PTY reader。
24. resize 采用前端 throttle、后端去重。
25. Web 会话必须走当前 Workspace / Session 领域模型，并复用 session/history/log/state 能力。
26. Web terminal 页面采用左右分离结构：左侧 Workspace / Session 两级结构，右侧 terminal 界面。
27. 验证由用户自行执行；文档按 SpecFlow 推进，无需体现在 README。

## Risk

### 风险一：原型被误读为完整正式产品

M2.5 的 Web terminal 是 Gate Web Terminal 的前身，但还不是完整 Gateway 产品。如果把它包装成包含 auth、device routing、reverse tunnel 的正式 Web 产品，会偏离当前阶段边界，并把 M6 的产品化风险提前混在一起。

### 风险二：原型被写成不可复用的一次性 spike

如果以“只是 spike”为理由绕开清晰的协议、组件边界和 runtime 复用，后续做 Gate 时仍然要从零开始。M2.5 必须避免一次性 hack，至少在 WebSocket message schema、terminal component、relay abstraction 上保留可演进路径。

### 风险三：Web 风险继续后移

如果不做 M2.5，Gateway 阶段会同时面对 Web terminal、auth、routing、reverse tunnel、reconnect、device registry 等问题，排障成本过高。

### 风险四：慢客户端拖死 PTY reader

PTY 通常只有一个 reader。若 WebSocket 写入慢客户端时阻塞 reader goroutine，可能导致整个 session 卡死。M2.5 必须重点观察 fan-out、buffer 和 drop/backpressure 策略。

### 风险五：xterm.js 输入与本地终端语义不一致

浏览器快捷键、IME、复制粘贴、Ctrl+C、Alt/Meta 组合键都可能与本地 terminal 不一致。M2.5 需要记录差异，不应假设 Web 与本地终端天然等价。

### 风险六：resize 高频抖动

浏览器 resize 可能频繁触发，若每次都直达 PTY，可能造成 TUI 抖动或资源浪费。Spec 阶段需要设计节流或合并策略。

### 风险七：与 M3 session/history 边界不清

如果 Web harness 直接绕过 M3 session/runtime model，会让后续 Gateway/GUIs 无法复用；如果过早绑定完整 session lifecycle，又可能拖慢 spike。M2.5 需要在 Spec 阶段明确最小复用边界。

## User review notes

用户指出：阅读 `docs/design.md` 和 `docs/plan/20260617-roadmap-refresh.md` 后，当前应该先做 M2.5：Web Terminal Technical Spike。

用户进一步确认 M2.5 的关键决策：入口为 `termbridge web` 和 `just web`；前端放入当前仓库并参考主流实践，不复用原前端实现；xterm.js 使用官方包和 fit/web-links addon 并封装为 Vue component；后端新增可复用 relay 层；Web terminal 既支持新建会话也支持打开现有会话；detach / reattach 状态语义需要设计；Web 会话必须走 Workspace / Session 领域模型并复用 history/log/state；页面采用左侧 Workspace/Session、右侧 terminal 的左右分离结构；README 不展示，按 SpecFlow 文档推进。
