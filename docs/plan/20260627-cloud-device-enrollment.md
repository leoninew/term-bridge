# 云端 Gate 设备绑定与本地 Agent 接入计划
最后修改时间: 2026-06-28 09:25:15

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；计划 / Plan 已接受，当前进入实现 / Implementation。

## Basis

- Requirement: `docs/requirement/20260627-cloud-device-enrollment.md`，Review status: Accepted
- Spec: `docs/spec/20260627-cloud-device-enrollment.md`，Review status: Accepted

说明：Requirement 文档较早，仍保留了后来已被用户修正的 CLI OAuth / 另一台设备等场景；本 Plan 以最新 Accepted Spec 与用户后续澄清为准。

## Goal of this plan

本计划定义第一版实现顺序和验收边界：

1. 本地模式通过 `/setup` 使用一次性 setup token 换取与 login 等效的 Browser JWT，provider 为 `local_access`。
2. 不创建长期本地账号 / 密码，不做本地账号与云端账号合并。
3. 本地 Web 服务入口使用 `agent.listen_url` 语义；Agent 连接目标使用 `agent.connect_url`。
4. 固定 Cloud Gate URL 配置为 `cloud.gate_url`。
5. Cloud 授权完成后回调本机的地址基准配置为 `cloud.callback_base_url`，示例 `http://127.0.0.1:9030`。
6. 当前没有 GUI，也不新增新的 CLI binding 命令；第一版通过 `termbridge serve` 已启动的本地 Web 页面完成当前设备绑定。
7. Cloud binding 完成后，Cloud Gate 保存当前设备 public key 并建立 user-device binding；Agent tunnel 第一版迁移为 device key 签名认证。
8. `/api/devices` 在本地模式只返回当前 local device；在云端模式按当前 cloud user 的 device binding 过滤，再叠加在线状态。
9. 设备删除只删除云端记录；如果设备当前在线，则立即断开已有 Agent tunnel；允许用户重新绑定。
10. 本 feature 不实现 request log 敏感字段 redaction；基础 auth verification 已记录的敏感日志问题保留为独立后续项。

## Product and credential model clarified

### Browser JWT 与 device key 的关系

本 feature 中会出现多种身份材料，但它们不是同一种凭据：

| 材料 | 生成 / 颁发时机 | 使用者 | 用途 | 存储位置 | 是否可互换 |
|---|---|---|---|---|---|
| Browser access JWT | email / Google login 成功，或 local setup 成功 | Browser / Frontend | 访问 Web API、`/api/auth/me`、路由 guard、Browser WebSocket 鉴权 | 前端 token 存储 | 不可作为 Agent 设备材料 |
| local setup token | `termbridge serve` 本地生成 | Browser 一次性提交 | 换取 `provider=local_access` Browser JWT | `.termbridge` 状态目录中只保存 hash | 不是登录态，不是设备材料 |
| cloud binding code/state | Cloud Gate 与 Local Gate 绑定交接 | Browser redirect + Local Gate server | 让 Cloud Gate 接受当前 device identity 并建立 user-device binding | cloud DB + local `.termbridge` state hash | 不是登录态，不是设备材料 |
| device key pair | 本机首次需要 device identity 时生成 | Agent / Local Gate | 证明“这台设备是谁”；Agent tunnel 通过 private key 签名，Cloud Gate 用 public key 验证 | private key 只保存在本机 `.termbridge`；Cloud Gate 保存 public key | 不可作为 Browser JWT |

本任务中 `device credential` 的准确语义是设备长期身份材料，即同一台设备持有的一组 key pair：private key 只在本机，public key 可在 cloud binding 时登记到 Cloud Gate。Cloud binding 不是让 Cloud Gate 签发另一份设备 token，而是让 Cloud Gate 接受这台设备的 public key，并记录当前 cloud user 对该 device 的访问关系。

### local device 与当前 DeviceRegistry 的关系

