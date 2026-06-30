# Cloud / Local 模式分离与本机设备云端绑定计划
最后修改时间: 2026-06-30 12:00:00

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；计划 / Plan 已接受，当前实现已按本计划与实现阶段修正完成。

## Requirement / Spec basis

- Requirement: `docs/requirement/20260628-cloud-local-mode-enrollment.md`
- Requirement status: Accepted
- Spec: `docs/spec/20260628-cloud-local-mode-enrollment.md`
- Spec status: Accepted

## Implementation strategy

本轮实现目标：建立显式 `web.mode: local|cloud`，移除 setup token / local_access 主路径，接入本机 OAuth2 绑定 Cloud Gate，并保证本机 self-tunnel 与云端 tunnel 语义不混淆。

最终实现策略：

1. 配置层正式化 `web.mode`。
2. 保留 `agent.connect_url` 作为本机 self-tunnel target。
3. 把云端目标集中到 `cloud.gate_url`。
4. local mode 默认进入 dashboard，不要求本地 Browser auth。
5. cloud mode 继续要求 cloud user auth。
6. `/cloud/oauth/start` 与 `/cloud/oauth/callback` 使用前端 route + 后端 `/api/cloud-oauth/*` API。
7. Cloud Gate 精确校验 `client_id + redirect_uri`。
8. local callback 成功后上报 `id/name/public_key` 到 Cloud Gate。
9. Cloud Gate 保存 public key，并用它验证后续 `/api/agent/tunnel` 签名。
10. 本机 local serve 启动时只启动 self connector；OAuth2 完成后才启动 cloud connector。
11. Cloud deployment 不创建本机 device identity，也不启动本机 Agent connector。

## Implementation steps

### Step 1：配置模型新增 / 正式化 `web.mode`

涉及文件：

- `internal/infrastructure/config/config.go`
- `internal/infrastructure/config/config_test.go`
- `.termbridge.default.yaml`
- `.env.example`

任务：

1. Config 中支持：

   ```yaml
   web:
     mode: local # local | cloud
   ```

2. 默认 `local`。
3. 支持环境变量：

   ```text
   TERMBRIDGE_WEB__MODE=local|cloud
   ```

4. 非法 mode 返回配置错误。
5. 删除 `agent.connect_url == agent.listen_url` 对 web mode 的推导语义。
6. 示例配置说明：
   - `agent.connect_url` 是本机 self-tunnel 目标。
   - 云端连接目标使用 `cloud.gate_url`。

验收：

- 默认 local。
- env 可设置 cloud。
- 非法 mode 报错。
- `agent.connect_url` 不影响 web mode。

### Step 2：后端 capabilities / auth middleware mode-aware

涉及文件：

- `internal/application/auth/service.go`
- `internal/transport/http/gatewayapi/server.go`
- 相关 tests

任务：

1. Capabilities 返回 mode。
2. local mode：
   - `account_auth_enabled=false`。
   - `cloud_oauth_enabled` 取决于 `cloud.gate_url` / OAuth client 配置。
   - `/api/auth/me` 顶层返回 runtime-only `cloud_session`。
3. cloud mode：
   - 保留 email / Google / register / password reset 能力。
   - 受保护 API 要求 cloud user token。
4. local mode auth middleware 放行本机 dashboard / sessions / devices 所需 API。

验收：

- local `/dashboard` / `/sessions` 不因 `authenticated=false` 跳 login。
- cloud 受保护 API 无 token 返回 401。

### Step 3：移除 setup token / local_access 主路径

涉及文件：

- `internal/application/auth/setup.go`
- `internal/application/auth/service.go`
- `internal/app/app.go`
- `web/src/views/SetupView.vue`
- `web/src/router/index.ts`
- `web/src/store/gateway.ts`
- tests / i18n

任务：

1. `termbridge serve` 不再把 setup token 作为本机正式入口。
2. `/setup` 不再是 local 首次访问主路径。
3. local_access 不再作为本机 Browser 访问前置条件。
4. 相关测试改写为 local mode 无认证 / cloud mode 认证。

