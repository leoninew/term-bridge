# 基础用户系统注册、登录与邮箱用户改密规格

最后修改时间: 2026-06-27 14:21:31

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；规格 / Spec。

## Requirement basis

- Requirement: `docs/requirement/20260626-basic-user-system-auth.md`
- Requirement status: Accepted
- Long-term identity basis: `docs/requirement/20260623-long-term-identity-device-enrollment.md`
- Product shape analysis: `docs/analyze/20260626-user-system-product-shape.md`

本 Spec 只设计 Browser 用户注册、登录、邮箱验证、邮箱用户改密、password reset、Google OAuth、后端认证存储与本地/远程 Gate 模式边界。

Device enrollment、Agent device credential、正式 User -> Device 授权、tenant/org/RBAC 不在本轮实现范围内，但本设计不能阻碍后续演进。

## Overview

本轮将当前 Browser PoC 登录从固定 `admin/admin` 演进为正式用户系统：

```text
Browser
  -> Auth API
  -> User / Identity store
  -> JWT
  -> Protected /sessions workbench
```

长期产品模型仍保持：

```text
User -> Device -> Workspace -> Session -> Terminal
```

本轮只建立 User 认证层，不改变 Workspace / Session / Terminal 的主体产品模型。

核心设计方向：

1. 本地 / 远程 Gate 模式由配置关系判定：`agent.connect_url` 与 `gate.listen_url` 一致时为本地模式；不一致时为远程 Gate 模式。
2. 本地模式可使用配置中的 `admin/admin` shortcut。
3. `admin/admin` 不写入用户表，不作为正式用户记录。
4. 远程 Gate 模式禁用 `admin/admin`，并要求 Google OAuth、Resend、DB 必要配置齐全，否则启动失败。
5. 引入 DB 层，支持 SQLite / MySQL 与 embedded migration。
6. 用户身份使用内部 `user_id`，email / Google subject 只是 provider identity。
7. email 用户需要邮箱验证后才能登录。
8. email verification 与 password reset 使用 Resend 发送随机 6 位字母 + 数字验证码。
9. Google OAuth 真实可用；同邮箱冲突不自动合并。
10. Browser session 复用现有 JWT 能力，但 JWT claims 需要从 username 语义迁移到 user id 语义。
11. `/api/login`、`/api/me`、`/api/logout` 不向后兼容；新用户系统使用 `/api/auth/*`。
12. Agent tunnel credential 本轮使用相同认证方式，不引入独立 device credential。

## Design decisions

### 1. 本地 / 远程 Gate 模式判定

不新增 `deployment.mode`。本轮使用现有配置关系判定模式：

```text
local mode  := normalized(agent.connect_url) == normalized(gate.listen_url)
remote mode := normalized(agent.connect_url) != normalized(gate.listen_url)
```

语义：

| 模式 | 判定 | 产品语义 | `admin/admin` | 外部配置缺失行为 |
| --- | --- | --- | --- | --- |
| 本地模式 | `agent.connect_url == gate.listen_url` | self-connected / development / bootstrap | 可用 | Google OAuth / Resend 可缺失，对应入口禁用 |
| 远程 Gate 模式 | `agent.connect_url != gate.listen_url` | Cloud / remote Gate | 不可用 | Google OAuth / Resend / DB 必要配置缺失时启动失败 |

URL normalization 要求：

- 在 config load 后使用已经 trim、去尾斜杠、补默认 agent connect URL 后的值比较。
- 不使用 HTTP request Host、Origin、Forwarded headers、浏览器访问 URL 或反向代理头判断模式。
- 如果 `agent.connect_url` 为空并继承 `gate.listen_url`，则是本地模式。
- Docker / Cloud Gate 示例必须显式设置 `agent.connect_url` 为远端公开地址或与 `gate.listen_url` 不同的远程连接地址，从而进入远程 Gate 模式。

设计原因：

