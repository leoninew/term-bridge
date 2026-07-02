# RuntimeEndpoint 与 local direct runtime path 需求

最后修改时间: 2026-07-02 13:13:31

Review status: Accepted

## Background

此前为了统一 local/cloud 模型，local mode 也复用了 tunnel transport 进入 `RuntimeAccess` / `terminal.Registry`。

当前需要重新审视该设计。已形成的共识是：

```text
业务模型统一、API 语义统一、UI 模型统一；
local 与 cloud 使用不同 transport 路径。
```

本任务处理第一组架构调整：抽象 `RuntimeEndpoint`，让 local mode 使用直接 runtime path，让 cloud mode 继续使用 tunnel runtime path。

## Goal

本任务目标是定义并实现以下架构方向：

```text
第一层：UI 模型统一
第二层：API 语义统一
第三层：Transport 传输解耦
```

核心改法：抽象 `RuntimeEndpoint`。

本阶段同时处理已确认的配置归属：当前进程 HTTP/API server 配置统一归入 `server` 节点，包含 `server.listen_url`、`server.mode`、`server.static_dir` 与 `server.public_url`；设备身份作为 runtime state 存放在 `<runtime.state_dir>/agent.json`；Cloud Gate 目标地址继续由 `cloud.gate_url` 表达；Agent Client 的连接目标由应用装配传入，远程连接使用 `cloud.gate_url`。

### LocalRuntimeEndpoint

local mode 下，本地浏览器操作本机 runtime 走直接 runtime endpoint：

```text
Browser
↓
Gateway Handler
↓
LocalRuntimeEndpoint
↓
RuntimeAccess
↓
terminal.Registry
```

### TunnelRuntimeEndpoint

cloud mode 下，远程设备访问继续通过 tunnel route：

```text
Browser
↓
Cloud Gateway Handler
↓
TunnelRuntimeEndpoint
↓
agentRoute.request / terminalRelay
↓
tunnel.Frame
↓
Agent Client
↓
RuntimeAccess
↓
terminal.Registry
```

### API 设计选择

本任务采用以下设计选择：

```text
local 使用直接 runtime API；
cloud 使用 device-scoped API。
```

即 local mode 不强制暴露或依赖 `/api/devices/{local_device_id}/...` 作为本机工作区/会话操作入口；cloud mode 保持以 device 为路由维度。

## Non-goal

本任务不处理以下内容：

```text
1. 不拆分仓库。
2. 不拆分二进制。
3. 不移除 Cloud Gate 的真实 agent tunnel。
4. 不重写 terminal.Registry / SessionRuntime / RuntimeStore 的核心行为。
5. 不引入完整 Relay 领域模型或权限策略引擎。
6. 不处理 cloud connector 启动策略、offline cache 策略、API 矩阵清理等剩余问题。
```

上述剩余问题进入任务 2。

## User scenarios

### 场景 1：local mode 下创建本机会话

用户打开本地 Dashboard，创建一个新 Session。

期望路径：

```text
Browser -> Gateway Handler -> LocalRuntimeEndpoint -> RuntimeAccess -> terminal.Registry -> SessionRuntime / PTY
```

### 场景 2：local mode 下 attach 本机会话

用户在本地 Dashboard attach 一个运行中的 Session。

期望：

```text
Gateway Handler 通过 LocalRuntimeEndpoint 直接拿到 TerminalStream。
输入、输出、resize、detach 仍按 terminal protocol 正常工作。
```

### 场景 3：cloud mode 下访问远程设备 workspace/session

用户打开 Cloud Dashboard，选择在线设备并查看 workspace tree 或创建 session。

期望路径仍然是：

```text
Cloud Gateway Handler -> TunnelRuntimeEndpoint -> agentRoute.request -> tunnel.Frame -> Agent Client -> RuntimeAccess -> terminal.Registry
```

### 场景 4：cloud mode 下 terminal relay

用户通过 Cloud Dashboard attach 远程设备上的 Session。

期望仍然使用：

```text
terminalRelay + tunnel.FrameTerminalAttach/Input/Output/Resize/Closed
```

本任务不破坏 cloud terminal relay。

## Acceptance

### A1. RuntimeEndpoint 抽象存在

系统中应出现一个明确的 runtime endpoint 抽象，用于表达 Gateway Handler 对目标 runtime 的操作入口。

它至少需要覆盖当前 workspace/session relay 需要的能力：

```text
workspace tree/list
workspace order/delete
session list/order/create/get/update/delete/close/rerun
history
terminal attach / ws bridge 所需能力
```

可以直接复用或包裹现有 `agent.RuntimeAccess`；Gateway 层以 runtime target adapter 表达调用入口。

### A2. LocalRuntimeEndpoint 存在并直接调用 RuntimeAccess

