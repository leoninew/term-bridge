# API 请求日志需求
最后修改时间: 2026-06-18 15:59:31

Review status: Accepted

## Background

当前 TermBridge-go 已有本地 Web API 原型，`internal/webserver/server.go` 使用 `net/http` 的 `http.ServeMux` 暴露 `/api/health`、`/api/workspaces`、`/api/sessions`、`/api/sessions/{id}`、`/api/sessions/{id}/history` 和 `/api/sessions/{id}/ws` 等接口。应用启动时已创建 `internal/logging` 的 `slog` logger，并在 `runWeb` 中传入 `webterminal.Registry`，但 `webserver.Server` 本身尚未接收 logger，也没有统一记录 API 请求完成情况。

用户要求参考 `D:\SourceCodes\mywork\pomelo-orbit\backend-go` 为后端 API 添加请求日志。参考项目中，请求日志由 HTTP middleware 统一处理：记录 method、path、uri、status、bytes、duration、request_id、remote_addr、user_agent，并在 JSON 请求/响应场景记录 body；路由入口统一挂载 middleware，业务 handler 不逐个打印日志。

TermBridge-go 当前 API 面向本地 Web terminal / prototype，日志能力应先服务本地诊断：用户或开发者能够从 `termbridge.log` 判断 API 是否被调用、JSON 请求/响应内容、响应状态、耗时和关键路径，而不引入新的 Web 框架或改变现有 API contract。

## Goal

为 TermBridge-go 后端 API 增加统一请求日志，目标包括：

1. Web API 每次请求完成后写入结构化请求日志到现有 `termbridge.log`。
2. 日志至少包含 method、path、uri、status、bytes、duration_ms、remote_addr、user_agent 等基础诊断字段。
3. 仅在请求或响应为 JSON 格式时记录 request body 和 response body；非 JSON body 不记录。
4. request body 和 response body 记录长度上限均暂定为 4096，并分别通过 `log.request_body_limit` 与 `log.response_body_limit` 纳入配置管理，默认值应可由配置来源承载。
5. 日志实现应集中在 HTTP middleware 或等价包装层，避免在每个 handler 内重复记录。
6. 请求日志应复用现有 `internal/logging` / `slog` 体系，不新增独立日志文件或重复 logger 初始化路径。
7. WebSocket upgrade 入口也应至少产生一次 HTTP 请求层面的日志，能看到 upgrade 请求路径和最终状态；不记录 WebSocket 帧内容。
8. 保持现有 API 行为不变，包括响应 status、header、body、WebSocket 连接能力和错误响应格式。
9. 为请求日志行为补充单元测试，覆盖基础字段、状态码、响应字节数、默认 status、JSON request/response body 记录与截断、非 JSON body 跳过、错误或 method not allowed 场景，以及 WebSocket 所需的 writer interface 兼容性。

## Non-goal

本次不包括：

1. 不引入 `chi`、`gin`、`echo` 等新 HTTP router，仅为请求日志而替换现有 `http.ServeMux`。
2. 不实现 request_id 生成或跨服务 tracing；如果后续需要 request_id，应另起需求设计。
3. 不记录 WebSocket frame 内容。
4. 不记录非 JSON body，包括 text/plain、multipart、二进制或不可安全缓存的 streaming body；如果后续需要完整审计日志，应另起需求设计脱敏、权限和存储策略。
5. 不改变 API 路由、返回 JSON schema、错误码或前端调用方式。
6. 不新增日志轮转、日志清理或日志输出目的地；本次只为 request/response body 长度上限增加必要配置项。
7. 不实现访问控制、审计日志或安全合规审计事件。
8. 不执行 `git add`、`git commit`、`git push` 或其他 git 写操作。

## User scenarios

### 场景一：开发者排查 API 是否被调用

作为开发者，我启动 `termbridge web` 后访问 Web 页面或直接请求 `/api/health`，我应能在 `termbridge.log` 中看到一条请求完成日志，包含请求方法、路径、状态码、耗时和 JSON 响应 body。

### 场景二：开发者排查前端请求失败

作为开发者，如果前端请求 `/api/sessions` 返回 400、404、405 或 500，我应能在日志中看到对应 status、uri、响应字节数、user agent、JSON request body 和 JSON response body，用于判断请求路径、方法、参数或服务端处理是否异常。

### 场景三：WebSocket 连接诊断

作为开发者，当 Web terminal 尝试连接 `/api/sessions/{id}/ws` 时，我应能在请求日志中看到该 upgrade 请求发生过，以及 HTTP 层返回的状态，方便区分“前端没有发起连接”和“后端拒绝/失败”。日志不记录 WebSocket 帧内容。

## Acceptance

1. `termbridge web` 使用的所有 `/api/*` HTTP handler 都经过统一请求日志包装。
2. 每条请求完成日志包含：
   - `method`
   - `path`
   - `uri`
   - `status`
   - `bytes`
   - `duration_ms`
   - `remote_addr`
   - `user_agent`
