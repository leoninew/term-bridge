# OAuth2 Cloud Login Redirect Loop
最后修改时间: 2026-07-10 18:45:41

Review status: Accepted

## Background

本地 Agent 从 `http://localhost:9030` 发起 Cloud OAuth2 authorization_code 流程后，浏览器会在 Cloud 的 `/oauth2/authorize` 与 `/login` 之间循环跳转，导致无法完成授权并回调本地 Agent。

已确认 Cloud 授权页通过 `/cloud-api/auth/me` 判断 Cloud token 是否有效；当 token 无效时，接口返回 `authenticated: false`。当前 Cloud 登录页在初始化后仅检查浏览器本地是否存在 `cloudToken`，而不检查 Cloud 服务端是否认可该 token。因此浏览器遗留过期、无效或由不同 JWT 密钥签发的 token 时，登录页会直接跳回授权页，授权页再因未认证跳回登录页，形成循环。

循环修复后，运行日志确认 Cloud 已完成用户认证、授权码签发并跳转本地 `GET /oauth/callback`。但 callback 页面随后直接调用受保护的 `POST /local-api/cloud/oauth/exchange`，在浏览器尚无 local Agent JWT 时返回 401。`/oauth/callback` 位于本地路由 allowlist，路由守卫不会初始化 local auth；而 code exchange endpoint 必须继续由本地 Bearer JWT middleware 保护。

## Goal

1. Cloud 登录页只在 Cloud 服务端确认认证成功后，才继续跳转至 OAuth2 授权页。
2. Cloud token 不存在、过期或无效时，清除浏览器保存的无效 Cloud token，并停留在登录页供用户完成登录。
3. 完成有效 Cloud 登录后，OAuth2 流程能够继续请求授权接口、回调 `http://localhost:9030/oauth/callback`，并由本地 Agent 完成 code exchange。
4. 对用户呈现可理解的认证状态，不再出现 `/login` 与 `/oauth2/authorize` 的无提示循环跳转。
5. 本地 `/oauth/callback` 在兑换 Cloud authorization code 前，必须先校验 OAuth state，再获取或复用 local Agent JWT，以带 Bearer token 的请求完成受保护的本地 code exchange。

## Non-goal

- 不改变 OAuth2 client_id、redirect_uri、scope、authorization code 或 token exchange 协议。
- 不将 Cloud 登录态改为 Cookie Session。
- 不调整本地 Agent 登录机制、设备连接模型或 Cloud OAuth client 注册信息。
- 不将 `/local-api/cloud/oauth/exchange` 改为匿名端点，也不放宽其 Bearer JWT middleware。
- 不让浏览器直接调用 Cloud OAuth token endpoint，不重新耦合 callback 的 code exchange 与 Cloud connect、设备上报或 tunnel 生命周期。
- 不在本次修复中处理与该循环无直接因果关系的 HTTP 日志敏感字段记录问题；该问题应作为独立安全任务处理。

## User scenarios

1. 浏览器没有 Cloud token：访问 OAuth2 授权页后进入 Cloud 登录页；成功登录后自动返回原 OAuth2 授权请求并完成本地回调。
2. 浏览器保存了有效 Cloud token：OAuth2 授权页直接签发 authorization code 并跳转本地回调。
3. 浏览器保存了无效或过期 Cloud token：授权页识别为未认证后进入登录页；登录页清除无效 token 并保持在登录页面，不再跳回授权页。
4. Cloud 登录失败：停留在登录页并展示登录失败反馈，不进入 OAuth2 授权页。
5. 浏览器通过有效 Cloud 登录回调本地 `/oauth/callback`，但此前没有 local Agent JWT：callback 先校验 OAuth state，再通过既有 local auth bootstrap 获得 JWT，随后以 Bearer token 调用受保护的 `/local-api/cloud/oauth/exchange`；兑换成功后保存 Cloud token 并恢复原本的本地跳转目标。
6. OAuth state 无效或 local auth bootstrap 失败：不得调用 `/local-api/cloud/oauth/exchange`，并沿用 callback 的错误反馈和回首页行为。

## Acceptance

- [ ] `/cloud-api/auth/me` 返回 `authenticated: false` 时，Cloud 登录页不得仅因本地仍有 `cloudToken` 而跳转至授权页。
- [ ] 无效 Cloud token 被清除后，浏览器停留在 Cloud 登录页，用户可以提交凭据。
- [ ] 有效 Cloud token 经 `/cloud-api/auth/me` 确认后，登录页能保留并恢复原始 OAuth2 授权请求。
- [ ] 有效登录后的流程发起 `POST /cloud-api/oauth2/authorize`，并跳转至本地 `/oauth/callback`。
- [ ] 增加覆盖无效 token、有效 token 和 OAuth2 登录后恢复授权请求的业务语义测试。
- [ ] 没有 `termbridge_local_token` 的浏览器完成 Cloud authorization callback 后，callback 先完成 state 校验，再获得或复用 local Agent JWT，并以该 JWT 调用 `/local-api/cloud/oauth/exchange`。
- [ ] callback 的 local auth bootstrap 或 state 校验失败时，不能进行 Cloud authorization code exchange。
- [ ] 未携带 local Agent JWT 的 `/local-api/cloud/oauth/exchange` 请求继续返回 401，且不会触发 Cloud token endpoint 请求。
- [ ] callback 成功时仍只保存 Cloud token 并恢复原始本地跳转；不由 callback 直接调用 `/local-api/cloud/connect`。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- Cloud 服务端的 `authenticated` 结果是登录态有效性的唯一依据；浏览器 Local Storage 中 token 字符串的存在不能作为认证成功依据。
- 无效 Cloud token 需要在登录页初始化时清除，避免后续路由继续使用它。
- `/oauth/callback` 保留在本地路由 allowlist 中；该页面不执行通用 local auth state 初始化，而是在调用受保护 exchange API 前显式调用 `useLocalAuthStore.ensureToken()`。
- local Agent JWT 仅用于认证本地 Agent API；Cloud authorization code 仍只提交给本地 Agent，再由本地 Agent 使用已配置的 OAuth client 凭据调用 Cloud token endpoint。
- `POST /local-api/cloud/oauth/exchange` 必须继续由现有 auth middleware 保护；401 表示 local JWT 缺失或无效，不是放宽端点安全性的依据。

## Risk

- 清除无效 token 会使已过期的 Cloud 会话要求重新登录，这是必要的安全行为。
- 修复必须保持 OAuth2 redirect 参数的完整性，避免登录成功后丢失 `response_type`、`client_id`、`redirect_uri`、`scope` 或 `state`。
- local Agent JWT 获取必须位于 OAuth state 校验之后，避免无效 callback 触发无意义的 local auth 请求。
- `ensureToken()` 失败时必须停止 code exchange；不得在没有 local JWT 时重试或降级调用受保护端点。
