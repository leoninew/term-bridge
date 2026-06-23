# 统一 Gate / Device / Auth 工作台验证
最后修改时间: 2026-06-23 18:42:25

Review status: Accepted

## Verification scope

本验证对应严格模式 / strict 的 Verification / 验证阶段。

依据文档：

- Requirement: `docs/requirement/20260623-unified-gate-device-auth.md`
- Spec: `docs/spec/20260623-unified-gate-device-auth.md`
- Plan: `docs/plan/20260623-unified-gate-device-auth.md`
- Design note: `docs/design/20260623-unified-gate-device-model.md`

本次更新在既有 unified Gate / Device / Auth 验证基础上，补充验证后续 UI / UX 与配置收口调整：

- 登录后不再出现独立设备选择页，直接进入 `/sessions` 工作台壳。
- 设备选择移动到 `/sessions` 左侧栏顶部，使用 Reka UI `Select`。
- 只有一个设备时自动选中；多个设备未选择时保持工作台壳可见，但不刷新 workspace，也不允许 mutation。
- Select option / trigger 只展示 `device.name`，不显示“在线 / 离线”，也不提供退出按钮。
- Select 与搜索框在同一行，按 4:6 布局。
- 左侧栏底部“设置”按钮靠左。
- 默认 `device_name` 使用 hostname，不再追加 `device_id` 前缀或短 id 后缀；旧的自动生成名会被归一回 hostname。
- 新建会话普通入口默认目录为 `~`，workspace/目录加号入口仍使用对应 workspace path。
- 新建会话表单按“目录 / 名称 / 命令”顺序展示。
- 新建会话默认名称为“默认 / Default”，默认命令按浏览器 user agent 简单推断：Windows → `cmd`，macOS → `zsh`，Linux → `bash`，未知 → `bash`。
- 登录 i18n 文案保持泛化，不继续绑定 Gate / Device 流程细节。

验证过程中仅更新本文档；不继续修改产品代码。

## Requirement alignment

### 前端验收

| Requirement acceptance | 结果 | 说明 |
|---|---|---|
| `/gateway` 不再作为长期产品路由暴露 | 通过 | `/gateway` 路由重定向到 `/sessions`。 |
| `/sessions` 是统一会话工作台入口 | 通过 | `SessionsView.vue` 承载登录、设备选择、workspace/session、terminal/history。 |
| 页面打开后先完成登录检查 | 通过 | auth 初始化完成前显示检查状态；未登录时显示登录表单。 |
| 登录后不再展示独立设备选择页 | 通过 | 登录成功后进入 `/sessions` 工作台壳；设备选择位于左侧栏顶部。 |
| 只有一个设备时默认选中 | 通过 | `loadDevices()` 在设备数为 1 时设置 `selectedDeviceId`。 |
| 多设备且未选中时不刷新 workspace，不允许 mutation | 通过 | `refresh()` 在无 `selectedDeviceId` 时直接返回；`allowMutations` 要求存在 selected device 且在线非只读。 |
| 选择设备后展示现有 `/sessions` 工作台数据 | 通过 | selected device 驱动 workspace/session/history/terminal API。 |
| `/sessions` 主体布局和操作方式基本不变 | 通过 | 保留 sidebar、tabs、dialogs、terminal/history 组件和主要交互。 |
| 设备选择使用 Reka Select，并放在搜索左侧 | 通过 | `WorkspaceSessionSidebar.vue` 使用 `SelectRoot`；设备 Select 与搜索输入同处 `grid-cols-[4fr_6fr]`。 |
| 设备 Select 只展示设备名称 | 通过 | option 的 `SelectItemText` 与 `text-value` 均使用 `device.name`；未展示 online/offline。 |
| 设备选择区域不提供退出按钮 | 通过 | sidebar header 只有设备 Select、搜索和新建按钮；无 logout 控件。 |
| 左侧栏底部设置按钮靠左 | 通过 | footer 使用普通 `flex` 布局，未右对齐。 |
| 新建会话普通入口目录默认 `~` | 通过 | `resetCreateSessionForm()` 在无 workspace 时设置 `cwd = '~'`。 |
| workspace/目录加号入口沿用 workspace path | 通过 | `resetCreateSessionForm(workspace)` 优先使用 `workspace?.path`。 |
| 新建会话表单按“目录 / 名称 / 命令”顺序展示 | 通过 | 表单 template 中 cwd、name、command 依次排列。 |
| 新建会话默认名称稳定为“默认 / Default” | 通过 | `dialog.defaultSessionName` 用于重置 session name，移除了 command 改名联动。 |
| 默认命令按浏览器 user agent 简单推断 | 通过 | `defaultCommand()` 实现 Windows/macOS/Linux/fallback 规则。 |
| i18n 文案不过度绑定 Gate / Device 流程 | 通过 | 登录标题与描述使用泛化“登录 / Sign in”与“工作台 / workbench”。 |

