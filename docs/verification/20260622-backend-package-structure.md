# 后端包结构分层重组验证
最后修改时间: 2026-06-22 19:18:31

Review status: Accepted

## Requirement alignment / 需求对齐

依据 `docs/requirement/20260622-backend-package-structure.md` 核对，本次实现与需求目标对齐：

- 已引入 `transport`、`application`、`domain`、`protocol`、`infrastructure` 分层。
- HTTP 相关实现已归入 `internal/transport/http/...`。
- 唯一 HTTP server owner 已放在 `internal/transport/http/server`。
- 本地 workbench route adapter 已放在 `internal/transport/http/localapi`。
- Gateway Browser API / Agent tunnel / terminal WebSocket route adapter 已放在 `internal/transport/http/gatewayapi`。
- request logging middleware 已放在 `internal/transport/http/middleware/requestlog`。
- 文件系统 repository 实现已放在 `internal/infrastructure/repository/state`。
- workspace、session、process、identity 等核心模型已放在 `internal/domain/...`。
- terminal、tunnel wire protocol 已放在 `internal/protocol/...`。
- 未改变 HTTP API 路径、CLI 命令集合或 request log 行为；统一后端配置命名已从 `gateway.*` 调整为 `serve.*`，Agent 目标地址已从 `agent.gateway_url` 调整为 `agent.server_url`。

## Spec / Plan alignment

轻量模式 / light 无单独 spec 或 plan 文档；本次按已接受的 requirement 和已批准的包结构重组计划执行。

执行结果与计划中的推荐结构一致：

```text
internal/
  app/
  transport/
    cli/
    http/
      server/
      localapi/
      gatewayapi/
        auth/
      middleware/
        requestlog/
  application/
    agent/
    runner/
    terminal/
  domain/
    identity/
    process/
    session/
    workspace/
  protocol/
    terminal/
    tunnel/
  infrastructure/
    config/
    errors/
    history/
    logging/
    pty/
    repository/
      state/
    version/
```

## Actual diff summary / 实际改动摘要

本次验证覆盖的主要结构性改动：

- `internal/backend` 迁移为 `internal/transport/http/server`。
- `internal/webserver` 迁移为 `internal/transport/http/localapi`。
- `internal/gateway` 迁移为 `internal/transport/http/gatewayapi`。
- `internal/gatewayauth` 迁移为 `internal/transport/http/gatewayapi/auth`。
- `internal/requestlog` 迁移为 `internal/transport/http/middleware/requestlog`。
- `internal/state` 迁移为 `internal/infrastructure/repository/state`。
- `internal/config`、`internal/logging`、`internal/history`、`internal/errors`、`internal/pty`、`internal/version` 迁移到 `internal/infrastructure/...`。
- `internal/webterminal`、`internal/agent`、`internal/runner` 迁移到 `internal/application/...`。
- `internal/session`、`internal/workspace`、`internal/process`、`internal/identity` 迁移到 `internal/domain/...`。
- `internal/terminalproto`、`internal/tunnel` 迁移到 `internal/protocol/...`。
- `internal/cli` 迁移到 `internal/transport/cli`。
- `cmd/termbridge/main.go`、`internal/app/app.go`、测试文件及 import path 已更新。
- `README.md` 新增后端代码分层说明，并更新统一后端配置示例为 `serve.*` / `agent.server_url`。
- requirement 文档由旧 Air 热加载主题替换为当前包结构重组主题。

## Expected vs actual changed files / 预期与实际改动对比

| 预期 | 实际 | 结论 |
| --- | --- | --- |
| HTTP server owner 迁移到 `internal/transport/http/server` | 已迁移并保留 `Server` / `Info` / `New` / `ServeHTTP` / `Listen` / `Serve` | Pass |
| local API adapter 使用 `localapi` 包名 | 已迁移到 `internal/transport/http/localapi`，不再使用 `webserver` 包名 | Pass |
| Gateway HTTP/WS adapter 使用 `gatewayapi` 包名 | 已迁移到 `internal/transport/http/gatewayapi`，auth 子包归入其下 | Pass |
| request logging middleware 归入 HTTP middleware | 已迁移到 `internal/transport/http/middleware/requestlog` | Pass |
| repository 实现不放顶层 `state` | 已迁移到 `internal/infrastructure/repository/state` | Pass |
| domain/protocol/application/infrastructure 分层存在 | 已按目录结构完成迁移 | Pass |
| 不改变 API 路径和 CLI 命令 | 测试覆盖 `/api/health`、`/api/gateway/health`、Gateway tunnel/relay/WebSocket、CLI serve/exec/list 行为 | Pass |
| 配置语义反映入口合并 | 统一后端监听与 dev/open 配置已改为 `serve.host` / `serve.port` / `serve.open` / `serve.dev`；Agent 目标地址已改为 `agent.server_url` | Pass |

## Acceptance checklist / 验收清单

