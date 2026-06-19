# Workspace / Session / Terminal 后端接口规格
最后修改时间: 2026-06-19 22:34:45

Review status: Accepted

## Requirement basis

基于 `docs/requirement/20260619-workspace-session-terminal-api.md`，本规格定义后端为 workspace/session/terminal 产品模型提供的接口契约、数据模型、存储行为和错误语义。

已接受的关键约束：

- 外部 API 使用 `workspace_id` 标识 workspace；`workspace_key` 仅作为内部存储路径标识或必要输出字段。
- 新建 session 必须提供用户可编辑的 `name`，不生成默认名称。
- workspace tree 接口一次返回全部 workspace 及其 sessions。
- session list 支持按 `workspace_id` 查询。
- 删除 stopped session 只删除 TermBridge 保存的 session metadata/state/history/process/exit，不删除真实 workspace 目录。
- 删除 workspace 不删除真实文件系统目录；只要存在 running session 必须拒绝；没有 running session 时允许级联删除 stopped sessions 的 TermBridge metadata/history。
- workspace 排序接口接收前端拖拽后的有序 `workspace_id` 列表并持久化。
- 现有 WebSocket attach、history replay、session close 能力不能回退。

## Overview

后端保持本地优先模型，不引入正式用户体系。API 层以 workspace/session/terminal 为边界：

- workspace API 负责目录分组、排序、tree 查询和删除。
- session API 负责创建、查询、编辑、关闭、删除和 history 查询。
- terminal 能力继续通过现有 session WebSocket attach 暴露；stopped session 不提供 live PTY，只提供 history。

现有 state root 下 workspace/session 两级目录结构继续作为主存储结构。本轮是在现有模型上补齐字段、查询、删除和排序能力，而不是整体重写存储布局。

## Design decisions

### 1. Public identifier policy

外部 REST API 路径和请求体统一使用 `workspace_id`：

- `workspace_id` 是前端持久引用 workspace 的稳定标识。
- `workspace_key` 保留为内部目录名和必要输出字段，不作为新接口的主要入参。
- 新增路径参数不使用 `workspace_key`。

### 2. Session name policy

`session` model 增加必填字段：

```go
Name string `json:"name"`
```

规则：

- `POST /api/sessions` 必须提供非空 `name`。
- `PATCH /api/sessions/{session_id}` 是会话编辑接口，当前支持修改 `name` 字段；不要收窄为 rename-only 概念。
- 修改名称不改变 session ID、workspace、history、state 或运行中的 PTY。
- 不做历史数据兼容或迁移；旧 session 缺少 `name` 字段时不作为本轮适配目标，新建和编辑接口必须严格要求非空名称。

### 3. Workspace order policy

workspace 排序需要持久化，并影响所有 workspace 列表和 tree 返回顺序。

建议模型在 workspace metadata 中增加：

```go
SortOrder int `json:"sort_order"`
```

排序规则：

1. 有显式 `sort_order` 的 workspace 按 `sort_order` 升序。
2. 未排序或历史数据按 `created_at`、`name`、`workspace_id` 的稳定顺序追加在后面。
3. `PATCH /api/workspaces/order` 接收有序 `workspace_id` 列表，只更新列表中出现的 workspace 的排序值。
4. 请求中包含不存在的 `workspace_id` 应返回 usage 类错误。
5. 请求可以不包含全部 workspace；未出现的 workspace 保持原排序或追加在有序列表后，具体行为在实现时固定并测试。

### 4. Delete safety policy

session 删除：

- 只允许删除非 running session。
- 如果 session 存在 live runtime 或 state 为 running/starting/stopping，应拒绝。
- 删除内容包括该 session 在 TermBridge state root 下的 session 目录及其 metadata/state/history/process/exit。
- 不删除 workspace 对应的真实文件系统目录。

workspace 删除：

- 删除前必须枚举该 workspace 下所有 sessions。
- 只要存在 running/starting/stopping session，返回冲突错误并拒绝删除。
- 若不存在 running session，允许删除 workspace metadata/state 目录，并级联删除其下 stopped/failed sessions 的 TermBridge 数据。
- 不删除用户真实 filesystem workspace path。

### 5. Close policy for live and stale sessions

