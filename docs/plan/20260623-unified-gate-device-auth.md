# 统一 Gate / Device / Auth 工作台计划
最后修改时间: 2026-06-23 13:26:28

Review status: Accepted

## Basis

- Requirement: `docs/requirement/20260623-unified-gate-device-auth.md`，Review status: Accepted
- Spec: `docs/spec/20260623-unified-gate-device-auth.md`，Review status: Accepted
- Design note: `docs/design/20260623-unified-gate-device-model.md`

本计划采用严格模式 / strict。本阶段只定义实施步骤和验证方式，不修改产品代码。

## Plan-stage decisions

用户在 Plan 阶段明确补充并覆盖早先草案中的兼容假设：

1. 先后端，再前端。
2. 一次性修改，不按“可保留旧主路径”的分阶段交付。
3. 不保留 localapi；Browser API 不再存在 local direct path。
4. offline readonly history 本阶段必做。
5. 不迁移旧 state / workspace / session 数据。
6. 不向后兼容旧 runtime state 目录结构。

这些决策意味着：

- 本计划仍按后端到前端的顺序组织实施步骤，但交付目标是一次性完成统一路径切换。
- localapi 不再作为兼容层存在；需要移除 HTTP server 中对 localapi 的挂载，并删除或停用 localapi transport package。
- 旧 state 不迁移、不兼容、不做 fallback 读取；新实现只认 device-scoped state。
- offline readonly history 需要通过 Gate 侧 snapshot/cache 能力支持，但该 cache 只作为只读历史快照，不成为 runtime authoritative state。

## Implementation strategy

该变更横跨配置、认证、Device 状态、Gateway/Agent tunnel、前端工作台。实施顺序必须先后端再前端，否则前端一旦切到 Gate / Device API 会丢失现有 mutation 能力。

实施顺序：

```text
1. 配置 / Auth / Device bootstrap
2. Device-scoped state store
3. 移除 localapi direct path
4. Gateway canonical API + complete mutation relay
5. Gate offline readonly history snapshot/cache
6. Frontend auth/device/session unification
7. Documentation and verification
```

实施原则：

- 交付上一次性完成统一路径，不保留 localapi 作为运行时兼容层。
- 内部实现仍按后端优先、前端随后切换的顺序推进，保证每一步有清晰验证点。
- 后端先补齐 Gate mutation，再切前端主路径。
- Browser-facing API 采用 canonical `/api/devices/:deviceId/...`。
- `/api/agent/tunnel` 可作为 Agent tunnel 内部传输入口继续存在；它不是 localapi，也不是前端产品路由。
- 不迁移旧 state 数据；新 store 不扫描旧 root workspace/session。
- offline readonly history 是必做项，但语义是 Gate 已缓存的只读 snapshot/stale history，不是 Gate 接管 runtime authoritative history。

## Implementation steps

### Step 1. 扩展配置模型并实现 bootstrap ensure

目标：`termbridge serve` 启动时能够通过 viper 读取 `.termbridge.yaml`，检查并补齐 auth/device 字段。

计划：

1. 在 `internal/infrastructure/config/config.go` 增加配置结构：
   - `AuthConfig`：`Username`、`Password`
   - `AgentConfig` 扩展：`DeviceID`、`DeviceName`，保留 `ServerURL`
2. 将以下 key 加入 allowed config keys：
   - `auth.username`
   - `auth.password`
   - `agent.device_id`
   - `agent.device_name`
   - 既有 `agent.server_url`
3. 新增 config bootstrap helper，负责：
   - 读取已加载配置。
   - 判断 auth/device 字段是否缺失。
   - 缺失时生成 username/password/device_id/device_name。
   - 写回 `.termbridge.yaml`。
   - 返回 bootstrap result：是否写入、是否首次生成 password、生成的 credential hint。
