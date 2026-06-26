# TermBridge-go 路线图

最后修改时间: 2026-06-26 12:07:02

Review status: Accepted

## 如何使用本文

本文是 **当前路线图与下一步工作清单的唯一入口**。后续判断“下一步做什么”时，不再从多个 design memo、verification 或历史 requirement 中手工捞待办。

| 文档 | 角色 |
| --- | --- |
| `docs/design/design.md` | 当前架构事实和设计边界。 |
| `docs/plan/20260617-roadmap-refresh.md` | 当前路线图、活跃阶段、下一步工作清单和 gate。 |
| `docs/design/20260623-unified-gate-device-model.md` | Gate / Device 模型 supporting note，不作为独立待办来源。 |
| `docs/design/20260624-roadmap-design-implementation-gap-memo.md` | 历史差距核对备忘，P0 文档同步已完成，不作为当前 backlog。 |
| `docs/requirement/*`、`docs/spec/*`、`docs/verification/*` | 单项变更的过程记录和验收证据。 |

---

## 当前一句话状态

TermBridge-go 已经完成 CLI-first runtime 主线、统一 `termbridge serve` 入口、device-scoped Gateway/Agent relay 和 `/sessions` 统一工作台雏形；当前主线是 **M6.2 Unified Device Workbench Closeout**，并行补 **M5 manual verification**，之后才能进入 **M7 Multi-device Beta**。

---

## 当前架构方向

### 资源模型

```text
User
  -> Device
    -> Workspace
      -> Session
        -> Terminal
```

### 本地命令入口

```text
termbridge [options] exec -- <command...>
```

### 统一服务入口

```text
termbridge serve
  -> Gateway service
  -> Agent connector
```

### Browser 产品入口

```text
/sessions
  -> /api/devices/:deviceId/...
  -> Gateway Browser API
  -> Agent tunnel
  -> local runtime
  -> PTY / Process
```

本地 self-connected Gate 和云端 Gate 只由配置决定，不分裂前端工作台、不恢复 `/gateway` 产品页、不恢复 Browser -> localapi -> registry direct path。

---

## Milestone 总览

| Milestone | 状态 | 当前判断 |
| --- | --- | --- |
| M0 Runtime 技术决策 | 已完成 | Go + go-pty + Windows ConPTY 作为当前方向；风险进入后续 gate。 |
| M1 Go CLI Skeleton | 已完成 | CLI entrypoint、help/version、`--cwd`、`exec --`、config、logging、基础错误模型已具备。 |
| M2 PTY Command Runner MVP | 核心已完成 | PTY abstraction、CommandRunner、stdin/stdout、resize、基础 Ctrl+C、exit code、cwd/env 已具备；真实 TUI/cleanup 风险进 M5。 |
| M2.5 Web Terminal Technical Spike | 已完成 | xterm.js、WebSocket relay、session create/attach/detach/close、history replay 已具备；large output/backpressure/高频 resize 风险进 M5。 |
| M3 Session / Workspace Runtime Model | 主体已完成 | session/workspace metadata、runtime state、bounded history、local record、listing/history 已具备。 |
| M4 Local Product Surface | 基本收口 | CLI product surface、config、logs、workspace/session listing、local web/workbench 辅助入口已具备；人工验证项转 M5。 |
| M5 Runtime Hardening | 部分完成，仍需 manual verification | 多项代码 hardening 已完成；真实 Claude/Codex TUI、Ctrl+C、20+ sessions、backpressure、long-running 等仍需补证。 |
| M6 Gateway Web Terminal MVP | 工程链路已完成 | `termbridge serve`、Gateway service、Agent connector、device-scoped API、基础 mutation relay、terminal attach 已具备。 |
| M6.1 Serve entry consolidation | 已完成 | 统一入口为 `termbridge serve`，配置驱动 Gateway/Agent，前端保持单体 `/sessions`。 |
| M6.2 Unified Device Workbench Closeout | 当前主线 | 需要补齐 device UX、验证矩阵和断线/重连/terminal 真实验证。 |
| M7 Multi-device Beta | 未开始 | 需要 M6.2 gate 和 auth/device credential design 后进入。 |
| M8 Security / Packaging / Release | 未开始 | 发布、安全、安装、升级、配置迁移。 |

---

## 当前主线：M6.2 Unified Device Workbench Closeout

### 目标

