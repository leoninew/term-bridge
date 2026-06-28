# 云端 Gate 设备绑定与本地 Agent 接入需求
最后修改时间: 2026-06-28 11:54:25

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；需求 / Requirement 已接受，当前进入规格 / Spec。

用户已明确要求开始 `/specflow` Spec 阶段，并确认关键决策：采纳方案 A，本地模式跳转 setup 页面，在 setup 页面得到和 login 页面效果一致的凭据，作为后续身份凭据；Spec 需要包含关键流程图。

## Background

TermBridge 当前已经具备 Cloud Gate PoC 与基础用户系统的关键前置能力：

- Cloud Gate PoC 已验证前后端一体镜像、Go server 静态资源服务、`/api` 与 SPA fallback 共存，以及现有 Agent tunnel / device 上报的基础形态。
- 基础用户系统已引入 Browser 用户注册、登录、邮箱验证、Google OAuth、JWT session 与 `/api/auth/*` 路由，但最终验收仍有未完成项。
- 长期身份文档已经明确产品模型应为：`User -> Device -> Workspace -> Session -> Terminal`。
- 当前本地已经具备连接远程 Gate 的能力：通过 `TERMBRIDGE_AGENT__CONNECT_URL` / `agent.connect_url` 指向远程 Gate，Agent 可以 outbound 连接远端。

当前仍缺少的是正式产品语义：用户在云端 Gate 页面完成注册和登录后，如何把一台本地设备安全、可理解地绑定到该用户，并让该设备后续以独立设备身份连接云端 Gate。

本任务不是继续扩大基础用户系统，也不是重做 Cloud Gate PoC，而是为下一阶段“云端 Gate 设备绑定 / enrollment / local Agent 接入”形成需求依据。

相关既有文档：

- `docs/requirement/20260623-long-term-identity-device-enrollment.md`
- `docs/analyze/20260626-user-system-product-shape.md`
- `docs/requirement/20260626-cloud-gate-poc.md`
- `docs/spec/20260626-cloud-gate-poc.md`
- `docs/verification/20260626-cloud-gate-poc.md`
- `docs/requirement/20260626-basic-user-system-auth.md`
- `docs/spec/20260626-basic-user-system-auth.md`
- `docs/verification/20260626-basic-user-system-auth.md`

## Goal

1. 定义用户在云端 Gate 完成注册 / 登录后，如何添加、绑定和使用本地设备。
2. 明确 User Identity、Device Identity、Enrollment Identity 三者边界，避免把 Browser 用户凭据继续作为长期 Agent 设备凭据。
3. 明确当前阶段没有 GUI 应用，第一版必须能通过命令行完成设备绑定与远程连接。
4. 复用当前已有 `TERMBRIDGE_AGENT__CONNECT_URL` / `agent.connect_url` 的远程连接能力作为实现基础，而不是假设必须从零实现远程连接。
5. 支持类似 VS Code 的桌面 / CLI OAuth 授权码模式：命令行打开系统浏览器，使用 Google Desktop OAuth client、Authorization Code + PKCE、localhost / loopback callback 完成用户认证。
6. 认证完成后通过 enrollment exchange 绑定本机设备并获取独立 device credential，而不是把 Google token、Browser JWT 或用户密码保存为 Agent 凭据。
7. 设计时为未来 GUI 应用保留同一套 enrollment abstraction；GUI 只是未来更好的承载，不是第一版功能完成的前提。
8. 明确本地模式连接本地 Gate 的认证策略：本地模式也需要有认证边界，但不能制造与云端账号割裂的第二套长期账号体系。
9. 本地模式采纳方案 A：首次访问跳转 setup 页面，用户通过一次性 setup token 获得与 login 页面效果一致的 Browser 身份凭据，作为后续本地身份凭据；该凭据不要求用户创建独立 local account/password。
10. 保持普通用户不需要理解或手动输入 raw `device_id`。
11. 让登录后的空状态自然指向“添加设备 / 连接此设备”，而不是直接进入空 `/sessions` 或暴露底层 Agent 配置。
12. 设备绑定完成后，用户能在云端 Gate 看到自己有权访问的设备，并选择设备进入 workbench。
13. 本地 Agent 完成绑定后，应获得独立的长期 device credential；后续连接云端 Gate 不应使用 Browser 用户密码或 Browser JWT。
14. 为后续设备撤销、重命名、状态展示、token rotation、GUI onboarding、深链或桌面应用 handoff 留出模型边界。
15. 将本需求作为后续 Spec / Plan / Implementation 的依据。

