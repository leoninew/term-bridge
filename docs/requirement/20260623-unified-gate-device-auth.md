# 统一 Gate / Device / Auth 工作台需求

最后修改时间: 2026-06-23 13:26:28

Review status: Accepted

## Background

当前前端已引入 vue-router 和 Pinia 基础设施，页面规划为：

- `/`：未来认证入口或登录后跳转
- `/sessions`：会话工作台
- `/settings`：设置页
- `/help`：帮助页

当前后端 `termbridge serve` 启动后已经是一体化服务：

```text
termbridge serve
  ├─ HTTP server
  │   ├─ localapi      /api/...
  │   └─ gatewayapi    /api/gateway/...
  └─ agent client
      └─ connects to configured Gate URL
```

也就是说，backend / gateway / agent connector 在本地 serve 进程中已经同时存在。长期目标不是保留“本地模式”和“Gateway 模式”两套产品流程，而是统一为：

```text
Browser -> Gate -> Device Agent -> Workspace -> Session -> Terminal
```

本地使用时，Agent 连接的 Gate 配置成自己；接入云端时，Agent 连接的 Gate 配置成远程云端地址。

相关设计整理见：

- `docs/design/20260623-unified-gate-device-model.md`

## Goal

1. 移除前端产品概念中的 `/gateway` 独立模式。
2. 以 `/sessions` 作为统一会话工作台，在现有较完整交互基础上继续加法演进。
3. 后端引入 Device 概念，使本地 self-connected Gate 和云端 Gate 使用同一套资源模型。
4. 启动时检查 `.termbridge.yaml` 中的凭据和身份信息，并通过 viper 读取，包括：
   - 设备名
   - 用户账号
   - 连接 Gate 所需凭据
5. 如果本地配置缺失，则自动生成默认凭据并写入 `.termbridge.yaml`：账号使用当前操作系统用户名，密码随机生成并满足常规强度要求。
6. Agent 启动后将设备和必要身份信息上报到配置的 Gate。
7. 页面打开后必须经过用户账号登录，并需要选取设备后进入 `/sessions` 工作台。
8. 登录和设备选择完成后，`/sessions` 左面板底部展示当前设备名。
9. 保持当前 `/sessions` 页面主体布局和主要操作方式不变，避免重新设计工作台交互。

## Non-goal

1. 本阶段不重做 `/sessions` 的主工作台布局。
2. 本阶段不引入新的复杂导航结构。
3. 本阶段不要求一次性完成云端 SaaS 多租户能力。
4. 本阶段不要求彻底删除后端 localapi；localapi 可以先作为兼容层存在。
5. 本阶段不要求完整实现生产级账户体系、OAuth、SSO 或团队权限模型。
6. 本阶段不要求实现复杂设备管理页面；设备选择可以先作为登录后的必要选择流程。
7. 本阶段不要求改变终端组件 `TerminalView` 的交互形态。
8. 本阶段不要求改变当前 session tab、workspace tree、history 展示、终端输入输出的基本用户操作方式。

## User scenarios

### Scenario 1：本地单机使用

1. 用户启动 `termbridge serve`。
2. 程序检查本地配置或状态文件。
3. 如果没有用户账号、设备名或连接凭据，则自动生成并保存到 `.termbridge.yaml`。
4. 账号默认使用当前操作系统用户名，密码随机生成并满足常规强度要求。
5. 首次生成登录凭据后，程序在控制台打印提示信息，帮助用户完成登录。
6. Agent 使用本地配置连接本机 Gate。
7. 用户打开 Web 页面。
8. 页面要求用户使用本地配置中的用户账号登录。
9. 登录后页面要求用户选择设备。
10. 用户选择当前本机设备。
11. 页面进入 `/sessions`。
12. 左面板底部展示当前设备名。
13. 用户使用当前工作台方式查看 workspace、打开 session、连接 terminal。

### Scenario 2：接入云端 Gate

1. 用户在本地配置中将 `agent.server_url` 设置为云端 Gate 地址。
2. 用户启动 `termbridge serve` 或本地 Agent 连接器。
3. 本地 Agent 使用 `.termbridge.yaml` 中的凭据连接云端 Gate，并上报设备名和身份信息。
4. 用户打开云端 Web 页面。
5. 用户登录云端 Gate。
6. 用户选择已上报的本地设备。
7. 用户进入 `/sessions` 工作台。
8. 工作台通过云端 Gate 连接本地设备上的 workspace / session / terminal。

### Scenario 3：配置缺失

