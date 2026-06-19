# Workspace / Session / Terminal 后端接口验证
最后修改时间: 2026-06-19 22:34:45

Review status: Accepted

## Basis

本验证基于：

- `docs/requirement/20260619-workspace-session-terminal-api.md`，Review status: Accepted
- `docs/spec/20260619-workspace-session-terminal-api.md`，Review status: Accepted
- `docs/plan/20260619-workspace-session-terminal-api.md`，Review status: Accepted

验证目标：确认后端已按 workspace/session/terminal 产品模型补齐接口契约，并记录当前所有相关 API 的实际行为、测试结果、范围偏差和剩余风险。

## Requirement alignment

| Requirement acceptance | Verification result |
| --- | --- |
| 后端概念清晰区分 workspace、session、terminal | 通过。模型层、registry 层和 REST 路由已分别围绕 workspace/session/live terminal attach 组织。 |
| session model 支持用户可编辑名称 | 通过。`Session` 增加 `Name`，summary 输出 `name`，`PATCH /api/sessions/{session_id}` 可更新 name。 |
| 新建 session 必须传入 name、cwd、command、terminal size | 通过。`POST /api/sessions` 校验非空 `name` 和 command，size 继续走 terminal size 校验。 |
| 新建 session 后按 workspace/session 两级结构存储 | 通过。session metadata 写入 `workspace.json` 的 `children`，运行态文件继续位于 `<workspace_key>/<session_id>`。 |
| 支持修改 session name | 通过。`UpdateSession` 更新 `workspace.json` children 中对应 session 的 `Name` 和 `UpdatedAt`。 |
| 支持关闭 running session 对应 PTY | 通过保留。`POST /api/sessions/{session_id}/close` 仍调用 live runtime close。 |
| 支持删除 stopped session，且不能删除 running session | 通过。删除时同时检查 live runtime map 和 persisted state，running/starting/stopping 拒绝。 |
| 支持读取 workspace tree | 通过。`GET /api/workspaces/tree` 返回 workspace + sessions。 |
| 支持根据 `workspace_id` 读取 session list | 通过。`GET /api/workspaces/{workspace_id}/sessions` 使用 workspace id 查询。 |
| 支持删除 workspace，且不会删除真实文件系统目录 | 通过。删除封装在 store state root 下的 workspace 目录。 |
| 删除 workspace 时必须保护 running sessions | 通过。任一 session 有 live runtime 或非 terminal state 时拒绝。 |
| 支持 workspace 排序持久化，返回列表时顺序稳定 | 通过。`SortOrder` 写入 workspace metadata，`ListWorkspaces` 稳定排序。 |
| 现有 WebSocket attach、history replay、session close 能力不回退 | 通过现有实现和测试覆盖。routes 仍保留 close/history/ws。 |
| 错误区分 usage/config/runtime 类错误 | 部分通过。新增缺字段、非法状态、未知 workspace 等走 usage；cwd/config/runtime 仍沿用现有错误分类。 |
| 相关 Go 测试覆盖新增 store、registry、webserver 行为 | 通过。新增 store/registry/webserver 单元测试，Go 测试通过。 |

## Spec alignment

### Public identifier policy

- 新增 workspace 路径参数使用 `workspace_id`。
- `workspace_key` 保留为内部 state path 标识，并在响应中作为必要字段继续输出。
- Go 命名按项目约定使用 `workspaceId` / `WorkspaceId`，JSON 字段仍为 `workspace_id`。

结论：对齐。

### Session name policy

- `POST /api/sessions` 要求 `name` trim 后非空。
- `PATCH /api/sessions/{session_id}` 是 session edit/update 接口，当前只支持 `name` 字段。
- 不支持字段通过 `UpdateSessionRequest.UnmarshalJSON` 拒绝。
- 不做旧数据兼容或迁移；旧 session 缺 name 不在本轮适配范围。

结论：对齐。

### Workspace order policy

- `Workspace` 增加 `SortOrder int`，JSON 字段为 `sort_order`。
- `PATCH /api/workspaces/order` 接收有序 `workspace_ids`。
- 空列表、重复 id、不存在 id 返回 usage 类错误。
- 请求中未出现的 workspace 保留原排序字段；最终列表通过 `ListWorkspaces` 排序返回。

结论：对齐。

### Delete safety policy

- session 删除：只有无 live runtime 且 persisted state 为 terminal state（stopped/failed）时允许删除。
- workspace 删除：先按 `workspace_id` 找 workspace，再枚举 sessions；任一 live runtime 或非 terminal state 会拒绝删除。
- 删除操作只调用 `Store.DeleteSession` / `Store.DeleteWorkspace`，路径由 state root 推导，不操作真实 workspace path。

