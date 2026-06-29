# 数据库存储设备、工作区、会话与运行记录计划
最后修改时间: 2026-06-29 14:33:30

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；计划 / Plan。

## Basis

- Requirement: `docs/requirement/20260629-db-backed-runtime-state.md`，Review status: Accepted
- Spec: `docs/spec/20260629-db-backed-runtime-state.md`，Review status: Accepted

## Implementation principles

1. 数据库是设备、工作区、会话、运行记录的 authoritative storage。
2. 不迁移旧 `.termbridge` 业务数据，不读取旧业务 JSON 作为 fallback。
3. `history.log` 是终端输出，保持现状；run record 是业务运行记录，不保存 terminal bytes。
4. `private_key.pem` / `public_key.pem` 保持现状；本轮不删除 private key 能力。
5. `device_key` / public key 是 `devices` 的属性，不再作为独立 `device_keys` 表。
6. 不使用唯一索引；repository 在事务中业务防重，遇到已有 active 记录时复用。
7. workspace/session 使用软删除，列表默认过滤 `deleted_at IS NULL`。
8. 不引入新的 local direct backend，不改变 self-tunnel / unified gateway 模型。

## Implementation steps

### Step 0：隔离当前大 diff 风险

- 在开始产品代码实现前，先确认当前工作区已有 Cloud/Local mode 大 diff 是否已经准备好作为同一批变更继续叠加。
- 不执行 `git add` / `git commit` / `git push`。
- 如果实现阶段发现上一轮 diff 与本轮 runtime DB 改造相互影响，优先报告边界冲突，不静默扩大范围。

### Step 1：调整 device schema migration

修改既有原始 device binding migration，而不是新增迁移处理 `device_keys`：

- `internal/infrastructure/database/migrations/sqlite/202606280001_device_bindings.sql`
- `internal/infrastructure/database/migrations/mysql/202606280001_device_bindings.sql`

计划改动：

1. 从原 SQL 删除 `device_keys` 表创建语句。
2. 从 Down migration 删除 `DROP INDEX idx_device_keys_device` / `DROP TABLE IF EXISTS device_keys` 等相关语句。
3. 在 `devices` 表增加 public key / key metadata 属性字段，建议：
   - SQLite：`public_key TEXT NOT NULL DEFAULT ''`，`key_json TEXT NOT NULL DEFAULT '{}'` 或只保留 `public_key`。
   - MySQL：`public_key TEXT NOT NULL` 或 `public_key TEXT NOT NULL DEFAULT ('')` 视 MySQL 兼容性选择；`key_json JSON` 或 `LONGTEXT` 需与 repository 策略一致。
4. 不新增唯一索引；只添加普通查询索引，如设备更新时间、名称或未来需要的字段。

注意：这是用户明确要求“旧 `device_keys` 表直接删除，就地修改原 SQL”。由于该 migration 尚处当前开发 diff 阶段，允许按本需求直接修改历史草稿 SQL；如果后续已经发布，则需要另起兼容迁移，但本轮按未发布处理。

### Step 2：新增 runtime state migration

新增：

- `internal/infrastructure/database/migrations/sqlite/202606290001_runtime_state.sql`
- `internal/infrastructure/database/migrations/mysql/202606290001_runtime_state.sql`

表设计：

#### `workspaces`

字段建议：

- `id`：TEXT / VARCHAR(32)，主键。
- `device_id`：TEXT / VARCHAR(128)，非空，外键到 `devices(id)`。
- `name`：TEXT / VARCHAR(255)，非空。
- `path`：TEXT / VARCHAR(1024)，非空。
- `sort_order`：INTEGER / BIGINT，非空默认 0。
- `metadata_json`：TEXT / JSON，非空默认 `{}`。
- `created_at`：TEXT / DATETIME(6)，非空。
- `updated_at`：TEXT / DATETIME(6)，非空。
- `deleted_at`：TEXT / DATETIME(6)，可空。

普通索引：

- `(device_id, deleted_at)`
- `(device_id, path)`，非唯一，用于业务防重复用。
- `(device_id, sort_order)`
- `(updated_at)`

#### `sessions`

字段建议：

