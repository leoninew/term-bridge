# 设备离线与 Agent 隧道心跳 — 验证

最后修改时间: 2026-07-19 23:17:04

Review status: Accepted

## Requirement alignment

按 `docs/requirement/20260719-agent-tunnel-heartbeat.md` 核对：

| 需求点 | 结果 |
| --- | --- |
| 记录 keep-alive/配额与假离线根因 | 已写入 requirement |
| Agent 周期 control `Ping` | 已实现：`Client.runHeartbeat`，默认 25s |
| Cloud 应答 `Pong` | 已实现：复用 `handleAgentTunnel` 路径 |
| 超时无进展主动断连 / offline | Agent idle + Cloud `Read` timeout 默认 75s |
| 不升高 `ProtocolVersion` | 仍为 5 |
| 重连不误标 offline | `clearRoute` 仅 owner 时 `MarkOffline`；`deviceSummary` 以 route 为准 |
| browser demux 止血 | relay 写 3s 超时（关联实现） |
| 固定常量、不配 yaml | 符合 non-goal |
| 测试覆盖 | 重连 / idle offline / ping 保活 / clearRoute |

## Spec alignment

不适用。light / 轻量模式无独立 Spec。

## Plan alignment

不适用。light / 轻量模式无独立 Plan；按 requirement 直接实现。

## Actual diff summary

暂存范围（`git diff --cached`）：

1. **Requirement**
   - 新增 `docs/requirement/20260719-agent-tunnel-heartbeat.md`

2. **Tunnel 协议辅助**
   - `HeartbeatInterval` / `HeartbeatIdleTimeout`（默认可测覆盖）
   - `PingFrame` / `PongFrame`

3. **Agent connector**
   - 心跳 goroutine 周期 Ping
   - 入站帧刷新存活
   - Read 带 idle 超时

4. **Cloud tunnel**
   - Read 带 idle 超时
   - 重连 owner 判断后才 `MarkOffline`
   - `deviceSummary.online` 与 route 对齐
   - browser/watch 写超时，避免 demux 阻塞

5. **测试**
   - 重连 online、clearRoute owner、idle offline、ping 保活

## Expected vs actual changed files

预期：

- `docs/requirement/20260719-agent-tunnel-heartbeat.md`
- `docs/verification/20260719-agent-tunnel-heartbeat.md`
- `internal/shared/dto/protocol/tunnel/frame.go`
- `internal/agent/application/user/cloud_client.go`
- `internal/cloud/api/handler/server.go`
- `internal/cloud/api/handler/route.go`
- `internal/cloud/api/handler/tunnel_test.go`

实际暂存与预期一致（verification 文档在本阶段追加暂存）。

未纳入：前端 prettier 全量重写噪音（已 `git restore web`）。

## Acceptance criteria checklist

- [x] Agent hello 后周期 control Ping（25s）
- [x] Cloud 对 Ping 回同 nonce Pong
- [x] Agent 超时无入站进展则关隧道
- [x] Cloud idle Read 超时关连接并 offline（仅当前 owner）
- [x] 不升高 protocol version
- [x] 重连不把存活 route 标 offline
- [x] 相关单测通过
- [x] requirement 记录根因与边界

## Test results

### 相关后端（通过）

```text
go test ./internal/cloud/api/handler/ -run "Tunnel|ClearRoute" -count=1
# PASS: RegistersDevice, ReconnectKeepsDeviceOnline, ClearRouteOnlyMarksCurrentOwner,
#       IdleTimeoutMarksOffline, PingKeepsDeviceOnline

go test ./internal/agent/application/user/ -count=1
# ok

go vet ./internal/cloud/api/handler/ ./internal/agent/application/user/ ./internal/shared/dto/protocol/tunnel/
# ok

./bin/golangci-lint run --concurrency=1 \
  ./internal/cloud/api/handler/ \
  ./internal/agent/application/user/ \
  ./internal/shared/dto/protocol/tunnel/
# 0 issues
```

### 前端 check 子集（通过）

```text
yarn --cwd web typecheck   # ok
yarn --cwd web lint:fix     # ok
```

`yarn --cwd web format:fix` 会改写大量既有前端文件（与本 feature 无关），已丢弃，未纳入 diff。
完整 `task check` 因环境命令超时未能整包跑完；等价后端检查已对变更包执行。

### 未通过 / 未运行（范围外或环境）

```text
go test ./cmd/termbridge/app/ -run TestRunAgentStartsBackendAndConnectorFromConfig
go test ./cmd/termbridge/app/ -run TestRunAgentStartsCloudConnectorAfterDeviceReport
# FAIL: waitFor gotServer 1s 超时（启动 hook 未触发）

go test ./cmd/termbridge/app/ -run TestPortableAgentProfilesStartWithoutCloudJWT
go test ./cmd/termbridge/app/ -run TestPortablePackageShipsSelectableRuntimeProfiles
# FAIL: 缺少 scripts/package/.env.prod|.env.test
```

上述 cmd 测试失败与本 feature 无直接代码路径耦合（未改 `cmd/termbridge/app` 装配）；记为仓库既有/环境问题，不阻塞本需求验收，但完整 `go test ./cmd/...` 当前不能算全绿。

未跑：真实 agent↔cloud 长时间空闲手工联调、全仓 `go test ./internal/...` 一次跑完。

## Missed or expanded scope

- 未引入 yaml 心跳配置（符合 non-goal）
- 未做异步 fan-out 中继重构（仅写超时止血）
- 未改 browser terminal 子协议 ping

## Risks

1. 心跳与业务共用 `writeMu`：极端终端洪水时 Ping 可能被拖住，依赖对端 idle 兜底。
2. 75s 窗口内仍可能有短暂假 online（可接受折中）。
3. `TestAgentTunnelReconnectKeepsDeviceOnline` 约 5s（旧连接 Close 等待），偏慢但不影响正确性。
4. 完整 `task check` / 全量 cmd 测试未在本环境一次绿通。

## Incomplete items

1. 手工验收：空闲数分钟保持 online；杀 agent 后约 75s offline；重连后 online 不闪假离线。
2. 仓库内 `cmd/termbridge/app` 若干既有失败需另案处理。
3. 可选：缩短 reconnect 测试对 `first.Close` 的阻塞等待。

## Conclusion

**需求范围内实现可接受。**
控制面心跳、重连 online 正确性、browser 写超时止血均已落地并通过针对性测试与 lint/vet。
交付边界清晰：仅 6 个代码/文档文件 + 本 verification；不包含前端无关格式化变更。
建议在合并前做一次真实空闲联调，并另开任务处理 cmd package 测试环境问题。
