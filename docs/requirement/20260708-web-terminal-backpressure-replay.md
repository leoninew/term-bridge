# Web Terminal backpressure/replay 长会话治理需求
最后修改时间: 2026-07-08 08:30:17

Review status: Accepted

Flow mode: standard / 标准模式
Stage: Requirement / 需求

## Background

commit `9a1d1f0eefd8722b33f810c979b520eff95c5fe7` 中的分析文档指出，Web Terminal 在长会话或大量输出后可能出现：

```text
[termbridge:terminal_stream_error] client queue full
```

本轮先对当前代码进行只读核对，确认该问题仍然存在，并且当前代码路径已从分析文档中的旧路径迁移到新的 `internal/agent/application/task/terminal`、`internal/agent/api/handler` 与 `internal/agent/application/user` 分层。

已确认的当前问题证据：

1. 默认 `history.max_bytes` 为 `5242880`，即 5 MiB；见 `configs/config.yaml`。
2. 默认 `DefaultClientQueueBytes` 为 `4 * 1024 * 1024`，即 4 MiB；见 `internal/agent/application/task/terminal/registry.go`。
3. attach replay 当前在 `enqueueReplay()` 中 `os.ReadFile(historyPath)` 读取完整 history，并作为单个 `OutboundBinary` enqueue；如果超过 client queue byte cap，会返回 `client queue full`；见 `internal/agent/application/task/terminal/runtime.go`。
4. replay 完成消息当前固定 `truncated := false`，没有表达 history 被截断或只回放 tail 的语义；见 `internal/agent/application/task/terminal/runtime.go`。
5. `Client.MarkSent(outbound)` 虽然存在，但当前生产写出路径 `internal/agent/api/handler/runtime_endpoint.go` 与 `internal/agent/application/user/cloud_client.go` 裸读 `stream.Outbound()` 后没有调用 MarkSent，导致 `queuedBytes` 在成功写出后也不会释放，长时间输出后更容易永久性触发 queue full。
6. 前端 `TerminalView.vue` 对 terminal control error 仍会直接写入 `[termbridge:${code}] ${message}`，用户会看到裸内部错误。

因此，文档提到的 backpressure/replay 问题在当前代码中成立，并且 queued bytes 释放路径比原分析文档更需要作为 P0/P1 范围处理。

## Goal

1. 让 running session 在 history 较大或超过 client queue byte cap 时仍能 attach 成功。
2. 将 attach replay 从“完整 history 一次性入队”改为“有预算的 tail replay + chunked replay + truncated 语义”。
3. 区分 history 持久化容量、attach replay 容量和 client queue 积压容量，避免三者互相误用。
4. 修正 outbound queue byte accounting 生命周期，确保成功写出、写出失败或 detach 后 queued bytes 不会只增不减。
5. 区分 attach replay truncation 与 live output slow-client queue full，避免把 history 太长当成 terminal stream fatal error。
6. 改善用户可见错误：attach replay 被截断时给出可理解提示；live slow client 被断开时给出可恢复说明，避免裸露 `client queue full`。
7. 为上述行为补充 Go 测试和必要的前端行为测试。

## Non-goal

1. 不实现完整 seq replay、ack replay 或跨浏览器可靠续传协议。
2. 不实现 paginated history viewer、历史下载页或完整日志检索产品能力。
3. 不重写 PTY runtime、session store 或 tunnel 架构。
4. 不通过无限扩大 queue、无限缓存或阻塞 PTY read loop 来掩盖慢客户端问题。
5. 不改变 `history.max_bytes` 的持久化语义，除非为了配置结构增加独立 replay 配置。
6. 不执行 `git add`、`git commit`、`git push` 等 Git 写操作。

## User scenarios

