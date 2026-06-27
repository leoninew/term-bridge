# 长期身份、设备绑定与接入模型需求

最后修改时间: 2026-06-23 13:26:28

Review status: Draft

## Background

当前统一 Gate / Device / Auth 工作台已经将访问链路收敛为：

```text
Browser -> Gate -> Device Agent -> Runtime -> Workspace -> Session -> Terminal
```

本阶段实现中，为了快速打通本地 self-connected 和统一工作台路径，Browser user credential 与 Agent device credential 暂时共用 `.termbridge.yaml` 中的 `auth.username` / `auth.password`。首次启动时，如果缺失凭据，会自动生成账号和密码，并用于 Browser 登录与 Agent 连接 Gate。

这个模型适合作为本地 MVP bootstrap，但不应作为长期身份体系。长期产品将可能部署到云端，并接入第三方 OAuth / OIDC / SSO。届时需要明确：

- 用户身份不应由本地设备配置长期定义。
- 用户名不应被当作全局唯一身份。
- 设备不应参与用户登录本身。
- Agent 连接凭据不应复用 Browser 用户密码。
- Device 应通过 enrollment / pairing 流程绑定到 User / Org / Tenant。

因此需要单独记录长期身份、设备绑定和接入模型，供后续实现推进。

## Goal

1. 定义长期产品身份模型：User / Account / Tenant / Org 先于 Device 权限选择。
2. 将 Browser 用户身份与 Agent 设备身份拆分为两个认证域。
3. 定义 Device enrollment / pairing 流程，用于将设备绑定到用户或组织。
4. 支持未来 Cloud / SaaS 部署下的 OAuth / OIDC / SSO 登录。
5. 保留本地 self-connected 使用体验，但将当前生成 username/password 的方式定位为 first-run bootstrap shortcut，而非长期目标架构。
6. 明确登录、设备选择、设备接入三条流程的产品形态。
7. 避免让普通用户在登录表单中直接输入 raw `device_id`。
8. 为后续从当前 MVP credential 模型迁移到长期模型提供需求依据。

## Non-goal

1. 本需求不立即修改当前已实现的 unified Gate / Device / Auth 工作台。
2. 本需求不要求本阶段立刻接入 OAuth / OIDC / SSO。
3. 本需求不要求现在实现多租户 SaaS、团队权限、RBAC 或审计系统。
4. 本需求不要求现在重构 `.termbridge.yaml` 的所有配置字段。
5. 本需求不要求现在实现完整设备管理 UI。
6. 本需求不要求现在迁移已经生成的本地 auth/device 配置。

## Long-term product model

长期访问模型应为：

```text
Gate / Realm / Deployment
  └─ Tenant / Org / Personal Space
      └─ User / Account
          └─ authorized Devices
              └─ Workspace
                  └─ Session
                      └─ Terminal
```

Browser 访问链路：

```text
Browser User
  -> authenticates to Gate
  -> enters account / tenant context
  -> selects an authorized Device
  -> Gate routes requests to Device Agent
  -> Runtime / Workspace / Session / Terminal
```

Device 接入链路：

```text
Device Agent
  -> enrolls with Gate using one-time pairing material
  -> receives device credential
  -> reconnects with device credential
  -> reports device identity and capabilities
  -> appears in authorized device list
```

本地 self-connected 只是 Cloud 模型的压缩形态：

```text
Local Gate first-run
  -> bootstrap local user / admin
  -> local Agent enrolls or auto-claims local device
  -> Browser user logs in
  -> user selects local device
  -> enters /sessions
```

## Identity domains

### Browser User Identity

Browser User Identity 表示“人”或“账号”。长期应支持多种来源：

- local password
- OAuth
- OIDC
- SSO
- future enterprise identity provider

内部身份不应依赖全局唯一 username，而应有稳定主键：

```text
user_id
provider
provider_subject
tenant_id / realm_id
login_name / display name
email
```

唯一性建议：

```text
tenant_id + provider + provider_subject
```

对于 local password provider，可采用：

```text
realm_id + username
```

`username` 仅作为登录名或展示名，不作为全局唯一用户身份。

### Device Identity

Device Identity 表示机器或 Agent。Device 应有稳定设备标识和独立设备凭据：

```text
device_id
device_name
hostname
platform
capabilities
device_credential / device_token / client certificate
owner tenant / bound users
created_at
last_seen
revoked_at
```

Device id 是资源标识，可出现在 URL、日志和诊断信息中，不应被当作 secret。

Agent 连接 Gate 时应使用 device credential，而不是 Browser user password。

### Enrollment / Pairing Identity

