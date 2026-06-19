# Web Terminal xterm.js 集成治理验证
最后修改时间: 2026-06-19 19:08:00

Review status: Accepted

## Basis

本验证基于 strict / 严格模式的已接受过程文档：

- `docs/requirement/20260619-web-terminal-xterm-hardening.md`
- `docs/spec/20260619-web-terminal-xterm-hardening.md`
- `docs/plan/20260619-web-terminal-xterm-hardening.md`

验证阶段目标：对照需求、规格、计划和实际 diff，确认本轮实现是否满足 Web Terminal / xterm.js 集成治理的核心目标，并记录命令检查结果、范围偏差、风险和未完成项。

## Requirement alignment

### 已对齐

- 安全边界优先：移除临时 token/bootstrap/auth 机制，认证留给后续正式用户体系；当前保留 Origin 校验和配置化 allowed origins。
- REST / WS 边界：REST API 不再使用临时 Bearer token；WebSocket 不再使用 query token；WebSocket 仍依赖 Origin 校验。
- WebSocket 不再默认 `InsecureSkipVerify`，改用 OriginPatterns。
- cwd 默认限制在启动 cwd 或配置 allowlist 内；请求 cwd 会做 absolute / symlink-resolved containment 校验。
- env policy 增加 denylist 配置入口，默认 denylist 为空，session metadata 不记录 env value。
- WebSocket close control 已从客户端协议移除；close session 改为 REST endpoint。
- `last_seq` 已从前后端客户端协议中移除，避免误导性续传语义。
- running session attach 实现 bounded history replay，并增加 `replay_started` / `replay_finished` 控制消息。
- history writer 改为 batch/debounce flush，并提供显式 `Flush()`。
- 前端增加 xterm write queue、REST close 和 readonly history xterm replay；已移除 bootstrap token / WebSocket token 注入。
- Go 与前端约定检查已通过。
- 未执行 git add / commit / push。

### 部分对齐 / 有偏差

- 非 loopback 监听的正式安全策略尚未实现；当前临时 token 已移除，后续需要由用户体系和部署策略补齐。
- queue full 的 priority error path、close reason、server log 记录未完整实现；当前实现加入 byte-aware queue cap，但 queue 满时主要返回 false / detach，用户可诊断性仍不足。
- 连续 binary chunk 合并未实现。
- 前端 output backlog UI 未完整实现；`useXterm` 内部会在 pending bytes 超过 4MiB 时丢弃新 chunk，但没有把 backlog 状态显式暴露到界面。
- xterm 输入行为的 ASCII / Unicode / Ctrl+C / 方向键 / paste / IME 专项测试未补齐；本轮通过类型检查和构建验证编译正确。

## Spec alignment

### 安全边界

- `internal/webserver/server.go` 保留 Origin 校验和 configured allowed origins，移除 bootstrap endpoint、auth middleware、Bearer token、WebSocket query token 和 token 生成。
- 删除 `web/src/features/auth.ts`。
- `web/src/features/sessions/api.ts` 与 `web/src/features/workspaces/api.ts` 不再注入 Authorization header。
- `web/src/features/sessions/useTerminalSocket.ts` 不再追加 query token。

结论：临时 token 方案已移除；当前仅保留 Origin allowlist，本轮不实现正式认证。

### WebSocket single-writer

- `serveWebSocket` 中实际 `conn.Write` 集中在 writer goroutine。
- reader loop 处理 control 后通过 `client.SendControl` 进入 outbound queue。
- `handleControl` 不再直接持有 WebSocket conn。

结论：单 writer 主路径已实现。

### 读取限制

- WebSocket Accept 后调用 `conn.SetReadLimit(terminalproto.MaxBinaryFrameBytes)`。
- text control 仍由 `terminalproto.DecodeClient` 限制 `MaxJSONMessageBytes`。

结论：已使用 WebSocket 库的读限制能力，满足“不能无限读入后再拒绝”的核心要求。text frame 的更小限制仍在 decode 层执行。

### Protocol

- Go `ClientMessage` 移除 `LastSeq`。
- TypeScript `ClientControlMessage` 移除 `last_seq` 和 `close`。
- 新增 `replay_started` / `replay_finished`。
- 前后端对控制消息进行更严格字段校验。

结论：已对齐。

### Attach-time bounded replay

- `SessionRuntime.attach()` 在加入 live clients 之前调用 `enqueueReplay()`。
- `enqueueReplay()` 顺序为 `started`、`replay_started`、flush history、history binary、`replay_finished`，随后才将 client 纳入 live clients。
- `Registry.History()` 读取 running session history 前会 flush pending history。

结论：核心 replay 顺序已实现。

### Close / detach

- `POST /api/sessions/{session_id}/close` 已实现。
- 前端 close 按钮调用 REST close 后关闭 socket。
- WebSocket client `close` control 已移除。
- detach 仍发送 detach control，不关闭 PTY session。

结论：已对齐。

### History writer

