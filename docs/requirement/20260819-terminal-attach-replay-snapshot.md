# 终端 Attach 重放的快照恢复体验

最后修改时间: 2026-08-19 09:59:43

Review status: Accepted

## Flow mode / Stage

标准模式 / standard；需求 / Requirement。

## Background

`/sessions` 已实现 running terminal 的 keep-alive：最近使用的 terminal pane 会保留 xterm 与 WebSocket；离开 hot set 的 pane 会关闭 WebSocket，但在 30 秒延迟销毁窗口内继续保留 xterm 实例和其当前 buffer。

当冷 pane 再次进入 hot set 时，`TerminalView` 复用该 xterm 并建立新的 WebSocket attach。Agent 端 attach 会：

1. flush `history.log`；
2. 读取 history 文件最近 `terminal.replay.max_bytes`；
3. 按 `terminal.replay.chunk_bytes` 发送二进制数据；
4. 通过 `started -> replay_started -> binary chunks -> replay_finished` 表达生命周期。

当前默认配置是最大 256 KiB、每块 64 KiB。前端收到二进制帧后无条件调用 `xterm.write(data)`；`replay_started` 仅显示“正在回放历史”提示，没有重置或替换已有 xterm buffer。

因此，复用旧 pane 后 attach 的 history tail 会追加到旧画面。旧画面往往已经包含相同输出，用户看到大量重复的 scrollback，主观体验是“滚屏了一年”。这不是普通窗口尺寸变化的必然结果：纯 `ResizeObserver` 仅发送 resize control；产生该现象的前提是 WS 重新 attach。

## Current implementation and evidence

### Frontend state lifecycle

- `web/src/components/session/SessionWorkbench.vue` 将非 hot running tab 的 `connectionEnabled` 设为 `false`。
- `web/src/components/terminal/TerminalView.vue` 在 `connectionEnabled=false` 时关闭 socket，但不 dispose xterm；注释明确为“close WS but keep xterm (frozen cold pane)”。
- `connectionEnabled=true` 时，`TerminalView` 对同一实例再次调用 `connect()`；没有清理 scrollback 或 xterm write queue。
- `useTerminalSocket` 将每个 binary WebSocket 帧直接交给 `TerminalView` 的 `xterm.write(data)`。
- `replay_started` / `replay_finished` 目前仅改变 `replaying` UI 状态，未定义“这段二进制数据替换已有显示”的客户端语义。

### Agent attach ordering

- `internal/agent/application/task/terminal/runtime.go` 的 `attach()` 先把 client 加入 `r.clients`，随后调用 `enqueueReplay()`。
- `readLoop()` 每读到 PTY chunk 就先写 history、再调用 `publishBinary()` 向当前 clients 广播。
- attach replay 与 `readLoop()` 没有共享的 replay/live 串行边界。因此在 client 加入与 `replay_finished` 入队之间，新的 live chunk 可以并发入该 client queue。

这意味着“前端收到 `replay_started` 就清屏”不足以构成正确实现：虽然能减少旧 buffer 的重复，但服务端没有保证 replay snapshot 与后续 live output 的连续顺序，仍可能出现 replay 与 live chunk 交错、重复或缺口。

### 术语澄清

- **历史尾部 / history tail**：`history.log` 的最近原始字节，最多 256 KiB；不是 xterm 的序列化屏幕状态。
- **屏幕恢复 / screen restore**：客户端丢弃旧的 xterm buffer，以新的 history tail 和之后的 live stream 重建可见内容。
- **快照边界 / snapshot boundary**：一个明确时刻。此前已经写入 history 的输出属于 replay；此后的 PTY 输出仅作为 live stream 依序发送。

本任务不把原始 history tail 错称为完整 TUI snapshot。它只承诺恢复近期可见上下文，并消除旧 buffer 与回放尾部重复拼接。

## Goal