4. 生成规则：
   - username：当前 OS 用户名；失败时 fallback 为 `termbridge` 并记录 warning。
   - password：cryptographic random，至少 24 字符或等价 entropy。
   - device_id：stable random id，生成后持久化。
   - device_name：默认使用本机 hostname；不再追加 device id 片段。
5. `runServe` 启动前使用 bootstrap 后的 config。
6. 首次生成凭据时只打印一次 console hint；已有 password 不重复打印。
7. 失败策略：
   - 无法写入 `.termbridge.yaml`：fail fast。
   - random 生成失败：fail fast。
   - OS username/hostname 获取失败：允许 fallback + warning。

主要文件：

- `internal/infrastructure/config/config.go`
- `internal/infrastructure/config/*_test.go`
- 可能新增 `internal/infrastructure/config/bootstrap.go`
- `internal/app/app.go`

验收要点：

- 配置缺失时能写入 `.termbridge.yaml`。
- 再次启动不会重置 password/device_id。
- 控制台只在首次生成时打印 password。
- viper 最终读取到生成后的字段。

### Step 2. 重构 Device identity 生成与保存

目标：Device identity 从“state dir 中的辅助文件”提升为后端一等身份，并和 config bootstrap 统一。

计划：

1. 调整 `internal/application/agent/device.go`：
   - 优先使用 config 中的 `agent.device_id` / `agent.device_name`。
   - 若 config bootstrap 已保证存在，则 `LoadOrCreateDevice` 不应再次生成另一套身份。
   - device name 默认使用本机 hostname，不携带 device id 片段。
2. 明确 device identity 的事实来源：
   - `agent.device_id` / `agent.device_name` 在 `.termbridge.yaml` 中持久化。
   - device-scoped state 中的 `device.json` 作为 runtime metadata mirror 或 state metadata。
3. 扩展 `Device` 字段：
   - `id`
   - `name`
   - `hostname`
   - `created_at`
   - `updated_at`
4. 添加测试：
   - 首次生成。
   - 配置已有时稳定读取。
   - hostname fallback。
   - configured device name 覆盖默认 display name。

主要文件：

- `internal/application/agent/device.go`
- `internal/application/agent/device_test.go`
- `internal/infrastructure/config/config.go`

验收要点：

- Agent hello 使用稳定 device id/name。
- 本地多次重启 device id 不变。
- 自动 name 使用本机 hostname，不追加 device id 片段。

### Step 3. 设计并落地 device-scoped state store，不迁移旧数据

目标：runtime state 目录按 Device 组织，Device 成为 workspace/session 持久化数据的一部分。

计划：

1. 修改 `internal/infrastructure/repository/state/store.go` 的目录模型。
2. 推荐结构：

```text
<state_dir>/
  devices/
    <device_id>/
      device.json
      workspaces/
        <workspace_key>/
          workspace.json
          sessions/
            <session_id>/
              session.json
              history...
```

3. 选择实现方式之一：
   - Store 持有 `DeviceID`，所有 WorkspaceDir/SessionDir 自动落到 device root。
   - 或新增 `DeviceStore`，由 root store 创建 `ForDevice(deviceID)`。
4. 更新 registry construction，使 `newWebTerminalRegistry` 使用当前 local device 的 device-scoped store。
5. 不做旧数据迁移。
6. 不扫描旧 root workspace/session 数据。
7. 不提供旧 state fallback。
8. 增加路径安全测试：
   - device id 不能导致路径穿越。
   - workspace key/session id 仍不能逃逸 state root。

主要文件：

- `internal/infrastructure/repository/state/store.go`
- `internal/infrastructure/repository/state/*_test.go`
- `internal/app/app.go`
- 可能涉及 terminal runtime/repository 初始化文件

验收要点：

- 新 session/workspace 写入 `devices/<device_id>/...`。
- 旧 root 结构不被读取、不被迁移、不被兼容。
- 路径安全约束仍成立。

### Step 4. 移除 localapi direct path

目标：Browser API 不再绕过 Gate / Device 模型，后端不保留 localapi 作为兼容层。