当前代码中的 `DeviceRegistry` 是运行时内存里的在线连接表：Agent tunnel hello 成功后注册，断开后标记 offline。它不是产品授权表，也不是新的“local registry”概念。

计划中的术语定义：

- **local device**：当前 `termbridge serve` 所在机器的稳定 device identity，来源于 `agent.device_id` / `.termbridge/devices/<device_id>/device.json`，代表“这台本机设备”。
- **runtime registry / DeviceRegistry**：运行时在线状态表，回答“某个 device 当前有没有 tunnel 在线”。
- **cloud user-device binding**：云端持久化授权关系，回答“当前 cloud user 是否有权访问某个 device”。

因此 `/api/devices` 的第一版语义是：

- `provider=local_access`：只返回当前 local device，并叠加 runtime registry 的 online/offline 状态。
- cloud user：从 `user_devices` 按当前 user 过滤可访问设备，再叠加 runtime registry 的 online/offline 状态。

## Implementation steps

### Step 0：范围校准：不做 request log 敏感字段 redaction

基础用户系统 Verification 已记录普通 request started/completed log 仍可能泄露 token。用户已明确本 feature “不搞敏感”，因此本计划不把 request log redaction 作为前置项，也不在实现中扩大到该专项。

实现约束：

1. 本 feature 新增接口应尽量避免把 setup token / binding code / state / device private key / signature 写入业务日志。
2. 不改造现有普通 request logging middleware。
3. Verification 阶段只记录该风险为基础 auth 后续项，不把它作为本 feature 的验收阻塞。

### Step 1：配置语义迁移与新增 cloud 配置

实施内容：

1. 在 config model 中引入 canonical 字段：

   ```yaml
   agent:
     listen_url: http://127.0.0.1:9030
     connect_url: http://127.0.0.1:9030

   cloud:
     gate_url: https://gate.example.com
     callback_base_url: http://127.0.0.1:9030
   ```

2. `agent.listen_url` 表示 `termbridge serve` 启动的本地 Web 服务地址。
3. `agent.connect_url` 表示 Agent 要连接的 Gate 地址。
4. 本地模式判定改为：

   ```text
   normalized(agent.connect_url) == normalized(agent.listen_url)
   ```

5. 按用户后续实现指令，不保留 `gate.listen_url` deprecated alias；旧 key 作为未知配置直接失败，避免继续维护双源配置。
6. `agent.connect_url` 为空时默认继承 `agent.listen_url`。
7. 新增 `cloud.gate_url`：Local Gate 发起 cloud binding 时使用的固定 Cloud Gate 地址。
8. 新增 `cloud.callback_base_url`：Cloud 授权完成后回调当前 Local Gate 的地址基准，默认示例为 `http://127.0.0.1:9030`。
9. 新增环境变量：
   - `TERMBRIDGE_AGENT__LISTEN_URL`
   - `TERMBRIDGE_CLOUD__GATE_URL`
   - `TERMBRIDGE_CLOUD__CALLBACK_BASE_URL`

预期文件：

- `internal/infrastructure/config/config.go`
- `internal/infrastructure/config/config_test.go`
- `.termbridge.default.yaml`
- `.env.example`
- `README.md`

验收点：

- 旧配置 `gate.listen_url` 不再被接受，并以 unknown config key 失败。
- 新配置 `agent.listen_url` 可启动。
- `agent.connect_url` 为空时进入本地模式。
- `cloud.gate_url` 和 `cloud.callback_base_url` 能从文件 / env 加载。

### Step 2：`.termbridge` 状态目录持久化基础

实施内容：

1. 在 runtime state dir（默认 `.termbridge`）下新增本 feature 的状态文件布局：

   ```text
   .termbridge/
     auth/
       setup-tokens.json
       cloud-binding-attempts.json
     devices/
       <device_id>/
         device.json
         private_key.pem
        public_key.pem
   ```

