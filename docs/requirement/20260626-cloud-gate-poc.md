# Cloud Gate 最小 PoC 需求
最后修改时间: 2026-06-26 17:23:35

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；需求 / Requirement 已接受，当前进入计划 / Plan。

用户已明确要求开始 `/spec`，因此本任务从标准模式调整为严格模式：Requirement -> Spec -> Plan -> Implementation -> Verification。用户已审阅 Spec 决策并要求进入 Plan。

## Background

TermBridge-go 的长期产品模型是：Browser 通过 Gate 访问 Device Agent，再由 Agent 操作本地 Runtime / Workspace / Session / Terminal。当前本地 self-connected 路径已经具备 `termbridge serve` 统一入口、Device-scoped Browser API、Gateway / Agent relay、基础 Browser auth、workspace/session mutation、terminal attach/history/resize hardening 等能力。

当前路线图阶段仍是 M6.2 Unified Device Workbench Closeout，M7 Multi-device Beta 尚未开始。`docs/design/roadmap.md` 明确要求进入 M7 前完成 M6.2 P0、M5 关键补证、Remote Gate 部署与安全边界设计、Device credential / pairing / rotation 策略、Terminal websocket auth 与 route unavailable 策略。

本任务不是直接承诺进入 M7 Beta，而是做一个受控的 Cloud Gate 最小 PoC：把 Gate 部署到支持容器交付的服务器上，通过已有 HTTPS 证书提供 Browser / Agent 访问入口，让本地 Agent outbound 连接云端 Gate，Browser 从云端入口访问本地 runtime，以验证最短真实链路和暴露架构风险。

用户补充确认：本轮不在“Cloud Gate 不直接创建 PTY / 不运行 runtime”这类边界上花过多时间。远程 Gate 所在服务器本身也是一个环境，是否使用它作为 device 是产品选择和验证选择；本轮重点是 Cloud Gate 作为远端入口是否能访问用户指定的本地 Agent/device，并且 device 本身由账户隔离，云端设备就是云端设备。

相关既有文档：

- `docs/design/design.md`
- `docs/design/roadmap.md`
- `docs/requirement/20260620-m6-gateway-web-terminal-mvp.md`
- `docs/spec/20260620-m6-gateway-web-terminal-mvp.md`
- `docs/requirement/20260623-unified-gate-device-auth.md`
- `docs/spec/20260623-unified-gate-device-auth.md`
- `docs/requirement/20260625-jwt-auth.md`
- `docs/requirement/20260625-dotenv-config.md`

## Goal

1. 评估并推进 Cloud Gate 最小 PoC，使远端部署的 Gate 可以作为 Browser 入口和 Agent tunnel 接入点。
2. 部署目标是支持容器交付的服务器，服务器已有 HTTPS 证书。
3. HTTPS 由服务器侧反向代理、ingress 或等价组件终止，TermBridge Gate 容器内部可以继续使用 HTTP 监听。
4. 本地 Agent 不开放入站端口，通过 outbound 连接云端 Gate。
5. Browser 登录云端 Gate 后，可以看到本地 Agent 上报的 device，并选择该 device 进入 `/sessions` 工作台。
6. Browser 通过云端 Gate 访问所选 device 的 workspace/session/history，并 attach 到已有或新创建的 session terminal。
7. PoC 需要支持远程 session 管理，包括创建 session，以及现有 `/sessions` 工作台中的核心 session 操作。
8. PoC 重点验证远端链路下的 terminal attach、input/output、resize、Ctrl+C interrupt input path 和 route unavailable 行为。
9. PoC 使用现有 Browser / Agent tunnel 认证方式，不额外拆分 agent secret。
10. 产出可复查的任务拆解、验证记录和后续进入 M7 Beta 前必须补齐的事项。

## Non-goal

本 PoC 不做以下事项：

