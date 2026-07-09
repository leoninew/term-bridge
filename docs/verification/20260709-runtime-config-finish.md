# runtime config 与 Home 入口收尾验证
最后修改时间: 2026-07-09 19:12:00

Flow mode: light / 轻量模式
Stage: Verification / 验证
Review status: Draft

## Requirement alignment

按 `docs/requirement/20260709-runtime-config-finish.md`、`docs/requirement/20260709-package-runtime-cloud-connection.md`、用户追加的本地/云端连接语义指令（含本轮 connect 流程统一决策）核对：

1. Home 入口已拆分为 `LocalHome.vue` 和 `CloudHome.vue`，`HomeView.vue` 只负责按 `runtimeConfig.view.mode` 分发。
2. Local Home 保留本机工作台、工作区列表、连接/断开云端、hybrid 下切换云端模式。
3. Local Home 的本机设备信息来自本地 `auth/me` 返回的 `device`，不再从 Cloud session 取本机设备名。
4. Cloud Home 只作为入口页，不展示业务状态卡片，不展示用户私有设备列表。
5. Cloud Home 不再为了首页入口拉取设备列表，也不直接跳转到设备 sessions；已登录入口进入 `/dashboard`。
6. `LocalHome.vue` 未直接调用 Cloud API client；本地连接云端只通过本地后端 `/local-api/cloud/connect` 完成。
7. Cloud Dashboard `/dashboard` 继续负责设备列表、在线/离线状态和进入设备工作台。
8. 本地 sessions 路径已从 `/agent/sessions` 改为 `/sessions`，不保留旧路径兼容；route name 为 `local-sessions`。
9. 前端 env mode 文件收敛为 `web/.env.development`、`web/.env.local`、`web/.env.cloud`；`web/.env.production` 不再保留。
10. 旧 `web/src/store/cloud.ts` 混合职责已拆到 `authTokens`、`localAuth`、`cloudAuth`、`cloudDevices`，不保留 re-export 兼容层。
11. 最新用户指令修正了旧 requirement 中“断开清理 `cloud_binding`”的历史表述：当前验收语义是 `data/device.json` 不再持久化 `cloud_binding`，断开只停止 tunnel / 清理运行态连接。
12. 前端 connect 流程统一：后端 `/local-api/cloud/connect` 只接受 `{ cloud_token }`，不再接受 `{ code }`；code → token 兑换由独立端点 `/local-api/cloud/oauth/exchange` 完成。
13. 前端连接云端时，若已有 `termbridge_cloud_token`，直接请求本地 connect 接口完成设备上报与连接建立；没有云端认证时才进入 OAuth2。
14. OAuth2 完成后，`OAuthCallbackView.vue` 只做 code → token 兑换和 token 存储，不调 connect；connect 决策统一由 LocalHome 承担。
15. LocalHome 是 connect 决策唯一入口：页面加载完成时读 storage，有 token + 没有连接时间 → 自动发起 connect；按钮点击有 token 分支同样收敛到同一个 connect helper，避免刷新就重连。
16. 前端 connect 成功后把连接时间写入 sessionStorage；disconnect 后端返回 `200 OK` 后清理该 sessionStorage 连接时间。
17. 后端 disconnect 不调用 Cloud 侧解绑/删除设备接口，不改变本机 device identity。
18. 后端 connect 不把 cloud session、token 或 `cloud_binding` 写入 `data/device.json`。

## Spec alignment

不适用。light / 轻量模式未创建单独 Spec / 规格文档，按 Requirement / 需求和用户追加指令核对。

## Plan alignment

不适用。light / 轻量模式未创建单独 Plan / 计划文档，按 Requirement / 需求和用户追加指令核对。

## Actual diff summary

当前 diff 覆盖以下主要范围：

