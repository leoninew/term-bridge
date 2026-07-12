# 命令快捷方式实施计划

最后修改时间: 2026-07-12 20:59:11

Review status: Accepted

## Basis

本计划基于：

- `docs/requirement/20260710-command-shortcuts.md`，Review status: Accepted。

当前流程：标准模式 / standard，计划 / Plan。

本功能在 Agent runtime 数据面实现，不增加 Cloud 数据库表。快捷方式是当前设备本地数据；Cloud 只通过既有 device tunnel 将请求转发至被选中的在线 Agent，因而 Local 页面与该设备的 Cloud 页面读取同一份快捷方式。

## Design decisions

### 数据所有权与删除

1. 快捷方式表由 Agent 数据库拥有，每次查询都由 Agent 当前 `device_id` 限定；HTTP、Proto 与前端请求均不接受调用方指定的 `device_id`。
2. 快捷方式不与 `Session` 或 `SessionRun` 建立外键关系；新建/编辑 Session 时只复制所选快捷方式当时的原始命令文本。
3. 快捷方式允许硬删除：删除只影响后续选择，不追溯影响已保存的 Session 命令、历史运行记录或已归档输出。由于没有外键引用，硬删除避免为用户不可恢复的本地偏好数据引入多余的软删除语义。
4. 名称不要求唯一；用户可为不同环境或用途创建同名快捷方式，卡片以完整命令与描述帮助区分。
5. 快捷方式顺序为每设备持久化的 `sort_order`：新建追加到末尾，内容编辑不改变位置，列表按 `sort_order ASC, id ASC` 返回。
6. 排序接口仅接受当前设备完整、无重复的快捷方式 ID 排列；未知、跨设备、缺失和重复 ID 均拒绝，验证与顺序更新在同一事务内完成。
7. 前端以卡片主体作为拖动区域，不添加专用手柄；编辑、删除按钮通过 draggable filter 保持正常点击交互。

### 命令与会话编辑语义

1. 快捷方式的 `command` 为单个原始文本字段，不拆为 argv，不重新拼接；`ccs run c1`、包含引号/管道的 `codex ... -c xxxx` 和 `cmd` 均按原文保存。
2. 新建会话表单默认处于 `shortcut` 来源；选择某项后将其命令复制到表单的 `commandText`，最终沿用既有 `CreateSessionReq.command: [commandText]` 协议。
3. 没有快捷方式时，表单仍默认显示快捷方式来源和空状态；创建按钮不可提交，用户可明确切换到直接命令来源完成创建，不自动静默改变来源。
4. Session 不保存 shortcut ID。编辑快捷方式、删除快捷方式都不会改变既有 Session 或 `session_runs.command_json`。
5. `PATCH` 仅允许生命周期精确为 `stopped` 的 Session 更新名称和原始命令；`running` 和 `failed` 均拒绝。`cwd` 仅显示、不可更新；保存不会启动或重启 Session。
6. 下次 rerun 读取更新后的 Session 命令并写入新的 `session_runs` 快照，既有 run 的命令保持不变。

### API 与交互边界

1. 增加独立 `shortcut.proto`，提供列表、创建、更新与删除，无需“查看单项”接口；卡片列表已经满足查看需求。
2. 复用现有 Local runtime endpoint、Cloud device tunnel、权限校验和 offline `503` 行为，禁止 Cloud handler 直连 Agent 数据库或新增 Cloud shortcut 表。
3. 新建与编辑 Session 共用“命令来源选择 + 快捷方式选择 + 直接命令”字段组件；编辑模式的工作目录只读，以免两套表单校验与文案漂移。
4. 快捷方式管理使用设备上下文路由：本地与 Cloud 设备路径各自进入共享页面壳，页面在创建、编辑、删除成功后重新获取列表，避免失败时留下乐观但错误的 UI 状态。

## Implementation steps

### 1. 定义 Proto 与 Tunnel 契约并生成代码

修改：

- `proto/termbridge/agent/v1/shortcut.proto`（新增）
- `proto/termbridge/agent/v1/session.proto`
- `proto/termbridge/shared/v1/tunnel.proto`
- `internal/gen/proto/**`（由生成命令产生）
- `web/src/gen/proto/**`（由生成命令产生）

实施：

