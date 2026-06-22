# M6 Gateway Web Terminal MVP 计划
最后修改时间: 2026-06-22 14:25:00

Review status: Accepted

## Basis

- Requirement: `docs/requirement/20260620-m6-gateway-web-terminal-mvp.md`，Review status: Accepted
- Spec: `docs/spec/20260620-m6-gateway-web-terminal-mvp.md`，Review status: Accepted
- Roadmap: `docs/plan/20260617-roadmap-refresh.md`
- M5 verification: `docs/verification/20260620-m5-runtime-hardening.md`

M6 采用严格模式 / strict。本计划覆盖 Gateway Web Terminal MVP，不进入 M7 multi-device beta，也不实现正式用户系统。

用户已确认的 M6 关键边界：

1. 只支持 attach/list 已存在 session，不通过 Gateway 创建新 session。
2. Gateway service 与 Agent connector 使用同一个 `termbridge serve` 统一入口启动；历史独立 Gateway/Agent subcommand 不再保留。
3. Agent tunnel 采用单 WebSocket 多路复用。
4. 服务端正式用户系统稍后实现，M6 使用临时 `admin/admin`。
5. 继续在当前前端上实现 Gateway 能力；项目只有一个前端，不引入 Gateway mode 或第二套 frontend。
6. Device identity 存在本地 state dir 的 `device.json`。
7. Gateway 可以保留 shallow cache，但不作为 runtime authoritative state。
8. 每个用户连接自己的 Agent、工作区和会话；每个 session 只能有一个前端 browser 写入。
9. Agent disconnect 时 Browser terminal 显示 `device disconnected` 并要求用户重新 attach。
10. 单元测试、Pomelo PW 测试和人工测试组合验证。

## Implementation steps

### Step 1. 扩展 CLI command shape

目标：在同一个 `termbridge` binary 中增加统一 `serve` command，同时启动 Gateway service 和 Agent connector。

计划：

1. 在 `internal/cli/cli.go` 增加 command kind：`serve`。
2. 不保留独立 `gateway` / `agent` command。
3. Gateway/Agent 运行参数不通过 CLI flag 暴露；`gateway.host`、`gateway.port`、`gateway.open`、`gateway.dev`、`agent.gateway_url`、`agent.device_name` 由配置读取。
4. 更新 usage/help：展示 `termbridge serve`，不展示历史 Gateway/Agent command 或运行参数 flag。
5. 在 `internal/app/app.go` 增加 `serve` command dispatch，但只委托到独立 packages。
6. 补 CLI parsing tests，覆盖 `serve`、help、非法参数、历史入口拒绝。

### Step 2. 建立 tunnel protocol package

目标：定义单 WebSocket 多路复用的最小 frame 和 stream lifecycle。

计划：

1. 新增 `internal/tunnel`。
2. 定义 `Frame`、`FrameType`、`StreamID`、`ProtocolVersion`。
3. 定义最小 frame type：
   - `hello`
   - `hello_ack`
   - `ping`
   - `pong`
   - `request`
   - `response`
   - `terminal_attach`
   - `terminal_input`
   - `terminal_output`
   - `terminal_resize`
   - `terminal_closed`
   - `error`
   - `close`
4. M6 不改变 Browser ↔ Gateway 的 terminal WebSocket 输出语义。JSON/base64 仅作为 Gateway ↔ Agent 单 WebSocket 多路复用 tunnel 的内部 terminal output frame 编码候选；若大输出验证显示 tunnel 编码开销明显，再将内部 terminal output frame 优化为 binary framing。
5. 增加 frame encode/decode、unknown type、version mismatch、stream close/error 测试。

### Step 3. 实现 M6 临时 auth

目标：Gateway API 和 Browser terminal WebSocket 需要临时 `admin/admin` auth。

计划：

1. 新增 `internal/gatewayauth`。
2. 实现 `POST /api/gateway/login`：校验 username/password 为 `admin/admin`。
3. 登录成功后设置 HttpOnly session cookie。
4. 实现 `POST /api/gateway/logout` 和 `GET /api/gateway/me`。
5. Gateway Browser API 和 Browser terminal WebSocket 都经过 auth middleware。
6. Agent tunnel auth 暂用同一临时凭据或 derived header；具体在实现中保持最小可测。
7. 增加 auth tests：未登录拒绝、错误密码拒绝、登录后可访问、logout 后拒绝。

### Step 4. 实现 device identity

目标：Agent 具有稳定 device identity。