### 后端验收

| Requirement acceptance | 结果 | 说明 |
|---|---|---|
| 后端存在 Device 概念 | 通过 | Agent device identity 由配置驱动，并通过 tunnel hello 上报。 |
| `serve` 启动时通过 viper 检查 `.termbridge.yaml` 中的设备名、用户账号和 Gate 凭据 | 通过 | `config.Load` 读取 `auth.*` 与 `agent.device_*`；`serve` 路径调用 ensure。 |
| 配置缺失时自动生成并写入 `.termbridge.yaml` | 通过 | `EnsureLocalIdentity` 补齐 auth/device 字段并回写。 |
| 默认设备名称不再携带 device id | 通过 | `defaultDeviceName()` 返回 hostname 或 `termbridge-device` fallback，不再拼接短 device id。 |
| 旧自动生成设备名可归一 | 通过 | `legacyDefaultDeviceName()` 识别 `<hostname>-<device_id[:8]>` 并重写为 hostname；测试覆盖。 |
| 自定义设备名不被破坏 | 通过 | 仅空值或 legacy default pattern 会被重写；普通自定义名保留。 |
| Agent 连接 Gate 时上报设备信息 | 通过 | hello payload 包含 device id/name/protocol version。 |
| Gate 能够列出可用设备 | 通过 | canonical `/api/devices` 受 auth 保护并返回设备列表。 |
| 本地 self-connected Gate 流程可用 | 部分验证 | 代码路径为 `termbridge serve` 同时启动 Gateway 和 Agent connector；自动化测试覆盖 tunnel/terminal e2e，但本轮未做浏览器人工 self-connected 验证。 |

### 认证验收

| Requirement acceptance | 结果 | 说明 |
|---|---|---|
| Browser 访问工作台前需要登录 | 通过 | `/api/devices` 和 device-scoped API 受 cookie auth middleware 保护。 |
| 登录使用 `.termbridge.yaml` 中的用户账号 | 通过 | gateway auth manager 使用 config-driven credentials。 |
| 缺失密码随机生成并首次打印提示 | 通过 | bootstrap result 在首次生成时输出 username/password。 |
| Agent 和 Browser 暂时使用同一套凭据 | 通过 | Agent tunnel Basic auth 与 Browser login 共用 `auth.username/password`。 |
| 认证失败或设备凭据无效有明确反馈 | 部分通过 | 失败会返回 401/错误文本；尚未完全实现 Spec 中建议的稳定 structured error shape。 |

### 兼容与迁移验收

| Requirement acceptance | 结果 | 说明 |
|---|---|---|
| `/sessions` 核心能力不因登录/设备选择退化 | 自动化通过，需人工回归 | 编译、lint、单测、后端测试通过；未做真实浏览器端创建/attach 人工回归。 |
| 后端 localapi 可以暂时保留 | 按 Plan 覆盖为移除 | Plan 阶段用户明确要求“不保留 localapi”，实现已删除 localapi package。 |
| Device 成为 workspace/session 持久化数据的一部分 | 通过于 serve 路径 | `serve` runtime registry 使用 `state.NewDeviceStore`。非 serve CLI 仍使用 base store layout，见风险。 |
| runtime state 目录结构按 Device 重新设计 | 通过于 serve 路径 | serve path 使用 `<state_dir>/devices/<device_id>/workspaces/.../sessions/...`。 |
| 不向后兼容旧 state 目录结构或旧数据 | 通过 | 不迁移旧 state，不做 fallback。 |

## Spec alignment

