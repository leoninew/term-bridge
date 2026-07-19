# 活跃会话终端 Keep-Alive（减少切 Tab 重连）
最后修改时间: 2026-07-19 18:20:00

Review status: Accepted

## Flow mode / Stage

标准模式 / standard；需求 / Requirement（Accepted）→ 计划 / Plan（Accepted）→ **实现 / Implementation 已完成**（commit `1b86761`）。
验证 / Verification 文档为 Draft：自动化已完成，浏览器手测可能仍不完整。

## Related

- 资源配额 / 管理员动态调配（防客户端调大保活拖垮服务）：见独立需求 `docs/requirement/20260719-terminal-resource-quota.md`。
- **本需求继续独立实现**，不等待配额系统完成。
- 本需求内后端仅保留必要的最小兼容；**按用户配额、管理端调配不在本需求范围**。

## Current understanding（2026-07-19 固化）

### 问题定性

- 日志里 `/ws` 的 `duration_ms` 是 **连接存活时长**，不是握手耗时。
- 真卡顿主因是 active-only 挂载：切 tab 会 unmount → 关 WS + `xterm.dispose()`，切回再 connect/attach/replay。
- 频繁切会话会放大短生命周期 WS、closed connection write fail、重连体感；**不是**必须同时开很多 WS 才有问题。

### 保活策略（已实现）

- hot set = 最近 **4** 个激活的 **running** 已打开 tab
- hot：保留 xterm + live WS + 收输出
- 离开 hot：立即关 WS；xterm 冻结 **30s** 后 dispose
- 30s 内再进入：可复用 xterm 并 reconnect/attach/replay
- 关 tab / 页面卸载：立即 dispose，不走延迟
- stopped session 走 history 只读，不进 hot set

### 配置

| 层 | 来源 | 默认 |
| --- | --- | --- |
| L1 | 前端代码 | hot=4, dispose=30000ms |
| L2 | yaml/env → `BrowserRuntimeConfig.terminal.keepAlive` | 可覆盖 L1 |
| clamp | 前端 | hot 1–16；dispose 0–600000 |

- L2/`__CONFIG__` **可被篡改**，只做部署建议；防滥用靠独立需求 `20260719-terminal-resource-quota`（后端默认 attach 硬顶 **8**）。
- 调试：`localStorage.termbridge.terminalDebug=1` → `keepalive.*` 日志。

### 性能认识

- **本机**：多 hot xterm 增加内存/DOM/渲染；默认 4 是体验与资源折中。
- **服务器**：每路 live attach 占一条 browser WS + 输出 fan-out 带宽；PTY 本身在 running 时已存在，与是否 attach 部分解耦。
- 后端配额默认 8 > 前端 hot 4：给 headroom，正常路径仍约 4 路 attach。

### 关键路径快照

- `web/src/features/sessions/terminalKeepAlive.ts`
- `web/src/features/sessions/terminalKeepAliveConfig.ts`
- `web/src/components/session/SessionWorkbench.vue`（多 pane 挂载）
- `TerminalView`：`connectionEnabled` / `active`

## Background

当前 workbench 对已打开会话 tab 的处理方式是：

1. `SessionWorkbench` 只挂载 **当前激活** 的 `TerminalPane`。
2. 切换 tab 时，离开的 `TerminalView` 会 `unmount`：
   - 关闭终端 WebSocket
   - `xterm.dispose()`
3. 切回同一 running session 时重新创建 xterm，并重新 `connect` + `attach`。
4. 后端 `attach` 会对 history 做 `Flush` 并回放最近输出（默认最多约 256KB）。
5. Agent 侧同一 session 同一时刻只允许一个 browser writer；旧连接未释放时新连接会 conflict。

结合 `logs/termbridge.2026-07-19.log` 与代码排查结论：