- `id`：TEXT / VARCHAR(32)，主键。
- `workspace_id`：TEXT / VARCHAR(32)，非空，外键到 `workspaces(id)`。
- `device_id`：TEXT / VARCHAR(128)，非空，冗余用于查询。
- `name`：TEXT / VARCHAR(255)，非空。
- `launch_cwd`：TEXT / VARCHAR(1024)，非空。
- `command_json`：TEXT / JSON，非空。
- `history_json`：TEXT / JSON，非空，仅保存 history config / metadata，不保存终端输出。
- `current_state`：TEXT / VARCHAR(32)，非空。
- `current_state_reason`：TEXT / VARCHAR(255)，非空默认空。
- `current_run_id`：TEXT / VARCHAR(32)，可空。
- `sort_order`：INTEGER / BIGINT，非空默认 0。
- `created_at`：TEXT / DATETIME(6)，非空。
- `updated_at`：TEXT / DATETIME(6)，非空。
- `deleted_at`：TEXT / DATETIME(6)，可空。

普通索引：

- `(workspace_id, deleted_at)`
- `(workspace_id, sort_order)`
- `(device_id, deleted_at)`
- `(current_run_id)`

#### `session_runs`

字段建议：

- `id`：TEXT / VARCHAR(32)，主键。
- `session_id`：TEXT / VARCHAR(32)，非空，外键到 `sessions(id)`。
- `workspace_id`：TEXT / VARCHAR(32)，非空。
- `device_id`：TEXT / VARCHAR(128)，非空。
- `sequence`：INTEGER / BIGINT，非空。
- `command_json`：TEXT / JSON，非空。
- `terminal_size_json`：TEXT / JSON，非空默认 `{}`。
- `process_json`：TEXT / JSON，可空或非空默认 `{}`。
- `exit_json`：TEXT / JSON，可空或非空默认 `{}`。
- `state`：TEXT / VARCHAR(32)，非空。
- `state_reason`：TEXT / VARCHAR(255)，非空默认空。
- `started_at`：TEXT / DATETIME(6)，可空。
- `ended_at`：TEXT / DATETIME(6)，可空。
- `created_at`：TEXT / DATETIME(6)，非空。
- `updated_at`：TEXT / DATETIME(6)，非空。
- `deleted_at`：TEXT / DATETIME(6)，可空。

普通索引：

- `(session_id, sequence)`，非唯一；repository 事务内防重。
- `(session_id, deleted_at)`
- `(workspace_id, deleted_at)`
- `(device_id, deleted_at)`
- `(updated_at)`

### Step 3：扩展 device repository

修改：

- `internal/infrastructure/repository/device/repository.go`
- `internal/infrastructure/repository/device/repository_test.go`

计划改动：

1. `Device` struct 增加/确认 `PublicKey` 作为 `devices` 属性字段，不再依赖 `device_keys`。
2. `upsertDevice` 写入 `devices.public_key` 和更新时间。
3. `PublicKey(ctx, deviceId)` 从 `devices.public_key` 读取，不再查 `device_keys`。
4. `UpsertDeviceBinding` / `UpsertUserDevice` 不再 insert/delete `device_keys`。
5. 新增 `UpsertLocalDevice(ctx, device Device)` 或等价方法，用于 local/serve/exec 初始化设备业务记录。
6. 测试覆盖：
   - upsert 设备 public key 写在 `devices`。
   - `PublicKey` 从 `devices` 读取。
   - 不存在 `device_keys` 表时测试仍通过。
   - user-device 绑定仍可 list/delete。

### Step 4：新增 DB-backed runtime repository

新增建议路径：

- `internal/infrastructure/repository/runtime/repository.go`
- `internal/infrastructure/repository/runtime/repository_test.go`

如果为了减少包名冲突，也可以命名为：

- `internal/infrastructure/repository/state/db_store.go`

但推荐新包 `runtime`，并逐步让 application 层依赖新的 DB-backed concrete type。

实现能力：

