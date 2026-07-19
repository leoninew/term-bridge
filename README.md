# TermBridge

**中文** | [English](README.en.md)

**TermBridge** 是一个本地优先、可远程访问的 Workspace Runtime 平台。

它把你在某一台设备上运行的 CLI、Shell、构建与调试任务，整理成可管理、可恢复、可在浏览器中继续操作的工作流。浏览器负责查看与交互，命令仍然真实运行在设备本地的 runtime 上。


## 适合谁

- **本地开发者**：在本机运行 Claude Code、Codex、Shell、构建命令，希望在浏览器里统一查看、恢复和管理这些会话。
- **多设备用户**：把 Home PC、笔记本、VPS 等设备放进同一工作台，并始终清楚当前操作的是哪一台。
- **远程访问用户**：本地 Agent 主动连接云端，浏览器经云端访问本机 runtime，无需给设备开放入站端口。

## 能力概览

![](./assets/screenshot.png)

- **设备与工作区**：按 Device → Workspace → Session 组织任务，状态归属清晰。
- **浏览器工作台**：在 Web 中管理 session、查看历史、attach 终端。
- **远程终端**：基于 PTY 的交互体验，支持重连、resize 等常见终端行为。
- **代码工作台**：在在线设备上使用类 VS Code 的编辑与文件浏览能力。
- **云端入口**：账号、设备在线状态与路由，从浏览器选择可达设备进入工作台。
- **本地与远程一致**：同一套产品模型覆盖本机直连与经云端访问。

## 架构

![](./assets/arch.png)

数据面始终落在设备侧；Cloud 只做账号、在线状态与路由，不接管进程生命周期。

| 角色 | 作用 |
| --- | --- |
| **Browser** | 工作台 UI：设备、工作区、会话、终端与代码编辑 |
| **Cloud / Gate** | 账号、设备注册与路由，将浏览器请求转到目标设备 |
| **Agent** | 运行在用户设备上，持有本机 runtime 与 PTY |
| **Runtime** | 真实执行命令与进程，所有权始终在设备侧 |

Agent 出站连接 Cloud；浏览器不直连用户设备，也无需为设备开放入站端口。

## 快速开始

### 环境要求

- Go 1.25+
- Node.js 与 Yarn
- [Task](https://taskfile.dev/)

### 安装依赖

```bash
task install
```

### 本地开发

同时启动 Web、本机 Agent 与 Cloud：

```bash
task run
```

### 常用命令

```bash
task test    # 测试
task check   # 检查
task build   # 构建
```

运行角色：

```bash
termbridge agent   # 本机 Agent
termbridge cloud   # Cloud 服务
```

## 文档

- [产品设计](docs/design/design.md)
- [产品路线图](docs/design/roadmap.md)

## 状态

项目处于活跃开发中，API 与交互仍可能变化。欢迎通过 Issue 反馈问题与想法。

## License

[MIT](LICENSE)