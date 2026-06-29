# 数据库存储设备、工作区、会话与运行记录需求
最后修改时间: 2026-06-29 13:08:34

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；需求 / Requirement 已接受，当前进入规格 / Spec。

## Background

TermBridge 后端已经引入数据库存储，用于用户、认证状态、OAuth state、设备绑定等能力。当前 runtime 状态仍存在明显的文件存储路径：

- 本机设备 identity、device key、cloud binding pending state / connection 摘要仍在 state dir 下使用文件。
- workspace / session / state / process / exit 聚合主要通过 `.termbridge/workspaces/.../workspace.json` 与 `index.json` 管理。
- terminal history 目前通过 session 目录下的 `history.log` 文件读写、归档。
- 运行过程中的 process / exit 信息当前嵌入 workspace/session JSON 聚合中，而不是作为独立运行记录持久化。

这导致数据模型和产品模型开始不一致：云端账号、设备绑定已经进入数据库，但设备、工作区、会话和运行记录仍依赖本地文件结构；后续 cloud/local 统一 Gateway、自连接、云端设备视图、session 管理和历史查询都会被文件存储边界限制。

本需求要求将这四类核心运行数据迁移到数据库，以数据库作为 authoritative storage；该使用 JSON 保存的复杂结构仍可使用 JSON 字段保存，但不再把 `.termbridge` 下的业务 JSON 文件作为主存储。

## Goal

1. 新增 4 张业务表，分别承载设备、工作区、会话、运行记录四类核心数据。
2. 设备、工作区、会话、运行记录的新增、更新、查询、删除以数据库为主，不再以文件 JSON 作为主数据源。
3. 保留必要的 JSON 字段存储复杂结构，例如 command、history config、session state、process/exit detail、metadata 或运行输出摘要等，避免过度拆表。
4. 保持当前 domain model 与 API contract 的产品语义稳定：前端和 HTTP API 不应因为存储切换而出现不兼容响应。
5. 为 local mode 与 cloud mode 使用同一套持久化抽象打基础，避免再出现“本地文件一套、云端数据库一套”的分裂。
6. 工作区、会话、运行记录之间要有清晰外键或逻辑归属，保证数据完整性和可清理性。
7. 运行记录应能够表达一个 session 的多次 run / rerun，而不是只覆盖当前最后一次 process / exit 聚合。
8. 本轮保持 terminal history 文件/输出历史现状；运行记录是业务记录，不等同于 terminal history。
9. SQLite 与 MySQL migration 均需要覆盖，并且 repository 层要处理两种数据库的时间、JSON、upsert 差异。
10. 现有文件存储路径不保留、不向后兼容；新实现不能继续读取或写入旧业务文件作为兼容路径。

## Non-goal

1. 本需求阶段不写产品代码。
2. 本需求不做旧 `.termbridge` 文件数据兼容或迁移；旧文件数据不作为新版本运行依据。
3. 本需求不要求改变前端页面 IA 或 API response shape，除非发现当前 API 与数据库模型存在不可避免冲突。
4. 本需求不要求引入新的数据库产品；继续基于项目已有 database/migration/repository 体系支持 SQLite 与 MySQL。
5. 本需求不要求把所有字段完全范式化；复杂且低频查询的数据允许 JSON 存储。
6. 本需求不处理认证用户、OAuth、email delivery、device binding code 等既有 auth schema 的重构，除非与设备表冲突必须协调。
7. 本需求不要求立即实现云端同步、跨设备复制或冲突合并策略。
8. 本需求不以“还能读旧 JSON 文件”为成功标准；成功标准是数据库路径成为主路径。

## User scenarios

### Scenario 1：本地用户创建 workspace 与 session

1. 用户在 local dashboard / CLI 中创建或打开 workspace。
2. 系统把 workspace 写入数据库的 workspace 表，而不是写入 `.termbridge/workspaces/<id>/workspace.json`。
3. 用户启动 session。
4. 系统把 session 写入数据库的 session 表，并通过 workspace_id 建立归属。
5. 前端列出 workspaces / sessions 时，从 repository 查询数据库并返回与现有 API 兼容的数据。

### Scenario 2：session 启动并产生运行记录

