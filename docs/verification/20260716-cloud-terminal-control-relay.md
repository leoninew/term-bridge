# 云端终端生命周期控制转发验证

最后修改时间: 2026-07-16 19:53:57

Review status: Accepted

## Requirement alignment

按 `docs/requirement/20260716-cloud-terminal-control-relay.md` 核对：本次变更只修复 cloud terminal relay 丢失 `started` 控制帧的问题。

- 新增 tunnel `terminal_control` typed payload，保留二进制输出与服务端控制消息的类型边界。
- agent 将 `OutboundText` 原样映射为 `TerminalControl`，不再将其降级为只包含 `message` 字段的二进制数据。
- cloud relay 将 `TerminalControl` 编码为浏览器 WebSocket 文本控制帧。
- 既有前端 `TerminalView` 保持以 `started` 控制帧关闭接入遮罩的行为；本次没有以二进制输出、超时或 xterm 状态推断就绪。
- Tunnel protocol version 从 3 提升到 4，避免旧 agent/cloud 组合静默缺失该 payload。

## Spec alignment

不适用。light / 轻量模式根据 Requirement 核对。

## Plan alignment

不适用。light / 轻量模式根据 Requirement 核对。

## Actual diff summary

| 范围 | 实际改动 |
| --- | --- |
| Tunnel contract | `proto/termbridge/shared/v1/tunnel.proto` 导入 terminal proto 并新增 `terminal_control = 305`；重新生成 Go 和 TypeScript protobuf 文件。 |
| Compatibility | `internal/shared/dto/protocol/tunnel/frame.go` 将严格握手协议版本提升到 4。 |
| Agent mapping | `internal/agent/application/user/cloud_client.go` 按 `OutboundKind` 映射 binary 到 `TerminalOutput`、text 到 `TerminalControl`，拒绝 nil control。 |
| Cloud relay | `internal/cloud/api/handler/route.go` 用既有 `writeTerminalControl` 将 `TerminalControl` 转发为浏览器文本帧。 |
| Test coverage | runtime 测试覆盖 replay 顺序；agent 测试覆盖类型映射；cloud WebSocket 集成测试覆盖 `started` 文本帧先于二进制输出且输入仍能转发。 |
| Process record | 新增 `docs/requirement/20260716-cloud-terminal-control-relay.md`。 |

## Expected vs actual changed files

| 预期范围 | 实际情况 |
| --- | --- |
| tunnel schema、版本与生成代码 | 已修改：`proto/termbridge/shared/v1/tunnel.proto`、`internal/shared/dto/protocol/tunnel/frame.go`、`internal/gen/proto/termbridge/shared/v1/tunnel.pb.go`、`web/src/gen/proto/termbridge/shared/v1/tunnel.ts`。 |
| agent/cloud relay | 已修改：`internal/agent/application/user/cloud_client.go`、`internal/cloud/api/handler/route.go`。 |
| 对应业务语义测试 | 已修改：`internal/agent/application/task/terminal/registry_test.go`、`internal/agent/application/user/cloud_client_test.go`、`internal/cloud/api/handler/terminal_test.go`。 |
| 浏览器 socket 生命周期 | 未修改，符合收缩后的范围。 |

## Acceptance checklist

- [x] cloud tunnel 用独立 typed payload 承载 `ServerControlMessage`。
- [x] `started` 的 session、workspace、lifecycle 和 attachment 字段经 cloud relay 原样到达浏览器文本帧。
- [x] cloud WebSocket 集成测试断言 `started` 文本帧先于终端二进制输出，且输入仍能到达 agent。
- [x] 运行时 replay 测试断言 `started → replay_started → binary chunks → replay_finished`。
- [x] 二进制输出与控制消息的映射测试互斥。
- [x] 协议版本升级阻止新旧 peer 静默混用。

## Test results

| Command | Result |
| --- | --- |
| `task proto` | Passed；Go/TypeScript protobuf 重新生成成功。 |
| `go test -count=1 ./internal/agent/application/task/terminal ./internal/agent/application/user ./internal/cloud/api/handler` | Passed。 |
| `go test -count=1 ./...` | Passed。 |
| `yarn --cwd web test` | Passed：28 test files、135 tests。 |
| `yarn --cwd web typecheck` | Passed。 |
| `yarn --cwd web lint` | Passed。 |
| `git diff --check` | Passed。 |
| `yarn --cwd web format` | Failed，报告 5 个未涉及文件的既有格式问题：`src/components/terminal/diagnostics.ts`、`src/components/workspace/WorkspaceEditorTabs.test.ts`、`src/components/workspace/WorkspaceEditorTabs.vue`、`src/components/workspace/WorkspaceFileWorkbench.test.ts`、`src/components/workspace/WorkspaceFileWorkbench.vue`。本次未格式化或修改这些文件。 |

## Missed or expanded scope

- 没有实现前端 WebSocket 的异常关闭提示、`error`/`exited` 后关闭去重、旧连接事件隔离或自动重连；这些是独立健壮性工作，按确认后的范围未纳入本次修复。
- 未执行真实部署环境中的人工浏览器验收；cloud WebSocket 集成测试已覆盖 agent → cloud → browser 的协议帧顺序、内容和输入回传。

## Risks

- Cloud 与 agent 必须协同升级到 tunnel protocol v4。短暂混合版本会被握手拒绝，需按同版本发布安排部署。
- 真实浏览器界面仍需部署后人工确认控制台出现 `socket.control.received { type: 'started' }`、`socket.control.started`，并确认遮罩关闭；该项不影响已覆盖的 relay 协议契约。

## Incomplete items

- 不适用；范围内代码、生成契约与自动化验证均已完成。

## Conclusion

验证通过。自动化证据覆盖了导致“正在接入会话…”永久显示的完整 cloud relay 根因链：runtime 将 `started` 排在 replay 前、agent 将其作为 typed control 映射、cloud 以浏览器 WebSocket 文本帧转发，随后才转发二进制终端输出。前端现有 `TerminalView` 因而能够收到所需的 `started` 事件并关闭遮罩。
