# API 响应与错误结构规范化需求
最后修改时间: 2026-06-19 23:53:36

Review status: Accepted

## Background

当前后端 API 已经有集中错误处理路径，常见业务错误通过 `writeError` 返回 JSON 包装结构：

```json
{
  "error": {
    "code": "usage_error",
    "message": "..."
  }
}
```

但本次用户要求统一为：

```json
{"code":"...","message":"...","error":"..."}
```

同时要求所有成功响应“直接是预期内容”，即不再额外包一层业务字段，除非接口预期内容本身就是对象结构。

## Goals

1. 统一后端 API 成功响应：
   - 成功响应 body 直接返回接口预期内容。
   - 列表接口返回数组本身，而不是 `{ "items": [...] }` 或 `{ "sessions": [...] }` 这类包装结构，除非需求明确该接口预期内容就是对象。
2. 统一后端 API 错误响应结构：
   - 所有 API 错误响应使用顶层字段：
     ```json
     {"code":"...","message":"...","error":"..."}
     ```
   - `code` 表示机器可读错误码。
   - `message` 表示面向用户或调用方的错误说明。
   - `message` 是用户友好说明。
   - `error` 是开发阶段提供的堆栈或底层调试信息；生产阶段是否脱敏或置空在 Spec 阶段设计。
3. 覆盖所有 HTTP API 错误来源：
   - registry/domain 返回的业务错误。
   - JSON 请求解析错误。
   - method not allowed。
   - forbidden origin。
   - route not found。
4. WebSocket 错误能沿用 HTTP API 规则的场景应沿用，例如升级前的 attach 失败；升级后的 protocol error 如需调整，应尽量保持与 `{code,message,error}` 语义一致，同时不破坏 terminal protocol 的必要字段。

## Non-goals

1. 不改动 API 的业务能力，例如 session 创建、编辑、删除、workspace tree/order 等具体功能语义。
2. 不引入新的认证、鉴权或权限模型。
3. 不做历史 state 文件迁移。
4. 不主动修改前端调用逻辑，除非实现过程中发现测试或类型编译必须同步调整。
5. 不改变已有业务能力；但错误分类需要补充更符合语义的 HTTP 状态码，例如业务 not found 返回 `404` 与 `code: "not_found"`，其他异常按类似方式分类处理。

## User scenarios

1. 作为前端调用方，请求成功时可以直接消费响应主体，无需额外拆包装字段。
2. 作为前端调用方，请求失败时可以稳定读取：
   - `code`
   - `message`
   - `error`
3. 作为 API 调试者，访问不存在路由或使用错误方法时，也能收到一致 JSON 错误结构。
4. 作为前端用户，请求失败时 UI 优先展示稳定、友好的 `message`，不要默认暴露 `error` 调试详情。
5. 作为前端维护者，API client、协议类型和调用点需要与后端响应类型一致，避免在运行时依赖旧包装结构。
6. 作为维护者，新增 API handler 时可以复用统一 response/error helper，避免每个 handler 自行构造不一致结构。

## Acceptance criteria

1. 成功响应结构：
   - 成功响应格式与接口返回类型定义一致。
   - 不引入统一 `code`、`data` 等包装层。
   - `GET /api/sessions` 返回 session summary 数组本身。
   - workspace/session 相关新增接口成功响应直接返回其预期内容，不再使用无必要包装层。
2. 错误响应结构：
   - registry/domain error 返回：
     ```json
     {"code":"...","message":"...","error":"..."}
     ```
   - invalid JSON request 返回同样结构。
   - method not allowed 返回同样结构。
   - forbidden origin 返回同样结构。
   - API route not found 返回同样结构。
3. HTTP status code：
   - 业务 not found 返回 `404`，`code` 为 `not_found`。
   - validation / usage 类调用错误返回 `400`。
   - config 类错误返回 `400`，除非 Spec 阶段识别出更合适分类。
   - runtime/internal 错误返回 `500`。
   - 其他异常按语义做类似分类处理，并在 Spec 阶段列出映射表。
