# Runtime API Origin 集成改进任务需求

最后修改时间: 2026-07-02 15:13:20

Review status: Accepted

## Background

本任务基于以下最佳实践文档评估本项目当前 Vite 前端与 Go 后端的 API Origin 集成边界：

```text
D:\SourceCodes\mywork\best-practices\docs\guides\vite-runtime-api-origin-integration.md
D:\SourceCodes\mywork\best-practices\docs\guides\vite-runtime-api-origin-integration.manual.md
```

最佳实践的核心结论是：

```text
Vite dev proxy 是开发工具；生产 API Origin 是运行时交付配置；CORS 是后端 API 暴露策略。
```

项目当前相关现状：

1. `web/vite.config.ts` 已为本地开发配置 `/api -> http://127.0.0.1:9030` dev proxy。
2. `web/src/features/api/client.ts` 的 `apiClient` 当前 `baseURL` 为空，API module 多数直接传入 `/api/...` 路径。
3. `web/src/features/runtimeTarget.ts` 统一表达 local/cloud runtime path shape：
   - local -> `/api{path}`
   - cloud -> `/api/devices/{device_id}{path}`
4. terminal websocket URL 当前由 `terminalWsUrl` 生成相对路径，再由 `new URL(url, window.location.href)` 解析为当前页面 Origin。
5. `internal/transport/http/server/server.go` 当前静态服务会直接 `http.ServeFile(index.html)`，没有运行时 `window.__CONFIG__` 注入。
6. `internal/infrastructure/config/config.go` 的 `server` 配置已有 `mode`、`listen_url`、`static_dir`、`public_url`，但没有 `api_base_url` 或 `cors_allowed_origins`。
7. `internal/transport/http/gatewayapi` 目前只在 WebSocket accept 处根据 request host 生成 origin patterns，没有面向 HTTP API 的 CORS middleware。
8. 根目录 `.env.example` 与打包目录 `bin/package/TermBridge/.env.example` 尚未说明拆分 API Origin 需要的 runtime API base URL 与 CORS allowlist 配置。

因此，本改进任务不是修改 local/cloud runtime 业务语义，而是补齐“同一份 Vite 构建产物在部署时可指向同源或拆分 API Origin”的运行时交付能力。

## Goal

1. 让生产前端构建产物不绑定某个环境的 API Origin。
2. 在后端服务 `index.html` 时注入公开 runtime config，使前端能读取部署环境的 API base URL。
3. 让普通 HTTP API 请求通过统一入口拼接 runtime API base URL。
4. 让不能走普通 `apiClient` 的 URL 场景，例如 terminal WebSocket URL，具备明确的共享 URL 构造边界，避免继续隐式绑定当前页面 Origin。
5. 在 API Origin 与页面 Origin 拆分时，由后端通过保守 allowlist 返回 CORS 响应头。
6. 将 runtime API base URL 与 CORS allowlist 归属到 `server.*` 运行时交付配置中，并同步默认配置与 `.env.example`。
7. 保持本地 Vite dev proxy 只服务开发体验，不把生产 API Origin 写入 `vite.config.ts`。

## Non-goal

1. 不改变 `RuntimeTarget` 对 local/cloud runtime path shape 的业务语义。
2. 不改变 gateway API 的 route contract，例如 `/api/...` 与 `/api/devices/{device_id}/...`。
3. 不改变 auth、device、tunnel、session、workspace 的业务规则。
4. 不把 API Origin 写入 Vite build-time 必需环境变量。
5. 不默认开放 `Access-Control-Allow-Origin: *`。
6. 不默认启用 `Access-Control-Allow-Credentials`。
7. 不处理跨站 cookie、SameSite、CSRF 或多 API Origin 分流；当前项目主要按 Bearer token / Authorization header 场景评估。
8. 不在 Requirement 阶段修改产品代码。

## User scenarios

### Scenario 1: 本地开发继续使用 Vite dev proxy

开发者运行 Vite dev server 时，浏览器仍请求相对 `/api/...`，由 `web/vite.config.ts` proxy 到本地后端。

期望：

1. 本地开发不需要设置生产 API Origin。
2. `vite.config.ts` 的 proxy 不被误用为生产 routing 配置。
3. 本地 dev proxy 与生产 runtime config 职责清晰分离。

### Scenario 2: 生产同源部署

后端同时服务静态前端和 API，例如页面与 API 都在同一 Origin 下。

期望：

1. `server.api_base_url` 为空。
2. 前端继续请求 `/api/...`。
3. `server.cors_allowed_origins` 为空时，不额外开放跨域。
4. 现有 local/cloud route shape 与 terminal 行为保持兼容。

### Scenario 3: 生产拆分页面 Origin 与 API Origin

静态页面由 `https://termbridge.example.com` 提供，API 由 `https://termbridge-api.example.com` 提供。

期望：