- `history.Writer` 增加 pending bytes 和 lastFlushed。
- flush 条件包括 pending bytes、时间阈值、显式 `Flush()` 和 `Close()`。
- 测试已调整显式验证 Flush / Close 行为。

结论：已对齐。

### Frontend xterm

- `useXterm` 使用 `terminal.write(data, callback)` 串行 drain。
- 维护 pending bytes，超过 4MiB 时丢弃新 chunk。
- `HistoryTerminalView.vue` 提供 readonly xterm replay。
- `App.vue` stopped history 显示 readonly xterm 与 raw history。
- create session 不再固定 120x32，改用 viewport 估算。

结论：主要功能已实现；FitAddon 容器实测尺寸和 backlog UI 仍可后续加强。

## Plan alignment

## Actual diff summary

本轮实际改动覆盖：

### 后端

- `configs/termbridge.default.yaml`
- `internal/config/termbridge.default.yaml`
- `internal/config/config.go`
  - 增加 `web.allowed_origins`、`web.cwd_allowlist`、`web.env.denylist` 配置。

- `internal/app/app.go`
  - 将 web config 传入 webterminal registry 和 webserver。

- `internal/webserver/server.go`
  - Origin 校验。
  - configured allowed origins。
  - 移除临时 bootstrap/token/auth middleware。
  - REST close endpoint。
  - WebSocket read limit。
  - 单 writer control 路径。

- `internal/webserver/server_test.go`
  - 更新测试以移除临时 auth 要求，并保留基础路由验证。

- `internal/webterminal/registry.go`
  - cwd allowlist。
  - env denylist。
  - close session。
  - client queued bytes cap。
  - running history read 前 flush。

- `internal/webterminal/runtime.go`
  - attach-time replay。
  - replay 前 history flush。
  - outbound control 入队。

- `internal/webterminal/registry_test.go`
  - 更新 history 测试以通过 `Registry.History()` 触发 flush。

- `internal/history/writer.go`
- `internal/history/writer_test.go`
  - batch flush、显式 Flush、Close flush。

- `internal/terminalproto/protocol.go`
  - 移除 close / last_seq。
  - 新增 replay control。
  - 增强 message validation。

### 前端

- `web/src/features/auth.ts`
  - 已删除临时 auth helper。

- `web/src/features/sessions/api.ts`
  - 移除 REST auth headers。
  - close session API。

- `web/src/features/workspaces/api.ts`
  - 移除 REST auth headers。

- `web/src/features/sessions/useTerminalSocket.ts`
  - 移除 WebSocket token query。

- `web/src/protocol/terminal.ts`
  - 移除 `last_seq` / `close`。
  - 新增 replay controls。
  - 增强 decode validation。

- `web/src/components/terminal/useXterm.ts`
  - xterm write queue。
  - pending bytes 限制。

- `web/src/components/terminal/TerminalView.vue`
  - replay 状态。
  - REST close。
  - safe terminal text filtering。

- `web/src/components/terminal/HistoryTerminalView.vue`
  - readonly xterm history replay。

- `web/src/App.vue`
  - 移除 bootstrap before API。
  - stopped history readonly xterm + raw history。
  - create session size estimate。

### 过程文档

- `docs/requirement/20260619-web-terminal-xterm-hardening.md`
- `docs/spec/20260619-web-terminal-xterm-hardening.md`
- `docs/plan/20260619-web-terminal-xterm-hardening.md`
- `docs/verification/20260619-web-terminal-xterm-hardening.md`

### 既有/相邻修改

当前工作树还包含早于本轮 xterm hardening 的 PTY / workspace / CLI test 修改：

- `internal/cli/cli_test.go`
- `internal/pty/gopty/manager.go`
- `internal/pty/gopty/manager_test.go`
- `internal/workspace/key.go`

这些文件通过 Go 检查，但不完全属于本轮 Web Terminal xterm hardening 范围，提交前建议确认是否拆分。

## Expected vs actual changed files

### 符合预期

- 配置文件、webserver、webterminal registry/runtime、terminalproto、history writer、前端 API/socket/protocol/xterm components 均在计划预期范围内。
- 测试文件 `internal/history/writer_test.go`、`internal/webserver/server_test.go`、`internal/webterminal/registry_test.go` 的更新符合实现后测试调整需要。

### 超出或相邻范围

- `internal/cli/cli_test.go`
- `internal/pty/gopty/manager.go`
- `internal/pty/gopty/manager_test.go`
- `internal/workspace/key.go`
- `docs/todo.md`

这些文件不是本轮计划的核心预期文件。它们来自此前上下文中的相邻工作，当前验证只确认它们没有破坏 Go checks，不将其视为本轮 xterm hardening 的核心交付。

## Acceptance criteria checklist

