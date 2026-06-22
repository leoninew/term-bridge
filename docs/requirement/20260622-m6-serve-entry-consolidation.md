# M6.1 Serve 统一入口与文档收口需求
最后修改时间: 2026-06-22 14:28:00

Review status: Accepted

## Background

M6 Gateway Web Terminal MVP 已完成主要工程链路：Gateway service / Agent connector 使用同一 `termbridge` binary，Agent 通过 outbound tunnel 连接 Gateway，Browser 可以经 Gateway relay 访问 workspace tree、session list、history 和 terminal attach。M6.1 后用户入口统一为 `termbridge serve`，不保留历史独立 Gateway/Agent subcommand。M6 自动化检查、Web build 和 Pomelo PW flow 已覆盖基础路径。

但当前 M6 的完成质量存在明显产品化缺口：入口零散、正式入口与开发/验证入口边界不清、README 和设计文档落后于实现、`docs/todo.md` 与 M5/M6 已处理事实不一致、M6 verification 仍为 Draft 且缺少入口/文档质量验收口径。

当前问题不是单纯功能不可用，而是 Gateway 能力已经拼通，但使用路径、文档叙事和验收状态没有同步收口。若直接进入 M7 Multi-device Beta，现有入口混乱会被放大，并增加后续维护和验证成本。

## Goal

本任务目标是建立 M6.1 Serve Entry Consolidation，以严格模式 / strict 收口 M6 的入口和文档质量：

1. 将 Gateway 和 Agent 从用户感知上的两种模式收口为统一 `termbridge serve` 启动后的两类能力：服务启动后同时具备 Gateway service 和 Agent connector 能力，用不用由配置、路由和访问路径决定。
2. 更新 README，使用户可以从文档中理解本地使用、统一 serve 启动、Web 访问、配置项和当前限制。
3. 更新设计文档和 roadmap，使 Gateway/Agent 合并能力边界、配置驱动入口、前端单体路由区分、M6 状态和 M6.1 收口任务与当前实现一致。
4. 更新 M6 verification，明确 M6 当前是工程链路已实现但入口/文档未验收，并补充入口与文档缺口。
5. 整理 `docs/todo.md`，避免 M5 已处理或已缓解的问题继续以“待处理”形式误导后续工作。
6. Agent 连接哪个 Gateway 是配置问题；Gateway host/port、Agent gateway URL/device name 等运行参数必须收口到 Viper 配置和配置覆盖机制，不通过 CLI 参数或环境变量作为正式入口传递。
7. 删除独立 `termbridge gateway` / `termbridge agent` 正式入口和 `just gateway` / `just agent` 开发入口；保留统一 `termbridge serve` / `just serve` 作为启动入口。
8. 删除 M6 专用测试入口，避免 `m6-*` 成为长期入口负担。
9. 前端只有一个，通过不同路由区分 local/Gateway 访问面；本任务不引入第二套 frontend。
10. 将 M6.1 的验收标准固定下来，并在 M6.1 完成后把 M6 verification 更新为 Accepted。

## Non-goal

本任务不做以下事项：

1. 不实现 M7 Multi-device Beta。
2. 不实现正式用户系统、device pairing、token rotation、RBAC、审计日志或生产部署能力。
3. 不扩展 Gateway 的远程 mutation 能力；M6/M6.1 仍保持 Gateway MVP 对远程 workspace/session 的只读和 attach 约束。
4. 不重写 Gateway / Agent tunnel 协议，不重新实现 terminal relay。
5. 不改变 Gateway service 不拥有 PTY / process lifecycle 的架构边界。
6. 不删除底层测试、Pomelo PW flow 或可重复验证能力；但删除 `justfile` 中 M6 专用测试入口，不把 `m6-*` 作为长期命令入口保留。
7. 不要求前端在本任务中新增更完整的 Gateway 模式切换说明；前端保持单体，通过不同路由区分访问面。
8. 不用文档掩盖未完成验证；Agent reconnect、20+ sessions、真实 Claude/Codex TUI、Windows 真机验证等仍需如实记录，但 M6.1 完成后 M6 verification 可以更新为 Accepted。

## User scenarios

