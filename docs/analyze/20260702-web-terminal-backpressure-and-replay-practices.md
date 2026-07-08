# Web Terminal Backpressure 与长会话 Replay 实践分析

日期：2026-07-02

## 1. 背景

终端界面出现：

```text
[termbridge:terminal_stream_error] client queue full
```

结合当前实现，确认该错误主要由 **会话运行时间过久、终端输出内容过多** 触发。其本质不是子进程崩溃，而是 Web Terminal 输出链路的背压保护被触发：某个浏览器终端连接的后端输出队列塞满或超过字节上限，系统主动断开该 terminal stream，避免慢客户端拖垮 PTY/runtime。

## 2. 当前错误链路

### 2.1 前端展示

`web/src/components/terminal/TerminalView.vue:46` 在收到 terminal control error 后，会把错误写入 xterm：

```text
[termbridge:${message.code}] ${message.message}
```

所以：

```text
code    = terminal_stream_error
message = client queue full
```

最终显示为：

```text
[termbridge:terminal_stream_error] client queue full
```

### 2.2 Gateway 转发 stream error

`tunnel` 模式下，`internal/transport/http/gatewayapi/route.go:143` 处理 `FrameError`，并转成浏览器 terminal error：

```go
terminalproto.ServerMessage{
    Type: terminalproto.TypeError,
    Code: "terminal_stream_error",
    Message: message,
}
```

### 2.3 Agent attach 失败时返回 FrameError

`internal/application/agent/client.go:283` 调用 runtime attach：

```go
stream, err := c.config.Runtime.Attach(ctx, payload.WorkspaceId, payload.SessionId)
if err != nil {
    _ = writeTerminalError(ctx, conn, writeMu, frame.StreamId, err.Error())
    return
}
```

如果 `Attach` 返回 `client queue full`，该错误会通过 `FrameError` 传回 gateway。

### 2.4 Runtime attach/replay 返回 client queue full

`internal/application/terminal/runtime.go:85` attach 时会先 enqueue replay：

1. `started`
2. `replay_started`
3. history binary
4. `replay_finished`

其中任何一步 enqueue 失败都会返回：

```go
fmt.Errorf("client queue full")
```

尤其是 `internal/application/terminal/runtime.go:101` 当前会把 history 文件作为一个 binary payload 入队：

```go
if len(data) > 0 {
    if !client.enqueue(Outbound{Kind: OutboundBinary, Binary: data}) {
        return fmt.Errorf("client queue full")
    }
}
```

## 3. 当前实现中的关键不一致

### 3.1 History 上限大于 client queue 字节上限

当前默认值：

`internal/application/terminal/registry.go:34`

```go
DefaultClientQueueSize  = 64
DefaultClientQueueBytes = 4 * 1024 * 1024
```

历史默认上限在产品/配置文档中是：

```text
history.max_bytes = 5242880 // 5 MiB
```

这导致一个结构性问题：

```text
history.max_bytes       = 5 MiB
client.queue.max_bytes  = 4 MiB
attach replay           = 一次性 enqueue 整个 history
```

当长会话历史接近 5 MiB 时，attach replay 的单个 history payload 可能天然超过 client queue byte cap，从而 attach 失败。

### 3.2 Attach replay 与 history storage 没有分层

当前模型近似为：

```text
持久化保存多少历史 = attach 时尝试回放多少历史
```

这不适合长会话。标准实践应区分：

```text
history.max_bytes          // 持久化保存多少历史
terminal.replay.max_bytes  // attach 时最多回放多少历史
client.queue.max_bytes     // 单个客户端最多积压多少输出
```

### 3.3 Replay 一次性发送完整 history

当前 replay 把 history 文件作为一个 binary message 入队。长输出场景下，这会带来：

- 单包过大；
- 浏览器处理压力高；
- 无法在 replay 过程中平滑响应断开；
- 无法做细粒度 backpressure；
- history size 与 client queue cap 容易直接冲突。

### 3.4 queued bytes 生命周期需要由协议强制保证

`Client.enqueue` 会增加 `queuedBytes`：

`internal/application/terminal/registry.go:822`

```go
if c.queuedBytes+size > c.runtime.registry.clientQueueBytes {
    return false
}
c.queuedBytes += size
```

但 `queuedBytes` 需要在 outbound 被写出后扣减。当前 `Client.MarkSent(outbound)` 存在，但 `TerminalStream` interface 只暴露：