- `/ws` 的 `request completed.duration_ms` 表示 **连接存活时长**，不是握手耗时。
- 真正 `attach start → attach live` 通常只有几十毫秒。
- 用户频繁切 tab 时会出现大量短生命周期 WS、`write failed: use of closed network connection`，以及每次回来都要重新 fit / connect / replay 的体感卡顿。
- 后台 running session 的 PTY 本身继续存活（`detached`），问题集中在 **浏览器侧终端 UI 与 WS 不保活**。
- 现状下终端 live WS **通常最多 1 条**（只挂 active tab）；页面上还可能并行存在 workspace `fs/events` WS，与本需求无关。

因此本需求不是“修一个 11s 握手 bug”，而是治理 **频繁切会话导致的重连与体感延迟**。

## Goal

让用户在已打开的多个 running session tab 之间频繁切换时：

1. **切回最近使用过的 tab 时尽量秒开**，不必每次都完整重建 xterm + WebSocket + history replay。
2. 用明确的 **“最近 4 个激活 tab 保活 + 其余延迟销毁”** 策略，在体验与资源之间取得平衡。
3. 保持现有协议与后端约束兼容：
   - session 级单 browser writer
   - running session PTY 不因切 tab 而退出
   - 停止会话仍走 history 只读视图
4. 减少切 tab 引发的无害噪音日志（如 closed connection write fail），至少不因本方案新增更差竞态。
5. 行为可观测：关键保活/销毁/复用路径有调试日志（沿用 `termbridge.terminalDebug` 或等价机制）。
6. 参数可配置：至少有代码默认值；推荐经现有 BrowserRuntimeConfig 通道做部署期下发（含云端部署差异化）。

## Non-goal

1. 不改终端二进制协议帧格式、subprotocol、`hello/resize/replay_*` 语义本身。
2. 不做真正的多用户/多浏览器同时 attach 同一 session（仍保持单 writer 约束，除非后续独立需求）。
3. 不把 xterm 实例状态迁入 Pinia 作为权威状态。
4. 不在本需求中重写 history 存储格式，或把 replay 改为完全服务端推流架构。
5. 不优化 Cloud 隧道整体延迟、Agent 与 Cloud 之间的中继性能。
6. 不把“后台未打开 tab 的 running session”自动全部 attach；仅针对 **已打开 tab**。
7. 不解决用户首次打开某个 session 时的首次 fit / 首次 connect 体验（可顺带不回退，但不作为本需求主目标）。
8. 不改 session 创建/关闭/删除/rerun 业务语义。
9. 不提供用户可见的保活参数设置页。
10. 不实现动态 Client Policy API / 运营后台热更新（L3）；仅预留语义，不在本需求交付。
11. 不把 keep-alive 参数做成必须登录云端才能工作；缺省本地默认值始终可用。
12. **不**实现按用户资源配额、管理员动态调配、配额管理 API（见 `20260719-terminal-resource-quota.md`）。
13. **不**把“防客户端调大保活”的服务端硬顶作为本需求阻塞项；可在配额需求中系统化解决。若实现期顺手加进程级全局 attach 上限，仅作可选加固，不作本需求验收必选项。

## Keep-alive policy（已选定策略）

### 策略结论

**合理，作为本需求默认策略采纳**，并细化为可实现规则：

> **最近激活的 4 个 running tab 保持 `tab + xterm + WS` 全量保活并继续收输出；不在这 4 个内的 tab 不维持 live 收输出，并对残留终端资源做 30 秒延迟销毁。**

### 精确规则

1. **Hot set（保活集）**
   - 仅统计 `openedTabs` 中且 `lifecycle_state=running` 的 session tab。
   - 以 **最近激活时间** 排序（LRU / MRU）：保留最近 **4** 个。
   - 当前 active tab 一定在 hot set 中（激活即刷新其时间戳）。

2. **Hot set 内**
   - 保持 `TerminalView`（或等价 live 终端实例）与终端 WS。
   - **继续接收输出**（隐藏 tab 也不暂停收包/写 xterm）。
   - 切回时优先复用实例：强制 fit + focus，不走冷启动 connect/replay（除非连接已断需自愈重连）。
   - 保活命中时 **不展示** 长时间 connecting/attaching boot overlay。

