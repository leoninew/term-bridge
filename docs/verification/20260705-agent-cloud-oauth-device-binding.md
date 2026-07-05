# Agent Cloud 设备上报绑定收敛验证
最后修改时间: 2026-07-05 22:17:29

Review status: Accepted

## Requirement alignment

依据 `docs/requirement/20260705-agent-cloud-oauth-device-binding.md` 核对。用户后续明确纠正后，当前验收口径是：Cloud OAuth2 / Google 登录只属于 Cloud 账号登录；Agent 连接 Cloud 不是 OAuth2 flow，而是 device report。

| 验收项 | 结论 | 说明 |
|---|---|---|
| Agent 本机 identity 使用 `data/device.json`，不再以 `data/agent.json` 作为主路径，也不引入旧文件自动迁移业务 | 通过 | `internal/agent/application/user/device.go` 使用 `DeviceIdentityFileName = "device.json"`，未保留 `LegacyDeviceIdentityFileName` 或旧文件自动 rename 逻辑。 |
| Agent 启动不再为了本机设备身份写入 `agent.db.devices` | 通过 | Agent bootstrap 仍加载本地 device identity 并将 `device.Id` 传给 runtime store，但已移除 Agent-local device repository 写入路径。 |
| Agent runtime 业务表保留 `device_id` 数据归属，但不依赖本机 `devices` 外键 | 通过 | Agent sqlite/mysql migration 移除本机 `devices` 表和 `device_id -> devices(id)` 外键，保留 `device_id` 字段与索引。 |
| Agent 不再暴露 Cloud OAuth start/callback | 通过 | 活跃 Go/TS/Vue/YAML/proto 搜索未发现 `cloud-oauth`、`CloudOAuth`、`/oauth2/authorize`、`/cloud-api/oauth2/token` 等旧连接云端业务引用。 |
| Cloud 不再暴露设备绑定专用 OAuth authorize/token endpoint | 通过 | Cloud handler 删除 `/cloud-api/cloud-oauth/authorize` 与 `/cloud-api/oauth2/token` 注册和相关 code store；保留 Cloud Google 登录与 device endpoints。 |
| Agent 连接 Cloud 只做设备上报 | 通过 | `/agent-api/cloud/connect` 读取请求体 `cloud_token`，再 POST `cloud.gate_url + /cloud-api/devices/current`，请求体只包含当前本机 device id/name/public_key。 |
| Device 信息不进入账号登录/OAuth state/code/callback | 通过 | 旧 Agent Cloud OAuth state/callback 已删除；device 信息只在 `/cloud-api/devices/current` report body 中出现。 |
| `device.json` 不持久化 Cloud token | 通过 | Agent handler 测试覆盖连接成功后本地 summary 持久化，并断言文件内容不包含 `access_token`、`refresh_token`、`cloud-token`、`bearer`。 |
| 不再依赖 `capabilities.cloud_oauth_enabled` | 通过 | 前后端能力开关与 AuthCapabilities 业务已移除；Dashboard 用显式三步业务表达连接流程。 |
| Cloud 登录后未上报设备时，设备列表为空是正确行为 | 通过 | 当前设计要求 Cloud 登录和设备上报分离；只有 `/cloud-api/devices/current` 成功后 Cloud DB 才出现绑定设备。 |
| Device report 写入 Cloud `devices` / `user_devices` | 通过 | Cloud handler 测试覆盖 `/cloud-api/devices/current` 的设备上报、用户设备绑定、设备列表过滤与 tunnel 公钥验签。 |
| 缺少 `cloud.gate_url` 时明确失败 | 通过 | Agent `/agent-api/cloud/connect` 在 `CloudGateURL` 为空时返回明确 bad request，而不是隐藏入口。 |
| develop / hybrid 配置不再含 `cloud.oauth.*` | 通过 | `configs/config.yaml`、`configs/config.develop.yaml`、`Taskfile.yml` 已移除 `cloud.oauth` / `TERMBRIDGE_CLOUD__OAUTH__REDIRECT_URL` 残留，只保留 `cloud.gate_url`。 |
| Cloud tunnel 认证继续基于本机私钥签名与 Cloud DB 公钥验签 | 通过 | Cloud tunnel 相关 handler 测试仍覆盖 Cloud DB public key 验签路径。 |