## Non-goal

1. 本需求阶段不写产品代码。
2. 本任务不同时完成基础用户系统遗留问题修复，例如 email register partial success、request log 敏感信息泄露、account/security UI 或 Google capabilities gating。
3. 本任务不要求立即实现完整 Org / Team / RBAC / invite / audit log。
4. 本任务不要求立即实现生产级 device token rotation、mTLS、硬件密钥、OS credential manager 或企业设备管理策略。
5. 本任务不要求把所有本地 `termbridge serve` 行为改成全新的 GUI 流程；CLI、环境变量和配置文件仍需要作为高级 / 自动化路径存在。
6. 本任务不要求普通用户在登录表单、添加设备表单或 GUI 中输入 raw `device_id`。
7. 本任务不把“网页展示一条 CLI 命令”预设为唯一方案；它只是可能的 fallback / advanced path。
8. 本任务不要求本阶段解决 OAuth 用户与本地 bootstrap 用户合并问题；通过 setup token 得到的本地凭据不应设计成需要与云端账号合并的第二套用户。
9. 本任务不要求把 Cloud Gate 服务器自身是否作为 device 的产品策略一次性定死；它可以作为账户下的一个 device，但不是本需求核心。

## User scenarios

### Scenario 1：云端新用户登录后还没有设备

1. 用户打开云端 Gate 页面。
2. 用户通过邮箱或 Google OAuth 完成注册 / 登录。
3. 系统发现该用户尚无已绑定设备。
4. 页面展示清晰空状态：用户当前已登录，但还没有可访问的设备。
5. 页面提供“添加设备 / 连接此设备”的主操作。
6. 用户不需要看到 raw `device_id`、Agent tunnel credential 或底层配置字段。

期望用户理解：

> 我已经登录了云端 TermBridge。下一步是把一台本地设备加入我的账号。

### Scenario 2：当前无 GUI，用户通过 CLI OAuth 绑定当前设备到云端 Gate

1. 用户在本地命令行执行类似 `termbridge device enroll --gate https://gate.example.com` 的命令。
2. CLI 使用 Google Desktop OAuth client、Authorization Code + PKCE、localhost / loopback callback 打开系统浏览器。
3. 用户在浏览器中完成 Google OAuth。
4. CLI 得到用户认证结果，并通过 Gate 发起 enrollment exchange。
5. Gate 将本机 device 绑定到当前 cloud user，并签发独立 device credential。
6. CLI 写入或更新本地 `agent.connect_url`、device identity 与 device credential。
7. 本地 Agent 后续使用 device credential 连接远程 Gate。

### Scenario 3：用户希望绑定另一台设备

1. 用户在云端 Gate 页面选择“添加另一台设备”。
2. 系统生成短期 enrollment material。
3. 页面展示适合跨设备转移的说明，例如复制链接、复制 token、二维码、发送到目标设备、或复制 CLI 命令。
4. 用户在目标设备上打开未来 GUI 应用或运行命令完成绑定。
5. 目标设备使用 enrollment material 兑换长期 device credential。
6. 设备出现在当前用户的设备列表中。

### Scenario 4：未来 GUI 应用完成绑定

1. 用户打开本地 TermBridge GUI 应用。
2. GUI 检测当前尚未绑定云端账号，或者用户主动选择“连接到云端 Gate”。
3. GUI 支持用户输入 / 粘贴云端 Gate 地址，或接收来自云端网页的 deep link / handoff。
4. GUI 可以打开云端 Gate 的 Google OAuth 登录流程，让用户在浏览器中完成认证。
5. Google OAuth 成功后，GUI 不直接复用 Browser JWT 作为 Agent 凭据，而是拿到用户认证结果后发起或承接 enrollment。
6. GUI 使用 enrollment material 配置 `agent.connect_url` 等必要连接信息。
7. GUI 展示本机将作为设备加入哪个云端账号 / Gate，并要求用户确认。
8. 确认后，GUI 完成 enrollment exchange，保存 device credential，启动或重连本地 Agent。

