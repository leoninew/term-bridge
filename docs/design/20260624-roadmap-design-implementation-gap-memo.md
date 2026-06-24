# Roadmap / Design 与当前实现差距备忘

日期：2026-06-24

## 背景

本备忘基于 roadmap、design 文档与当前代码实现的核对结果，记录当前真实进度、文档滞后点和后续开发建议。重点用于收口“文档状态落后于当前代码”的问题，避免后续开发继续依据旧假设推进。

已核对的主要文档：

- `docs/requirement/20260617-roadmap-refresh.md`
- `docs/spec/20260617-roadmap-refresh.md`
- `docs/plan/20260617-roadmap-refresh.md`
- `docs/design/design.md`
- `docs/design/20260623-unified-gate-device-model.md`
- `docs/verification/20260620-m4-closeout.md`
- `docs/verification/20260620-m5-runtime-hardening.md`
- `docs/verification/20260620-m6-gateway-web-terminal-mvp.md`
- `docs/verification/20260622-m6-serve-entry-consolidation.md`
- `docs/verification/20260624-light-dark-theme-settings-entry.md`

已核对的关键实现：

- `internal/transport/cli/cli.go`
- `internal/app/app.go`
- `internal/transport/http/server/server.go`
- `internal/transport/http/gatewayapi/server.go`
- `internal/application/agent/client.go`
- `web/src/router/index.ts`
- `web/src/features/gateway/api.ts`
- `web/src/features/workspaces/api.ts`
- `web/src/features/sessions/api.ts`
- `web/src/store/workspaceSessions.ts`

## 当前真实状态

### CLI-first runtime 主线

当前实现与 roadmap 的 CLI-first 主线一致：

```text
termbridge [options] exec -- <command...>
```

`internal/transport/cli/cli.go` 已支持：

- `exec`
- `workspace`
- `session`
- `serve`
- `--cwd`
- `--version`
- `--help`

旧式 `termbridge -- <command>` 已被拒绝，并提示使用 `termbridge exec -- <command>`。

### Serve 统一入口

当前 `termbridge serve` 已是统一后端入口。`internal/app/app.go` 中 `runServe` 会：

1. 创建同一个 web terminal registry。
2. 创建 `gatewayapi.Handler`。
3. 启动 HTTP server。
4. 启动 Agent connector。
5. Agent connector 连接目标来自配置 `agent.server_url`。

这与统一 Gate / Device 模型的本地 self-connected Gate 雏形一致。

### 前端访问面

长期产品访问面已经收敛为：

```text
/sessions
/settings
/help
```

`/sessions` 是统一 session workbench。

### Device-scoped API

当前前端 API 已经走 device-scoped 路径：

```text
/api/devices/:deviceId/workspaces
/api/devices/:deviceId/workspaces/tree
/api/devices/:deviceId/workspaces/order
/api/devices/:deviceId/workspaces/:workspaceId
/api/devices/:deviceId/workspaces/:workspaceId/sessions
/api/devices/:deviceId/workspaces/:workspaceId/sessions/:sessionId
/api/devices/:deviceId/workspaces/:workspaceId/sessions/:sessionId/history
/api/devices/:deviceId/workspaces/:workspaceId/sessions/:sessionId/ws
```

前端业务代码的主要产品路径应描述为 device-scoped API。

### Gateway mutation 能力

当前 `internal/transport/http/gatewayapi/server.go` 和 `internal/application/agent/client.go` 已经实现多项 mutation relay：

- create session
- get session
- update session
- close session
- delete session
- rerun session
- delete workspace
- update workspace order

文档应同步为：Gateway / Agent relay 已支持基础 workspace/session mutation。

## 已识别的文档滞后点

### 1. README 访问面描述需同步

README 应只描述当前产品入口：`/sessions` 统一工作台。

### 2. README Gateway 能力描述需同步

当前代码已经支持 device-scoped mutation relay，因此 README 应说明已支持基础 workspace/session mutation，并把正式用户系统、device pairing、token rotation、生产部署能力作为后续阶段能力。

### 3. 主设计文档资源模型需同步

`docs/design/design.md` 中仍保留较早的三级模型：

```text
User -> Device -> Workspace
```

以及“不存在额外 Session 层”的描述。当前代码已经有明确 Session / Terminal / History 资源，因此应更新为：

```text
User -> Device -> Workspace -> Session -> Terminal
```

同时保留关键原则：Gateway 不拥有 PTY / Process lifecycle，Session 是 runtime 管理的命令运行记录 / attach 单元，不是 tmux session。

### 4. Roadmap M6 状态需同步

`docs/plan/20260617-roadmap-refresh.md` 的 milestone overview 仍只写 M6 工程链路和 M6.1 serve 统一入口。当前已进入更靠后的状态：device-scoped Browser API 与多项 Gateway mutation 已实现，下一步应进入 unified device workbench 收口。

## 后续建议

### P0：文档同步收口

已将本备忘作为设计记录写入 `docs/design/`。后续文档应同步：

1. README：更新 `/sessions` 统一入口、device-scoped API 和 Gateway mutation 能力。
2. 主设计文档：更新资源模型为 `User -> Device -> Workspace -> Session -> Terminal`。
3. Roadmap：在 M6/M6.1 后增加 M6.2 unified device workbench closeout 状态和目标。

### P1：M6.2 Unified Device Workbench Closeout

建议下一阶段命名为：

```text
M6.2 Unified Device Workbench Closeout
```

目标：让 `/sessions` 真正成为统一 device/workspace/session 工作台。

建议覆盖：

- 默认 local device 选择。
- 多 device 切换。
- device offline / reconnect / empty / loading 状态。
- device 切换时 workspace/session store reset 或隔离。
- terminal socket lifecycle。
- create / edit / close / delete / rerun / reorder / history / attach 的 device-scoped e2e 验证。

### P1：M5 manual verification 补证

M5 代码层 hardening 已完成大量工作，但仍需补真实验证记录：

- Claude Code / Codex 真实 TUI。
- Windows 真机 Ctrl+C / process tree cleanup。
- 20+ sessions。
- 高频 resize。
- long-running stability。
- slow client / backpressure 真实链路。

### P1/P2：Auth / Device credential 设计

进入 M7 前需要 strict design：

- Browser auth。
- Agent device credential。
- pairing flow。
- token rotation。
- WebSocket auth。
- local self-connected gate 首次运行体验。

## 结论

当前项目实际进度已经超过部分文档描述：

- CLI-first runtime 主线已成型。
- `termbridge serve` 已是统一入口。
- 前端统一产品入口已收敛为 `/sessions`。
- 前端已转向 `/api/devices/:deviceId/...`。
- Gateway mutation relay 已覆盖多项 workspace/session 写操作。

需要立即收口的是文档事实同步，确保文档只描述当前存在的产品入口、device-scoped API 和 Gateway mutation 能力。