2. `setup-tokens.json` 只保存 token hash、expires_at、used_at、created_at，不保存明文 setup token。
3. `cloud-binding-attempts.json` 保存 local binding state hash、callback URL、cloud authorize URL、expires_at、completed_at、error。
4. `private_key.pem` 保存本机 device private key；`public_key.pem` 保存对应 public key；private key 文件权限尽量使用 `0600`，Windows 下记录 best-effort 限制。
5. 写入必须采用 atomic write：先写临时文件，再 rename，避免半写状态。
6. 读取时对 schema_version 做校验；脏数据返回明确错误，不静默忽略。

预期文件：

- `internal/application/agent/device.go` 或新增 `internal/application/device/*`
- `internal/infrastructure/repository/state/*`
- `internal/app/app.go`
- 对应测试文件

验收点：

- setup token / binding attempt 不保存明文。
- device private key 不写入普通 YAML 配置。
- 状态文件损坏时返回可诊断错误。

### Step 3：local setup token -> login-equivalent Browser JWT

实施内容：

1. `termbridge serve` 在 local mode 下生成一次性 setup token，并写入 `.termbridge/auth/setup-tokens.json` 的 hash 状态。
2. setup token TTL 默认 10 分钟，单次使用；可在后续版本增加配置，本版不额外扩大配置面。
3. 默认支持 direct URL token：本地 Gate 生成 `/setup?token=<setup_token>` 形式的一次性 setup URL。
4. 不实现终端 setup code、粘贴输入或 CLI fallback；Cloud Gate 不能生成本地 setup token，因为该 token 授权的是 Local Gate，本地授权材料必须由 Local Gate 生成。
5. 前端读取 URL token 后立即调用 `history.replaceState` 移除地址栏 token。
6. 新增 API：

   ```text
   GET  /api/auth/setup/status
   POST /api/auth/setup/complete
   ```

7. `POST /api/auth/setup/complete` 成功返回与 login 一致的 `TokenResp`：

   ```json
   {
     "access_token": "<browser_jwt>",
     "token_type": "bearer"
   }
   ```

8. `local_access` Browser JWT：
   - provider = `local_access`
   - sub = `local:<device_id>`
   - TTL 第一版沿用 `auth.jwt_ttl`
   - 不引入 refresh token
   - token 过期后前端清理并重新进入 `/setup`
9. `/api/auth/me` 对 `local_access` 返回本地访问用户信息和 local mode capabilities。
10. setup 成功后的前端路径复用已有 login 成功逻辑：保存 token、初始化 `/api/auth/me`、进入受保护路由。

预期文件：

- `internal/application/auth/*`
- `internal/transport/http/gatewayapi/*`
- `web/src/features/gateway/api.ts`
- `web/src/store/gateway.ts`
- `web/src/router/index.ts`
- `web/src/views/SetupView.vue`

验收点：

- 未认证访问本地受保护页面会进入 `/setup`。
- setup token 一次性、过期后不可用。
- setup 成功后 `/api/auth/me` 返回 `provider=local_access`。
- 前端不会把 setup token 留在地址栏。
- 过期 Browser JWT 不刷新，重新走 setup。

### Step 4：device key 与 Agent tunnel 签名认证迁移

为什么本 feature 要替换基础用户系统里的 AuthService verifier 复用路径：

1. AuthService verifier 的职责是验证 Browser 用户登录域，例如 email/password、Google user、local_admin shortcut；它回答“人是谁”。
2. Agent tunnel 的长期连接需要验证 device identity；它回答“这台设备是否被允许连接 Gate”。
3. 设备删除、重绑、key rotation 都应该只影响某台 device，不应该要求修改用户密码、撤销用户 Browser session，或保存用户密码给 Agent。
4. Google 用户没有本地密码；如果 Agent tunnel 继续依赖用户凭据，Google-only 用户无法自然拥有 Agent 长期连接凭据。
5. local setup 返回的 Browser JWT 只是允许当前浏览器访问本机 Gate，不应被 Agent 保存并长期用于 tunnel。