3. JSON 请求 body 应写入 `request_body` 字段；为了不破坏 handler 读取，请求 body 被日志层读取后必须恢复给后续 handler。
4. JSON 响应 body 应写入 `response_body` 字段；日志捕获不应改变客户端实际收到的响应 body。
5. 非 JSON 请求或响应不应写入 `request_body` / `response_body` 字段。
6. JSON body 判断应基于 `Content-Type`，覆盖 `application/json` 和 `+json` 后缀类型；缺失或非 JSON `Content-Type` 默认不记录 body。
7. request body 和 response body 长度上限默认均暂定为 4096，并应分别通过 `log.request_body_limit` 与 `log.response_body_limit` 纳入配置管理；超出对应上限时截断并保留可识别的截断标记，避免无界内存增长和超大日志。
8. 不记录 WebSocket frame、二进制、multipart 或无法安全缓存的 streaming body。
9. 当 handler 没有显式调用 `WriteHeader` 但写入响应体时，日志 status 应为 `200`。
10. 当 handler 只调用 `WriteHeader` 或无响应体时，日志仍能记录正确 status，bytes 为实际写入字节数或 `0`。
11. 请求日志 wrapper 不破坏 `http.Flusher`、`http.Hijacker`、`http.Pusher` 等可选接口；至少 WebSocket upgrade 所依赖的接口行为应保持可用。
12. 日志记录发生在请求处理完成后，duration_ms 反映 handler 执行耗时。
13. 单元测试覆盖请求日志字段、状态码/字节数、默认状态、JSON request body 恢复、JSON response body 透传、非 JSON body 跳过、body 截断和配置上限、method not allowed 或错误响应路径，以及 WebSocket/可选接口兼容性中与实现相关的关键行为。
14. 现有测试应继续通过；如果发现既有测试因请求日志引入而失败，应优先调整实现保持行为兼容，而不是放宽测试断言。

## Open questions

1. WebSocket upgrade 成功时，`net/http` wrapper 在 hijack 后是否能稳定记录最终状态和 bytes，需要在实现阶段通过测试或最小验证确认；如果状态不可精确捕获，应至少不破坏连接并记录可获得的 HTTP 层信息。
2. body 长度上限已暂定为 4096，配置项命名确定为 `log.request_body_limit` 与 `log.response_body_limit`，非法配置值均采用快速失败；实现阶段仍需确认默认配置文件位置和配置结构映射细节。

## Decisions

1. 采用标准模式 / standard：这是一个普通后端功能变更，涉及需求、计划、实现和验证；不需要 strict 的单独 Spec 阶段。
2. 参考 pomelo-orbit 的 middleware 思路，并采纳用户反馈记录 request body 和 response body；TermBridge 仅记录 JSON 格式请求或响应。
3. request/response body 长度上限均暂定为 4096，并纳入配置管理；配置项分别使用 `log.request_body_limit` 与 `log.response_body_limit`，非法配置值均快速失败。
4. 复用现有 `slog` logger 和 `termbridge.log` 输出，不新增日志基础设施。
5. 请求日志应作为 `webserver` 层能力接入，而不是由 `webterminal.Registry` 或业务 handler 负责。
6. 暂不增加 `request_id`，避免引入 tracing 语义和 header contract；后续如需 request_id 单独设计。
7. 暂不跳过 `/api/health`，因为本地诊断阶段 health 请求也有排查价值。

## Risk

1. 如果 response writer 包装不完整，可能破坏 WebSocket upgrade 或 streaming/flush 行为；实现必须保留相关可选接口并测试关键路径。
2. 记录 `uri` 可能包含查询参数；当前 API 查询参数不应包含敏感信息，但后续新增接口时需要注意日志暴露边界。
3. 记录 JSON request body 和 response body 会增加敏感信息暴露风险，尤其是命令、路径、history 或错误详情；当前按用户要求记录，但必须限制为 JSON、设置长度上限并纳入配置。
4. 请求日志会增加少量同步写日志和 body 缓存开销；本地产品阶段可接受，但不得无界读取/缓存请求体或响应体。
5. 配置项增加后需要保持默认配置、用户覆盖配置和测试配置一致，避免缺省值缺失导致 web server 启动失败；非法 `log.request_body_limit` 或 `log.response_body_limit` 必须快速失败并给出可理解的配置错误。
6. 如果 `webserver.Server` 构造函数签名需要接收 logger，相关测试和 `app.runWeb` 需要同步调整，避免 nil logger 导致 panic。

## User review notes

- Draft: 根据用户“参考 pomelo-orbit backend-go，为后端 API 添加请求日志”的要求创建需求草稿。当前仅进入 Requirement / 需求阶段，尚未编写 plan 或产品代码。
- Draft update: 用户明确要求记录 request body、response body，并采纳其他建议；已将 body logging 纳入目标、验收、决策和风险，同时保留长度上限、内容类型边界和不记录 WebSocket frame 的约束。
- Draft update: 用户明确要求只记录 JSON 格式请求或响应，body 长度上限暂定 4096，并纳入配置管理；已更新目标、非目标、验收、决策、风险和待定事项。
- Accepted: 用户要求开始 Plan / 计划，并补充决定配置项使用 `log.request_body_limit`，非法配置快速失败；Requirement 视为已接受。
- Accepted update: 用户审查 Plan 后要求拆分为 `log.request_body_limit` 与 `log.response_body_limit`，并采纳 nil logger 使用 discard logger、WebSocket upgrade 最低要求等建议；Requirement 同步修订。
