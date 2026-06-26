# TermBridge-go 产品路线图

最后修改时间: 2026-06-26 21:02:12

## 1. 产品目标

TermBridge-go 的产品目标是提供一套本地优先、可远程访问的 workspace runtime：

1. 用户可以在本地通过 CLI 快速启动任意命令。
2. 用户可以在 Browser workbench 中管理设备、工作区、会话和终端。
3. 用户可以从其他设备安全访问本地 runtime。
4. 终端交互、历史、重连、resize 和 session 操作在本地与远程路径下保持一致。

长期产品形态：

```text
CLI runtime
Browser workbench
Device registry
Remote access
Secure pairing
Release package
```

---

## 2. 当前产品状态

当前阶段：**M6.2 Unified Device Workbench Closeout**。

已经具备：

- CLI command runner。
- Workspace / Session runtime model。
- Browser `/sessions` workbench。
- Gateway service + Agent connector unified serve。
- Device-scoped Browser API。
- Gateway / Agent relay。
- 基础 Browser auth。
- 基础 workspace/session mutation。
- Terminal attach、history、resize hardening。

当前还不应进入 Multi-device Beta，原因是：

- `/sessions` device workbench 还需要完成状态表达与验证闭环。
- Device 切换隔离已有实现基础，但还缺面向用户路径的验证结论。
- Claude Code / Codex TUI 启动、Ctrl+C 退出和进程出现/消失已有人工 smoke 观察；backpressure、long-running、压力场景等 runtime 风险仍需补证。
- Cloud Gate 镜像交付面已完成 PoC，支持镜像内前后端一体交付。
- Remote Gate / Local Agent 形态仍需要完成真实跨网络路径验证、身份、凭据、授权、重连和安全边界设计。

---

## 3. 路线图总览

| 阶段 | 名称 | 状态 | 产品结果 |
| --- | --- | --- | --- |
| M0 | Runtime 技术决策 | 完成 | 明确 Go + PTY abstraction 方向。 |
| M1 | CLI Skeleton | 完成 | `termbridge` CLI 可启动、可解析命令、可加载配置。 |
| M2 | PTY Command Runner MVP | 完成核心能力 | CLI 能运行用户命令并接入 PTY。 |
| M2.5 | Web Terminal Spike | 完成核心能力 | Browser terminal 链路可用，风险进入 hardening。 |
| M3 | Session / Workspace Runtime Model | 完成主体 | workspace/session/history 模型可支撑产品化。 |
| M4 | Local Product Surface | 基本完成 | CLI 与本地 workbench 具备日常使用基础。 |
| M5 | Runtime Hardening | 部分完成 | 代码 hardening 推进中，真实场景验证仍需补证。 |
| M6 | Gateway Web Terminal MVP | 完成工程链路 | Gate / Agent / Browser workbench 打通。 |
| M6.1 | Serve Entry Consolidation | 完成 | 统一入口为 `termbridge serve`。 |
| M6.2 | Unified Device Workbench Closeout | 当前主线 | 收口 `/sessions` 统一 device workbench；Cloud Gate 镜像交付面已完成 PoC。 |
| M7 | Multi-device Beta | 未开始 | 多设备远程访问进入可测试 beta；仍需真实 Remote Gate + Local Agent 验证和正式安全模型。 |
| M8 | Security / Packaging / Release | 未开始 | 安全模型、发布包、安装升级和发布文档。 |

---

## 4. 当前主线：M6.2 Unified Device Workbench Closeout

### 产品目标

让 `/sessions` 成为稳定、可理解、可验证的统一工作台：

```text
Device selector
  -> Workspace tree
  -> Session actions
  -> Terminal attach
```

用户应该能清楚知道：

- 当前选中哪个 device。
- 当前 device 是否可操作。
- 当前 workspace / session / terminal 是否属于当前 device。
- 操作失败是 route unavailable、runtime error、auth error 还是输入错误。

### P0 工作项

| 工作项 | 状态 | 路线图目标 |
| --- | --- | --- |
| Device selector 状态收口 | 待收口 | 工作台能清楚表达 device 可用性和不可用原因。 |
| Device 默认选择 | 基本具备，待验证 | 单 device 直达工作台；多 device 由用户明确选择。 |
| Device 切换隔离 | 实现基础已具备，待验证 | 切换 device 不产生 workspace、session、tab、terminal 状态污染。 |
| Device-scoped 操作验证 | 待补 | 证明主要 workspace / session / terminal 操作都命中 selected device。 |

### Cloud Gate / container PoC 进度

已完成：

- 镜像交付面已打通，支持将 Browser workbench 与后端服务放入同一交付单元。
- 本地开发仍保持前后端分离，交付镜像采用前后端一体形态。
- PoC 认证与 device 上报链路已收口到当前最小可验证形态。

尚未完成：

- 容器运行态 smoke 未作为本轮验证项执行。
- 真实 HTTPS 反向代理与 WebSocket upgrade 未做部署验证。
- Remote Gate + separate Local Agent 的真实跨网络路径未验证。
- 用户系统、用户-device 绑定、pairing、credential rotation 仍属于 M7/M8 前置设计。

