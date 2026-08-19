# 终端 Attach 重放的快照恢复计划

最后修改时间: 2026-08-19 10:04:20

Review status: Accepted

## Flow mode / Stage

标准模式 / standard；计划 / Plan。

## Requirement basis

- 需求依据：[docs/requirement/20260819-terminal-attach-replay-snapshot.md](../requirement/20260819-terminal-attach-replay-snapshot.md)，Review status: Accepted。
- 用户已明确：不在本任务追求完整、精确的终端屏幕快照；先将默认 replay 上限从 256 KiB 降至 64 KiB。
- 本任务仍须消除复用 xterm 后把 history tail 追加到旧画面的根因，并建立 replay 与 live output 的有序边界。减小预算只能缓解体积，不能替代根治。

## Confirmed configuration

| 配置层 | 当前值 | 计划值 | 原因 |
| --- | ---: | ---: | --- |
| `configs/config.yaml` `terminal.replay.max_bytes` | 262144 | 65536 | 正常部署的基线配置 |
| `config.DefaultTerminalReplayMaxBytes` | `256 * 1024` | `64 * 1024` | 缺省/非法值归一化的权威后备 |
| `terminal.DefaultReplayMaxBytes` | `256 * 1024` | `64 * 1024` | Registry 被直接构造时的运行时后备，必须与集中配置一致 |
| `terminal.replay.chunk_bytes` | 65536 | 65536 | 等于新 replay 上限，默认只发送一个 binary chunk |
| `terminal.client.queue.max_bytes` | 4194304 | 不变 | 新上限仍远小于队列 4 MiB，满足既有校验 |

现有 Viper 配置键、环境变量映射和校验均已支持 `terminal.replay.max_bytes`。本次不新增配置字段或兼容别名；显式设置的值继续生效，只改变未覆盖配置时的默认值。

## Design

### 1. 同步默认配置

修改 `internal/shared/infrastructure/config/config.go`、`configs/config.yaml` 和对应配置测试，将默认 replay max bytes 统一为 `64 * 1024` / `65536`。同时修改 `internal/agent/application/task/terminal/registry.go` 的本地 fallback，防止单元测试、嵌入调用或未来漏传配置的路径恢复成 256 KiB。

保持 `chunk_bytes=65536`：默认 history tail 只会占一个 chunk；若部署方把 replay 上限调大，仍会按 64 KiB 正常分块。现有约束 `chunk_bytes <= replay.max_bytes < client.queue.max_bytes` 不变。

### 2. 在 Agent 建立 replay/live 快照边界

为 `SessionRuntime` 增加专门保护终端输出时间线的同步边界（名称以实现为准，例如 `outputMu`）。它只保护：

1. `readLoop` 对单个 PTY chunk 的 `history.Write` 和向已注册 clients 的 live fan-out；
2. attach 的 history flush、bounded tail 读取、replay control/chunk 入队，以及新 client 向 live fan-out 集合的可见性提交。

attach 的 client 将先完成控制权角色预留，但在 replay 的 `started -> replay_started -> binary chunks -> replay_finished` 全部非阻塞入队前，不能对 `publishBinary` 可见。提交到 `r.clients` 后，后续 PTY chunk 才会进入该 client 的队列。这样某一 chunk 只能在 replay tail 或之后的 live stream 中出现一次，浏览器可依赖顺序恢复。

实现时遵守以下边界：

- 不在锁内进行网络写入或等待 client queue；现有 `enqueue`/`canEnqueue` 为非阻塞操作。
- bounded tail 最多读取配置上限（新默认 64 KiB）；短暂阻塞 PTY read loop 仅覆盖本地 flush/read/内存入队，不扩大为无界读取或网络等待。
- attach replay 任何步骤失败时，撤销 controller/client 的预留、关闭队列并恢复一致的 attachment 状态；不能留下不可见 controller 或泄漏 queue。
- 明确并测试锁顺序，避免与 `controlMu`、`resizeMu`、`mu` 形成死锁；控制权选举、detach 和 concurrent attach 继续保持单 controller 语义。
- Cloud relay 不改变帧类型或协议，必须按 Agent outbound 的既有先后顺序转发控制帧和二进制帧。

