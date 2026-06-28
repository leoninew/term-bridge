# 云端 Gate 设备绑定与本地 Agent 接入验证
最后修改时间: 2026-06-28 14:16:29

Review status: Draft

## Flow mode / Stage

严格模式 / strict；验证 / Verification。

## Basis

- Requirement: `docs/requirement/20260627-cloud-device-enrollment.md`，Review status: Accepted
- Spec: `docs/spec/20260627-cloud-device-enrollment.md`，Review status: Accepted
- Plan: `docs/plan/20260627-cloud-device-enrollment.md`，Review status: Accepted

说明：Requirement 文档较早，仍包含 CLI OAuth、绑定另一台设备、GUI 任意 Gate URL 输入等后来已被 Spec / Plan 与用户后续反馈收敛或否决的描述。本次验证以 Accepted Spec、Accepted Plan 和用户在实现阶段追加的明确指令为准：不新增 CLI binding 命令、不实现终端 setup code、不做敏感日志专项、不引入 profile、不保留 `gate.listen_url` alias。

## Verification scope

本次验证覆盖 Implementation 阶段实际 diff 与以下目标是否对齐：

1. `gate.listen_url` 从活跃配置与示例中移除，不保留 deprecated alias。
2. 本地 setup token 换取 `provider=local_access` Browser JWT。
3. 设备长期身份材料采用本机 Ed25519 key pair，Agent tunnel 使用 device signature。
4. Cloud binding backend 闭环：start / authorize / callback / exchange。
5. 持久化表：`devices`、`user_devices`、`device_keys`、`device_binding_codes`。
6. `/api/devices` 按 local access 或 cloud user-device binding 过滤，并叠加在线状态。
7. 设备删除删除云端绑定并断开在线 tunnel。
8. 前端 `/connect`、`/device-bindings/authorize`、设备状态与离线禁选。
9. 本地浏览器 setup 入口使用前端 `/setup`，启动输出指向 `agent.public_url`，而不是把后端 API 地址当作前端地址。
10. Vite dev host 由 `vite.config.ts` 统一控制，避免 package script CLI 参数覆盖配置。
11. 文档与示例同步到最终实现边界。

不验证以下事项为通过条件：

- request log 敏感字段 redaction；用户已明确“不搞敏感”。
- 新 CLI binding 命令、终端粘贴 setup code、headless/env-only binding。
- GUI 应用、跨设备二维码 / token / deep link、revoke tombstone / deny list。
- 真实外部 Google OAuth 或真实 Cloud Gate 部署；自动化测试使用本地测试与 mock/httptest 语义。

## Actual diff summary

当前相对 `HEAD` 的 diff 统计：

```text
54 files changed, 4894 insertions(+), 153 deletions(-)
```

主要改动类别：

- 配置与示例：`.termbridge.default.yaml`、`.env.example`、`Dockerfile`、`Dockerfile.cn`、`README.md`、`.air.toml`。
- SpecFlow 文档：新增/更新 requirement、spec、plan，并新增本 verification 文档。
- Agent / device identity：新增 device key storage、signature helper，Agent tunnel header 签名与验证相关测试。
- Auth / setup：新增 setup token store、local setup complete flow、local_access claims；本地 setup 链接由 typed `agent.public_url` 生成。
- Cloud binding：新增 local cloud binding attempt store、Cloud authorize/callback/exchange handlers 与测试。
- Database / repository：新增 device binding migrations 与 `internal/infrastructure/repository/device`。
- Gateway API：扩展 `/api/devices` 过滤、设备删除、device signature tunnel verification、binding endpoints。
- Frontend：新增 setup、connect、device-binding authorize 页面；设备状态、离线禁选、登录 redirect 保留。

## Expected vs actual changed files

### Expected files from Plan and matched actual files

