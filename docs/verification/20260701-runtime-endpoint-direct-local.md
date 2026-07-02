# RuntimeEndpoint 与 local direct runtime path 验证

最后修改时间: 2026-07-02 13:32:16

Review status: Accepted

## Requirement alignment

对照 `docs/requirement/20260701-runtime-endpoint-direct-local.md`：

1. **RuntimeEndpoint 抽象存在**：已在 `internal/transport/http/gatewayapi/runtime_endpoint.go` 中新增 `runtimeEndpoint`，覆盖 JSON runtime 调用、history、terminal attach 与 available 状态。
2. **LocalRuntimeEndpoint 直接调用 RuntimeAccess**：`localRuntimeEndpoint` 持有 `agentapp.RuntimeAccess`，JSON 调用共享 `agentapp.HandleRuntimeRequest`，history 直接调用 `ReadHistory`，terminal attach 通过本地 websocket bridge 调用 `RuntimeAccess.Attach`。
3. **TunnelRuntimeEndpoint 保留 cloud tunnel 行为**：`tunnelRuntimeEndpoint` 继续包裹 `agentRoute`，JSON/history 走 `route.request`，terminal attach 委托现有 tunnel terminal relay。
4. **local 使用直接 runtime API**：local mode 新增 `/api/workspaces...` 与 `/api/sessions` 路由。
5. **UI 模型保持统一**：前端新增 `RuntimeTarget`，workspace/session store 仍维护统一的 workspace tree、session list、tab/history 模型。
6. **现有 cloud tunnel 保持可用**：真实 cloud tunnel `/api/agent/tunnel`、device-scoped API、`agentRoute`、terminal relay 均保留。
7. **cloud connector 使用 cloud.gate_url**：Cloud OAuth 完成后的 cloud connector 启动保留。
8. **当前阶段配置归属已落实**：HTTP/API server 配置统一使用 `server` 节点，包含 `server.listen_url`、`server.mode`、`server.static_dir` 与 `server.public_url`；设备身份改为 `<runtime.state_dir>/agent.json` state；远程 Agent Client 目标由 `cloud.gate_url` 装配。

结论：实现与 Requirement 对齐。

## Spec alignment

对照 `docs/spec/20260701-runtime-endpoint-direct-local.md`：

1. `RuntimeEndpoint` 按 Spec 放在 `gatewayapi` 包内，是 Gateway 侧 transport adapter，而非替代 `RuntimeAccess` 的新领域接口。
2. `RuntimeEndpoint` 保持薄接口：`JSON` / `History` / `Attach` / `Available`，未复制 `RuntimeAccess` 的全部方法。
3. direct path 与 tunnel path 共用现有 tunnel request method 与 DTO 映射：`agent.Client.handleRuntimeRequest` 已改为委托 `agent.HandleRuntimeRequest`。
4. local direct route 形态落地为 `/api/workspaces...` 与 `/api/sessions`。
5. cloud device-scoped API 保持 `/api/devices/{device_id}/...`。
6. 前端使用 `RuntimeTarget = { mode: 'local' } | { mode: 'cloud'; deviceId: string }`，避免 local mode 继续伪造 device-scoped URL。
7. `/sessions` 页面进一步移除了设备切换逻辑，改为在状态栏展示当前设备名称；该 UI 调整来自实现过程中用户补充要求。

结论：实现与 Spec 对齐；UI 状态栏抽取属于用户后续明确补充范围。

## Plan alignment

对照 `docs/plan/20260701-runtime-endpoint-direct-local.md`：

