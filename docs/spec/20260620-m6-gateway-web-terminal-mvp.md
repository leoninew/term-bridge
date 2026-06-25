# M6 Gateway Web Terminal MVP 规格
最后修改时间: 2026-06-22 14:25:00

Review status: Accepted

## Requirement basis

依据：`docs/requirement/20260620-m6-gateway-web-terminal-mvp.md`

当前 Requirement 已接受，M6 采用严格模式 / strict。M6 的核心目标是引入 Gateway 作为正式远程 Web 入口，同时保持 TermBridge runtime ownership：Gateway 不拥有 PTY、不直接运行用户命令、不拥有 Process lifecycle。

用户已确认的关键决策：

1. M6 支持 attach/list 已存在 session，不通过 Gateway 创建新 session。
2. Gateway service 与 Agent connector 使用同一个 `termbridge serve` 统一入口启动；历史独立 Gateway/Agent subcommand 不再保留。
3. Agent tunnel 采用单 WebSocket 多路复用。
4. 正式用户系统稍后实现，M6 使用临时 `admin/admin` 用户。
5. M6 支持多个会话。
6. 可单元测试和 Pomelo PW 测试的自行组合验证，剩余项列入人工测试列表。
7. 继续在当前前端上实现 Gateway 能力；项目只有一个前端，不引入 Gateway mode 或第二套前端。
8. Gateway shallow cache 采纳：Gateway 可保留显示/路由用快照，但不作为 runtime authoritative state。
9. 每个用户连接自己的 Agent、工作区和会话；每个 session 只能有一个前端 browser 写入。

## Overview

M6 在现有 local CLI/runtime 和 local Web/workbench 之上增加 Gateway/Agent 拓扑：Browser 通过 Gateway 访问远程 Web 入口，Gateway 通过单 WebSocket 多路复用 tunnel relay 到 Agent，Agent 通过本地 `webterminal.Registry` / runtime adapter 访问 workspace、session、history 和 terminal attach 能力。

Gateway 是远程入口和 relay，不是 runtime。Agent 与本地 runtime 同进程/同 binary 内运行，主动连 Gateway，并通过 tunnel 响应 Gateway 的 registry/list/attach 请求。Browser 访问 Gateway，经过临时 `admin/admin` auth 后查看 device、workspace、session，并 attach 到已有 session。

组件仍复用当前 workbench、sidebar、history 和 terminal 组件；local 与 Gateway 的区别集中在当前 backend 的 API path、terminal WebSocket URL、以及 Gateway 下 mutation 能力禁用。架构图和流程图作为验证交付物记录在 `docs/verification/20260620-m6-gateway-web-terminal-mvp.md`。

M6 不支持通过 Gateway 创建新 session。用户可通过本地 CLI/local Web/workbench 创建 session；Gateway 只 list/attach 已存在 session。这样避免在 M6 同时引入远程命令启动、cwd/env 安全边界、远程 session creation authorization 等 M7/M8 问题。

## Design decisions

### 1. Same binary, unified serve entry

M6.1 后 Gateway service 与 Agent connector 不再作为独立用户模式暴露，统一由现有 `termbridge` binary 的 `serve` command 启动：

```text
termbridge serve
```

现有命令继续保持：

```text
termbridge exec -- <command...>
termbridge web --dev
termbridge session
termbridge workspace
```

CLI parser 只需要暴露 `serve` command kind。Gateway/Agent 运行参数不通过 CLI flag 传递，统一从 `.termbridge.default.yaml` 和 `.termbridge.yaml` 读取。Gateway/Agent 仍应拆成独立 internal packages，避免把 relay 状态混入 local webterminal runtime。

### 2. Gateway and Agent packages are separate from runtime ownership

建议新增包边界：

```text
internal/gateway
internal/agent
internal/tunnel
internal/gatewayauth
```

职责：

- `internal/gateway`: HTTP server、Browser API、device registry、session route、terminal relay route。
- `internal/agent`: outbound tunnel client、local runtime adapter、workspace/session listing provider、terminal attach provider。
- `internal/tunnel`: 单 WebSocket 多路复用协议、frame encode/decode、stream lifecycle、backpressure/error/close 语义。
- `internal/gatewayauth`: M6 临时 `admin/admin` auth，后续可替换为正式用户系统。

