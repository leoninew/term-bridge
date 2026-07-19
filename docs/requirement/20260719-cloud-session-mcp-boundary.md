# 云端会话接入与 MCP 边界澄清

最后修改时间: 2026-07-19 18:31:00

Review status: Draft

## Background

TermBridge 的终端体验由浏览器 xterm.js 呈现，但 PTY 与 session 生命周期归 Agent 运行时所有。云端路径下，Browser 经 Gateway 访问设备上的 session，WebSocket 是 **已有 session 的 terminal attach 通道**，而不是「给定目录就即时开出裸 PTY」的入口。

在讨论「是否把当前 xterm.js 接入 MCP」时，容易把 **UI 渲染层** 与 **云端/设备会话控制面** 混为一谈。实际上：

- xterm.js 只负责渲染、输入与尺寸测量；
- 云端拿到的 WS 经 Gateway relay 到某台 online 设备上的 **既有 session PTY**；
- 会话的工作目录、启动命令在 **REST 创建 session** 时确定，不是 WS 握手参数。

本需求记录这一产品/架构理解，作为后续「外部集成（含 MCP）」与文档/API 表述的共同前提。当前阶段只澄清边界与验收意图，不进入 Spec / Plan / 实现。

## Goal

- 明确云端终端接入的正确调用序列与对象模型：认证 → 设备 → session → attach WS。
- 明确 **cwd / command 属于 create session**，**WS 属于 attach 已有 session**。
- 明确若做 MCP / 外部 AI 集成，**只连云端**（Cloud Session + Terminal attach），不做本地 stdio/本地 Agent 直连 MCP。
- 明确 MCP（或其它 AI/外部客户端）应集成 **TermBridge Cloud Session/Terminal API**，而不是浏览器 xterm.js 实例，也不是本地 xterm 进程。
- 明确 **每个 session 的 terminal attach 按单客户端写入口径**：不设计人机双写、不设计 MCP 与 Browser 同时写同一 session。
- 把上述理解固化为团队共享的需求文档，避免后续设计把「给目录开 PTY」或「本地 MCP」当成目标。
- 明确 MCP **终端/attach 能力** 的目标负载是 **常驻交互 shell**（cmd / bash / pwsh / Git Bash 等），不是「跑完即退」的一次性命令，也不是通用 TUI 遥控。
- 明确就绪/命令分界若需要自动化判定，优先 **PTY 输入 marker 探针 + idle/timeout**，禁止只写 xterm 画布、禁止默认跨 shell prompt 正则。
- 为后续可选的云端 MCP / 外部 client 工作划清非目标与安全边界输入。

## Non-goal

- 本轮不实现 MCP Server、不改造 xterm、不新增 Gateway/Agent 协议。
- **不做本地 MCP**（本地 stdio MCP、本机 Agent 旁路 MCP 均不在范围）。
- 不把 Browser 中的 xterm 组件改造成 MCP transport 或 tool 运行时。
- 不设计完整交互式 TUI 镜像（vim / htop / Claude TUI 像素级同步）的 AI 操作方案；**全屏 TUI 明确非 MCP 终端 MVP 目标**（难在尺寸竞态、控制序列、无可靠命令结束语义）。
- **不将「跑完即退」的一次性命令**（如 `go version`）作为 MCP terminal attach / 交互 `run_command` 的目标负载；此类需求若有，应另立 **oneshot/exec（等进程 exit）** 通道，而不是伪装成长寿终端会话。
- **不将跨 shell 的 prompt 检测**（cmd / PowerShell / Git Bash / Cygwin 等）承诺为可靠的「命令结束」判定。
- **不把仅向浏览器 xterm.js 本地 `write` 的内容**当作会话就绪或命令完成信号（本地写画布不进入 PTY）。
- **不设计同一 session 的双写 / 多写协作**（Browser 与 MCP 同时 attach 写入、多 AI 抢写等）。
- 不改变现有 cloud terminal control relay、history replay、attach 配额等既有行为（除非后续独立需求明确修改）。
- 不在本轮展开完整 tools/resources 清单的协议定稿（留给 Spec）；本需求只固定分层与心智模型。
- 不处理 `/code` workbench 或文件编辑能力接入 MCP。