保留现有 `POST /api/sessions/{id}/close`。语义需要明确：

- live running session：关闭实际 PTY，等待 runtime 写入 exit/state，保留 session record 和 history。
- stopped session：返回成功或 usage/conflict 错误需要在实现阶段统一；建议返回成功以支持幂等 close。
- state 为 running 但无 live runtime 的 stale session：不得误删；建议将错误归类为 runtime/config 可展示错误，并在响应中说明 runtime 不可 attach/close。

## Affected components

### `internal/session`

需要更新持久化模型：

- `Session` 增加 `Name`。
- `Workspace` 增加排序字段，推荐 `SortOrder`。
- 不做旧数据兼容或迁移；新增字段按新契约写入。

### `internal/state`

需要补齐 store 能力：

- 按 `workspace_id` 查找 workspace metadata。
- 按 `workspace_id` 查询 sessions。
- 删除 session 目录。
- 删除 workspace metadata/state 目录，并支持级联 stopped sessions。
- 更新 session metadata 的 name 和 updated_at。
- 更新 workspace sort order 和 updated_at。
- tree/list 返回时使用稳定排序。

### `internal/webterminal`

registry 负责组合 runtime 状态和 store 状态：

- `CreateSessionRequest` 增加 `Name`。
- `SessionSummary` 增加 `name`，并继续包含 command/cwd/status/update time 等摘要。
- 新增 Update/Delete/ListByWorkspace/WorkspaceTree/WorkspaceOrder/DeleteWorkspace 等业务方法。
- 删除和关闭操作必须结合 live runtime map 与持久 state 判断安全性。

### `internal/webserver`

需要扩展路由和 handler：

- workspace routes：列表、tree、排序、删除、workspace 下 sessions。
- session routes：创建、详情、编辑、删除、关闭、history、ws。
- 错误响应继续使用统一 JSON error body，并区分 usage/config/runtime。

### Tests

需要覆盖：

- store 层 metadata 字段读写、排序、删除。
- registry 层 session name、edit/delete、workspace tree、workspace delete running 保护。
- webserver 层新增 REST API、错误码和请求校验。
- 现有 close/history/ws 行为不回退。

## Interfaces

### Common response shape

错误响应沿用现有结构：

```json
{
  "error": {
    "code": "usage_error | config_error | runtime_error | ...",
    "message": "human readable message"
  }
}
```

新增接口应尽量保持：

- 请求格式错误、缺字段、非法状态操作：usage 类错误。
- cwd/workspace 配置不允许、状态文件不一致：config 类错误。
- PTY/runtime close、runtime attach、IO 失败：runtime 类错误。

### Workspace summary

建议响应字段：

```json
{
  "id": "workspace-id",
  "key": "workspace-key",
  "name": "TermBridge-go",
  "path": "/path/to/workspace",
  "sort_order": 10,
  "created_at": "2026-06-19T00:00:00Z",
  "updated_at": "2026-06-19T00:00:00Z"
}
```

### Session summary

建议响应字段：

```json
{
  "id": "session-id",
  "name": "Run server",
  "workspace_id": "workspace-id",
  "workspace_key": "workspace-key",
  "cwd": "/path/to/workspace",
  "command": "zsh",
  "lifecycle_state": "running",
  "attachment_state": "attached | detached | none",
  "created_at": "2026-06-19T00:00:00Z",
  "updated_at": "2026-06-19T00:00:00Z"
}
```

### `GET /api/workspaces`

返回 workspace 平铺列表，按持久化排序返回。

用途：保留现有 workspace 平铺列表能力。

### `GET /api/workspaces/tree`

返回全部 workspace 及其 sessions 两级结构。

响应示例：

```json
[
  {
    "id": "workspace-id",
    "key": "workspace-key",
    "name": "TermBridge-go",
    "path": "/repo/TermBridge-go",
    "sort_order": 10,
    "created_at": "2026-06-19T00:00:00Z",
    "updated_at": "2026-06-19T00:00:00Z",
    "children": [
      {
        "id": "session-id",
        "name": "Dev shell",
        "cwd": "/repo/TermBridge-go",
        "command": "zsh",
        "lifecycle_state": "running",
        "attachment_state": "detached",
        "created_at": "2026-06-19T00:00:00Z",
        "updated_at": "2026-06-19T00:00:00Z"
      }
    ]
  }
]
```

