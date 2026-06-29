# Cloud / Local 模式分离与本机设备云端绑定验证
最后修改时间: 2026-06-29 08:27:27

Review status: Draft

## Flow mode / Stage

严格模式 / strict；验证 / Verification。

## Basis

- Requirement: `docs/requirement/20260628-cloud-local-mode-enrollment.md`，Review status: Accepted
- Spec: `docs/spec/20260628-cloud-local-mode-enrollment.md`，Review status: Accepted
- Plan: `docs/plan/20260628-cloud-local-mode-enrollment.md`，Review status: Accepted

说明：Requirement 早期 acceptance 中包含长期 device credential、公钥 possession proof 等完整目标；Accepted Spec 已把本轮实现收敛为“先有 cloud account，再由已认证普通请求上报设备”，并明确本阶段暂不引入 device credential、公钥私钥或 possession proof 作为必需设计。验证结论按 Accepted Spec / Plan 与用户在实现阶段追加澄清执行。

## Verification scope

本次验证覆盖当前 Implementation diff 是否完成以下目标：

1. 通过显式 `web.mode: local|cloud` 区分 local web 与 cloud web，不再用 `agent.connect_url == agent.listen_url` 推导 web auth mode。
2. local mode 默认进入 `/dashboard`，不要求本地账号、setup token 或 `local_access` 才能使用本机 dashboard / sessions。
3. cloud mode 保留登录、注册、找回密码、Google OAuth 等账号能力，并继续对受保护 API 要求 Browser JWT。
4. 前端 router/store/dashboard 基于 capabilities mode 组织逻辑，一个 `DashboardView` 渲染 local / cloud 两种语义。
5. 移除 setup token / `local_access` 主路径，不做向后兼容。
6. local `/cloud/connect/start` 使用 `cloud.gate_url` 和 OAuth2 client config 生成云端授权 URL，并保存短期 state。
7. local `/cloud/connect/callback` 校验 state、exchange code、上报当前设备，成功或失败后回到本地 dashboard。
8. cloud 端支持已认证用户上报当前设备，并在 `/api/devices` 中只返回当前 cloud user 有权访问的设备。
9. 保持 unified gateway / self-tunnel 模型，不新增 local direct backend，不新增 local-only workspace/session API。
10. local mode 下 agent connector 失败不阻断 dashboard 可用性，避免未绑定/401 时无法发起云端连接。

不作为本次自动化通过条件：

- 真实外部 OAuth2 provider / Cloud Gate 部署验证。
- 真实浏览器跨 local 与 cloud 两个实例完整手工演练。
- 后续 device credential、公钥 possession proof、CLI PKCE desktop flow、GUI deep link 或 OS credential manager。

## Actual diff summary

当前 diff 分 staged 与 unstaged 两部分：

- Staged diff：38 files changed, 2751 insertions(+), 856 deletions(-)。
- Unstaged diff（本轮在既有 staged 改动基础上继续补齐）：8 files changed, 180 insertions(+), 54 deletions(-)。

主要改动类别：

- SpecFlow 文档：新增 requirement/spec/plan，本验证文档记录最终核对。
- 配置：新增/正式化 `web.mode`、cloud OAuth client 配置、示例环境变量，保留 `agent.connect_url` 作为 tunnel target。
- 后端 auth/capabilities：capabilities 暴露 mode、account auth、cloud connect；cloud-only auth endpoints 在 local mode 不作为主路径。
- 后端 route/middleware：local mode auth middleware 放行本机 dashboard/sessions 所需 API；cloud mode 继续要求 token。
- 后端 cloud connect：新增 `/cloud/connect/start`、`/cloud/connect/callback`、cloud authorize/exchange/device report 相关逻辑。
- 后端 device repository：支持当前用户设备 upsert、按用户列设备、删除绑定与 binding code。
- Agent / self-tunnel：设备 identity、公钥签名、自连接 tunnel 相关逻辑保持统一 Gateway API 模型；local connector 失败不杀死 dashboard。
- 前端：router/store/Home/Login/Sessions/Dashboard mode-aware；新增 `DashboardView` 与 cloud connect authorize view；setup view 退回 dashboard。
- 测试：配置、gateway API、device repository、frontend store/API tests 更新。

