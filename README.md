# TermBridge-go

TermBridge-go 是 TermBridge 的 Go 重写版本，一个本地浏览器工作区，用于管理终端会话。

## 设计文档

- [设计文档](docs/design.md) - 项目背景、愿景、架构设计、技术路线

## 开发要求

### 开发依赖

- Go 1.25+
- [just](https://github.com/casey/just)

## 后端代码分层

后端代码按主流 Go 项目分层组织在 `internal/` 下：

- `internal/transport/cli`：命令行入口解析与用户输出。
- `internal/transport/http/server`：唯一 HTTP server owner，负责监听、生命周期、全局 middleware 和 route mount。
- `internal/transport/http/localapi`：本地 workbench 的 `/api/...` route adapter。
- `internal/transport/http/gatewayapi`：Gateway Browser API、Agent tunnel 和 terminal WebSocket route adapter。
- `internal/transport/http/middleware/requestlog`：HTTP request/response logging middleware。
- `internal/application`：Agent connector、terminal registry 和 command runner 等 application/use case 层。
- `internal/domain`：workspace、session、process、identity 等核心领域模型。
- `internal/protocol`：terminal 和 tunnel wire protocol。
- `internal/infrastructure`：config、logging、PTY adapter、history writer 和 filesystem repository 实现。

`gatewayapi` 和 `localapi` 只是 HTTP route 模块，不拥有独立后端 server lifecycle；统一后端 server 只在 `internal/transport/http/server` 中存在。

## 开发方式

### 统一后端服务

```bash
just serve
```

`just serve` 通过 Air 热加载启动正式入口 `termbridge serve`。统一后端默认监听 `localhost:9010`，同一个 HTTP server 挂载 local Web API 路由和 `/api/gateway/...` 路由，并同时启动 Agent connector。运行参数来自 `.termbridge.default.yaml` 和 `.termbridge.yaml`。

### 前端开发服务

```bash
just web
```

前端开发服务监听 `localhost:9011`，读取 `web/.env` 中的 `VITE_TERMBRIDGE_BACKEND`，默认指向 `just serve` 提供的 `http://localhost:9010` 后端。

## 使用方式

### 在当前目录运行命令

```powershell
termbridge exec -- claude
```

### 在指定目录运行命令

```powershell
termbridge --cwd D:\project exec -- claude
```

### 查看本地会话和工作区

```powershell
termbridge session
termbridge workspace
```

### 启动统一后端服务

```powershell
termbridge serve
```

`serve` 启动后只有一个后端 HTTP server，并挂载：

- `/api/...`：本地 workbench 所需的 workspace/session/history/terminal API。
- `/api/gateway/...`：Browser API、terminal WebSocket、device registry、routing 和 relay。
- Agent connector：按配置主动连接目标后端，并通过本地 runtime adapter 暴露 workspace/session/history/terminal attach 能力。

Gateway 路由不拥有 PTY 或 process lifecycle；用户进程仍由本地 TermBridge runtime 管理。

### Gateway/Agent 配置

运行参数通过配置文件声明，不通过 `termbridge serve` 参数传递。

默认配置在 `.termbridge.default.yaml`，项目级覆盖写入 `.termbridge.yaml`：

```yaml
serve:
  host: 127.0.0.1
  port: 9010
  open: false
  dev: false

agent:
  server_url: http://127.0.0.1:9010
  device_name: local-dev
```

其中：

- `serve.host` / `serve.port` 控制统一后端监听地址。
- `serve.open` / `serve.dev` 控制统一后端的本地开发行为。
- `agent.server_url` 控制 Agent connector 连接哪个统一后端地址。
- `agent.device_name` 控制设备显示名；为空时由本地 device identity 逻辑决定。

### 前端访问面

前端只有一套。当前同一前端承载 local 和 Gateway 访问面：

- `/`：本地 workbench 访问面。
- `/gateway`：Gateway 访问面。

M6 Gateway MVP 当前支持：

- 查看 device / workspace / session。
- 读取 session history。
- attach 已存在 terminal session。

M6 Gateway MVP 当前不支持：

- 通过 Gateway 创建 session。
- 通过 Gateway 修改 workspace/session。
- 正式用户系统、device pairing、token rotation 或生产部署能力。
