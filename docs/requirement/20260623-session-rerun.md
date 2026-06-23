# 停止态会话重新运行需求
最后修改时间: 2026-06-23 18:44:58

Review status: Accepted

## Background

当前 `/sessions` 页面中，running 会话可以停止，stopped / failed 会话可以查看历史或删除。用户希望对已经停止的会话提供“重新运行”能力，减少重复填写会话名称、工作目录和命令的操作成本。

本需求已切换为 SpecFlow 严格模式 / strict。此前 Requirement / Spec 已接受，但现在追加了存储结构收敛要求，因此本 Requirement 重新进入 Draft，等待本轮 review。

现有存储已经在 `workspace.json` 的 session node 中保存了 session 大部分元数据，但同时还写入 `session.json`、`state.json`、`process.json`、`exit.json` 等碎片文件。重新运行会话时，如果继续围绕这些碎片文件设计归档和状态更新，会让 session 当前事实分散在多个文件里，增加一致性风险。

用户已明确：启动、关闭、删除会话都是同步操作；状态集合明确收敛为 `running` / `stopped` / `failed`，不进行 `starting` / `stopping` 历史兼容。创建失败和重新运行失败都必须返回明确 failed 状态，否则前端无法组织 UI。

用户进一步明确：存储结构不向后兼容，不迁移已有碎片文件；`exit.json` 也要一起收拾进 `workspace.json` 的聚合结构中。

## Goals

1. 在 stopped / failed 会话上提供“重新运行”入口。
2. 重新运行时复用同一个 session 及其 `session.id`，不创建新的 session record。
3. 重新运行复用原会话名称、工作目录和命令。
4. 重新运行后的 terminal 不复用旧 history 输出；用户看到的是本次重新运行产生的新输出。
5. 以 `workspace.json` 作为 workspace 与 session 当前元数据的聚合存储。
6. 合并并收拾当前 `session.json`、`state.json`、`process.json`、`exit.json` 的职责，不再把它们作为当前 session 事实来源。
7. 当前 session 的名称、命令、工作目录、history 配置、生命周期状态、进程记录、退出记录等小型结构化信息集中保存在 `workspace.json` 对应 session 节点中。
8. `history.log` 仍作为大体积追加日志单独保留在 session 目录下。
9. 重新运行前，本地 runtime / registry 将旧 `history.log` 按时间戳重命名；旧 run 的小型 metadata 归入 `workspace.json` 中的 archived/current run 结构，而不是继续制造 `process.<archive_id>.json`、`exit.<archive_id>.json`、`state.<archive_id>.json` 这类碎片。
10. 不向后兼容旧碎片文件，不迁移已有 `session.json` / `state.json` / `process.json` / `exit.json`。
11. 归档仅是本机 session 目录与 `workspace.json` 内部整理行为，不需要云端 / gateway / agent API 感知。
12. 重新运行成功后，用户可以直接进入该 session 的 live terminal。
13. 创建失败和重新运行失败后，当前 session 状态固定为 failed，并返回可供前端组织 UI 的明确状态。
14. 使用“文案 B”作为界面提示方向；当前按 `重新运行` 记录。

## Non-goals

1. 不新增编辑后再运行的复杂表单。
2. 不创建新的 session id 或新的 session record 来表达重新运行。
3. 不在本次运行 terminal 中回放旧 history 输出。
4. 不要求恢复旧 PTY runtime；重新运行应启动新的 PTY runtime。
5. 不保留 `starting` / `stopping` 作为状态，也不做历史兼容。
6. 不改变 running 会话 attach、history、输入输出的核心行为。
7. 不引入批量重新运行、多会话编排或定时重试。
8. 不新增归档列表 API、归档 history API 或云端归档同步能力。
9. 不要求前端展示已归档的旧运行输出；本轮只要求当前重新运行不回放旧输出。
10. 不引入数据库或新的索引文件来替代 `workspace.json`。
11. 不继续维护 `session.json` / `state.json` / `process.json` / `exit.json` 与 `workspace.json` 双写双读。
12. 不读取、折叠、迁移、清理已有碎片文件；已有文件不是新结构的数据来源。

## User scenarios

### 1. 重新运行已停止会话

用户在左侧会话树中看到一个 stopped 会话，点击“重新运行”。系统复用该 session 的 `session.id`、名称、工作目录和命令同步启动新的 PTY runtime。成功后，该 session 进入 running 状态，并打开对应 tab / terminal。

### 2. 重新运行失败会话

用户在 failed 会话上点击“重新运行”。系统复用该 session 原有启动信息同步尝试再次启动。如果工作目录不存在、命令不可执行或 PTY 启动失败，系统显示明确错误，并将当前 session 状态固定为 failed。

