# Cloud 认证 Provider 扩展（GitHub 等来源）
最后修改时间: 2026-07-15 07:30:26

Review status: Accepted

## Background

当前 Cloud 模式已将 Google OAuth2 作为 Cloud 账号认证 Provider 接入。Agent Dashboard 仅对接 Cloud 自身的 OAuth2 authorization endpoint；Cloud 在内部完成邮箱/密码或 Google 登录后，再向 Agent 签发 Cloud authorization code 和 token。因此，新增 GitHub 等第三方来源应扩展 **Cloud 账号认证层**，不得让 Agent 直接感知或调用任一第三方 Provider。

现有持久化模型已采用 `user_identities(provider, provider_subject)` 表达身份来源，Google 使用 provider=`google` 与 Google `sub` 查找身份；`users` 保存规范化邮箱、状态和资料。该模型具备承载多个 Provider 的基础，但现有用户模型会把相同邮箱视为同一用户，无法满足“相同邮箱可在不同 Provider 下分别注册/登录”的新规则。应用服务、HTTP API、Proto、前端路由及配置也仍以 Google 单一 Provider 命名和注入。

## Goal

1. 将 GitHub 作为第二个 Cloud 登录 Provider 接入，并形成可继续扩展到其他 OAuth2/OIDC Provider 的方向。
2. 保持 Agent 与第三方 Provider 解耦：Agent 只继续面向 Cloud OAuth2，不读取、保存或交换 GitHub/Google token。
3. 以 `(provider, email)` 表达 Cloud 账号的业务唯一性：同一 Provider 下相同邮箱只能对应一个账号；相同邮箱允许在 `email`、`google`、`github` 等不同 Provider 下分别存在账号，账号绑定能力后续单独处理。
4. 使用 Provider 稳定 subject 验证同一第三方身份的再次登录，避免以可变用户名作为身份主键。
5. 明确 Provider 配置、启用状态、回调、错误反馈、测试和安全约束，为后续实现提供验收基准。

## Non-goal

- 本需求阶段不修改产品代码、数据库迁移、Proto、前端路由或配置文件。
- 不改变现有 Agent ↔ Cloud OAuth2 authorization-code 协议、client 注册、设备上报或 tunnel 生命周期。
- 不让浏览器或 Agent 直接调用 GitHub OAuth token endpoint，也不将第三方 access token / refresh token 持久化到 Agent 或 Cloud。
- 不在本期引入 GitHub repository、组织、团队、安装、Webhook、代码读取或写入能力。
- 不提供账户 link、unlink、合并或跨 Provider 统一身份视图；这些能力在后期单独处理。
- 不把 GitHub login/username 作为 `provider_subject`。
- 不在本期新增 Provider capability API 或要求登录页由服务端动态枚举 Provider；前端能力发现方式可在后续需求中处理。
- 不以本需求顺带解决 Cloud 内存 authorization-code store 的高可用问题；当前暂不考虑多实例/滚动重启场景。

## User scenarios

1. 未登录用户访问 Cloud 登录页，且 GitHub Provider 已启用时，可选择“使用 GitHub 登录”。
2. 用户完成 GitHub OAuth App 授权后，Cloud 校验一次性 state、交换授权码并读取最小身份资料；首次成功登录创建一个以 GitHub 不可变用户 ID 为 subject、以 `(github, email)` 唯一标识的 Cloud 账号。
3. 已通过 GitHub 创建 Cloud 账号的用户再次登录时，Cloud 按 `(provider="github", provider_subject=<immutable-id>)` 找回同一账号并签发 Cloud JWT。
4. 同一规范化邮箱已作为 `email` 或 `google` Provider 的账号存在时，GitHub 登录仍可创建独立的 `github` Provider 账号；反之亦然。系统不得自动合并或绑定这些账号。
5. 同一 Provider 下，邮箱已存在但 external subject 不一致时，Cloud 拒绝创建重复账号并给出不泄露敏感信息的认证失败反馈。
6. GitHub 未提供可用且已验证的邮箱时，Cloud 拒绝登录并明确说明“GitHub 未提供可用的已验证邮箱”；state 无效/过期/已消费或 Provider 返回的 subject 无效时，Cloud 拒绝登录并给出不泄露敏感信息的认证失败反馈。
7. 用户通过 GitHub 登录后，从 Agent Dashboard 发起 Cloud OAuth2 授权，流程与邮箱/Google 登录一致：Agent 仅接收由 Cloud 签发的 code/token 并完成既有设备连接。
8. GitHub Provider 未配置或被禁用时，Cloud 登录页不展示可用入口（或展示受控不可用提示）；直接调用相应入口也必须由服务端拒绝。
9. 后续添加其他 Provider 时，应复用统一的 Provider 契约、身份模型、state 校验和配置/启用规则，而非复制 Google/GitHub 专用完整链路。

