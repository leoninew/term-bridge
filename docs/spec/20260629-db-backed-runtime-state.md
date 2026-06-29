# 数据库存储设备、工作区、会话与运行记录规格
最后修改时间: 2026-06-29 13:11:57

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；规格 / Spec 已接受，当前进入计划 / Plan。

## Requirement basis

- Requirement: `docs/requirement/20260629-db-backed-runtime-state.md`
- Requirement Review status: Accepted

本规格基于用户已确认的关键决策：

- 数据库是设备、工作区、会话、运行记录的 authoritative storage。
- 不保留旧文件存储路径，不向后兼容，不迁移旧 `.termbridge` 数据。
- 设备采用选项 A：复用并演进现有 `devices`，避免新增第二套设备概念。
- terminal history / `history.log` 保持现状；运行记录是业务记录，不等同于 terminal history。
- 每个 session 有多条运行记录，最后一条是当前运行记录。
- workspace/session 删除采用软删除。
- 取消删除 private key 的决策；当前本地 device private key 能力保持现状。
- device key 应作为 device 的属性建模，不再作为独立业务表。
- 不使用唯一索引，业务唯一性由 repository 防重；已有 active 记录时复用。
- 不引入新的 local direct backend，不改变 self-tunnel / unified gateway 模型。

## Overview

本规格将当前 `state.Store` 文件聚合模型替换为数据库 repository 模型。新的主数据模型由四类核心表组成：

1. `devices`：复用并演进现有设备表，承载设备业务记录。
2. `workspaces`：新增工作区表。
3. `sessions`：新增会话表。
4. `session_runs`：新增运行记录表，一个 session 可以有多条 run，最后一条为当前运行记录。

现有业务文件路径如 `.termbridge/workspaces/**/workspace.json`、session fragment JSON、设备 `device.json` 不再作为新业务状态的主写入路径，也不作为旧数据兼容读取路径。`history.log` 与 `private_key.pem` / `public_key.pem` 属于本轮明确保持现状的例外：history 不属于本轮业务运行记录入库范围，private key 能力不在本轮删除。

为了控制本轮范围，本规格不把 terminal history/output 纳入 DB 迁移。运行记录只表达业务运行事实，例如 command、process started、lifecycle state、exit、错误原因、时间戳和必要 metadata；它不负责 replay terminal bytes，现有 history 行为保持现状。

## Current implementation facts

当前代码中与本规格相关的现实状态：

- `internal/infrastructure/repository/state/store.go` 以 `.termbridge/workspaces` 为 root，写入 `workspace.json` 和 `index.json`，并把 session state/process/exit 聚合进 workspace children。
- `Store.HistoryPath()` 返回 session 目录下的 `history.log`，`terminal.Registry` 和 `SessionRuntime` 依赖它创建 writer、读取 replay、rerun archive。
- `internal/app/app.go` 的 `runExec` 使用 `state.NewStore(cfg.Runtime.StateDir)`，并通过 `workspace.Resolver` / `session.Manager` 创建 workspace/session。
- `internal/application/terminal.Registry` 通过 `state.Store` 完成 workspace/session/run state 读写，并在 rerun 时归档 history 文件。
- `internal/application/agent/device.go` 当前会在 state dir 下创建：
  - `devices/<device_id>/device.json`
  - `devices/<device_id>/private_key.pem`
  - `devices/<device_id>/public_key.pem`
- `internal/application/agent/client.go` 当前 tunnel header 通过 `LoadDevicePrivateKey` 读取 private key 并调用 `SignedTunnelHeader`。
- `internal/transport/http/gatewayapi/server.go` 当前支持 `DevicePublicKeys` map 或 `DeviceRepository.PublicKey()` 校验 agent tunnel 签名。
- 用户已更新决策：private key 文件能力保持现状；但 `device_keys` 不应作为独立业务表，device key 应作为 `devices` 的属性。
- 现有 DB migration 已有 `devices`、`user_devices`、`device_keys`、`device_binding_codes`；用户要求旧 `device_keys` 表直接删除，并就地修改原 SQL。

## Design decisions

### 1. 四张核心业务表定义

本轮形成四张核心业务表语义：

