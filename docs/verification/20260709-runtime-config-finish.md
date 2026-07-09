# runtime config 与 Home 入口收尾验证
最后修改时间: 2026-07-09 13:18:06

Flow mode: light / 轻量模式
Stage: Verification / 验证
Review status: Accepted

## Requirement alignment

按 `docs/requirement/20260709-runtime-config-finish.md` 和 `docs/requirement/20260709-local-cloud-terminology.md` 核对：

1. Home 入口已拆分为 `LocalHome.vue` 和 `CloudHome.vue`，`HomeView.vue` 只负责按 `runtimeConfig.view.mode` 分发。
2. Local Home 保留本机工作台、工作区列表、连接/断开 Cloud、hybrid 下切换云端模式。
3. Local Home 的本机设备信息来自本地 `auth/me` 返回的 `device`，不再从 Cloud session 取本机设备名。
4. Cloud Home 只作为入口页，不展示业务状态卡片，不展示用户私有设备列表。
5. Cloud Home 不再为了首页入口拉取设备列表，也不直接跳转到设备 sessions；已登录入口只进入 `/dashboard`。
6. `LocalHome.vue` 未直接调用 `cloud-api` / `features/cloud/api`；`CloudHome.vue` 未直接调用本地 API。
7. Cloud Dashboard `/dashboard` 继续负责设备列表、在线/离线状态和进入设备工作台。
8. Cloud Dashboard 离线设备右侧只保留不可用 arrow-right icon，不展示“设备离线→”文字按钮。
9. 设备列表卡片右侧保留返回首页入口。
10. 卡片 title 右上角操作已改成 text action / title link 样式，不使用边框、背景或蓝色实心按钮。
11. 本地 sessions 路径已从 `/agent/sessions` 改为 `/sessions`，不保留旧路径兼容；route name 为 `local-sessions`。
12. `/sessions` 和 `/devices/:deviceId/sessions` 继续使用现有 sessions 页面能力。
13. 前端 generated proto 目录和后端 generated proto 目录已纳入格式化/检查排除配置；本轮不继续处理 gen 输出格式。
14. 前端 env mode 文件收敛为 `web/.env.development`、`web/.env.local`、`web/.env.cloud`；`web/.env.production` 不再保留。
15. 根目录 `.env` / `.env.<env>` 保持 machine-local 覆盖文件语义并默认忽略。
16. 旧 `web/src/store/cloud.ts` 混合职责已拆到 `authTokens`、`localAuth`、`cloudAuth`、`cloudDevices`，不保留 re-export 兼容层。

## Spec alignment

不适用。light / 轻量模式未创建单独 Spec / 规格文档，按 Requirement / 需求核对。

## Plan alignment

不适用。light / 轻量模式未创建单独 Plan / 计划文档，按 Requirement / 需求和用户追加指令核对。

## Actual diff summary

当前 diff 覆盖以下主要范围：

- 文档：合并 runtime config / Home / env mode requirement；同步本地/云端术语需求和验证文档。
- 配置：更新 `.gitignore`、`.golangci.yml`、`.env.example`、`web/.prettierignore`、`web/.env.*`、Dockerfile 构建入口、README。
- Proto/API：`AuthMeResp` 增加 `device`，本地 `auth/me` 返回本机 device summary。
- 后端：Cloud connect/disconnect、本机 cloud session、本机设备 identity、`/local-api` 路由和配置校验相关调整与测试。
- 前端 runtime config：移除旧 `appMode`，新增 `runtimeConfig` store 和测试，收敛配置读取边界。
- Home 前端：`HomeView.vue` 简化为模式分发，新增 `LocalHome.vue` 和 `CloudHome.vue`。
- Cloud Home：移除设备列表拉取、设备入口直跳 sessions 和“进入 xxxx 工作台”逻辑，已登录入口回到 `/dashboard`。
- Dashboard：设备列表右侧离线 arrow-right 不可用；卡片 title 右上角操作改为 text action。
- 路由：本地 sessions 从 `/agent/sessions` 改为 `/sessions`，不保留旧路径。
- Store：拆分旧混合 cloud store，删除 `cloud.ts` re-export 兼容层和临时 local cloud connection store。
- Generated code：proto 生成物随字段增加变化；按用户要求不继续处理格式化细节。