4. 前端适配：
   - 前端 API client 解析成功响应时与后端返回类型一致，不再读取旧 `{workspaces}` / `{sessions}` 包装。
   - 前端 HTTP 错误解析支持顶层 `{code,message,error}`，UI-facing 错误优先使用 `message`，可携带 HTTP status 与 `code` 便于定位。
   - 前端默认不展示 `error` 调试详情，避免在普通 UI 中暴露开发期堆栈、路径或底层错误链。
   - 前端协议类型包含 HTTP `ApiErrorResponse` 与 WebSocket `type: "error"` 消息中的可选 `error` 字段。
   - 前端所有 `fetch` 调用点都应走统一错误解析 helper，避免局部拼接旧错误格式。
5. 测试：
   - 更新或新增 webserver 层测试覆盖成功响应无包装和错误响应三字段结构。
   - 如果涉及 registry error mapping 调整，补充相应测试。
   - 前端至少通过项目现有 typecheck；如后续增加前端 API client 测试，应覆盖 raw success response 与 `{code,message,error}` 错误解析。
6. 兼容性记录：
   - 如现有前端或测试依赖旧错误结构 `{ "error": { ... } }`，需在 Spec 或 Plan 中明确列出并处理。

## Open questions

1. WebSocket 升级后的 protocol error 如果不能完全采用 `{code,message,error}` 顶层结构，需要在 Spec 阶段说明原因与替代结构。

## Decisions

1. `message` 是用户友好说明。
2. `error` 是开发阶段提供的堆栈或底层调试信息。
3. 正确响应格式必须和返回类型定义一致，不使用统一 `code` / `data` 包装。
4. 业务 not found 返回 HTTP `404`，`code` 为 `not_found`。
5. 其他异常也应按语义类似分类处理。
6. WebSocket 能沿用 `{code,message,error}` 语义的场景就沿用；不能沿用时实现方自行处理，并在过程文档中说明。
7. 开发阶段是否输出 `error` 堆栈由现有配置系统控制，不通过硬编码 build mode 判断。

## Risks / Assumptions

1. 假设本次变更主要约束 HTTP API；WebSocket 升级后的 terminal protocol 如不能完全沿用，需要在 Spec / Verification 中说明。
2. 成功响应拆包装可能影响现有前端请求解析逻辑和测试，需要在 Spec 阶段识别受影响调用点。
3. 错误响应结构从嵌套对象改为顶层三字段属于 API breaking change，需要确认前端同步策略。
4. 引入 `404 not_found` 等更细 HTTP 映射会改变现有调用方对 status code 的判断。
5. 开发阶段输出堆栈到 `error` 字段可能泄露本地路径或内部实现细节，Spec 阶段必须明确通过现有配置系统控制其启用条件。
6. 前端如果只展示 `message`，调试人员需要从 Network 面板或日志查看 `error` 字段；这是默认 UI 安全性与调试便利性的取舍。
7. 当前前端缺少 API client 单元测试，现阶段主要依赖 typecheck 与人工核对调用点，未来可补充 mock fetch 测试。

## User review notes

- 用户原始要求：确保所有请求：正确响应直接是预期内容；错误响应是 `{ "code": "...", "message": "...", "error": "..." }` 结构；使用 `/specflow` 严格模式。
- 用户补充决策：`message` 是用户友好说明，`error` 是开发阶段提供的堆栈；正确响应格式和返回类型定义一致，没有 `code` / `data` 包装；业务 not found 返回 HTTP `404` 与 `code: "not_found"`，其他异常类似处理；WebSocket 能沿用以上规则就沿用，不能沿用则自行处理并说明。
- 用户补充决策：开发阶段是否输出堆栈使用项目已有配置系统控制；WebSocket 升级后的处理需要解释并展示建议。
- 用户补充要求：前端也需要纳入异常响应调整需求，并检查前端适配工作。
