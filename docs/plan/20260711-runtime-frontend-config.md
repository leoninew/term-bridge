# 运行时前端配置与同源 API 契约实施计划
最后修改时间: 2026-07-11 15:22:08

- Flow mode: standard
- Stage: Plan
- Review status: Accepted
- Requirement: `docs/requirement/20260711-runtime-frontend-config.md`

## Implementation steps

1. 在 shared 层新增公开、强类型的 browser runtime config DTO 和构造函数。它从完整后端配置投影 Local/Cloud public URL、Local OAuth client ID/redirect URL/scopes、role-derived mode 与固定 `/api`，并在静态站点启用时校验公开字段；不得输出服务端敏感字段。
2. 在 `cmd/termbridge/app/app.go` 加载完整配置后、压缩为 Agent/Cloud bootstrap config 前构造 DTO，并传入两个 role 的 bootstrap server config。
3. 抽取 Agent 与 Cloud 重复的静态 SPA 服务逻辑。共享 helper 校验 HTML entry 的唯一 `__RUNTIME_CONFIG__` marker，仅渲染 entry/SPA fallback，使用安全 JSON 注入 `window.__CONFIG__`，对 HTML 设置 `no-store`，保持资源直出与 `HEAD` 语义。
4. 将 Agent、Cloud、CORS path 判定、路径解析、Agent-to-Cloud HTTP 调用、OAuth authorize URL、terminal/tunnel WebSocket 与 tunnel 签名由 `/local-api`、`/cloud-api` 一次性迁移为 `/api`。不注册兼容路由；`/api` 未知请求必须保持 API 404。
5. 删除无实际路由语义的后端 `ApiBaseUrl` 配置全链路，并更新相关 YAML、模板、Docker、package 和测试。
6. 仅保留 `web/.env.development` 作为开发前端环境配置；删除 `web/.env.local`、`web/.env.cloud`。保留 build script 名称，但使 Local/Cloud build 都生成不含部署地址的同一静态 bundle。Vite 的 `/local-api`、`/cloud-api` proxy 保留并 rewrite 到 `/api`。
7. 删除 `Dockerfile.preflite`，并从其余 Dockerfile 移除所有 `TERMBRIDGE_*` image ENV。建立 portable package `.env.prod`（Preflite）和 `.env.test`（lvh.me）配置，更新 package 任务与启动脚本：默认 prod，外部 `TERMBRIDGE_ENV=test` 选择 test。
8. 更新 Go/Vue 测试与 README、环境配置说明；验证 DTO allowlist、HTML 注入、API/SPA 边界、旧路径无兼容、Vite rewrite、Package profile 和镜像无固化运行时配置。

## Files to change

- Configuration/composition: `internal/shared/infrastructure/config/config.go`, new shared browser runtime-config DTO/helper, `cmd/termbridge/app/app.go`, Agent/Cloud bootstrap configs and routers.
- API migration: Agent/Cloud API handlers, `internal/agent/application/user/cloud_client.go`, request-log/CORS-related tests, corresponding HTTP/WebSocket/tunnel tests.
- Frontend/dev server: `web/index.html`, `web/.env.development`, `web/.env.local` (delete), `web/.env.cloud` (delete), `web/vite.config.ts`, runtime/API/WebSocket tests and package scripts.
- Packaging/deployment: `Dockerfile`, `Dockerfile.cn`, `Dockerfile.preflite` (delete), `Taskfile.yml`, `scripts/package/.env.prod` (new), `scripts/package/.env.test` (new), `.env.local` (delete), `start.cmd`, `start.sh`.
- Documentation/tests: `README.md`, `.env.example`, `configs/config*.yaml`, affected config/router/app tests.

## Assumptions and risks

- The same `/api` namespace is valid because Agent and Cloud run at different origins; development target selection belongs exclusively to Vite proxy keys.
- All Cloud static deployments require complete Local OAuth public configuration because the existing browser runtime schema is complete for both Local and Cloud values.
- No compatibility route means operational rollback requires redeploying the prior complete version; partial rollout is unsafe because the tunnel’s signed request path changes.
- Browser runtime config uses an explicit allowlist. Existing backend config must never be serialized directly.

## Verification plan

1. Run focused and full Go tests: `go test ./cmd/... ./internal/...`.
2. Run Vue checks: `yarn --cwd web typecheck`, `yarn --cwd web test`, `yarn --cwd web lint`, `yarn --cwd web format`.
3. Build package and Docker images; verify Dockerfiles have no `TERMBRIDGE_*` defaults, no preflite Dockerfile remains, and the package ships `.env.prod`/`.env.test`.
4. Drive development proxy rewrite, packaged Local prod/test profiles, and test/production Cloud image runtime configurations. Confirm injected HTML changes after backend restart without rebuilding assets; static assets remain unchanged; `/api/missing` is an API 404; no sensitive fields appear in HTML.
