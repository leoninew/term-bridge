# Structured API Error Verification
最后修改时间: 2026-06-24 20:16:35

## Review status

Draft

## Flow mode / Stage

标准模式 / standard；当前阶段：验证 / Verification。

## Requirement basis

- Requirement: `docs/requirement/20260624-structured-api-error.md`
- Requirement status: `Accepted`
- Plan: `docs/plan/20260624-structured-api-error.md`
- Plan status: `Accepted`
- Spec: 不适用；本任务采用 standard 模式，未创建独立 Spec 阶段。

## Requirement alignment

| Requirement / Acceptance | Verification result |
| --- | --- |
| Gateway HTTP API error body 使用 `{ code, error, requestId, details? }` | 已实现。新增 `internal/transport/http/gatewayapi/errors.go`，集中输出 `code`、`error`、`requestId`，`details` 为 optional 且当前未强制调用点构造。 |
| HTTP status 不放入 JSON body | 已对齐。错误 body 未包含 `status` 字段，状态仍由 HTTP status line 表达。 |
| 成功响应不增加统一 envelope | 已对齐。`writeJSON` 和前端 axios client 均保持成功响应直接使用 DTO / `response.data`。 |
| 普通 HTTP API 错误入口统一转换 | 已覆盖核心路径：invalid JSON、method not allowed、unauthorized、not found、device offline、upstream failure、history JSON decode failure、terminal ws 升级前 conflict / offline、agent tunnel unauthorized。 |
| 不直接向 API body 透传内部错误 | 已实现默认安全摘要；`DebugErrors` 为 true 时才允许输出有限 cause，并通过敏感词过滤回退到安全摘要。 |
| request id 接入响应、日志和下游 | 已实现：`X-Request-ID` 沿用或生成并回写，error body `requestId` 一致；request log / gateway API error log / agent request log 均使用 `request_id`；普通 tunnel request 和 terminal attach frame 携带 `request_id`。 |
| 前端引入 axios 并统一注入 request id / 处理错误 | 已实现。新增 `web/src/features/api/client.ts` 和测试，feature API 已从 `fetch` 迁移到集中 axios client。 |
| 不兼容旧 `{ code, message, error }` | 已实现。axios response interceptor 只接受 best-practices error body；旧结构或纯文本错误走 contract mismatch。 |
| DTO 命名使用 `Req` / `Resp`，代码标识符使用 `Id` 而非 `ID` | 已执行触达范围重命名：前端 API DTO、后端 terminal registry DTO、tunnel request/response DTO 和相关引用已收敛；JSON 字段未因此改变。 |
| WebSocket 升级后的 terminal protocol 不强行重写 | 已对齐。HTTP 升级前错误结构化；升级后的 terminal control protocol 仍保留现有 `{ type, code, message }`。 |
| 不执行 git 写操作 | 已遵守。验证过程中只执行 `git status` / `git diff` / `git diff --cached` 等只读命令，未执行 `git add`、`git commit`、`git push` 等写操作。 |

## Spec alignment

不适用。standard 模式未创建独立 Spec 文档；按已接受 Requirement 和 Plan 核对。

## Plan alignment

| Plan step | Verification result |
| --- | --- |
| Step 1: 新增前端 axios API client 和 request id interceptor | 已完成。新增 `web/src/features/api/client.ts`，request interceptor 注入 `X-Request-ID`，response interceptor 归一化结构化错误。 |
| Step 2: 新增 gatewayapi structured error helper | 已完成。新增 `internal/transport/http/gatewayapi/errors.go`，包含 error code/message、request id、writer、debug error text 和日志。 |
| Step 3: tunnel request 传递 request id | 已完成。`RequestReq` / `TerminalAttachReq` 增加 optional `request_id`，gateway 调用下游时传递 request id。 |
| Step 4: auth middleware 写结构化错误 | 已完成。`auth.Manager.Middleware` 现在必须接收调用方提供的 unauthorized handler，不再保留纯文本 fallback 或旧错误契约兼容入口。 |
| Step 5: 替换 gatewayapi `http.Error` 路径 | 已完成。静态搜索 `internal/transport/http/gatewayapi/**/*.go` 未发现 `http.Error`。 |
| Step 6: API 404 / frontend fallback 边界 | 已完成计划范围。gateway handler 内 `/api` 和 `/api/` unknown route 返回结构化 not_found；未新增非 API 静态服务或 SPA fallback，避免扩大范围。 |
| Step 7: API contract DTO 重命名 | 已完成触达范围。旧 `RequestPayload` / `ResponsePayload` / `TerminalAttachPayload`、前端旧 API request/response 类型名静态搜索无匹配。 |
| Step 8: feature API 迁移到 axios client | 已完成。`web/src` 中静态搜索未发现 `fetch(` 调用。 |
| Step 9: Tests | 已完成并通过相关后端、前端和更广验证命令。 |

