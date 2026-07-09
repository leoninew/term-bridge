# 本地/云端术语收口
最后修改时间: 2026-07-09 13:09:14

Flow mode: light / 轻量模式
Stage: Requirement / 需求
Review status: Accepted

## Background

当前前端和文档中仍混用 `Agent mode` / `Cloud mode`、`agent.mode`、`config.agent`、`gateway` 等历史命名。随着 Home 入口、runtime config 和 cloud 侧能力边界收敛，用户希望进一步统一术语：产品表达使用“本地模式 / 云端模式”，配置与代码结构也按 local/cloud 对齐，避免把 Agent、gateway 等历史命名继续暴露为模式或 cloud 侧业务边界。

## Goal

- 将用户可见的“Agent 模式”统一改称“本地模式”。
- 将用户可见的“Cloud 模式”统一改称“云端模式”。
- 将模式值从 `agent | cloud | hybrid` 收口为 `local | cloud | hybrid`，其中：
  - `local` 表示本地模式。
  - `cloud` 表示云端模式。
  - `hybrid` 表示本地开发/联调可在本地模式与云端模式之间切换。
- 将前端 runtime config 中的 `agent` 配置节点改为 `local` 配置节点。
- 将对应环境变量从 `TERMBRIDGE_AGENT__...` 改为 `TERMBRIDGE_LOCAL__...`。
- 消除前端 cloud 认证、cloud 设备、cloud token 等状态边界里的 `gateway` 命名，改为 `cloud` 命名。
- 将本地侧 HTTP API 前缀从 `/agent-api` 改为 `/local-api`，前端、后端路由、测试、配置和文档同步更新。
- 更新 README、当前 requirement/verification 等新近文档中的术语，避免继续写“Agent 模式 / Cloud 模式 / gateway”。
- 拆分旧 `web/src/store/cloud.ts` 的混合职责，不保留 `cloud.ts` 兼容导出或适配层。

## Non-goal

- 不重命名 Go 后端包路径或 Agent 产品实体本身。
- 不改变 `/cloud-api` HTTP API 前缀。
- 不保留 `/agent-api` 兼容路径。
- 本地侧视图、组件、路由命名应同步改为 local 语义；与真实 Agent OAuth/连接协议绑定的路径若保留 `agent`，必须能说明它表示 Agent 实体而非模式命名。
- 不修改历史归档文档中无关旧项目阶段的大量“gateway”记录；本轮优先更新当前需求、README、前端代码和当前验证文档。
- 不做旧配置名兼容 fallback；配置错误继续 fail fast。
- 不新增兼容层、适配层或复杂迁移适配代码。

## User scenarios

- 作为用户，我在页面上看到“本地模式 / 云端模式”，不会看到“Agent 模式 / Cloud 模式”这种内部实现术语。
- 作为维护者，我在前端配置中看到 `local` 与 `cloud` 两个清晰节点，不再把 `agent` 当作 UI 模式名称。
- 作为维护者，我在 cloud 状态 store 和调用处看到 `cloud` 命名，不再看到 `gateway` 这种历史抽象。
- 作为部署者，我通过 `TERMBRIDGE_LOCAL__...` 配置本地侧公开地址、API base URL 和 OAuth 配置，通过 `TERMBRIDGE_CLOUD__...` 配置云端侧公开地址和 API base URL。
- 作为接口调用方，我访问本地侧能力时使用 `/local-api`，不再使用 `/agent-api`。
- 作为前端维护者，我直接使用 `authTokens`、`localAuth`、`cloudAuth`、`cloudDevices` 等 store，不通过旧 `cloud.ts` 兼容层。

## Acceptance

- 前端用户可见文案不再出现“Agent 模式”或“Cloud 模式”，对应改为“本地模式 / 云端模式”。
- 前端模式类型、runtime config view mode 和路由 meta mode 使用 `local | cloud | hybrid` 语义；不再使用 `agent` 表示 UI 模式值。
- runtime config 的配置结构从 `config.agent` 改为 `config.local`。
- 前端 env 类型、`.env.*`、README 和测试中的本地配置环境变量从 `TERMBRIDGE_AGENT__...` 改为 `TERMBRIDGE_LOCAL__...`。
- 本地侧 API base URL 的默认/示例值改为 `/local-api`，前端请求和后端 HTTP 路由同步使用 `/local-api`。
- `gateway` store、`useCloudStore` 和相关变量命名改为 cloud/local/token 语义命名。
- CloudHome / Dashboard / cloud auth 页面使用 cloud store 命名，不再暴露 gateway 命名。
- LocalHome / 本地模式页面中涉及云端连接状态时，可以使用 cloud session / cloud connection 命名；不使用 gateway。
- `web/src/store/cloud.ts` 不作为兼容 re-export 或适配层保留；调用方直接依赖职责明确的 store。
- 当前 runtime config requirement、verification、README 同步使用本地/云端术语。
- 旧 `agent` 模式值、`config.agent`、`TERMBRIDGE_AGENT__` 在前端当前代码和当前文档中不再作为配置/模式语义出现；`/agent-api` 不再出现。若 `agent` 出现在后端包路径、设备实体、OAuth callback path、内部生成类型、数据库文件名等位置，应确认其表示真实 Agent 实体而不是 UI 模式。
- 前端 typecheck、相关前端测试和构建通过。

## Open questions

- 暂无阻塞实现的问题。用户已明确后端 API 前缀和视图也要同步改；实现时按 local 语义推进，只有真实 Agent 实体/协议语义才保留 `agent` 命名。

## Decisions

- 使用 light / 轻量模式推进。
- 本轮术语收口以当前前端代码、README、当前 requirement/verification 为主，不大规模重写历史归档文档。
- 不提供旧 env 名或旧 mode 值兼容；继续遵守配置 fail fast。
- “Agent”仅在真实后端本地服务、设备实体或外部协议语义中保留；前端视图、模式、配置和本地 API 前缀统一使用 local。
- “Cloud”用户可见中文改为“云端”；代码中 cloud 作为云端边界名称保留。
- store 拆分不做兼容或适配层；删除旧 `cloud.ts` 聚合导出。

## Risk

- 这是命名收口，触及文件较多，容易出现旧新命名混用。
- `agent` 同时表示真实本地 Agent 服务和旧 UI 模式；实现时需要把本地 API 前缀和视图改为 local，但不能把仍表示真实 Agent 实体/协议的后端名称机械改坏。
- `gateway` / `cloud` / `local` store 重命名会影响大量 import、测试和变量名，需要依赖 typecheck 与测试兜底。
- 如果文档更新范围过大，容易污染历史记录；本轮应限制在当前交付相关文档。

## User review notes

- 用户要求继续收口，使用 `/specflow light`。
- 用户要求：“Agent 模式”改称“本地模式”，对应 `mode=local`。
- 用户要求：“Cloud 模式”改称“云端模式”，对应 `mode=cloud`。
- 用户要求配置里的 `agent` 节点改为 `local`。
- 用户要求消除 `gateway` 一类说法，它们应该是 `cloud`。
- 用户追加要求 `/agent-api` 改为 `/local-api`，后端和视图也同步改。
- 用户追加要求 `web/src/store/cloud.ts` 进一步拆分。
- 用户明确要求不做兼容或适配层。
