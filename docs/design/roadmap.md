# TermBridge-go 产品路线图

最后修改时间: 2026-06-26 12:07:02

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

- Device selector 的 online/offline/loading/reconnect UX 仍需收口。
- Device-scoped 操作还缺完整验证矩阵。
- 真实 Claude Code / Codex TUI、Ctrl+C、backpressure、long-running 等 runtime 风险仍需补证。
- Agent credential、pairing flow、token rotation 和 WebSocket auth 仍需产品化设计。

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
| M6.2 | Unified Device Workbench Closeout | 当前主线 | 收口 `/sessions` 统一 device workbench。 |
| M7 | Multi-device Beta | 未开始 | 多设备远程访问进入可测试 beta。 |
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
- device 是 online、offline、loading 还是 reconnecting。
- 当前 workspace/session 列表是否属于当前 device。
- terminal 是否连接到当前 device 的当前 session。
- 操作失败是因为 route unavailable、runtime error、auth error 还是输入错误。

### P0 工作项

#### 4.1 Device selector 状态收口

目标：让 device 选择成为可靠的工作台入口。

交付：

- 展示 device name。
- 展示 online / offline 状态。
- 展示 loading / empty 状态。
- route unavailable 时给出明确提示。
- offline device 不应被表现为“正常可操作”。

完成标准：

- 无 device 时用户看到明确 empty 状态。
- device loading 时用户看到 loading 状态。
- offline device 与 online device 在 UI 上可区分。
- 当前 selected device 状态变化不会造成 workspace/session 误显示。

#### 4.2 Local device 默认选择规则

目标：降低本地 self-connected 场景的首次使用摩擦。

交付：

- 单 device 自动选择。
- 多 device 时优先选择当前 local/self-connected device。
- 无法判断 local device 时提示用户选择。

完成标准：

- 本地默认启动后，用户进入 `/sessions` 能直接看到本机 workspace/session。
- 多 device 场景不会错误切到其他设备。
- 规则可测试或可人工验证。

#### 4.3 Device 切换隔离

目标：避免跨 device 状态污染。

交付：

- 切换 device 时清理 workspace/session store。
- 切换 device 时关闭或清理 active tabs。
- 切换 device 时旧 terminal socket 不再写入当前 workbench。
- 切换后 history、session actions、terminal attach 都使用新 device id。

完成标准：

- 在两个 device 间切换不会保留旧 device 的 session tab。
- 旧 terminal 输出不会出现在新 device 的 terminal pane。
- create / edit / close / delete / rerun / reorder 都命中新 selected device。

#### 4.4 Device-scoped 操作验证矩阵

目标：把 `/sessions` 的主路径变成可验证产品能力。

验证矩阵：

| 操作 | 期望 |
| --- | --- |
| list workspaces | 只列出 selected device 的 workspace。 |
| create session | 在 selected device 创建 session。 |
| edit session | 更新 selected device 下对应 session。 |
| close session | 关闭 selected device 下对应 session。 |
| delete session | 删除 selected device 下 stopped/failed session。 |
| rerun session | 在 selected device 复用原 session metadata rerun。 |
| reorder workspace | 只影响 selected device 的 workspace order。 |
| reorder session | 只影响 selected workspace 的 session order。 |
| read history | 读取 selected device/session history。 |
| attach terminal | terminal websocket attach 到 selected device/session。 |

完成标准：

- 每一项有自动化测试或人工验证记录。
- offline / route unavailable 有明确错误或 cached fallback。
- 验证结果记录到对应 verification 文档或 release checklist。

### P1 工作项

#### 4.5 Browser 断线 / 重连体验

目标：让 device disconnect / reconnect 对用户可理解。

交付：

- Agent disconnect 后 device 状态更新。
- Reconnect 后 device 状态恢复。
- Terminal relay close 有明确提示。
- 用户可以刷新或重新 attach。

完成标准：

- 手动断开 Agent 能看到 device offline 或 route unavailable。
- Agent 重连后 Browser 能恢复可操作状态。
- 旧 terminal socket 不产生幽灵输出。

#### 4.6 Terminal 真实交互补证

目标：验证 browser terminal 在真实 CLI agent 下可用。

覆盖：

- Claude Code TUI。
- Codex TUI。
- Shell。
- IME 输入。
- 高频 resize。
- 大量输出。

完成标准：

- 有明确观察记录。
- 已知限制进入 backlog。
- 阻塞问题在 M7 前修复或降级。

---

## 5. 并行主线：M5 Runtime Hardening 补证

M5 的当前重点是补真实验证，不是继续堆新功能。

### P0 验证

| 验证项 | 目标 |
| --- | --- |
| Claude Code 真实 TUI | 确认 CLI 和 Browser attach 下可交互。 |
| Codex 真实 TUI | 确认 CLI 和 Browser attach 下可交互。 |
| Ctrl+C / interrupt | 确认 Windows 真机下 soft interrupt、close、kill escalation 行为。 |

### P1 验证

| 验证项 | 目标 |
| --- | --- |
| 20+ sessions | 确认多 session 管理、history、close/delete 不失控。 |
| Large output | 确认输出不会导致内存无限增长或 UI 卡死。 |
| Slow client / backpressure | 确认慢客户端有可观测降级或保护。 |
| Long-running stability | 确认长时间运行后状态、history、attach 正常。 |
| 高频 resize / IME | 确认 terminal resize hardening 在真实浏览器中有效。 |

### M5 验证输出

每项真实验证应记录：

```text
环境
命令
步骤
观察结果
失败/异常
是否阻塞 M7
后续处理
```

---

## 6. M7 Multi-device Beta

### 进入条件

进入 M7 前必须满足：

1. M6.2 P0 完成。
2. M5 P0 补证完成。
3. Auth / Device credential design 完成。
4. Terminal websocket auth 策略明确。
5. 多设备访问安全边界明确。

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

### 交付范围

- Device pairing flow。
- Agent credential。
- Token rotation 基础策略。
- Multi-device workbench。
- Remote terminal attach。
- Connection status。
- Basic audit / privacy boundary。

### 验收标准

- 至少两个设备可以访问同一个 Gate。
- Agent 不需要入站端口。
- Browser 能区分多个 device。
- Remote attach 不改变 local runtime ownership。
- 权限与 credential 边界清晰。
- 断线、重连、token 失效有明确 UX。

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