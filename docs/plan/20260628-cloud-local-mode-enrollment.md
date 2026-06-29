# Cloud / Local 模式分离与本机设备云端绑定计划
最后修改时间: 2026-06-28 20:11:20

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；计划 / Plan 已接受，当前进入实现 / Implementation。

## Requirement / Spec basis

- Requirement: `docs/requirement/20260628-cloud-local-mode-enrollment.md`
- Requirement status: Accepted
- Spec: `docs/spec/20260628-cloud-local-mode-enrollment.md`
- Spec status: Accepted

本 Plan 基于已接受的需求和规格，目标是把 TermBridge 从旧的 setup token / local_access 本地主路径，迁移到显式 `web.mode: local|cloud` 模型：

- local mode：本机 dashboard / sessions 不要求 TermBridge Browser 账号认证；通过 `/cloud/connect/start` 发起云端 OAuth2，并在 callback 后上报当前设备。
- cloud mode：云端账号系统、设备列表和远程 workbench 入口。
- 前端和后端都按 local/cloud mode 组织逻辑。
- 同一套镜像 / 二进制通过环境变量 `TERMBRIDGE_WEB__MODE=local|cloud` 决定运行模式。
- 旧 setup token / local_access 主路径直接删除，不做向后兼容；删除产生的问题在实现阶段逐项修复。

## Implementation strategy

采用分层改造，先建立 mode 基础，再删除旧路径，最后接入 local web OAuth2 流程：

1. **配置层先落地 `web.mode`**，避免继续依赖 `agent.connect_url == agent.listen_url` 推导 web auth mode。
2. **后端 capabilities 与 auth middleware mode-aware**，让 local mode 能进入 dashboard / sessions，cloud mode 继续要求账号认证。
3. **前端 router/store/dashboard mode-aware**，让 UI 按 capabilities.mode 组织逻辑。
4. **删除 setup token / local_access 主路径**，包括前端路由、后端接口和相关测试引用。
5. **实现 local `/cloud/connect/start` 与 callback**，使用 OAuth2 库生成 authorize URL、exchange code，并上报设备。
6. **调整云端设备 API**，支持基于已认证 cloud account 的当前设备 upsert / report。
7. **验证 local / cloud 两类运行模式**，确保同一镜像可通过环境变量切换。

## Implementation steps

### Step 1：配置模型新增 `web.mode`

涉及文件：

- `internal/infrastructure/config/config.go`
- `internal/infrastructure/config/config_test.go`
- `.termbridge.default.yaml`
- `.env.example`
- 可能涉及 `.air.toml` / docs 示例配置

任务：

1. 在 Config 结构中新增：

   ```go
   Web struct {
       StaticDir string
       Mode string
   }
   ```

   若当前已有 `web.static_dir`，在同一结构内加入 `mode`。

2. 默认值设为：

   ```yaml
   web:
     mode: local
   ```

3. 支持环境变量：

   ```text
   TERMBRIDGE_WEB__MODE=local|cloud
   ```

4. 校验规则：
   - 空值归一化为 `local`。
   - 只允许 `local` / `cloud`。
   - 非法值返回 config error。

5. 保留 `agent.connect_url` 作为 tunnel target，不再作为 web mode 判断依据。

6. 将现有 `config.Mode(cfg)` / `config.IsLocalMode(cfg)` 的语义拆分：
   - 如仍需 tunnel/self-connect 判断，改名为类似 `IsSelfConnectedAgent` 或只在 Agent 连接逻辑使用。
   - 新增或调整 `config.WebMode(cfg)` / `config.IsCloudMode(cfg)` / `config.IsLocalWebMode(cfg)`。

验收：

- config test 覆盖默认 local。
- config test 覆盖 env 设置 cloud。
- config test 覆盖非法 mode 报错。
- `agent.connect_url=http://termbridge.lvh.me` 不会导致 web mode 变为 cloud，除非显式 `web.mode=cloud`。

### Step 2：后端 capabilities mode-aware

