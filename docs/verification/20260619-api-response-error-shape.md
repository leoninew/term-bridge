# API 响应与错误结构规范化验证
最后修改时间: 2026-06-19 23:53:36

Review status: Accepted

## Requirement alignment

基于 `docs/requirement/20260619-api-response-error-shape.md`，Review status: Accepted。

| Requirement | Result | Notes |
|---|---|---|
| 成功响应直接返回接口预期内容，不使用统一 `code` / `data` 包装 | Pass | `/api/workspaces`、`/api/workspaces/order`、`/api/workspaces/{id}/sessions` 已改为 raw array；原本就是接口对象的 health / close response 保持对象。 |
| 错误响应为顶层 `{code,message,error}` | Pass | `internal/webserver/server.go` 新增 `apiErrorResponse`，统一 `writeAPIError`。 |
| `message` 为用户友好说明 | Pass | HTTP API 使用 `apperrors.Message(err)`，WebSocket protocol error 使用固定友好 message。 |
| `error` 为开发阶段调试信息，由配置控制 | Pass | 新增 `web.error.debug`，默认 false；`DebugErrors` 开启时填充 `apperrors.Debug(err)`。 |
| 业务 not found 返回 HTTP 404 与 `code: "not_found"` | Pass | 新增 `KindNotFound`，registry 中 session/workspace not found 映射到 `apperrors.NotFound`。 |
| WebSocket 能沿用就沿用，不能则说明 | Pass | Upgrade 前走 HTTP 错误结构；Upgrade 后保留 `type: "error"` protocol envelope，并增加 `error` 字段。 |

## Spec alignment

基于 `docs/spec/20260619-api-response-error-shape.md`，Review status: Accepted。

| Spec item | Result | Notes |
|---|---|---|
| HTTP error response 顶层字段 | Pass | `apiErrorResponse{Code, Message, Error}`。 |
| HTTP status/code 映射 | Pass | `statusCodeForError` 覆盖 not_found、usage、config、runtime、internal。 |
| App error model extension | Pass | `KindNotFound`、`NotFound`、`Message`、`Debug` 已实现。 |
| Debug error 配置 | Pass | `WebErrorConfig`、默认 YAML、app 到 webserver wiring 已实现。 |
| 成功响应结构 | Pass | 计划中列出的 wrapper 已移除。 |
| API not found JSON 化 | Pass | API handler 中 `http.NotFound` 已替换为 `apiNotFound`。 |
| WebSocket error strategy | Pass | `ServerMessage.Error` 已添加，protocol error helper 按 debug 配置填充。 |

## Plan alignment

基于 `docs/plan/20260619-api-response-error-shape.md`，Review status: Accepted。

计划步骤完成情况：

1. 扩展错误模型：Pass。
2. 增加配置项：Pass。
3. 传递配置到 webserver：Pass。
4. 统一 HTTP 错误响应 helper：Pass。
5. 统一特殊 HTTP 错误：Pass。
6. 调整业务 not found 映射：Pass。
7. 拆掉成功响应无必要包装：Pass。
8. 调整 WebSocket 错误结构：Pass。
9. 同步前端 API client：Pass。
10. 更新测试：Pass。

## Actual diff summary

本次验证范围内的实际代码改动：

- `configs/termbridge.default.yaml`
- `internal/app/app.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/config/termbridge.default.yaml`
- `internal/errors/errors.go`
- `internal/terminalproto/protocol.go`
- `internal/webserver/server.go`
- `internal/webserver/server_test.go`
- `internal/webterminal/registry.go`
- `web/src/features/sessions/api.ts`
- `web/src/features/workspaces/api.ts`
- `web/src/protocol/terminal.ts`

统计：13 个文件，约 260 行新增、72 行删除。

过程文档新增：

- `docs/requirement/20260619-api-response-error-shape.md`
- `docs/spec/20260619-api-response-error-shape.md`
- `docs/plan/20260619-api-response-error-shape.md`
- `docs/verification/20260619-api-response-error-shape.md`

## Expected vs actual changed files

| Expected | Actual | Notes |
|---|---|---|
| `internal/errors/errors.go` | Changed | 符合计划。 |
| `internal/config/config.go` | Changed | 符合计划。 |
| `internal/config/termbridge.default.yaml` | Changed | 符合计划。 |
| `configs/termbridge.default.yaml` | Changed | 计划未显式列出，但项目测试要求 public default 与 embedded default 一致，属于必要同步。 |
| `internal/config/config_test.go` | Changed | 符合计划“可能修改”。 |
| `internal/app/app.go` | Changed | 符合计划。 |
| `internal/webserver/server.go` | Changed | 符合计划。 |
| `internal/webserver/server_test.go` | Changed | 符合计划。 |
| `internal/webterminal/registry.go` | Changed | 符合计划。 |
| `internal/webterminal/registry_test.go` | Not changed | 未发现必须更新的现有断言；webserver 层已覆盖 not found HTTP 行为。 |
| `internal/terminalproto/protocol.go` | Changed | 符合计划。 |
| `internal/terminalproto/*_test.go` | Not changed | 未发现需要调整的现有测试。 |
| `web/src/features/workspaces/api.ts` | Changed | 符合计划。 |
| `web/src/features/sessions/api.ts` | Changed | 符合计划。 |
| `web/src/protocol/terminal.ts` | Changed | 符合计划。 |

