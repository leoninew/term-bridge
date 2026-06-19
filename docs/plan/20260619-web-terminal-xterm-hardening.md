# Web Terminal xterm.js 集成治理计划
最后修改时间: 2026-06-19 19:08:00

Review status: Accepted

## Basis

本计划基于：

- `docs/requirement/20260619-web-terminal-xterm-hardening.md`，Review status: Accepted
- `docs/spec/20260619-web-terminal-xterm-hardening.md`，Review status: Accepted

当前流程：strict / 严格模式，Plan / 计划阶段。

本计划只定义实施步骤和验证方式，不修改产品代码。

## Implementation steps

### 1. 后端安全边界与配置

1. 在 web server 配置路径中增加本地 auth guard 所需字段：
   - local token 生成与保存到进程内存。
   - allowed origins。
   - dev origin 配置。
   - cwd allowlist。
   - env denylist。
2. 新增 bootstrap endpoint：
   - `GET /api/bootstrap`
   - 只允许通过 Origin 校验后的请求访问。
   - 返回前端需要的 local token 和必要 client config。
3. 为 `/api/*` 受保护 endpoint 增加 token 校验：
   - `Authorization: Bearer <token>`。
   - `/api/bootstrap` 除外。
4. 为 WebSocket attach 增加 token 校验：
   - URL query `?token=<token>`。
   - 认证失败时拒绝 upgrade。
   - 日志脱敏 token。
5. 移除默认 WebSocket `InsecureSkipVerify`：
   - 默认启用 Origin 校验。
   - 测试/dev 通过显式配置跳过。
6. 非 loopback host 检查：
   - host 为 `0.0.0.0`、`::`、局域网 IP 或公网 IP 时必须有 token。
   - token 不可用时拒绝启动。
   - 启动输出提示暴露范围和风险。

### 2. cwd / command / env policy

1. 实现 cwd allowlist：
   - 默认 allowlist 为启动 effective cwd。
   - 配置可增加其他 workspace root。
   - 请求 cwd 使用 `filepath.Abs` + `filepath.EvalSymlinks` 规范化。
   - Windows containment 比较大小写不敏感。
2. 创建 session 时校验 cwd containment：
   - 不在 allowlist 内则返回明确 API error。
3. command policy：
   - loopback + token 允许任意 command。
   - 非 loopback 必须满足 token、Origin allowlist 和 cwd allowlist。
   - UI 和启动日志提示远程执行风险。
4. env policy：
   - 实现 `web.env.denylist` 配置入口。
   - 默认 denylist 为空。
   - session metadata 只记录 env strategy、denylist 数量和 env count，不记录 env value。

### 3. WebSocket 单 writer 与读取限制

1. 重构 `serveWebSocket` 为单 writer actor：
   - reader loop 只读消息、校验并调用 runtime action。
   - reader loop 不直接 `conn.Write`。
   - 所有 control/error/output 都进入 writer channel。
2. 增加 priority error path：
   - 用于 queue full / frame too large / protocol error 的最后错误提示。
   - 如果无法发送，则依赖 close reason 和 server log。
3. 使用受限读取方式：
   - text control frame 最大 `terminalproto.MaxJSONMessageBytes`。
   - binary input frame 最大 `terminalproto.MaxBinaryFrameBytes`。
   - 超限关闭连接并记录 reason。
4. 更新 close/detach 行为：
   - detach 只断开当前 client。
   - close session 不再走 WebSocket control。

### 4. REST close session endpoint

1. 新增：

   ```http
   POST /api/sessions/{session_id}/close
   ```

2. endpoint 行为：
   - 校验 token。
   - 找到 running runtime。
   - 请求关闭 PTY session。
   - 广播 state/exited。
   - 关闭 attached clients。
   - 保留 session record、history 和 metadata。
3. 前端 close session 按钮改为调用 REST close。
4. 从客户端 WebSocket protocol 类型中移除 `close`。

### 5. Protocol 更新

1. 移除或隐藏 `last_seq`：
   - TypeScript client control 不再包含 `last_seq`。
   - Go client message 不再暴露或不再使用 `LastSeq`。
2. 新增 server control：
   - `replay_started`
   - `replay_finished`
3. 增强 schema validation：
   - `resize` 必须有合法 cols/rows。
   - `ping` nonce 长度受限。
   - server `error` 必须有 code/message。
   - server `exited` 必须有 exit_code/state。
   - unknown type 拒绝。
4. 前端 decode control 做字段校验，不只检查 `type`。

### 6. Attach-time bounded history replay

1. runtime attach 顺序改为：

   ```text
   attach client
   create writer channel but do not subscribe to live output yet
   flush history writer
   send started
   send replay_started
   send history binary
   send replay_finished
   subscribe to live output
   begin live output
   ```

2. replay 使用 current bounded history file。
3. replay binary 直接写入 xterm。
4. `replay_finished.truncated` 本轮固定为 `false` 或省略。
5. UI 显示 bounded history replay 状态。