- `FindWorkspaceByPath`
- `FindWorkspaceById`
- `SaveWorkspace`
- `SaveSession`
- `LoadWorkspace`
- `LoadSession`
- `UpdateSession`
- `DeleteSession`（软删除）
- `DeleteWorkspace`（事务内软删除 workspace + sessions + runs）
- `SaveState`
- `LoadState`
- `SaveProcess`
- `LoadProcess`
- `SaveExit`
- `LoadExit`
- `ListWorkspaces`
- `ListSessionsByWorkspaceId`
- `ListSessions`
- `UpdateWorkspaceOrder`
- `UpdateSessionOrder`
- `HistoryPath` 保持现状所需能力：返回现有 history 文件路径，但不把 history 视为 run record。

关键语义：

1. `FindWorkspaceByPath`：按当前 device + normalized path 查 active workspace；存在则复用并更新 `updated_at`。
2. `SaveWorkspace`：如 active `(device_id, path)` 已存在，复用已有 row；不创建重复 active workspace。
3. `SaveSession`：创建 session row，并创建第一条 `session_runs`，设置 `sessions.current_run_id` 指向该 run。
4. `SaveProcess`：更新 current run 的 `process_json`、`state`，同步 `sessions.current_state`。
5. `SaveExit`：更新 current run 的 `exit_json`、`ended_at`、`state`，同步 `sessions.current_state/current_state_reason`。
6. Rerun：追加新 `session_runs` row，sequence = 当前 max + 1；更新 `sessions.current_run_id`。
7. 删除：软删除，不物理删 row；列表默认过滤。
8. JSON marshal/unmarshal 错误必须显式返回，不吞错。

### Step 5：调整 domain/application 对 run record 的表达

涉及：

- `internal/domain/session/session.go`
- `internal/domain/process/record.go`
- `internal/application/terminal/registry.go`
- `internal/application/terminal/runtime.go`

计划：

1. 如现有 domain 类型足够，优先不新增大抽象。
2. 如果需要表达 run id / sequence，可新增 `internal/domain/session/run.go` 或在 repository 内部 model 处理。
3. 保持 API response shape 不变：`SessionSummary` / `WorkspaceSessionSummary` 仍展示当前 run 的 state/exit。
4. `RerunSession` 不再通过 archive history 推导运行记录；它应调用 repository 追加 run，然后启动 runtime。
5. 保持 `history.log` 现状：history writer、replay、rerun archive 行为不纳入 run record。

### Step 6：调整 app wiring

修改：

- `internal/app/app.go`

计划：

1. `runExec` 打开 DB、执行 migration、初始化 device repository 与 runtime repository。
2. `runExec` 不再使用 `state.NewStore(cfg.Runtime.StateDir)` 写 workspace/session/run 业务记录。
3. `runServe` 继续打开 DB；registry 使用 DB-backed runtime repository。
4. `runServe` 调用 device repository upsert 当前 device 业务记录；保留 private key 文件初始化/读取现状。
5. 保证 `history.NewWriter(store.HistoryPath(...))` 仍可获得路径，history 行为不变。

### Step 7：调整 agent/device 初始化边界

修改：

- `internal/application/agent/device.go`
- `internal/application/agent/client.go`
- 相关 tests

计划：

1. 保留 private key 文件能力，不删除 `LoadDevicePrivateKey` / tunnel signing。
2. 停止把 `device.json` 作为设备业务记录主存储。
3. 如果 `LoadOrCreateDevice` 当前同时负责 device.json 与 key 生成，拆分职责：
   - key 文件 load/create 保持；
   - device business record 通过 DB repository upsert。
4. 现有调用方若需要 `agent.Device`，从配置 + public key 构造，再写 DB。
5. 测试调整为不再断言 `device.json` 必须存在；仍可断言 private/public key 文件按现状存在。

### Step 8：调整 terminal registry/runtime

修改：

- `internal/application/terminal/registry.go`
- `internal/application/terminal/runtime.go`
- 相关 gateway API terminal tests

计划：

1. Registry 的 `Store` 字段切换到 DB-backed runtime repository。
2. `CreateSession` 创建 session 后应已有第一条 current run。
3. `RerunSession` 追加新 run；history archive 保持现状，仅处理终端输出文件，不作为业务 run record。
4. `CloseSession` / missing runtime close 更新 current run + session snapshot。
5. `History` 继续读 history 文件；该路径不参与本轮 DB 运行记录。
6. 列表、树、排序、编辑、删除全部从 DB repository 读取/更新。

