# 运行时前端配置与同源 API 契约
最后修改时间: 2026-08-19 11:07:07

- Flow mode: standard
- Stage: Requirement
- Review status: Accepted

## Background

当前前端以 Vite `web/.env.*` 在构建时注入 `TERMBRIDGE_*` 配置。`web/src/store/runtimeConfig.ts` 已支持优先读取 `window.__CONFIG__`，而 `web/index.html` 也保留 `__RUNTIME_CONFIG__` 标记，但 Agent 和 Cloud 的静态资源处理器仍将 `index.html` 原样返回；因此打包产物不能由后端运行时环境配置改变。

开发模式必须继续保留 `web/.env.development`：Vite 前端运行在 `http://localhost:9030`，Local Agent 后端运行在 `http://localhost:9031`，Cloud 后端运行在 `http://localhost:9032`。同一 Vite origin 需要通过 `/local-api` 与 `/cloud-api` 将浏览器请求分别代理给两个开发后端。

测试和正式部署的拓扑不同：

| 环境 | Local 前后端 | Cloud 前后端 |
| --- | --- | --- |
| 测试 | `http://localhost:9030` | `http://termbridge.lvh.me` |
| 正式 | `http://localhost:9030` | `https://termbridge.preflite.cn` |

在这些部署中，每个 Local 或 Cloud 制品各自托管自己的前端和后端，前后端同源。测试与正式制品仅因地址不同而构建不同的 `web/.env.*`，会导致浏览器 bundle 与实际 Go 服务运行时配置漂移；`Dockerfile.preflite` 与 `web/.env.cloud` 的 Cloud public URL 已体现该风险。

此外，`/local-api` 与 `/cloud-api` 当前同时是 Vite 开发代理前缀和 Go 后端硬编码的路由契约。它们不应继续成为打包、测试或正式部署中后端 API 的模式标识。服务身份应由访问 origin 或开发代理目标决定。

## Goal

1. 让测试和正式的 Local / Cloud 前端制品使用后端在 HTML entry 响应中安全注入的 `window.__CONFIG__`，在运行时获得正确的公开地址、API 地址、模式和 OAuth 浏览器配置，而不依赖 `web/.env.local` 或 `web/.env.cloud` 中的部署地址。
2. 保留 `web/.env.development` 作为 Vite 本地开发的完整浏览器配置来源；开发不要求经由后端注入配置。
3. 将 Agent 与 Cloud 后端 API 迁移为不带 `/local-api`、`/cloud-api` 模式前缀的 canonical path；让这些前缀仅在 Vite development proxy 中承担浏览器分流职责，并由 proxy 去除前缀后转发。
4. 让打包 Local、测试 Cloud、正式 Cloud 的浏览器 API 调用使用各自 origin 下的 canonical API path，而非依赖 mode-specific prefix。
5. 清理或重新定义目前 Go 配置中未被路由或浏览器消费的 `local.api_base_url`、`cloud.api_base_url`，消除“配置存在但不生效”的误导。

## Non-goal

- 不改变开发环境的端口拓扑：Vite `:9030`、Local Agent `:9031`、Cloud `:9032`。
- 不把本地 Agent 服务暴露给云端页面；Cloud 访问远程设备仍通过 Agent 主动建立的认证隧道。
- 不将 OAuth client secret、JWT secret、数据库 DSN、Turnstile secret、邮件服务密钥或其他服务端敏感配置注入浏览器。
- 不在本需求阶段指定最终 canonical API path 命名、HTML 注入编码实现或缓存策略细节；这些属于后续 Plan 的设计决策。
- 不在本需求阶段实现 `/dashboard` 的“切换到本地模式”入口；该入口应在运行时配置契约确定后单独恢复。

## User scenarios

### 开发者本地开发

