# 基础用户系统注册、登录与邮箱用户改密需求

最后修改时间: 2026-06-27 14:21:31

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；需求 / Requirement。

## Background

当前 TermBridge-go 已经具备本地 self-connected Gate、Agent、Runtime 与 `/sessions` 工作台，Cloud Gate / container PoC 也已将镜像交付面推进到前后端一体形态。

但当前 PoC 认证仍是临时形态：Browser login 与 Agent tunnel credential 暂时固定或复用最小凭据。这只能作为 bootstrap / PoC shortcut，不适合作为长期用户系统。

已有长期身份需求文档已明确方向：

- `docs/requirement/20260623-long-term-identity-device-enrollment.md`
- `docs/analyze/20260626-user-system-product-shape.md`

本需求是引入正式用户系统的第一步：先实现最基本的用户注册与登录能力，让 Browser 用户身份从 PoC 固定凭据中独立出来。

核心产品方向保持：

```text
User -> Device -> Workspace -> Session -> Terminal
```

用户系统首先回答“谁在访问”，Device 仍是用户有权访问的 runtime 资源，不是登录凭据。

## Goal

1. 引入最基本的 Browser 用户系统，替代当前远程 Gate 面向 Browser 的 PoC 固定登录形态。
2. 支持邮箱来源用户注册与登录。
3. 支持真实 Google OAuth 来源用户注册与登录；Google OAuth 必要配置由部署者提供。
4. 支持用户会话，使登录后的 Browser 可以访问受保护的工作台。
5. 对来源为邮箱的用户，支持修改密码。
6. 对邮箱注册用户，基于 Resend 发送邮箱验证码；邮箱验证后才可以登录。
7. 支持忘记密码 / password reset，并基于 Resend 发送重置验证码。
8. 引入后端持久化存储，用于用户、身份来源、验证码、session 等认证数据。
9. 后端存储需要同时支持 SQLite 和 MySQL，并包含数据迁移能力。
10. 复用现有 JWT 能力作为 Browser session / access token 基础。
11. 明确 email user 与 Google OAuth user 的身份来源差异。
12. 为后续 User -> Device 授权、device enrollment、正式用户-device 绑定留下清晰模型边界。
13. 不让普通用户在登录流程中输入 raw `device_id`。
14. 区分本地模式与远程 Gate 模式：本地模式仍可支持 `admin/admin` shortcut；远程 Gate 模式不支持 `admin/admin`。
15. 本地 / 远程模式通过显式配置判定，不依赖请求 URL 猜测。

## Non-goal

1. 本需求不实现完整 device enrollment / pairing。
2. 本需求不实现 Agent device credential 与 rotation。
3. 本需求不实现多租户 SaaS、Org、Team、RBAC、invite、audit log。
4. 本需求不实现完整设备管理 UI。
5. 本需求不自动合并 email 用户与 Google OAuth 同邮箱用户；同邮箱冲突应断言并提示邮箱已使用。
6. 本需求不要求现在重构 workspace/session/terminal 主工作台的产品模型。
7. 本需求不把 Google OAuth 用户的密码修改纳入范围；Google 来源用户应由 Google 管理凭据。
8. 本需求不将 Agent tunnel credential 继续绑定为 Browser 用户密码；Agent 身份拆分属于后续需求。
9. 本需求不要求本轮完成本地 first-run setup token / setup URL 完整替代；本地 `admin/admin` shortcut 的保留方式进入 Spec 审视。
10. 本需求不要求本轮完成邮箱修改、账号 linking、OAuth linking 或账号合并流程。

## User scenarios

### Scenario 1: 邮箱用户注册

用户打开 TermBridge 登录入口，选择使用邮箱注册。

期望：

1. 用户输入 email 与 password。
2. 系统创建 email 来源用户，但标记为未验证。
3. 系统基于 Resend 发送邮箱验证码。
4. 验证码为随机 6 位字母 + 数字组合。
5. 用户提交正确验证码后，邮箱状态变为已验证。
6. 邮箱验证后，用户可以登录进入 Browser workbench。
7. 登录后用户看到自己当前身份。
8. 登录流程不要求用户输入 device id。

