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

## 待处理

### webterminal: 大量输出时 client 被 detach
- **现象**: 进程持续快速输出时，前端终端突然断开，不再接收后续输出
- **根因**: `Client.queue` 容量仅 64，`publishBinary` 用非阻塞 `enqueue`，queue 满时直接 detach client
- **影响**: 一旦 detach，用户需重新 attach 才能看到后续输出，中间丢失的内容不可恢复
- **位置**: `internal/webterminal/registry.go:358-365`, `internal/webterminal/runtime.go:179-186`

### webterminal: history 全量刷盘性能问题
- **现象**: 大量输出时磁盘 I/O 频繁
- **根因**: `history.Writer.Write` 每次调用都 `os.WriteFile` 将全部行刷新到磁盘
- **配置**: 默认上限 10000 行 / 5MB / 64KB 每行 (`configs/termbridge.default.yaml`)
- **位置**: `internal/history/writer.go:104-111`

### webterminal: readLoop 无背压控制
- **现象**: 进程输出速率超过 WebSocket 消费速率时，内存中积累大量未发送数据
- **根因**: `readLoop` 只读不阻塞，不感知 client 的消费能力
- **位置**: `internal/webterminal/runtime.go:126-144`

### webterminal: copyBytes 高频内存分配
- **现象**: 高频输出时 GC 压力大
- **根因**: `readLoop` 每个 chunk 都调用 `copyBytes` 分配新内存
- **位置**: `internal/webterminal/runtime.go:131`, `internal/webterminal/registry.go:371-375`

---

## 已完成功能

- [x] M2.5 Web Terminal 原型 (web 界面 + PTY 会话)
- [x] 会话创建/附加/分离/关闭
- [x] WebSocket 双向通信 (输入/输出/resize/detach/close)
- [x] 历史记录持久化
- [x] Unicode/ANSI 输出支持
- [x] 用户退出码传递
- [x] 大终端 resize 支持 (最大 500x500)
