# 共享工作区 VS Code Online 工作台需求评估
最后修改时间: 2026-07-15 07:30:19

Review status: Accepted

## Background

TermBridge 当前以 `User → Device → Workspace → Session → Terminal` 为核心模型：工作区与会话实际运行在 Agent 所在设备，Web 在本地模式直连 Agent、在云端模式经 Cloud Gate 与 Agent tunnel 访问目标设备。前端已经提供会话工作台、工作区—会话侧栏和 xterm 终端，但工作区当前只表达为会话分组，未提供目录、文件或文本内容能力。

用户指出仅集成 Monaco Editor 无法自然覆盖完整目录 Explorer、Git 文件变更、集成终端、调试和扩展等 Cloud IDE 工作台能力。因此，目标调整为：在工作区目录节点增加 VS Code Online 入口，并将会话工作台的主要内容区切换到完整 IDE，而非在 xterm 旁叠加一个文本展示组件。

当前不存在 `SharedWorkspace`、工作区成员关系、邀请、工作区角色、IDE provider、IDE URL 或通用 IDE HTTP/WebSocket tunnel。Cloud 当前也缺少对按 device ID 路由的工作区/会话代理统一执行设备归属/角色校验的边界。安全与细粒度共享授权不阻塞本期可行性 PoC，但在生产交付前必须补齐；现有“同一设备可被访问”的能力不能视为可安全交付的共享工作区能力。

## Goal

1. 在工作区目录节点的操作区增加显式“Open IDE”入口；保留节点点击的展开/收起语义，不将打开 IDE 隐含在目录节点的单击行为中。
2. 使用自托管、完整的 VS Code Online 兼容工作台（PoC 首选 code-server）打开 Agent 已登记的 workspace，使用户获得 Explorer、文件变更、编辑器、集成终端、任务、调试与受控扩展等完整 IDE 能力。
3. 进入 IDE 后，将当前 Sessions workbench 的主内容区切换为 IDE 页面；PoC 可使用同源 iframe 验证体验，但生产默认支持独立同源 IDE 路由和“新标签页打开”回退。
4. IDE provider 运行在目标 Agent 所在设备，只监听 loopback；浏览器在本地模式经 Agent、在云端模式经 Cloud Gate 与 Agent outbound tunnel 访问 IDE，不能直接访问 Agent 本地端口或任意文件系统路径。
5. 保持 TermBridge Session 与 IDE 内置终端为两个独立资源模型：工作区节点负责 IDE 入口，session 节点继续打开 TermBridge 管理的 xterm session；首期不承诺将 IDE 内创建的终端、进程或 history 纳入 Session 生命周期。
6. 本需求先完成技术可行性 PoC；工作区成员 ACL、细粒度文件限制、IDE terminal 权限与生产隔离在 PoC 之后进入生产化设计，不能被误认为已经由现有设备访问模型解决。

## Non-goal

- 不采用 Microsoft 托管 `vscode.dev` / VS Code for the Web 作为 TermBridge 的完整 IDE provider；它不能直接提供 Agent Windows 工作目录、集成终端与完整服务器端扩展能力。
- 不采用 Microsoft Remote Tunnels / VS Code Server 作为面向 TermBridge 用户的嵌入式或集成 IDE 服务；其外部身份与服务依赖不属于 TermBridge 控制面，且许可边界不适合该产品化用途。
- 首期不自建或长期维护 Code - OSS Web Server 发行版、扩展宿主、Extension Gallery 与上游补丁体系；如 code-server PoC 不满足 Windows 支持矩阵，需另立需求重新评估。
- 首期不提供多人同文档实时协作、共同光标、presence、CRDT/OT、离线编辑或协作内容持久化。
- 首期不将 IDE 终端、任务、调试器产生的进程/history 自动纳入现有 Session 的 close、rerun、history replay、排序或审计语义。
- 首期不将 Cloud Gate 实现成可代理 Agent 任意 host、端口或 TCP 服务的通用隧道；IDE relay 只能面向 Agent 已启动且已绑定 workspace 的固定 provider。
- 首期不承诺 iframe 在所有浏览器、扩展和企业策略中可用；独立 IDE 路由与新窗口打开必须是可用回退。

