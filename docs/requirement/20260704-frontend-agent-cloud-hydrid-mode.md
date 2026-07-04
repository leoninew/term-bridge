# 前端 Agent / Cloud / Hybrid 模式
最后修改时间: 2026-07-04 21:55:00

Review status: Accepted

## Background

当前前端通过后端 `/api/auth/me` 返回的 capabilities 中的模式信息区分本机与云端展示，并且历史说法中使用了 `local mode`。现在需要统一产品和代码语义：不再使用 `local mode`，只使用 `agent mode` 和 `cloud mode`。同时，为本地开发场景支持一个前端同时连接 agent 后端和 cloud 后端，并能在界面内动态切换。

目标运行形态：

- `agent`：本地应用 / Agent 控制台，只展示 agent 模式界面，只调用 agent 模式接口。
- `cloud`：云端部署 / Cloud Gate 控制台，只展示 cloud 模式界面，只调用 cloud 模式接口。
- `hybrid`：混合模式，一个前端同时连接 agent 和 cloud，右上角提供入口动态切换当前模式。

## Goal

1. 统一前端模式概念，废弃用户可见和主要前端业务语义中的 `local mode`。
2. 增加明确的前端运行模式配置：`agent`、`cloud`、`hybrid`。
3. `agent` 模式下：只展示 Agent 相关页面和入口，只调用 Agent API。
4. `cloud` 模式下：只展示 Cloud 相关页面和入口，只调用 Cloud API。
5. `hybrid` 模式下：提供右上角全局悬浮模式切换入口，在 Agent 与 Cloud 视图/API 之间动态切换。
6. 首页、登录页、dashboard 分发、路由守卫应只基于前端运行模式和当前 active mode，而不是继续依赖后端 capabilities 的 `local/cloud` 判断。
7. 移除后端 capabilities 中用于表达前端模式的 mode 字段；前端运行模式只认配置，不由后端 auth/me 决定。
8. 不保留旧 `RuntimeConfig.apiBaseUrl` / `VITE_API_BASE_URL` / `/api` 前端入口兼容；对前端和浏览器可见的 API 入口只允许 `/agent-api` 或 `/cloud-api`。
9. Agent API 与 Cloud API 都应具备独立认证边界：除白名单公共地址外，业务接口必须要求对应 API 自己的 JWT 凭据。
10. Agent 模式进入 Agent Dashboard 时，应由 Agent API 提供本机信任登录能力，前端通过 Agent API 获取 Agent JWT，表现为已登录状态；该登录不需要用户名密码，不依赖 `cfg.Auth.Username`。
11. Agent API 凭据和 Cloud API 凭据必须分开存储和使用：Agent 使用 `termbridge_agent_token`，Cloud 使用 `termbridge_cloud_token`，移除旧的 `termbridge_gateway_token`，避免 hybrid 切换时两个 API 混用 token。

## Non-goal

1. 不改动 Git 历史，不执行 `git add`、`git commit`、`git push` 等 Git 写操作。
2. 不重做完整 UI 设计系统，只实现必要的模式切换入口和路由/状态语义调整。
3. 不改变 Agent 连接 Cloud Gate 的 OAuth 业务语义：本地 Agent 完成云端连接后仍回到 Agent 控制台。
4. 不在本任务中重构所有后端领域分层；后端仅在前端契约需要时做最小必要调整。
5. 不区分开发或生产启用规则；是否启用 `agent`、`cloud`、`hybrid` 只认前端配置。

## User scenarios

1. 作为本地应用用户，我打开配置为 `agent` 的前端时，直接看到 Agent Dashboard，可以管理本机 sessions，并能发起连接 Cloud Gate。
2. 作为云端用户，我打开配置为 `cloud` 的前端时，如果未登录则进入登录页，登录后进入 Cloud Dashboard 管理设备和远程 sessions。
3. 作为开发者或部署者，我打开配置为 `hybrid` 的前端时，可以通过右上角全局悬浮入口切换 Agent / Cloud 当前模式，并分别调试或使用两套界面和接口。
4. 作为 hybrid 模式用户，我刷新页面后，希望当前选择的 active mode 尽量保持，不需要每次重新切换。
5. 作为本机 Agent 用户，我进入 Agent Dashboard 时不需要输入用户名密码；前端应调用 Agent API 的本机信任登录接口获取 Agent JWT，并用该 JWT 访问 Agent 业务接口。
6. 作为 hybrid 模式用户，我在 Agent 与 Cloud 之间切换时，两个 API 的登录态互不覆盖：Agent token 只发给 Agent API，Cloud token 只发给 Cloud API。

