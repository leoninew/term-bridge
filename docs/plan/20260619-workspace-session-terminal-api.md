# Workspace / Session / Terminal 后端接口实施计划
最后修改时间: 2026-06-19 22:34:45

Review status: Accepted

## Basis

本计划基于：

- `docs/requirement/20260619-workspace-session-terminal-api.md`，Review status: Accepted
- `docs/spec/20260619-workspace-session-terminal-api.md`，Review status: Accepted

目标是在现有 state root 下 workspace/session 两级存储结构上，补齐后端 API 契约：session name、session edit/delete、workspace tree、workspace sessions list、workspace delete、workspace order，并保持现有 close/history/ws 能力不回退。

## Implementation steps

### 1. 更新持久化模型

修改 model 字段，先保证 JSON 读写结构具备新增契约。

- 在 `internal/session/session.go`：
  - 为 `Session` 增加 `Name string json:"name"`。
  - 确认 `UpdatedAt` 在 session edit 时会被更新。
- 在 `internal/workspace/workspace.go`：
  - 为 `Workspace` 增加 `SortOrder int json:"sort_order"`。
  - 为 `Workspace` 增加 `Children []SessionNode json:"children"`，把 workspace/session 后端存储表达为树型结构。
  - 不做旧 workspace JSON 兼容或迁移；新契约写入 `SortOrder` 和 `children`。

实现约束：

- 不做历史 state 文件兼容或批量迁移。
- 新建 session 必须写入 name。
- 不做历史数据兼容或迁移；旧 session 缺 name 不作为本轮适配目标，新建和编辑接口严格校验 name。

### 2. 扩展 state store 能力

修改 `internal/state/store.go`，补齐 registry 需要的 store API。

建议新增或调整方法：

- `FindWorkspaceByID(workspaceId string) (workspace.Workspace, error)`
- `ListSessionsByWorkspaceId(workspaceId string) ([]session.View, []Warning, error)`
- `UpdateSession(workspaceKey string, sessionId string, update func(*session.Session) error) (session.Session, error)`，用于会话编辑接口；当前只允许更新 `Name`
- `DeleteSession(workspaceKey string, sessionId string) error`
- `DeleteWorkspace(workspaceKey string) error`
- `UpdateWorkspaceOrder(workspaceIds []string) ([]workspace.Workspace, error)` 或 store 层基础更新方法供 registry 组合。

实现细节：

- 常规 session metadata create/update/delete 同步写入 `workspace.json` 的 `children`。
- `LoadSession()` / `ListSessionsByWorkspaceId()` / workspace tree 优先直接读取 `workspace.json` 的 `children`。
- 删除 session 从 `workspace.json` children 移除节点，并删除 `<state_root>/<workspace_key>/<session_id>`。
- 删除 workspace 只删除 `<state_root>/<workspace_key>`。
- 所有删除路径必须通过 `Store.WorkspaceDir` / `Store.SessionDir` 推导，避免操作 workspace 真实路径。
- `ListWorkspaces()` 返回顺序改为稳定排序：优先 `SortOrder`，再 fallback 到 `CreatedAt`、`Name`、`ID`。
- `ListSessionsByWorkspaceId()` 先通过 workspace id 找到 workspace，再从 `workspace.json` 的 children 列出 sessions。

### 3. 更新 session 创建链路

涉及：

- `internal/webterminal/registry.go`
- `internal/session/manager.go`
- 相关测试

变更：

- `webterminal.CreateSessionRequest` 增加 `Name string json:"name"`。
- `Registry.CreateSession` 校验 `name` trim 后非空；为空返回 usage 类错误。
- `session.CreateOptions` 增加 `Name`。
- `session.Manager.Create` 写入 `Session.Name`。
- `SessionSummary` 增加 `Name`，`summaryFromView` 输出该字段。
- `CreateSessionResponse` 不必新增 name，除非实现时发现前端同步需要；核心 summary 接口必须包含 name。

契约注意：

- 现有 registry tests 创建 session 的 request 需要补充 name，或者明确测试空 name 被拒绝。
- 前端当前 `createSession` 仍可能不传 name；本轮以后前端需要按新契约调整，但本计划只实现后端。

### 4. 增加 registry 业务方法

在 `internal/webterminal/registry.go` 增加业务方法，避免 webserver handler 直接操作 store。

建议新增类型：

- `WorkspaceTreeResponse` 或 `WorkspaceTree`。
- `WorkspaceNode`，包含 `Workspace WorkspaceSummary` 和 `Sessions []SessionSummary`。
- `UpdateSessionRequest`。
- `UpdateWorkspaceOrderRequest`。

建议新增方法：