## User scenarios

### 场景一：浏览器用户打开云端运行中会话

作为已登录云端用户，我选择一台 online 设备，在会话树中打开一个已存在且 running 的 session。

期望：

- 系统使用 cloud token 鉴权；
- 通过 `deviceId + workspaceId + sessionId` 组装 terminal WebSocket URL；
- WS 上完成 attach 后，按既有 terminal 协议收发二进制输出与控制消息（含 `started` / replay 等）；
- xterm 仅作为 UI，不创建新的 PTY 语义。

### 场景二：浏览器用户在指定目录新建会话

作为已登录云端用户，我在某设备的 workspace 下创建新会话，指定工作目录与启动命令（或 shortcut）。

期望：

- 通过 REST `createSession` 提交 `cwd`、`command` / `command_source`、workspace 等字段；
- Agent 在设备上创建 session 并启动 PTY；
- 随后再 attach WebSocket 进入终端；
- **目录不是 WS 参数**，而是 create 的一部分。

### 场景三：云端外部集成方（含未来 MCP）希望在设备上执行命令

作为外部客户端作者，我希望 AI 或自动化经 **云端** 在用户授权下使用设备上的终端能力。

期望：

- 客户端只走 **Cloud Session API + Terminal attach**，不连本地 Agent，不驱动用户浏览器里的 xterm；
- 需要交互时 **创建/使用自己的 session 并独占 attach**，而不是与用户当前已打开的 Browser 终端双写同一 session；
- 若目标 session 已被其它客户端 attach，产品语义按 **单 attach 写入口径** 处理（拒绝、顶替或要求用户先断开——具体策略留给 Spec，但不得假设可安全双写）；
- 鉴权、device online、workspace 边界与 attach 配额与现有云端路径一致；
- 不假设「认证 + cwd → 一条万能 PTY WS」，也不假设「本地 MCP 旁路云端」；
- create 的 command **面向交互 shell 族**（或明确指向 shell 的 shortcut），而不是任意 oneshot argv。

### 场景三-B：MCP 不应把 oneshot 当交互终端

作为 MCP 集成方，我若创建 `command = go version` 这类会立即退出的 session。

期望：

- 产品语义上 **不把它当作有意义的 MCP 交互终端会话**；
- attach / `run_command` 工具可拒绝或明确报错（「非交互/已退出会话」），引导使用 shell 类 create，或未来独立 oneshot API；
- Web UI 仍可继续支持 oneshot session（history 回放有价值）；MCP 可以比 Web **更严**。

### 场景三-C：就绪探测（若实现 run_command / ready）

作为 MCP，我需要在独占 shell session 上知道「可以注入下一条命令」。

期望：

- 通过 **terminal WS 向 PTY 写输入** 发送带唯一 nonce 的 marker 探针（例如 `echo __TB_READY_<nonce>__`），在输出流中等待 marker；
- **禁止** 仅 `xterm.write` 本地探测；
- **禁止** 仅依赖 create 后首段输出猜 prompt 作为唯一就绪条件；
- 创建成功 ≠ 一定已有输出；首段输出也 ≠ 可解析 prompt。

### 场景四：错误理解被纠正

作为维护者，我在设计文档或对话中描述云端终端时。

期望：

- 统一表述为「对 session 的 attach」，而不是「云端下发任意目录的裸 PTY」；
- 明确 Gateway 是 relay，PTY 所有权在 Agent runtime。

## Acceptance