1. 新建 `Shortcut` 消息，包含 `id`、`name`、单个 `command`、可选 `description`、`created_at`、`updated_at`。
2. 定义 `ListShortcuts`、`CreateShortcut`、`UpdateShortcut`、`DeleteShortcut` 请求/响应。创建/更新请求的名称和命令为必填语义，描述可缺省；不暴露 `device_id`。
3. 为 `UpdateSessionReq` 增加可选的单个原始 `command` 字段，保留现有 `name` 字段和 create 的单元素 `repeated command` 兼容语义；更新请求至少应包含一个可变字段。
4. 在 `TunnelFrame` 为四个 shortcut 操作加入新的请求/响应 oneof 编号，编号追加在现有 runtime 操作之后，禁止复用已有字段号。
5. 执行 `task proto`，仅接受生成器产生的 Go/TypeScript 绑定，不手改 `*.pb.go` 或 web 生成文件。
6. 为新增和变更请求补充 Proto/JSON 译码测试，锁定 optional description 和原始 command 文本行为。

### 2. 新增 Agent 数据迁移与完整性保护

修改：

- `migrations/agent/sqlite/<new>_command_shortcuts.sql`（新增）
- `migrations/agent/mysql/<new>_command_shortcuts.sql`（新增）
- `internal/agent/infrastructure/database/migrator.go`
- `internal/agent/infrastructure/database/migrator_test.go`

实施：

1. 为 SQLite 与 MySQL 使用相同 migration 版本号创建 `shortcuts` 表：稳定主键、`device_id`、`name`、原始 `command`、可空 `description`、创建/更新时间。
2. 为按设备列出活动快捷方式建立 `(device_id, updated_at)` 索引；不添加名称唯一约束，避免超出需求的产品限制。
3. 由于快捷方式为硬删除，本表不添加 `deleted_at`；删除操作直接按 `id + device_id` 删除。
4. 保持 SQLite/MySQL 的可空描述、时间字段、外键/索引语义一致，并遵循现有 Agent runtime schema 的方言写法。
5. 将 `shortcuts` 纳入 Agent migrator 的不完整 runtime schema 检测，保证缺表时在启动阶段明确失败。
6. 开发阶段允许修订这两个尚未发布 migration 文件；一旦 migration 已应用到共享/发布环境，后续变化必须通过新 migration 追加，不能重写历史。

### 3. 建立快捷方式领域模型和设备范围 Repository

修改：

- `internal/agent/model/task/shortcut/shortcut.go`（新增）
- `internal/agent/model/task/shortcut/shortcut_test.go`（新增）
- `internal/agent/repository/task/state/db_store.go`
- `internal/agent/repository/task/state/db_store_test.go`
- 必要时 `internal/agent/repository/task/state/store.go` 及其测试 fake

实施：

1. 建立专用 `shortcut` model，显式表达 ID、设备归属、名称、原始命令、可选描述和时间；按项目命名约定使用 `Id`，不引入 map 嵌套结构。
2. 将输入规范化和业务校验集中在 model/application 层：`name`、`command` trim 后不可为空；描述 trim 后为空时统一为无描述；不在 handler/SQL 中散落重复校验。
3. 在 `DbStore` 实现设备作用域的 list/create/update/delete：所有 `WHERE` 均带当前 store 的 `device_id`；其他设备的 ID 不可读、不可改、不可删。
4. List 使用稳定排序（建议 `updated_at DESC, id ASC`），以便卡片列表和快捷方式选择器表现一致。
5. 用现有 ID 生成、事务、行扫描与错误分类模式实现，不引入第二个数据库连接或泛化 Repository 框架。
6. 将 test SQLite schema fixture 纳入 `shortcuts`，补充以下业务结果测试：创建后可按设备列出、原始命令无损、描述可省略、更新刷新时间、删除后不可选择、跨设备隔离、空名称/命令被拒绝。

### 4. 扩展 Agent 应用服务并收紧停止会话编辑规则

修改：

- `internal/agent/application/task/terminal/registry.go`
- `internal/agent/application/task/terminal/registry_test.go`
- `internal/agent/application/user/runtime_access.go`
- `internal/agent/application/bootstrap/server.go`（按依赖注入实际需要调整）

实施：

