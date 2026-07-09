# Docker 前端运行时配置问题记录
最后修改时间: 2026-07-09 22:09:27

Flow mode: light / 轻量模式
Stage: Requirement / 需求
Review status: Draft

## Background

当前 Docker 镜像在 `web-build` 阶段执行 `yarn build:cloud`，Vite 会读取 `web/.env.cloud` 并将前端公开配置固化进 JS bundle。运行时阶段的 Docker `ENV TERMBRIDGE_CLOUD__...` 只会影响 Go 后端配置，不会改写已经构建好的前端 JS。

本轮已先做最小收口：将 `Dockerfile` 与 `Dockerfile.cn` 中的 Cloud runtime ENV 调整为与 `web/.env.cloud` 一致，避免同一个镜像内后端运行时配置和前端构建期配置明显漂移。但这只是缓解，不是最终设计决策。

本轮继续更正本地模式连接云端流程：OAuth2 认证、浏览器持有的 `cloud_token`、本地 Agent 到云端的连接状态是三个不同层级，不能混在 callback 页面里一次性完成。callback 只负责写入 `cloud_token`；Local Home 负责根据 `cloud_token` 与 sessionStorage 中的连接时间决定是否调用本地 connect。

## Goal

- 记录 Docker 镜像前端配置固化问题，作为后续新任务的决策入口。
- 明确当前事实：Cloud 镜像前端配置来自 build-time `web/.env.cloud`，Docker runtime ENV 只影响 Go 后端。
- 后续需要在“继续构建期固化”与“实现运行时注入”之间做正式决策。
- 决策时必须同时考虑 Cloud 镜像、portable package、本地开发和 OAuth redirect URL 的一致性。
- 明确本地模式连接云端的前端状态机：OAuth2 只负责获得 `cloud_token`，Local Home 负责 connect / disconnect 与 sessionStorage 连接时间。
- 明确断开后保留 `cloud_token`，再次连接云端时直接使用已有 token connect，不重新进入 OAuth2。

## Non-goal

- 本任务只记录问题，不在本轮实现后端 `window.__CONFIG__` 注入。
- 不恢复旧的后端复杂配置注入实现。
- 不改变当前 `build:cloud` / `build:local` 的产品线构建入口。
- 不引入临时兼容层或双配置兜底。

## User scenarios

- 作为部署者，我希望知道 Docker `-e TERMBRIDGE_CLOUD__PUBLIC_URL=...` 是否会影响浏览器前端行为。
- 作为维护者，我希望 Cloud 镜像中的前端公开配置和 Go 后端运行时配置不会悄悄漂移。
- 作为发布负责人，我希望明确一个镜像是否可以部署到多个 Cloud 域名，还是每个域名需要单独构建前端。

## Acceptance

后续 Docker 前端配置任务需要至少回答并落实以下问题：

- Docker runtime ENV 是否应该影响前端 `runtimeConfig`。
- 如果继续构建期固化：
  - `web/.env.cloud` 是否就是 Cloud 镜像唯一前端公开配置来源。
  - Dockerfile runtime ENV 中哪些值只允许作为 Go 后端配置存在。
  - 文档如何避免部署者误以为 `docker run -e` 会改变前端 JS。
- 如果改为运行时注入：
  - Go 后端如何在服务 `index.html` 或等价入口时生成 `window.__CONFIG__`。
  - 注入内容只允许公开配置，不能包含 `client_secret`、JWT secret、数据库 DSN 等 secret。
  - 空 `window.__CONFIG__ = {}` 占位与 build-time fallback 的关系如何处理。
  - 相关测试如何覆盖 Docker/runtime env 改变前端配置的行为。
- Cloud OAuth redirect URL、Cloud public URL、API base URL 在前端和后端之间必须保持一致。

本轮本地模式连接云端流程需要满足：

- 只有一条 OAuth2 认证路径；没有 `cloud_token` 时才进入 OAuth2。
- `OAuthCallbackView.vue` 只做 OAuth state 校验、code → token 兑换、写入 `cloud_token`、重定向回 Local Home。
- OAuth callback 不调用 `/local-api/cloud/connect`，不写 sessionStorage 连接时间。
- Local Home 页面加载完成后统一判断：有 `cloud_token` 且没有 sessionStorage 连接时间时，调用 `/local-api/cloud/connect` 并写入连接时间。
- Local Home 刷新时执行同一套判断：有连接时间则展示已连接；没有连接时间但有 `cloud_token` 则直接 connect。
- 用户已 OAuth2 认证且曾连接成功后，即使断开连接导致连接时间被清理，`cloud_token` 仍保留；再次连接云端时直接使用已有 `cloud_token` connect，不重新发起 OAuth2。
- disconnect 仍调用后端断开 tunnel / 清理运行态连接；前端在成功后清理 sessionStorage 连接时间并清空页面运行态 cloud session；不清理 `cloud_token`。

## Open questions

1. Cloud 产品镜像是否要求“一次构建，多域名部署”？
2. Cloud 最终公网域名是否稳定，能否接受写入 `web/.env.cloud` 并在构建期固化？
3. 如果启用运行时注入，是恢复后端 HTML 注入，还是采用独立 runtime config JSON 端点？
4. Docker runtime ENV 和前端 build-time env 命名是否继续共用 `TERMBRIDGE_*`，还是需要区分公开前端配置与后端运行配置？

## Decisions

- Docker 前端配置本轮暂不做最终方案决策。
- 本轮只把镜像 runtime ENV 调整到与 `web/.env.cloud` 一致，避免当前 Cloud public/API 配置明显漂移。
- 后续新任务单独评估前端运行时配置注入或继续构建期固化。
- 本地模式连接云端采用“认证 token”和“连接时间”分离的状态机：`cloud_token` 表示已完成 OAuth2 认证，sessionStorage 连接时间表示当前浏览器会话内已完成 connect。
- connect 决策统一放在 Local Home，OAuth callback 不承担连接副作用。

## Risk

- 如果继续构建期固化，一个 Docker 镜像无法靠 runtime ENV 改变浏览器前端的 Cloud public URL。
- 如果恢复运行时注入，必须严格限制注入内容，避免把后端 secret 暴露到浏览器。
- 如果前端和后端 Cloud public URL / redirect URL 不一致，OAuth 授权、token exchange 或页面跳转会失败。
- 如果文档没有明确 build-time 与 runtime 边界，部署时容易误配并产生难以排查的问题。
- 如果 OAuth callback 同时承担 connect，会让“认证完成”和“连接完成”耦合，导致刷新、断开后重连和错误恢复路径不一致。
- 如果 disconnect 清理了 `cloud_token`，用户已完成的 OAuth2 认证会被误丢弃，断开后重连会被迫重新走 OAuth2。
- 如果只有 `cloud_token` 没有 sessionStorage 连接时间却不自动 connect，OAuth 回调回首页后会停留在“已认证但未连接”的半完成状态。

## User review notes

- 用户指出 Dockerfile 看起来固化了前端环境变量。
- 用户要求本轮先将镜像中的环境变量改得和 `.env.cloud` 一致，并结束本轮任务；后续新任务再做正式决策。
- 用户更正：当前工作没有完成，前端连接云端需要先 OAuth2 认证取得 `cloud_token`，回到 Local Home 后再由 Local Home 根据 token 和连接时间发起 connect。
- 用户纠偏：断开连接在后端会关闭 tunnel / 清理运行态连接；前端只清理 sessionStorage 连接时间，不清理 `cloud_token`；已有 token 时再次连接云端直接 connect，不重新 OAuth2。