禁止：Gateway 直接 import `internal/pty/gopty` 或创建 PTY。Gateway 不能调用 `runner.CommandRunner` 运行命令。

### 3. Single WebSocket multiplexed tunnel

Agent 到 Gateway 使用一条 WebSocket tunnel，承载多种 logical streams：

```text
control stream: device hello, heartbeat, registry sync, errors
request stream: list workspaces, list sessions, read history metadata
terminal stream: attach terminal, input, output, resize, close
```

所有 tunnel frame 都带 `stream_id` 和 `type`。Gateway 根据 `device_id + stream_id` 路由 Browser 请求与 Agent 响应。

M6 最小 frame 类型：

```text
hello
hello_ack
ping
pong
request
response
terminal_attach
terminal_input
terminal_output
terminal_resize
terminal_closed
error
close
```

M6 不实现复杂可靠消息队列、不做断线后 terminal stream 自动续传；Agent reconnect 后重新注册 device，Browser 需要重新 attach。

### 4. Temporary admin/admin auth

M6 使用临时用户：

```text
username: admin
password: admin
```

目标是让 Gateway API/Web UI 有统一 auth 边界。正式用户系统稍后实现。

建议采用最小 session cookie：

- `POST /api/gateway/login` with `{ username, password }`
- 成功后设置 HttpOnly cookie。
- `POST /api/gateway/logout` 清理 cookie。
- Browser API 和 terminal WebSocket 都要求 cookie auth。

M6 不实现 password hashing、user database、token rotation、RBAC、remember me、OAuth。为了避免误用，Gateway 启动日志和 help 应明确这是 MVP auth。

### 5. Multiple sessions supported, session creation excluded

M6 支持多个 session 的含义：

- Gateway 可 list 多个 session。
- Browser 可选择不同 session 并分别 attach。
- Gateway/Agent tunnel 可同时存在多个 terminal streams。

M6 明确每个 session 只能有一个前端 browser 写入。产品模型是每个用户连接自己的 Agent、工作区和会话；M6 不做多 browser 同时写同一 session 的协作能力。多用户/多浏览器协作行为进入后续阶段。

### 6. Gateway capability is implemented in the current frontend

M6 继续在当前 `web/` 前端上实现 Gateway 能力。项目只有一个前端，不引入 Gateway mode 概念，也不新建第二套 frontend。

理由：

- 当前 `web/` 已具备 workbench、session list、xterm、history、Pomelo PW flow 基础。
- Gateway 能力的主要新增点是登录、device selection、API path、WebSocket URL 和 disconnected 状态展示。
- 重建 frontend 会重复 M2.5/M5 已验证的 xterm 和 backpressure 工作。

实现上应抽象 session/workspace API 客户端和 WebSocket URL 生成逻辑，使当前前端可以访问 Gateway API，同时避免在组件中散落硬编码路径。当前实现采用 `activeBackend` 收敛 local/Gateway 差异：组件复用，路径和 mutation 能力由 backend abstraction 决定。

### 7. Device identity stored locally

M6 采用本地 state dir 中稳定生成 device id：

```text
.termbridge/device.json
```

最小字段：

```json
{
  "id": "...",
  "name": "hostname or configured name",
  "created_at": "..."
}
```

Agent 启动时读取或生成 device identity，并在 tunnel hello 中发送。后续 M7 可扩展 pairing、rename、device registry persistence。

### 8. Gateway holds routing state and shallow registry snapshots

Gateway 可保存在线 device 的 shallow state：

- device id/name
- online/offline
- tunnel connected time
- last heartbeat
- latest workspace/session listing snapshot

Gateway 不保存 runtime authoritative state。workspace/session listing 由 Agent relay 当前 runtime 状态；Gateway cache 只用于 UI 快速显示、路由判断和 disconnect 后状态提示。shallow cache 的含义是：Gateway 可以暂存 device/session/workspace 的显示用快照和 route metadata，但这些数据不是事实来源；Agent/local runtime 才是 session lifecycle、process、history、workspace state 的事实来源。

### 9. Disconnect semantics are explicit

Agent disconnect 后：