排序：workspace 按 workspace order；sessions 建议按 `updated_at desc` 或 `created_at desc`，实现阶段需固定并测试。

### `PATCH /api/workspaces/order`

请求：

```json
{
  "workspace_ids": ["workspace-a", "workspace-b", "workspace-c"]
}
```

行为：

- 校验数组非空、无重复。
- 校验每个 `workspace_id` 存在。
- 按数组顺序持久化排序。
- 返回更新后的 workspace 列表或 `204 No Content`；建议返回更新后的 workspace 列表便于前端同步。

### `DELETE /api/workspaces/{workspace_id}`

行为：

- 如果 workspace 不存在，返回 usage/not found 类错误。
- 如果存在 running/starting/stopping session，拒绝删除。
- 如果仅存在 stopped/failed sessions，级联删除 TermBridge session 数据。
- 删除 workspace state metadata 目录。
- 不删除真实 filesystem path。

成功响应建议 `204 No Content`。

### `GET /api/workspaces/{workspace_id}/sessions`

返回指定 workspace 下 session 列表。

行为：

- 按 `workspace_id` 查找 workspace。
- 返回该 workspace 下所有 session summary。
- session item 不包含 `workspace_id` / `workspace_key`，因为路径已经限定 workspace。
- 不存在的 workspace 返回 usage/not found 类错误。

响应示例：

```json
{
  "sessions": [
    {
      "id": "session-id",
      "name": "Dev shell",
      "cwd": "/repo/TermBridge-go",
      "command": "zsh",
      "lifecycle_state": "running",
      "attachment_state": "detached",
      "created_at": "2026-06-19T00:00:00Z",
      "updated_at": "2026-06-19T00:00:00Z"
    }
  ]
}
```

### `GET /api/sessions`

保留现有平铺 session 列表接口，响应直接返回 session summary 数组，不包裹在 `sessions` 字段中。

响应示例：

```json
[
  {
    "id": "session-id",
    "name": "Dev shell",
    "workspace_id": "workspace-id",
    "workspace_key": "workspace-key",
    "cwd": "/repo/TermBridge-go",
    "command": "zsh",
    "lifecycle_state": "running",
    "attachment_state": "detached",
    "created_at": "2026-06-19T00:00:00Z",
    "updated_at": "2026-06-19T00:00:00Z"
  }
]
```

可选增强：支持 query 参数 `workspace_id`：

```text
GET /api/sessions?workspace_id=...
```

但由于需求明确“根据 workspace id 读取 session list”，优先新增 `/api/workspaces/{workspace_id}/sessions`，避免与旧接口语义混淆。

### `POST /api/sessions`

请求新增 `name`：

```json
{
  "name": "Dev shell",
  "cwd": "/repo/TermBridge-go",
  "command": ["zsh"],
  "cols": 120,
  "rows": 32
}
```

规则：

- `name` 必填且 trim 后非空。
- `cwd`、`command`、`cols`、`rows` 继续按现有规则校验。
- 后端根据 cwd resolve workspace，并按 workspace/session 两级结构持久化。
- 响应继续返回 session id、workspace id/key 和 ws url。

### `GET /api/sessions/{session_id}`

保留并增强现有 session summary，新增 `name` 字段。

### `PATCH /api/sessions/{session_id}`

会话编辑接口。当前支持 `name` 字段，不设计为 rename-only 接口；后续如扩展其它会话可编辑字段，应沿用该 edit/update 语义。

请求：

```json
{
  "name": "New session name"
}
```

行为：

- session 不存在返回 usage/not found 类错误。
- 当前只接受 `name` 字段；不支持的编辑字段返回 usage 类错误。
- `name` 为空返回 usage 类错误。
- 更新 session metadata `name` 和 `updated_at`。
- 不影响 running runtime、history、state、workspace。
- 返回更新后的 session summary。

### `POST /api/sessions/{session_id}/close`

保留现有接口。

增强规格：

- live running session 关闭实际 PTY。
- close 不删除 session record/history。
- 对 stale running session 的响应必须可区分为 runtime/config 类错误。

### `DELETE /api/sessions/{session_id}`

删除 stopped session。

