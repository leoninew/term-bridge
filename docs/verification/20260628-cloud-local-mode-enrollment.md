# Cloud / Local 模式分离与本机设备云端绑定验证
最后修改时间: 2026-06-30 12:00:00

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；验证 / Verification 已按当前 Requirement / Spec / Plan 与实现阶段澄清更新。

## Basis

- Requirement: `docs/requirement/20260628-cloud-local-mode-enrollment.md`，Review status: Accepted
- Spec: `docs/spec/20260628-cloud-local-mode-enrollment.md`，Review status: Accepted
- Plan: `docs/plan/20260628-cloud-local-mode-enrollment.md`，Review status: Accepted

本验证按最终收敛后的架构语义核对：

- `web.mode=local` 是本机控制台。
- `web.mode=cloud` 只用于 Cloud Gate 部署。
- `agent.*` 只描述本机 listen / public / self-tunnel / device identity。
- `agent.connect_url` 只表达本机 self-tunnel target。
- `cloud.gate_url` 是本机连接 Cloud Gate 的唯一云端目标配置。
- OAuth2 绑定成功只表示设备已绑定到 cloud user；设备 online 取决于 Cloud Gate runtime tunnel route。
- OAuth2 callback 成功并上报 `public_key` 后，local serve 才启动 cloud connector，避免未绑定前大量 401。

## Verification scope

本次验证覆盖当前实现是否完成以下目标：

1. 通过显式 `web.mode: local|cloud` 区分 local web 与 cloud web，不再用 `agent.connect_url == agent.listen_url` 推导 web auth mode。
2. local mode 默认进入 `/dashboard`，不要求本地账号、setup token 或 `local_access` 才能使用本机 dashboard / sessions。
3. cloud mode 保留登录、注册、找回密码、Google OAuth 等账号能力，并继续对受保护 API 要求 Browser JWT。
4. 前端 router / store / dashboard 基于 capabilities mode 组织逻辑，一个 `DashboardView` 渲染 local / cloud 两种语义。
5. 移除 setup token / `local_access` 主路径，不把它作为本机正式入口。
6. local `/cloud/oauth/start` 使用 `cloud.gate_url` 和 OAuth2 client config 生成云端授权 URL，并保存短期 state。
7. Cloud Gate authorize / exchange 只在 `web.mode=cloud` 可用，并精确校验 `client_id + redirect_uri`。
8. local `/cloud/oauth/callback` 校验 state、exchange code，并使用 cloud account token 上报当前设备 `id/name/public_key`。
9. Cloud Gate 持久化设备 `public_key`，后续 `/api/agent/tunnel` 用该 public key 验证本机 device private key 签名。
10. cloud 端 `/api/devices` 只返回当前 cloud user 有权访问的设备。
11. 设备绑定记录与 online 状态分离：绑定来自 DB user-device 关系，online 来自 runtime active tunnel route。
12. local serve 启动时只启动 self connector；OAuth2 callback + device report 成功后才启动 cloud connector。
13. cloud serve 不创建本机 device identity，也不启动本机 Agent connector。
14. 保持 unified Gateway API / tunnel 模型，不新增 local direct backend，不新增 local-only workspace/session API。

不作为本次自动化通过条件：

- 真实浏览器跨 local 与 Cloud Gate 两个部署的端到端 OAuth2 手工演练。
- 真实外部 OAuth2 provider / production Cloud Gate 部署验收。
- CLI PKCE、GUI deep link、自定义 URI scheme、OS credential manager。
- non-loopback local exposure 的强制保护策略。

## Actual diff summary

当前工作区覆盖 cloud/local mode 分离、OAuth2 本机绑定、Cloud Gate device public key enrollment、cloud connector 启动门控、以及 RSPV 文档同步。

主要改动类别：