- Gateway 标记 device offline。
- 关闭该 device 下所有 active terminal relay streams。
- Browser terminal 显示明确错误：`device disconnected` 或同等文案。
- 新 attach 请求返回 route unavailable。

Agent reconnect 后：

- Gateway 重新标记 device online。
- 旧 tunnel 不再接收新路由。
- Browser 可刷新 session list 并重新 attach。

## Affected components

### CLI / app command dispatch

- `internal/cli/cli.go`
- `internal/cli/cli_test.go`
- `internal/app/app.go`
- `internal/app/app_test.go`

暴露：

```text
termbridge serve
```

Gateway/Agent 参数通过配置表达：

```yaml
gateway:
  host: 127.0.0.1
  port: 8080
  open: false
  dev: false

agent:
  gateway_url: http://127.0.0.1:8080
  device_name: local-dev
```

Auth 暂不暴露复杂配置，M6 默认 `admin/admin`。

### Gateway service

新增：

- `internal/gateway`
- `internal/gatewayauth`

职责：

- Gateway HTTP server。
- Login/logout/session auth middleware。
- Browser API：devices、workspaces、sessions、history metadata 或 relay-only endpoint。
- Browser terminal WebSocket accept。
- Agent tunnel WebSocket accept。
- Online device registry。
- Route cleanup。

### Agent service

新增：

- `internal/agent`

职责：

- 读取/generate device identity。
- 主动连接 Gateway tunnel。
- 使用本地 `state.Store` / `webterminal.Registry` / runtime adapter 提供 workspace/session listing。
- 接收 Gateway attach 请求，并桥接 local session terminal stream。
- reconnect/backoff。

### Tunnel protocol

新增：

- `internal/tunnel`

职责：

- JSON control frame。
- Gateway ↔ Agent tunnel 内部 terminal output frame 编码；不改变 Browser ↔ Gateway terminal WebSocket 语义。
- stream id 分配。
- per-stream close/error。
- bounded queue/backpressure。

### Existing local Web/workbench

- `web/src/features/sessions/api.ts`
- `web/src/features/workspaces/api.ts`
- `web/src/features/sessions/useTerminalSocket.ts`
- `web/src/App.vue`
- possibly new gateway login/device selection components

原则：只有一个前端；在当前 UI 上增加 Gateway 能力，不新建 Gateway mode 或第二套 frontend。

### Web terminal/runtime

- `internal/webterminal/*`
- `internal/webserver/*`

M6 需要复用 local session list / attach 能力，但 Gateway 不能直接拥有 runtime。Agent 可以在本地进程内使用已有 runtime adapter。

## Interfaces

### CLI shape

```text
termbridge serve
```

Gateway listen address、Agent upstream URL 和 device name 由配置控制，不通过 CLI 参数传递。

### Browser Gateway API

最小 API：

```text
POST /api/gateway/login
POST /api/gateway/logout
GET  /api/gateway/me
GET  /api/gateway/devices
GET  /api/gateway/devices/{device_id}/workspaces/tree
GET  /api/gateway/devices/{device_id}/sessions
GET  /api/gateway/devices/{device_id}/sessions/{session_id}/history
GET  /api/gateway/devices/{device_id}/sessions/{session_id}/ws
```

M6 不提供：

```text
POST /api/gateway/devices/{device_id}/sessions
```

即不支持 Gateway 创建 session。

### Agent tunnel endpoint

```text
GET /api/agent/tunnel
```

需要 Agent auth。M6 可复用临时 admin/admin 派生的 basic credential 或专用 gateway agent secret；为避免扩展正式 auth，Spec 倾向使用同一临时凭据，但在 Plan 中需明确传输方式。

### Tunnel frame model

控制 frame：

```json
{
  "stream_id": "control",
  "type": "hello",
  "payload": {
    "device_id": "...",
    "device_name": "...",
    "protocol_version": 1
  }
}
```

请求/响应 frame：

```json
{
  "stream_id": "req-...",
  "type": "request",
  "payload": {
    "method": "workspace_tree"
  }
}
```

Terminal frame：

```json
{
  "stream_id": "term-...",
  "type": "terminal_resize",
  "payload": { "cols": 120, "rows": 32 }
}
```