1. **Step 1**：已提取 `agent.HandleRuntimeRequest`，保持 method 名称、参数 DTO、响应 DTO 不变。
2. **Step 2**：已新增 `runtimeEndpoint`、`localRuntimeEndpoint`、`tunnelRuntimeEndpoint`。
3. **Step 3**：已新增 local direct terminal bridge `bridgeTerminalStream`，未复用 tunnel 语义的 `terminalRelay` 名称。
4. **Step 4**：`gatewayapi.Config` 已注入 `LocalRuntime`，`runServe` 传入本地 runtime。
5. **Step 5**：已新增 local direct runtime routes。
6. **Step 6**：cloud device-scoped handler 已迁移为通过 `tunnelRuntimeEndpoint` 适配现有 `agentRoute`。
7. **Step 7**：前端已引入 `RuntimeTarget` 与 `runtimePath`。
8. **Step 8**：gateway store 已提供 `runtimeTarget` 与 `currentDeviceName`。
9. **Step 9**：workspace/session 调用点已迁移到 `RuntimeTarget`，并对无 target 情况做保护。
10. **Step 10**：cloud connector 使用 `cloud.gate_url`，本地配置中的旧 tunnel 目标字段与相关配置测试已清理。

结论：Plan 中的实施步骤已完成。

## Actual diff summary

### Backend

- `internal/application/agent/client.go`
  - 将 runtime method switch 提取为共享 `HandleRuntimeRequest`。
  - 保持 tunnel `RequestReq` / `ResponseResp` 方法名与 DTO response shape。

- `internal/transport/http/gatewayapi/runtime_endpoint.go`
  - 新增 runtime endpoint adapter。
  - 新增 local direct runtime JSON/history/terminal attach 实现。
  - 新增 tunnel endpoint 包装，保留 cloud route/relay。

- `internal/transport/http/gatewayapi/server.go`
  - `Config` 增加 `LocalRuntime`。
  - Handler 持有 local runtime endpoint。
  - 新增 local `/api/workspaces...` 与 `/api/sessions` routes。
  - workspace/session route handler 从只接受 `*agentRoute` 泛化为接受 `runtimeEndpoint`。
  - JSON/history/no-content helpers 改为 runtime endpoint 版本，并保留 cloud offline cache 行为。

- `internal/app/app.go`
  - cloud connector 按 OAuth callback 后启动。
  - gateway config 注入 local runtime。

- `internal/infrastructure/config/config.go`
  - 新增 `server.listen_url`、`server.mode`、`server.static_dir`、`server.public_url` 配置模型与校验。
  - 将运行模式、静态资源目录和浏览器访问地址统一归入 `server` 配置节点。
  - `cloud.gate_url` 作为 cloud connector 目标。

- `internal/application/agent/device.go`
  - 设备身份生成并读取自 `<runtime.state_dir>/agent.json`。
  - 设备 key 仍按设备 ID 存放在 state 目录下。

- `configs/config.yaml`
  - 将当前进程 HTTP/API server 配置统一放到 `server` 节点。
  - 默认配置按 `server` 节点组织运行模式、静态资源目录和浏览器访问地址。

- `README.md`、`.env.example`、`Dockerfile`、`Dockerfile.cn`、`bin/package/TermBridge/*`
  - 更新环境变量示例和发布包配置为 `TERMBRIDGE_SERVER__LISTEN_URL`、`TERMBRIDGE_SERVER__MODE`、`TERMBRIDGE_SERVER__STATIC_DIR`、`TERMBRIDGE_SERVER__PUBLIC_URL`。
  - 更新设备身份说明为 state 文件。

- `internal/app/app_test.go`
  - 更新 serve 相关测试断言，覆盖 OAuth 后启动 cloud connector。
  - 测试配置改用 `server.listen_url`、`server.mode`、`server.public_url`。

- `internal/infrastructure/config/config_test.go`
  - 更新配置加载、环境覆盖与校验测试为 `server.listen_url`、`server.mode`、`server.static_dir`、`server.public_url`。
  - 覆盖 `server` 节点加载、环境覆盖和校验测试。

### Frontend

- `web/src/features/runtimeTarget.ts`
  - 新增 `RuntimeTarget` 与 `runtimePath`。

