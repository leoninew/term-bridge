# 基础用户系统注册、登录与邮箱用户改密计划

最后修改时间: 2026-06-27 14:21:31

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；计划 / Plan。

## Basis

- Requirement: `docs/requirement/20260626-basic-user-system-auth.md`，Review status: Accepted
- Spec: `docs/spec/20260626-basic-user-system-auth.md`，Review status: Accepted
- Long-term identity requirement: `docs/requirement/20260623-long-term-identity-device-enrollment.md`
- Product shape analysis: `docs/analyze/20260626-user-system-product-shape.md`
- Reference for DB / Google OAuth: `D:\SourceCodes\mywork\pomelo-orbit\backend-go`
- Migration framework: `https://github.com/pressly/goose`
- Reference for Resend: `D:\SourceCodes\mywork\typing-island\backend`

## Implementation strategy

本轮一步完成，不再拆成多个 SpecFlow feature。实现仍按层次推进，避免在一个提交范围里失控：

1. 配置模型。
2. DB / goose migration 基础设施。
3. 用户与身份 repository。
4. Auth application service。
5. Resend 与 Google OAuth adapter。
6. JWT claims 迁移。
7. `/api/auth/*` routes 与 Agent tunnel auth。
8. 前端登录/注册/验证/重置密码入口。
9. 配置示例、README 和验证。

本轮不做：

- refresh token / revoke list。
- device enrollment / device token。
- tenant / org / RBAC。
- account linking / OAuth linking。
- 旧 `/api/login`、`/api/me`、`/api/logout` 向后兼容。

当前开发验证以 SQLite 为主。MySQL migration 验证由用户在 Verification 阶段提供测试迁移脚本或环境后执行。

## Key design decisions carried into implementation

1. 本地 / 远程 Gate 模式由 `agent.connect_url` 与 `gate.listen_url` 归一化后是否一致判定。
2. `admin/admin` 写入配置，作为本地模式 shortcut 使用，不写入用户表。
3. 远程 Gate 模式禁用 `admin/admin`。
4. Agent tunnel credential 本轮使用同一认证方式。
5. email verification / password reset 验证码为随机 6 位 `A-Z0-9`。
6. 验证码 TTL 2 分钟，重发冷却 1 分钟，最多 3 次错误。
7. 邮箱验证后才可以登录。
8. Google OAuth 真实支持，配置由部署者提供。
9. Resend 真实发送邮件，配置由部署者提供。
10. 远程 Gate 模式缺 Google OAuth / Resend / DB 必要配置时启动失败。
11. JWT 复用现有 HS256 能力，但 `sub` 改为 user id。
12. Migration 框架使用 `pressly/goose`。

## Implementation steps

### Step 1: 配置模型扩展与模式判定

目标：让后续基础设施可以从统一配置读取 DB、auth、Google OAuth、Resend 和验证码策略。

涉及文件：

- `internal/infrastructure/config/config.go`
- `internal/infrastructure/config/config_test.go`
- `.termbridge.default.yaml`
- `.env.example`

计划：

1. 新增配置结构：
   - `DatabaseConfig`
   - `Auth.LocalAdmin`
   - `Auth.Google`
   - `Auth.JWTTTL`
   - `Auth.PasswordPolicy`
   - `Auth.CodePolicy`
   - `ResendConfig`
2. 保留现有 Auth 语义，但重构为 `auth.local_admin`，避免误认为正式用户凭据。
3. 新增本地 / 远程模式派生函数：
   - 归一化 `gate.listen_url`。
   - 归一化 `agent.connect_url`。
   - `agent.connect_url` 为空时继续默认等于 `gate.listen_url`。
   - 二者相等为 local mode，不等为 remote mode。
4. 配置校验：
   - `database.driver` 只能为 `sqlite` 或 `mysql`。
   - SQLite 需要 path。
   - MySQL 需要 DSN。
   - `jwt.secret_key` 仍必填。
   - remote mode 缺 `auth.google.*`、`resend.*`、DB 必需配置时返回 config error。
