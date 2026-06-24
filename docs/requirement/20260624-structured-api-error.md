# Structured API Error Requirement
最后修改时间: 2026-06-24 18:14:36

## Review status

Accepted

## Flow mode / Stage

标准模式 / standard；需求 / Requirement 已接受，当前进入计划 / Plan。

## Background

当前 gateway HTTP API 仍存在多种错误输出形态：

- `internal/transport/http/gatewayapi/server.go` 中大量路径直接使用 `http.Error(...)` 返回纯文本错误，例如 unauthorized、bad request、device offline、bad gateway、method not allowed。
- `methodNotAllowed()` 和 `decodeJSONRequest()` 只是集中了一部分错误入口，但仍返回 `text/plain` 风格 body。
- agent relay 失败时直接把 `route.request()` 或 JSON decode 的底层错误文本写回浏览器，错误语义和泄露边界不稳定。
- `internal/transport/http/gatewayapi/auth/auth.go` 的 auth middleware 也直接返回纯文本 `unauthorized`。
- 前端 `web/src/features/*/api.ts` 通过 `responseError()` 解析失败响应，但当前类型 `ApiErrorResponse` 偏向旧的 `{ code, message, error }` 结构，和本次希望参考的 best-practices error contract 不完全一致。

用户要求启动新的“结构化 API error”任务，采用标准模式 / standard，思路参考：

- `D:\SourceCodes\mywork\best-practices\docs\practices\error-handling.md`
- `D:\SourceCodes\mywork\best-practices\examples\project-structures\go-service`

参考文档的核心原则是：内部保留足够诊断信息，在 HTTP 边界统一转换为稳定、可理解、不会泄露实现细节的错误形态。参考 go-service 示例中的 HTTP error response 形态为：

```json
{
  "code": "validation.failed",
  "error": "Request validation failed.",
  "requestId": "req_123",
  "details": {
    "fields": [
      {
        "path": "name",
        "location": "query",
        "code": "required",
        "error": "Query parameter name is required."
      }
    ]
  }
}
```

本任务和历史文档 `docs/requirement/20260619-api-response-error-shape.md` 有主题重叠。按用户本次“新任务 / 开始结构化 API error 任务”的表达，本需求新建独立文档；用户已确认本次使用 best-practices 结构，即 `{ code, error, requestId, details? }`，替代历史 `{ code, message, error }` 作为本任务目标契约。

## Goal

1. 为 gateway HTTP API 建立统一的结构化错误响应契约，避免继续由各 handler 直接 `http.Error` 输出纯文本错误。
2. 在 HTTP 边界集中完成错误映射：handler / middleware / relay 边界把内部错误转换为稳定 JSON error response。
3. 默认错误 body 面向调用方稳定、可展示、可测试，不暴露数据库错误、源码路径、堆栈、连接串、token、完整底层异常链等内部细节。
4. 错误响应应包含机器可读 `code`、人可读安全摘要、可追踪 `requestId`。`details` 可以先不实现，但需要预留契约方向，避免后续补字段时再次破坏结构。
5. 成功响应保持现状：成功 body 仍直接返回接口预期 DTO，不额外包统一 envelope。
6. 前端引入 axios 作为 HTTP API client 基础能力：通过 request interceptor 为每个 HTTP API 请求生成或补齐 `requestId` / `X-Request-ID`，通过 response interceptor 统一归一化结构化错误；组件层不需要理解后端底层错误文本。
7. 后端收到前端传入的 request id 时必须沿用该值，并输出到响应和下游；如果请求没有 request id，后端必须自行生成，并同样输出到响应和下游。
8. 保持 WebSocket 升级后的 terminal protocol 不做无谓重写；仅覆盖升级前 HTTP 失败和普通 HTTP API 失败。升级后的 terminal control error 是否调整，留到 Plan 阶段评估。
9. 前后端请求/响应 DTO 命名统一：所有前后端请求结构使用 `XxxxReq` 形式，所有前后端响应结构使用 `XxxxResp` 形式；嵌套结构按具体表达需求决定，不强制套用该后缀。
10. 代码标识符中的缩写 `ID` 不使用全大写形式：所有 `AbcdID` 应写为 `AbcdId`，所有 `abcdID` 应写为 `abcdId`；JSON 字段名和外部 wire format 不因此改动。
11. 保持现有 auth、device、workspace、session、history、terminal attach 等业务能力不变；本任务只调整错误契约、request id 传递、DTO 命名和边界转换。