- 复用当前配置语义：本机 self-connected 时 Agent 连接本机 Gate；远程 Gate 时 Agent 连接远端 Gate。
- 避免 URL / Host 被代理、容器端口映射或请求伪造影响安全模式。
- 不额外引入 deployment 配置，减少配置面。

### 2. 本地 `admin/admin` shortcut

`admin/admin` 写入配置，作为本地模式 shortcut 使用，不写入用户表。

推荐配置：

```yaml
auth:
  local_admin:
    username: admin
    password: admin
```

环境变量：

```dotenv
TERMBRIDGE_AUTH__LOCAL_ADMIN__USERNAME=admin
TERMBRIDGE_AUTH__LOCAL_ADMIN__PASSWORD=admin
```

规则：

1. 只在本地模式可用。
2. 远程 Gate 模式即使配置存在，也必须拒绝 `local_admin` 登录。
3. `local_admin` 不出现在用户表中，不参与 email verification、password reset、Google OAuth 或后续 user-device binding。
4. `/api/auth/me` 对 local_admin 登录返回 provider=`local_admin` 的虚拟用户信息，user id 可使用稳定虚拟值，例如 `local-admin`。
5. 该 shortcut 是 bootstrap / development 过渡，不是长期正式身份模型。

### 3. 后端存储与迁移

新增配置：

```yaml
database:
  driver: sqlite # sqlite | mysql
  sqlite:
    path: .termbridge/termbridge.db
  mysql:
    dsn: ""
```

环境变量示例：

```dotenv
TERMBRIDGE_DATABASE__DRIVER=sqlite
TERMBRIDGE_DATABASE__SQLITE__PATH=.termbridge/termbridge.db
TERMBRIDGE_DATABASE__MYSQL__DSN=user:pass@tcp(127.0.0.1:3306)/termbridge?parseTime=true
```

实现参考：`D:\SourceCodes\mywork\pomelo-orbit\backend-go`。

设计：

```text
internal/infrastructure/database/
  db.go              # Open(cfg)
  dialect.go         # driver-specific SQL helpers
  migrator.go        # migration runner + status
  migrations/
    migrations.go    # go:embed sqlite/*.sql mysql/*.sql
    sqlite/*.sql
    mysql/*.sql
```

数据库连接：

- SQLite 使用文件路径，自动创建父目录。
- SQLite 开启：`busy_timeout`、`journal_mode=WAL`、`foreign_keys=ON`。
- SQLite `SetMaxOpenConns(1)`。
- MySQL 使用 DSN，配置连接池。
- `Open` 阶段必须 `Ping`。

Migration：

- 使用 embedded SQL migration。
- SQLite / MySQL 分目录，同版本文件名保持一致。
- 使用 `__migration_history` 记录 filename、checksum、executed_at、execution_time_ms。
- 已执行 migration checksum 不一致时启动失败。
- 应提供测试覆盖 migration up/status/checksum mismatch。

### 4. 用户与身份数据模型

推荐表结构方向：

```text
users
  id
  email_normalized
  display_name
  status                  # pending_verification | enabled | disabled
  email_verified_at
  created_at
  updated_at
  last_login_at

user_identities
  id
  user_id
  provider                # email | google
  provider_subject         # normalized email | google sub
  password_hash            # only email
  oauth_email
  created_at
  updated_at

auth_codes
  id
  user_id nullable
  email_normalized
  purpose                 # email_verification | password_reset
  code_hash
  used_at nullable
  expires_at
  attempts
  created_at

email_delivery_logs
  id
  email_normalized
  purpose
  provider_message_id
  status                  # success | failed
  response_body
  created_at
```

约束：

- `users.id` 是内部用户主键，前端和 JWT 的用户身份以它为准。
- `users.email_normalized` 对非空值唯一，用于同邮箱冲突判断。
- `user_identities(provider, provider_subject)` 唯一。
- Google 用户使用 `provider=google`、`provider_subject=<google sub>`。
- email 用户使用 `provider=email`、`provider_subject=<normalized email>`。
- password 不保存明文，使用 bcrypt hash。
- 验证码不保存明文，保存 hash，并以 TTL、attempts 和 resend cooldown 约束使用。
- `local_admin` 不落表。