计划：

1. 从 `internal/transport/http/server/server.go` 移除 localapi handler 挂载：
   - 删除 `/api` / `/api/` 指向 localapi 的路由。
   - 保留 Gateway / canonical API handler。
2. 调整 `internal/app/app.go`：
   - 不再构造 `localapi.New(...)`。
   - `httpserver.New` 不再接收 localHandler，或 localHandler 参数删除。
3. 删除或停用 `internal/transport/http/localapi`：
   - 如果 localapi 中存在仍需复用的 DTO/error helper，先迁移到 application/protocol 层。
   - 然后删除 localapi transport package 或确保它不再参与 build/runtime。
4. 将 localapi tests 改写为 canonical Gate/device API tests，或删除仅验证 local direct path 的测试。
5. 清理前端 local API client 的主路径依赖；最终前端不再调用 `/api/sessions` 或 `/api/workspaces` direct path。

主要文件：

- `internal/transport/http/server/server.go`
- `internal/app/app.go`
- `internal/transport/http/localapi/*`
- 相关 tests
- 需要承接 DTO 的 shared protocol/application 文件

验收要点：

- HTTP server 不再暴露 local direct session/workspace API。
- `/api/sessions`、`/api/workspaces` direct path 不再作为 Browser 工作台路径存在。
- localapi 不作为兼容层保留。
- 现有 localapi 业务能力已通过 Gate / Agent tunnel 补齐。

### Step 5. 将 gateway auth 从 hardcoded admin/admin 改为 config-driven

目标：Browser 和 Agent 都使用 `.termbridge.yaml` 中的 `auth.username` / `auth.password`。

计划：

1. 修改 `internal/transport/http/gatewayapi/auth/auth.go`：
   - 移除 hardcoded `Username` / `Password` 常量作为认证事实来源。
   - `Manager` 接收 credential validator 或 config credential。
   - 使用 constant-time password compare。
2. Gateway API 增加 canonical auth endpoints：
   - `POST /api/login`
   - `POST /api/logout`
   - `GET /api/me`
3. 如果保留 `/api/gateway/login` 等旧路径，只作为内部/过渡 alias；前端不使用。
4. `internal/application/agent/client.go` 移除 `basicAuthHeader("admin", "admin")`。
5. Agent tunnel dial 使用 config-driven credentials。
6. terminal WebSocket、devices API、device-scoped API 都经过同一 Browser auth middleware。
7. 测试覆盖：
   - 默认 hardcoded admin/admin 不再有效，除非配置确实如此。
   - 错误密码拒绝。
   - 登录后 cookie 可访问 protected API。
   - logout 后拒绝。
   - Agent tunnel auth 成功/失败。

主要文件：

- `internal/transport/http/gatewayapi/auth/auth.go`
- `internal/transport/http/gatewayapi/server.go`
- `internal/application/agent/client.go`
- `internal/app/app.go`
- 相关 auth/server/client tests

验收要点：

- Browser 登录使用配置凭据。
- Agent tunnel 使用配置凭据。
- 日志不打印 password。
- 未登录不能访问 devices/session/terminal WS。

### Step 6. 增加 canonical device-scoped Gateway routes

目标：新增 `/api/devices/:deviceId/...` 作为正式 Browser API。

计划：

1. 在 gateway handler routing 中接入 canonical routes：
   - `/api/login`
   - `/api/logout`
   - `/api/me`
   - `/api/devices`
   - `/api/devices/...`
2. Browser-facing 新代码只使用 canonical `/api/devices/...`。
3. Agent tunnel 内部入口可继续使用 `/api/agent/tunnel`，因为它不是 localapi，也不是 Browser 工作台 API。
4. 在 gateway handler 中抽取 device route dispatcher：
   - `handleDeviceWorkspaceTree`
   - `handleDeviceWorkspaceOrder`
   - `handleDeviceWorkspaceDelete`
   - `handleDeviceSessions`
   - `handleDeviceSession`
   - `handleDeviceSessionHistory`
   - `handleDeviceSessionWS`