### 3. 本次输出与旧历史隔离

用户重新运行会话后，terminal 不回放旧会话历史作为本次输出。本地 runtime / registry 在重新运行前把旧 `history.log` 重命名归档，本次运行创建新的 `history.log`，并在 `workspace.json` 中更新当前 run 的 state/process/exit 信息。

### 4. 同一 tab 身份延续

由于重新运行复用同一个 `session.id`，已打开的 tab 应继续指向同一个 session 身份。重新运行成功后，该 tab 应展示新的 live terminal，而不是旧 stopped history。

### 5. workspace 聚合存储当前 session 事实

用户或开发者查看 `.termbridge/workspaces/<workspace_id>/workspace.json` 时，应能看到该 workspace 下 session 的当前名称、命令、工作目录、状态、进程和退出信息。当前事实不再散落在 `session.json`、`state.json`、`process.json`、`exit.json` 中。

## Acceptance criteria

1. 只有 `stopped` 和 `failed` 会话展示或允许触发“重新运行”。
2. `running` 会话不能触发“重新运行”。
3. 重新运行不创建新的 `session.id`。
4. 重新运行成功后，同一个 `session.id` 的生命周期变为 running。
5. 重新运行失败后，同一个 `session.id` 的生命周期变为 failed。
6. 创建失败后，相关 session 状态必须明确为 failed，并返回给前端可识别的状态 / 错误信息。
7. 重新运行复用原会话名称。
8. 重新运行复用原会话工作目录。
9. 重新运行复用原会话命令。
10. 重新运行启动新的 PTY runtime，而不是恢复已结束的旧 runtime。
11. 重新运行前，旧 `history.log` 在本机 session 目录内按时间戳重命名归档。
12. `session.json`、`state.json`、`process.json`、`exit.json` 不再作为当前 session 事实来源写入或读取。
13. `workspace.json` 中的 session 节点保存当前 session metadata、lifecycle state、process record、exit record 和 history 配置。
14. 列出 workspace/session 不需要逐个读取 `session.json`、`state.json`、`process.json`、`exit.json`。
15. `history.log` 保持单独文件，不塞入 `workspace.json`。
16. 不迁移已有碎片文件；已有旧文件在新代码路径下被忽略。
17. 归档动作不需要云端 / gateway / agent API 感知。
18. 重新运行后的 terminal 不回放旧 history 内容。
19. 重新运行成功后，前端打开或切换到该 session 的 tab。
20. 如果该 session 原本已有打开 tab，重新运行成功后该 tab 应展示新 live terminal。
21. 设备离线或只读状态下，不能触发重新运行，并应沿用现有离线只读提示策略。
22. 工作目录不存在、命令解析失败或 PTY 启动失败时，用户能看到失败原因。
23. 已有停止、删除、当前 history 查看能力不被破坏。
24. 状态机支持 `stopped -> running` 和 `failed -> running`，用于同步重新运行路径。
25. 启动、关闭、删除会话都是同步操作；状态集合只有 `running` / `stopped` / `failed`。
26. 不保留当前碎片文件与新 workspace 聚合结构的长期双写、双读或迁移机制。

## State model

目标用户可见 / 持久化生命周期状态：

- `running`：有 live PTY runtime，可 attach、输入、停止。
- `stopped`：进程正常结束或用户同步停止后的终态，可查看当前 history、删除、重新运行。
- `failed`：创建、运行、重新运行或等待过程失败后的终态，可查看当前 history / 错误、删除、重新运行。

不保留历史兼容状态：

- `starting`
- `stopping`

目标状态机：

```text
create ok ------------------------------> running
create failure -------------------------> failed
running -- natural exit ok / close ok --> stopped
running -- runtime/wait failure --------> failed
stopped -- rerun ok --------------------> running
failed  -- rerun ok --------------------> running
stopped -- rerun failure ---------------> failed
failed  -- rerun failure ---------------> failed
stopped -- delete ----------------------> deleted
failed  -- delete ----------------------> deleted
```

`deleted` 不是 lifecycle state，只表示记录被同步删除。

## Storage model

目标当前存储：

```text
.termbridge/
  workspaces/
    <workspace_id>/
      workspace.json                  # workspace + sessions aggregate
      sessions/
        <session_id>/
          history.log                  # 当前 run 输出
          history.<archive_id>.log     # 旧 run 输出归档
```

不再作为当前事实来源：

```text
session.json
state.json
process.json
exit.json
```

