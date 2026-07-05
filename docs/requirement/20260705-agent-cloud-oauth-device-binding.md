# Agent Cloud OAuth 设备绑定收敛需求
最后修改时间: 2026-07-05 16:51:16

Review status: Accepted

## Background

当前 Agent / Cloud / Hybrid 模式已经区分了 Agent API 与 Cloud API，并引入了 Agent 本机登录与 Cloud 账号 OAuth 连接流程。但在继续实现前，需要先收敛 Agent 设备身份、Cloud OAuth2 授权、设备上报和账号绑定之间的边界。

已确认的现状：

1. Agent 本机设备身份曾同时存在于本地文件与 `agent.db.devices`，本次收敛后权威来源只保留本地文件：
   - 文件：`data/device.json`、`data/private_key.pem`、`data/public_key.pem`。
   - DB：不再调用 `UpsertLocalDevice` 写入 Agent 本机设备身份。
2. Agent runtime schema 中，`workspaces`、`sessions`、`session_runs` 保留 `device_id` 数据归属字段，但不再通过外键依赖本机 `devices` 表。
3. Cloud DB 中 `devices` 与 `user_devices` 是 Cloud 账号绑定和权限边界，不能被本地 identity 文件替代。
4. OAuth2 协议本身不需要本机设备信息；本机设备信息只属于 TermBridge 的“设备上报 / Cloud 账号绑定”业务。
5. 当前本地 develop 配置没有设置 `cloud.gate_url`，导致 Agent Cloud OAuth capability 被判定为不可用，正常 UI 不会进入 Cloud 设备绑定链路。
6. 当前存在 `cloud-oauth-attempts.json` 文件态 attempt store，其职责与 OAuth2 标准 state/session 语义和本地状态收敛方向不一致。
7. 当前前端通过 `capabilities.cloud_oauth_enabled` 控制 Agent Dashboard 的 Cloud 连接入口，但产品已经显式区分 Agent / Cloud 页面，能力开关会扩大配置分支和 UI 复杂度。
8. 当前实现需要充分审视 OAuth2 正确性：Cloud 应作为 authorization server 持有 authorize/code/token 能力；Agent 作为 client 不应本地写入、持久化或伪造 OAuth2 code，也不应把设备绑定流程状态混入 OAuth2 state/code。
9. 当前没有 PKCE 相关实现，本任务不泛化引入 PKCE；只在现有能力范围内纠正 OAuth2 与设备绑定职责边界。

## Goal

1. Agent 本机设备身份只由本地 identity 文件负责，文件名为 `data/device.json`。
2. 移除 `agent.db` 中作为本机设备身份存储的 `devices` 表与相关 repository 写入逻辑。
3. Agent runtime 业务表可以保留 `device_id` 字段作为数据归属字段，但不得依赖本机 `devices` 表外键作为身份权威。
4. OAuth2 流程按更标准的职责拆分并纠正当前实现：
   - Cloud 是 authorization server，负责 authorize、authorization code、token 签发等 OAuth2 服务端能力。
   - Agent 是 OAuth2 client，只发起授权并接收 Cloud 回调结果。
   - Agent 不在本地写入或持久化 OAuth2 state/code，不生成 OAuth2 code，不把 binding code 伪装成 OAuth2 authorization code。
   - 当前任务不引入 PKCE 等尚未存在的泛化能力；只纠正当前 OAuth2 与设备绑定混杂的问题。
   - OAuth2 authorize / callback 只负责确认 Cloud 用户授权上下文。
   - 本机 device id/name/public_key 不进入 OAuth2 state/code。
   - 设备上报在 OAuth2 授权完成后由 Agent 后端显式调用 Cloud API 完成。
