# Cloudflare Turnstile 登录与注册保护验证
最后修改时间: 2026-07-10 23:34:48

Review status: Accepted

## 需求对齐

按 `docs/requirement/20260710-cloudflare-turnstile-registration.md` 核对。

- Cloud 邮箱密码登录和注册请求均新增 `turnstile_token`；登录另新增 `csrf_token`，由 proto 生成的 Go/TypeScript DTO 传递。
- Cloud Handler 在调用原有 `AuthService.Login` 或 `AuthService.Register` 前执行安全校验；登录顺序为 CSRF 单次消费后再校验 Turnstile。
- Turnstile 服务端使用 Cloudflare Siteverify HTTPS endpoint。失败、空 token、非 200、非成功响应、生产 hostname 不一致均 fail-closed。
- production 配置要求 Turnstile site key、secret key 和可解析为 HTTPS hostname 的 `cloud.public_url`；配置加载阶段会拒绝不满足条件的配置。
- development 配置提供 Cloudflare 官方 test site key / secret key；生产 baseline 只保留空占位。
- 登录与注册页面复用 `TurnstileChallenge`；挑战成功前邮箱密码提交不可用，挑战失效或错误会清空 token。登录请求前即时获取 CSRF token，请求结束后重置 Turnstile。
- Google OAuth 登录/注册路径未接入 Turnstile 或 CSRF，符合非目标范围。
- HTTP request log 不记录 Cloud 登录/注册请求体；CSRF token 发放响应体也不记录，避免 password、Turnstile token 与 CSRF token 进入日志。

## 实际改动摘要

- `configs/config.yaml`：增加生产 Turnstile 空占位与安全注释。
- `configs/config.development.yaml`：增加 Cloudflare 官方测试凭据。
- `internal/shared/infrastructure/config/config.go`：加载、规范化、绑定环境变量并验证 `cloud.turnstile.*`；production 要求 HTTPS `cloud.public_url`。
- `cmd/termbridge/app/app.go`、`internal/cloud/application/bootstrap/{config.go,server.go}`：将环境和 Turnstile 配置注入 Cloud bootstrap，生产从 `cloud.public_url` 解析 expected hostname。
- `proto/termbridge/cloud/v1/auth.proto` 及生成文件：扩展登录/注册请求，新增公开 site-key 和 CSRF-token 响应。
- `internal/cloud/api/handler/auth_security.go`：实现 Siteverify verifier、生产 hostname 比对和内存型单次 CSRF token service。
- `internal/cloud/api/handler/server.go`：新增 `/cloud-api/auth/turnstile/config`、`/cloud-api/auth/login/csrf`，并在登录/注册业务调用前执行安全检查。
- `internal/shared/api/middleware/requestlog/*`：针对 Cloud 登录、注册、CSRF 发放路径增加敏感请求/响应日志隔离。
- `web/src/components/cloud/TurnstileChallenge.vue`：加载 Turnstile 脚本、显式渲染 widget，处理成功、过期、错误与 reset。
- `web/src/views/cloud/{LoginView,RegisterView}.vue`、`web/src/features/cloud/api.ts`、`web/src/store/cloudAuth.ts`、`web/src/components/session/LoginPanel.vue`、`web/src/i18n.ts`：完成挑战、CSRF 获取、提交禁用与用户反馈接入。
- `internal/cloud/api/handler/auth_security_test.go`、`internal/shared/infrastructure/config/config_test.go`、`internal/shared/api/middleware/requestlog/logging_test.go`：补充 Handler、配置和敏感日志业务语义测试。

## 预期与实际文件对比

预期文件覆盖配置、Cloud bootstrap/handler、安全组件、认证 proto/生成代码、HTTP 日志保护、登录/注册前端与测试。实际改动覆盖上述区域。

工作区同时包含 command-shortcuts 及其生成 proto、路由、前端页面等无关变更；本次验证未将它们纳入 Turnstile 交付范围，也未尝试回退或修改这些并行改动。

## 验收标准