- `web/src/features/workspaces/api.ts`
  - workspace API 从 `deviceId` 参数迁移到 `RuntimeTarget`。

- `web/src/features/sessions/api.ts`
  - session API 与 terminal websocket URL 从 `deviceId` 参数迁移到 `RuntimeTarget`。

- `web/src/store/gateway.ts`
  - 新增 `runtimeTarget` 与 `currentDeviceName`。

- `web/src/store/workspaceSessions.ts`
  - refresh/reorder 接受 nullable `RuntimeTarget`。
  - `applyWorkspaceTree` 对非数组输入做防御，避免 workspace tree 状态形态错误。

- `web/src/store/workbench.ts`
  - history loading 接受 nullable `RuntimeTarget`。

- `web/src/views/SessionsView.vue`
  - session 操作调用 `gateway.runtimeTarget`。
  - 移除 `/sessions` 页面设备切换逻辑。

- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
  - 移除 selected device prop 与选择设备占位逻辑。

- `web/src/components/session/SessionWorkbench.vue`
  - 移除 selected device prop。
  - 接收当前设备名称。
  - 使用抽取后的 `SessionStatusBar`。

- `web/src/components/session/SessionStatusBar.vue`
  - 新增状态栏组件，接收当前 session 与 device name，自行组织状态显示。

### Tests

- `web/src/store/gateway.test.ts`
  - 增加 local/cloud `runtimeTarget` 与 `currentDeviceName` 断言。

- `web/src/store/workspaceSessions.test.ts`
  - 更新 reorder 调用与断言为 `RuntimeTarget`。

- `web/src/store/workbench.test.ts`
  - 更新 history loading 调用与断言为 `RuntimeTarget`。

## Expected vs actual changed files

### Plan expected backend files

- `internal/application/agent/client.go`：已修改。
- `internal/transport/http/gatewayapi/server.go`：已修改。
- `internal/transport/http/gatewayapi/runtime_endpoint.go`：已新增。
- `internal/app/app.go`：已修改。
- `internal/transport/http/gatewayapi/route.go`：未修改；实现选择是在 `server.go` 和 `runtime_endpoint.go` 中包裹现有 `agentRoute`，无需改 `route.go`。

### Plan expected frontend files

- `web/src/features/runtimeTarget.ts`：已新增。
- `web/src/features/workspaces/api.ts`：已修改。
- `web/src/features/sessions/api.ts`：已修改。
- `web/src/store/gateway.ts`：已修改。
- `web/src/store/workspaceSessions.ts`：已修改。
- `web/src/views/SessionsView.vue`：已修改。

### Additional changed files