行为：

- session 不存在返回 usage/not found 类错误。
- running/starting/stopping session 拒绝删除。
- 若有 live runtime，拒绝删除。
- stopped/failed session 删除 TermBridge session 目录。
- 不删除 workspace 真实目录。

成功响应建议 `204 No Content`。

### `GET /api/sessions/{session_id}/history`

保留现有接口，不改变语义。

### `GET /api/sessions/{session_id}/ws`

保留现有 live terminal attach。

规则：

- 仅 running/live runtime 可 attach。
- stopped session 不再有 live PTY，前端应使用 history。

## Storage behavior

### Workspace directory

后端 workspace 存储以 `workspace.json` 作为常规会话 metadata 的读取来源，workspace 和 sessions 形成树型结构：

```text
<state_root>/<workspace_key>/
  workspace.json
  <session_id>/...
```

`workspace.json` 增加排序字段和 `children` session 节点：

```json
{
  "workspace_id": "workspace-id",
  "workspace_key": "workspace-key",
  "name": "workspace-name",
  "path": "/path/to/workspace",
  "sort_order": 1,
  "children": [
    {
      "session_id": "session-id",
      "name": "Dev shell",
      "launch_cwd": "/path/to/workspace",
      "command": { "command": "zsh", "args": [] },
      "log_path": "/path/to/termbridge.log",
      "created_at": "2026-06-19T00:00:00Z",
      "updated_at": "2026-06-19T00:00:00Z"
    }
  ]
}
```

常规 session create/update/delete 会同步写入 `workspace.json` 的 `children`；workspace tree 和按 workspace 读取 sessions 直接读取 `workspace.json`。

### Session directory

session 目录继续保存运行态和输出相关文件：

```text
<state_root>/<workspace_key>/<session_id>/
  state.json
  process.json
  exit.json
  history.log
```

实现可保留 `session.json` 作为过渡写入产物，但常规读取以 `workspace.json` 中的 children 为准，不做旧数据迁移。

### Delete implementation constraints

删除操作必须限制在 TermBridge state root 内部，不得对 workspace `path` 指向的真实目录执行删除。

实现时应优先通过 store 封装删除路径，避免 handler 或 registry 拼接路径后直接删除。

## Technical questions

暂无必须由用户确认的技术问题。

实现阶段需要在代码中确认并固定以下细节：

1. workspace order 对未出现在请求列表中的 workspace 的精确处理方式。
2. sessions 在 tree 和 workspace session list 中的排序字段。
3. stopped session close 是否幂等返回成功，或返回 usage/conflict 错误。

这些属于实现细节，不阻塞当前 Spec。

## Risks

1. `workspace_id` 和 `workspace_key` 混用会导致 API 调用和 state 路径不一致；新增接口必须以 `workspace_id` 为外部入参。
2. 删除 workspace 如果直接操作真实 workspace path，会造成用户数据丢失；必须只删除 state root 下数据。
3. stale running state 无 live runtime 时，close/delete 语义不清会导致 PTY 泄漏或无法清理；实现需明确错误分类。
4. 本轮不做旧 session 兼容和迁移；已有旧数据如果缺少 `name` 字段，可能不满足新接口契约，需要在交付说明中明确。
5. workspace 排序若只在内存保存，服务重启后前端拖拽结果会丢失；必须持久化。

## Alternatives considered

### 使用 `workspace_key` 作为 API 入参

拒绝。`workspace_key` 是存储路径标识，不适合作为前端稳定产品 ID。需求已决定外部 API 使用 `workspace_id`。

### session 删除时保留 history

拒绝。需求定义删除 session 会删除 TermBridge 保存的 session metadata、state、history、process、exit 等 session 目录内容。

### workspace 删除只允许空 workspace

拒绝。需求已决定没有 running session 时允许删除 workspace，并级联删除 stopped sessions 的 TermBridge 数据。

### 前端本地保存 workspace order

拒绝。需求要求后端持久化排序，保证列表顺序稳定。

## User review notes

本 Spec 已覆盖 Requirement 中确认的核心接口：workspace tree、workspace order、workspace delete、workspace sessions list、session create name、session edit、session delete，以及保留 close/history/ws。

暂无需要用户确认的未决事项。