结论：对齐。

### Close policy

- live running session close 行为保留。
- stopped session close 幂等语义未新增；stale running 无 live runtime 时仍由 `CloseSession` 返回 runtime 类错误 `session not closable`。

结论：保留现有能力，未扩展 stopped close 幂等；该点符合 Plan 中“至少不回退现有行为，并在验证中记录”。

## Current API documentation

### Common error response

```json
{
  "error": {
    "code": "usage_error | config_error | runtime_error | ...",
    "message": "human readable message"
  }
}
```

### `GET /api/health`

用途：健康检查。

成功响应：

```json
{
  "status": "ok"
}
```

### `GET /api/workspaces`

用途：返回 workspace 平铺列表，按 persisted workspace order 稳定排序。

成功响应：

```json
{
  "workspaces": [
    {
      "id": "workspace-id",
      "key": "workspace-key",
      "name": "workspace-name",
      "path": "/path/to/workspace",
      "sort_order": 1,
      "updated_at": "2026-06-19T00:00:00Z"
    }
  ]
}
```

### `GET /api/workspaces/tree`

用途：返回全部 workspace 及其 sessions 两级结构，用于左侧 tree。

成功响应：

```json
[
  {
    "id": "workspace-id",
    "key": "workspace-key",
    "name": "workspace-name",
    "path": "/path/to/workspace",
    "sort_order": 1,
    "updated_at": "2026-06-19T00:00:00Z",
    "children": [
      {
        "id": "session-id",
        "name": "Dev shell",
        "command": "zsh",
        "cwd": "/path/to/workspace",
        "lifecycle_state": "running",
        "attachment_state": "attached | detached | unattached | reattaching",
        "exit_code": 0,
        "updated_at": "2026-06-19T00:00:00Z",
        "log_path": "/path/to/termbridge.log"
      }
    ]
  }
]
```

### `PATCH /api/workspaces/order`

用途：持久化前端拖拽后的 workspace 顺序。

请求：

```json
{
  "workspace_ids": ["workspace-a", "workspace-b"]
}
```

规则：

- `workspace_ids` 必须非空。
- 不能包含重复 workspace id。
- 每个 workspace id 必须存在。
- 返回更新后的 workspace 列表。

成功响应：

```json
{
  "workspaces": [
    {
      "id": "workspace-a",
      "key": "workspace-key-a",
      "name": "workspace-a",
      "path": "/path/a",
      "sort_order": 1,
      "updated_at": "2026-06-19T00:00:00Z"
    }
  ]
}
```

### `DELETE /api/workspaces/{workspace_id}`

用途：删除 workspace 的 TermBridge state metadata，并级联删除其下 stopped/failed sessions 的 TermBridge 数据。

规则：

- `workspace_id` 不存在返回 usage 类错误。
- workspace 下存在 live runtime 或 starting/running/stopping session 时拒绝。
- 不删除真实 filesystem workspace path。

成功响应：`204 No Content`。

### `GET /api/workspaces/{workspace_id}/sessions`

用途：返回指定 workspace 下的 session 列表。

规则：

- 使用 `workspace_id` 查找 workspace。
- workspace 不存在返回 usage 类错误。

成功响应：

```json
{
  "sessions": [
    {
      "id": "session-id",
      "name": "Dev shell",
      "command": "zsh",
      "cwd": "/path/to/workspace",
      "lifecycle_state": "running",
      "attachment_state": "unattached",
      "updated_at": "2026-06-19T00:00:00Z",
      "log_path": "/path/to/termbridge.log"
    }
  ]
}
```

### `GET /api/sessions`

用途：返回所有 sessions 的平铺列表，响应直接返回数组，不包裹在 `sessions` 字段中。

成功响应：

```json
[
  {
    "id": "session-id",
    "name": "Dev shell",
    "workspace_id": "workspace-id",
    "workspace_key": "workspace-key",
    "command": "zsh",
    "cwd": "/path/to/workspace",
    "lifecycle_state": "running",
    "attachment_state": "attached | detached | unattached | reattaching",
    "exit_code": 0,
    "updated_at": "2026-06-19T00:00:00Z",
    "log_path": "/path/to/termbridge.log"
  }
]
```

### `POST /api/sessions`

用途：创建 session、启动 PTY，并写入 workspace/session 两级 state。

请求：