Enrollment / Pairing 是将 Device 绑定到 User / Org / Tenant 的临时过程。

典型材料：

- one-time setup token
- pairing code
- device claim URL
- short-lived enrollment token

要求：

- 短期有效。
- 一次性使用或可撤销。
- 不等同于长期 user credential。
- 不等同于长期 device credential。
- 使用后换取长期 device credential。

## Product flow

### Cloud / SaaS login flow

```text
User opens Cloud Gate
  -> signs in with OAuth / SSO / password
  -> enters account / org
  -> sees authorized devices
  -> selects device
  -> enters /sessions
```

设备列表只能在用户认证后展示。未登录时不展示 device list。

### Cloud / SaaS device enrollment flow

```text
User opens Cloud UI
  -> Add Device
  -> Gate creates enrollment code / claim URL
  -> user runs local command with code
  -> Agent exchanges code for device credential
  -> Device becomes bound to user/org
  -> Device appears in Cloud device list
```

示例命令形态仅作为后续设计输入：

```text
termbridge agent enroll --gate https://gate.example.com --code <pairing-code>
```

或：

```text
termbridge serve --enroll <claim-url>
```

### Local self-connected first-run flow

本地首次启动时，可能没有云端用户、没有 OAuth、没有已存在账号。此时应进入 bootstrap mode：

```text
termbridge serve
  -> local Gate starts
  -> detects no local user/admin
  -> creates one-time setup token
  -> prints setup URL once
  -> user opens setup URL
  -> user creates or confirms local admin
  -> local Agent enrolls / auto-claims local device
  -> user logs in and selects local device
```

示例控制台提示：

```text
TermBridge first-run setup:
Open http://127.0.0.1:9010/setup?token=xxxx

This token is shown once and expires after setup.
```

当前 `.termbridge.yaml` 自动生成 `auth.username/password` 的方式可视为该流程的 MVP shortcut。长期应逐步演进为 setup token + local user + device enrollment，而不是长期复用同一 username/password 给 Browser 和 Agent。

### Login and device selection UI

登录和设备选择可以出现在同一个视觉入口卡片内，但语义应分段：

```text
TermBridge

未登录：
  username/password 或 OAuth button

登录后：
  device list / device selector
  selected device status
  enter workbench
```

不建议主流程要求用户直接填写 raw `device_id`。

原因：

1. `device_id` 不是用户友好字段。
2. device list 应在登录后按权限返回。
3. device id 不是 secret，不应混入认证表单。
4. 错误语义会混乱：登录失败、设备不存在、设备无权限、设备离线应分别表达。

可以作为高级能力支持：

- URL 预选：`/sessions?device_id=<id>`。
- 单设备场景默认选中或提示确认。
- 高级调试里手动输入 device id。

## Configuration direction

当前 MVP 配置：

```yaml
auth:
  username: <os username>
  password: <generated password>

agent:
  server_url: http://127.0.0.1:9010
  device_id: <generated id>
  device_name: <hostname>
```

长期应拆分语义，避免 user credential 与 device credential 绑定。

可能方向：

```yaml
gate:
  auth:
    provider: local

local_user:
  username: <local admin>
  password_hash: <hash>

agent:
  server_url: http://127.0.0.1:9010
  device_id: <stable device id>
  device_name: <hostname>
  device_token: <device credential>
```

或 Cloud enrollment 后：

```yaml
agent:
  server_url: https://gate.example.com
  device_id: <stable device id>
  device_name: <display name>
  device_token: <issued by gate>
```

要求：

- Browser user password 不写入 Agent credential 字段。
- Agent device token 不作为 Browser 登录密码。
- local first-run setup token 不长期保存为登录密码。
- password 后续应考虑 hash、文件权限和 secret 管理。

## Acceptance

### Conceptual model

- [ ] 长期模型明确区分 Browser User Identity、Device Identity 和 Enrollment Identity。
- [ ] User / Account / Tenant 是权限选择的上层语义，Device 是被授权访问的资源。
- [ ] Device id 是资源标识，不是认证 secret。
- [ ] username 不被视为全局唯一身份。

### Product flow

- [ ] Cloud 场景下，用户先通过 OAuth / SSO / password 登录，再选择有权访问的设备。
- [ ] Cloud 场景下，设备通过 enrollment / pairing 绑定到用户或组织。
- [ ] 本地 self-connected 场景下，首次启动使用 bootstrap/setup 流程创建本地 user/admin 并绑定本机 device。
- [ ] 登录和设备选择可以在同一视觉表单/卡片中完成，但语义上先认证用户，再选择设备。
- [ ] 普通登录表单不要求用户手动输入 raw `device_id`。

### Credential separation

