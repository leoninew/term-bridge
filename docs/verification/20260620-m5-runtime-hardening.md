# M5 Runtime Hardening 验证
最后修改时间: 2026-06-20 18:11:48

Review status: Accepted

## Requirement alignment

依据：`docs/requirement/20260620-m5-runtime-hardening.md`

- [x] 加固 process cleanup 和 process tree cleanup，并提供可重复验证方式。
  - macOS / Unix: 新增进程组级 `KillTree()` 实现和真实子进程清理测试。
  - Windows: 新增 Job Object cleanup 代码路径，并通过 Windows 交叉编译检查。
- [x] 明确 Ctrl+C / interrupt / close / kill 的边界和升级行为。
  - `CommandRunner` escalation 测试覆盖 interrupt → close → kill、stderr 提示和 `OnStopping` reason。
- [x] Claude Code / Codex 真实 TUI 验证项已纳入 manual verification checklist。
  - 当前自动化未直接驱动 Claude/Codex TUI；保留为人工验证项。
- [x] Web terminal 大输出、slow client backpressure、history flush、copyBytes 高频分配风险已有处理。
  - client queue/queued bytes 可配置，slow client queue full 会 detach 且 runtime/history 继续工作。
  - `publishBinary` 不再为每个 client 重复 copy。
  - `history.Writer` 增加并发安全和批量 flush 测试。
- [x] session/workspace metadata、state、process、exit、history 与 runtime 状态一致性由现有测试和新增 backpressure/history 测试覆盖。
- [x] 建立 M5 自动化测试命令和人工验证清单。
- [x] 保持 CLI-first 和 runtime ownership；未引入 Gateway/M6，也未让 Web/workbench 拥有 PTY/Process runtime。

## Spec alignment

依据：`docs/spec/20260620-m5-runtime-hardening.md`

- [x] Process lifecycle hardening：实现 Unix/macOS process group kill 和 Windows Job Object cleanup。
- [x] Runtime consistency hardening：保持 `PTYSession.KillTree()` 作为边界，不扩张业务接口；新增 runner、webterminal、history 测试。
- [x] Web terminal output hardening：实现 bounded queue/bytes backpressure，slow client detach 后 runtime/history 继续工作。
- [x] Verification hardening：运行相关包测试、race-sensitive 测试、Windows 交叉编译和全量 Go 测试。
- [x] `PTY Session` interface 未扩张。
- [x] `Runner Hooks` 继续使用 `OnStopping`，并补测试验证 reason。
- [x] `history.Writer` public methods 保持不变；内部增加 mutex 以修复 Web runtime 并发 flush/write race。

## Plan alignment

依据：`docs/plan/20260620-m5-runtime-hardening.md`

| Step | Plan item | Status | Notes |
|---|---|---:|---|
| 1 | 建立 M5 hardening 测试基线 | Done | runner/webterminal/history/gopty 新增测试 |
| 2 | 实现 macOS / Unix process tree cleanup | Done | `process_tree_unix.go` + `process_tree_unix_test.go` |
| 3 | 实现 Windows process tree cleanup 能力 | Done with environment limitation | Job Object 代码已实现并交叉编译；真实运行需 Windows 环境 |
| 4 | 加固 CLI interrupt / close / kill 可观察性 | Done | 新增 escalation 测试 |
| 5 | 实现 Web terminal backpressure 策略 | Done | queue bytes 可配置，slow client detach 测试覆盖 |
| 6 | 加固 history 写盘和 replay | Done | pending flush、interval flush、race 修复和测试覆盖 |
| 7 | 验证 session/workspace metadata 与 runtime 状态一致 | Partially automated | 现有 app/session/state 测试继续通过；本轮新增 Web runtime/history 状态相关覆盖 |
| 8 | 建立 M5 自动化验证命令和人工验证清单 | Done | 本文记录自动化命令和 manual checklist |

## Actual diff summary

本次实现：

1. 在 go-pty adapter 层新增平台 process tree cleanup：
   - Unix/macOS 使用进程组 kill。
   - Windows 使用 Job Object + kill-on-job-close，并保留 fallback。