1. 不承诺 M7 Multi-device Beta 完成。
2. 不实现完整 secure pairing UX。
3. 不实现 token rotation / refresh token / 长期设备凭据轮换策略。
4. 不实现细粒度 terminal authorization、RBAC、多用户、多租户或组织权限。
5. 不实现完整监控、告警、审计日志、SLO 或生产运维体系。
6. 不实现安装包、发布包、升级、回滚或 public release 文档。
7. 不把“云端环境绝不能拥有 runtime / 绝不能创建 PTY”作为本轮硬性工程目标；如果云端服务器同时运行 Agent/runtime，它只是账户下的另一个 device。
8. 不为“最低访问控制和风险提示”增加额外工程；复用现有认证与文档说明即可。
9. 不要求由 coding agent 覆盖 Claude Code / Codex 真实 TUI；该项由用户自行验证。
10. 不要求覆盖全部 M5 hardening 压测，例如 long-running、大输出 backpressure、20+ session 压力和完整 IME 体验；这些可以作为 PoC 风险输入或后续补证。
11. 不改变 `/sessions` 的核心布局和主要交互方式。
12. 不把 HTTPS 证书自动签发、证书续期或域名管理纳入本 PoC；服务器已具备 HTTPS 证书，本任务只要求部署形态正确使用它。

## User scenarios

### Scenario 1：部署者启动云端 Gate

1. 维护者在支持容器交付的服务器上部署 Gate。
2. 服务器使用已有 HTTPS 证书对外提供 `https://<gate-domain>`。
3. 反向代理或 ingress 将 Browser API、terminal WebSocket 和 Agent tunnel 请求转发到 TermBridge Gate 容器。
4. Dockerfile 将 web 资源和 Go binary 构建进入镜像。
5. Gate 启动时使用明确的配置来源加载 listen URL、JWT secret、允许的 Browser origin 等配置。
6. PoC 阶段 Browser / Agent tunnel 认证先固化为 `admin/admin`；后续补用户系统后再设计用户与 device 的绑定关系。
7. 若关键配置缺失或明显不安全，Gate 应 fail fast 或输出清晰错误。

### Scenario 2：本地 Agent outbound 连接云端 Gate

1. 用户在本地配置中将 Agent connect URL 指向云端 HTTPS Gate 地址。
2. 用户启动本地 `termbridge serve` 或等价 Agent 连接流程。
3. Agent 复用现有 PoC 认证方式通过 WSS 连接云端 Gate，并上报本地启动生成或读取到的 device id / device name / capabilities。
4. Gate 将该 device 标记为 online，并用于后续 Browser 选择。
5. Agent 断开后，Gate 能将 device 标记为 offline 或 route unavailable。

### Scenario 3：Browser 通过云端 Gate 管理 session

1. 用户打开云端 Gate 的 Web 页面。
2. 用户登录后看到自己账户下的 device。
3. 用户选择目标 device 进入 `/sessions` 工作台。
4. 工作台展示该 device 下的 workspace/session/history。
5. 用户可以在云端 `/sessions` 创建 session。
6. 用户可以 attach 到 session terminal，看到输出、输入命令、resize terminal，并触发 Ctrl+C。
7. 用户可以使用现有 `/sessions` 核心 session 管理能力，例如 close、rerun、edit、delete，具体以当前工作台已有能力为准。

### Scenario 4：route unavailable / device offline

1. Browser 已选择某个 device 或正在 attach terminal。
2. 本地 Agent 断开、凭据错误、网络中断或 cloud Gate 找不到有效 tunnel。
3. Browser 操作返回明确的 route unavailable / device offline / auth error，而不是空白页面或模糊失败。
4. terminal 连接中断时，用户能看到需要重新 attach 或等待 Agent 重连的状态。

## Acceptance

### PoC 链路