```json
{
  "name": "Dev shell",
  "cwd": "/path/to/workspace",
  "command": ["zsh"],
  "cols": 120,
  "rows": 32
}
```

规则：

- `name` 必填且 trim 后非空。
- `command` 必须非空。
- `cwd` 为空时使用 registry 默认 cwd。
- `cwd` 支持 `~` 和 `~/...` 展开，并必须落在 cwd allowlist 内。
- terminal size 使用默认补齐并执行范围校验。

成功响应：

```json
{
  "session_id": "session-id",
  "workspace_id": "workspace-id",
  "workspace_key": "workspace-key",
  "state": "running",
  "ws_url": "/api/sessions/session-id/ws"
}
```

### `GET /api/sessions/{session_id}`

用途：返回单个 session summary。

成功响应：

```json
{
  "id": "session-id",
  "name": "Dev shell",
  "workspace_id": "workspace-id",
  "workspace_key": "workspace-key",
  "command": "zsh",
  "cwd": "/path/to/workspace",
  "lifecycle_state": "running",
  "attachment_state": "attached | detached | unattached | reattaching",
  "exit_code": 0,
  "updated_at": "2026-06-19T00:00:00Z",
  "log_path": "/path/to/termbridge.log"
}
```

### `PATCH /api/sessions/{session_id}`

用途：session edit/update 接口。当前支持更新 `name` 字段，不是 rename-only 专用接口。

请求：

```json
{
  "name": "New session name"
}
```

规则：

- `name` 必填且 trim 后非空。
- 不支持的字段返回 usage 类错误。
- 更新 `workspace.json` children 中对应 session 的 `Name` 和 `UpdatedAt`。
- 不影响运行中的 PTY、history、state 或 workspace path。

成功响应：返回更新后的 session summary。

### `DELETE /api/sessions/{session_id}`

用途：删除 stopped/failed session 的 TermBridge state 目录。

规则：

- session 不存在返回错误。
- live runtime 存在时拒绝。
- starting/running/stopping state 拒绝。
- stopped/failed state 允许删除。
- 不删除 workspace 真实目录。

成功响应：`204 No Content`。

### `POST /api/sessions/{session_id}/close`

用途：关闭 live running session 对应 PTY。

规则：

- 仅 live runtime 可关闭。
- 无 live PTY handle 时返回 runtime 类错误。
- close 不删除 session record/history。

成功响应：

```json
{
  "state": "closing"
}
```

### `GET /api/sessions/{session_id}/history`

用途：读取 session bounded history。

规则：

- 如果 session 仍有 live runtime，先 flush history writer。
- history 文件不存在时返回空内容。
- 响应 `Content-Type: text/plain; charset=utf-8`。

成功响应：纯文本 history 内容。

### `GET /api/sessions/{session_id}/ws`

用途：attach live terminal WebSocket。

规则：

- 仅 live runtime 可 attach。
- 使用 terminal protocol 的文本控制帧和二进制输入/输出帧。
- 继续支持 control messages：`hello`、`resize`、`detach`、`ping`。

## Plan alignment

| Plan item | Verification result |
| --- | --- |
| 更新持久化模型 | 已实现：`Session.Name`、`Workspace.SortOrder`、`Workspace.Children`。 |
| 扩展 state store 能力 | 已实现：按 workspace id 查找/list，session metadata 写入/读取 `workspace.json` children，session/workspace delete，session update，workspace order。 |
| 更新 session 创建链路 | 已实现：create request/manager/session summary 均包含 name。 |
| 增加 registry 业务方法 | 已实现：tree、workspace sessions、session update/delete、workspace order/delete。 |
| 扩展 webserver routes 和 handlers | 已实现：workspace tree/order/delete/sessions，session patch/delete，保留 close/history/ws。 |
| 更新协议/前端类型所需响应字段 | 前端类型已有未暂存改动同步 `name` / `sort_order`；本次已运行前端检查并记录失败原因。 |
| 增加和调整测试 | 已实现 store/registry/webserver 相关测试；Go 测试通过。 |
| 最小清理 | 后端范围内未引入认证、用户体系、workspace edit 或真实目录删除逻辑。 |

## Actual diff summary

### Staged backend-related changes

当前已暂存：

- `docs/requirement/20260619-workspace-session-terminal-api.md`
- `docs/spec/20260619-workspace-session-terminal-api.md`
- `docs/plan/20260619-workspace-session-terminal-api.md`
- `internal/app/app.go`
- `internal/session/manager.go`
- `internal/session/recovery.go`
- `internal/session/session.go`
- `internal/state/store.go`
- `internal/state/store_test.go`
- `internal/terminalproto/protocol.go`
- `internal/webserver/server.go`
- `internal/webserver/server_test.go`
- `internal/webterminal/registry.go`
- `internal/webterminal/registry_test.go`
- `internal/webterminal/runtime.go`
- `internal/workspace/workspace.go`

