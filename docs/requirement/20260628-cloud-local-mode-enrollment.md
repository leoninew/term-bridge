# Cloud / Local 模式分离与本机设备云端绑定需求
最后修改时间: 2026-06-28 23:08:00

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；需求 / Requirement 已接受，当前进入规格 / Spec。

本需求是一个新的 SpecFlow 任务，用于重新收敛 TermBridge 的 Cloud mode 与 Local mode 产品模型。此前“云端 Gate 设备绑定与本地 Agent 接入”需求中，将本地首次访问、setup token、local_access、云端账号登录和设备绑定混在同一条路径中，导致本地 web 与云端 web 的身份边界不清。

本需求以新的产品判断为基础：TermBridge 仍然是一个 Go 后端 + 一个前端，但前端运行时根据环境进入 local mode 或 cloud mode；账号系统只在 cloud mode 激活；local mode 默认是本机 Agent 控制台，并通过 cloud.gate_url 发起云端授权和设备绑定。

## Background

TermBridge 当前形态包括：

- 一个 Go 后端，负责 API、Agent tunnel、认证、设备与会话相关能力。
- 一个 Vue 前端，既可作为本地 web UI，也可随 Go 后端一起部署到云端 Gate。
- 已有基础账号能力：email 登录 / 注册、Google OAuth、JWT session、密码找回等。
- 已有 Agent 远程连接底层能力：通过 `agent.connect_url` / `TERMBRIDGE_AGENT__CONNECT_URL` 指向远程 Gate。
- 已有设备 identity、公钥签名和云端设备列表方向的初步实现。

近期实现与验证暴露出一个核心偏差：系统把 local web 也当成需要本地登录 / setup token 的认证 Gate，导致用户打开 `http://localhost:9031` 时被引导到 `/login` 或 `/setup`，而用户真正期望的是打开本机 dashboard，然后在右上角选择“连接云端账号”。

新的目标不是继续修补 setup token 路径，而是明确区分：

- Local mode：本机 Agent 控制台，不提供云端账号登录表单，默认打开 dashboard。
- Cloud mode：云端 Gate 门户，提供账号、OAuth、设备列表和远程 workbench 入口。
- Device enrollment：由 local dashboard 发起，通过 `cloud.gate_url` 完成用户认证，再回到 localhost 完成设备公钥上报和绑定。

## Goal

1. 明确 Cloud mode 与 Local mode 是 web runtime mode，不应再由 `agent.connect_url == agent.listen_url` 间接推导。
2. 明确非 cloud mode 下，本机打开 `agent.public_url`，例如 `http://localhost:9031`，默认进入 dashboard，不要求本地账号认证。
3. 明确账号能力只在 cloud mode 激活，包括登录、注册、找回密码、Google OAuth 等。
4. 明确 local mode 的右上角账号 UI 表达的是“云端账号连接状态”，不是本地登录状态。
5. 明确 local mode 用户未连接云端时，可通过 `cloud.gate_url` 打开云端 Gate 完成 OAuth / 账号认证，并返回 `agent.public_url`。
6. 明确 OAuth / 登录完成后，local backend 继续调用 `cloud.gate_url`，将当前设备信息、公钥和 possession proof 上报到云端完成绑定。
7. 明确云端用户登录后能在 cloud dashboard 查看、选择和连接自己已绑定的设备。
8. 保持 Browser 用户登录态、一次性 enrollment authorization、长期 device credential 三者边界分离。
9. Agent 后续连接 Cloud Gate 必须使用独立 device credential，不使用 Browser JWT、Google token、用户密码或本地 web 临时状态。
10. 为未来 GUI 应用保留同一套 enrollment abstraction：本机应用打开云端授权，授权后回到本机应用完成设备绑定。

## Non-goal

1. 本需求阶段不写产品代码。
2. 本需求不继续沿用“local setup token 是本地 web 主入口”的产品路径。
3. 本需求不要求本地 mode 提供 email 登录、注册、找回密码或 Google 登录表单。
4. 本需求不要求立即实现完整 GUI 应用、安装器、deep link、自定义 URI scheme 或 OS credential manager。
5. 本需求不要求立即实现完整组织 / 团队 / RBAC / invite / audit log。
6. 本需求不把 Browser JWT 或 Google OAuth token 保存为 Agent 长期凭据。
7. 本需求不要求普通用户手动输入 raw `device_id`、公钥或设备 token。
8. 本需求不要求废弃 CLI / env / config 高级路径；这些仍可作为自动化或无头环境入口。
9. 本需求不处理历史 setup token 代码的删除策略；是否保留为 dev / legacy / non-loopback protection 进入后续 Spec 决策。

