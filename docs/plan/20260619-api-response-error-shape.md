# API 响应与错误结构规范化实施计划
最后修改时间: 2026-06-19 23:14:56

Review status: Accepted

## Basis

本计划基于：

- `docs/requirement/20260619-api-response-error-shape.md`，Review status: Accepted
- `docs/spec/20260619-api-response-error-shape.md`，Review status: Accepted

目标：统一 HTTP API 成功响应与错误响应结构，补充 not found 错误分类，通过配置控制开发阶段调试错误输出，并让 WebSocket 错误尽可能沿用相同语义。

## Implementation steps

### 1. 扩展错误模型

修改 `internal/errors/errors.go`：

1. 新增错误类型：
   - `KindNotFound Kind = "not_found"`
2. 新增构造函数：
   - `NotFound(message string, err error) error`
3. 新增 helper，用于分离用户友好说明与调试信息：
   - `Message(err error) string`
   - `Debug(err error) string`
4. 调整或保留现有 helper：
   - `KindOf(err)` 识别 `KindNotFound`
   - `FormatUser(err)` 如仍被其他 CLI 使用，不直接删除；HTTP API 新逻辑不再用它作为 `message`

设计约束：

- `Message(err)` 对 app error 返回 `Msg`，对普通 error 返回通用或 `err.Error()`，以不扩大本次范围为准。
- `Debug(err)` 对 app error 返回完整错误字符串或 unwrap 后错误链；对普通 error 返回 `err.Error()`。

### 2. 增加配置项

修改 `internal/config/config.go`：

1. 新增：
   ```go
   type WebErrorConfig struct {
       Debug bool
   }
   ```
2. 在 `WebConfig` 增加：
   ```go
   Error WebErrorConfig
   ```
3. 在 unknown key allowlist 增加：
   - `web.error.debug`

修改 `internal/config/termbridge.default.yaml`：

```yaml
web:
  error:
    debug: false
```

如已有 config tests 覆盖 unknown keys 或 defaults，需要同步调整测试。

### 3. 传递配置到 webserver

修改 `internal/app/app.go`：

- 在 `webserver.Config` 中传入：
  ```go
  DebugErrors: cfg.Web.Error.Debug,
  ```

修改 `internal/webserver/server.go`：

- 在 `Config` 增加：
  ```go
  DebugErrors bool
  ```
- `normalizeConfig` 不需要额外处理 bool 默认值。

### 4. 统一 HTTP 错误响应 helper

修改 `internal/webserver/server.go`：

1. 定义响应结构：
   ```go
   type apiErrorResponse struct {
       Code string `json:"code"`
       Message string `json:"message"`
       Error string `json:"error"`
   }
   ```
2. 修改 `errorBody` 或替换为 `apiErrorBody`，返回顶层 `{code,message,error}`。
3. 修改 `writeError` 映射：
   - `NotFound` -> `404`, `not_found`
   - `Usage` -> `400`, `usage_error`
   - `Config` -> `400`, `config_error`
   - `Runtime` -> `500`, `runtime_error`
   - `Internal` / plain error -> `500`, `internal_error`
4. `message` 使用 `apperrors.Message(err)`。
5. `error` 仅在 `s.config.DebugErrors` 为 true 时使用 `apperrors.Debug(err)`；否则为空字符串。

注意：当前 `writeError` 是 package-level 函数，没有 `Server` receiver。为了读取配置，计划改为：

```go
func (s *Server) writeError(w http.ResponseWriter, err error)
```

并更新所有调用点从 `writeError(w, err)` 到 `s.writeError(w, err)`。

对不依赖 app error 的直接错误：

- `decodeJSONRequest` 需要读取 debug 配置，因此建议改为 method：
  ```go
  func (s *Server) decodeJSONRequest(...)
  ```
- 或保留函数但传入 `debugErrors bool`。优先选择 method，减少参数传播。

