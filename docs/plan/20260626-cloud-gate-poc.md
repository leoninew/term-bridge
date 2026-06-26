# Cloud Gate 最小 PoC 计划
最后修改时间: 2026-06-26 17:23:35

Review status: Accepted

## Basis

- Requirement: `docs/requirement/20260626-cloud-gate-poc.md`，Review status: Accepted
- Spec: `docs/spec/20260626-cloud-gate-poc.md`，Review status: Accepted

本计划采用严格模式 / strict。本阶段只定义实施步骤和验证方式，不修改产品代码。

## Plan-stage decisions

用户已在 Spec 阶段明确以下决策，本计划以这些决策为准：

1. 部署目标是支持容器交付的服务器，服务器已有 HTTPS 证书。
2. Cloud Gate 使用同源 HTTPS，由服务器反向代理 / ingress / LB 终止 TLS；TermBridge 容器内部只监听 HTTP。
3. Dockerfile 必须把 web 资源和 Go binary 构建进镜像。
4. PoC 复用现有 Browser / Agent tunnel 认证链路，不单独拆分 agent secret；本轮先固化为 `admin/admin`，用户系统和用户-device 绑定后续再补。
5. 本地 `serve` 启动生成/读取的 `deviceId` + `deviceName` 必须由 Agent hello 上报。
6. PoC 需要支持远程 session 创建和现有 `/sessions` 核心 session 管理能力。
7. Ctrl+C input path 作为本轮 interrupt 定义；不扩展完整 kill escalation。
8. Claude Code / Codex 真实 TUI 由用户自行验证，Verification 只记录用户反馈。
9. 本轮不在 gate-only / cloud runtime 隔离上投入过多工程；远程服务器如作为 device 出现，它就是账户下的云端 device。
10. 最低访问控制和风险提示无须额外工程，复用现有认证与说明。

## Implementation strategy

本 PoC 的实施重心是“把当前 unified Gate workbench 变成可容器交付、可 HTTPS 反代访问、可由本地 Agent 远程连接的形态”。现有 Gateway API 已经具备 device-scoped 路由、Agent tunnel、session create/rerun/update/delete/close、terminal attach、history 和 route unavailable 基础，因此本计划不重写 relay 架构。

实施顺序：

```text
1. 认证与 device 上报收口：PoC 固化 admin/admin，保留本地 deviceId/deviceName 生成与 Agent hello 上报
2. Web 静态资源服务：让生产 /sessions 脱离 Vite dev server
3. route unavailable 收口
4. Dockerfile / .dockerignore
5. 远程 session 管理与 terminal 链路验证
```

实施原则：

- 复用当前 `/api/devices/{deviceId}/...` canonical path。
- 复用现有 Agent tunnel Basic Auth / Browser JWT 模型，不拆分 agent secret；本轮 PoC 凭据固定为 `admin/admin`。
- 不新增 gate-only 作为硬依赖；如果现有 `termbridge serve` 在云端也注册 device，验证时把它视为账户下另一个 device。
- 不依赖 Vite dev server 承载 Cloud Gate Browser 页面。
- 不把 public HTTPS URL 填进 `gate.listen_url`；`gate.listen_url` 仍是容器内部 HTTP bind 地址。
- Docker image 和示例文件不得包含真实 `.env` 或 secret。

## Implementation steps

### Step 1. 固化 PoC 认证并保留 device 上报

目标：Cloud Gate PoC 先使用固定 `admin/admin` 完成 Browser login 与 Agent tunnel 认证；本地 `serve` 仍生成/读取稳定 `deviceId` + `deviceName`，并由 Agent hello 上报给 Gate。

计划：

1. 不新增 `auth.username/password` config/env，不把 Browser credential 当作本轮部署配置面。
2. 在 config/app/gateway/agent 现有链路中收口 PoC 默认认证：
   - Browser login 用户名：`admin`
   - Browser login 密码：`admin`
   - Agent tunnel Basic Auth 同样使用 `admin/admin`
