# Agent Cloud 设备上报绑定收敛需求
最后修改时间: 2026-07-05 22:17:29

Review status: Accepted

## Background

当前 Agent / Cloud / Hybrid 模式已经区分了 Agent API 与 Cloud API。继续推进后，边界进一步收敛为：Cloud OAuth2 / Google 登录只负责 Cloud 账号登录，Agent 连接 Cloud 不再是 OAuth2 client flow，而是使用浏览器已经取得的 Cloud 账号 token 上报当前 Agent 设备。

已确认的现状与纠正：

1. Agent 本机设备身份曾同时存在于本地文件与 `agent.db.devices`，本次收敛后权威来源只保留本地文件：
   - 文件：`data/device.json`、`data/private_key.pem`、`data/public_key.pem`。
   - DB：不再调用 `UpsertLocalDevice` 写入 Agent 本机设备身份。
2. Agent runtime schema 中，`workspaces`、`sessions`、`session_runs` 保留 `device_id` 数据归属字段，但不再通过外键依赖本机 `devices` 表。
3. Cloud DB 中 `devices` 与 `user_devices` 是 Cloud 账号绑定和权限边界，不能被本地 identity 文件替代。
4. Cloud 账号登录仍属于 Cloud：浏览器通过 Cloud Google/OAuth2 登录取得 Cloud token。
5. Agent 连接 Cloud 不是 OAuth2 start / authorize / callback 流程；它只是设备上报：Agent 后端用浏览器提供的 Cloud token 调 Cloud `/cloud-api/devices/current`。
6. 本机 device id/name/public_key 不进入 OAuth state/code/authorize URL；device 信息只出现在设备上报请求体里。
7. 不持久化 Cloud access/refresh token 到 `device.json` 或其它 Agent 本地文件；Agent 本地只保存非 token 的 Cloud binding summary。
8. `cloud-oauth-attempts.json`、Agent-side Cloud OAuth start/callback、Cloud-side device-binding OAuth authorize/token endpoint 都属于旧设计残留，应移除。
9. 前端不再通过 `capabilities.cloud_oauth_enabled` 控制入口；Agent Dashboard 只表达三件事：进入 session、登录 Cloud 账号、登录后连接当前 Agent 到云端。

## Goal

1. Agent 本机设备身份只由本地 identity 文件负责，文件名为 `data/device.json`。
2. 移除 `agent.db` 中作为本机设备身份存储的 `devices` 表与相关 repository 写入逻辑。
3. Agent runtime 业务表可以保留 `device_id` 字段作为数据归属字段，但不得依赖本机 `devices` 表外键作为身份权威。
4. 保留 Cloud 账号登录能力：Cloud `/cloud-api/auth/google` 与 `/cloud-api/auth/google/callback` 继续只处理 Cloud 用户登录。
5. 删除 Agent 侧 Cloud OAuth 连接流程：
   - 不保留 `/agent-api/cloud-oauth/start`。
   - 不保留 `/agent-api/cloud-oauth/callback`。
   - 不保留 Agent Cloud OAuth state/code/token exchange 实现。
6. 删除 Cloud 侧专用于 Agent 设备绑定的 OAuth 服务端残留：
   - 不保留 `/cloud-api/cloud-oauth/authorize`。
   - 不保留 `/cloud-api/oauth2/token`。
   - 不保留 Cloud OAuth authorization code store。
7. Agent 连接 Cloud 统一收敛为设备上报：
   - 前端在已登录 Cloud 账号后调用 Agent `/agent-api/cloud/connect`。
   - 请求体携带浏览器当前 Cloud token。
   - Agent 读取本机 `device.json` 与 public key，向 `cloud.gate_url + /cloud-api/devices/current` POST 当前设备。
   - Cloud 用 Cloud token 识别用户，并写入 `devices` / `user_devices`。
