# Cloud Gate 最小 PoC 规格
最后修改时间: 2026-06-26 17:23:35

Review status: Accepted

## Requirement basis

依据：`docs/requirement/20260626-cloud-gate-poc.md`

当前 Requirement 已接受，流程模式为严格模式 / strict。用户已审阅规格阶段决策并要求进入 Plan。

已确认的关键需求：

1. 本任务是 Cloud Gate 最小 PoC，不是 M7 Beta，也不是生产发布级部署。
2. 部署目标是支持容器交付的服务器。
3. 服务器已有 HTTPS 证书；PoC 要通过 HTTPS / WSS 对外提供 Browser 与 Agent 访问入口，但不负责证书签发和续期。
4. 本地 Agent 不开放入站端口，通过 outbound tunnel 连接云端 Gate。
5. Browser 登录云端 Gate 后选择账户下的 device，并通过 `/sessions` 访问该 device 的 workspace/session/history/terminal。
6. Device 本身由账户隔离；云端服务器如果也作为 device 出现，它就是云端的 device，不需要本轮为此做额外隔离工程。
7. PoC 需要支持远程 session 创建和现有 `/sessions` 核心 session 管理能力。
8. 验证重点是 attach、input/output、resize、Ctrl+C interrupt input path、route unavailable。
9. Agent tunnel 认证复用现有认证链路，不单独拆分 agent secret；本轮 PoC 先固化为 `admin/admin`，用户系统与用户-device 绑定后续再补。
10. Claude Code / Codex 真实 TUI 由用户自行验证。
11. Dockerfile 需要把 web 资源和 Go binary 构建进入镜像。
12. 最低访问控制和风险提示不做额外工程；复用现有认证与现有文档说明。

相关既有实现和文档：

- `docs/design/design.md`
- `docs/design/roadmap.md`
- `docs/spec/20260620-m6-gateway-web-terminal-mvp.md`
- `docs/spec/20260623-unified-gate-device-auth.md`
- `docs/requirement/20260625-jwt-auth.md`
- `docs/requirement/20260625-dotenv-config.md`
- `README.md`
- `internal/app/app.go`
- `internal/infrastructure/config/config.go`
- `internal/transport/http/server/server.go`
- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/route.go`
- `internal/transport/http/gatewayapi/errors.go`
- `internal/application/agent/client.go`
- `web/src/features/sessions/api.ts`

## Overview

Cloud Gate PoC 的目标部署形态是：一台支持容器交付的服务器通过已有 HTTPS 证书对外暴露 TermBridge Gate；本地用户机器上的 Agent 主动通过 WSS 连接该 Gate；Browser 通过同一个 HTTPS origin 访问 Web UI 和 `/api`。

```text
Browser
  -> https://gate.example.com
  -> HTTPS reverse proxy / ingress / LB
  -> TermBridge container on server
  -> Agent outbound tunnel over WSS
  -> selected Device Agent
  -> selected Device Runtime
  -> PTY / Process
