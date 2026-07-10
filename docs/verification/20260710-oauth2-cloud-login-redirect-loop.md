# OAuth2 Cloud 登录跳转循环修复验证
最后修改时间: 2026-07-10 19:18:51

Review status: Accepted

## 需求对齐

按 `docs/requirement/20260710-oauth2-cloud-login-redirect-loop.md` 核对。

- Cloud 登录页现在以 Cloud 服务端 `/cloud-api/auth/me` 的 `authenticated` 结果决定是否恢复 OAuth2 授权请求；明确未认证时会清除浏览器保存的 Cloud token。
- Cloud 登录与 Google callback 成功取得 token 后，均重新向 Cloud 确认认证状态，再继续授权路径。
- Cloud 登录 redirect 使用统一、安全的站内路径存储与消费逻辑，完整保留 OAuth2 query。
- 本地 `/oauth/callback` 先校验 OAuth state，再调用 `useLocalAuthStore.ensureToken()` 获取或复用 local Agent JWT，最后调用继续受保护的 `/local-api/cloud/oauth/exchange`。
- callback 不直接调用 `/local-api/cloud/connect`；Cloud token 保存后仍由既有本地页面流程负责后续设备连接。

## 实际改动摘要

- `web/src/store/cloudAuth.ts`：Cloud 服务端明确返回未认证时清除失效 Cloud token。
- `web/src/views/cloud/LoginView.vue`：仅在 Cloud 服务端认证成功后恢复授权路径；密码登录后重新确认认证状态。
- `web/src/views/cloud/GoogleCallbackView.vue` 与 `web/src/features/cloud/loginRedirect.ts`：统一 Google 登录后的安全 redirect 存储与恢复。
- `web/src/views/local/OAuthCallbackView.vue`：在 state 校验后、authorization code exchange 前初始化 local Agent JWT。
- `web/src/store/authStores.test.ts`：验证 local token 已存在时不会重复发起 local login。
- `internal/agent/api/handler/cloud_binding_test.go`：验证未携带 local JWT 的 exchange 仍返回 401，且不会请求 Cloud token endpoint。

## 预期与实际文件对比

预期涉及 Cloud 登录态、Cloud redirect、local callback、对应测试与 requirement/verification 文档。实际变更位于上述范围内；未修改 OAuth client 注册、Cloud token endpoint 协议、local auth middleware、设备连接模型或生成代码。

## 验收标准

- [x] Cloud 未认证时不会因 Local Storage 中存在 token 而自动回到 `/oauth2/authorize`。
- [x] Cloud token 经服务端确认无效时从浏览器 token store 和 Local Storage 清除。
- [x] 成功 Cloud 登录后保留完整 OAuth2 authorization request 并继续授权。
- [x] local callback 在没有 local token 时，先获取 local Agent JWT，再调用受保护 exchange API。
- [x] OAuth state 无效或 local token 获取失败时，不继续 exchange。
- [x] 无 local JWT 的 exchange 继续返回 401，且不会向 Cloud 发起 token exchange。
- [x] callback 不负责 Cloud connect。

## 验证命令

| 命令 | 结果 |
| --- | --- |
| `yarn --cwd web test` | 通过：12 个 test files、61 个 tests。 |
| `yarn --cwd web typecheck` | 通过。 |
| `yarn --cwd web lint` | 通过。 |
| `yarn --cwd web format` | 通过。 |
| `go test ./cmd/... ./internal/...` | 通过。 |
| `git diff --check` | 通过。 |
| `task check` | 失败：前端 typecheck、lint:fix、format:fix 通过；backend `golangci-lint run` 报告既有未使用函数 `internal/agent/api/handler/errors.go:102` 的 `decodeJSONStructRequest`。该函数不属于本次 OAuth2 变更。 |

## 生成文件与格式化范围

已检查 `task check` 的格式化配置：

- 前端 Prettier 的 `web/.prettierignore` 忽略 `src/gen`；ESLint 的 `web/eslint.config.js` 忽略 `src/gen/proto/**`。
- Go `golangci-lint fmt` 与 lint 配置在 `.golangci.yml` 中排除 `internal/gen/proto/`。
- `Taskfile.yml` 的 `task check` 使用上述前端与 Go 工具，因此不会格式化或检查生成的 proto 输出目录。

## 范围偏差与风险

- `task check` 会执行 `lint:fix` 与 `format:fix`，即使前端文件没有实际格式差异也具有写入能力；此次运行输出全部为 `unchanged`。
- `task check` 不能作为全绿结论，因为 Go lint 被既有未使用函数阻断；完整 Go unit suite、前端 suite 与本次功能相关的 handler tests 均已通过。
- 尚未在浏览器执行人工端到端复验。应清除 `termbridge_local_token` 后发起 OAuth，确认 Network 顺序为 `/local-api/auth/login` 后接带 Bearer token 的 `/local-api/cloud/oauth/exchange`，并确认 Cloud 收到 `/cloud-api/oauth2/token`。

## 结论

实现与已接受 Requirement 对齐；自动化测试、类型检查、格式检查、完整 Go unit suite 和 diff 检查均通过。`task check` 的 backend lint 仅因本次范围外的既有未使用函数失败，需在独立任务中处理后才能取得该聚合检查的全绿结果。
