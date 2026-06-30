# Cloud / Local 模式分离与本机设备云端绑定需求
最后修改时间: 2026-06-30 12:00:00

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；需求 / Requirement 已接受，当前实现已按后续 Spec / Plan 与实现阶段澄清修正。

## Background

TermBridge 采用一个 Go 后端 + 一个前端的交付形态。同一套二进制 / 镜像通过运行时配置决定 Web 语义：

- `web.mode=local`：本机 Agent 控制台。
- `web.mode=cloud`：云端 Cloud Gate 门户。

此前实现中曾把本地首次访问、setup token、local_access、云端账号登录、设备绑定和 Agent tunnel 目标混在一起，导致用户打开本机 `agent.public_url` 时被错误引导到 `/login` / `/setup`，也导致 `agent.connect_url` 被误用成 web cloud/local mode 或云端连接目标。

本需求的最终收敛语义是：

- `web.mode` 只表达 Web runtime mode。
- `web.mode=cloud` 只给 Cloud Gate 部署使用，不是本机 UI 的“切云端模式”。
- `agent.*` 只描述本机 listen/public/self-tunnel/device identity。
- `cloud.*` 集中描述云端配置，尤其 `cloud.gate_url`。
- 本机 local serve 始终是本机控制台，可以在完成 OAuth2 绑定后额外连接 Cloud Gate。
- OAuth2 绑定成功只表示设备已绑定到 cloud user；云端 dashboard 显示 online 还需要后续 Agent tunnel route 成功建立。

## Goal

1. 明确 Cloud mode 与 Local mode 是显式 `web.mode`，不再由 `agent.connect_url == agent.listen_url` 推导。
2. 明确 `web.mode=cloud` 只用于云端 Cloud Gate 部署。
3. 明确本机 local serve 不存在“切云端模式”产品语义。
4. 明确 `agent` 配置节点只描述本机地址和设备身份：`listen_url`、`public_url`、`connect_url`、`device_id`、`device_name`。
5. 明确 `agent.connect_url` 是本机 self-tunnel 目标；云端连接目标必须使用 `cloud.gate_url`。
6. local mode 打开 `agent.public_url` 默认进入 dashboard，不要求本地账号、setup token 或 local_access。
7. cloud mode 才提供账号系统：登录、注册、找回密码、Google OAuth 等。
8. local mode 右上角账号 UI 表达“云端账号连接 / 绑定状态”，不是本地登录态。
9. local dashboard 通过 `cloud.gate_url` 发起 OAuth2，到 Cloud Gate 完成 cloud user 登录 / 授权后返回 localhost。
10. local backend 在 OAuth2 callback 中完成 code exchange，并用 cloud account token 调用 Cloud Gate 上报当前设备 `id/name/public_key`。
11. Cloud Gate 必须基于已认证 cloud user 建立 user-device binding，并持久化设备 public key，用于后续 Agent tunnel 签名校验。
12. 本机 cloud connector 只能在 OAuth2 callback 成功、设备上报成功后启动，避免未绑定前持续请求 Cloud Gate 产生大量 401。
13. Cloud dashboard 设备列表只展示当前 cloud user 绑定的设备；设备 online/offline 取决于 Cloud Gate runtime route，而不是绑定记录本身。
14. Browser 登录态、一次性 OAuth/binding code、本机 device private key / public key 签名三者边界分离。

## Non-goal

1. 不恢复 setup token / local_access 作为本机主入口。
2. 不把 local mode 改成本地 email 登录、注册、找回密码页面。
3. 不用 `agent.connect_url` 表达云端连接目标或 Web mode。
4. 不让 OAuth2 callback 将 Browser JWT、Google token 或长期 secret 暴露给前端 URL。
5. 不要求普通用户手动输入 raw device id、公钥或 token。
6. 不在本轮实现完整组织 / 团队 / RBAC / invite / audit log。
7. 不在本轮实现 CLI PKCE、GUI deep link、自定义 URI scheme 或 OS credential manager。
8. 不引入另一套 local-only workspace/session API；local 与 cloud 继续复用统一 Gateway API / tunnel 模型。

## User scenarios

### Scenario 1：本机用户打开 local dashboard

1. 用户在本机启动 `termbridge serve`，配置为 `web.mode=local`。
2. 用户打开 `agent.public_url`，例如 `http://localhost:9031`。
3. 前端进入 `/dashboard`，不跳 `/login` 或 `/setup`。
4. dashboard 展示本机设备、workspace/session 入口和云端连接状态。
5. 右上角账号 UI：
   - 未连接云端时显示“连接云端账号”。
   - 已完成云端 OAuth2 绑定时显示当前 Gate / device / connected_at 摘要。
6. 本机 self-tunnel 使用 `agent.connect_url` / 默认 `agent.listen_url`，用于统一 Gateway API 访问本机 runtime。

### Scenario 2：本机用户通过 cloud.gate_url 绑定云端账号

1. 用户在 local dashboard 点击“连接云端账号”。
2. 前端路由 `/cloud/oauth/start` 调用 local API `GET /api/cloud-oauth/start`。
3. local backend 根据 `cloud.gate_url`、`cloud.oauth.client_id`、`cloud.oauth.redirect_url` 创建短期 state，并返回 Cloud Gate authorize URL。
4. 浏览器打开 Cloud Gate authorize 页面。
5. 如果用户尚未登录 Cloud Gate，cloud mode 页面要求 email / Google OAuth 登录。
6. Cloud Gate 校验 `client_id + redirect_uri` 必须精确匹配已注册 OAuth client 配置。
7. Cloud Gate 签发一次性 binding code，并返回到 local callback：`/cloud/oauth/callback?code=...&state=...`。
8. local callback 前端调用 `POST /api/cloud-oauth/callback`。
9. local backend 校验 state，调用 Cloud Gate `POST /api/cloud-oauth/exchange` 换取 cloud account access token。
10. local backend 调用 Cloud Gate `POST /api/devices/current`，请求体包含当前设备 `id`、`name`、`public_key`。
11. Cloud Gate 以 cloud user token 识别用户，upsert device 与 user-device binding，并保存 public key。
12. local backend 建立 runtime-only `cloud_session` 摘要。
13. OAuth2 callback 成功后，本机启动指向 `cloud.gate_url` 的 cloud connector。