1. 用户在 session 中运行长时间构建、测试、日志 tail 或 AI/TUI 工具，history 接近或超过 5 MiB 后刷新页面，仍应能重新 attach。
2. 用户重新 attach running session 时，只需要看到最近上下文；较早历史可被截断，但必须有明确提示。
3. 浏览器 tab 暂停、网络变慢或 xterm 消费过慢时，只应 detach 当前慢 client，不应影响 PTY、history writer 或其他 client。
4. Agent 通过 Cloud tunnel 转发 terminal output 时，queue byte accounting 应与本地 direct WebSocket 路径一致可靠。
5. 维护者排查大输出问题时，应能从日志中区分 replay 截断、live queue full、write timeout 或 browser detach。

## Acceptance

1. 当 history 文件大于 client queue byte cap 时，`Registry.Attach()` / runtime attach 不应返回 `client queue full`；应发送 bounded tail replay 并继续 live stream。
2. attach replay 不再一次性 enqueue 完整 history；应按固定 chunk size 分段发送。
3. attach replay 需要设置准确的 `truncated` 语义：被 replay budget 截断时，`replay_finished.truncated=true`。
4. replay budget 与 chunk size 应由集中配置或 runtime config 表达，不应散落魔法数字；默认值必须满足 `terminal.replay.max_bytes < client.queue.max_bytes`。
5. live output 阶段仍保留慢客户端保护：client queue 满时 detach 当前 client，但不能阻塞 PTY read loop、history writer 或其他 clients。
6. outbound 写出路径必须在写成功或失败后释放 queued bytes；本地 direct WebSocket 与 Cloud tunnel 路径都要覆盖。
7. 用户不再在 attach replay 历史过大场景看到裸 `[termbridge:terminal_stream_error] client queue full`。
8. 前端在 `replay_finished.truncated=true` 时展示用户可理解提示，例如“当前仅显示最近终端历史，较早输出已截断”。
9. 相关 Go 测试覆盖：history 大于 queue cap 仍可 attach、replay chunked、truncated 标记、MarkSent/release 语义、live queue full detach 不影响 runtime。
10. 必要的前端测试或现有测试更新覆盖 truncated replay 提示和 terminal error 文案映射。

## Open questions

暂无必须阻塞进入 Plan / 计划阶段的问题。以下取舍可在计划中作为默认方案记录：

1. replay 默认预算建议先采用 1 MiB，chunk 默认 32 KiB 或 64 KiB。
2. replay 行数上限是否本轮引入：若实现成本过高，本轮可先按 byte tail 实现，保留 max lines 为后续增强。
3. `TerminalStream` 是否直接扩展 `MarkSent`/`Release` 方法，还是改为返回带 release callback 的 outbound wrapper；计划阶段需选择最小侵入方案。

## Decisions

1. 本任务采用 standard / 标准模式。
2. 用户要求“commit picked，确认问题后进行 spec”，视为在确认问题成立后接受 Requirement 并进入 Plan / 计划阶段。
3. 当前标准模式不单独创建 `docs/spec` 阶段文档；实现方案写入 `docs/plan/20260708-web-terminal-backpressure-replay.md`。
4. 本轮优先修复 attach 失败和 queued bytes 生命周期；不做完整 history viewer 或 seq replay。
5. 慢客户端 live output 仍应被 detach；本轮不是取消背压保护，而是让 attach replay 走截断语义。

## Risk

1. 修改 TerminalStream/Client outbound 生命周期可能影响本地 WebSocket 与 Cloud tunnel 两条路径，需要同步测试。
2. replay tail 按 byte 截断可能落在 UTF-8 或 ANSI escape sequence 中间；需要用 xterm 可接受策略或在文档中记录兼容风险。
3. 如果只修 replay 不修 MarkSent/release，长时间 live output 仍可能因为 queuedBytes 不释放而再次触发 queue full。
4. 如果只扩大 queue，不做 bounded replay，会把故障从 attach 失败变成内存积压，不符合本需求。
5. 前端提示需要避免污染 terminal 内容或破坏 TUI 画面，但必须让用户知道历史被截断。

## User review notes

- 2026-07-08：用户要求基于已 pick 的 commit，按 SpecFlow standard / 标准模式先确认文档中问题，再进行 spec/方案。问题已在当前代码中确认成立，Requirement 视为 Accepted，进入 Plan / 计划阶段。