### 7. Queue full / 大输出 / 慢客户端

1. client queue 增加 queued bytes 计数。
2. 保留 bounded channel，但加入 byte 上限。
3. 连续 binary chunks 进入 queue 前允许合并。
4. queue full 或 bytes 超限时：
   - detach reason 为 `client_queue_full`。
   - 尝试 priority error control。
   - 失败则使用 close reason。
   - server log 记录 session id、client id、queue size、queued bytes、reason。
5. 前端展示断开原因，不再只显示普通 disconnected。

### 8. History writer batch flush

1. 修改 history writer：
   - 内存仍维护 bounded lines/bytes。
   - 写入时不每次全量刷盘。
2. flush 条件：
   - pending bytes ≥ 64 KiB。
   - 距离上次 flush 超过 200ms。
   - 显式 `Flush()`。
   - `Close()`。
3. 新增公开或内部 `Flush()` 方法。
4. `SessionRuntime` attach-time replay 前调用 `Flush()`。
5. `Close()` 必须强制 flush，避免最后输出丢失。

### 9. Frontend bootstrap / API / socket

1. 前端启动时调用 `GET /api/bootstrap`。
2. API wrapper 自动附带 `Authorization: Bearer <token>`。
3. WebSocket URL 自动附带 `?token=<token>`。
4. `useTerminalSocket`：
   - 移除 hello `last_seq`。
   - 处理 `replay_started` / `replay_finished`。
   - 处理 close reason / error reason。
5. `api.ts`：
   - 新增 close session API。
   - read history 保留给 raw 辅助视图。

### 10. Frontend xterm components

1. 调整 session 创建流程：
   - 创建 session 前基于 terminal 容器计算 cols/rows。
   - create request 使用实际 fit 后尺寸。
   - 无法测量时使用保守 fallback，并在 attach 后立即 resize。
2. `useXterm` 增加 write queue：
   - 使用 `terminal.write(data, callback)`。
   - 维护 pending bytes。
   - pending bytes 阈值为 4 MiB。
   - 超限时 UI 显示 output backlog 状态。
3. 新增 `HistoryTerminalView`：
   - readonly。
   - 加载 history 后 chunked write 到 xterm。
   - 禁用 input forwarding。
   - raw text 作为辅助诊断视图保留。
4. 错误展示：
   - terminal 内只写简短安全提示。
   - 详细错误展示到 UI panel。
   - 写入 terminal 前转义控制字符。
5. 输入编码测试：
   - 保留 `onData` + `onBinary` 双通道。
   - 若测试发现重复发送，删除重复路径。

## Files to change

### Backend expected files

- `internal/webserver/server.go`
  - bootstrap endpoint。
  - auth guard。
  - Origin 校验。
  - WebSocket token 校验。
  - single writer。
  - close endpoint。
  - read limit。

- `internal/webserver/server_test.go` 或新增相关测试文件
  - auth/origin/close/ws 测试。

- `internal/webterminal/registry.go`
  - cwd allowlist。
  - env policy 接入。
  - close session 支撑。
  - history replay 数据读取。

- `internal/webterminal/runtime.go`
  - attach-time replay。
  - queue bytes / queue full reason。
  - outbound control 统一入队。
  - Flush history before replay。

- `internal/webterminal/registry_test.go` / `internal/webterminal/runtime_test.go`
  - cwd policy、queue full、replay、close 相关测试。

- `internal/terminalproto/protocol.go`
  - 移除/隐藏 `LastSeq`。
  - 新增 replay message type。
  - schema validation。

- `internal/terminalproto/protocol_test.go`
  - control validation 测试。

- `internal/history/writer.go`
  - batch flush。
  - `Flush()`。

- `internal/history/writer_test.go`
  - flush 条件、Close flush、bounded history 测试。

- 配置相关文件：
  - `configs/termbridge.default.yaml`
  - `internal/config` 或现有配置加载位置
  - 增加 web auth/origin/cwd/env 配置字段。

### Frontend expected files

- `web/src/features/sessions/api.ts`
  - bootstrap。
  - Authorization header。
  - close session。

- `web/src/features/sessions/useTerminalSocket.ts`
  - token query。
  - replay controls。
  - remove last_seq。
  - close/error reason。

- `web/src/protocol/terminal.ts`
  - remove last_seq。
  - add replay controls。
  - stronger decode validation。

- `web/src/components/terminal/useXterm.ts`
  - write queue。
  - pending bytes。
  - safe error write helper if colocated。

- `web/src/components/terminal/TerminalView.vue`
  - replay state。
  - detach behavior。
  - no close control。

- `web/src/components/terminal/HistoryTerminalView.vue`
  - new readonly replay component。

- `web/src/App.vue`
  - bootstrap before API use。
  - create session with real cols/rows path。
  - stopped history xterm replay + raw view。
  - close session API integration。