5. 移除 `cloud-oauth-attempts.json` 及其相关实现，不再用本地 JSON 文件保存 Cloud OAuth attempt。
6. 移除 `device_binding_codes` 及其相关实现；OAuth2 和设备上报/绑定是两码事，不能通过 binding code 把两者搅在一起。
7. 移除 `/cloud-api/cloud-oauth/exchange`：OAuth2 授权完成后不应再通过独立的 cloud-oauth exchange 用 code 换业务 token；设备绑定应走明确的设备上报接口。
8. 移除 `capabilities.cloud_oauth_enabled` 这类前端能力开关；Agent 页面按明确路由和配置直接发起 OAuth2，不通过 capability 决定是否展示入口。
9. Cloud 端继续以 `devices` + `user_devices` 表作为 Cloud 账号与设备绑定的权威数据。
10. 本地开发 hybrid 模式下，Cloud Gate URL / OAuth authorize 入口需要明确可用，使 Agent 发起 Cloud OAuth 后能够实际完成 Cloud DB 落库。

## Non-goal

1. 不移除 Cloud DB 的 `devices` / `user_devices`。
2. 不改变 Agent token 与 Cloud token 分离原则：浏览器侧仍分别使用 `termbridge_agent_token` 与 `termbridge_cloud_token`。
3. 不把 Cloud 用户 token 长期写入 Agent 本地文件或浏览器 Agent token 存储。
4. 不在 OAuth2 authorize 阶段写入 Cloud `devices` / `user_devices`；设备绑定必须由独立上报接口完成。
5. 不引入 PKCE 等当前不存在的 OAuth2 扩展能力；本任务聚焦纠正现有 OAuth2 与设备绑定职责混杂问题。
6. 不引入多个本机 device identity 管理能力；当前仍按“一个本机 Agent 进程 = 一个本机设备身份”处理。
7. 不把 Agent 本地 `device_id` 从所有业务数据中完全移除；是否保留字段用于数据归属，由实现阶段基于 schema 和 store 依赖处理。

## User scenarios

1. 用户在 Agent 页面显式发起连接 Cloud。
2. 浏览器进入 Cloud OAuth authorize 页面，使用当前 Cloud 用户登录态完成授权；OAuth2 授权本身不写设备绑定数据。
3. Agent 收到 Cloud OAuth 授权完成回调后，从 `data/device.json` 与 key 文件读取本机设备身份。
4. Agent 后端调用 Cloud API 上报当前设备：device id、device name、public key。
5. Cloud API 根据 Cloud 用户 token 将设备写入 `cloud.db.devices`，并将用户-设备关系写入 `cloud.db.user_devices`。
6. Agent 后续建立 tunnel 时使用本机私钥签名；Cloud 使用 `cloud.db.devices.public_key` 验签，并通过 `user_devices` 判断用户可见和可操作设备。

## Acceptance

1. 新建 Agent 本机 identity 文件路径为 `data/device.json`，不再以 `data/agent.json` 作为主路径，也不引入旧文件自动迁移业务。
2. Agent 启动不再为了本机设备身份调用 `agentdb` device repository 写入 `agent.db.devices`。
3. Agent runtime 业务数据仍能创建 workspace/session/run，并能通过 `device_id` 归属到当前本机设备；如保留 `device_id` 字段，不再依赖本机 `devices` 外键。
4. Cloud OAuth start / callback 的本地流程状态不再写入 `data/auth/cloud-oauth-attempts.json`。
5. OAuth2 state/code/token 职责必须归 Cloud authorization server；Agent 端不得本地持久化 state/code，不得生成 OAuth2 authorization code，不得用设备绑定 code 替代 OAuth2 code。
6. OAuth2 state / callback 校验中不携带 device id/name；device 信息只在 OAuth 授权完成后的 `/cloud-api/devices/current` 上报请求中出现。
7. 前端和后端不再暴露或依赖 `capabilities.cloud_oauth_enabled` 控制 Cloud 连接入口。
8. Agent 发起 Cloud OAuth 后，如果 Cloud 用户授权成功，Cloud DB 中必须出现对应记录：
   - `devices.id = 本机 device id`
   - `devices.public_key = 本机 public key`
   - `user_devices.user_id = 授权 Cloud 用户 id`
   - `user_devices.device_id = 本机 device id`