计划：

1. 新增 `internal/agent/device.go` 或 `internal/device`。
2. 使用 runtime state dir 生成/读取：`.termbridge/device.json`。
3. 字段：`id`、`name`、`created_at`。
4. `agent.device_name` 通过配置覆盖显示名；如果未配置，使用 hostname 或已有记录。
5. 增加 tests：首次生成、再次读取稳定、配置覆盖 device name、坏文件处理。

### Step 5. 实现 Gateway service skeleton 和 device registry

目标：Gateway 能启动，接受 Agent tunnel，维护 online device registry。

计划：

1. 新增 `internal/gateway`。
2. 实现 Gateway server：
   - `/api/gateway/health`
   - auth endpoints
   - `/api/gateway/devices`
   - `/api/gateway/agent/tunnel`
3. Agent tunnel hello 成功后注册 device online。
4. Agent disconnect 后标记 offline 并清理 active routes。
5. Gateway 保留 shallow cache：device id/name、online/offline、connected_at、last_seen、workspace/session snapshot。
6. 增加 tests：device register、disconnect cleanup、reconnect replaces stale tunnel。

### Step 6. 实现 Agent outbound tunnel client

目标：Agent 主动连接 Gateway，并通过 tunnel 发送 hello/heartbeat。

计划：

1. 新增 `internal/agent`。
2. Agent command 读取 config/state dir、device identity、gateway URL。
3. 建立 outbound WebSocket tunnel。
4. 发送 `hello` frame，等待 `hello_ack`。
5. 实现 ping/pong 或 heartbeat。
6. 实现基础 reconnect/backoff。
7. 增加 tests：hello 成功、auth 失败、disconnect 后重连、context cancel 停止。

### Step 7. 定义 Agent runtime access interface

目标：Agent 可以读取本地 runtime 的 workspace/session/history 和 attach terminal，但不把 Gateway 接入 runtime ownership。

计划：

1. 在 `internal/agent` 或 `internal/gateway` 定义窄接口：

```go
type RuntimeAccess interface {
    WorkspaceTree(ctx context.Context) ([]webterminal.WorkspaceTreeNode, error)
    ListSessions(ctx context.Context) ([]webterminal.SessionSummary, error)
    ReadHistory(ctx context.Context, sessionID string) ([]byte, error)
    Attach(ctx context.Context, sessionID string) (TerminalStream, error)
}
```

2. 使用 `webterminal.Registry` 或其 adapter 实现该接口。
3. 禁止 Gateway 直接依赖 `gopty`、`runner` 或 process lifecycle。
4. 增加 tests：Gateway/Agent package 不直接 import runtime PTY 包的边界可通过代码 review 和 package dependency 保持。

### Step 8. 实现 workspace/session/history relay

目标：Browser 通过 Gateway 能查看 Agent/local runtime 的 workspace/session/history。

计划：

1. Browser request 到 Gateway：
   - `GET /api/gateway/devices/{device_id}/workspaces/tree`
   - `GET /api/gateway/devices/{device_id}/sessions`
   - `GET /api/gateway/devices/{device_id}/sessions/{session_id}/history`
2. Gateway 转换为 tunnel request frame。
3. Agent 调用 `RuntimeAccess` 返回结果。
4. Gateway 返回 Browser；可更新 shallow cache。
5. Agent offline 或 route missing 时返回明确错误。
6. 增加 tests：success、offline、timeout、invalid device/session、cache update。

### Step 9. 实现 terminal relay

目标：Browser 经 Gateway attach 已存在 session，terminal input/output/resize 通过单 tunnel 多路复用 relay。

计划：

1. Gateway 增加 Browser terminal WebSocket endpoint：
   - `GET /api/gateway/devices/{device_id}/sessions/{session_id}/ws`
2. Gateway 创建 terminal stream id，发送 `terminal_attach` 到 Agent。
3. Agent 调用 `RuntimeAccess.Attach` 或 adapter 建立 local terminal stream。
4. Browser input -> Gateway -> tunnel `terminal_input` -> Agent -> local terminal。
5. Local output -> Agent -> tunnel `terminal_output` -> Gateway -> Browser。
6. Browser resize -> Gateway -> tunnel `terminal_resize` -> Agent -> local terminal。
7. 每个 session 只允许一个前端 browser 写入；已有 writer 时拒绝新的 writer attach 或返回 conflict。
8. Agent disconnect 时 Gateway 关闭 Browser terminal 并发送/显示 `device disconnected`。
9. 增加 tests：attach success、input relay、output relay、resize relay、single-writer conflict、disconnect close。