- `WorkspaceTree() ([]WorkspaceNode, error)`
- `ListSessionsByWorkspaceId(workspaceId string) ([]SessionSummary, error)`
- `UpdateSession(sessionId string, request UpdateSessionRequest) (SessionSummary, error)`，当前仅支持 `name` 字段
- `DeleteSession(sessionId string) error`
- `UpdateWorkspaceOrder(workspaceIds []string) ([]WorkspaceSummary, error)`
- `DeleteWorkspace(workspaceId string) error`

关键安全判断：

- `DeleteSession`：
  - 先通过 store 找到 session view。
  - 若 runtime map 中存在该 session，拒绝删除。
  - 若 state 为 `starting/running/stopping`，拒绝删除。
  - 仅 stopped/failed 等非运行状态允许删除。
- `DeleteWorkspace`：
  - 先按 `workspace_id` 找 workspace。
  - 枚举 workspace 下 sessions。
  - 任一 session 有 live runtime 或 state 为 `starting/running/stopping`，拒绝删除。
  - 全部安全后调用 store 删除 workspace state 目录。
- `CloseSession`：
  - 保留现有 live close 行为。
  - 若实现过程中能低风险补齐 stopped 幂等或 stale running 错误分类，则补齐；否则至少不回退现有行为，并在验证中记录。

### 5. 扩展 webserver routes 和 handlers

修改 `internal/webserver/server.go`。

路由建议：

- 保留：`GET /api/workspaces`
- 新增：`GET /api/workspaces/tree`
- 新增：`PATCH /api/workspaces/order`
- 新增：`DELETE /api/workspaces/{workspace_id}`
- 新增：`GET /api/workspaces/{workspace_id}/sessions`
- 保留：`GET /api/sessions`
- 更新：`POST /api/sessions` 接收 `name`
- 保留：`GET /api/sessions/{session_id}`
- 新增：`PATCH /api/sessions/{session_id}`
- 新增：`DELETE /api/sessions/{session_id}`
- 保留：`POST /api/sessions/{session_id}/close`
- 保留：`GET /api/sessions/{session_id}/history`
- 保留：`GET /api/sessions/{session_id}/ws`

实现注意：

- 当前只有 `/api/workspaces` 和 `/api/sessions/` 两类 handler，需要为 `/api/workspaces/` 增加分派 handler。
- 对 `tree`、`order` 这类固定子路径要避免被误当成 workspace id。
- `PATCH` 和 `DELETE` 需要补齐 method 分支。
- 请求体 decode 失败或字段非法走 usage 类错误。
- 成功删除建议返回 `204 No Content`。

### 6. 更新协议/前端类型所需响应字段

后端实现后，如果仓库内 TS 类型需要配合编译，应最小化更新：

- `web/src/protocol/terminal.ts` 中的 `SessionSummary` 增加 `name`。
- `WorkspaceSummary` 增加 `sort_order`，如果现有前端编译依赖该字段。
- 不做 UI 重构，不实现拖拽，不调整业务交互；仅保证类型与后端契约不冲突。

如果后端测试和 Go 编译不依赖前端类型，可将前端类型更新留到后续前端任务，但 Verification 需说明。

### 7. 增加和调整测试

#### state 层

文件：`internal/state/store_test.go`

新增覆盖：

- session 保存/读取包含 `Name`。
- workspace 保存/读取包含 `SortOrder`。
- `ListWorkspaces` 按 `SortOrder` 稳定排序。
- 按 `workspace_id` 查询 sessions。
- `DeleteSession` 只删除 session 目录，不影响 workspace metadata。
- `DeleteWorkspace` 删除 state workspace 目录。

#### registry 层

文件：`internal/webterminal/registry_test.go`

新增/调整覆盖：

- `CreateSession` 缺 name 返回 usage error。
- `CreateSession` 持久化 name，并在 summary 返回。
- `UpdateSession` 作为会话编辑接口更新 name，不影响 runtime。
- `DeleteSession` 拒绝 running/live session。
- `DeleteSession` 允许 stopped/failed session。
- `WorkspaceTree` 返回 workspace + sessions 两级结构。
- `ListSessionsByWorkspaceId` 只返回指定 workspace sessions。
- `DeleteWorkspace` 有 running session 时拒绝。
- `DeleteWorkspace` 无 running session 时级联删除 stopped sessions 的 state 数据。
- `UpdateWorkspaceOrder` 持久化排序并影响列表顺序。

#### webserver 层

文件：`internal/webserver/server_test.go`

新增覆盖：