### Step 9：调整 HTTP/Gateway 使用路径

涉及：

- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/*test.go`

计划：

1. device list/current device report 继续使用 DB device repository。
2. tunnel public key 校验通过 `DeviceRepository.PublicKey()` 从 `devices.public_key` 读取。
3. 删除对 `device_keys` 表的隐式假设。
4. workspace/session API response shape 尽量保持不变。

### Step 10：删除或隔离 file state store 主路径

涉及：

- `internal/infrastructure/repository/state/store.go`
- `internal/infrastructure/repository/state/store_test.go`

计划：

1. 如果 DB runtime repository 完整替换后不再需要 `state.Store`，删除该文件与测试。
2. 如果 history path helper 暂时仍依赖 `StateDir` 组织路径，可以提取一个小的 `HistoryPath` helper，不保留 workspace/session JSON store 能力。
3. 删除旧文件兼容测试，如 `TestStoreIgnoresLegacySessionFragments`。
4. 新增 DB repository tests 替代旧 state store tests。

### Step 11：更新 app/CLI 测试

涉及：

- `internal/app/app_test.go`
- `internal/transport/cli/cli_test.go`
- `internal/application/terminal/*test.go`
- `internal/transport/http/gatewayapi/*test.go`

计划：

1. 不再断言 `.termbridge/workspaces/*/workspace.json` 存在。
2. 断言 SQLite DB 中存在 workspace/session/session_runs/devices 记录。
3. 保留 history 文件相关测试，只要测试目的确实是终端输出 replay/archive。
4. 保留 private key 文件现状测试，但移除 device.json 业务记录依赖。
5. 增加 rerun 多 run 测试：同一 session 多条 run，最后一条 current。
6. 增加软删除测试：删除后列表不可见，DB row 有 `deleted_at`。
7. 增加业务防重复用测试：同一 active device/path 重复 resolve 复用 workspace。

### Step 12：更新文档和配置说明

可能涉及：

- `.termbridge.default.yaml`
- `.env.example`
- README / docs 中有关 `.termbridge/workspaces` 的说明（如存在）

计划：

1. 如文档提到 workspace/session 存储在 `.termbridge/workspaces`，更新为 DB-backed runtime state。
2. 明确 history 文件仍是终端输出，不是 run record。
3. 明确 private key 文件保持现状。

## Files to change

### Migrations

- `internal/infrastructure/database/migrations/sqlite/202606280001_device_bindings.sql`
- `internal/infrastructure/database/migrations/mysql/202606280001_device_bindings.sql`
- `internal/infrastructure/database/migrations/sqlite/202606290001_runtime_state.sql`
- `internal/infrastructure/database/migrations/mysql/202606290001_runtime_state.sql`

### Repository

- `internal/infrastructure/repository/device/repository.go`
- `internal/infrastructure/repository/device/repository_test.go`
- `internal/infrastructure/repository/runtime/repository.go`（新增，或等价路径）
- `internal/infrastructure/repository/runtime/repository_test.go`（新增）
- `internal/infrastructure/repository/state/store.go`（删除、瘦身或退出主路径）
- `internal/infrastructure/repository/state/store_test.go`（删除或替换）

### Domain / Application

- `internal/domain/workspace/workspace.go`
- `internal/domain/session/session.go`
- `internal/domain/session/manager.go`
- `internal/domain/workspace/resolver.go`
- `internal/application/agent/device.go`
- `internal/application/agent/client.go`
- `internal/application/terminal/registry.go`
- `internal/application/terminal/runtime.go`
- `internal/app/app.go`

### Transport / Tests

- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/server_test.go`
- `internal/transport/http/gatewayapi/tunnel_test.go`
- `internal/transport/http/gatewayapi/terminal_e2e_test.go`
- `internal/transport/cli/cli_test.go`
- `internal/app/app_test.go`

### Docs / Config as needed

- `.termbridge.default.yaml`
- `.env.example`
- docs mentioning file-backed workspace/session state

