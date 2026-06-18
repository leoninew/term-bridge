# API 请求日志验证
最后修改时间: 2026-06-18 16:19:12

Review status: Accepted

## Verification basis

本验证基于：

- `docs/requirement/20260618-api-request-logging.md`（Accepted）
- `docs/plan/20260618-api-request-logging.md`（Accepted）
- 当前工作区实际 diff

当前流程：标准模式 / standard，验证 / Verification。

## Requirement alignment

| Requirement | Verification |
| --- | --- |
| Web API 每次请求完成后写入结构化请求日志到现有 `termbridge.log` | 已实现。`internal/webserver/server.go` 将 `http.ServeMux` 包装为 `logRequests(...)`；正式 app 路径通过 `internal/app/app.go` 传入现有 `slog` logger。 |
| 日志包含 method/path/uri/status/bytes/duration_ms/remote_addr/user_agent | 已实现并由 `internal/webserver/request_logging_test.go` 覆盖基础字段断言。 |
| 仅记录 JSON request/response body | 已实现。`isJSONContentType` 仅接受 `application/json` 与 `+json` 后缀；非 JSON request/response body 测试覆盖。 |
| request/response body limit 分别通过 `log.request_body_limit` / `log.response_body_limit` 管理，默认 4096，非法配置快速失败 | 已实现。两个默认配置文件均新增默认值；`internal/config/config.go` 读取、allowlist 和校验两个字段；配置测试覆盖默认值、覆盖值和非法值。 |
| 集中式 middleware/wrapper，避免 handler 内重复记录 | 已实现。新增 `internal/webserver/request_logging.go`，业务 handler 未逐个添加日志。 |
| 复用现有 `internal/logging` / `slog` 体系 | 已实现。正式 `runWeb` 传入现有 `logger.Slog`；nil logger 仅在非正式构造中使用 discard logger。 |
| WebSocket upgrade 至少记录 HTTP 层请求，不记录 frame | 部分验证。wrapper 实现 `http.Hijacker` 并在 hijack 时标记 `101`，不捕获 WebSocket frame；当前未做真实 WebSocket smoke，只通过 optional interface 测试覆盖不破坏接口暴露。 |
| 保持现有 API 行为不变 | 已由 `go test ./cmd/... ./internal/...` 与 `just check` 覆盖现有测试；未发现回归。 |
| 补充单元测试 | 已补充配置、server logging 和 request logging 单元测试。 |

## Spec alignment

不适用。标准模式 / standard 未创建独立 Spec 文档，本验证按 Requirement 与 Plan 核对。

## Plan alignment

| Plan item | Verification |
| --- | --- |
| Step 1: 扩展配置结构与默认配置 | 已完成。修改 `internal/config/config.go`、`internal/config/config_test.go`、两个 default YAML。 |
| Step 2: 为 webserver 注入 logger 和 body limit | 已完成。`webserver.Config` 新增 `Logger`、`RequestBodyLimit`、`ResponseBodyLimit`；`app.runWeb` 传入配置值。 |
| Step 3: 实现请求日志 wrapper | 已完成。新增 `internal/webserver/request_logging.go`，记录基础字段、JSON body、截断和错误字段。 |
| Step 4: 保持 optional ResponseWriter interface 兼容 | 已实现并单测覆盖接口暴露；未执行真实 WebSocket smoke。 |
| Step 5: 更新/补充测试 | 已完成。新增 `request_logging_test.go`，更新 `server_test.go` 与 `config_test.go`。 |
| Step 6: 保持格式化与行为兼容 | 已执行 `gofmt`、Go tests、`go vet` 和 `just check`。 |

## Actual diff summary

本次实际改动：

- 新增过程文档：
  - `docs/requirement/20260618-api-request-logging.md`
  - `docs/plan/20260618-api-request-logging.md`
  - `docs/verification/20260618-api-request-logging.md`
- 配置：
  - `configs/termbridge.default.yaml`
  - `internal/config/termbridge.default.yaml`
  - `internal/config/config.go`
  - `internal/config/config_test.go`
- Web server / app 接入：
  - `internal/app/app.go`
  - `internal/webserver/server.go`
  - `internal/webserver/server_test.go`