1. 定义窄的 `ShortcutStore` / `ShortcutService` 依赖，供 `Registry` 或独立 runtime application service 使用；避免把四个 shortcut 方法无条件塞入已有庞大的 `RuntimeStore`，导致与快捷方式无关的 fake 大量膨胀。
2. 通过 `WebTerminalAccess` 暴露 list/create/update/delete 快捷方式，先检查 `context` 再委托应用层，沿用现有 runtime access 结构。
3. 修改 `Registry.UpdateSession`：加载 Session view 后只接受 `StateStopped`，明确拒绝 `running` 与 `failed`；校验请求至少有一个字段，更新 `name` 和/或原始 `command`，绝不更新 `LaunchCwd`。
4. Repository 更新以当前 `updated_at` 和 lifecycle `stopped` 作为条件，避免 `rerun` 已将状态改为 running 后，延迟编辑覆盖其命令；条件更新失败返回可识别的冲突/不可编辑错误。
5. 不在 update 路径启动 PTY、创建 `session_run` 或改变 history。Rerun 继续通过既有运行创建路径从更新后的 Session 生成下一个 run 快照。
6. 修改原先“运行中会话可以改名”的测试，新增 stopped 可编辑、running/failed 不可编辑、cwd 不变、原始命令更新、rerun 使用新命令且旧 run 未变，以及 edit/rerun 竞争保护测试。

### 5. 补齐 Local HTTP 路由和帧映射

修改：

- `internal/agent/api/handler/server.go`
- `internal/agent/api/handler/runtime_endpoint.go`
- `internal/agent/api/handler/*_test.go`
- `internal/shared/dto/protocol/tunnel/frame.go`

实施：

1. 在 Agent 已认证 Local API 注册：
   - `GET /local-api/shortcuts`
   - `POST /local-api/shortcuts`
   - `PATCH /local-api/shortcuts/{shortcutId}`
   - `DELETE /local-api/shortcuts/{shortcutId}`
2. 使用与 Session 请求一致的 typed `TunnelFrame` 创建和 runtime endpoint 代理方式，保留统一错误响应、请求 ID、JSON 处理与认证检查。
3. 扩展 tunnel 响应 JSON 抽取，使四种 shortcut response 能返回给 Local handler 和 Cloud relay。
4. Handler 测试覆盖所有 method/path、非法 JSON、空 name/command、可选 description、成功删除的响应语义、错误 ID 不能越权访问，以及现有 session 子路径仍正常分发。

### 6. 贯通 Cloud Tunnel 和设备级 HTTP 路由

修改：

- `internal/agent/application/user/cloud_client.go`
- `internal/cloud/api/handler/runtime_endpoint.go`
- `internal/cloud/api/handler/route.go`
- `internal/cloud/api/handler/server.go`
- `internal/cloud/api/handler/*_test.go`

实施：

1. 在 Agent connector 的 frame dispatch、请求/响应 allowlist 与 runtime response 构造中完整增加四种 shortcut 操作。
2. 在 Cloud tunnel runtime endpoint 增加相同 frame 映射；Cloud 不读取或写入 Agent shortcut 表。
3. 在设备授权路径加入：
   - `GET /cloud-api/devices/{deviceId}/shortcuts`
   - `POST /cloud-api/devices/{deviceId}/shortcuts`
   - `PATCH /cloud-api/devices/{deviceId}/shortcuts/{shortcutId}`
   - `DELETE /cloud-api/devices/{deviceId}/shortcuts/{shortcutId}`
4. 复用 `handleDevice` 的设备访问授权、在线路由选择与 existing `device_offline` 失败策略；列表也不能为了快捷方式新增不具备一致性的 Cloud 缓存。
5. 以现有 Cloud relay/session 测试为模板，验证请求和 JSON response 经 tunnel 往返、设备离线返回既有错误，以及一个设备无法通过 Cloud 路径操作另一个设备的本地记录。

### 7. 扩展前端 API、路由与快捷方式管理界面

修改：

- `web/src/features/sessions/runtime.ts`
- `web/src/features/local/api.ts`
- `web/src/features/cloud/api.ts`
- `web/src/views/local/ShortcutsView.vue`（新增）
- `web/src/views/cloud/ShortcutsView.vue`（新增）
- `web/src/components/shortcut/ShortcutsPageShell.vue`（新增）
- `web/src/components/shortcut/ShortcutCard.vue`（新增）
- `web/src/components/shortcut/ShortcutEditorDialog.vue`（新增）
- `web/src/components/shortcut/DeleteShortcutDialog.vue`（新增）
- `web/src/composable/useShortcuts.ts`（新增）
- `web/src/router/index.ts`
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
- `web/src/components/session/SessionsPageShell.vue`
- `web/src/i18n.ts`

实施：