| Plan area | Actual files | Result |
|---|---|---|
| Config canonical `agent.listen_url` / `agent.public_url` / cloud config | `.termbridge.default.yaml`, `.env.example`, `.air.toml`, `Dockerfile`, `Dockerfile.cn`, `README.md`, `Taskfile.yml`, `internal/infrastructure/config/config.go`, `internal/infrastructure/config/config_test.go`, `internal/app/app.go`, `internal/app/app_test.go` | 对齐；旧 `gate.listen_url` 不保留 alias；`agent.listen_url` 表达后端 API listen 地址，`agent.public_url` 表达浏览器访问本机 Agent 前端/setup 的地址。 |
| Local setup token | `internal/application/auth/setup.go`, `internal/application/auth/service.go`, `internal/transport/http/gatewayapi/server.go`, `web/src/views/SetupView.vue`, tests | 对齐；setup token hash-only、TTL、single-use 与 `local_access` JWT 路径有测试覆盖。 |
| Device key pair and signing | `internal/application/agent/device.go`, `internal/application/agent/signature.go`, `internal/application/agent/client.go`, related tests | 对齐；private key 本地，public key 可登记，Agent tunnel 使用签名 header。 |
| Cloud binding backend | `internal/application/auth/cloud_binding.go`, `internal/transport/http/gatewayapi/server.go`, `internal/transport/http/gatewayapi/cloud_binding_test.go`, `web/src/views/ConnectView.vue`, `web/src/views/DeviceBindingAuthorizeView.vue` | 对齐；包含 start / authorize / callback / exchange；前端 authorize 页面解决直接导航 API 无 Bearer token 的闭环问题。 |
| Persistent device model | `internal/infrastructure/database/migrations/mysql/202606280001_device_bindings.sql`, `internal/infrastructure/database/migrations/sqlite/202606280001_device_bindings.sql`, `internal/infrastructure/repository/device/repository.go`, `repository_test.go` | 对齐；覆盖 devices/user_devices/device_keys/device_binding_codes。 |
| `/api/devices` filtering and delete | `internal/transport/http/gatewayapi/server.go`, `cloud_binding_test.go`, `server_test.go`, `tunnel_test.go`, `terminal_e2e_test.go` | 对齐；local_access 仅本机，cloud user 按 binding 过滤，删除绑定并断开 route。 |
| Frontend status UX | `web/src/features/gateway/api.ts`, `web/src/store/gateway.ts`, `web/src/components/workspace/WorkspaceSessionSidebar.vue`, `web/src/views/SessionsView.vue`, `web/src/i18n.ts` | 对齐；设备状态类型、离线禁选、selected device 离线清理、文案已更新。 |

### Actual changed files

```text
.air.toml
.env.example
.pomelo-pw/m6-gateway-web-terminal.yaml
.termbridge.default.yaml
Dockerfile
Dockerfile.cn
README.md
Taskfile.yml
docs/plan/20260627-cloud-device-enrollment.md
docs/requirement/20260627-cloud-device-enrollment.md
docs/spec/20260627-cloud-device-enrollment.md
docs/verification/20260627-cloud-device-enrollment.md
internal/app/app.go
internal/app/app_test.go
internal/application/agent/client.go
internal/application/agent/client_test.go
internal/application/agent/device.go
internal/application/agent/device_test.go
internal/application/agent/signature.go
internal/application/auth/cloud_binding.go
internal/application/auth/service.go
internal/application/auth/setup.go
internal/infrastructure/config/config.go
internal/infrastructure/config/config_test.go
internal/infrastructure/database/migrations/mysql/202606280001_device_bindings.sql
internal/infrastructure/database/migrations/sqlite/202606280001_device_bindings.sql
internal/infrastructure/repository/device/repository.go
internal/infrastructure/repository/device/repository_test.go
internal/transport/http/gatewayapi/cloud_binding_test.go
internal/transport/http/gatewayapi/server.go
internal/transport/http/gatewayapi/server_test.go
internal/transport/http/gatewayapi/terminal_e2e_test.go
internal/transport/http/gatewayapi/tunnel_test.go
internal/transport/http/server/server.go
internal/transport/http/server/server_test.go
web/.env
web/package.json
web/src/components/workspace/WorkspaceSessionSidebar.vue
web/src/features/api/client.ts
web/src/features/gateway/api.test.ts
web/src/features/gateway/api.ts
web/src/features/sessions/useTerminalSocket.test.ts
web/src/i18n.ts
web/src/router/index.ts
web/src/store/gateway.test.ts
web/src/store/gateway.ts
web/src/views/ConnectView.vue
web/src/views/DeviceBindingAuthorizeView.vue
web/src/views/GoogleCallbackView.vue
web/src/views/HomeView.vue
web/src/views/LoginView.vue
web/src/views/SessionsView.vue
web/src/views/SetupView.vue
web/vite.config.ts
```

### Scope notes

- `web/src/views/DeviceBindingAuthorizeView.vue` 是实现闭环时新增的必要前端承接页。原因：浏览器从 local `/connect` 跳到 Cloud Gate 时不能直接给 API endpoint 附带 Bearer token；前端页面先经过 router guard / login，再携带 Browser JWT 调用 `/api/device-bindings/authorize`，后端用 JSON 返回本地 callback redirect URL。
- `web/src/features/api/client.ts` 与 `web/src/views/LoginView.vue` / `GoogleCallbackView.vue` 的 redirect 保留逻辑是该 authorize 页面闭环的必要支持。
- `internal/transport/http/server/server_test.go` 仅跟随 server route / SPA fallback 行为变化更新测试；最终没有把 `/device-bindings/` 直接挂到 API handler，因为该路径应由 Cloud Gate SPA 页面承接。