- RSPV 文档：Requirement / Spec / Plan / Verification 统一到当前架构语义。
- 配置：正式化 `web.mode`；示例中明确 `agent.connect_url` 是 self-tunnel target，云端目标使用 `cloud.gate_url`。
- 后端 app startup：local serve 创建本机 identity 并启动 self connector；cloud connector 仅在 OAuth2 callback + device report 成功后启动；cloud serve 不创建本机 identity / connector。
- 后端 auth / capabilities：capabilities 暴露 mode 与 cloud OAuth 能力；local `/api/auth/me` 返回 runtime-only `cloud_session`。
- 后端 OAuth2：local start/callback 与 cloud authorize/exchange 分离；Cloud Gate 精确校验 `client_id + redirect_uri`；authorize/exchange cloud-only。
- 后端 device API：`/api/devices/current` 要求 `id/name/public_key`，并按 cloud user 建立 user-device binding。
- 后端 tunnel security：Cloud Gate 可从 DB 加载 `devices.public_key` 验证 `/api/agent/tunnel` 签名；online 来自 runtime route。
- 前端：router / store / dashboard 按 mode 分支；local dashboard 显示云端连接入口与 runtime `cloud_session`；callback 完成后刷新 auth/session 状态。
- 测试：覆盖 config mode、OAuth exact redirect、callback device report public key、DB public key tunnel 验签、local startup self connector、OAuth 成功后 cloud connector、cloud mode 无本机 connector。

## Expected vs actual changed areas

| Plan area | Actual files / implementation area | Result |
|---|---|---|
| Step 1：`web.mode` 配置 | `.termbridge.default.yaml`, `.env.example`, `internal/infrastructure/config/config.go`, `internal/infrastructure/config/config_test.go` | 对齐；默认 local、env cloud、非法 mode、`agent.connect_url` 不决定 web mode 均有覆盖。 |
| Step 2：capabilities mode-aware | `internal/application/auth/service.go`, `internal/transport/http/gatewayapi/server.go`, frontend API/store | 对齐；capabilities 返回 mode、account auth 与 cloud OAuth 能力，`/api/auth/me` 顶层返回 local runtime `cloud_session`。 |
| Step 3：移除 setup token / local_access 主路径 | `internal/application/auth/setup.go`, `internal/application/auth/service.go`, `internal/app/app.go`, router / setup view / tests | 对齐；setup/local_access 不再作为本地正式入口。 |
| Step 4：前端 router / store / dashboard mode-aware | `web/src/router/index.ts`, `web/src/store/gateway.ts`, `web/src/views/DashboardView.vue`, `web/src/views/HomeView.vue`, `web/src/views/LoginView.vue`, `web/src/views/SessionsView.vue`, `web/src/i18n.ts` | 对齐；local mode 不因 `authenticated=false` 跳 login，cloud mode 继续要求登录；dashboard 文案区分本机与云端。 |
| Step 5：local OAuth2 start | `internal/application/auth/cloud_binding.go`, `internal/transport/http/gatewayapi/server.go`, `web/src/features/gateway/api.ts`, `CloudConnectAuthorizeView.vue` | 对齐；local start 生成 Cloud Gate authorize URL，包含 `client_id`、精确 `redirect_uri`、`state`、scope。 |
| Step 6：Cloud Gate authorize / exchange | `internal/transport/http/gatewayapi/server.go`, `cloud_binding_test.go` | 对齐；仅 cloud mode 可用，要求 cloud user token，精确校验 `client_id + redirect_uri`，binding code 一次性。 |
| Step 7：local OAuth2 callback 与设备上报 | `internal/transport/http/gatewayapi/server.go`, `internal/infrastructure/repository/device/repository.go`, `cloud_binding_test.go` | 对齐；callback exchange 成功后上报 `id/name/public_key`；本机 public key 缺失时 callback 失败。 |
| Step 8：Cloud 设备 API 与 online 语义 | `internal/infrastructure/repository/device/repository.go`, `internal/transport/http/gatewayapi/server.go`, tests | 对齐；device report 要求 public key；online 由 runtime registry / active route 决定，绑定不会伪装成 online。 |
| Step 9：App startup connector 编排 | `internal/app/app.go`, `internal/app/app_test.go`, config examples | 对齐；local startup 只启动 self connector；OAuth2 成功后启动 cloud connector；cloud mode serve 不创建本机 identity / connector。 |
| Step 10：验证与文档同步 | RSPV docs | 对齐；术语统一为 `cloud_session`，路径统一为 `/cloud/oauth/*` 与 `/api/cloud-oauth/*`，文档记录 public key、online 分离与 OAuth2 前不连云端。 |

## Requirement alignment

