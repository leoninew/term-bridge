# Web Terminal xterm.js 集成治理规格
最后修改时间: 2026-06-19 19:08:00

Review status: Accepted

## Requirement basis

本规格基于已接受的需求文档：

- `docs/requirement/20260619-web-terminal-xterm-hardening.md`

当前流程：strict / 严格模式。

需求已确认的关键决策：

1. 安全边界是本轮治理第一优先级。
2. reconnect / replay 纳入本轮核心治理范围，但短期优先实现 attach-time bounded history replay，不要求一次性实现完整 seq replay。
3. `last_seq` 短期移除或隐藏，避免协议误导；未来真正实现 seq replay 时再恢复。
4. stopped session history 应增加 readonly xterm replay，raw text 可作为辅助视图或诊断视图保留。
5. 本地默认应提供 token + Origin 保护；非 loopback 监听必须具备 token 保护。
6. cwd 默认限制在启动 cwd 或配置 allowlist 内。
7. command 在 token 保护下保留本地工具所需灵活性；非 loopback 场景必须更严格。
8. env policy 进入本规格设计，至少考虑可配置 denylist。

## Overview

本轮治理采用“先建立安全与协议边界，再改善 terminal 体验”的设计顺序。

目标架构仍保持当前 Go backend + Vue/xterm.js frontend 的形态，但需要明确以下边界：

```text
Browser UI
  ↓ REST with local protection
Session create/list/history/close API
  ↓
Browser xterm.js
  ↓ WebSocket with Origin + token/session attach protection
Single-writer terminal relay
  ↓
SessionRuntime
  ↓
PTY + bounded history + client output policy
```

本规格将当前 Web terminal 归类为本地受保护 terminal surface，而不是无保护 HTTP API。默认场景是本机用户通过同源页面访问；非 loopback 监听必须被视为更高风险场景，必须启用 token 保护，并必须在启动输出中提示暴露范围。

## Design decisions

### 1. 安全边界

#### 1.1 Origin 校验

WebSocket Accept 不应默认跳过 Origin 校验。

设计方向：

- 默认只允许同源 Origin。
- 允许空 Origin 的条件需要谨慎定义，仅用于明确的本地 CLI / 测试场景。
- dev server proxy 场景需要在配置中允许开发 Origin，例如 Vite dev server 地址。
- 移除默认 `InsecureSkipVerify`；仅测试代码可通过显式 test/dev 配置跳过 Origin 校验。

#### 1.2 REST / WebSocket token

本地 Web terminal 应引入启动期 local token。

设计方向：

- Web server 启动时生成一次性随机 token，或从配置读取显式 token。
- 前端页面由同一个 server 提供时，可通过初始 HTML、配置 endpoint 或 cookie 获取 token。
- REST API 要求 token。
- WebSocket attach 要求 token。
- token 不写入 git，不写入公开静态资源。

规格决策：

- 后端增加统一 auth guard。
- Web server 启动时生成 32 字节以上随机 local token，编码为 URL-safe 字符串。
- 后端通过同源 bootstrap endpoint 暴露 token 给前端，例如 `GET /api/bootstrap`；该 endpoint 只允许通过 Origin 校验后的同源请求访问。
- REST API 使用 `Authorization: Bearer <token>`。
- WebSocket 使用 `?token=<token>` 携带 token，因为浏览器 WebSocket 不能设置自定义 header；服务端认证失败时拒绝升级，并且日志不得记录完整 token。
- 前端 API wrapper 与 WebSocket wrapper 统一从 bootstrap 状态读取 token。

#### 1.3 非 loopback 监听

非 loopback host 包括但不限于：

- `0.0.0.0`
- `::`
- 局域网 IP
- 公网 IP

设计方向：

- 非 loopback 监听必须启用 token。
- 如果 token 不可用，必须拒绝非 loopback 启动。
- 启动日志/CLI 输出必须明确提示当前 Web terminal 暴露范围。

#### 1.4 cwd policy

创建 session 的 cwd 默认限制在安全根内。

设计方向：