### 5. 统一特殊 HTTP 错误

修改 `internal/webserver/server.go`：

1. `methodNotAllowed` 输出：
   ```json
   {"code":"method_not_allowed","message":"method not allowed","error":""}
   ```
2. origin guard 输出：
   ```json
   {"code":"forbidden_origin","message":"origin is not allowed","error":""}
   ```
3. invalid JSON 输出：
   ```json
   {"code":"invalid_request","message":"invalid request","error":"...debug when enabled..."}
   ```
4. 新增 API not found helper，替换 API handler 中所有 `http.NotFound(w, r)`：
   ```json
   {"code":"not_found","message":"not found","error":""}
   ```

### 6. 调整业务 not found 映射

修改 `internal/webterminal/registry.go`：

1. `findSessionView` 找不到 session 时返回：
   ```go
   apperrors.NotFound("session not found", nil)
   ```
2. `ListSessionsByWorkspaceId` 中 workspace 不存在时返回：
   ```go
   apperrors.NotFound("workspace not found", err)
   ```
3. `DeleteWorkspace` 中 workspace 不存在时返回：
   ```go
   apperrors.NotFound("workspace not found", err)
   ```
4. 检查其他把 `os.ErrNotExist` 包为 `Usage` 或 `Runtime` 的 API 路径，按语义改为 `NotFound`。

### 7. 拆掉成功响应无必要包装

修改 `internal/webserver/server.go`：

1. `GET /api/workspaces`：
   - 从 `map[string]any{"workspaces": workspaces}` 改为 `workspaces`
2. `PATCH /api/workspaces/order`：
   - 从 `map[string]any{"workspaces": workspaces}` 改为 `workspaces`
   - 注意 `handleWorkspaces` 与 `handleWorkspace` 中重复处理 `/order` 的分支都要改。
3. `GET /api/workspaces/{id}/sessions`：
   - 从 `map[string]any{"sessions": sessions}` 改为 `sessions`

保持：

- `/api/health` 返回 `{status:"ok"}`
- `/api/sessions` raw array
- session create/get/update raw struct
- close response `{state:"closing"}`
- DELETE `204` empty body

### 8. 调整 WebSocket 错误结构

修改 `internal/terminalproto/protocol.go`：

- `ServerMessage` 增加：
  ```go
  Error string `json:"error,omitempty"`
  ```

修改 `internal/webserver/server.go` WebSocket error 发送点：

1. 保留 `Type: terminalproto.TypeError`。
2. `Code` / `Message` 保持用户友好语义。
3. `Error` 在 `DebugErrors` 开启时包含底层错误或调试信息。
4. 对当前直接把 `err.Error()` 放入 `Message` 的位置，改为稳定友好 message，例如：
   - `write_failed` -> `write failed`
   - `invalid_control` -> `invalid control message`
   - `resize_failed` -> `resize failed`
   - `unknown_control` -> `unknown control message`

### 9. 同步前端 API client

需要修改：

1. `web/src/features/workspaces/api.ts`
   - `listWorkspaces()` 从解析 `{ workspaces }` 改为解析 raw `WorkspaceSummary[]`。
   - 检查 remove/list tree/order API 是否还有包装假设。
2. `web/src/features/sessions/api.ts`
   - `responseError()` 解析新的 `{code,message,error}`。
   - UI-facing thrown error 优先展示 `message`，可带 status 和 code。
   - 不默认展示 `error` 堆栈，避免开发调试信息直接污染普通 UI；如要展示，可在 message 后附加简短 detail，但本次计划保持前端用户提示简洁。
3. `web/src/protocol/terminal.ts`
   - 如需要，增加 HTTP error response type：
     ```ts
     export interface ApiErrorResponse { code: string; message: string; error: string }
     ```
   - WebSocket error message类型增加 `error?: string`。

### 10. 更新测试

Backend tests：