2. `gopty.session` 保存平台 `killTree` / `cleanupTree` 回调，`KillTree()` 和 `Wait()` 调用平台 cleanup。
3. Web terminal backpressure 增强：
   - `Config.ClientQueueBytes` 可配置。
   - `Client.enqueue` 使用 registry-level byte limit。
   - slow client queue full 时 detach，不阻塞 runtime/history。
   - `publishBinary` 避免 per-client 重复 copy。
4. `history.Writer` 增加 mutex，修复 Web runtime 中 `readLoop` 写入与 `History()/attach replay` flush 的数据竞争。
5. 增加 runner、history、webterminal、gopty 测试覆盖 hardening 行为。
6. 创建并接受 M5 Requirement / Spec / Plan 文档。

## Expected vs actual changed files

| File | Expected | Actual | Notes |
|---|---:|---:|---|
| `docs/requirement/20260620-m5-runtime-hardening.md` | Yes | Yes | M5 requirement accepted |
| `docs/spec/20260620-m5-runtime-hardening.md` | Yes | Yes | strict mode spec accepted |
| `docs/plan/20260620-m5-runtime-hardening.md` | Yes | Yes | strict mode plan accepted |
| `docs/verification/20260620-m5-runtime-hardening.md` | Yes | Yes | 本验证文档 |
| `internal/pty/gopty/manager.go` | Yes | Yes | 接入平台 kill/cleanup 回调 |
| `internal/pty/gopty/manager_windows.go` | Yes | Yes | 更新说明，Windows cleanup 在新文件 |
| `internal/pty/gopty/process_tree_unix.go` | Yes | Yes | Unix/macOS process tree cleanup |
| `internal/pty/gopty/process_tree_unix_test.go` | Yes | Yes | Unix/macOS 子进程清理测试 |
| `internal/pty/gopty/process_tree_windows.go` | Yes | Yes | Windows Job Object cleanup |
| `internal/runner/runner_test.go` | Yes | Yes | interrupt escalation 可观察性测试 |
| `internal/webterminal/registry.go` | Yes | Yes | queue bytes 可配置，backpressure 限制 |
| `internal/webterminal/runtime.go` | Yes | Yes | 减少 per-client copy |
| `internal/webterminal/registry_test.go` | Yes | Yes | slow client detach/runtime/history 测试 |
| `internal/history/writer.go` | Yes | Yes | mutex 修复并发 flush/write race |
| `internal/history/writer_test.go` | Yes | Yes | flush/batch/interval 测试 |
| `internal/runner/runner.go` | Possible | No | 行为已存在，本轮只补测试 |
| `internal/pty/pty.go` | Possible | No | 接口无需变更 |
| `internal/session/*_test.go` | Possible | No | 现有测试通过，本轮未新增 |
| `internal/state/store_test.go` | Possible | No | 现有测试通过，本轮未新增 |
| `internal/app/app_test.go` | Possible | No | 现有测试通过，本轮未新增 |
| frontend files | Conditional | No | backpressure 可通过现有 detach reason/runtime state 表达，未扩展 frontend |

## Acceptance criteria checklist

- [x] CLI command runner 长时间运行相关 cleanup path 有自动化覆盖。
- [x] Ctrl+C soft interrupt、close PTY、kill process tree 的边界和升级行为有测试覆盖。
- [ ] Claude Code / Codex 真实交互有验证记录。
  - 未在自动化中执行，保留为 manual verification item。
- [ ] 20+ command/session 或同等压力场景有验证记录。
  - 本轮未新增 20+ session 压测记录，保留为 manual verification item。
- [x] 大量 output 不导致 runtime 因 slow client 被拖死；slow client detach 后 runtime/history 继续工作。
- [x] 高频 resize 既有测试继续通过；未发现协议错误、panic 或 runtime 卡死。
- [x] process cleanup / process tree cleanup 有测试或可编译验证；Windows 真实运行待 Windows 环境核验。
- [x] Web terminal backpressure 策略明确：bounded queue + bounded queued bytes，超过限制 detach slow client，runtime/history 继续工作。
- [x] history 有明确上限和批量 flush 策略；并发 flush/write race 已修复。
- [x] session/workspace metadata 与真实进程状态一致性由现有测试和本轮新增 runtime/history 测试覆盖。
- [x] 可自动化项有测试覆盖；不可稳定自动化项列入 manual verification checklist。
- [x] M5 代码层 hardening 已满足进入 M6 前的主要 runtime 风险收敛；仍需人工验证记录补齐。