1. 在 target-neutral runtime API 中增加 shortcut CRUD；Local client 使用 `/local-api/shortcuts`，Cloud client 使用当前 target 的 `/cloud-api/devices/{deviceId}/shortcuts`，不添加客户端指定设备 ID 的请求 body。
2. 添加 local `/shortcuts` 与 cloud `/devices/:deviceId/shortcuts` 路由；两种 view 各自构造 target-aware API 后交给共享 `ShortcutsPageShell`，遵循现有 Local/Cloud SessionsView 适配模式。
3. 在 `WorkspaceSessionSidebar` 设置下拉菜单新增“快捷方式”项和 `openShortcuts` emit；`SessionsPageShell` 通过当前 runtime target 导航，保持 Sidebar 不耦合具体 local/cloud route name。
4. `ShortcutsPageShell` 首次载入列表，卡片展示名称、完整命令与存在时的描述；空状态提供创建入口；每个 mutation 成功后重新获取列表并通知会话表单选择数据的消费者。
5. 使用 `DialogRoot` 构建新增/编辑模态窗，使用 `AlertDialogRoot` 构建删除确认；沿用现有 dialog、button 和 notification 样式与失败提示，不新增“查看详情”模态窗。
6. 创建、编辑、删除过程中禁用重复提交；失败保留原列表/表单数据并展示错误，避免未确认的乐观写入。
7. 增加完整中英文 i18n：设置菜单、标题、空状态、字段、placeholder、命令来源、创建/编辑/删除确认、成功/失败 toast、会话不可编辑提示和只读目录标识。

### 8. 将快捷方式来源接入新建和停止会话编辑表单

修改：

- `web/src/composable/useCreateSessionDraft.ts`
- `web/src/composable/useCreateSessionDraft.test.ts`
- `web/src/components/session/SessionFormFields.vue`（新增）
- `web/src/components/session/CreateSessionPanel.vue`
- `web/src/components/session/EditSessionDialog.vue`
- `web/src/components/session/SessionWorkbench.vue`
- `web/src/components/session/SessionsPageShell.vue`
- `web/src/composable/useSessionDialogs.ts`
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
- 相应前端单测

实施：

1. 让 draft 增加纯 UI 状态：命令来源 `shortcut | command` 和选中的 shortcut ID；`commandText` 仍是唯一提交给 Session API 的值。
2. 打开新建表单时默认来源为 `shortcut`；选择快捷方式时将其当前原始命令复制至 `commandText`，切换到直接命令时允许用户更新 `commandText`。切换来源不提交 shortcut ID。
3. 将 `CreateSessionPanel` 的字段提取为共享 `SessionFormFields`，接受 create/edit mode、快捷方式列表、来源、选中项、name、command、cwd 和 disabled/read-only 状态。创建面板保留 workbench 外壳和创建按钮。
4. 重写现有仅名称的 `EditSessionDialog` 以使用共享字段：预填 Session 名称/命令/cwd，cwd 显示为只读；保存仅发 `name` 与 `command` 更新，不触发 create/rerun。
5. Sidebar 的编辑入口只显示给 `lifecycle_state === 'stopped'` 的 Session；前端也在打开/提交前再检查状态，后端仍是 authoritative 规则。
6. 更新会话成功后以 API 返回的 summary 覆盖 workspace store；不重启、不新开 terminal tab。快捷方式选择后创建成功时维持已有 create → refresh → open tab 流程。
7. 覆盖 draft 原始命令无损、默认快捷方式、空列表禁止 shortcut 提交、切换直接命令、选择后复制快照；覆盖 stopped-only 编辑可见性、只读 cwd、编辑请求和后续 rerun 命令语义。新增 shortcut API/composable 测试，分别断言 Local 与 device-Cloud URL 及 mutation 后刷新逻辑。

### 9. 增加设备级拖拽排序

修改：

- `proto/termbridge/agent/v1/shortcut.proto`
- `proto/termbridge/shared/v1/tunnel.proto`
- `migrations/agent/sqlite/202607120001_shortcut_ordering.sql`
- `migrations/agent/mysql/202607120001_shortcut_ordering.sql`
- `internal/agent/repository/task/state/db_store.go` 及测试
- Agent Local runtime、Agent tunnel、Cloud relay 的 endpoint/route 及测试
- `web/src/features/{local,cloud}/api.ts`、`web/src/features/sessions/runtime.ts`
- Local/Cloud `SessionsView.vue`、`ShortcutsView.vue` 与 `ShortcutsPageShell.vue`

实施：