1. running session 重新 attach 时，将 replay 定义为一次**屏幕恢复**，而不是对旧 xterm buffer 的追加。
2. 对复用的 frozen xterm：在新 replay 生效后，画面只保留该次 replay 的近期 history tail 与其后的 live output，不保留旧 buffer 中重复的内容。
3. 建立 replay snapshot 与 live output 的服务端顺序边界，确保客户端可按 `replay_started -> replay bytes -> replay_finished -> later live bytes` 重建，不会因 attach 并发丢失或重复一个 PTY chunk。
4. 保留 hot terminal 的无重连切换体验；已有 WS 未断开时不触发重放，也不清空画面。
5. 继续沿用 bounded replay、truncated 提示、控制权和 resize 语义；修复不得重新引入长 history 导致 attach 失败的问题。
6. 将默认 attach replay 上限从 256 KiB 降为 64 KiB，减少必要重连时的首屏写入量与 scrollback 压力；显式配置仍可覆盖默认值。

## Non-goal

1. 不实现跨 attach 的可靠 seq/ack、断点续传或完整终端录制协议。
2. 不把 `history.log` 改为 xterm buffer 的精确快照，不保证从任意 ANSI/UTF-8 字节边界开始都能完整还原远端 TUI 内部状态。
3. 不提高默认 replay 预算、history 容量、xterm scrollback 或 client queue 容量来掩盖重复问题。
4. 不改变纯本地布局 resize 的行为；它不应主动重连或 replay。
5. 不重做 keep-alive 的 hot-set 选择、配额、多设备 controller 选举或 session 生命周期。
6. 不为用户增加 replay 大小、清屏或保活策略的设置入口。

## User scenarios

### S1. 冷 pane 回到 hot set

1. 用户打开 running session A；A 离开 hot set，WS 被关闭但 xterm 在 30 秒窗口内保留。
2. A 再次成为 hot，前端复用 xterm 并重新 attach。
3. 期望：旧 buffer 被作为过期画面替换；只显示本次 history tail 和之后的 live output，不出现两份相同历史连续拼接。

### S2. hot tab 间切换

1. A、B 都在 hot set，二者 WS 均持续连接。
2. 用户在 A、B 间切换和改变窗口尺寸。
3. 期望：没有 reconnect/replay；既有 scrollback 保持，尺寸变化只触发必要 resize。

### S3. attach 期间仍有 PTY 输出

1. session 正在持续输出，浏览器同时发起 attach。
2. 期望：客户端看到一个有序流：快照尾部后紧接 attach 之后的 live output；不漏掉或重复快照边界附近的 PTY chunk。

### S4. 已销毁或首次创建 pane

1. 新建 xterm 或超过 keep-alive 延迟后重新创建 xterm，再 attach running session。
2. 期望：同样以 replay 作为该 xterm 的初始画面；不依赖旧 buffer，也不出现重复或空白卡死。

### S5. 截断 replay

1. history 大于 replay budget。
2. 期望：仅显示最近 history tail；保留现有“历史已截断”可见反馈，且不因替换行为把旧 scrollback 当作补全数据保留下来。

## Acceptance

1. 任意一次重新 attach 的 `replay_started` 明确启动客户端的“替换旧画面”生命周期；该周期完成后，旧 xterm buffer 的终端输出不会与本次 replay 并存。
2. 对 xterm 内部异步 write queue，替换操作必须处理已排队或正在写入的旧数据，不能在 reset 后又由旧回调把过期输出写回。
3. Agent 保证单个 newly attached client 的 outbound 顺序为：`started`、`replay_started`、该时刻的 bounded history tail、`replay_finished`、随后发生的 live binary output。
4. 对 replay snapshot 边界：每个已被 read loop 接收并写入 history 的 PTY chunk 要么进入 replay，要么在 replay 完成后作为 live output 发送，不能两者皆无或两者皆有。
5. hot WS 未断开时的 tab 切换和纯 layout resize 不触发客户端 reset 或 replay。
6. cold pane 30 秒内重连、超过 30 秒冷启动、首次打开 running session、cloud relay 路径均遵守同一恢复语义。
7. `replay_finished.truncated=true` 继续向用户表达“仅显示最近历史”；不再让 xterm 的旧 scrollback 掩盖这一事实。
8. 添加前端与 Go 回归测试，至少覆盖：
   - 复用 xterm 后 replay 替换旧内容；
   - reset 与 pending write queue 的先后顺序；
   - attach 与 concurrent PTY output 的 replay/live 边界；
   - 已有 replay byte/chunk budget、队列背压和控制帧顺序不回退。