- [ ] 需求文档清晰写出云端接入最小序列：`Auth → 选 online device → (可选) create session(cwd, command, …) → WS attach(session) → I/O/resize`。
- [ ] 需求明确：WebSocket URL 绑定 `device/workspace/session`，query 可含 `token`、`cols`、`rows`；**不含**「用 cwd 开会话」的语义。
- [ ] 需求明确：`CreateSessionReq` 至少包含 `cwd`、`command`、`command_source` 等创建期字段；目录在 create 阶段确定。
- [ ] 需求明确：MCP/外部集成的正确对象是 **云端** Session/Terminal API，不是 xterm.js，也不是本地 Agent MCP；xterm 保持人类 UI。
- [ ] 需求明确：Gateway 不拥有 PTY；Agent/runtime 为事实来源。
- [ ] 需求明确：每个 session 的 terminal attach 按 **单客户端写入口径**；不把「人机双写」列为能力。
- [ ] 需求列出云端 MCP/外部集成的主要风险输入：鉴权、任意命令执行面、attach 独占/顶替策略、输出截断/ANSI、device offline、并发 attach 配额。
- [ ] 非目标明确排除：本轮实现、本地 MCP、完整 TUI AI 操控、同 session 双写协作、跨 shell 可靠 prompt 检测。
- [ ] 明确 MCP terminal/attach 目标负载为 **交互 shell**；oneshot（如 `go version`）与 TUI 不在 attach 主路径。
- [ ] 文档说明独立 PAT 与浏览器 cloud token 的差异，以及 `run_command` 等待策略选项与默认约束（marker/idle/timeout/exit，而非 prompt；非仅写 xterm）。
- [ ] 文档区分：Web 可保留 oneshot session；MCP 可更严，仅接受 shell 类 create 或对已 exit 会话拒绝交互工具。
- [ ] 与既有云端终端 control relay、sessions 壳、terminalWsUrl 事实一致，不引入与代码相反的假设。

## Glossary / 术语说明

### 独立 PAT（Personal Access Token）

PAT 是给 **机器/外部客户端** 使用的访问令牌，与浏览器登录后的短期 cloud session token 不同。

| | 浏览器 cloud token | 独立 PAT（可选方向） |
|--|--|--|
| 使用者 | 人在 Web 登录 | MCP、脚本、CI、其它 AI host |
| 获取方式 | 登录 / OAuth 会话 | 用户在设置中生成并可吊销 |
| 寿命 | 通常较短，随会话/刷新 | 可较长，可限权、可撤销 |
| 形态 | Cookie / 短期 Bearer | 形如长期 API 密钥的 token |
| 典型场景 | 打开 Cloud Web | Cursor/Claude 等连接云端 MCP |

**当前状态**：产品尚未实现独立 PAT。Open questions 中「认证载体」是在问未来云端 MCP 起步时：

- **复用现有 cloud token**：实现快，但 AI 工具保管「登录态」风险大，过期/刷新更麻烦；
- **独立 PAT**：更适合自动化与限权吊销，但需要额外账号/令牌体系设计。

### MCP 目标会话类型（产品口径）

| 类型 | 例子 | 生命周期 | MCP 终端/attach 意义 |
|------|------|----------|----------------------|
| **交互 shell（主目标）** | `cmd` / `bash` / `pwsh` / Git Bash / Cygwin bash | 常驻 running，反复接受命令 | **MCP attach / 交互 `run_command` 的目标负载** |
| **一次性命令（oneshot）** | `go version`、短脚本、单次 `ls` | 输出后进程 exit | **不作为** MCP 交互终端目标；若需要，另立 **exec/oneshot**（等 exit 收 stdout） |
| **全屏 TUI** | vim / htop / Claude TUI 等 | 常驻但语义是 UI 应用 | **明确麻烦、非 MVP**；无通用「命令结束」；不做像素级 AI 操控承诺 |

原则：

1. MCP 终端能力 = **云端可鉴权的远程交互 shell 控制面**，不是「任意 PTY 的通用 TUI 遥控器」。
2. `go version` 一类 create **对 Web history 仍有意义**，但对 MCP attach「再跑命令」**意义很低**（会话已结束或即将结束）。
3. Web 与 MCP 的 create 策略可以不一致：Web 宽、MCP 严。