### Scenario 2: 邮箱用户登录

已有邮箱用户打开登录页。

期望：

1. 用户输入 email 与 password。
2. 邮箱已验证且凭据正确时建立 Browser session。
3. 邮箱未验证时不能登录，并提示需要完成邮箱验证。
4. 凭据错误时显示明确错误，不泄露账号枚举细节。
5. 登录成功后进入可访问的产品入口。

### Scenario 3: 邮箱用户修改密码

来源为邮箱的已登录用户希望修改密码。

期望：

1. 用户在账号设置中发起修改密码。
2. 用户需要提供当前密码。
3. 用户输入新密码并确认。
4. 当前密码正确且新密码满足规则时，密码被更新。
5. 后续登录使用新密码。
6. 修改密码不影响 Google OAuth 用户，因为 Google 来源用户没有本地密码。

### Scenario 4: 邮箱用户忘记密码 / password reset

来源为邮箱的用户忘记密码。

期望：

1. 用户在登录页发起忘记密码。
2. 用户输入邮箱。
3. 系统基于 Resend 发送 password reset 验证码。
4. 验证码为随机 6 位字母 + 数字组合，并具有短期有效期。
5. 用户通过验证码设置新密码。
6. 重置成功后旧密码不能继续用于登录。
7. 为避免账号枚举，提交忘记密码请求时不直接暴露邮箱是否存在。

### Scenario 5: 邮箱验证

邮箱来源用户注册后，系统发送验证邮件。

期望：

1. 用户注册后暂不能登录。
2. 系统通过 Resend 向注册邮箱发送 6 位字母 + 数字验证码。
3. 验证码短期有效。
4. 用户提交正确验证码后，账号邮箱状态变为已验证。
5. 邮箱验证后才可以登录。
6. 如果验证码过期、错误或已使用，系统给出可理解提示。

### Scenario 6: Google OAuth 用户注册 / 登录

用户打开登录入口，选择 Google 登录。

期望：

1. 用户跳转到 Google OAuth 授权流程。
2. Google 授权成功后，系统基于 Google subject 创建或找到用户。
3. 用户进入 Browser workbench。
4. Google 来源用户不需要设置 TermBridge 本地密码。
5. Google 来源用户不展示“修改本地密码”的主路径，或展示为不可用并解释凭据由 Google 管理。
6. Google OAuth 必须真实可用；部署者提供 client id / client secret / redirect URL 等必要配置。

### Scenario 7: 同邮箱来源冲突

用户使用某个邮箱注册了 email 来源账号，之后尝试用同邮箱 Google OAuth 登录；或先存在 Google OAuth 同邮箱用户，再尝试注册 email 来源账号。

期望：

1. 系统不自动合并两个来源的账号。
2. 系统断言该邮箱已经使用，并给出用户可理解提示。
3. 系统不创建新的冲突账号。
4. 系统不因为 Google 返回同邮箱而接管既有 email/password 账号。

### Scenario 8: 登录后进入工作台

用户完成任一来源登录后进入 TermBridge。

期望：

1. 系统知道当前 Browser user。
2. 用户可以进入现有 `/sessions` 工作台或后续 device selector 入口。
3. 当前阶段如果 device authorization 尚未完整实现，可以沿用现有本地 device 路径作为过渡，但文档和实现不得把 device 当作登录凭据。

### Scenario 9: 未登录访问受保护页面

未登录用户访问需要身份的页面。

期望：

1. 用户被引导到登录页。
2. 登录完成后可以进入目标入口或默认工作台。
3. 未登录时不展示授权设备列表。

### Scenario 10: 本地模式与远程 Gate 模式的 PoC 过渡

部署者以本地模式运行 TermBridge 时，仍希望保留快速进入本地工作台的 `admin/admin` shortcut；部署者以远程 Gate 模式运行时，不应允许 `admin/admin` 成为可登录的 Browser 用户入口。

期望：