- 默认安全根为启动时 effective cwd。
- 可通过配置增加 workspace allowlist。
- 请求 cwd 必须解析为 absolute + clean path，并验证在 allowlist 根内。
- containment 判断必须基于 `filepath.Abs` + `filepath.EvalSymlinks` 后的路径；Windows 下比较时应采用大小写不敏感策略。

#### 1.5 command policy

本地 token 保护下允许灵活 command，以保留本地工具能力。

设计方向：

- loopback + token 场景允许任意 command，但仍需记录 command/cwd。
- 非 loopback 场景必须同时满足 token、Origin allowlist 和 cwd allowlist；command 仍允许任意本地命令，但 UI 和启动日志必须明确该模式具有远程执行风险。
- command resolution 继续由现有 process 层负责，但错误必须返回可理解 API error。

#### 1.6 env policy

本轮引入 env policy，但默认仍保持本地兼容性。

规格决策：

- 默认 env strategy 为 `inherit`，继续继承环境。
- 配置支持 `web.env.denylist`，按变量名 pattern 过滤；默认 denylist 为空，避免破坏 Claude/Codex/GitHub/AWS 等本地工具。
- 非 loopback 场景仍默认继承环境，但启动输出必须提示环境继承风险。
- session metadata 只记录 env strategy、denylist pattern 数量和 env count，不记录 env value。

### 2. WebSocket 写入模型

当前设计必须收敛为单 writer 模型。

设计方向：

- `webserver.serveWebSocket` 中只有 writer goroutine 可以调用 `conn.Write`。
- reader loop 不直接调用 `writeControl` 写 WebSocket。
- reader loop 处理错误、pong、resize error、write input error 时，只向 client outbound queue 投递 control message。
- close/detach/close session 需要通过 runtime/client 状态触发 writer 结束。

目标语义：

```text
reader goroutine
  read ws frame
  validate / execute runtime action
  enqueue outbound control if needed

writer goroutine
  read client outbound queue
  encode text/binary
  conn.Write
```

实现必须采用单 writer actor：reader loop、runtime 和 error handler 只能向 writer channel 投递 outbound item，不能直接写 WebSocket connection。

### 3. Terminal protocol

#### 3.1 `last_seq`

短期移除或隐藏 `last_seq`。

设计方向：

- TypeScript `ClientControlMessage` 不再暴露 `last_seq`。
- Go `ClientMessage.LastSeq` 可移除；如为兼容保留，也不在前端发送、不在文档宣称支持。
- hello 仅表示客户端已连接，可以触发 server-side started/snapshot/replay 行为。

#### 3.2 control message schema

前后端都需要更严格 schema 校验。

最小要求：

- `resize` 必须包含合法 cols/rows。
- `ping` nonce 长度受限。
- server `error` 必须包含 code/message。
- server `exited` 必须包含 exit_code/state。
- unknown type 明确拒绝。

#### 3.3 binary frame size

读取层需要限制 WebSocket message size。

设计方向：

- 使用 WebSocket 库的 Reader API 或等效受限读取方式，不能依赖读完整帧后再检查大小。
- text control frame 最大为 `terminalproto.MaxJSONMessageBytes`。
- binary input frame 最大为 `terminalproto.MaxBinaryFrameBytes`。
- 超限时发送明确 error control；如果无法可靠发送 error，则关闭连接并记录 reason。

### 4. Attach-time bounded history replay

本轮不做完整 seq replay，但要改善 running session 重新 attach 的上下文。

设计方向：

- running session attach 后，服务端按固定顺序发送 `started`、`replay_started`、bounded history snapshot、`replay_finished`，然后开始 live output。
- snapshot 使用当前 history 文件作为数据源；history writer 在读取 snapshot 前必须 flush 当前内存内容。
- snapshot 作为 binary 写入 xterm，使 ANSI escape 尽可能按 terminal 语义渲染。
- replay 期间新 live output 必须排在 replay 之后，避免 snapshot 与新输出乱序。

固定顺序：

