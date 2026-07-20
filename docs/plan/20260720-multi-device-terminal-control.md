# Plan / 计划：多设备终端观察与控制权转移（V1）

最后修改时间: 2026-07-20 15:10:00

Review status: Accepted

## Flow mode / Stage

标准模式 / standard；计划 / Plan（Draft）← Spec Accepted + Requirement Accepted（V1 语义收缩）。

## Requirement / Spec basis

- Requirement: [`docs/requirement/20260720-multi-device-terminal-control.md`](../requirement/20260720-multi-device-terminal-control.md)（Accepted + Implementation scope note V1）
- Spec: [`docs/spec/20260720-multi-device-terminal-control.md`](../spec/20260720-multi-device-terminal-control.md)（Accepted · V1）

**V1 产品句**

> 同一 session 可多端同时看；任意时刻只有一端能打字和改 PTY 尺寸；另一端点「接管」或确认后成为唯一控制端；当前 controller 断开且仍有其它 live attach 时自动移交控制权；不做严格并发 CAS 与 attach preflight。

## Locked V1 scope

| 项 | 值 |
| --- | --- |
| 多 attach | 去掉 Cloud/Agent `writers` 409；同 session 多 stream/client 收输出 |
| 写权限 | 仅 `controllerClientId` 可 input + PTY resize + attach 初始 size |
| Takeover | browser `take_control`；LWW；旧端降 observer 不断开 |
| 断开移交 | 剩余 client 中 **最早 attach** 自动成为 controller；`control_auto_granted` |
| Tunnel | `streamId` = attachment；新增 `TerminalTakeControl`；下行 `terminal_control` |
| 配额 | 不变；每 attach 计数；429 语义不变 |
| 角色通知 | best-effort（queue 满只打日志；不因 role event 失败 detach/重选） |
| 不做 | epoch CAS、preflight、Cloud early reject、replay 专项、双写、role 可靠投递/generation |

## Implementation approach

```text
1) Proto + protocol constants
   agent.v1: ClientControlMessage take_control; ServerControlMessage control_role
   shared.v1 tunnel: TerminalTakeControl + TunnelFrame oneof
   regen Go/TS (buf)

2) Agent SessionRuntime (source of truth)
   controllerClientId + client join order
   attach elect / TakeControl LWW / detach auto-promote
   gate WriteInput / Resize / attach-size
   notify started|state|error via client queue

3) Agent local WS + Cloud tunnel client
   remove writers mutex for terminal
   handle take_control; only controller applies attach size
   streamId → Client map (cloud path)

4) Cloud handleTerminalWS
   remove writers 409; multi stream unchanged
   forward TerminalTakeControl on take_control
   forward input/resize always (Agent gates)

5) Frontend
   control_role state; observer UI + modal/button
   layout resize local-only; take_control + post-grant resize
   neutral WS error; toast dedupe

6) Tests (backend heavy, frontend unit light)
```

## Implementation steps

### Step 1 — Proto & generated code

1. `proto/termbridge/agent/v1/terminal.proto`
   - `ClientControlMessage`: 支持 `type=take_control`（无新字段亦可，仅 type 区分）。
   - `ServerControlMessage`: 增加 `string control_role = 14`。
2. `proto/termbridge/shared/v1/tunnel.proto`
   - 新增 `message TerminalTakeControl { string workspace_id = 1; string session_id = 2; }`
   - `TunnelFrame` oneof 增加 `TerminalTakeControl terminal_take_control = 307;`（号以不冲突为准）。
3. 按仓库既有链路生成：`buf generate`（根 + `web/` 若分离）。
4. **禁止**手改 `internal/gen` / `web/src/gen`。

### Step 2 — Shared terminal protocol helpers

文件：`internal/shared/dto/protocol/terminal/protocol.go`（+ test）

1. 常量：`TypeTakeControl = "take_control"`；role：`controller` / `observer`。
2. reason：`control_granted` / `control_lost` / `control_auto_granted` / `controller_detached`（后两者按 Spec）。
3. 错误：`ErrorCodeNotController = "not_controller"` + 固定安全文案。
4. `ValidateClient`：允许 `take_control`；`ValidateServer`：`started`/`state` 可带 `control_role`。

### Step 3 — SessionRuntime controller 状态机（核心）

文件：`internal/agent/application/task/terminal/runtime.go`、`registry.go`、既有 tests

1. 字段：
   - `controllerClientId string`
   - 保持 `clients map`；为「最早 attach」维护 **join 顺序**（如 `clientOrder []string` 或 client 上 `attachedAt`/`seq`）。
