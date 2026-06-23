# Workspace / Session Contract Cleanup Requirement
最后修改时间: 2026-06-23 18:29:31

## Review status

Accepted

## Flow mode / Stage

轻量模式 / light；需求 / Requirement 已接受，当前推进实现 / Implementation。

## Background

本轮清理已经删除 API / frontend / domain 中的 `workspace_key`、workspace `key`、session response `log_path`、`CreateSessionResponse.ws_url` 等旧字段，并将 state layout 收敛到 `workspaces/<workspace_id>/...`。

继续审视后发现仍有几个会造成长期混乱的反模式：session HTTP API 仍残留 device 下全局 session 形态，create session 未区分“已有 workspace id”和“按 cwd 创建/复用 workspace”，registry 内部存在按 `session_id` 全局找 view 的路径，`FindWorkspaceById` 在目录已经以 id 命名后仍可能退化为扫描，`session.json` 与 `workspace.json.children` 的事实源边界不清晰，验证文档仍记录已被删除的 `ws_url` 风险。

## Goal

本轮目标是把 workspace / session contract 收敛到当前确定的模型：

1. session HTTP API 的读、改、删、关闭、历史、terminal attach 全面改为 workspace-scoped：`/api/devices/:deviceId/workspaces/:workspaceId/sessions/:sessionId...`。
2. create session 支持两种入口：
   - 已有 workspace 入口提供 `workspace_id`，直接使用 `workspaces/<workspace_id>`。
   - 普通 cwd 入口不提供 `workspace_id`，按 cwd resolve / create workspace。
3. 去掉 workspace-scoped 操作中的 `findSessionView(sessionId)` 全局扫描，使用 `(workspace_id, session_id)` 定位。
4. state metadata 以 `workspace.json.children` 作为 session identity/listing 的主事实源；`session.json` 保留剩余 session 记录，不作为列表反查来源。
5. workspace/device/session id 均由系统生成或由受控协议传入，路径拼接直接使用 id；不再引入 `safeSegment` / `mustSegment` / `ValidateIdSegment` 这类包装。
6. 更新 `docs/verification/20260623-unified-gate-device-auth.md`，移除已过期的 `CreateSessionResponse.ws_url` 风险，记录不做持久化 Gate snapshot 的决定，以及 workspace-scoped session API 的现状。
7. 不执行 git 写操作。

## Non-goal

本轮不做以下事项：

- 不迁移旧 state 数据。
- 不实现持久化 Gate snapshot；离线只读 cache 保持进程内缓存，Gate 重启后不保留。
- 不完成 API structured error contract；该项仍作为后续风险。
- 不把 terminal socket/xterm 状态迁移到 Pinia。
- 不重写历史 SpecFlow 文档中的旧方案，只更新本轮指定 verification 文档。
- 不提交 git，不执行 git 写操作。

## User scenarios

1. 作为前端调用方，我通过 workspace tree 已知 workspace id 和 session id 后，所有 session 操作都应直接命中 workspace-scoped path，不需要额外 path -> workspace 反查。
2. 作为创建会话入口，我从 workspace 加号创建时应把 workspace id 带给后端；从普通入口创建时才允许按 cwd resolve / create workspace。
3. 作为维护者，我希望相同 session id 即使理论上出现在不同 workspace 下，也不会被全局扫描误命中。
4. 作为后续开发者，我希望 `workspace.json` 是列表与 workspace tree 的主入口，`session.json` 不再和 workspace children 竞争事实源。
5. 作为验证文档读者，我不应再看到已删除字段 `workspace_key` / `log_path` / `ws_url` 被描述成当前实现风险。

## Acceptance

- [x] `Store.WorkspaceDir` 参数命名从 `key` 收敛为 `workspaceId`。
- [x] `Store.FindWorkspaceById` 直接读取 id 目录，不再调用 `ListWorkspaces()` 扫描。
- [x] 增加测试覆盖：workspace JSON 若放在非 id 目录下，即使 JSON 内 `workspace_id` 匹配，也不应被 `FindWorkspaceById(id)` 找到。
- [x] Gateway session read/update/delete/close/history/ws routes 使用 `/api/devices/:deviceId/workspaces/:workspaceId/sessions...`。
- [x] `CreateSessionRequest` 支持可选 `workspace_id`；workspace-scoped create 由 path 注入 workspace id，普通 create 仍按 cwd resolve。
- [x] tunnel attach payload 和 runtime access session 操作携带 workspace id。
- [x] frontend session API helper 和 `SessionsView` 使用 workspace-scoped session route。
- [x] workspace-scoped session 操作不再调用全局 `findSessionView(sessionId)`。
- [x] `workspace.json.children` 作为 session list/load 的主事实源。
- [x] id path join 直接拼接，不引入额外 id segment 校验包装。
- [x] 更新 `docs/verification/20260623-unified-gate-device-auth.md`。
- [x] 运行相关 Go 测试和前端 typecheck。
- [x] 不触碰 git 写操作。

## Open questions

暂无需要用户确认的未决事项。当前实现按用户最新纠正执行：id 直接拼接，不做额外 path segment validation helper。

## Decisions

- 对已经携带 workspace id 的 session API，不做 path -> workspace 查找/索引；直接使用 `workspaces/<workspace_id>/sessions/<session_id>`。
- 普通 create-by-cwd 入口可以保留 `POST /api/devices/:deviceId/sessions`，作为“未提供 workspace_id 时按 cwd 反查/创建 workspace”的兼容入口；读、改、删、关闭、历史和 ws 不保留全局 session route。
- `workspace_id` 是 state 目录事实路径，不再把 path hash / workspace key 当作目录语义。
- `FindWorkspaceById` 不通过扫描所有 workspace 实现。
- `workspace.json.children` 主导 workspace tree 和 session listing；`session.json` 保留 session 记录但不作为列表反查依据。
- 不实现持久化 Gate snapshot；文档需明确这是非目标。

## Risk

- 直接按 id 目录读取会暴露旧数据或损坏数据中“目录名与 workspace_id 不一致”的问题；这是期望中的 fail-fast 方向，而不是兼容目标。
- 普通 create-by-cwd fallback 仍存在 `/api/devices/:deviceId/sessions` POST；它只服务没有 workspace id 的创建入口，不应扩展回全局 session CRUD。
- 历史 requirement/spec/verification 文档仍包含旧 `workspace_key` / `ws_url` 方案；本轮只更新指定 verification 文档，历史方案文档不作为当前 contract。