## Acceptance

1. 前端类型或配置中存在明确的运行模式：`agent`、`cloud`、`hybrid`。
2. 前端主要路由分发不再依赖 `capabilities.mode === 'local'` 或任何后端返回的前端模式字段。
3. 后端 auth capabilities 不再暴露用于表示前端运行模式的 mode 字段。
4. `agent` 模式访问首页时进入 Agent Dashboard；不显示 Cloud Dashboard 作为主入口。
5. `cloud` 模式访问首页时：未登录进入 Login，已登录进入 Cloud Dashboard；不显示 Agent Dashboard 作为主入口。
6. `hybrid` 模式下出现右上角全局悬浮模式切换入口，可以在 Agent 与 Cloud 之间切换，并跳转到对应 dashboard 或登录流程。
7. Agent 页面调用 Agent API；Cloud 页面调用 Cloud API；hybrid 下由 active mode 显式决定当前展示与调用目标。
8. 用户可见文案和主要代码命名中不再使用 `local mode` 表达产品模式，应替换为 `agent mode` 或 Agent 相关说法。
9. Agent Cloud OAuth callback 仍回到 Agent Dashboard，不误跳 Cloud Dashboard。
10. 现有前端 typecheck、lint、format 和相关测试通过；后端如有契约调整，Go 检查也需通过。
11. 当前活跃代码不保留旧单一 `RuntimeConfig.apiBaseUrl` 或 `VITE_API_BASE_URL` fallback，不接受浏览器继续通过 `/api/...` 调用 agent/cloud API。
12. Agent API 新增 `/agent-api/auth/login`，不需要用户名密码，信任本机访问并颁发 Agent JWT；前端进入 Agent 模式或 Agent Dashboard 时应自动获取该凭据。
13. 前端不再使用 `termbridge_gateway_token`；Agent JWT 存入 `termbridge_agent_token`，Cloud JWT 存入 `termbridge_cloud_token`，请求拦截器必须按 API target 选择对应 token。
14. Agent API 与 Cloud API 除明确白名单外都必须认证。白名单至少包括 health、login、注册/验证/密码重置/OAuth start/callback/authorize/exchange、agent tunnel 握手等无需已有用户 JWT 的入口；workspace/session/device/settings 等业务接口必须要求对应 token。
15. Agent 后端认证不再依赖 `cfg.Auth.Username`；本机 Agent 登录身份由 Agent API 自身签发的本地身份表达。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

1. 统一术语：没有 `local mode`，只有 `agent mode` 和 `cloud mode`。
2. 混合模式命名为 `hybrid`。
3. `agent`、`cloud`、`hybrid` 是否启用只认前端配置，不按开发/生产环境做额外判断。
4. `hybrid` 模式切换入口采用右上角全局悬浮入口。
5. 移除后端 capabilities 中用于表达前端模式的 mode 字段；前端运行模式不再由后端 auth/me 决定。
6. `agent` 和 `cloud` 是前端部署/运行模式；`hybrid` 下另有 active mode，在 Agent 和 Cloud 间切换。
7. Agent 连接 Cloud Gate 的 session 与浏览器登录 Cloud 账号的 authenticated/token 是两个不同概念，不应混为一个状态。
8. 不做旧前端 API base 兼容：`RuntimeConfig.apiBaseUrl`、`VITE_API_BASE_URL` 和浏览器可见 `/api/...` 入口应删除或拒绝，不能作为 fallback 保留。

## Risk

1. 当前 `gateway` store 同时承载 agent auth/me、cloud auth、devices、runtime target 等状态，直接追加模式切换可能继续放大耦合；实现时需要至少明确状态边界，必要时拆 store 或先做小步解耦。
2. 移除后端 capabilities mode 字段会影响前端类型、测试和路由守卫；需要同步更新契约与测试，避免旧判断残留。
3. 路由守卫修改影响登录、OAuth callback、agent sessions、cloud sessions 等路径，容易出现 redirect loop，需要通过测试覆盖关键路径。
