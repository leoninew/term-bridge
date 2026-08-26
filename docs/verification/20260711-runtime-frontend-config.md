# 运行时前端配置与同源 API 契约验证
最后修改时间: 2026-07-11 16:10:51

- Flow mode: standard
- Stage: Verification
- Review status: Accepted
- Requirement: `docs/requirement/20260711-runtime-frontend-config.md`
- Plan: `docs/plan/20260711-runtime-frontend-config.md`

## Requirement alignment

- 开发环境继续由 `web/.env.development` 提供 hybrid 配置；Vite 仅在开发代理边界保留 `/local-api`、`/cloud-api`，并 rewrite 为 canonical `/api`。
- Agent 和 Cloud 统一注册 `/api`，未知 `/api/...` 由 API handler 返回 404，不会落入 SPA fallback。
- 静态 HTML 通过服务端投影的公开强类型 DTO 注入 `window.__CONFIG__`；公开 URL、OAuth browser 字段、mode 和 `/api` 被允许输出，服务端 secret、数据库和其他完整配置不会序列化到页面。
- `web/.env.local`、`web/.env.cloud`、`web/.env`、`Dockerfile.preflite` 与后端 `ApiBaseUrl` 配置链路已移除；打包和镜像不再绑定测试或生产公开地址。
- Portable Agent package 随包提供 `.env.prod`（`https://termbridge.preflite.cn`）和 `.env.test`（`http://termbridge.lvh.me`）；启动脚本默认 prod，调用方设置 `TERMBRIDGE_ENV=test` 时选择 test。
- 未为旧开发前缀添加兼容或适配路由，也未将“旧地址必须不存在”作为额外行为契约。

## Plan alignment

| 计划项 | 验证结论 |
| --- | --- |
| 公开 DTO 与 allowlist 投影 | 已实现并由 shared config/API 测试覆盖。 |
| HTML 注入与静态资源边界 | 已实现，覆盖 marker、脚本边界安全、资源直出、SPA fallback、`HEAD` 和 `no-store`。 |
| `/api` 迁移 | Agent/Cloud handler、CORS、日志、OAuth、设备请求、terminal/tunnel 及测试均已迁移。 |
| 移除后端 API base 配置 | Go 配置模型、YAML、模板、Docker/package profile 与相关测试已更新。 |
| 前端构建边界 | 仅开发环境文件保留；`build:local` 与 `build:cloud` 使用同一中性构建。 |
| Docker 与 package profile | 两份 Dockerfile 构建通过，package 输出包含 prod/test profile。 |

## Actual diff summary

- 新增 browser runtime-config DTO、后端配置投影及共享静态 HTML 注入 handler。
- Agent 与 Cloud router/API 调用从角色前缀迁移为 `/api`；Vite 代理负责开发模式前缀转换。
- 清理废弃配置文件和 API base 字段，Docker 镜像不再固化 `TERMBRIDGE_*` 值。
- 更新 portable package 运行 profile、启动脚本、README、配置模板及覆盖测试。

## Expected and actual file scope

- **预期范围：** shared config/API、Agent/Cloud API 与 bootstrap、Vite/build、Docker/package、说明及测试。
- **实际范围：** 与预期一致。工作树中尚有 `web/src/components/**`、`web/src/i18n.ts` 及 `docs/requirement/20260711-local-home-shortcuts.md` 的快捷方式/页面相关未暂存改动，不属于本功能，未纳入本次 staged scope。

## Acceptance checklist

- [x] 开发代理保留双目标分流，后端接收 `/api`。
- [x] 打包/镜像 HTML 在运行时注入公开完整 schema。
- [x] 敏感配置未进入浏览器 DTO。
- [x] `/api` 未知路径保持 API 404。
- [x] 后端 `ApiBaseUrl` 配置和部署绑定的前端 env 已移除。
- [x] Dockerfile 无固化 `TERMBRIDGE_*` runtime ENV；portable profile 可选择 prod/test。
- [x] 未引入旧路径兼容或适配逻辑。

## Test results

| 命令 | 结果 |
| --- | --- |
| `go test ./cmd/... ./internal/...` | 通过。 |
| `yarn --cwd web typecheck` | 通过。 |
| `yarn --cwd web test` | 通过：12 个测试文件、62 个测试。 |
| `yarn --cwd web lint` | 通过。 |
| `yarn --cwd web build:local` | 通过。 |
| `yarn --cwd web build:cloud` | 通过。 |
| `sh -n scripts/package/start.sh` | 通过。 |
| `docker build -f Dockerfile -t termbridge-runtime-config-test .` | 通过。 |
| `docker build -f Dockerfile.cn -t termbridge-runtime-config-cn-test .` | 通过。 |
| `git -c core.whitespace=cr-at-eol diff --check` | 通过。 |

`yarn --cwd web format` 未通过：Prettier 报告 12 个 session/shortcut Vue 文件存在格式问题。这些文件是本次范围外的并行快捷方式工作；未自动格式化，避免将无关变更并入。

两次 Vite build 产生 `@vueuse/core` 的 Rolldown `#__PURE__` 注释警告，但构建成功，未影响产物。

## Risks and incomplete items

- 未在本机实际启动三种拓扑并以浏览器端到端操作验证；当前验证覆盖单元/集成测试、产物、package 和 Docker 构建。部署时仍需提供完整的服务端运行时配置。
- staged diff 的默认 whitespace 检查会将 CRLF 行尾报告为 trailing whitespace；使用项目适用的 `git -c core.whitespace=cr-at-eol diff --check` 已通过。
- 范围外快捷方式页面文件仍未通过 Prettier，不影响本功能的 typecheck、test、lint 或 build。

## Conclusion

实现与已接受的 Requirement/Plan 对齐，核心自动化校验、package 及两份 Docker image 构建均通过。上述范围外格式问题和未执行真实浏览器端到端拓扑验证已记录；本功能可作为独立变更提交。