5. 将新增配置加入 `configKeys()` allowlist。
6. 更新默认配置与 `.env.example`。

测试：

- 默认配置为本地模式。
- `agent.connect_url == gate.listen_url` 判定 local。
- `agent.connect_url != gate.listen_url` 判定 remote。
- remote 缺 Google OAuth 配置失败。
- remote 缺 Resend 配置失败。
- SQLite / MySQL 配置校验。
- env 覆盖嵌套配置。

### Step 2: DB / goose migration 基础设施

目标：引入后端持久化存储，支持 SQLite / MySQL 与 migration。

新增文件：

- `internal/infrastructure/database/db.go`
- `internal/infrastructure/database/dialect.go`
- `internal/infrastructure/database/migrator.go`
- `internal/infrastructure/database/migrations/migrations.go`
- `internal/infrastructure/database/migrations/sqlite/*.sql`
- `internal/infrastructure/database/migrations/mysql/*.sql`
- `internal/infrastructure/database/*_test.go`

依赖：

- Migration：`github.com/pressly/goose/v3`。
- SQLite driver：优先使用 `modernc.org/sqlite`，保持 Windows / CGO 友好。
- MySQL driver：`github.com/go-sql-driver/mysql`。
- SQL helper：如有必要使用 `github.com/jmoiron/sqlx`，但不强制；能保持标准库 `database/sql` 清晰则优先减少依赖。

计划：

1. 实现 `Open(cfg)`：
   - SQLite 自动创建父目录。
   - SQLite 设置 WAL、foreign_keys、busy_timeout。
   - SQLite `SetMaxOpenConns(1)`。
   - MySQL DSN + 连接池。
   - `Ping` 验证连接。
2. 实现 dialect helper：
   - 当前时间表达式。
   - placeholder / quote / datetime 差异封装。
3. 使用 goose：
   - 通过 `go:embed` 嵌入 migration 文件。
   - 按 driver 选择 `sqlite` 或 `mysql` 目录。
   - 调用 goose Up。
   - 保留 migration status / version 查询能力，便于 verification。
4. 迁移文件第一版创建认证表：
   - `users`
   - `user_identities`
   - `auth_codes`
   - `email_delivery_logs`
5. SQLite / MySQL migration 文件保持同版本同语义。

测试：

- SQLite temp file open。
- SQLite PRAGMA 生效。
- goose migration up。
- migration version/status。
- SQLite migration 后关键表存在。
- MySQL migration 文件存在且与 SQLite 版本一致。
- MySQL 真实迁移由用户在 Verification 阶段提供脚本 / 环境后执行。

### Step 3: Repository 层

目标：让应用层通过 repository 操作用户、身份、验证码和邮件发送日志。

新增文件：

- `internal/infrastructure/repository/auth/model.go`
- `internal/infrastructure/repository/auth/user_repository.go`
- `internal/infrastructure/repository/auth/code_repository.go`
- `internal/infrastructure/repository/auth/email_log_repository.go`
- `internal/infrastructure/repository/auth/*_test.go`

计划：

1. 定义模型：
   - `User`
   - `UserIdentity`
   - `AuthCode`
   - `EmailDeliveryLog`
2. Repository 能力：
   - create user + identity in transaction。
   - find user by email。
   - find identity by provider + subject。
   - update password hash。
   - mark email verified。
   - update last_login_at。
   - create auth code。
   - invalidate old auth codes。
   - find latest valid auth code。
   - increment auth code attempts。
   - mark auth code used。
   - count recent code send events。
   - save email delivery log。
3. SQL 写法避免 SQLite/MySQL 差异泄漏到应用层。

测试：

- SQLite temp file 下 repository CRUD。
- 唯一约束：email 冲突、provider+subject 冲突。
- code TTL / used / attempts 行为。
- resend cooldown 查询。

