# portable package 运行时云端对接收口
最后修改时间: 2026-07-09 14:33:34

Flow mode: light / 轻量模式
Stage: Requirement / 需求
Review status: Accepted

## Background

本地/云端前端入口已经拆成 `build:local` 与 `build:cloud`，并删除了前端 `web/.env.production`。但 Windows portable package 仍沿用旧 production 运行时假设：`task package` 复制已删除的 `configs/config.production.yaml`，启动脚本设置 `TERMBRIDGE_ENV=production`，且 package 构建通过 `yarn build` 间接打出 Cloud 前端入口。这样会导致打包失败，或者包能运行但本地 Agent 前端入口、Go 后端运行时配置和云端 OAuth 对接不一致。

## Goal

- portable package 明确是本地 Agent 包，前端构建使用 `build:local`。
- package 不再依赖已删除的 `configs/config.production.yaml`。
- package 启动脚本使用 `TERMBRIDGE_ENV=local`，让运行时加载 `configs/config.yaml`、可选 `configs/config.local.yaml`、根目录 `.env.local` 和 OS env。
- package 内提供根目录 `.env.local` 运行时模板，用于本地静态目录、本机访问地址、云端 public/API 地址和 Agent OAuth client 配置。
- 区分 `web/.env.local` 与 package 根目录 `.env.local`：前者是前端构建输入，后者是 Go 后端运行时覆盖文件。

## Non-goal

- 不恢复 `configs/config.production.yaml`。
- 不恢复 `web/.env.production`。
- 不把 `TERMBRIDGE_LOCAL__OAUTH__CLIENT_SECRET` 注入浏览器前端构建输入。
- 不新增后端 `index.html` runtime config 注入机制。
- 不在本轮解决多 Cloud 域名动态切换；当前 package 面向一个确定的 Cloud 入口构建和配置。

## User scenarios

- 作为维护者，我运行 `task package` 时得到本地 Agent portable zip，而不是 Cloud-only 前端包。
- 作为本地用户，我解压 zip 后通过 `start.cmd` / `start.sh` 启动，程序按 local 环境读取 package 根目录 `.env.local`。
- 作为部署者，我可以在 package `.env.local` 中配置 Cloud 地址和 OAuth client secret，使本地 Agent 能对接云端授权与 API。

## Acceptance

- `Taskfile.yml` 的 `package` 任务使用 `yarn build:local`。
- `Taskfile.yml` 不再复制 `configs/config.production.yaml`。
- `Taskfile.yml` 将 `scripts/package/.env.local` 复制为 package 根目录 `.env.local`，并保留 `.env.example` 参考文件。
- `scripts/package/start.cmd` 和 `scripts/package/start.sh` 设置 `TERMBRIDGE_ENV=local`。
- 启动脚本从 package 根目录 `.env.local` 读取 `TERMBRIDGE_LOCAL__PUBLIC_URL` 打开浏览器。
- `scripts/package/.env.local` 包含本地静态目录、本机 listen/public URL、Cloud public/API URL、local OAuth client id/secret/redirect/scopes。
- package 运行时 secret 只出现在根目录 `.env.local` 模板，不进入 `web/.env.*` 前端构建输入。
- 相关静态测试或 Go 测试覆盖 package 构建入口与启动脚本语义。

## Open questions

- 暂无阻塞实现的问题。Cloud 最终公网域名和 OAuth client secret 需要在发布前替换模板值。

## Decisions

- portable package 是 local runtime package，不再使用 production 环境名。
- package 内实际运行覆盖文件使用根目录 `.env.local`。
- Cloud 镜像仍使用 `build:cloud`；portable package 使用 `build:local`。
- package 的 Cloud 对接配置必须和 Cloud 后端 `cloud.oauth.clients[]` 中登记的 client 信息一致。

## Risk

- 如果发布时未替换 package `.env.local` 中的 Cloud 域名或 OAuth secret，连接云端会失败。
- 如果前端 `web/.env.local` 与 package 根目录 `.env.local` 中的 Cloud 地址不一致，浏览器入口和 Agent 后端对接目标会漂移。
- 当前仍未实现后端 runtime config 注入，因此前端公开 Cloud 地址仍是构建期固化。