- [ ] Browser user credential 和 Agent device credential 可独立轮换、撤销和审计。
- [ ] Agent 连接 Gate 不使用 Browser 用户密码。
- [ ] Device enrollment 使用短期一次性材料，完成后换取长期 device credential。
- [ ] 当前共用 `auth.username/password` 被定位为 MVP bootstrap shortcut，不作为长期目标。

### Migration readiness

- [ ] 后续实现可以从当前 `.termbridge.yaml` 生成凭据平滑演进到 setup token + local user + device token。
- [ ] 文档明确旧 MVP credential 模型的边界和未来替换方向。
- [ ] UI 可以从当前登录后设备选择演进为 OAuth/local login + device selector，而无需改变 workspace/session 主工作台。

## Open questions

1. 本地 first-run 是否应该默认自动创建 local admin，还是必须通过 setup URL 交互创建？
2. 本地 self-connected 的本机 device 是否自动绑定到 local admin，还是仍要求显式确认？
3. Cloud enrollment 使用 pairing code、claim URL、device login flow 还是多种方式并存？
4. Device credential 长期使用 bearer token、mTLS client certificate，还是平台密钥库托管 token？
5. local password 是否要继续存储于 `.termbridge.yaml`，还是迁移到单独 secret store / OS credential manager？
6. 单用户本地部署是否需要显式 Tenant / Org 概念，还是使用隐式 personal realm？
7. OAuth 用户和 local bootstrap 用户如何合并或迁移？

## Decisions

当前已形成的方向性判断：

1. 长期应先认证 User，再选择该 User 有权访问的 Device。
2. 不使用 raw `device_id` 作为普通用户登录表单字段。
3. Device id 是资源标识，不是 secret。
4. Browser 用户凭据和 Agent 设备凭据长期必须拆分。
5. 当前 Browser 和 Agent 共用 `auth.username/password` 只能视为 Phase 1 bootstrap shortcut。
6. Cloud 场景下设备应通过 enrollment / pairing 绑定到 User / Org / Tenant。
7. 本地 first-run 应逐步演进为 setup token / setup URL，而不是长期生成可复用明文密码。

## Risks

1. **过早引入复杂身份系统**：如果在本地 MVP 未稳定前引入完整 OAuth/tenant/RBAC，可能拖慢核心工作台迭代。
2. **当前 credential 模型被误认为长期模型**：需要在文档和后续计划中持续标注它只是 bootstrap shortcut。
3. **设备绑定语义不清**：如果 device 既像登录因子又像资源，会导致 UI、权限和错误处理混乱。
4. **迁移风险**：从 `.termbridge.yaml` username/password 切到 setup token + device token 时，需要处理已有本地用户配置。
5. **Cloud 安全边界风险**：OAuth user、device token、enrollment token、session cookie 的生命周期和撤销策略必须独立设计。
6. **本地体验复杂化风险**：长期模型不能让单机用户承担云端组织/设备绑定的复杂度；本地应有压缩、自动化但语义一致的流程。

## Suggested future decomposition

### Requirement / Spec follow-up

1. Local first-run setup flow。
2. Device enrollment / pairing flow。
3. Agent device token and credential rotation。
4. OAuth / OIDC browser login。
5. User / tenant / device authorization model。
6. Device management UI。

### Implementation migration sketch

1. 保留当前 local credential 行为作为兼容启动路径。
2. 新增 setup mode：无 local user/admin 时打印一次性 setup URL。
3. 引入 local user store，逐步替代 `auth.username/password` 明文事实来源。
4. 引入 `agent.device_token`，Agent tunnel 不再使用 Browser password。
5. 新增 enrollment endpoint 和 CLI enroll flow。
6. UI 登录卡片支持 local login / OAuth button / setup state / device selector。
7. 后续移除 Browser-Agent 共用 password 的路径。

## User review notes

用户指出：

- 未登录就展示设备列表不合适。
- 让用户填写 `device_id` 又不友好。
- 如果只提供用户名密码就登录，将来用户名可能相同。
- 需要从产品形态和部署架构更长远地设想。
- 将来可能部署到云端，可能接入第三方 OAuth。
- 因此需要推敲到底是先有用户再有设备，还是当前本地生成凭据的形式是否合理。

本需求文档的归纳结论：

- 长期应先有 User / Account / Tenant 认证，再有 Device 选择和授权访问。
- 当前生成凭据形式适合作为本地 first-run bootstrap shortcut，但不是长期身份架构。
- Device 通过 enrollment / pairing 绑定到用户或组织。
- 登录 UI 不要求普通用户填写 raw device id；可以在同一个入口卡片中完成登录后设备选择。