## User scenarios

### Scenario 1：本机用户打开 local dashboard

1. 用户在本机启动 TermBridge。
2. 用户打开 `agent.public_url`，例如 `http://localhost:9031`。
3. 系统识别当前 web runtime 是 local mode。
4. 前端默认进入 `/dashboard`，而不是 `/login` 或 `/setup`。
5. dashboard 展示本机设备、workspace/session 入口、云端连接状态。
6. 右上角显示常规账号 UI：
   - 未连接云端时显示“连接云端账号”。
   - 已连接云端时显示云端账号 / Gate 绑定状态。
7. 用户无需创建本地账号，也无需使用 `admin/admin` 或 setup token 才能进入本机 dashboard。

期望用户理解：

> 我正在使用本机 TermBridge；如果需要云端能力，我可以把这台设备连接到云端账号。

### Scenario 2：本机用户通过 cloud.gate_url 连接云端账号

1. 用户在 local dashboard 点击“连接云端账号”。
2. 本地前端 / 后端根据配置读取 `cloud.gate_url`，例如 `http://termbridge.lvh.me`。
3. 系统打开云端 Gate 授权入口。
4. 如果用户尚未登录 cloud gate，云端页面要求用户通过 email 或 Google OAuth 登录。
5. 用户完成云端账号认证。
6. 云端 Gate 创建短期、一次性 enrollment authorization。
7. 云端 Gate 将浏览器重定向回 local `agent.public_url` 的回调地址，例如 `http://localhost:9031/cloud/callback?code=...&state=...`。
8. 本地前端把回调 code 交给 local backend。
9. local backend 调用 `cloud.gate_url` 的 enrollment exchange 接口，上报当前设备信息、公钥和 possession proof。
10. Cloud Gate 校验授权、state、设备公钥证明后，将设备绑定到当前 cloud user。
11. Cloud Gate 返回独立 device credential。
12. local backend 保存 device credential，并使本地 Agent 后续使用该 credential 连接 Cloud Gate。

### Scenario 3：云端用户登录后查看设备列表

1. 用户打开 cloud gate，例如 `http://termbridge.lvh.me`。
2. 系统识别当前 web runtime 是 cloud mode。
3. 未登录用户进入 `/login`，可使用 email 或 Google OAuth 完成登录。
4. 登录成功后进入 cloud `/dashboard`。
5. 如果该用户还没有绑定设备，dashboard 显示清晰空状态。
6. 空状态告诉用户：需要在本机 TermBridge 中连接此云端账号，而不是要求用户在云端页面输入 raw device id。
7. 如果用户已有设备，dashboard 展示设备列表、online/offline 状态、last seen 等信息。
8. 用户选择 online device 后进入 `/sessions` / workbench。

### Scenario 4：本地设备已绑定云端账号

1. 用户打开 local dashboard。
2. 右上角账号 UI 显示当前设备已连接到某个 cloud gate / cloud user。
3. 本地 dashboard 仍然可以访问本机 workspace/session 能力。
4. 用户可以看到当前 Cloud Gate 地址、绑定状态和连接状态。
5. Agent 使用保存的 device credential 连接 cloud gate。
6. Cloud dashboard 中该设备显示为 online。

### Scenario 5：cloud mode 才提供账号能力

1. 用户访问 cloud gate。
2. cloud mode 开启 `/login`、注册、找回密码、Google OAuth 等账号能力。
3. 用户访问 local web。
4. local mode 不把 `/login` 作为默认入口，不展示本地 email 登录、注册或找回密码表单。
5. 如果用户主动访问 local `/login`，系统应引导用户回到 dashboard 或说明账号登录需要在 cloud gate 完成。

### Scenario 6：高级用户 / 无 GUI / CLI 路径

1. 用户在服务器、CI、无头环境或高级自动化场景下需要绑定设备。
2. 系统可以提供 CLI / env / config 入口，例如使用 `cloud.gate_url` 或 `agent.connect_url` 完成 enrollment。
3. CLI 路径仍应复用同一套 enrollment exchange 和 device credential 模型。
4. CLI 不应要求普通用户手动输入 raw `device_id`，除非作为高级覆盖项。