### Step 4: Password / code / JWT 基础服务

目标：完成认证服务需要的基础能力。

涉及文件：

- `internal/transport/http/gatewayapi/auth/jwt.go`
- `internal/transport/http/gatewayapi/auth/auth.go`
- 新增 `internal/application/auth/password.go`
- 新增 `internal/application/auth/code.go`

计划：

1. Password hasher：
   - 使用 bcrypt。
   - hash password。
   - constant-time compare 由 bcrypt 校验。
2. Code generator：
   - 6 位 `A-Z0-9`。
   - crypto/rand。
   - DB 存 hash，不存明文。
3. JWT claims 调整：
   - `sub=user_id`。
   - `email`。
   - `provider`。
   - `iat`。
   - `exp`。
4. JWT TTL 从配置读取。
5. 保留 Bearer token middleware，但改为返回 user claims / user id 语义。
6. 明确 query token 仍可用于 WebSocket，但日志需要脱敏。

测试：

- JWT sign/verify 新 claims。
- expired token。
- invalid signature。
- code 长度和字符集。
- code hash 校验。
- bcrypt verify。

### Step 5: Resend 邮件发送基础设施

目标：发送 email verification 和 password reset 验证码。

新增文件：

- `internal/infrastructure/email/resend.go`
- `internal/infrastructure/email/templates.go`
- `internal/application/auth/email_sender.go` 或等效 interface
- `internal/infrastructure/email/*_test.go`

计划：

1. 参考 `typing-island/backend`，封装 Resend HTTP API：
   - POST `https://api.resend.com/emails`
   - Bearer API key。
   - from / to / subject / html。
2. Sender 返回：
   - message id。
   - success。
   - response body。
3. response body 截断保存。
4. 邮件模板支持：
   - `email_verification`
   - `password_reset`
5. Application service 负责：
   - 生成 code。
   - 发送邮件。
   - 记录 delivery log。
   - 发送失败不产生可用 code，或立即标记 code invalid。

测试：

- unit test 使用 fake sender。
- HTTP client 层使用 httptest mock Resend。
- 发送失败仍落日志。
- 不访问真实 Resend。

### Step 6: Auth application service

目标：实现注册、邮箱验证、登录、改密、password reset、Google callback 的业务逻辑。

新增文件：

- `internal/application/auth/service.go`
- `internal/application/auth/oauth_google.go`
- `internal/application/auth/errors.go`
- `internal/application/auth/service_test.go`

计划：

1. Register email user：
   - normalize email。
   - validate password policy。
   - 检查 email 是否已使用。
   - 创建 pending user + email identity。
   - 发送 verification code。
2. Verify email：
   - 校验 code。
   - attempts + expiry + used。
   - mark email verified + enabled。
3. Login：
   - 本地模式允许 local admin config credential。
   - 远程模式拒绝 local admin。
   - email 用户必须 verified/enabled。
   - 签发 JWT。
4. Change password：
   - JWT user 必须是 email provider。
   - 校验当前密码。
   - 更新 hash。
5. Password reset request：
   - 泛化返回，避免账号枚举。
   - verified email user 存在才发送 code。
6. Password reset confirm：
   - 校验 code。
   - 更新 password hash。
7. Google OAuth：
   - 生成 auth URL + state。
   - callback 校验 state。
   - 交换 code。
   - 获取 userinfo。
   - provider+sub 查找。
   - email 冲突则报错。
   - 新建 google user。
   - 签发 JWT。
8. Agent tunnel credential verify：
   - 使用同一 credential verifier。
   - local admin 仅本地模式可用。
   - email/password 用户可用。
   - Google 用户无 password，不可用于 Basic Auth。

测试：

- email register -> verify -> login。
- 未验证 email 不能登录。
- duplicate email。
- Google same email conflict。
- local admin local mode 可登录。
- local admin remote mode 被拒绝。
- reset request 不暴露邮箱存在性。
- reset confirm 后旧密码失效。
- code TTL / attempts / resend cooldown。
- fake Google OAuth client。
- fake email sender。