## User scenarios

1. 用户在工作区目录行的操作区点击 Open IDE；目录行本身仍只执行展开或收起，现有新建/删除会话操作不回归。
2. 系统根据当前 local 或 cloud runtime target、device 和 workspace 解析一个受控 IDE 入口；浏览器进入独立 IDE 页面，或在 PoC 中以内嵌 pane 显示同源 IDE。
3. IDE 打开已登记 workspace 后，用户可使用完整 Explorer 浏览目录、在 Source Control 中查看文件变更、编辑/保存文件，并使用 IDE 内置 terminal、任务、调试器和符合 provider 策略的扩展。
4. 用户从 IDE 返回 sessions 页面后，已打开的 TermBridge session tabs、各 session 的 running/stopped 状态和既有 history 行为保持不变；IDE 内终端不伪装为 TermBridge session。
5. Agent 断开、IDE provider 未安装、启动失败、健康检查失败、Cloud relay 不可用或 iframe 因策略不可嵌入时，用户得到明确状态，并能使用独立页/新窗口回退或继续使用原有 sessions workbench。
6. PoC 在 Windows Agent 上以固定 Node 与 code-server 版本验证：可打开 workspace、编辑保存文件、启动 PowerShell/pwsh 终端、加载至少一个常用 Go/Node 扩展和一个 Webview 扩展，并经 Cloud Gate 完成刷新与 WebSocket 工作流。
7. 未打开 IDE 的 workspace 与 session 继续按现有方式运行；新增 IDE 不改变现有 PTY 的 attach、history replay、rerun、编辑或删除语义。

## Acceptance

- [ ] workspace 目录节点的操作区有独立 Open IDE 入口；入口不会改变工作区节点的展开/收起语义，也不破坏新建 session、删除工作区、session 选择和现有拖拽操作。
- [ ] IDE 使用经验证的完整 provider（PoC 为固定版本 code-server），可在目标 Agent 已登记 workspace 中展示 Explorer、文件变更、编辑器、集成 terminal、任务、调试器和受支持扩展；不以 Monaco 自建上述工作台能力。
- [ ] IDE provider 由 Agent 基于 workspace ID 启动或复用，只监听 loopback；浏览器不接收 Agent 本地端口、provider 密码、Agent 设备凭据或可任意指定的本地路径/上游地址。
- [ ] local 模式和 cloud 模式都能从工作区入口获取受控 IDE URL/状态；Cloud 模式通过独立 IDE HTTP/WebSocket relay 访问 Agent provider，不复用 terminal WebSocket 或把 Cloud Gate 变成任意端口代理。
- [ ] IDE 代理正确支持 HTTP request/response、流式 body、WebSocket Upgrade 和双向帧；同时对 route 前缀、上游 host、并发、超时、负载、背压和关闭语义执行限制。
- [ ] 浏览器能以同源独立 IDE 路由完成工作区打开、刷新、静态资源、worker、WebSocket、IDE 登录/回调所需状态；iframe 仅在兼容时作为工作台嵌入方式，并始终提供新窗口回退。
- [ ] Windows Agent 的 PoC 明确锁定并验证 Node、code-server、PowerShell/pwsh、Go/Node 扩展和 Webview 扩展版本组合；provider 启停、端口冲突、崩溃、Agent 断连和健康检查均给出可恢复的状态。
- [ ] IDE 打开/关闭与 TermBridge Session 的生命周期边界明确：IDE 内置 terminal/任务/调试器不改变或伪造 Session 的 history、close、rerun 和 writer lock 行为。
- [ ] 共享工作区的生产化设计补充 workspace identity、成员角色、设备/工作区操作授权、路径根约束、IDE terminal 权限、审计和撤销后的会话失效；PoC 只在受控测试账户、设备和 workspace 范围内验证，不能作为生产安全结论。
- [ ] 覆盖 workspace 入口、local/cloud 路由、provider 生命周期、Agent/Cloud IDE relay、WebSocket、iframe/新窗口回退、现有 sessions/xterm 回归，以及 Windows PoC 支持矩阵的关键测试。