### Scenario 5：高级用户或无 GUI 环境使用 CLI / 环境变量

1. 用户在服务器、CI、无头环境或偏好命令行场景下添加设备。
2. 云端页面或文档提供 CLI / env / config 形式的 fallback。
3. 该路径可以复用当前已有的 `TERMBRIDGE_AGENT__CONNECT_URL=https://gate.example.com` 能力。
4. CLI 不应要求用户手动指定 raw `device_id`，除非作为高级覆盖项。
5. CLI 完成绑定后，设备凭据独立保存，后续连接不依赖 Browser 用户密码。

### Scenario 6：用户已有设备

1. 用户登录云端 Gate。
2. 系统列出该用户已绑定的设备。
3. 每台设备展示用户可理解的信息：名称、平台、online/offline、last seen。
4. 如果只有一台 online 设备，产品可以直接进入 workbench，但仍应保留当前设备上下文。
5. 如果多台设备，用户先选择设备再进入 `/sessions`。
6. 设备 offline 时，页面显示 offline 状态，而不是让用户误以为登录失败。

### Scenario 7：enrollment 失败或过期

1. 用户创建了添加设备流程，但 token 过期、已使用、被撤销或网络失败。
2. 本地 GUI / CLI 显示可理解错误。
3. 云端页面显示该 enrollment 已失败或过期，并允许重新生成。
4. 系统不创建半绑定设备，不签发长期 device credential。

### Scenario 8：设备撤销

1. 用户在云端设备列表中选择移除或 revoke 某台设备。
2. 系统撤销该设备的长期 device credential。
3. 已连接 Agent 应断开或在下一次请求时被拒绝。
4. Browser 不再能进入该设备的 workspace/session/terminal。

### Scenario 9：本地模式连接本地 Gate，setup 页面获取 login 等效凭据

1. 用户只在本机运行 `termbridge serve`，`agent.connect_url` 与 `gate.listen_url` 归一化后一致，系统处于本地模式。
2. 本地 Browser 打开本地 Gate 页面。
3. 如果当前浏览器没有有效身份凭据，系统跳转到 `/setup?token=...` 或等价 setup 页面。
4. setup token 来自本地 Gate first-run / current-run 输出、内部跳转或受限引导，短期有效且不可作为长期凭据。
5. 用户在 setup 页面确认“允许此浏览器访问本机 TermBridge”。
6. setup 页面调用后端换取与 login 页面效果一致的 Browser 身份凭据，例如与 `/api/auth/login` 同形态的 JWT / access token。
7. 前端按正常 login 成功路径保存该凭据，后续 `/api/auth/me`、路由 guard、`/sessions` 工作台都使用同一认证机制。
8. 本机 device 自动绑定 / auto-claim 到该本地访问上下文。
9. 本地模式不要求用户创建独立 local owner/password，也不强制依赖 Google OAuth 或云端账号。
10. 后续切换到远程 Gate 时，用户通过 cloud enrollment 把当前本机 device 绑定到云端账号，而不是合并两个用户账号。
11. 如果用户裸打开本地 Web 根路径 `/`，且当前浏览器未登录但本地 Gate 仍有可用 setup token，页面应进入 setup 引导。
12. 如果用户进入 `/setup` 但 URL 中没有 setup token，页面应解释需要使用 `termbridge serve` 启动时输出的 `/setup?token=...` 本地访问链接。
13. `/login` 是云端账号登录入口。

## Acceptance

### 产品与用户路径