- `internal/app/app_test.go`：更新 cloud connector 启动测试。
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`：按用户补充要求移除 `/sessions` 设备切换逻辑。
- `web/src/components/session/SessionWorkbench.vue`：按用户补充要求接入状态栏组件。
- `web/src/components/session/SessionStatusBar.vue`：按用户补充要求新增。
- `web/src/store/gateway.test.ts`、`web/src/store/workbench.test.ts`、`web/src/store/workspaceSessions.test.ts`：更新单元测试。

### Pre-existing unrelated/unverified docs in working tree

当前工作树还存在其他未跟踪文档，不属于本任务实现验证范围：

- `docs/analyze/20260701-core-abstractions-and-flows.md`
- `docs/analyze/20260701-grounded-uml-sequence-flow-diagrams.md`
- `docs/requirement/20260701-packaged-session-os-path.md`

这些文件未纳入本 Verification 结论。

## Acceptance checklist

- [x] A1. RuntimeEndpoint 抽象存在。
- [x] A2. LocalRuntimeEndpoint 存在并直接调用 RuntimeAccess。
- [x] A3. TunnelRuntimeEndpoint 存在并保留当前 cloud tunnel 行为。
- [x] A4. local 使用直接 runtime API。
- [x] A5. UI 模型保持统一。
- [x] A6. 现有 cloud tunnel 不被移除。
- [x] 用户补充：清理旧 local tunnel 目标配置和相关测试用例。
- [x] 用户补充：当前阶段配置归属迁移到 `server.listen_url`、`server.mode`、`server.static_dir`、`server.public_url`、state `agent.json`、`cloud.gate_url`。
- [x] 用户补充：`/sessions` 移除设备切换逻辑。
- [x] 用户补充：抽取状态栏组件，传入当前设备与会话后由组件自行组织显示。

## Command results

### Formatting

```text
gofmt -w "internal/app/app.go" "internal/app/app_test.go" "internal/infrastructure/config/config.go" "internal/infrastructure/config/config_test.go" "internal/transport/http/gatewayapi/server.go" "internal/transport/http/gatewayapi/server_test.go" "internal/transport/http/gatewayapi/cloud_binding_test.go"
```

结果：通过，无输出。

### Automated tests / checks

```text
go test ./...
```

结果：通过。

摘要：

```text
ok   termbridge-go/internal/app 5.570s
ok   termbridge-go/internal/infrastructure/config 1.479s
ok   termbridge-go/internal/transport/cli 2.465s
ok   termbridge-go/internal/transport/http/gatewayapi 0.267s
```

其余 Go package 为 cached 或 no test files。

```text
npm --prefix web run typecheck
```

结果：通过。

输出：

```text
> typecheck
> vue-tsc --noEmit
```

```text
npm --prefix web test
```

结果：通过。

输出：

```text
Test Files 10 passed (10)
Tests 44 passed (44)
```

```text
git diff --check
```

结果：通过，无输出。

## Missed or expanded scope

### Expanded scope

以下内容是用户在 Implementation 阶段明确补充要求，因此纳入本次实现：

1. `/sessions` 页面移除设备切换逻辑。
2. 只在右下角状态栏右侧显示当前设备名称。
3. 抽取状态栏组件，由组件接收当前设备和当前会话并自行组织显示。

### Not fully covered by automated tests

1. 未进行真实浏览器手工验证：未实际启动本地 UI 创建/attach session、输入命令、resize/detach/close/rerun。
2. local direct terminal websocket bridge 的双向细节依赖现有 Go test 覆盖与编译校验，未在本次新增独立 websocket 行为测试。
3. cloud 真实远程 tunnel 端到端未连接线上环境验证；本次以现有自动化测试与代码路径保持为准。

## Risks

1. `RuntimeEndpoint.JSON` 仍使用 method string，符合复用现有 Req/Resp 规约的目标，但编译期类型安全有限。
2. local direct terminal bridge 与 cloud tunnel terminal relay 是两条实现路径，后续应补充更细粒度的 websocket 行为测试，防止 ping/resize/detach 漂移。
3. 当前 `go test ./...` 会遍历到 `web/node_modules/flatted/golang/pkg/flatted`，虽然本次通过，但后续若 node_modules 内容变化可能影响 Go 包扫描边界。
4. 工作树中存在本任务之外的未跟踪文档，提交前需要用户自行确认是否与本任务一起提交或拆分。

## Incomplete items

1. 未完成浏览器人工验证。
2. 未新增专门覆盖 local direct terminal websocket 的细粒度测试。
3. 未处理任务 2 范围内的 offline cache 边界、local device online 语义等问题。

## Conclusion

本次实现完成了任务 1 的核心目标：Gateway 层通过 `RuntimeEndpoint` 解耦 local direct runtime path 与 cloud tunnel runtime path；cloud tunnel 行为保留；前端通过 `RuntimeTarget` 统一 workspace/session UI 模型并分流 local/cloud API；`/sessions` 的设备切换逻辑已移除并抽取状态栏组件。

自动化验证结果全部通过：

```text
go test ./...
npm --prefix web run typecheck
npm --prefix web test
```

交付状态：可进入人工浏览器验收；提交前建议用户检查并拆分当前工作树中与本任务无关的未跟踪文档。