### Scenario 3：Cloud dashboard 查看设备列表

1. 用户打开 Cloud Gate，例如 `http://termbridge.lvh.me`，该部署配置 `web.mode=cloud`。
2. 未登录用户进入 `/login`。
3. 登录成功后进入 cloud `/dashboard`。
4. 无设备时展示空状态，引导用户回本机 TermBridge 连接云端账号。
5. 有设备时展示当前 cloud user 绑定的设备、online/offline、last seen 等信息。
6. `accepted:true` / 绑定存在不等于 online；online 需要 Cloud Gate 当前存在 active Agent tunnel route。
7. 用户只能连接 online device；offline device 显示为不可连接状态。

### Scenario 4：OAuth2 已完成但设备仍 offline

1. local callback 返回 200，Cloud Gate `/api/devices/current` 返回 `accepted:true`。
2. Cloud dashboard 出现设备记录，但可能暂时 `online:false`。
3. 这表示设备已绑定，但 cloud connector 尚未建立或 tunnel 签名校验 / route 注册尚未成功。
4. 正常情况下，OAuth2 成功后 local serve 会启动 cloud connector 到 `cloud.gate_url`。
5. Cloud Gate 使用设备 public key 校验 `/api/agent/tunnel` 签名，成功后注册 runtime route，设备才变为 online。

### Scenario 5：OAuth2 未完成时不连接云端

1. local serve 启动时，即使配置了 `cloud.gate_url`，也只启动本机 self connector。
2. 本机不会在未绑定、未上报 public key 前持续连接 Cloud Gate。
3. Cloud Gate 不应因此收到大量未授权 tunnel 401。
4. 只有 OAuth2 callback 成功、设备上报成功后，local serve 才启动 cloud connector。

### Scenario 6：cloud mode 才提供账号能力

1. 用户访问 Cloud Gate，cloud mode 开启登录、注册、找回密码、Google OAuth。
2. 用户访问 local web，local mode 不把 `/login` 作为默认入口。
3. local `/login` 若可访问，也只能提示账号登录需要在 Cloud Gate 完成，并引导回 dashboard / 连接云端。

### Scenario 7：高级用户 / 无 GUI / CLI 路径

1. 高级用户可以通过 config / env 设置 `cloud.gate_url`、OAuth client 配置和本机 device identity。
2. CLI / 无头路径后续应复用同一套 OAuth2 / binding / public key / signed tunnel 模型。
3. 普通用户主路径仍是 local dashboard 发起 OAuth2。

## Acceptance

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

## Decisions

1. 使用 `web.mode: local|cloud`。
2. 一个 `DashboardView` 根据 mode 渲染 local / cloud 语义。
3. 本地连接云端入口采用 `/cloud/oauth/start`，后端 API 为 `/api/cloud-oauth/start`。
4. OAuth2 callback 前端路由为 `/cloud/oauth/callback`，后端 API 为 `/api/cloud-oauth/callback`。
5. Cloud Gate OAuth authorize / exchange 后端 API 为 `/api/cloud-oauth/authorize` / `/api/cloud-oauth/exchange`。
6. Cloud Gate 必须精确校验 `client_id + redirect_uri`。
7. local `cloud_session` 是当前 local backend runtime 摘要；重启后可以丢失，不作为长期凭据。
8. 设备长期在线能力依赖本机 device private key 与 Cloud Gate 保存的 public key。
9. `agent` 节点是本机配置；`cloud` 节点是云端配置。
10. Cloud connector 目标取 `cloud.gate_url`，但只在 OAuth2 绑定成功后启动。
11. Cloud deployment 不生成本机 device identity，也不启动本机 Agent connector。
12. 旧 setup token / local_access 主路径退场，不作为本机正式入口。

## Risks

1. **OAuth2 配置风险**：`client_id`、`redirect_uri`、`agent.public_url`、`cloud.gate_url` 任一不匹配都会导致授权失败。
2. **online 误解风险**：绑定成功但 tunnel 未连上时 cloud dashboard 会显示 offline；UI / 日志必须避免把这解释成 OAuth2 失败。
3. **签名 audience 风险**：local cloud connector 使用 `cloud.gate_url` 签名；Cloud Gate 也必须用自身 `cloud.gate_url` 验签。
4. **配置语义回归风险**：不能再把 `agent.connect_url` 写成云端目标示例。
5. **local 暴露风险**：local mode 无 Browser auth 依赖 loopback / OS 用户 / 部署配置边界，非 loopback 暴露需要后续保护。

## User review notes

用户关键澄清：

> 我不想引入“模式”切换或“agent.connect_url”切换语义，配置里agent 节点是本地各种地址，云端模式连云端，云端配置集中在 cloud 节点里。 web.model=cloud 是给云端用的，没有所谓本地模式，也没有所谓本地模式切换。

本需求按该澄清修正为：

- 本机 local serve 不切 web mode。
- 本机 `agent.connect_url` 只服务 self-tunnel。
- 本机连接云端只使用 `cloud.gate_url`。
- Cloud connector 只在 OAuth2 绑定成功后启动。