### Scenario 7：local mode 暴露到非 loopback 地址

1. 用户将 local web 监听地址配置为非 loopback，例如 `0.0.0.0`、LAN IP 或公网地址。
2. 系统不能继续默认无认证暴露本机终端能力。
3. 系统应拒绝启动、要求显式安全配置，或启用额外认证边界。
4. 该安全策略进入 Spec 阶段设计，但需求上必须承认 local 无认证只成立于 loopback-only 场景。

### Scenario 8：enrollment 失败或过期

1. 用户从 local dashboard 发起云端连接。
2. OAuth / 登录成功后，回到 localhost 进行 enrollment exchange。
3. 如果 code 过期、state 不匹配、签名校验失败、网络失败或云端拒绝绑定，local dashboard 显示可理解错误。
4. 系统不创建半绑定设备，不保存无效 device credential。
5. 用户可以重新发起连接云端账号流程。

## Acceptance

### 模式与路由

- [ ] 系统存在明确的 web runtime mode：local / cloud，且不再仅依赖 `agent.connect_url == agent.listen_url` 推导。
- [ ] local mode 打开 `agent.public_url` 默认进入 `/dashboard`。
- [ ] local mode 未连接云端账号时，不要求进入 `/login` 或 `/setup` 才能使用本机 dashboard。
- [ ] cloud mode 未认证访问受保护页面时进入 `/login`。
- [ ] cloud mode 登录成功后进入 cloud `/dashboard`。
- [ ] `/sessions` 保留 workbench 语义，并可从 dashboard 显式进入。

### 账号能力边界

- [ ] 登录、注册、找回密码、Google OAuth 等账号能力只在 cloud mode 作为正式产品能力提供。
- [ ] local mode 不提供本地 email 登录、注册、找回密码表单作为主路径。
- [ ] local mode 右上角账号 UI 表示 cloud account connection 状态，而不是本地账号登录状态。
- [ ] local mode 不创建长期 local owner/password，不要求用户使用 `admin/admin` 作为正式入口。

### 云端连接与设备绑定

- [ ] local dashboard 提供“连接云端账号”主操作。
- [ ] “连接云端账号”使用 `cloud.gate_url` 打开 cloud gate 授权入口。
- [ ] cloud gate 完成用户认证后，通过短期 enrollment authorization 返回 localhost。
- [ ] local backend 使用 authorization code / enrollment code 与 cloud gate exchange，不把 Browser JWT 或 Google token 保存为 Agent 凭据。
- [ ] enrollment exchange 上报 device info、公钥和 possession proof。
- [ ] cloud gate 校验后，将 device 绑定到当前 cloud user，并签发独立 device credential。
- [ ] device credential 保存到本地后，Agent 后续使用该 credential 连接 cloud gate。

### Cloud dashboard 设备管理

- [ ] cloud dashboard 在用户无设备时展示清晰空状态。
- [ ] cloud dashboard 空状态引导用户在本机 TermBridge 中连接云端账号。
- [ ] cloud dashboard 在用户已有设备时展示设备列表与 online/offline 状态。
- [ ] cloud dashboard 只展示当前 cloud user 有权访问的设备。
- [ ] 用户选择 online device 后可以进入 `/sessions` workbench。
- [ ] offline 设备显示为不可连接状态，不应让用户误以为登录失败。

### 安全与凭据边界

- [ ] Browser 用户登录态、一次性 enrollment authorization、长期 device credential 三者语义分离。
- [ ] Cloud Gate redirect 回 localhost 时不能携带长期 token、Google token 或 device credential，只能携带短期一次性 code / state。
- [ ] enrollment code 短期有效、一次性使用、可撤销或可过期。
- [ ] Device credential 独立于用户密码、Browser JWT 和 Google OAuth token。
- [ ] Cloud Gate 不以明文保存长期 device token、enrollment token 或敏感 secret。
- [ ] Device id 是资源标识，不是 secret；绑定必须依赖签名或等价 possession proof。
- [ ] local mode 默认无认证只允许在 loopback-only 暴露场景成立；非 loopback 暴露必须有额外保护或拒绝启动。

### 兼容与迁移

- [ ] CLI / env / config 高级路径仍可存在，但应复用同一套 enrollment 和 device credential 模型。
- [ ] 已有 `agent.connect_url` 继续作为 Agent tunnel 目标配置，但不应单独决定 web auth mode。
- [ ] 已有 setup token / local_access 路径是否保留、降级或删除，需要在 Spec 阶段明确迁移策略。
- [ ] 当前已有 `/dashboard` 页面可以复用，但需要区分 local dashboard 与 cloud dashboard 的产品语义。