1. 后端服务 `index.html` 时注入公开配置，例如：

```html
<script>window.__CONFIG__ = {"apiBaseUrl":"https://termbridge-api.example.com"};</script>
```

2. 前端 HTTP API 请求实际发往 `https://termbridge-api.example.com/api/...`。
3. API 服务只允许配置中的页面 Origin，例如 `https://termbridge.example.com`。
4. 带 `Authorization` header 的跨域请求 preflight 成功。
5. 未被 allowlist 命中的 Origin preflight 被拒绝。

### Scenario 4: 维护者审查前端 API 调用入口

维护者需要确认是否有请求绕过 runtime API base URL。

期望：

1. 普通 JSON API 请求统一经过 `apiClient` 或等价 request wrapper。
2. runtime path shape 仍由 `RuntimeTarget` 负责，但最终 URL origin 由 runtime API base URL 负责。
3. terminal WebSocket URL 或其他不能走 `apiClient` 的 URL 使用共享 builder，而不是散落 `window.location.origin` 或隐式当前 Origin 假设。

## Acceptance

### A1. 配置结构表达 runtime API origin 与 CORS allowlist

后续实现应在 `server` 配置域增加运行时交付配置。

验收标准：

1. `configs/config.yaml` 中 `server.api_base_url` 默认为空，表示同源 `/api`。
2. `configs/config.yaml` 中 `server.cors_allowed_origins` 默认为空列表，表示不开放跨域。
3. `internal/infrastructure/config.Config.Server` 暴露对应字段。
4. 环境变量绑定支持：
   - `TERMBRIDGE_SERVER__API_BASE_URL`
   - `TERMBRIDGE_SERVER__CORS_ALLOWED_ORIGINS`
5. `api_base_url` 通过 URL 校验，允许空值；非空时必须是 `http` 或 `https` absolute URL。
6. `cors_allowed_origins` 为空时保持保守默认，不返回跨域 allow header。

### A2. 后端服务 index.html 时注入 runtime config

后续实现应让静态服务对 `index.html` 和 SPA fallback 注入 runtime config。

验收标准：

1. `web/index.html` 或构建后的 `index.html` 有稳定注入点，且位于应用入口脚本之前。
2. 普通静态资源，例如 `.js`、`.css`、图片，不注入 runtime config。
3. `server.api_base_url` 有值时，HTML 中包含 `window.__CONFIG__.apiBaseUrl`。
4. `server.api_base_url` 空时，HTML 中的 runtime config 为空对象或等价空配置。
5. `HEAD` 请求不写 body，但仍保持合理 content type / status 行为。
6. API missing route 不应被 SPA fallback 吃掉。

### A3. 前端有统一 runtime API URL 边界

后续实现应让前端业务代码不直接关心 API Origin 来源。

验收标准：

1. 新增或调整 `web/src/config.ts` 或等价模块读取 `window.__CONFIG__`。
2. 普通 API client 使用 runtime API base URL 作为 base URL，当前 API module 仍可传入 `/api/...` 路径。
3. `RuntimeTarget` 继续只负责 local/cloud path shape，不负责读取部署 Origin。
4. 若保留 `import.meta.env.VITE_API_BASE_URL`，只能作为 fallback 或特殊本地构建路径，不作为生产镜像主配置面。
5. 前端测试覆盖同源空 base URL 与拆分 Origin base URL 的 URL 生成行为。

### A4. terminal WebSocket URL 不隐式绑定错误 Origin

terminal WebSocket 是当前项目不能直接走 axios request wrapper 的关键场景，需要纳入 runtime API Origin 设计。

验收标准：

1. `terminalWsUrl` 或其调用链使用共享 URL builder 支持 runtime API base URL。
2. 同源部署时，WebSocket 仍连接当前页面 Origin 下的 `/api/.../ws`。
3. 拆分 Origin 且 `apiBaseUrl=https://termbridge-api.example.com` 时，WebSocket URL 转为对应 `wss://termbridge-api.example.com/api/.../ws` 或按浏览器规则等价处理。
4. query token、terminal size 参数保持兼容。
5. 相关单元测试覆盖 `http -> ws` 与 `https -> wss` 的 scheme 转换。

### A5. 后端 CORS middleware 只覆盖 API 路径并使用 allowlist

后续实现应在 HTTP API 路径上支持拆分 Origin 的浏览器访问。

验收标准：

1. allowed origins 为空时 middleware no-op。
2. 请求路径不是 `/api` 或 `/api/` 前缀时不处理 CORS。
3. 请求没有 `Origin` 时不处理 CORS。
4. Origin 命中 allowlist 时返回：
   - `Access-Control-Allow-Origin`
   - `Access-Control-Allow-Headers: Authorization, Content-Type`
   - `Access-Control-Allow-Methods` 覆盖当前 API 使用的主要方法
   - `Access-Control-Max-Age`
   - `Vary: Origin`