```

PoC 的关键验证目标是“Browser 通过 cloud-hosted Gate 能稳定访问账户下所选 device，并完成远程 session 管理和 terminal 交互”。远程 Gate 所在服务器如果也运行 Agent/runtime，它就是账户下另一个 device；本轮不把 gate-only 运行模式作为硬性前置。

本规格采用以下方案：

1. **同源 HTTPS 部署**：Browser 静态资源、REST API、terminal WebSocket、Agent tunnel 都从同一个 public HTTPS origin 访问，避免在 PoC 中引入 REST CORS。
2. **外部 TLS 终止**：服务器已有 HTTPS 证书，由反向代理 / ingress / LB 终止 TLS；Go server 容器内继续监听 HTTP。
3. **复用现有 serve、device 上报与认证链路**：保留当前 `termbridge serve` 的 Gate + Agent connector 语义；本地启动生成/读取的 `deviceId` + `deviceName` 由 Agent hello 上报；Agent tunnel 继续复用现有认证链路，本轮 PoC 先固化为 `admin/admin`，不拆分 agent secret。
4. **容器内提供 Web 与 Go binary**：Dockerfile 构建 web 资源和 Go binary，产出可运行镜像；Browser 访问 `/sessions` 不依赖 Vite dev server。
5. **远程 session 管理纳入 PoC**：远端 `/sessions` 需要支持 create、attach、close、rerun、edit、delete 等当前工作台核心能力。
6. **Ctrl+C 作为 interrupt PoC 定义**：本轮只验证 terminal input path 中的 Ctrl+C，不扩展完整 kill escalation。

## Design decisions

### 1. Cloud Gate 使用同源 HTTPS + 反向代理 TLS 终止

Cloud Gate public URL 形态：

```text
https://gate.example.com/
https://gate.example.com/sessions
https://gate.example.com/api/health
wss://gate.example.com/api/agent/tunnel
wss://gate.example.com/api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/ws
```

TermBridge 容器内部监听形态：

```text
http://0.0.0.0:9010
```

设计理由：

- 当前 `internal/transport/http/server/server.go` 只根据 `gate.listen_url` 做 `net.Listen`，不会因为 URL scheme 是 `https` 就启用 TLS。
- 因此 `gate.listen_url` 是内部监听地址，不应填 public HTTPS URL。
- 服务器已有 HTTPS 证书，应由反向代理 / ingress / LB 负责 TLS 和 WebSocket upgrade。
- 前端 API client 使用相对 `/api`，terminal WS URL 也是相对路径，适合同源部署。
- 当前 `gate.browser.allowed_origins` 主要用于 terminal WebSocket Origin 检查，不是 REST API CORS；同源部署可避免本轮新增 CORS。

PoC 要求：

- Public Browser URL 必须是 HTTPS。
- Local Agent `agent.connect_url` 必须使用 HTTPS public URL，Agent client 会转换成 WSS tunnel。
- HTTP 只允许存在于容器内部网络或反向代理到应用的内部链路。
- 反向代理必须支持 WebSocket upgrade。

### 2. `gate.listen_url` 与 public URL 分离

当前配置已有 `gate.listen_url` 和 `agent.connect_url`。本 PoC 不新增 `gate.public_url`，但必须在配置和部署文档中明确两者语义：

| 配置/概念 | 语义 | Cloud Gate PoC 用法 |
|---|---|---|
| `gate.listen_url` | Go server 进程监听地址 | `http://0.0.0.0:9010` |
| public Gate URL | Browser / Agent 从外部访问的 HTTPS 地址 | `https://gate.example.com` |
| `agent.connect_url` | Device Agent 连接 Gate 的外部地址 | `https://gate.example.com` |
| `gate.browser.allowed_origins` | Browser terminal WebSocket Origin allowlist | `https://gate.example.com` |

### 3. 复用现有 Browser / Agent tunnel 认证

当前 Browser 登录和 Agent tunnel 已经复用同一套认证材料：

- Browser 通过 `/api/login` 获取 JWT Bearer token。
- Browser 后续 REST API 使用 Bearer token。
- terminal WebSocket 通过 query token 完成 Browser auth。
- Agent tunnel 使用 Basic Auth 连接 `/api/agent/tunnel`。

本 PoC 不新增独立 agent secret，不拆分 Browser credential 和 Agent tunnel credential。用户系统后续再补，本轮先将 Browser login 与 Agent tunnel Basic Auth 固化为 `admin/admin`；用户登录后通过 device list 选取由 Agent 上报的 device。

### 4. Web 静态资源由容器内 TermBridge 服务提供

当前 `just build` 会构建 `web/dist` 和 Go binary，但 Go HTTP server 只挂载 `/api`，没有生产静态资源服务。Cloud Gate PoC 不能依赖 Vite dev server。

本规格要求：

- Dockerfile 构建 web 资源和 Go binary。
- 容器运行后，TermBridge 服务可以提供 `/`、`/sessions`、`/settings`、`/help` 和静态 assets。
- `/api` 继续进入 gateway API handler，不能被 SPA fallback 吞掉。
- 前端 production build 不应强依赖 `web/.env` 中的 dev proxy 配置。

实现方式在 Plan 阶段选择，优先选择最少组件的方式：Go server 提供 `web/dist` 静态文件和 SPA fallback。