8. 移除 `cloud-oauth-attempts.json` 及其相关实现，不再用本地 JSON 文件保存 Cloud OAuth attempt。
9. 移除 `device_binding_codes` 及其相关实现；设备绑定只通过明确的设备上报接口完成。
10. 移除 `capabilities.cloud_oauth_enabled` 这类前端能力开关；入口由页面业务流程表达，配置缺失时由 API 返回明确错误。
11. 本地 develop / hybrid 配置只需要 `cloud.gate_url` 指向 Cloud Gate；不再配置 `cloud.oauth.*`。
12. Cloud tunnel 认证继续基于 Agent 本地私钥签名与 Cloud DB 公钥验签。

## Non-goal

1. 不移除 Cloud DB 的 `devices` / `user_devices`。
2. 不改变 Agent token 与 Cloud token 分离原则：浏览器侧仍分别使用 `termbridge_agent_token` 与 `termbridge_cloud_token`。
3. 不把 Cloud 用户 token 长期写入 Agent 本地文件或浏览器 Agent token 存储。
4. 不在 Cloud 账号登录 callback 阶段写入设备绑定数据；设备绑定必须由独立上报接口完成。
5. 不引入 PKCE 或自建 OAuth authorization server；本任务已经明确不需要 Agent Cloud OAuth flow。
6. 不引入多个本机 device identity 管理能力；当前仍按“一个本机 Agent 进程 = 一个本机设备身份”处理。
7. 不把 Agent 本地 `device_id` 从所有业务数据中完全移除；该字段仍可用于本机 runtime 数据归属。

## User scenarios

1. 用户进入 Agent Dashboard。
2. 用户进入本机 session 页面，确认本机 Agent 可用。
3. 用户登录 Cloud 账号；该步骤使用 Cloud Google/OAuth2 登录，只产生 Cloud 账号登录态。
4. Cloud 登录完成后，Agent Dashboard 的“连接当前 Agent 到云端”入口可用。
5. 用户点击连接，前端用 Agent token 调 `/agent-api/cloud/connect`，请求体携带当前 Cloud token。
6. Agent 后端读取当前本机 device id/name/public_key，并 POST 到 Cloud `/cloud-api/devices/current`。
7. Cloud API 根据 Cloud token 将设备写入 `cloud.db.devices`，并将用户-设备关系写入 `cloud.db.user_devices`。
8. Agent 后续建立 tunnel 时使用本机私钥签名；Cloud 使用 `cloud.db.devices.public_key` 验签，并通过 `user_devices` 判断用户可见和可操作设备。

## Acceptance

1. 新建 Agent 本机 identity 文件路径为 `data/device.json`，不再以 `data/agent.json` 作为主路径，也不引入旧文件自动迁移业务。
2. Agent 启动不再为了本机设备身份调用 `agentdb` device repository 写入 `agent.db.devices`。
3. Agent runtime 业务数据仍能创建 workspace/session/run，并能通过 `device_id` 归属到当前本机设备；如保留 `device_id` 字段，不再依赖本机 `devices` 外键。
4. 前后端不再暴露 Agent Cloud OAuth start/callback：`/agent-api/cloud-oauth/start` 与 `/agent-api/cloud-oauth/callback` 不存在。
5. Cloud 不再暴露设备绑定专用 OAuth endpoint：`/cloud-api/cloud-oauth/authorize` 与 `/cloud-api/oauth2/token` 不存在。
6. 代码与当前配置不再引用 `cloud.oauth.*`、`CloudOAuth*`、`cloud-oauth`、`/oauth2/authorize`、`/cloud-api/oauth2/token` 等旧连接云端业务。
7. Agent 连接 Cloud 只通过 `/agent-api/cloud/connect`，且该 endpoint 使用请求体中的 Cloud token 上报当前设备到 `/cloud-api/devices/current`。
8. Device 信息不进入 Cloud 账号登录 URL、OAuth state、authorization code 或 callback；device 信息只在 `/cloud-api/devices/current` 上报请求中出现。
9. `device.json` 可以保存非 token 的 Cloud binding summary，但不得包含 Cloud access token、refresh token、bearer token 或浏览器 Cloud token。
10. 前端和后端不再暴露或依赖 `capabilities.cloud_oauth_enabled` 控制 Cloud 连接入口。
11. Cloud 账号登录成功但未执行设备上报时，`GET /cloud-api/devices` 返回空列表是正确行为。
12. 执行设备上报后，Cloud DB 中必须出现对应记录：
    - `devices.id = 本机 device id`
    - `devices.public_key = 本机 public key`
    - `user_devices.user_id = 当前 Cloud 用户 id`
    - `user_devices.device_id = 本机 device id`