## Non-goal

1. 不实现完整 `details.fields` 细粒度校验错误；用户明确说明 details 可以先不实现。
2. 不引入统一成功响应 envelope，例如 `{ data, code }`。
3. 不改变业务 API 路由、请求参数语义、成功响应字段语义或 terminal tunnel protocol 行为；但允许为符合 `XxxxReq` / `XxxxResp` 命名规则重命名前后端 DTO 类型。
4. 不在普通响应中返回 debug、exceptionType、stackTrace、traceback、源码路径、SQL、连接串、token 等敏感或不稳定字段。
5. 不把所有内部错误类型重构为庞大异常体系；优先在 HTTP 边界建立可落地的最小稳定分类。
6. 不修改历史 SpecFlow 文档，除非后续用户明确要求归并或替代历史需求。
7. 不执行 git add / commit / push / checkout / stash / reset / rebase / merge 等 git 写操作。

## User scenarios

1. 作为前端调用方，请求失败时可以稳定读取错误码和安全错误说明，而不是解析纯文本 `bad request`、`device offline` 或代理返回的底层错误字符串。
2. 作为 UI 用户，我看到的是可理解的错误摘要，不会看到堆栈、源码路径、连接串或 agent 内部错误链。
3. 作为 API 调试者，我可以通过 `requestId` 把浏览器 Network 面板中的失败和服务端日志关联起来。
4. 作为后端维护者，我新增 handler 时复用统一 error writer / mapper，而不是继续分散调用 `http.Error`。
5. 作为测试维护者，我可以用结构化 JSON 断言错误响应的 `code`、HTTP status 和 request id，而不是依赖纯文本 body。
6. 作为前端维护者，我希望所有 HTTP API 请求统一经过 axios client，request id 生成、header 注入和结构化错误归一化都在拦截器中处理，组件和 feature API 不再各自拼接错误。
7. 作为前后端维护者，我希望请求/响应 DTO 命名能直接表达方向：请求结构统一以 `Req` 结尾，响应结构统一以 `Resp` 结尾，减少 `Request` / `Response` / `Summary` 等混杂命名在 API contract 层的歧义。

## Acceptance

- [ ] Gateway HTTP API 的错误响应默认为 JSON，至少包含：
  - `code`: 机器可读错误码。
  - `error`: 安全、人可读错误摘要。
  - `requestId`: 请求追踪 ID。
- [ ] `details` 字段本轮可以不实现；如果保留类型或 helper，也必须是可选字段，不能要求调用点立即构造 details。
- [ ] HTTP status 不放入 JSON body；status 仍由 HTTP status line 表达。
- [ ] 成功响应保持自然 DTO，不增加统一包装层。
- [ ] 前后端 API contract 层请求 DTO 使用 `XxxxReq` 命名，响应 DTO 使用 `XxxxResp` 命名；嵌套结构按具体表达需求决定，不强制要求。
- [ ] 代码标识符中的 ID 缩写统一为 Id：`AbcdID` → `AbcdId`，`abcdID` → `abcdId`；JSON 字段名和外部 wire format 保持不变。
- [ ] 所有普通 HTTP API 错误入口进入统一转换路径，至少包括：
  - JSON decode / invalid body。
  - method not allowed。
  - unauthorized。
  - not found。
  - device offline。
  - agent relay / bad gateway。
  - history JSON decode failure。
  - terminal ws 升级前的 conflict / offline / bad method。