### 就绪与探测：PTY 探针 vs 只写 xterm

| 做法 | 是否进入 PTY | 能否判定 shell 就绪 |
|------|--------------|---------------------|
| 仅 `xterm.write(本地字符串)` | 否 | **否**（只改浏览器画布） |
| attach 后读 history/首段输出猜 `$`/`>` | 只读 | **不可靠**（非必有输出；形态多变） |
| 经 terminal WS `WriteInput` 注入 **唯一 nonce marker**，在输出中等待 | 是 | **推荐**（交互 shell 上 best-effort 就绪/分界） |
| 进程 exit | n/a | 仅适合 oneshot/exec，不适合长寿 shell 的「单条命令结束」 |

探针示例（概念，非协议定稿）：在独占 shell session 上发送 `echo __TB_READY_<nonce>__`（按 shell 族选择换行/转义），在 stdout 流中匹配完整 marker。可将 marker 前尾部当作 **弱 prompt 样本**，但不得单独作为完成条件。

### `run_command` 与等待策略

PTY 是持续字节流，不是「调用一次即返回」的 RPC。高层 tool（例如 `run_command(session, "ls")`）需要在 **write 命令之后** 决定何时把缓冲输出返回给 AI，这个「何时结束」即 **等待策略**。

常见策略：

| 策略 | 含义 | 特点 |
|------|------|------|
| 固定/总超时 | 最多等待 T 秒后返回当前缓冲 | 可预期；慢命令可能被截断 |
| 空闲超时（idle） | 输出停止 N ms 后视为告一段落 | 跨 shell 较稳妥的 MVP；中间停顿可能误切 |
| 进程退出 | 等子进程 exit | 适合会退出的任务；长寿交互 shell 本身不退出 |
| 显式结束标记 | 包装命令输出 `__TB_DONE__:code` 等 | 准确，但改变调用方式 |
| prompt 检测 | 匹配 `$` / `>` / `PS ...>` 等 | **跨 shell 不可靠**（见下） |
| 原始 attach | 只提供 write/read，由 AI 自行轮询 | 灵活但难用 |

**默认分层假设**：

- 第一期云端 MCP：至少 list/create/attach 与底层 write/read；create **默认/要求交互 shell 族**（或等价 shortcut）。
- `run_command` 类「在交互 shell 上注入命令 + 等输出再返回」可作为第二期；对象仍是 **shell session**，不是 oneshot 进程。
- 若实现 `run_command` / ready，**完成/就绪条件优先**：
  1. 显式 **marker 探针**（经 PTY 输入，非 xterm 本地写）；
  2. idle + max timeout 兜底；
  3. 仅 oneshot/exec 通道使用进程 exit。
- **不将跨 shell prompt 检测作为唯一或默认完成条件**。
- **不把 oneshot create + attach** 当作 `run_command` 的替代实现。

### 为何不做跨 shell 准确 prompt 检测

用户可能在同一产品中使用 cmd、PowerShell、Git Bash、Cygwin 等，提示符由本地主题/配置决定，没有统一「命令结束」协议。

主要困难：

1. **无标准帧**：PTY 只有字符流，没有 shell-agnostic 的 command-finished 事件。
2. **形态多变**：`C:\>`、`PS C:\>`、`user@host MINGW64 ~/work $`、Oh-My-Posh 等均可任意定制。
3. **假阳性/假阴性**：日志里的 `>`/`$`、多行 prompt、右 prompt、密码输入、pager、全屏 TUI（vim 等）都会误判。
4. **云端更难**：create 时 command 可能是任意 shell/shortcut；服务端通常不知道最终 prompt 形态。
5. **ANSI/CJK**：颜色与光标控制使基于行文本的正则更脆。

因此：**在免配置、覆盖全部用户 shell 的前提下，无法承诺 prompt 识别准确。**

可选增强（均属后续 Spec，不承诺本需求实现）：

