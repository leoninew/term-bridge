# Web Terminal backpressure/replay 长会话治理计划
最后修改时间: 2026-07-08 08:39:08

Review status: Accepted

Flow mode: standard / 标准模式
Stage: Plan / 计划

## Requirement basis

依据 `docs/requirement/20260708-web-terminal-backpressure-replay.md`，当前代码已确认存在以下问题：

1. `history.max_bytes` 默认 5 MiB，而 `DefaultClientQueueBytes` 默认 4 MiB。
2. attach replay 读取完整 history 并作为单个 binary outbound 入队，history 较大时 attach 可因 `client queue full` 失败。
3. replay 当前不支持 bounded tail、chunked replay 或准确 truncated 标记。
4. 生产写出路径未调用 `Client.MarkSent(outbound)`，导致 queued bytes 成功写出后不释放。
5. 用户可见错误仍可能暴露裸内部错误 `[termbridge:terminal_stream_error] client queue full`。

## Implementation steps

### 1. 引入 terminal replay/client queue 配置模型

1. 在配置层增加独立 replay/client queue 配置，避免把 history capacity 与 replay capacity 混用。
2. 推荐结构：

```yaml
terminal:
  replay:
    max_bytes: 1048576
    chunk_bytes: 65536
  client:
    queue:
      max_messages: 64
      max_bytes: 4194304
```

3. 在 `internal/shared/infrastructure/config` 中增加 typed config、环境变量绑定、默认值归一化和校验。
4. 校验不变量：`terminal.replay.max_bytes < terminal.client.queue.max_bytes`，`chunk_bytes > 0` 且不大于 replay max bytes。
5. 将 agent bootstrap 中 `terminalapp.NewRegistry` 的 `ClientQueueSize` / `ClientQueueBytes` 和 replay 配置从 config 传入。

### 2. 扩展 terminal registry/runtime replay 配置

1. 在 `internal/agent/application/task/terminal.Config` 增加 replay 配置字段，例如：
   - `ReplayMaxBytes int64`
   - `ReplayChunkBytes int`
2. 在 `Registry` 保存规范化后的 replay 参数，并提供默认值。
3. 保留现有 client queue 默认值，避免未显式配置时行为不可用。

### 3. 将 attach replay 改为 bounded tail + chunked

1. 替换 `SessionRuntime.enqueueReplay()` 中的 `os.ReadFile(historyPath)` 完整读取/完整入队。
2. 实现只读取 history tail 的 helper，优先避免把完整 history 读入内存：
   - `os.Stat` 获取文件大小；
   - 若文件大小大于 `ReplayMaxBytes`，从 `size - ReplayMaxBytes` seek 后读取；
   - 标记 `truncated=true`；
   - 若文件不存在，按空 history 处理。
3. 将 replay data 按 `ReplayChunkBytes` 分片 enqueue。
4. `started`、`replay_started`、binary chunks、`replay_finished` 的顺序必须稳定。
5. replay 由于 queue full 时不应把 history 太大转换成 attach 失败；默认策略为停止 replay、发送 `replay_finished.truncated=true`，然后进入 live stream。
6. 如果 control message 本身无法 enqueue，仍可按 attach 失败处理，因为这代表 client 已不可用或 queue 配置异常。

### 4. 修正 queued bytes 生命周期

1. 不再依赖调用方“记得”裸调用 `MarkSent`。
2. 优先采用最小侵入方案：扩展 `TerminalStream` interface，增加 `MarkSent(outbound terminalapp.Outbound)` 或 `Release(outbound)` 方法，并在所有生产消费者中强制调用。
3. 本地 direct WebSocket 路径：`internal/agent/api/handler/runtime_endpoint.go`
   - binary/text 写成功后调用 release；
   - 写失败返回前也调用 release 或 detach，避免 bytes 悬挂。
4. Cloud tunnel 路径：`internal/agent/application/user/cloud_client.go`
   - `writeTunnelFrame` 成功或失败后 release；
   - 若写失败需要停止 stream 或 detach，避免继续消费失效连接。
5. 测试 fake consumer 也按新接口更新，避免只在测试中释放而生产遗漏。

### 5. 区分 replay truncation 与 live queue full

1. attach replay 截断：通过 `replay_finished.truncated=true` 表达，不发送 terminal error，不关闭 stream。
2. live output queue full：保留 detach 当前 client 的策略。
3. 在 `publishBinary` / broadcast 队列失败路径增加结构化日志字段：session id、client id、queued bytes、queue byte limit、queue message limit、reason。
4. reason 建议区分：
   - `replay_truncated`
   - `client_queue_full`
   - `client_write_failed`
   - `browser_detached`

### 6. 前端用户提示

1. 在 `web/src/components/terminal/TerminalView.vue` 处理 `replay_finished.truncated === true`。
2. 展示用户友好提示：

```text
[termbridge] 当前仅显示最近终端历史，较早输出已截断。
```