## Test results

### Package hardening tests

已运行：

```text
go test ./internal/runner ./internal/pty/gopty ./internal/webterminal ./internal/history ./internal/session ./internal/state ./internal/app
```

结果：

```text
ok  	termbridge-go/internal/runner	(cached)
ok  	termbridge-go/internal/pty/gopty	(cached)
ok  	termbridge-go/internal/webterminal	(cached)
ok  	termbridge-go/internal/history	(cached)
ok  	termbridge-go/internal/session	(cached)
ok  	termbridge-go/internal/state	(cached)
ok  	termbridge-go/internal/app	0.041s
```

### Race-sensitive tests

首次运行：

```text
go test -race ./internal/webterminal ./internal/runner
```

发现问题：

```text
WARNING: DATA RACE
termbridge-go/internal/history.(*Writer).Write()
termbridge-go/internal/history.(*Writer).Flush()
```

处理：

- 为 `history.Writer` 增加 `sync.Mutex`。
- 将内部 flush 改为 locked path，避免 `Write()` 持锁时递归调用 public `Flush()`。
- 更新测试中直接修改 `lastFlushed` 的位置，按 mutex 保护。

修复后重跑：

```text
go test ./internal/history ./internal/webterminal
go test -race ./internal/webterminal ./internal/runner
```

结果：

```text
ok  	termbridge-go/internal/history	0.023s
ok  	termbridge-go/internal/webterminal	0.183s
ok  	termbridge-go/internal/webterminal	1.229s
ok  	termbridge-go/internal/runner	(cached)
```

### Windows compile boundary

已运行：

```text
GOOS=windows GOARCH=amd64 go test -c -o /tmp/termbridge-gopty-windows.test.exe ./internal/pty/gopty
```

结果：通过编译。

说明：macOS 环境不能执行 Windows `.exe` 测试二进制，因此 Windows process tree cleanup 仍需在 Windows 环境做真实运行核验。

### Pomelo PW Web/workbench flow

新增 flow：

```text
.pomelo-pw/m5-web-terminal.yaml
```

已运行：

```text
pomelo-pw validate .pomelo-pw/m5-web-terminal.yaml
pomelo-pw run .pomelo-pw/m5-web-terminal.yaml -o .pomelo-pw/output --headless -v --var base_url=http://localhost:9011 --var cwd=/Users/leon
```

结果：

```text
Validation passed
All steps completed successfully
Completed: 16 steps, 4 screenshots
```

覆盖：

- 前端 workbench 页面可访问。
- 后端 `/api/sessions` 可创建 session。
- session summary 可从 `/api/sessions` 查询。
- history endpoint 可读到 marker 输出 `POMELO_M5_WEB_TERMINAL_OK`。
- viewport resize 后页面可截图。
- session 进入终态 `stopped`，exit code 为 `0`。

截图输出：

```text
.pomelo-pw/output/01-workbench-loaded.png
.pomelo-pw/output/02-session-created.png
.pomelo-pw/output/03-resized-workbench.png
.pomelo-pw/output/04-session-closed.png
```

备注：当前 flow 使用浏览器端 API 创建短命令 session，并通过页面截图验证 workbench 状态；未覆盖 xterm 手动输入。

### Full regression

已运行：

```text
go test ./...
```

结果：

```text
?   	termbridge-go/cmd/termbridge	[no test files]
ok  	termbridge-go/internal/app	(cached)
ok  	termbridge-go/internal/cli	0.081s
ok  	termbridge-go/internal/config	(cached)
ok  	termbridge-go/internal/errors	(cached)
ok  	termbridge-go/internal/history	(cached)
ok  	termbridge-go/internal/identity	(cached)
ok  	termbridge-go/internal/logging	(cached)
ok  	termbridge-go/internal/process	(cached)
?   	termbridge-go/internal/pty	[no test files]
ok  	termbridge-go/internal/pty/gopty	(cached)
ok  	termbridge-go/internal/runner	(cached)
ok  	termbridge-go/internal/session	(cached)
ok  	termbridge-go/internal/state	(cached)
ok  	termbridge-go/internal/terminalproto	(cached)
?   	termbridge-go/internal/version	[no test files]
ok  	termbridge-go/internal/webserver	0.022s
ok  	termbridge-go/internal/webterminal	(cached)
ok  	termbridge-go/internal/workspace	(cached)
?   	termbridge-go/web/node_modules/flatted/golang/pkg/flatted	[no test files]
```