- [ ] 云端 Gate 可以通过 Dockerfile 构建出的容器镜像启动，并对 Browser API 与 Agent tunnel 提供可访问入口。
- [ ] 公网或跨网络访问必须使用 HTTPS / WSS；HTTP 只允许容器内或反向代理到应用的内部网络。
- [ ] 本地 Agent 可以通过 outbound tunnel 连接云端 Gate，不需要本地入站端口。
- [ ] Agent hello / registration 能让 Gate 识别 device id、device name、online 状态和必要 capabilities。
- [ ] Browser 登录云端 Gate 后可以列出当前账户下 online device。
- [ ] Browser 选择目标 device 后可以加载该 device 的 workspace tree、session list 和 session history。
- [ ] Browser 可以通过云端 Gate 创建 session。
- [ ] Browser 可以通过云端 Gate attach 到 session terminal。
- [ ] Browser 可以通过云端 Gate 执行现有 `/sessions` 核心 session 管理能力：close、rerun、edit、delete，若当前工作台已有对应能力则必须在远端路径保持可用。
- [ ] terminal input/output 经 Browser -> Cloud Gate -> selected Device Agent -> selected Device Runtime 链路传递成功。
- [ ] terminal resize 经远端链路传递到 selected device runtime，并有可观察结果或日志证明。
- [ ] Ctrl+C interrupt input path 在远端 attach 下有明确验证结论。
- [ ] Agent disconnect 后，新请求返回 route unavailable 或 device offline；已有 terminal attach 有明确中断状态。

### 容器与部署

- [ ] Dockerfile 构建 web 资源和 Go binary，并产出可运行 Cloud Gate 镜像。
- [ ] Browser 访问 `/sessions` 不依赖 Vite dev server。
- [ ] Web 静态资源由容器内 TermBridge 服务提供，或由镜像内明确配置的静态服务提供。
- [ ] `/api`、terminal WebSocket 和 `/api/agent/tunnel` 在 HTTPS Gate URL 下可用，并支持 WebSocket upgrade。
- [ ] 配置示例明确区分容器内部监听地址与外部 HTTPS 访问地址。
- [ ] Docker/container 交付物不包含真实 `.env`、JWT secret、Browser password 或 Agent credential。

### 安全与边界

- [ ] PoC 复用现有 Browser / Agent tunnel 认证链路；本轮先固化为 `admin/admin`，不新增用户系统或独立 agent secret。
- [ ] 未认证请求不能访问 device/session/history/terminal attach。
- [ ] 未授权 Agent 不能注册 device 或接收路由。
- [ ] JWT secret、Agent connect URL、Browser allowed origins 等部署敏感配置不能硬编码到代码中；`admin/admin` 仅作为本轮 PoC 固化登录与 tunnel 凭据。
- [ ] 日志、错误和风险提示不能打印明文 password、JWT secret、Authorization header、Bearer token、WebSocket query token 或等价 secret。
- [ ] Device id 只作为标识，不作为 secret 使用。

### 验证记录

- [ ] 有 SpecFlow Plan / 计划文档记录任务拆解、文件范围、验证方式、风险和回滚/降级策略。
- [ ] 有 Verification / 验证文档记录实际 diff、预期与实际改动对比、验收清单、测试/手工验证结果和未完成项。
- [ ] 至少记录一条真实路径验证：Cloud Gate 容器 + Local Agent 分离部署或等效跨进程/跨网络环境。
- [ ] Claude Code / Codex 真实 TUI 由用户自行验证；coding agent 的 Verification 只记录是否已由用户反馈通过。
- [ ] 记录哪些能力只是 PoC 通过，哪些仍是进入 M7 Beta 前置工作。

## Open questions

暂无阻塞进入 Plan 的未决事项。

## Decisions

