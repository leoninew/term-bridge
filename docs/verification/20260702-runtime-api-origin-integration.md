# Runtime API Origin 集成改进任务验证

最后修改时间: 2026-07-02 16:41:33

Review status: Draft

## Requirement alignment / 需求对齐

本次验证依据 `docs/requirement/20260702-runtime-api-origin-integration.md` 执行，需求文档状态为 `Accepted`。

总体结论：实现范围与 requirement 的核心目标基本一致，覆盖 runtime API base URL、`index.html` 运行时配置注入、前端统一 URL 构造、terminal WebSocket URL、API CORS allowlist，以及相关 Go / 前端测试。

## Spec alignment / 规格对齐

不适用。本任务按 light / 轻量模式推进，未创建独立 `docs/spec/20260702-runtime-api-origin-integration.md`。

## Plan alignment / 计划对齐

不适用。本任务按 light / 轻量模式推进，未创建独立 `docs/plan/20260702-runtime-api-origin-integration.md`；验证按 requirement 中的实现建议顺序和验收标准核对。

## Actual diff summary / 实际 diff 摘要

当前 staged diff 涉及 19 个文件，核心变更如下：

1. 配置层：
   - `configs/config.yaml` 增加 `server.api_base_url` 与 `server.cors_allowed_origins` 默认配置。
   - `internal/infrastructure/config/config.go` 增加字段、环境变量 key、normalize 和 URL 校验。
   - `internal/infrastructure/config/config_test.go` 覆盖默认空值、环境配置、`.env` 解析与 normalize 行为。
2. 后端 HTTP 服务：
   - `internal/transport/http/server/server.go` 对 `index.html` 和 SPA fallback 注入 `window.__CONFIG__`，普通静态资源不注入。
   - API handler 包装 CORS middleware。
   - `internal/transport/http/server/server_test.go` 覆盖静态服务、SPA fallback、API missing route、runtime config 注入和 `HEAD` 行为。
3. CORS：
   - 新增 `internal/transport/http/middleware/cors.go`，只处理 `/api` 与 `/api/` 前缀，使用 allowlist，拒绝未知 Origin preflight。
   - 新增 `internal/transport/http/middleware/cors_test.go` 覆盖 allowlist 为空、命中、未命中、preflight、非 API path、无 Origin、credentials header 不设置。
4. WebSocket Origin：
   - `internal/transport/http/gatewayapi/server.go` 将 `server.cors_allowed_origins` 复用于 WebSocket accept origin patterns，解决拆分 Origin 下 terminal WebSocket Origin 校验问题。
5. 前端：
   - `web/index.html` 增加稳定 runtime config 注入点。
   - 新增 `web/src/config.ts`，集中读取 `window.__CONFIG__`，构造 HTTP API URL 与 WebSocket API URL。
   - `web/src/features/api/client.ts` 在请求阶段设置 axios `baseURL` 为 runtime API base URL。
   - `web/src/features/sessions/api.ts` 的 `terminalWsUrl` 改用共享 WebSocket URL builder。
   - 前端测试覆盖 runtime config、API URL 与 terminal WebSocket URL。
6. 文档与样例：
   - 根目录 `.env.example` 增加 runtime API base URL 与 CORS allowlist 说明。
   - `bin/package/TermBridge/.env.example` 当前工作区文件也包含对应说明，但该路径位于 `bin/` 下，未被 git 跟踪；打包流程会从根目录 `.env.example` 复制生成。

## Expected vs actual changed files / 预期与实际改动对比

| 类别 | Requirement 预期 | 实际改动 | 结论 |
| --- | --- | --- | --- |
| 配置默认值 | `configs/config.yaml` 增加 `server.api_base_url` 与 `server.cors_allowed_origins` | 已修改 | 符合 |
| 配置结构与校验 | `internal/infrastructure/config` 暴露字段、env 绑定、URL 校验 | 已修改并有测试 | 符合 |
| runtime config 注入 | HTTP static server 注入 `window.__CONFIG__` | 已修改并有测试 | 符合 |
| 前端 runtime config | `web/src/config.ts` 或等价模块读取 `window.__CONFIG__` | 已新增 | 符合 |
| API client | 普通请求统一使用 runtime API base URL | 已修改 axios request interceptor | 符合 |
| terminal WebSocket URL | 使用共享 builder，支持 `http/https -> ws/wss` | 已修改并有测试 | 符合 |
| CORS middleware | API path allowlist、preflight 行为 | 已新增并有测试 | 符合 |
| WebSocket Origin 校验 | 拆分 Origin 下不只接受 request host | 已复用 `CORSAllowedOrigins` | 符合 |
| 样例配置 | 根目录与发布包 `.env.example` 同步说明 | 根目录已跟踪修改；`bin/package/...` 当前文件已同步但路径未被 git 跟踪，打包时复制根目录文件 | 基本符合，需注意交付形态 |
| 部署文档 | 若已有相关部署章节，避免暗示 dev proxy 可解决生产跨域 | README 当前仅描述本地开发与镜像静态资源，没有新增误导性 dev proxy 说明 | 符合 |
| 无关文件 | 不应修改无关 feature 文档 | `docs/requirement/20260702-agent-cloud-internal-split.md` 为 untracked，未纳入 staged diff | 不属于本次验证范围 |