让 `/sessions` 成为稳定的统一 device / workspace / session / terminal 工作台。

```text
/sessions
  -> device selector
  -> workspace/session tree
  -> session actions
  -> terminal attach
```

### 已具备

- 前端统一入口为 `/sessions`。
- 前端 session/workspace API 已走 `/api/devices/:deviceId/...`。
- Gateway / Agent relay 已覆盖 workspace tree、history、terminal attach。
- Gateway / Agent relay 已覆盖 create/update/close/delete/rerun session 与 workspace reorder/delete 等 mutation。
- `termbridge serve` 已同时启动统一后端和 Agent connector。
- 只有一个 device 时，前端会自动选择该 device；多 device 时保留显式选择。
- device 切换时会 reset workspace/session store 和 workbench tabs，降低跨 device 状态污染风险。
- Gateway 已记录 device online/offline/last_seen；device disconnect/reconnect 会关闭旧 terminal relay。
- Gateway 离线读取 workspace tree/history 时可返回 cached response，并通过 `X-TermBridge-Offline` 标记。
- Terminal resize hardening 已补齐 attach 初始尺寸、resize 失败重试、错误传播和前端 fit safety margin。

### 剩余工作清单

| 优先级 | 工作项 | 目标 | 完成标准 |
| --- | --- | --- | --- |
| P0 | Device selector 状态收口 | 让用户能理解当前 device 是 online、offline、loading、reconnecting 还是空列表 | selector 或周边 UI 展示 device online/offline；loading/no device/route unavailable 文案清晰；不会把 offline 误呈现为可正常操作。 |
| P0 | Local device 默认选择规则 | 多 device 时优先选择本机 self-connected device，而不是永远要求手选 | 有明确规则和测试/验证：单 device 自动选择；多 device 时 local/self device 优先；无法判断时才提示用户选择。 |
| P0 | Device 切换与 terminal lifecycle 验证 | 确认切换 device 不污染 workspace/session/tab/terminal | 切换 device 后旧 tabs/terminal socket 不继续写入当前工作台；active session 清空或切到新 device 的合法 session。 |
| P0 | Device-scoped 操作验证矩阵 | 固化 `/sessions` 主要动作在 device-scoped API 下可用 | create / edit / close / delete / rerun / reorder / history / attach 都有自动化或人工验证记录。 |
| P1 | Browser 断线/重连验证 | 补真实 browser + Gateway/Agent disconnect/reconnect 场景 | 记录 device offline、route unavailable、terminal close、重新连接后的行为。 |
| P1 | Terminal 真实交互补证 | 观察真实 TUI、IME、高频 resize、输入输出 | 记录 Claude Code / Codex 或 shell 下的浏览器实际表现。 |

### M6.2 Gate

进入 M7 前必须满足：

```text
1. /sessions 是唯一统一 workbench，不需要 /gateway 或 local direct path。
2. device selector 能表达 online/offline/loading/empty/reconnect。
3. device 切换不会污染 workspace/session/terminal 状态。
4. create/edit/close/delete/rerun/reorder/history/attach 有 device-scoped 验证矩阵。
5. terminal attach、disconnect、reconnect、resize 有真实或自动化验证记录。
```

当前不能仅凭代码实现视为 M6.2 关闭。

---

## 并行补证：M5 Runtime Hardening

M5 不再作为纯代码开发阶段；当前主要是补真实场景验证，并把验证结果沉淀为 verification 文档。

### 已完成的 hardening 方向

- Runtime resize 失败后不推进 current，允许同尺寸重试。
- CLI runner resize 失败后不推进 last。
- PTY initial resize 失败不静默忽略。
- Gateway / Agent terminal attach/resize 错误可观测。
- Reattach 初始尺寸传递到 Agent stream resize。
- Resize 上限收紧到 `1000x1000`。
- 前端 xterm fit 增加 one-cell safety margin。

### 仍需补证

| 优先级 | 验证项 | 完成标准 |
| --- | --- | --- |
| P0 | Claude Code 真实 TUI | 通过 CLI 和/或 `/sessions` attach 记录真实输入、输出、resize、退出行为。 |
| P0 | Codex 真实 TUI | 同上。 |
| P0 | Ctrl+C / interrupt | Windows 真机记录 Claude/Codex/shell 中 soft interrupt、close、kill escalation 行为。 |
| P1 | 20+ sessions | 记录多 session 创建、attach、history、close/delete 行为。 |
| P1 | 大量 output / slow client / backpressure | 记录内存、pending queue、丢弃/降级行为是否可观测。 |
| P1 | long-running stability | 记录长时间运行 session 与 reconnect 行为。 |
| P1 | 高频 resize / IME | 记录真实浏览器下高频 resize 与输入法状态，确认 safety margin 是否足够。 |