1. 用户明确要求开始 `/spec`，因此本任务采用严格模式 / strict。
2. 本任务作为新的 SpecFlow feature 文档创建，文件名为 `20260626-cloud-gate-poc.md`。
3. PoC 范围采用用户确认的“最小 PoC”，不是 M7 Beta 或生产发布级。
4. 部署目标是支持容器交付的服务器。
5. 服务器已有 HTTPS 证书；PoC 要使用 HTTPS / WSS 对外访问，但不负责证书签发和续期。
6. PoC 复用现有 Browser / Agent tunnel 认证方式，不单独拆分 agent secret。
7. PoC 需要支持云端 `/sessions` 创建 session 和现有核心 session 管理能力。
8. PoC 验证重点是 attach、input/output、resize、Ctrl+C interrupt input path、route unavailable。
9. Claude Code / Codex 真实 TUI 由用户自行验证。
10. Dockerfile 需要把 web 资源和 Go binary 构建进入镜像。
11. 同源 HTTPS + 服务器反向代理 TLS 终止被接受；TermBridge 容器内部只监听 HTTP。
12. 继续遵守统一产品模型：Browser -> Gate -> Device Agent -> Runtime -> Workspace -> Session -> Terminal。
13. 本轮不在“Cloud Gate 不直接创建 PTY / 不运行 runtime”上投入过多工程；远程 Gate 所在服务器如果也运行 Agent/runtime，可以视为账户下的另一个 device。
14. Plan 阶段应优先复用当前 Gate / Device / Auth / JWT / dotenv 配置能力，而不是重新设计一套并行机制。
15. 用户进一步确认：本地 `serve` 启动时生成/读取的 `deviceId` + `deviceName` 需要由 Agent 上报；用户系统后续再补，本轮先将 Browser login 与 Agent tunnel 凭据固化为 `admin/admin`，用户登录后选取设备即可。

## Risk

1. **安全误用风险**：最小 PoC 若直接暴露公网，仍缺少 pairing、rotation、细粒度授权、审计和监控。
2. **配置泄露风险**：JWT secret、Cloud Gate URL、allowed origins 等配置如果进入日志、仓库或公开文档，会破坏 PoC 安全边界；本轮 `admin/admin` 是临时 PoC 凭据，不能被描述成正式用户系统。
3. **远端链路放大 runtime 风险**：M5 仍有真实 TUI、backpressure、long-running、压力场景补证，Cloud Gate 会让这些问题更难定位。
4. **route stale 风险**：Agent reconnect / disconnect 清理不严谨时，Browser 可能 attach 到失效 tunnel 或看到错误 device 状态。
5. **本地与云端状态污染风险**：如果 selected device 与 workspace/session/terminal 状态隔离不完整，云端 PoC 会放大 M6.2 尚未完全收口的问题。
6. **容器交付风险**：当前仓库尚无 Dockerfile、compose、生产静态资源服务和正式部署文档，PoC 需要补齐最小交付面。
7. **HTTPS 误配置风险**：当前 Go HTTP server 不提供 TLS；若误把外部 HTTPS URL 当作内部 listen URL，会导致监听和安全边界错误。
8. **远程 session 管理风险**：创建、rerun、edit、delete 等能力通过云端暴露后，误操作和认证边界风险高于 attach-only PoC。
9. **范围扩张风险**：如果在 PoC 中同时做生产安全、安装包、监控、正式多设备体验，会偏离“最短真实链路”目标。

## User review notes

用户已确认：

1. 这次评估按“最小 PoC”目标范围规划。
2. 不纳入完整 pairing、token rotation、细粒度 terminal 授权、监控、安装包、升级回滚。
3. 重点验证 attach、input/output、resize、interrupt、route unavailable。
4. 用户要求使用 `/specflow` 开始这个任务。
5. 部署目标是支持容器交付的服务器。
6. 服务器已有 HTTPS 证书。
7. 用户要求开始 `/spec`，随后确认进入 Plan。
8. 用户补充：本轮不在“Cloud Gate 不直接创建 PTY”这类事情上花太多时间，因为远程 Gate 启动也是一个环境，只是用与不与而已。
9. 用户补充：device 本身就是账户隔离的，云端设备是云端的设备。
10. 用户补充：最低访问控制和风险提示无须额外工作。
11. 用户确认：Agent tunnel 认证复用现有认证。
12. 用户确认：PoC 需要支持会话创建和远程 session 管理。
13. 用户确认：Claude Code / Codex 真实 TUI 由用户自行验证。
14. 用户确认：添加 Dockerfile，web 资源和 Go binary 构建进入 Dockerfile。
15. 用户确认：接受同源 HTTPS，由服务器反向代理终止 TLS，TermBridge 容器内部只监听 HTTP。
16. 用户确认：interrupt 以 Ctrl+C 为准。