| Spec decision / interface | 结果 | 说明 |
|---|---|---|
| `/sessions` is the only workbench route | 通过 | `/gateway` 重定向，不再承载独立 workbench。 |
| Login required before workbench | 通过 | 未登录时显示登录表单；已登录后进入 `/sessions` 工作台壳。 |
| Device selection required before device-scoped data access | 通过 | 无 selected device 时不刷新 workspace，不允许 mutations/terminal attach。 |
| Device selection UI should not be a separate post-login page | 通过 | 当前设备选择在左侧栏顶部 Select 内完成。 |
| Device-scoped API 使用 path scope | 通过 | 前后端使用 canonical `/api/devices/:deviceId/...`。 |
| Gate mutation coverage matches former localapi capabilities | 通过 | create/get/update/delete/close session、workspace tree/order/delete、history、terminal attach 已通过 Gateway/Agent relay 覆盖。 |
| Agent RuntimeAccess expands to full control surface | 通过 | `RuntimeAccess` 扩展到 workspace/session mutation、history、attach。 |
| Device identity and persistence first-class | 通过 | device id/name 由 config 持久化，device metadata 写入 device-scoped state；默认 name 已收敛为 hostname-only。 |
| Bootstrap writes missing credentials and viper reads them | 通过 | `auth.username/password` 与 `agent.device_id/name` 被读取和生成。 |
| Browser auth and Agent auth share credentials | 通过 | config-driven credentials 替代 hardcoded auth。 |
| Gate registry remains routing state, not runtime owner | 通过 | Gateway relay 到 Agent/Runtime，未接管 PTY/process lifecycle。 |
| Offline readonly history | 按当前决策通过 | Gateway 内存 cache 支持在线读过后的离线只读 history/workspace tree snapshot；持久化 Gate snapshot 已明确不做，Gate 重启后 cache 丢失。 |
| Structured error shape | 未完全完成 | 当前多处仍用 `http.Error` 返回文本，尚未统一为 Spec 建议的 `{ error: { code, message } }`。 |

## Plan alignment

| Plan step | 结果 | 说明 |
|---|---|---|
| Step 1. 配置 / Auth / Device bootstrap | 通过 | 新增 auth/device config 与 ensure。 |
| Step 2. Device identity 生成与保存 | 通过 | config 为事实来源；default device name 为 hostname-only；legacy default name 会被重写。 |
| Step 3. device-scoped state store，不迁移旧数据 | 通过于 serve 路径 | `NewDeviceStore` 用于 serve runtime registry；旧数据不迁移。 |
| Step 4. 移除 localapi direct path | 通过 | 删除 `internal/transport/http/localapi/*`，HTTP server 只挂载统一 API handler。 |
| Step 5. gateway auth config-driven | 通过 | auth manager 使用配置凭据，agent tunnel 使用 Basic auth。 |
| Step 6. canonical device-scoped Gateway routes | 通过 | `/api/login/logout/me/devices/devices/:id/...` 已接入。 |
| Step 7. RuntimeAccess 完整控制面 | 通过 | Agent runtime access 和 WebTerminalAccess adapter 已扩展。 |
| Step 8. tunnel request/response method 和错误映射 | 部分通过 | mutation methods 已补齐；method 名称按实现采用 `create_session`、`workspace_order` 等；structured error/status mapping 尚未完全稳定化。 |
| Step 9. Gate offline readonly history snapshot/cache | 按当前决策通过 | 已实现进程内 cache；持久化 `gateway-cache` 已明确不做。 |
| Step 10. 前端 canonical Gate/Device API client 和 Pinia 状态边界 | 部分通过 | API client 已切到 canonical path；Pinia/router 基础已接入；auth/device/workbench 状态仍在 `SessionsView.vue` 局部 refs 中。 |
| Step 11. 移除 `/gateway` 产品路由并改造 `/sessions` | 通过 | `/gateway` redirect；`SessionsView` 使用 selected device；设备选择已整合到 sidebar。 |
| Step 12. 文档更新 | 通过 | README 和设计/需求/规格/计划/验证文档记录新边界；本次更新补充后续 UI 与默认值调整验证。 |

## Actual diff summary

### Backend

主要变更：