### 3. 前端将 replay 作为替换式恢复

在 `web/src/components/terminal/useXterm.ts` 为 live xterm 增加受控的“开始 replay 恢复”能力，由 `TerminalView` 在收到当前 WebSocket 的 `replay_started` 后调用。

该能力必须处理 xterm 的异步写入：

1. 标记新的 output epoch，丢弃尚未送入 xterm 的旧 epoch write queue；
2. 若旧 epoch 有正在执行的 `terminal.write`，等待该回调完成；
3. 仅在旧 write 不会再回写后清空终端 buffer/scrollback；
4. 将已收到的 replay bytes 和其后的有序 live bytes 写入新 epoch；
5. 保留现有 pending-byte 上限、drain 逻辑、scroll edge 计算、fit 和 theme 行为。

实现不得简单在控制帧回调中直接调用裸 `clear()`/`reset()`：旧的异步 write callback 仍可能随后写回。清空发生前到达的新 epoch 数据应被保留而不提前解析；清空后按 WebSocket 接收顺序 drain。

`TerminalView` 只在真正收到 attach 的 `replay_started` 时启动替换恢复。连接持续存在的 hot tab 切换、`ResizeObserver` 和 `fit` 不触发该逻辑。首次新建 xterm 时运行同一逻辑是无害的，且能统一 cold reconnect 与初始 attach 的语义。

为防止快速 reconnect 中旧 socket 的迟到事件改变新连接状态，审查并在必要时为 `useTerminalSocket` 增加连接世代校验：只允许当前 socket 触发 output/control/status 回调。

### 4. 恢复状态与截断反馈

保留现有 replay loading 状态，但其含义改为“正在以近期历史替换旧画面”。当 `replay_finished.truncated=true` 时，在终端外的现有消息区域显示当前仅恢复最近输出的反馈；不把说明文字写入终端数据流，也不继续仅依赖 console warning。

该提示在下一次成功 replay 开始时重置。若产品实现采用常驻提示，文案必须说明“当前画面仅为最近输出”，不能暗示这是完整的 TUI 状态快照。

## Implementation steps

1. 更新 Requirement 的最终配置决策，并修改 Go/yaml 的三处 replay 默认值及 `config_test.go` 的默认配置断言。
2. 重构 `SessionRuntime.attach` 与 `readLoop` 的 client 注册和输出同步，封装 snapshot/replay enqueue 与 rollback，维持现有控制帧与队列背压行为。
3. 在 `useXterm` 实现 epoch-aware replay replacement，扩展 `TerminalView` 的 replay 控制处理并加入截断的可见反馈；按需加 socket generation guard。
4. 为 Agent replay/live 边界、attach 清理、前端旧 write queue replacement、socket stale-event 防护和保留的 chunk/truncated 行为添加最小有效回归测试。
5. 运行前端与 Go 的项目固定检查，并手工核对 `/sessions` 的 cold reconnect、hot 切换、窗口 resize 与持续输出 attach。

## Files to change

| 文件 | 计划变更 |
| --- | --- |
| `configs/config.yaml` | 默认 replay 上限改为 65536 |
| `internal/shared/infrastructure/config/config.go` | 集中默认值改为 64 KiB |
| `internal/shared/infrastructure/config/config_test.go` | 默认配置断言更新为 65536 |
| `internal/agent/application/task/terminal/registry.go` | Registry replay fallback 对齐 64 KiB |
| `internal/agent/application/task/terminal/runtime.go` | replay 快照与 live 输出的有序注册边界 |
| `internal/agent/application/task/terminal/*_test.go` | replay/live 并发、队列、failure cleanup 回归测试 |
| `web/src/components/terminal/useXterm.ts` | epoch-aware 的 buffer 替换和 write queue 收敛 |
| `web/src/components/terminal/TerminalView.vue` | replay restore 生命周期与截断反馈 |
| `web/src/features/sessions/useTerminalSocket.ts` | 必要时增加当前连接世代防护 |
| `web/src/**/**.test.ts` | 前端 lifecycle/socket 回归测试 |
| `web/src/i18n.ts` | 截断恢复的中英文案（若缺现有可复用文案） |