3. **离开 Hot set（第 5 个及更旧）**
   - **立即停止 live 收输出**：关闭该 tab 的终端 WS（释放后端 writer）。
   - **不**采用“挂着 WS 但前端丢弃输出”的做法（仍占 writer/队列，收益差）。
   - xterm / pane 缓存进入 **30s 延迟销毁** 窗口：
     - 30s 内再次进入 hot set：可复用尚未销毁的 xterm，再重新 connect/attach/replay。
     - 超过 30s：销毁 xterm 与相关资源；再进入时完整冷启动。

4. **关闭 tab / 页面卸载**
   - 立即取消延迟定时器并释放该 tab 的 WS + xterm。
   - 不 close 远端 PTY（除非用户显式 close session）。

5. **stopped / failed session**
   - 不进入 hot set 的 live 保活逻辑；继续 history 只读路径。

6. 参数默认值（语义固定；下发方式见 Configuration）

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `maxHotTerminals` | `4` | 最近激活且保持 WS 收输出的 running tab 上限 |
| `disposeDelayMs` | `30000` | 离开 hot set 后，xterm 等 UI 资源延迟销毁时间 |
| hot set 是否收输出 | 是 | 隐藏也收 |
| 非 hot set 是否收输出 | 否 | 先关 WS |

### 为何合理

| 维度 | 评价 |
| --- | --- |
| 切换体验 | 常见 2–4 个 tab 来回切可秒开 |
| 资源上限 | 同时最多约 4 条终端 WS + 4 套 xterm，避免无限堆积 |
| 与后端模型匹配 | 每 session 仍单 writer；跨 session 多 WS 是预期 |
| “不收输出”语义 | 用关 WS 表达，而不是假连接丢包 |
| 30s 延迟销毁 | 快速切回第 5+ tab 时，有机会复用 xterm、减少闪烁 |
| 实现复杂度 | 比“全部永久保活”可控，比“仅 active 一个”体验明显更好 |

### 需要实现时注意的边界

1. 从 hot set 淘汰时：**先关 WS，再开始 30s 计时**；避免“不收输出但仍占 writer”。
2. 30s 内复用 xterm 再连 WS：仍要 replay/补齐离线输出，不能假设画面已是最新。
3. 隐藏 hot 终端切回必须 re-fit + resize。
4. 快速切换导致的 close/write 竞态要收敛，避免持续 conflict。

## Performance impact（补记）

### 成本拆分

| 成本 | 何时产生 | 与多开终端 WS 关系 |
| --- | --- | --- |
| Running PTY / shell | session running 即存在 | **基本无关**；detached 也在 |
| 终端 WS + attach client | 浏览器 attach 后 | **直接相关** |
| xterm 渲染/scrollback/写队列 | live 终端实例存在时 | **直接相关，浏览器侧更重** |

### 本机（浏览器 + 本机 Agent）

- **浏览器**：每 hot 终端约 1× xterm（scrollback 5000）+ 最多 4MB write queue + 1 WS。3–4 路通常可接受；多路同时狂刷日志时主线程/内存更敏感。
- **本机 Agent**：每 attach 多 1 个 client queue（默认 64 条 / 最多 4MB）与桥接 goroutine；**不**额外复制 PTY。4 路空闲/轻输出通常很轻。
- 本机开发时两边叠加，但瓶颈更常在 **多 xterm + 高输出**，不是握手。

### 服务器 / Cloud

- Local-only：影响≈ Agent 侧。
- Cloud 中继：每 hot 终端多 1 条浏览器 WS + 隧道 stream 转发；连接数随 hot set 线性增加，仍通常小于多 PTY 本身。

### 与本策略匹配

- 上限 4 路 live 收输出：把成本锁在可预期范围。
- 非 hot 立即停收输出：避免“后台很多假活连接”。
- 30s 仅延迟 UI/xterm 销毁，不延长非 hot 的 WS 占用。

## Configuration（配置化与云端控制）

### 现状：项目里已有的下发通道