```text
attach client
create writer channel but do not subscribe to live output yet
flush history writer
send control: started
send control: replay_started
send binary: bounded history
send control: replay_finished, include truncated=false until history metadata exists
subscribe client to live output
begin live output
```

本轮不实现准确 history truncation metadata；`replay_finished.truncated` 固定为 `false` 或省略，UI 显示“bounded history replay”。

### 5. Stopped session readonly xterm replay

stopped session 不应只依赖 `<pre>` 展示。

设计方向：

- 新增或复用 readonly terminal component。
- stopped session 加载 history 后写入 xterm。
- terminal 禁用 input forwarding。
- raw text view 可作为辅助诊断保留。
- history 过大时应考虑 chunked write，避免一次性阻塞前端。

### 6. Close / detach 语义

#### 6.1 detach

语义：断开当前 client，不关闭 session。

设计方向：

- 前端发送 detach 后等待服务端关闭或 ack，避免立即 close 导致消息丢失。
- 服务端收到 detach 后 detach client，并主动关闭 WebSocket 或发送 state/control。

#### 6.2 close session

语义：请求关闭 PTY session。

改为 REST close endpoint，原因：

- close 是关键状态变更，不应依赖 WebSocket send 后立即 close。
- REST 可复用 token/auth，错误处理更明确。
- WebSocket close control 可保留为辅助，但前端按钮优先调用 REST close。

设计方向：

- 增加 `POST /api/sessions/{id}/close`。
- close 成功后 runtime 广播 state/exited，并关闭 clients。
- 前端 close session 按钮只调用 REST close，不再发送 WebSocket `close` control。
- WebSocket `close` control 从客户端协议中移除。

### 7. 大输出、慢客户端与背压

本轮目标不是完整生产级 backpressure，但要消除明显粗糙语义。

设计方向：

- client queue 从纯 message-count 思维转向 byte-aware 或至少增加可观测性。
- queue full 时不要静默 detach；应尽量发送明确 error/close reason。
- writer close reason 应能传到前端 UI。
- 对慢客户端，detach 仍可作为本地阶段策略，但必须明确、可诊断、可恢复。
- attach-time bounded history replay 是 queue full 后恢复上下文的最低能力。

本轮采用以下策略：

1. client queue 继续保留 bounded channel，但增加 queued bytes 计数。
2. queue 满或 queued bytes 超限时，标记 detach reason 为 `client_queue_full`。
3. queue full 前尝试通过 writer priority path 发送一次 `error` control；如果无法发送，则使用 WebSocket close reason 和 server log 记录。
4. 连续 binary chunks 在进入 client queue 前允许合并，降低 message 数。
5. 必须记录日志：session id、client id、queue size、queued bytes、reason。

### 8. History writer 性能

当前每次 write 全量 flush 的行为对大输出不友好。

设计方向：

- 本轮纳入 batch/debounce flush。
- Writer 内存仍保持 bounded lines/bytes。
- flush 触发条件固定为：pending bytes 达到 64 KiB、距离上次 flush 超过 200ms、`Flush()` 被显式调用、`Close()` 被调用。
- `Close()` 必须强制 flush。
- `SessionRuntime` 在 attach-time replay 读取 history 前必须调用 `Flush()`。
- Writer 不引入长期后台 goroutine；debounce 可在 `Write()` 时基于时间判断。

### 9. xterm.js frontend

#### 9.1 初始尺寸

避免长期依赖固定 `120x32`。

设计方向：

- 前端在创建 session 前创建 xterm 实例并基于 terminal 容器计算 cols/rows。
- create session request 必须使用实际 fit 后的 cols/rows。
- 若容器不可测量，使用当前协议允许范围内的保守 fallback，并在 attach 后立即发送 resize。
- 本轮不引入 pending session lifecycle；只消除固定 `120x32`。

#### 9.2 xterm write queue

大量输出时不能无约束调用 `terminal.write(data)`。

设计方向：

- 使用 `terminal.write(data, callback)` 串行处理前端 write backlog。
- 维护 pending bytes 或 pending chunks 计数。
- 超过阈值时在 UI 中显示 slow terminal / output backlog 状态。

