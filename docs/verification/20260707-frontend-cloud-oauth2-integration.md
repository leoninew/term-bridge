# 前端接入 Cloud OAuth2 验证
最后修改时间: 2026-07-07 17:47:32

Review status: Accepted

## Requirement alignment

依据 `docs/requirement/20260707-frontend-cloud-oauth2-integration.md` 核对。用户后续进一步明确：Agent 侧 OAuth client 配置归属为 `agent.oauth.*`，Cloud 侧仍是 `cloud.oauth.clients[]` client registry。

| 验收项 | 结果 | 说明 |
| --- | --- | --- |
| Agent Dashboard Cloud 登录入口由前端 OAuth2 接入逻辑驱动，不跳本机 `/login?redirect=/agent/dashboard` | 通过 | Agent Dashboard 的 Cloud 登录入口调用前端 Cloud OAuth helper，跳 Cloud OAuth authorize route。 |
| Agent Dashboard 不直接调用 Google provider | 通过 | Agent 侧 Cloud 连接流程不调用 `authGoogleUrl()` / `startGoogleLogin()`；Google provider 仍只在 Cloud 登录/注册页面内部使用。 |
| 不新增或保留 Agent backend OAuth start/callback 路由 | 通过 | 本轮没有使用 `/agent-api/cloud-oauth/start` 或 `/agent-api/cloud-oauth/callback` 作为 flow controller。 |
| 前端 OAuth2 不通过新增 proto DTO 承载标准 OAuth2 redirect/token 协议 | 通过 | Agent frontend callback 使用本地 TS request `{ code }` 调 Agent API；未新增 OAuth token exchange proto DTO。 |
| 清理错误方向产生的无效 proto/generated contract | 通过 | 当前 Agent 连接 Cloud 的 OAuth code 提交没有依赖新增 proto DTO；已有 generated 变更属于当前工作区既有 contract 调整的一部分。 |
| Agent 后端设备上报发生在 token 获取之后 | 通过 | Agent frontend callback 将 authorization code 交给 Agent backend；Agent backend 使用 `golang.org/x/oauth2` 换取 Cloud access token 后再调用 Cloud device API。 |
| OAuth state/code/authorize/provider callback 不携带 device id/name/public_key | 通过 | Device 信息只在 token 获取后由 Agent backend 上报到 Cloud device endpoint，不进入 OAuth state/code/authorize/provider callback。 |
| Cloud token 不写入 Agent 本地 device state | 通过 | 连接摘要仍只保留非 token 信息；token 用于当前 device report。 |
| 使用 `public_url` / `PublicUrl`，不恢复 `gate_url` / `GateURL` | 通过 | 当前连接 Cloud 使用 Cloud `public_url`，未恢复 Gate 语义。 |
| 不执行 git 写操作 | 通过 | 只执行了 `git status` / `git diff` 等只读检查；未执行 add/commit/push/checkout/reset 等写操作。 |
| Agent 侧 OAuth client 配置放到 `agent.oauth.*` | 通过 | Root config、env key、README、`.env.example`、`scripts/run.py` 与配置测试均使用 `agent.oauth.*` / `TERMBRIDGE_AGENT__OAUTH__*`。 |
| Cloud 侧 OAuth client registry 是多 client 配置 | 通过 | Cloud 配置使用 `cloud.oauth.clients[]`，包含 `client_id`、`client_secret`、`redirect_url`、`scopes`。 |
| Cloud token endpoint 校验 client secret | 通过 | Cloud token handler 按注册 client 校验 `client_id + redirect_uri + client_secret`；Agent backend 使用 `oauth2.AuthStyleInParams` 发送 secret。 |

## Spec alignment

本任务采用 light / 轻量模式，没有同名 `docs/spec/20260707-frontend-cloud-oauth2-integration.md`。验证按已接受 requirement 和用户后续明确修正核对。

## Plan alignment

本任务采用 light / 轻量模式，没有同名 `docs/plan/20260707-frontend-cloud-oauth2-integration.md`。实际实现按 requirement 执行，并在实现过程中根据用户明确要求将 Agent OAuth client 配置归属修正为 `agent.oauth.*`。

## Actual diff summary

本次验证覆盖的核心改动：

- Agent frontend：新增/调整 Cloud OAuth helper 与 Agent callback route；callback 校验 state 后把 authorization code 提交给 Agent local API。
- Agent backend：`/agent-api/cloud/connect` 支持接收 OAuth authorization code；使用 `golang.org/x/oauth2` 的 `oauth2.Config.Exchange` 换取 Cloud token；使用 `oauth2.AuthStyleInParams` 配合 Cloud token endpoint 的 form secret 校验。
- Cloud backend：恢复/实现 Cloud 作为 OAuth2 Authorization Server 的 authorize/token 能力；Cloud OAuth config 改为 `cloud.oauth.clients[]` 多 client registry；token endpoint 校验 client secret；authorization code 一次性消费。
- 配置：Agent OAuth client 配置归属为 `agent.oauth.*`；Cloud OAuth client registry 归属为 `cloud.oauth.clients[]`；浏览器 runtime config 只注入 public OAuth client 信息，不注入 `client_secret`。
- 文档/示例：更新 `configs/config.yaml`、`configs/config.develop.yaml`、`.env.example`、`README.md`、`scripts/run.py` 中的 Agent OAuth 配置命名空间。
- 测试：补充/调整 config、Cloud OAuth token exchange、Agent callback/runtime config 等相关测试。

## Expected vs actual changed files