13. 如果 `cloud.gate_url` 缺失，Agent `/agent-api/cloud/connect` 返回明确错误，而不是通过 capability 隐藏入口。
14. 本地 develop / hybrid 配置应支持完整链路：Cloud 账号登录、Agent device report、Cloud 设备落库、Agent tunnel 验签。
15. Cloud tunnel 认证继续基于设备私钥签名和 Cloud DB 公钥验签，不使用浏览器 Agent token 或 Cloud browser token。

## Open questions

暂无需要用户确认的未决事项。用户已明确纠正：Agent 连接 Cloud 就是设备上报，不再保留 Agent Cloud OAuth flow。

## Decisions

1. Agent 本机设备身份的权威来源是本地文件，不是 `agent.db.devices`。
2. Cloud 账号设备绑定的权威来源是 Cloud DB 的 `devices` 与 `user_devices`。
3. 保留 `data/public_key.pem`；运行时仍可从 private key 推导 public key，但文件保留便于排查。
4. 不保留 `LegacyDeviceIdentityFileName`，也不实现旧 `data/agent.json` 到 `data/device.json` 的自动重命名迁移；当前开发阶段按 `data/device.json` 命名落盘。
5. 本地库由用户重建，本次可直接就地更新 agent migration 脚本，不需要兼容旧 `agent.db.devices` 外键 schema。
6. OAuth2 只属于 Cloud 账号登录，不属于 Agent 连接 Cloud。
7. Agent 连接 Cloud 不再生成 state，不再 callback，不再 exchange token，只做 device report。
8. 移除 `cloud-oauth-attempts.json` 文件态 attempt store。
9. 移除 `capabilities.cloud_oauth_enabled`，Agent Cloud 连接入口不再由 capability 开关控制。
10. 本地 develop / hybrid 只保留 `cloud.gate_url` 作为 Agent 上报设备时的 Cloud Gate 地址。
11. Agent 已绑定 Cloud 后需要继续在 `data/device.json` 更新非 token 的绑定摘要。

## Risk

1. Agent DB 当前 schema 曾存在 `devices` 外键依赖，移除本地 device 表涉及 migration 和 store 查询边界，不能只删除 repository 调用。
2. 如果代码、fixture 或文档继续引用旧 `data/agent.json`，会把开发阶段文件命名误写成长期业务兼容逻辑；当前实现不得新增旧文件自动迁移路径。
3. 移除 Agent Cloud OAuth flow 后，旧 `/cloud/oauth/start` / callback / authorize 页面、路由、API 类型如果残留，会误导为“连接云端还要 auth/callback”。
4. Cloud DB 落库必须用真实 Cloud 用户上下文，不能误用 Agent local token，也不能把 Cloud browser token、Agent 本机 token 和设备上报授权上下文混用。
5. 浏览器会把 Cloud token 发给本机 Agent `/agent-api/cloud/connect`；Agent 不得持久化该 token，只能用于本次设备上报请求。

## User review notes

- 用户要求先讨论清楚，不接受“可能”式分析后直接实现。
- 用户明确提出：
  1. 移除 `agent.db` 里的 device，当前开发阶段直接使用 `data/device.json`，不把 `agent.json` 到 `device.json` 的改名写成业务迁移。
  2. 移除 `AuthCapabilities` / `capabilities` 业务概念。
  3. 移除 `cloud-oauth-attempts.json` 相关实现。
  4. Agent Dashboard 只表达：进入 session、登录 Cloud 账号、登录后连接当前 Agent 到云端。
  5. Agent 连接 Cloud 就是设备上报，不再走 OAuth2 start / authorize / callback。
  6. `CloudConnectAuthorizeView.vue` / callback / start 这批页面是旧设计残留，不应该再参与“连接云端”。