### 5. 远程 session 管理纳入 PoC 完成定义

用户已确认 PoC 需要支持会话创建和远程 session 管理。因此云端 `/sessions` 不只是 attach-only。

PoC 需要覆盖当前工作台核心能力：

- workspace tree。
- session list。
- create session。
- attach terminal。
- input/output。
- resize。
- Ctrl+C。
- close session。
- rerun session。
- edit session。
- delete session。
- history read。
- route unavailable / device offline。

如果某项能力当前 UI 已有入口，则远端路径必须保持可用；如果 UI 没有入口，不为本 PoC 新增产品入口。

### 6. 云端服务器本机 device 不是本轮重点隔离对象

本轮不要求第一轮必须新增 gate-only / agent-only 运行模式。远程 Gate 所在服务器本身也是一个环境，如果它也注册为 device，它就是账户下的云端 device。

规格关注点是：

- device 由账户隔离。
- Browser device list 展示账户下 device。
- 用户选择哪个 device，workspace/session/terminal 就属于哪个 device。
- 验证记录应明确测试的是哪个 device。

如果 Plan 阶段发现低成本即可避免云端服务器自注册，可以作为优化；否则不作为阻塞项。

### 7. Ctrl+C 是本轮 interrupt 定义

当前 Browser/Gate/Agent terminal relay 没有单独的 `interrupt` control message 或 server-side interrupt endpoint。已有能力是：Browser 发送 terminal input，Ctrl+C 可以作为 `0x03` 字节经 input path 写入 PTY。

本 PoC 的 interrupt 定义为：

```text
Browser terminal 中触发 Ctrl+C
  -> terminal input bytes
  -> Cloud Gate
  -> selected Device Agent
  -> selected Device Runtime PTY
  -> 前台进程收到中断字符
```

不新增完整 server-side interrupt / kill escalation 作为本规格要求。

### 8. Claude Code / Codex TUI 由用户自行验证

coding agent 的 Verification 需要记录基础远端链路和自动化检查结果。Claude Code / Codex 真实 TUI 不作为 coding agent 必须自动跑通的验收项，由用户自行验证并反馈结论；Verification 文档只记录用户反馈是否通过。

## Affected components

### Config

- `internal/infrastructure/config/config.go`
  - 需要继续支持 `gate.listen_url`、`gate.browser.allowed_origins`、`gate.api.expose_errors`、`agent.connect_url`、`agent.device_id`、`agent.device_name`、`jwt.secret_key`。
  - 需要支持生产静态资源目录配置。
  - 不在本轮新增 `auth.username/password` 配置；PoC 登录与 tunnel 凭据先固化为 `admin/admin`。
  - 需要避免生产构建依赖本地 `.env` 中的真实 secret。

- `.termbridge.default.yaml`
  - 如新增 web static config key，应同步默认配置。

- `.env.example`
  - 增加 Cloud Gate container profile、Local Agent profile 示例。
  - 示例只写占位符，不写真实 secret。

### HTTP server / static assets

- `internal/transport/http/server/server.go`
  - 支持 `/api` 与 SPA/static fallback 共存。
  - 确保 `/api` 不被前端 fallback 吞掉。

- `web/vite.config.ts`
  - production build 不应因为缺少 dev proxy backend 而失败。

- `web/`
  - production build 进入 Dockerfile。

### Gateway API / route / logging

- `internal/transport/http/gatewayapi/server.go`
  - 现有 device-scoped session management path 需要在 Cloud PoC 下保持可用。
  - `/api/agent/tunnel` 继续复用现有认证方式。

- `internal/transport/http/gatewayapi/route.go`
  - Agent disconnect 时应返回可理解 terminal 状态。

- `internal/transport/http/gatewayapi/errors.go`
  - 避免错误日志泄露 WebSocket query token。

- `internal/transport/http/middleware/requestlog/logging.go`
  - 避免 request log 泄露 WebSocket query token。

### Agent

- `internal/application/agent/client.go`
  - HTTPS connect URL 转换为 WSS。
  - 继续使用现有认证连接 tunnel。
  - 保持 RuntimeAccess 作为 selected device runtime 边界。