验收：

- 打开 local `/` 进入 dashboard。
- 打开 local `/dashboard` 不要求 setup token。
- 旧 local_access 主路径不再驱动正式产品流程。

### Step 4：前端 router / store / dashboard 按 mode 组织

涉及文件：

- `web/src/router/index.ts`
- `web/src/store/gateway.ts`
- `web/src/features/gateway/api.ts`
- `web/src/views/HomeView.vue`
- `web/src/views/LoginView.vue`
- `web/src/views/DashboardView.vue`
- `web/src/views/SessionsView.vue`
- `web/src/i18n.ts`
- tests

任务：

1. 初始化时读取 `/api/auth/me`。
2. store 区分：
   - mode / capabilities。
   - Browser cloud user auth。
   - local `cloud_session`。
   - devices / selected device。
3. router guard：
   - local mode 允许 dashboard / sessions。
   - cloud mode 未登录跳 login。
4. DashboardView 单页面分支：
   - local section：本机状态、workspace/session、连接云端账号。
   - cloud section：设备列表、空状态、online/offline。
5. i18n 补齐 local/cloud dashboard 与 callback loading 文案。

验收：

- local mode 无 token 正常进 dashboard。
- cloud mode 无 token 进入 login。
- dashboard 文案不混淆本机与云端。

### Step 5：实现 local OAuth2 start

涉及文件：

- `internal/application/auth/cloud_binding.go`
- `internal/transport/http/gatewayapi/server.go`
- `web/src/views/CloudConnectAuthorizeView.vue`
- `web/src/features/gateway/api.ts`
- tests

任务：

1. 前端 route `/cloud/oauth/start` 调用 API。
2. local backend API：

   ```http
   GET /api/cloud-oauth/start?redirect=/dashboard
   ```

3. 仅 local mode 可用。
4. 创建 state 并保存到 state dir。
5. state 绑定 callback URL、post-auth redirect、Gate URL、device id/name、过期时间。
6. 使用 `golang.org/x/oauth2` 生成 Cloud Gate authorize URL。
7. 返回 JSON `authorize_url`。

验收：

- authorize URL 包含 `client_id`、精确 `redirect_uri`、`state`、scope。
- cloud mode 不执行 local start。

### Step 6：实现 Cloud Gate authorize / exchange

涉及文件：

- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/cloud_binding_test.go`
- 前端 Cloud authorize view / API tests

任务：

1. Cloud authorize API：

   ```http
   GET /api/cloud-oauth/authorize?client_id=...&redirect_uri=...&state=...
   ```

2. 仅 cloud mode 可用。
3. 要求 cloud user token。
4. 精确校验：
   - `client_id == cloud.oauth.client_id`
   - `redirect_uri == cloud.oauth.redirect_url`
   - `state != ""`
5. 创建短期一次性 binding code。
6. 返回 local callback `redirect_url`。
7. Cloud exchange API：

   ```http
   POST /api/cloud-oauth/exchange
   {"code":"..."}
   ```

8. 仅 cloud mode 可用。
9. 消费 binding code，签发 cloud account token。
10. binding code 不可复用。

验收：

- 缺失 / 错误 client_id 拒绝。
- `localhost` 与 `127.0.0.1` redirect URI 不可互换。
- local mode 访问 authorize / exchange 返回 not found。

### Step 7：实现 local OAuth2 callback 与设备上报

涉及文件：

- `internal/transport/http/gatewayapi/server.go`
- `internal/infrastructure/repository/device/repository.go`
- tests

任务：

1. 前端 callback route `/cloud/oauth/callback` 读取 query code/state。
2. 前端调用：

   ```http
   POST /api/cloud-oauth/callback
   {"code":"...","state":"..."}
   ```

3. local backend 消费 state。
4. 调用 Cloud Gate `/api/cloud-oauth/exchange`。
5. exchange 成功后调用 Cloud Gate：

   ```http
   POST /api/devices/current
   Authorization: Bearer <cloud token>
   {"id":"...","name":"...","public_key":"..."}
   ```

6. 如果本机 device identity 或 public key 缺失，callback 失败。
7. device report 成功后设置 runtime `cloud_session`。
8. 返回 `cloud_session` 与 redirect。

验收：

- callback 成功后 local UI 能看到 cloud_session。
- device repository 保存 public key。
- tunnel signature 可用 DB public key 验证。

### Step 8：Cloud 设备 API 与 online 语义

涉及文件：

- `internal/infrastructure/repository/device/repository.go`
- `internal/transport/http/gatewayapi/server.go`
- tests

任务：

1. `/api/devices/current` 要求 cloud user auth。
2. 请求体要求 id/name/public_key。
3. Upsert device 和 user-device binding。
4. `/api/devices` 只返回当前 cloud user 的 devices。
5. DeviceSummary online 来自 runtime registry / active route。
6. 绑定成功但无 route 时返回 offline。

验收：

- 已认证设备上报成功。
- public key 为空时报错。
- `/api/devices` user 隔离。
- offline 不被错误标记成 online。

### Step 9：App startup connector 编排

涉及文件：

- `internal/app/app.go`
- `internal/app/app_test.go`
- `.termbridge.default.yaml`
- `.env.example`

任务：

1. local serve：
   - Ensure local identity。
   - Load/Create device。
   - Upsert local device。
   - 启动 backend。
   - 启动 self connector 到 `agent.connect_url`。
2. local serve 不在启动时连接 `cloud.gate_url`。
3. OAuth2 callback 成功并上报设备后，启动 cloud connector 到 `cloud.gate_url`。
4. cloud connector 不重复启动。
5. connector 失败只 warning + retry，不杀死 local dashboard。
6. cloud serve：
   - 不 Ensure local identity。
   - 不 Load/Create local device。
   - 不启动 connector。
7. Cloud Gate tunnel audience 使用 `cloud.gate_url`；self-tunnel audience 使用 `agent.connect_url`。

验收：

- 配置了 `cloud.gate_url` 但未 OAuth2 时只启动 self connector。
- OAuth2 callback 成功后启动 cloud connector。
- cloud mode serve 不创建本地 device 私钥。
- 示例配置不再指示设置 `TERMBRIDGE_AGENT__CONNECT_URL=https://gate.example.com`。

### Step 10：验证与文档同步

涉及文件：

- `docs/requirement/20260628-cloud-local-mode-enrollment.md`
- `docs/spec/20260628-cloud-local-mode-enrollment.md`
- `docs/plan/20260628-cloud-local-mode-enrollment.md`
- `docs/verification/20260628-cloud-local-mode-enrollment.md`

任务：

1. 文档统一术语：`cloud_session`，不是旧 `cloud_connection`。
2. 文档统一路径：
   - frontend `/cloud/oauth/start` / `/cloud/oauth/callback`
   - backend `/api/cloud-oauth/start` / `/api/cloud-oauth/callback`
   - cloud API `/api/cloud-oauth/authorize` / `/api/cloud-oauth/exchange`
3. 文档说明 public_key 必需。
4. 文档说明绑定和 online 分离。
5. 文档说明 OAuth2 未完成前不连接 Cloud Gate。
6. 文档说明 `agent.*` 与 `cloud.*` 的配置边界。

## Files expected to change

实际核心变更范围：