- frontend tests if test harness exists; if absent, add minimal TypeScript unit-like tests only if project convention supports it. Otherwise cover via typecheck/build and manual verification notes in Verification.

## Verification plan

### Automated checks

Use project explicit entries:

```text
go test ./cmd/... ./internal/...
cd web && yarn typecheck
cd web && yarn lint
cd web && yarn build
```

If formatting changes are made, run:

```text
go fmt ./cmd/... ./internal/...
cd web && yarn format:check
```

`just check` is available and covers Go fmt/vet/test plus frontend format/typecheck/lint, but it runs `yarn format` which writes files. Use it only when write-formatting is acceptable during Implementation/Verification; otherwise run non-writing checks individually.

### Backend test coverage

- bootstrap endpoint returns token only for allowed Origin.
- protected REST endpoint rejects missing/invalid token.
- WebSocket attach rejects missing/invalid token.
- WebSocket Origin reject works by default.
- test/dev config can explicitly bypass Origin only in tests.
- non-loopback without token rejects startup path.
- cwd outside allowlist rejects create session.
- cwd inside allowlist accepts create session.
- env denylist filters configured names and does not persist values.
- `last_seq` removed/ignored consistently.
- `replay_started` / history binary / `replay_finished` order is deterministic.
- close endpoint closes PTY/runtime and preserves session record/history.
- queue full sets `client_queue_full` reason and logs context.
- history writer batch flush does not write every chunk and Close flushes final data.
- oversized text/binary WS frame rejected before unbounded memory growth.

### Frontend verification

- bootstrap token loaded before session API calls.
- REST API sends Authorization header.
- WebSocket URL includes token query and does not include `last_seq`.
- `replay_started` / `replay_finished` update UI state.
- close button calls REST close endpoint, not WebSocket close control.
- stopped session displays readonly xterm replay.
- raw history remains available as auxiliary view.
- xterm write queue compiles and handles callback sequencing.
- protocol decode rejects malformed control messages.
- error messages displayed safely without raw control character injection into terminal.

### Manual verification

- Start backend on loopback and open frontend.
- Create session with shell command.
- Verify initial terminal size is not fixed `120x32`.
- Resize browser and confirm PTY resize works.
- Run commands producing ANSI color and carriage return output.
- Refresh page while session is running and verify bounded history replay appears before live output.
- Close session through UI and verify process exits and session record remains.
- Open stopped session and verify readonly xterm replay.
- Run large output command and verify no silent detach; if detach occurs, reason is visible.
- Attempt API call without token and confirm rejection.
- Attempt WebSocket without token and confirm rejection.
- Attempt cwd outside allowlist and confirm rejection.

## Blockers

暂无外部 blocker。实现前需要先读取当前相关代码并按本计划分批修改。

## Assumptions

1. 当前 WebSocket library 支持 Reader API 或可实现等效受限读取。
2. 当前前端没有成熟测试框架；若没有测试框架，本轮不强行引入新测试框架，只通过 typecheck/build/manual verification 补足前端验证。
3. local token 保存在进程内存即可，不需要跨重启持久化。
4. 本轮仍以本地 Web terminal 为边界，不引入完整 Gateway 用户模型。

## Risks

1. WebSocket token 放在 query string，可能出现在浏览器开发工具或代理日志中；服务端日志必须脱敏。
2. Origin allowlist 可能破坏 Vite dev proxy；需要覆盖 dev host 配置。
3. attach-time replay 若没有正确隔离 live output，可能出现输出乱序。
4. cwd allowlist 的 symlink 和 Windows path case 处理容易遗漏，需要测试覆盖。
5. batch flush 如果实现错误可能造成 history 丢最后输出，因此 Close/Flush 测试必须覆盖。
6. xterm write queue 可能改变输出时序，需验证 replay 和 live output 都能正常显示。
7. queue full 的最后 error control 可能无法送达，必须依赖 close reason 和 server log 兜底。

## Rollback

如果实现后出现不可接受问题，按以下顺序回滚：

1. 保留安全 guard，回滚前端 replay/write queue 相关改动。
2. 保留 REST close endpoint，临时恢复原 stopped history raw view。
3. 保留 Origin allowlist，回滚 history writer batch flush 到同步 flush。
4. 如 single-writer 重构出现严重问题，先恢复原 WebSocket relay，但必须保留测试暴露问题，不作为最终方案。

## User review notes

本计划按已接受 Requirement / Spec 展开，暂无需要用户确认的未决事项。若用户接受 Plan，下一阶段进入 Implementation / 实现。实现阶段应按安全边界、协议/后端、history/replay、前端 UX、测试验证的顺序分批提交改动，但不执行 git 写操作。

后续修正：用户明确要求移除本轮临时 token/bootstrap/auth 机制，认证留给后续正式用户体系；计划中的 token 相关实施项不再作为当前交付范围，当前只保留 Origin allowlist、cwd allowlist 和 WebSocket Origin 校验。