涉及文件：

- `internal/application/auth/service.go`
- `internal/application/auth/*_test.go`
- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/server_test.go`

任务：

1. `Capabilities` 返回值必须表达 `mode=local|cloud`。
2. local mode：
   - `account_auth_enabled=false` 或等价字段。
   - 不暴露 email / google / register / password reset 为本地主路径能力。
   - 暴露 cloud connect 是否可用，例如 `cloud_connect_enabled=true/false`，取决于 `cloud.gate_url` 是否配置。
3. cloud mode：
   - 保留 email / google providers。
   - register / password reset 按现有 auth 配置启用。
   - 不暴露 local connect start 入口。
4. 如果暂不新增 features 对象，也至少保证前端能通过 `mode` 和 providers 判断。

验收：

- local mode capabilities 不让前端误判为未登录而跳 `/login`。
- cloud mode capabilities 保留当前账号登录能力。

### Step 3：auth middleware / API guard 按 mode 分支

涉及文件：

- `internal/transport/http/gatewayapi/server.go`
- 相关 middleware / auth helper 文件
- `internal/transport/http/gatewayapi/*_test.go`

任务：

1. 定义 local mode 放行范围：
   - 本机 dashboard 所需 API。
   - 本机 `/sessions` / workspace / terminal 所需 API。
   - capabilities。
   - `/cloud/connect/start`。
   - `/cloud/connect/callback`。
2. cloud mode 继续要求 Browser cloud auth：
   - cloud dashboard。
   - `/api/devices`。
   - `/sessions` 相关远程设备操作。
3. auth middleware 必须识别 `web.mode=local`：
   - local mode 不要求 Browser JWT。
   - 不把 `authenticated=false` 当成全局不可访问。
4. 明确 local mode 的安全边界依赖 loopback / 配置 / OS 用户；本轮不新增本地账号。

验收：

- local mode 无 token 访问 dashboard / sessions API 不返回 401。
- cloud mode 无 token 访问受保护 API 仍返回 401。
- local-only API 在 cloud mode 返回 404 或 400。

### Step 4：删除 setup token / local_access 主路径

涉及文件：

- `internal/app/app.go`
- `internal/application/auth/setup.go`
- `internal/application/auth/service.go`
- `internal/transport/http/gatewayapi/server.go`
- `web/src/views/SetupView.vue`
- `web/src/router/index.ts`
- `web/src/store/gateway.ts`
- `web/src/features/gateway/api.ts`
- 相关 tests / i18n / docs 引用

任务：

1. 删除 `termbridge serve` 启动时创建 setup token / 打印 setup URL 的正式路径。
2. 删除或停用 `/api/auth/setup/status`、`/api/auth/setup/complete`。
3. 删除 `/setup` route 和 `SetupView.vue`，或至少从正式路由中移除。
4. 删除 `local_access` provider 主路径和相关 UI 文案。
5. 删除相关 tests；若有业务行为被 setup 测试覆盖，改写为 local mode 无认证 / cloud mode 认证测试。
6. 不提供 backward compatibility。

验收：

- 代码中不再有本地主路径依赖 setup token。
- 打开 local `/` 不会跳 `/setup`。
- 打开 local `/dashboard` 不要求 local_access。
- 删除暴露的编译 / 测试错误必须按新模型修复。

### Step 5：前端 router / store 按 mode 组织

涉及文件：

- `web/src/router/index.ts`
- `web/src/store/gateway.ts`
- `web/src/features/gateway/api.ts`
- `web/src/views/HomeView.vue`
- `web/src/views/LoginView.vue`
- `web/src/views/DashboardView.vue`
- `web/src/views/SessionsView.vue`
- `web/src/i18n.ts`
- 相关 tests

任务：

1. `gateway.initializeAuth()` 或等价流程先加载 capabilities / mode。
2. router guard：
   - local mode：允许 `/dashboard`、`/sessions`。
   - cloud mode：受保护页面要求 cloud auth。
3. `/`：
   - local mode -> `/dashboard`。
   - cloud mode：已登录 -> `/dashboard`；未登录 -> `/login`。
4. `/login`：
   - cloud mode 正常显示登录 / 注册 / Google OAuth。
   - local mode 不作为默认入口；直接访问时提示“账号登录在 Cloud Gate 完成”并提供返回 dashboard 或连接云端入口。
5. `useGatewayStore` 区分：
   - cloud user auth 状态。
   - local mode cloud connection 状态。
   - devices / selected device。
6. local mode 的 “未连接云端” 不能等同于整个 app `authenticated=false`。

验收：

- local mode 无 token 打开 `/dashboard` 正常显示。
- local mode 无 token 打开 `/sessions` 不被 guard 送到 `/login`。
- cloud mode 无 token 打开 `/dashboard` 进入 `/login`。
- cloud mode 登录后进入 `/dashboard`。

### Step 6：DashboardView 按 mode 渲染

涉及文件：

- `web/src/views/DashboardView.vue`
- 可能拆分子组件但保留一个 route：
  - `LocalDashboardSection.vue`
  - `CloudDashboardSection.vue`
- `web/src/i18n.ts`

任务：

1. 一个 `/dashboard` route，一个 `DashboardView`。
2. mode=local：
   - 展示本机设备名称 / 状态。
   - 展示本机 workspace / sessions 入口。
   - 展示 cloud gate 配置状态。
   - 展示“连接云端账号”按钮。
   - 展示 cloud connection 成功 / 失败状态。
3. mode=cloud：
   - 展示设备列表。
   - 无设备时展示空状态。
   - online device 可进入 `/sessions`。
   - offline device 不可连接。
4. 右上角账号 UI：
   - local mode：cloud connection UI。
   - cloud mode：cloud user account UI。

验收：

- 单个 DashboardView 中 local/cloud 文案不混淆。
- cloud 空状态不再要求用户寻找本地 setup token。
- local dashboard 提供 `/cloud/connect/start` 的入口。

### Step 7：实现 local `/cloud/connect/start`

OAuth2 库选型：

```text
golang.org/x/oauth2 v0.36.0
```

当前项目已经在 `go.mod` 中通过现有 Google OAuth 间接引入该库，并在 `internal/application/auth/service.go` 中使用 `oauth2.Config` / `google.Endpoint` / `AuthCodeURL` / `Exchange`。本轮继续使用该库，不新增其他 OAuth2 客户端依赖；如果新增代码直接 import，应将其提升为 direct dependency。

涉及文件：

- 新增或修改 `internal/application/cloudconnect/*`
- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/server_test.go`
- config / state dir helper

任务：

1. 仅 local mode 可用。
2. 校验 `cloud.gate_url`。
3. 生成 state nonce。
4. 将 state 保存到 `.termbridge` state dir。
5. state 绑定：
   - cloud.gate_url
   - callback URL
   - post-auth redirect
   - device id / device name
   - expires_at
6. 使用 OAuth2 client config 生成标准 authorize URL。
7. 返回 302 到 authorize URL。
8. cloud mode 访问返回 404 或 400。

验收：

- local mode 调用 `/cloud/connect/start` 返回 302。
- cloud gate 未配置返回明确错误。
- cloud mode 调用该路径不执行 local connect 流程。
- state 文件写入 `.termbridge`。

### Step 8：实现 local `/cloud/connect/callback`

涉及文件：

- `internal/application/cloudconnect/*`
- `internal/transport/http/gatewayapi/server.go`
- tests

任务：

1. 读取 `code` 和 `state`。
2. 校验 state 存在、未过期、未使用。
3. 使用 OAuth2 client config exchange code。
4. 使用 cloud account authentication context 调用 cloud device report endpoint。
5. 保存 cloud connection / bound device metadata 到 `.termbridge`。
6. 标记 state used 或清理 state。
7. 成功后 302 回 `/dashboard?cloud_connected=1`。
8. 失败后 302 回 `/dashboard?cloud_connect_error=<reason>`。

验收：

- state 缺失 / 过期 / mismatch 均失败。
- exchange 失败有可理解错误。
- 设备上报失败有可理解错误。
- 成功后本地 dashboard 能显示已连接云端状态。

### Step 9：cloud 设备上报 API

涉及文件：

- `internal/transport/http/gatewayapi/server.go`
- 设备 repository / service 相关文件
- tests

任务：

1. 确认现有设备绑定 / upsert API 是否可复用。
2. 若无合适接口，新增类似：

   ```http
   POST /api/devices/current
   Authorization: Bearer <cloud account auth>
   ```

3. 请求体包含：
   - `device_id`
   - `device_name`
   - 可选 agent metadata
4. cloud backend 从认证上下文取得 user id。
5. upsert user-device binding。
6. 返回 bound device summary。

验收：

- 未认证 cloud 请求返回 401。
- 已认证请求能绑定 / 更新当前设备。
- `/api/devices` 能看到该设备。

### Step 10：Agent remote 401 不阻断 local dashboard

涉及文件：

- `internal/app/app.go`
- agent client 启动 / error handling 相关文件
- tests

任务：

1. local mode 下，如果 agent remote connect 因未绑定 / 401 失败，不应直接退出整个 `serve`。
2. 本地 dashboard 必须继续可用，用户才能完成 cloud connect。
3. 记录 warning / pending 状态。
4. 后续是否自动重试可先简单处理；至少不能杀死 local web。

验收：

- `agent.connect_url=http://termbridge.lvh.me` 且尚未绑定时，`termbridge serve` 仍保持 local web 可用。
- 用户能打开 `/dashboard` 发起 OAuth2。

### Step 11：环境变量驱动同一镜像部署

涉及文件：

- `.env.example`
- Docker / deployment 相关配置，如存在
- README / docs 示例，如必要

任务：

1. 文档化 local 默认：

   ```text
   TERMBRIDGE_WEB__MODE=local
   ```

2. 文档化 cloud 部署：

   ```text
   TERMBRIDGE_WEB__MODE=cloud
   TERMBRIDGE_CLOUD__GATE_URL=http://termbridge.lvh.me
   ```

3. 确认构建产物不区分 local/cloud 镜像；运行时 env 决定。

验收：

- 同一 build 产物可用不同 env 得到 local / cloud 行为。
- 示例配置不再暗示通过 `agent.connect_url` 判断 web mode。

## Files expected to change

预计变更范围：

- `.termbridge.default.yaml`
- `.env.example`
- `internal/infrastructure/config/config.go`
- `internal/infrastructure/config/config_test.go`
- `internal/app/app.go`
- `internal/application/auth/service.go`
- `internal/application/auth/setup.go`（删除或大幅移除）
- `internal/application/agent/*`（如设备上报 / client 状态需要调整）
- `internal/application/cloudconnect/*`（新增）
- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/server_test.go`
- `web/src/features/gateway/api.ts`
- `web/src/store/gateway.ts`
- `web/src/router/index.ts`
- `web/src/views/HomeView.vue`
- `web/src/views/LoginView.vue`
- `web/src/views/DashboardView.vue`
- `web/src/views/SetupView.vue`（删除）
- `web/src/views/SessionsView.vue`
- `web/src/i18n.ts`
- 相关 frontend tests
- 相关 backend tests

实际实现时如果发现现有代码结构不同，以最小必要变更为准，但不得偏离 accepted Spec。

## Verification plan

### 后端测试

建议运行：

```powershell
go test ./internal/infrastructure/config ./internal/application/auth ./internal/transport/http/gatewayapi ./internal/app
```

必要时扩大到：

```powershell
go test ./internal/...
```

重点断言：

- `web.mode` 默认和 env override。
- local mode capabilities。
- cloud mode capabilities。
- local mode middleware 放行。
- cloud mode middleware 仍要求 auth。
- setup path 删除后无残留测试失败。
- `/cloud/connect/start` state / redirect 行为。
- callback state 校验与失败路径。
- cloud device report authenticated behavior。

### 前端测试

建议运行：

```powershell
yarn --cwd web test --run
yarn --cwd web typecheck
yarn --cwd web lint
yarn --cwd web format:check
```

重点断言：

- local mode router 不跳 login。
- cloud mode router 未登录跳 login。
- DashboardView local/cloud 分支。
- setup route 删除后无 dead import。
- gateway store 区分 mode、cloud auth 和 cloud connection。

### 格式 / diff 检查

```powershell
git diff --check
```

### 手工验证

Local mode：

1. 启动本机：`TERMBRIDGE_WEB__MODE=local`。
2. 打开 `http://localhost:9031`。
3. 应进入 `/dashboard`。
4. 不应进入 `/login` 或 `/setup`。
5. 打开 `/sessions` 不应因为无 Browser token 被踢到登录页。
6. 点击“连接云端账号”。
7. 应跳转到 cloud OAuth2 authorize URL。
8. callback 成功后返回 local dashboard。
9. local dashboard 显示已连接 / 或明确失败原因。

Cloud mode：

1. 启动云端：`TERMBRIDGE_WEB__MODE=cloud`。
2. 打开 cloud gate。
3. 未登录访问 `/dashboard` 应进入 `/login`。
4. 登录后进入 cloud dashboard。
5. 无设备时显示空状态。
6. local 完成设备上报后，cloud dashboard 能看到该设备。
7. cloud mode 访问 `/cloud/connect/start` 返回 404/400，不执行 local connect。

## Rollback / recovery

如果实现中发现 OAuth2 完整 callback / device report 一次性落地风险过大，可按以下顺序拆分，但不得回到 setup token 主路径：

1. 先完成 `web.mode`、auth middleware、router、DashboardView mode 分支。
2. 再删除 setup token / local_access 主路径。
3. 再落地 `/cloud/connect/start` 和 callback。
4. 最后接设备上报。

回滚原则：

- 不恢复 setup token 作为正式本地入口。
- 不恢复 `agent.connect_url` 推导 web mode。
- 不把 local mode 重新绑定到 Browser auth。

## Risks

1. **变更面较大**：涉及 config、auth middleware、frontend router、dashboard、setup 删除和 OAuth2 callback，容易出现连锁编译 / 测试失败。
2. **旧测试大量失效**：setup token / local_access 删除后，旧测试需要按新模型重写，不应机械跳过。
3. **OAuth2 配置依赖外部环境**：redirect URI、client id、client secret、cloud.gate_url 不匹配时，手工验证会失败，需要清晰错误。
4. **local mode 无认证边界**：必须避免在 cloud mode 或非预期暴露场景误放行。
5. **单 DashboardView 复杂度**：需要通过局部组件 / computed 降低条件分支复杂度。
6. **Agent 401 体验**：未绑定设备连接 cloud gate 401 时不能杀死本机 dashboard。
7. **同一镜像运行时切换**：构建期和运行期配置不要混用，否则 cloud/local 行为会不可预测。

## Open items for Implementation

- 需要先阅读当前 auth middleware 和 route guard 具体实现，再决定最小改法。
- 需要确认现有 OAuth2 client 封装是否只支持 Google OAuth，还是能复用为 local web -> cloud gate OAuth2。
- 需要确认现有 device repository / API 是否已有 upsert 当前设备的能力。
- 需要确认 `.termbridge` state dir 当前结构和权限处理。
- 需要确认前端现有 tests 覆盖 router 的方式，避免引入过重测试基础。

## User review notes

用户在 Spec review 中确认：

> 实现上，auth middleware 需要能识别 local mode
>
> web 和 后端均基于 local/cloud 模式组织逻辑，基于环境变量组织镜像构建

随后用户要求：

> 进入 Plan

用户已要求进入实现 / Implementation，本 Plan 标记为 Accepted。