#### 9.3 输入编码

本轮保留 xterm `onData` 与 `onBinary` 双通道，但必须补测试。

规格决策：

- `onData` 使用 `TextEncoder` 发送 UTF-8 文本输入。
- `onBinary` 仅用于 xterm 标记为 binary 的输入，按低 8 位发送。
- 测试必须覆盖 ASCII、Unicode、Ctrl+C、方向键、paste、初步 IME。
- 如果测试发现重复发送，本轮实现必须删除重复路径，而不是保留已知错误。

#### 9.4 错误展示

server error 不应直接无过滤写入 terminal。

设计方向：

- terminal 内可以显示简短安全提示。
- 详细错误放 UI panel 或 toast。
- 写入 terminal 的 error message 需要转义控制字符，避免 escape sequence 混淆界面。

## Affected components

### Backend

- `internal/webserver/server.go`
  - WebSocket Accept Origin/token 校验。
  - REST auth guard。
  - WebSocket single-writer 重构。
  - close session endpoint。
  - read limit / frame size 限制。

- `internal/webterminal/registry.go`
  - session create cwd policy。
  - command/env policy 接入。
  - attach token 或 auth context 接入。
  - history read for replay。

- `internal/webterminal/runtime.go`
  - attach-time replay 顺序。
  - outbound control 统一入队。
  - queue full reason/error。
  - close/detach 状态语义。

- `internal/terminalproto/protocol.go`
  - 移除或隐藏 `LastSeq`。
  - 增强 client/server message validation。
  - 新增 `replay_started` / `replay_finished` 控制消息。

- `internal/history/writer.go`
  - batch/debounce flush。
  - Close 强制 flush。

- 配置相关模块
  - local token / auth 配置。
  - allowed origins。
  - cwd allowlist。
  - env denylist。

### Frontend

- `web/src/features/sessions/api.ts`
  - REST token/cookie 支持。
  - close session API。
  - history API 支持 readonly xterm replay。

- `web/src/features/sessions/useTerminalSocket.ts`
  - WebSocket auth token 支持。
  - 移除 `last_seq` 使用。
  - close/detach ack 或 server-driven close。
  - error/close reason 展示。

- `web/src/protocol/terminal.ts`
  - control schema 更新。
  - 移除或隐藏 `last_seq`。
  - 新增 replay 控制消息类型。

- `web/src/components/terminal/useXterm.ts`
  - write queue。
  - resize 初始化优化。
  - input behavior 验证后调整。

- `web/src/components/terminal/TerminalView.vue`
  - close/detach 语义调整。
  - replay 状态展示。

- `web/src/App.vue`
  - stopped session readonly xterm replay。
  - raw history 辅助视图。
  - create session cols/rows 改进。

## Interfaces

### REST API

#### Auth

所有 `/api/*` 默认需要 local token 保护，`GET /api/bootstrap` 除外。REST 请求使用 `Authorization: Bearer <token>`。WebSocket 使用 `?token=<token>`。所有受保护请求仍必须通过 Origin 校验。

#### Close session

新增：

```http
POST /api/sessions/{session_id}/close
```

该接口关闭 runtime / PTY，但不删除 session record、history 或 metadata。

### WebSocket control messages

Client to server：

```ts
type ClientControlMessage =
  | { type: 'hello' }
  | { type: 'resize'; cols: number; rows: number }
  | { type: 'detach' }
  | { type: 'ping'; nonce: string }
```

客户端 WebSocket 协议不再包含 `close`。前端 close session 只使用 REST close。

Server to client：

```ts
type ServerControlMessage =
  | { type: 'started'; session_id: string; workspace_id: string; state: 'running'; cwd: string; command: string[] }
  | { type: 'replay_started' }
  | { type: 'replay_finished'; truncated?: boolean }
  | { type: 'state'; state: string; attachment: string }
  | { type: 'exited'; exit_code: number; state: 'stopped' | 'failed' }
  | { type: 'error'; code: string; message: string }
  | { type: 'pong'; nonce: string }
```

