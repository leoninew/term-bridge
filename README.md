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

Cloud Gate PoC 阶段仍是验证态，不是生产发布态。当前 Browser login 与 Agent tunnel 暂时固定为 `admin/admin`；正式用户系统、device 绑定、pairing、credential rotation 后续再补。

## 开发依赖

- Go 1.25+
- Node.js / Yarn
- [just](https://github.com/casey/just)
- [Air](https://github.com/air-verse/air)

安装依赖：

```bash
just install
```

## 本地开发

启动后端：

```bash
just serve
```

启动前端开发服务：

```bash
just web
```

默认本地开发地址：

```text
Backend: http://127.0.0.1:9010
Frontend: http://127.0.0.1:9011
```

## 常用命令

运行测试：

```bash
just test
```

运行完整检查：

```bash
just check
```

构建本地二进制和镜像：

```bash
just build
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

示例：

```dotenv
TERMBRIDGE_GATE__LISTEN_URL=http://127.0.0.1:9010
TERMBRIDGE_GATE__BROWSER__ALLOWED_ORIGINS=http://127.0.0.1:9011
TERMBRIDGE_RUNTIME__STATE_DIR=.termbridge
TERMBRIDGE_WEB__STATIC_DIR=/opt/termbridge/web/dist
```

`agent.device_id` 和 `agent.device_name` 为空时，`termbridge serve` 会生成并写入 `.termbridge.yaml`，随后由 Agent 上报给 Gate。

## 当前边界

当前还不是完整生产版本，仍未完成：

- 正式用户系统。
- 用户与 device 绑定。
- secure pairing。
- device credential / rotation。
- 完整远程 Gate + 独立 Local Agent 的真实部署验证。
- 发布包、安装升级、生产运维文档。

README 当前只作为开发入口说明；正式发布前会重写用户向文档。