### Step 10. 在当前前端实现 Gateway 能力

目标：不新建第二套前端，也不引入 Gateway mode 概念，在当前 `web/` 中增加 Gateway 正式入口能力。

计划：

1. 增加登录视图：`admin/admin`。
2. 增加 device list / device selection。
3. 将现有 workspace/session API client 抽象为 Gateway API path。
4. 将 terminal WebSocket URL 生成逻辑接入 Gateway endpoint。
5. 复用现有 workbench/session/xterm 组件。
6. 显示 device disconnected / route unavailable / auth expired 等错误。
7. 不实现 Gateway session creation；隐藏或禁用通过 Gateway 新建 session 的入口，保留 attach/list 语义。
8. 增加前端基础测试或 Pomelo PW flow 覆盖登录、device list、session list、attach。

### Step 11. Pomelo PW flow

目标：通过浏览器自动化验证 Gateway Web Terminal MVP 的核心路径。

计划：

1. 在 `.pomelo-pw/` 新增 M6 flow。
2. flow 启动前假设 Gateway server 和 Agent 已运行。
3. flow 覆盖：
   - 打开 Gateway Web。
   - 使用 `admin/admin` 登录。
   - 看到 device。
   - 看到已有 session。
   - attach session。
   - 验证 history/output 或 xterm marker。
   - 截图留证。
4. 对无法稳定自动化的真实 TUI 输入、Agent 重连和 20+ session 压测列入人工测试。

### Step 12. Documentation and verification updates

目标：保持 SpecFlow 和 roadmap 状态一致。

计划：

1. 实现完成后进入 Verification 时创建 `docs/verification/20260620-m6-gateway-web-terminal-mvp.md`。
2. Verification 记录 unit tests、Pomelo PW flow、manual checklist、incomplete items。
3. 如 M6 完成后满足 gate，更新 roadmap M6 状态。

## Files to change

预计修改：

1. `internal/cli/cli.go`
2. `internal/cli/cli_test.go`
3. `internal/app/app.go`
4. `internal/app/app_test.go`
5. `internal/config/*`
6. `internal/webserver/*`（仅在复用 server utility 时）
7. `web/src/App.vue`
8. `web/src/features/sessions/api.ts`
9. `web/src/features/workspaces/api.ts`
10. `web/src/features/sessions/useTerminalSocket.ts`
11. `web/src/i18n.ts`
12. `.pomelo-pw/m6-gateway-web-terminal.yaml`
13. `docs/verification/20260620-m6-gateway-web-terminal-mvp.md`
14. `justfile`（统一 `serve` 开发入口与通用验证入口）
15. `docs/spec/20260620-m6-gateway-web-terminal-mvp.md`（架构图与流程图）

预计新增：

1. `internal/tunnel/*`
2. `internal/gateway/*`
3. `internal/gatewayauth/*`
4. `internal/agent/*`
5. 可能新增 `internal/device/*` 或 agent-local device identity 文件。
6. 前端 Gateway 登录/device selection 组件。

## Developer just entries

M6.1 后开发入口不再保留历史 `gateway` / `agent` / `m6-*` target，避免历史验证命令成为长期使用路径。

当前入口：

```text
just serve
just web
just test
just check
just build
```

入口用途：

1. `just serve`：调用 `termbridge serve`，按配置同时启动 Gateway service 和 Agent connector。
2. `just web`：启动唯一前端项目的开发服务。
3. `just test`：前后端单元测试统一入口，先运行前端 Vitest，再运行后端 Go tests。
4. `just check`：作为单一检查入口，按“前端在前、后端在后”直接运行前端 typecheck/lint/format/test 与后端 Go fmt/vet/test。
5. `just build`：先构建前端生产产物，再构建 termbridge binary。

## Verification plan

### Automated Go tests

优先运行：

```text
go test ./internal/tunnel ./internal/gatewayauth ./internal/gateway ./internal/agent ./internal/cli ./internal/app
```

回归运行：

```text
go test ./...
```

如 terminal relay 并发和 stream routing 涉及 goroutine，补充：

```text
go test -race ./internal/tunnel ./internal/gateway ./internal/agent
```

### Frontend checks

根据项目现有 web scripts 选择运行：

```text
npm --prefix web run build
```

如已有 lint/test script，再按实际 package scripts 补充。

### Pomelo PW

计划新增并运行：