3. 调整 `EnsureLocalIdentity` 语义：
   - 保留 `agent.device_id` / `agent.device_name` 缺失时的生成与写回。
   - 不再为 Browser/Agent tunnel 生成随机 username/password。
   - 不兼容读取历史 `runtime.state_dir/device.json` 中的 auth 或 agent 字段。
4. 确认 `internal/application/agent/client.go` 的 hello payload 继续上报本地 device id/name。
5. 后续用户系统完成后，再设计用户与 device 的绑定关系；本轮不实现。

主要文件：

- `internal/infrastructure/config/config.go`
- `internal/infrastructure/config/config_test.go`
- `internal/app/app.go`
- `README.md`（仅同步现有 PoC 凭据与 device state 说明）

测试场景：

- `Load` 后默认 auth 为 `admin/admin`。
- `EnsureLocalIdentity` 在缺失 agent device id/name 时仍写入 `.termbridge.yaml`。
- `EnsureLocalIdentity` 不再生成随机 auth password，也不再读取 legacy `device.json`。
- Agent hello 使用生成/读取到的 device id/name 上报。

验收要点：

- 用户可以用 `admin/admin` 登录 PoC Gate。
- Local Agent 可以用同一固定凭据连接 tunnel。
- device 选择依赖 Agent 上报的 device id/name，而不是用户系统绑定。

### Step 2. 让生产 Web 静态资源由 TermBridge 服务提供

目标：Cloud Gate 容器启动后，Browser 可以直接访问 `/sessions`，不依赖 Vite dev server。

计划：

1. 在配置中增加可选 Web 静态资源目录，例如：
   - `web.static_dir`
   - env：`TERMBRIDGE_WEB__STATIC_DIR`
2. `internal/transport/http/server/server.go` 支持同时挂载：
   - `/api` 和 `/api/` -> gateway API handler
   - 静态 assets -> static file handler
   - SPA fallback -> `index.html`
3. 路由规则必须保证 `/api` 不会被 SPA fallback 吞掉。
4. 当 `web.static_dir` 为空时，保持当前 API-only 行为，便于本地 Vite 开发。
5. 当 `web.static_dir` 非空但目录或 `index.html` 不存在时，启动应返回明确错误，避免容器看似启动但 Browser 页面不可用。
6. 调整 `web/vite.config.ts`：
   - Vite dev proxy 固定将 `/api` 转发到默认本地后端 `http://127.0.0.1:9010`。
   - production `yarn build` 不依赖 `web/.env` 的 dev backend。

主要文件：

- `internal/infrastructure/config/config.go`
- `internal/infrastructure/config/config_test.go`
- `internal/transport/http/server/server.go`
- `internal/transport/http/server/server_test.go`
- `web/vite.config.ts`
- `.termbridge.default.yaml`
- `.env.example`

测试场景：

- `web.static_dir` 为空时，HTTP server 只提供 `/api`，保持现有测试通过。
- `web.static_dir` 指向包含 `index.html` 的目录时：
  - `GET /` 返回 index。
  - `GET /sessions` 返回 index。
  - `GET /settings` 返回 index。
  - `GET /assets/<file>` 返回静态文件。
  - `GET /api/health` 仍进入 API handler。
  - 不存在的 `/api/...` 不被 index fallback 吞掉。
- `web.static_dir` 配置错误时启动失败且错误可定位。
- `cd web && yarn build` 不依赖 dev backend env，production build 可成功。

验收要点：

- 容器内 TermBridge 服务可直接承载 Browser workbench。
- Cloud Gate `/sessions` 不依赖 Vite dev server。

### Step 3. 收口 route unavailable terminal 错误

目标：Agent disconnect / route unavailable 时 terminal 展示可理解状态。

计划：

