# API 请求日志计划
最后修改时间: 2026-06-18 16:05:41

Review status: Accepted

## Plan basis

本计划基于：

- `docs/requirement/20260618-api-request-logging.md`（Accepted）
- 当前代码结构：
  - `internal/config/config.go` 负责默认配置、用户配置、环境变量、未知 key 拒绝和配置校验。
  - `internal/config/termbridge.default.yaml` 与 `configs/termbridge.default.yaml` 必须保持一致。
  - `internal/logging/logging.go` 已提供 `slog` logger 和 `termbridge.log` 输出。
  - `internal/app/app.go` 在 `Run` 中创建 logger，并在 `runWeb` 中构造 `webserver.Server`。
  - `internal/webserver/server.go` 使用 `http.ServeMux` 暴露 `/api/*` 和 WebSocket endpoint。
  - `internal/webserver/server_test.go` 当前覆盖 health 和 sessions 基础路由。

用户补充决策：

- 只记录 JSON 格式请求或响应 body。
- body 记录长度上限暂定 `4096`。
- 配置项使用 `log.request_body_limit` 和 `log.response_body_limit`。
- 两个 limit 的非法配置均快速失败。
- 其他先前建议采纳：不增加 request_id，不跳过 `/api/health`，不记录 WebSocket frame。

## Implementation strategy

按“配置先行、middleware 独立、server 接入、测试覆盖”的顺序实施，避免把请求日志散落到业务 handler 中。

关键设计：

1. 配置层新增 `log.request_body_limit` 与 `log.response_body_limit`，默认均为 `4096`，并在 `Load` 阶段校验为正数；任一非法值都返回 config error，阻止应用继续启动。
2. Web server 层新增请求日志 wrapper，复用 `slog.Logger`，在请求完成后记录统一字段。
3. request/response body 只在 `Content-Type` 为 `application/json` 或 `*+json` 时记录。
4. request body 使用 `log.request_body_limit` 截断，response body 使用 `log.response_body_limit` 截断。
5. WebSocket upgrade 只记录 HTTP upgrade 请求本身；upgrade 后的 WebSocket frame 不进入请求日志。

## Implementation steps

### Step 1: 扩展配置结构与默认配置

修改：

```text
internal/config/config.go
internal/config/config_test.go
internal/config/termbridge.default.yaml
configs/termbridge.default.yaml
```

计划：

1. 在 `config.Config` 中新增字段，例如：

   ```go
   LogRequestBodyLimit  int
   LogResponseBodyLimit int
   ```

2. 在 `rejectUnknownKeys` allowlist 中加入：

   ```text
   log.request_body_limit
   log.response_body_limit
   ```

3. 从 viper 读取：

   ```text
   log.request_body_limit
   log.response_body_limit
   ```

4. 新增校验函数或合并到 log 校验：

   - `log.request_body_limit <= 0` 时返回 `apperrors.Config(...)`。
   - `log.response_body_limit <= 0` 时返回 `apperrors.Config(...)`。
   - 错误信息应能指向对应配置 key，满足“非法配置快速失败”。

5. 在两个默认配置文件中加入：

   ```yaml
   log:
     request_body_limit: 4096
     response_body_limit: 4096
   ```

6. 更新配置测试：

   - `TestLoadDefaults` 断言两个默认值均为 `4096`。
   - local config override 能分别覆盖 request/response limit。
   - 新增非法值测试，例如任一配置为 `0` 或 `-1` 应返回 config error。
   - 既有 public default 与 embedded default 一致性测试继续覆盖两个默认配置文件同步。

### Step 2: 为 webserver 注入 logger 和 body limit

修改：

```text
internal/webserver/server.go
internal/webserver/server_test.go
internal/app/app.go
internal/app/app_test.go
```

计划：

1. 扩展 `webserver.Config`，加入：

   ```go
   Logger *slog.Logger
   RequestBodyLimit int
   ResponseBodyLimit int
   ```

   或等价命名。`RequestBodyLimit` 使用 `cfg.LogRequestBodyLimit` 传入，`ResponseBodyLimit` 使用 `cfg.LogResponseBodyLimit` 传入。

2. `app.runWeb` 构造 `webserver.New(...)` 时传入：

   - 当前 `logger.Slog` 或等价 `*slog.Logger`。
   - `cfg.LogRequestBodyLimit`。
   - `cfg.LogResponseBodyLimit`。

3. `webserver.New` 中保留现有 `http.ServeMux` 路由注册方式，不引入新 router。

