# API 响应与错误结构规范化规格
最后修改时间: 2026-06-19 23:11:07

Review status: Accepted

## Requirement basis

基于 `docs/requirement/20260619-api-response-error-shape.md`，Review status: Accepted。

核心要求：

1. 成功响应直接返回接口定义的返回类型，不增加统一 `code` / `data` 包装。
2. HTTP 错误响应统一为：
   ```json
   {"code":"...","message":"...","error":"..."}
   ```
3. `message` 为用户友好说明。
4. `error` 为开发阶段调试信息或堆栈，由已有配置系统控制是否输出。
5. 业务 not found 返回 HTTP `404`，`code` 为 `not_found`。
6. WebSocket 能沿用以上规则就沿用；不能完全沿用时需要说明并给出兼容结构。

## Current implementation summary

### HTTP server

现有 HTTP error helper 位于 `internal/webserver/server.go`：

- `writeError`：将 `apperrors.Usage` / `apperrors.Config` 映射到 `400`，其他映射到 `500`。
- `errorBody`：当前返回嵌套结构 `{ "error": { "code", "message" } }`。
- `decodeJSONRequest` / `methodNotAllowed` / origin guard 也复用 `errorBody`。
- 多个 `http.NotFound(w, r)` 当前返回标准库纯文本 404，需要改为 JSON。

### App errors

现有错误类型位于 `internal/errors/errors.go`：

- `KindUsage`
- `KindConfig`
- `KindInternal`
- `KindRuntime`

目前缺少 `NotFound` 等更细粒度错误类型。

### Config system

配置系统位于 `internal/config/config.go`，已有：

- `Config`
- `WebConfig`
- defaults: `internal/config/termbridge.default.yaml`
- unknown key allowlist: `rejectUnknownKeys`
- app wiring: `internal/app/app.go`
- server config: `internal/webserver/server.go` 的 `webserver.Config`

本规格使用现有配置系统新增 web API 错误调试配置，不通过 build mode 硬编码。

### WebSocket protocol

WebSocket 升级后使用 terminal protocol：

```json
{"type":"error","code":"...","message":"..."}
```

`type` 字段用于区分消息类型，不能直接去掉。建议在保持 `type: "error"` 的前提下增加 `error` 字段。

## Design decisions

### 1. HTTP error response shape

统一顶层结构：

```json
{
  "code": "not_found",
  "message": "session not found",
  "error": "stack or debug detail when enabled"
}
```

字段语义：

- `code`: 机器可读错误码，稳定、小写、snake_case。
- `message`: 用户友好说明，默认不包含堆栈。
- `error`: 调试信息。仅当配置启用时输出底层错误链或堆栈；未启用时为空字符串。

不返回旧结构：

```json
{"error":{"code":"...","message":"..."}}
```

### 2. HTTP status 与 code 映射

建议映射表：

| 场景 | HTTP status | code |
|---|---:|---|
| route not found | 404 | `not_found` |
| session/workspace not found | 404 | `not_found` |
| invalid JSON / bad request body | 400 | `invalid_request` |
| validation / usage error | 400 | `usage_error` |
| config error | 400 | `config_error` |
| forbidden origin | 403 | `forbidden_origin` |
| method not allowed | 405 | `method_not_allowed` |
| runtime error | 500 | `runtime_error` |
| internal error | 500 | `internal_error` |

### 3. App error model extension

扩展 `internal/errors`：

- 新增 `KindNotFound Kind = "not_found"`
- 新增构造函数：
  ```go
  func NotFound(message string, err error) error
  ```

现有业务 “not found” 不再用 `Usage` 表示，改为 `NotFound`。

建议保留 `Usage(message string)` 现有签名，避免扩大变更范围。

### 4. Debug error 配置

新增配置项：

```yaml
web:
  error:
    debug: false
```

Go config 结构建议：

```go
 type WebErrorConfig struct {
     Debug bool
 }

 type WebConfig struct {
     AllowedOrigins []string
     CwdAllowlist []string
     EnvDenylist []string
     Error WebErrorConfig
 }
```

Wiring：

- `config.Config.Web.Error.Debug`
- 传入 `webserver.Config.DebugErrors bool`
- `webserver.Server` 使用该配置决定 `error` 字段是否输出调试信息。

Unknown key allowlist 增加：

```text
web.error.debug
```

默认值为 `false`，避免默认泄露本地路径、堆栈和内部实现。

### 5. `error` 字段内容

配置关闭时：

```json
{"code":"not_found","message":"session not found","error":""}
```

配置开启时：

- 对 app error：输出完整错误字符串或错误链，例如 `session not found: open ...: no such file or directory`。
- 对普通 error：输出 `err.Error()`。
- 如后续需要真正 stack trace，可以在错误创建点或统一 helper 中补充；本次不强制引入第三方 stack 库。

重要约束：

- `message` 不应使用 `apperrors.FormatUser(err)`，否则可能混入底层错误。
- `message` 应优先使用 app error 的 `Msg`。
- `error` 才承载底层错误链或调试信息。

如当前 `internal/errors` 未暴露 `Msg`，可新增 helper：

```go
func Message(err error) string
func Debug(err error) string
```

### 6. 成功响应结构

成功响应遵守“返回类型即响应体”。建议调整当前 wrapper：