## Open questions

1. **Provider 选择与 Windows 支持。** 是否确认以 code-server 作为 PoC provider，并接受 Windows Agent 通过受控 Node/npm 安装运行（code-server 没有官方 Windows release binary）？若不接受，应评估 WSL2/Docker 或另立 Code - OSS 路线。
2. **工作台呈现。** “将终端界面切换成 VS Code Online”是指点击工作区后进入独立 IDE 路由，还是要求 IDE 成为现有 session tab strip 中的一种 tab？建议首期用独立路由，避免破坏 session tab、history 和终端状态模型；iframe 只作可选嵌入体验。
3. **IDE 生命周期。** provider 是每个 workspace 独立一个实例、每个 Agent 一个共享实例，还是按用户/共享 workspace 创建？建议 PoC 每 workspace 一个受控实例，以简化根目录和故障隔离；生产再根据资源成本调整。
4. **共享范围。** PoC 是否限定一个测试账户、一个 device 和一个 workspace，先验证完整 IDE 技术链路；生产才引入 Cloud workspace identity、成员关系和角色？
5. **IDE 与现有 Session 的关系。** 是否确认 IDE 内置 terminal、task 和 debugger 的进程不纳入 TermBridge Session/history，并继续由 session 节点打开当前 xterm？若要求统一管理，需要单独重构 runtime ownership。
6. **扩展策略。** PoC 是否只使用 Open VSX/本地 VSIX，且限定验证的 Go/Node 与 Webview 扩展？生产是否需要私有 Extension Gallery、allowlist 或禁止安装扩展？
7. **访问方式。** 云端 PoC 是否必须完成同源 Cloud Gate relay，还是先允许本地 Agent 的独立 IDE 页面验证 provider，再实现 Cloud HTTP/WebSocket relay？建议分两步，避免把 provider 兼容问题与 tunnel 代理问题混在同一排查面。

## Decisions

- 本需求采用 strict / 严格模式，因为它跨 Agent process provider、Cloud tunnel、HTTP/WebSocket relay、前端工作台、Windows 运行支持和后续共享授权模型。
- 用户已明确完整目录 Explorer 与文件变更属于首要体验，Monaco 仅作为编辑器控件无法独立满足目标；需求转向完整 VS Code Online 工作台。
- PoC 的候选 provider 为 code-server：它以 MIT 许可提供自托管完整 IDE 能力，且比自建 Code - OSS 发行版更适合验证。其 Windows 场景必须作为正式 PoC 条件验证，不能假定与现有 Go Agent 的 Windows 支持等价。
- 不采用 VS Code for the Web：它没有完整 server-side terminal/debugger/extension host，不能自然打开 Agent 真实 workspace；不采用 Remote Tunnels/VS Code Server 面向 TermBridge 用户提供集成服务。
- 工作区行维持点击展开/收起；Open IDE 作为独立操作区入口。推荐独立 IDE 路由作为稳定交付面，iframe 是经过 CSP、Cookie、WebSocket、Webview 和浏览器策略验证后才启用的体验优化，并始终保留新窗口回退。
- Cloud 模式的 IDE 访问必须新建固定 provider 范围的 HTTP/WebSocket relay；不将 code-server 流量塞入 terminal frame，也不提供 Agent 任意端口转发。
- IDE 内置 terminal 和现有 Session terminal 是不同 runtime ownership；首期并存而非互相映射。后续若要统一，必须另立 Session/runtime 模型重构。
- 安全、成员 ACL 与运行隔离不作为阻塞 PoC 的前置实施范围，但必须在需求中保留为生产化硬门槛，不能因 PoC 成功而默认豁免。