同邮箱冲突：

- email 注册时，如果 `users.email_normalized` 已存在，无论来源是 email 还是 Google，都返回“邮箱已使用”。
- Google OAuth callback 时，如果 Google 返回 email 已存在但该用户没有相同 `google sub` identity，则返回“邮箱已使用”，不自动合并。
- 本轮不实现 account linking。

### 5. Email registration and verification

API 流程：

```text
POST /api/auth/register
  -> create pending email user
  -> create email_verification code
  -> send code through Resend
  -> return accepted response

POST /api/auth/email/verify
  -> verify 6-char code
  -> mark email verified and user enabled

POST /api/auth/email/verification/resend
  -> send new verification code with cooldown/rate limits
```

验证码规则：

- 随机 6 位字母 + 数字组合。
- 字符集：`A-Z` + `0-9`。
- 用户输入大小写不敏感。
- TTL：2 分钟。
- 同 email + purpose 重发冷却：1 分钟。
- 同 code 最大错误次数：3 次。
- 验证成功后立即标记 used。
- 新验证码生成后，旧的同 purpose 未使用验证码标记失效，或查询时只接受最新一条。

注册后状态：

- 注册成功但未验证：不能登录。
- 邮箱验证成功后：可以登录。

错误语义：

- 注册同邮箱已存在：提示邮箱已使用。
- 验证码错误 / 过期 / 已使用：提示验证码错误或已过期。
- 发送失败：提示邮件发送失败，请稍后重试。

### 6. Password reset and password change

已登录改密：

```text
POST /api/auth/password/change
Authorization: Bearer <jwt>
body: current_password, new_password
```

规则：

- 仅 email provider 用户可用。
- 必须校验当前密码。
- 新密码通过 password policy 后更新 bcrypt hash。
- Google / local_admin 不进入此主路径。

忘记密码：

```text
POST /api/auth/password-reset/request
body: email
  -> if verified email user exists, send reset code
  -> always return generic accepted response

POST /api/auth/password-reset/confirm
body: email, code, new_password
  -> verify code
  -> update password hash
```

规则：

- 仅 verified email user 可 reset。
- 请求接口不暴露 email 是否存在。
- reset code 使用同样的 6 位字母 + 数字组合。
- reset code TTL：2 分钟。
- reset code 使用后失效。
- 重发冷却：1 分钟。
- 最大错误次数：3 次。

### 7. Google OAuth

配置：

```yaml
auth:
  google:
    client_id: ""
    client_secret: ""
    redirect_url: ""
```

环境变量：

```dotenv
TERMBRIDGE_AUTH__GOOGLE__CLIENT_ID=...
TERMBRIDGE_AUTH__GOOGLE__CLIENT_SECRET=...
TERMBRIDGE_AUTH__GOOGLE__REDIRECT_URL=https://gate.example.com/auth/google/callback
```

实现参考：`D:\SourceCodes\mywork\pomelo-orbit\backend-go`。

推荐 OAuth flow：

```text
GET /api/auth/google
  -> create high-entropy OAuth state
  -> persist state with TTL
  -> return { auth_url }

Frontend redirects browser to auth_url

Google redirects to frontend callback route with code/state

POST /api/auth/google/callback
  -> verify state
  -> exchange code for token
  -> fetch Google userinfo
  -> find by provider=google + sub
  -> if not found, check email_normalized conflict
  -> create google user if no conflict
  -> issue JWT
```

注意：

- OAuth state 不能使用 6 位验证码；必须使用高熵随机值。
- Google 返回 email 与既有账号冲突时，不自动合并。
- 远程 Gate 模式下 Google config 缺失启动失败。
- 本地模式下 Google config 缺失时隐藏 / 禁用 Google 登录入口；实际当前 UI 已在 login/register 暴露 Google 入口，后续需接 capabilities 控制，避免未配置时展示不可用入口。