## Manual verification checklist

以下项目仍需人工执行并记录结果：

### macOS 当前环境

- [ ] `termbridge exec -- <long-running shell command>` 启动、输出、退出。
- [ ] CLI Ctrl+C 第一次 soft interrupt；未退出时升级 close，再升级 kill tree。
- [x] shell 子进程树 force stop 后不遗留：已由 `process_tree_unix_test.go` 自动覆盖。
- [ ] `termbridge exec -- claude` 真实启动、输入、输出、退出、中断。
- [ ] `termbridge exec -- codex` 真实启动、输入、输出、退出、中断。
- [x] local Web/workbench 创建 session、输出、resize、close/终态：已由 `.pomelo-pw/m5-web-terminal.yaml` 覆盖。
- [ ] local Web/workbench xterm 手动输入、attach/detach 交互。
- [x] 大输出命令下 slow client 不拖死 runtime，detach reason/runtime/history 可观察：已由 `TestSlowClientDetachDoesNotStopRuntimeOrHistory` 自动覆盖。
- [ ] 高频 resize 人工操作不出现 panic、协议错误或 runtime 卡死。
- [x] history replay / history endpoint 可用：Pomelo PW flow 已验证 marker output 可从 history 读取。
- [x] workspace/session list 与真实 state/exit/process/history 一致：Pomelo PW flow 已验证 session summary、history、终态 `stopped` 和 exit code `0`。

### Windows 后续环境

- [ ] shell 子进程树 force stop 后不遗留。
- [ ] Ctrl+C / close / kill escalation 行为符合记录。
- [ ] Claude Code / Codex 真实 TUI 能启动、交互、退出、中断。
- [ ] Web session close 后进程树清理。
- [ ] process/exit/state/history 与真实进程状态一致。

## Missed or expanded scope

### 未进入范围

- 未进入 Gateway/M6。
- 未实现多用户 auth、device registry、agent tunnel 或 remote pairing。
- 未改变正式 CLI 入口。
- 未实现显式 `recent` UX。
- 未做 release packaging / installer / upgrade strategy。

### 实际扩展

- 在 Verification 过程中通过 `go test -race` 发现并修复 `history.Writer` 并发数据竞争。该问题属于 M5 runtime hardening 范围，已纳入本次实现。

## Risks

1. Windows Job Object cleanup 已实现并通过交叉编译，但真实进程树清理仍需 Windows 环境核验。
2. Claude Code / Codex 真实 TUI 尚未人工执行；自动化测试不能替代账号/交互环境验证。
3. 20+ session 压测尚未人工或自动化执行；当前主要通过单元测试、targeted runtime tests 和 Pomelo PW Web flow 覆盖关键路径。
4. Web frontend 未新增 slow-client UI 展示；当前可观察性主要是 detach reason、attachment state、runtime/history 行为、Pomelo PW 截图和测试覆盖。

## Incomplete items

进入后续人工验证或 Windows 环境验证：

1. Windows 真实 process tree cleanup 核验。
2. Claude Code / Codex 真实 TUI 验证。
3. 20+ command/session 或同等压力验证记录。
4. local Web/workbench 剩余人工交互验证：xterm 手动输入、attach/detach、高频 resize。
5. 高频 resize 人工验证。

## Conclusion

M5 Runtime Hardening 的核心代码项已完成：process tree cleanup、interrupt escalation 可观察性、Web backpressure、history 并发安全和 flush 行为均有自动化覆盖。相关包测试、race-sensitive 测试、Windows 交叉编译、Pomelo PW Web/workbench flow 和全量 Go 测试均通过。

M5 可以作为代码层 hardening 收口；进入 M6 前建议补齐人工验证记录，尤其是 Claude Code / Codex 真实 TUI、20+ session 压测和 Windows 真实环境 process tree cleanup。
