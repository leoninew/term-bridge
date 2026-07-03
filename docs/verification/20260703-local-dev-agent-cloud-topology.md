# 本地开发与 agent/cloud 部署拓扑验证

最后修改时间: 2026-07-03 20:07:44

Review status: Accepted

## Requirement alignment

依据 `docs/requirement/20260703-local-dev-agent-cloud-topology.md` 核对。

- 本地开发拓扑已表达为一个 Web/Vite dev server 同时代理 agent 与 cloud。
- Vite dev server 监听 `localhost:9030`。
- Vite dev proxy 支持：
  - `/agent-api` -> `http://127.0.0.1:9031`
  - `/cloud-api` -> `http://127.0.0.1:9032`
- `/agent-api` 与 `/cloud-api` 被限定为本地开发代理前缀，不写入最终运行 baseline 语义。
- `configs/config.yaml` 保持最终运行 baseline：agent/cloud 默认同源 `/api`，`api_base_url` 为空。
- 新增 `configs/config.develop.example.yaml` 表达本地联调端口与代理前缀。
- 前端已区分 agent/cloud API base：agent/runtime local 请求走 agent client，cloud auth/device/cloud runtime 请求走 cloud client。
- local cloud OAuth callback 统一到 `http://localhost:9030/cloud/oauth/callback`。

## Spec alignment

不适用。当前流程为 light / 轻量模式，没有单独 spec 文档；按 requirement / 需求核对。

## Plan alignment

不适用。当前流程为 light / 轻量模式，没有单独 plan 文档；实现按 requirement / 需求直接推进。

## Actual diff summary

主要变更：

1. 配置与环境
   - 更新 `configs/config.yaml`，恢复最终运行 baseline：agent/cloud 默认 `9030` 同源运行，`api_base_url: ""`。
   - 新增 `configs/config.develop.example.yaml`，记录本地开发联调：web `9030`、agent `9031`、cloud `9032`、`/agent-api`、`/cloud-api`。
   - 更新 `.env.example` 与 README，明确本地 dev proxy 路径不是最终产品 API path。
   - 更新 `.gitignore`，允许提交 `configs/config.*.example.yaml`。

2. 本地开发启动
   - 更新 `Taskfile.yml`：`task run` 描述为同时启动 web/agent/cloud；`task agent` 和 `task cloud` 注入本地联调端口与 API base。
   - 更新 `scripts/run.py`：默认注入本地联调环境变量，先执行 agent/cloud migrate，再启动 agent、cloud 与 web。
   - 更新 `web/vite.config.ts`：Vite 监听 `9030`，代理 `/agent-api` 到 agent `9031`，代理 `/cloud-api` 到 cloud `9032`。
   - 将本地 dev proxy 的 Vite 环境变量放入 `web/.env.development`，避免 `web/.env` 在 production build 中固化 `/agent-api` 和 `/cloud-api`。

3. 前端 API 分流
   - 更新 `web/src/config.ts`，支持 `agentApiBaseUrl`、`cloudApiBaseUrl` 与 target-specific API URL/WebSocket URL 构造。
   - 更新 `web/src/features/api/client.ts`，保留兼容的 `apiClient` 作为 agent client，并新增 cloud client。
   - 更新 gateway、sessions、workspaces API 模块：local runtime/agent 请求走 agent client，cloud login/device/cloud runtime 请求走 cloud client。
   - 更新相关前端测试覆盖 agent/cloud base 分流与 path-based WebSocket URL。

4. 后端配置验证与测试
   - `internal/shared/config` 允许 `api_base_url` 使用本地 path base（如 `/agent-api`），但拒绝 protocol-relative URL（如 `//example`）。
   - 更新配置测试覆盖 agent/cloud 本地开发端口与 path API base。
   - 补齐 `internal/agent/api/test_keys_test.go`，使 agent API 测试包可构建。
   - 更新 cloud OAuth callback 测试端口为 `9030`。

5. 其他修复
   - 修复 `web/src/i18n.ts` 中已有的缺失逗号语法错误；否则 typecheck 和 format:check 无法通过。

## Expected vs actual changed files

预期涉及：