Browser ↔ Gateway 的 terminal WebSocket 输出语义保持现有 xterm terminal output 模型不变。JSON/base64 仅作为 Gateway ↔ Agent 单 WebSocket 多路复用 tunnel 的内部 frame 编码候选；若大输出验证显示 tunnel 编码开销明显，再将内部 terminal output frame 优化为 binary framing。

## Technical questions

1. Gateway ↔ Agent tunnel 内部 terminal output frame 采用 binary multiplex framing 还是 JSON/base64；该选择不改变 Browser ↔ Gateway terminal WebSocket 输出语义。
2. Agent tunnel auth 是否复用 `admin/admin`，还是增加单独 `agent_secret`？
3. 当前前端的 API base 如何注入：build-time env、runtime config endpoint，还是 URL path detection？
4. 当前前端如何组织 Gateway 能力相关组件：在同一 `App.vue` 中接入，还是通过 route/component split 区分登录、device selection 和 workbench？
5. Agent 的 local runtime adapter 是否直接复用 `webterminal.Registry`，还是定义更窄的 `RuntimeAccess` interface？
6. Gateway cache 的过期策略是什么：Agent heartbeat 更新，还是每次 Browser 请求都 request/reply relay？
7. 多个 session 同时 attach 的 backpressure 是 tunnel-level queue，还是 per-stream queue？

## Risks

1. 临时 `admin/admin` 不适合公网或生产，只能作为 M6 MVP 受控环境验证。
2. 单 WebSocket 多路复用如果缺少 per-stream backpressure，会让一个慢 terminal stream 影响其他 session/listing/control stream。
3. 在唯一前端中加入 Gateway 能力时，若 API/client 抽象不清，会把 local API 和 Gateway API 混杂在组件层。
4. Agent 使用本地 runtime adapter 时，如果接口过宽，可能让 Gateway 间接获得 runtime ownership。
5. 不支持 Gateway session creation 会降低远程入口完整度，但能显著降低 M6 范围和安全风险。
6. Agent reconnect 处理不完善会产生 stale route、重复 device、Browser attach 到失效 tunnel 等问题。
7. M5 未完成的真实 TUI/20+ session/Windows cleanup 验证仍可能在 M6 人工验证中暴露问题。

## Alternatives

### Alternative A: Gateway 创建 session

不采用。用户已确认 M6 支持 attach/list 已存在 session。远程创建 session 涉及 cwd/env/command authorization 和安全边界，留到后续阶段。

### Alternative B: 拆分 gateway/agent binary 或保留独立 gateway/agent subcommand

不采用。用户已确认 Gateway/Agent 对外收口为统一 `termbridge serve` 启动入口；启动后同时具备 Gateway service 和 Agent connector 能力。

### Alternative C: REST + WebSocket 多连接 tunnel

不采用作为默认方案。用户已确认 Agent tunnel 采用单 WebSocket 多路复用。

### Alternative D: 新建 Gateway frontend 或引入 Gateway mode

不采用。用户已确认项目只有一个前端，M6 继续在当前 `web/` 上实现 Gateway 能力，不新建第二套 frontend，也不引入 Gateway mode 概念。

### Alternative E: 正式用户系统

不采用。用户已确认正式用户系统稍后实现，M6 使用临时 `admin/admin`。

## User review notes

本 Spec 已根据用户确认决策设计：

1. attach/list only，不做 Gateway session creation。
2. same binary, unified `serve` entry。
3. single WebSocket multiplexed tunnel。
4. temporary `admin/admin` auth。
5. 支持多个 session。
6. 单元测试 + Pomelo PW + 人工测试组合验证。

用户已补充并确认：

1. 继续在当前前端上实现 Gateway 能力；不是 Gateway mode，项目只有一个前端。
2. 接受 device identity 存在本地 state dir 的 `device.json`。
3. 接受 Gateway 保留 shallow cache，但不作为 authoritative runtime state。
4. 每个用户连接自己的 Agent、工作区和会话；每个 session 只能有一个前端 browser 写入。
5. 接受 Agent disconnect 时 Browser terminal 显示 `device disconnected` 并要求用户重新 attach。
6. 用户已确认进入 Plan。