- 受控 MCP session 固定 shell 并注入 OSC/唯一结束标记；
- 非交互执行通道（等进程 exit，而不是猜交互 prompt）；
- prompt 仅作 **弱信号**，必须与 idle/max timeout 组合，并文档化为 best-effort。

## Open questions

以下事项影响后续 Spec，需要用户确认或默认假设：

1. **云端 MCP 是否立项与排期**  
   本需求仅澄清边界。是否在下一阶段正式立项「云端 remote MCP」？  
   **已确认**：若做 MCP，只做云端，不做本地。  
   **默认假设**：未立项前只保留边界，不排期实现。

2. **外部客户端是否允许 create session**  
   历史 M6 文档曾限制 Gateway 仅 list/attach；当前产品 Web 已支持 cloud create。云端 MCP/外部客户端是否与 Browser 同权 create，还是仅 attach 已有 session？  
   **默认假设（可推翻）**：与当前 Cloud Web 对齐，允许在授权下 create；若安全策略收紧，可降为 attach-only。

3. **同一 session 已被 Browser attach 时，MCP 再 attach 的策略**  
   **已确认产品口径**：不做双写；每个 session 的 terminal attach 视为单客户端写入。  
   仍待 Spec 细化的是 **冲突时的工程行为**：拒绝第二个 attach、顶替旧 attach、还是要求显式 steal？  
   **默认假设**：拒绝第二个写入口 attach（最保守）；若实现层当前允许多 client fan-out，不得据此把双写写成产品能力。

4. **`run_command` 类能力是否进入第一期云端 MCP**  
   即「在交互 shell 上写一行命令 + 等待结束后返回输出」是否作为 MVP tool，还是只暴露原始 write/read attach。  
   **默认假设**：若做 MCP，MVP 至少包含 list/create/attach 与底层 I/O；`run_command` 等待策略第二期。  
   **已确认约束**：不做跨 shell 可靠 prompt 检测；若实现等待，优先 **marker 探针 + idle/max timeout**；oneshot 用独立 exec/exit，不塞进交互 `run_command`。

5. **认证载体**  
   云端外部客户端使用与 Browser 相同的 cloud token，还是 **独立 PAT**（见 Glossary）？  
   **默认假设**：复用现有 cloud auth 模型起步；PAT 作为更适合自动化的后续增强，本需求不实现。

6. **MCP Server 实现语言 / 类库**  
   用 Go 还是 Python FastMCP？  
   **当前认识（推荐默认）**：生产路径用 **Go**（官方 `modelcontextprotocol/go-sdk` 或社区 `mark3labs/mcp-go`），调用现有 Cloud Session/Terminal API；FastMCP 仅适合原型。  
   **未立项**：在用户明确进入 Spec/Plan 前不实现。

7. **MCP create 是否强制交互 shell**  
   Web 允许任意 command（含 `go version`）；MCP 是否只允许 shell 族 / 指定 shortcut？  
   **当前认识（推荐默认）**：**是**——MCP 终端工具只面向交互 shell；oneshot 另通道或直接拒绝。  
   **已确认方向（用户）**：oneshot 对 MCP 交互终端没意义；TUI 很麻烦，非目标。

## Decisions

