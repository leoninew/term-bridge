# 多设备终端会话观察与控制权转移规格 / Multi-Device Terminal Observation and Control Transfer Spec
最后修改时间: 2026-07-20 15:10:00

Review status: Accepted

## Requirement basis

本规格依据 [`docs/requirement/20260720-multi-device-terminal-control.md`](../requirement/20260720-multi-device-terminal-control.md)（Accepted），并按 **V1 语义收缩** 实现可覆盖主场景的简化设计。

**V1 产品句（本规格唯一范围锚点）**

> 同一 session 可多端同时看；任意时刻只有一端能打字和改 PTY 尺寸；另一端点「接管」或确认后成为唯一控制端；**当前 controller 断开后，若仍有其它 live attach，则自动将控制权交给其中一个**（确定性选举，见 D4）；**不做**严格并发 CAS 与 attach preflight。

交叉文档：

- [`docs/requirement/20260719-terminal-resource-quota.md`](../requirement/20260719-terminal-resource-quota.md)（多 attach 计配额；废止每 session 单 live attach / 409）
- [`docs/requirement/20260719-terminal-tab-keepalive.md`](../requirement/20260719-terminal-tab-keepalive.md)
- [`docs/requirement/20260719-cloud-session-mcp-boundary.md`](../requirement/20260719-cloud-session-mcp-boundary.md)
- [`docs/requirement/20260716-cloud-terminal-control-relay.md`](../requirement/20260716-cloud-terminal-control-relay.md)

## Overview

```text
Browser A (controller) ──WS──┐
                             ├─ Cloud / Agent local attach surface
Browser B (observer)  ──WS──┤      │
                             │      ▼
                             │  multi stream / multi client
                             ▼
                    SessionRuntime (Agent)
                    - clients (output fan-out)
                    - controllerClientId (0..1)
                    - WriteInput / Resize / attach-size gate
                    - take_control → last-writer-wins
                             │
                             ▼
                           PTY
```