## Actual diff summary

### New files

- `docs/requirement/20260624-structured-api-error.md`：记录本任务 accepted requirement。
- `docs/plan/20260624-structured-api-error.md`：记录本任务 accepted implementation plan。
- `docs/verification/20260624-structured-api-error.md`：本验证文档。
- `internal/transport/http/gatewayapi/errors.go`：gateway API 结构化错误响应、request id、日志与 debug error helper。
- `web/src/features/api/client.ts`：集中 axios API client、request id interceptor、结构化错误归一化。
- `web/src/features/api/client.test.ts`：axios client 单元测试。

### Modified backend files

- `internal/transport/http/gatewayapi/server.go`：替换纯文本 `http.Error` / `http.NotFound` / method helper，接入结构化错误；relay / history / terminal pre-upgrade 错误使用统一 writer；agent tunnel unauthorized 结构化；terminal attach 携带 request id。
- `internal/transport/http/gatewayapi/auth/auth.go`：`Middleware` 改为必须注入 unauthorized handler，移除旧纯文本 unauthorized fallback。
- `internal/transport/http/gatewayapi/route.go`：普通 agent request 携带 request id，并按 `ResponseResp` 解码。
- `internal/protocol/tunnel/frame.go`：tunnel DTO 改名为 `RequestReq` / `ResponseResp` / `TerminalAttachReq`，追加 optional `request_id`。
- `internal/application/agent/client.go`、`internal/application/agent/runtime_access.go`、`internal/application/terminal/registry.go`：同步 DTO 命名和下游 protocol 类型变化；agent request / terminal attach 处理日志纳入 `request_id`。
- `internal/application/terminal/registry_test.go`、`internal/domain/identity/ulid_test.go`、`internal/transport/http/gatewayapi/*_test.go`：同步断言、DTO 名称和结构化错误测试。

### Modified frontend files

- `web/package.json`、`web/yarn.lock`：新增 axios 依赖。
- `web/src/protocol/terminal.ts`：API error 类型改为 `ApiErrorResp`，request/response DTO 改名为 `Req` / `Resp`。
- `web/src/features/gateway/api.ts`、`web/src/features/sessions/api.ts`、`web/src/features/workspaces/api.ts`：从裸 `fetch` 迁移到集中 axios client，成功响应保持自然 DTO。

## Expected vs actual changed files

### Expected and changed