5. allowed preflight 返回 `204`。
6. denied preflight 返回 `403`，且不回显攻击者 Origin。
7. 非 OPTIONS 的未知 Origin 请求不设置 allow header，仍由后续 API/auth 逻辑处理。
8. 不设置 `Access-Control-Allow-Credentials`。

### A6. 文档与样例配置同步

验收标准：

1. 根目录 `.env.example` 说明 `TERMBRIDGE_SERVER__API_BASE_URL` 与 `TERMBRIDGE_SERVER__CORS_ALLOWED_ORIGINS`。
2. `bin/package/TermBridge/.env.example` 同步说明这两个变量。
3. 注释明确：
   - `API_BASE_URL` 为空表示同源 `/api`。
   - `CORS_ALLOWED_ORIGINS` 配的是前端页面 Origin，不是 API Origin。
   - 多 Origin 使用逗号分隔。
4. README 或部署文档如已有相关部署章节，应避免继续暗示 Vite dev proxy 可解决生产跨域。

### A7. 验证覆盖默认值、拆分 Origin 和拒绝未知 Origin

验收标准：

1. Go 配置测试覆盖默认空值与环境变量解析。
2. Go 静态服务测试覆盖 runtime config 注入。
3. Go CORS middleware 测试覆盖 allowlist 为空、命中、未命中、preflight、非 API path、无 Origin。
4. 前端测试覆盖 runtime config 读取、API URL build、terminal WebSocket URL build。
5. 项目验证命令应优先使用现有工具链，例如 Go test 与 `web/package.json` 中的 `test` / `typecheck` / `lint`。

## Open questions

1. Runtime config 模块命名是否采用 `web/src/config.ts`，还是放入现有 `web/src/features/api` 边界下？建议优先使用独立 `web/src/config.ts`，因为它表达部署配置而非业务 API。
2. 后端 runtime config 注入逻辑是否放在 `internal/transport/http/server`，还是由 `internal/app` 传入一个静态服务配置对象？建议放在 HTTP server 静态服务边界，但配置值由 app 装配传入。
3. CORS middleware 是否放在 `internal/transport/http/middleware/cors`，还是先作为 `server` 包私有 middleware？建议使用独立 transport middleware，便于 gateway API 复用和测试。
4. `server.api_base_url` 是否允许尾部 `/` 并 normalize，还是按最佳实践由配置提供方保证不带尾部 `/`？建议实现层做最小 normalize：trim 空白、去掉尾部 `/`，但不无限兜底错误 URL。
5. 当前 WebSocket accept 的 `originPatterns(r)` 只允许同 request host。拆分 Origin 后，terminal WebSocket 请求来自页面 Origin、目标是 API Origin，是否需要复用 `cors_allowed_origins` 扩展 WebSocket Origin allowlist？这会影响 terminal 拆分 Origin 是否可用，应在实现前确认。

## Decisions

已形成的初步决策：

1. 本任务使用 light / 轻量模式推进，但 requirement 需要保留适中澄清与接近 standard 的验证要求。
2. API Origin 不进入 Vite build-time 主配置面。
3. `server.api_base_url` 与 `server.cors_allowed_origins` 属于同一运行时交付配置域。
4. 默认同源、默认不开放跨域、默认不启用 credentials。
5. 本任务不改变 local/cloud runtime 业务边界，只补齐页面 Origin 与 API Origin 拆分部署能力。
6. terminal WebSocket URL 必须纳入同一 runtime API Origin 方案，不能只覆盖 axios HTTP 请求。

## Risk

1. 如果只修改 axios baseURL，不处理 terminal WebSocket URL，拆分 Origin 下终端连接仍会打到静态站点 Origin。
2. 如果只注入 runtime config，不配置 CORS allowlist，浏览器会因 Authorization preflight 被拦截。
3. 如果 CORS 默认过宽，例如 `*` 或 credentials，可能扩大 API 暴露面。
4. 如果 `server.api_base_url` 在 build 阶段使用，会重新引入“构建产物绑定环境”的技术债。
5. 如果 WebSocket Origin 校验仍只接受 request host，拆分 Origin 下即使 HTTP API 可用，terminal WebSocket 也可能被拒绝。
6. 如果 `go test ./...` 扫描 `web/node_modules` 中的 Go 包，验证可能受前端依赖目录影响；验证阶段应选择项目约定的 Go package 范围或先确认当前测试入口。

## User review notes

暂无。

## Next stage

当前处于 light / 轻量模式的 Requirement / 需求阶段。

如果用户接受本需求或要求开始实现，则进入 Implementation / 实现阶段。实现阶段建议优先顺序：

1. 配置结构与样例配置。
2. runtime config 注入。
3. 前端 runtime config / URL builder / API client 接入。
4. terminal WebSocket URL 接入。
5. CORS middleware 与相关测试。