## Acceptance criteria checklist / 验收清单

### A1. 配置结构表达 runtime API origin 与 CORS allowlist

- [x] `configs/config.yaml` 中 `server.api_base_url` 默认为空。
- [x] `configs/config.yaml` 中 `server.cors_allowed_origins` 默认为空列表。
- [x] `internal/infrastructure/config.Config.Server` 暴露 `APIBaseURL` 与 `CORSAllowedOrigins`。
- [x] 支持 `TERMBRIDGE_SERVER__API_BASE_URL`。
- [x] 支持 `TERMBRIDGE_SERVER__CORS_ALLOWED_ORIGINS`。
- [x] `api_base_url` 允许空值；非空时按 http/https absolute URL 校验。
- [x] `cors_allowed_origins` 为空时 CORS middleware no-op，不返回跨域 allow header。

### A2. 后端服务 index.html 时注入 runtime config

- [x] `web/index.html` 有稳定注入点，位于入口脚本之前。
- [x] 普通静态资源不注入 runtime config。
- [x] `server.api_base_url` 有值时 HTML 包含 `window.__CONFIG__.apiBaseUrl`。
- [x] `server.api_base_url` 空时 HTML 注入空对象。
- [x] `HEAD` 请求不写 body，并保持 200 与 HTML content type 行为。
- [x] API missing route 不被 SPA fallback 吃掉。

### A3. 前端有统一 runtime API URL 边界

- [x] `web/src/config.ts` 读取 `window.__CONFIG__`。
- [x] 普通 API client 使用 runtime API base URL，业务 API module 仍传 `/api/...` 路径。
- [x] `RuntimeTarget` 未被改为读取部署 Origin。
- [x] `VITE_API_BASE_URL` 仅作为 fallback，不是主要生产配置面。
- [x] 前端测试覆盖同源空 base URL 与拆分 Origin URL 行为。

### A4. terminal WebSocket URL 不隐式绑定错误 Origin

- [x] `terminalWsUrl` 使用共享 URL builder 支持 runtime API base URL。
- [x] 同源部署时仍返回当前页面 Origin 下的相对 `/api/.../ws`。
- [x] 拆分 Origin 时 `https` API base URL 转为 `wss` WebSocket URL。
- [x] query token、terminal size 参数保持兼容。
- [x] 单元测试覆盖 `http -> ws` 与 `https -> wss`。

### A5. 后端 CORS middleware 只覆盖 API 路径并使用 allowlist

- [x] allowed origins 为空时 middleware no-op。
- [x] 非 `/api` 或 `/api/` 前缀路径不处理 CORS。
- [x] 无 `Origin` 请求不处理 CORS。
- [x] 命中 allowlist 时返回 `Access-Control-Allow-Origin`。
- [x] 命中 allowlist 时返回 `Access-Control-Allow-Headers: Authorization, Content-Type`。
- [x] 命中 allowlist 时返回覆盖主要 API 方法的 `Access-Control-Allow-Methods`。
- [x] 命中 allowlist 时返回 `Access-Control-Max-Age`。
- [x] 命中 allowlist 时返回 `Vary: Origin`。
- [x] allowed preflight 返回 `204`。
- [x] denied preflight 返回 `403`，且不回显未知 Origin。
- [x] 非 OPTIONS 未知 Origin 不设置 allow header，继续后续 API/auth 逻辑。
- [x] 不设置 `Access-Control-Allow-Credentials`。

### A6. 文档与样例配置同步

- [x] 根目录 `.env.example` 说明 `TERMBRIDGE_SERVER__API_BASE_URL`。
- [x] 根目录 `.env.example` 说明 `TERMBRIDGE_SERVER__CORS_ALLOWED_ORIGINS`。
- [x] 注释说明 API base URL 为空表示同源 `/api`。
- [x] 注释说明 CORS allowlist 配置前端页面 Origin，不是 API Origin。
- [x] 注释说明多 Origin 使用逗号分隔。
- [x] `bin/package/TermBridge/.env.example` 当前工作区文件已同步，但该路径不在 git 跟踪中；发布包由 Taskfile 从根目录 `.env.example` 复制生成。