| Core table | Physical action | Purpose |
|---|---|---|
| `devices` | 演进现有表 | 设备业务记录；不再依赖 state dir `device.json`。 |
| `workspaces` | 新增表 | 工作区业务记录、排序、软删除。 |
| `sessions` | 新增表 | 会话业务记录、当前状态快照、排序、软删除。 |
| `session_runs` | 新增表 | session 的多次业务运行记录，最后一条为当前运行记录。 |

说明：用户选择“选项 A”，所以设备表不是另起 `runtime_devices`。从业务模型看仍然是四张核心表；从 migration 实现看，是演进现有 `devices` 加新增三张表。

### 2. 不做旧文件兼容 / 迁移

新 repository 不读取旧 `.termbridge/workspaces`、`device.json`、session fragment JSON 作为 fallback。升级后的空库就是空 runtime state；用户需要重新创建 workspace/session/设备业务记录。

Plan / Implementation 阶段应删除或停用以下主路径：

- `internal/infrastructure/repository/state/store.go` 的文件写入型主 repository。
- `workspace.json` / `index.json` 写入。
- session `state.json` / `process.json` / `exit.json` 兼容测试语义。
- device `device.json` 业务记录生成/读取路径；`private_key.pem`、`public_key.pem` 保持现状，不纳入本轮业务记录入库范围。

如果部分历史 helper 暂时无法物理删除，必须从产品运行路径断开，并在测试中证明不会产生旧业务文件。

### 3. Device key 建模

取消“删除 private key”的决策。本轮保持当前本地 device private key 文件能力，不改变现有 tunnel signing 行为。

目标调整为：`device_key` 是 device 的属性，而不是独立业务表。后续实现应：

- 保留当前 `PrivateKeyFileName` / `LoadDevicePrivateKey` / `SignedTunnelHeader` 运行行为。
- 不再把 `device_keys` 作为目标业务表或目标 repository 抽象。
- 将 public key / key metadata 建模为 `devices` 的字段或 JSON 属性，例如 `public_key`、`key_json` 或 `metadata_json` 中的 key 子结构。
- `DeviceRepository.PublicKey()` 如果仍需要存在，应从 `devices` 读取 device 属性，而不是 JOIN `device_keys`。
- 旧 `device_keys` 表直接删除：就地修改原 `202606280001_device_bindings.sql`，不再创建 `device_keys` 表与相关索引；public key / device_key 字段归入 `devices`。

本轮不引入新的长期 device credential 设计，也不改变现有 private key 文件生命周期。

### 4. Workspace model

`workspaces` 表保存 workspace 主记录：

- `id`：workspace id，主键。
- `device_id`：所属设备 id，外键到 `devices(id)`；local 单机也要写入当前 device id。
- `name`：展示名。
- `path`：normalized workspace path。
- `sort_order`：用户排序；替代 `index.json`。
- `metadata_json`：低频扩展字段。
- `created_at` / `updated_at` / `deleted_at`。

约束：

- active workspace 的 `(device_id, path)` 防重由 repository 业务逻辑在事务中完成，不使用数据库唯一索引。
- 数据库只提供普通查询索引；业务层负责识别重复并复用既有 active workspace。
- list 默认过滤 `deleted_at IS NULL`。

### 5. Session model

`sessions` 表保存 session 主记录与当前状态快照：

- `id`：session id，主键。
- `workspace_id`：外键到 `workspaces(id)`。
- `device_id`：冗余所属设备 id，便于按设备过滤和未来清理。
- `name`。
- `launch_cwd`。
- `command_json`：保存 `session.CommandRecord`。
- `history_json`：只保存现有 API 需要的 history config / metadata；不保存 terminal output。
- `current_state`。
- `current_state_reason`。
- `current_run_id`：可为空，指向最后一条 run。
- `sort_order`：workspace 内 session 排序；替代 workspace aggregate 的 `session_ids`。
- `created_at` / `updated_at` / `deleted_at`。

约束：

- list 默认过滤 deleted session。
- 软删除 workspace 时，repository 在事务中软删除其 sessions。
- session current state 是 UI 快照；运行事实保存在 `session_runs`。

### 6. Session run model