## Open questions

1. **local mode 无认证的准确安全边界如何落地？**
   - 用户已确认 local mode 满足配置即可，不正确的配置会导致云端 OAuth2 流程无法通过。
   - Spec 阶段仍需把配置校验、错误提示和失败路径设计清楚。

2. **CLI PKCE desktop flow 是否纳入本轮？**
   - 用户已确认先完成本地 web OAuth2 认证。
   - CLI / GUI 变体保留为后续扩展，不作为本轮主路径。

## Decisions

1. 本任务使用严格模式 / strict，从 Requirement / 需求阶段开始。
2. 本任务是新需求，不直接修改既有 `20260627-cloud-device-enrollment` 文档作为主文档。
3. TermBridge 仍然保持一个 Go 后端 + 一个前端的交付形态。
4. 前端按 web runtime mode 分为 local mode 与 cloud mode。
5. 账号系统只在 cloud mode 作为正式产品能力激活。
6. Local mode 打开 `agent.public_url` 默认进入 dashboard，不以 `/login` 或 `/setup` 作为主入口。
7. Local mode 的账号 UI 表示 cloud account connection，不表示本地登录。
8. Local mode 不创建长期 local owner/password。
9. Local mode 连接云端账号时，通过 `cloud.gate_url` 完成云端认证，再回到 localhost 完成 device enrollment。
10. Agent 长期连接 Cloud Gate 使用独立 device credential，不使用 Browser JWT、Google token 或用户密码。
11. `agent.connect_url` 是 Agent tunnel 目标配置，不应单独决定 web auth mode。
12. Local mode 默认无认证只在 loopback-only 场景成立。
13. web runtime mode 配置名采用 `web.mode: local|cloud`。
14. `/dashboard` 采用一个 `DashboardView` 根据 mode 渲染 local / cloud 两种语义。
15. 本地发起云端账号连接的入口路径采用 `/cloud/connect/start`。
16. 本轮先完成本地 web OAuth2 认证，不把 CLI PKCE desktop flow 作为主路径。
17. OAuth2 client 配置基于现有 OAuth2 库颁发 / 管理的 client 配置设计。
18. 设备私钥、device credential 和 enrollment 本地状态保存到 state dir，即现有 `.termbridge` 状态目录。
19. 已有 setup token / local_access 主路径移除，不做向后兼容。

## Risks

1. **安全边界风险**：local mode 无认证如果被错误暴露到非 loopback 地址，会直接暴露本机终端能力。
2. **Open redirect 风险**：cloud gate 若允许任意 `return_to`，可能被用于钓鱼、code 泄露或错误绑定。
3. **Token 泄露风险**：如果 cloud redirect 携带 Browser JWT、Google token 或 device credential 到 localhost URL，会进入浏览器历史、日志或第三方扩展风险面。
4. **凭据混用风险**：如果 Agent 继续使用 Browser JWT 或用户 token 连接 cloud gate，将破坏设备撤销、rotation 和审计模型。
5. **模式混淆风险**：如果继续用 `agent.connect_url` 推导 web mode，本地已连接远程 gate 的场景仍会被错误当作 cloud web。
6. **迁移风险**：已有 setup token / local_access 实现与文档需要重新定位，直接删除可能破坏已有本地首次访问测试或开发路径。
7. **UI 复用风险**：同一个 `/dashboard` 同时承载 local 与 cloud 语义，若组件边界不清，容易再次混淆用户心智。
8. **CLI 路径分叉风险**：如果 CLI enrollment 与 local dashboard enrollment 使用不同接口模型，未来 GUI 和自动化路径会出现行为不一致。

## User review notes

用户提出新的收敛方向：

> 仍然是一个 go 后端 + 一个前端，但是为 web 端添加 cloud 模式，云端部署时才激活部分功能
>
> 登录流程
> 1. web 在非 cloud 模式不需要认证。本机打开 agent.public_url 即 http://localhost:9031 时，打开 dashboard 页面。右上角显示常规的账号UI：用户未登录时打开 cloud.gate_url 即 http://termbridge.lvh.me 完成 oauth2 认证并返回 http://localhost:9031
> 2. web 仅在 cloud 模式下提供账号（登录、注册、找回密码等）能力，本地不提供
> 3. 用户登录完成后，继续调用 cloud.gate_url 将设备信息，公钥上报到云端
>
> 这样用户在云端就能看到和连接自己的设备列表

