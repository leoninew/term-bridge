# 会话页设备切换计划
最后修改时间: 2026-08-17 13:16:07

Review status: Accepted

## Requirement Basis

依据 `docs/requirement/20260817-session-device-switching.md`：本机 `/sessions` 需要支持从状态栏切换在线设备；本机浏览器请求必须经本地 Agent；进入 Cloud 工作台后仍应能持续切换设备。

## Implementation Steps

1. 在 Agent 的 Cloud service 与 HTTP client 中增加设备列表能力，端到端复用 `ListDevicesResp`。
2. 增加 `GET /api/cloud/devices`；要求 bearer token，并由 Agent 向 Cloud 请求设备列表。
3. 在本机前端 API 中封装该本地路径，不从 `/sessions` 直接使用 `cloudApiClient`。
4. 将设备菜单的状态、数据源和切换策略上移到 `SessionsPageShell`：
   - 菜单入口始终可用；前端直接绑定后端输出的设备列表，不补充、去重或排序当前 PC。
   - 本机持有 Cloud token 时通过 Agent 代理加载设备，并整页进入 Cloud 工作台。
   - 选择其他设备前若本机 Cloud session 缺失，则先通过本地 Agent 恢复 session，避免设备菜单可选但无导航结果。
   - 本机进入 Cloud 时携带本机设备 ID；Cloud 菜单标记该设备为“本地”，选择它时跳转 `local.publicUrl` 的 `/sessions`。
   - Cloud device ID 改变时重置旧终端标签和工作区数据，顺序刷新目标设备的工作区/会话与快捷方式；刷新期间显示 loading 并禁止重复切换。
   - Cloud 模式复用当前 Cloud 工作台的设备状态，在在线设备间继续切换。
5. 保持 `SessionStatusBar` 为展示组件，只发送“加载设备”和“切换设备”事件；`SessionWorkbench` 负责事件转发。
6. 补充 Agent 代理成功、凭据缺失时不出站、Cloud 上游失败返回 `502` 的测试，并修复发现的 `session_not_editable` 过期文案断言。

## Files To Change

- `internal/agent/application/user/cloud_service.go`
- `internal/agent/infrastructure/cloudapi/client.go`
- `internal/agent/infrastructure/cloudapi/client_test.go`
- `internal/agent/api/handler/server.go`
- `internal/agent/api/handler/cloud_binding_test.go`
- `internal/agent/api/handler/errors_test.go`
- `web/src/features/local/api.ts`
- `web/src/components/session/SessionStatusBar.vue`
- `web/src/components/session/SessionWorkbench.vue`
- `web/src/components/session/SessionsPageShell.vue`
- `web/src/features/sessions/deviceNavigation.ts`
- `web/src/features/sessions/deviceNavigation.test.ts`
- `web/src/i18n.ts`

## Verification Plan

- 运行 `go test ./cmd/... ./internal/agent/...`，覆盖 Agent 路由、Cloud client 和相关 CLI 组装。
- 运行 `yarn --cwd web typecheck`、`yarn --cwd web lint`、`yarn --cwd web format` 和 `yarn --cwd web test`。
- 确认本机状态栏和前端本地 API 未引用 `cloudApiClient`；`useCloudDevicesStore` 只在 Cloud runtime 分支读取或更新设备状态。
- 覆盖本地/Cloud session URL 构造及本机设备 ID query 传递，确保本地 URL 的路径前缀不丢失。
- 由用户手工验证：本机切到设备 A，再从设备 A 切到设备 B。

## Blockers

无。

## Assumptions

- Cloud 工作台的既有登录态、设备路由和 `cloud.publicUrl` 配置有效。
- 本机连接 Cloud 时，浏览器持有用于 Agent 代理设备列表的 Cloud token。

## Risks

- 本机切换到 Cloud 工作台是跨文档导航，依赖既有 Cloud 登录流程；该任务不承担跨站会话迁移。
- 缺少真实多设备环境时，自动化测试不能替代端到端终端 attach 验证。

## Rollback

- 删除状态栏设备菜单与 Agent `/api/cloud/devices` 路由，即可回到仅展示当前设备名的行为。
- 不涉及数据库、迁移或持久化状态变更。

## User Review Notes

- 用户要求本机页面所有请求只能访问本地服务，不能直连 Cloud。
- 用户要求进入其他 PC 后仍能继续切换设备，且设备菜单入口不依赖云端连接状态。
