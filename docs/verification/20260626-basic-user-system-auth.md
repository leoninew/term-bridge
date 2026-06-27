# 基础用户系统注册、登录与邮箱用户改密验证

最后修改时间: 2026-06-27 14:44:45

Review status: Draft

## Flow mode / Stage

严格模式 / strict；验证 / Verification。

## Basis

- Requirement: `docs/requirement/20260626-basic-user-system-auth.md`，Review status: Accepted
- Spec: `docs/spec/20260626-basic-user-system-auth.md`，Review status: Accepted
- Plan: `docs/plan/20260626-basic-user-system-auth.md`，Review status: Accepted

## Requirement alignment

### 已对齐

- 已引入 Browser 用户认证基础：email/password、Google OAuth callback、JWT session、受保护 API。
- 已引入后端持久化存储：`users`、`user_identities`、`auth_codes`、`email_delivery_logs`、`oauth_states`。
- 已支持 SQLite / MySQL 配置与 migration 文件；运行时会打开 DB 并执行 goose migration。
- email 用户注册后为 pending 状态，邮箱验证后才可登录。
- email verification 与 password reset 使用 6 位 `A-Z0-9` 验证码，DB 保存 hash，不保存明文。
- password reset 请求对不存在或不可 reset 的邮箱返回泛化 accepted，不直接暴露邮箱是否存在。
- email 用户支持修改密码；Google / local admin 不走本地改密主路径。
- Google OAuth 使用真实 `golang.org/x/oauth2/google` flow，callback 获取 Google userinfo，按 `provider=google` + `sub` 识别用户。
- email 与 Google 同邮箱冲突不自动合并，返回邮箱已使用。
- Browser session 复用现有 JWT，claims 迁移到 `sub=user_id`、`email`、`provider`、`iat`、`exp`。
- 本地 / 远程模式按 `agent.connect_url` 与 `gate.listen_url` 归一化后是否一致判定，不依赖请求 URL。
- local admin 为配置 shortcut，不写入用户表；远程 Gate 模式不允许 local admin 登录。
- 旧 `/api/login`、`/api/me`、`/api/logout` 已不再注册；测试迁移到 `/api/auth/*`。
- 登录流程仍不要求用户输入 raw `device_id`。

### 部分对齐 / 后续项

- Requirement 早期条目曾写“本地模式下，用户表中的 `admin/admin` shortcut 可用”，但后续用户决策、Spec 和 Plan 已明确改为“配置 shortcut，不写入用户表”。本次实现按后者执行。
- 前端已增加注册、邮箱验证、忘记密码、重置密码、Google callback 与首页中转页面；登录页和注册页均已暴露 Google OAuth 入口。
- 后端具备 `/api/auth/password/change` 与前端 API client 方法，但仍缺少用户可见的账号安全 UI。
- 自动化测试仍主要覆盖既有网关、配置与前端基础检查；新 auth service、repository、Resend adapter、migration 的专门单元测试未补齐。

## Spec alignment

### 已对齐

- 本地 / 远程 Gate 模式：实现 `config.IsLocalMode` / `config.Mode`，按配置 URL 归一化比较。
- local admin：配置项为 `auth.local_admin.username/password`，仅 local mode 生效，返回虚拟 `local-admin` user。
- 远程启动校验：remote mode 缺 Google OAuth 或 Resend 配置时 `Load` 返回 config error。
- DB / migration：新增 `internal/infrastructure/database`，使用 goose，按 driver 选择 embedded SQLite / MySQL migration。
- 用户模型：实现内部 user id、provider identity、email/google provider 区分。
- 验证码：6 位 `A-Z0-9`、TTL、attempts、resend cooldown、hash 存储、旧 code invalidation、成功后 used。
- Resend：新增 sender adapter，发送结果和 response body 截断写入 `email_delivery_logs`；发送失败会 invalidate code。
- Google OAuth：state 为高熵随机值并持久化，有 TTL 与 one-time use；callback exchange code 并 fetch userinfo。
- JWT：保留 HS256 自实现，扩展 claims，TTL 从 `auth.jwt_ttl` 读取。
- WebSocket token query fallback 保留；API error log 对 query 中敏感字段 redaction 已实现，但普通 request started/completed log 仍存在 raw query/body 脱敏缺口。
- API route shape 已迁移到 `/api/auth/*`。
- Agent tunnel Basic Auth 通过 AuthService verifier；local admin 仅 local mode 可用，email/password 可用，Google 无本地密码不可用于 Basic Auth。
- 前端 API client、store、router、登录页链接与 auth views 已迁移到新接口。

### 偏差 / 不完整