`session_runs` 表保存每次业务运行：

- `id`：run id，主键。
- `session_id`：外键到 `sessions(id)`。
- `workspace_id`：冗余，用于列表/清理。
- `device_id`：冗余，用于按设备过滤。
- `sequence`：同一 session 内递增序号。
- `command_json`：本次运行命令快照。
- `terminal_size_json`：启动尺寸等低频参数。
- `process_json`：保存 `process.Record`。
- `exit_json`：保存 `process.ExitRecord`。
- `state` / `state_reason`：本次 run 当前/最终状态。
- `started_at` / `ended_at`。
- `created_at` / `updated_at` / `deleted_at`。

约束：

- `(session_id, sequence)` 由 repository 在事务中业务防重，不使用唯一索引；遇到已有 active/current 记录按业务语义复用。
- 创建 rerun 时追加新 row，不能覆盖旧 row。
- session 的 `current_run_id` 总是指向该 session 最后一条未删除 run。
- 更新 process/exit/state 时同时维护 `sessions.current_*` 快照，保证列表 API 不需要每次聚合所有 runs。

### 7. Terminal history boundary

本轮保持 terminal history 现状，并严格区分 history 与 run record。具体含义：

- history 文件是终端输出；run record 是运行记录；两者不要混为一谈。
- 不新增 history/output chunks 表。
- 不把 terminal bytes 写入 `session_runs`。
- 不把 terminal history 纳入“设备、工作区、会话、运行记录入库”的验收范围。
- 当前 `History(workspaceId, sessionId)`、live replay、rerun archive 等 history 行为继续沿用现有实现。
- `history.log` 不是本轮运行记录业务表的一部分；它的去文件化应作为后续独立需求处理。

因此，本轮“不保留旧文件路径”仅约束设备业务记录、workspace/session/run 业务记录；不扩大解释到 history 文件和 private key 文件。

### 8. Repository abstraction

新增数据库 repository 建议放在：

- `internal/infrastructure/repository/runtime`，或
- 拆分为 `workspace` / `session` / `run` repository。

为了减少 application 层改动，repository 应实现或替代当前 domain 依赖的能力：

- `FindWorkspaceByPath`
- `FindWorkspaceById`
- `SaveWorkspace`
- `SaveSession`
- `LoadWorkspace`
- `LoadSession`
- `UpdateSession`
- `DeleteSession`（软删除）
- `DeleteWorkspace`（软删除 workspace + sessions + runs）
- `SaveState`
- `LoadState`
- `SaveProcess`
- `LoadProcess`
- `SaveExit`
- `LoadExit`
- `ListWorkspaces`
- `ListSessions`
- `ListSessionsByWorkspaceId`
- `UpdateWorkspaceOrder`
- `UpdateSessionOrder`

但方法语义要从“workspace aggregate JSON”改为 DB row + joins：

- `LoadWorkspace` 返回 domain workspace 时，children 可由 active sessions 映射。
- `ListSessions` 返回 `session.View`，state/exit 来自 `sessions.current_*` 和 current run。
- `SaveProcess` / `SaveExit` 操作 current run，而不是覆盖 session 主记录里的单个 JSON 聚合。

### 9. Application wiring

当前 `runServe` 已打开 DB 并执行 migration，但 `runExec` 还直接使用文件 `state.NewStore(cfg.Runtime.StateDir)`。规格要求：

- `runExec` 也必须打开 DB、执行 migration，并使用 DB runtime repository。
- `runServe` 的 terminal registry 使用 DB runtime repository，而不是 file state store。
- `agent.LoadOrCreateDevice` 不能再写 state dir；设备初始化应变为 repository upsert：从配置得到 device id/name，写入 `devices`。
- `gatewayapi` 的 device list / current device report 继续使用演进后的 device repository。

### 10. JSON storage rules

允许 JSON 字段保存复杂结构，但以下字段必须结构化，不能只藏在 JSON：

- 所有表主键、外键。
- `device_id`、`workspace_id`、`session_id`、`current_run_id`。
- 排序字段。
- lifecycle state / state reason。
- `created_at`、`updated_at`、`deleted_at`、`started_at`、`ended_at`。
- 常用列表展示字段：workspace name/path、session name/launch_cwd。