## Acceptance

- [ ] GitHub 被明确定位为 Cloud 登录 Provider；Agent 前端、Agent API 和设备绑定流程不直接依赖 GitHub。
- [ ] 本期采用 **GitHub OAuth App**，仅授权完成用户登录所需的最小身份资料；不申请仓库、组织、团队或写入权限。
- [ ] Provider identity 使用 GitHub 不可变用户 ID（字符串化）作为 `provider_subject`，不使用 login、用户名或邮箱作为稳定主键。
- [ ] 账号业务唯一性为 `(provider, normalized_email)`：同一邮箱可分别属于 `email`、`google`、`github` 等不同 Provider；相同 Provider 不允许同邮箱重复建号。
- [ ] 再次登录身份匹配以 `(provider, provider_subject)` 为依据；对于同一 Provider，邮箱与已存账号不一致或 subject 与已存邮箱账号不一致时必须明确拒绝，不能创建歧义账号。
- [ ] 本期不自动合并或绑定跨 Provider 的同邮箱账号，也不提供 link/unlink 管理入口；后期另行设计账户绑定和统一身份语义。
- [ ] 每次外部 OAuth 回调均校验服务端生成、保存且一次性消费的 state；多 Provider 并存时，state 能证明或约束其所属 Provider，防止 callback/provider 混用。
- [ ] GitHub 仅在获得可用且已验证的邮箱后创建或登录账号；没有公开邮箱、返回多个但无法选出可用已验证邮箱、或仅有未验证邮箱时，拒绝登录并向用户说明原因。
- [ ] Provider client 以 provider-neutral 契约表达：生成授权 URL、交换 code、返回标准化外部身份（provider、subject、邮箱验证状态、显示名）及可控错误；Provider endpoint、scope 和资料读取逻辑封装在 adapter/config 层。
- [ ] 用户与 identity 创建/查找接口不再绑定 `Google` 命名，支持以 provider、subject、邮箱和显示名创建外部身份账号；新建 GitHub 账号的 `display_name` 使用规范化邮箱，并保留既有 email/password 用户语义。
- [ ] Cloud HTTP API、Proto、Web route/store 不再只能表示 Google；Provider 入口和 callback 以统一、可枚举且受服务端校验的方式暴露。动态 capability API 不属于本期范围。
- [ ] GitHub OAuth App 凭据、redirect URL、启用状态和 scope 通过独立配置项表达；secret 不进入 browser runtime config、日志或错误响应。
- [ ] GitHub 登录成功后的 Cloud JWT `provider` claim 与实际登录 Provider 一致；Cloud OAuth2 authorization-code 向 Agent 签发的 token 也必须保留用户实际登录 Provider，不能改写为 `email`。
- [ ] Google 登录、邮箱/密码登录、Cloud OAuth2 authorization-code exchange 和 Agent 设备连接维持现有行为；其中不同 Provider 的相同邮箱不得被错误视为同一 Cloud 账号。
- [ ] 补充服务层、HTTP、配置与前端测试，以业务结果验证：成功首次/再次 GitHub 登录、允许跨 Provider 同邮箱建号、拒绝同 Provider 同邮箱歧义建号、无可用已验证邮箱、state 重放、Provider 禁用，以及 GitHub 登录后的 Agent Cloud 连接。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- 新增 Provider 的边界固定在 Cloud account login；Cloud 继续作为 Agent 所见的唯一 OAuth2 Authorization Server。
- 本期 GitHub 集成采用 GitHub OAuth App，范围严格限于登录/注册 Cloud 账号。
- 账号业务唯一性采用 `(provider, normalized_email)`；同一邮箱允许在 `email`、`google`、`github` 等不同 Provider 中重复存在，并被视为独立账号。账号绑定/合并留待后期处理。
- `(provider, provider_subject)` 继续作为外部 Provider 身份的稳定再次登录定位键；Provider subject 必须是上游声明的不可变 ID。
- 多 Provider 设计应优先形成 Provider-neutral registry/adapter 和统一外部身份 DTO，避免继续新增 `GitHubClient`、`GitHubCallback` 等平行的端到端专用链路；HTTP 和前端可保留清晰的 provider 路径，但必须共享校验与错误语义。
- 第三方 token 仅用于 callback 期间获取身份资料，默认使用最小 scope、online access，并且不落库。
- GitHub 没有可用已验证邮箱时拒绝登录，并向用户明确说明原因；多邮箱时仅可选择符合实现阶段确定规则的可用已验证邮箱，否则同样拒绝。
- GitHub 新建账号的 `display_name` 使用规范化邮箱，不使用 GitHub `name` 或 `login`。
- Cloud OAuth2 authorization-code 向 Agent 换发的 token 必须保留用户实际登录 Provider；不得固定改写为 `email`。
- 本期不新增登录页动态 Provider capability API；Provider 显示和启用控制采用实现阶段确定的最小方案，服务端仍必须是最终校验边界。
- 当前暂不考虑 Cloud 多实例、滚动重启及 Agent-facing authorization-code 内存存储的高可用改造。