- `internal/app/app.go`
  - `serve` 路径调用本地身份 bootstrap。
  - 不再构造 localapi。
  - Gateway API 和 Agent client 使用同一 auth/device config。
  - serve runtime registry 使用 device-scoped state store。

- `internal/infrastructure/config/config.go`
  - 增加 `AuthConfig`、扩展 `AgentConfig`。
  - 增加 `.termbridge.yaml` 身份生成/写回逻辑。
  - 增加 allowed keys 和相关校验。
  - 默认 `device_name` 改为 hostname-only；旧自动生成 `<hostname>-<short-device-id>` 名称会被归一。

- `internal/infrastructure/config/config_test.go`
  - 覆盖 hostname-only default device name。
  - 覆盖 legacy default device name rewrite。

- `internal/application/agent/*`
  - Device identity 改为 config-driven。
  - Agent tunnel 使用配置凭据。
  - RuntimeAccess 扩展到完整工作台控制面。

- `internal/transport/http/gatewayapi/*`
  - 增加 canonical auth/device-scoped Browser API。
  - 补齐 workspace/session mutation relay。
  - terminal WS 改为 canonical device path。
  - 增加离线只读 cache 逻辑。

- `internal/transport/http/server/server.go`
  - HTTP server 只接收统一 API handler。
  - 移除 localapi handler 参数。
  - logger nil 改为 panic。

- `internal/transport/http/localapi/*`
  - 删除 localapi transport package。

- `internal/infrastructure/repository/state/store.go`
  - 新增 base workspace root 与 device-scoped store。

- 多个 domain/protocol/application 文件
  - 按用户要求统一 `Id` / `Url` 命名，保留协议 JSON/header 字符串。

### Frontend

主要变更：

- `web/package.json` / `web/yarn.lock`
  - 引入 `vue-router` 与 `pinia`。

- `web/src/main.ts`
  - 注册 Pinia、router 和 i18n。

- `web/src/App.vue`
  - 变为薄路由壳，只渲染 `RouterView`。

- `web/src/router/index.ts`
  - `/` redirect 到 `/sessions`。
  - `/sessions` 为统一工作台。
  - `/settings`、`/help` 为后续页面入口。
  - `/gateway` redirect 到 `/sessions`。

- `web/src/views/SessionsView.vue`
  - 移除 local/gateway product mode。
  - 增加 auth check、login、device loading、selected device 状态。
  - 登录后直接进入工作台壳，设备选择由 sidebar 负责。
  - 单设备自动选中；无 selected device 时 `refresh()` no-op。
  - 使用 selected device 调用 canonical API。
  - 离线时禁用 mutation/terminal attach，可读 cached history。
  - 新建会话普通入口默认 cwd 为 `~`；workspace 入口保留 workspace path。
  - 新建会话表单顺序为 cwd/name/command。
  - 默认 session name 使用 `dialog.defaultSessionName`。
  - 默认 command 根据 user agent 简单推断，且移除 command 改动时自动改名联动。

- `web/src/views/SettingsView.vue` / `web/src/views/HelpView.vue`
  - 提供 `/settings` 空白容器和 `/help` 轻量占位。

- `web/src/features/gateway/api.ts`
  - canonical auth/device API client。

- `web/src/features/sessions/api.ts`
  - session read/update/delete/close/history/ws API 改为 `/api/devices/:deviceId/workspaces/:workspaceId/sessions...`。
  - workspace-scoped create 由路径 workspace id 注入 `workspace_id`；普通 create-by-cwd 入口保留 `POST /api/devices/:deviceId/sessions`。
  - history 返回 offline marker。

- `web/src/features/workspaces/api.ts`
  - workspace API 改为 `/api/devices/:deviceId/workspaces...`。

- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
  - sidebar header 使用 Reka UI Select 选择设备。
  - 设备 Select 位于搜索框左侧，4:6 布局。
  - Select 只展示 `device.name`，不显示 online/offline。
  - 无 selected device 时显示设备选择提示。
  - footer 只保留靠左设置菜单，不再展示 device name。

- `web/src/i18n.ts`
  - 更新登录、设备选择、离线只读文案。
  - 登录文案保持泛化。
  - 新建会话字段改为“目录 / 名称 / 命令”。
  - 新增默认会话名“默认 / Default”。

### Docs

新增或更新：