1. 通过 SQLite/MySQL migration 增加 `sort_order`，按原有 `updated_at DESC, id ASC` 顺序为每设备确定性回填连续位置；SQLite/MySQL 均提供可逆 migration。
2. `DbStore` 在创建时分配尾部位置；MySQL 创建使用串行化事务与锁定查询，防止并发追加重复位置。编辑不更新排序字段。
3. 新增 `UpdateShortcutOrderReq/Resp` 和 tunnel oneof，打通 Local HTTP `PATCH /shortcuts/order`、Cloud device route、Agent runtime access 和 response dispatch。
4. Repository 在单一事务内读取当前设备列表、校验精确排列并更新所有位置；返回规范排序，验证失败零写入。
5. 快捷方式页面用 `VueDraggable` 绑定卡片网格，拖动开始保存快照，结束后提交完整 ID 顺序；成功替换为服务端顺序，失败恢复快照、提示错误并重新加载；无实际位移不请求。
6. 不添加拖拽手柄；拖动来源为卡片表面，`button` filter 让编辑/删除按钮维持独立操作。
7. 生成 binding 后补充 repository、service、Local frame、Agent tunnel 和 Cloud relay 测试，重点覆盖设备隔离、非法排列零写入与 Cloud response 分发。

### 10. 格式化、收敛和防回归

1. 用项目工具生成 Proto 后执行 Go 格式化；不手工格式化或重排无关的生成文件。
2. 清除被替代的 name-only 编辑状态、重复校验和无调用 helper，保持创建/编辑表单的单一命令来源实现。
3. 复核工作区已有 `internal/cloud/api/handler/server.go` 未提交改动；它不属于快捷方式功能，Implementation 与 Verification 中必须与本功能 diff 分开报告，不能覆盖、撤销或混入功能测试结论。

## Files to change

### 预计新增

- `proto/termbridge/agent/v1/shortcut.proto`
- `migrations/agent/sqlite/<new>_command_shortcuts.sql`
- `migrations/agent/mysql/<new>_command_shortcuts.sql`
- `internal/agent/model/task/shortcut/shortcut.go`
- `internal/agent/model/task/shortcut/shortcut_test.go`
- `web/src/views/local/ShortcutsView.vue`
- `web/src/views/cloud/ShortcutsView.vue`
- `web/src/components/shortcut/ShortcutsPageShell.vue`
- `web/src/components/shortcut/ShortcutCard.vue`
- `web/src/components/shortcut/ShortcutEditorDialog.vue`
- `web/src/components/shortcut/DeleteShortcutDialog.vue`
- `web/src/composable/useShortcuts.ts`
- `web/src/components/session/SessionFormFields.vue`
- 相关单测文件。

### 预计修改

- `proto/termbridge/agent/v1/session.proto`
- `proto/termbridge/shared/v1/tunnel.proto`
- 由 `task proto` 生成的 `internal/gen/proto/**` 与 `web/src/gen/proto/**`
- `internal/agent/infrastructure/database/migrator.go`
- `internal/agent/infrastructure/database/migrator_test.go`
- `internal/agent/repository/task/state/db_store.go`
- `internal/agent/repository/task/state/db_store_test.go`
- `internal/agent/application/task/terminal/registry.go`
- `internal/agent/application/task/terminal/registry_test.go`
- `internal/agent/application/user/runtime_access.go`
- `internal/agent/application/user/cloud_client.go`
- `internal/agent/api/handler/server.go`
- `internal/agent/api/handler/runtime_endpoint.go`
- `internal/shared/dto/protocol/tunnel/frame.go`
- `internal/cloud/api/handler/server.go`
- `internal/cloud/api/handler/runtime_endpoint.go`
- `internal/cloud/api/handler/route.go`
- 相应 Go handler/relay/tunnel 测试。
- `web/src/features/sessions/runtime.ts`
- `web/src/features/local/api.ts`
- `web/src/features/cloud/api.ts`
- `web/src/router/index.ts`
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
- `web/src/components/session/CreateSessionPanel.vue`
- `web/src/components/session/EditSessionDialog.vue`
- `web/src/components/session/SessionWorkbench.vue`
- `web/src/components/session/SessionsPageShell.vue`
- `web/src/composable/useCreateSessionDraft.ts`
- `web/src/composable/useSessionDialogs.ts`
- `web/src/i18n.ts`
- 对应前端测试。

### 不计划修改

- `migrations/cloud/**`：快捷方式不属于 Cloud identity/binding 数据。
- PTY manager、shell command parse、terminal WebSocket 协议和原始命令执行逻辑。
- 已有 Session / SessionRun 的历史命令与输出数据。
- 快捷方式跨设备同步、账号级共享、导入导出、模板变量、标签/分组功能。

## Verification plan