## Risk

- 现有 `users.email_normalized` 的唯一性及“一个 user 多 identity”的模型与“`(provider, email)` 独立账号”规则可能冲突；实现必须重新确认用户表、identity 表与迁移约束，不能只调整 service 层判断。
- GitHub 的 `/user` 资料不保证携带可用邮箱；读取邮箱列表的 API、所需 scope、primary/verified 选择策略和错误分类都需要在技术方案中落实，不能照搬 Google `email_verified` 字段。
- 当前 `oauth_states` 不记录 Provider，单 Google 时可用；多 Provider 后若不绑定/命名空间化 state，可能出现 callback 被错误 Provider 消费的混淆风险。
- 当前 Service、Repository、HTTP API、Proto 和 Web 均有 Google 专用类型/命名，直接复制 GitHub 版本会造成 provider 数量线性扩张和长期维护债务。
- 当前 Cloud OAuth token 签发路径将 user provider 固定为 `email`；本期已要求保留实际登录 Provider，实施必须调整该路径并覆盖其与 Agent 的传递语义。
- Provider secret、callback URL 和 scope 配置错误会导致登录失败或凭据暴露；配置校验、运行时禁用和日志脱敏必须是交付的一部分。
- 对现有身份模型的通用化会触及认证主路径，必须使用业务语义测试覆盖既有 email/Google 登录、跨 Provider 同邮箱隔离和新增 GitHub 登录，避免回归。

## User review notes

- 2026-07-15：确认 GitHub 严格用于 Cloud 登录/注册，并采用 GitHub OAuth App。
- 2026-07-15：确认以 `(provider, normalized_email)` 作为账号业务唯一性；允许 email、Google、GitHub 等不同 Provider 使用相同邮箱创建独立账号，账户绑定后期再处理。
- 2026-07-15：确认本期暂不考虑登录页动态 Provider capability API，也不考虑 Cloud 多实例/authorization-code 高可用改造。
- 2026-07-15：确认 GitHub 缺少可用已验证邮箱时拒绝登录并说明原因；新建账号显示名使用邮箱；Cloud OAuth2 向 Agent 换发 token 时保留实际登录 Provider。
- 当前阶段仅完成现状调研和 Requirement 草稿，未修改产品代码或运行产品验证。