## Verification plan

### Targeted repository tests

Run after repository implementation:

```text
go test ./internal/infrastructure/repository/device ./internal/infrastructure/repository/runtime
```

Expected coverage:

- `devices.public_key` stores device key property.
- no `device_keys` table dependency.
- workspace create/reuse/list/order/soft delete.
- session create/list/update/order/soft delete.
- session run append/current run/state/process/exit.
- JSON marshal/unmarshal failure paths.

### Application and terminal tests

```text
go test ./internal/application/terminal ./internal/app
```

Expected coverage:

- `runExec` writes DB runtime records.
- serve registry lists DB-backed workspaces/sessions.
- rerun appends run record.
- history remains terminal output path and is not confused with run record.

### Gateway/API tests

```text
go test ./internal/transport/http/gatewayapi
```

Expected coverage:

- device public key lookup reads from `devices` property.
- device list/current report still works.
- workspace/session API response shape remains compatible.

### Broader Go tests

```text
go test ./internal/...
```

### Frontend checks

Frontend behavior should not change, but if API contract or test fixtures shift:

```text
npm --prefix web test -- --run
npm --prefix web run typecheck
npm --prefix web run lint
npm --prefix web run format:check
```

### Diff hygiene

```text
git diff --check
```

## Blockers

当前没有需要用户再次决策的 Plan 阻塞项。以下事项是实现时必须遵守的约束，不再作为 open question：

- 旧 `device_keys` 表直接从原 SQL 删除。
- history 文件保持终端输出语义，不入 run record。
- private key 文件能力保持现状。
- 防重时复用已有 active 记录。
- 不使用唯一索引。
- 不迁移旧 `.termbridge` 业务数据。

## Assumptions

1. 当前 device binding migration 尚未作为稳定发布迁移对外固化，因此可按用户要求就地修改原 SQL。
2. runtime DB repository 可以继续使用当前 domain `workspace.Workspace`、`session.Session`、`session.View`、`process.Record`、`process.ExitRecord`，不需要先重构整个 domain。
3. `history.log` 当前路径可继续由 state dir 派生；这不违反本轮“业务记录不基于文件”的要求，因为 history 是终端输出。
4. private/public key 文件继续作为 tunnel signing material；设备业务记录入库不等于 secret material 入库。
5. SQLite 是 local 默认 DB，MySQL 是 cloud/部署 DB；两者 migration 和 repository 行为都需要通过测试或等价覆盖。

## Risks

1. **迁移就地修改风险**：如果 `202606280001_device_bindings.sql` 已被某个环境执行过，直接修改会造成 schema drift；当前按未发布开发阶段处理。
2. **history 边界误伤风险**：替换 file state store 时容易误删 `HistoryPath` / replay / archive 逻辑，需要保留 history 文件能力。
3. **private key 边界误伤风险**：删除 `device.json` 业务路径时不能误删 private/public key 文件路径。
4. **无唯一索引并发风险**：业务防重需要事务处理；并发创建同一 workspace/path 或 run sequence 时可能出现重复，需要 repository 层加锁/事务策略。
5. **大范围测试重写风险**：旧测试大量断言文件结构，实施时需要同时更新测试语义，避免用旧文件断言掩盖 DB 主路径。
6. **当前工作区已有大 diff**：本计划后续实现会叠加到已有 Cloud/Local mode 变更上，提交前需要特别审查边界。

## Rollback

如果实现过程中发现 DB-backed runtime repository 无法在本轮稳定落地：

1. 保留 migration 和 repository work-in-progress 在工作区，不提交。
2. 回退 application wiring 到当前 file store 仅作为开发回滚，不作为产品最终方案。
3. 更新 Plan 或 Spec 记录阻塞原因，重新切分为较小阶段，例如先完成 device schema + repository，再完成 workspace/session/run。
4. 不通过恢复旧 `.termbridge` 兼容路径来规避需求；旧文件主路径不是可接受的最终 rollback 方案。

## User review notes

- 2026-06-29：用户要求“开始计划”。已将 Spec 标记为 Accepted，并创建本 Plan 草稿。
