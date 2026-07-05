# Agent Cloud OAuth 设备绑定收敛验证
最后修改时间: 2026-07-05 20:32:22

Review status: Accepted

## Requirement alignment

依据 `docs/requirement/20260705-agent-cloud-oauth-device-binding.md` 核对。

| 验收项 | 结论 | 说明 |
|---|---|---|
| Agent 本机 identity 使用 `data/device.json`，不再以 `data/agent.json` 作为主路径，也不引入旧文件自动迁移业务 | 通过 | `internal/agent/application/user/device.go` 使用 `DeviceIdentityFileName = "device.json"`，未保留 `LegacyDeviceIdentityFileName` 或旧文件自动 rename 逻辑。 |
| Agent 启动不再为了本机设备身份写入 `agent.db.devices` | 通过 | Agent bootstrap 仍加载本地 device identity 并将 `device.Id` 传给 runtime store，但已移除 Agent-local device repository 写入路径。 |
| Agent runtime 业务表保留 `device_id` 数据归属，但不依赖本机 `devices` 外键 | 通过 | Agent sqlite/mysql migration 移除本机 `devices` 表和 `device_id -> devices(id)` 外键，保留 `device_id` 字段与索引。相关 app/runtime 测试通过。 |
| Cloud OAuth start/callback 本地状态不再写入 `data/auth/cloud-oauth-attempts.json` | 通过 | 旧 attempt store 文件与测试删除；Agent OAuth state 改为内存态，带 TTL 并在 callback 消费一次。 |
| OAuth2 state/code/token 职责归 Cloud authorization server；Agent 不本地持久化 state/code、不生成 OAuth2 authorization code | 通过 | Agent 只生成 client state 并调用 Cloud `/cloud-api/oauth2/token` 换 token；Cloud handler 持有 authorize/code/token 逻辑。 |
| OAuth2 state/callback 不携带 device id/name/public key；device 信息只在 `/cloud-api/devices/current` 上报 | 通过 | Agent handler 测试断言 authorize URL 不包含本机 device 信息，并断言 callback 成功后才 POST 当前 device id/name/public key。 |
| 前后端不再暴露或依赖 `capabilities.cloud_oauth_enabled` 控制 Cloud 连接入口 | 通过 | 当前活跃 Go/TS/Vue 代码搜索未发现 `AuthCapabilities`、`Capabilities` 或 `cloud_oauth_enabled` 的运行时代码引用；命中项仅为历史过程文档和本需求文档。 |
| Cloud OAuth 成功后 Cloud DB 写入 `devices` 与 `user_devices` | 通过 | Cloud handler/repository 测试覆盖 `/cloud-api/devices/current` 的设备上报和用户设备绑定写入；Cloud DB migration 保留 `devices` / `user_devices`。 |
| OAuth 必要配置缺失时明确失败，不通过 capability 隐藏入口 | 通过 | capability 开关已移除；Agent API 在缺少 Cloud OAuth/Gate 配置时由 handler 返回明确 API 错误。 |
| develop / hybrid 配置支持完整链路 | 通过 | `configs/config.develop.yaml` 已配置 `cloud.gate_url: http://localhost:9030` 与 Cloud OAuth redirect。App-level OAuth completion 测试覆盖 token exchange、device report 与 connector start。 |
| Cloud tunnel 认证继续基于本机私钥签名与 Cloud DB 公钥验签 | 通过 | 本次未移除 Cloud `devices.public_key` 权威路径；Cloud tunnel 相关测试仍在 `go test ./cmd/... ./internal/...` 中通过。 |

## Spec alignment

不适用。本功能当前没有同名 `docs/spec/20260705-agent-cloud-oauth-device-binding.md`；验证按已接受的 requirement 与实现计划核对。

## Plan alignment

项目内没有同名 `docs/plan/20260705-agent-cloud-oauth-device-binding.md`。实现过程遵循会话中已批准的计划，覆盖以下方向：

- 移除 Agent DB device identity 写入与 Agent-local `devices` schema 依赖。
- 将 Cloud binding summary 以非 token 摘要写入 `device.json`。
- 移除 `cloud-oauth-attempts.json` / `CloudOAuthAttemptStore`。
- Agent OAuth state 增加 TTL、过期清理、一次性消费。
- Cloud OAuth token endpoint 收敛到 `/cloud-api/oauth2/token`，并使用 form-encoded token request。
- 移除旧 exchange DTO/proto 类型与前端 capability 依赖。
- 更新 app integration、develop config 与当前 README/requirement 文档。

## Actual diff summary

直接属于本需求的主要改动：

- Agent local identity：`internal/agent/application/user/device.go`、`device_test.go`。
- Agent OAuth handler：`internal/agent/api/handler/server.go`、`dto.go`、`cloud_binding_test.go`，并删除旧 attempt store。
- Agent bootstrap/runtime boundary：`internal/agent/application/bootstrap/*`、`internal/agent/repository/task/state/*`、删除 `internal/agent/repository/device/repository.go`。
- Agent migrations：`migrations/agent/mysql/202607020001_agent_runtime_schema.sql`、`migrations/agent/sqlite/202607020001_agent_runtime_schema.sql`。
- Cloud OAuth/device binding：`internal/cloud/api/handler/*`、`internal/cloud/repository/user/device/repository.go`、Cloud identity migrations。
- API contract/proto/frontend：`proto/termbridge/cloud/v1/cloud.proto`、generated Go/TS proto、`web/src/features/*`、`web/src/store/gateway*`、相关 views。
- Integration/config/docs：`cmd/termbridge/app/app_test.go`、`configs/config.develop.yaml`、`README.md`、本 requirement 文档。

本次工作树中也存在与本需求不完全同域的已暂存改动，例如：

