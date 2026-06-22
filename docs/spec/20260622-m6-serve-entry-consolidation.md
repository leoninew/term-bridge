# M6.1 Serve 统一入口与文档收口规格
最后修改时间: 2026-06-22 17:01:33

Review status: Accepted

## Requirement basis

- Requirement: `docs/requirement/20260622-m6-serve-entry-consolidation.md`，Review status: Accepted
- M6 requirement: `docs/requirement/20260620-m6-gateway-web-terminal-mvp.md`，Review status: Accepted
- M6 plan: `docs/plan/20260620-m6-gateway-web-terminal-mvp.md`，Review status: Accepted
- M6 verification: `docs/verification/20260620-m6-gateway-web-terminal-mvp.md`，Review status: Accepted

用户要求将流程从标准模式 / standard 改为严格模式 / strict，并进入 Plan。由于严格模式需要 Spec 阶段，本规格文档用于补齐 `termbridge serve` 统一入口的设计约束，并作为 Plan 的依据。

## Overview

M6.1 将 M6 中分散的 Gateway / Agent 入口收口为统一服务入口：

```text
termbridge serve
  ├─ Gateway service
  │    ├─ Browser HTTP API
  │    ├─ Browser terminal WebSocket
  │    ├─ auth / device registry / routing / relay
  │    └─ current single frontend serving local/Gateway routes
  └─ Agent connector
       ├─ outbound tunnel to configured Gateway URL
       ├─ local runtime adapter
       └─ workspace/session/history/terminal access
```

这不是把 Gateway service 和 Agent connector 合并为一个职责对象，而是把它们收口到同一个启动入口和生命周期中。启动后同时具备两类能力；是否连接远端、访问哪个 Gateway、浏览器访问 local 还是 Gateway 路由，由配置、路由和访问路径决定。

## Design decisions

### 1. CLI 入口

正式 CLI 入口收口为：

```text
termbridge serve
```

删除以下用户可见 command：

```text
termbridge gateway
termbridge agent
```

删除 Gateway/Agent 运行参数 flag：

```text
--host
--port
--open
--dev
--gateway-url
--device-name
--seed-session-command
```

CLI 只负责选择行为入口，不承载 Gateway/Agent 运行参数。运行参数全部通过配置表达。

### 2. 开发入口

开发入口收口为：

```text
just serve
```

删除长期 just target：

```text
just gateway
just agent
just m6-*
```

M6 验证所需底层能力不删除；以通用测试命令、Pomelo flow 文件和 verification 文档中的命令记录保留。

### 3. 配置模型

新增配置结构：

```yaml
gateway:
  host: 127.0.0.1
  port: 9010
  open: false
  dev: false

agent:
  gateway_url: http://127.0.0.1:9010
  device_name: local-dev
```

设计约束：

1. `gateway.*` 控制 Gateway service 的监听和开发行为。
2. `agent.*` 控制 Agent connector 连接哪个 Gateway 以及设备显示名。
3. Agent 连接哪个 Gateway 是配置问题，不是 CLI 参数问题。
4. 默认配置写入 `.termbridge.default.yaml`。
5. 用户覆盖写入 `.termbridge.yaml`，沿用现有 Viper merge 和 unknown-key reject 机制。
6. 不新增环境变量作为正式入口；Gateway/Agent 运行参数只通过默认配置和 `.termbridge.yaml` 覆盖表达。

### 4. App runtime lifecycle

`app.Run` 中新增 `CommandServe`，统一启动：

1. Gateway service。
2. Agent connector。

生命周期要求：

1. 使用同一个 parent context。
2. 任一组件出现非 `context.Canceled` 错误时，取消整体服务并返回错误。
3. 正常取消时两个组件都应停止。
4. Gateway service 不直接拥有 PTY/process lifecycle。
5. Agent connector 通过 `webterminal.Registry` adapter 访问本地 runtime。
6. 删除 seed-session 行为，不再由 serve 自动创建 dev-only session。

### 5. 前端入口

前端只有一个，不新增 Gateway 专用 frontend、构建产物或 dev target。

设计表达：

1. 同一个 frontend 承载 local/Gateway 访问面。
2. 通过不同路由或访问面区分 local/Gateway 能力。
3. 如果当前代码尚未具备显式 route 结构，Implementation 阶段不得在文档中虚构；应记录为当前单前端承载，后续继续向 route 区分收口。
4. 本任务不做大规模前端重构，除非实现现实与需求冲突并经过用户确认。

### 6. 文档与验收状态

M6.1 完成后：

1. README 以 `termbridge serve` 和配置为正式使用路径。
2. `docs/design.md` 说明 serve 统一入口与 Gateway/Agent 双能力边界。
3. Roadmap 说明 M6 工程链路已实现，M6.1 收口入口与文档。
4. M6 verification 更新为 `Accepted`，但保留剩余人工验证项。
5. `docs/todo.md` 不再把 M5/M6 已处理事项列为待处理。