- [ ] 错误码命名稳定且语义化；建议 Plan 阶段给出映射表，例如 `bad_request`、`unauthorized`、`not_found`、`method_not_allowed`、`device_offline`、`upstream_error`、`conflict`、`internal_error`。
- [ ] 内部错误不直接透传到 API body；relay 或 unexpected failure 使用安全摘要，并通过日志和 request id 保留诊断路径。
- [ ] 前端引入 axios，并通过 request interceptor 为每个 HTTP API 请求生成或补齐 request id，写入 `X-Request-ID` header。
- [ ] 前端通过 axios response interceptor 统一处理结构化 API error，把 `{ code, error, requestId, details? }` 归一化为调用层可处理的错误对象或错误消息；feature API 不再分散解析错误响应。
- [ ] 不向后兼容旧 `{ code, message, error }` 错误结构；本任务完成后前后端只按 best-practices 结构对齐。
- [ ] `requestId` 必须接入服务端日志，保证响应中的 request id 能关联到对应请求日志和错误日志。
- [ ] 如果请求已携带 `X-Request-ID`，后端优先沿用为 `requestId`，回写响应 header，并传递到下游；否则后端自行生成 request id，并同样输出到响应和下游。
- [ ] `error` 字段是否赋值由配置控制；关闭时不泄露底层错误细节，开启时也不得输出密钥、token、连接串等敏感信息。
- [ ] 前端 `ApiErrorResponse` 类型和 `responseError()` 适配新结构化错误，优先展示安全摘要，并包含 HTTP status / code 便于定位。
- [ ] 前端不保留旧结构兼容分支；如果收到非 best-practices JSON 错误结构，应按协议不匹配处理。
- [ ] 测试覆盖结构化错误响应：至少覆盖 invalid JSON、method not allowed、unauthorized、device offline / upstream failure 中的关键路径。
- [ ] 不执行 git 写操作。

## Open questions

1. WebSocket 升级后的 terminal control error 是否保持现有 protocol `{ type: "error", code, message }`，还是也增加 request id？本轮需求倾向先不扩大到升级后协议；普通 HTTP API 和 agent relay 请求必须传递 request id。
2. 404 兜底是否覆盖 Go `ServeMux` 自动生成的 not found，由 Plan 阶段结合“serve 前端”需求自行评估；如果会影响前端静态资源 fallback，应避免破坏 SPA 服务。

## Decisions

- 使用标准模式 / standard：本任务涉及后端 HTTP 边界、前端 API client 和测试，但目标相对聚焦，不需要 strict 的独立 Spec 阶段。
- 用户确认使用 best-practices 结构：错误响应采用 `{ code, error, requestId, details? }`，不沿用也不向后兼容历史 `{ code, message, error }`。
- 参考 best-practices 的默认策略：内部错误保留诊断上下文，HTTP 边界统一转换为稳定、安全、可测试的错误响应。
- 本轮 `details` 可以先不实现；结构设计需允许将来追加可选 details，不把 details 作为当前必填交付。
- 前端引入 axios，并使用 interceptor 统一生成/注入 request id、统一处理结构化 API error。
- `requestId` 必须接入日志，不能只在响应 body 中出现；后端收到 request id 必须沿用并传递到下游，没有则自行生成。
- `error` 字段是否赋值使用配置控制；关闭时不得透出内部错误，开启时仍需避免敏感信息泄露。
- 404 兜底为了服务前端路由和 API 调用的一致体验，具体覆盖范围由 Plan 阶段结合现有 frontend serving 方式自行评估。
- 成功响应不包装，保持现有 DTO 字段语义。
- 前后端请求/响应结构命名统一使用 `XxxxReq` / `XxxxResp`；嵌套结构视具体需求，不强制要求。
- 代码标识符中的 ID 缩写统一使用 `Id` 而非全大写 `ID`；只改代码层命名，不改变 JSON 字段名和外部 wire format。
- 优先调整 `gatewayapi` HTTP 边界；现有 CLI / infrastructure error kind 不作为本轮主要改造对象，除非 Plan 阶段发现可复用最小映射。
- 不修改历史相邻 SpecFlow 文档；只在本需求中记录兼容关系和待决策项。