1. 修改 `internal/transport/http/gatewayapi/route.go` 中 terminal disconnect 行为：
   - Agent disconnect 时不要向 Browser terminal WebSocket 写裸字符串作为 text frame。
   - 改为发送 terminal protocol error control message，例如 `type=error`、code 表达 device disconnected / route unavailable，再 close。
2. 前端 `web/src/features/sessions/useTerminalSocket.ts` 如已能展示 terminal protocol error，则只补测试；如不能，需要补展示逻辑。

主要文件：

- `internal/transport/http/gatewayapi/terminal_test.go`
- `internal/transport/http/gatewayapi/route.go`
- `web/src/features/sessions/useTerminalSocket.ts`
- `web/src/features/sessions/useTerminalSocket.test.ts`

测试场景：

- Agent route close terminal 时，Browser 收到结构化 terminal error，而不是无法解析的裸 text。
- terminal route unavailable / device offline 的 UI 状态可读。

验收要点：

- Agent disconnect 后 terminal pane 显示可理解错误。

### Step 4. 添加 Dockerfile 与 .dockerignore

目标：产出包含 web 资源和 Go binary 的可运行镜像。

计划：

1. 新增 `Dockerfile`，使用多阶段构建：
   - web build stage：安装 yarn 依赖，执行 `yarn build`，产出 `web/dist`。
   - go build stage：下载 Go module，执行 `go build`，产出 `termbridge` binary。
   - runtime stage：复制 binary、`web/dist`、必要默认配置和 CA certificates。
2. runtime stage 默认工作目录应稳定，例如 `/opt/termbridge`。
3. runtime stage 默认环境：
   - `TERMBRIDGE_GATE__LISTEN_URL=http://0.0.0.0:9010`
   - `TERMBRIDGE_WEB__STATIC_DIR=/opt/termbridge/web/dist`
   - `TERMBRIDGE_RUNTIME__STATE_DIR=/var/lib/termbridge`
4. 不在镜像中复制真实 `.env`、`.termbridge` state、logs、node_modules 或本地 build cache。
5. 新增 `.dockerignore`，排除：
   - `.env`
   - `.termbridge/`
   - `logs/`
   - `bin/`
   - `web/node_modules/`
   - `web/dist/`
   - git metadata 以外不必要缓存
6. Dockerfile 不负责 HTTPS 证书，不内置反向代理；HTTPS 由目标服务器提供。
7. 如 Go runtime 需要 shell / ca-certificates / timezone，应在 runtime image 中明确安装或选择包含它们的 base image。

主要文件：

- `Dockerfile`
- `.dockerignore`
- `web/package.json`
- `web/yarn.lock`
- `go.mod`
- `go.sum`
- `.env.example`

测试场景：

- `docker build` 能完成 web build 和 Go build。
- 镜像内不包含项目根 `.env`。
- 容器启动后 `/api/health` 可访问。
- 容器启动后 `/sessions` 返回生产前端页面。
- 容器内部监听 HTTP，外部 HTTPS 由反向代理提供。

验收要点：

- Dockerfile 是 Cloud Gate PoC 的主要交付物。
- Web 资源和 Go binary 均来自 Dockerfile 构建流程。

### Step 5. 保留最小配置示例，不新增部署说明文档

目标：本轮不编写独立 Cloud Gate PoC 部署说明，只在现有配置示例中保留 Docker/Cloud Gate 运行所需 key。

计划：

1. 不新增 `docs/deploy/cloud-gate-poc.md`。
2. `.env.example` 仅补充当前实现真实支持的最小配置 key：
   - `TERMBRIDGE_GATE__LISTEN_URL=http://0.0.0.0:9010`
   - `TERMBRIDGE_WEB__STATIC_DIR=/opt/termbridge/web/dist`
   - `TERMBRIDGE_RUNTIME__STATE_DIR=/var/lib/termbridge`
   - `TERMBRIDGE_GATE__BROWSER__ALLOWED_ORIGINS=https://<gate-domain>`
   - `TERMBRIDGE_GATE__API__EXPOSE_ERRORS=false`
   - `TERMBRIDGE_JWT__SECRET_KEY=<secret>`
   - `TERMBRIDGE_AGENT__CONNECT_URL=https://<gate-domain>`