- 请求日志实现与测试：
  - `internal/webserver/request_logging.go`
  - `internal/webserver/request_logging_test.go`

主要行为变化：

1. `termbridge web` 的所有现有 `/api/*` handler 现在经过请求日志 wrapper。
2. 日志记录到现有 logger，对应现有 `termbridge.log` 输出路径。
3. JSON request body 记录到 `request_body`，使用 `log.request_body_limit` 截断。
4. JSON response body 记录到 `response_body`，使用 `log.response_body_limit` 截断。
5. 非 JSON body 不记录。
6. 两个 body limit 默认均为 `4096`，非法值快速失败。

## Expected vs actual changed files

| File | Expected | Actual | Notes |
| --- | --- | --- | --- |
| `configs/termbridge.default.yaml` | 是 | 是 | 新增两个 log body limit 默认值。 |
| `internal/config/termbridge.default.yaml` | 是 | 是 | 与 public default 保持一致。 |
| `internal/config/config.go` | 是 | 是 | 新增配置字段、读取、allowlist、校验。 |
| `internal/config/config_test.go` | 是 | 是 | 覆盖默认值、覆盖值、非法值。 |
| `internal/app/app.go` | 是 | 是 | 将 logger 与两个 limit 传入 webserver。 |
| `internal/app/app_test.go` | 计划中可能修改 | 否 | 现有 app 测试未必需要改动；通过现有 `RunWeb` 测试和编译覆盖配置传递。 |
| `internal/webserver/server.go` | 是 | 是 | 接入请求日志 wrapper，扩展 config。 |
| `internal/webserver/server_test.go` | 是 | 是 | 覆盖 server 层日志和 configured response limit。 |
| `internal/webserver/request_logging.go` | 是 | 是 | 新增集中实现。 |
| `internal/webserver/request_logging_test.go` | 是 | 是 | 新增单元测试。 |
| `docs/requirement/20260618-api-request-logging.md` | 是 | 是 | SpecFlow Requirement。 |
| `docs/plan/20260618-api-request-logging.md` | 是 | 是 | SpecFlow Plan。 |
| `docs/verification/20260618-api-request-logging.md` | 是 | 是 | 当前验证文档。 |

未发现超出计划范围的产品代码改动。

## Acceptance criteria checklist

- [x] `termbridge web` 使用的所有现有 `/api/*` HTTP handler 都经过统一请求日志包装。
- [x] 请求日志包含 `method`、`path`、`uri`、`status`、`bytes`、`duration_ms`、`remote_addr`、`user_agent`。
- [x] JSON request body 写入 `request_body` 字段。
- [x] 请求 body 被日志层读取后仍恢复给后续 handler。
- [x] JSON response body 写入 `response_body` 字段。
- [x] 日志捕获不改变客户端实际收到的 response body。
- [x] 非 JSON 请求或响应不写入 `request_body` / `response_body`。
- [x] JSON body 判断覆盖 `application/json` 与 `+json` 后缀类型。
- [x] request body 与 response body limit 默认均为 `4096`，分别由 `log.request_body_limit` 与 `log.response_body_limit` 配置。
- [x] 超出对应 limit 时截断并追加 `...`。
- [x] 不记录 WebSocket frame、二进制、multipart 或 streaming body。
- [x] 未显式 `WriteHeader` 但写 body 时日志 status 为 `200`。
- [x] 只调用 `WriteHeader` 或无响应体时记录正确 status 和 bytes。
- [x] wrapper 暴露 `http.Flusher`、`http.Hijacker`、`http.Pusher`、`Unwrap()`。
- [x] 日志记录发生在请求处理完成后，duration_ms 基于 handler 执行时间。
- [x] 单元测试覆盖核心字段、状态码、body 恢复/透传、非 JSON 跳过、截断和配置上限。
- [x] 现有 Go tests 与项目 `just check` 通过。

## Test results

### gofmt + focused Go tests

命令：

```text
gofmt -w "internal/config/config.go" "internal/config/config_test.go" "internal/app/app.go" "internal/webserver/server.go" "internal/webserver/server_test.go" "internal/webserver/request_logging.go" "internal/webserver/request_logging_test.go" && go test ./internal/config ./internal/webserver ./internal/app
```