- [ ] 用户登录云端 Gate 后，如果没有设备，会看到“添加设备 / 连接设备”的清晰空状态。
- [ ] 用户登录云端 Gate 后，如果已有设备，会看到自己有权访问的设备列表。
- [ ] 普通用户不需要在登录或添加设备主路径中输入 raw `device_id`。
- [ ] 产品文案把设备表达为“加入账号 / 连接设备 / 我的设备”，而不是要求用户理解 Agent credential。
- [ ] 设备 offline、未授权、enrollment 过期、网络失败有不同且可理解的反馈。
- [ ] 单设备场景可以自然进入 workbench，多设备场景需要明确选择设备。

### 与现有远程连接能力的关系

- [ ] 需求明确复用现有 `TERMBRIDGE_AGENT__CONNECT_URL` / `agent.connect_url` 作为本地 Agent 连接远程 Gate 的底层能力。
- [ ] Spec 阶段必须说明 enrollment 成功后如何写入或管理 `agent.connect_url`、`device_id`、`device_name`、device credential 等本地配置。
- [ ] CLI / env / config 路径保留为高级或无 GUI 场景，不作为唯一用户体验。

### 本地模式认证

- [ ] 本地模式连接本地 Gate 时仍需要认证边界，不能把本地 HTTP 端口视为天然可信。
- [ ] 本地模式认证不应强制依赖云端 Google OAuth 或云端账号。
- [ ] 本地 first-run / unauthenticated access 采纳方案 A：跳转 setup 页面。
- [ ] 未认证打开本地 Web 根路径 `/` 时，如果本地 Gate 有可用 setup token，应进入 setup 引导。
- [ ] 未认证访问受保护页面时，如果本地 Gate 有可用 setup token，应进入 setup 引导，并保留安全 redirect。
- [ ] setup 页面能通过一次性 setup token 换取与 login 页面效果一致的 Browser 身份凭据。
- [ ] `/setup` 缺少 token 时，应展示需要使用启动时输出的本地访问链接的说明。
- [ ] setup 成功后的前端保存、认证恢复、路由 guard 与 `/api/auth/me` 行为应与普通 login 成功保持一致。
- [ ] setup token 不作为长期登录凭据，不进入普通 request log。
- [ ] 本地模式不要求用户创建独立 local owner/password，不制造与云端账号割裂的第二套长期账号体系。
- [ ] 本地 Agent 连接本地 Gate 应通过本地 setup 或自动 claim 获取本地 device credential。
- [ ] `auth.local_admin` shortcut 只能作为开发 / bootstrap 兼容路径。

### GUI 应用适配

- [ ] 当前无 GUI 时，第一版仍可通过 CLI OAuth enrollment 完成云端设备绑定。
- [ ] 未来 GUI 应用应能复用同一套 enrollment abstraction。
- [ ] GUI 可以使用云端 Gate 已有 Google OAuth 作为用户认证入口。
- [ ] Spec 阶段需要比较 CLI OAuth、未来 GUI 内 Google 登录、系统浏览器 OAuth + loopback/deep link callback、云端网页 deep link 唤起 GUI 等方式。
- [ ] GUI 完成 Google OAuth 后仍必须通过 enrollment exchange 获取 device credential，不能把 Google token、Browser JWT 或用户密码保存为 Agent 凭据。
- [ ] 用户在 GUI 中能确认“这台设备将加入哪个 Gate / 哪个账号”。
- [ ] GUI 绑定流程不泄露长期 device credential 明文。

### 身份与凭据边界

- [ ] Browser 用户登录态、一次性 setup/enrollment material、长期 device credential 三者语义分离。
- [ ] Setup token / enrollment material 短期有效、一次性使用或可撤销，不能作为长期 device credential。
- [ ] Device credential 独立于用户密码和 Browser JWT。
- [ ] 服务端不以明文保存 setup token、enrollment token 或 device token。
- [ ] Agent 后续连接云端 Gate 使用 device credential，而不是 email/password、Google OAuth token 或 Browser JWT。
- [ ] Device id 是资源标识，不是 secret。

### 数据与授权模型

- [ ] 用户与设备之间存在明确绑定关系。
- [ ] `/api/devices` 或等价接口只返回当前用户有权访问的设备。
- [ ] 进入 `/sessions` 前存在 selected device 上下文。
- [ ] workspace/session/terminal 操作必须归属于 selected device。
- [ ] 设备撤销后，Agent 连接和 Browser 操作都应失效。
- [ ] 本地 setup 访问上下文与后续 cloud account 不需要做账号合并；切换远程 Gate 时绑定的是 device。

