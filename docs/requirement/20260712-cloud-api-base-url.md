# Cloud API Base URL 配置与 Cloud SDK 抽取需求
最后修改时间: 2026-07-12 13:37:00

Review status: Accepted

## Background

hybrid 开发模式中，`local.public_url` 与 `cloud.public_url` 都是 Vite 入口 `http://localhost:9030`，而 Cloud 后端实际监听于 `http://127.0.0.1:9032`。Agent 的 `connectCloudWithToken` 当前将 `CloudPublicURL` 与 `/api/devices/current` 直接拼接，导致 Agent 到 Vite 的 `/api/devices/current` 发起服务间请求；该路径未被 Vite 代理，最终上游返回 404、浏览器调用 `/cloud/connect` 返回 502。

现有 Agent handler 还承担了 Cloud OAuth token 交换、设备注册的 HTTP 客户端细节，并在业务处理器中重复进行 Cloud URL 的空值校验和字符串拼接。这违背配置集中校验、入站 Handler 只处理 HTTP 协议适配、外部服务调用通过应用层端口与基础设施实现隔离的架构边界。

## Goal

1. 新增 `cloud.api_base_url` 配置及环境变量 `TERMBRIDGE_CLOUD__API_BASE_URL`，专门定义 Agent 到 Cloud API 与 Tunnel 的服务间目标地址。
2. 在 hybrid 开发配置中将 Cloud API Base URL 指向 Cloud 后端 `http://127.0.0.1:9032`，保留 `cloud.public_url=http://localhost:9030` 作为浏览器可见的 Cloud 页面/OAuth 地址。
3. 将 Agent 对 Cloud 的设备注册与 OAuth token exchange HTTP 调用抽取为 Cloud SDK（基础设施适配器）；Handler 仅负责请求解析、调用应用用例和 HTTP 响应映射。
4. 消除 `internal/agent/api/handler/server.go` 中针对 Cloud URL 的空值校验、URL 字符串拼接和配置规范化等职责；地址格式与必填校验必须在 `internal/shared/infrastructure/config/config.go` 的配置加载/校验阶段完成。
5. 将 Agent persistent Tunnel 的目标和签名 audience 改为 Cloud API Base URL，以保证服务间路径在 hybrid 及部署拓扑下一致。

## Non-goal

- 不提供向 `cloud.public_url` 回退的兼容行为；`cloud.api_base_url` 为 Agent 使用 Cloud 能力的必填配置。
- 不改变浏览器运行时配置中 `cloud.publicUrl` 的语义，也不将 `cloud.api_base_url` 注入 `window.__CONFIG__`。
- 不通过让 Agent 调用 Vite `/cloud-api` 代理或新增泛化 `/api` Vite 代理来规避问题。
- 不改变 Cloud 对外 API 路径、OAuth 协议、设备注册请求/响应契约或前端 Cloud 登录流程。

## User scenarios

1. 开发者以 hybrid 配置运行 Vite、Agent（9031）和 Cloud（9032），在本地首页完成 Cloud OAuth 后连接设备；Agent 必须直接向 9032 的 Cloud API 注册当前设备，连接成功后能建立 Tunnel。
2. 部署者配置公开 Cloud 页面地址与 Agent 服务间 API 地址不同；浏览器继续使用 `cloud.public_url`，Agent 使用 `cloud.api_base_url`，两者互不混用。
3. 部署者遗漏或提供无效 `cloud.api_base_url`；应用必须在启动时因配置校验失败而退出，不得在 API Handler 中延后暴露“未配置”错误。

## Acceptance

- [ ] `TERMBRIDGE_CLOUD__API_BASE_URL` 被明确绑定、加载、去空白/尾斜杠规范化并在配置阶段校验为必填绝对 HTTP(S) URL。
- [ ] hybrid 开发配置的 `cloud.api_base_url` 为 `http://127.0.0.1:9032`，而 `cloud.public_url` 继续为 `http://localhost:9030`。
- [ ] Agent 的 Cloud 设备注册、OAuth token exchange 和 persistent Tunnel 均以 Cloud API Base URL 为目标；所有路径构造集中在 Cloud SDK，不散落在 Handler 或 bootstrap 业务流程中。
- [ ] Cloud SDK 以明确的应用层端口和基础设施实现分层：入站 Handler 不直接构造 Cloud HTTP 请求、不使用 `http.DefaultClient` 与外部 URL 拼接。
- [ ] `server.go` 不再包含针对 Cloud URL 的运行时空字符串校验或 URL 规范化逻辑。
- [ ] Cloud API Base URL 不出现在浏览器 DTO、运行时注入配置或前端 Vite 环境变量读取中。
- [ ] 覆盖配置校验、应用装配、Cloud SDK 请求目标与 hybrid 拓扑差异的回归测试；既有正式部署不被自动兼容，缺少该配置应明确失败。

## Open questions

暂无需要用户确认的未决事项。Cloud SDK 的最终包路径与端口接口命名将在实现时按项目现有 `internal/agent/application` 与 `internal/agent/infrastructure` 结构确定，保持依赖方向由外向内。

## Decisions

- 采用 `cloud.api_base_url`，而非 `cloud.api_url` 或复用 `cloud.listen_url`。
- 不进行旧配置兼容或 `cloud.public_url` fallback。
- 浏览器公开入口与 Agent 服务间入口是不同配置概念；后者仅供服务端使用。
- Cloud SDK 遵循 backend-service-architecture 指引：HTTP Handler 为入站适配器，应用层声明外部 Cloud 能力端口，基础设施层实现 HTTP SDK。

## Risk

- 此变更为刻意的破坏性配置变更：所有运行 Agent Cloud 连接、OAuth exchange 或 Tunnel 的环境必须显式提供 `cloud.api_base_url`。
- Tunnel 签名 audience 改为 API Base URL 后，Cloud 端预期 audience 必须同步使用相同值；否则连接会因签名校验失败。
- 现有测试使用 `cloud.public_url` 注入 Agent 目标的假设需要更新，不能保留隐藏 fallback。
