# Workspace 与 Session 排序验证
最后修改时间: 2026-06-26 12:11:58

Review status: Accepted

## Requirement alignment

已按 `docs/requirement/20260626-workspace-session-ordering.md` 核对：

- Workspace 排序改为父级 `workspace_ids` 顺序数组模型。
- Session 排序改为 workspace 父级 `session_ids` 顺序数组模型。
- 后端返回 workspace/session 列表时按父级 ID 顺序数组排序。
- 新建 workspace/session 默认追加到对应父级顺序末尾。
- 前端支持每个 workspace 内 session 节点拖拽排序。
- Session 节点拖拽无单独拖拽手柄，整行可拖。
- 不同 workspace 的 session 拖拽列表互不干扰。
- 搜索过滤状态下禁用 workspace/session 拖拽。
- 前后端不再依赖 `sort_order` 字段。

## Spec alignment

不适用。轻量模式 / light 未创建单独 spec 文档，按 requirement / 需求核对。

## Plan alignment

不适用。轻量模式 / light 未创建单独 plan 文档，按 requirement / 需求核对。

## Actual diff summary

- `internal/domain/workspace/workspace.go`
  - 新增 `WorkspaceIndex`，用于根级 `workspace_ids` 顺序。
  - `Workspace` 移除 `SortOrder`，新增 `SessionIds`。
- `internal/infrastructure/repository/state/store.go`
  - 新增根级 workspace index 读写。
  - Workspace 列表按 `workspace_ids` 排序。
  - Workspace 创建时追加到 `workspace_ids`。
  - Session 创建时追加到 `session_ids`。
  - Session 列表按 `session_ids` 排序。
  - 删除 workspace/session 时清理对应 ID 顺序数组。
- `internal/application/terminal/registry.go`
  - 移除 workspace summary/tree 中的 `sort_order`。
  - 新增 `UpdateSessionOrder`。
  - Workspace scoped session summary 不再按 `updated_at` 覆盖排序。
- `internal/application/agent/*`、`internal/transport/http/gatewayapi/server.go`
  - 新增 `session_order` relay 和 HTTP endpoint。
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
  - 每个 workspace children 使用独立 `VueDraggable`。
  - session 节点整行可拖，无拖拽手柄。
  - 搜索状态禁用拖拽。
- `web/src/store/workspaceSessions.ts`、`web/src/features/sessions/api.ts`
  - 新增 session reorder API/store 乐观更新和失败回滚。
- `web/src/protocol/terminal.ts`
  - 移除 `sort_order` 类型字段。
- 测试文件
  - 更新 Go 和前端测试以覆盖新排序模型。

## Expected vs actual changed files

预期改动范围：后端 workspace/session state、registry、gateway/agent relay、前端 sidebar/API/store/protocol、相关测试与 light requirement/verification 文档。

实际改动符合预期范围。当前工作区中存在其他早先文档改动，未纳入本需求验证结论。

## Acceptance checklist

- [x] Workspace 排序由父级 `workspace_ids` 表达。
- [x] Session 排序由父级 `session_ids` 表达。
- [x] 后端返回列表按父级 ID 顺序数组排序。
- [x] 新建 workspace/session 默认追加到末尾。
- [x] session 节点支持所属 workspace 内拖拽排序。
- [x] session 节点无拖拽手柄。
- [x] 不同 workspace 的 session 排序互不干扰。
- [x] 搜索状态禁用拖拽。
- [x] 前后端不再依赖 `sort_order`。
- [x] 相关测试与类型检查通过。

## Test results

2026-06-26 12:11:58 重新执行验证。

已运行：

```bash
go test ./internal/infrastructure/repository/state ./internal/application/terminal ./internal/application/agent ./internal/transport/http/gatewayapi
```

结果：通过。

已运行：

```bash
cd web
npm test -- workspaceSessions.test.ts useCreateSessionDraft.test.ts
npm run typecheck
```

结果：通过。

## Missed or expanded scope

- 本次按用户要求扩展了既有 workspace 排序模型：从实体 `sort_order` 字段迁移为根级 `workspace_ids`。
- 本次未做浏览器人工拖拽验收；已通过 store/API/typecheck 和后端测试覆盖核心排序逻辑。

## Risks

- 旧数据迁移依赖 fallback 行为：缺少 `workspace_ids` / `session_ids` 时按创建时间、名称、ID 排序；新增项会初始化或追加到父级顺序数组。
- 如果未来允许跨 workspace 拖动 session，需要新增明确的数据迁移和 API 语义；当前明确不支持。

## Incomplete items

无已知未完成项。

## Conclusion

验证通过。实现满足 light requirement 中的排序数据模型、拖拽行为和兼容性约束。