`internal/application/agent/runtime_access.go:26`

```go
Outbound() <-chan terminalapp.Outbound
WriteInput(data []byte) error
Resize(cols int, rows int) error
Detach(reason string)
```

接口没有强制消费者在 WebSocket 写出后 ack/mark sent。长期看，queue byte accounting 不应依赖调用方“记得调用某个额外方法”，而应由接口或封装结构保证。

## 4. 主流/标准实践

### 4.1 Runtime 输出不能被任一客户端拖慢

标准原则：

```text
PTY/runtime 的 stdout/stderr 读取和 history 写入是主路径；Web 客户端只是订阅者，不能反向阻塞 runtime。
```

也就是说：

```text
PTY output -> history writer: 必须持续写，受 history policy 限制
PTY output -> attached clients: 尽力推送
某个 browser client 慢了: detach/drop 这个 client
```

不能因为一个 WebSocket 慢导致：

- PTY read loop 阻塞；
- history 不写；
- 其他客户端卡住；
- 进程执行受影响。

当前 `slow client queue full -> detach client` 的方向是正确的，但 attach-time replay 不应走同样的失败语义。

### 4.2 History storage 与 attach replay 必须分离

标准模型：

```text
history.max_bytes          = 持久化历史容量
terminal.replay.max_bytes  = attach 时最多回放容量
client.queue.max_bytes     = 单个客户端实时积压容量
```

推荐不变量：

```text
terminal.replay.max_bytes < client.queue.max_bytes
```

例如：

```text
history.max_bytes          = 20 MiB
terminal.replay.max_bytes  = 1 MiB 或 2 MiB
client.queue.max_bytes     = 4 MiB 或 8 MiB
```

长会话可以保存较多历史，但 attach 时只回放最近上下文。

### 4.3 Attach replay 应回放 tail，而不是全量历史

主流做法是：

```text
started
replay_started(truncated=true/false)
last N bytes / last N lines
replay_finished
进入 live stream
```

而不是：

```text
started
replay_started
entire history file
replay_finished
```

常见策略：

| 策略 | 说明 |
| --- | --- |
| last N lines | 类似 terminal scrollback，保留最近 1000/5000/10000 行 |
| last N bytes | 比 line 更容易控制内存，例如最近 1-2 MiB |
| viewport + scrollback | 初次只发最近屏幕附近内容，更多历史另走 API |
| paginated history | 用户滚动或点击时再按范围加载旧历史 |

对 TermBridge 当前阶段，推荐：

```text
attach replay 默认只发最近 1-2 MiB 或最近 5000-10000 行，以先达到的限制为准。
如果历史被截断，明确发送 truncated=true。
```

完整历史查询应走 history API、下载日志或专门的 history viewer，而不是依赖 WebSocket attach replay。

### 4.4 Replay 必须 chunked

即使 replay 只发 1 MiB，也不应作为一个 binary frame 一次性发送。

推荐：

```text
history tail -> 32 KiB / 64 KiB chunks -> client queue
```

好处：

1. 避免单个 message 超过 queue byte cap；
2. replay 过程中可响应断开；
3. 能和 live output 做公平调度；
4. 能更准确地做 backpressure；
5. 避免浏览器一次处理超大 binary frame 卡 UI。

建议默认 chunk size：

```text
32 KiB 或 64 KiB
```

### 4.5 Replay 超预算应截断，不应 attach 失败

Attach replay 是“尽力补上下文”，不是“必须完整交付历史”。

标准行为应是：

```text
历史过大 -> 只发送最近部分 -> replay_finished.truncated = true -> 继续 live stream
```

不应是：

```text
历史过大 -> client queue full -> attach 失败
```

只有以下情况才应导致 attach 失败：

- session 不存在；
- runtime/PTY 不存在；
- 权限失败；
- WebSocket 已断开；
- protocol error；
- server 内部错误。

历史太长不应导致用户无法连接。

### 4.6 慢客户端策略：detach，而不是无限缓存

实时 live output 阶段，慢客户端策略应是：

```text
超过 queue/message/bytes/time budget -> 发送 error/close reason -> detach client only
```

不要无限缓存，因为 terminal output 可能无限，浏览器 tab 可能后台冻结，网络可能长期慢。

推荐 reason 区分：

```text
client_queue_full       // live output 阶段队列满
client_write_timeout    // WebSocket 写超时
client_disconnected     // 浏览器断开
replay_truncated        // replay 截断，不应作为 detach reason
```