3. 示例不包含 `TERMBRIDGE_AUTH__USERNAME/PASSWORD`，因为本轮 PoC 固化为 `admin/admin`。
4. README 只同步当前认证与 device state 事实，不写完整部署教程。

主要文件：

- `README.md`
- `.env.example`

测试 / 检查场景：

- 示例中的配置 key 与 `configKeys()` 支持的 key 一致。
- 示例没有真实 secret。

验收要点：

- 本轮交付不包含独立部署说明文档。

### Step 6. 验证远程 session 管理与 terminal 链路

目标：证明 Cloud Gate PoC 覆盖用户要求的 session 管理、terminal 交互和 route unavailable。

计划：

1. 后端自动化验证：
   - Gateway API session create / close / rerun / update / delete 仍通过 Agent tunnel 工作。
   - terminal attach / input / output / resize E2E 保持。
   - route unavailable 返回 `device_offline` 或等价稳定错误。
2. 前端自动化验证：
   - `/sessions` 使用 device-scoped API 的 create/close/rerun/edit/delete 入口仍指向 canonical path。
   - terminal socket 处理 route unavailable / structured error。
3. 容器 smoke 验证：
   - build image。
   - run container with Cloud Gate env。
   - open `/api/health`。
   - open `/sessions`。
4. 手工/半手工真实路径验证：
   - Cloud Gate 容器运行在目标或等效服务器环境。
   - Local Agent 使用 `https://<gate-domain>` 连接。
   - Browser 登录。
   - device list 出现目标 device。
   - 创建 session。
   - attach terminal。
   - 输入命令并看到输出。
   - resize terminal。
   - 对 long-running shell command 发送 Ctrl+C。
   - 断开 Agent，确认 route unavailable / device offline。
5. Claude Code / Codex TUI：
   - 用户自行验证。
   - Verification 文档只记录用户反馈结果。

主要文件：

- `internal/transport/http/gatewayapi/relay_test.go`
- `internal/transport/http/gatewayapi/terminal_test.go`
- `internal/transport/http/gatewayapi/terminal_e2e_test.go`
- `internal/application/agent/client_test.go`
- `web/src/features/sessions/useTerminalSocket.test.ts`
- `web/src/store/workspaceSessions.test.ts`
- `docs/verification/20260626-cloud-gate-poc.md`（Verification 阶段创建）

验收要点：

- Cloud Gate container + Local Agent 分离路径可复查。
- 远程 session 管理不是 attach-only。
- route unavailable 行为可理解。

## Expected changed files

预计会修改或新增：

- `docs/requirement/20260626-cloud-gate-poc.md`
- `docs/spec/20260626-cloud-gate-poc.md`
- `docs/plan/20260626-cloud-gate-poc.md`
- `README.md`
- `Dockerfile`
- `.dockerignore`
- `.termbridge.default.yaml`
- `.env.example`
- `internal/infrastructure/config/config.go`
- `internal/infrastructure/config/config_test.go`
- `internal/transport/http/server/server.go`
- `internal/transport/http/server/server_test.go`
- `internal/transport/http/gatewayapi/route.go`
- `internal/transport/http/gatewayapi/terminal_test.go`
- `internal/transport/http/gatewayapi/terminal_e2e_test.go`
- `web/vite.config.ts`
- `web/src/features/sessions/useTerminalSocket.ts`（如当前结构化错误处理不足）
- `web/src/features/sessions/useTerminalSocket.test.ts`（如前端需要补测试）

预计不需要修改：