主要变更：

- 新增 workspace/session/terminal 后端接口过程文档。
- session 持久化模型增加 `name`，workspace 持久化模型增加 `sort_order` 和 `children` session 节点，常规 session metadata 读写以 `workspace.json` 为准。
- state store 增加 workspace id 查询、workspace sessions 查询、session/workspace 删除、session update、workspace order 持久化和 workspace 稳定排序。
- registry 增加 session edit/delete、workspace tree、workspace sessions list、workspace order、workspace delete。
- webserver 增加 REST routes，并保留既有 health/workspaces/sessions/close/history/ws。
- terminal protocol 和 runtime 命名同步为 `SessionId` / `WorkspaceId` Go 风格，JSON 不变。
- 新增和调整 Go tests。

### Unstaged changes observed

当前仍有未暂存前端/设计相关改动：

- `web/src/App.vue`
- `web/src/components/terminal/HistoryTerminalView.vue`
- `web/src/components/terminal/TerminalView.vue`
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
- `web/src/features/sessions/api.ts`
- `web/src/features/workspaces/api.ts`
- `web/src/protocol/terminal.ts`
- `web/src/styles.css`
- `web/yarn.lock`
- `.DS_Store`
- `docs/design/`
- `web/package-lock.json`

这些未暂存内容包含前端重构和类型同步，影响前端 typecheck 结果；不属于当前已暂存的后端接口提交边界。

## Expected vs actual changed files

### Expected by plan

- `internal/session/session.go`
- `internal/session/manager.go`
- `internal/workspace/workspace.go`
- `internal/state/store.go`
- `internal/state/store_test.go`
- `internal/webterminal/registry.go`
- `internal/webterminal/registry_test.go`
- `internal/webserver/server.go`
- `internal/webserver/server_test.go`
- 必要时更新 `web/src/protocol/terminal.ts` / API TS 类型

### Actual staged backend files

实际已暂存后端文件包含计划文件，并额外包含：

- `internal/app/app.go`：CLI session 创建链路补齐必填 name。
- `internal/session/recovery.go`：命名风格同步为 `sessionId`。
- `internal/terminalproto/protocol.go`：Go 字段命名同步为 `SessionId` / `WorkspaceId`，JSON 不变。
- `internal/webterminal/runtime.go`：runtime 输出协议字段命名同步。

结论：额外后端文件均由新契约或用户要求的 `workspaceId` / `WorkspaceId` 命名统一引起，属于合理范围扩展。

## Test results

### Go unit tests

命令：

```sh
PATH="/Users/leon/.version-fox/sdks/golang/bin:$HOME/go/bin:$PATH" go test ./cmd/... ./internal/...
```

结果：通过。

摘要：

```text
?    termbridge-go/cmd/termbridge [no test files]
ok   termbridge-go/internal/app (cached)
ok   termbridge-go/internal/cli (cached)
ok   termbridge-go/internal/config (cached)
ok   termbridge-go/internal/errors (cached)
ok   termbridge-go/internal/history (cached)
ok   termbridge-go/internal/identity (cached)
ok   termbridge-go/internal/logging (cached)
ok   termbridge-go/internal/process (cached)
?    termbridge-go/internal/pty [no test files]
?    termbridge-go/internal/pty/gopty [no test files]
ok   termbridge-go/internal/runner (cached)
ok   termbridge-go/internal/session (cached)
ok   termbridge-go/internal/state (cached)
ok   termbridge-go/internal/terminalproto (cached)
?    termbridge-go/internal/version [no test files]
ok   termbridge-go/internal/webserver (cached)
ok   termbridge-go/internal/webterminal (cached)
ok   termbridge-go/internal/workspace (cached)
```

### Go vet

命令：

```sh
PATH="/Users/leon/.version-fox/sdks/golang/bin:$HOME/go/bin:$PATH" go vet ./cmd/... ./internal/...
```

结果：通过，无输出。

### golangci-lint

命令：

```sh
PATH="/Users/leon/.version-fox/sdks/golang/bin:$HOME/go/bin:$PATH" golangci-lint run ./cmd/... ./internal/...
```

结果：通过，无输出。

### Frontend typecheck

命令：

```sh
yarn --cwd /Volumes/media/SourceCodes/mywork/TermBridge-go/web typecheck
```