1. 用户启动一个 session command。
2. process started 信息写入运行记录表。
3. process exit / forced close / wait error 等结束信息继续更新同一条或关联的运行记录。
4. 如果用户 rerun，同一个 session 下生成新的 run record，而不是覆盖历史 run。
5. UI 仍能展示当前状态、exit code、exit reason 和命令文本。

### Scenario 3：session 多次运行记录

1. 用户对同一个 session 执行首次运行。
2. 系统创建一条 run record，记录该次业务运行的 command、process started、state、exit 等信息。
3. 用户 rerun 同一个 session。
4. 系统追加新的 run record，而不是覆盖上一条运行记录。
5. 系统以该 session 下最后一条未删除 run record 作为当前运行记录。

### Scenario 4：设备上报与绑定

1. local backend 上报当前设备。
2. cloud backend 将设备信息写入设备表，并建立 user-device 关系或复用既有绑定表语义。
3. 设备 public key、capabilities、metadata 等复杂或可扩展字段可使用 JSON / 独立字段组合保存。
4. Cloud dashboard 查询设备列表时从数据库读取设备状态与绑定关系，不依赖本地 connection JSON。

### Scenario 5：删除与清理

1. 用户删除 session。
2. 系统删除或软删除 session 记录，并按策略处理其 run records / history。
3. 用户删除 workspace。
4. 系统删除或软删除 workspace 下的 sessions 和 run records，避免残留孤儿数据。
5. 清理行为要在数据库事务或明确的补偿策略中完成。

## Acceptance

### 数据模型

- [ ] 新增且迁移 SQLite / MySQL 的 4 张核心业务表：设备、工作区、会话、运行记录。
- [ ] 表结构表达必要主键、外键和索引，避免 workspace/session/run/device 之间出现孤儿数据；业务唯一性由 repository 防重，不依赖唯一索引。
- [ ] 复杂结构可以使用 JSON 字段，但必须明确哪些字段用于查询、排序、过滤，哪些字段仅作为 JSON payload。
- [ ] 运行记录支持一个 session 多次 run / rerun。
- [ ] 时间字段统一使用 UTC，并保持 SQLite / MySQL repository 行为一致。

### Repository 与业务路径

- [ ] 设备、工作区、会话、运行记录的读写路径经由 repository 层访问数据库。
- [ ] `.termbridge/workspaces/**/workspace.json`、`state.json`、`process.json`、`exit.json` 不再作为 workspace/session/run 业务主存储写入目标；`history.log` 保持现状，不纳入本轮运行记录入库范围。
- [ ] 当前 workspace/session API 返回语义保持兼容。
- [ ] CLI `exec` / `serve` / terminal runtime 等入口使用同一套数据库持久化抽象。
- [ ] 软删除 workspace/session 时保持数据库一致性，不留下无法访问的 run 业务记录；history 保持现状。

### 设备与 cloud/local 边界

- [ ] 设备记录进入数据库主路径，cloud dashboard 设备列表以数据库为准。
- [ ] 本地 device private key 保持当前能力；device key 作为 device 属性建模，不作为独立业务表。
- [ ] 本地 cloud connection summary JSON 不再承担“设备是否已绑定”的 authoritative 语义。
- [ ] 不破坏既有 self-tunnel / unified gateway 模型，不引入新的 local direct backend。

### 兼容与迁移

- [ ] Spec / Plan 阶段明确是否需要读取旧 `.termbridge` 文件并迁移到 DB；如果不做自动迁移，也要给出产品行为和风险说明。
- [ ] 若保留旧文件读取逻辑，只能作为 migration/import fallback，不应在新写入路径继续产生旧业务 JSON。
- [ ] 测试覆盖空库启动、创建 workspace/session/run、更新 state/exit、删除清理、设备 upsert/list 等关键路径。

## Open questions

1. “新增 4 张表”中的设备表如何与当前已存在的 `devices` / `user_devices` / `device_keys` / `device_binding_codes` 协调？
   - 选项 A：复用并演进现有 `devices` 作为 4 张表之一，只新增 workspace/session/run 三张表。
   - 选项 B：新增 runtime_devices 表，与既有 cloud binding devices 表分离。
   - 当前倾向：优先复用/演进现有 `devices`，避免重复设备概念；但需要在 Spec 阶段确认是否仍符合“新增4张表”的表述。