### 8. Resend email sending

配置：

```yaml
resend:
  api_key: ""
  from_email: ""
```

环境变量：

```dotenv
TERMBRIDGE_RESEND__API_KEY=...
TERMBRIDGE_RESEND__FROM_EMAIL=noreply@example.com
```

实现参考：`D:\SourceCodes\mywork\typing-island\backend`。

推荐接口：

```text
EmailSender.Send(to, subject, html) -> (message_id, success, response_body)
```

设计：

- infrastructure 层封装 Resend HTTP 调用。
- application/auth service 负责渲染模板、记录发送日志、处理业务错误。
- 发送失败也记录 `email_delivery_logs`。
- 发送失败不生成可用验证码，或生成后必须标记为不可用。
- response body 需要截断保存，避免日志/DB 过大。
- 单元测试 mock EmailSender；集成测试不访问真实 Resend。

邮件场景：

- `email_verification`
- `password_reset`

### 9. JWT session

复用现有 JWT 签发/校验能力，但 claims 语义需要升级。

当前 JWT：

```text
sub = username
exp = now + 24h
```

目标 JWT：

```text
sub = user_id
email = email_normalized or empty
provider = email | google | local_admin
exp = now + auth.jwt_ttl
iat = issued_at
```

配置：

```yaml
auth:
  jwt_ttl: 24h
```

本轮不引入 refresh token / revoke list。

Logout 策略：

- 本轮保持轻量：前端清除本地 JWT，后端 logout 返回成功。
- 后端不维护 refresh token、denylist 或服务端 session revoke。
- 后续如需要可加入 token version / session id / denylist。

WebSocket 鉴权：

- 当前 terminal WebSocket 使用 query token fallback。
- 本轮可继续复用，但必须把 query token 视为敏感数据。
- request/error logs 不应记录原始 `token` query 值。当前实现已处理 API error log 的 query redaction；普通 request started/completed log 仍需补齐 query/body redaction，特别是 WebSocket query token 和 OAuth callback code/state。
- 后续更安全方案可以是短期 WebSocket ticket 或 cookie-based auth，本轮不强制实现。

### 10. API route shape

不保留旧 `/api/login`、`/api/me`、`/api/logout` 兼容入口。

新认证 API 使用：

```text
POST /api/auth/login
POST /api/auth/logout
GET  /api/auth/me
POST /api/auth/register
POST /api/auth/email/verify
POST /api/auth/email/verification/resend
POST /api/auth/password/change
POST /api/auth/password-reset/request
POST /api/auth/password-reset/confirm
GET  /api/auth/google
POST /api/auth/google/callback
```

影响：

- 前端 `gateway` store 和 API client 必须迁移到 `/api/auth/*`。
- 后端测试中旧 `/api/login`、`/api/me`、`/api/logout` 预期需要删除或改为 404。
- 文档和 README 不再宣传旧入口。

### 11. Agent tunnel authentication

用户已明确本轮 Agent tunnel credential 使用相同认证方式。

设计：

- 不引入独立 device credential。
- Agent tunnel 继续使用 HTTP Basic Auth 形式传递 credential。
- Basic Auth 后端验证改为调用同一 AuthService credential verifier。
- 本地模式：允许 `local_admin` 配置 credential，即配置中的 `admin/admin`。
- 远程 Gate 模式：不允许 local_admin；允许 email/password 用户作为 Agent tunnel credential。
- Google OAuth 用户没有本地密码，不能直接作为 Basic Auth credential。

风险标注：

- 这仍不是长期 device credential 模型。
- 后续 device enrollment / device token 需求必须替换该路径。
- 本轮只是按用户要求让 Agent tunnel 与 Browser 使用同一认证域，避免同时设计第二套 device credential。

### 12. Frontend UX

新增或调整页面：