5. 返回统一 error response shape。
6. 对 device offline、not found、agent timeout 做稳定 HTTP status 映射。

主要文件：

- `internal/transport/http/server/server.go`
- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/route.go`
- gatewayapi tests

验收要点：

- canonical `/api/devices` 返回设备列表。
- canonical `/api/devices/:deviceId/workspaces/tree` 可用。
- canonical mutation routes 可用。
- 错误响应稳定。

### Step 7. 扩展 Agent RuntimeAccess 到完整工作台控制面

目标：Gate path 覆盖 localapi 原有 mutation 能力，确保删除 localapi 后不退化。

计划：

1. 扩展 `internal/application/agent/runtime_access.go`：
   - workspace tree
   - workspace reorder
   - delete workspace
   - list sessions
   - create session
   - get session
   - update session
   - close session
   - delete session
   - read history
   - attach terminal
2. 在 `agent.WebTerminalAccess` adapter 中调用现有 terminal application/registry 能力。
3. 将 localapi 原先使用的 request/response DTO 迁移到 shared application/protocol 层，避免 gateway 或 agent 继续依赖 localapi package。
4. 如现有 application 层缺少某些窄接口，先在 application 层补统一方法，不在 gateway handler 中绕过。
5. 添加单元测试覆盖 adapter 到 registry/application 的方法映射。

主要文件：

- `internal/application/agent/runtime_access.go`
- `internal/application/agent/client.go`
- `internal/app/app.go`
- terminal application/registry 相关文件
- shared protocol/application DTO 文件

验收要点：

- Agent 端支持所有 Spec 列出的 tunnel methods。
- Gateway 不直接 import PTY/process runtime 实现。
- mutation 行为与 localapi 原能力保持一致。

### Step 8. 扩展 tunnel request/response method 和错误映射

目标：Gate 能通过 tunnel 调用 Agent 的完整控制面，并得到可映射错误。

计划：

1. 在 `internal/application/agent/client.go` 的 request switch 中新增：
   - `workspace_reorder`
   - `workspace_delete`
   - `session_create`
   - `session_get`
   - `session_update`
   - `session_close`
   - `session_delete`
2. 在 `internal/transport/http/gatewayapi/route.go` 中确保 request timeout、response waiting、stream cleanup 对 mutation 同样可靠。
3. 定义 structured tunnel error：

```json
{
  "code": "session_not_found",
  "message": "session not found"
}
```

4. 如果现有 tunnel response payload 无法兼容 structured error，则 bump `ProtocolVersion` 并在 hello/hello_ack 中拒绝不兼容版本。
5. Gateway 将 tunnel error 映射到 HTTP status。
6. 添加测试：
   - mutation success。
   - validation error。
   - not found。
   - conflict active session。
   - agent timeout。
   - offline route。

主要文件：

- `internal/application/agent/client.go`
- `internal/protocol/tunnel/frame.go`
- `internal/transport/http/gatewayapi/route.go`
- related tests

验收要点：

- 所有 mutation 方法可通过 tunnel 调用。
- 错误不会只以裸字符串泄漏到 Browser API。
- timeout 后 pending map 清理。

### Step 9. 实现 Gate offline readonly history snapshot/cache

目标：设备离线时，前端仍能查看 Gate 已缓存的只读历史快照。

计划：

1. 在 Gateway 侧新增 read-only snapshot/cache store，用于保存每个 device/session 的最后可用历史快照和必要 session metadata。
2. 推荐结构：

```text
<state_dir>/
  gateway-cache/
    devices/
      <device_id>/
        sessions/
          <session_id>/
            session.json
            history.snapshot
            snapshot.json