进入 Verification / 验证阶段后，按实际修改范围执行并记录结果：

1. Proto：`task proto`，确认 Go 与 web bindings 由生成器更新且无手改生成文件。
2. Go 定向测试：Agent migrator、`state.DbStore`、terminal registry、Agent HTTP handler、Agent tunnel dispatch、Cloud relay handler 对应 package 的 `go test`。
3. 前端定向测试：`yarn --cwd web test` 并确认新增 draft/API/composable/组件测试通过。
4. 全量单测：`task test`（等价于 web Vitest 与 `go test ./cmd/... ./internal/...`）。
5. 静态检查：`yarn --cwd web typecheck`、`yarn --cwd web lint`，以及可用时 `./bin/golangci-lint run ./cmd/... ./internal/...`。
6. 迁移冒烟：对临时 Agent SQLite 数据库运行 migration，执行 shortcut CRUD，验证命令原文/描述可选/设备作用域；如环境具备 MySQL，运行对应 migrator 测试或方言连接检查。
7. 端到端/手工流：
   - 创建、编辑、删除设备本地快捷方式；
   - Local 与同一设备 Cloud 视图分别读取该设备列表；切换其他设备不出现该记录；
   - 新建会话默认快捷方式、空列表切直接命令、带 shell 特殊字符命令无损；
   - stopped Session 编辑后 cwd 未变，下一次 rerun 使用新命令，旧 run 仍保留旧命令；
   - running/failed Session 不能编辑，Cloud 离线按既有错误返回。
8. Diff 审查：核对所有实际变更与本 Plan 一致，并将预先存在的 `internal/cloud/api/handler/server.go` 改动单列，不归因于本功能。

## Blockers

暂无实现阻塞项。

## Assumptions

1. “停止的会话”精确对应 `lifecycle_state == stopped`；`failed` 虽可 rerun，但不属于本轮可编辑集合。
2. 系统没有默认内置快捷方式；用户首次使用在空列表状态下可创建快捷方式或主动切换直接命令。
3. 快捷方式的名称允许重复，描述为空不在卡片上渲染占位内容。
4. 开发期 migration 尚未应用到共享/发布环境，因而可按用户授权反复修订；发布后遵循 append-only migration 原则。
5. 设备本地数据在 Cloud 页面需要在线 Agent；本轮不为 shortcut list 引入离线缓存。

## Risks

1. Proto oneof、endpoint 映射、Agent dispatch 与 Cloud response allowlist 有多个对应点；遗漏任一处会导致 Cloud 请求编译通过但运行时失败，必须以端到端 relay 测试覆盖。
2. Session edit 与 rerun 竞争可能导致状态/命令不一致；必须在 repository conditional update 中把 `stopped` 作为写入前置条件，而不能只依赖 UI 隐藏按钮。
3. 新建默认快捷方式而本地无数据时可能阻断用户；空状态必须清楚提供创建与切换直接命令的路径。
4. 快捷方式降低复用危险命令的摩擦；不得因此绕过既有认证、设备授权、Cloud route ownership 或原始命令执行安全边界。
5. MySQL/SQLite 对 NULL、TEXT、时间和索引细节有差异；迁移与 repository 测试需覆盖两者的等价业务行为。
6. 当前工作树已有与本功能无关的 Cloud OAuth handler 改动，验证与最终交付必须避免把其结果误报为快捷方式变更。

## Rollback

若实现后需回退：

1. 撤回前端快捷方式路由、设置入口、管理组件与 session 命令来源 UI。
2. 撤回 Local/Cloud shortcut routes、Tunnel frame mapping 和 RuntimeAccess 方法。
3. 撤回 Agent shortcut model/repository 方法和会话编辑 command 扩展；恢复既有 name-only edit 行为前需重新评估“运行中不可编辑”的产品约束，不应无意重新放开 running edit。
4. 已应用的数据库 migration 不删除或改写；通过新 migration 删除/废弃 `shortcuts` 表或保留未使用表，以符合发布后的 migration 规则。
5. 不影响既有 Session / SessionRun 命令快照、PTY history 或 Cloud identity schema。

## User review notes

- 用户确认快捷方式为当前机器的本地数据，不跨设备共享。
- 用户确认新建会话默认基于快捷方式，也可切换到直接命令。
- 用户确认可删除曾被会话使用的快捷方式，Session 保存命令快照。
- 用户确认卡片直接展示信息，不需要单独详情查看模态窗。
- 用户明确要求快捷方式通过 migration 持久化，开发阶段可反复调整。