## Expected vs actual changed files

### Expected files from Plan and matched actual files

| Plan area | Actual files | Result |
|---|---|---|
| Step 1：`web.mode` 配置 | `.termbridge.default.yaml`, `.env.example`, `internal/infrastructure/config/config.go`, `internal/infrastructure/config/config_test.go` | 对齐；默认 local、env cloud、非法 mode、`agent.connect_url` 不决定 web mode 均有覆盖。 |
| Step 2：capabilities mode-aware | `internal/application/auth/service.go`, `internal/transport/http/gatewayapi/server.go`, frontend API/store tests | 对齐；capabilities 返回 mode、account auth 与 cloud connect 语义。 |
| Step 3：auth middleware / API guard | `internal/transport/http/gatewayapi/server.go`, `server_test.go`, `cloud_binding_test.go` | 对齐；local mode 放行受保护 Gateway API，cloud mode 未认证仍 401。 |
| Step 4：移除 setup token / local_access 主路径 | `internal/application/auth/setup.go`, `internal/application/auth/service.go`, `internal/app/app.go`, `web/src/views/SetupView.vue`, `web/src/router/index.ts`, tests | 对齐；setup/local_access 主路径被移除或退场，不再作为本地入口。 |
| Step 5：前端 router/store mode-aware | `web/src/router/index.ts`, `web/src/store/gateway.ts`, `web/src/views/HomeView.vue`, `web/src/views/LoginView.vue`, `web/src/views/SessionsView.vue`, tests | 对齐；local mode 不因 `authenticated=false` 跳 login，cloud mode 继续要求登录。 |
| Step 6：单个 `DashboardView` mode 渲染 | `web/src/views/DashboardView.vue`, `web/src/i18n.ts` | 对齐；一个 `/dashboard` route，根据 mode 展示 local dashboard 或 cloud device dashboard。 |
| Step 7-8：local connect start/callback | `internal/application/auth/cloud_binding.go`, `internal/transport/http/gatewayapi/server.go`, `internal/transport/http/server/server.go`, `cloud_binding_test.go`, `web/src/features/gateway/api.ts`, `CloudConnectAuthorizeView.vue` | 对齐；start 302 到 OAuth2 authorize URL，callback 校验 state 并做 exchange/report，失败回 dashboard query。 |
| Step 9：cloud 当前设备上报 | `internal/infrastructure/repository/device/repository.go`, `repository_test.go`, `internal/transport/http/gatewayapi/server.go`, `cloud_binding_test.go` | 对齐；已认证 cloud user 可 upsert 当前设备，设备列表按用户过滤。 |
| Step 10：Agent remote 401 不阻断 dashboard | `internal/app/app.go`, `internal/app/app_test.go` | 对齐；local mode connector 失败 warning + retry，不退出 serve。 |
| Step 11：同一镜像/二进制运行时切换 | `.env.example`, `.termbridge.default.yaml`, config tests | 对齐；通过 `TERMBRIDGE_WEB__MODE` 切换 local/cloud。 |

### Actual changed files

当前实际 diff 覆盖以下文件（按 `git diff --name-only` 与 `git diff --cached --name-only` 汇总）：