```

3. Cache 更新时机：
   - 在线调用 `GET /api/devices/:deviceId/sessions/:sessionId/history` 成功后，写入 history snapshot。
   - terminal relay 关闭或 session close 成功后，如果 device 仍在线，主动刷新一次 history snapshot。
   - workspace/session list 成功后，保存浅 metadata snapshot。
4. 设备离线时：
   - `GET /api/devices/:deviceId/sessions` 可返回 cached session metadata，并标记 stale/offline。
   - `GET /api/devices/:deviceId/sessions/:sessionId/history` 返回 cached history snapshot。
   - mutation 和 terminal attach 返回 device offline，不允许写操作。
5. 如果某个 session 没有 cached history：
   - 返回明确 `history_snapshot_unavailable`，前端显示“无离线历史快照”。
6. Snapshot 不作为 runtime authoritative state：
   - 不用于恢复 session lifecycle。
   - 不用于创建/更新/删除 runtime 数据。
   - UI 必须标注 stale/offline。
7. 添加测试：
   - 在线 history 成功后写 cache。
   - device offline 后可读 cached history。
   - no snapshot 时返回明确错误。
   - offline mutation/attach 被拒绝。
   - cache path 不能被 device/session id 路径穿越。

主要文件：

- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/route.go`
- 可能新增 `internal/transport/http/gatewayapi/cache.go`
- gateway cache tests
- 前端 history/offline 状态相关文件

验收要点：

- 设备离线后可以查看已缓存的只读历史。
- UI 明确 stale/offline，不误导用户这是实时数据。
- 无快照时有清晰空状态。
- 离线状态不允许 mutation 或 terminal attach。

### Step 10. 前端建立 canonical Gate/Device API client 和 Pinia 状态边界

目标：前端数据加载围绕 selected device，而不是 local backend / gateway backend 产品分支。

计划：

1. 新增或重构 API client：
   - `web/src/features/gate/api.ts` 或 `web/src/features/devices/api.ts`
   - `getMe()` / `login()` / `logout()`
   - `listDevices()`
   - `listDeviceWorkspaceTree(deviceId)`
   - `reorderDeviceWorkspaces(deviceId, workspaceIds)`
   - `deleteDeviceWorkspace(deviceId, workspaceId)`
   - `listDeviceSessions(deviceId)`
   - `createDeviceSession(deviceId, request)`
   - `getDeviceSession(deviceId, sessionId)`
   - `updateDeviceSession(deviceId, sessionId, request)`
   - `closeDeviceSession(deviceId, sessionId)`
   - `deleteDeviceSession(deviceId, sessionId)`
   - `readDeviceHistory(deviceId, sessionId)`
   - `deviceTerminalWsUrl(deviceId, sessionId)`
2. Pinia store 建议：
   - `auth`：checking/me/login/logout/auth error。
   - `devices`：devices list、selected device id、loading/error/empty/offline。
   - 可选 `workbench`：workspace tree/session loading/error。
3. 不将 terminal socket/xterm 实例状态迁入 Pinia。
4. 删除或停用 local direct API client 的主路径使用；前端不再调用 `/api/sessions` 或 `/api/workspaces`。
5. 支持 offline readonly history：
   - device offline 时允许读取 cached history。
   - 明确显示 stale/offline snapshot 状态。
   - 禁用 create/update/delete/close/reorder/terminal attach。

主要文件：

- `web/src/features/gateway/api.ts` 或新增 `web/src/features/devices/api.ts`
- `web/src/features/sessions/api.ts`
- `web/src/features/workspaces/api.ts`
- `web/src/stores/auth.ts`
- `web/src/stores/devices.ts`
- 可能新增 `web/src/stores/workbench.ts`

验收要点：

- 新 client 只调用 canonical `/api/devices/...` 和 canonical auth API。
- Store 状态边界清晰，不出现无法解释的 loading boolean 组合。
- TypeScript 类型覆盖 Device/Auth/Error/Offline snapshot DTO。

### Step 11. 前端移除 `/gateway` 产品路由并改造 `/sessions`

目标：用户只通过登录/设备选择进入 `/sessions`，工作台布局保持基本不变。