所以“必须替换”是模型收敛要求，不是说当前 Basic Auth 代码立刻不能工作。若实现需要拆分，旧 Basic Auth 可短期保留为兼容 fallback，但不能作为本 feature 的最终验收标准。

实施内容：

1. 新增 device key service，确保每台设备有一组长期 key pair。
2. private key 只保存在本机 `.termbridge/devices/<device_id>/private_key.pem`。
3. public key 可在 cloud binding 时登记到 Cloud Gate。
4. Agent tunnel header 采用：

   ```text
   X-TermBridge-Device-ID: <device_id>
   X-TermBridge-Device-Timestamp: <unix_seconds>
   X-TermBridge-Device-Nonce: <random_nonce>
   X-TermBridge-Device-Signature: <signature>
   ```

5. 签名内容覆盖 method、path、device_id、timestamp、nonce 和 Gate audience。
6. 移除 Agent tunnel 对 Browser email/password、local_admin Basic Auth 的正式依赖。
7. Agent client 从 state dir 读取 private key，并用 device key 签名连接 `/api/agent/tunnel`。
8. 服务端根据 device id 查 public key，验证签名后再接受 WebSocket，并将 tunnel 绑定到 device id。
9. 如果实现过程中必须临时保留 Basic Auth fallback，只能作为开发兼容开关；在 Verification 结论中不能把它标为最终完成，必须列为阻断最终验收风险。

预期文件：

- `internal/application/device/*` 或等价 package
- `internal/transport/http/gatewayapi/auth/*`
- `internal/transport/http/gatewayapi/server.go`
- `internal/application/agent/client.go`
- `internal/app/app.go`
- `internal/transport/http/gatewayapi/tunnel_test.go`

验收点：

- Browser JWT 不能连接 `/api/agent/tunnel`。
- Device signature 不能访问 Browser API。
- 删除 public key / binding 记录后，旧 device signature 连接失败。
- Agent tunnel 不再依赖用户密码或 Browser JWT。

### Step 5：Cloud binding backend 闭环

实施内容：

1. 本地 Web 入口：

   ```text
   GET /connect
   ```

   未 local_access 时跳转 `/setup`；已有 local_access 时展示固定 Cloud Gate 目标。

2. 本地启动绑定：

   ```text
   POST /api/cloud-binding/start
   Authorization: Bearer <local_access_jwt>
   ```

   Local Gate：
   - 读取 `cloud.gate_url`。
   - 生成 local state。
   - 保存 state hash 到 `.termbridge/auth/cloud-binding-attempts.json`。
   - 构造 callback URL：`<agent.listen_url>/api/cloud-binding/callback`。
   - 返回 Cloud authorize URL。

3. Cloud authorize：

   ```text
   GET /device-bindings/authorize?callback=...&state=...
   ```

   Cloud Gate：
   - 要求 cloud Browser user 已登录。
   - 只允许 loopback callback host，例如 `127.0.0.1` / `localhost`。
   - 展示“将当前设备连接到当前 cloud account”的确认页。
   - 生成短期 binding code，hash 存储在 cloud DB。
   - redirect 回 local callback。

4. Local callback：

   ```text
   GET /api/cloud-binding/callback?code=<binding_code>&state=<local_state>
   ```

   Local Gate：
   - 校验 local state hash、TTL、未完成状态。
   - server-to-server 调用 Cloud exchange。

5. Cloud exchange：

   ```text
   POST /api/device-bindings/exchange
   ```

   Cloud Gate：
   - 校验 binding code hash、TTL、未使用。
   - 创建 / 更新 `devices`。
   - 创建 `user_devices` owner binding。
   - 保存 device public key。
   - 返回 binding accepted 和 device 摘要。

6. Local Gate 持久化：
   - 不把 localhost 本地配置原地改成远程配置。
   - 不引入 `cloud profile` / `remote connection profile` 概念。
   - 标记 binding attempt completed。
   - Cloud Gate 通过已保存 public key 接受该 device；本机继续持有同一份 private key。
   - serve 运行后，打开 localhost 是本地访问，打开 Cloud URL 是云端访问，不存在“切换模式”。