开发者在三个终端分别执行 `task agent`、`task cloud` 和 `task web`，并访问 `http://localhost:9030`。浏览器继续从 `web/.env.development` 读取 hybrid 配置；Vite 将 `/local-api/*` 代理到 `http://127.0.0.1:9031`，将 `/cloud-api/*` 代理到 `http://127.0.0.1:9032`，并在转发时使后端接收到 canonical API path。

### 用户运行本地打包制品

用户打开本地 Agent 提供的 `http://localhost:9030`。该 Agent 以自身运行时配置注入完整公开浏览器配置；本地页面和 Local API 同源，浏览器使用不包含 mode prefix 的 canonical API path。Cloud 公开地址可以在本地制品运行时配置，而无需重新构建前端。

### 测试环境访问 Cloud

用户访问 `http://termbridge.lvh.me`。Cloud 服务以当前运行时配置注入浏览器配置，Cloud 页面和 Cloud API 同源；将服务切换到另一个测试域名或端口时，不需重新构建前端 bundle。

### 正式环境访问 Cloud

用户访问 `https://termbridge.preflite.cn`。Cloud 服务以当前运行时配置注入浏览器配置，浏览器调用该 origin 的 canonical Cloud API path，并生成与该公开地址一致的 OAuth、WebSocket 与跨入口链接。

### Agent 连接 Cloud

本地 Agent 使用 Cloud 公开地址建立 OAuth token、设备注册和 authenticated tunnel 连接。相关 HTTP / WebSocket / 签名请求与 Cloud canonical API path 一致，不再依赖 `/cloud-api`。

## Acceptance

1. `web/.env.development` 仍提供开发环境所需的完整配置，并且 Vite `:9030` 仍可将 Local 与 Cloud 请求分别代理到 `:9031`、`:9032`。
2. Local 打包制品和 Cloud 镜像可使用不包含测试/正式部署地址的同一浏览器 bundle；修改后端启动环境中的公开地址等浏览器运行时配置后，不重新构建前端即可生效。
3. Agent 与 Cloud 静态处理器仅对 HTML entry / SPA fallback 响应注入完整、已校验、仅公开字段的 `window.__CONFIG__`；JS、CSS、图片等静态资源保持原样可缓存。
4. 注入结果能安全嵌入 `<script>` 上下文，不允许配置值突破脚本边界；缺失或无效的浏览器运行时配置必须在受控边界失败，不能静默产生错误地址。
5. 浏览器配置能完整表达当前 schema 中的 local mode、Local public URL、Cloud public URL、OAuth client ID/redirect/scopes 与 API endpoint 语义；服务端密钥绝不出现在 HTML 响应。
6. Local 与 Cloud 后端注册并处理不含 `/local-api`、`/cloud-api` 的 canonical API paths；API namespace 下未知端点返回 API 404，不得被 SPA fallback 返回 `index.html` 吞没。
7. 开发中的 `/local-api`、`/cloud-api` 仅保留在 `web/vite.config.ts` 的代理边界；代理会移除前缀，生产后端路由、handler path parsing、CORS API 分类、Agent-to-Cloud HTTP/WebSocket 请求及签名路径不再依赖这些前缀。
8. 已移除或真正实现 `local.api_base_url`、`cloud.api_base_url` 的后端配置契约，不再保留目前“加载但不影响路由或浏览器”的状态；文档、环境变量模板、Docker/package 配置与测试须和最终契约一致。
9. 相关单元与集成测试覆盖 Agent / Cloud HTML 注入、配置覆盖、空或无效配置、敏感字段排除、canonical HTTP/WS/tunnel paths、开发代理 rewrite 以及 SPA/API 404 分流。

## Open questions