## Requirement alignment

| Requirement topic | Verification result |
|---|---|
| 用户登录云端后可看到/使用自己有权访问的设备 | 后端 `/api/devices` 已按 cloud user binding 过滤；前端设备下拉读取并展示设备状态。 |
| 普通用户不需要输入 raw `device_id` | 主路径 `/connect` 不要求用户输入 device id；device identity 来自本地配置/状态目录。 |
| 本地模式有认证边界，setup token 换取 login-equivalent credential | 已实现 setup token store、`/api/auth/setup/complete`、`provider=local_access`，并由 setup 页面保存同形态 token。 |
| Browser credential / setup material / device credential 语义分离 | 已实现 local_access Browser JWT、one-time setup/binding material、device key pair；Agent tunnel 使用 device signature。 |
| `/api/devices` 只返回当前用户有权访问设备 | 已实现 local_access 与 cloud user 两种过滤路径，并有测试覆盖。 |
| 设备撤销/删除后 Agent 和 Browser 操作失效 | 删除绑定并断开在线 tunnel；离线设备下一次连接因 public key/binding 不存在失败。 |
| 不把本地账号与云端账号合并 | 实现中 local_access 与 cloud user 仍是不同 claims/context，cloud binding 只绑定 device public key 到 cloud user。 |

Requirement 中的 CLI OAuth / 另一台设备 / GUI 任意 Gate URL 输入等早期内容已被后续 Accepted Spec、Plan 和用户指令否决，因此不作为本次实现验收缺口。

## Spec alignment

| Spec decision | Verification result |
|---|---|
| `agent.listen_url` 是本地后端 API listen 地址，`agent.connect_url` 是 Agent 连接目标，`agent.public_url` 是浏览器访问本机 Agent 前端/setup 的地址 | Config model、默认配置、Docker env、README、Taskfile 与 tests 已切换；setup URL 不再直接使用 `agent.listen_url`。 |
| `cloud.gate_url` / `cloud.callback_base_url` 固定配置 | Config、默认配置、env 示例与 binding start URL 构造已实现。 |
| 不新增 CLI binding 命令，不实现终端 setup code | 未新增 CLI binding 命令；setup 只通过 URL token 页面完成。 |
| 当前设备绑定通过 `termbridge serve` 已启动的本地 Web 页面完成 | `/setup`、`/connect`、local callback 与 Cloud authorize page 已实现。 |
| 设备列表展示状态，离线不可选 | `DeviceSummary.status`、前端 disabled offline item、store 选择约束已实现。 |
| 删除设备简单删除云端记录，在线则断开 tunnel | `DELETE /api/devices/{id}` 删除 binding，调用 disconnect route/terminal。 |
| Device credential 是长期 key pair；Cloud 保存 public key | `.termbridge/devices/<device_id>` key files + Cloud `device_keys.public_key` 已实现。 |
| Agent tunnel 使用 device key 签名认证 | Signed header 与 verification path 已实现；测试覆盖签名 tunnel。 |

## Plan alignment

| Plan step | Verification result |
|---|---|
| Step 0：不做 request log 敏感字段 redaction | 对齐；未扩大范围。 |
| Step 1：配置语义迁移与 cloud 配置 | 对齐；并按用户后续指令移除 alias，新增 typed `agent.public_url` 承载本地浏览器 setup 前端地址。 |
| Step 2：`.termbridge` 状态目录持久化基础 | 对齐；setup token、cloud binding attempt、device key 文件均落状态目录。 |
| Step 3：local setup token -> Browser JWT | 对齐；`local_access` token 与 setup 页面实现。 |
| Step 4：device key 与 Agent tunnel 签名认证 | 对齐；Agent client 与 gateway verification 都已迁移。 |
| Step 5：Cloud binding backend 闭环 | 对齐；start / authorize / callback / exchange + frontend authorize page 实现。 |
| Step 6：设备数据模型、列表过滤与删除断开 | 对齐；migrations、repository、API 与 tests 实现。 |
| Step 7：前端页面和路由 | 对齐；`/setup`、`/connect`、`/device-bindings/authorize`、offline disabled 实现。 |
| Step 8：文档、示例与迁移说明 | 对齐；README/config/docs 已更新，且本 Verification 文档记录实际偏差和风险。 |

## Acceptance criteria checklist

### Passed