## Expected vs actual changed files

| 范围 | 预期 | 实际 | 结论 |
| --- | --- | --- | --- |
| Home 拆分 | Local / Cloud Home 分离，HomeView 只分发 | 已新增 `LocalHome.vue`、`CloudHome.vue`，`HomeView.vue` 只按模式渲染 | 符合 |
| LocalHome API 边界 | 不直接调用 Cloud API | grep 未发现 `features/cloud/api`、`cloudApiClient`、`/cloud-api` | 符合 |
| CloudHome API 边界 | 不调用本地 API，不拉设备列表 | grep 未发现本地 API 调用、`/local-api`、`loadDevices`、`openCloudSessions` | 符合 |
| Cloud Home 入口 | 已登录只进入 `/dashboard` | 已移除设备选择/直跳 sessions 逻辑 | 符合 |
| 本地 sessions 路径 | `/sessions`，无旧路径兼容 | router path 已改，`web/src` 下无 `/agent/sessions` 残留 | 符合 |
| Store 拆分 | 不保留 `cloud.ts` 兼容层 | 调用方直接依赖 `authTokens`、`localAuth`、`cloudAuth`、`cloudDevices` | 符合 |
| 卡片 title action | 右上角无边框、无背景、非蓝色实心 | `LocalHome` 和 `DashboardView` title actions 已改为 muted text action | 符合 |
| Dashboard 设备入口 | 离线只显示不可用 arrow-right icon | 设备项右侧为无边框 icon button，离线 disabled | 符合 |
| 本机设备来源 | 本地 device identity，不取 Cloud | 本地 `auth/me` 返回 `device`，Home 使用 `me.device` | 符合 |
| Env 文件 | 前端 mode 文件收敛，root env 忽略 | `.env.local` / `.env.cloud` / `.env.development` 与 `.gitignore` 已调整 | 符合 |
| Generated 目录 | 前后端排除格式化/检查 | `.golangci.yml` 排除 `internal/gen/proto/`，`web/.prettierignore` 排除 `src/gen` | 符合 |

## Acceptance checklist

- [x] `HomeView.vue` 不再混放本地/云端两套页面主体。
- [x] `LocalHome.vue` 保留本地模式入口能力。
- [x] `CloudHome.vue` 保留云端模式入口能力。
- [x] Cloud Home 不展示业务状态卡片。
- [x] Cloud Home 不展示或拉取用户设备列表。
- [x] Cloud Home 不直跳 `/devices/:deviceId/sessions`。
- [x] Cloud Home 已登录入口进入 `/dashboard`。
- [x] LocalHome 没有直接调用 Cloud API。
- [x] CloudHome 没有调用本地 API。
- [x] 本地 sessions 路径为 `/sessions`。
- [x] `web/src` 下无 `/agent/sessions` 残留。
- [x] 旧 `cloud.ts` 混合 store 已拆分，且没有兼容 re-export 层。
- [x] 卡片 title 右上角操作融入 title 区，不使用边框、背景或蓝色实心按钮。
- [x] Dashboard 设备列表离线设备右侧 arrow-right 不可用。
- [x] 本机设备卡片不从 Cloud session 取值。
- [x] 前端 typecheck、lint、测试通过。
- [x] 相关 Go 测试通过。
- [x] 本地 / Cloud 前端构建入口均可构建。

## Command results

### Web typecheck

```text
yarn --cwd web typecheck
$ vue-tsc --noEmit
Done in 2.50s.
```

结果：通过。