```text
pomelo-pw validate .pomelo-pw/m6-gateway-web-terminal.yaml
pomelo-pw run .pomelo-pw/m6-gateway-web-terminal.yaml -o .pomelo-pw/output-m6 --headless -v
```

### Manual verification checklist

进入 Verification 阶段时补齐结果：

1. 在配置中设置 `gateway.*` 和 `agent.*`。
2. 启动统一服务：`termbridge serve` 或 `just serve`。
3. Browser 使用 `admin/admin` 登录。
4. Browser 能看到 online device。
5. Browser 能看到已有 workspace/session。
6. Browser attach 已存在 session，能看到输出。
7. Browser 输入能到达本地 session。
8. Browser resize 能到达本地 session。
9. 同一 session 第二个 browser writer 被拒绝或明确提示。
10. Agent 断开后 Browser terminal 显示 `device disconnected`。
11. Agent reconnect 后 device online 恢复，Browser 可重新 attach。
12. 多个 session 可分别 attach。
13. Claude Code / Codex 真实 TUI 通过 Gateway attach 的人工验证。
14. 20+ session 或同等压力人工/脚本验证。
15. Windows 环境下 Agent/Gateway 真实连接与 process cleanup 验证。

## Blockers

当前无阻塞 Plan 的用户决策项。已确认：

1. attach/list only。
2. same binary unified `serve` entry。
3. single WebSocket multiplexed tunnel。
4. temporary `admin/admin`。
5. current frontend only。
6. shallow cache accepted。
7. single browser writer per session。
8. device disconnected UX accepted。

## Assumptions

1. M6 可以在本地开发环境中让 Gateway 和 Agent 同机运行，但仍保持逻辑边界。
2. Gateway API 和 Agent tunnel 在 M6 使用 HTTP/WebSocket；生产 TLS/公网部署不在本阶段。
3. `admin/admin` 只用于 MVP 和受控环境，不作为生产安全方案。
4. Gateway ↔ Agent tunnel 内部 JSON/base64 payload 在 M6 性能可接受；Browser ↔ Gateway terminal WebSocket 输出语义保持不变，binary multiplex framing 可留到后续优化。
5. Gateway shallow cache 只用于 UI/display/route 状态，不作为 runtime truth。
6. 前端只有一套，新增 Gateway 能力时允许调整 API client 和入口 UI。

## Risks

1. M6 范围跨 CLI、backend Gateway、Agent、tunnel protocol、frontend 和 Pomelo PW，单次实现可能较大，需要保持步骤拆分。
2. Gateway ↔ Agent tunnel 如果使用 JSON/base64 承载内部 terminal output frame，会增加大输出场景的编码开销；这不改变 Browser 端 terminal WebSocket 语义。
3. 临时 `admin/admin` 若被误用于公网会有安全风险，help/log/docs 必须提示 MVP 限制。
4. Agent reconnect 与 stale route 清理容易产生边界 bug。
5. 单 WebSocket 多路复用如果没有 per-stream backpressure，一个慢 stream 可能影响 control/listing/其他 session。
6. 当前前端加入 Gateway 能力时，可能影响 local Web/workbench 行为，需要 Pomelo PW 和回归测试覆盖。
7. M5 剩余人工验证项在 Gateway 路径上仍可能暴露 runtime 或 platform 问题。

## Rollback

1. `serve` command 增加应独立，不影响 `exec`、`web`、`session`、`workspace`。
2. Gateway/Agent/tunnel 包新增应与 local runtime 隔离；若 Gateway 实现不稳定，可暂停 `serve` 入口但不恢复历史独立 Gateway/Agent command。
3. 前端 Gateway 能力应通过 API client/component 分层实现；如出现问题，可回退相关组件，不影响 local Web terminal 核心组件。
4. Auth 临时实现应可替换；后续正式用户系统上线时删除或替换 `admin/admin`。
5. Gateway ↔ Agent tunnel 内部 JSON/base64 实现可替换为 binary framing，Browser terminal WebSocket 语义保持稳定。

## User review notes

本 Plan 已采用用户对 Spec 的确认：

1. 当前前端唯一，不引入 Gateway mode 或第二套 frontend。
2. shallow cache 采纳，但 Gateway 不成为 runtime truth。
3. 每个 session 只能有一个前端 browser 写入。
4. `device disconnected` UX 采纳。
5. 用户已要求进入 Plan。

等待用户 review Plan。接受后进入 Implementation / 实现阶段。