| 通道 | 能力 | 是否适合 keep-alive |
| --- | --- | --- |
| 前端写死常量 | 最简单 | 可做 v0，但难云端差异化 |
| Agent/Cloud `configs/*.yaml` + env | 后端/部署期配置 | 适合作为 **权威默认值来源** |
| `BrowserRuntimeConfig` → `index.html` 注入 `window.__CONFIG__` | 页面加载时注入公开 bootstrap 配置 | **最贴合**“部署/云端决定前端行为” |
| 前端 `import.meta.env`（Vite dev） | 仅本地开发 | 可对齐 yaml，不作为生产主路径 |
| 独立 Client Policy API（现无） | 可动态、可按用户/租户 | 真·远程运营开关，**本需求默认可不做** |
| `localStorage` 覆盖 | 调试/高级用户 | 仅调试，不作为云端控制 |

当前 `BrowserRuntimeConfig` 只承载公开连接信息（`version` / `local.*` / `cloud.*`），**还没有 terminal/UI 策略字段**。

后端已有 `terminal.history|replay|client` 配置，但是 **Agent 侧 history/queue**，不是浏览器 keep-alive。

### 推荐分层（由浅到深）

#### L1. 代码默认值（必有）

- 前端内置：`maxHotTerminals=4`，`disposeDelayMs=30000`。
- 即使没有任何下发，行为也可预期。

#### L2. 部署期配置 → BrowserRuntimeConfig（**本需求推荐纳入**）

在 yaml/env 增加类似：

```yaml
terminal:
  keepalive:
    max_hot_terminals: 4
    dispose_delay_ms: 30000
```

env 示例：

- `TERMBRIDGE_TERMINAL__KEEPALIVE__MAX_HOT_TERMINALS=4`
- `TERMBRIDGE_TERMINAL__KEEPALIVE__DISPOSE_DELAY_MS=30000`

并通过已有 `BuildBrowserRuntimeConfig` 投影到浏览器：

```json
{
  "terminal": {
    "keepAlive": {
      "maxHotTerminals": 4,
      "disposeDelayMs": 30000
    }
  }
}
```

前端 `useRuntimeConfigStore` 读取；缺省/非法值回退 L1。

**“云端控制”在这一层的含义：**

- Cloud 进程用自己的 `configs/config.yaml` / 环境变量决定 **Cloud 托管 Web** 的 keep-alive 参数；
- Agent 本地 Web 用 Agent 自己的配置；
- 改配置后需 **重启对应服务**（或至少重建 runtime config 注入），不是运行中热推送给已打开页面；
- 这是 **部署/环境级控制**，不是按用户实时远程运营台。

#### L3. 动态 Client Policy / 按用户配额（**非本需求；见独立文档**）

若未来要“云端运营后台随时改、已打开页面也生效 / 按租户不同策略”：

1. Cloud 提供 `GET /api/client-policy`（或挂在既有 bootstrap 接口）。
2. 前端启动或 workbench 进入时拉取并缓存。
3. 可选：事件推送失效刷新。
4. 需鉴权、缓存、默认回退。

这会引入新 API 与运营面，**超出本次 keep-alive 体验优化范围**。需求中仅预留字段语义兼容，不实现 L3。

#### L4. 用户 UI 设置页

- 本次 **不做**。
- 避免普通用户把 `maxHotTerminals` 拉太高导致本机卡顿。

### 校验规则（无论哪层）

| 字段 | 约束 |
| --- | --- |
| `maxHotTerminals` | 整数，建议 clamp 到 `1..16`；非法回退默认 4 |
| `disposeDelayMs` | 整数毫秒，建议 clamp 到 `0..600000`；非法回退默认 |
| 未知字段 | 忽略 |

### 与“只改前端常量”的取舍

| 方案 | 优点 | 缺点 |
| --- | --- | --- |
| 仅常量 | 实现最快 | 云端/本地无法差异化，调参要发版 |
| L2 RuntimeConfig | 复用现有注入链；cloud/local 可不同默认 | 要改 Go DTO + yaml + 前端 parse；改参需重启服务 |
| L3 Policy API | 真动态云控 | 范围大，本需求不值得 |

**本需求默认推荐：L1 + L2。** 若希望首版更小，可只做 L1，L2 列 follow-up。