3. 对 `terminal_stream_error/client queue full` 的 live 错误路径，可映射为更可理解的文案：

```text
[termbridge] 当前浏览器连接消费终端输出过慢，已断开以保护会话。重新连接可继续查看最新输出。
```

4. 保留 `safeTerminalText`，避免服务端错误消息中的控制字符污染 terminal。

### 7. 测试覆盖

1. Go runtime/registry 测试：
   - history 大于 client queue byte cap 时 attach 仍成功；
   - replay 输出被 chunk；
   - `replay_finished.truncated=true`；
   - replay chunk 顺序在 started/replay_started/replay_finished 之间稳定；
   - live output queue full detach 当前 client，不影响 runtime read loop 和其他 client；
   - queued bytes 在 consumer release 后下降。
2. Handler/client 测试：
   - direct WebSocket 写出路径 release outbound；
   - Cloud tunnel 写出路径 release outbound。
3. Config 测试：
   - terminal replay/client queue 默认值；
   - 环境变量覆盖；
   - `replay.max_bytes >= client.queue.max_bytes` 被配置校验拒绝。
4. 前端测试：
   - `replay_finished.truncated=true` 时展示截断提示；
   - `terminal_stream_error/client queue full` 不再直接展示裸 `client queue full`。

## Files to change

预期主要修改：

1. `configs/config.yaml`
2. `configs/config.development.yaml` / `configs/config.production.yaml`（如需要显式覆盖）
3. `internal/shared/infrastructure/config/config.go`
4. `internal/shared/infrastructure/config/config_test.go`
5. `internal/agent/application/bootstrap/config.go`
6. `internal/agent/application/bootstrap/server.go`
7. `cmd/termbridge/app/app.go`
8. `internal/agent/application/task/terminal/registry.go`
9. `internal/agent/application/task/terminal/runtime.go`
10. `internal/agent/application/task/terminal/registry_test.go`
11. `internal/agent/application/user/runtime_access.go`
12. `internal/agent/api/handler/runtime_endpoint.go`
13. `internal/agent/application/user/cloud_client.go`
14. `web/src/components/terminal/TerminalView.vue`
15. 相关前端测试文件，如已有 socket/protocol/component test 可复用则优先复用。

预期不修改：

1. 不修改 proto 字段；`ServerControlMessage.truncated` 已存在。
2. 不修改 Git 历史，不执行提交或推送。
3. 不引入完整 history viewer 或 seq replay API。

## Verification plan

1. 运行相关 Go 单测，至少覆盖 terminal runtime/registry/config/API handler/cloud client 相关包。
2. 运行前端相关测试，至少覆盖 terminal protocol/socket/component 行为。
3. 如改动配置解析，运行 `go test ./internal/shared/infrastructure/config ./internal/agent/application/task/terminal ...` 等针对性测试。
4. 如改动前端展示，运行 `yarn test` 或针对性 vitest 命令；必要时运行 `yarn typecheck`。
5. 手工或自动构造一个 history 文件大于 client queue byte cap 的 session attach 场景，观察：
   - attach 成功；
   - replay_finished truncated 为 true；
   - live stream 继续；
   - 用户不见裸 `client queue full`。

## Assumptions

1. 当前 proto 中已有 `ServerControlMessage.truncated` 字段，本轮无需 proto 变更。
2. replay byte-tail 截断可接受；行数上限可作为后续增强，除非实现中低成本可一起完成。
3. 直接在 TerminalStream interface 上增加 release/MarkSent 方法是最小侵入方案；如实现中发现循环依赖或接口污染严重，再改为 outbound wrapper。
4. 前端提示写入 terminal 内容是可接受的短期方案；如后续有更完整 toast/status bar，可再迁移展示位置。

## Risks

1. byte-tail 截断可能切断 UTF-8 或 ANSI escape sequence；需要通过 chunk 边界和 xterm 行为验证是否可接受。
2. release/MarkSent 如果遗漏任一路径，queue byte accounting 仍可能失真；必须用测试覆盖 direct 和 tunnel 两条路径。
3. replay 阶段 queue full 后继续 live stream 的策略需要避免 control message 顺序不一致。
4. 配置结构新增后，环境变量命名和缩写大小写必须符合项目约束，使用 `Url`/`Jwt` 等项目内部缩写规则。
5. 如果配置校验过严，可能影响现有开发环境；默认值应保证现有配置无需额外编辑即可启动。

## Rollback

1. 若 replay tail/chunked 实现引入不可接受的 attach 行为，可先回滚 replay helper 和配置传递，保留 queued bytes release 修复。
2. 若 TerminalStream interface 扩展影响范围过大，可回滚为在 `Outbound()` 外提供 adapter/writer 封装，保持生产路径强制 release。
3. 前端提示变更可独立回滚，不影响后端 bounded replay 的核心稳定性。

## User review notes

- 2026-07-08：根据用户要求，已确认 commit 分析文档中问题在当前代码成立，并按 standard / 标准模式起草 Plan / 计划草稿。