2. terminal history / history.json / history.log 是否纳入本轮？
   - 用户决策：history 保持现状；本轮运行记录是业务记录，不是 terminal history 持久化。
3. 旧 `.termbridge/workspaces` 数据是否需要自动迁移？
   - 用户决策：不向后兼容，不迁移旧数据。
4. 本地 device private key 是否继续保留文件存储？
   - 用户决策更新：保留当前本地 device private key 能力，不在本轮删除。
   - 现状核对：当前代码存在 `internal/application/agent/device.go` 中的 `PrivateKeyFileName = "private_key.pem"`、`LoadDevicePrivateKey`，以及 `internal/application/agent/client.go` 的 tunnel signing 调用。本轮应保持该路径行为。
5. workspace/session 删除采用硬删除还是软删除？
   - 用户决策：软删除。
6. run record 与 current session state 的关系如何定义？
   - 用户决策：运行记录是业务记录；每个 session 有多条运行记录，其中最后一条是当前运行记录。

## Decisions

- 本需求为新的 SpecFlow 任务，不沿用 `20260628-cloud-local-mode-enrollment` 文档；该文档只作为背景上下文。
- 数据库是设备、工作区、会话、运行记录的 authoritative storage。
- 允许 JSON 字段保存复杂结构，但不得用文件 JSON 替代数据库主存储。
- history 文件是终端输出，不是 run record；run record 只表达业务运行记录。
- 不保留旧文件存储路径，不向后兼容，不迁移旧 `.termbridge` 数据。
- 设备表采用选项 A：复用并演进现有 `devices`，避免新增第二套设备概念。
- 本轮保持 terminal history / history.json / history.log 现状；运行记录是业务记录。
- 每个 session 有多条运行记录，最后一条是当前运行记录。
- workspace/session 删除采用软删除。
- 保留当前本地 device private key 能力，不在本轮删除；但设备业务记录仍进入数据库主路径。
- 不引入新的 local direct backend，不改变 self-tunnel / unified gateway 模型。
- 本阶段只形成需求草稿，不进入 Spec / Plan / Implementation。

## Risk

1. **现有 schema 冲突风险**：项目已有 `devices` 相关表，直接“新增设备表”可能造成两个设备概念并存，需要在 Spec 阶段统一命名和边界。
2. **history 边界风险**：用户已确认本轮 history 保持现状；需要防止运行记录入库被误解为 history 入库，也不能误伤现有 history 行为。
3. **迁移风险**：已有 `.termbridge` 文件数据如何处理会影响用户升级体验；不迁移会丢历史视图，自动迁移则增加实现复杂度。
4. **事务一致性风险**：session state、process started、exit record 写入跨多个操作，若事务边界不清会出现 UI 状态与运行事实不一致；history 保持现状，不纳入本轮事务模型。
5. **跨数据库差异风险**：SQLite 与 MySQL 对 JSON、upsert、时间和外键行为存在差异，migration 和 repository 需要显式覆盖；唯一性由业务防重承担，不依赖数据库唯一索引。
6. **安全边界风险**：device private key 这类敏感材料不应因为“设备入库”被误写入普通数据库；需要区分业务记录与 secret material。
7. **范围膨胀风险**：如果同时做自动迁移、history chunking、审计和云同步，可能超过单轮改造范围；Spec 阶段需要切分 MVP 与后续增强。

## User review notes

- 2026-06-29：用户要求严格模式推进：后端已有数据库存储，需要将设备、工作区、会话、运行记录存储到库里，不再基于文件；新增 4 张表；适合使用 JSON 存储的内容使用 JSON 存储。
- 2026-06-29：用户确认不保留旧文件存储路径、不向后兼容；设备采用选项 A 复用现有 `devices`；本轮不处理 history；不迁移旧数据；删除语义为软删除；每个 session 有多条运行记录，最后一条是当前运行记录。
- 2026-06-29：用户更新决策：取消删除 private key 的决策，保持现状；history 保持现状；device_key 应作为 device 的属性而不是独立表；不使用唯一索引，业务防重即可。
- 2026-06-29：用户补充决策：旧 `device_keys` 表直接删除，就地修改原 SQL；history 文件是终端输出，run record 是运行记录，两者不要混为一谈；业务防重遇到已有 active 记录时复用。