- Spec 推荐 migration history 使用 `__migration_history` 和 checksum mismatch 检查；实际采用 goose 默认 `goose_db_version`，未实现自定义 checksum history。
- Spec 建议 login 页包含 Google login button：当前 login/register 已展示 Google 入口，callback route 可完成 OAuth 返回处理。
- Spec 建议 capabilities gating：当前 Google 入口尚未基于 `/api/auth/me` capabilities 隐藏/禁用，未配置 Google 时仍可能显示不可用入口。
- Spec 建议 `/account/security`；当前未新增该页面。
- Spec 中“本地模式下 Google/Resend 缺失时隐藏或 disabled”依赖前端 capabilities，但当前 login panel/register view 未使用 capabilities 展示/隐藏 Google 或邮件能力。

## Plan alignment

### 已完成

- Step 1 配置模型扩展与模式判定：已完成。
- Step 2 DB / goose migration 基础设施：已完成基础实现和 migration 文件。
- Step 3 Repository 层：已完成集中式 auth repository。
- Step 4 Password / code / JWT 基础服务：已完成在 auth service 与 JWT package 中。
- Step 5 Resend 邮件发送基础设施：已完成基础 adapter。
- Step 6 Auth application service：已完成主要业务流程。
- Step 7 HTTP API routes：已完成 `/api/auth/*` 与旧路由移除。
- Step 8 app wiring：已完成 serve 入口 DB/migration/repository/auth service/sender/OAuth client 组装。
- Step 9 Frontend auth flow migration：已完成 API/store/router/login link/注册验证重置页面的基础迁移。
- Step 10 Docs and examples：已更新 `.env.example`、`.termbridge.default.yaml`、`README.md`。

### 与计划相比的未完成项

- 未新增 `internal/infrastructure/database/*_test.go`、`internal/infrastructure/repository/auth/*_test.go`、`internal/application/auth/service_test.go`。
- 未新增 Resend adapter httptest 单元测试。
- 未实现 `AccountSecurityView.vue`。
- 未运行 MySQL 真实 migration 验证；按 Plan，需要用户提供 MySQL 环境或脚本后执行。
- 未单独执行 `goose status` 类命令；当前通过 `go test` 编译和 app wiring 覆盖基础可用性。

## Actual diff summary

### Config / docs

- `.termbridge.default.yaml`
- `.env.example`
- `README.md`
- `go.mod`
- `go.sum`

新增 DB、auth、local admin、验证码、Google OAuth、Resend 配置示例；引入 goose、SQLite、MySQL、OAuth 依赖。

### Backend

