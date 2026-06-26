# TermBridge-go

TermBridge-go 是 TermBridge 的 Go 重写版本，目标是提供一个由 TermBridge 自主管理的 workspace / session / terminal runtime。它不再把 ttyd、tmux 或本地直连 API 当作核心架构，而是统一收敛到：

```text
Browser -> Gate -> Agent -> Runtime -> PTY / Process
```

本地使用时，Gate 和 Agent 可以在同一个 `termbridge serve` 进程内 self-connected；接入云端时，Agent 连接远端 Gate，但 Browser、Device、Workspace、Session、Terminal 的产品模型保持一致。

## 设计文档

- [架构设计](docs/design/design.md) - 当前产品模型、目标架构、组件职责、运行链路和安全边界
- [产品路线图](docs/design/roadmap.md) - 当前阶段、后续优先级、验收门和发布路线

## 用户视角

### 1. 直接运行一次命令

在当前目录启动一个由 TermBridge 记录的 terminal session：

```powershell
termbridge exec -- claude
```

指定工作目录：

```powershell
termbridge --cwd D:\project exec -- claude
```

运行链路：

```text
termbridge exec
  -> resolve workspace
  -> create session record
  -> create PTY
  -> run command
  -> write history / lifecycle state
```

### 2. 查看本地工作区和会话

```powershell
termbridge workspace
termbridge session
```

这两个命令面向本地 CLI 视角，用于快速查看 TermBridge 已记录的 workspace 和 session。

### 3. 启动统一工作台

```powershell
termbridge serve
```

然后打开 Web 工作台：

```text
http://127.0.0.1:9010/sessions
```

当前前端只有一套主要产品入口：`/sessions`。它以 Device 为第一层资源，再进入 Workspace、Session 和 Terminal。

本地 self-connected 流程：

```text
User
  -> termbridge serve
      -> HTTP Gate listens on 127.0.0.1:9010
      -> Local Agent connects back to the Gate
      -> Local Device becomes online
  -> Browser /sessions
      -> select Device
      -> browse Workspaces
      -> create / attach / close / rerun Sessions
      -> interact with Terminal
```

对应运行链路：

```text
Browser /sessions
  -> Gate Browser API
  -> Device route
  -> Agent tunnel
  -> Runtime adapter
  -> Workspace / Session registry
  -> PTY / Process
```

Gateway / Browser API 不拥有 PTY 或 process lifecycle；用户进程仍由 Agent 所在机器上的本地 TermBridge runtime 管理。

### 4. 本地和云端的区别

本地使用时，Gate 监听本机地址，Agent 的 `connect_url` 留空时自动回落到 `gate.listen_url`：

```yaml
gate:
  listen_url: http://127.0.0.1:9010

agent:
  connect_url: ""
```

云端接入时，Agent 连接远端 Gate：

```yaml
agent:
  connect_url: https://gate.example.com
```

产品模型不变：

```text
User -> Device -> Workspace -> Session -> Terminal
```

变化的只是 Gate URL 和部署位置。

### 当前能力边界

当前重点是本地 self-connected Gate 和 unified device workbench。仍未完成正式生产级能力：正式用户系统、device pairing、device credential、token rotation 和生产部署流程。

## 开发者视角

### 开发依赖