## Verification plan

### Automated

1. Go 配置测试：加载基线配置后 replay max bytes 为 65536，chunk 为 65536；显式配置和既有合法性校验不回退。
2. Go runtime 测试：持续 PTY 输出与 attach 并发时，client 收到完整、有序且不重复的 `started -> replay_started -> replay -> replay_finished -> live`；覆盖 replay enqueue 失败时不残留 client/controller。
3. Go runtime 测试：保留 bounded tail、64 KiB chunk 分割、`truncated` 和 client queue protection 的现有承诺。
4. 前端测试：旧 epoch 的 queued/in-flight output 不会在 replay replace 后重新出现；新 replay/live output 仍按接收顺序写入。
5. 前端测试：旧 socket 的迟到 close/message（若 generation guard 被引入）不会改变当前连接；resize/hot switch 不调用 replay replacement。
6. 固定检查：`yarn --cwd web lint:fix`、`yarn --cwd web typecheck`、`task check`、`go test ./cmd/... ./internal/...`。

### Manual

1. 在 `/sessions` 打开一个持续输出的 running session，使其离开 hot set 后在 30 秒内切回；确认旧输出不会接在 replay 后重复出现。
2. 在 replay 中持续产生输出；确认最后画面只含近期 history tail 和后续 live output，未出现空白卡死或乱序明显拼接。
3. 在两个持续连接的 hot tabs 间切换，再改变窗口/侧栏尺寸；确认不清屏、不重新 attach、不 replay。
4. 令 history 超过 64 KiB 后 reconnect；确认只恢复最近内容，并显示截断反馈。

## Risks and mitigations

| 风险 | 缓解 |
| --- | --- |
| 64 KiB 让可恢复上下文更短 | 这是已确认的默认体验取舍；运维仍可通过既有配置显式调大 |
| xterm 异步 write 在 reset 后污染新画面 | 通过 epoch、in-flight drain 和前端回归测试保证旧 write 先完成再清空 |
| output lock 引入 read loop 延迟或锁死 | 锁内只做有界本地 I/O 与非阻塞 enqueue；定义锁顺序并覆盖 concurrent attach/detach |
| replay tail 起点位于 ANSI/UTF-8 中间 | 不承诺精确 TUI snapshot；维持现有 raw-tail 容错，禁止 panic/attach 失败 |
| Cloud relay 改变顺序 | 保持既有控制/二进制协议，补充 relay 路径验证 |

## Rollback

若 64 KiB 对特定部署的上下文不足，可通过已有 `terminal.replay.max_bytes` 配置覆盖回更大值，无需代码兼容层。若有序恢复实现出现回归，回滚该功能变更必须连同前端替换语义和 Agent 快照边界一起回退，不能只回滚一侧。

## Open questions

暂无需要用户进一步决定的事项。计划采用：64 KiB 默认值、64 KiB chunk、替换式前端恢复，以及服务端 snapshot/live 有序边界。

## User review notes

- 2026-08-19：用户明确暂不要求完整终端屏幕快照，并选择将默认 replay 从 256 KiB 降为 64 KiB。
- 2026-08-19：用户要求开始 Plan；本计划基于已接受的 Requirement，未进入产品代码实施。
- 2026-08-19：用户确认本计划；Plan 标记为 Accepted，等待明确进入 Implementation。