```text
/login
  - email/password login
  - Google login button
  - link to register
  - link to forgot password
  - local mode can show local admin shortcut only when backend capabilities says so

/register
  - email/password registration
  - registration success -> verification code page

/verify-email
  - email + code
  - resend code

/forgot-password
  - request reset code

/reset-password
  - email + code + new password

/account/security
  - change password for email users
```

登录后：

- 显示当前用户身份。
- 继续进入现有 `/sessions`。
- 不要求用户输入 raw `device_id`。

Provider gating：

- 远程模式：Google / Resend / DB 缺配置会启动失败，因此登录页不应出现半可用状态。
- 本地模式：Google / email mail flows 可因缺配置隐藏或 disabled；`admin/admin` 可作为本地 shortcut。

前端通过 `/api/auth/me` 或 capabilities endpoint 获取：

```text
mode: local | remote
providers: email, google, local_admin
features: registration, password_reset, email_verification
```

这样前端不会靠 URL 猜测是否显示本地 shortcut。

### 13. Startup validation

启动校验规则：

```text
Always:
  - jwt.secret_key required
  - database.driver valid
  - selected database config valid
  - migrations must run successfully
  - local/remote mode derived from normalized agent.connect_url and gate.listen_url

remote mode:
  - auth.google.client_id required
  - auth.google.client_secret required
  - auth.google.redirect_url required
  - resend.api_key required
  - resend.from_email required
  - local_admin login disabled

local mode:
  - local_admin config available
  - Google/Resend config optional
  - if Google/Resend missing, corresponding UI capability disabled
```

远程模式启动失败必须说明缺失字段，例如：

```text
remote gate requires TERMBRIDGE_AUTH__GOOGLE__CLIENT_ID
remote gate requires TERMBRIDGE_RESEND__API_KEY
```

## Affected components

### Backend

- `internal/infrastructure/config/config.go`
  - 新增 `database`、`auth.local_admin`、`auth.google`、`auth.jwt_ttl`、`auth.code`、`resend` 配置。
  - 增加本地/远程模式派生结果。
  - 更新 `configKeys()`、默认配置、环境变量测试。

- `internal/app/app.go`
  - 打开 DB，运行 migrations。
  - 构造 user repository、auth service、email sender、Google OAuth client。
  - 将新的 AuthService 注入 gateway handler。

- `internal/infrastructure/database/*`
  - 新增 SQLite/MySQL open、dialect、migrator。

- `internal/infrastructure/database/migrations/*`
  - 新增用户系统相关 SQLite/MySQL migration。

- `internal/infrastructure/repository/*`
  - 新增 user / identity / auth code / delivery log repository。

- `internal/application/auth/*`
  - 新增注册、登录、邮箱验证、改密、password reset、Google callback 业务服务。

- `internal/transport/http/gatewayapi/*`
  - 新增 `/api/auth/*` routes。
  - 移除旧 `/api/login`、`/api/me`、`/api/logout`。
  - Agent tunnel Basic Auth 改用同一 AuthService credential verifier。
  - 继续保护 `/api/devices/**`。

- `internal/transport/http/gatewayapi/auth/*`
  - JWT claims 与 token service 调整。
  - 旧 single-user credential validator 下沉为 local_admin provider 或移除。

### Frontend

- `web/src/views/LoginView.vue`
- `web/src/components/session/LoginPanel.vue`
- `web/src/store/gateway.ts`
- `web/src/features/gateway/api.ts`
- `web/src/features/api/client.ts`
- 新增 register / verification / password reset / account security 页面或组件。

### Config / docs

- `.termbridge.default.yaml`
- `.env.example`
- `README.md`
- 后续 verification 文档记录外部 Google / Resend 未真实调用时的 mock 策略。

## Interfaces

### Config summary

```yaml
database:
  driver: sqlite
  sqlite:
    path: .termbridge/termbridge.db
  mysql:
    dsn: ""

auth:
  jwt_ttl: 24h
  local_admin:
    username: admin
    password: admin
  password:
    min_length: 8
    max_length: 128
  code:
    length: 6
    ttl: 2m
    resend_cooldown: 1m
    max_attempts: 3
  google:
    client_id: ""
    client_secret: ""
    redirect_url: ""

resend:
  api_key: ""
  from_email: ""
```