### 云端控制场景对照

| 诉求 | 用哪一层 |
| --- | --- |
| Cloud 部署默认 4，某私有化想改成 2 | L2 yaml/env |
| 本地 Agent 与 Cloud Web 不同策略 | 各自 L2 配置 |
| 运营后台按租户远程改、不发版 | L3（后续） |
| 开发者临时调试 | 常量 / 临时 env / 可选 debug 覆盖 |

## User scenarios

### S1. 两个 running session 来回切

1. 用户打开 session A、B，均为 running。
2. 在 A 输入并看到输出，切到 B，再立刻切回 A。
3. 期望：A/B 都在 hot set；切回秒开，不完整冷启动 replay。

### S2. 频繁在 ≤4 个 tab 间切换

1. 打开 3–4 个 running tab，10–30 秒内快速切换。
2. 期望：均保活收输出；无明显 connecting 长等待；无持续 conflict。

### S3. 第 5 个 tab 挤出 hot set

1. 已有 A/B/C/D 在 hot set，再激活 E。
2. 最久未激活者（如 A）离开 hot set：立即关 WS，开始 30s 销毁计时。
3. 30s 内回到 A：可复用未毁 xterm，但需重新 connect/attach/replay。
4. 超过 30s 再回 A：完整冷启动。

### S4. 关闭 tab / 关闭页面

1. 关闭某 tab 或刷新页面。
2. 期望：立即释放对应 WS/xterm；PTY 不因此退出。

### S5. stopped session

1. stopped tab 走 history 只读。
2. 期望：不建立 live WS，不占 hot set 名额。

### S6. 布局变化后的尺寸

1. hot 且隐藏的终端，切回时窗口尺寸可能已变。
2. 期望：re-fit 并发送正确 resize。

### S7. 部署配置覆盖

1. 运维将 `max_hot_terminals` 设为 2 并重启服务。
2. 浏览器加载页面后最多保活 2 路 live WS。
3. 缺省或非法配置时回退 4 / 30000。

## Acceptance

1. **策略落地**
   - 最近 N 个激活的 running tab（默认 N=4）：保活 xterm + WS，并继续收输出。
   - 非 hot set：不维持 live 收输出（关 WS）；xterm 等资源按 `disposeDelayMs`（默认 30s）延迟销毁。
   - 关闭 tab/页面：立即清理，不等待延迟窗口。

2. **切换体验**
   - hot set 内切回：非冷启动；无长时间 boot overlay。
   - 非 hot / 已销毁后切回：允许重连 + replay，行为正确。

3. **正确性**
   - 切 tab 不误杀 running PTY。
   - 同一 session 无长期双 writer / 持续 409 conflict。
   - hot 隐藏期间输出，切回后仍可见。
   - 非 hot 期间漏接的输出，再次 attach 后通过 replay/live 恢复近期内容（受现有 replay 上限约束）。

4. **尺寸与焦点**
   - 切回后尺寸正确并 resize。
   - 焦点可回到终端。

5. **资源边界**
   - 同时 live 收输出的终端 WS ≤ 配置的 maxHotTerminals（自愈重连瞬间重叠应很快收敛）。
   - 延迟销毁定时器可取消、无泄漏。

6. **配置**
   - L1 默认值始终可用。
   - 若实现 L2：yaml/env 可覆盖，并经 BrowserRuntimeConfig 到达前端；非法值回退默认。
   - 不依赖动态 Policy API。

7. **可观测性**
   - hot 命中、挤出、关 WS、延迟销毁、复用、冷启动、配置来源/生效值等路径有 debug 日志。

8. **回归**
   - 单 session、history 只读、主题切换、replay 提示、trusted size attach 不回退。

## Open questions

无（已按推荐默认关闭）。

### 关闭记录（2026-07-19 用户：keep-alive 按推荐，进入 Plan）