`client_queue_full` 更适合 live-stream slow client，不适合 attach-time history 太大。

### 4.7 Live output 可以合并，但不能乱序

连续 binary chunks 可以 coalesce：

```text
如果 client queue 末尾也是 binary output，则合并到一定上限，例如 64 KiB / 256 KiB。
```

但必须保持：

- binary output 与 control message 顺序不乱；
- `started` / `replay_started` / `replay_finished` / `state` / `exited` / `error` 顺序稳定；
- error/close priority path 不能造成用户可见顺序混乱。

## 5. 推荐 TermBridge 目标策略

建议引入分层配置：

```text
history.max_bytes:
  持久化历史容量。5 MiB / 20 MiB 都可以。

terminal.replay.max_bytes:
  attach replay 容量。默认 1 MiB 或 2 MiB。

terminal.replay.max_lines:
  attach replay 行数。默认 5000 或 10000。

terminal.replay.chunk_bytes:
  replay chunk 大小。默认 32 KiB 或 64 KiB。

client.queue.max_messages:
  单 client outbound message 队列。默认 64 或 128。

client.queue.max_bytes:
  单 client outbound byte 队列。默认 4 MiB 或 8 MiB。

client.write_timeout:
  WebSocket 写超时。默认 5s-15s。
```

核心不变量：

```text
terminal.replay.max_bytes < client.queue.max_bytes
```

并且 replay 必须是：

```text
bounded tail + chunked + truncated-not-error
```

## 6. 用户体验建议

### 6.1 Attach replay 被截断

不应显示底层 queue error。应显示：

```text
[termbridge] Showing recent terminal history only; older output was truncated.
```

或中文：

```text
[termbridge] 当前仅显示最近终端历史，较早输出已截断。
```

该场景不应断开连接。

### 6.2 Live output 过快或浏览器消费过慢

可以显示：

```text
[termbridge] Terminal output was too large or too fast for this browser connection. Reconnect to continue.
```

或中文：

```text
[termbridge] 当前浏览器连接消费终端输出过慢，已断开以保护会话。重新连接可继续查看最新输出。
```

该场景可以 detach 当前 client，但不能影响 runtime、history writer 或其他 client。

## 7. 建议实施优先级

### P0：避免长会话 attach 失败

1. 增加 attach replay 上限，默认 1-2 MiB。
2. replay 只读取 history tail。
3. replay 超预算时发送 `truncated=true`，继续 live stream。
4. 禁止因 history 过大返回 `client queue full` 导致 attach 失败。

### P1：Replay chunked

1. 将 replay history 按 32 KiB / 64 KiB chunk 入队。
2. 每个 chunk 都受 queue byte cap 约束。
3. replay 过程中如果 client 已断开，及时停止 replay。

### P1：修正 queued bytes 生命周期

1. 不要让调用方裸读 `Outbound()` 后自行决定是否 `MarkSent`。
2. 将 ack/release 语义纳入 `TerminalStream` interface，或者把 WebSocket writer loop 封装到 terminal client 内部。
3. 确保 WebSocket 写成功或失败后，queued bytes 都能按设计释放或关闭 client。

### P2：慢客户端可诊断化

1. queue full 日志必须包含：session id、client id、queue size、queued bytes、limit、reason。
2. 区分 attach replay truncation 与 live client queue full。
3. 前端展示稳定、用户可理解的 message，不直接暴露裸内部错误。

### P2：Live output coalescing

1. 连续 binary chunks 可以合并到 64 KiB / 256 KiB。
2. control message 与 binary output 顺序必须保持。
3. error/close priority path 要避免破坏用户可见顺序。

## 8. 验收标准

长会话、大输出场景下应满足：

1. 一个 session 输出超过 `history.max_bytes` 后，仍可 attach。
2. attach 时只显示最近历史，并明确标记历史被截断。
3. attach 不因 history 大于 client queue byte cap 失败。
4. live output 极快时，慢浏览器 client 被 detach，但 runtime 继续运行。
5. 慢 client detach 不影响其他 client。
6. PTY read loop 与 history writer 不被 WebSocket 慢写阻塞。
7. queue byte accounting 在长时间运行后不会只增不减。
8. 用户不再看到裸内部错误：

```text
[termbridge:terminal_stream_error] client queue full
```

而是看到区分场景的用户友好提示。