## Spec alignment

不适用。本功能当前没有同名 `docs/spec/20260705-agent-cloud-oauth-device-binding.md`；验证按已接受的 requirement 与当前用户纠正后的实现口径核对。

## Plan alignment

项目内没有同名 `docs/plan/20260705-agent-cloud-oauth-device-binding.md`。原计划中关于“Agent OAuth client / Cloud authorization server for device binding”的部分已经被用户后续明确纠正并废弃；实际实现按最新决策执行：Agent 连接 Cloud = device report。

已覆盖方向：

- 移除 Agent DB device identity 写入与 Agent-local `devices` schema 依赖。
- 将 Cloud binding summary 以非 token 摘要写入 `device.json`。
- 移除 `cloud-oauth-attempts.json` / `CloudOAuthAttemptStore`。
- 删除 Agent Cloud OAuth start/callback/state/token exchange。
- 删除 Cloud 设备绑定专用 authorize/token endpoint。
- 将连接云端收敛到 `/agent-api/cloud/connect` -> `/cloud-api/devices/current`。
- 移除旧 DTO/proto/前端 API/路由/config 中的 CloudOAuth 残留。
- 更新 app integration、develop config、Taskfile 与当前 requirement 文档。

## Actual diff summary

直接属于本需求的主要改动：

- Agent local identity：`internal/agent/application/user/device.go` 及相关测试。
- Agent connect handler：`internal/agent/api/handler/server.go`、`dto.go`、`cloud_binding_test.go`。
- Agent bootstrap/runtime boundary：`internal/agent/application/bootstrap/*`、`internal/agent/repository/task/state/*`。
- Agent migrations：`migrations/agent/mysql/202607020001_agent_runtime_schema.sql`、`migrations/agent/sqlite/202607020001_agent_runtime_schema.sql`。
- Cloud device binding：`internal/cloud/api/handler/*`、`internal/cloud/repository/user/device/repository.go`、Cloud identity migrations。
- API contract/proto/frontend：`proto/termbridge/cloud/v1/cloud.proto`、generated Go/TS proto、`web/src/features/*`、`web/src/store/gateway*`、相关 views。
- Integration/config/docs：`cmd/termbridge/app/app_test.go`、`configs/config.yaml`、`configs/config.develop.yaml`、`Taskfile.yml`、本 requirement / verification 文档。

本次工作树中也存在与本需求不完全同域的已修改文件。若准备提交，建议用户按交付边界拆分 review 或拆分 commit，避免把设备上报绑定收敛与其它主题混在一个提交里。

## Expected vs actual changed files

| 类别 | 预期 | 实际 |
|---|---|---|
| Agent local identity / DB boundary | 应修改 | 已修改，覆盖 device file persistence 与 runtime DB 不依赖 Agent-local devices。 |
| Agent Cloud connect | 应修改 | 已修改，删除 start/callback，保留 `/agent-api/cloud/connect` 设备上报入口。 |
| Cloud account auth vs device binding | 应修改 | 已修改，保留 Google 登录；删除设备绑定专用 OAuth authorize/token；device report 继续写 Cloud DB。 |
| Proto / frontend store / dashboard | 应修改 | 已修改，移除 CloudOAuth 类型/API/路由，Dashboard 表达 session、Cloud 登录、设备上报连接三步。 |
| develop config / Taskfile / requirement | 应修改 | 已修改，只保留 `cloud.gate_url`，删除 `cloud.oauth.*`。 |
| 无关过程文档和其它主题改动 | 不属于本需求核心 | 实际存在，需要提交前人工确认是否拆分。 |

## Acceptance checklist