## Acceptance criteria checklist

- [x] `GET /api/sessions` 返回 raw array，并有现有测试覆盖。
- [x] `GET /api/workspaces` 返回 raw array，并新增测试覆盖。
- [x] workspace/session 新接口成功响应不再使用无必要包装。
- [x] registry/domain error 返回顶层 `{code,message,error}`。
- [x] invalid JSON request 返回顶层 `{code,message,error}`。
- [x] method not allowed 返回顶层 `{code,message,error}`。
- [x] API route not found 返回顶层 `{code,message,error}`。
- [x] 业务 not found 返回 `404 not_found`。
- [x] `DebugErrors=false` 时 `error` 为空字符串。
- [x] `DebugErrors=true` 时 `error` 包含调试信息。
- [x] WebSocket error 保留 `type: "error"` 并支持 `error` 字段。
- [x] 前端 API client 已适配 raw workspace array 与新错误结构。
- [x] 前端 HTTP API client 统一通过 `responseError()` 解析顶层 `{code,message,error}`。
- [x] 前端 UI-facing HTTP 错误默认展示 `message`，不展示 `error` 调试详情。
- [x] 前端 WebSocket protocol 类型支持 `type: "error"` 的可选 `error` 字段。

## Frontend adaptation check

本次补充检查前端适配范围：

| Area | Result | Evidence | Notes |
|---|---|---|---|
| Raw sessions array | Pass | `web/src/features/sessions/api.ts` 的 `listSessions()` 返回 `SessionSummary[]` | 不读取旧 `{sessions}` 包装。 |
| Raw workspaces array | Pass | `web/src/features/workspaces/api.ts` 的 `listWorkspaces()` 返回 `WorkspaceSummary[]` | 不读取旧 `{workspaces}` 包装。 |
| Workspace tree raw array | Pass | `listWorkspaceTree()` 返回 `WorkspaceTreeSummary[]` | 与后端 `/api/workspaces/tree` 保持一致。 |
| HTTP error parsing | Pass | `responseError()` 解析 `ApiErrorResponse` 的 `code` / `message` | 返回字符串包含 prefix、HTTP status、code、message。 |
| UI debug detail exposure | Pass | `responseError()` 不拼接 `parsed.error` | 默认不把开发调试详情展示给普通 UI。 |
| WebSocket error type | Pass | `ServerControlMessage` 包含 `{ type: 'error'; code; message; error? }` | 保留 protocol envelope，并支持调试字段。 |
| Fetch 调用集中处理错误 | Pass | `web/src/features/sessions/api.ts`、`web/src/features/workspaces/api.ts` 中 HTTP API 均在 `!response.ok` 时调用 `responseError()` | `readHistory()` 返回 text，错误路径仍统一解析。 |

发现的边界：

- `web/src/features/workspaces/api.ts` 当前没有导出 workspace order 或按 workspace 查询 sessions 的前端 client；因此不存在旧 `{workspaces}` / `{sessions}` 包装读取点需要继续修改。
- `web/src/features/sessions/useTerminalSocket.ts` 对 WebSocket `type: "error"` 消息只负责 decode 后交给上层 `onControl`；当前 `TerminalView` 对控制消息中的 `error` 类型没有额外 UI 展示逻辑。本次需求只要求类型兼容与不默认暴露 debug detail，因此未判定为缺口。
- 当前前端没有 API client 单元测试；已有验证依赖 typecheck 与代码核对。

## Test results

### Targeted Go tests

Command:

```bash
go test ./internal/errors ./internal/config ./internal/webserver ./internal/webterminal ./internal/terminalproto
```

Result: Pass。

### Full Go tests

Command:

```bash
go test ./...
```

Result: Pass。

### Web typecheck

Command:

```bash
npm --prefix web run typecheck
```

Result: Pass。

## Missed or expanded scope

Expanded scope:

- 同步修改 `configs/termbridge.default.yaml`。原因：`internal/config` 测试要求 public default config 与 embedded default config 保持一致。
- 前端 `responseError` 保持 UI message 简洁，不展示 `error` 调试字段；这符合 Plan 中“不默认展示 error 堆栈”的约束。

未完成项：无。

## Risks

1. API breaking change 仍然存在：旧调用方如果依赖 `{workspaces}` / `{sessions}` 或旧嵌套错误结构，需要同步更新。
2. `web.error.debug: true` 会暴露底层错误字符串，应仅在开发阶段启用。
3. 当前 `error` 字段实现为错误字符串/错误链，不是真正 runtime stack trace；这符合 Spec 中“不强制引入第三方 stack 库”的设计，但与“堆栈”一词相比更偏调试详情。
4. 当前前端 API client 缺少单元测试，错误响应适配主要通过 typecheck 与代码核对确认。
5. 当前 working tree 中存在本次变更以外的未提交/已暂存改动，提交前需要拆分确认。

## Incomplete items

无。

## Conclusion

验证通过。实现与 Requirement / Spec / Plan 对齐，相关 Go 测试与 Web typecheck 均通过。
