# TermBridge TODO

## 已修复 (2026-06-18)

### webterminal: newsession 立即返回 exit 1
- **现象**: POST /api/sessions 创建会话后进程立即退出，exit_code=1
- **根因**: `gopty.Manager.Start` 将 HTTP 请求的 `context.Context` 传入 `CommandContext`，HTTP 返回时 context 被取消，gopty 调用 `Process.Kill()` 终止进程
- **修复**: `internal/pty/gopty/manager.go` 改用 `context.Background()`，进程生命周期由 session runtime 管理

### webterminal: 终端页面 spam "rows out of range"
- **现象**: 打开终端后持续输出 `[termbridge:invalid_control] rows out of range: 201+`
- **根因**: `terminalproto.MaxRows = 200`，但浏览器窗口较大时 xterm fitAddon 计算出超过 200 行
- **修复**: `internal/terminalproto/protocol.go` 将 `MaxRows` 从 200 提升到 500

---

## 已由 M5/M6 处理或缓解

以下事项曾作为 M2.5/M5 风险记录，当前不再作为未处理 TODO 保留；事实来源见 `docs/verification/20260620-m5-runtime-hardening.md` 和 `docs/verification/20260620-m6-gateway-web-terminal-mvp.md`。

### webterminal: 大量输出时 client 被 detach / readLoop 背压
- **原风险**: 进程持续快速输出时，前端终端可能因 client queue 满而 detach，或 readLoop 不感知 WebSocket 消费能力。
- **当前处理**: M5 引入 bounded queue / queued bytes backpressure，slow client 超限 detach，但 runtime/history 继续工作。
- **剩余边界**: 仍需更大规模端到端压力验证，尤其是 Gateway tunnel 场景。

### webterminal: history 全量刷盘性能与并发安全
- **原风险**: 高频输出时 history flush 导致磁盘 I/O 频繁，并可能与 attach replay / history read 发生并发 race。
- **当前处理**: M5 加固 `history.Writer`，增加 mutex、pending flush、interval flush 和 race-sensitive 测试。
- **剩余边界**: 仍需真实长时间运行和大输出场景观察。

### webterminal: copyBytes 高频内存分配
- **原风险**: 高频输出时每个 client 重复 copy 带来 GC 压力。
- **当前处理**: M5 减少 per-client copy，避免 `publishBinary` 为每个 client 重复复制。
- **剩余边界**: Gateway tunnel terminal output 仍需继续压测。

---

## 仍需人工验证

1. Agent disconnect / reconnect 行为和 stale route cleanup。
2. 20+ sessions 或同等级压力场景。
3. 真实 Claude Code / Codex TUI 通过 local Web 和 Gateway attach 的交互行为。
4. Windows 真机上的 Agent/Gateway 连接、ConPTY 和 process tree cleanup。
5. 存在 running session 时的 Pomelo PW terminal attach 分支。

---

## 已完成功能

- [x] M2.5 Web Terminal 原型 (web 界面 + PTY 会话)
- [x] 会话创建/附加/分离/关闭
- [x] WebSocket 双向通信 (输入/输出/resize/detach/close)
- [x] 历史记录持久化
- [x] Unicode/ANSI 输出支持
- [x] 用户退出码传递
- [x] 大终端 resize 支持 (最大 500x500)
- [x] M5 Runtime Hardening: process cleanup、backpressure、history 并发安全
- [x] M6 Gateway Web Terminal MVP: Gateway relay、Agent tunnel、device/session/history relay
- [x] M6.1 Serve 统一入口: `termbridge serve` / `just serve` 和配置驱动 Gateway/Agent 参数
