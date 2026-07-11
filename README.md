# TermBridge-go

TermBridge-go 是 TermBridge 的 Go 重写版本，当前主要服务于本地开发和产品验证。

核心模型：

```text
Browser -> Gate -> Agent -> Runtime -> PTY / Process
User -> Device -> Workspace -> Session -> Terminal
```

## 文档

- [产品设计](docs/design/design.md)
- [产品路线图](docs/design/roadmap.md)

![](./assets/sketch.png)

## 当前形态

- `termbridge agent` 启动本地 Agent 后端、本机 runtime 与 PTY 能力。
- `termbridge cloud` 启动 Cloud 门户后端。
- Browser 通过 `/sessions` 进入 workspace、session 和 terminal。
- 本地开发使用一个 Vite dev server 同时代理 agent 与 cloud。
- 镜像交付使用 Go 后端提供 API 和已构建的前端静态资源。

## 开发依赖

- Go 1.25+
- Node.js / Yarn
- [Task](https://taskfile.dev/)
- [Air](https://github.com/air-verse/air)

安装依赖：

```bash
task install
```

## 本地开发

同时启动 Web、agent 与 cloud：

```bash
task run
```

也可以分别启动：

```bash
task agent
task cloud
task web
```

默认本地联调地址：

```text
web:   http://localhost:9030
agent: http://127.0.0.1:9031
cloud: http://127.0.0.1:9032
```

Vite dev proxy：

```text
/local-api/* -> http://127.0.0.1:9031
/cloud-api/* -> http://127.0.0.1:9032
```

## 常用命令

```bash
task test    # 运行测试
task check   # 运行完整检查
task build   # 构建本地二进制和镜像
```

## 配置

后端配置从 `configs/config.yaml` 开始加载，并可被环境配置、`.env` 和 OS env 覆盖。环境变量命名规则：

```text
TERMBRIDGE_ + 配置 key 大写，并将 . 转为 __
```

加载顺序：

```text
configs/config.yaml
  < configs/config.<TERMBRIDGE_ENV>.yaml
  < .env
  < .env.<TERMBRIDGE_ENV>
  < OS env
```

`TERMBRIDGE_ENV` 只从 OS env 读取；`.env` / `.env.<env>` 不覆盖已存在的 OS env。根目录 `.env` / `.env.<env>` 是本机运行覆盖文件，默认不提交；可从 `.env.example` 复制后按环境维护。

`task agent`、`task cloud` 和 `task run` 会设置 `TERMBRIDGE_ENV=development`，Air 启动时读取 `configs/config.development.yaml` 和根目录 `.env.development`。

前端仅保留 `web/.env.development` 作为 Vite 本地联调配置，使用 `TERMBRIDGE_LOCAL__MODE=hybrid`。`build:local` 和 `build:cloud` 保留产品线脚本名称，但生成不含部署地址的相同静态制品；测试和正式部署的公开配置由 Go 服务在运行时提供。

```bash
yarn --cwd web build:local
yarn --cwd web build:cloud
```

Docker 镜像不固化任何 `TERMBRIDGE_*` 运行时环境变量，部署时必须显式提供 Cloud 服务配置。

Windows portable package 是本地 Agent 包：`task package` 使用 `build:local` 构建前端，并同时包含 `.env.prod` 和 `.env.test`。启动脚本默认选择 `.env.prod`（Preflite HTTPS）；调用方设置 `TERMBRIDGE_ENV=test` 时选择 `.env.test`（lvh HTTP）。两个 profile 都是 Go 后端运行时配置，负责本地静态目录、本机访问地址、云端地址和 Agent OAuth client secret；不要与前端开发环境文件混淆。

## 运行角色

运行角色由 CLI 子命令选择：

```bash
termbridge agent
termbridge cloud
```

设备身份由 `termbridge agent` 在 `<runtime.state_dir>/device.json` 中生成并读取。Cloud 绑定摘要只记录非 token 的 public URL、device id、device name 和连接时间。
