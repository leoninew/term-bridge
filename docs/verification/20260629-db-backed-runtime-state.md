# 数据库存储设备、工作区、会话与运行记录验证
最后修改时间: 2026-06-29 14:36:30

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；验证 / Verification。

## Basis

- Requirement: `docs/requirement/20260629-db-backed-runtime-state.md`，Review status: Accepted
- Spec: `docs/spec/20260629-db-backed-runtime-state.md`，Review status: Accepted
- Plan: `docs/plan/20260629-db-backed-runtime-state.md`，Review status: Accepted

## Requirement alignment

### 数据模型

- [x] 复用并演进 `devices`，新增 `workspaces`、`sessions`、`session_runs`，形成四类核心业务表语义。
- [x] SQLite / MySQL migration 均覆盖 runtime state 表。
- [x] `devices.public_key` 承载 public key；`device_keys` 独立表已从原 device binding migration 删除。
- [x] workspace / session / run 表包含主键、外键、普通索引和软删除字段。
- [x] 未新增业务唯一索引；workspace path 复用、session run sequence 等由 repository 逻辑处理。
- [x] command/history/process/exit/terminal size 等复杂结构使用 JSON 字段。
- [x] session 支持多条 run；rerun 会追加新的 `session_runs` 并更新 `sessions.current_run_id`。

### Repository 与业务路径

- [x] `runExec`、`runServe`、workspace list、session list 已切到 DB-backed runtime store。
- [x] workspace/session/run 主写入路径不再产生 `workspace.json`、`state.json`、`process.json`、`exit.json`。
- [x] `history.log` 保持文件路径，继续用于 terminal output、replay 和 rerun archive，不进入 `session_runs` terminal bytes。
- [x] API response shape 保持当前 `SessionSummary` / `WorkspaceSessionSummary` 语义，列表展示仍读取当前 state/exit。
- [x] workspace/session 删除在 DB 中软删除，并同步 runs。

### 设备与 cloud/local 边界

- [x] local device 在 `exec` / `serve` / list 入口 upsert 到 DB。
- [x] cloud device report / list 继续走 device repository，public key 从 `devices.public_key` 读取。
- [x] private key 文件能力保留；tunnel signing 仍通过 `LoadDevicePrivateKey`。
- [x] `device.json` 不再作为 local device 业务记录写入路径。
- [x] 未引入新的 local direct backend，仍保持 self-tunnel / unified gateway 模型。

### 兼容与迁移

- [x] 不迁移旧 `.termbridge/workspaces` 业务 JSON，不读取旧 business JSON 作为 fallback。
- [x] 新空库启动、workspace/session/run 创建、state/process/exit、软删除、device upsert/list 有测试覆盖。
- [x] history 文件作为明确例外保留。

## Spec alignment

- `devices`：已加入 `public_key` 字段，repository 不再依赖 `device_keys`。
- `workspaces`：新增表与 `state.DBStore` 支持 create/reuse/list/order/soft delete。
- `sessions`：新增表保存 command/history JSON、current state/current run 快照、排序和软删除。
- `session_runs`：新增表保存 run sequence、command/process/exit/state/timestamps；rerun append 当前 run。
- history boundary：实现中 `HistoryPath()` 仍派生文件路径；run record 不保存 terminal bytes。
- application wiring：`internal/app/app.go` 已打开 DB、执行 migration、upsert local device，并将 terminal registry 接到 DB store。
- private key boundary：`internal/application/agent/device.go` 保留 private/public key 文件，移除 device business JSON 写入。

## Plan alignment

| Plan step | Result |
|---|---|
| Step 0：隔离当前大 diff 风险 | 未执行 git 写操作；验证中通过 diff summary 核对范围。 |
| Step 1：调整 device schema migration | 已完成 SQLite/MySQL 原 migration 修改，删除 `device_keys`，加入 `devices.public_key`。 |
| Step 2：新增 runtime state migration | 已新增 SQLite/MySQL `workspaces`、`sessions`、`session_runs` migration。 |
| Step 3：扩展 device repository | 已完成并更新测试。 |
| Step 4：新增 DB-backed runtime repository | 已以 `internal/infrastructure/repository/state/db_store.go` 落地。 |
| Step 5：调整 run record 表达 | 未新增 domain run 类型；在 repository 内部处理 current run / sequence，保持 domain/API 形状稳定。 |
| Step 6：调整 app wiring | 已完成 `exec`、`serve`、workspace/session list DB 接入。 |
| Step 7：调整 agent/device 初始化边界 | 已保留 key 文件，移除 `device.json` 业务记录写入。 |
| Step 8：调整 terminal registry/runtime | Registry 改为 store interface；DB store 支持 `BeginSessionRun`；history replay/archive 保持。 |
| Step 9：调整 HTTP/Gateway 使用路径 | gateway tests 更新为 `devices.public_key`。 |
| Step 10：删除或隔离 file state store 主路径 | 产品主路径已断开；旧 `state.Store` 仍保留供既有 terminal/state package tests 使用，后续可瘦身。 |
| Step 11：更新 app/CLI 测试 | 已更新 DB-backed assertions。 |
| Step 12：更新文档和配置说明 | SpecFlow 文档已更新；未发现本轮必须修改 README 的验证证据。 |

## Actual diff summary

按 `git diff HEAD --stat`，本轮相关变更包含 19 个文件，约 2544 行新增、150 行删除：