- `internal/protocol/tunnel/frame.go`，除非 route unavailable 结构化错误发现需要新 frame。
- `internal/application/agent/runtime_access.go`，因为现有远程 session 管理已通过 RuntimeAccess 覆盖。
- `web/src/features/sessions/api.ts`，除非验证发现远端 session 管理路径缺漏。

## Verification plan

### 自动化检查

- Go：`go test ./cmd/... ./internal/...`
- Go vet：`go vet ./cmd/... ./internal/...`
- Web typecheck：`cd web && yarn typecheck`
- Web lint：`cd web && yarn lint`
- Web tests：`cd web && yarn test`
- Web build：`cd web && yarn build`
- Docker build：构建 Cloud Gate PoC image。

### 容器 smoke

- 使用 Cloud Gate env 启动容器。
- `GET /api/health` 返回 ok。
- `GET /sessions` 返回前端页面。
- `/api` 路径不被 SPA fallback 吞掉。
- 日志中不出现 WebSocket query token 明文。

### 真实链路验证

- Cloud Gate 容器运行在目标服务器或等效跨网络环境。
- Local Agent 使用 HTTPS public URL 连接 Cloud Gate。
- Browser 通过 HTTPS 登录 Cloud Gate。
- Browser 可以看到 device list。
- Browser 在目标 device 上创建 session。
- Browser attach terminal。
- input/output 正常。
- resize 正常。
- Ctrl+C 能中断 long-running shell command。
- Agent 断开后，Browser 看到 route unavailable / device offline。

### 用户验证

- Claude Code / Codex 真实 TUI 由用户自行验证。
- Verification 文档记录用户反馈结论。

## Rollback / fallback

如果实现后发现风险或回归：

1. Dockerfile / static serving 可以作为独立交付回滚，不影响本地 `just serve` + Vite dev 流程。
2. `web.static_dir` 为空时保持 API-only 行为，便于本地开发回退。
3. 固定 PoC 凭据若影响后续用户系统设计，应在后续需求中引入用户与 device 绑定模型，而不是在本轮临时扩展 auth config。
4. route unavailable 结构化错误若影响前端，可先保留后端 close 行为并在前端兼容两种消息，但最终应避免裸字符串成为主要用户反馈。

## Risks and mitigations

1. **容器内静态资源服务破坏 API 路由**
   - 缓解：server tests 覆盖 `/api` 优先级和 SPA fallback。
2. **production build 被 Vite dev env 阻塞**
   - 缓解：`web/vite.config.ts` 区分 dev server proxy 与 production build。
3. **PoC 固定凭据不是正式用户系统**
   - 缓解：在本轮边界中明确 `admin/admin` 只用于 PoC；后续用户系统完成后再设计用户与 device 的绑定关系。
4. **远程 session 管理带来误操作风险**
   - 缓解：本轮按用户要求支持；Verification 明确覆盖 create/close/rerun/edit/delete，并在后续 M7 安全边界中继续处理细粒度授权。
5. **Cloud server 自身也作为 device 出现造成混淆**
   - 缓解：本轮不做 gate-only；验证时明确 selected device，依赖 device name/account isolation。
6. **Claude Code / Codex TUI 未由 coding agent 自动验证**
   - 缓解：按用户要求由用户自行验证；Verification 文档记录用户反馈。

## User review notes

本 Plan 需要用户重点审阅：

1. 用户已修正：本轮不做 `auth.username/password` 配置化；PoC 固化 `admin/admin`。
2. 用户已修正：本轮不新增 Cloud Gate PoC 部署说明文档。
3. 用户已确认：本地 `serve` 生成/读取的 device id/name 需要由 Agent 上报。
4. `web.static_dir` 仍作为 Cloud Gate 容器提供生产前端的最小配置面。
5. Dockerfile 只负责构建/运行 TermBridge，HTTPS 仍由服务器反向代理处理。
6. route unavailable 结构化错误作为本轮必要 hardening。

用户已要求 `/specflow 开始实现`，本 Plan 标记为 Accepted 并进入 Implementation 阶段。