- [x] `gate.listen_url` 不在活跃代码/配置示例中保留；非 docs 搜索无 `gate.listen_url` / `TERMBRIDGE_GATE__LISTEN_URL` / `Gate.ListenUrl`。
- [x] `agent.listen_url`、`agent.public_url`、`agent.connect_url`、`cloud.gate_url`、`cloud.callback_base_url` 可配置并有测试覆盖。
- [x] 本地 setup token hash-only、TTL、single-use，并换取 `local_access` Browser JWT。
- [x] Device id 不是 secret；device private key 本地保存，public key 可登记到 Cloud Gate。
- [x] Agent tunnel 使用 method/path/device_id/timestamp/nonce/audience 签名认证。
- [x] Cloud binding backend start / authorize / callback / exchange 实现，并有自动化测试覆盖主要 happy path、非 loopback callback、code replay。
- [x] 新增 `devices`、`user_devices`、`device_keys`、`device_binding_codes` migrations。
- [x] `/api/devices` 对 local_access 与 cloud user 分别过滤设备。
- [x] 删除设备会删除 user-device binding；在线 route 会被断开。
- [x] 前端 `/setup`、`/connect` 与 `/device-bindings/authorize` 路由实现；本地首页/受保护路由会按 setup 状态进入 setup 引导。
- [x] 前端设备下拉展示 online/offline，离线设备不可选。
- [x] 单个在线设备自动选择；已选设备变离线后不继续保留。
- [x] 不新增 CLI binding 命令、不实现终端 setup code、不引入 profile 概念。

### Not fully verified manually

- [ ] 未在真实浏览器中手动跑完整跨两个 Gate 实例的 Cloud binding 流程。
- [ ] 未连接真实 Google OAuth provider 或真实外部 Cloud Gate；自动化测试覆盖本地逻辑，外部登录/部署留给后续部署验收。
- [ ] 未手动验证 Docker 镜像启动后的完整浏览器流；仅更新配置环境变量并通过 Go/Web 自动化检查。

## Automated test results

### Go

Command:

```text
go test ./cmd/... ./internal/...
```

Result:

```text
PASS / ok for all packages with tests
```

The focused config/app check was also run after the final `agent.public_url` migration:

```text
go test ./internal/infrastructure/config ./internal/app
```

Result:

```text
ok  termbridge-go/internal/infrastructure/config
ok  termbridge-go/internal/app
```

Observed package summary:

```text
?    termbridge-go/cmd/termbridge [no test files]
ok   termbridge-go/internal/app
ok   termbridge-go/internal/application/agent
?    termbridge-go/internal/application/auth [no test files]
ok   termbridge-go/internal/application/runner
ok   termbridge-go/internal/application/terminal
ok   termbridge-go/internal/domain/identity
ok   termbridge-go/internal/domain/process
ok   termbridge-go/internal/domain/session
ok   termbridge-go/internal/domain/workspace
ok   termbridge-go/internal/infrastructure/config
?    termbridge-go/internal/infrastructure/database [no test files]
?    termbridge-go/internal/infrastructure/database/migrations [no test files]
?    termbridge-go/internal/infrastructure/email [no test files]
ok   termbridge-go/internal/infrastructure/errors
ok   termbridge-go/internal/infrastructure/history
ok   termbridge-go/internal/infrastructure/logging
?    termbridge-go/internal/infrastructure/pty [no test files]
ok   termbridge-go/internal/infrastructure/pty/gopty
?    termbridge-go/internal/infrastructure/repository/auth [no test files]
ok   termbridge-go/internal/infrastructure/repository/device
ok   termbridge-go/internal/infrastructure/repository/state
?    termbridge-go/internal/infrastructure/version [no test files]
ok   termbridge-go/internal/protocol/terminal
ok   termbridge-go/internal/protocol/tunnel
ok   termbridge-go/internal/transport/cli
ok   termbridge-go/internal/transport/http/gatewayapi
ok   termbridge-go/internal/transport/http/gatewayapi/auth
ok   termbridge-go/internal/transport/http/middleware/requestlog
ok   termbridge-go/internal/transport/http/server
```

### Web typecheck

Command:

```text
yarn --cwd web typecheck
```

Result:

```text
yarn run v1.22.22
$ vue-tsc --noEmit
Done in 1.76s.
```

### Web lint

Command:

```text
yarn --cwd web lint
```

Result:

```text
yarn run v1.22.22
$ eslint .
Done in 1.25s.
```

### Web tests

Command:

```text
yarn --cwd web test --run
```

Result:

```text
Test Files  10 passed (10)
Tests  42 passed (42)
```

### Web format

Command:

```text
yarn --cwd web format:check
```

Result:

```text
All matched files use Prettier code style!
```

### Go vet

Command:

```text
go vet ./cmd/... ./internal/...
```

Result: no output, meaning no vet diagnostics reported.

### Whitespace / patch sanity

Command:

```text
git diff --check
```

Result: no output, meaning no whitespace errors reported.

### Active config key search

Command category: repository content search excluding docs.

Patterns:

```text
TERMBRIDGE_GATE__LISTEN_URL|Gate\.ListenUrl|gate\.listen_url
web\.public_url|WEB__PUBLIC_URL
TERMBRIDGE_AGENT__PUBLIC_URL=https://gate\.example\.com
```

Result: no matches outside historical docs for removed/invalid active config names; Cloud Gate example no longer assigns `agent.public_url` to the Cloud Gate publish URL.

## Missed or expanded scope

### Expanded but justified

1. 新增 `web/src/views/DeviceBindingAuthorizeView.vue`。
   - 原因：Cloud authorize 是浏览器用户会话动作；直接导航 API endpoint 无法自动附带 Bearer token。前端承接页可经过 router guard / login，再调用 API 并拿到 `redirect_url`。
2. 修改 `web/src/features/api/client.ts`、`LoginView.vue`、`GoogleCallbackView.vue` 的 redirect 处理。
   - 原因：Cloud authorize 页面需要在登录后恢复原始目标 URL，否则未登录 cloud user 无法完成 binding。
3. 更新 `Dockerfile` / `Dockerfile.cn`。
   - 原因：用户要求不保留 `gate.listen_url`，Docker runtime env 也必须迁移到 `TERMBRIDGE_AGENT__LISTEN_URL`。

### Not implemented by design

1. 前端没有提供 cloud 设备删除按钮 / 确认弹窗。
   - 后端 `DELETE /api/devices/{device_id}` 已实现；Plan 重点要求后端删除与断开语义。若产品需要 UI 删除入口，应作为后续 UX 任务补充。
2. 未实现 request log redaction。
   - 用户明确“不搞敏感”，Plan Step 0 已声明不纳入本 feature。
3. 未实现 revoke tombstone / deny list。
   - Spec/Plan 明确第一版简单删除记录，允许重新绑定。
4. 未实现 CLI binding command、terminal setup code、headless/env-only binding。
   - 用户明确不实现。

## Risks and residual items

1. **真实跨 Gate 流程仍需人工/部署验收**：自动化测试覆盖本地 handler/repository/前端类型与 lint，但未启动两个真实 Gate 实例做浏览器端到端验证。
2. **OAuth redirect storage 使用 localStorage**：为保持 Google OAuth 后回到 authorize page，前端暂存 redirect 到 `termbridge.oauth_redirect`。值经过同源路径校验，但仍应在真实浏览器测试中确认异常中断后的行为。
3. **Cloud binding attempt local state file 损坏场景**：实现有错误返回路径，但 Verification 未专门注入损坏 JSON 做负向测试。
4. **Device private key 文件存储**：第一版按计划保存在 `.termbridge/devices/<device_id>/private_key.pem`，OS keychain 是后续增强。
5. **Docker runtime 行为未手动启动验证**：Dockerfile env 已迁移；镜像默认 `agent.public_url=http://localhost` 适配本机浏览器访问容器发布端口，未构建镜像实测。
6. **旧配置迁移是破坏性变更**：按用户指令不保留 alias；旧 `gate.listen_url` 配置会失败，需要用户迁移到 `agent.listen_url`。
7. **本地 setup 链接依赖部署者理解浏览器可访问地址**：`agent.public_url` 是浏览器看到的本机 Agent 前端/setup 地址，不跟随 Cloud Gate 公网发布地址；反向代理、端口映射或非 localhost 访问需要部署时覆盖该值。

## Conclusion

Verification 结论：通过，带残余人工验收建议。

本实现与 Accepted Spec / Plan 及用户后续明确指令整体对齐。自动化检查全部通过：

- `go test ./cmd/... ./internal/...`
- `go vet ./cmd/... ./internal/...`
- `yarn --cwd web test --run`
- `yarn --cwd web typecheck`
- `yarn --cwd web lint`
- `yarn --cwd web format:check`
- `git diff --check`

本次没有执行 `git add`、`git commit`、`git push` 或其他 git 写操作。

建议在合并/提交前追加一次人工浏览器验收：本地 `termbridge serve` 打开 `/setup` 获取 local_access → `/connect` → Cloud login/authorize → local callback success → Cloud `/api/devices` 可见当前设备 → 删除设备后在线 tunnel 断开。
