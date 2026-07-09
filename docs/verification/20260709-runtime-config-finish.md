# runtime config 与 Home 入口收尾验证
最后修改时间: 2026-07-09 11:10:35

Flow mode: light / 轻量模式
Stage: Verification / 验证
Review status: Accepted

## Requirement alignment

按 `docs/requirement/20260709-runtime-config-finish.md` 核对：

1. Home 入口已拆分为 `AgentHome.vue` 和 `CloudHome.vue`，`HomeView.vue` 只负责按 `runtimeConfig.view.mode` 分发。
2. Agent Home 保留本机工作台、工作区列表、连接/断开 Cloud、hybrid 下切换 Cloud 模式。
3. Agent Home 的本机设备信息来自 Agent `auth/me` 返回的 `device`，不再从 Cloud session 取本机设备名。
4. Cloud Home 只作为入口页，不展示业务状态卡片，不展示用户私有设备列表。
5. Cloud Home 不再为了首页入口拉取设备列表，也不直接跳转到设备 sessions；已登录入口只进入 `/dashboard`。
6. `AgentHome.vue` 未直接调用 `cloud-api` / `features/cloud/api`；`CloudHome.vue` 未直接调用 `agent-api` / `features/agent/api`。
7. Cloud Dashboard `/dashboard` 继续负责设备列表、在线/离线状态和进入设备工作台。
8. Cloud Dashboard 离线设备右侧只保留不可用 arrow-right icon，不展示“设备离线→”文字按钮。
9. 设备列表卡片右侧保留返回首页入口。
10. 卡片 title 右上角操作已改成 text action / title link 样式，不使用边框、背景或蓝色实心按钮。
11. Agent sessions 路径已从 `/agent/sessions` 改为 `/sessions`，不保留旧路径兼容；route name 仍为 `agent-sessions`。
12. `/sessions` 和 `/devices/:deviceId/sessions` 继续使用现有 sessions 页面能力。
13. 前端 generated proto 目录和后端 generated proto 目录已纳入格式化/检查排除配置；本轮不继续处理 gen 输出格式。
14. 前端 env mode 文件收敛为 `web/.env.development`、`web/.env.agent`、`web/.env.cloud`；`web/.env.production` 不再保留。
15. 根目录 `.env` / `.env.<env>` 保持 machine-local 覆盖文件语义并默认忽略。

## Spec alignment

不适用。light / 轻量模式未创建单独 Spec / 规格文档，按 Requirement / 需求核对。

## Plan alignment

不适用。light / 轻量模式未创建单独 Plan / 计划文档，按 Requirement / 需求和用户追加指令核对。

## Actual diff summary

当前 diff 覆盖以下主要范围：

- 文档：合并 runtime config / Home / env mode requirement；新增本 verification；旧 Home Cloud verification 保留为历史记录。
- 配置：更新 `.gitignore`、`.golangci.yml`、`.env.example`、`web/.prettierignore`、`web/.env.*`、Dockerfile 构建入口、README。
- Proto/API：`AuthMeResp` 增加 `device`，Agent `auth/me` 返回本机 device summary。
- Agent 后端：Cloud connect/disconnect、本机 cloud session、本机设备 identity 相关调整和测试。
- 前端 runtime config：移除旧 `appMode`，新增 `runtimeConfig` store 和测试，收敛配置读取边界。
- Home 前端：`HomeView.vue` 简化为模式分发，新增 `AgentHome.vue` 和 `CloudHome.vue`。
- Cloud Home：移除设备列表拉取、设备入口直跳 sessions 和“进入 xxxx 工作台”逻辑，已登录入口回到 `/dashboard`。
- Dashboard：设备列表右侧离线 arrow-right 不可用；卡片 title 右上角操作改为 text action。
- 路由：Agent sessions 从 `/agent/sessions` 改为 `/sessions`，不保留旧路径。
- Generated code：proto 生成物随字段增加变化；按用户要求不继续处理格式化细节。

## Expected vs actual changed files

| 范围 | 预期 | 实际 | 结论 |
| --- | --- | --- | --- |
| Home 拆分 | Agent / Cloud Home 分离，HomeView 只分发 | 已新增 `AgentHome.vue`、`CloudHome.vue`，`HomeView.vue` 只按模式渲染 | 符合 |
| AgentHome API 边界 | 不直接调用 Cloud API | grep 未发现 `features/cloud/api`、`cloudApiClient`、`/cloud-api` | 符合 |
| CloudHome API 边界 | 不调用 Agent API，不拉设备列表 | grep 未发现 `features/agent/api`、`agentApiClient`、`/agent-api`、`loadDevices`、`openCloudSessions` | 符合 |
| Cloud Home 入口 | 已登录只进入 `/dashboard` | 已移除设备选择/直跳 sessions 逻辑 | 符合 |
| Agent sessions 路径 | `/sessions`，无旧路径兼容 | router path 已改，`web/src` 下无 `/agent/sessions` 残留 | 符合 |
| 卡片 title action | 右上角无边框、无背景、非蓝色实心 | `AgentHome` 和 `DashboardView` title actions 已改为 muted text action | 符合 |
| Dashboard 设备入口 | 离线只显示不可用 arrow-right icon | 设备项右侧为无边框 icon button，离线 disabled | 符合 |
| 本机设备来源 | Agent device identity，不取 Cloud | Agent `auth/me` 返回 `device`，Home 使用 `me.device` | 符合 |
| Env 文件 | 前端 mode 文件收敛，root env 忽略 | `.env.agent` / `.env.cloud` / `.env.development` 与 `.gitignore` 已调整 | 符合 |
| Generated 目录 | 前后端排除格式化/检查 | `.golangci.yml` 排除 `internal/gen/proto/`，`web/.prettierignore` 排除 `src/gen` | 符合 |