1. 本地 / 远程 Gate 模式通过显式配置判定。
2. 本地模式仍可使用配置中的 `auth.local_admin.username/password` 作为 bootstrap / development shortcut；该 shortcut 不写入 `users` 表。
3. 远程 Gate 模式不支持 `admin/admin` 登录。
4. 产品和配置语义能清楚区分本地模式与远程 Gate 模式。
5. 该兼容路径不阻碍后续 first-run setup token / local owner 模型替代。

### Scenario 11: 远程 Gate 配置缺失

部署者以远程 Gate 模式启动 TermBridge，但未提供 Google OAuth、Resend 或数据库必要配置。

期望：

1. 远程 Gate 模式缺少 Google OAuth 必要配置时启动失败。
2. 远程 Gate 模式缺少 Resend 必要配置时启动失败。
3. 远程 Gate 模式缺少数据库必要配置或迁移失败时启动失败。
4. 启动失败错误应明确指出缺少哪类配置。
5. 系统不在远程 Gate 模式下静默隐藏登录入口或降级为不完整认证能力。

## Acceptance

### Identity model

- [ ] 系统存在稳定的内部 user id，不以 email 或 username 作为全局唯一主键语义。
- [ ] 用户身份记录能区分来源：email 或 Google OAuth。
- [ ] Google OAuth 用户使用 provider + provider subject 作为外部身份绑定依据。
- [ ] email 可作为登录名，但不被视为跨 provider 的全局唯一身份语义。
- [ ] 普通登录流程不要求用户输入 raw `device_id`。
- [ ] email 来源账号与 Google OAuth 来源账号同邮箱时不自动合并。

### Backend storage / migration

- [ ] 系统引入后端持久化存储，用于 user、identity provider binding、email verification code、password reset code、JWT 相关状态等认证数据。
- [ ] 后端存储同时支持 SQLite 和 MySQL。
- [ ] 系统包含数据库迁移能力。
- [ ] 迁移能力和存储抽象在 Spec 阶段参考 `D:\SourceCodes\mywork\pomelo-orbit\backend-go` 的实现方式。
- [ ] 存储配置缺失、连接失败或迁移失败时，系统有明确错误。

### Email registration / login

- [ ] 用户可以使用 email + password 注册。
- [ ] 注册后系统通过 Resend 发送邮箱验证码。
- [ ] 验证码为随机 6 位字母 + 数字组合。
- [ ] 用户完成邮箱验证后才可以登录使用。
- [ ] 用户可以使用 email + password 登录。
- [ ] password 不以明文形式存储。
- [ ] 登录失败时返回用户可理解但不过度泄露细节的错误。
- [ ] 注册时处理重复 email / 已存在 email 来源用户的情况。
- [ ] 注册时处理已存在 Google OAuth 同邮箱用户的情况，并提示邮箱已使用。
- [ ] 基础密码规则明确并在前后端保持一致。

### Email verification

- [ ] 邮箱注册用户会收到基于 Resend 发送的验证邮件。
- [ ] 邮箱验证码为随机 6 位字母 + 数字组合。
- [ ] 验证码具有有效期。
- [ ] 用户提交正确验证码后，账号邮箱状态变为已验证。
- [ ] 邮箱验证后才可以登录。
- [ ] Resend 配置缺失时，远程 Gate 模式启动失败。

### Password reset / password change for email users

- [ ] 已登录 email 来源用户可以修改密码。
- [ ] 修改密码需要验证当前密码。
- [ ] 新密码需要满足基础密码规则。
- [ ] 修改成功后旧密码不能继续用于登录。
- [ ] 未登录 email 来源用户可以发起忘记密码 / password reset。
- [ ] password reset 邮件通过 Resend 发送。
- [ ] password reset 验证码为随机 6 位字母 + 数字组合。
- [ ] password reset 验证码短期有效，使用后失效。
- [ ] 忘记密码请求不暴露邮箱是否存在。
- [ ] Google OAuth 来源用户不能通过本地改密或 password reset 流程修改 Google 凭据。

### Google OAuth login