- 新增 SpecFlow 文档：requirement/spec/plan。
- 修改 app/agent/terminal wiring。
- 修改 device migrations 与新增 runtime state migrations。
- 修改 device repository 与新增 DB-backed state store。
- 更新 app、CLI、device、state、gateway API tests。

## Expected vs actual changed files

### Expected and changed

- `docs/requirement/20260629-db-backed-runtime-state.md`
- `docs/spec/20260629-db-backed-runtime-state.md`
- `docs/plan/20260629-db-backed-runtime-state.md`
- `internal/app/app.go`
- `internal/app/app_test.go`
- `internal/application/agent/client.go`
- `internal/application/agent/device.go`
- `internal/application/agent/device_test.go`
- `internal/application/terminal/registry.go`
- `internal/infrastructure/database/migrations/mysql/202606280001_device_bindings.sql`
- `internal/infrastructure/database/migrations/mysql/202606290001_runtime_state.sql`
- `internal/infrastructure/database/migrations/sqlite/202606280001_device_bindings.sql`
- `internal/infrastructure/database/migrations/sqlite/202606290001_runtime_state.sql`
- `internal/infrastructure/repository/device/repository.go`
- `internal/infrastructure/repository/device/repository_test.go`
- `internal/infrastructure/repository/state/db_store.go`
- `internal/infrastructure/repository/state/db_store_test.go`
- `internal/transport/cli/cli_test.go`
- `internal/transport/http/gatewayapi/cloud_binding_test.go`

### Expected but not changed / intentionally deferred

- `internal/infrastructure/repository/state/store.go`：未删除，因当前 terminal/state tests 仍使用旧 file store；产品主路径已通过 app wiring 迁移到 DB store。
- 前端文件：未修改，API response shape 未变化。

### Unexpected changed files

- 未发现本轮 verification 范围内的额外前端或配置文件改动。

## Acceptance checklist

- [x] SQLite / MySQL migration 覆盖四类核心业务表语义。
- [x] `device_keys` 不再作为目标业务表。
- [x] 设备 public key 是 `devices` 属性。
- [x] runtime state 主路径进入 DB。
- [x] 不写旧 workspace/session/process/exit business JSON。
- [x] history 文件保持 terminal output 语义。
- [x] private key 文件能力保持。
- [x] rerun 追加 run record。
- [x] soft delete workspace/session/runs。
- [x] Go internal 测试通过。
- [x] whitespace diff check 通过。

## Test results

### Passed

```text
go test ./internal/...
```

结果：通过。覆盖包包括 app、agent、terminal、device repository、state repository、CLI、gateway API 等。

```text
git diff --check
```

结果：通过，无输出。

### Earlier failures fixed during verification

- `go test ./internal/infrastructure/repository/state` 初次失败：SQLite `:memory:` 多连接导致 schema 不可见，后续测试 DB 设置 `SetMaxOpenConns(1)` 并调整 nested query 关闭 rows 后通过。
- `go test ./internal/app` 初次失败：默认 exec/list 没有生成 device id/name，修正为相关命令入口调用 `config.EnsureLocalIdentity` 后通过。
- `go test ./internal/...` 初次失败：CLI / gateway tests 仍断言旧 workspace JSON 或旧 devices schema，更新为 DB-backed assertions 与 `devices.public_key` 后通过。

### Not run

- Frontend `npm --prefix web ...` checks 未运行。本轮未修改前端文件，API response shape 保持兼容；如提交前需要覆盖整个工作区已有前端 diff，应单独运行前端测试、typecheck、lint、format check。
- MySQL live migration 未连接真实 MySQL 执行；仅通过 migration SQL 与 repository driver branch 代码审查覆盖。SQLite migration 已由 Go tests 间接覆盖。

## Missed or expanded scope

- 未将 terminal history/output 入库，符合用户明确边界。
- 未迁移旧 `.termbridge` 业务 JSON，符合用户明确决策。
- 未删除旧 `state.Store` 文件实现；这是保守处理，避免一次性重写大量 terminal unit tests。产品主路径已从 app wiring 断开，旧 store 后续可作为独立清理项删除或瘦身为 history path helper。
- 未实现云同步、跨设备冲突合并、history chunking，均为 non-goal。

## Risks and follow-ups

1. **旧 `state.Store` 残留风险**：虽然产品入口已切到 DB store，但测试仍大量使用旧 file store。后续应单独清理 terminal tests，或把 history path helper 从旧 store 中抽离。
2. **MySQL 实机验证缺口**：MySQL migration 使用 JSON 字段，repository 会显式插入 JSON payload；但本轮未连接 MySQL 实例跑 migration。
3. **schema drift 风险**：按用户要求就地修改 `202606280001_device_bindings.sql`。如果该 migration 已在任何环境执行过，需要单独处理已部署环境 schema drift。
4. **并发防重风险**：用户要求不使用唯一索引；repository 事务内复用 active row，但极端并发下仍需后续压力测试验证 SQLite/MySQL 行为。

## Conclusion

Verification 结论：通过。当前实现与已接受的 Requirement / Spec / Plan 对齐，核心 Go 测试与 diff whitespace 检查通过。剩余风险主要是旧 file store 测试残留、MySQL 实机 migration 未跑，以及就地修改未发布 migration 的 schema drift 风险。