### Frontend

- `web/src/features/api/client.ts`
  - 继续使用相对 `/api` 和 Bearer token。

- `web/src/features/sessions/api.ts`
  - terminal WS 继续相对 URL。
  - create/close/rerun/edit/delete/history/attach 等远端 session 能力保持。

- `web/src/features/sessions/useTerminalSocket.ts`
  - Agent disconnect / terminal error 需要展示结构化 route unavailable 状态。
  - Ctrl+C input path 需要可验证。

### Container / deployment docs

- `Dockerfile`
  - 多阶段构建 Web 与 Go binary。
  - final image 不包含真实 `.env`。

- `.dockerignore`
  - 排除 `.env`、state、logs、node_modules、build cache 等不应进入镜像的内容。

- 配置示例
  - 记录 Cloud Gate container profile、Local Agent profile、HTTPS reverse proxy / WebSocket upgrade 要求和验证步骤；本轮不新增独立部署说明文档。

## Interfaces

### Cloud Gate container profile

示例形态，具体值在部署时提供：

```dotenv
TERMBRIDGE_GATE__LISTEN_URL=http://0.0.0.0:9010
TERMBRIDGE_GATE__BROWSER__ALLOWED_ORIGINS=https://gate.example.com
TERMBRIDGE_GATE__API__EXPOSE_ERRORS=false
TERMBRIDGE_JWT__SECRET_KEY=<secret-managed-random-value>
TERMBRIDGE_RUNTIME__STATE_DIR=/var/lib/termbridge
```

本轮不新增 `auth.username/password` env key；Browser login 与 Agent tunnel 先使用固化 PoC 凭据 `admin/admin`。

Cloud Gate container profile 的约束：

- 不包含真实 `.env`。
- `gate.listen_url` 是内部 HTTP bind 地址，不是 public HTTPS URL。
- 用户系统不是本轮内容；PoC 凭据固定为 `admin/admin`，后续用户系统完成后再设计用户与 device 的绑定关系。

### Local Agent profile

```dotenv
TERMBRIDGE_AGENT__CONNECT_URL=https://gate.example.com
TERMBRIDGE_AGENT__DEVICE_ID=<stable-local-device-id>
TERMBRIDGE_AGENT__DEVICE_NAME=<human-readable-device-name>
```

认证链路与 Cloud Gate 复用现有方式；本轮 PoC 凭据固定为 `admin/admin`。本地 `serve` 启动生成/读取的 `deviceId` 与 `deviceName` 通过 Agent hello 上报。

### Reverse proxy requirements

反向代理 / ingress / LB 必须满足：

```text
https://gate.example.com/*       -> http://termbridge-gate:9010/*
wss://gate.example.com/api/...   -> ws://termbridge-gate:9010/api/...
```

并且：

- 支持 WebSocket upgrade。
- 对 `/api/agent/tunnel` 允许长连接。
- 对 terminal WebSocket 允许长连接和 binary frames。
- 不把 HTTP public endpoint 暴露给用户。

### Browser API

继续沿用当前 device-scoped API，包括远程 session 管理：

```text
POST /api/login
GET  /api/me
GET  /api/devices
GET  /api/devices/{deviceId}/workspaces/tree
GET  /api/devices/{deviceId}/workspaces/{workspaceId}/sessions
POST /api/devices/{deviceId}/workspaces/{workspaceId}/sessions
POST /api/devices/{deviceId}/sessions
GET  /api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/history
GET  /api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/ws
POST /api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/close
POST /api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/rerun
PATCH /api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}
DELETE /api/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}
```

### Terminal interrupt

PoC interrupt 验证以 Ctrl+C input path 为准：

```text
Browser keyboard Ctrl+C -> terminal input byte 0x03 -> selected device PTY
```

## Technical questions

暂无阻塞进入 Plan 的技术问题。Plan 阶段需要选择的实施细节：