- 文档：新增 Docker 前端 runtime config 后续决策 requirement；本验证文档同步最新连接/断开语义（含本轮 connect 流程统一决策）。
- 后端设备身份：`Device` / `deviceIdentity` 删除持久化 `CloudBinding` 字段和相关 load/save/clear helper；legacy `cloud_binding` 字段读取时忽略。
- 后端本地 Cloud session：connect 仅设置运行态 session 并触发 connector；disconnect 清空运行态 session、触发 connector 停止并返回 `200 OK`。
- 后端 connect 流程统一：`/local-api/cloud/connect` 不再接受 `{ code }`，code → token 兑换由新端点 `/local-api/cloud/oauth/exchange` 承担；删除 `cloudAccessTokenFromCode` 和 `cloudConnectRequest.Code` 字段。
- 后端测试：覆盖 cloud token connect、connect 不持久化 cloud session、disconnect 不解绑 Cloud device、无 token 不上报设备、legacy cloud binding 忽略。所有既有测试 body 均已使用 `cloud_token`。
- 前端本地 connect：保留 `connectCloudWithToken` 作为唯一 connect 函数，删除 `connectCloudWithAuthorizationCode`；新增 `exchangeOAuthCode` 调用 `/local-api/cloud/oauth/exchange`。
- 前端 connect 决策统一到 LocalHome：抽取 `ensureCloudConnection` 私有 helper，`loadLocalHome`（页面加载）和 `toggleCloudConnection`（按钮点击）共用同一套判断——有 token + 没有连接时间才 connect，避免刷新页面重连。
- `OAuthCallbackView.vue` 职责收窄：只兑换 code → token 并存储，不再调 connect；跳回 LocalHome 后由 LocalHome 读取 storage 决定是否 connect。
- 前端连接时间：新增 sessionStorage helper，connect 成功后写入连接时间，disconnect 成功后清理。
- 前端构建/类型：本地和云端构建入口均通过验证。

## Expected vs actual changed files

| 范围 | 预期 | 实际 | 结论 |
| --- | --- | --- | --- |
| device identity | 不再持久化 `cloud_binding` | `Device` / `deviceIdentity` 无 cloud binding 字段，测试检查 `device.json` 不含该字段 | 符合 |
| legacy 文件 | 已存在 `cloud_binding` 不应影响 device identity | 测试覆盖 legacy 字段被忽略，device identity 不变 | 符合 |
| connect 入口统一 | 前端只有一个 connect 函数 `connectCloudWithToken`，不再有 `connectCloudWithAuthorizationCode` | `LocalHome.vue` / `OAuthCallbackView.vue` 均不再引用旧函数 | 待验证 |
| connect body 统一 | 后端 `/local-api/cloud/connect` 只接受 `{ cloud_token }`，删除 `code` 分支 | handler 删除 `code` 分支 + `cloudAccessTokenFromCode` 调用；`cloudConnectRequest` 删除 `Code` 字段 | 待验证 |
| code 兑换端点 | code → token 由 `/local-api/cloud/oauth/exchange` 承担，返回 `{ access_token }` | 新 handler 复用既有 OAuth2 配置；`OAuthCallbackView.vue` 改用 `exchangeOAuthCode` | 待验证 |
| connect 决策统一到 LocalHome | 页面加载 + 按钮点击收敛到同一个 `ensureCloudConnection` helper，判断依据：有 token + 没有连接时间 | `LocalHome.vue` `loadLocalHome` 末尾和 `toggleCloudConnection` 有 token 分支共用 helper | 待验证 |
| OAuth callback 职责 | 只兑 token、存 token、跳回；不调 connect | `OAuthCallbackView.vue` 调 `exchangeOAuthCode` + `setCloudToken`，不再调 `connectCloudWith*` | 待验证 |
| connect persistence | connect 不写 token/session/binding 到 `device.json` | 后端移除保存逻辑，测试检查无 token、bearer、`cloud_binding` | 符合 |
| disconnect | 只停止本机连接，返回 200，不解绑 Cloud device | handler 清 session 并调用 `OnLocalCloudSession(nil)`；测试检查 Cloud 只收到 device report | 符合 |
| connection time | 仅本次浏览器 session 记连接时间 | `markCloudConnected` 写 sessionStorage，`clearCloudConnectionTime` 清理 | 符合 |
| 前端类型 | 新增 helper 和 API 调用类型正确 | `yarn --cwd web typecheck` 通过 | 待验证 |
| Go 回归 | 后端行为无回归 | `go test ./cmd/... ./internal/...` 通过 | 待验证 |

## Acceptance checklist

**第一轮已实现（已验收）：**