- [x] `internal/transport/cli` 存在并承载 CLI transport。
- [x] `internal/transport/http/server` 存在并承载唯一 HTTP server owner。
- [x] `internal/transport/http/localapi` 存在并承载本地 workbench API routes。
- [x] `internal/transport/http/gatewayapi` 存在并承载 Gateway API / Agent tunnel / terminal WebSocket routes。
- [x] `internal/transport/http/middleware/requestlog` 存在并承载 request logging middleware。
- [x] `internal/application/...` 存在并承载 agent、runner、terminal use case 层。
- [x] `internal/domain/...` 存在并承载 identity、process、session、workspace。
- [x] `internal/protocol/...` 存在并承载 terminal/tunnel protocol。
- [x] `internal/infrastructure/...` 存在并承载 config、logging、history、PTY、filesystem repository 等基础设施。
- [x] 原 `internal/backend` 目录已不存在。
- [x] 原 `internal/webserver` 目录已不存在。
- [x] 原 `internal/state` 目录已不存在。
- [x] Go import 中未发现旧顶层包路径引用。
- [x] `go test ./...` 通过。
- [x] `just --list` 仍列出 `serve` 和 `web`。

## Verification commands / 验证命令

### `gofmt -l`

命令：

```powershell
gofmt -l <all go files>
```

结果：通过。没有输出，表示没有待格式化的 Go 文件。

### `go test ./...`

命令：

```powershell
go test ./...
```

结果：通过。

关键包结果包括：

```text
ok termbridge-go/internal/app
ok termbridge-go/internal/application/agent
ok termbridge-go/internal/application/runner
ok termbridge-go/internal/application/terminal
ok termbridge-go/internal/domain/identity
ok termbridge-go/internal/domain/process
ok termbridge-go/internal/domain/session
ok termbridge-go/internal/domain/workspace
ok termbridge-go/internal/infrastructure/config
ok termbridge-go/internal/infrastructure/errors
ok termbridge-go/internal/infrastructure/history
ok termbridge-go/internal/infrastructure/logging
ok termbridge-go/internal/infrastructure/pty/gopty
ok termbridge-go/internal/infrastructure/repository/state
ok termbridge-go/internal/protocol/terminal
ok termbridge-go/internal/protocol/tunnel
ok termbridge-go/internal/transport/cli
ok termbridge-go/internal/transport/http/gatewayapi
ok termbridge-go/internal/transport/http/gatewayapi/auth
ok termbridge-go/internal/transport/http/localapi
ok termbridge-go/internal/transport/http/middleware/requestlog
ok termbridge-go/internal/transport/http/server
```

### `just --list`

命令：

```powershell
just --list
```

结果：通过，仍列出：

```text
build
check
clean
exec
install
serve
test
web
```

### 旧 import path 搜索

命令：使用内容搜索检查 Go import 中是否仍引用旧顶层包路径。

结果：通过，未发现旧路径：

```text
termbridge-go/internal/backend
termbridge-go/internal/webserver
termbridge-go/internal/gatewayauth
termbridge-go/internal/requestlog
termbridge-go/internal/state
termbridge-go/internal/webterminal
termbridge-go/internal/agent
termbridge-go/internal/runner
termbridge-go/internal/workspace
termbridge-go/internal/session
termbridge-go/internal/process
termbridge-go/internal/identity
termbridge-go/internal/terminalproto
termbridge-go/internal/tunnel
termbridge-go/internal/cli
```

## Scope deviations / 范围偏差

- 当前工作区还包含本任务之外的历史/相邻改动，例如 `.air.toml`、`.gitignore`、`justfile`、旧 request logging 文件删除，以及 `web/src/...` 前端文件改动。它们不是本次包结构验证的核心范围。
- 历史 verification / design 文档中仍可能引用旧包名。本次 requirement 已明确“不主动改写历史记录”，因此没有迁移历史文档中的旧路径。
- 未运行 `just serve` 冒烟启动，原因是此前已用 `go test ./...` 和 `just --list` 验证结构与入口；本次没有持续启动服务以避免占用本地端口和再次生成 Air runtime artifact。

## Risks / 风险

- 这是大范围包移动，后续开发分支如果仍引用旧 import path，会产生 merge conflict 或编译错误。
- 本次已将统一后端 server lifecycle 配置从 `gateway.*` 改为 `serve.*`；现有本地 `.termbridge.yaml` 如果仍使用旧 key，会被 unknown-key 校验拒绝，需要同步迁移。
- 当前工作区存在与本任务无关的前端文件改动，提交前建议拆分或单独确认来源。

## Incomplete items / 未完成项

- 无阻塞项。
- 未创建新的 git commit，也未执行 git 写操作。
- 未对历史文档做批量路径迁移。

## Conclusion / 结论

验证通过。后端包结构已按 requirement 完成分层重组，核心行为通过 `go test ./...` 覆盖，开发入口通过 `just --list` 确认仍存在。当前变更满足 light / 轻量模式 verification 要求，可以进入人工审查或后续提交准备。