1. 作为新用户，我阅读 README 后能知道如何通过 `termbridge serve` 启动统一 Gateway/Agent 服务，以及如何通过配置决定 Gateway 监听地址和 Agent 连接目标。
2. 作为维护者，我能从 justfile 或文档中区分正式统一启动入口与验证辅助命令；不会再看到 gateway 和 agent 作为平级模式入口。
3. 作为 Gateway 用户，我沿用当前前端 Gateway 交互；前端仍只有一个，通过不同路由区分本地和 Gateway 访问面，不因本次入口收口引入第二套前端。
4. 作为后续开发者，我阅读 roadmap 和 M6 verification 后能知道 M6 的工程链路已打通，但入口和文档收口由 M6.1 承接。
5. 作为验证人员，我能根据 M6.1 verification 判断 README、design、roadmap、M6 verification、todo、justfile/CLI help 和前端路由说明是否一致。
6. 作为项目维护者，我不会被 `docs/todo.md` 中已过期的 M2.5/M5 风险误导，而是能看到哪些问题已处理、哪些仍需人工验证、哪些仍未解决。

## Acceptance

M6.1 完成需要满足以下验收标准：

1. 存在清晰的入口矩阵，至少区分：
   - 正式用户入口 / Formal user entry：`termbridge serve`。
   - 本地开发入口 / Development entry：`just serve`。
   - 自动化或人工验证入口 / Verification entry：通用 test/build/Pomelo 命令或文档化命令。
   - 临时辅助入口 / Temporary helper entry：不得以长期 `m6-*` just target 保留。
2. README 覆盖当前主要使用路径：
   - 本地 `termbridge exec -- <command...>`。
   - 本地 Web/workbench 启动。
   - 通过配置声明 Gateway service 监听地址、Agent 连接目标和 device name。
   - 通过统一入口启动 Gateway/Agent 服务：`termbridge serve`。
   - 浏览器通过同一个前端的不同路由访问本地或 Gateway 能力。
   - M6/M6.1 Gateway MVP 当前支持与不支持的能力。
3. README 明确 Gateway MVP 当前限制：
   - 支持查看 device/workspace/session。
   - 支持读取 history。
   - 支持 attach existing terminal session。
   - 暂不支持通过 Gateway 创建 session 或修改 workspace/session。
   - 不在 README 中公开临时 `admin/admin`。
4. 设计文档补齐当前 Gateway/Agent 合并能力边界：
   - Browser -> Gateway service -> Agent connector -> local runtime -> PTY。
   - Gateway service 不拥有 PTY/process lifecycle。
   - Agent connector 是本地 runtime 的远程 adapter。
   - 启动后同时具备 Gateway service 和 Agent connector 能力，用不用由配置、路由和访问路径决定。
   - 前端只有一个，通过不同路由区分 local/Gateway 访问面，backend capability 不同。
5. Roadmap 中 M6 状态反映真实情况：工程链路已实现，入口和文档收口由 M6.1 承接；M6.1 完成后 M6 verification 可进入 Accepted。
6. M6 verification 增加入口与文档收口说明，并在 M6.1 完成后更新为 Accepted，同时保留 Agent reconnect、20+ sessions、真实 Claude/Codex TUI、Windows 真机验证等未完成项。
7. `docs/todo.md` 被整理为与 M5/M6 事实一致，至少区分：
   - 已由 M5/M6 处理或缓解。
   - 仍需人工验证。
   - 仍未解决。
8. justfile 保留统一 `serve` 开发入口，删除 `gateway` / `agent` 平级开发入口和 M6 专用测试入口；长期验证应复用通用测试/build/Pomelo 能力或文档化命令。
9. CLI 正式入口收口为 `termbridge serve`；删除独立 `termbridge gateway` / `termbridge agent` 入口和 Gateway/Agent 的运行参数 flag（如 `--host`、`--port`、`--gateway-url`、`--device-name`、`--seed-session-command`）；相关设置通过配置和配置覆盖机制表达，并按需要调整 help，使正式入口与 README 叙述一致。
10. 配置结构、默认配置和配置校验覆盖 Gateway service 与 Agent connector 运行参数；Agent 连接哪个 Gateway 由配置决定。
11. 前端保持单体，不新增第二套 frontend；文档说明通过不同路由区分 local/Gateway 访问面。
12. M6.1 不引入新的 Gateway runtime ownership 问题；任何 UI 或文档调整不得暗示 Gateway service 可以直接创建 PTY 或运行命令。
13. 相关文档和代码改动有 verification 记录，包含实际 diff、预期与实际改动对比、检查命令结果、剩余风险和未完成项。