1. Go server 提供 `web/dist` 时，静态目录使用配置项还是固定容器路径。
2. PoC 固定 `admin/admin` 后，后续用户系统如何将用户与 device 绑定；该问题不阻塞本轮实现。
3. Dockerfile 是否只提供镜像，还是同时补充 compose 示例；PoC 必须有 Dockerfile，compose 可选。
4. route unavailable 的 terminal 错误是否需要调整后端 frame，还是前端兼容现有关闭行为即可满足 PoC。

## Risks

1. **HTTPS 误解风险**：当前 Go server 不提供 TLS；错误配置 `gate.listen_url=https://...` 不会自动变成安全 HTTPS 服务。
2. **secret 泄露风险**：terminal WS query token、JWT secret、Browser password 可能进入应用日志、代理日志、镜像层或示例文件。
3. **认证复用风险**：Browser 和 Agent tunnel 复用现有认证，适合 PoC，但不是完整 device credential / pairing 模型。
4. **远程 session 管理风险**：创建、rerun、edit、delete 等能力通过云端暴露后，误操作和认证边界风险高于 attach-only PoC。
5. **terminal interrupt 语义不足风险**：Ctrl+C input path 不是完整 interrupt/kill escalation，不能替代 M5/M7 hardening。
6. **route unavailable UX 风险**：如果 terminal disconnect 仍通过裸 text frame 通知，前端可能显示解析错误而不是可理解状态。
7. **静态资源交付风险**：当前没有生产 static serving；若未补齐，Cloud Gate Browser 路径仍依赖 Vite dev server，PoC 不成立。
8. **CORS 误判风险**：`gate.browser.allowed_origins` 不是 REST API CORS；如果不采用同源部署，需要额外设计 CORS。
9. **容器状态风险**：Cloud Gate registry 当前以内存为主，容器重启后 online device 丢失，需要 Agent 重连；这是 PoC 可接受限制，但不能描述成稳定云端 device registry。

## Alternatives

### Alternative A：Cloud Gate 容器必须 gate-only

暂不作为本轮硬要求。用户已确认远程 Gate 所在服务器也是一个环境，是否使用它只是选择问题。本轮优先打通 Cloud Gate + Local Agent 的真实链路；gate-only 可作为后续收口或低成本可选项。

### Alternative B：Go server 直接处理 TLS 证书

不采用为 PoC 默认方案。用户已有服务器 HTTPS 证书，且当前 server 实现不提供 TLS；由反向代理 / ingress / LB 终止 TLS更小、更贴近容器部署。

### Alternative C：前后端分离域名部署

暂不采用。当前 REST API 没有 CORS 交付面，PoC 优先同源部署。若后续需要分离域名，必须单独设计 CORS 和 WebSocket Origin 策略。

### Alternative D：拆分 Browser credential 与 Agent tunnel credential

不采用。用户已确认复用现有认证；完整 pairing / rotation / device credential 拆分留到后续。

### Alternative E：只支持 attach 已存在 session

不采用。用户已确认需要支持会话创建和远程 session 管理。

### Alternative F：把 Ctrl+C 以外的 kill escalation 纳入 PoC

暂不采用。PoC interrupt 先验证 terminal input path。完整 interrupt / close / kill escalation 继续作为 M5/M7 hardening 补证。

## User review notes

用户已确认并覆盖早先草案中的以下点：

1. device 本身就是账户隔离的，云端设备就是云端设备；无需围绕云端 device 混淆额外投入过多。
2. 最低访问控制和风险提示无须额外工作。
3. Agent tunnel 认证复用现有认证链路；本轮 PoC 先固化为 `admin/admin`，用户系统和用户-device 绑定后续再补。
4. 本地启动 `serve` 时生成/读取 `deviceId` + `deviceName`，它们需要由 Agent 上报。
5. PoC 需要支持会话创建和远程 session 管理。
6. Claude Code / Codex 真实 TUI 由用户自行验证。
7. Web 静态资源与 Go binary 构建进入 Dockerfile。
8. 接受同源 HTTPS，由服务器反向代理终止 TLS，TermBridge 容器内部只监听 HTTP。
9. interrupt 以 Ctrl+C 为准。
10. 用户要求进入 Plan。

本 Spec 已按上述反馈更新并标记为 Accepted。