本 Requirement 按以下理解收口：

- 这是新的严格模式 SpecFlow 需求，不继续在旧 setup token 主路径上修补。
- Local web 是本机 Agent 控制台，不是账号登录页面。
- Cloud web 是账号系统和远程设备管理门户。
- 本机连接云端账号是 device enrollment 流程，不是把 cloud Browser JWT 复制给 Agent。
- 未来 GUI 应复用同一套 local app -> cloud auth -> return local app -> device credential exchange 的模型。

用户进入 Spec 前补充并确认：

> web.mode: local|cloud    一个 DashboardView 根据 mode 渲染   - /cloud/connect/start    - 基于 oauth2 库颁发 client 配置  - state dir 即现在的 .termbridge - 已有移除，不向后兼容 - local mode 满足配置即可，不正确的配置云端 oath2 不会过  8. 先完成本地 web oauth2 认证
>
> 进入 spec

本次补充按以下决策记录：

- mode 配置名采用 `web.mode: local|cloud`。
- dashboard 不拆页面，使用一个 `DashboardView` 根据 mode 渲染。
- 本地连接云端入口采用 `/cloud/connect/start`。
- OAuth2 client 配置基于 OAuth2 库颁发 / 管理的 client 配置。
- 设备和 enrollment 本地持久化使用 state dir，即当前 `.termbridge` 状态目录。
- 旧 setup token / local_access 主路径移除，不向后兼容。
- local mode 是否能完成 OAuth2 由配置正确性决定；配置不正确时云端 OAuth2 流程不会通过。
- 本轮优先完成本地 web OAuth2 认证。

## 2026-06-28 Implementation discussion: local mode runtime backend 与“本机绕 tunnel”

实现阶段复盘中发现：如果 local mode 的 `/api/devices` 将当前本机设备展示为 `online`，但 `/api/devices/{deviceId}/workspaces/tree` 仍完全依赖 Agent tunnel route，那么在本机没有 route 时会返回 `device_offline`。这暴露的是实现编排问题，不是用户设备真正离线。

本次讨论对需求语义做如下补充：

1. **后端 Gateway API 需要支持 local runtime backend。**
   - 支持方向成立：Web 前端仍使用统一 Gateway API，后端在 local mode 下可以把当前本机设备的 workspace/session/terminal 请求编排到本机 runtime。
   - 统一的是外部 API、业务语义和领域模型，不是要求所有本机请求都必须绕 Agent tunnel。

2. **本机设备判断不能只靠 route 是否存在。**
   - 用户澄清：只要 `serve` 启动，预期就是本机能力可用；用户也可以随时登录到云端，让云端看到和连接这台设备。
   - 因此 local mode 下“本机是否可用”不应由 `routeFor(deviceId)` 决定。
   - `route` 更适合表达 cloud/remote 设备是否在线；local mode 当前本机设备应由 `serve` 中已初始化的本机 runtime 能力支撑。

3. **JSON relay、history、terminal websocket 不应理解为另一套业务逻辑。**
   - 用户澄清：本地模式同样要走既有业务逻辑和领域模型；不同的是应用层如何编排。
   - 因此不应在 handler 中复制一套脱离领域模型的 local API 逻辑。
   - 正确边界是：Gateway API 入口保持统一；应用层根据 mode / 当前设备，把请求编排给本机 runtime 或远端 tunnel backend；底层仍复用 terminal/workspace/session 的应用服务与领域模型。

4. **app.go 启动逻辑和 `/api/devices` 列表也属于同一编排问题。**
   - local mode 的本机设备列表、workspace tree、session 操作、history 和 terminal attach 必须语义一致。
   - 不应出现列表显示本机设备 online，但进入 workspace/session 后返回 `device_offline` 的状态。

5. **权限边界在本轮不是主要阻塞。**
   - 用户澄清：无须过度担心 local mode 构造任意 device id 的访问问题，因为连不了。
   - 当前重点是先把 local mode 当前本机设备的业务编排做通，避免把实现复杂度提前转移到权限假设上。