适合 JSON 的字段：

- `command_json`
- `history_json`（仅 config/metadata，不含 terminal output）
- `process_json`
- `exit_json`
- `terminal_size_json`
- `metadata_json`
- future capabilities / labels / low-frequency options

Repository 负责 marshal/unmarshal 并对无效 JSON 返回明确错误；不要把 JSON 解析错误吞成空对象。

## Affected components

| Component | Change |
|---|---|
| DB migrations | SQLite/MySQL 新增 runtime state migration；演进 `devices`，新增 `workspaces`、`sessions`、`session_runs`。 |
| Device repository | 复用现有 `internal/infrastructure/repository/device`，扩展设备业务字段；public key / device key metadata 作为 `devices` 属性读取，不再作为独立 `device_keys` 业务表写入。 |
| Runtime repository | 新增 DB-backed workspace/session/run repository，替代 `repository/state` 文件 store 主路径。 |
| Domain workspace/session/process | 尽量保持 domain struct 与 API contract；必要时新增 run domain struct。 |
| `internal/app/app.go` | `runExec` 与 `runServe` 都从 DB 初始化 runtime repository；不再用 `state.NewStore` 作为主路径。 |
| `internal/application/terminal` | registry/runtime 使用 DB store；rerun 追加 run record；history 文件 replay 退出本轮主路径。 |
| `internal/application/agent` | 保持 private key 文件能力现状；设备业务记录初始化改为配置 + DB upsert，不再以 `device.json` 作为业务主存储。 |
| `internal/transport/http/gatewayapi` | tunnel public key 校验保持行为，但 public key 来源调整为 `devices` 属性；设备列表继续以 DB 为准。 |
| Tests | 删除旧文件断言，新增 DB repository / app / terminal tests。 |

## Interfaces

### Runtime repository shape

建议接口按当前 application 需要组织，而不是暴露 SQL 细节：

```go
type Store interface {
    FindWorkspaceByPath(path string) (workspace.Workspace, error)
    FindWorkspaceById(workspaceId string) (workspace.Workspace, error)
    SaveWorkspace(workspace.Workspace) error
    SaveSession(session.Session) error
    LoadWorkspace(workspaceId string) (workspace.Workspace, error)
    LoadSession(workspaceId string, sessionId string) (session.Session, error)
    UpdateSession(workspaceId string, sessionId string, update func(*session.Session) error) (session.Session, error)
    DeleteSession(workspaceId string, sessionId string) error
    DeleteWorkspace(workspaceId string) error
    SaveState(workspaceId string, sessionId string, value session.StateRecord) error
    LoadState(workspaceId string, sessionId string) (session.StateRecord, error)
    SaveProcess(workspaceId string, sessionId string, value process.Record) error
    LoadProcess(workspaceId string, sessionId string) (process.Record, error)
    SaveExit(workspaceId string, sessionId string, value process.ExitRecord) error
    LoadExit(workspaceId string, sessionId string) (process.ExitRecord, error)
    ListWorkspaces() ([]workspace.Workspace, []Warning, error)
    ListSessionsByWorkspaceId(workspaceId string) ([]session.View, []Warning, error)
    ListSessions() ([]session.View, []Warning, error)
    UpdateWorkspaceOrder(workspaceIds []string, now time.Time) ([]workspace.Workspace, error)
    UpdateSessionOrder(workspaceId string, sessionIds []string, now time.Time) ([]session.View, []Warning, error)
}
```

Implementation 可以保留接口名兼容 application 层，但 concrete type 必须 DB-backed。

### Device initialization

建议设备初始化接口由 DB repository 承担：

```go
type DeviceRepository interface {
    UpsertLocalDevice(ctx context.Context, device Device) (Device, error)
    UpsertUserDevice(ctx context.Context, userId string, device Device) error
    ListDevicesForUser(ctx context.Context, userId string) ([]Device, error)
    PublicKey(ctx context.Context, deviceId string) (string, error)
}
```

`UpsertLocalDevice` 不写 `device.json`，只写业务设备记录。private key 文件能力保持现状；public key / device key metadata 作为 device 属性写入或读取。

## Migration design