### 安全与日志

- [ ] Setup token、enrollment token、device token、Browser JWT、OAuth code/state、password 不应进入普通 request log 或错误日志。
- [ ] Setup / enrollment exchange 接口需要考虑 rate limit / replay protection。
- [ ] 过期或已使用 setup / enrollment material 不能再次兑换 Browser credential 或 device credential。
- [ ] 设备绑定动作需要有明确用户确认边界，尤其是 GUI / deep link 方式。

## Open questions

1. **CLI 命令具体形态是什么？**
   - 可能是 `termbridge device enroll --gate ...`、`termbridge agent enroll --gate ...` 或 `termbridge serve --connect ... --enroll`。
   - 具体命名进入 Spec / Plan 阶段比较。

2. **本地 setup token 如何展示或传递？**
   - 可由 `termbridge serve` 控制台输出 setup URL。
   - 也可由未认证访问本地 Gate 时引导用户回到终端复制 token。
   - 具体 UX 进入 Spec 阶段设计。

3. **本地 setup 换取的 login 等效凭据使用什么 provider 标识？**
   - 候选：`provider=local_setup`、`provider=local_access`、`provider=local_admin`。
   - 需要避免和正式 email / google account 混淆。

4. **本地 setup 后是否签发本地 device credential？**
   - 当前需求倾向需要，使 Browser credential 与 Agent credential 分离。
   - Spec 阶段需明确是否第一版就落地，或作为紧随后续。

5. **Enrollment 成功后是否自动启动 / 重启 Agent？**
   - CLI 场景可能只写配置并提示下一步，也可能直接启动连接。
   - GUI 场景未来可能应该自动启动并展示连接状态。

6. **本地已有 `.termbridge.yaml`、已有 `agent.device_id`、已有连接远程 Gate 配置时如何处理？**
   - 需要定义覆盖、迁移、确认和冲突处理。

7. **是否需要支持一个设备绑定多个用户或多个 personal space？**
   - 第一版可以只支持 owner 绑定，但数据模型不宜完全阻断未来 shared device / org。

8. **device credential 本地存储位置是什么？**
   - 第一版可能写配置文件或 state dir；长期可能进入 OS credential manager / GUI app secure storage。

## Decisions

1. 本任务使用严格模式 / strict，从 Requirement / 需求阶段开始。
2. 本任务创建新的 SpecFlow 文档，不沿用基础用户系统或 Cloud Gate PoC 文档。
3. 用户已要求进入 Spec，因此本 Requirement 标记为 Accepted。
4. 产品模型继续坚持 `User -> Device -> Workspace -> Session -> Terminal`。
5. 用户登录云端 Gate 只证明“用户是谁”，不自动证明“用户控制哪台本地设备”。
6. 设备绑定必须通过显式 enrollment / pairing / claim 流程完成。
7. 当前已有 `TERMBRIDGE_AGENT__CONNECT_URL` / `agent.connect_url` 远程连接能力，应作为底层能力复用。
8. 当前没有 GUI，第一版云端设备绑定必须能通过 CLI OAuth enrollment 完成。
9. 未来设想中存在 GUI 应用，因此需求不能把“网页展示命令行”固化为唯一主路径；GUI 应复用同一套 enrollment abstraction。
10. 普通用户主路径不要求输入 raw `device_id`。
11. Browser user credential、setup/enrollment material、device credential 必须拆分语义。
12. Agent 长期连接 Gate 不应使用 Browser 用户密码或 Browser JWT。
13. Google GUI / CLI 认证可以采用类似 VS Code 的桌面应用 OAuth 授权码模式：系统浏览器 + Authorization Code + PKCE + localhost / loopback callback；用户可在 Google Cloud Console 添加独立 Desktop OAuth client。
14. 本地模式采纳方案 A：未认证访问本地 Gate 时跳转 setup 页面；setup 页面用一次性 setup token 换取与 login 页面效果一致的 Browser 身份凭据，作为后续本地身份凭据。
15. 本地模式不默认创建与云端账号并列的长期 local account/password；后续切换远程 Gate 时绑定的是当前 device，而不是合并本地账号和云端账号。
16. Spec 需要包含关键流程图。
17. 2026-06-28 review 采纳改进：本地首次访问 UX 应围绕 setup token；裸打开本地首页或受保护页面时，如果存在可用 setup token，应进入 setup 引导。