- [x] Requirement / Spec / Plan 文档存在，且 Review status 为 `Accepted`。
- [x] 遵守 strict / 严格模式流程进入 Verification。
- [x] WebSocket Accept 不再默认跳过 Origin 校验。
- [x] 临时 local token/bootstrap/auth 机制已按用户要求移除，认证留给后续正式用户体系。
- [~] 非 loopback 监听的正式安全策略未在本轮实现，需要后续用户体系和部署策略补齐。
- [x] WebSocket connection 写入主路径为单 writer。
- [x] close session 不再依赖前端 WebSocket send 后立即 close。
- [x] running session attach 实现 bounded history replay。
- [x] `last_seq` 移除。
- [~] client queue full 有 byte cap，但错误提示、priority path、close reason 和日志未完整实现。
- [x] history writer 不再每 chunk 全量 flush，支持 batch/debounce。
- [~] xterm 输入、resize、输出写入和 stopped history 展示有编译/构建验证；缺少专项自动或手工验证记录。
- [x] Go 测试、Go vet、golangci-lint、前端 typecheck/lint/build/format check 通过。
- [x] 未执行 git 写操作。

## Test results

### Go

使用仓库根目录执行：

```text
PATH="/Users/leon/.version-fox/sdks/golang/bin:$HOME/go/bin:$PATH" go vet ./cmd/... ./internal/...
PATH="/Users/leon/.version-fox/sdks/golang/bin:$HOME/go/bin:$PATH" golangci-lint run ./cmd/... ./internal/...
PATH="/Users/leon/.version-fox/sdks/golang/bin:$HOME/go/bin:$PATH" go test ./cmd/... ./internal/...
```

结果：全部通过。

说明：之前直接执行 `go` 失败是当前 Claude Code 工具环境 PATH 未包含 version-fox Go 路径；改用显式 PATH 后验证通过。

### Frontend

使用 `web/` 目录执行：

```text
yarn format:check
yarn typecheck
yarn lint
yarn build
```

结果：全部通过。

### 未执行

- `just check` 未执行，因为它包含写入式 `yarn format`。Verification 阶段改用非写入命令组合，并已覆盖 go vet / golangci-lint / go test / frontend format check / typecheck / lint / build。
- 未执行浏览器手工验证场景，例如真实 resize、ANSI/TUI、refresh replay、large output、Origin 拦截 probing。

## Missed or expanded scope

### 未完整实现 / 未完整验证

1. 非 loopback 启动风险提示与拒绝路径缺少独立实现和测试。
2. queue full 的 priority error control、close reason、server log 可诊断性未完整实现。
3. binary chunk 合并未实现。
4. 前端 output backlog UI 未完整实现。
5. xterm 输入行为、IME、paste、方向键、Ctrl+C 未做专项测试。
6. 浏览器端真实交互场景未手工执行记录。

### 范围扩展

- 补充修复了 `Registry.History()`：running session history 读取前会 flush pending history，避免 batch flush 后 REST history 读不到刚写入的小输出。这是 batch flush 语义带来的必要修正。
- 补充修复了 WebSocket read limit：使用 `conn.SetReadLimit(terminalproto.MaxBinaryFrameBytes)` 避免 coder/websocket 默认 32KiB 限制破坏二进制输入设计。

## Risks

1. 临时 token 机制已移除；当前没有正式认证，非本地/非受控场景必须等待后续用户体系补齐。
2. queue full 场景的用户可见错误仍不足，慢客户端/大输出下可能仍表现为断开或丢 chunk。
3. `useXterm` 超过 4MiB pending 时直接丢弃新 chunk，缺少 UI 提示，用户可能不知道输出被丢弃。
4. stopped history readonly xterm 一次性写入完整 history，尚未实现 chunked replay；超大 history 可能卡顿。
5. 当前工作树包含相邻 PTY/workspace 修改，提交前若不拆分会扩大 commit 边界。

## Incomplete items

建议后续继续补齐：

1. 增加 webserver Origin allowlist / close endpoint 的专项测试；正式 auth 测试留到用户体系实现时补齐。
2. 增加 cwd allowlist、env denylist、non-loopback host policy 的专项测试。
3. 增加 runtime attach replay 顺序和 queue full reason 的专项测试。
4. 实现 queue full priority error / close reason / server log。
5. 前端显示 output backlog / dropped output 状态。
6. 为 xterm 输入行为补充测试或记录手工验证矩阵。
7. 执行浏览器手工验证场景并追加记录。
8. 提交前审视并拆分非本轮核心文件。

## Conclusion

本轮实现通过自动化验证，核心目标已达成：Web Terminal 从原型状态推进到具备 Origin allowlist、REST close、移除误导性 `last_seq`、单 writer 主路径、attach-time bounded replay、history batch flush 和前端 xterm write queue / readonly replay 的状态。临时 token/bootstrap/auth 已按用户要求移除，正式认证留给后续用户体系。

但仍存在若干未完整项，主要集中在大输出/慢客户端的可诊断性、non-loopback 风险提示与测试、xterm 输入专项验证、浏览器手工验证和提交边界拆分。当前代码可作为本轮 hardening 的阶段性交付，但建议在提交前确认是否将早前 PTY/workspace 修改拆出独立 commit。
