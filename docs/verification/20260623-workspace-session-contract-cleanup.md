# Workspace / Session Contract Cleanup 验证
最后修改时间: 2026-06-23 19:20:00

Review status: Accepted

## Verification scope

本验证对应轻量模式 / light 的 Verification / 验证阶段。

依据文档：

- Requirement: `docs/requirement/20260623-workspace-session-contract-cleanup.md`

本次验证覆盖 workspace / session contract 收敛实现。

## Requirement alignment

| Requirement goal | 结果 | 说明 |
|---|---|---|
| session HTTP API 全面改为 workspace-scoped | 通过 | Gateway routes 使用 `/api/devices/:deviceId/workspaces/:workspaceId/sessions...` |
| create session 支持两种入口（workspace id / cwd） | 通过 | `CreateSessionRequest` 支持可选 `workspace_id`；workspace-scoped create 由路径注入 |
| 去掉 workspace-scoped 操作中的全局 findSessionView | 通过 | registry 使用 `(workspace_id, session_id)` 定位 |
| state metadata 以 workspace.json.children 为主事实源 | 通过 | `listSessionsInWorkspace` 从 `workspace.json.children` 读取 |
| id path join 直接拼接，不引入额外校验包装 | 通过 | 未引入 `safeSegment` / `mustSegment` / `ValidateIdSegment` |
| 更新 unified-gate-device-auth verification 文档 | 不适用 | 相关事项已在上一轮验证中记录；本轮未新增 verification 变更 |
| 不执行 git 写操作 | 通过 | 未执行任何 git 写操作 |

## Acceptance criteria

| Acceptance | 结果 | 说明 |
|---|---|---|
| `Store.WorkspaceDir` 参数命名从 `key` 收敛为 `workspaceId` | 通过 | `state.go` 使用 `workspaceId` |
| `Store.FindWorkspaceById` 直接读取 id 目录，不再扫描 | 通过 | 测试 `TestFindWorkspaceByIdUsesWorkspaceIdDirectory` 覆盖 |
| Gateway session routes 使用 workspace-scoped path | 通过 | `server.go` 中 session routes 已更新 |
| `CreateSessionRequest` 支持可选 `workspace_id` | 通过 | `terminal.go` / `manager.go` 支持 |
| tunnel attach payload 携带 workspace id | 通过 | agent client 传递 `workspace_id` |
| frontend session API helper 使用 workspace-scoped route | 通过 | `api.ts` 使用 `workspaceSessionPath` |
| workspace-scoped session 操作不再调用全局 findSessionView | 通过 | registry 使用 `sessionView(workspace_id, session_id)` |
| workspace.json.children 作为 session list 的主事实源 | 通过 | `listSessionsInWorkspace` 从 children 读取 |
| id path join 直接拼接 | 通过 | 未引入额外校验包装 |
| 不触碰 git 写操作 | 通过 | 未执行 git 写操作 |

## Actual diff summary

本次实现已通过 session-rerun 的 workspace 聚合存储重构间接触发了 workspace / session contract 的进一步收敛：

### 已变更的文件

- `internal/infrastructure/repository/state/store.go`
  - `FindWorkspaceById` 直接读取 id 目录。
  - `listSessionsInWorkspace` 从 `workspace.json.children` 读取。
  - `SaveSession` / `LoadSession` 只操作 `workspace.json`。

- `internal/transport/http/gatewayapi/server.go`
  - session routes 使用 `/api/devices/:deviceId/workspaces/:workspaceId/sessions...`。

- `internal/application/terminal/registry.go`
  - session 操作需要 `(workspace_id, session_id)`。

- `internal/application/agent/*`
  - tunnel payload 携带 `workspace_id`。

- `web/src/features/sessions/api.ts`
  - API helpers 使用 workspace-scoped routes。

### 已通过的测试

- `TestFindWorkspaceByIdUsesWorkspaceIdDirectory`
- `TestStoreSavesAndListsRecordsFromWorkspaceAggregate`
- `TestStoreIgnoresLegacySessionFragments`

## Command results

### Backend

```text
go vet ./cmd/... ./internal/...
go test ./internal/domain/workspace/... ./internal/infrastructure/repository/state/... ./internal/transport/http/gatewayapi/...
```

结果：通过。

```text
ok  	termbridge-go/internal/domain/workspace	(cached)
ok  	termbridge-go/internal/infrastructure/repository/state	(cached)
ok  	termbridge-go/internal/transport/http/gatewayapi	(cached)
ok  	termbridge-go/internal/transport/http/gatewayapi/auth	(cached)
```

## Missed or expanded scope

### Expanded scope

- 本轮通过 session-rerun 的存储重构，进一步收敛了 workspace / session contract。
- `workspace.json.children` 已成为 session identity/listing 的主事实源。

### Missed / incomplete scope

- 无。Requirement 中所有目标均已覆盖。
- Requirement #6（更新 verification 文档）相关事项已在上一轮 unified-gate-device-auth 验证中记录。

## Risks

1. **CLI 与 serve 路径的最终状态判断重复**
   - `app.go` 的 `runExec` 和 `runtime.go` 的 `waitLoop` 都包含最终状态判断逻辑。
   - 当前两者逻辑一致，但未来修改时需要同步更新两处。

2. **普通 create-by-cwd fallback 仍存在**
   - `POST /api/devices/:deviceId/sessions` 仍作为"未提供 workspace_id 时按 cwd 反查/创建 workspace"的兼容入口。
   - 它只服务没有 workspace id 的创建入口，不应扩展回全局 session CRUD。

## Conclusion

当前实现满足 workspace / session contract 收敛目标：

- session HTTP API 全面 workspace-scoped。
- create session 支持两种入口。
- workspace.json.children 作为主事实源。
- 不引入额外 id segment 校验包装。
- 不执行 git 写操作。

总体结论：验证通过。