- [x] `data/device.json` 不再持久化 `cloud_binding`。
- [x] legacy `device.json` 中已有 `cloud_binding` 时，加载设备身份会忽略该字段。
- [x] connect 成功后不把 cloud session、cloud token、bearer token 或 cloud binding 写入 device identity 文件。
- [x] connect 成功后把当前连接时间写入 sessionStorage。
- [x] disconnect 后端返回 `200 OK`。
- [x] disconnect 成功后前端清理 sessionStorage 连接时间。
- [x] disconnect 不调用 Cloud 侧解绑/删除设备。
- [x] disconnect 后本机 `auth/me` 不再返回 `cloud_session`。
- [x] disconnect 后本机 device identity 不变。
- [x] 第一轮相关 Go 测试、全量 Go 测试、前端 typecheck、前端测试、前端 lint、本地/云端前端 build 均通过。

**第二轮 connect 流程统一（待验收）：**

- [ ] 后端 `/local-api/cloud/connect` handler 不再接受 `{ code }`——`cloudConnectRequest` 删除 `Code` 字段、handler 删除 code 分支和 `cloudAccessTokenFromCode` 调用。
- [ ] `cloudAccessTokenFromCode` 方法从 `server.go` 移除（逻辑迁移到 `/cloud/oauth/exchange` handler）。
- [ ] 新端点 `/local-api/cloud/oauth/exchange` 实现 code → token 兑换，返回 `{ access_token }`。
- [ ] 前端 `connectCloudWithAuthorizationCode` 从 `local/api.ts` 移除，新增 `exchangeOAuthCode` 调用新端点。
- [ ] `OAuthCallbackView.vue` 改用 `exchangeOAuthCode` + `setCloudToken`，不再调任何 connect 函数；跳回 home。
- [ ] `LocalHome.vue` `loadLocalHome` 末尾加入 connect 判断（有 token + 没有连接时间 → connect）。
- [ ] `LocalHome.vue` 抽取 `ensureCloudConnection` helper，`loadLocalHome`（页面加载）和 `toggleCloudConnection`（按钮点击）共用同一套逻辑。
- [ ] 删除 `connectCloudWithAuthorizationCode` 后无残留引用。
- [ ] 新端点有 Go 测试覆盖：happy path（code 有效返回 token）、缺失 code 返回 400、OAuth 配置缺失返回 503、Cloud token endpoint 失败返回 502。
- [ ] 相关 Go 测试、全量 Go 测试、前端 typecheck、前端测试、前端 lint、本地/云端前端 build 均通过。

## Command results

### Targeted Go tests

```text
go test ./internal/agent/application/user ./internal/agent/api/handler -run "TestDeviceIdentityDoesNotPersistCloudBinding|TestLegacyCloudBindingIsIgnoredWhenLoadingDeviceIdentity|TestCloudConnectReportsCurrentDeviceWithCloudToken|TestCloudConnectDoesNotPersistLocalCloudSession|TestCloudDisconnectClearsLocalCloudSessionWithoutUnbindingCloudDevice|TestCloudConnectRequiresCloudTokenBeforeDeviceReport"
ok  gitee.com/leoninew/TermBridge-go/internal/agent/application/user  (cached)
ok  gitee.com/leoninew/TermBridge-go/internal/agent/api/handler        (cached)
```

结果：通过。

### Web typecheck

```text
yarn --cwd web typecheck
$ vue-tsc --noEmit
Done in 4.65s.
```

结果：通过。

### Full Go tests

```text
go test ./cmd/... ./internal/...
```

结果：通过。覆盖 `cmd/termbridge`、`cmd/termbridge/app`、`cmd/termbridge/cli`、`internal/agent/...`、`internal/cloud/...`、`internal/shared/...` 下测试包；所有有测试包均为 `ok`，无测试文件包为 `[no test files]`。

### Full web tests

```text
yarn --cwd web test
$ vitest run --passWithNoTests
Test Files  11 passed (11)
Tests       57 passed (57)
Done in 1.17s.
```

结果：通过。

### Web lint

```text
yarn --cwd web lint
$ eslint src --cache
Done in 1.51s.
```

结果：通过。

### Go formatting check

```text
gofmt -l internal/agent/application/user/device.go internal/agent/application/user/device_test.go internal/agent/api/handler/server.go internal/agent/api/handler/cloud_binding_test.go
```