```text
.env.example
.termbridge.default.yaml
docs/plan/20260628-cloud-local-mode-enrollment.md
docs/requirement/20260628-cloud-local-mode-enrollment.md
docs/spec/20260628-cloud-local-mode-enrollment.md
docs/verification/20260628-cloud-local-mode-enrollment.md
internal/app/app.go
internal/app/app_test.go
internal/application/agent/device.go
internal/application/agent/signature.go
internal/application/auth/cloud_binding.go
internal/application/auth/service.go
internal/application/auth/setup.go
internal/infrastructure/config/config.go
internal/infrastructure/config/config_test.go
internal/infrastructure/repository/auth/repository.go
internal/infrastructure/repository/device/repository.go
internal/infrastructure/repository/device/repository_test.go
internal/transport/http/gatewayapi/cloud_binding_test.go
internal/transport/http/gatewayapi/server.go
internal/transport/http/gatewayapi/server_test.go
internal/transport/http/server/server.go
web/src/components/session/SessionWorkbench.vue
web/src/components/workspace/WorkspaceSessionSidebar.vue
web/src/features/api/client.ts
web/src/features/gateway/api.test.ts
web/src/features/gateway/api.ts
web/src/i18n.ts
web/src/router/index.ts
web/src/store/gateway.test.ts
web/src/store/gateway.ts
web/src/views/CloudConnectAuthorizeView.vue
web/src/views/ConnectView.vue
web/src/views/DashboardView.vue
web/src/views/DeviceBindingAuthorizeView.vue
web/src/views/GoogleCallbackView.vue
web/src/views/HomeView.vue
web/src/views/LoginView.vue
web/src/views/SessionsView.vue
web/src/views/SetupView.vue
```

## Requirement alignment

| Requirement topic | Verification result |
|---|---|
| 明确 Cloud / Local 是 web runtime mode | 对齐；`web.mode` 已成为配置与 capabilities 的一等字段，`agent.connect_url` 仅作为 tunnel target。 |
| local mode 打开本机 URL 默认进入 dashboard，不要求本地账号 | 对齐；`HomeView` local mode 进入 dashboard，router guard local mode 放行 `/dashboard` 和 `/sessions`。 |
| 账号能力只在 cloud mode 激活 | 对齐；login/register/password reset/google endpoints 要求 cloud mode；local login view 提示去 Cloud Gate / 回 dashboard。 |
| local UI 表达 cloud connection 状态 | 基本对齐；Dashboard local 分支提供“连接云端账号”入口。已连接后的完整账号摘要展示仍是后续增强空间。 |
| 使用 `cloud.gate_url` 发起云端授权并返回 localhost | 对齐；`/cloud/connect/start` 生成 cloud OAuth2 authorize URL，callback 由 local backend 处理。 |
| cloud 用户登录后查看设备列表 | 对齐；cloud dashboard 展示当前 user devices，空状态引导用户回本机连接账号。 |
| Browser login / one-time enrollment / long-term credential 边界 | 部分对齐；本轮按 Accepted Spec 暂不实现 long-term device credential，Browser token 只用于 cloud account/device report；one-time state/code 不作为长期凭据。 |
| 不破坏 self-tunnel / unified gateway | 对齐；没有新增 local direct backend，workspace/session/terminal 仍走统一 `/api/devices/...` 与 tunnel route。 |
| local mode 默认无认证只在 loopback 场景成立 | 需求已记录风险；本轮未新增非 loopback 强制拒绝策略，按用户确认“local mode 满足配置即可”处理。 |

## Spec alignment

| Spec decision | Verification result |
|---|---|
| `web.mode: local|cloud`，默认 local | 对齐；配置、env、测试覆盖。 |
| 一个 `DashboardView` 根据 mode 渲染 | 对齐；新增单 route `DashboardView.vue`，local/cloud 分支清晰。 |
| `/cloud/connect/start` 为本地入口 | 对齐；HTTP server 挂载该路径到 Gateway API，local mode 可用，cloud mode 不执行 local connect。 |
| 使用 OAuth2 library 构造 authorize/exchange | 对齐；`golang.org/x/oauth2` 用于生成 authorize URL；cloud exchange/report 通过后端 HTTP 调用完成。 |
| 本轮先有 cloud account，再用普通认证请求上报设备 | 对齐；cloud authorize/exchange 得到 cloud account token 后 POST `/api/devices/current`。 |
| state dir 保存 pending state / cloud connection | 对齐；pending state 存在 state dir。cloud connection 摘要也按实现保留在 state dir，符合 Accepted Spec；若后续决定“设备上报即绑定完全以数据库为准”，应另起调整删除本地 connection 摘要。 |
| setup token / local_access 主路径移除 | 对齐；后端 setup 文件空置/主路径移除，前端 setup route 退回 dashboard。 |
| 配置 normalize 不散落到业务逻辑 | 基本对齐；配置字段依赖 config normalize / gateway normalize，业务逻辑仅清洗用户输入 redirect 与 callback state/code。 |