| 项 | 结论 |
| --- | --- |
| Q1–Q3 / Q5 | 最近 4 hot 保活收输出；非 hot 关 WS；30s 延迟销毁；跨 session 多 WS 允许；hot 无长 overlay |
| Q4 | 前端为主；后端仅 close 竞态必要时小修；L2 含 Go RuntimeConfig 投影 |
| Q6 | **是**：离开 hot set 后保留冻结 xterm 30s |
| Q7 | **B：L1 + L2**（常量 + yaml/env → BrowserRuntimeConfig） |
| 配额 | 独立需求，不阻塞本实现 |


## Decisions

1. 问题主因是 active-only 挂载导致切 tab 冷启动，不是 `duration_ms` 误读。
2. running PTY 在 detached 时继续运行；切 tab 不 close session。
3. xterm/WS 保持组件局部，不迁入 Pinia 权威存储。
4. stopped session 走 history 只读，不进 live hot set。
5. **保活策略**：最近 4 个激活 running tab 保活并收输出；其余关 WS 不收输出，xterm 30s 延迟销毁。
6. 跨 session 多 WS（≤ maxHotTerminals）预期允许。
7. **配置**：**L1 + L2**（代码默认 + yaml/env → BrowserRuntimeConfig）；L3/按用户配额不做（独立需求）。
8. 标准模式：Requirement → Plan → Implementation → Verification。
9. **范围拆分**：防滥用/按用户配额/管理员调配 → `20260719-terminal-resource-quota.md`；本需求只做 keep-alive 体验与（可选）部署期 L2 建议配置。
10. 用户确认：本需求继续推进实现，不阻塞于配额需求。

## Risk

1. **内存与 CPU**：最多约 N 套 xterm 同时存活；低配机 + 多路高输出仍可能吃紧。
2. **隐藏终端尺寸失真**：切回必须强制 re-fit/resize。
3. **关闭竞态**：挤出 hot set / 快速切换时旧连接 write fail；需收敛。
4. **输出积压**：hot 隐藏终端高输出仍可能触达前后端 4MB 队列上限。
5. **非 hot 漏输出**：关 WS 后只能靠再次 attach 的 replay 恢复近期内容。
6. **范围蔓延**：避免扩成虚拟化 tab / 改协议 / L3 动态策略平台。
7. **Vue 生命周期**：多实例缓存时，关 tab、session 停止、workspace 切换必须清定时器与连接。
8. **配置错误**：过小导致体验回退、过大导致资源压力；必须 clamp + 默认回退。

## Assumptions

1. local 与 cloud 同一套前端 workbench 行为对齐。
2. 主痛点是已打开 tab 切换，不是首次打开。
3. 用户接受“只有最近 N 个 live 收输出”，更老 tab 需重连。
4. 默认参数 4 / 30s；可通过 L2 部署配置覆盖，无需用户设置页。
5. “不收输出”= 不维持 live WS，而不是挂 socket 丢消息。
6. “云端控制”若指部署差异化，用 Cloud 进程的 yaml/env + RuntimeConfig 注入即可；若指运营热更新，需后续 L3。

## User review notes

- 2026-07-19：补充性能影响说明。
- 2026-07-19：用户提出策略“最后启用的 4 个 tab+ws 保活；其他 tab 的 ws 不收输出、30 秒延迟销毁”。评估为 **合理**，已写入 Keep-alive policy。
- 2026-07-19：补充 Configuration：L1 常量 / L2 RuntimeConfig / L3 Policy API 分层；推荐本需求做 L1+L2。
- 2026-07-19：用户决定 **按用户资源配额 + 管理员动态调配** 拆到新需求 `20260719-terminal-resource-quota.md`；**本 keep-alive 需求继续实现**，不依赖配额系统。
- 2026-07-19：用户确认 keep-alive 按推荐，进入 Plan → Accepted。
- Q6/Q7 按推荐关闭。

### 补充认识（2026-07-19 实现后）

| 项 | 结论 |
| --- | --- |
| 实现状态 | 已合入 `1b86761` |
| 配额关系 | 独立需求 `05efe92`；keep-alive 不依赖配额即可工作 |
| 默认 hot/dispose | 4 / 30000ms |
| Verification | Draft：单测 OK；浏览器手测可能未完成 |

