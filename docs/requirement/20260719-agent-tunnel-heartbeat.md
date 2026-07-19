# 设备离线与 Agent 隧道心跳

最后修改时间: 2026-07-19 22:56:31

Review status: Accepted

## Mode

light / 轻量模式

## Background

近期落地 **terminal tab keep-alive（默认 4 路 hot WS）** 与 **per-user concurrent attach 配额（默认 8）** 后，观测到：设备曾可长时间 online，现在会“自己离线”。

排查结论：

1. **假离线（重连 race）**
   Cloud `handleAgentTunnel` 在连接结束时无条件 `MarkOffline`。设备重连后旧连接 defer 仍会把 registry 标成 offline，即使新 route 仍在。
   keep-alive 提高中继压力 → 隧道更容易断/重连 → race 更容易被看见。

2. **隧道 demux 头阻塞**
   Cloud 单线程读 agent tunnel，并在 demux 内同步 `browser.Write`。多 tab 输出时任一浏览器 WS 变慢会堵死整条设备隧道，引发断连与重连。

3. **无控制面心跳（本需求补齐）**
   协议已有 `Ping`/`Pong`，但双方只做被动应答，**没有主动周期 ping / idle 读超时**。
   长时间无流量时，反向代理/NAT 可能静默掐断连接；或 peer 已死而本端仍认为 online，直到 TCP 层超时。

相关既有改动（实现前已在工作区落地，本 feature 验收时一并对照）：

- `clearRoute` 仅在当前 owner 时 `MarkOffline`
- `deviceSummary.online` 以 `routeFor != nil` 为准
- browser relay 写 3s 超时，避免堵死 demux

## Goal

1. 记录上述根因与边界，避免与 “keep-alive 关 terminal WS” 或 “配额 429” 混淆。
2. 为 **Agent ↔ Cloud 设备隧道** 增加控制面心跳：
   - Agent 周期发送 `Ping`
   - Cloud 应答 `Pong`（已有路径复用）
   - 任一侧在超时无进展时主动结束连接，触发既有 connector 重连 / `MarkOffline` 路径
3. 心跳不得提升 tunnel protocol version（继续使用现有 `Ping`/`Pong` payload）。
4. 保持重连后 online 状态正确（不回退已修的 MarkOffline race）。

## Non-goal

1. 不改 browser↔server 的 terminal 子协议 ping（那是另一条 WS）。
2. 不调整 keep-alive hot set 数量或 attach 配额默认值。
3. 不引入可配置 yaml 心跳参数（本轮固定常量即可；后续需要再开配置项）。
4. 不做完整异步 fan-out 中继重构（browser 写超时已止血；异步队列另案）。
5. 不改 device 绑定 / OAuth / 签名校验。

## User scenarios

### Scenario 1：空闲设备保持 online

1. Agent 已出站连上 Cloud，无终端/无 API 流量。
2. 经过数分钟（超过常见代理 idle 门槛），设备列表仍显示 online。
3. Agent 日志无连续 connector failed（允许偶发网络抖动后自动重连并恢复 online）。

### Scenario 2：对端消失后及时 offline

1. Agent 进程被强杀或网络黑洞。
2. Cloud 在心跳超时内结束隧道并 `MarkOffline`。
3. 设备列表变为 offline，而不是长期假 online。

### Scenario 3：重连不闪假 offline

1. 隧道因网络抖动断开后 connector 重连成功。
2. 旧连接清理不得把仍存活的新 route 标成 offline。
3. `GET /api/devices` 中该设备 `online=true`。

## Acceptance

1. Agent 在 hello/ack 成功后按固定间隔发送 control `Ping`。
2. Cloud 对 control `Ping` 回复同 nonce `Pong`（行为保持）。
3. Agent 若在超时窗口内未收到任何进展（至少含 `Pong` 或等价入站帧刷新），主动关闭隧道并返回错误，由既有 lifecycle 重连。
4. Cloud 读循环带 idle 超时：超时内无任何入站帧则关闭连接并走现有 offline 清理（仅当前 owner）。
5. 不升高 `tunnel.ProtocolVersion`。
6. 既有 tunnel/terminal 测试通过；新增覆盖：重连 online、心跳/超时相关单测或集成测至少一项。
7. 需求文档记录 keep-alive/配额与本问题的因果关系与非目标边界。

## Open questions

暂无必须阻塞实现的未决问题。

## Decisions

1. 流程模式：**light / 轻量**（范围明确的稳定性 bugfix + 小增强）。
2. 心跳由 **Agent 主动 Ping** + **Cloud idle Read 超时** 双侧保障。
3. 默认常量（实现固定，不配 yaml）：
   - interval ≈ 25s
   - timeout ≈ 75s（约 3 个 interval）
4. 入站任意合法帧都可刷新 Cloud 侧 idle（不仅 Pong），避免业务繁忙时误杀。
5. Agent 侧以 “最近收到 Pong 或任意入站帧” 刷新存活；发送 Ping 失败或超时则断连。
6. 用户要求本轮 “记录问题，然后补心跳”：Requirement 直接 `Accepted` 后进入 Implementation。

## Risk

1. 超时过短：弱网误重连抖动 → 选 75s 保守窗口。
2. 超时过长：假 online 窗口仍存在 → 可接受的折中。
3. 心跳写与业务写共用 `writeMu`：极端阻塞时 Ping 也发不出 → 依赖对端 idle 超时兜底。
4. 测试中 httptest 长连接超时：单测用可注入 interval/timeout 或缩短常量仅测试包。

## Related code

- `internal/agent/application/user/cloud_client.go`
- `internal/cloud/api/handler/server.go` (`handleAgentTunnel`)
- `internal/cloud/api/handler/route.go` (browser relay timeout)
- `internal/shared/dto/protocol/tunnel/`
- `proto/termbridge/shared/v1/tunnel.proto` (`Ping`/`Pong`)