计划：

1. 修改 `web/src/router/index.ts`：
   - 删除 `/gateway` route，或直接 redirect 到 `/sessions`，但不再承载独立 gateway 工作台。
   - `/` 根据 auth 状态显示登录或跳转 `/sessions`。
2. 修改 `web/src/views/SessionsView.vue`：
   - 删除 `gatewayRoute` prop。
   - 删除 local/gateway product-mode 分支。
   - 使用 selected device 生成 workspace/session/history/terminal API。
   - 保留现有 tab、dialogs、terminal、history、toast 交互。
3. 增加登录/设备选择视图：
   - 可作为 `HomeView` / `LoginView` / `DeviceSelectionView`。
   - 或在 `SessionsView` 前置 guard 中显示，但不能让未登录用户看到完整工作台。
4. 设备选择完成后，进入 `/sessions` 并加载工作台数据。
5. `WorkspaceSessionSidebar.vue` 左面板底部展示当前设备名。
6. 设备 offline 时：
   - 禁用 mutation 和 terminal attach。
   - 允许查看已缓存的 readonly history snapshot。
   - 无 snapshot 时显示明确空状态。
   - 不进入空白崩溃。

主要文件：

- `web/src/router/index.ts`
- `web/src/App.vue`
- `web/src/views/SessionsView.vue`
- 可能新增 `web/src/views/LoginView.vue`
- 可能新增 `web/src/views/DeviceSelectionView.vue`
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
- `web/src/i18n.ts`

验收要点：

- 访问 `/gateway` 不再出现独立 gateway 工作台。
- 未登录访问 `/sessions` 不展示工作台。
- 登录后必须选择设备。
- 选择设备后工作台操作方式基本不变。
- 左面板底部显示设备名。
- 设备离线时可读 cached history snapshot，不能执行写操作。

### Step 12. 文档更新

目标：记录一次性切换后的新边界。

计划：

1. 文档记录：
   - `/api/devices/...` 为 canonical Browser API。
   - localapi 已移除，不存在 local direct Browser path。
   - 旧 state 不迁移、不兼容。
   - offline readonly history 为 Gate cached snapshot，不是 runtime authoritative state。
2. README 或使用说明补充首次启动凭据提示。
3. Verification 阶段创建 `docs/verification/20260623-unified-gate-device-auth.md`。

主要文件：

- `docs/verification/20260623-unified-gate-device-auth.md`（Verification 阶段）
- 可能涉及 README / docs 使用说明

验收要点：

- 用户知道首次启动 credential 如何获取。
- 用户知道旧 state 不迁移。
- 用户知道离线历史是 stale snapshot。

## Files to change

### Backend

预计修改：