- **分层决策**：xterm.js = UI；Session/PTY = Agent runtime；Gateway = auth + relay。MCP 若存在，作为与 Browser 同级的 **云端 API 客户端**，不挂在 xterm 上，也不走本地 Agent 旁路。
- **MCP 范围决策（用户确认）**：**只连云端**；本地 MCP 不在范围。
- **Attach 写入语义（用户确认）**：每个 session 的 terminal attach 按 **单客户端写入口径**。不存在「Browser 与 MCP 双写同一 session」的产品设计；MCP 应使用独立 session 或在独占 attach 前提下工作。
- **`run_command` 完成条件（讨论确认）**：不承诺跨 shell prompt 检测准确；不以 `$`/`>`/`PS` 等提示符匹配作为默认/唯一结束条件。优先 **PTY marker 探针**、idle + max timeout；进程 exit 仅用于 oneshot/exec 通道。
- **MCP 终端目标负载（用户确认）**：面向 **cmd/bash/pwsh 等交互 shell**；`go version` 类跑完即退的 session **不作为** MCP attach/`run_command` 有意义目标；TUI **非 MVP / 明确麻烦**。
- **oneshot 与交互分离**：一次性命令若需要 MCP 能力，应走 **exec/oneshot（等 exit）**，不要伪装成常驻终端 attach。
- **探测写入点（讨论确认）**：检测/就绪只能经 **terminal 输入 → PTY**；仅向 xterm.js 本地写入无效。
- **创建成功与首段输出**：create 成功不保证已有输出；首段输出不保证可解析 prompt；不得依赖「新建后一定有 prompt」。
- **认证载体说明**：独立 PAT 指面向 MCP/自动化的可吊销长期令牌，有别于浏览器登录 token；当前未实现，仅作后续选项说明。
- **会话创建 vs attach**：`cwd` / `command` 仅出现在 create（或等价 REST）路径；WS 只 attach 已有 `sessionId`。
- **云端 WS 心智模型**：  
  `wss://…/devices/{deviceId}/workspaces/{workspaceId}/sessions/{sessionId}/ws?token&cols&rows`  
  表示对某设备上某 session 的 terminal attach，不表示「按目录即时开 PTY」。
- **最小调用序列（规范表述）**：
  1. 认证，取得 cloud token（或等价凭证）
  2. 选择 online `deviceId`
  3. （可选）REST 创建 session：`cwd` + `command`/`command_source` + workspace 等
  4. 对 `sessionId` 建立 WebSocket attach，并携带初始 `cols`/`rows`
  5. 按 terminal 协议 `hello` / 二进制 I/O / `resize` / control 消息交互
- **与错误说法的对照**：
  | 简化说法 | 准确说法 |
  |----------|----------|
  | 云端给一个 WS 就是设备 PTY | WS 是对 **既有 session** 的 relay attach |
  | 认证 + 告诉目录即可 | 认证 + 设备 + create(含 cwd/command) + attach |
  | xterm 接入 MCP / 本地 MCP | **云端** Session API 接入 MCP；xterm 保持 UI；不做本地 MCP |
  | 人机双写同一终端 | **不支持**；单 attach 写入口径 |
- **实现备注（非产品能力）**：runtime 代码路径上 `SessionRuntime` 可维护 `clients` map 做输出 fan-out；产品需求仍按单写入口径描述，后续 Spec 须对齐「冲突 attach」策略，避免把实现细节误写成多写协作。
- **实现语言认识（未立项）**：若做云端 MCP Server，优先 **Go** + 官方/社区 MCP Go SDK，作为 Cloud Session/Terminal 的客户端适配层；不把 xterm 或本地 FastMCP 当主路径。
- **本轮交付物**：仅本 requirement 文档；不进入 Spec/Plan/实现，除非用户明确推进。


## Implementation language note（当前认识，未立项实现）

若后续推进「云端 MCP Server」实现，**推荐用 Go** 与 TermBridge 现有 Cloud/Agent 运行时同栈，直接复用现有 Session REST + Terminal attach 客户端逻辑与鉴权中间件；**不推荐**把 Python FastMCP 作为生产主路径（可用原型/对照实验，但不进主服务进程）。

### Go 生态（存在成熟类库）

| 库 | 说明 |
| --- | --- |
| 官方 `modelcontextprotocol/go-sdk` | 官方 Go SDK，优先评估 |
| 社区 `mark3labs/mcp-go` | 社区常用 MCP Go 实现，生态案例多 |

Python **FastMCP** DX 更好、与部分 LLM 实验栈集成更顺，但 TermBridge 生产 MCP 若挂在云端控制面，Go 更利于：

- 与现有 Gateway/Cloud 二进制、配置、鉴权、配额、审计同进程或同仓库集成；
- 避免再引入一套 Python 部署与会话生命周期；
- attach WS / 二进制 terminal 协议与现有 Go 客户端一致。

