# 前后端认证集成操作手册

最后修改时间: 2026-06-27 15:22:00

## 使用方式

当项目需要接入 email/password 登录、Google OAuth、Resend 邮件验证码、前端路径守卫时，按本文顺序落地。每完成一节，用该节的“完成标准”自查，不要等到全部写完后才排查认证链路。

推荐顺序：

1. 定义认证 API 契约。
2. 实现后端 auth service 和 provider adapter。
3. 实现前端 auth API client 与 auth store。
4. 接入路径守卫和入口页。
5. 接入 Google OAuth。
6. 接入 Resend 邮件验证码和 password reset。
7. 补齐日志脱敏。
8. 跑失败路径验证。

## 1. 定义认证 API 契约

### 操作步骤

1. 把认证接口收敛到独立 namespace。

   ```text
   POST /api/auth/login
   POST /api/auth/logout
   GET  /api/auth/me
   POST /api/auth/register
   POST /api/auth/email/verify
   POST /api/auth/email/verification/resend
   POST /api/auth/password-reset/request
   POST /api/auth/password-reset/confirm
   GET  /api/auth/google
   POST /api/auth/google/callback
   ```

2. 统一 token 响应。

   ```json
   {
     "access_token": "<first-party-token>",
     "token_type": "bearer"
   }
   ```