1. `internal/webserver/server_test.go`
   - `/api/sessions` 成功响应仍为 raw array。
   - 新增 `/api/workspaces` 成功响应 raw array 测试。
   - 新增 `/api/workspaces/{id}/sessions` raw array 测试，如测试 setup 可行。
   - 新增 invalid request 错误结构测试：顶层 `code/message/error`。
   - 新增 method not allowed 错误结构测试。
   - 新增 route not found JSON 错误结构测试。
   - 新增 not found 业务错误返回 `404 not_found` 测试，例如 `GET /api/sessions/missing`。
2. `internal/webterminal/registry_test.go`
   - 如已有 missing session/workspace 行为测试，更新期望为 `NotFound`。
3. `internal/config` tests
   - 如果存在 defaults 或 unknown key 测试，覆盖 `web.error.debug`。
4. `internal/terminalproto` tests
   - 如存在 server message marshal/validate 测试，加入 `error` 字段兼容。

Frontend：

- 当前未发现 `web/src` 下测试文件。若项目已有 lint/typecheck 命令，验证阶段运行。

## Files to change

预计修改：

- `internal/errors/errors.go`
- `internal/config/config.go`
- `internal/config/termbridge.default.yaml`
- `internal/app/app.go`
- `internal/webserver/server.go`
- `internal/webserver/server_test.go`
- `internal/webterminal/registry.go`
- `internal/webterminal/registry_test.go`（视测试影响）
- `internal/terminalproto/protocol.go`
- `web/src/features/workspaces/api.ts`
- `web/src/features/sessions/api.ts`
- `web/src/protocol/terminal.ts`

可能修改：

- `internal/config/config_test.go`
- `internal/terminalproto/*_test.go`
- 前端调用处，如果类型变化导致编译错误

## Verification plan

实现后，在 Verification 阶段执行：

1. 查看项目显式测试入口：
   - Go tests：优先运行相关包，再视情况运行全量 `go test ./...`。
   - Web：查看 `web/package.json` scripts，运行合适的 typecheck/lint/test/build 命令。
2. 建议命令：
   - `go test ./internal/errors ./internal/config ./internal/webserver ./internal/webterminal ./internal/terminalproto`
   - 如通过，再运行 `go test ./...`
   - web 侧根据 scripts 运行，例如 `yarn lint` / `yarn build` / `yarn test` 中存在的命令。
3. 手动或测试级核对响应结构：
   - 成功响应无无关 wrapper。
   - 错误响应顶层包含 `code/message/error`。
   - 业务 not found 是 `404 not_found`。
   - DebugErrors 关闭时 `error` 为空字符串。
   - DebugErrors 开启时 `error` 包含调试信息。
   - WebSocket error message 保留 `type: "error"` 并支持 `error` 字段。

## Rollback plan

如发现前端或外部调用方无法接受 breaking change：

1. 回滚成功响应拆包装相关 handler 与前端解析。
2. 保留错误分类和 not found 映射可以独立回滚。
3. 配置项 `web.error.debug` 可保留为无害配置，或随错误结构回滚删除。

## Assumptions

1. 本次允许 API breaking change，因为用户明确要求响应结构规范化。
2. `error` 字段默认空字符串仍满足结构要求，且避免默认泄露调试信息。
3. WebSocket `type` 字段是 protocol 必需字段，不视为 `code/data` 类 API 包装。
4. 前端需要同步最小改动以适配新成功响应和错误响应。

## Risks

1. 成功响应拆包装会影响所有依赖旧 `{workspaces}` / `{sessions}` 的调用方。
2. `404` 替代部分 `400` 会影响调用方错误分支。
3. `DebugErrors` 开启后会暴露本地路径或底层错误，需要默认关闭。
4. WebSocket 错误 message 从 `err.Error()` 改为友好说明，可能减少前端直接显示的细节；调试细节迁移到 `error` 字段。

## User review notes

- 用户确认 Spec 并要求进入 Plan。