## Risks

1. **产品路径风险**：如果第一版只在网页展示 CLI 命令，未来 GUI 应用接入时可能需要重做用户路径和接口模型。
2. **OAuth 安全边界风险**：CLI / GUI OAuth loopback callback 若设计不严谨，可能引入 code/token 泄露、state 校验缺失或错误 Gate 绑定风险。
3. **凭据混用风险**：继续让 Agent 使用用户密码或 Browser JWT 会阻碍设备撤销、rotation、OAuth/SSO 和审计。
4. **配置迁移风险**：当前已有 `agent.connect_url`、`agent.device_id`、`.termbridge.yaml`，enrollment 实现需要避免覆盖用户已有配置或制造半绑定状态。
5. **范围膨胀风险**：设备绑定容易扩展到完整设备管理、Org/RBAC、安装包、自动更新和 secure storage；本任务需要先定义第一版最小闭环。
6. **日志泄露风险**：setup token、enrollment token、device token、OAuth code/state、Browser JWT 如果进入 request log，将直接破坏绑定安全性。
7. **体验分叉风险**：CLI、GUI、网页三条路径若缺少统一 enrollment abstraction，会导致行为不一致。
8. **本地与云端模式混淆风险**：本地 self-connected、云端 Gate、本地 Agent 连接远端 Gate 三种形态需要保持同一产品心智，但配置语义不同。
9. **本地凭据误解风险**：setup 页面签发 login 等效凭据后，必须在 UI 和 claims 语义上避免让用户以为创建了另一个需要与云端合并的正式账号。

## User review notes

用户原始请求：

> 严格 开始新任务
> 目前本地是有连接远程能力的，只不过是基于 TERMBRIDGE_AGENT__CONNECT_URL，而且设想中我们会提供 gui 应用，在页面上展示命令行的方式还要考虑下

后续用户补充：

> 当前云端和本地 gate 页面已经接入 google oauth，是不是可以添加 google gui 认证？

> 基于和vscode 标准授权码模式一样的方式，使用 goolge 认证能否做到？我可以在 cloud.google.com 上添加 oauth 客户端

> 注意我们当前并没有 gui，是一个命令行，是否能完成功能

> 这个策略没有问题。同时还要继续明确，本地模式连接本地gate而不是云端gate的模式，1. 要不要认证 2. 如何认证

> 本地模式如果这么一套流程走下来' creates local owner / sets password'，又如何切换到远程gate模式？我不太想搞出多套账号

> 采纳方案A，本地模式跳转 setup 页面，在哪里得到和 login 页面效果一致的凭据，作为后续身份凭据。/specflow 开始 spec，需要包含关键流程图

本需求按以下理解收口：

- 这是一个新的严格模式 SpecFlow 任务。
- 当前没有 GUI，因此第一版必须能通过 CLI 完成云端设备绑定。
- Google OAuth 可采用类似 VS Code 的系统浏览器 + Authorization Code + PKCE + loopback callback。
- 用户可在 Google Cloud Console 添加 Desktop OAuth client。
- 未来 GUI 应复用同一套 enrollment abstraction。
- 本地模式也需要认证边界，但不创建另一套长期 local account/password。
- 本地模式采纳方案 A：跳转 setup 页面，用 setup token 换取和 login 页面效果一致的 Browser 身份凭据。
- 2026-06-28 review 决定改进本地首次访问 UX：存在可用 setup token 时自动进入 setup 引导，`/setup` 缺 token 时说明应使用启动输出的本地访问链接。
- 切换远程 Gate 时，绑定的是本机 device 到 cloud account，不是合并本地账号和云端账号。
