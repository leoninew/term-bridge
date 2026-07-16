# 云端终端生命周期控制转发

最后修改时间: 2026-07-16 19:43:15

Review status: Accepted

## Background

云端会话的终端 WebSocket 已能展示输出并接收输入，但前端持续显示“正在接入会话…”。`TerminalView` 只在收到服务端 `started` 控制消息后将 `sessionStarted` 设为真并关闭遮罩。

本地运行时按 `started → replay_started → replay output → replay_finished` 的顺序产生事件，但云端 agent 将 `ServerControlMessage` 降级为 `TerminalOutput` 字节，仅保留 `message` 字段。`started` 的关键会话和生命周期字段因此丢失，浏览器无法收到有效的文本控制帧。

## Goal

- 在 cloud tunnel 中保留终端二进制输出与服务端生命周期控制的类型边界。
- 使 cloud relay 将完整的 `ServerControlMessage` 作为浏览器 WebSocket 文本帧转发。
- 保持前端现有的 `started` 会话就绪判定，使其通过 cloud relay 收到缺失的有效控制帧。

## Non-goal

- 不以终端二进制输出、固定超时或 xterm 渲染结果替代 `started` 就绪协议。
- 不修改本地直连终端的既有控制帧转发逻辑。
- 不引入自动重连策略、不改变终端单写入者约束，也不调整浏览器 WebSocket 的异常关闭或过期回调策略。

## User scenarios

1. 用户从会话树打开运行中的云端会话时，浏览器先收到 `started` 文本控制帧，随后接收回放/实时二进制输出，遮罩立即关闭。
2. 用户可以继续在同一终端查看输出和发送输入，二进制数据不被误解为控制协议。

## Acceptance

- cloud tunnel 对 `ServerControlMessage` 使用独立的 typed payload，并通过协议版本保护避免静默降级。
- `started` 的会话、工作区、运行状态和附加状态字段完整到达浏览器 WebSocket 文本帧，且先于二进制回放输出。
- 二进制帧只流向 xterm 输出回调，不能关闭接入遮罩。
- 覆盖 runtime、agent tunnel 映射和 cloud relay 的业务语义测试通过。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- 以完整 typed control relay 修复根因，不采用前端以二进制输出推断就绪的兼容性回退。
- 在既有严格 tunnel handshake 中提升协议版本，禁止新旧组件混用时静默缺失生命周期控制。

## Risk

- 本次协议版本提升要求云端服务和 agent 协同升级；短暂混合部署会被明确拒绝连接，而不会发生无提示的控制帧丢失。
- 终端流会新增控制 payload 分支，必须通过端到端顺序测试保证 `started` 仍在回放二进制输出之前。
- 浏览器 WebSocket 的异常关闭、重复报错和旧连接回调属于后续独立健壮性工作，不作为本次故障修复范围。