- `docs/design/20260623-unified-gate-device-model.md`
- `docs/requirement/20260623-unified-gate-device-auth.md`
- `docs/spec/20260623-unified-gate-device-auth.md`
- `docs/plan/20260623-unified-gate-device-auth.md`
- `docs/verification/20260623-unified-gate-device-auth.md`
- `README.md`

## Expected vs actual changed files

### 符合预期的变更范围

- Backend app/config/agent/gateway/state/protocol/terminal registry 相关文件：符合 Plan 的后端优先实施范围。
- `internal/transport/http/localapi/*` 删除：符合 Plan 阶段用户明确“不保留 localapi”。
- Frontend router、SessionsView、API client、sidebar、i18n：符合前端统一路径范围。
- README 和 SpecFlow 文档：符合文档更新范围。

### 超出或偏离预期但有原因的变更

- 大量 `Id` / `Url` / `Pid` / `OwnerPid` 命名变更：来自用户实现阶段明确要求，属于全局一致性调整。
- 删除 localapi 比 Requirement 初稿更激进，但符合 Plan 阶段用户明确覆盖决策。
- 前端 device selector 从独立 post-login 页面移动到 `/sessions` sidebar：来自后续 UX 决策，避免登录后仍被设备选择页阻断。
- 默认 device name 从 `<hostname>-<short-device-id>` 改为 hostname-only：来自后续 UX 决策，避免 name 与 id 重复表达。
- 后续 session contract 清理已删除 `CreateSessionResponse.ws_url`、session response `log_path` 和 workspace `workspace_key` 输出；当前前端基于 selected device、workspace id、session id 生成 canonical terminal URL。
- session read/update/delete/close/history/ws 已收敛为 workspace-scoped path；仅普通 create-by-cwd fallback 保留 `POST /api/devices/:deviceId/sessions`。
- `go test ./...` 会经过 `web/node_modules/flatted/golang/pkg/flatted` 并显示 `[no test files]`，不是失败，但说明当前 Go package discovery 会扫到 node_modules 下的 Go package。

### 未纳入本轮的内容

- 没有新增设备管理页面。
- 没有实现生产级权限系统、OAuth、SSO、多租户。
- 没有实现 enrollment / pairing flow。
- 没有迁移旧 state 数据。
- 没有把 terminal socket/xterm 状态迁入 Pinia。
- 没有进行真实浏览器人工回归或远程 Gate 环境验证。

## Acceptance checklist

- [x] `/gateway` 不再承载独立产品工作台。
- [x] `/sessions` 是统一工作台。
- [x] Browser 登录前置。
- [x] 登录后直接进入工作台壳，不再展示独立设备选择页。
- [x] 设备选择在 `/sessions` 左侧栏顶部完成。
- [x] 设备 Select 使用 Reka UI Select。
- [x] 设备 Select 位于搜索左侧，按 4:6 布局。
- [x] 设备 Select 不显示“在线 / 离线”文字。
- [x] 设备选择区域不显示“退出”。
- [x] 只有一个设备时自动选中。
- [x] 多设备未选中时不刷新 workspace，不允许 mutation。
- [x] selected device 驱动 workspace/session/history/terminal API。
- [x] 左侧栏底部设置按钮靠左。
- [x] Agent 上报 device id/name。
- [x] 默认 device name 使用 hostname-only，不追加 id 片段。
- [x] legacy default device name 自动归一。
- [x] Browser 和 Agent 使用配置凭据。
- [x] `.termbridge.yaml` 缺失 auth/device 字段时自动生成并写回。
- [x] 首次生成 password 时控制台打印提示。
- [x] localapi runtime path 被移除。
- [x] Gateway canonical device-scoped routes 接入。
- [x] Gateway mutation relay 覆盖主要工作台能力。
- [x] offline readonly history 在本进程 cache 场景可用。
- [x] 设备离线时 mutation 和 terminal attach 不允许。
- [x] 旧 state 不迁移、不兼容。
- [x] 新建会话普通入口目录默认 `~`。
- [x] workspace/目录加号入口仍使用 workspace path。
- [x] 新建会话表单顺序为“目录 / 名称 / 命令”。
- [x] 新建会话默认名称为“默认 / Default”。
- [x] 默认命令按 UA 简单推断：Windows `cmd`、macOS `zsh`、Linux `bash`、fallback `bash`。
- [x] Gate snapshot 明确不做持久化；offline history cache 仅为进程内缓存，重启后不保留。
- [x] `CreateSessionResponse.ws_url`、session response `log_path`、workspace `workspace_key` 已从当前代码 contract 删除。
- [x] session read/update/delete/close/history/ws 已改为 workspace-scoped device API。
- [ ] HTTP/API 错误响应未统一为 structured error shape。
- [ ] 未进行浏览器人工端到端验收。