- `internal/app/app.go`
- `internal/infrastructure/config/config.go`
- `internal/infrastructure/config/*_test.go`
- `internal/application/agent/device.go`
- `internal/application/agent/device_test.go`
- `internal/application/agent/client.go`
- `internal/application/agent/runtime_access.go`
- `internal/protocol/tunnel/frame.go`
- `internal/transport/http/server/server.go`
- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/route.go`
- `internal/transport/http/gatewayapi/auth/auth.go`
- `internal/infrastructure/repository/state/store.go`
- state / terminal registry / application 层相关 tests

预计删除或停用：

- `internal/transport/http/localapi/*`
- localapi direct path tests

可能新增：

- `internal/infrastructure/config/bootstrap.go`
- `internal/infrastructure/config/bootstrap_test.go`
- `internal/transport/http/gatewayapi/cache.go`
- gateway canonical route tests
- gateway offline history cache tests
- tunnel mutation tests
- shared protocol/application DTO 文件

### Frontend

预计修改：

- `web/src/router/index.ts`
- `web/src/App.vue`
- `web/src/views/SessionsView.vue`
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
- `web/src/features/gateway/api.ts`
- `web/src/features/sessions/api.ts`
- `web/src/features/workspaces/api.ts`
- `web/src/i18n.ts`

可能新增：

- `web/src/features/devices/api.ts`
- `web/src/features/auth/api.ts`
- `web/src/stores/auth.ts`
- `web/src/stores/devices.ts`
- `web/src/stores/workbench.ts`
- `web/src/views/LoginView.vue`
- `web/src/views/DeviceSelectionView.vue`

### Docs

预计新增或更新：

- `docs/verification/20260623-unified-gate-device-auth.md`（Verification 阶段）
- 可能更新 README 或 docs 使用说明，记录首次凭据生成、旧 state 不兼容、offline history snapshot 语义。

## Verification plan

Verification 阶段再执行具体命令并记录结果。本计划建议优先使用项目显式入口，不修改 `justfile` 做一次性验证。

### Backend verification

建议命令：

```text
go test ./...
```

重点测试覆盖：

1. config bootstrap：
   - 缺失 auth/device 时生成并写回。
   - 已存在配置不重置。
   - 生成失败 fail fast。
   - 首次生成提示不重复。
2. device identity：
   - device id/name 稳定。
   - hostname 默认名，不追加 device id 片段。
   - configured name 覆盖。
3. state store：
   - 新目录结构 `devices/<device_id>/...`。
   - 路径安全。
   - 旧 root state 不被新 store 当成当前数据。
   - 不做迁移、不做 fallback。
4. localapi removal：
   - HTTP server 不再挂载 localapi。
   - local direct `/api/sessions` / `/api/workspaces` 不再作为工作台 API。
   - localapi package 不再参与 runtime。
5. auth：
   - Browser login/logout/me。
   - cookie protected endpoints。
   - Agent tunnel credential。
   - hardcoded admin/admin 不再作为默认事实来源。
6. gateway canonical API：
   - `/api/devices`。
   - `/api/devices/:deviceId/workspaces/tree`。
   - `/api/devices/:deviceId/sessions`。
   - mutation success/error。
7. tunnel：
   - request/response。
   - mutation methods。
   - structured error。
   - timeout cleanup。
8. offline readonly history：
   - online history response writes snapshot。
   - offline device can read cached history。
   - no snapshot returns clear error。
   - offline mutation/attach rejected。
9. terminal WS：
   - canonical device path attach。
   - auth required。
   - device offline handling。

### Frontend verification

建议命令：

```text
yarn --cwd web typecheck
yarn --cwd web build
yarn --cwd web lint
```

可选：

```text
yarn --cwd web test
```

重点验证：

1. `/gateway` 不再作为独立产品路由。
2. 未登录访问 `/sessions` 不展示工作台。
3. 登录失败有明确错误。
4. 登录成功后加载设备列表。
5. 无设备/设备离线有明确状态。
6. 选择设备后加载 workspace/session。
7. 创建/重命名/停止/删除 session 仍可用。
8. 删除 workspace / workspace reorder 仍可用。
9. terminal attach/history 仍可用。
10. 左面板底部显示当前 device name。
11. 设备离线时能查看 cached readonly history snapshot。
12. 设备离线时写操作和 terminal attach 被禁用或返回明确错误。

### Manual / integration verification

建议人工或端到端验证：

1. 删除或临时移动 `.termbridge.yaml` 后启动 `termbridge serve`：
   - 生成 auth/device 字段。
   - 控制台打印首次凭据。
   - 不重复打印已存在密码。
2. 本地 self-connected：
   - `agent.server_url` 指向本机 Gate。
   - 前端登录。
   - 看到本机 device。
   - 进入 `/sessions`。
   - 创建 session 并 attach terminal。
3. localapi removal：
   - 旧 direct local API 不再作为工作台入口可用。
   - 前端没有请求 `/api/workspaces` 或 `/api/sessions` direct path。
4. 远程 Gate 模拟：
   - Agent 指向另一个 Gate URL。
   - Browser 登录 Gate。
   - 选择远程上报 device。
5. Agent 未连接或断开：
   - device offline 状态清晰。
   - terminal attach 不崩溃。
   - cached history 可读。
   - no snapshot 状态清晰。
6. 旧 state 不兼容：
   - 新结构不读取旧 root workspace。
   - 不迁移旧数据。
   - 交付说明明确。

## Blockers

当前无必须阻塞 Plan 的用户决策。

实现前需要注意的技术前置：

1. 必须先补齐 Gate mutation，再删除 localapi runtime path，否则会丢工作台能力。
2. 必须先迁移 localapi 中可复用 DTO/helper，再删除 localapi package。
3. offline readonly history 是必做项，需要 Gate-side snapshot/cache；不能只靠 Agent 在线读取。
4. 如果 tunnel response 结构无法兼容 structured error，需决定是否 bump protocol version；默认实现时根据代码现实判断。

## Assumptions

1. `termbridge serve` 继续作为本地主要入口。
2. 本地 self-connected Gate 是 first-class 使用场景。
3. `.termbridge.yaml` 可作为本阶段本地明文 secret 文件。
4. 本阶段不迁移旧 state 数据。
5. 本阶段不兼容旧 runtime state 目录结构。
6. 本阶段不保留 localapi 作为兼容层。
7. 设备管理 UI 不在本阶段展开。
8. 权限系统、OAuth、SSO、多租户隔离不在本阶段实现。
9. 前端主工作台布局不重做。
10. offline readonly history 的范围是 Gate 已缓存的 stale snapshot；没有 snapshot 时显示明确不可用状态。

## Risks

1. **一次性修改范围大**：用户明确要求一次性修改；风险是 review 和回归面较大。缓解方式是在实现内部按后端到前端顺序推进，并保持每步测试。
2. **删除 localapi 后功能退化**：如果 Gate mutation 未完全覆盖原 localapi，`/sessions` 会丢失创建/重命名/删除/停止/排序等能力。
3. **offline history 语义误导**：Gate cache 是 stale snapshot，不是实时 runtime history；UI 必须明确标注。
4. **认证绕过风险转移**：删除 localapi 能减少 direct path 绕过，但 canonical API 和 terminal WS 必须全部纳入 auth。
5. **明文密码**：`.termbridge.yaml` 中保存 password，有误提交和本机权限风险。必须减少日志暴露并提示用户保密。
6. **state 不兼容**：旧 workspace/session 数据不可见可能被用户误解为数据丢失。需要在交付说明中明确“不迁移、不兼容”。
7. **启动时序**：HTTP server ready 不等于 device ready；前端必须能等待 Agent 注册。
8. **过度前端重构**：不要把 terminal socket/xterm 局部状态强行迁入 Pinia，否则可能引入 race 和终端回归。

## Rollback plan

如果实施过程中出现严重回归：

1. 开发期间可以在未提交前回退具体文件修改，但不得未经用户授权执行 `git reset`、`git checkout`、`git stash` 等 git 写操作。
2. 因用户明确要求不保留 localapi，交付方案不包含“恢复 localapi 作为兼容层”的正式 rollback。
3. 如果 Gate mutation 未能一次性补齐，应停止实现并报告 blocker，而不是保留 localapi 悄悄兜底。
4. 如果 offline history cache 无法安全完成，应停止并更新 Plan/Spec；不能将其降级为后续项。
5. state store 改动如影响 runtime，可在开发中回退代码；交付语义仍是不迁移旧数据。

## User review notes

本 Plan 已按用户最新反馈更新：

1. 先后端再前端。
2. 一次性修改。
3. 不保留 localapi。
4. offline readonly history 本阶段必做。
5. 不迁移旧数据。
6. 不向后兼容旧 runtime state。

暂无必须由用户补充决策才能进入 Implementation / 实现的事项。若该计划被接受，下一阶段应从后端配置/Auth/Device bootstrap 开始实现，并在同一轮实现链路中继续完成 Gate mutation、offline history、localapi removal 和前端切换。