- Go 1.25+
- Node.js / Yarn（前端开发）
- [just](https://github.com/casey/just)
- [Air](https://github.com/air-verse/air)（`just serve` 热加载）

安装依赖：

```bash
just install
```

### 本地开发流程

启动统一后端：

```bash
just serve
```

`just serve` 通过 Air 热加载正式入口 `termbridge serve`，默认监听 `127.0.0.1:9010`。同一个进程中会启动：

```text
termbridge serve
  -> create runtime registry
  -> start HTTP Gate
  -> start Agent connector
  -> Agent connects to configured Gate URL
```

启动前端开发服务：

```bash
just web
```

前端开发服务默认监听 `localhost:9011`，通过 `web/.env` 中的 `VITE_TERMBRIDGE_BACKEND` 指向后端。

运行测试：

```bash
just test
```

运行完整检查：

```bash
just check
```

构建：

```bash
just build
```

### 后端架构

后端代码按 transport / application / domain / infrastructure 分层组织在 `internal/` 下：

```text
cmd/termbridge
  -> internal/transport/cli
      -> internal/app
          -> internal/application
              -> internal/domain
              -> internal/protocol
              -> internal/infrastructure

internal/transport/http/server
  -> owns the single HTTP server lifecycle
  -> mounts gatewayapi as the unified API handler

internal/transport/http/gatewayapi
  -> Browser auth and device-scoped workbench routes
  -> Agent tunnel route at /api/agent/tunnel
  -> Terminal WebSocket relay
  -> does not own PTY / Process lifecycle

internal/application/agent
  -> outbound Agent connector
  -> bridges tunnel requests to local runtime adapter

internal/application/terminal
  -> workspace / session / terminal registry
  -> creates and attaches PTY-backed sessions
```

架构边界：

```text
Browser
  |
  v
HTTP server
  |
  v
Gateway API route adapter
  |
  v
Agent tunnel / route
  |
  v
Agent runtime adapter
  |
  v
Terminal registry
  |
  v
PTY / Process
```

`gatewayapi` 只是 HTTP route adapter，不是独立后端服务；唯一 HTTP server owner 是 `internal/transport/http/server`。Gateway 负责认证、设备路由、relay 和错误边界，但不直接创建 PTY，也不直接拥有用户命令生命周期。

### 前端架构

前端的长期产品入口是 `/sessions`：

```text
/sessions
  -> device selection
  -> workspace tree
  -> session list / detail
  -> terminal attach
```

前端业务代码应通过统一 device-scoped workbench 模型组织，不再区分“本地页面”和“Gateway 页面”。`/gateway` 不应作为新的产品入口恢复。

### 配置模型

默认配置来自 `.termbridge.default.yaml`，项目级覆盖写入 `.termbridge.yaml`。如果需要临时覆盖配置，也可以在项目根目录放置 `.env`；OS env 仍具有最高优先级：

```text
.termbridge.default.yaml
  < .termbridge.yaml
  < .env
  < OS env
```

`.env` 不会覆盖 Docker / systemd / Kubernetes / CI / shell 已注入的同名环境变量。环境变量命名规则是 `TERMBRIDGE_` + 配置 key 大写，并将 key 中的 `.` 转为 `__`，例如 `gate.listen_url` 对应 `TERMBRIDGE_GATE__LISTEN_URL`。可复制 `.env.example` 作为本地模板。

最小本地配置形态：

```yaml
log:
  http:
    request_body_limit: 4096
    response_body_limit: 4096

runtime:
  state_dir: .termbridge

gate:
  listen_url: http://127.0.0.1:9010
  browser:
    allowed_origins:
      - http://localhost:9011
  api:
    expose_errors: false

agent:
  connect_url: ""
  device_id: ""
  device_name: ""
```

关键配置含义：

- `log.http.request_body_limit`：request JSON body 日志最多记录多少字节；`<= 0` 视为 `0`，即不记录 request body。
- `log.http.response_body_limit`：response JSON body 日志最多记录多少字节；`<= 0` 视为 `0`，即不记录 response body。
- `runtime.state_dir`：本地 runtime 状态目录，包含 workspace/session/history 和设备运行记录。
- `gate.listen_url`：统一 Gate 的监听地址。
- `gate.browser.allowed_origins`：Browser WebSocket / API 允许的前端 origin；env 形式使用逗号分隔，例如 `TERMBRIDGE_GATE__BROWSER__ALLOWED_ORIGINS=http://localhost:9011,http://127.0.0.1:9011`。
- `gate.api.expose_errors`：是否在 API 错误响应中输出非敏感底层异常详情；生产环境应保持 `false`。
- `agent.connect_url`：Agent connector 连接哪个 Gate；为空时默认使用 `gate.listen_url`，用于本地 self-connected。
- `agent.device_id` / `agent.device_name`：Agent 代表的设备身份和显示名；为空时 `termbridge serve` 会写入 `.termbridge.yaml`。

常用 env 覆盖示例：

```dotenv
TERMBRIDGE_GATE__LISTEN_URL=http://127.0.0.1:9010
TERMBRIDGE_AGENT__CONNECT_URL=https://gate.example.com
TERMBRIDGE_GATE__API__EXPOSE_ERRORS=false
TERMBRIDGE_LOG__HTTP__REQUEST_BODY_LIMIT=4096
```

`.termbridge/device.json` 只保存本地临时登录凭据（`auth.username/password`）。Agent 的设备配置归 `.termbridge.yaml` 的 `agent` 节点承担，Agent 运行时设备记录则位于 `.termbridge/devices/<device_id>/device.json`。

### 路由设计原则

README 不维护接口清单，具体接口以代码和测试为准。开发时只需要记住三个边界：

1. Browser workbench 走统一 device-scoped 模型。
2. Agent connector 使用内部 tunnel 入口 `/api/agent/tunnel`。
3. PTY / Process lifecycle 只属于 Agent 所在机器上的 runtime，不属于 Gateway route adapter。