3. 统一 `/api/auth/me` 响应。

   ```json
   {
     "authenticated": true,
     "user": {
       "id": "internal-user-id",
       "email": "user@example.com",
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

4. 统一错误响应。

   ```json
   {
     "code": "invalid_credentials",
     "error": "Email or password is incorrect.",
     "requestId": "req_123"
   }
   ```

### 完成标准

- 前端恢复认证态只需要调用 `/api/auth/me`。
- 登录、Google callback 成功后都返回相同 token shape。
- 错误响应包含稳定 `code` 和 `requestId`。
- 错误响应不包含 password、verification code、OAuth code/state、provider 原始敏感响应。

### 常见错误

- 旧 `/api/login`、`/api/me` 和新 `/api/auth/*` 混用。
- `/api/auth/me` 同时加载业务数据，导致认证恢复和业务页面耦合。
- callback 返回 provider 原始 token，而不是本系统 token。

## 2. 实现后端认证服务

### 操作步骤

1. 后端登录成功后签发本系统 token，不把 provider 身份直接当 session。

   推荐 claims：

   ```json
   {
     "sub": "internal-user-id",
     "email": "user@example.com",
     "provider": "email",
     "iat": 1760000000,
     "exp": 1760086400
   }
   ```

2. `sub` 使用内部 user id。

3. `provider` 只记录本次认证来源：`email`、`google`、`local_admin` 等。

4. email/password 登录时检查：

   - email 已存在。
   - password 正确。
   - email 用户已完成验证。
   - 用户状态允许登录。

5. Google 登录时检查：

   - OAuth state 有效。
   - Google code exchange 成功。
   - provider subject 可识别。
   - 本地用户状态允许登录。
   - 同 email 跨 provider 冲突时拒绝。

6. local admin shortcut 只在 local mode 可用。

### 完成标准

- email/password、Google OAuth、local admin 最终都签发同一种本系统 token。
- 受保护 API 只认本系统 token。
- Google 返回的 email 不会自动接管 email/password 账号。
- local admin 不会在 remote mode 登录成功。

### 常见错误

- JWT `sub` 使用 email 或 Google sub。
- Google email 与本地 email 相同就自动合并。
- local admin shortcut 在 remote mode 仍可用。
- provider 失败时把原始错误直接返回给前端。

## 3. 实现前端 auth API client

### 操作步骤

1. 为每个 auth endpoint 写明确方法。

   ```text
   authLogin(email, password)
   authMe()
   authRegister(email, password)
   authVerifyEmail(email, code)
   authResendVerification(email)
   authPasswordResetRequest(email)
   authPasswordResetConfirm(email, code, newPassword)
   authGoogleURL()
   authGoogleCallback(code, state)
   ```

2. API client 请求拦截器只从 auth store 读取 token。

   ```text
   request interceptor
     -> read token from auth store
     -> set Authorization: Bearer <token>
   ```

3. 不在 interceptor 里从 `localStorage` 做 fallback。

4. 所有 auth API 错误走统一错误解析和 toast 展示。

### 完成标准

- token 来源只有 auth store。
- localStorage 只由 auth store 读写。
- API client 不知道登录页、callback 页或 router 的存在。
- 401 由调用方或路由守卫处理，不在 API client 里随意跳转页面。

### 常见错误

```text
apiClient token = store.token || localStorage.getItem("token")
```

这种双来源会掩盖 store 初始化问题，后续很难判断 401 是后端问题、前端状态问题，还是 token 传播问题。

## 4. 实现前端 auth store

### 操作步骤

1. store 持有这些状态：

   ```text
   token
   authInitialized
   authenticated
   user
   capabilities
   loggingIn
   ```

2. `setToken(token)` 只做：

   ```text
   set token in state
   persist token
   reset authInitialized/authenticated/user/capabilities
   reset business-resource state if needed
   ```

3. `clearToken()` 只做：

   ```text
   clear token from state
   remove persisted token
   reset auth state
   reset business-resource state if needed
   ```

4. `initializeAuth()` 只做 `/api/auth/me`。

   ```text
   if no token:
     authenticated=false
     authInitialized=true
     return

   if same token already initialized:
     return

   GET /api/auth/me
   update authenticated/user/capabilities
   authInitialized=true
   ```

5. 业务数据加载拆成单独方法，例如 `loadDevices()`、`loadWorkspaceTree()`。

### 完成标准

- `initializeAuth()` 不调用 `/api/devices`、workspace tree 或 session API。
- 同一个 token 重复进入业务路由时不会重复 `/api/auth/me`。
- logout 后进入 `/login` 不会卡在“正在检查登录状态”。
- 登录成功和 Google callback 成功后只调用 `setToken()`，不额外调用业务数据接口。

### 常见错误

- `login()` 内部调用 `initializeAuth()`，页面跳转后 router guard 又调用一次。
- `GoogleCallbackView` 调用 `initializeAuth()`，跳转业务页后业务页再调用一次。
- `initializeAuth()` 顺便加载 devices，导致认证恢复和业务加载重复触发。

## 5. 设置路径守卫

### 操作步骤

1. 默认业务路由需要认证。

2. 只列出认证入口和中转页作为 skip list。

   ```text
   skipAuthGuardRoutes:
     - home
     - login
     - register
     - verify-email
     - forgot-password
     - reset-password
     - google-callback
   ```

3. router guard 按以下逻辑写：

   ```text
   if route.name in skipAuthGuardRoutes:
     return

   await authStore.initializeAuth()
   if !authStore.authenticated:
     redirect to login
   ```

4. `/` 用入口页处理分流。

   ```text
   HomeView mounted:
     await initializeAuth()
     if authenticated:
       replace sessions
     else:
       replace login
   ```

5. `/login` 直接显示登录表单。

### 完成标准

- 未登录访问业务路由会跳转 `/login`。
- 访问 `/login` 不等待 `authInitialized`。
- `/` 是中转页，只负责检查后跳转。
- Google callback route 不被全局 guard 拦截。
- 业务页面 mounted 只加载业务数据，不负责判断是否登录。

### 常见错误

- 把 `/` 称作“无须认证业务页”。它只是入口中转页。
- login 页复用 `/` 的“正在检查登录状态” loading。
- 每个业务路由手写 `requiresAuth`，漏掉新增页面。
- 业务页面 mounted 里再次跳 login，和 router guard 形成双重跳转。

## 6. 接入 Google OAuth

### 操作步骤

1. 后端提供 `GET /api/auth/google`。

   ```text
   generate high entropy state
   persist state with TTL
   build Google auth URL from configured redirect URL
   return auth_url
   ```

2. 前端 login/register 点击 Google 按钮。

   ```text
   authGoogleURL()
   window.location.href = auth_url
   ```

3. Google redirect 到前端 callback route，URL 带 `code` 和 `state`。

4. 前端 callback 页面只做：

   ```text
   read code/state from query
   if missing: notify + redirect login
   POST /api/auth/google/callback { code, state }
   store access_token
   replace sessions or business entry
   ```

5. 后端 callback 只信任自己验证后的 state 和 Google response。

   ```text
   validate state exists / not expired / not used
   exchange code with Google
   fetch or verify Google userinfo
   find user by provider=google + subject
   if not found and email conflict exists: reject
   if not found and no conflict: create Google user
   issue first-party token
   mark state used
   ```

6. Google redirect URL 使用显式配置，并和 Google Console 完全一致。

### 完成标准

- OAuth state 是高熵随机值，不是 6 位邮箱验证码。
- state 有 TTL，且只能使用一次。
- 前端 callback 不加载 `/api/auth/me`、`/api/devices`、workspace tree。
- Google email 与 email/password 用户冲突时拒绝，不自动合并。
- redirect URL 路径一致，例如都使用 `/auth/google/callback`。
- 本地开发时浏览器 Origin 与后端允许 Origin 一致，例如不要混用 `localhost` 与 `127.0.0.1`。

### 常见错误

- redirect URL 写成 `/google/callback`，前端 route 是 `/auth/google/callback`。
- state 只存在前端，后端无法验证。
- callback 页面换 token 后又立刻 `/me` 和 `/devices`，跳转后业务页再重复请求。
- 看到 Google email 相同就自动合并账号。
- Google client secret 暴露到前端配置。

## 7. 接入 Resend 邮件验证码

### 操作步骤

1. 定义 Resend adapter，只负责发送。

   ```text
   Send(to, subject, html)
     -> POST Resend API
     -> return success/message_id/response_body_summary
   ```

2. Auth service 负责验证码业务。

   ```text
   normalize email
   generate code
   hash code
   bind code to email + purpose
   enforce ttl/cooldown/max attempts
   send email through Resend adapter
   record delivery result
   ```

3. 注册发送 verification code。

   ```text
   register email/password
   create pending user or pending registration state
   create verification code
   send through Resend
   if send failed:
     invalidate code or make it unusable
     return actionable error
     keep a resend/retry path
   ```

4. password reset request 返回泛化 accepted。

   ```text
   if verified email user exists:
     create reset code
     send through Resend
   always return accepted shape
   ```

5. 邮件内容只包含用户需要的信息。

   ```text
   Your verification code is ABC123. It expires in 2 minutes.
   ```

### 完成标准

- Resend adapter 不生成验证码、不判断 TTL、不决定用户 verified。
- 注册邮件发送失败不会留下可用验证码。
- 注册邮件发送失败后用户有恢复路径：resend、retry 或重新注册策略明确。
- delivery result 记录 success/failed 和 provider message id 或截断 response。
- 服务端日志不记录完整验证码。
- 缺 Resend 配置时，前端 capabilities 不展示不可用入口，或后端启动校验直接失败。

### 常见错误

- `sender == nil` 时已经创建用户，然后直接返回错误，导致用户存在但没有 code/log。
- Resend adapter 里混入验证码 TTL、attempts、cooldown 规则。
- password reset request 对不存在邮箱返回不同错误，暴露账号枚举。
- 日志打印完整验证码或 Resend 原始敏感响应。

## 8. 接入 capabilities gating

### 操作步骤

1. 后端 `/api/auth/me` 或公开 capabilities endpoint 返回当前认证能力。

   ```json
   {
     "mode": "local",
     "providers": ["email", "google", "local_admin"],
     "password_reset_enabled": true,
     "email_verification_enabled": true
   }
   ```

2. 前端按 capabilities 控制入口：

   - Google 登录按钮。
   - 注册入口。
   - 忘记密码入口。
   - local admin shortcut 提示。

3. 后端 API 仍做最终校验。capabilities 只影响 UX。

### 完成标准

- Google 未配置时，前端不展示或禁用 Google 登录入口。
- Resend 未配置时，前端不展示或禁用依赖邮件的入口。
- 即使前端显示错误，后端仍拒绝不可用 provider。

### 常见错误

- 前端根据当前 URL、Vite 环境变量或 local mode 猜测 provider 是否可用。
- 只隐藏按钮，后端没有禁止不可用 provider。
- login/register 两个页面展示能力不一致。

## 9. 做日志脱敏

### 操作步骤

1. 整理敏感字段列表。

   ```text
   authorization
   cookie
   token
   access_token
   id_token
   refresh_token
   code
   state
   password
   current_password
   new_password
   verification_code
   reset_code
   client_secret
   api_key
   secret
   ```

2. request started/completed log 处理 query redaction。

   ```text
   /ws?token=...                  -> token=[REDACTED]
   /callback?code=...&state=...   -> code=[REDACTED]&state=[REDACTED]
   ```

3. request body 如果会记录，必须做字段级 redaction；更稳妥是默认不记录认证 body。

4. API error log 使用同一套 redaction 规则。

5. provider response body 保存前截断，并排除 secret/header/token。

### 完成标准

- 正常 2xx/4xx request log 不出现 JWT、OAuth code/state、password。
- error log 不出现 JWT、OAuth code/state、password。
- WebSocket query token 不以明文出现在日志。
- Resend API key、Google client secret 不出现在任何日志。

### 常见错误

- 只给 error log 脱敏，普通 request log 仍记录 raw query。
- 为了调试 callback，把完整 query/body 打进日志。
- 记录 provider 原始 response，里面包含敏感字段。

## 10. 验证失败路径

### 手工请求顺序

1. 未登录访问业务路由。

   ```text
   GET /sessions
   expect: redirect /login
   ```

2. 打开 `/login`。

   ```text
   expect: 直接显示登录表单
   expect: 不显示“正在检查登录状态”
   ```

3. email/password 登录成功。

   ```text
   POST /api/auth/login
   expect: access_token
   expect: route -> business entry
   expect: no duplicate /api/auth/me before guard
   ```

4. Google callback 成功。

   ```text
   POST /api/auth/google/callback
   expect: access_token
   expect: callback page only stores token and redirects
   expect: business page loads business data once
   ```

5. logout。

   ```text
   POST /api/auth/logout
   clear token
   route -> /login
   expect: login form visible immediately
   ```

### 测试矩阵

| 主题 | 场景 | 预期 |
| --- | --- | --- |
| email login | 未验证邮箱登录 | 拒绝 |
| email login | 错误密码 | 拒绝，不泄露账号枚举细节 |
| register | 重复 email | 拒绝，不能创建冲突用户 |
| register + Resend | 发送失败 | 不留下可用验证码；用户可恢复或可重试 |
| email verify | code 错误、过期、已使用、超过次数 | 拒绝 |
| password reset request | 不存在邮箱 | 返回泛化 accepted |
| password reset confirm | reset 成功后旧密码登录 | 拒绝 |
| Google OAuth | state 不存在、不匹配、已使用、过期 | 拒绝 |
| Google OAuth | Google email 与本地 email 冲突 | 拒绝，不合并 |
| JWT/session | 过期、签名错误、缺 sub | 拒绝 |
| route guard | 未登录访问业务路由 | 跳转 login |
| route guard | 访问 login | 直接显示登录表单 |
| callback page | Google callback 成功 | 只换 token 并跳转，不加载业务数据 |
| logging | query/body 包含 token/code/password | 日志中为 `[REDACTED]` 或不记录 |
| capabilities | Google 未配置 | 前端不展示或禁用 Google 入口；后端仍拒绝不可用 provider |

## 11. 排查重复请求

### 观察现象

Google callback 后如果出现下面序列，说明职责混在一起：

```text
POST /api/auth/google/callback
GET  /api/auth/me
GET  /api/devices
GET  /api/auth/me
GET  /api/devices
GET  /api/devices/:id/workspaces/tree
```

### 排查步骤

1. 查 callback 页面是否调用 `initializeAuth()`。
2. 查 login action 是否调用 `initializeAuth()`。
3. 查业务页面 mounted 是否调用 `initializeAuth()`。
4. 查 `initializeAuth()` 是否加载 devices。
5. 查 router guard 是否在业务路由统一调用 `initializeAuth()`。

### 修正目标

成功登录或 callback 后，请求应该接近：

```text
POST /api/auth/google/callback
route -> /sessions
router guard: GET /api/auth/me
SessionsView: GET /api/devices
SessionsView: GET /api/devices/:id/workspaces/tree
```

如果 callback 后直接跳业务页且 guard 已经完成认证，也可以没有额外 `/me`，但不能出现 callback、store、业务页各自重复初始化。

## 12. 提交前检查清单

- [ ] 后端认证接口都在 `/api/auth/*`。
- [ ] `/api/auth/me` 只返回认证态、用户、capabilities，不加载业务数据。
- [ ] token claims 的 `sub` 是本系统内部 user id。
- [ ] 前端 API client 只从 auth store 读取 token。
- [ ] `setToken()` 和 `clearToken()` 会重置 auth 状态。
- [ ] `initializeAuth()` 只调用 `/api/auth/me`。
- [ ] login 成功后只保存 token 并跳转。
- [ ] Google callback 成功后只保存 token 并跳转。
- [ ] router guard 默认保护业务路由。
- [ ] `/` 是入口中转页。
- [ ] `/login` 直接显示登录表单。
- [ ] Google OAuth state 高熵、有 TTL、一次性使用。
- [ ] Google email 冲突默认拒绝。
- [ ] Resend adapter 只负责发送。
- [ ] Resend 发送失败有恢复路径。
- [ ] capabilities 控制 Google/注册/忘记密码入口展示。
- [ ] request log 和 error log 都脱敏 token/code/password/secret。
- [ ] 覆盖失败路径测试或手工验证记录。

## TermBridge 当前落地参考

- 需求文档：`docs/requirement/20260626-basic-user-system-auth.md`
- 规格文档：`docs/spec/20260626-basic-user-system-auth.md`
- 计划文档：`docs/plan/20260626-basic-user-system-auth.md`
- 验证文档：`docs/verification/20260626-basic-user-system-auth.md`
- Auth application service：`internal/application/auth/service.go`
- Resend adapter：`internal/infrastructure/email/resend.go`
- HTTP auth/JWT：`internal/transport/http/gatewayapi/auth/`
- Gateway auth routes：`internal/transport/http/gatewayapi/server.go`
- 前端 auth API：`web/src/features/gateway/api.ts`
- 前端 auth store：`web/src/store/gateway.ts`
- 前端 router guard：`web/src/router/index.ts`
- Login panel：`web/src/components/session/LoginPanel.vue`
- Google callback：`web/src/views/GoogleCallbackView.vue`
- Home entry：`web/src/views/HomeView.vue`