4. 将 `http.Server.Handler` 设置为请求日志 wrapper 包装后的 mux，例如：

   ```go
   handler := logRequests(config.Logger, config.RequestBodyLimit, config.ResponseBodyLimit, mux)
   s.server = &http.Server{Handler: handler}
   ```

5. 测试中可传入 buffer-backed `slog.Logger` 用于断言日志；未传 logger 的场景需要有明确策略：

   - 推荐在 `normalizeConfig` 中为空 logger 使用 discard logger，避免测试或非正式构造 panic。
   - 正式 `app.runWeb` 始终传入真实 logger。

### Step 3: 实现请求日志 wrapper

新增或修改：

```text
internal/webserver/request_logging.go
internal/webserver/request_logging_test.go
```

计划：

1. 新增集中式 wrapper，例如：

   ```go
   func logRequests(logger *slog.Logger, requestBodyLimit int, responseBodyLimit int, next http.Handler) http.Handler
   ```

2. 请求进入时记录开始时间。

3. 如果 request `Content-Type` 是 JSON：

   - 从 `r.Body` 读取最多 `requestBodyLimit + 1` 字节用于日志截断判断。
   - 不读取完整超大 body 到内存。
   - 将已读取前缀和原始剩余 body 重新组合回 `r.Body`，确保后续 handler 仍能读取完整请求 body。
   - 记录字段名：`request_body`。
   - 超限时追加可识别截断标记，例如 `...`。

4. 使用 response writer wrapper 捕获：

   - status code。
   - 实际写入客户端的 bytes。
   - JSON response body 前 `responseBodyLimit + 1` 字节。

5. response body 仅当 response `Content-Type` 是 JSON 时记录：

   - 支持 `application/json`。
   - 支持 `application/problem+json` 等 `+json` 后缀类型。
   - 缺失或非 JSON 时不记录 `response_body`。

6. 请求完成后以 `logger.Info("request completed", ...)` 记录字段：

   ```text
   method
   path
   uri
   status
   bytes
   duration_ms
   remote_addr
   user_agent
   request_body    // 仅 JSON 且非空时
   response_body   // 仅 JSON 且非空时
   ```

7. status 规则：

   - 未显式 `WriteHeader` 但调用 `Write`：记录 `200`。
   - 只调用 `WriteHeader`：记录该 status。
   - handler 没有写任何内容：记录 `200`，与 `net/http` 默认语义一致。

### Step 4: 保持 optional ResponseWriter interface 兼容

修改：

```text
internal/webserver/request_logging.go
internal/webserver/request_logging_test.go
```

计划：

1. response writer wrapper 实现或转发：

   - `http.Flusher`
   - `http.Hijacker`
   - `http.Pusher`
   - `Unwrap() http.ResponseWriter`

2. 对不支持的底层 writer 返回标准错误或保持 `net/http` 兼容行为。

3. WebSocket 相关处理：

   - `coder/websocket.Accept` 可能依赖 hijack/response controller 能力。
   - 如果 `Hijack()` 被调用且 status 尚未设置，可将 status 标记为 `101 Switching Protocols`，用于日志反映 upgrade 成功路径。
   - 不记录 hijack 后 WebSocket frame bytes；`bytes` 字段只反映 wrapper 可观察到的 HTTP response write bytes。

4. 测试应至少覆盖 wrapper 不吞掉/破坏可选接口；如可行，增加一个使用 `httptest.Server` + websocket client 的最小 upgrade smoke。

### Step 5: 更新现有 Web API 测试并补充请求日志测试

修改：

```text
internal/webserver/server_test.go
internal/webserver/request_logging_test.go
internal/app/app_test.go
```

计划覆盖：

1. 基础字段：method/path/uri/status/bytes/duration_ms/remote_addr/user_agent。
2. 默认 status：handler 只写 body 时为 `200`。
3. `WriteHeader` + no body：status 正确、bytes 为 `0`。
4. method not allowed 或错误响应路径仍记录 JSON response body。
5. JSON request body：
   - 写入 `request_body`。
   - handler 仍能读取完整 body。
6. 非 JSON request body：不写入 `request_body`。
7. JSON response body：
   - 写入 `response_body`。
   - 客户端实际 response body 不变。
8. 非 JSON response body：不写入 `response_body`。
9. body limit：
   - 配置上限生效。
   - 超限截断并带标记。
   - 截断逻辑按字节还是 rune 需实现时统一；推荐按 rune 截断以避免 UTF-8 日志乱码，但读取上限仍需避免无界内存。