## Risk

1. 历史需求曾要求 `{ code, message, error }`，本次已确认采用 best-practices 的 `{ code, error, requestId, details? }` 且不向后兼容；实现必须一次性同步前后端，否则会出现运行时契约断裂。
2. 直接替换 `http.Error` 可能影响现有测试对纯文本 body 的断言，需要同步更新测试并审查状态码是否保持语义一致。
3. agent relay 当前把远端错误文本作为 `502` body 透传；改为安全摘要后，前端展示更安全，但调试必须依赖日志和 request id，否则定位体验会下降。
4. `net/http` 的默认 not found / method handling 若绕过自定义 helper，可能仍返回纯文本；Plan 阶段需要识别哪些路径能被当前 handler 覆盖。
5. WebSocket 握手前错误可以走 HTTP JSON；握手后的 terminal protocol 不适合强行改成 HTTP error body，需避免过度统一破坏协议。
6. 如果错误响应全部 JSON 化，前端读取 history 等 text endpoint 的失败路径仍可走 JSON error，但成功路径保持 text；axios client 需要区分 success responseType/content 和 error response content。
7. request id 必须接入日志和下游请求；如果实现时只生成响应字段而未进入请求日志/错误日志/agent relay，则不满足需求。
8. 前端引入 axios 会改变 API client 基础设施；必须集中封装，避免组件层直接依赖 axios 细节或形成 fetch/axios 双轨长期并存。
9. `error` 字段由配置控制可能造成开发/生产行为差异；测试需要覆盖关闭配置下不泄露内部错误，必要时覆盖开启配置下有诊断信息。
10. `XxxxReq` / `XxxxResp` 命名会触及前后端导出类型；实现必须区分 API contract DTO 与内部/domain/display 类型，不能为了命名统一误改非 API 结构或业务语义。
11. `ID` → `Id` 是代码标识符命名规则，不是 wire format 变更；实现时必须避免误改 `json:"..._id"`、URL 参数、localStorage key 或协议字段名。

## User review notes

- 用户原始指令：`标准 开始结构化 API error 任务，思路 D:\SourceCodes\mywork\best-practices\docs\practices\error-handling.md 参考 D:\SourceCodes\mywork\best-practices\examples\project-structures\go-service，details 可以先不实现`。
- 已读取 best-practices error handling 文档，确认推荐契约重点是边界统一转换、稳定错误码、安全摘要、request id、可选 details，不把内部 debug/stack/source path 暴露进稳定响应。
- 已读取 go-service 示例 `internal/http/errors.go`、`handler.go`、`handler_test.go`，其示例结构为 `{ code, error, requestId, details? }`，成功响应不包装。
- 已初步核对当前项目：`gatewayapi/server.go`、`gatewayapi/auth/auth.go` 存在多处 `http.Error` 纯文本错误；前端 `responseError()` 和 `ApiErrorResponse` 目前仍偏向旧结构，需要纳入后续 Plan。
- 用户确认：1) 使用 best-practices 结构；2) requestId 接入日志；3) 使用配置控制 `error` 是否赋值；4) 404 兜底是为了 serve 前端，具体实现自行评估；5) 采纳本 Requirement。
- 用户补充要求：前端引入 axios，由 request interceptor 生成/注入 request id，由 response interceptor 统一处理异常；后端收到 request id 必须沿用并输出到下游和响应，没有则自行生成。
- 用户补充确认：不向后兼容旧错误结构，前后端一次性切换到 best-practices 结构。
- 用户补充要求：所有前后端请求结构使用 `XxxxReq`，所有前后端响应结构使用 `XxxxResp`；嵌套结构视具体需求，不强制要求。
- 用户补充要求：所有代码标识符中的 `AbcdID` 改为 `AbcdId`，`abcdID` 改为 `abcdId`；该规则不改变 JSON 字段名和外部协议字段。