| Endpoint | 当前 | 调整后 |
|---|---|---|
| `GET /api/workspaces` | `{ "workspaces": [...] }` | `[...]` |
| `GET /api/workspaces/tree` | `[...]` | 不变 |
| `PATCH /api/workspaces/order` | `{ "workspaces": [...] }` | `[...]` |
| `GET /api/workspaces/{id}/sessions` | `{ "sessions": [...] }` | `[...]` |
| `GET /api/sessions` | `[...]` | 不变 |
| `POST /api/sessions` | session create response struct | 不变 |
| `GET /api/sessions/{id}` | session summary struct | 不变 |
| `PATCH /api/sessions/{id}` | session summary struct | 不变 |
| `POST /api/sessions/{id}/close` | `{ "state": "closing" }` | 不变，因返回类型本身是 close response object |
| `DELETE ...` | empty body `204` | 不变 |

`GET /api/health` 返回 `{ "status": "ok" }`，属于该接口预期对象，不需要改。

### 7. NotFound handling

将 server 内所有 API handler 中的 `http.NotFound(w, r)` 替换为统一 helper，例如：

```go
func notFound(w http.ResponseWriter) {
    writeAPIError(w, http.StatusNotFound, "not_found", "not found", nil)
}
```

业务 not found：

- `findSessionView` 返回 `apperrors.NotFound("session not found", err)` 或无底层 err 的 not found。
- `FindWorkspaceById` 相关调用处将 `os.ErrNotExist` 映射为 `apperrors.NotFound("workspace not found", err)`。

### 8. WebSocket error strategy

#### Upgrade 前

Upgrade 前仍是 HTTP 请求，完全沿用 HTTP API 错误结构与 status code。

例如 attach session 不存在：

```http
HTTP/1.1 404 Not Found
Content-Type: application/json
```

```json
{"code":"not_found","message":"session not found","error":""}
```

#### Upgrade 后

Upgrade 后无法再发送 HTTP status code，且 terminal protocol 需要 `type` 字段进行消息分发。因此不直接使用纯 `{code,message,error}`，而采用兼容结构：

```json
{
  "type": "error",
  "code": "invalid_control",
  "message": "invalid terminal control message",
  "error": "debug detail when enabled"
}
```

说明：

- `type: "error"` 是 protocol envelope，不是 API response wrapper。
- `code` / `message` / `error` 语义与 HTTP API 保持一致。
- `error` 字段同样受 web error debug 配置控制。

需要在 `internal/terminalproto/protocol.go` 的 `ServerMessage` 增加：

```go
Error string `json:"error,omitempty"`
```

WebSocket 发送错误时通过 webserver 的统一 helper 构造，避免直接将 `err.Error()` 放入用户友好 `message`。

## Affected components

1. `internal/errors/errors.go`
   - 新增 `KindNotFound`、`NotFound` helper。
   - 新增用户消息与调试信息 helper。
2. `internal/config/config.go`
   - 增加 `WebErrorConfig`。
   - 更新 unknown key allowlist。
   - 如需要，增加配置校验。
3. `internal/config/termbridge.default.yaml`
   - 增加默认 `web.error.debug: false`。
4. `internal/app/app.go`
   - 将 config 中的 debug error 开关传入 webserver。
5. `internal/webserver/server.go`
   - 修改 `Config`、`writeError`、`errorBody`、not found helper、method/origin/decode JSON 错误。
   - 拆掉成功响应中无必要包装。
   - WebSocket protocol error 使用统一语义。
6. `internal/terminalproto/protocol.go`
   - `ServerMessage` 增加 `Error` 字段。
7. `internal/webterminal/registry.go`
   - 将业务 not found 从 `Usage` 或 `Runtime(os.ErrNotExist)` 语义调整为 `NotFound`。
8. Tests
   - `internal/webserver/server_test.go`
   - `internal/webterminal/registry_test.go` 如涉及 not found 行为。
   - `internal/config/config_test.go` 如已有配置测试需要扩展。
   - `internal/terminalproto` 相关测试如存在。

## Interfaces

### HTTP error body

```go
type APIErrorResponse struct {
    Code string `json:"code"`
    Message string `json:"message"`
    Error string `json:"error"`
}
```

### WebSocket error message

```go
type ServerMessage struct {
    Type string `json:"type"`
    Code string `json:"code,omitempty"`
    Message string `json:"message,omitempty"`
    Error string `json:"error,omitempty"`
}
```

### Webserver config

```go
type Config struct {
    Host string
    Port int
    Dev bool
    Open bool
    Logger *slog.Logger
    RequestBodyLimit int
    ResponseBodyLimit int
    AllowedOrigins []string
    DebugErrors bool
}
```

## Alternatives considered

### Alternative A: `error` 永远等于 `err.Error()`

优点：实现简单。

缺点：默认泄露内部路径和实现细节，不符合“开发阶段提供”的要求。

结论：不采用。

### Alternative B: WebSocket error 改为纯 `{code,message,error}`

优点：与 HTTP body 完全一致。

缺点：破坏 terminal protocol 基于 `type` 的消息分发，客户端无法统一识别消息类型。

结论：不采用；保留 `type: "error"` 作为 protocol envelope。

### Alternative C: 不扩展 `internal/errors`，在 webserver 用字符串判断 not found

优点：改动少。

缺点：错误分类脆弱，无法形成稳定 API 语义。

结论：不采用；新增 `NotFound` kind。

## Risks

1. 这是 API breaking change，前端如果依赖 `{ "workspaces": [...] }` 或 `{ "error": { ... } }` 需要同步调整。
2. `error` debug 开启后可能暴露本地路径、命令、内部错误链，应默认关闭。
3. 将 not found 从 `400` 改为 `404` 可能影响现有测试和前端错误分支。
4. WebSocket 升级后无法表达 HTTP status code，只能在 protocol message 内表达 code/message/error。

## Technical questions

暂无阻塞问题。实现时需要确认现有前端是否依赖被拆掉的成功响应包装，并在 Plan 阶段列出对应文件。

## User review notes

- 用户确认 Requirement 并要求进入 Spec。