### A7. 验证覆盖默认值、拆分 Origin 和拒绝未知 Origin

- [x] Go 配置测试覆盖默认空值与环境变量解析。
- [x] Go 静态服务测试覆盖 runtime config 注入。
- [x] Go CORS middleware 测试覆盖 allowlist 为空、命中、未命中、preflight、非 API path、无 Origin。
- [x] 前端测试覆盖 runtime config 读取、API URL build、terminal WebSocket URL build。
- [x] 已按项目工具链运行 Go test、Go vet、前端 test/typecheck/lint/format check。

## Test results / 命令结果

| 命令 | 结果 | 说明 |
| --- | --- | --- |
| `go test ./cmd/... ./internal/...` | Pass | Go cmd/internal 测试通过。 |
| `go vet ./cmd/... ./internal/...` | Pass | Go vet 无输出，表示通过。 |
| `yarn --cwd web test` | Pass | Vitest：12 个 test files、51 个 tests 全部通过。 |
| `yarn --cwd web typecheck` | Pass | `vue-tsc --noEmit` 通过。 |
| `yarn --cwd web lint` | Pass | `eslint .` 通过。 |
| `yarn --cwd web format:check` | Fail | Prettier 报告 `src/views/DashboardView.vue` 格式问题；该文件不在本次 staged diff 中。 |
| `yarn --cwd web prettier --check index.html src/config.ts src/config.test.ts src/env.d.ts src/features/api/client.ts src/features/api/client.test.ts src/features/sessions/api.ts src/features/sessions/api.test.ts` | Pass | 本次涉及的前端文件均符合 Prettier。 |
| `gofmt -l internal/app/app.go internal/infrastructure/config/config.go internal/infrastructure/config/config_test.go internal/transport/http/gatewayapi/server.go internal/transport/http/middleware/cors.go internal/transport/http/middleware/cors_test.go internal/transport/http/server/server.go internal/transport/http/server/server_test.go` | Pass | 无输出，表示列出的 Go 改动文件已格式化。 |

## Missed or expanded scope / 范围偏差

1. 未发现产品代码实现超出 requirement 的核心范围。
2. `internal/transport/http/gatewayapi/server.go` 额外复用 `CORSAllowedOrigins` 扩展 WebSocket Origin allowlist；这是 requirement open question 第 5 项指出的关键风险，属于为满足 terminal 拆分 Origin 可用性所需的范围补齐。
3. `bin/package/TermBridge/.env.example` 位于 `bin/` 忽略目录下，未被 git 跟踪。当前文件内容已同步，但最终发布包实际来源是 Taskfile 中 `cp .env.example {{.PACKAGE_ROOT}}/.env.example`，因此 tracked 根目录 `.env.example` 才是可审查来源。
4. `docs/requirement/20260702-agent-cloud-internal-split.md` 是 untracked 文件，不属于本次 runtime API origin 集成验证范围。

## Risks / 风险

1. 全量 `yarn --cwd web format:check` 仍失败，原因是未参与本次 diff 的 `src/views/DashboardView.vue` 存在格式问题；如果 CI 使用全量 format check，仍会阻塞交付，需要单独修复或确认该文件是否应纳入本次提交。
2. `bin/package/TermBridge/.env.example` 不被 git 跟踪；如果交付审查要求直接看到该路径的 diff，需要改为审查打包流程复制结果，或调整仓库忽略规则后再跟踪该文件。
3. CORS allowlist 复用于 WebSocket Origin allowlist，语义上符合“允许该页面 Origin 访问 API/WS”的部署需求；后续如果 HTTP CORS 与 WebSocket Origin 策略需要拆分，应再引入独立配置，而不是继续复用。

## Incomplete items / 未完成项

1. 本次验证未修复 `src/views/DashboardView.vue` 的既有 Prettier 问题，因为该文件不在本任务需求范围内。
2. 未执行 `task check`，因为其包含全量 `yarn format:check`，当前会被上述无关文件格式问题阻塞；已分别执行等价相关命令并对本次改动文件执行 scoped Prettier check。
3. 未执行真实浏览器端到端部署验证；当前验证覆盖单元测试、静态服务行为测试和 URL 构造测试。

## Conclusion / 结论

本次 runtime API origin 集成实现与 requirement 的主要验收标准对齐，Go 与前端相关测试、类型检查、lint、Go vet 和改动文件格式检查均通过。

交付前需要注意：全量前端 format check 目前因 `src/views/DashboardView.vue` 的既有格式问题失败；若目标 CI 强制运行 `task check` 或全量 `yarn format:check`，应先处理该无关格式问题或单独拆分确认。除此之外，未发现阻塞本需求交付的实现缺口。