## Affected components

### CLI

- `internal/cli/cli.go`
- `internal/cli/cli_test.go`

影响：command kind、parse logic、usage/help、测试断言。

### Config

- `.termbridge.default.yaml`
- `internal/config/config.go`
- `internal/config/config_test.go`

影响：新增 Gateway/Agent config、unknown-key allowlist、默认值、覆盖和校验。

### App orchestration

- `internal/app/app.go`
- `internal/app/app_test.go`

影响：新增 serve command dispatch，组合 Gateway service 和 Agent connector 生命周期。

### Gateway / Agent packages

- `internal/gateway/*`
- `internal/agent/*`

影响：原则上不重写协议或业务实现；仅在 constructor/config shape 需要时做最小适配。

### Frontend

- `web/src/*`

影响：默认只核对，不主动大改。若 route 现实与需求冲突，回到流程文档调整。

### Docs / process

- `README.md`
- `docs/design.md`
- `docs/plan/20260617-roadmap-refresh.md`
- `docs/todo.md`
- `docs/verification/20260620-m6-gateway-web-terminal-mvp.md`
- `docs/verification/20260622-m6-serve-entry-consolidation.md`

## Interfaces

### CLI interface

保留：

```text
termbridge exec -- <command...>
termbridge web [web options]
termbridge serve
termbridge session
termbridge workspace
```

删除：

```text
termbridge gateway [gateway options]
termbridge agent [agent options]
```

### Config interface

新增：

```yaml
gateway:
  host: 127.0.0.1
  port: 9010
  open: false
  dev: false

agent:
  gateway_url: http://127.0.0.1:9010
  device_name: local-dev
```

Validation：

1. `gateway.port` in `[0, 65535]`。
2. `agent.gateway_url` trim 后非空。
3. `agent.device_name` 可为空，空值走现有 device fallback。
4. Unknown config keys 继续拒绝。

### App interface

内部 command shape：

```go
const CommandServe CommandKind = "serve"

type ServeCommand struct{}
```

Serve runtime 从 `config.Config` 获取：

```go
cfg.Gateway
cfg.Agent
cfg.Runtime
cfg.Web
cfg.History
```

不从 CLI command options 获取 Gateway/Agent runtime 参数。

## Technical questions

当前没有需要用户继续确认的开放问题。Implementation 阶段需要通过代码核对回答以下技术细节：

1. `runGatewayServer` 与 `runAgentClient` 的测试替换点是否足以覆盖统一生命周期。
2. Agent connector 在默认 `agent.gateway_url` 指向本地 Gateway 时，serve 同进程启动顺序是否需要等待 Gateway listener ready 后再启动 Agent。
3. 当前前端是否已有明确 route 区分 local/Gateway；如果没有，文档措辞需避免虚构。
4. Gateway/Agent 运行参数是否已完全脱离环境变量覆盖，只保留默认配置和 `.termbridge.yaml` 覆盖。

## Risks

1. 统一启动会引入生命周期耦合：Gateway service 或 Agent connector 任一失败都可能影响整个 serve。
2. 默认配置让 Agent connector 指向本地 Gateway，若启动顺序不当可能导致首连失败；需要依赖现有 reconnect/backoff 或在 app 层协调启动顺序。
3. 历史 `termbridge agent` / `termbridge gateway` 使用习惯不作为兼容目标；文档只保留当前 `termbridge serve` 入口，避免继续制造双入口认知。
4. 历史 `--seed-session-command` 不作为兼容目标；后续 attach 验证通过已有 session 或手动 session 创建完成。
5. README 不提及临时 auth 后，用户可能能启动服务但不知道如何登录；这是已确认取舍，后续正式 auth 需要补齐。
6. 如果前端 route 现实不支持“不同路由区分”，继续写成已完成会形成新文档债务。

## Alternatives considered

### 保留历史独立 Gateway/Agent 入口

优点：实现改动小，延续 M6 工程形态。

缺点：继续暴露两种模式，入口认知仍然零散，不符合当前用户决策，也不符合“不向后兼容历史问题”的处理原则。

结论：放弃。

### 保留隐藏 Agent 开发入口

优点：调试 Agent 更方便。

缺点：会继续制造长期入口和文档歧义，且会让历史问题以隐藏兼容形式继续存在。

结论：放弃。开发调试通过配置、通用测试或后续明确内部工具解决。

### 只改文档，不改 CLI/app

优点：短期成本最低。

缺点：文档与产品入口不一致，不能真正解决 M6 入口质量问题。

结论：放弃。

## User review notes

用户要求改为严格模式 / strict，并进入 Plan。本 Spec 根据已接受 Requirement 补齐，状态标记为 Accepted，以允许进入 Plan 阶段。
