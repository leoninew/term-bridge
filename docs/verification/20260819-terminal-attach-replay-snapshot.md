# 终端 Attach 重放的快照恢复验证

最后修改时间: 2026-08-19 11:00:45

Review status: Accepted

## Flow mode / Stage

标准模式 / standard；验证 / Verification。

## Requirement alignment

需求依据：[终端 Attach 重放的快照恢复体验](../requirement/20260819-terminal-attach-replay-snapshot.md)，Review status: Accepted。

- `replay_started` 现在启动 xterm 的替换式恢复，不会将 history tail 追加到复用 pane 的旧 buffer。
- 重放默认预算统一为 `65536` bytes，chunk 仍为 `65536` bytes；显式 `terminal.replay.max_bytes` 配置不受影响。
- Agent 以 `outputMu` 串行 `history.Write + live fan-out` 与 attach replay；client 仅在 replay 帧完整入队后对 live fan-out 可见。
- 客户端丢弃未写入的旧数据，等待 in-flight write 回调后 reset，再按既有接收顺序写入 replay 和后续 live output。
- `replay_finished.truncated=true` 会显示“当前仅显示最近终端输出”的中英文可见提示。

未实现精确 TUI 屏幕快照、seq/ack 或断点续传，符合需求的非目标。

## Plan alignment

计划依据：[终端 Attach 重放的快照恢复计划](../plan/20260819-terminal-attach-replay-snapshot.md)，Review status: Accepted。

| 计划项 | 验证结果 |
| --- | --- |
| 默认配置收敛为 64 KiB | 已完成：yaml、env 示例、集中配置、Registry fallback 和配置测试一致。 |
| replay/live 快照边界 | 已完成：`outputMu` 覆盖 replay 入队和 PTY output 的 history/fan-out，client 延迟公开注册。 |
| 前端替换式恢复 | 已完成：`TerminalView` 在 `replay_started` 调用 `beginReplayRestore()`，xterm 写队列具备 in-flight drain 与 reset 边界。 |
| 截断与 stale socket 防护 | 已完成：截断提示移出 console-only 路径，WebSocket 使用连接世代忽略旧事件。 |
| 最小回归测试 | 已完成：覆盖默认值、replay/live 顺序、replay 入队失败清理、异步 xterm write 和旧 socket 事件。 |

## Actual diff summary

- Go 终端 runtime 增加有序 replay/live 边界和 attach 失败回滚；默认 replay 上限从 `256 KiB` 变为 `64 KiB`。
- 前端将 replay 定义为 buffer 替换，避免 frozen pane 重连后重复堆叠旧 scrollback，并展示 history 截断状态。
- 新增 Go 和 Vitest 回归测试，保持已有 bounded chunk、队列背压与控制帧语义。

## Expected vs actual changed files

| 范围 | 实际文件 | 结果 |
| --- | --- | --- |
| 配置与配置测试 | `.env.example`、`configs/config.yaml`、`internal/shared/infrastructure/config/config.go`、`config_test.go` | 符合计划。 |
| Agent runtime 与测试 | `internal/agent/application/task/terminal/{runtime,registry,registry_test}.go` | 符合计划。 |
| 前端恢复与 socket | `TerminalView.vue`、`useXterm.ts`、`useTerminalSocket.ts`、对应测试、`i18n.ts` | 符合计划。 |
| 过程文档 | requirement、plan、本 verification 文档 | 符合 SpecFlow 标准模式。 |
| 无关暂存内容 | 开发与打包 scripts 变更 | 不属于本任务，未纳入本次验收或建议提交。 |

## Acceptance checklist

- [x] reattach 的 replay 替换旧 xterm 输出。
- [x] in-flight 与 queued 的旧 xterm write 不会在 reset 后污染新画面。
- [x] outbound 顺序保持 `started -> replay_started -> replay -> replay_finished -> live`。
- [x] history 中的 PTY chunk 位于 replay 或后续 live stream 的其中之一；client 在 replay 完整入队后才接收 live fan-out。
- [x] 未断开的 hot WS 与纯 resize 没有新增 reset/replay 触发路径。
- [x] 新建、复用和重连 xterm 共享 `replay_started` 恢复语义；旧 socket 的迟到事件被隔离。
- [x] 截断 replay 维持 bounded tail，并提供可见反馈。
- [x] 已添加前端和 Go 回归测试。

## Test results

| 检查 | 结果 |
| --- | --- |
| `task check` | 通过。用户在进入本阶段时确认已运行且无问题。 |
| `task test` | 通过：Vitest 31 files / 167 tests；`go test ./cmd/... ./internal/...` 通过。 |
| `git -c core.whitespace=cr-at-eol diff --cached --check` | 通过，无 whitespace 报告。 |

## Risks and incomplete items

- 未启动开发服务器，未进行浏览器人工冒烟：cold pane 在 30 秒内切回、replay 期间持续输出、hot tab/resize、超过 64 KiB 的截断提示仍需在 `/sessions` 手工确认。
- 未单独执行 race detector：当前 Windows 环境的默认 `CGO_ENABLED=0`，启用 CGO 的尝试缺少 `gcc`。全量普通测试已通过。
- 未新增 Cloud relay 专项测试；协议帧类型与 relay 路径未改变，仍应在实际 Cloud 路径进行一次冒烟确认。
- 暂存区含两项无关 scripts 变更，提交时必须与本任务拆分。

## Conclusion

代码与已接受的 Requirement、Plan 对齐，自动化检查通过，未发现本任务范围内的行为回退。用户接受本验收结论；上述手工冒烟项保留为后续发布前检查。

## User review notes

- 2026-08-19：用户明确进入 Verification，并确认 `task check` 已运行且无问题。
- 2026-08-19：用户采纳终端修复提交建议，接受本 Verification 结论；Verification 标记为 Accepted。