1. 用户首次启动程序。
2. 程序发现本地配置缺少设备名、用户账号或连接凭据。
3. 程序将账号设为当前操作系统用户名，随机生成满足常规强度要求的密码，并补齐设备名和连接凭据。
4. 程序将生成结果写入 `.termbridge.yaml`，后续通过 viper 读取。
5. 首次生成登录凭据后，程序在控制台打印提示信息。
6. 程序继续启动并尝试连接 Gate。
7. 用户无需手动编辑配置即可进入最小可用流程。

### Scenario 4：设备未就绪

1. 用户打开页面并完成登录。
2. Gate 尚未收到任何设备上报，或目标设备离线。
3. 页面显示设备选择/设备未就绪状态。
4. 页面不应进入空白或错误崩溃状态。
5. 设备上线后，用户可以选择设备并进入 `/sessions`。

## Acceptance

### 前端

- [ ] `/gateway` 不再作为长期产品路由暴露。
- [ ] `/sessions` 是统一会话工作台入口。
- [ ] 页面打开后进入登录流程，而不是直接展示工作台。
- [ ] 登录完成后进入设备选择流程。
- [ ] 选择设备后展示现有 `/sessions` 工作台。
- [ ] `/sessions` 主体布局、tab、workspace/session tree、terminal 操作方式保持基本不变。
- [ ] `/sessions` 左面板底部展示当前设备名。
- [ ] 前端不再以 local backend / gateway backend 作为产品级分支，而是围绕 selected device 加载数据。

### 后端

- [ ] 后端存在 Device 概念，能够表示本地 self-connected device 和未来云端 device。
- [ ] `serve` 启动时能够通过 viper 检查 `.termbridge.yaml` 中的本地设备名、用户账号和 Gate 连接凭据。
- [ ] 配置缺失时能够自动生成并写入 `.termbridge.yaml`。
- [ ] Agent 连接 Gate 时上报设备信息。
- [ ] Gate 能够列出可用设备，供前端选择。
- [ ] 本地 self-connected Gate 流程可用：Agent 连接自己，前端通过 Gate 选择本机设备。

### 认证

- [ ] Browser 访问工作台前需要登录。
- [ ] 登录使用 `.termbridge.yaml` 中的用户账号，缺失时账号使用当前操作系统用户名自动生成。
- [ ] 缺失密码时随机生成满足常规强度要求的密码，并在首次生成时于控制台打印提示信息。
- [ ] Agent 连接 Gate 和 Browser 登录使用同一套凭据；后续权限系统再细分权限。
- [ ] 认证失败或设备凭据无效时，有明确错误反馈。

### 兼容与迁移

- [ ] 现有 `/sessions` 工作台的核心能力不因引入设备选择和登录而退化。
- [ ] 后端 localapi 可以暂时保留，但不应作为前端长期主路径。
- [ ] Device 成为 workspace / session 持久化数据的一部分。
- [ ] runtime state 目录结构按 Device 重新设计。
- [ ] 本次不要求向后兼容旧 state 目录结构或旧 workspace/session 数据。

## Decisions

1. 使用严格模式 / strict 推进该变更。
2. 前端长期不保留 `/gateway` 作为独立业务页面。
3. `/sessions` 是统一会话工作台，在现有较完整页面上做加法。
4. 本地使用不是绕过 Gate，而是 Agent 连接本机 Gate。
5. 云端接入不是切换业务模式，而是将 Agent 连接的 Gate URL 改成云端地址。
6. 后端 localapi 不要求在第一阶段立即删除，但应逐步降级为兼容层或内部接口。
7. 当前页面布局和主要操作方式保持不变，避免把架构收敛和 UI 重设计混在一起。
8. 自动生成的用户账号使用当前操作系统用户名，密码随机生成并满足常规强度要求。
9. 自动生成的凭据写入 `.termbridge.yaml`，并接入 viper 读取。
10. 首次启动生成登录凭据后，在控制台打印提示信息。
11. Agent device credential 和 Browser user credential 暂时使用同一套凭据，后续由权限系统细分。
12. 本次目标需要补齐 Gate path 的全部 mutation 能力。
13. Device 作用域 API 有两个候选方向：路径式 `/api/devices/:deviceId/sessions`，或资源路径 `/api/sessions` 加 header 指定 device；需在 Spec 阶段评估。
14. 设备名生成规则为默认使用本机 hostname；device id 已单独存在，设备显示名不再追加 id 前缀或后缀。
15. Device 必须成为 workspace / session 持久化数据的一部分，runtime state 目录结构需要按 Device 重新设计。
16. 本次不要求向后兼容旧 state 目录结构或旧 workspace/session 数据。
17. 设备离线时是否进入只读历史模式取决于实现复杂度：若易于实现则列入计划，否则记录为后续能力。
18. 需要系统性设计加载流程和状态，包括 serve 启动、agent 注册、登录、设备列表加载、设备选择、工作台数据加载和错误/空状态。