- `.termbridge.default.yaml`
- `.env.example`
- `internal/app/app.go`
- `internal/app/app_test.go`
- `internal/application/auth/cloud_binding.go`
- `internal/application/auth/service.go`
- `internal/application/auth/setup.go`
- `internal/infrastructure/config/config.go`
- `internal/infrastructure/config/config_test.go`
- `internal/infrastructure/repository/device/repository.go`
- `internal/infrastructure/repository/device/repository_test.go`
- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/cloud_binding_test.go`
- `internal/transport/http/gatewayapi/server_test.go`
- `internal/transport/http/server/server.go`
- `web/src/features/gateway/api.ts`
- `web/src/features/gateway/api.test.ts`
- `web/src/store/gateway.ts`
- `web/src/store/gateway.test.ts`
- `web/src/router/index.ts`
- `web/src/views/DashboardView.vue`
- `web/src/views/CloudConnectAuthorizeView.vue`
- `web/src/views/CloudConnectCallbackView.vue`
- `web/src/views/HomeView.vue`
- `web/src/views/LoginView.vue`
- `web/src/views/SessionsView.vue`
- `web/src/views/SetupView.vue`
- `web/src/i18n.ts`
- RSPV docs

## Verification plan

### Targeted backend tests

```powershell
go test ./internal/app ./internal/infrastructure/config ./internal/infrastructure/repository/device ./internal/transport/http/gatewayapi
```

重点断言：

- `web.mode` 默认 / env / invalid。
- local mode 不要求 Browser auth。
- cloud mode protected API 要求 auth。
- OAuth authorize 精确校验 client_id + redirect_uri。
- exchange code 一次性。
- `/api/devices/current` 要求 public_key。
- DB public key 可验证 tunnel signature。
- local serve OAuth2 未完成前只启动 self connector。
- OAuth2 callback 成功后启动 cloud connector。
- cloud mode serve 不创建本机 identity / connector。

### Frontend checks

```powershell
npm --prefix web test -- --run
npm --prefix web run typecheck
npm --prefix web run lint
npm --prefix web run format:check
```

重点断言：

- local router 不跳 login。
- cloud router 未登录跳 login。
- gateway store 区分 `authenticated` 与 `cloud_session`。
- start / authorize / callback API 参数正确。
- dashboard i18n key 完整。

### Diff whitespace

```powershell
git diff --check
```

### Manual verification

Local：

1. 启动 local serve，`web.mode=local`。
2. 确认启动后只有 self connector，例如：

   ```text
   TermBridge agent connector targeting http://127.0.0.1:9030
   ```

3. 不应出现 cloud connector target。
4. 打开 `http://localhost:9031/dashboard`。
5. 点击连接云端账号。
6. 完成 Cloud Gate 登录 / authorize。
7. callback 成功后本机出现 cloud connector target：

   ```text
   TermBridge agent connector targeting http://termbridge.lvh.me
   ```

Cloud：

1. Cloud deployment 配置 `web.mode=cloud` 和 `cloud.gate_url`。
2. 打开 cloud dashboard，登录。
3. OAuth2 绑定完成后应看到设备记录。
4. `/api/devices/current` 成功后设备可能先 offline。
5. `/api/agent/tunnel` 验签成功并注册 route 后设备变为 online。

## Rollback / recovery

如果 cloud connector 相关逻辑异常：

- 不回滚到 `agent.connect_url` 指向云端。
- 不恢复 setup token / local_access 主路径。
- 优先检查：
  1. local 是否完成 OAuth2 callback。
  2. Cloud Gate 是否保存 public key。
  3. local cloud connector 是否启动。
  4. tunnel signature audience 是否等于 Cloud Gate `cloud.gate_url`。
  5. Cloud Gate `/api/agent/tunnel` 是否注册 route。

## Risks

1. OAuth2 redirect mismatch 会导致流程失败；这是安全约束，不应放宽。
2. OAuth2 成功但 connector 未启动会造成 Cloud dashboard offline。
3. public_key 缺失会导致 Cloud Gate 无法验证 tunnel signature。
4. `cloud.gate_url` 与 tunnel audience 不一致会导致验签失败。
5. 非 loopback local exposure 保护仍未作为本轮强制策略落地。

## User review notes

实现阶段用户明确要求：

- 不引入“本地模式切换”。
- 不引入 `agent.connect_url` 切换语义。
- `agent` 节点是本地各种地址。
- 云端配置集中在 `cloud` 节点。
- `web.mode=cloud` 是给云端部署用。
- OAuth2 未完成前无须连接云端，避免大量 401。

本 Plan 已按这些约束修正，并作为后续实现 / 验证依据。