6. **关于“本机绕 tunnel”的备选分析。**
   - 如果继续采用“本机绕 tunnel”，则 local mode 也需要启动本机 Agent connector，让浏览器请求经 Gateway API 转成 tunnel request，再由本机 Agent 回连处理 runtime。
   - 这种做法可以复用远端设备的 route/tunnel 执行路径，但会让本机能力依赖自连接 tunnel 生命周期；一旦 connector 未启动、握手失败、签名 audience 不一致或 reconnect 失败，local dashboard 就会出现 `device_offline`。
   - 这种做法还容易让 `agent.connect_url` 再次影响 local web 的可用性，与 `web.mode` 作为一等 runtime mode 的设计目标产生张力。
   - 如果选择继续本机绕 tunnel，必须补齐 connector 生命周期、启动顺序、ready 状态、重连/backoff、错误提示、签名校验、测试和观测日志；否则 local mode 会把“本机 runtime 可用性”错误地暴露为“设备在线/离线”问题。
   - 如果不采用本机绕 tunnel，则需要在 Gateway 应用层显式支持 local runtime backend；该 backend 不是另一套业务模型，而是同一领域能力在 local mode 下的不同编排方式。

## 2026-06-28 Correction: 本机绕 tunnel 是既有能力，不是新方案

用户进一步澄清：本机绕 tunnel 不是本轮新提出的备选方案，而是此前统一模型工作中已经实现的能力。本轮任务的重点是 OAuth2 与云端设备绑定；`device_offline` 问题说明这轮实现把既有 self-tunnel 能力改坏了，而不是说明必须改成 local runtime direct backend。

本次纠偏后，疑点确认如下：

1. **本机绕 tunnel 与 `web.mode=local` 不冲突。**
   - `web.mode=local|cloud` 决定 Web runtime 的产品语义、账号能力和浏览器认证边界。
   - 本机绕 tunnel 是本机 runtime 接入统一 Gateway API 的 transport / application orchestration 方式。
   - 因此 local mode 可以继续使用 self-tunnel；不能把“local mode 不需要云端账号认证”误解成“local mode 不使用 Agent tunnel”。

2. **统一模型的既有方向是：一个 Gateway API + 统一 device/workspace/session 访问模型。**
   - 前端不区分 local API 与 cloud API。
   - 本机设备和远端设备都通过 `/api/devices/...` 这套统一路径进入 workspace/session/terminal 能力。
   - 本机 self-tunnel 已经承担了把本机 runtime 接入这套统一路径的职责。

3. **`agent.connect_url` 不决定 web mode，但仍可以作为 self-tunnel 的连接目标。**
   - `web.mode` 是明确配置，不再由 `agent.connect_url == agent.listen_url` 推导。
   - 但在 local mode 中，Agent connector 连接本机 Gate 是既有 self-tunnel 能力的一部分。
   - 这里的关键区别是：`agent.connect_url` 只表达 tunnel 目标，不表达 Web 账号能力或 cloud/local mode。

4. **当前修复方向应是恢复 self-tunnel，不是引入另一套 direct backend。**
   - 不应在 Gateway handler 中临时复制一套 local direct path 来绕过既有 tunnel 模型。
   - 应恢复 `serve` 启动后本机 Agent connector 与 Gateway route 的闭环，让 `/api/devices/{localDevice}/workspaces/tree` 等统一 API 能继续通过既有 route/tunnel 工作。
   - 本轮 OAuth2 / cloud enrollment 改动不得破坏此前 unified gateway / self-tunnel 的基本能力。

5. **`device_offline` 的真实含义要回到 route 生命周期。**
   - 对 cloud/remote 设备：没有 route 返回 `device_offline` 是合理语义。
   - 对 local self-tunnel：如果 `serve` 已启动但本机设备仍 `device_offline`，应优先视为 self-tunnel route 没有建立或被本轮改坏，而不是产品上要求 direct runtime backend。
   - 因此需要检查启动编排、connector 是否被跳过、签名 audience / public key / device id 是否一致、route 是否被注册。

6. **后续实现约束。**
   - 不引入新 local API。
   - 不把本机 workspace/session/terminal 访问改成另一套 handler 逻辑。
   - 不让 cloud OAuth2 / cloud device enrollment 的配置改动影响本机 self-tunnel route。
   - 测试应覆盖：local mode serve 启动后，本机 self-tunnel route 能建立；本机设备列表与 workspace tree/session/terminal 请求语义一致；OAuth2/cloud enrollment 改动不会让本机设备退化为 `device_offline`。