预期文件：

- `internal/application/device/*`
- `internal/infrastructure/repository/*`
- `internal/infrastructure/database/migrations/*`
- `internal/transport/http/gatewayapi/*`
- `web/src/views/ConnectView.vue`
- `web/src/features/gateway/api.ts`
- `web/src/router/index.ts`

验收点：

- 第一版不新增 `termbridge device enroll` 命令。
- Cloud Gate 未登录时走现有 Web login / Google OAuth。
- binding code/state 一次性、过期不可复用。
- Cloud exchange 后 device 出现在当前用户设备列表中。
- Local Agent 使用 device private key 签名连接 Cloud Gate。

### Step 6：设备数据模型、列表过滤与删除断开

实施内容：

1. 新增云端持久化模型：

   ```text
   devices
   user_devices
   device_keys
   device_binding_codes / attempts
   ```

2. `/api/devices`：
   - `provider=local_access`：返回当前 local device。
   - cloud user：只返回当前 user 在 `user_devices` 中有 binding 的设备。
   - 返回值叠加 runtime `DeviceRegistry` online/offline 状态。
3. 设备下拉响应增加 `status` 字段，保持 `online` 兼容：

   ```json
   {
     "id": "dev_...",
     "name": "DESKTOP-5C03B03",
     "online": true,
     "status": "online",
     "last_seen": "2026-06-27T...Z"
   }
   ```

4. 前端选择设备时只能选择 `online=true` 的设备。
5. 如果当前 selected device 变为 offline：
   - UI 保留当前设备上下文但禁用新 session 操作。
   - 显示 offline 状态。
6. 删除设备：

   ```text
   DELETE /api/devices/{device_id}
   ```

   - 检查当前 cloud user 是否拥有该 device binding。
   - 删除 user-device binding 和对应 public key 记录。
   - 如果该 device 当前在线，调用 runtime disconnect：关闭 Agent tunnel、关闭 terminal relay、从 route map 清理、标记 offline。
   - 不创建 revoked tombstone。
   - 不阻止同一 device 重新绑定。

预期文件：

- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/route.go`
- `internal/application/device/*`
- `internal/infrastructure/repository/*`
- `web/src/store/gateway.ts`
- `web/src/components/*` 或 `web/src/views/SessionsView.vue`

验收点：

- 用户 A 看不到用户 B 的设备。
- local_access 只看到本机 local device。
- 离线设备 disabled，不可选。
- 删除在线设备会立即断开 tunnel。
- 删除离线设备后旧签名连接下次失败。

### Step 7：前端页面和路由

实施内容：

1. 新增 `/setup` 页面：
   - 只支持 URL token。
   - 不实现终端 setup code、粘贴输入或 CLI fallback。
   - setup 成功后调用 store `setToken()`，复用 login 成功路径。
   - 成功后跳转原目标或 `/sessions`。
2. 新增 `/connect` 页面：
   - 要求 local_access。
   - 展示固定 `cloud.gate_url` 目标。
   - 调用 `/api/cloud-binding/start`。
   - 导航到 Cloud authorize URL。
   - 处理 callback 后展示成功 / 失败状态。
3. Cloud Gate 空设备状态：
   - 无设备时展示“连接当前设备”。
   - 说明需要先在当前设备运行 `termbridge serve`，并通过本机 `/connect` 发起绑定。
   - 绑定流程使用 `cloud.callback_base_url` 派生本机 callback，例如 `http://127.0.0.1:9030/api/cloud-binding/callback`。
4. Device dropdown：
   - 显示设备名与 online/offline 状态。
   - offline disabled。
   - 单 online 设备可以自动选中；多设备必须显式选择。
5. `SessionsView`：
   - 依赖 selected online device 才加载 workspace/session。
   - local_access 下默认选择当前 local device。

预期文件：

- `web/src/router/index.ts`
- `web/src/views/SetupView.vue`
- `web/src/views/ConnectView.vue`
- `web/src/views/SessionsView.vue`
- `web/src/components/session/*`
- `web/src/features/gateway/api.ts`
- `web/src/store/gateway.ts`
- `web/src/i18n.ts`

验收点：

- setup 与 login 成功后的 token 保存路径一致。
- URL 中 setup token 会被清除。
- Cloud empty state 显示本地连接 URL。
- Offline device 不可选。

### Step 8：文档、示例与迁移说明

实施内容：

1. 更新 README：
   - 本地 setup 流程。
   - 连接 Cloud Gate 当前设备流程。
   - `agent.listen_url` / `agent.connect_url` / `cloud.gate_url` / `cloud.callback_base_url` 语义。
   - Browser credential 与 device key 边界。
2. 更新 `.env.example` 和 `.termbridge.default.yaml`。
3. 标注 `gate.listen_url` 已移除，不作为 deprecated alias 保留。
4. 说明第一版不支持：
   - 新 CLI binding 命令。
   - 绑定另一台设备。
   - headless / env-only binding。
   - revoke tombstone / 永久 deny list。
5. 更新 SpecFlow Verification 文档时记录实际偏差和未完成项。

预期文件：

- `README.md`
- `.env.example`
- `.termbridge.default.yaml`
- `docs/verification/20260627-cloud-device-enrollment.md`（Verification 阶段再创建）

## Files to change summary

### Backend / Go

- `internal/infrastructure/config/config.go`
- `internal/infrastructure/config/config_test.go`
- `internal/app/app.go`
- `internal/application/auth/*`
- `internal/application/agent/*`
- `internal/application/device/*`（新增，或等价命名）
- `internal/infrastructure/repository/*`
- `internal/infrastructure/repository/state/*`
- `internal/infrastructure/database/migrations/*`
- `internal/transport/http/gatewayapi/*`
- `internal/transport/http/httpserver/*`（若 request logging 在该 package）

### Frontend / Web

- `web/src/features/gateway/api.ts`
- `web/src/store/gateway.ts`
- `web/src/router/index.ts`
- `web/src/views/SetupView.vue`（新增）
- `web/src/views/ConnectView.vue`（新增）
- `web/src/views/SessionsView.vue`
- `web/src/components/session/*`
- `web/src/i18n.ts`

### Config / docs

- `.termbridge.default.yaml`
- `.env.example`
- `README.md`
- `docs/spec/20260627-cloud-device-enrollment.md`（已按用户反馈修正配置命名和 Mermaid 错误）
- `docs/verification/20260627-cloud-device-enrollment.md`（Verification 阶段创建）

## Verification plan

### Automated checks

1. Go tests：

   ```text
   go test ./cmd/... ./internal/...
   ```

2. Go vet：

   ```text
   go vet ./cmd/... ./internal/...
   ```

3. Web tests：

   ```text
   npm --prefix web run test
   ```

4. TypeScript：

   ```text
   npm --prefix web run typecheck
   ```

5. ESLint：

   ```text
   npm --prefix web run lint
   ```

6. Web build：

   ```text
   npm --prefix web run build
   ```

7. Project check：

   ```text
   task check
   ```

### Targeted backend tests

- Config：
  - `agent.listen_url` 新配置。
  - `gate.listen_url` unknown config key 失败。
  - `cloud.gate_url` / `cloud.callback_base_url` env override。
- Local setup：
  - setup token hash 存储。
  - TTL / used / replay。
  - `TokenResp` shape。
  - `/api/auth/me` returns `provider=local_access`。
- Device key / tunnel signature：
  - Browser JWT cannot auth Agent tunnel。
  - Unsigned tunnel request cannot auth Agent tunnel。
  - Invalid device signature is rejected。
  - deleted public key / binding cannot reconnect。
- Cloud binding：
  - state mismatch rejected。
  - binding code replay rejected。
  - callback host allowlist only loopback。
  - exchange stores device public key and creates user-device binding atomically。
- Device API：
  - cloud user only sees own devices。
  - local_access only sees current local device。
  - deleting online device disconnects tunnel。
- Logging：
  - 本 feature 不改造普通 request log redaction；只检查新增业务日志不主动写入 setup token、binding code/state、device private key、signature、Browser token 或 password 明文。

### Targeted frontend tests

- `/setup?token=...` reads token, completes setup, removes token from URL, saves access token through existing store path。
- `/setup` without token shows an actionable error / guidance, but does not provide paste-code input。
- `/connect` requires local_access and starts binding。
- Cloud binding flow derives local callback from `cloud.callback_base_url`。
- Device dropdown disables offline devices。
- Selected device cannot become an offline target for new session creation。

### Manual verification scenarios

1. 本地首次访问：
   - 启动 `termbridge serve`。
   - 打开 setup URL。
   - 完成 setup。
   - 进入 `/sessions`。
   - `/api/auth/me` 显示 `local_access`。
   - `/api/devices` 只显示本机设备。

2. 本地绑定云端：
   - 本地 `/connect` 发起 binding。
   - 浏览器跳到 Cloud Gate。
   - Cloud Gate 登录 / Google OAuth。
   - 确认绑定当前设备。
   - redirect 回本地 callback。
   - Cloud Gate 保存 device public key。
   - Agent 使用 device private key 签名连接 Cloud Gate。
   - Cloud Gate device dropdown 显示该设备 online。

3. 删除在线设备：
   - Cloud Gate 删除当前 online device。
   - Agent tunnel 立即断开。
   - 前端 dropdown 状态变为无设备或 offline / deleted。
   - 已删除 public key / binding 的设备重连失败。
   - 用户可重新绑定同一设备。

4. 日志范围检查：
   - 不做普通 request log redaction 专项。
   - 检查新增业务日志不要主动输出 setup token、code、state、device private key、signature、Browser JWT 明文。

5. Mermaid 文档检查：
   - 确认 Spec 中“通过本地 Web 页面绑定当前设备到云端 Gate”和“本地访问与云端访问同一设备”两个 Mermaid 图可渲染。

## Assumptions

1. 第一版仍以单用户 personal device binding 为主，不实现 Org / Team / RBAC。
2. 第一版不实现绑定另一台设备，不生成跨设备二维码 / 命令 / token。
3. 第一版不新增 CLI binding 命令；`termbridge serve` 是唯一需要用户启动的本地入口。
4. 第一版 device credential 定义为每台设备一组长期 key pair；Cloud Gate 保存 public key，以支持删除绑定后失效。
5. 第一版 `local_access` JWT 不引入 refresh token；过期后重新 setup。
6. Cloud Gate 不能生成 Local Gate setup token；Cloud binding 只使用 `cloud.callback_base_url` 派生的 callback，真正授权本机访问的 setup token 由 Local Gate 生成。
7. `.termbridge` 状态目录是第一版本地敏感状态落点；OS keychain 是后续增强。

## Risks

1. **配置迁移风险**：从 `gate.listen_url` 迁移到 `agent.listen_url` 会影响已有配置；按用户后续实现指令不保留 alias，旧 key 必须显式失败，避免配置语义继续漂移。
2. **签名认证风险**：device signature 必须绑定 method、path、device_id、timestamp、nonce 和 Gate audience，并校验时间窗口 / nonce，避免重放或跨 Gate 复用。
3. **日志范围风险**：基础 auth 已知普通 request log redaction 缺口不在本 feature 范围内；本 feature 只约束新增业务日志不主动输出敏感明文。
4. **状态文件风险**：`.termbridge` 明文保存 device private key 比 OS keychain 弱；第一版接受但要限制权限并记录后续迁移。
5. **本地 / 远程访问边界风险**：本地和远程是不同访问入口，不应引入 profile 或“切换模式”心智；Cloud binding 只让 Cloud Gate 接受当前 device identity / public key，不破坏 localhost 本地访问。
6. **云端本地链接可达性风险**：`cloud.callback_base_url` 默认 `127.0.0.1:9030` 假设本机端口一致；非默认端口需要用户配置，UI 要给出清晰错误。
7. **删除即断开风险**：关闭在线 tunnel 会影响正在运行的 session；UI 删除操作需要确认，后端要保证关闭 route/terminal relay 后状态一致。
8. **Requirement stale 风险**：Requirement 中仍包含已被 Spec 否决的 CLI OAuth/另一台设备描述；实现和验收必须以 Spec/Plan 为准。

## Rollback / recovery plan

1. 配置迁移出错：旧 `gate.listen_url` 配置会显式失败；回滚方式是恢复旧版本代码，或把配置迁移到 `agent.listen_url` 后继续启动。
2. Local setup 失败：删除 `.termbridge/auth/setup-tokens.json` 后重启 `termbridge serve` 重新生成 setup token。
3. Cloud binding 中途失败：标记 local binding attempt failed，不修改本地配置，不覆盖已有 device key。
4. Cloud exchange 后若本地记录完成状态失败，前端提示重新检查 / 重新绑定；Cloud 侧可能已有 device record / public key，需要允许用户删除后重试。
5. Agent tunnel device auth 失败：保留测试覆盖，禁止静默 fallback 到 Browser credential；开发阶段可有显式 debug fallback，但最终验收不能依赖。
6. 删除设备误操作：第一版无 tombstone，可通过重新走当前设备 binding 流程恢复。

## Blockers / dependencies

1. 普通 request log redaction 不作为本 feature blocker；它保留为基础 auth 后续项。
2. 需要确认当前 auth repository / migration 是否承载 device 表，或新建 device repository；Plan 倾向新建 device application/repository，避免继续膨胀 auth service。
3. 若要让当前 serve 进程中的 Agent 立即以 device signature 连接 Cloud Gate，需要梳理 `runServe` 中 backend server 与 agent client 的 lifecycle；但这不是“模式切换”，也不得破坏 localhost 本地访问。
4. Cloud binding 需要 Cloud Gate 可用的外部 URL 与 Google/email 登录配置；自动化测试应使用 httptest/mock，不依赖真实 Google。

## User review notes

本 Plan 已吸收用户最新澄清：

1. `agent.listen_url` 是本地 Web 服务地址；`agent.connect_url` 是 Agent 要连接的 Gate 服务地址；二者相等表示本地模式，`agent.connect_url` 指向远程地址表示远程 Gate 模式。
2. 固定 Cloud Gate URL 配置名确定为 `cloud.gate_url`。
3. Cloud 授权完成后回调本机的地址基准配置为 `cloud.callback_base_url`，值示例为 `http://127.0.0.1:9030`。
4. setup token 只走 direct URL token；不实现终端 setup code、粘贴输入或 CLI fallback。授权本机访问的 setup token 必须由 Local Gate 生成，Cloud Gate 只展示本地连接 URL。
5. `local_access` JWT 第一版不引入 refresh token；TTL 沿用 `auth.jwt_ttl`，过期后重走 setup。
6. 本地 setup token / local cloud binding attempts / device credential 落在 `.termbridge` 状态目录。
7. device credential 定义为设备长期 key pair：private key 留在本机，Cloud Gate 保存 public key；Cloud Gate 不签发另一份长期 device token。
8. `/api/devices` 只是读取设备列表；所谓 registry 只是当前代码里的在线连接表，不引入额外用户概念。
9. 删除设备时如果在线就立即断开已有 Agent tunnel。
10. 不引入 `cloud profile` / `remote connection profile`；只有一份配置，serve 启动后打开 localhost 是本地访问，打开 Cloud URL 是云端访问，不存在“切换”模式。