## Command results

### Backend

```text
go test ./...
```

结果：通过。

摘要：

```text
?   	termbridge-go/cmd/termbridge	[no test files]
ok  	termbridge-go/internal/app	1.446s
ok  	termbridge-go/internal/application/agent	(cached)
ok  	termbridge-go/internal/application/runner	(cached)
ok  	termbridge-go/internal/application/terminal	(cached)
ok  	termbridge-go/internal/domain/identity	(cached)
ok  	termbridge-go/internal/domain/process	(cached)
ok  	termbridge-go/internal/domain/session	(cached)
ok  	termbridge-go/internal/domain/workspace	(cached)
ok  	termbridge-go/internal/infrastructure/config	(cached)
ok  	termbridge-go/internal/infrastructure/errors	(cached)
ok  	termbridge-go/internal/infrastructure/history	(cached)
ok  	termbridge-go/internal/infrastructure/logging	(cached)
?   	termbridge-go/internal/infrastructure/pty	[no test files]
ok  	termbridge-go/internal/infrastructure/pty/gopty	(cached)
ok  	termbridge-go/internal/infrastructure/repository/state	(cached)
?   	termbridge-go/internal/infrastructure/version	[no test files]
ok  	termbridge-go/internal/protocol/terminal	(cached)
ok  	termbridge-go/internal/protocol/tunnel	(cached)
ok  	termbridge-go/internal/transport/cli	1.517s
ok  	termbridge-go/internal/transport/http/gatewayapi	(cached)
ok  	termbridge-go/internal/transport/http/gatewayapi/auth	(cached)
ok  	termbridge-go/internal/transport/http/middleware/requestlog	(cached)
ok  	termbridge-go/internal/transport/http/server	(cached)
?   	termbridge-go/web/node_modules/flatted/golang/pkg/flatted	[no test files]
```

额外执行无缓存后端验证：

```text
go test -count=1 ./...
```

结果：通过。

摘要：

```text
?   	termbridge-go/cmd/termbridge	[no test files]
ok  	termbridge-go/internal/app	7.198s
ok  	termbridge-go/internal/application/agent	6.251s
ok  	termbridge-go/internal/application/runner	6.632s
ok  	termbridge-go/internal/application/terminal	7.489s
ok  	termbridge-go/internal/domain/identity	6.543s
ok  	termbridge-go/internal/domain/process	6.667s
ok  	termbridge-go/internal/domain/session	6.597s
ok  	termbridge-go/internal/domain/workspace	6.685s
ok  	termbridge-go/internal/infrastructure/config	6.693s
ok  	termbridge-go/internal/infrastructure/errors	6.389s
ok  	termbridge-go/internal/infrastructure/history	7.076s
ok  	termbridge-go/internal/infrastructure/logging	6.662s
?   	termbridge-go/internal/infrastructure/pty	[no test files]
ok  	termbridge-go/internal/infrastructure/pty/gopty	11.123s
ok  	termbridge-go/internal/infrastructure/repository/state	6.692s
?   	termbridge-go/internal/infrastructure/version	[no test files]
ok  	termbridge-go/internal/protocol/terminal	6.571s
ok  	termbridge-go/internal/protocol/tunnel	6.581s
ok  	termbridge-go/internal/transport/cli	6.092s
ok  	termbridge-go/internal/transport/http/gatewayapi	6.952s
ok  	termbridge-go/internal/transport/http/gatewayapi/auth	4.977s
ok  	termbridge-go/internal/transport/http/middleware/requestlog	4.356s
ok  	termbridge-go/internal/transport/http/server	0.845s
?   	termbridge-go/web/node_modules/flatted/golang/pkg/flatted	[no test files]
```