- [x] 登录和注册请求契约包含 `turnstile_token`，登录还包含 `csrf_token`，并已生成 Go / TypeScript DTO。
- [x] 登录在凭据认证和 JWT 签发前消费 CSRF token、校验 Turnstile；注册在创建账户前校验 Turnstile。
- [x] 空/无效 CSRF、空/被拒绝的 Turnstile 会阻止对应 AuthService 调用；测试验证不会执行凭据认证或账户创建。
- [x] Siteverify 使用 Cloudflare HTTPS endpoint；secret key 不在前端公开配置端点或 DTO 响应中出现。
- [x] production 从 `cloud.public_url` 取得 expected hostname，Siteverify hostname 缺失或不匹配时被拒绝；非 production 不强制 hostname。
- [x] 登录和注册 UI 在无挑战 token 时禁用邮箱密码提交；完成、过期、错误、reset 和提交结束都会清理 token。
- [x] 验证组件有标题、辅助说明和 `role="alert"` 的失败反馈，并使用现有主题变量、边框、间距和响应式宽度约束。
- [x] development 配置使用官方测试凭据；生产 baseline 未包含真实凭据。
- [x] production 配置缺失 site key / secret key 或未提供 HTTPS `cloud.public_url` 时加载失败。
- [x] Cloud 登录/注册 request body 与 CSRF 发放 response body 不再进入 HTTP 日志。
- [ ] 未新增前端组件自动化测试：当前 Web 测试栈没有 Vue component mount / DOM script lifecycle 基础设施。因此 Turnstile 脚本加载、callback、expiry/error/reset 的浏览器行为仅由实现审查与前端静态检查覆盖，未满足 Requirement 中“增加前端测试”的完整要求。
- [ ] 未执行真实浏览器/Cloudflare Siteverify 端到端验证；开发者需以 development 配置手动完成一次测试 challenge、登录和注册流程。

## 验证命令

| 命令 | 结果 |
| --- | --- |
| `task proto` | 通过：重新生成 Go 与 TypeScript protobuf DTO。 |
| `go test ./internal/cloud/api/handler ./internal/cloud/application/bootstrap ./internal/shared/api/middleware/requestlog ./internal/shared/infrastructure/config ./cmd/termbridge/app` | 通过。 |
| `yarn --cwd web eslint <本次认证相关前端文件>` | 通过。 |
| `yarn --cwd web prettier --check <本次认证相关前端文件>` | 通过。 |
| `git diff --check` | 通过。 |
| `yarn --cwd web typecheck` | 失败：`web/src/views/cloud/SessionsView.vue` 与 `web/src/views/local/SessionsView.vue` 的 `SessionRuntimeApi` 缺少并行 command-shortcuts 工作引入的 `ShortcutRuntimeApi` 方法；与 Turnstile 认证变更无直接关系。 |

## 范围偏差与风险

- 完整前端 typecheck 被工作区并行的 shortcuts 变更阻塞，不能作为全绿结论；本次认证相关前端文件的 ESLint、Prettier 均通过。
- Frontend 未具备现成 DOM 组件测试能力，导致 Turnstile 的外部脚本生命周期未有自动化覆盖；应在后续单独建立 Vue DOM test harness 后补充。
- CSRF token 是单实例内存态、单次使用、10 分钟 TTL、最大 1024 个；多实例 Cloud 部署需会话亲和或共享短期存储，否则 token 可能在不同实例间无法消费。
- 生产环境必须在 Turnstile Dashboard 中配置与 `cloud.public_url` 相同的 hostname；不一致将按安全策略阻断用户登录和注册。
- 未执行 live Cloudflare 验证，避免本次验证过程使用外部生产凭据或改变账户数据。

## 未完成项

1. 为 `TurnstileChallenge.vue` 建立 DOM/component test 基础设施，覆盖脚本成功与失败、token 成功、过期、错误、reset、unmount 和登录 CSRF 获取失败。
2. 在 development 运行 topology 中人工完成一次邮箱登录和注册：确认 Siteverify、CSRF 获取、token 重置、邮箱验证跳转与日志脱敏行为。
3. 解决并行 command-shortcuts 的 `SessionRuntimeApi` / `ShortcutRuntimeApi` 类型契约后，重新运行全量 `yarn --cwd web typecheck`。

## 结论

后端安全边界、配置 fail-closed 策略、DTO、敏感日志保护和前端接入均已完成，并由聚焦 Go 测试、前端 lint/format 与 diff 检查验证。由于缺少前端组件测试和真实浏览器端到端验证，交付存在两项明确的验证缺口；建议在合并/发布前至少完成 development 手动流程验证，并规划补齐前端 DOM 自动化测试。