## Open questions

暂无需要用户确认的未决事项。以下事项已在本轮 review 中形成决策，并进入 Decisions。

## Decisions

当前已确认：

1. 本任务使用严格模式 / strict。
2. 本任务作为新任务创建独立 SpecFlow 文档，文件名为 `20260622-m6-serve-entry-consolidation.md`。
3. 本任务定位为 M6.1 Gateway 入口与文档收口，不直接进入 M7。
4. M6 当前问题定性为：工程链路已实现，但产品入口、文档和验收口径未收口。
5. Gateway 和 Agent 不再作为用户感知上的两种模式；启动后同时具备 Gateway service 和 Agent connector 能力，用不用由配置、路由和访问路径决定。
6. Agent 连接哪个 Gateway 是配置问题；`agent.gateway_url` / `agent.device_name` 等不通过 CLI 参数或环境变量作为正式入口传递。
7. CLI 正式入口收口为 `termbridge serve`。
8. 删除独立 `termbridge gateway` / `termbridge agent` 入口。
9. `just serve` 作为统一开发入口，调用无运行参数的正式命令并依赖配置。
10. 删除 `just gateway` / `just agent` 平级开发入口。
11. 删除 M6 专用测试入口，例如 `m6-frontend`、`m6-agent`、`m6-agent-with-session`、`m6-test`、`m6-race`、`m6-check`、`m6-pw-validate`、`m6-pw-run`；验证能力通过通用命令、底层测试或文档记录保留。
12. 删除 Gateway/Agent 运行参数 flag，包括 `--host`、`--port`、`--open`、`--dev`、`--gateway-url`、`--device-name`、`--seed-session-command`；对应值进入配置结构、默认配置和配置校验。
13. 前端只有一个，通过不同路由区分 local/Gateway 访问面；不新增第二套 frontend。
14. M6 verification 在 M6.1 完成后更新为 Accepted，同时保留剩余人工验证项。
15. README 不提及临时 `admin/admin`。

## Risk

1. 如果只更新 README 而不整理 justfile、roadmap、verification 和 todo，入口混乱仍会留在维护路径中。
2. 删除独立 Gateway/Agent CLI 后，已有脚本或个人验证路径可能需要迁移到统一 `termbridge serve` + 配置方式。
3. 删除 M6 专用 just target 可能影响已有个人验证习惯；需要在文档中保留等价验证命令或说明底层测试/Pomelo flow 仍存在。
4. 如果 M6.1 过度扩张到 reconnect、20+ session、真实 TUI、Windows 真机等深度验证，范围会从入口收口变成 M6 完整补完，影响交付节奏。
5. README 不提及临时 `admin/admin` 后，真实登录方式仍可能需要通过开发者上下文或后续正式 auth 文档补齐。
6. M6 verification 在 M6.1 后改为 Accepted 时，必须清楚保留剩余人工验证项，避免读者误解为所有深度验证已完成。
7. 前端路由区分如果只写文档但实现不一致，会继续造成入口认知混乱；Implementation 阶段需要核对当前前端路由现实。

## User review notes

用户反馈：M6 完成质量不高，入口零散杂乱，文档落后。用户先要求使用 SpecFlow 标准模式开始这个任务，后续要求修改为严格模式并进入 Plan。

用户后续决策：

1. 删除所有 `m6-*` just target。
2. 删除 `--seed-session-command`。
3. 接受 M6 verification 在 M6.1 完成后改为 Accepted。
4. Gateway/Agent 运行参数不应通过 CLI 参数或环境变量传递，应全部收口到 Viper 配置及配置覆盖机制。
5. Gateway 和 Agent 模式合二为一：启动后同时有 agent 和 gateway 的能力，用不用是另一件事。
6. Agent 连接哪个 Gateway 是配置的事情。
7. 前端也只有一个，使用不同路由区分。
8. Gateway/Agent 对外收口为统一 `termbridge serve` 启动入口；开发阶段是 `just serve`。