### API summary

```text
POST /api/auth/login
POST /api/auth/logout
GET  /api/auth/me
POST /api/auth/register
POST /api/auth/email/verify
POST /api/auth/email/verification/resend
POST /api/auth/password/change
POST /api/auth/password-reset/request
POST /api/auth/password-reset/confirm
GET  /api/auth/google
POST /api/auth/google/callback
```

### Auth response

```json
{
  "access_token": "<jwt>",
  "token_type": "bearer"
}
```

### Me response target

```json
{
  "authenticated": true,
  "user": {
    "id": "...",
    "email": "user@example.com",
    "display_name": "User",
    "provider": "email",
    "email_verified": true
  },
  "capabilities": {
    "mode": "local",
    "providers": ["email", "google", "local_admin"],
    "password_reset_enabled": true,
    "email_verification_enabled": true
  }
}
```

## Implementation feedback notes

以下为 Implementation / manual debugging 阶段已确认并反馈到规格边界的事实：

1. **SQLite 时间存储格式**：SQLite auth 表时间列为 `TEXT`，repository 必须以 `time.RFC3339Nano` 存储 UTC 字符串；读取侧只接受 RFC3339Nano 字符串或 driver 返回的 `time.Time`。不兼容旧的 Go `time.Time.String()` 脏数据，旧本地开发库需要清理或重建。
2. **前端认证初始化职责**：Google callback 和 email/password login 只负责交换凭据并保存 token；认证恢复集中在 router guard；`/` 首页作为入口中转页自行检查认证态并跳转 `/login` 或 `/sessions`；普通业务路由默认要求认证，少量认证入口路由跳过全局 guard。
3. **业务数据加载职责**：`SessionsView` 显式加载 devices 和 workspace tree；`initializeAuth()` 只负责 `/api/auth/me`，并对同一 token 做幂等保护。
4. **登出交互**：登出后跳转 `/login` 应立即展示登录表单；“正在检查登录状态…”只属于 `/` 首页入口检查，不属于明确未登录后的登录页。
5. **Terminal WebSocket Origin**：终端 WebSocket 除 token 外还受 `gate.browser.allowed_origins` / request Host 约束；本地 Vite 使用 `localhost` 与默认 `127.0.0.1` origin 不一致时会导致 WebSocket 403，需要配置包含实际浏览器 Origin。
6. **缺失 live PTY close 语义**：关闭会话时若 runtime 中 live PTY handle 已不存在，应记录 warn 并将非终态 session 标记为 stopped，不应向前端返回 upstream error。
7. **邮箱注册发送缺口**：当前 service 在 `CreateEmailUser` 后调用 `sendCode`，若 `sendCode` 因 sender 未配置或发送失败返回错误，用户记录可能已经落库而 `auth_codes` / `email_delivery_logs` 为空或不完整；该事务边界需要在后续修正中收敛，避免“用户存在但验证码/邮件日志不存在”的部分成功状态。

## Technical questions resolved in Spec

1. **本地 / 远程模式如何判定？**
   - 由 `agent.connect_url` 与 `gate.listen_url` 归一化后是否相同判定。

2. **SQLite / MySQL / migration 如何接入？**
   - 新增 DB infrastructure，参考 Pomelo Orbit 的 driver + dialect + embedded migrations 结构。

3. **Google OAuth / Resend 配置如何命名和校验？**
   - Google 放在 `auth.google.*`。
   - Resend 放在 `resend.*`。
   - 远程 Gate 模式缺失必需配置启动失败。

4. **verification/reset code 形式和生命周期？**
   - 用户可见验证码为随机 6 位 `A-Z0-9`。
   - TTL 2 分钟，重发冷却 1 分钟，最大错误 3 次。
   - DB 存 hash，不存明文。