## Risk

- **PoC 与生产的边界。** 用户要求安全问题后置可缩小首轮范围，但 code-server 的 terminal、task、debugger 和扩展本质上会以 Windows Agent 用户身份运行代码。PoC 必须限制为测试设备/workspace/account；生产不得以 PoC relay 直接对共享用户开放。
- **Windows provider 支持。** TermBridge 已验证 Windows Agent/PTy，但 code-server 没有官方 Windows release binary；Node/npm 路线需实测安装、版本锁定、PowerShell/pwsh、端口、AV/EDR、重启和卸载，不能只依据 Linux 文档推断可交付。
- **Cloud tunnel 能力缺口。** 当前 Cloud tunnel 是 typed runtime RPC 加 terminal relay，而不是任意 HTTP/WebSocket proxy。IDE 将要求新协议、流式转发、Upgrade、背压、超时、取消和上游限制；这是决定云端可行性的主要工程量。
- **iframe 兼容性。** CSP `frame-ancestors`、`X-Frame-Options`、Cookie 的 Path/SameSite、WebSocket Origin、worker/service worker、webview 和焦点/快捷键都可能使 iframe 失败；必须优先验证独立同源页，并保留新窗口打开。
- **路径与 provider root。** Workspace `Path` 是 Agent 本机路径；provider 只能由 Agent 基于 workspace ID 启动，不可接受浏览器传入的目录、启动参数、端口、扩展目录或上游 URL。生产仍需处理 canonical path、符号链接和 Windows 路径语义。
- **双终端模型。** IDE 的终端/任务与 TermBridge Session terminal 并存会造成进程、历史和审计分裂；若产品要求统一关闭、rerun、history 或 writer control，不能通过 UI 嵌入解决，需重构 runtime ownership。
- **扩展供应链。** code-server 不应默认使用 Microsoft Marketplace；Open VSX、私有 gallery 或受控 VSIX 的来源、升级、允许列表与漏洞响应是生产化必要项。
- **现有 Cloud 授权缺口。** 当前按 device ID 路由的 workspace/session/terminal 访问未见统一 device ownership check；IDE relay 不能复制这一行为。共享 workspace 的成员、角色、撤销和每次请求授权需要在生产阶段补齐。

## User review notes

- 2026-07-14：用户提出“了解工作区和会话的设计”，并希望对共享工作区会话提供“基于 VS Code 编辑器的文本展示、类似 Cloud IDE”的能力进行需求评估。
- 2026-07-15：用户指出 Monaco 不覆盖目录 Explorer 与文件变更，要求评估在工作区目录节点添加入口、将终端界面切换至 VS Code Online 的可行性，并要求安全问题后置。
- 2026-07-15：Requirement 已转为完整 VS Code Online 工作台的技术可行性 PoC；候选方案为 Windows Agent 本地 code-server + local/Cloud IDE relay。安全、成员 ACL 和隔离保留为生产化硬门槛，尚未进入 Spec / 规格或 Plan / 计划，未修改产品代码。
- 2026-07-15：用户确认以 `code-server` 通过 Windows Node/npm 路线作为 PoC provider；首期仅验证本地 Agent IDE，不实现 Cloud Gate relay。
- 2026-07-15：用户确认 IDE 从工作区入口打开并覆盖整个右侧会话区域；不创建 IDE tab，也不保留右侧 session tab strip。再次从其他工作区打开 IDE 时，右侧直接切换到新的工作区 IDE。
- 2026-07-15：用户确认 IDE 内 terminal 与 TermBridge Session terminal 独立；本期遵循最小引入，重点验证完整目录 Explorer 与文本能力。用户明确要求进入 Spec / 规格阶段，因此 Requirement 已接受。