9. 如果 Cloud OAuth 必要配置缺失，失败应在发起或回调时以明确错误暴露，而不是通过 capability 隐藏入口。
10. 本地 develop / hybrid 配置应支持完整链路：Agent 页面发起 OAuth、Cloud 页面授权、Agent 回调、Cloud 设备落库、Agent tunnel 验签。
11. Cloud tunnel 认证继续基于设备私钥签名和 Cloud DB 公钥验签，不使用浏览器 Agent token 或 Cloud browser token。

## Open questions

暂无需要用户确认的未决事项。当前约定已经足够进入后续实现讨论；实现前如发现 `data/device.json` 持久化字段过多，应先列出实际字段再决定是否拆分。

## Decisions

1. Agent 本机设备身份的权威来源是本地文件，不是 `agent.db.devices`。
2. Cloud 账号设备绑定的权威来源是 Cloud DB 的 `devices` 与 `user_devices`。
3. 保留 `data/public_key.pem`；运行时仍可从 private key 推导 public key，但文件保留便于排查。
4. 不保留 `LegacyDeviceIdentityFileName`，也不实现旧 `data/agent.json` 到 `data/device.json` 的自动重命名迁移；当前开发阶段按 `data/device.json` 命名落盘。
5. 本地库由用户重建，本次可直接就地更新 agent migration 脚本，不需要兼容旧 `agent.db.devices` 外键 schema。
6. OAuth2 不携带 device 信息；device 信息只通过设备上报接口进入 Cloud。
7. OAuth2 正确性必须作为本任务审视重点：Cloud 持有 authorization code/token 等服务端能力；Agent 不本地写 state/code、不生成 authorization code、不把绑定 code 当 OAuth2 code。
8. 移除 `cloud-oauth-attempts.json` 文件态 attempt store；不为 OAuth2 state 设计跨 Agent 重启恢复能力。
9. 移除 `capabilities.cloud_oauth_enabled`，Agent Cloud 连接入口不再由 capability 开关控制。
10. 本地 develop / hybrid 必须配置出可走通的 Cloud Gate / OAuth authorize 链路。
11. Agent 已绑定 Cloud 后需要继续在 `data/device.json` 更新相关摘要；如果字段变多，先检查实际数据量和职责，再决定是否拆出独立文件。

## Risk

1. Agent DB 当前 schema 存在 `devices` 外键依赖，移除本地 device 表涉及 migration 和 store 查询边界，不能只删除 repository 调用。
2. 如果代码、fixture 或文档继续引用旧 `data/agent.json`，会把开发阶段文件命名误写成长期业务兼容逻辑；当前实现不得新增旧文件自动迁移路径。
3. 移除 `cloud-oauth-attempts.json` 后，未完成 OAuth 流程在 Agent 重启后不恢复；这是本任务明确接受的行为，不再设计额外持久化策略。
4. 移除 `capabilities.cloud_oauth_enabled` 后，配置缺失会从“入口不可见/disabled”变为“点击后明确失败”；需要统一错误体验。
5. Cloud DB 落库必须用真实 Cloud 用户上下文，不能误用 Agent local token，也不能把 Cloud browser token、Agent 本机 token 和设备上报授权上下文混用。

## User review notes

- 用户要求先讨论清楚，不接受“可能”式分析后直接实现。
- 用户明确提出：
  1. 移除 `agent.db` 里的 device，当前开发阶段直接使用 `data/device.json`，不把 `agent.json` 到 `device.json` 的改名写成业务迁移。
  2. OAuth2 实现要标准一些，device 信息要移除，交由上报处理。
  3. 移除 `cloud-oauth-attempts.json` 相关实现。
  4. 移除/收敛 `capabilities.cloud_oauth_enabled`，显式 Agent / Cloud 页面下不再需要这类 capability 开关。
  5. 本任务必须充分审视并纠正 OAuth2 实现正确性；authorization code/token 应属于 Cloud authorization server 能力，不应由 Agent 本地文件态实现，且不引入当前不存在的 PKCE 泛化设计。