### Affected web tests after store split

```text
yarn --cwd web test src/store/authStores.test.ts src/features/api/client.test.ts
Test Files  2 passed (2)
Tests       15 passed (15)
```

结果：通过。

### Previous targeted web tests

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

### App / local Go tests

```text
go test ./cmd/termbridge/app ./internal/agent/api/handler ./internal/agent/application/user ./internal/agent/application/bootstrap
ok  gitee.com/leoninew/TermBridge-go/cmd/termbridge/app                     5.838s
ok  gitee.com/leoninew/TermBridge-go/internal/agent/api/handler              1.573s
ok  gitee.com/leoninew/TermBridge-go/internal/agent/application/user         (cached)
ok  gitee.com/leoninew/TermBridge-go/internal/agent/application/bootstrap    0.718s
```

结果：通过。

### Frontend builds

第一次执行 `build:local` 时发现 Vite 不允许使用 `local` 作为 mode 名称，因为它与 `.env.local` 后缀规则冲突；因此保留文件名 `web/.env.local`，但 `build:local` 不显式传 `--mode`，让 Vite 默认 production mode 自动加载 `.env.local`，公开配置值仍是 `TERMBRIDGE_LOCAL__MODE=local`。

修正后重新执行：

```text
yarn --cwd web build:local && yarn --cwd web build:cloud
$ vue-tsc --noEmit && vite build
✓ built
$ vue-tsc --noEmit && vite build --mode cloud
✓ built
```

结果：两个构建均通过。

构建期间 Vite / Rolldown 对 `node_modules/@vueuse/core/dist/index.js` 输出 `INVALID_ANNOTATION` warning，提示第三方包中的 `/* #__PURE__ */` 注释位置无法解释并被忽略；构建最终 `✓ built`，不是本轮代码错误。

### App / config Go tests

```text
go test ./cmd/termbridge/app ./internal/shared/infrastructure/config
ok  gitee.com/leoninew/TermBridge-go/cmd/termbridge/app                    4.478s
ok  gitee.com/leoninew/TermBridge-go/internal/shared/infrastructure/config 1.170s
```

结果：通过。

### Static grep checks

```text
Grep /agent/sessions in web/src
No matches found

Grep useCloudStore|store/cloud|localCloudConnection|cloud.test
No current code imports or files rely on the old compatibility layer

Grep Cloud mode|cloud mode|Agent mode|agent mode|Cloud 模式|Agent 模式 in web/src
No matches found

Grep TERMBRIDGE_AGENT|.env.agent|build:agent|agent-api|/agent/sessions|agent-sessions in web
No matches found

Glob web/.env*
web/.env
web/.env.development
web/.env.cloud
web/.env.local
```

结果：符合 API 边界、路由路径、store 拆分和用户可见术语要求。

## Missed or expanded scope

- 用户追加要求将 `/agent/sessions` 改为 `/sessions` 且不兼容旧路径；已纳入 requirement 和实现。
- 用户追加要求卡片右上角按钮融入 title；已将相关 header actions 改为 text action / title link。
- 用户追加要求 `/agent-api` 改为 `/local-api`，后端、前端和视图同步调整；已纳入本地/云端术语需求。
- 用户追加要求 `web/src/store/cloud.ts` 进一步拆分且不做兼容或适配层；已删除旧兼容层并改为直接依赖新 store。
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

本轮实现与 `docs/requirement/20260709-runtime-config-finish.md`、`docs/requirement/20260709-local-cloud-terminology.md` 对齐。Home 已拆成 LocalHome / CloudHome，Cloud Home 不再拉设备或直跳会话列表，本地 sessions 已改为 `/sessions` 且无旧路径残留，旧混合 cloud store 已拆分且无兼容层，卡片 title 右上角操作已改为 text action。前端 typecheck、lint、测试、本地/Cloud build，以及相关 Go 测试均通过。实现可进入人工验收。