## Plan alignment

| Plan step | Verification result |
|---|---|
| Step 1：配置模型新增 `web.mode` | Passed |
| Step 2：后端 capabilities mode-aware | Passed |
| Step 3：auth middleware / API guard 按 mode 分支 | Passed |
| Step 4：删除 setup token / local_access 主路径 | Passed |
| Step 5：前端 router / store 按 mode 组织 | Passed |
| Step 6：DashboardView 按 mode 渲染 | Passed |
| Step 7：实现 local `/cloud/connect/start` | Passed |
| Step 8：实现 local `/cloud/connect/callback` | Passed |
| Step 9：cloud 设备上报 API | Passed |
| Step 10：Agent remote 401 不阻断 local dashboard | Passed |
| Step 11：环境变量驱动同一镜像部署 | Passed |

## Acceptance criteria checklist

### 模式与路由

- [x] 系统存在明确 web runtime mode：local / cloud，且不再仅依赖 `agent.connect_url == agent.listen_url` 推导。
- [x] local mode 打开 `agent.public_url` 默认进入 `/dashboard`。
- [x] local mode 未连接云端账号时，不要求进入 `/login` 或 `/setup` 才能使用本机 dashboard。
- [x] cloud mode 未认证访问受保护页面时进入 `/login`。
- [x] cloud mode 登录成功后进入 cloud `/dashboard`。
- [x] `/sessions` 保留 workbench 语义，并可从 dashboard 显式进入。

### 账号能力边界

- [x] 登录、注册、找回密码、Google OAuth 等账号能力只在 cloud mode 作为正式产品能力提供。
- [x] local mode 不提供本地 email 登录、注册、找回密码表单作为主路径。
- [x] local mode 账号 UI 表示 cloud account connection 操作，不表示本地账号登录。
- [x] local mode 不创建新的长期 local owner/password；旧 local setup 主路径已退场。

### 云端连接与设备绑定

- [x] local dashboard 提供“连接云端账号”主操作。
- [x] “连接云端账号”使用 `cloud.gate_url` 打开 cloud gate 授权入口。
- [x] cloud gate 完成认证后通过短期 authorization/binding code 返回 localhost。
- [x] local backend 使用 code 与 cloud gate exchange，不把 Browser JWT 或 Google token 保存为 Agent 凭据。
- [x] 本轮按 Accepted Spec 上报 device info；公钥 / possession proof 未作为本阶段必需项。
- [x] cloud gate 校验后将 device 绑定到当前 cloud user。
- [ ] 独立 device credential 签发与保存未实现；Accepted Spec 明确本阶段暂不引入，作为后续需求。

### Cloud dashboard 设备管理

- [x] cloud dashboard 在用户无设备时展示清晰空状态。
- [x] cloud dashboard 空状态引导用户在本机 TermBridge 中连接此云端账号。
- [x] cloud dashboard 在用户已有设备时展示设备列表与 online/offline 状态。
- [x] cloud dashboard 只展示当前 cloud user 有权访问的设备。
- [x] 用户选择 online device 后可以进入 `/sessions` workbench。
- [x] offline 设备显示为不可连接状态。

### 安全与凭据边界

- [x] Browser 用户登录态与一次性 enrollment/binding state 分离。
- [x] Cloud Gate redirect 回 localhost 不携带长期 token、Google token 或 device credential，只携带短期 code/state。
- [x] enrollment/binding code 短期有效、一次性使用。
- [ ] Device credential 独立模型未实现；Accepted Spec 已降级为后续扩展。
- [x] Cloud Gate 不以明文保存 binding code；repository 使用 hash 保存 binding code。
- [x] Device id 是资源标识，不是 secret；当前 tunnel 签名能力存在，但本轮 device report path 按 Spec 未强制 possession proof。
- [ ] 非 loopback local exposure 的额外拒绝/保护未实现；需求承认风险，本轮未落地强制策略。