### 架构落点（认识）

```text
MCP Client (LLM host)
  → Cloud MCP Server (Go, 推荐)
    → Cloud Session REST（list/create/stop…）
    → Cloud Terminal WS attach（单写入口径）
      → Gateway relay → Agent PTY
```

- xterm.js **不参与** MCP。
- MCP **不**连本地 Agent stdio 旁路（本需求已确认只连云端）。
- MCP attach 受既有 **单 session 单写入口径** 与 **用户 attach 配额** 约束。
- 本文件仍是 **边界澄清 Draft**，不因上述认识自动进入 Spec/Plan/实现。

## Risk

- **安全**：把「任意 cwd + command」暴露给云端 MCP/外部客户端等于远程可控 shell；权限、确认与审计必须在后续 Spec 中一等公民对待。
- **文档漂移**：历史 M6「Gateway 不 create session」与当前 Cloud Web 可 create 并存，外部集成范围若未写清会产生错误 API 假设。
- **协议复杂度**：attach 后仍有 control 帧、`started`、replay、resize、配额；仅拿 WS URL 不够完成可用终端客户端。
- **attach 冲突**：用户 Browser 已占用 session 时，MCP 再 attach 必须有明确拒绝/顶替策略；若实现层曾允许多 client，文档与实现可能短暂不一致。
- **device offline**：Gateway 在线不等于 PTY 可用；外部客户端必须处理 offline / relay 断开。
- **等待策略误判**：若错误采用跨 shell prompt 检测作为完成条件，在 Windows 多 shell/主题下会高频假阳性/假阴性，导致 AI 截断输出或挂死等待。
- **范围蔓延**：若把完整 TUI 操控、可靠续传、本地 MCP、多写协作、通用 prompt 识别、oneshot 与交互 shell 混在同一 attach 模型里塞进「MCP 接入」会显著放大范围；本需求明确排除。
- **会话类型误用**：若 MCP 允许任意 create 而不区分 shell/oneshot/TUI，工具语义会混乱（exit 后仍 attach、TUI 下 marker 探针失败等）。

## User review notes

- 用户确认理解：云端 WS 实际对应设备侧某 session 的 PTY attach；创建会话时通过认证并指定目录（及命令等）开会话，而不是 WS 自身携带 cwd。
- 用户要求：严格模式记录需求，包含以上理解。
- 用户此前相关结论：MCP 应包 Session/PTY 能力，而非把当前 xterm.js 组件直接作为 MCP 中心。
- **2026-07-19 用户确认**：MCP 没有必要连本地，**肯定连云端**。
- **2026-07-19 用户确认**：WS 一次只能被一个客户端连接使用，**没有双写说法**；产品口径为单 attach 写入。
- **2026-07-19 用户追问并记录**：独立 PAT 的含义；`run_command` 等待策略的含义。
- **2026-07-19 用户确认约束**：跨 shell（git bash / cygwin / cmd / powershell）prompt 检测难以准确，不得作为可靠完成条件写入后续方案默认路径。

- **2026-07-19 认识更新**：讨论“推进实现是否用 Go MCP”。结论写入本文：Go 有官方/社区 MCP 库；相对 Python FastMCP，Go 更贴合 TermBridge 云端运行时。**仍不实现**，仅固化边界与技术倾向。
- **2026-07-19 关联已落地能力**：终端 keep-alive（前端 hot=4）与 attach 配额（后端默认 8）已上线；未来 MCP attach 同样计入/受制于用户 `terminal.concurrent_attaches`。

- **2026-07-19 用户确认**：`go version` 一类输出完就退出的 session，作为 MCP 交互终端 **没意义**；MCP 应针对 **cmd/bash 类交互式命令**；当前 TUI **很麻烦**，非目标。
- **2026-07-19 讨论固化**：新建成功 **不保证** 必有输出/可解析 prompt；若需就绪探测，应 **PTY 注入 marker**，不能只写 xterm.js。