## Acceptance checklist

- [x] `HomeView.vue` 不再混放 agent/cloud 两套页面主体。
- [x] `AgentHome.vue` 保留 agent 模式入口能力。
- [x] `CloudHome.vue` 保留 cloud 模式入口能力。
- [x] Cloud Home 不展示业务状态卡片。
- [x] Cloud Home 不展示或拉取用户设备列表。
- [x] Cloud Home 不直跳 `/devices/:deviceId/sessions`。
- [x] Cloud Home 已登录入口进入 `/dashboard`。
- [x] AgentHome 没有直接调用 Cloud API。
- [x] CloudHome 没有调用 Agent API。
- [x] Agent sessions 路径为 `/sessions`。
- [x] `web/src` 下无 `/agent/sessions` 残留。
- [x] 卡片 title 右上角操作融入 title 区，不使用边框、背景或蓝色实心按钮。
- [x] Dashboard 设备列表离线设备右侧 arrow-right 不可用。
- [x] Agent 本机设备卡片不从 Cloud session 取值。
- [x] 前端 typecheck、lint、测试通过。
- [x] 相关 Go 测试通过。
- [x] Agent / Cloud 前端构建入口均可构建。

## Command results

### Web typecheck

```text
yarn --cwd web typecheck
$ vue-tsc --noEmit
Done in 2.70s.
```

结果：通过。

### Targeted web tests

第一次命令使用了带 `web/` 前缀的过滤参数，Vitest 未匹配到测试文件：

```text
yarn --cwd web test --run web/src/features/sessions/runtime.test.ts web/src/features/api/client.test.ts
No test files found, exiting with code 0
```

随后使用相对于 `web` 工作目录的路径重新执行：

```text
yarn --cwd web test --run src/features/sessions/runtime.test.ts src/features/api/client.test.ts
Test Files  2 passed (2)
Tests       11 passed (11)
```

结果：通过。

### Web lint

```text
yarn --cwd web lint
$ eslint src --cache
Done in 29.68s.
```

结果：通过。

### Full web tests

```text
yarn --cwd web test
$ vitest run --passWithNoTests
Test Files  11 passed (11)
Tests       57 passed (57)
```

结果：通过。

### Targeted Go tests

```text
go test ./internal/agent/api/handler -run "TestAuthMeReturnsLocalDeviceSummary|TestAuthMeReturnsRuntimeLocalCloudSessionSummary|TestAgentLoginIssuesLocalTokenAndProtectsBusinessRoutes"
ok  gitee.com/leoninew/TermBridge-go/internal/agent/api/handler  (cached)
```

结果：通过。

### App / agent Go tests

```text
go test ./cmd/termbridge/app ./internal/agent/api/handler ./internal/agent/application/user ./internal/agent/application/bootstrap
ok  gitee.com/leoninew/TermBridge-go/cmd/termbridge/app                     5.838s
ok  gitee.com/leoninew/TermBridge-go/internal/agent/api/handler              1.573s
ok  gitee.com/leoninew/TermBridge-go/internal/agent/application/user         (cached)
ok  gitee.com/leoninew/TermBridge-go/internal/agent/application/bootstrap    0.718s
```

结果：通过。

### Frontend builds

```text
yarn --cwd web build:agent && yarn --cwd web build:cloud
```

结果：两个构建均通过。

构建期间 Vite / Rolldown 对 `node_modules/@vueuse/core/dist/index.js` 输出 `INVALID_ANNOTATION` warning，提示第三方包中的 `/* #__PURE__ */` 注释位置无法解释并被忽略；构建最终 `✓ built`，不是本轮代码错误。

### Static grep checks

```text
Grep /agent/sessions in web/src
No matches found

Grep loadDevices|cloudWorkbenchTargetDevice|openCloudSessions|features/agent/api|agentApiClient|/agent-api in CloudHome.vue
No matches found

Grep features/cloud/api|cloudApiClient|/cloud-api in AgentHome.vue
No matches found
```

结果：符合 API 边界和路由路径要求。

## Missed or expanded scope

- 用户追加要求将 `/agent/sessions` 改为 `/sessions` 且不兼容旧路径；已纳入 requirement 和实现。
- 用户追加要求卡片右上角按钮融入 title；已将相关 header actions 改为 text action / title link。
- 用户明确不要求处理 generated output 格式；本轮只保留字段必要变更和格式排除配置。
- 本次验证未执行浏览器视觉截图；卡片 title action 的视觉效果仍建议由用户人工看一眼。

## Risks

1. 当前工作区 diff 较大，且包含用户会自行合并的多组收尾变更；提交前建议用户按实际交付边界复核 diff。
2. Vite build 中的 Rolldown `INVALID_ANNOTATION` warning 来自 `node_modules/@vueuse/core`，当前不阻断构建；如未来 CI 将 warning 升级为 error，需要单独处理工具链或依赖。
3. Cloud Home 和 Dashboard 的最终视觉仍依赖人工确认，本轮验证只覆盖结构、类型、测试、构建和静态边界。

## Incomplete items

- 无阻塞项。
- 未做浏览器截图/手工视觉验收。

## Conclusion

本轮实现与 `docs/requirement/20260709-runtime-config-finish.md` 对齐。Home 已拆成 AgentHome / CloudHome，Cloud Home 不再拉设备或直跳会话列表，Agent sessions 已改为 `/sessions` 且无旧路径残留，卡片 title 右上角操作已改为 text action。前端 typecheck、lint、测试、agent/cloud build，以及相关 Go 测试均通过。实现可进入人工验收。