`replay_started` / `replay_finished` 必须加入，以便前端区分历史写入和 live 输出状态。

## Resolved technical decisions

1. WebSocket 读取必须使用 Reader API 或等效受限读取方式，不能读完整帧后才做大小判断。
2. REST token 使用 `Authorization: Bearer <token>`；WebSocket token 使用 `?token=<token>`，服务端日志必须脱敏。
3. dev server Origin allowlist 固定支持同源后端 Origin 和配置中的 Vite dev Origin。
4. attach-time replay 固定在 live output 订阅前完成，顺序为 `started` → `replay_started` → history binary → `replay_finished` → live output。
5. history writer batch flush 使用 pending bytes、时间阈值、显式 `Flush()` 和 `Close()` 触发，测试可直接调用 `Flush()`。
6. cwd allowlist containment 使用 absolute + symlink-resolved path；Windows 比较大小写不敏感。
7. readonly xterm replay 新增独立 `HistoryTerminalView`，不复用带 WebSocket 输入语义的 `TerminalView`。
8. queue full 时通过 writer priority path 尝试发送最后 error；失败则依赖 WebSocket close reason 和 server log。
9. xterm write queue 以 pending bytes 为阈值，默认阈值为 4 MiB。
10. env denylist 默认不启用过滤，但配置入口必须实现。

## Alternatives

### Alternative A：完整 seq replay

优点：

- 协议语义完整。
- 适合未来 Gateway。
- reconnect 可恢复断线期间输出。

缺点：

- 实现复杂度更高。
- 需要 output envelope，不再是纯 binary PTY frame。
- 需要 ring buffer、seq、truncation、client last seen 状态。

本轮不采用，作为后续 Gateway / hardening 方向保留。

### Alternative B：仅实时 attach，不做 replay

优点：

- 实现最简单。

缺点：

- 用户刷新页面后上下文丢失。
- 大输出 detach 后不可恢复。
- 不符合本轮治理目标。

本轮不采用。

### Alternative C：继续 `<pre>` 展示 stopped history

优点：

- 无需新增 xterm readonly 组件。

缺点：

- ANSI/TUI 显示不真实。
- 无法验证 terminal 历史行为。

本轮不作为唯一展示方式，可作为 raw 辅助视图保留。

### Alternative D：默认过滤敏感 env

优点：

- 更安全。

缺点：

- 可能破坏 Claude/Codex/GitHub/AWS 等本地工具运行。

本轮实现 env policy 和 denylist 配置入口，但默认 denylist 为空，不强制过滤。

## Risks

1. token 机制如果设计不当，可能在日志、URL、浏览器历史中泄漏。
2. Origin 校验过严可能破坏 Vite dev proxy 或本地开发入口。
3. attach-time replay 如果与 live output 同时写入，可能造成输出乱序。
4. batch history flush 如果 Close 未强制 flush，可能丢失最后输出。
5. xterm write queue 如果实现过重，可能引入前端状态复杂度。
6. cwd allowlist 如果路径规范化不完整，可能被符号链接或 Windows 路径绕过。
7. REST close endpoint 如果没有和 runtime waitLoop 协调，可能产生重复 close 或状态竞争。
8. queue full error control 可能因为 queue 已满而无法送达，需要特殊处理或 close reason 补充。

## User review notes

本规格已按用户要求收敛，不保留待用户选择的尾巴。关键决策已固定为：

1. close session 使用 `POST /api/sessions/{id}/close`。
2. 本轮实现 attach-time bounded history replay，不做完整 seq replay。
3. 本轮移除或隐藏 `last_seq`。
4. stopped session 增加 readonly xterm replay，raw text 仅作为辅助诊断视图。
5. history writer batch flush 纳入本轮治理。
6. env policy 实现 denylist 配置入口，但默认 denylist 为空。
7. 后续修正：用户明确要求移除本轮临时 token/bootstrap/auth 机制，认证留给后续正式用户体系；本轮只保留 Origin allowlist、cwd allowlist 等本地边界。

暂无需要用户确认的未决事项。