| 路径 | 行为 |
|------|------|
| Cloud | Browser WS → 多 `streamId` → tunnel attach/input/resize/**take_control** → Agent 门禁 → PTY |
| Local | Browser WS → 多 attach → 同一 runtime 门禁 → PTY |

配额：每个存活 browser attach 仍计数；与 role 无关。默认上限 8 不变。

## Scope and non-goals

### Included（V1）

1. 去掉 per-session 独占 `writers`（原第二 attach 409）。
2. 同 session 多客户端同时收实时输出（+ 既有 replay 行为，不单独立项「零空洞」）。
3. 同一时刻仅一个 controller：可 binary input + PTY resize。
4. 后续 attach 默认为 observer；非 controller 的 input 拒绝、resize/attach 初始 size **不改 PTY**。
5. 用户确认或点「接管」后 `take_control`；成功后请求方成为唯一 controller；旧 controller 降为 observer **不断开**。
6. **当前 controller 断开**且仍有其它 live attach 时：**自动**将控制权交给按规则选出的剩余 client（无需再点接管）。
7. Agent 为最终 PTY 写门禁；Cloud 转发写操作，**不做**派生 early reject 镜像。
8. Tunnel：`streamId` 绑定 attachment；新增上行 `TerminalTakeControl`；下行角色经既有 `terminal_control`。
9. 前端：只读状态、输入/粘贴确认或显式接管、layout 不发 PTY resize；WS 失败中性文案 + toast 去重。

### Excluded（V1 明确不做）

1. `control_epoch` / `expected_control_epoch` **CAS**；并发 takeover 的「至多一个成功」严格语义。
2. Attach **preflight** API；依赖浏览器解析 WebSocket upgrade 前 HTTP body。
3. Cloud 侧 controller 镜像 early reject（未知/已知一律：input/resize/take_control **转发** Agent）。
4. Replay/live 无丢失的专项改造（沿用现有；残余窗口记 Risk）。
5. 多写协作、**观察者未确认时抢写**、跨 Cloud 实例协调、新 ACL。
   （**例外**：controller **断开**时 V1 **会**自动把控制权交给仍在线的某一 attach，见 D4；这不是「第二端一连就踢/抢」。）
6. 改配额主体/默认值/管理 API；改 session key 拼 `deviceId`。
7. MCP Server 实现（未来共用同一 controller 规则即可）。

## Design decisions

### D1. Agent 权威 + 简单 controller 指针

- `SessionRuntime` 持有 `controllerClientId`（`""` = 无 controller）与 `clients`。
- **最终门禁**：仅 `controllerClientId` 对应 client 可 `WriteInput`、应用 PTY `Resize`、应用 attach 初始 size。
- Cloud / Local **删除** terminal 路径上的独占 `writers` 409。
- Cloud **不**维护权威 controller 表；只按 `streamId` 转发。

### D2. 首 attach 选举

在 `attach` 锁内：

1. 创建 client 并加入 `clients`。
2. 若当前无 controller → 该 client 为 controller；否则为 observer。
3. **仅 controller** 且 attach 带合法 cols/rows 时才 `Resize` PTY。
4. 向该 client 发 `started`（含 `control_role`）→ 既有 replay → live。

并发双 attach：锁串行后至多一个成为首任 controller（此为 attach 选举，不是 takeover CAS）。

### D3. Browser 协议（最小）

**Client 新增**

| type | 含义 |
|------|------|
| `take_control` | 请求成为唯一 controller（无 epoch 字段） |

保留：`hello` / `resize` / `detach` / `ping`。

**Server**

`ServerControlMessage` 增加：

- `string control_role = 14`：`controller` | `observer`

（V1 **不**引入 `none` / `control_epoch`。仅当 **零** live client 时无 controller；只要还有 live client，断开后会立刻有人成为 controller，见 D4。）

下发：

| type | 何时 |
|------|------|
| `started` | attach 后：本连接 `control_role` |
| `state` | 角色变更：`control_role` + `reason` |

**reason（V1）**

| reason | 含义 |
|--------|------|
| `control_granted` | 成为 controller |
| `control_lost` | 由 controller 降为 observer |
| `controller_detached` | 原 controller 断开（被通知方若未获权，可作 informational） |
| `control_auto_granted` | 因原 controller 断开，本连接被自动选为新 controller |

**错误码**

| code | 场景 |
|------|------|
| `not_controller` | 非 controller 的 **binary input**（WS control error，不断开） |
| `terminal_attach_quota_exceeded` | 配额（HTTP 429，upgrade 前，保持现状） |

非 controller 的 **resize**：**静默忽略**（不报错、不改 PTY）。  
废止：第二 attach 的 409 active writer。

### D3.1 Tunnel 与 attachment identity（Cloud 必做，保持简单）

- **Attachment identity** = `TunnelFrame.stream_id`。
- Agent：`streamId` → 一个 runtime `Client`，随 Close/detach 销毁。
- 新增上行：

```text
message TerminalTakeControl {
  string workspace_id = 1;  // 可选冗余
  string session_id = 2;
}
// TunnelFrame: terminal_take_control = <new oneof tag>
```

| Browser | Tunnel | Agent |
|---------|--------|-------|
| binary | `TerminalInput` | Client.WriteInput（门禁） |
| `resize` | `TerminalResize` | Client.Resize（非 controller 忽略） |
| `take_control` | `TerminalTakeControl` | Client.TakeControl() |
| detach/断连 | `Close` | Client.Detach |
| — | `TerminalAttach` | Attach + 选举 + 条件 size |
| outbound | 同 stream：`terminal_control` / `TerminalOutput` | 该 Client 队列 |

Local：无 tunnel，browser 控制直接调同一 Client API。

### D4. Takeover = last-writer-wins（无 CAS）

```text
take_control from client X (in clients):
  lock:
    old = controllerClientId
    controllerClientId = X
  notify X: control_granted
  if old != "" && old != X: notify old: control_lost
```

规则：

1. 请求者必须已是本 session 的 live client。
2. 已是 controller：幂等成功（可静默或再发 granted）。
3. 有其他 controller：**直接替换**（后到的 takeover 获胜）。
4. 无 controller：请求者成为 controller。
5. **并发双 takeover**：串行后两者都可能「成功」，最终 controller 为后者；**V1 接受**（个人双端同时点接管极罕见）。不引入 epoch / `takeover_denied`。
6. 确认前 input：前端丢弃；若到达服务端 → `not_controller`，不写 PTY。
7. Takeover **不**自动 resize；新 controller 前端在 granted 后**发一次**当前尺寸 `resize`。
8. **Controller 断开 → 自动移交（V1 必须）**：
   - 若 `clients` 中仍有其它 live client：按 **确定性规则** 选一个升级为 controller（推荐：**最早 attach / 加入 `clients` 的顺序**，即 attach 时间最早的剩余 client；实现可用有序结构或 clientId 插入序，须稳定可测）。
   - 新 controller 收 `state{control_role=controller, reason=control_auto_granted}`（或 `control_granted` + 可区分 reason）。
   - 其它仍在线 peers 保持 `observer`（可选广播 `controller_detached` 仅作信息）。
   - 新 controller 前端在获权后**应**发一次当前尺寸 `resize`（与主动 takeover 相同）。
   - 若断开后 **无** 剩余 client：`controllerClientId=""`，session 无 controller（无 attach 时自然状态）。
   - **不是**：任意新 attach 自动抢权；也不是第二端一连接就踢当前 controller。仅 **当前 controller 的连接结束** 触发自动移交。

### D4b. 一致性边界：PTY 强门禁 + 角色 UI best-effort（V1）

**强一致（不可降级）** — Agent `SessionRuntime` 是唯一控制权权威：

- `controlMu` 串行化：attach 选举、`take_control`、controller detach/auto-promote、PTY input、PTY resize。
- 仅 `controllerClientId` 可 `WriteInput` / 应用 PTY resize / 应用 attach 初始 size。
- 即使浏览器 UI 过期：旧端 input → `not_controller`（不写 PTY）；旧端 resize → 静默忽略。

**尽力同步（可丢）** — `control_granted` / `control_lost` / `control_auto_granted`：

- 经每 client 有界 outbound 队列投递，供浏览器切可写/只读。
- **enqueue 失败：打日志并丢弃；不得因此 Detach 该 client，不得触发额外 controller 重选。**
- 慢浏览器仍可因 **输出 fan-out** queue 满被既有逻辑断开（资源保护）；与「角色通知失败」解耦。
- V1 **不做**：role epoch/generation、可靠角色事件队列、因 role notify 失败递归 detach/re-election、完整 actor 化 runtime。

| 情况 | V1 行为 |
|------|---------|
| 正常网络角色变化 | 浏览器收到 granted/lost/auto-granted 并更新 UI |
| 极端慢客户端 queue 满 | role UI 可能延迟/遗漏；Agent 仍阻止双写 |
| 同时接管 | LWW；后完成者为最终 controller |
| tunnel 断开 | 对应 runtime client 及时 detach → 自动移交 |

### D5. Attach 初始 size


| 角色 | attach cols/rows |
|------|------------------|
| controller | 合法则应用到 PTY |
| observer | **忽略** |

### D6. 输出

- 既有单 reader + 每 client 队列 fan-out。
- 慢客户端不得阻塞 PTY/controller。
- Replay 沿用现有；不在 V1 做 watermark 专项（Risk 记录）。

### D7. 前端（V1）

1. 解析 `started`/`state` 的 `control_role`。
2. **Observer**（及无控制权时）：
   - 状态栏只读 +「接管」按钮（文案如「只读模式中，接管会话」）。
   - 打开会话成为 observer：toast **一次**提示另一端控制 + 可点底部接管（**不**弹模态确认）。
   - 输入/粘贴：直接丢弃；不弹接管确认窗。
   - layout/ResizeObserver：只本地 fit，**不**发 PTY `resize`。
3. **Controller**：现有输入 + 尺寸变化可发 PTY `resize`。
4. 收到 `control_lost`：切只读，**不断开** WS。
5. 收到 `control_granted` 或 `control_auto_granted`：允许输入，并立即 `resize` 一次；自动移交时 toast/状态栏提示「已获得控制权」。
6. Toast：`sessionId + reason` 去重/限频。
7. WS `onerror`：**中性**「终端连接失败」；**不**默认归因 quota；**不** pretence 读取 upgrade HTTP body。配额等精确文案仅当未来有 HTTP 通道时再增强（V1 不做 preflight）。

### D8. attachment_state vs control_role

- `attachment_state`：session 是否仍有人 attach（列表摘要），语义不变。
- `control_role`：本连接是否可写；不复用 attachment 字符串。

## Affected components

| 区域 | 预期变更 |
|------|----------|
| `proto/termbridge/agent/v1/terminal.proto` | `control_role`；`take_control` 相关 client 字段若需要 |
| `proto/termbridge/shared/v1/tunnel.proto` | `TerminalTakeControl` + oneof |
| `internal/shared/dto/protocol/terminal` | type/常量/校验/`not_controller` |
| `internal/agent/.../terminal` runtime | controller 指针、门禁、LWW takeover、通知 |
| Agent/Cloud API handler | 去 `writers` 独占；处理 take_control；observer attach size |
| `cloud_client` tunnel | streamId→Client；TakeControl；条件 resize |
| Web terminal / shell / i18n | role UI、接管、layout、中性错误、toast 去重 |
| Tests | 多 attach、门禁、LWW takeover、controller 断开自动移交、quota 仍 429 |

## Data model / wire（摘要）

```text
SessionRuntime
  clients map
  controllerClientId string  // "" = none

// browser
{ "type": "take_control" }

// server started
{ "type": "started", "control_role": "controller"|"observer", ... }

// server state
{ "type": "state", "control_role": "observer", "reason": "control_lost", ... }

// server error
{ "type": "error", "code": "not_controller", "message": "This terminal connection is read-only." }
```

Cloud：`take_control` → `TunnelFrame{stream_id, terminal_take_control}`；角色/error 经同 stream 的 `terminal_control` 回传。

## Sequences（简）

**多端看**：A attach → controller；B attach → observer；输出 fan-out 至 A、B。

**接管**：B `take_control` → B controller、A `control_lost` → B `resize` → B 可输入。

**误输入**：observer binary → `not_controller`；PTY 不变。

**Controller 断开自动移交**：A(controller) 断连 → B 自动 `control_auto_granted` → B 可输入；C 仍为 observer。

## Error / UX

| 场景 | V1 行为 |
|------|---------|
| 配额满 | HTTP 429（upgrade 前）；能区分则用既有 i18n，否则中性 |
| 只读输入 | 前端拦截 + 可选服务端 `not_controller` |
| 控制权被夺 | `control_lost` → 只读栏 |
| 其它 WS 失败 | 中性连接失败；toast 去重 |

## Testing plan（V1）

1. 双 attach 均成功；仅一 controller；双方收输出。
2. Observer input 不进 PTY；resize/attach-size 不改 PTY。
3. `take_control` LWW：后者成为 controller；前者 `control_lost`。
4. Controller 断开后：若仍有 observer，**自动**选出新 controller 并通知；无剩余 client 则无 controller。
5. 配额：多 attach 计数；超限 429。
6. Cloud 与 local 语义一致（含 tunnel take_control）。
7. 前端：layout 不发 resize；确认/按钮接管；lost 后只读。

## Risks and trade-offs

1. **并发双 takeover**：V1 允许短时间两次 granted/lost 翻转；个人场景可接受。
2. **Upgrade 错误笼统**：无 preflight；可能仍难区分 quota vs 网络；用中性文案 + 限频，避免误导。
3. **Replay 窗口**：既有 attach 竞态可能丢极短输出；V1 不修。
4. **旧客户端**：不懂 role 时仍可能本地乱输入；服务端门禁兜底。
5. 漏做 tunnel `TerminalTakeControl` → Cloud 无法接管（实现阻断）。
6. **自动移交**：用户可能未意识到另一设备已获得输入权；UI 应在 `control_auto_granted` 时有明确提示。选举规则须稳定，避免测试/复现困难。
7. **角色通知 best-effort**：queue 满时 UI 可能与服务端角色短暂不一致；安全靠 Agent PTY 门禁，不靠 UI 可靠投递。

## Alternatives（已弃用的加重设计）

| 曾考虑 | V1 结论 |
|--------|---------|
| epoch CAS | 不做 |
| attach preflight | 不做 |
| Cloud early reject 镜像 | 不做 |
| role notify 失败 → detach + 重选 | **不做**（best-effort 通知；避免级联状态机） |
| 后连踢前连 | 用户否决 |
| 维持 409 | 否 |
| 双写 | 否 |

## Acceptance mapping（相对 requirement，V1 收缩）

| Requirement 点 | V1 |
|----------------|-----|
| 多端同时看 | 做 |
| 单 controller 输入+PTY resize | 做 |
| 确认/接管转移 | 做（LWW） |
| 旧端不断开 | 做 |
| controller 断开后自动移交 | **做**（有剩余 client 时） |
| 断开后无 controller 再确认 claim | **不做**（已改为自动移交） |
| 服务端门禁 | 做（Agent） |
| attach size 只读 | 做 |
| layout 不弹窗/不改 PTY | 做 |
| 配额不退化 | 做 |
| 严格并发「至多一个成功」 | **不做**（LWW） |
| 可识别 upgrade 失败原因 | **弱化**（中性文案；无 preflight） |
| Cloud early reject | **不做** |
| Replay 无空洞 | **不做专项** |

若与 Accepted requirement 字面冲突：以本 Spec 的 **V1 产品句与 Excluded** 为准进入 Plan；requirement 可在进入 Plan 时补一行「实现按 Spec V1 收缩」。

## Open technical questions

1. 自动移交选举键：推荐「最早 attach 的剩余 client」。若需「最近活跃」可后续再改；V1 以可测稳定性为准。
2. 前端是否 **必须** 键盘拦截弹窗，还是 V1 仅「接管按钮」也可？（推荐：按钮 + 首次输入弹窗都做，实现量仍可控）

## User review notes

- 用户要求语义收缩为：多端同看、单端控制、确认/按钮接管；**不做** CAS 与 preflight。
- 本版 Spec 删除 epoch、preflight、Cloud early reject、replay 硬约束；保留 Agent 门禁、tunnel take_control、LWW takeover、前端只读/接管。
- 用户更新：controller **断开**后不再保持无 controller，而是 **自动**让仍在线的某一端成为 controller（确定性选举）。
- 待用户接受本 V1 Spec 后进入 Plan。