### Step 7: HTTP API routes

目标：公开 `/api/auth/*`，移除旧认证入口。

涉及文件：

- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/server_test.go`
- `internal/transport/http/gatewayapi/auth/*`
- `internal/transport/http/gatewayapi/tunnel_test.go`

计划：

1. 移除或停止注册：
   - `POST /api/login`
   - `POST /api/logout`
   - `GET /api/me`
2. 新增：
   - `POST /api/auth/login`
   - `POST /api/auth/logout`
   - `GET /api/auth/me`
   - `POST /api/auth/register`
   - `POST /api/auth/email/verify`
   - `POST /api/auth/email/verification/resend`
   - `POST /api/auth/password/change`
   - `POST /api/auth/password-reset/request`
   - `POST /api/auth/password-reset/confirm`
   - `GET /api/auth/google`
   - `POST /api/auth/google/callback`
3. `/api/auth/me` 返回 user + capabilities。
4. Protected routes 继续使用 JWT middleware。
5. Agent tunnel Basic Auth 改用 AuthService verifier。
6. 错误响应语义统一：
   - invalid credentials。
   - email not verified。
   - email already used。
   - code invalid or expired。
   - provider unsupported。
7. 日志：
   - request log / API error log 不记录 raw password/code/token。
   - query token 脱敏。

测试：

- 旧 `/api/login` 返回 404 或不再可用。
- 新 `/api/auth/login` 正常。
- `/api/auth/me` protected。
- logout 前端清 token，后端返回 success。
- protected `/api/devices` 使用新 JWT。
- terminal WebSocket query token 仍可用。
- Agent tunnel local admin local mode 可用。
- Agent tunnel remote mode 禁用 local admin。

### Step 8: app wiring

目标：在 serve 入口组装 DB、migration、repository、auth service、email sender、Google OAuth client。

涉及文件：

- `internal/app/app.go`
- `internal/app/app_test.go`

计划：

1. `runServe`：
   - Load config。
   - EnsureLocalIdentity 仍处理 device id/name。
   - Open DB。
   - Run goose migrations。
   - Build repositories。
   - Build AuthService。
   - Build EmailSender。
   - Build Google OAuth client。
   - Build gateway handler。
2. local / remote mode 派生结果传给 AuthService。
3. remote mode 启动校验放在 config load 或 app bootstrap，避免服务半启动。
4. 测试使用 SQLite temp DB + fake email / OAuth。

### Step 9: Frontend auth flow migration

目标：将前端迁移到 `/api/auth/*`，新增注册、验证、重置密码和 Google 登录入口。

涉及文件：

- `web/src/features/gateway/api.ts`
- `web/src/features/api/client.ts`
- `web/src/features/sessions/api.ts`
- `web/src/store/gateway.ts`
- `web/src/router/index.ts`
- `web/src/views/LoginView.vue`
- `web/src/components/session/LoginPanel.vue`
- 新增：
  - `web/src/views/RegisterView.vue`
  - `web/src/views/VerifyEmailView.vue`
  - `web/src/views/ForgotPasswordView.vue`
  - `web/src/views/ResetPasswordView.vue`
  - `web/src/views/AccountSecurityView.vue` 或对应组件

计划：

1. API endpoint 从 `/api/login` 等迁移到 `/api/auth/*`。
2. Store 用户状态从 username 扩展为 user object + capabilities。
3. 登录页：
   - email/password。
   - Google button。
   - register link。
   - forgot password link。
   - local admin shortcut 只在 backend capabilities 允许时显示。
4. 注册页：
   - email/password。
   - 提交后进入验证码页。
5. 验证页：
   - email + code。
   - resend code。
6. Forgot/reset：
   - request reset code。
   - confirm code + new password。
7. Account security：
   - email provider 展示改密。
   - Google / local_admin 不展示或提示不可用。
8. Router guard 使用新 token 和 `/api/auth/me`。

测试：

- 前端 API tests 更新。
- gateway store 登录、登出、me 初始化。
- terminal ws token 获取仍可用。
- 注册 / 验证 / 重置密码组件基本交互。

### Step 10: Docs and examples

涉及文件：

- `.env.example`
- `.termbridge.default.yaml`
- `README.md`
- `docs/verification/20260626-basic-user-system-auth.md` 后续 Verification 阶段创建

计划：

1. README 只保留开发入口，不写成完整部署文档。
2. `.env.example` 增加：
   - DB。
   - Google OAuth。
   - Resend。
   - auth code policy。
   - local admin。
3. 默认配置保持 local 友好。
4. 远程 Gate 示例明确必要配置缺失会启动失败。

## Files to change

### Backend core

- `go.mod`
- `go.sum`
- `internal/infrastructure/config/config.go`
- `internal/infrastructure/config/config_test.go`
- `internal/app/app.go`
- `internal/app/app_test.go`

### New DB / migration

- `internal/infrastructure/database/db.go`
- `internal/infrastructure/database/dialect.go`
- `internal/infrastructure/database/migrator.go`
- `internal/infrastructure/database/migrations/migrations.go`
- `internal/infrastructure/database/migrations/sqlite/v0.1.0__auth_schema.sql`
- `internal/infrastructure/database/migrations/mysql/v0.1.0__auth_schema.sql`
- `internal/infrastructure/database/*_test.go`

### New repositories / auth application

- `internal/infrastructure/repository/auth/*`
- `internal/application/auth/*`

### HTTP transport

- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/errors.go`
- `internal/transport/http/gatewayapi/auth/auth.go`
- `internal/transport/http/gatewayapi/auth/jwt.go`
- `internal/transport/http/gatewayapi/auth/*_test.go`
- `internal/transport/http/gatewayapi/server_test.go`
- `internal/transport/http/gatewayapi/tunnel_test.go`
- `internal/transport/http/gatewayapi/terminal_test.go`

### Agent client

- `internal/application/agent/client.go`
- `internal/application/agent/client_test.go`

### Frontend

- `web/src/features/gateway/api.ts`
- `web/src/features/api/client.ts`
- `web/src/features/sessions/api.ts`
- `web/src/store/gateway.ts`
- `web/src/router/index.ts`
- `web/src/views/LoginView.vue`
- `web/src/components/session/LoginPanel.vue`
- new auth views/components as needed
- related frontend tests

### Config / docs

- `.termbridge.default.yaml`
- `.env.example`
- `README.md`

## Verification plan

### Go unit / integration

Run:

```bash
go test ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
```

Expected coverage:

- config load / validation。
- local vs remote mode derivation。
- SQLite DB open。
- goose migrations。
- repository。
- auth service。
- JWT。
- HTTP auth routes。
- Agent tunnel auth。
- request log / error log redaction for token/password/code。

### MySQL migration verification

当前开发默认使用 SQLite。

MySQL migration verification 在 Verification 阶段由用户提供测试迁移脚本或 MySQL 环境后执行。未提供前不作为普通本地验证必跑项，但 migration 文件必须同步提交并接受结构检查。

### Web checks

Run inside `web`:

```bash
yarn test
yarn typecheck
yarn lint
yarn build
```

Expected coverage:

- auth API client。
- gateway store。
- router guard。
- login/register/verify/reset UI behavior。
- build succeeds without Google OAuth / Resend runtime secrets.

### Manual / mocked external-provider verification

- Google OAuth should be tested via fake OAuth client in automated tests.
- Resend should be tested via fake EmailSender / mocked HTTP client in automated tests.
- Real Google OAuth and Resend require user-provided credentials and are not required for automated CI unless configured.

## Risks and mitigations

1. **Large diff risk**
   - Mitigation: implement in layers; run Go tests after backend foundation before frontend migration.

2. **DB and auth coupling risk**
   - Mitigation: keep DB infrastructure generic; keep auth repository under auth namespace.

3. **Agent tunnel security debt**
   - Mitigation: document same-credential behavior as transitional; do not present it as final device credential model.

4. **Local admin remote exposure risk**
   - Mitigation: AuthService must reject local_admin in remote mode regardless of config.

5. **JWT migration breakage**
   - Mitigation: update all token consumers together; tests for `/api/devices/**` and terminal ws.

6. **Token/code leakage risk**
   - Mitigation: redact password/code/token fields in request/error logs before verification acceptance.

7. **Google OAuth / Resend external dependency risk**
   - Mitigation: config validation in remote mode, fake clients in tests.

8. **SQLite/MySQL divergence risk**
   - Mitigation: maintain parallel migration files and repository tests with dialect helpers; MySQL migration verified when user provides environment/script.

## Implementation feedback / adjustments

Implementation 与人工调试阶段产生以下计划调整：

1. **Task runner 迁移**：用户后续要求使用 Go Taskfile 替换 justfile，实际新增 `Taskfile.yml` 并删除 `justfile`；README 中开发命令同步为 `task ...`。
2. **Google OAuth 本地入口**：用户要求本地模式也暴露 Google OAuth，实际 login/register 均增加 Google 入口与 `/auth/google/callback` 前端 route。
3. **前端认证初始化收敛**：原计划只写“router guard 使用新 token 和 `/api/auth/me`”，实际进一步收敛为默认业务路由需要认证、认证入口路由跳过全局 guard、`/` 首页自行分流、`initializeAuth()` 只负责 `/api/auth/me` 且同 token 幂等。
4. **SQLite 时间存储修正**：SQLite auth 时间列按 RFC3339Nano 字符串写入，不兼容旧脏数据；本地旧 DB 需要清理或重建。
5. **Terminal close 容错**：关闭会话时 live PTY handle 缺失改为 warn + stopped summary，避免 upstream 502。
6. **注册事务边界风险**：当前 email register 先创建用户再发送验证码；若 sender 未配置或发送失败，会出现 users 已写入但 `auth_codes` / `email_delivery_logs` 为空或不完整。后续需要把用户创建、code 生成和发送日志语义重新收敛，至少避免部分成功后用户无法重新注册。
7. **日志脱敏缺口**：API error log query redaction 已有；普通 request log middleware 仍会记录 raw query/body，已在人工验证中看到 WebSocket query token 明文，需要修正后再进入最终安全验收。

## Rollback plan

If implementation fails before completion:

1. Revert code changes in DB/auth/frontend layers together; do not leave half-migrated auth endpoints.
2. Restore old fixed login only if the user explicitly approves rollback, because Spec says old `/api/login` compatibility is intentionally removed.
3. If migrations were run locally, remove the new auth DB file in local dev only after confirming it contains no user data to preserve.
4. Do not delete or reset git state without explicit user authorization.

## Blockers / dependencies

Implementation requires adding Go dependencies for DB, migration and auth support, likely including:

- `github.com/pressly/goose/v3`
- SQLite driver
- MySQL driver
- OAuth2 client package if not hand-writing Google flow

Dependency changes must be reviewed in `go.mod` / `go.sum` and justified by implementation.

No real Google OAuth / Resend credentials are required for automated tests, but real manual verification requires user-provided config.

## User review notes

用户最新决策已纳入本计划：

- 一步完成，不拆多个 SpecFlow feature。
- Migration 框架选取 `https://github.com/pressly/goose`。
- Agent tunnel 本轮继续用同一认证方式，风险接受。
- `/api/login` / `/api/me` / `/api/logout` 不向后兼容。
- 当前使用 SQLite 开发；MySQL 测试迁移脚本由用户在验证阶段提供。
- README / `.env.example` 更新保持开发说明，不写完整部署文档。