5. **JWT 如何复用？**
   - 复用当前 HS256 JWT 能力，但 `sub` 改为 user id，并补充 provider/email/iat。
   - 本轮不做 refresh token / revoke list。

6. **admin/admin 如何处理？**
   - 写入配置，作为本地模式 local_admin shortcut 使用，不写入用户表。

7. **未验证邮箱能否登录？**
   - 不能。邮箱验证后才可以登录。

8. **旧 auth API 是否兼容？**
   - 不兼容。迁移到 `/api/auth/*`。

9. **Agent tunnel credential 是否拆分？**
   - 本轮不拆分，使用同一认证方式。

## Alternatives considered

### 1. 新增 `deployment.mode`

拒绝。

原因：用户明确要求基于 `agent.connect_url` 与 `gate.listen_url` 是否一致判定本地 / 远程模式，避免新增模式配置。

### 2. 继续使用旧 `/api/login` / `/api/me` / `/api/logout`

拒绝。

原因：用户明确要求不向后兼容。新用户系统统一使用 `/api/auth/*`。

### 3. 将 `admin/admin` seed 到用户表

拒绝。

原因：用户明确要求写入配置、本地模式使用、不写入表。

### 4. 自动合并同邮箱 Google / email 账号

拒绝。

原因：同邮箱自动合并可能导致 account takeover。当前策略是提示邮箱已使用，后续如需要再设计显式 linking。

### 5. 本轮引入 refresh token / session revoke

暂不采用。

原因：用户明确接受本轮不做 refresh token / revoke list。

### 6. 本轮完成 device enrollment

拒绝。

原因：本轮只建立 Browser 用户系统。Device enrollment 与 Agent device credential 属于后续需求。

## Risks

1. **认证安全风险**：JWT query token、localStorage、日志、错误信息、验证码爆破都需要 Plan 阶段明确实现防护。
2. **迁移面扩大风险**：新增 DB 层、SQLite/MySQL、migration 会扩大本轮变更面；Plan 阶段需要拆分实现单元。
3. **本地 shortcut 风险**：`admin/admin` 必须严格受本地模式限制。
4. **OAuth 测试风险**：真实 Google OAuth 需要配置和外部服务，自动化测试应使用 mock OAuth provider/client。
5. **Resend 测试风险**：测试不能依赖真实 Resend；需要 mock email sender。
6. **Agent tunnel 过渡风险**：Agent tunnel 与 Browser 使用同一认证方式仍不是长期 device credential 模型。
7. **MySQL 差异风险**：SQLite/MySQL SQL 语法、时间函数、唯一索引、事务行为不同，migration 和 repository 测试必须覆盖。
8. **模式判定风险**：如果 URL normalization 不严格，可能误判本地/远程模式。

## Plan-stage focus

进入 Plan 阶段时必须拆清：

1. DB / migration 基础设施先做，还是与 auth service 同步做。
2. Agent tunnel 使用同一认证方式下，email/local_admin/Google 三类 provider 的可用边界。
3. JWT claims 迁移对 `/api/devices/**`、terminal WebSocket、前端 store 的影响。
4. 日志中 token/password/code 的脱敏要求。
5. SQLite 普通测试与 MySQL opt-in E2E 的测试矩阵。
6. Google OAuth 和 Resend 的 mock 测试方式。
7. 旧 `/api/login`、`/api/me`、`/api/logout` 移除后的前后端影响。

## User review notes

用户明确要求：

- 本地 / 远程模式基于 `agent.connect_url` 与 `gate.listen_url` 是否一致。
- `/api/login` / `/api/me` / `/api/logout` 不向后兼容。
- `admin/admin` 写入配置，作为本地模式使用，不写入表。
- 验证码 2 分钟可用，1 分钟重发，最多 3 次。
- JWT 本轮不做 refresh token / revoke list。
- Agent tunnel credential 使用相同认证方式。
- 进入 Plan。