结果：通过。

```text
ok  	termbridge-go/internal/config	1.174s
ok  	termbridge-go/internal/webserver	1.063s
ok  	termbridge-go/internal/app	1.453s
```

### 全量 Go package tests

命令：

```text
go test ./cmd/... ./internal/...
```

结果：通过。

```text
?   	termbridge-go/cmd/termbridge	[no test files]
ok  	termbridge-go/internal/app	(cached)
ok  	termbridge-go/internal/cli	1.117s
ok  	termbridge-go/internal/config	(cached)
ok  	termbridge-go/internal/errors	(cached)
ok  	termbridge-go/internal/history	(cached)
ok  	termbridge-go/internal/identity	(cached)
ok  	termbridge-go/internal/logging	(cached)
ok  	termbridge-go/internal/process	(cached)
?   	termbridge-go/internal/pty	[no test files]
ok  	termbridge-go/internal/pty/gopty	(cached)
ok  	termbridge-go/internal/runner	(cached)
ok  	termbridge-go/internal/session	(cached)
ok  	termbridge-go/internal/state	(cached)
ok  	termbridge-go/internal/terminalproto	(cached)
?   	termbridge-go/internal/version	[no test files]
ok  	termbridge-go/internal/webserver	(cached)
ok  	termbridge-go/internal/webterminal	(cached)
ok  	termbridge-go/internal/workspace	(cached)
```

### go vet

命令：

```text
go vet ./cmd/... ./internal/...
```

结果：通过，无输出。

### just check

命令：

```text
just check
```

结果：通过。

关键输出：

```text
go fmt ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./cmd/... ./internal/...; else echo "golangci-lint not found; skipped"; fi
go test ./cmd/... ./internal/...
...
ok  	termbridge-go/internal/webserver	(cached)
ok  	termbridge-go/internal/webterminal	(cached)
ok  	termbridge-go/internal/workspace	(cached)
cd web && yarn format
...
Done in 0.41s.
cd web && yarn typecheck
...
Done in 1.15s.
cd web && yarn lint
...
Done in 1.17s.
```

说明：`just check` 中的 `golangci-lint` 分支未输出 `golangci-lint not found; skipped`，也未输出 lint 错误；整体命令退出成功。

## Missed or expanded scope

### Missed scope

- 未执行真实 WebSocket client smoke。当前通过 wrapper interface 暴露测试和现有 webserver/webterminal tests 间接覆盖“不破坏可选接口”的基础要求。真实 upgrade status 是否记录为 `101` 建议后续如有需要用端到端 smoke 进一步确认。

### Expanded scope

- 无产品功能扩展超出 Requirement / Plan。
- `just check` 运行了 web format/typecheck/lint；未产生 web 文件修改。

## Risks

1. JSON body logging 本身会记录请求/响应内容，仍存在敏感信息暴露风险；当前通过 JSON-only 与可配置长度上限控制范围。
2. body 截断按字节上限截断，并通过 `bytes.ToValidUTF8` 避免非法 UTF-8；中文等多字节字符跨边界时可能少量丢弃边界字符，但不会产生乱码。
3. 真实 WebSocket upgrade status 的精确日志值未做端到端 smoke，保留为后续可增强验证点。

## Incomplete items

无阻塞交付的 incomplete items。

可选后续增强：

1. 增加真实 WebSocket client smoke，验证 upgrade 成功路径的日志 status 是否为 `101`。
2. 如将来日志用于审计而非本地诊断，需另起需求设计脱敏、采样、权限和存储保留策略。
3. 如需要 request tracing，可另起需求增加 request_id。

## Conclusion

本次 API 请求日志变更已按 Requirement 与 Plan 实现并验证通过。配置项 `log.request_body_limit` / `log.response_body_limit` 已纳入默认配置和快速失败校验；Web API 请求日志已集中接入 webserver wrapper，并覆盖基础字段、JSON body 捕获、非 JSON 跳过、截断、状态码/字节数和 optional ResponseWriter interface。`go test ./cmd/... ./internal/...`、`go vet ./cmd/... ./internal/...` 与 `just check` 均通过。