### 阶段验收

- 单 device 与多 device 场景下，`/sessions` 的状态和操作路径可预测。
- 切换 device 不遗留旧 device 的会话状态或 terminal 输出。
- offline / route unavailable / auth error 有明确用户反馈。
- M6.2 P0 主路径有自动化测试或人工验证记录。

### P1 工作项

| 工作项 | 路线图目标 |
| --- | --- |
| Browser 断线 / 重连体验 | 用户能理解 Agent disconnect / reconnect 后工作台处于什么状态，以及如何恢复。 |
| Terminal 真实交互补证 | 在进入 M7 前确认 Claude Code、Codex、Shell、IME、resize、大输出等关键交互风险。 |

---

## 5. 并行主线：M5 Runtime Hardening 补证

M5 的当前重点是补真实验证结论，不是继续堆新功能。

当前已有人工 smoke 观察：

- Claude Code TUI 可以启动。
- Codex TUI 可以启动。
- Ctrl+C 可以退出。
- 进程有出现和消失。

进入 M7 前仍需形成结论：

| 补证项 | 需要回答的问题 |
| --- | --- |
| Browser attach 下的真实 TUI | Claude Code / Codex / Shell 是否能稳定交互。 |
| interrupt / close / kill escalation | 不同 shell / TUI 下退出语义是否清晰可靠。 |
| backpressure / large output | 慢客户端或大量输出是否有保护或明确降级。 |
| long-running stability | 长时间运行后状态、history、attach 是否仍可信。 |
| resize / IME | 真实浏览器下 resize hardening 与输入体验是否满足日常使用。 |

---

## 6. M7 Multi-device Beta

M7 的核心变化是 Gate 从本机发布到远端，Agent 从用户本机主动连接远端 Gate，Browser 通过远端 Gate 访问本地 runtime。

目标链路：

```text
Browser
  -> Remote Gate
  -> Local Agent outbound tunnel
  -> Local Runtime
  -> PTY / Process
```

### 核心工作包

| 工作包 | 产品结果 |
| --- | --- |
| Remote Gate 部署基线 | 镜像交付面已具备；仍需真实 HTTPS reverse proxy、WebSocket upgrade、runtime smoke 和远端服务安全边界验证。 |
| Device trust / pairing | 用户可以把本地 Agent 可信地绑定到自己的 Gate 账号或访问主体。 |
| Agent outbound tunnel | 本地 Agent 不需要入站端口，也能稳定连接远端 Gate。 |
| Remote terminal path | 远端链路下 attach、input、output、resize、interrupt、history 与本地路径保持一致。 |
| Auth / authorization boundary | Browser auth、device credential、terminal websocket 授权边界一致。 |
| Beta verification | 至少在 Remote Gate + Local Agent 分离部署下完成真实路径验证。 |

### 进入条件

进入 M7 前必须满足：

1. M6.2 P0 完成。
2. M5 关键补证形成结论。
3. Remote Gate 真实部署验证与安全边界设计完成。
4. Device credential / pairing / rotation 策略明确。
5. Terminal websocket auth 与 route unavailable 策略明确。

### 产品目标

让用户可以从另一台设备访问本地 runtime。

用户故事：

```text
用户在 Home-PC 启动 TermBridge Agent。
用户在 Laptop Browser 登录 Gate。
用户看到 Home-PC device。
用户选择 workspace/session。
用户 attach terminal 并查看或继续操作。
```

### Beta 验收

- Remote Gate 可以通过 HTTPS 对 Browser 和 Agent 提供服务，并已完成真实 reverse proxy / WebSocket upgrade 验证。
- 至少两个设备可以访问同一个 Gate。
- Agent 不需要入站端口。
- Browser 能区分多个 device。
- Remote attach 不改变 local runtime ownership。
- 断线、重连、route unavailable、token 失效有明确 UX。
- 远端链路下 terminal 主路径与本地 self-connected 保持一致。

---

## 7. M8 Security / Packaging / Release

### 产品目标

准备真实用户发布。

### 交付范围

- Security model。
- Threat model。
- Release package。
- Installer / portable archive。
- Config migration。
- Upgrade / rollback。
- Public network exposure warning。
- Minimal CI / release checks。
- User-facing docs。

### 验收标准

- 用户可以按文档安装并启动。
- 默认配置安全可用。
- 远程访问有明确认证和授权边界。
- 升级不会破坏已有 state/config。
- Release artifact 可重复构建。

---

## 8. 产品优先级原则

1. 先保证单机 self-connected 体验稳定，再进入多设备 beta。
2. 先保证 `/sessions` 工作台状态可信，再增加复杂远程能力。
3. 先补真实 TUI / interrupt / backpressure 验证，再承诺 beta 可用。
4. 先设计 credential / pairing / token rotation，再开放多设备远程访问。
5. 每个阶段都必须有可验证的用户路径，而不是只完成内部代码。