2. `attach()`：
   - 加入 clients + order。
   - 无 controller → 自己为 controller；否则 observer。
   - `started` 消息填 `ControlRole`。
   - **仅 controller** 在上层应用 attach size（runtime 也可提供 `ApplyAttachSizeIfController`）。
3. `Client.WriteInput` / `Resize`：
   - 非 controller：input 返回/映射为 not_controller（由 API 层发 error frame）；resize **静默 no-op**。
4. `Client.TakeControl()`：
   - LWW 设 `controllerClientId`；尽力通知新/旧 client `state`（granted/lost）。
5. `detachClient`：
   - 若 detaching 是 controller：从剩余 order 选最早者 → 尽力发 `control_auto_granted`；若无剩余清空 controller。
6. **角色通知 best-effort**：enqueue 失败只 warn，**不** detach、**不** 因 notify 失败再选举；输出 fan-out queue 满仍按既有逻辑 detach 慢 client。
7. `controlMu` 串行化 attach 选举 / take_control / detach promote / input / resize。
8. 保持现有 fan-out / history / closed 行为；**不**做 replay watermark 专项。

### Step 4 — Agent local terminal WS

文件：`internal/agent/api/handler/runtime_endpoint.go`、`server.go`

1. **删除** `bridgeTerminalStream` 内 `writers[writerKey]` 409 互斥（及对应 defer delete）。
2. attach size：仅当 runtime 将该 client 选为 controller 时 resize（attach 后根据 role 或 `Resize` 门禁）。
3. 控制循环：
   - `take_control` → `stream.TakeControl()`（需扩展 `TerminalStream` / `Client` 接口）。
   - binary input 失败且 not_controller → 写 server error control，不断开。
   - resize：调用 `Resize`（非 controller 静默）。
4. 配额 `TryAcquire`/`Release` 保持不变。

### Step 5 — Agent cloud tunnel terminal

文件：`internal/agent/application/user/cloud_client.go`（及 stream 注册结构）

1. 维持 `streamId → inbound`；`TerminalAttach` 创建的 runtime Client 与 stream 生命周期绑定。
2. attach 后：若带 cols/rows，仅 **controller** 调用 `Resize`（attach 返回 role 或尝试 Resize 门禁）。
3. 处理 `TunnelFrame_TerminalTakeControl` → 对应 stream 的 `TakeControl()`。
4. input/resize 经同一 Client 门禁；not_controller 经该 stream 的 `terminal_control` / 现有 outbound 回传 browser（与现有 error 路径一致）。
5. 确保 `ServerControlMessage`（含 control_role）经 tunnel `terminal_control` 到 Cloud → browser。

### Step 6 — Cloud terminal WS

文件：`internal/cloud/api/handler/server.go`、`route.go` 如需

1. **删除** `handleTerminalWS` 内 `writers` 409 互斥。
2. 每个 browser 连接独立 `streamId` + `addTerminal`（已有）。
3. 收到 browser `take_control` → 写 `TunnelFrame{StreamId, TerminalTakeControl{workspace,session}}`。
4. input/resize 照旧转发；**不做** Cloud 侧 controller 判断。
5. 配额逻辑不变。

### Step 7 — Frontend

| 区域 | 工作 |
|------|------|
| gen types | 随 buf 生成 `control_role` / take_control |
| `useTerminalSocket.ts` | onerror 中性文案（去掉默认 quota 暗示）；可选透传 close reason |
| `TerminalView.vue` / 终端状态 | 持有 `controlRole`；observer 不 `sendControl(resize)`；输入前确认或按钮 `take_control`；granted/auto_granted 后 resize 一次 |
| `useXterm.ts` | 支持 live 只读（disableStdin 或上层吞 onData）；layout fit 仍可 |
| `SessionStatusBar` / workbench | 只读标签 +「接管」 |
| 确认模态 | 输入/粘贴触发；取消丢弃序列 |
| i18n | 接管文案、已获控制权、只读、中性连接失败 |
| toast | `sessionId+reason` 去重/限频（shell 层） |

### Step 8 — Tests

**Backend（优先）**

1. runtime：双 attach → 一 controller 一 observer；双方可收 binary fan-out。
2. observer WriteInput 失败；Resize 不改变 PTY size。
3. TakeControl LWW：后调用者 controller，前者收 control_lost。
4. controller detach → 剩余最早 client auto_granted。
5. 仅 controller attach-size 生效（测 Resize 调用次数或 size）。
6. Cloud/Agent handler：第二 WS attach **不再** 409（可用现有 terminal test 模式扩展）。
7. 配额：多 attach 仍受 limit（若有现成测试则补一条）。

**Frontend（轻量）**

1. role=observer 时不发送 resize control（unit/mock）。
2. take_control 在确认后发送。
3. toast dedupe 辅助函数若抽出则单测。