1. `window.__CONFIG__` 的运行时配置源是否应直接复用现有 Go 配置字段，还是引入独立的、明确只含公开字段的 `BrowserRuntimeConfig`？后者更利于防止敏感字段泄漏。
2. same-origin 浏览器 API base 是否统一为 `/api`，并在开发时由 Vite 将 `/local-api`、`/cloud-api` rewrite 为 `/api`？该选择会影响 Axios、WebSocket URL 生成、部署在子路径的支持和 CORS 策略。
3. HTML 注入后的 HTTP 缓存策略、ETag / `Content-Length` 处理和 CDN/proxy 行为应采用何种契约？
4. 是否需要将 runtime config 从静态 HTML 注入改为由后端提供 JSON/JS endpoint：虽然这会更利于独立缓存和运行时更新，但会引入前端异步 bootstrap、初始路由前配置加载、失败 UI/重试、开发环境中配置归属及额外请求等复杂度。当前倾向保留内联注入，等待计划阶段确认。


## Decisions

1. 开发环境和测试/正式部署的配置来源应分离：开发使用 `web/.env.development`，测试/正式由后端运行时注入 `window.__CONFIG__`。
2. 测试 Cloud 的公开地址为 `http://termbridge.lvh.me`，正式 Cloud 的公开地址为 `https://termbridge.preflite.cn`；本地打包 Local 公开地址为 `http://localhost:9030`。
3. canonical API 使用统一中性前缀 `/api/...`；Local 与 Cloud 服务在不同 origin 下各自提供同名或相近 API namespace。
4. `/local-api` 与 `/cloud-api` 在目标状态中仅是 development proxy 的识别前缀，并由 proxy rewrite 到 `/api/...`；它们不再是后端 API contract 或跨服务调用路径。
5. 不为旧 `/local-api`、`/cloud-api` 路由提供兼容或适配；迁移为一次性 contract 变更。
6. Cloud 页面不会直接调用用户机器的 Local API；远程设备访问继续经由 Agent 到 Cloud 的认证隧道。

## Risk

- 迁移 API path 会影响所有 Agent/Cloud handlers、路由挂载、CORS、HTTP client、terminal WebSocket、tunnel 签名与验签、测试以及外部或已部署客户端；需要明确兼容策略并避免部分迁移。
- 失去 mode prefix 后，SPA fallback 与 API 404 的分流会成为路由关键点；错误实现会将 API 404 返回为 `index.html`，掩盖客户端错误。
- 运行时配置注入若采用不安全的字符串拼接，可能导致 XSS 或泄露服务端配置；必须仅序列化 allowlist 的公开字段并使用 script-safe encoding。
- 当前 `runtimeConfig` 对任何非空 `window.__CONFIG__` 要求完整 schema；注入源不完整会在前端初始化失败。
- Vite `.env.local` 有特殊覆盖语义；继续将其作为通用 build 输入容易混淆 development、portable package 和部署配置边界。
- Docker/打包运行时环境与构建时 bundle 的解耦可能暴露未配置的 public URL、OAuth redirect 或 origin 问题，必须通过启动期校验和端到端测试保障。

## User review notes

- 2026-07-11：开发模式中 Local 与 Cloud 前端同在 `http://localhost:9030`，后端分别在 `http://localhost:9031` 与 `http://localhost:9032`，所以前端需要 `/local-api` 与 `/cloud-api` 分流。
- 2026-07-11：本地打包后 Local 前后端同在 `http://localhost:9030`；正式 Cloud 前后端同在 `https://termbridge.preflite.cn`，测试 Cloud 前后端同在 `http://termbridge.lvh.me`；这些部署不需要以 mode prefix 区分后端。
- 2026-07-11：canonical API 确定使用 `/api/...`；不保留旧 `/local-api`、`/cloud-api` 的兼容路由或适配层。
- 2026-07-11：初始计划是内联 HTML 注入 `window.__CONFIG__`。已评估后端提供 runtime-config endpoint 的替代方案：它可以更独立地缓存或扩展为运行时刷新，但当前同源部署与同步启动模型下会额外引入异步 bootstrap、首屏失败处理和开发代理归属问题；倾向保留内联注入，在 Plan 阶段确定最终交付方式与 CSP/缓存契约。