结果：无输出，目标 Go 文件 gofmt 已对齐。

### Frontend local build

```text
yarn --cwd web build:local
$ vue-tsc --noEmit && vite build
✓ built in 489ms
Done in 3.37s.
```

结果：通过。

### Frontend cloud build

```text
yarn --cwd web build:cloud
$ vue-tsc --noEmit && vite build --mode cloud
✓ built in 480ms
Done in 3.38s.
```

结果：通过。

构建期间 Vite / Rolldown 对 `node_modules/@vueuse/core/dist/index.js` 输出 `INVALID_ANNOTATION` warning，提示第三方包中的 `/* #__PURE__ */` 注释位置无法解释并被忽略；构建最终 `✓ built`，不是本轮代码错误。

### Static grep checks

```text
Grep CloudBinding|cloud_binding|SaveCloudBinding|ClearCloudBinding|LoadCloudBinding|connectCloudWithCurrentAccount
```

结果：运行代码中未发现相关旧 helper 或旧前端函数残留；仅测试断言、历史文档、历史计划/验证记录和 `cloud_binding_test.go` 文件名仍包含 `cloud_binding` 字样。

## Missed or expanded scope

- 用户追加修正了断开云端语义：断开只停止 tunnel / 清理运行态连接，不是清理持久化 binding；已按追加指令验证。
- 用户追加要求 `data/device.json` 不再持久化 `cloud_binding`；已纳入实现和测试。
- 用户追加要求已有 `termbridge_cloud_token` 时快速 connect；已纳入前端实现和后端 connect 测试。
- 本次验证没有新增浏览器级交互测试；连接按钮真实点击、sessionStorage 展示效果仍建议人工在浏览器中看一眼。
- 历史文档仍可能描述旧 `cloud_binding` 行为；本验证文档以最新用户指令为准，没有主动清理历史验证记录。

## Risks

1. 当前工作区已有 staged diff，提交前建议用户复核 staged/unstaged 边界，避免把后续验证文档或历史文档调整混入不期望的提交。
2. `docs/requirement/20260709-runtime-config-finish.md` 第 48 行仍保留旧“清理 `cloud_binding`”表述；本验证已记录该语义被最新用户指令取代。如需要过程文档完全一致，建议后续单独更新 requirement。
3. Vite build 的 Rolldown `INVALID_ANNOTATION` warning 来自 `node_modules/@vueuse/core`，当前不阻断构建；如果未来 CI 将 warning 升级为 error，需要单独处理工具链或依赖。
4. 前端 connect 快路径依赖浏览器已有 `termbridge_cloud_token`；如果 token 过期，本地 connect 会失败并显示连接失败，当前未实现自动刷新或回退 OAuth2。

## Incomplete items

- 无阻塞项。
- 未做浏览器截图/手工视觉验收。
- 未新增 LocalHome 组件级测试来模拟已有 `termbridge_cloud_token` 的点击路径；当前通过类型检查、API helper、后端行为测试和全量 web tests 兜底。

## Conclusion

**第一轮**实现与最新用户指令对齐：`data/device.json` 不再持久化 `cloud_binding`；connect 在已有 `termbridge_cloud_token` 时直接上报设备并建立连接，没有 token 才走 OAuth2；connect 成功后连接时间只写入 sessionStorage；disconnect 只停止本机 tunnel / 清理运行态 session，返回 `200 OK` 后前端清理 sessionStorage 连接时间，不解绑或删除 Cloud 侧设备。targeted Go tests、全量 Go tests、前端 typecheck、全量前端 tests、lint、gofmt 检查、本地/云端 build 均已通过。

**第二轮**目标：connect 流程统一。后端 `/local-api/cloud/connect` 只接受 `{ cloud_token }`，code → token 兑换由独立端点承担；前端 connect 入口收敛到 `connectCloudWithToken` 单一函数；connect 决策统一由 LocalHome 通过"有 token + 没有连接时间"判断承担，页面加载自动 connect 与按钮点击共用同一套 helper；`OAuthCallbackView.vue` 职责收窄为纯 token 交接。第二轮验收清单见上方 Acceptance checklist，当前为 Draft 状态，待实现与跑通后翻 Accepted。
