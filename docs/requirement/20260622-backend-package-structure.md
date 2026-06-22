# 后端包结构分层重组
最后修改时间: 2026-06-22 19:18:31

Review status: Accepted

## Background

当前后端已经收口为一个 `serve` 入口和一个统一 HTTP server，但代码目录仍然保留 `internal/backend`、`internal/gateway`、`internal/webserver`、`internal/requestlog`、`internal/state` 等平铺包名。

这些包名混合了传输层、业务域、存储实现和历史遗留概念，容易继续制造“gateway server / webserver server / backend server”之类的误解。用户明确要求不要只围绕当前结构打补丁，而是引入主流 Go 项目的分层组织，把已有实现分门别类放进去，避免 `http`、`repo` 等所有东西都堆在 `internal` 顶层。

本文件由旧的 `20260620-backend-air-hot-reload.md` 重命名并改写，以适配当前“后端包结构分层重组”任务。

## Goal

- 引入更合理的后端包结构分层，按职责组织现有代码。
- 保留单一 `termbridge serve` 后端入口和单一 HTTP server ownership。
- 将 HTTP 相关实现归入 `internal/transport/http/...`，而不是继续平铺在 `internal` 顶层。
- 将本地 workbench API、Gateway API、request logging middleware、HTTP server lifecycle 按 transport 子包拆分。
- 将 use case / runtime orchestration、domain model、protocol、infrastructure / repository 实现分别归类。
- 通过结构和命名消除 `webserver`、`gateway server` 等误导性历史概念。

## Non-goal

- 不改变 HTTP API 路径。
- 不改变 CLI 命令集合。
- 更新统一后端相关配置命名，使配置语义反映入口合并后的实际模型。
- 不改变 request log 行为。
- 不引入新的 framework、DI container 或泛化 repository abstraction。
- 不重写业务逻辑；本轮以机械迁移、命名归位和 import 修正为主。
- 不使用 git 命令完成文件迁移或清理。

## User scenarios

- 开发者阅读目录结构时，可以直接区分 transport、application、domain、protocol 和 infrastructure。
- 开发者能从 `internal/transport/http/server` 看出唯一 HTTP server owner。
- 开发者能从 `internal/transport/http/localapi` 和 `internal/transport/http/gatewayapi` 看出它们只是 route adapters，不是独立后端服务。
- 开发者查找文件系统持久化实现时，可以定位到 `internal/infrastructure/repository/state`。
- 开发者查找 workspace/session/process 等核心模型时，可以定位到 `internal/domain/...`。

## Acceptance

- 后端包结构按以下方向重组：
  - `internal/transport/cli`
  - `internal/transport/http/server`
  - `internal/transport/http/localapi`
  - `internal/transport/http/gatewayapi`
  - `internal/transport/http/middleware/requestlog`
  - `internal/application/...`
  - `internal/domain/...`
  - `internal/protocol/...`
  - `internal/infrastructure/...`
- 原 `internal/webserver` 不再作为包名存在。
- 原 `internal/backend` 不再作为顶层包名存在，唯一 server owner 迁移到 HTTP transport server 层。
- 原 `internal/state` 不再作为顶层包名存在，迁移到 infrastructure repository 层。
- `termbridge serve` 行为不变。
- 统一后端监听配置使用 `serve.host` / `serve.port` / `serve.open` / `serve.dev`，不再使用误导性的 `gateway.*` 表达 server lifecycle。
- Agent connector 目标地址配置使用 `agent.server_url`，表达它连接的是统一后端 server。
- `/api/...` 和 `/api/gateway/...` 路径不变，并仍挂在同一个 HTTP server 上。
- request logging 仍只在统一 backend server 外层应用一次。
- 现有 Go 测试通过，至少包括 `go test ./...`。
- `just --list` 仍能列出 `serve` 和 `web` 等开发入口。

## Open questions

- 暂无需要用户确认的未决事项；当前按已确认方向执行结构性迁移。

## Decisions

- 使用 `internal/transport/http/...` 表达 HTTP transport 层，不在 `internal/http` 下直接放 Go package，避免与标准库 `net/http` 语义混淆。
- 使用 `localapi` 替代 `webserver`，强调它是本地 workbench API route adapter。
- 使用 `gatewayapi` 表达 Gateway HTTP/WebSocket route adapter，而不是 Gateway server。
- 使用 `internal/infrastructure/repository/state` 承载当前文件系统持久化实现，不新增顶层 `repo` 包。
- 统一后端 server lifecycle 配置使用 `serve.*`，避免把 server listen/open/dev 配置继续挂在 Gateway route 名下。
- Agent connector 目标地址使用 `agent.server_url`，避免把统一后端地址继续命名为 Gateway URL。
- 用户已批准包结构分层重组计划，需求状态记为 `Accepted`。

## Risk

- 本次是大范围包移动，主要风险是 import path、package name、测试 package 声明遗漏。
- Go 包名迁移可能与标准库 `net/http` 命名产生局部冲突，需要避免 package `http`。
- 如果迁移过程中顺手改业务逻辑，会扩大风险；实现应保持机械迁移优先。
- 文档和历史 verification 中可能仍有旧包名引用；本轮优先更新当前 README / requirement，不主动改写历史记录。