local mode 下应存在 `LocalRuntimeEndpoint` 或等价实现，直接持有并调用 `RuntimeAccess`。

验收重点：

```text
local 创建 session 走 `RuntimeAccess.CreateSession`。
local attach session 走 `RuntimeAccess.Attach`。
local history/workspace/session 操作走 `RuntimeAccess`。
```

### A3. TunnelRuntimeEndpoint 存在并保留当前 cloud tunnel 行为

cloud mode 下应存在 `TunnelRuntimeEndpoint` 或等价实现，内部继续使用当前 `agentRoute.request` 和 `terminalRelay` / tunnel frame 机制。

验收重点：

```text
cloud device-scoped workspace/session API 行为保持兼容。
cloud terminal relay 行为保持兼容。
agent.Client.handleRuntimeRequest 仍调用 RuntimeAccess。
```

### A4. local 使用直接 runtime API

local mode 下，本机 workspace/session API 应走 direct runtime path。

本任务选择：

```text
local: 直接 runtime API
cloud: device-scoped API
```

因此前端或 API adapter 需要能区分 local/cloud API 语义，但 UI 模型仍应尽量保持一致。

### A5. UI 模型保持统一

前端仍应以一致概念展示：

```text
workspace
session
terminal attach
history
```

local mode 使用 direct runtime route，cloud mode 使用 device-scoped route。

### A6. 现有 cloud tunnel 不被移除

真实 Cloud Gate 与 Local Agent 之间的：

```text
/api/agent/tunnel
hello/hello_ack
request/response
terminal_attach/input/output/resize/closed
```

必须保持可用。

### A7. 当前阶段配置归属清晰

本阶段需要完成已确认的配置组织调整：

```text
server.listen_url 表达当前进程 HTTP/API server 监听地址。
server.mode 表达当前进程运行模式。
server.static_dir 表达当前进程提供的 Web 静态资源目录。
server.public_url 表达浏览器访问当前 server 前端/setup 的地址。
cloud.gate_url 表达 Cloud Gate 目标地址。
设备身份生成并读取自 <runtime.state_dir>/agent.json。
```

验收重点：配置模型、默认配置、环境变量示例、应用装配和相关测试使用上述归属。

## Open questions

1. local direct runtime API 的最终路径是否沿用已有前端调用路径，还是新增 `/api/workspaces/...` 与 `/api/sessions/...` 形式？
   - 当前用户已指定“local 使用直接 runtime API，cloud 使用 device-scoped API”，但具体 URL 形态需要在 Spec 阶段落细。

2. 前端 API client 是在调用层区分 local/cloud，还是在 store/action 层做 adapter？
   - Requirement 阶段只要求 UI 模型统一，不指定具体实现位置。

3. `RuntimeEndpoint` 放在 `internal/application/gateway`、`internal/transport/http/gatewayapi`，还是其他模块？
   - Requirement 阶段只要求抽象存在，具体包边界留给 Spec 阶段。

4. local mode 下 `/api/devices` 是否仍展示本机 device？
   - 本任务聚焦 workspace/session runtime path；device 展示语义留给后续 API 矩阵整理。

## Decisions

1. 不拆仓库。
2. 不拆二进制。
3. 通过不同 `internal` 模块分清职责，仍然保留代码共用。
4. 模型统一建立在 `RuntimeAccess` / `RuntimeEndpoint` 这类内部 contract 上。
5. local 使用 direct runtime path。
6. cloud 保留 tunnel runtime path。
7. local 使用直接 runtime API，cloud 使用 device-scoped API。

## Risks

1. 前端 API 语义分叉后，local/cloud 调用层可能产生重复代码。
2. 如果 `RuntimeEndpoint` 抽象设计过厚，可能变成新的“大一统”中间层。
3. 如果 local direct path 和 cloud tunnel path 没有 contract tests，后续行为可能漂移。
4. 设备列表/selectedDeviceId 相关 UI 可能暴露旧的 online 状态假设。
5. Terminal WebSocket 的 local direct bridge 与 cloud tunnel bridge 需要谨慎拆分，避免输入/输出/resize/detach 行为不一致。

## User review notes

- 用户纠正：讨论的是同一仓库、同一项目内通过不同 `internal` 模块分离职责，不是拆分仓库。
- 用户确认任务 1 范围：三层统一/解耦、抽 `RuntimeEndpoint`、local direct path、cloud tunnel path、local 直接 runtime API、cloud device-scoped API。

## Pending user attention

当前仍需要用户后续关注的事项：

1. local direct runtime API 的具体 URL 形态需要在 Spec 阶段确认。
2. 前端 local/cloud API adapter 放置位置需要在 Spec 阶段确认。
3. `RuntimeEndpoint` 所属 internal 模块边界需要在 Spec 阶段确认。

以上事项不阻塞进入 Spec 阶段，但会影响具体方案。