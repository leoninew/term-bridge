# TermBridge-go

TermBridge-go 是 TermBridge 的 Go 重写版本，当前主要服务于本地开发和产品验证。

核心模型：

```text
Browser -> Gate -> Agent -> Runtime -> PTY / Process
```

产品资源模型：

```text
User -> Device -> Workspace -> Session -> Terminal
```

## 设计文档

- [产品设计](docs/design/design.md)
- [产品路线图](docs/design/roadmap.md)

![](./assets/sketch.png)

## 当前状态

当前重点是本地 self-connected Gate 和统一 `/sessions` 工作台：

- `termbridge serve` 启动后端 Gate、Local Agent 和本地 runtime。
- Browser 通过 `/sessions` 选择 device、workspace、session 并 attach terminal。
- 本地开发采用前后端分离。
- 镜像交付采用前后端一体：Go 服务提供后端 API 和已构建的前端静态资源。

Cloud Gate PoC 阶段仍是验证态，不是生产发布态。Browser login 已切到 `/api/auth/*` 用户系统；本地模式仍支持配置里的 `admin/admin` shortcut，远程 Gate 模式要求配置 Google OAuth 与 Resend。device 绑定、pairing、credential rotation 后续再补。

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

启动后端：

```bash
task serve
```

启动前端开发服务：

```bash
task web
```

默认本地开发地址：

```text
Backend: http://127.0.0.1:9030
Frontend: http://localhost:9031
```

## 常用命令

运行测试：

```bash
task test
```

运行完整检查：

```bash
task check
```

构建本地二进制和镜像：

```bash
task build
```

## 配置

默认配置来自 `.termbridge.default.yaml`，项目级覆盖写入 `.termbridge.yaml`。也可以使用项目根目录 `.env` 或系统环境变量覆盖配置。

配置优先级：

```text
.termbridge.default.yaml
  < .termbridge.yaml
  < .env
  < OS env
```

环境变量命名规则：

```text
TERMBRIDGE_ + 配置 key 大写，并将 . 转为 __
```

本地开发示例：

```dotenv
TERMBRIDGE_AGENT__LISTEN_URL=http://127.0.0.1:9030
TERMBRIDGE_GATE__BROWSER__ALLOWED_ORIGINS=http://localhost:9031
TERMBRIDGE_RUNTIME__STATE_DIR=.termbridge
TERMBRIDGE_WEB__STATIC_DIR=
TERMBRIDGE_AGENT__PUBLIC_URL=http://localhost:9031
```

镜像内置静态资源示例：

```dotenv
TERMBRIDGE_AGENT__LISTEN_URL=http://0.0.0.0:80
TERMBRIDGE_WEB__STATIC_DIR=/opt/termbridge/web/dist
TERMBRIDGE_AGENT__PUBLIC_URL=http://localhost
TERMBRIDGE_RUNTIME__STATE_DIR=/var/lib/termbridge
```

用户系统相关配置：

```dotenv
TERMBRIDGE_DATABASE__DRIVER=sqlite
TERMBRIDGE_DATABASE__SQLITE__PATH=.termbridge/termbridge.db
TERMBRIDGE_AUTH__LOCAL_ADMIN__USERNAME=admin
TERMBRIDGE_AUTH__LOCAL_ADMIN__PASSWORD=admin
TERMBRIDGE_AUTH__GOOGLE__CLIENT_ID=
TERMBRIDGE_AUTH__GOOGLE__CLIENT_SECRET=
TERMBRIDGE_AUTH__GOOGLE__REDIRECT_URL=
TERMBRIDGE_RESEND__API_KEY=
TERMBRIDGE_RESEND__FROM_EMAIL=
```

`agent.connect_url` 与 `agent.listen_url` 规范化后一致时为本地模式；本地模式支持 `auth.local_admin` shortcut。两者不一致时为远程 Gate 模式，启动时会要求 Google OAuth 与 Resend 配置齐全。

`agent.device_id` 和 `agent.device_name` 为空时，`termbridge serve` 会生成并写入 `.termbridge.yaml`，随后由 Agent 上报给 Gate。

## 当前边界

当前还不是完整生产版本，仍未完成：

- 用户与 device 绑定。
- secure pairing。
- device credential / rotation。
- 完整远程 Gate + 独立 Local Agent 的真实部署验证。
- 发布包、安装升级、生产运维文档。

README 当前只作为开发入口说明；正式发布前会重写用户向文档。