### Step 9 — 手工验证清单（实现后、Verification 阶段）

1. 两浏览器同 session：均可看输出；仅先开端可输入。
2. 第二端输入弹窗 → 确认接管 → 第一端只读提示。
3. 关掉 controller 标签：剩余端自动可输入并提示。
4. 旋转/缩放 observer：PTY 尺寸不乱跳。
5. 开满 hot tabs：配额 429 行为不回归。

## Files to change（预期）

```text
proto/termbridge/agent/v1/terminal.proto
proto/termbridge/shared/v1/tunnel.proto
internal/gen/...                          # generated
web/src/gen/...                           # generated

internal/shared/dto/protocol/terminal/protocol.go
internal/shared/dto/protocol/terminal/protocol_test.go

internal/agent/application/task/terminal/runtime.go
internal/agent/application/task/terminal/registry.go
internal/agent/application/task/terminal/registry_test.go
internal/agent/application/user/runtime_access.go   # TerminalStream interface if needed
internal/agent/application/user/cloud_client.go
internal/agent/api/handler/runtime_endpoint.go
internal/agent/api/handler/server.go                # writers map may become unused for terminal
internal/agent/api/handler/errors.go                # not_controller mapping if HTTP ever used

internal/cloud/api/handler/server.go
internal/cloud/api/handler/terminal_test.go         # if exists / extend

web/src/features/sessions/useTerminalSocket.ts
web/src/components/terminal/TerminalView.vue
web/src/components/terminal/useXterm.ts
web/src/components/session/SessionStatusBar.vue     # or equivalent
web/src/components/session/SessionsPageShell.vue
web/src/i18n.ts
(+ small composable for takeover modal / toast dedupe if clean)
```

## Verification plan

| 检查 | 命令/方式 |
|------|-----------|
| Go 单测 | 针对 terminal package / cloud handler 的 `go test` |
| 前端单测 | `web` 内 vitest 相关文件 |
| Lint | 项目既有 ruff/不适用则 gofmt/vet + 前端 biome（按仓库习惯） |
| 手测 | Step 9 |

实现阶段默认：跑相关测试；完整 Verification 文档等用户要求 Verification 阶段再写。

## Blockers

- 无外部依赖。
- 需本机 `buf` 生成链路可用；若 CI 生成，本地按 README/just 执行。

## Assumptions

1. 自动移交选举 = **剩余 client 中 attach 顺序最早**（稳定、可测）。
2. `control_role` 仅 `controller` \| `observer`；零 client 时无连接可通知。
3. `TerminalStream` / local bridge 与 cloud 共用同一 runtime Client API 扩展（`TakeControl`）。
4. Cloud `writers` 若仍被其它功能使用则只去掉 **terminal attach** 路径用法；勿误删无关逻辑。
5. 前端 observer 本地可暂时保留光标；以「不发 PTY 输入/resize + 状态栏」为最低验收，stdin disable 为应达项。

## Risks

| 风险 | 缓解 |
|------|------|
| 漏改 tunnel → Cloud 无法 takeover | Step 5/6 联调；测试覆盖 take_control 帧 |
| writers 删除不彻底仍 409 | 双路径 handler 代码审 + 集成测 |
| 自动移交用户未注意 | `control_auto_granted` toast |
| 角色 UI 与服务端短暂不一致 | Agent PTY 门禁兜底；notify best-effort |
| 多 attach 放大 fan-out | 既有 per-client 队列；不扩 scope |
| 旧客户端乱输入 | 服务端门禁兜底 |
| Release 配额泄漏 | 保持 defer Release |

## Rollback

1. 配置无法关闭时：回滚 PR/commit。
2. 临时缓解：前端隐藏第二 attach（不推荐作正式开关）。
3. 无数据迁移；纯运行时行为。

## Out of order / do not

- 不实现 preflight、epoch CAS、Cloud controller 镜像。
- 不改 quota 默认 8 / admin API。
- 不修 replay 空洞专项。
- 不在本 Plan 改 MCP。

## User review notes

- 用户要求进入 Plan；Spec 按 V1 收缩 + controller 断开自动移交 已 Accepted。
- Plan 将实现拆为 proto → runtime → local/cloud wire → frontend → tests。
- 待用户接受 Plan 后开始 Implementation。

## 实现状态

- **2026-07-20 14:09:01**：Implementation 已落地 V1 核心（proto、runtime 门禁/LWW/自动移交、Agent local + Cloud tunnel、前端 role/接管 UI）。
- 自动化：go test terminal packages 通过；vue-tsc --noEmit 通过。
- 手测清单（Step 9）待 Verification 阶段执行。
