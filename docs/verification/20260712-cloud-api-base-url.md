# Cloud API Base URL 配置与 Cloud SDK 抽取验证
最后修改时间: 2026-07-12 14:11:26

- Flow mode: light
- Stage: Verification
- Review status: Accepted
- Requirement: `docs/requirement/20260712-cloud-api-base-url.md`
- Spec / Plan: 不适用（light 流程按已接受的 Requirement 核对）

## Requirement alignment

- `cloud.api_base_url` 已加入共享配置模型、配置键注册、环境变量绑定、规范化与启动阶段校验；其环境变量为 `TERMBRIDGE_CLOUD__API_BASE_URL`。
- 配置校验要求最终值为非空、绝对 `http` 或 `https` URL；不会在 Agent Handler 中延后检查 Cloud URL。
- hybrid 配置将浏览器入口与服务间入口分离：`cloud.public_url=http://localhost:9030`，`cloud.api_base_url=http://127.0.0.1:9032`。
- Agent Cloud HTTP 调用已收敛至 `internal/agent/infrastructure/cloudapi`。设备注册使用 `POST <ApiBaseUrl>/api/devices/current`，OAuth code exchange 使用 `<ApiBaseUrl>/api/oauth2/token`。
- Agent Handler 仅调用 `*application.CloudService`；不再持有 Cloud URL、构造 Cloud HTTP 请求、拼接 Cloud API 路径、创建 OAuth client 或作 Cloud URL 规范化。
- persistent Tunnel 的连接目标与 Cloud 端 `AgentTunnelAudience` 均直接使用 `cfg.Cloud.ApiBaseUrl`，没有 `cloud.public_url` 或 `cloud.listen_url` fallback，也没有转发适配函数。
- 浏览器 DTO 继续使用同源 `/api` 作为浏览器 API base；服务端 `cloud.api_base_url` 没有投影到 `window.__CONFIG__`。前端开发环境的 `TERMBRIDGE_CLOUD__API_BASE_URL=/cloud-api` 属于浏览器 Vite proxy 路径，与服务端环境变量同名但位于独立配置边界，未被 Go 服务端读取。

## Actual diff summary

- 新增 Cloud API Base URL 的共享配置字段、环境变量绑定、格式校验及回归测试，并更新 baseline、development、package 与示例配置。
- 新增 Agent Cloud HTTP SDK 及设备注册请求目标测试；通过 `CloudApi` 应用端口由 `CloudService` 编排设备注册和 OAuth token exchange。
- 精简 Agent Handler Cloud 连接/OAuth 逻辑，bootstrap 在组合根创建 SDK 与应用服务后注入 Handler。
- 将 Agent Tunnel 目标及 Cloud Tunnel audience 改为 `ApiBaseUrl`。
- 同步 portable runtime profile 的配置预期。

## Expected and actual file scope

| 范围 | 验证结论 |
| --- | --- |
| Shared config、示例与运行 profile | 已修改，符合 Requirement。 |
| Agent Handler、application、bootstrap 与 Cloud SDK | 已修改，符合 Handler → application service → outbound port → infrastructure adapter 分层。 |
| Cloud bootstrap Tunnel audience | 已修改，符合服务间 endpoint 对齐要求。 |
| 回归测试 | 已更新配置、应用装配、Handler binding 与 SDK 请求目标覆盖。 |
| Browser runtime config | 未向 DTO 注入服务端 `cloud.api_base_url`；现有 `/api` 浏览器契约保持不变。 |

当前 staged diff 还包含两项与本功能无关的既有前端修改：`web/src/views/cloud/LoginView.vue`、`web/src/views/cloud/OAuthAuthorizeView.vue` 的 i18n key 修正，以及 `web/.env.development` 的开发浏览器配置。它们不属于本次 Cloud API Base URL 的功能范围；提交前应按功能边界拆分。

## Acceptance checklist

- [x] `TERMBRIDGE_CLOUD__API_BASE_URL` 已绑定、加载、去空白/尾斜杠规范化，并在配置阶段校验为绝对 HTTP(S) URL。
- [x] hybrid 配置使用 `http://127.0.0.1:9032` 作为 `cloud.api_base_url`，并保留 `http://localhost:9030` 作为 `cloud.public_url`。
- [x] 设备注册、OAuth token exchange 与 persistent Tunnel 使用 Cloud API Base URL；Cloud HTTP 路径构造集中在 SDK。
- [x] Handler 不直接构造 Cloud HTTP 请求、拼接 Cloud URL 或创建 OAuth client；应用层经 `CloudApi` 端口依赖基础设施 SDK。
- [x] Agent Handler 已移除 Cloud URL 的运行时空值校验和规范化职责。
- [x] 服务端 `cloud.api_base_url` 未进入浏览器 DTO 或 `window.__CONFIG__`。
- [x] 覆盖配置缺失/非法 URL、SDK 设备注册目标及完整 Go 回归测试；未保留 `cloud.public_url` fallback 或转发适配层。

## Test results

| 命令 | 结果 |
| --- | --- |
| `go test ./...` | 通过。 |
| `go vet ./...` | 通过。 |
| `git diff --check -- '*.go'` | 通过。 |
| `git -c core.whitespace=cr-at-eol diff --cached --check` | 通过。 |
| `git -c core.whitespace=cr-at-eol diff --check` | 通过。 |

普通 `git diff --check` 会将 Windows CRLF 行尾报告为 trailing whitespace，涉及 `.env.example`、`configs/config.development.yaml`、`configs/config.yaml` 与既有的 `web/.env.development`；使用项目适用的 `core.whitespace=cr-at-eol` 后 staged 与 working-tree diff 均通过。

## Risks and incomplete items

- 这是刻意的破坏性配置变更：Agent 连接、OAuth exchange 和 Tunnel 所在部署必须使最终加载的 `cloud.api_base_url` 有效；不提供对 `cloud.public_url` 的自动兼容。
- baseline `configs/config.yaml` 提供同源默认值，development/package profile 则显式覆盖为其目标地址。配置阶段验证保证最终值不可缺失；部署者仍应按实际网络拓扑明确设置环境变量或环境配置。
- 本轮未实际启动 Vite、Agent、Cloud 三个进程并执行浏览器端到端连接；自动化测试验证了配置、HTTP SDK 请求目标和全量 Go 回归路径，真实部署拓扑仍需运行时 smoke test。
- staged diff 混入前述范围外前端修改；没有改动、回退或提交这些文件。

## Conclusion

实现与已接受的 light Requirement 对齐：Agent 到 Cloud 的服务间 endpoint 已从浏览器公开入口解耦，配置在启动阶段验证，Cloud HTTP 调用被隔离至 SDK，Tunnel target 与 audience 一致使用 `ApiBaseUrl`，且未保留兼容 fallback 或无意义适配层。全量 Go 测试、vet 与适配 CRLF 的 diff 检查均通过。实现可交付，但提交前应先将范围外前端改动拆分，并在目标部署拓扑执行一次端到端 smoke test。