- [ ] 用户可以通过真实 Google OAuth 登录。
- [ ] 首次 Google OAuth 登录可创建 Google 来源用户。
- [ ] 后续 Google OAuth 登录可识别同一 Google 用户。
- [ ] Google OAuth 用户不要求设置本地密码。
- [ ] Google OAuth 用户不进入邮箱用户修改密码或 password reset 流程。
- [ ] Google OAuth 返回的 email 如果已被 email 来源账号占用，系统不自动合并，并提示邮箱已使用。
- [ ] Google OAuth 必要配置由部署者提供；远程 Gate 模式缺失配置时启动失败。

### Session / access

- [ ] 登录成功后 Browser 基于现有 JWT 能力建立用户会话。
- [ ] 登出后受保护页面不能继续访问。
- [ ] 未登录访问受保护入口会被引导到登录。
- [ ] 现有 `/sessions` 工作台能在登录后继续使用。
- [ ] 当前阶段的用户身份与后续 device authorization 边界清楚，不把 device credential 混入 Browser login。

### Local vs remote mode

- [ ] 本地 / 远程 Gate 模式通过显式配置判定。
- [ ] 本地模式下，配置中的 `auth.local_admin.username/password` shortcut 可用，且不写入用户表。
- [ ] 远程 Gate 模式不支持 `admin/admin` 登录。
- [ ] 远程 Gate 模式必须使用正式用户系统入口。
- [ ] 本地 `admin/admin` shortcut 被标注为 bootstrap / development shortcut，不作为长期正式身份模型。

### Product UX

- [ ] 登录页清楚区分邮箱登录和 Google 登录。
- [ ] 注册、登录、邮箱验证、改密、忘记密码失败都有可理解反馈。
- [ ] 登录后用户能看到当前登录身份。
- [ ] 第一阶段不要求用户理解 tenant / org / realm。
- [ ] 第一阶段不把用户系统 UI 扩展成完整企业后台。

## Open questions

1. **本地 / 远程 Gate 模式配置字段如何命名？** 已确定通过显式配置判定，不再依赖 URL 猜测；字段命名、默认值和迁移策略进入 Spec。
2. **验证码生命周期的具体时长和重发策略是什么？** 已确定验证码为随机 6 位字母 + 数字组合；有效期、重试次数、频率限制进入 Spec。
3. **本地 `admin/admin` shortcut 细节是什么？** 已在 Spec 中修正为配置 shortcut：仅 local mode 可用，不写入用户表，不参与正式用户身份模型。
4. **公开注册是否允许关闭？** Cloud / SaaS 可能需要邀请制或首个 admin bootstrap；当前需求默认支持注册，具体开关进入 Spec。

## Decisions

当前阶段先记录以下方向性判断：

1. 本需求使用严格模式 / strict，并从 Requirement 开始。
2. 本需求作为新的 feature 文档，不直接修改 `20260623-long-term-identity-device-enrollment.md`。
3. 本轮用户系统优先解决 Browser 用户注册、登录、邮箱用户改密、邮箱验证邮件和 password reset。
4. 用户身份先于 device 选择；device 不是登录凭据。
5. 支持两类登录来源：email 与 Google OAuth。
6. Google OAuth 本轮要求真实可用，必要配置由用户 / 部署者自行填充。
7. 邮箱注册后先验证邮箱，验证后才可以登录。
8. 邮箱验证与 password reset 都通过 Resend 发送随机 6 位字母 + 数字验证码。
9. 忘记密码 / password reset 本轮纳入范围。
10. 只有 email 来源用户支持本地修改密码和 password reset。
11. Google OAuth 来源用户凭据由 Google 管理，TermBridge 不保存其本地密码。
12. email 用户与 Google OAuth 返回同邮箱时不自动合并，应断言并提示邮箱已使用。
13. 远程 Gate 模式缺少 Google OAuth、Resend 或 DB 必要配置时启动失败。
14. 引入后端存储，同时支持 SQLite 和 MySQL，并包含数据迁移能力；Spec 阶段参考 `D:\SourceCodes\mywork\pomelo-orbit\backend-go`。
15. Google OAuth 配置与实现参考 `D:\SourceCodes\mywork\pomelo-orbit\backend-go`。
16. Resend 配置与邮件发送参考 `D:\SourceCodes\mywork\typing-island\backend`。
17. Browser session 复用现有 JWT 能力。
18. 本地 / 远程模式通过显式配置判定，不依赖 URL 猜测。
19. 本地模式仍支持配置中的 `admin/admin` shortcut；远程 Gate 模式不支持。
20. `admin/admin` shortcut 不写入用户表，不作为正式用户身份模型。
21. 第一阶段不暴露完整 tenant / org / RBAC。
22. 第一阶段不实现完整 device enrollment，但实现不能阻碍后续 User -> Device 授权模型。