- [x] Agent identity 主路径为 `device.json`。
- [x] 不保留旧 `agent.json` 自动迁移业务。
- [x] 不再写 Agent-local `devices` 表。
- [x] Agent runtime `device_id` 不再 FK 到本机 `devices`。
- [x] 不再持久化 Cloud OAuth attempt store。
- [x] 不再暴露 Agent Cloud OAuth start/callback。
- [x] 不再暴露 Cloud 设备绑定专用 OAuth authorize/token endpoint。
- [x] Agent 连接 Cloud 只调用 `/agent-api/cloud/connect`。
- [x] Agent 后端只把当前设备上报到 `/cloud-api/devices/current`。
- [x] Device 信息不进入 OAuth state/code/authorize/callback。
- [x] Cloud DB `devices` / `user_devices` 保留为账号设备绑定权威。
- [x] 不持久化 Cloud access/refresh token 到 `device.json`。
- [x] 不引入 PKCE。
- [x] 不再依赖 `capabilities.cloud_oauth_enabled`。
- [x] 配置层移除 `cloud.oauth.*`。

## Command results

| 命令 | 结果 | 说明 |
|---|---|---|
| `task proto` | Passed | 已重新生成 Go 与 frontend TypeScript proto 文件，旧 `CloudOAuth*` proto 类型被移除。 |
| `go test ./internal/agent/api/handler ./internal/cloud/api/handler ./cmd/termbridge/app ./internal/shared/infrastructure/config` | Passed | 聚焦 Agent connect、Cloud device binding、app integration 与 config 清理。 |
| `yarn --cwd web typecheck` | Failed | 当前失败集中在另一项“TS 接口转 proto 生成定义”的进行中改动：`../protocol/terminal` 缺少 `SessionSummary` / `WorkspaceSummary` 等导出、`ClientControlMessage` 新增 `nonce` 必填、`CreateSessionReq.workspace_id` 必填等。此前 `AgentDashboardView.vue` 的 `CloudSessionSummary | undefined` 问题已修正，第二次 typecheck 不再报告该文件。 |

## Missed or expanded scope

- Expanded：根据用户最新纠正，把需求和验证文档从“Agent Cloud OAuth 设备绑定”更新为“Agent Cloud 设备上报绑定”。
- Expanded：同步删除 `configs/config.yaml`、`configs/config.develop.yaml`、`Taskfile.yml` 中旧 `cloud.oauth` 配置。
- Expanded：同步删除 proto/generated/frontend types 中旧 `CloudOAuth*` 残留。
- Missed：未完成全量 frontend typecheck，因为另一项 TS/proto 类型收敛任务仍在进行中并导致大量与本连接云端改动无关的错误。

## Risks

1. 当前工作树 diff 较大且混有其它主题改动，提交前需要人工确认拆分边界。
2. `yarn --cwd web typecheck` 仍失败，但剩余错误属于 `protocol/terminal` 类型导出/控制消息协议/会话请求类型的另一项进行中迁移；不是 CloudOAuth 残留。
3. 浏览器 Cloud token 会被传给本机 Agent `/agent-api/cloud/connect` 用于本次上报；必须维持当前不持久化 token 的约束。
4. Cloud 登录和设备上报已明确解耦；手动验证时如果只完成 Google callback 而未点击连接当前 Agent，`/cloud-api/devices` 为空是正确结果。

## Incomplete items

- [ ] 等另一项 TS 接口到 proto 类型迁移完成后，补跑 `yarn --cwd web typecheck`。
- [ ] 提交前拆分或确认包含其它主题改动的 diff 边界。
- [ ] 如 CI 要求 lint/format，按项目当前工具链补跑对应检查。

## Conclusion

Agent Cloud 设备上报绑定收敛的核心业务验收通过：Agent 本机 identity 已收敛为文件权威，Agent runtime 不再依赖本机 `devices` 表，Cloud OAuth2 / Google 登录只负责 Cloud 账号登录，Agent 连接 Cloud 删除 OAuth start/authorize/callback/token 旧流，统一通过 `/agent-api/cloud/connect` 上报当前设备到 `/cloud-api/devices/current`，Cloud 继续用 `devices` / `user_devices` 作为账号设备绑定权威，tunnel 继续依赖设备私钥签名与 Cloud DB 公钥验签。