| Requirement topic | Verification result |
|---|---|
| 明确 Cloud / Local 是 web runtime mode | 对齐；`web.mode` 已成为配置与 capabilities 的一等字段，`agent.connect_url` 仅作为 self-tunnel target。 |
| `web.mode=cloud` 只用于 Cloud Gate 部署 | 对齐；cloud serve 不创建本机 device identity，也不启动本机 connector。 |
| `agent.*` / `cloud.*` 配置边界 | 对齐；`agent.*` 只描述本机地址和设备身份，cloud target 统一为 `cloud.gate_url`。 |
| local mode 打开本机 URL 默认进入 dashboard，不要求本地账号 | 对齐；router guard local mode 放行 `/dashboard` 和 `/sessions`，不把 `/login` / `/setup` 作为主路径。 |
| 账号能力只在 cloud mode 激活 | 对齐；cloud mode 才提供正式登录、注册、找回密码、Google OAuth 能力，受保护 API 继续要求 token。 |
| local UI 表达 cloud session / binding 状态 | 对齐；`/api/auth/me` 顶层返回 runtime-only `cloud_session`，前端 store 与 Browser `authenticated` 分离保存。 |
| 使用 `cloud.gate_url` 发起云端授权并返回 localhost callback | 对齐；local start 使用 Cloud Gate authorize URL，callback 由 local frontend route + backend API 完成。 |
| Cloud Gate 精确校验 `client_id + redirect_uri` | 对齐；`localhost` 与 `127.0.0.1` 不可互换，错误 client_id / redirect_uri 被拒绝。 |
| callback 上报 `id/name/public_key` | 对齐；device report 要求 public key，repository 校验非空并持久化。 |
| Cloud Gate 用 public key 验证 tunnel 签名 | 对齐；测试覆盖 DB public key 读取与 signed tunnel request 验证。 |
| OAuth2 未完成前不连接 Cloud Gate | 对齐；local startup 即使配置 `cloud.gate_url` 也只启动 self connector。 |
| OAuth2 成功后 cloud connector 启动 | 对齐；callback 成功、device report 成功、`cloud_session` 设置后触发 cloud connector。 |
| 绑定成功不等于 online | 对齐；`/api/devices/current` 可返回 offline，online 需要 `/api/agent/tunnel` 注册 runtime route。 |
| 不破坏 self-tunnel / unified gateway | 对齐；workspace/session/terminal 仍复用统一 Gateway API / tunnel 模型。 |
| non-loopback local exposure 保护 | 未完成；需求已列为后续安全专项，不作为本轮阻塞。 |

## Spec alignment

| Spec decision | Verification result |
|---|---|
| `web.mode: local|cloud`，默认 local | Passed |
| `web.mode=cloud` 不是本机“云端模式切换” | Passed |
| `agent.connect_url` 只作为 self-tunnel target | Passed |
| `cloud.gate_url` 是 cloud connector 唯一目标 | Passed |
| 一个 `DashboardView` 根据 mode 渲染 | Passed |
| `/cloud/oauth/start` / `/cloud/oauth/callback` 为前端产品路由 | Passed |
| `/api/cloud-oauth/start` / `/api/cloud-oauth/callback` 为 local backend API | Passed |
| `/api/cloud-oauth/authorize` / `/api/cloud-oauth/exchange` 为 Cloud Gate API | Passed |
| Cloud authorize exact `client_id + redirect_uri` | Passed |
| `/api/devices/current` 要求并保存 `public_key` | Passed |
| `cloud_session` 是 runtime-only local summary | Passed |
| `cloud_session` 不包含 Browser JWT / Google token / binding code / private key | Passed |
| cloud connector 只在 OAuth2 callback + device report 成功后启动 | Passed |
| device online 来源为 runtime route，不是 DB binding | Passed |
| setup token / local_access 主路径退场 | Passed |

## Plan alignment

| Plan step | Verification result |
|---|---|
| Step 1：配置模型新增 / 正式化 `web.mode` | Passed |
| Step 2：后端 capabilities / auth middleware mode-aware | Passed |
| Step 3：移除 setup token / local_access 主路径 | Passed |
| Step 4：前端 router / store / dashboard 按 mode 组织 | Passed |
| Step 5：实现 local OAuth2 start | Passed |
| Step 6：实现 Cloud Gate authorize / exchange | Passed |
| Step 7：实现 local OAuth2 callback 与设备上报 | Passed |
| Step 8：Cloud 设备 API 与 online 语义 | Passed |
| Step 9：App startup connector 编排 | Passed |
| Step 10：验证与文档同步 | Passed |