## Open questions

1. **客户端 reset 时机与异步写入实现**：`Terminal.reset()` 不能假设会取消正在进行的 `terminal.write()`。实现需要在 xterm controller 中定义受控的“替换 epoch”，并缓冲新 replay bytes，直到旧 write drain 后再 reset；或采用能证明不会回写旧数据的等价机制。计划阶段须确认具体 API 和测试方式。
2. **服务端同步粒度**：推荐新增专门的 output/replay mutex，使 `history.Write + publishBinary` 与“flush history、取 tail、登记 client、排 replay”形成原子顺序段。计划阶段须验证锁顺序、队列上限和 attach 并发不会引入死锁或阻塞 PTY read loop。
3. **UI 反馈**：现有“正在回放历史”文案可复用；是否在 restore 期间遮挡旧画面或显示“已恢复最近输出”，在计划阶段根据现有 overlay 交互决定。无论选择何种文案，不应把提示写入终端数据流。

## Decisions

1. 本任务采用标准模式 / standard：Requirement → Plan → Implementation → Verification；本轮只完成 Requirement 草稿。
2. 将 attach replay 产品语义定为**替换式屏幕恢复**，不是追加式日志输出。
3. 该问题同时包含前端 buffer 生命周期与 Agent replay/live 并发顺序；两者必须一起解决，不能只在 `replay_started` 调用一个裸 `clear()`。
4. 默认 `terminal.replay.max_bytes` 从 256 KiB 调整为 64 KiB；`terminal.replay.chunk_bytes` 保持 64 KiB，因此默认 replay 最多一个 binary chunk。history 持久化容量和 client queue 容量保持现状。
5. 历史 tail 只能恢复近期上下文，不作为精确的 TUI screen snapshot 承诺；该限制必须在实现和用户反馈中保持诚实。

## Risk

1. xterm 的 parser、write callback 与本地 write queue 是异步的；错误的 reset 会让旧输出在新 replay 之后回写，形成隐蔽竞态。
2. 若服务端仅以大锁包住文件读取或 client queue 操作，可能拖慢 PTY read loop；实现必须采用无阻塞 enqueue 与清晰锁边界。
3. history tail 的 byte 截断仍可能从 UTF-8 字符或 ANSI escape sequence 中间开始；本任务不扩大为完整 terminal state checkpoint，但实现不得因此 panic 或阻塞 attach。
4. cloud relay 必须保留 browser 看到的文本/二进制顺序，否则本地 Agent 的正确顺序无法转化为实际体验。
5. 终端在 reconnect 期间会丢失超过 bounded replay window 的更早 scrollback；这是当前 bounded-history 产品边界，不能再用旧 buffer 静默掩盖。

## User review notes

- 2026-08-19：用户在 `/sessions` 观察到尺寸变化或会话加载后的大段历史重放，追问实际 replay 范围。
- 2026-08-19：当前实现核对确认默认 replay 为 256 KiB、按 64 KiB chunk 发送；纯 resize 不应 replay，重新 attach 才会发生。
- 2026-08-19：用户指出重复追加导致“滚屏了一年”的体验不可接受，要求优化 attach 时的屏幕重放。
- 2026-08-19：用户要求使用 `$specflow` 标准模式记录任务，并把问题与实现约束厘清；本文件据当前代码事实建立 Requirement Draft。
- 2026-08-19：用户确认暂不追求完整终端屏幕快照，要求先将默认 replay 从 256 KiB 降为 64 KiB，并要求开始 Plan；Requirement 视为 Accepted。