### Focused workspace/session contract regression

```text
go test ./internal/application/terminal ./internal/application/agent ./internal/transport/http/gatewayapi ./internal/protocol/tunnel ./internal/infrastructure/repository/state
```

结果：通过。

```text
ok  	termbridge-go/internal/application/terminal	(cached)
ok  	termbridge-go/internal/application/agent	(cached)
ok  	termbridge-go/internal/transport/http/gatewayapi	(cached)
ok  	termbridge-go/internal/protocol/tunnel	(cached)
ok  	termbridge-go/internal/infrastructure/repository/state	(cached)
```

### Frontend typecheck

```text
yarn --cwd web typecheck
```

结果：通过。最新补充执行结果：

```text
yarn run v1.22.22
$ vue-tsc --noEmit
Done in 2.01s.
```

### Frontend lint

```text
yarn --cwd web lint
```

结果：通过。

```text
yarn run v1.22.22
$ eslint .
Done in 1.42s.
```

### Frontend tests

```text
yarn --cwd web test
```

结果：通过。

```text
yarn run v1.22.22
$ vitest run --passWithNoTests

 RUN  v4.1.9 D:/SourceCodes/mywork/TermBridge-go/web

 Test Files  1 passed (1)
      Tests  4 passed (4)
   Start at  18:41:47
   Duration  339ms (transform 35ms, setup 0ms, import 199ms, tests 18ms, environment 0ms)

Done in 0.90s.
```

### Frontend build

```text
yarn --cwd web build
```

结果：通过，有 warning。

主要输出：

```text
yarn run v1.22.22
$ vue-tsc --noEmit && vite build
vite v8.0.16 building client environment for production...
✓ 2392 modules transformed.
dist/index.html                                      0.40 kB │ gzip:   0.27 kB
dist/assets/SessionsView-BrP-ENHg.css                3.93 kB │ gzip:   1.01 kB
dist/assets/index-Ch8OWeSY.css                      19.44 kB │ gzip:   4.99 kB
dist/assets/_plugin-vue_export-helper-BDNMzG2s.js    0.08 kB │ gzip:   0.09 kB
dist/assets/SettingsView-B6f87vXU.js                 0.25 kB │ gzip:   0.22 kB
dist/assets/HelpView-DMoYd1VD.js                     0.55 kB │ gzip:   0.38 kB
dist/assets/index-BYr7mM_V.js                      205.69 kB │ gzip:  74.90 kB
dist/assets/SessionsView-CeVcewOG.js               593.15 kB │ gzip: 159.97 kB
✓ built in 446ms
Done in 3.39s.
```

Warnings：

- Rolldown 对 `node_modules/@vueuse/core/dist/index.js` 中 `/* #__PURE__ */` 注释位置给出 `INVALID_ANNOTATION` warning。
- `SessionsView` chunk 超过 500 kB。

这些 warning 没有导致 build 失败。

## Static code inspection

本轮除命令验证外，还对关键 UI / 默认值实现做了静态核对：

- `web/src/router/index.ts`
  - `/` redirect 到 `sessions`。
  - `/sessions` lazy-load `SessionsView.vue`。
  - `/settings` lazy-load `SettingsView.vue`。
  - `/help` lazy-load `HelpView.vue`。
  - `/gateway` redirect 到 `sessions`。

- `web/src/App.vue`
  - 只渲染 `<RouterView />`。

- `web/src/main.ts`
  - 注册 `createPinia()`、router、i18n。

- `web/src/views/SessionsView.vue`
  - `loadDevices()` 保留已选设备，否则单设备自动选中，多设备时 selected device 为空。
  - `refresh()` 在无 selected device 时直接返回。
  - `allowMutations` 要求 selected device、在线且非 offline readonly。
  - 新建会话普通入口默认 cwd 为 `~`。
  - workspace 入口保留 workspace path。
  - `defaultCommand()` 使用 browser user agent 简单推断命令。

- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
  - header 内使用 Reka UI `SelectRoot`。
  - Select 与搜索框同处 `grid-cols-[4fr_6fr]`。
  - Select item 仅展示 `device.name`。
  - footer 未右对齐。

- `web/src/i18n.ts`
  - 登录文案为泛化“登录 / Sign in”。
  - 新建会话字段与默认名符合当前 UX 决策。

