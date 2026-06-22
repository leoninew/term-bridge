# M6 Gateway Web Terminal MVP 需求
最后修改时间: 2026-06-22 14:28:00

Review status: Accepted

## Background

M5 Runtime Hardening 已完成代码层收口：process tree cleanup、interrupt escalation、Web backpressure、history 并发安全和 flush 行为均有自动化覆盖；相关包测试、race-sensitive 测试、Windows 交叉编译、Pomelo PW Web/workbench flow 和全量 Go 测试通过。

仍有若干 M5 人工/环境验证项未完全完成：Claude Code / Codex 真实 TUI、20+ session 压测、Windows 真实 process tree cleanup、高频 resize 和 xterm 手动输入/attach/detach。它们不阻止开始 M6 需求设计，但必须作为 M6 风险输入，避免 Gateway 放大 runtime 或 Web terminal 的未验证行为。

根据 roadmap，M6 的目标是引入 Gateway，并把 Web 作为正式跨设备产品入口。Gateway 不拥有 PTY / Process / Workspace runtime，不直接运行用户命令，只负责 auth、device registry、session/workspace registry、routing 和 tunnel relay。TermBridge runtime 仍在本地 Agent/CLI 侧拥有进程生命周期。

## Goal

完成 M6 Gateway Web Terminal MVP 的需求定义：

1. 引入 Gateway service，作为 Browser 与本地 TermBridge runtime / Agent 之间的连接、认证和路由层。
2. 引入 Agent outbound tunnel，使本地 runtime 主动连接 Gateway，而不是 Gateway 主动访问本地机器。
3. 通过 Gateway 暴露 Web terminal 正式入口，使 Browser 可以查看 device、workspace、session 并 attach terminal。
4. Gateway 只做 relay / registry / routing / auth，不拥有 PTY，不启动用户命令，不修改 runtime ownership。
5. 复用 M2.5/M5 已验证的 Web terminal 协议、history、backpressure 和 runtime hardening 基础，避免在 M6 重新实现另一套 terminal runtime。
6. 建立 M6 临时 admin/admin 登录和 device registration，满足 MVP 的访问边界和本地开发验证；正式用户系统稍后实现。
7. 提供 connection status：device online/offline、tunnel connected/disconnected、session attach 状态。
8. 建立 M6 自动化测试、Pomelo PW flow 和人工验证清单。

## Non-goal

本阶段不做以下事项：

1. 不做正式用户系统、多人账号体系、组织、团队、RBAC 或完整 SaaS auth；M6 只使用临时 `admin/admin`。
2. 不做公网生产部署、TLS 证书自动签发、域名管理或云端发布流程。
3. 不做多设备 Beta 的完整产品体验；多设备扩大验证属于 M7。
4. 不实现 token rotation、审计日志、device pairing 完整 UX；这些进入 M7/M8 或后续阶段。
5. 不让 Gateway 直接创建 PTY、直接运行命令、直接读写本地 workspace 文件系统。
6. 不改变现有正式 CLI 入口 `termbridge exec -- <command...>`。
7. 不实现复杂协同功能，例如多浏览器同时写入同一 terminal、共享光标、多人 session。
8. 不把 local Web/workbench 删除或改造成唯一入口；M6 新增 Gateway 正式远程入口，local CLI/runtime 仍成立。

## User scenarios

1. 作为本地用户，我可以启动 TermBridge Agent，使它主动连接 Gateway 并注册当前 device。
2. 作为远程浏览器用户，我可以通过临时 `admin/admin` 登录 Gateway，看到在线 device 列表。
3. 作为远程浏览器用户，我可以选择一个在线 device，查看该 device 暴露的 workspace/session 列表。
4. 作为远程浏览器用户，我可以通过 Gateway attach 到某个 session，并在 xterm.js 中看到输出、输入命令、resize terminal。
5. 作为本地 runtime 维护者，我可以确认 Gateway 没有直接运行用户进程；所有 PTY/Process lifecycle 仍由本地 runtime 管理。
6. 作为用户，我可以看到 device / tunnel / session 的连接状态；Agent 断线后 Gateway 清理路由，重连后恢复 online 状态。
7. 作为维护者，我可以通过测试和 Pomelo PW flow 验证 Gateway relay、auth、device registry、session listing、terminal attach 和 reconnect 基础行为。

## Acceptance

M6 Gateway Web Terminal MVP 完成需要满足：