## Acceptance criteria checklist

### 模式与配置

- [x] 存在明确 `web.mode: local|cloud`。
- [x] `web.mode=cloud` 只用于 Cloud Gate 部署。
- [x] `agent.connect_url` 不再决定 web auth mode。
- [x] `agent.connect_url` 只表达本机 self-tunnel 目标。
- [x] 云端连接目标集中在 `cloud.gate_url`。
- [x] 示例配置不再要求通过 `TERMBRIDGE_AGENT__CONNECT_URL` 切到云端。

### Local dashboard

- [x] local mode 打开 `agent.public_url` 默认进入 `/dashboard`。
- [x] local mode 未连接云端账号时，不要求 `/login`、`/setup` 或 local_access。
- [x] local mode 右上角账号 UI 表示 cloud session / cloud binding 状态。
- [x] local mode 仍启动 self-tunnel，保持统一 Gateway API 访问本机 runtime。

### Cloud OAuth2 与设备绑定

- [x] local dashboard 提供“连接云端账号”主操作。
- [x] 前端产品路由使用 `/cloud/oauth/start` / `/cloud/oauth/callback`。
- [x] 后端 API 使用 `/api/cloud-oauth/start` / `/api/cloud-oauth/callback`。
- [x] Cloud Gate API 使用 `/api/cloud-oauth/authorize` / `/api/cloud-oauth/exchange`。
- [x] Cloud Gate 校验 `client_id + redirect_uri` 精确匹配。
- [x] callback exchange 成功后，local backend 用 cloud account token 调用 `/api/devices/current`。
- [x] `/api/devices/current` 要求设备 `public_key`，并保存到 Cloud Gate DB。
- [x] OAuth2 未完成前不启动 cloud connector。
- [x] OAuth2 成功与设备上报成功后才启动 cloud connector。

### Cloud dashboard 与 online 状态

- [x] Cloud dashboard 只展示当前 cloud user 绑定的设备。
- [x] 设备绑定成功不伪装成 online。
- [x] online 由 active `/api/agent/tunnel` route 决定。
- [x] Cloud Gate 用持久化 public key 校验 agent tunnel 签名。

### 安全边界

- [x] Browser 用户登录态、一次性 binding code、设备私钥签名边界分离。
- [x] OAuth2 redirect 回 localhost 只携带短期 code/state。
- [x] Binding code 一次性使用。
- [x] Cloud Gate 不把 Google token 或 Browser JWT 作为 Agent tunnel 凭据。
- [x] Device id 是资源标识，不是 secret；tunnel 在线证明依赖 device private key 签名。
- [ ] 非 loopback local exposure 的强制保护策略仍作为后续安全专项。

## Automated test results

### Latest targeted backend verification

Command:

```text
go test ./internal/app ./internal/infrastructure/config ./internal/infrastructure/repository/device ./internal/transport/http/gatewayapi
```

Result:

```text
ok   termbridge-go/internal/app
ok   termbridge-go/internal/infrastructure/config
ok   termbridge-go/internal/infrastructure/repository/device
ok   termbridge-go/internal/transport/http/gatewayapi
```

Coverage emphasis:

- `web.mode` default / env / invalid。
- local mode 不要求 Browser auth。
- cloud mode protected API 要求 auth。
- OAuth authorize 精确校验 `client_id + redirect_uri`。
- binding code 一次性。
- `/api/devices/current` 要求 `public_key`。
- DB public key 可验证 tunnel signature。
- local serve OAuth2 未完成前只启动 self connector。
- OAuth2 callback 成功后启动 cloud connector。
- cloud mode serve 不创建本机 identity / connector。

### Gateway-focused backend verification

Command:

```text
go test ./internal/app ./internal/transport/http/gatewayapi
```

Result:

```text
ok   termbridge-go/internal/app
ok   termbridge-go/internal/transport/http/gatewayapi
```

### Frontend verification already executed during this change set

Commands:

```text
npm --prefix web test -- --run
npm --prefix web run typecheck
npm --prefix web run lint
npm --prefix web run format:check
```

Observed result during this change set:

```text
Test Files 10 passed (10)
Tests 42 passed (42)
vue-tsc --noEmit completed successfully
eslint . completed successfully
All matched files use Prettier code style
```

说明：后续 cloud connector gating 主要修改 Go app startup / Gateway callback / Go tests；该阶段未再次修改前端逻辑。

### Diff whitespace check

Command:

```text
git diff --check
```

Result:

```text
completed successfully with no whitespace errors
```

## Manual verification checklist

尚未完成真实浏览器 / 真实部署手工验收。后续手工验收应按以下步骤执行。

### Local

1. 启动 local serve，配置 `web.mode=local`。
2. 确认启动后只有 self connector，例如：

   ```text
   TermBridge agent connector targeting http://127.0.0.1:9030
   ```

3. OAuth2 未完成前不应出现 cloud connector target。
4. 打开 `http://localhost:9031/dashboard`。
5. 点击连接云端账号。
6. 完成 Cloud Gate 登录 / authorize。
7. callback 成功后本机出现 cloud connector target，例如：

   ```text
   TermBridge agent connector targeting http://termbridge.lvh.me
   ```

8. local `/api/auth/me` 返回 runtime-only `cloud_session`。

### Cloud

1. Cloud deployment 配置 `web.mode=cloud` 和 `cloud.gate_url`。
2. 打开 cloud dashboard，登录。
3. OAuth2 绑定完成后应看到设备记录。
4. `/api/devices/current` 成功后设备可能先 offline。
5. Cloud Gate 日志应出现带 `public_key` 的 device report。
6. `/api/agent/tunnel` 验签成功并注册 route 后设备变为 online。
7. 若仍 offline，应优先检查：
   - local cloud connector 是否已启动；
   - Cloud Gate DB 是否保存 public key；
   - tunnel signature audience 是否等于 Cloud Gate `cloud.gate_url`；
   - Cloud Gate `/api/agent/tunnel` 是否注册 route。

## Missed or expanded scope

- 未完成真实浏览器端到端手工验收。
- 未完成真实 cloud deployment / OAuth client 配置验收。
- 未实现 non-loopback local mode 强制保护策略；已作为后续安全专项保留。
- 未实现 CLI PKCE、GUI deep link、自定义 URI scheme、OS credential manager。
- 不应回滚到 `agent.connect_url` 指向云端；该方案已被明确拒绝。

## Risks

1. **OAuth2 配置风险**：`client_id`、`redirect_uri`、`agent.public_url`、`cloud.gate_url` 任一不匹配都会导致授权失败；这是安全边界，不应放宽。
2. **online 误解风险**：绑定成功但 tunnel 未连上时 cloud dashboard 会显示 offline；UI / 日志必须避免把这解释成 OAuth2 失败。
3. **public key 风险**：Cloud Gate 未保存 `devices.public_key` 时，后续 `/api/agent/tunnel` 无法验签，设备会保持 offline。
4. **签名 audience 风险**：local cloud connector 使用 `cloud.gate_url` 签名；Cloud Gate 也必须用自身 `cloud.gate_url` 验签。
5. **connector lifecycle 风险**：OAuth2 callback 成功后若 cloud connector 未启动或持续失败，Cloud dashboard 会有绑定记录但保持 offline。
6. **local 暴露风险**：local mode 无 Browser auth 依赖 loopback / OS 用户 / 部署配置边界，非 loopback 暴露需要后续保护。

## Incomplete items

- 未完成真实浏览器端到端手工验收。
- 未完成真实 cloud deployment / OAuth client 配置验收。
- 未实现 non-loopback local mode 强制保护策略。
- 未实现 CLI / desktop deep-link 类后续体验增强。

## Conclusion

Verification 结论：当前实现与 Accepted Requirement / Spec / Plan 的本轮范围对齐。自动化验证已覆盖 mode 配置、local/cloud auth 边界、OAuth exact redirect、device public key report、DB public key tunnel 验签、self connector 启动、OAuth2 后 cloud connector 启动、cloud mode 无本机 connector，以及 diff whitespace。保留未完成项主要属于真实部署 / 浏览器手工验收与后续安全 / UX 增强，不阻塞本轮 Cloud / Local mode 分离、本机 OAuth2 绑定、public key enrollment、以及 OAuth-gated cloud connector 启动的代码交付。