### M5 Gate

M5 不要求阻塞所有 M6.2 UI 收口，但进入 M7 前必须至少完成 P0，并明确 P1 的风险接受或补测计划。

---

## M7 前必须补的设计

M7 是 Multi-device Beta，不能只靠当前 MVP auth 和配置驱动 identity 进入。

| 主题 | 当前状态 | M7 前要求 |
| --- | --- | --- |
| Browser auth | JWT Bearer token 已实现基础边界 | 明确用户系统、会话有效期、logout、刷新/过期体验。 |
| Agent credential | 当前 device identity 已在配置/state 中存在 | 设计正式 device credential、存储、吊销、轮换。 |
| Pairing flow | 未实现正式 pairing | 设计用户如何把本机 Agent 绑定到 Gate。 |
| Token rotation | 未实现 | 设计 rotation 策略和兼容窗口。 |
| WebSocket auth | terminal WS 支持 token/query 路径 | 收口为一致的 auth 策略，避免 HTTP/WS 边界漂移。 |
| Audit / privacy | 未系统设计 | 设计命令、路径、history、terminal output 的记录与脱敏边界。 |

---

## 后续 milestone 定义

### M7：Multi-device Beta

目标：让跨设备访问成为可测试产品形态。

交付：

```text
device pairing
formal agent credential
multi-device workbench
multi attach over Gateway
remote access UX
connection status
audit/privacy baseline
token rotation
```

进入条件：M6.2 gate 通过，且 auth/device credential design accepted。

### M8：Security / Packaging / Release

目标：准备真实发布。

交付：

```text
security model
threat model
release package
installer / archive
upgrade strategy
config migration
docs
minimal CI/release checks
```

进入条件：M7 beta 验证完成，远程访问安全边界明确。

---

## 历史 milestone 摘要

这些 milestone 已不再展开作为日常待办来源；需要追溯时看对应 requirement/spec/verification。

### M0 Runtime 技术决策

- Go + `github.com/aymanbagabas/go-pty` 可作为 Windows PTY runtime 技术方向。
- Windows backend 使用 ConPTY / Pseudoconsole。
- 不再恢复 ttyd/tmux。

### M1 Go CLI Skeleton

- CLI entrypoint、命令解析、`--help`、`--version`、`--cwd`、`exec --`、config、logging、基础错误模型已完成。

### M2 PTY Command Runner MVP

- CommandRunner、ProcessSpec、PTY abstraction、stdin/stdout relay、resize relay、exit code、cwd/env、基础 cleanup 已完成。
- 真实 TUI、复杂 Ctrl+C、进程树清理风险进入 M5。

### M2.5 Web Terminal Technical Spike

- xterm.js、WebSocket relay、stdin/stdout bridge、resize bridge、session create/attach/detach/close、history replay 已实现。
- 大输出、backpressure、高频 resize 风险进入 M5。

### M3 Session / Workspace Runtime Model

- session/workspace metadata、runtime state、bounded history、local session record、process/exit/log record、workspace/session listing 已完成。

### M4 Local Product Surface

- CLI help/errors/config/log path、workspace/session listing、local web/workbench 辅助入口已具备。
- 人工交互验证项进入 M5。

### M6 / M6.1 Gateway and Serve consolidation

- Gateway service、Agent outbound tunnel、device registry、routing/relay、terminal websocket relay、single frontend `/sessions` 已成型。
- 对外入口收口为 `termbridge serve`。
- Gateway/Agent 参数进入配置。

---

## 每次推进工作的要求

1. 先检查本文的当前主线和 gate，不再从历史 memo 捞待办。
2. 新功能或 bugfix 仍按 SpecFlow 创建/更新对应 requirement / verification。
3. 实现完成后，必须把状态回写到本文的当前主线或 milestone 状态，避免文档再次落后。
4. 如果某项只完成代码但没有真实验证，状态只能写“部分完成”或“仍需补证”，不能标完成。
5. 不执行 `git add` / `git commit` / `git push`，除非用户明确授权。