### SQLite

新增 migration 例：`internal/infrastructure/database/migrations/sqlite/202606290001_runtime_state.sql`。

设计要求：

- 对现有 `devices` 增加业务扩展字段时要兼容 SQLite `ALTER TABLE` 限制；device key/public key 字段作为 `devices` 属性建模。
- 新增 `workspaces`、`sessions`、`session_runs`。
- 使用 `TEXT` 保存 timestamps / JSON。
- 开启 foreign keys 已由 `database.Open` 设置。
- Down migration 删除新增表；对 `devices` 新增列如果 SQLite 不便 drop，可在 Plan 中选择可接受策略或重建表。

### MySQL

新增 migration 例：`internal/infrastructure/database/migrations/mysql/202606290001_runtime_state.sql`。

设计要求：

- 使用 `DATETIME(6)` 保存时间。
- JSON 字段可以使用 `JSON` 类型；若为了跨驱动 scan 简化，也可使用 `LONGTEXT` + repository JSON validation，但必须在 Spec/Plan 保持一致。
- 外键使用 InnoDB。
- 索引覆盖列表查询：`device_id`、`workspace_id`、`session_id`、`sort_order`、`updated_at`、`deleted_at`。

## Technical questions

1. **History 与 runtime repository 的边界如何落地？**
   - 用户确认 history 文件是终端输出，run record 是运行记录，两者不要混为一谈；Plan 阶段需要确保只替换 workspace/session/run 业务记录，不误删现有 history replay 行为。
2. **业务防重复用如何统一实现？**
   - 用户确认不使用唯一索引；已有 active 记录时复用。Plan 阶段需要定义 repository 事务内查重与复用语义，覆盖 workspace path、session order、run sequence 等场景。

## Alternatives considered

### Alternative A：保留文件 store 并加 DB mirror

拒绝。用户明确要求不再基于文件、不保留、不向后兼容。双写会制造新的 authoritative source 冲突。

### Alternative B：新增 `runtime_devices`

拒绝。用户选择选项 A；新增第二套设备表会造成 cloud binding device 与 runtime device 概念分裂。

### Alternative C：把 workspace/session/run 全部放一个 JSON blob

拒绝。虽然实现简单，但无法可靠支持排序、软删除、多 run、按设备过滤与未来 cloud dashboard 查询。

### Alternative D：本轮同时实现 terminal history chunks

拒绝。用户明确“不处理 history，见 6”。history/output 是后续独立需求。

## Risks

1. **device key schema 调整风险**：用户要求旧 `device_keys` 表直接删除并就地修改原 SQL；需要同步调整 repository 和测试，避免仍有 JOIN/写入 `device_keys` 的路径残留。
2. **现有测试大面积失效风险**：当前 app/CLI/state/terminal 测试大量断言 `workspace.json`、`history.log` 或 private key 文件，需要系统性改写。
3. **history 边界风险**：本轮 history 保持现状；需要避免把“业务文件不保留”扩大到 history，从而误伤 workbench replay/rerun archive。
4. **软删除一致性风险**：workspace/session/run 软删除需要事务化，列表查询必须统一过滤 deleted rows。
5. **跨数据库 JSON 差异风险**：SQLite 与 MySQL JSON 类型和 scan 行为不同；repository 应统一当作 bytes/string 解析或明确类型策略。唯一性不依赖唯一索引，由业务防重承担。
6. **当前大 diff 叠加风险**：工作区已有 Cloud/Local mode 大 diff，本需求后续实现前建议先完成或隔离上一轮变更，避免混合提交边界不清。

## User review notes

- 2026-06-29：用户要求进入 Spec。已将 Requirement 标记为 Accepted，并基于现有文件 store、device key、terminal registry、DB migration 现状形成本规格草稿。
- 2026-06-29：用户修正规格决策：取消删除 private key，保持现状；history 保持现状；device_key 应作为 device 属性而不是独立表；不使用唯一索引，业务防重即可。
- 2026-06-29：用户补充规格决策：旧 `device_keys` 表直接删除，就地修改原 SQL；history 文件是终端输出，run record 是运行记录，两者不要混为一谈；业务防重遇到已有 active 记录时复用。