本轮按以下原则处理 `exit`：退出记录是当前 session/run 的小型结构化信息，和 state/process 一起收进 `workspace.json`，避免列表和详情继续依赖额外碎片文件。

`workspace.json` 中每个 session 节点至少承载：

- `session_id`
- `name`
- `launch_cwd`
- `command`
- `history`
- `state`
- `current_run.process`
- `current_run.exit`
- `archived_runs` 中的旧 run 小型 metadata 与对应 `history.<archive_id>.log` 路径
- `created_at`
- `updated_at`

## Compatibility and migration

本轮明确不做兼容和迁移：

1. 不读取旧 `session.json` / `state.json` / `process.json` / `exit.json`。
2. 不把旧碎片文件折叠进 `workspace.json`。
3. 不为旧碎片文件提供 fallback。
4. 不写迁移脚本。
5. 不在实现中维护新旧结构双写。
6. 旧文件如果留在磁盘上，只是未使用文件；除非用户后续单独要求清理，否则本功能不主动迁移它们。

## Open questions

暂无阻塞实现的未决事项。本轮需要 review 的核心变化是：将 session 当前事实收敛到 `workspace.json`，取消当前 `session.json` / `state.json` / `process.json` / `exit.json` 的碎片化存储，并且不做旧文件兼容或迁移。

## Decisions

1. 流程模式已切换为严格模式 / strict。
2. 本轮需求范围限定为停止态会话重新运行，不扩展为编辑后运行。
3. 用户已明确：`复用 id` 表达的是复用同一个 session / `session.id`，不是新建 session。
4. 用户已明确：`不复用历史` 表达的是本次重新运行无视之前 session 输出。
5. 用户已明确：旧运行输出按时间戳重命名归档。
6. 用户已明确：归档就是本地文件整理，不需要独立归档接口。
7. 用户已明确：归档不需要让云端知晓。
8. 用户已明确：状态机扩展 `stopped -> running` 和 `failed -> running`。
9. 用户已明确：启动、关闭、删除会话都是同步操作。
10. 用户已明确：不进行 `starting` / `stopping` 历史兼容。
11. 用户已明确：create failure 固定为 failed，必须返回明确状态供前端组织 UI。
12. 用户已采纳：rerun 失败固定进入 failed。
13. 用户已指定：采用文案 B；当前按 `重新运行` 记录。
14. 用户追加要求：`workspace.json` 内部已有 session 大部分信息，应合并收拾 `process.json` / `session.json` / `state.json`，不需要这么碎片化的存储。
15. 用户进一步确认：不向后兼容，不迁移已有文件，`exit.json` 也要收拾。

## Risks

1. 当前实现中 `workspace.json` 已有 session node，但 state/process/exit 仍分散读取；收敛时需要避免新旧来源并存导致不一致。
2. 由于明确不迁移已有碎片文件，旧本地数据如果没有完整写入 `workspace.json` session node，将不会被新代码恢复。
3. `workspace.json` 会变成 workspace/session 当前事实的聚合根，写入频率会提高；必须继续使用临时文件替换，避免半写入破坏 JSON。
4. `history.log` 仍是独立追加文件；rerun 前必须先归档旧 history，避免新旧输出混淆。
5. 状态机新增终态到 running 的转换后，需要确保只有用户显式触发的 rerun 路径使用该语义，避免列表刷新、进程状态校验、attach、history 读取、delete 判断等现有路径自动复活 stopped / failed session。
6. 如果前端仅用 `SessionSummary.command` 字符串重新拆命令，复杂参数可能无法完全还原；更稳妥的实现应由后端使用持久化的原始 command record。
7. 重新运行失败需要沿用现有创建会话的严格错误返回，不能无限容错或静默降级。

## User review notes

- 用户澄清：“复用 id”表达的是复用 session，而不是新建 session。
- 用户澄清：“不复用历史”表达的是无视之前 session 输出。
- 用户确认：旧运行输出加时间戳重命名。
- 用户确认：归档是文件整理，不需要归档接口。
- 用户确认：归档不需要云端知晓。
- 用户确认：扩展终态到 running 的状态机。
- 用户确认：启动、关闭、删除会话都是同步操作。
- 用户确认：不进行历史兼容。
- 用户确认：create failure 固定为 failed 并返回明确状态。
- 用户采纳：rerun 失败固定进入 failed。
- 用户要求切换为严格模式 / strict，并补充 Plan 和文档。
- 用户追加：`workspace.json` 内部有 session 大部分信息，合并收拾 `process.json` / `session.json` / `state.json`，重新组织结构并在这里 review，不需要这么碎片化的存储。
- 用户确认：不向后兼容，不迁移已有文件，`exit.json` 也要收拾。