- `docs/requirement/20260705-acronym-casing-cleanup.md` 与对应 verification。
- `docs/requirement/20260705-transaction-usage-audit-findings.md` 与对应 verification。
- `CLAUDE.md`、若干命名/配置/中间件/邮件实现相关文件。

这些改动可能来自同一工作会话的其他任务。若准备提交，建议用户按交付边界拆分 review 或拆分 commit，避免把 OAuth/device-binding 收敛与其它主题混在一个提交里。

## Expected vs actual changed files

| 类别 | 预期 | 实际 |
|---|---|---|
| Agent local identity / DB boundary | 应修改 | 已修改，且测试覆盖 device file persistence、runtime DB 不依赖 Agent-local devices。 |
| Agent OAuth state / callback | 应修改 | 已修改，覆盖 start URL、invalid/expired/reused state、token exchange、device report、restart summary load。 |
| Cloud OAuth/token/device API | 应修改 | 已修改，token endpoint 使用 `/cloud-api/oauth2/token`，device report 继续写 Cloud DB。 |
| Proto / frontend store / dashboard | 应修改 | 已修改，移除旧 exchange req 与 capability 依赖。 |
| develop config / README / requirement | 应修改 | 已修改，`cloud.gate_url` 与 `device.json` 行为已落文档。 |
| 无关过程文档和其它主题改动 | 不属于本需求核心 | 实际存在，需要提交前人工确认是否拆分。 |

## Acceptance checklist

- [x] Agent identity 主路径为 `device.json`。
- [x] 不保留旧 `agent.json` 自动迁移业务。
- [x] 不再写 Agent-local `devices` 表。
- [x] Agent runtime `device_id` 不再 FK 到本机 `devices`。
- [x] 不再持久化 Cloud OAuth attempt store。
- [x] OAuth state 内存态、带 TTL、callback 一次性消费。
- [x] Agent 不生成或持久化 OAuth2 authorization code。
- [x] Device 信息不进入 authorize URL / OAuth state / OAuth code。
- [x] Device 信息只在 OAuth token exchange 成功后上报 `/cloud-api/devices/current`。
- [x] Cloud DB `devices` / `user_devices` 保留为账号设备绑定权威。
- [x] 不持久化 Cloud access/refresh token 到 `device.json`。
- [x] 不引入 PKCE。
- [x] 不再依赖 `capabilities.cloud_oauth_enabled`。

## Command results

| 命令 | 结果 | 说明 |
|---|---|---|
| `go test ./internal/agent/application/user ./internal/agent/api/handler` | Passed | 聚焦 Agent device identity 与 OAuth handler。 |
| `go test ./internal/cloud/api/handler ./internal/cloud/repository/user/device` | Passed | 聚焦 Cloud OAuth/device binding。 |
| `go test ./internal/agent/infrastructure/database ./internal/agent/repository/task/state ./cmd/termbridge/app` | Passed | 聚焦 Agent migration/runtime/app integration。 |
| `go test ./cmd/... ./internal/...` | Passed | 后端更宽测试套件通过。 |
| `yarn --cwd web test` | Passed | 11 个 test files、55 个 tests 通过。 |
| `yarn --cwd web typecheck` | Passed | Vue/TypeScript 类型检查通过。 |
| `yarn --cwd web lint` | Passed | ESLint 通过。 |
| `yarn --cwd web format:check` | Failed | Prettier 报 7 个文件格式不符合：`src/features/api/client.ts` 与 generated proto TS 文件。未自动执行 `format`，因为这会批量改写文件。 |
| `./bin/golangci-lint run ./cmd/... ./internal/...` | Skipped | 本地 `./bin/golangci-lint` 不存在；未安装工具，未执行 Go lint。 |

## Missed or expanded scope

- Expanded：实现中同步修正了 requirement 文档中旧 `agent.json -> device.json` 迁移表述，使文档与用户后续决策一致。
- Expanded：当前工作树包含其它任务文档和命名/配置相关改动；这些不是 OAuth/device-binding 验证的核心范围。
- Missed：未运行完整 `task check`，因为 `format:check` 已失败且 `./bin/golangci-lint` 不存在。已分别运行可用的 typecheck、lint、test 与 Go tests。

## Risks

1. 当前工作树 diff 较大且混有其它主题改动，提交前需要人工确认拆分边界。
2. `yarn --cwd web format:check` 失败，虽然失败集中在 TS client/generated proto 文件，但在交付前应决定是否运行格式化或调整生成器输出策略。
3. Go lint 未运行，因为本地缺少 `./bin/golangci-lint`；如果 CI 要求 lint，提交前需要安装工具并补跑。
4. Cloud OAuth 仍未引入 PKCE，这是本需求明确 non-goal，不作为缺陷。
5. OAuth state 不跨 Agent 重启恢复，这是本需求接受的行为；如果未来要求恢复未完成授权，需要新需求单独设计。

## Incomplete items

- [ ] 处理或确认 `yarn --cwd web format:check` 报告的 7 个文件格式问题。
- [ ] 安装 `./bin/golangci-lint` 后补跑 Go lint，或确认当前交付不以本地 lint 为准。
- [ ] 提交前拆分或确认包含其它主题改动的 diff 边界。

## Conclusion

Agent Cloud OAuth 设备绑定收敛的核心业务验收通过：Agent 本机 identity 已收敛为文件权威，Agent runtime 不再依赖本机 `devices` 表，OAuth2 职责回到 Cloud authorization server，device binding 在 OAuth 完成后通过 Cloud device report 落库，Cloud tunnel 继续依赖设备私钥签名与 Cloud DB 公钥验签。

当前交付仍有工程交付层面的未完成项：前端 format check 失败、Go lint 因工具缺失未运行、工作树包含其它主题改动。建议在正式提交前处理这些事项。