- `internal/app/app.go`
- `internal/application/auth/`
- `internal/infrastructure/config/config.go`
- `internal/infrastructure/config/config_test.go`
- `internal/infrastructure/database/`
- `internal/infrastructure/email/`
- `internal/infrastructure/repository/auth/`
- `internal/transport/http/gatewayapi/auth/auth.go`
- `internal/transport/http/gatewayapi/auth/jwt.go`
- `internal/transport/http/gatewayapi/errors.go`
- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/server_test.go`
- `internal/transport/http/gatewayapi/relay_test.go`
- `internal/transport/http/gatewayapi/tunnel_test.go`

实现 auth service、DB/migration、repository、Resend sender、Google OAuth client、JWT claims、API routes、Agent tunnel auth 和日志脱敏。

### Frontend

- `web/src/features/gateway/api.ts`
- `web/src/store/gateway.ts`
- `web/src/router/index.ts`
- `web/src/components/session/LoginPanel.vue`
- `web/src/i18n.ts`
- `web/src/views/RegisterView.vue`
- `web/src/views/VerifyEmailView.vue`
- `web/src/views/ForgotPasswordView.vue`
- `web/src/views/ResetPasswordView.vue`
- `web/src/views/GoogleCallbackView.vue`
- `web/src/views/HomeView.vue`
- `web/src/views/SessionsView.vue`

迁移到 `/api/auth/*`，新增注册、验证邮箱、忘记密码、重置密码、Google OAuth callback、首页中转入口和基础文案；认证恢复从页面 mounted 收敛到 router guard，业务路由默认要求认证，认证入口路由跳过全局 guard。

### Process docs

- `docs/requirement/20260626-basic-user-system-auth.md`
- `docs/spec/20260626-basic-user-system-auth.md`
- `docs/plan/20260626-basic-user-system-auth.md`
- `docs/verification/20260626-basic-user-system-auth.md`

本 feature 的 SpecFlow 过程文档。

## Expected vs actual changed files

### 符合预期

- 配置、DB、migration、repository、auth application、HTTP transport、frontend gateway store/API/router/views、README 与 env examples 都在计划范围内。
- `go.mod` / `go.sum` 变更符合新增 goose、SQLite、MySQL、OAuth 依赖预期。

### 需要注意

- `docs/analyze/` 当前在工作区中为 untracked，但属于本 feature 早期产品形态分析产物，不是本次 Verification 新增内容。
- `web/dist` 构建产物未出现在当前 `git status --short` 中。

## Acceptance checklist

### Identity model

- [x] 稳定内部 user id。
- [x] 区分 email / Google provider identity。
- [x] Google 使用 provider + subject 绑定。
- [x] email 不作为跨 provider 自动合并依据。
- [x] 普通登录不要求 device id。
- [x] 同邮箱冲突不自动合并。

### Backend storage / migration

- [x] 引入 user / identity / code / delivery log / oauth state 存储。
- [x] 支持 SQLite / MySQL 配置与 schema migration 文件。
- [x] 包含 goose migration 能力。
- [x] 存储配置缺失、连接失败、migration 失败会返回错误。
- [ ] MySQL 真实迁移未运行。
- [ ] migration/repository 专门测试不足。

### Email registration / login

- [x] email + password register。
- [x] 注册后发送 Resend verification code。
- [x] 6 位 `A-Z0-9` code。
- [x] 验证后才可登录。
- [x] password 使用 bcrypt hash。
- [x] 重复 email / Google 同邮箱冲突处理。
- [x] 基础密码规则后端明确。
- [ ] 前端未显式展示完整 password policy。

### Email verification

- [x] Resend sender adapter。
- [x] code TTL / attempts / used / invalidated。
- [x] resend cooldown。
- [x] 远程模式缺 Resend 配置启动失败。

### Password reset / password change

- [x] 后端支持 email 用户改密。
- [x] 改密校验当前密码。
- [x] password reset request 泛化返回。
- [x] reset code TTL / attempts / cooldown / used。
- [x] Google / local admin 不支持本地改密主路径。
- [ ] 前端账号安全改密页面未实现。

### Google OAuth login

- [x] Google OAuth client 使用真实 Google endpoint。
- [x] 首次 Google 登录可创建 Google user。
- [x] 后续可按 Google sub 识别。
- [x] Google 用户无本地密码。
- [x] Google 返回同邮箱冲突不合并。
- [x] 远程模式缺 Google 配置启动失败。
- [x] 登录页和注册页已有 Google OAuth 入口，callback route 可保存 token 并跳转工作台。
- [x] 已使用用户提供的真实 Google OAuth 配置完成一次本地人工登录验证。
- [ ] Google 入口尚未基于 capabilities 做隐藏/禁用。

### Session / access

- [x] Browser session 复用 JWT。
- [x] `/api/auth/me` protected 并返回 user/capabilities。
- [x] `/sessions` 相关既有测试通过。
- [x] WebSocket query token fallback 保留。
- [x] API error log query token/code/password redaction 已实现。
- [ ] 普通 request started/completed log 仍会记录 raw query/body，已观察到 WebSocket query token 明文，需修复。
- [x] Logout 仍是轻量策略：前端清 token，后端不做 revoke list；这符合 Spec。
- [x] 登出后 `/login` 立即展示登录表单，不再卡在检查登录状态。

### Local vs remote mode

- [x] 配置判定 local / remote。
- [x] local admin 配置 shortcut 本地可用。
- [x] remote mode 禁用 local admin。
- [x] README 标注 local admin 为 bootstrap / development shortcut。

### Product UX

- [x] 登录页从 username 文案迁移为 email，并提供注册/忘记密码链接。
- [x] 注册、邮箱验证、忘记密码、重置密码基础页面存在。
- [x] 登录页和注册页已提供 Google OAuth button。
- [ ] Google button 未接 capabilities gating。
- [ ] 登录后当前用户身份的展示依赖 store/API，尚未新增显式账号 UI。

## Command results

### Go

```text
go test ./cmd/... ./internal/...
```

结果：通过。

```text
go vet ./cmd/... ./internal/...
```

结果：通过。

```text
go test ./internal/application/terminal ./internal/application/agent ./internal/transport/http/gatewayapi
```

结果：通过。用于覆盖关闭会话缺失 live PTY handle 的降级行为，以及 agent / gateway 相关路径未回归。

### Web

```text
npm --prefix web run test
```

结果：通过，8 个 test files，36 个 tests。

```text
npm --prefix web run typecheck
```

结果：通过。

```text
npm --prefix web run lint
```

结果：通过。

```text
npm --prefix web run build
```

结果：通过。

构建 warning：

- `node_modules/@vueuse/core/dist/index.js` 中 `/* #__PURE__ */` annotation 位置被 Rolldown 忽略。
- `SessionsView` chunk 超过 500 kB。

这些 warning 来自现有依赖/打包体积，不阻断 build。

### Taskfile

```text
task check
```

结果：通过。`justfile` 已被 `Taskfile.yml` 替换，README 开发命令已迁移到 `task ...`。

```text
yarn --cwd web typecheck && yarn --cwd web lint
```

结果：通过。用于验证后续前端 auth 初始化、路由 guard、登录页、HomeView 调整未引入类型或 lint 回归。

### MySQL / external providers

已人工验证：

- 使用用户提供的真实 Google OAuth 配置完成本地 Google 登录；期间修正了 redirect URL、SQLite 时间存储格式、前端 callback 与 auth 初始化职责。

未完成 / 存疑：

- MySQL 真实 migration 未运行。
- Resend 真实邮件发送链路未形成完整成功证据；当前观察到 email register 可写入 `users`，但 `auth_codes` 与 `email_delivery_logs` 为空，说明注册/验证码/邮件日志事务边界仍需修正或重新验证。

原因：Plan 已约定 MySQL migration verification 需要用户提供环境或脚本；Resend 真实发送虽然已有配置，但当前人工验证暴露出 partial success 状态，需要继续收敛实现。

## Scope deviation

### 扩展但合理

- 新增 `oauth_states` 表，用于高熵 OAuth state 持久化和 one-time use；这是 Google OAuth 安全必要支撑。
- `auth_codes` 增加 `invalidated_at`，用于 resend / failed-send invalidation；符合验证码安全要求。

### 未完成且影响最终验收

- email register / verification 真实链路存在 partial success 风险：用户记录已创建，但 `auth_codes` 与 `email_delivery_logs` 可为空。需要修正事务边界或失败补偿策略。
- 普通 request started/completed log 仍会记录 raw query/body，已观察到 terminal WebSocket query token 明文。

### 未完成但不阻断当前编译/构建

- 缺少新 auth service/repository/migration/email sender 专门测试。
- 前端账号安全改密页面未完成。
- Google OAuth 入口未接 capabilities gating。
- MySQL / Resend 真实外部验证未完整执行。

## Risks

1. **邮箱注册一致性风险**：当前观察到 users 已创建但 `auth_codes` / `email_delivery_logs` 为空，说明注册、验证码生成、邮件发送日志之间存在事务边界或失败补偿问题。该项影响最终验收。
2. **日志敏感信息风险**：普通 request log 仍记录 raw query/body，WebSocket query token 已出现在日志中。该项影响安全验收。
3. **测试覆盖风险**：核心 auth business logic 当前主要靠编译、集成 wiring 和既有 HTTP tests 间接覆盖，缺少针对 code TTL、attempts、cooldown、OAuth conflict、password reset 的专门单元测试。
4. **UI 完整性风险**：后端能力超过当前 UI 暴露范围，尤其 change password 和 capabilities gating。
5. **Migration 风险**：SQLite / MySQL schema 文件存在，但未在真实 MySQL 环境执行。
6. **外部服务风险**：Google OAuth 已本地人工验证；Resend 真实发送仍需在修正注册链路后重新验证。
7. **server.go 可维护性风险**：HTTP handler 本次改动较大，文件可读性下降，后续建议拆分 auth handlers。

## Incomplete items

- 修正 email register / verification 的事务边界或失败补偿，避免 users 已创建但 `auth_codes` / `email_delivery_logs` 为空导致账号卡死。
- 修正普通 request started/completed log 的 query/body redaction，覆盖 token、code、state、password、secret 等敏感字段。
- 为 `internal/application/auth` 增加 service tests。
- 为 `internal/infrastructure/repository/auth` 增加 SQLite repository tests。
- 为 `internal/infrastructure/database` 增加 migration tests。
- 为 `internal/infrastructure/email` 增加 httptest Resend tests。
- 前端补 account/security change password UI。
- 前端 Google OAuth 入口接入 capabilities gating。
- 用户提供 MySQL 环境或脚本后执行 MySQL migration verification。
- 修正注册链路后重新执行真实 Resend 邮件发送验证。

## Conclusion

当前实现已通过 Go tests、Go vet、Web tests、TypeScript typecheck、ESLint、Web build、`task check` 以及针对 terminal close / agent / gateway 的补充测试。核心后端认证链路、配置、migration、JWT、HTTP route、Google OAuth 人工登录、前端 auth flow、Taskfile 迁移与关闭缺失 PTY 降级行为已落地。

本 Verification 结论为：当前实现不建议直接作为单一最终提交合入。主要原因是仍有两项影响最终验收的问题：

1. email register / verification 存在 partial success 风险：`users` 已写入但 `auth_codes` / `email_delivery_logs` 可为空。
2. 普通 request log 仍可能明文记录 WebSocket query token、OAuth code/state 或其他敏感 body/query 字段。

在修正上述两项前，可以将当前工作视为已完成主要功能实现与人工调试阶段，但不应标记为“auth feature complete”。