| 类别 | 预期 | 实际 |
| --- | --- | --- |
| Agent frontend OAuth 接入 | 需要改前端 helper、Agent callback route、dashboard 登录入口/API 调用 | 已改 `web/src/features/cloud/oauth.ts`、`web/src/views/agent/OAuthCallbackView.vue`、`web/src/features/agent/api.ts`、`web/src/router/index.ts`、dashboard 菜单相关代码。 |
| Agent backend token exchange / device report | 需要改 Agent cloud connect handler 和 wiring | 已改 `internal/agent/api/handler/server.go`、`errors.go`、Agent bootstrap config/server/router 相关代码。 |
| Cloud OAuth2 Authorization Server | 需要 Cloud authorize/token endpoint 与 client registry | 已改 `internal/cloud/api/handler/server.go`、`errors.go`、Cloud bootstrap config/server 和 Cloud handler tests。 |
| 配置 | Agent client config 应为 `agent.oauth.*`；Cloud registry 应为 `cloud.oauth.clients[]` | 已改 `internal/shared/infrastructure/config/config.go`、`config_test.go`、`configs/config.yaml`、`configs/config.develop.yaml`、`.env.example`、`README.md`、`scripts/run.py`。 |
| Proto/generated contract | 不应为标准 OAuth redirect/token 新增 DTO | 当前 Agent connect 提交 code 未依赖新增 OAuth token DTO；工作区仍有历史/并行 generated 变更，属于已有 contract 变更集合。 |
| 过程文档 | 需要 verification 文档 | 已创建本文档；同时将 requirement 中“前端获得 Cloud token”的旧表述修正为“前端获得 code，Agent backend 使用 oauth2.Exchange 换 token”。 |

注意：当前工作区还包含此前同一会话内的 view 拆分、frontend non-session rewrite requirement、generated proto 文件等较大范围变更；这些不是本文单独验证的新增扩散动作，但会影响最终提交边界，提交前建议按功能拆分审视。

## Acceptance checklist

- [x] Agent Dashboard Cloud 登录不再跳本机 login。
- [x] Agent Dashboard 不直接认识 Google provider。
- [x] Agent frontend 只面向 Cloud OAuth authorize endpoint。
- [x] Agent frontend callback 校验 state。
- [x] Agent frontend 不在浏览器中执行 token exchange。
- [x] Agent backend 使用 `golang.org/x/oauth2` 交换 authorization code。
- [x] Agent backend 使用 confidential `client_secret`，且 secret 不注入 browser runtime config。
- [x] Cloud OAuth token endpoint 校验 client secret。
- [x] Agent OAuth client config 使用 `agent.oauth.*`。
- [x] Cloud OAuth registry 使用 `cloud.oauth.clients[]`。
- [x] Device id/name/public_key 不进入 OAuth state/code/authorize/provider callback。
- [x] Device report 发生在 token 获取之后。
- [x] 未执行 git 写操作。

## Test results

已运行并通过：

```text
go test ./internal/shared/infrastructure/config ./internal/cloud/api/handler ./internal/cloud/application/bootstrap ./internal/agent/api/handler ./internal/agent/application/bootstrap ./cmd/termbridge/app
```

结果：

```text
ok  	gitee.com/leoninew/TermBridge-go/internal/shared/infrastructure/config	(cached)
ok  	gitee.com/leoninew/TermBridge-go/internal/cloud/api/handler	(cached)
?   	gitee.com/leoninew/TermBridge-go/internal/cloud/application/bootstrap	[no test files]
ok  	gitee.com/leoninew/TermBridge-go/internal/agent/api/handler	(cached)
ok  	gitee.com/leoninew/TermBridge-go/internal/agent/application/bootstrap	(cached)
ok  	gitee.com/leoninew/TermBridge-go/cmd/termbridge/app	(cached)
```

已运行并通过：

```text
yarn typecheck
```

结果：

```text
yarn run v1.22.22
$ vue-tsc --noEmit
Done in 2.48s.
```

另外在迁移 `agent.oauth.*` 后单独运行并通过：

```text
go test ./internal/shared/infrastructure/config ./cmd/termbridge/app
```

结果：

```text
ok  	gitee.com/leoninew/TermBridge-go/internal/shared/infrastructure/config	1.359s
ok  	gitee.com/leoninew/TermBridge-go/cmd/termbridge/app	4.459s
```

## Missed or expanded scope

- 未运行完整 `task check` 或完整 `task test`。
- 未做打包运行实测。
- 未执行任何 git 写操作。
- 当前工作区包含本任务之前/同会话内的较大范围改动，最终提交前建议按“Agent frontend Cloud OAuth2 integration”、“frontend view split / non-session rewrite”、“generated contract changes”等边界拆分审视。

## Risks

- Cloud OAuth client registry 的真实生产 client secret 需要由部署环境提供；示例中的 `agent-secret` 只适合本地开发。
- 当前验证命令为 targeted test 和 frontend typecheck；如果准备合入，仍建议在提交前运行项目完整检查。
- 工作区存在 staged 与 unstaged 混合状态，虽然本轮未执行 git 写操作，但提交前需要人工确认最终 diff 边界。

## Incomplete items

- 无本任务内阻塞项。
- 未覆盖完整构建/打包实测，属于后续发布验证范围。

## Conclusion

通过。实现已按最新要求收敛为：Agent frontend 发起 Cloud OAuth2 authorize，Cloud 作为 Authorization Server 完成授权，Agent frontend callback 校验 state 并提交 code，Agent backend 使用 `golang.org/x/oauth2` 和 `agent.oauth.*` confidential client 配置换取 Cloud token，随后上报当前设备；Cloud backend 使用 `cloud.oauth.clients[]` registry 并校验 client secret。