## Open questions

1. Device 作用域 API 采用哪种形态？
   - 路径式：`/api/devices/:deviceId/sessions`
   - Header 式：`/api/sessions` + device header
   - 需要在 Spec 阶段评估缓存、可观察性、URL 可分享性、浏览器 WebSocket 支持、代理兼容和权限边界。
2. 设备离线时进入只读历史模式是否易于实现？若实现成本低则列入计划，否则作为后续能力。
3. 系统性加载流程如何设计？需要覆盖 serve 启动、agent 注册、登录态检查、设备列表加载、设备选择、workspace/session 加载、terminal attach、错误状态和空状态。

## Risks

1. **范围较大**：该需求跨前端路由、后端 API、Agent tunnel、配置、认证和状态模型。
2. **Gateway mutation 不完整**：当前 Gate path 偏 attach/read，如果前端立即全走 Gate，可能丢失现有 `/sessions` 的写操作能力。
3. **认证边界尚不成熟**：当前 gateway auth / agent auth 仍有 MVP 痕迹，不能只做 UI 登录而保留未鉴权 direct API 作为长期入口。
4. **启动时序风险**：本地 self-connected 模式下，HTTP server ready 不代表 agent 已注册；页面需要处理 device loading 状态。
5. **配置安全风险**：自动生成并写入凭据如果使用明文，需要明确本地威胁模型和文件权限。
6. **数据重建风险**：本次不向后兼容旧 state 目录结构或旧 workspace/session 数据，用户升级后可能看不到旧数据；需要在交付说明中明确。
7. **过度重构风险**：如果同时重做页面布局、API 命名、认证体系和 tunnel mutation，容易扩大范围并影响已可用的 session 工作台。
8. **概念命名风险**：`gateway` 作为内部能力合理，但作为产品路由和用户概念可能误导，需要逐步收敛到 Device / Gate 模型。

## Assumptions

1. `termbridge serve` 仍是本地主要启动入口。
2. 本地 self-connected Gate 是 first-class 使用场景，不依赖云端可用。
3. 云端接入通过配置 Gate URL 实现，而不是切换另一套前端页面或业务流程。
4. 现有 `/sessions` 工作台能力优先保留。
5. 后端可以分阶段从 localapi direct path 迁移到 Gate / device-scoped path。

## Suggested decomposition

### Frontend track

1. 移除 `/gateway` 路由和 gateway-only 页面分支。
2. 保留 `/sessions` 主工作台布局。
3. 增加登录前置流程。
4. 增加设备选择前置流程。
5. 将数据加载从 local/gateway backend 分支改为 selected device 驱动。
6. 左面板底部展示当前设备名。

### Backend device track

1. 定义 Device domain model。
2. 定义 device identity / device credential 存储。
3. 扩展 Agent hello / register payload。
4. Gate 维护 device registry。
5. API 支持列出设备和按 device 加载 workspace/session。

### Auth / bootstrap track

1. 启动时检查用户账号、设备名、device credential。
2. 缺失时自动生成并持久化。
3. Browser 登录使用 user credential。
4. Agent 连接 Gate 使用 device credential。
5. 明确凭据文件位置、格式和安全边界。

### Compatibility track

1. 保留 localapi 作为过渡兼容层。
2. 补齐 Gate mutation 后再让前端完全停止调用 localapi。
3. 设计旧 workspace/session 数据的 device 归属兼容策略。

## User review notes

用户初始想法：

- 前端先干掉 `/gateway` 路由和组件，`/sessions` 功能已较完整，在它基础上做加法。
- 后端添加 Device 等概念。
- 认证在程序启动时检查本地配置文件中的凭据，包括设备名、用户账号；不存在就自动生成和写入，并上报到 Gate。
- 页面打开即需要选取设备，使用用户账号登录。
- 完成后 `/sessions` 左面板底部展示设备名，当前页面布局和操作方式不变。

审视结论：

- 方向与统一 Gate / Device 模型一致。
- 最大风险不在前端删 `/gateway`，而在后端 Gate path 当前尚未覆盖 localapi 的完整 mutation 能力。
- 应避免一次性把认证、设备模型、API 重命名、页面布局重做混在一起。
- 推荐先确认 Requirement，再进入 Spec 阶段拆清楚控制面、数据面、认证和迁移边界。