10. WebSocket/optional interface：不破坏 upgrade 所需能力。
11. `app.runWeb` 将配置中的 `LogRequestBodyLimit` 与 `LogResponseBodyLimit` 传入 webserver。

### Step 6: 保持格式化与现有行为兼容

修改范围应限制在：

```text
configs/termbridge.default.yaml
internal/config/config.go
internal/config/config_test.go
internal/config/termbridge.default.yaml
internal/app/app.go
internal/app/app_test.go
internal/webserver/server.go
internal/webserver/server_test.go
internal/webserver/request_logging.go
internal/webserver/request_logging_test.go
docs/requirement/20260618-api-request-logging.md
docs/plan/20260618-api-request-logging.md
```

如果实现阶段发现必须修改其他文件，需要先说明原因，确保不静默扩大范围。

## Verification plan

实施完成后优先运行：

```text
go test ./internal/config ./internal/webserver ./internal/app
```

随后运行项目级检查：

```text
go test ./cmd/... ./internal/...
```

如环境具备 `just`、Go 和 web 依赖，再运行：

```text
just check
```

说明：`just check` 会运行 Go fmt/vet/test 以及 web 的 format/typecheck/lint；如果本次后端变更无需触碰 web，但 web 依赖缺失或环境不满足，应在 Verification / 验证阶段如实记录跳过原因和已运行的替代命令。

可选手工 smoke（Verification 阶段视时间和环境执行）：

```text
just backend
```

然后请求：

```text
GET /api/health
POST /api/sessions with application/json
```

检查 `logs/termbridge.log` 是否出现请求日志和 JSON body 字段。

## Blockers

暂无实现前阻塞项。Requirement 中剩余 WebSocket status/bytes 精确性属于实现验证点，不阻塞进入实现。

## Assumptions

1. `log.request_body_limit` 只用于 request body 日志截断，`log.response_body_limit` 只用于 response body 日志截断；两者默认值均为 `4096`。
2. 非 JSON body 不记录，以 `Content-Type` 为准；缺失 `Content-Type` 默认不记录。
3. 超限截断标记使用简单后缀 `...` 即可满足可识别要求。
4. nil logger 在非正式构造场景下不应导致 panic；正式 app 路径始终传入真实 logger。
5. WebSocket upgrade 成功时如底层通过 hijack 完成，日志 status 可通过 wrapper 的 `Hijack()` 路径标记为 `101`；若库行为不同，以“不破坏连接并记录可获得 HTTP 层信息”为最低要求。

## Risks

1. request body 读取若实现不当，可能导致 handler 只能读到截断 body；必须通过“读取前缀 + 原始剩余 body 回放”或等价方式避免。
2. response writer wrapper 若未正确转发 optional interfaces，可能破坏 WebSocket upgrade。
3. body logging 会增加敏感信息暴露风险；本次按用户要求记录 JSON body，但必须严格限制 Content-Type 和长度上限。
4. 配置项新增后，embedded default 与 public default 不一致会导致现有一致性测试失败。
5. `log.request_body_limit` 与 `log.response_body_limit` 的非法值都需要在配置加载阶段快速失败，不能延迟到 web server 运行时才发现。
6. 如果响应 `Content-Type` 在写入后才设置，wrapper 可能无法捕获 body；现有 `writeJSON` 先设置 header 再写入，满足本次需求。

## Rollback

如请求日志实现导致行为异常，可回滚：

1. 移除 webserver handler wrapper 接入，恢复 `http.Server{Handler: mux}`。
2. 保留或移除 `log.request_body_limit` / `log.response_body_limit` 配置需视是否已有用户依赖；若尚未发布，可同步移除默认配置和 allowlist。
3. 删除新增 request logging 测试或调整为 pending 后续需求。
4. 不需要数据迁移；本次仅涉及配置、日志和 HTTP wrapper。

## User review notes

- Draft: 用户要求开始 Plan / 计划；Requirement 已标记为 Accepted，并补充 `log.request_body_limit` 与非法配置快速失败决策。
- Draft update: 用户审查后要求拆分为 `log.request_body_limit` 与 `log.response_body_limit`，并采纳 nil logger 使用 discard logger、WebSocket upgrade 最低要求等建议；Plan 已同步更新。
- Accepted: 用户明确要求开始 Implementation / 实现，Plan 视为已接受。