- `GET /api/workspaces/tree` 返回两级结构。
- `PATCH /api/workspaces/order` 校验和返回排序结果。
- `DELETE /api/workspaces/{workspace_id}` 对 running session 返回错误。
- `GET /api/workspaces/{workspace_id}/sessions` 返回指定 workspace sessions。
- `POST /api/sessions` 缺 name 返回 4xx usage error。
- `PATCH /api/sessions/{session_id}` 是会话编辑接口，当前支持更新 name。
- `DELETE /api/sessions/{session_id}` 拒绝 running、允许 stopped。
- 现有 `history`、`ws`、`close` 路由仍可匹配。

### 8. 最小清理

- 更新所有 Go 编译错误和测试 fixture。
- 删除实现过程中产生的无用 helper，不保留一次性历史数据适配 hack。
- 不引入认证、用户体系、云同步或前端 UI 功能。
- 不提供 workspace 编辑接口；workspace 本轮只支持列表、tree、排序和删除。
- 不执行真实 workspace path 删除。

## Files to change

预计修改：

- `internal/session/session.go`
- `internal/session/manager.go`
- `internal/workspace/workspace.go`
- `internal/state/store.go`
- `internal/state/store_test.go`
- `internal/webterminal/registry.go`
- `internal/webterminal/registry_test.go`
- `internal/webserver/server.go`
- `internal/webserver/server_test.go`

可能修改：

- `web/src/protocol/terminal.ts`：仅在前端类型需要跟随后端响应字段时更新。
- `web/src/features/sessions/api.ts`：如果 TypeScript build 因 create session request 缺 name 类型失败，最小化补齐类型；不做 UI 交互重构。
- `web/src/features/workspaces/api.ts`：如果需要声明新增 API client 类型，最小化补齐；不做 UI 重构。

不计划修改：

- 真实 workspace 目录内容。
- 认证/用户体系相关设计。
- WebSocket terminal 协议本身。
- xterm UI 结构或前端拖拽排序 UI。

## Verification plan

进入 Verification 阶段后执行或收集：

1. Go 单元测试：
   - `PATH="/Users/leon/.version-fox/sdks/golang/bin:$HOME/go/bin:$PATH" go test ./cmd/... ./internal/...`
2. Go vet：
   - `PATH="/Users/leon/.version-fox/sdks/golang/bin:$HOME/go/bin:$PATH" go vet ./cmd/... ./internal/...`
3. 如项目环境可用，运行 golangci-lint：
   - `PATH="/Users/leon/.version-fox/sdks/golang/bin:$HOME/go/bin:$PATH" golangci-lint run ./cmd/... ./internal/...`
4. 如果修改了前端 TS 文件，运行前端检查：
   - `yarn --cwd /Volumes/media/SourceCodes/mywork/TermBridge-go/web typecheck`
   - `yarn --cwd /Volumes/media/SourceCodes/mywork/TermBridge-go/web lint`
5. 对照 requirement acceptance criteria 做 checklist。
6. 检查 diff，确认没有删除真实 workspace path 的逻辑。

## Blockers

暂无当前阻塞项。

## Assumptions

1. 本轮不做旧数据兼容，不做数据迁移；新接口以新契约为准。
2. 新建 session 从本功能开始严格要求 `name`；旧前端如果仍不传 name，会收到 usage error。
3. workspace order 使用 `SortOrder int` 存在 `workspace.json`，而不是单独 order 文件。
4. workspace list/tree 的 workspace 排序必须稳定；session 排序在实现中固定并用测试锁定。
5. 删除 workspace/session 只操作 TermBridge state root 内部目录。

## Risks

1. 如果 handler 路由分派不严谨，`/api/workspaces/tree` 或 `/api/workspaces/order` 可能被误解析为 workspace id。
2. 如果 delete 逻辑只看 state 不看 runtime map，可能误删 live session 数据。
3. 如果只看 runtime map 不看 persisted state，stale running session 可能被误删。
4. 如果前端当前 create session 未同步 name，新后端会按新契约拒绝请求；这符合需求，但需要在交付说明中提示。
5. 如果 workspace order 对部分列表请求的语义没有测试锁定，后续顺序可能不稳定。

## Rollback

如实现后需要回滚：

1. 回退新增 REST handler 和 registry 方法，恢复旧 API 表面。
2. 保留或回退新增 JSON 字段需谨慎：
   - `Name` 和 `SortOrder` 是新契约字段；如完全回滚，应同步回退相关读写和测试。
   - 若完全回滚，应同步回退相关 tests 和前端类型。
3. 删除能力回滚时确认没有残留 handler 仍调用未实现 store 方法。
4. 不需要恢复真实 workspace 目录，因为本功能不应修改真实 workspace 目录。

## User review notes

本 Plan 将实现范围限定在后端接口和必要类型/测试更新，不进入前端 UI 重构，不引入临时认证，不删除真实 workspace 目录。

暂无需要用户确认的未决事项。