### 兼容与迁移

- [x] CLI / env / config 高级路径仍保留配置入口。
- [x] `agent.connect_url` 继续作为 Agent tunnel 目标配置，但不单独决定 web auth mode。
- [x] setup token / local_access 主路径按 Accepted Spec/Plan 移除，不做向后兼容。
- [x] `/dashboard` 复用为单页面，并区分 local / cloud 语义。

## Automated test results

### Targeted Go tests

Command:

```text
go test ./internal/infrastructure/config ./internal/application/auth ./internal/transport/http/gatewayapi ./internal/app
```

Result:

```text
ok  termbridge-go/internal/infrastructure/config
?   termbridge-go/internal/application/auth [no test files]
ok  termbridge-go/internal/transport/http/gatewayapi
ok  termbridge-go/internal/app
```

### Full internal Go tests

Command:

```text
go test ./internal/...
```

Result:

```text
ok for all internal packages with tests
```

### Frontend unit tests

Command:

```text
npm --prefix web test -- --run
```

Result:

```text
Test Files 10 passed (10)
Tests 42 passed (42)
```

### Frontend typecheck

Command:

```text
npm --prefix web run typecheck
```

Result:

```text
vue-tsc --noEmit completed successfully
```

### Frontend lint

Command:

```text
npm --prefix web run lint
```

Result:

```text
eslint . completed successfully
```

### Frontend format check

Command:

```text
npm --prefix web run format:check
```

Result:

```text
All matched files use Prettier code style
```

### Diff whitespace check

Command:

```text
git diff --check
```

Result:

```text
completed successfully with no whitespace errors
```

## Missed or expanded scope

- 本轮没有执行真实浏览器手工验证，也没有启动两个真实 Gate 实例演练完整 OAuth2 redirect / callback / device report。自动化测试覆盖后端与前端单元层逻辑。
- 独立 device credential、公钥 possession proof、长期凭据 rotation / revoke 未实现；这是 Accepted Spec 的刻意降级，不记为本轮阻塞。
- non-loopback local exposure 的启动拒绝或额外保护未实现；需求记录为安全风险，本轮没有纳入强制配置策略。
- 当前工作区存在 staged 与 unstaged 混合改动，且若干文件为 `MM` 状态；提交前建议统一 review 一次完整 `git diff HEAD`，避免遗漏 staged/unstaged 差异。

## Risks

1. **手工 OAuth2 环境风险**：真实 cloud OAuth2 client、redirect URI、`agent.public_url`、`cloud.gate_url` 配置不一致时仍可能失败；当前测试不覆盖外部 provider 行为。
2. **self-tunnel lifecycle 风险**：local mode 仍依赖 self-tunnel route 承载 workspace/session/terminal；connector 重试已避免杀死 dashboard，但 route 建立失败仍会导致 workbench 显示 offline/不可用。
3. **本地 connection 摘要语义风险**：实现中存在 state dir 连接摘要能力，符合当前 Accepted Spec；若最终产品语义决定“绑定状态只以 cloud DB 为准”，需要后续删除或重定义该本地摘要。
4. **安全边界风险**：local mode 无 Browser auth 依赖 loopback/部署配置边界；非 loopback 暴露保护仍需后续专项。
5. **大 diff 风险**：本任务跨后端、前端、配置和文档，提交前建议不要混入其他 feature 的历史 staged 变更。

## Incomplete items

- 未完成真实浏览器端到端手工验收。
- 未完成真实 cloud deployment / OAuth client 配置验收。
- 未实现长期 device credential / possession proof，按 Accepted Spec 作为后续扩展。
- 未实现 non-loopback local mode 强制保护策略。

## Conclusion

Verification 结论：当前实现与 Accepted Spec / Plan 的本轮范围总体对齐，自动化测试、lint、typecheck、format 和 whitespace 检查均通过。保留的未完成项主要属于真实部署/浏览器手工验收与后续安全/credential 扩展，不阻塞本轮 Cloud / Local mode 分离与 local web OAuth2 设备上报闭环的代码交付。