- `internal/transport/http/gatewayapi/errors.go`
- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/server_test.go`
- `internal/transport/http/gatewayapi/relay_test.go`
- `internal/transport/http/gatewayapi/terminal_test.go`
- `internal/transport/http/gatewayapi/auth/auth.go`
- `internal/transport/http/gatewayapi/route.go`
- `internal/protocol/tunnel/frame.go`
- `internal/application/terminal/registry.go`
- `internal/application/agent/runtime_access.go`
- `internal/application/agent/client.go`
- `web/package.json`
- `web/yarn.lock`
- `web/src/protocol/terminal.ts`
- `web/src/features/sessions/api.ts`
- `web/src/features/gateway/api.ts`
- `web/src/features/workspaces/api.ts`
- `web/src/features/api/client.ts`
- `web/src/features/api/client.test.ts`

### Expected but not changed

- `internal/transport/http/server/server.go`：按 Plan 评估后无需修改；当前项目未在 Go server 中新增或修改 frontend static / SPA fallback，本轮只覆盖 gateway handler 内 API 404。
- `internal/transport/http/middleware/requestlog/logging_test.go`：未修改；现有 requestlog tests 通过，本轮未改变 requestlog middleware。

### Changed beyond explicit expected list

- `internal/application/terminal/registry_test.go`：同步后端 API DTO 命名和断言。
- `internal/domain/identity/ulid_test.go`：同步 `ID` → `Id` 标识符命名规则。
- `internal/transport/http/gatewayapi/auth_test.go`：同步 `auth.Manager.Middleware` 新签名，确保 unauthorized handler 必须由调用方注入。
- `internal/transport/http/gatewayapi/terminal_e2e_test.go`：同步 terminal attach / API error 行为断言。

这些额外改动均由 DTO 命名、request id、移除旧 auth fallback 和测试同步引起，未发现偏离需求的业务范围扩张。

## Acceptance checklist

- [x] Gateway HTTP API 错误响应默认为 JSON，至少包含 `code`、`error`、`requestId`。
- [x] `details` 为可选字段，本轮未要求调用点构造 details。
- [x] HTTP status 不放入 JSON body。
- [x] 成功响应保持自然 DTO，不增加统一包装层。
- [x] 前后端 API contract 层请求 DTO 使用 `Req`，响应 DTO 使用 `Resp`。
- [x] 触达代码标识符遵守 `Id` 命名规则；JSON / wire format 未因此改变。
- [x] 普通 HTTP API 错误入口进入统一转换路径，覆盖 invalid JSON、method not allowed、unauthorized、not found、device offline、upstream、history decode、terminal pre-upgrade conflict/offline。
- [x] 内部错误默认不直接透传到 API body。
- [x] 前端 axios request interceptor 注入 `X-Request-ID`。
- [x] 前端 axios response interceptor 归一化 `{ code, error, requestId, details? }`。
- [x] 不保留旧 `{ code, message, error }` 兼容分支。
- [x] `requestId` 接入 gateway API error 日志、下游 tunnel request / terminal attach，以及 agent 侧 request / terminal attach 业务日志。
- [x] 请求已携带 `X-Request-ID` 时后端沿用；否则生成并回写。
- [x] `error` 字段按 `DebugErrors` 控制默认安全摘要和调试 cause。
- [x] 测试覆盖结构化错误关键路径和前端 client 行为。
- [x] 未执行 git 写操作。

## Static checks

- `git diff --check`：通过，无 whitespace error。
- `rg "http\.Error" internal/transport/http/gatewayapi`：无匹配。
- `rg "MiddlewareWithUnauthorized|unauthorized\\n|text/plain.*unauthorized" **/*.go`：无旧 auth fallback 匹配；`auth.Manager.Middleware` 只保留必须注入 unauthorized handler 的新签名。
- `rg "TraceId|trace_id|traceId" internal web/src`：无匹配，产品代码已统一为 `requestId` / `request_id`。
- `rg "\buri\b|RequestURI\(" internal/**/*.go`：无匹配，请求日志已拆分为 `path` + `query`。
- `rg "request_id" internal/application/agent`：agent request / terminal attach 处理日志包含 `request_id`。
- `rg "\bfetch\s*\(" web/src`：无匹配。
- `rg "\b(RequestPayload|ResponsePayload|TerminalAttachPayload|CreateSessionRequest|RerunSessionRequest|UpdateSessionRequest|CreateSessionResponse|ApiErrorResponse)\b"`：无匹配。
- `rg 'json:"(message|debug|stackTrace|traceback)'`：仅发现既有 WebSocket / tunnel protocol control message 字段，不属于本轮 HTTP API error body；未发现结构化 HTTP error response 新增这些字段。

## Command results

| Command | Result |
| --- | --- |
| `go test ./internal/transport/http/middleware/requestlog/...` | Passed。 |
| `go test ./internal/transport/http/server/...` | Passed。 |
| `go test ./internal/application/agent/... ./internal/transport/http/gatewayapi/...` | Passed：affected Go packages 均通过。 |
| `go test ./internal/transport/http/gatewayapi/...` | Passed：`gatewayapi` 与 `gatewayapi/auth` 均通过。 |
| `go test ./cmd/... ./internal/...` | Passed；所有有测试包通过，无测试文件包正常跳过。 |
| `go vet ./cmd/... ./internal/...` | Passed；无输出。 |
| `yarn --cwd web typecheck` | Passed。 |
| `yarn --cwd web lint` | Passed。 |
| `yarn --cwd web test` | Passed：5 test files，23 tests。 |
| `yarn --cwd web format:check` | Passed；Prettier 检查通过。 |

## Missed or expanded scope

- 未修改 `internal/transport/http/server/server.go`：符合 Plan 中“仅在需要时修改”的约束；当前项目未发现必须调整的 Go 层 frontend serving / SPA fallback。
- 未修改升级后的 terminal control error protocol：符合 Non-goal 和 Plan，保留现有 WebSocket protocol。
- `auth.Manager.Middleware` 已移除旧纯文本 fallback；调用方必须显式注入 unauthorized handler，不保留旧错误契约兼容入口。
- 本轮没有实现 `details.fields` 细粒度校验错误：符合 Requirement 中 details 可以先不实现的约束。

## Risks

1. `DebugErrors: true` 仍可能暴露未被简单敏感词过滤识别的第三方错误文本；生产环境应保持关闭。
2. 本任务是前后端错误契约破坏性切换；旧客户端若仍期待 `{ code, message, error }` 将无法兼容。

## Incomplete items

无已知未完成项。

## Conclusion

验证通过。当前实现与 accepted Requirement / Plan 对齐：gateway HTTP API 错误响应已结构化，request id 已贯穿前端请求、后端响应、gateway 错误日志、下游 tunnel payload 和 agent 侧 request / terminal attach 业务日志；前端 HTTP API 已集中到 axios client；auth 不再保留旧纯文本 fallback；相关 DTO 命名和测试已同步。建议用户 review 本验证文档和当前 diff 后再决定是否提交。