1. Agent 可以主动连接 Gateway，并完成 device registration。
2. Gateway 能维护在线 device registry，Agent 断开后 device 状态变为 offline 或被清理。
3. Agent reconnect 后 device online 状态恢复，旧 tunnel 不继续接受新路由。
4. Browser 通过 Gateway 临时 `admin/admin` auth 后可以看到在线 device。
5. Browser 通过 Gateway 可以看到来自 Agent/runtime 的 workspace/session listing。
6. Browser 可以通过 Gateway attach terminal，terminal output/input/resize 经 relay 到本地 runtime。
7. Gateway 不直接创建 PTY、不直接运行用户命令、不拥有 Process lifecycle。
8. Gateway terminal websocket relay 有明确 backpressure / close / error 行为，不绕过 M5 已有 runtime backpressure 边界。
9. M6 使用临时 `admin/admin` 登录，未授权请求不能访问 device/session/terminal relay；正式用户系统留到后续实现。
10. Gateway 和 Agent 的错误、断线、重连、路由缺失有可观察日志或状态。
11. 自动化测试覆盖 auth、device registry、session/workspace relay、terminal relay 基础消息、disconnect cleanup。
12. Pomelo PW flow 覆盖 Browser 经 Gateway 访问 Web terminal 的基本路径。
13. 人工验证清单覆盖真实 terminal attach、输入、输出、resize、Agent disconnect/reconnect。
14. M6 通过后，可以进入 M7 Multi-device Beta。

## Open questions

1. Gateway Web UI 是否复用当前 `web/` 前端并增加 Gateway mode，还是新建 Gateway frontend entry？
2. Device identity 的稳定来源是什么：本地 state dir 中生成 device id，还是用户配置 name/key？
3. M6 是否允许 Gateway 在本地开发模式下与 Agent 同机运行，但仍保持逻辑边界？
4. Workspace/session listing 是否直接 relay 当前 runtime API，还是在 Gateway 保留只读 cache？
5. Agent disconnect 时，Browser terminal 应显示何种状态和错误文案？
6. 多个会话的支持范围是多个 session 可同时存在并分别 attach，还是还包括多个 browser 同时 attach 同一个 session？
7. M5 未完成的 20+ session、Claude/Codex、Windows 真实验证作为 M6 verification 风险继续跟踪；具体哪些纳入自动化、Pomelo PW 或人工验证需在 Spec/Plan 中拆分。

## Decisions

当前已确认：

1. M6 使用严格模式 / strict。
2. M6 是新 feature，使用独立 SpecFlow 文档：`20260620-m6-gateway-web-terminal-mvp.md`。
3. Gateway 不拥有 runtime；PTY / Process / Workspace lifecycle 仍由本地 TermBridge runtime 管理。
4. Agent 使用 outbound tunnel 连接 Gateway。
5. M6 使用临时 `admin/admin` auth；正式用户系统稍后实现。
6. M6 复用 M2.5/M5 的 Web terminal/runtime 基础，不重写另一套 terminal runtime。
7. M6 支持 attach/list 已存在 session，不通过 Gateway 创建新 session。
8. Gateway service 与 Agent connector 使用同一个 `termbridge serve` 统一入口；历史独立 subcommand 不再保留。
9. Agent tunnel 采用单 WebSocket 多路复用。
10. M6 支持多个会话。
11. 可单元测试和 Pomelo PW 测试的自行组合验证，剩余项列入人工测试列表。

## Risk

1. 如果 M5 的剩余人工验证问题在 Gateway 之上暴露，排障会同时涉及 runtime、relay、auth 和 Web UI，复杂度明显上升。
2. Gateway 如果缓存或复制过多 runtime 状态，容易破坏 runtime ownership 边界。
3. Tunnel relay 若缺少 backpressure 和 close/error 语义，会重新引入 M2.5/M5 已处理过的问题。
4. 临时 `admin/admin` auth 仅适合 MVP 和受控环境；若误用于公网或生产环境，会产生明显安全风险。
5. Agent reconnect / stale tunnel cleanup 若不清晰，Browser 可能 attach 到失效路由。
6. 复用当前 local Web frontend 时，local mode 与 Gateway mode 的 API base、WebSocket URL、auth header、error handling 可能互相污染。
7. 如果 M6 同时做 session creation、pairing UX、正式用户系统和 token rotation，范围会扩张到 M7/M8。

## User review notes

用户已确认进入 Spec，并补充以下决策：

1. M6 支持 attach/list 已存在 session，不通过 Gateway 创建新 session。
2. Gateway service 与 Agent connector 使用同一个 `termbridge serve` 统一入口；历史独立 subcommand 不再保留。
3. Agent tunnel 采用单 WebSocket 多路复用。
4. 服务端稍后实现正式用户系统，M6 使用临时 `admin/admin` 作为用户。
5. M6 支持多个会话。
6. 可单元测试和 Pomelo PW 测试的自行组合验证，剩余项列入人工测试列表。

仍需在 Spec 阶段收敛：

1. Gateway Web UI 复用当前 `web/` 还是新建 entry。
2. Device identity 来源。
3. 多个会话是否包含多个 browser 同时 attach 同一 session。
4. Gateway 是否保留 session/workspace 只读 cache。
5. Agent disconnect 的浏览器状态和错误文案。