结果：失败。

失败集中在未暂存前端重构文件 `web/src/components/workspace/WorkspaceSessionSidebar.vue`，错误包括：

```text
Property 'children' does not exist on type 'TreeItemModel'.
Type 'TreeItemModel' is not assignable to type 'PropertyKey | undefined'.
Property '_item' does not exist on type 'FlattenedItem<TreeItemModel>'.
```

结论：该失败由当前工作树中的前端重构未完成导致，不是已暂存后端接口变更本身的 Go 验证失败；但由于本轮存在前端类型同步文件未暂存改动，交付前需要注意整体工作树前端 typecheck 尚未通过。

### Frontend lint

命令：

```sh
yarn --cwd /Volumes/media/SourceCodes/mywork/TermBridge-go/web lint
```

结果：通过。

```text
$ eslint .
Done in 7.99s.
```

## Acceptance checklist

- [x] `GET /api/workspaces` 保留并按 workspace order 返回。
- [x] `GET /api/workspaces/tree` 返回 workspace + sessions 两级结构。
- [x] `PATCH /api/workspaces/order` 接收并持久化有序 `workspace_ids`。
- [x] `DELETE /api/workspaces/{workspace_id}` 删除 state workspace 目录，不删除真实 path。
- [x] `GET /api/workspaces/{workspace_id}/sessions` 按 workspace id 返回 sessions。
- [x] `GET /api/sessions` 保留并返回包含 `name` 的 summary。
- [x] `POST /api/sessions` 要求 `name`，创建 session 并启动 PTY。
- [x] `GET /api/sessions/{session_id}` 保留并返回包含 `name` 的 summary。
- [x] `PATCH /api/sessions/{session_id}` 作为 edit/update 接口支持 `name`。
- [x] `DELETE /api/sessions/{session_id}` 允许 stopped/failed，拒绝 running/live。
- [x] `POST /api/sessions/{session_id}/close` 保留 live PTY close。
- [x] `GET /api/sessions/{session_id}/history` 保留 history replay。
- [x] `GET /api/sessions/{session_id}/ws` 保留 live terminal attach。
- [x] 删除 session/workspace 只操作 state root，不操作真实 workspace filesystem path。
- [x] 不引入认证、用户体系、云同步或 workspace edit API。
- [x] 不做旧数据兼容或迁移。
- [x] Go tests / vet / golangci-lint 通过。
- [ ] 前端 typecheck 通过。当前失败来自未暂存前端重构文件。

## Scope deviations

1. 为满足用户命名要求，额外同步了 Go 代码中的 `WorkspaceID` / `SessionID` 风格到 `WorkspaceId` / `SessionId`，JSON 字段保持不变。
2. `internal/app/app.go` 需要补齐 session create name，否则 CLI 创建链路无法满足新 session model 契约。
3. 前端已有未暂存重构和类型同步改动，因此 frontend typecheck 结果受到当前工作树影响；本次已记录但未修复前端重构错误。

## Risks

1. 当前 working tree 中未暂存前端重构导致 `yarn typecheck` 失败；如果需要整仓交付，需先修复前端 tree component 类型问题。
2. `CloseSession` 对 stopped session 未改为幂等成功，仍只支持 live runtime close；这是保留现有能力而非扩展 close 语义。
3. 本轮明确不做旧数据兼容或迁移；旧 state 中缺少 `name` 的 session 不满足新前端契约。
4. webserver 层新增接口已有基础测试，但 `DELETE /api/workspaces/{workspace_id}`、`GET /api/workspaces/{workspace_id}/sessions`、`PATCH /api/sessions/{session_id}`、`DELETE /api/sessions/{session_id}` 的 HTTP 层深度断言仍可后续补充；registry 层已覆盖核心行为。

## Incomplete items

- 前端 typecheck 未通过，原因是未暂存前端重构文件 `WorkspaceSessionSidebar.vue` 类型错误。
- 未新增 stopped session close 幂等语义；当前行为记录为保留 live close。

## Conclusion

后端 workspace/session/terminal API 实现与已接受 Requirement、Spec、Plan 基本对齐：核心后端接口、持久化字段、删除安全、workspace order、session edit、workspace tree 和 workspace session list 均已实现；Go 测试、Go vet 和 golangci-lint 均通过。

当前交付风险主要来自工作树中未暂存的前端重构导致 typecheck 失败，以及 close stopped 幂等语义未扩展。若以后端已暂存变更为提交边界，后端验证结论为通过；若以整个工作树为交付边界，需要先修复前端 typecheck。