- `docs/requirement/20260703-local-dev-agent-cloud-topology.md`
- `docs/verification/20260703-local-dev-agent-cloud-topology.md`
- `configs/config.yaml`
- `configs/config.develop.example.yaml`
- `.env.example`
- `.gitignore`
- `README.md`
- `Taskfile.yml`
- `scripts/run.py`
- `web/.env`
- `web/.env.development`
- `web/vite.config.ts`
- `web/src/config.ts`
- `web/src/env.d.ts`
- `web/src/features/api/client.ts`
- `web/src/features/gateway/api.ts`
- `web/src/features/runtimeTarget.ts`
- `web/src/features/sessions/api.ts`
- `web/src/features/workspaces/api.ts`
- 对应测试文件

实际额外涉及：

- `web/src/i18n.ts`：为通过现有 typecheck/format 检查修复缺失逗号。
- `internal/agent/api/test_keys_test.go`：为通过完整 Go 测试补齐 agent API test key helper。
- `internal/agent/api/cloud_binding_test.go`：同步 OAuth callback 端口为 `9030`。

这些额外变更都与验证当前拓扑及保持测试套件可运行直接相关。

## Acceptance criteria checklist

- [x] 本地开发默认拓扑在 README / config example / task runner 中表达为 web `9030`、agent `9031`、cloud `9032`。
- [x] Vite dev server 监听 `localhost:9030`。
- [x] Vite dev proxy 支持 `/agent-api` -> `http://127.0.0.1:9031`。
- [x] Vite dev proxy 支持 `/cloud-api` -> `http://127.0.0.1:9032`。
- [x] 前端本地开发能通过 agent/cloud 两个 base 分流请求，不依赖单一全局切换。
- [x] `/agent-api` 与 `/cloud-api` 只作为本地开发代理前缀描述。
- [x] agent 本机最终/打包语义保持同源 `/api`，不要求用户运行 Vite。
- [x] cloud 云端部署 baseline 保持同源 `/api`。
- [x] `configs/config.yaml`、`.env.example`、README、Taskfile 与 package 语义不再互相矛盾。
- [x] 配置加载测试覆盖 agent/cloud 端口与 role-specific `expose_errors`，并新增 path API base 覆盖。

## Test results

已运行并通过：

```text
go test ./internal/shared/config ./cmd/termbridge/app ./internal/agent/app ./internal/cloud/app
```

```text
go test ./internal/agent/api
```

```text
go test ./...
```

```text
go vet ./cmd/... ./internal/...
```

```text
yarn --cwd web typecheck
```

```text
yarn --cwd web lint
```

```text
yarn --cwd web test
```

```text
yarn --cwd web format:check
```

过程中曾出现但已修复：

- `web/src/i18n.ts` 缺失逗号导致 `vue-tsc` 与 `prettier --check` 失败。
- `internal/agent/api` 缺失 `testJWTKey` 导致 `go test ./...` 与 `go vet` 失败。

## Missed or expanded scope

- 未实际启动 `task run` 做浏览器端手工联调；本次验证覆盖配置、构建类型检查、lint、前端单元测试和 Go 测试。
- 未新增生产部署脚本、Kubernetes、systemd 或反向代理配置，符合 non-goal。
- 未重构完整前端认证产品流，只在现有 API 层做 agent/cloud base 分流。

## Risks

- `task run` 中 cloud 仍通过 `go run` 直接启动，不具备 agent 侧 Air 热重载能力；这符合当前最小实现，但开发体验后续可再优化。
- 前端 runtime config 仍保留兼容字段 `apiBaseUrl`；如果部署同时设置 legacy `apiBaseUrl` 与新的 target-specific base，需要以 target-specific base 为准，目前代码已按此优先级处理。
- 当前 cloud 登录相关 API 默认走 cloud client；如果后续产品要求 local mode 下某些认证动作仍由 agent 代理，需要再按具体流程拆分。

## Incomplete items

暂无阻塞当前需求交付的未完成项。

## Conclusion

当前实现满足 light / 轻量模式 requirement。配置 baseline、本地开发代理、前端 API 分流、文档说明与测试验证均已对齐。完整 Go 测试、Go vet、前端 typecheck/lint/unit/format 检查均通过。