## Risks

1. **范围膨胀风险**：用户系统容易扩展到 OAuth linking、tenant、RBAC、device enrollment。本轮必须限制在注册、登录、邮箱验证、邮箱改密和 password reset。
2. **安全边界风险**：认证系统涉及密码存储、session、OAuth callback、CSRF、cookie 安全属性、验证码和错误信息，需要在 Spec / Plan 阶段明确。
3. **账号合并风险**：email 登录用户与 Google OAuth 同邮箱用户如果自动合并，可能引入 account takeover 风险；本轮明确不自动合并。
4. **迁移风险**：当前本地 `admin/admin` shortcut 如何与正式用户系统共存，需要避免远程 Gate 暴露默认凭据。
5. **模式配置风险**：本地 / 远程模式如果配置默认值或校验不清，可能导致远程环境错误保留 `admin/admin`。
6. **后端存储引入风险**：同时支持 SQLite、MySQL 和迁移能力会扩大实现面，需要避免把用户系统与 workspace/session 存储重构混成一个不可控变更。
7. **本地体验复杂化风险**：如果第一阶段过早引入 Cloud / tenant 概念，会损害本地开发者体验。
8. **Google OAuth 配置风险**：缺少 client id/secret 时，本地开发、测试与部署体验需要明确降级方式。
9. **Resend 配置风险**：缺少 API key、发件域名或回调 URL 时，邮箱验证和 password reset 的产品行为需要明确。
10. **验证码安全风险**：6 位字母 + 数字验证码需要有效期、频率限制和错误次数限制，否则容易被暴力尝试。
11. **测试复杂度风险**：Google OAuth 与 Resend 真实外部流程不适合完全依赖在线测试，需要 mock/provider abstraction 支撑自动化验证。

## User review notes

用户原始请求：

> 严格模式  我们引入最基本的用户系统，先实现用户注册与登录（基于邮箱或google oauth），来源为邮箱的支持修改密码

本需求草稿按以下理解展开：

- “基于邮箱或google oauth”理解为本轮支持 email/password 与真实 Google OAuth 两种用户来源。
- Google OAuth 必要配置由用户 / 部署者自行填充，并参考 `D:\SourceCodes\mywork\pomelo-orbit\backend-go`。
- “来源为邮箱的支持修改密码”理解为只有 email/password 用户拥有 TermBridge 本地密码，并支持已登录后修改密码。
- 邮箱注册后需要验证邮箱，验证后才可以登录。
- 邮箱验证和忘记密码 / password reset 都基于 Resend 发送随机 6 位字母 + 数字验证码；Resend 参考 `D:\SourceCodes\mywork\typing-island\backend`。
- email 用户与 Google OAuth 返回同 email 时不自动合并，系统应断言并提示邮箱已使用。
- 远程 Gate 模式缺少 Google OAuth / Resend / DB 配置时启动失败。
- 后端存储需要同时支持 SQLite 和 MySQL，并包含数据迁移能力，Spec 阶段参考 `D:\SourceCodes\mywork\pomelo-orbit\backend-go`。
- Browser session 复用现有 JWT 能力。
- 本地 / 远程模式通过显式配置判定，不依赖 URL 猜测。
- 本地模式仍支持配置中的 `admin/admin` shortcut；远程 Gate 模式不支持。
- `admin/admin` 不写入用户表，不作为正式用户身份模型。
- 本轮不同时实现完整 device enrollment、tenant、RBAC 和 OAuth account linking。