## Missed or expanded scope

### Expanded scope

- 按用户实现阶段要求，将代码标识符中的 `ID` / `URL` 大范围统一为 `Id` / `Url`，包括 Go 类型、字段、方法和部分 TS 命名。
- 删除 localapi 比 Requirement 初稿更激进，但符合 Plan 阶段用户明确覆盖决策。
- offline readonly history 从 Spec 的可选/后续能力被 Plan 阶段提升为必做；当前决策明确只做进程内 cache，不做持久化 Gate snapshot。
- 后续 UX 决策进一步调整了设备选择位置、设备名策略、新建会话默认值和 i18n 文案策略。

### Missed / incomplete scope

1. **Persistent offline snapshot 不做**
   - Plan 曾描述 Gateway-side snapshot/cache store，可落盘到 `gateway-cache`。
   - 当前决策明确不做持久化 Gate snapshot；实现保留进程内 cache，设备离线但 Gate 不重启时可读，Gate 重启后不可读。

2. **Structured error 未完整实现**
   - Spec/Plan 建议稳定 `{ error: { code, message } }`。
   - 当前多处仍是 `http.Error` 文本响应和 `502 Bad Gateway` 映射。

3. **Pinia 状态边界未继续深化**
   - Pinia 已作为基础设施引入，但本功能未新增 auth/devices/workbench store。
   - 当前 auth/device/workbench 状态仍集中在 `SessionsView.vue`。

4. **Manual / integration 验证未执行**
   - 未启动真实 `termbridge serve` 做浏览器登录、设备选择、创建 session、terminal attach 人工回归。
   - 未模拟远程云端 Gate。
   - 未实际断开 Agent 后手工验证 UI 离线历史状态。

## Risks

1. **离线历史语义风险**
   - 当前离线只读历史依赖 Gate 进程内 cache。用户可能理解为持久 snapshot；交付说明需要明确“Gate 重启后 cache 不保留”。

2. **API error contract 风险**
   - 前端当前可显示错误文本，但 API contract 还不是稳定 structured error；未来客户端/云端 API 需要补齐。

3. **非 serve CLI state 路径风险**
   - `serve` runtime 使用 device-scoped store；部分非 serve CLI 命令仍使用 base store layout。若长期模型要求所有 workspace/session 都强制归属 Device，后续需要继续收口。

4. **chunk size 风险**
   - `SessionsView` 继续超过 500 kB warning，短期不影响功能，长期应拆分 view/store/dialog/terminal 逻辑。

5. **明文凭据风险**
   - `.termbridge.yaml` 保存明文 password。本阶段按计划接受，但后续需要文件权限、secret 管理或 pairing flow。

6. **人工端到端缺口**
   - 自动化检查通过，但真实 terminal attach、浏览器交互、Agent 断连后的 UX 仍建议人工验收。

## Conclusion

当前实现满足 Plan 阶段确定的统一 Gate / Device / Auth 工作台目标，并覆盖了后续 UI / UX 收口调整：

- `/sessions` 成为统一工作台。
- Browser 和 Agent 都通过 Gate/device 模型访问 runtime。
- localapi runtime path 已移除。
- 设备身份、配置凭据、canonical device/workspace-scoped API、Gate mutation relay 和离线只读 cache 已落地。
- `workspace_key`、session `log_path`、`CreateSessionResponse.ws_url` 已从当前 API contract 删除。
- session read/update/delete/close/history/ws 已通过 `/api/devices/:deviceId/workspaces/:workspaceId/sessions...` 定位。
- 登录后不再进入独立设备选择页；设备选择已整合到 `/sessions` sidebar。
- 单设备自动选中，多设备未选中时保持工作台壳但禁止 device-scoped 数据刷新和 mutation。
- 默认 device name 已改为 hostname-only。
- 新建会话默认值和表单顺序符合当前 UX 决策。
- 自动化验证命令全部通过。

但交付前仍需明确以下未完成/风险事项：

1. offline readonly history 当前明确为进程内 cache，不做持久化 Gate snapshot。
2. API structured error contract 尚未完成。
3. 尚未做真实浏览器人工端到端验收。

总体结论：验证通过，但带上述已知风险和后续修正项。
