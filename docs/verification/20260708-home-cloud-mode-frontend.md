# Home 页 Cloud 模式前端界面验证
最后修改时间: 2026-07-08 18:02:12

Flow mode: light / 轻量模式
Stage: Verification / 验证
Review status: Accepted

## Requirement alignment

按 `docs/requirement/20260708-home-cloud-mode-frontend.md` 核对：

1. Home 页 agent 模式保留本机工作台、工作区列表、连接云端和切换 Cloud 模式入口。
2. Home 页 cloud 模式已改为产品入口首页，不作为管理后台或企业/项目介绍页；页面使用约 1200px 内容区，并避免整页大卡片承载。
3. agent 模式右上角保留主题、语言和云端连接状态。
4. cloud 模式右上角展示主题、语言和账号区域；未登录展示登录入口，已登录展示账号菜单。
5. cloud Home 以核心入口为主，未使用“项目/产品介绍”类标题或大段企业介绍文案；未确认文案保持 TODO 占位。
6. cloud Home 使用 `/image-gen` 生成的主视觉素材 `web/src/assets/cloud-home-hero-candidate.png`。
7. cloud Home 提供 Dashboard 入口；登录态由现有 Cloud auth/store 路径处理。
8. cloud Home 提供 Agent 入口：cloud-only 新开 Agent 页面，hybrid 在 SPA 内切换 Agent 模式。
9. Cloud Dashboard 路径为 `/dashboard`，route name 为 `cloud-dashboard`，要求 Cloud 登录。
10. Cloud 会话路径为 `/devices/:deviceId/sessions`，route name 为 `cloud-sessions`。
11. `/agent/dashboard` 页面和路由已删除，相关回跳改为 Home 或明确目标。
12. Agent 侧新增本地断开云端连接能力：使用独立 `POST /agent-api/cloud/disconnect`，清理本机 `cloud_session` / `cloud_binding`，停止 Cloud connector；不调用 Cloud 侧解绑/删除设备接口。
13. 前端 Agent Home 已连接时点击“断开云端”调用新增 Agent API，成功后按钮回到“连接云端”。
14. `/agent/sessions` 与 `/devices/:deviceId/sessions` 共用侧栏左上角已展示 `TB` 蓝块 Logo 链接，尺寸适配当前 `h-11` 头部高度，点击回到对应 Home。
15. 两个会话页面右上角 Home / 账号类入口按钮已移除，避免与左上 Logo 链接重复。
16. Home agent 的“打开工作区”和 Cloud Dashboard 在线设备“进入工作台”按钮已改为 primary 样式。
17. 未引入 reka-ui、Tailwind v4、`@lucide/vue` 之外的新前端组件库。

## Spec alignment

不适用。light / 轻量模式未创建单独 Spec / 规格文档，按 Requirement / 需求核对。

## Plan alignment

不适用。light / 轻量模式未创建单独 Plan / 计划文档，按 Requirement / 需求和实现过程核对。

## Actual diff summary

当前暂存区 diff 涉及：

- `.golangci.yml`
  - 新增 Go lint 配置入口。
- `docs/requirement/20260708-home-cloud-mode-frontend.md`
  - 记录 Cloud Home、Dashboard、路由、Agent 断开云端连接语义和用户审查意见。
- `docs/verification/20260708-home-cloud-mode-frontend.md`
  - 新增本验证文档。
- `internal/agent/api/handler/server.go`
  - 保留 `POST /agent-api/cloud/connect`，新增 `POST /agent-api/cloud/disconnect`。
  - 断开时清理本机 cloud session 并触发本地连接生命周期回调。
- `internal/agent/api/handler/cloud_binding_test.go`
  - 覆盖 Agent 断开云端连接不会触发 Cloud 侧解绑/删除请求。
- `internal/agent/application/bootstrap/server.go`
  - 新增 Cloud connector 生命周期控制，连接时启动、断开时停止。
- `internal/agent/application/user/device.go`
  - 新增 `ClearCloudBindingSummary`，只清理本机持久化 cloud binding。
- `internal/agent/application/user/device_test.go`
  - 覆盖本机断开后设备身份不变、cloud binding 清理。
- `web/src/assets/cloud-home-hero-candidate.png`
  - 新增 Cloud Home 主视觉素材。
- `web/src/components/ModeSwitcher.vue`
  - 删除右下角全局模式切换组件。
- `web/src/components/dashboard/CloudAccountMenu.vue`
  - 新增 Cloud Home 顶部账号菜单。
- `web/src/config.ts`
  - 新增 Agent 页面 URL 构造能力。
- `web/src/features/agent/api.ts`
  - 新增 `disconnectCloud()`，调用 `POST /cloud/disconnect`。
- `web/src/features/cloud/oauth.ts`
  - 调整 OAuth 回跳目标。
- `web/src/i18n.ts`
  - 新增/调整 Home、Dashboard、断开云端、账号菜单等文案。
- `web/src/router/index.ts`
  - Home 真实路由、Cloud Dashboard 路由、Cloud sessions 路由和旧 agent dashboard 移除相关调整。
- `web/src/store/appMode.ts`
  - 模式入口状态调整。
- `web/src/views/HomeView.vue`
  - 新增统一 Home 页面，按 agent/cloud 模式渲染不同入口；接入 Agent 断开云端按钮。
- `web/src/views/agent/*`
  - 移除旧 agent dashboard，并同步回跳到 Home 或相关目标。
- `web/src/views/cloud/*`
  - Cloud Dashboard、OAuth authorize、sessions 回跳和登录态入口调整。
- `web/src/components/session/*`、`web/src/components/workspace/*`
  - 同步 Home / sessions 相关导航与布局细节。

## Expected vs actual changed files

| 范围 | 预期 | 实际 | 结论 |
| --- | --- | --- | --- |
| 需求文档 | 更新 cloud Home 与 Agent 断开语义 | 已更新 `docs/requirement/20260708-home-cloud-mode-frontend.md` | 符合 |
| 验证文档 | 进入 Verification 后新增文档 | 已新增本文档 | 符合 |
| Cloud Home | 新增产品入口首页 | 已由 `HomeView.vue` 在 cloud 模式渲染 | 符合 |
| Agent Home | 不回退，新增断开云端可用动作 | 已保留本机工作台，并接入 `disconnectCloud()` | 符合 |
| Agent 断开 API | 独立 `POST /agent-api/cloud/disconnect` | 已新增路由与 handler | 符合 |
| 本机状态 | 清理本机 `cloud_session` / `cloud_binding` | 已清理 runtime session 与 device identity cloud binding | 符合 |
| Cloud 侧解绑 | 不调用 Cloud 删除/解绑设备 | 测试覆盖只有 `/cloud-api/devices/current`，无 delete/unbind | 符合 |
| Connector 生命周期 | 断开后停止 connector | 已新增 lifecycle Stop | 符合 |
| 路由 | `/dashboard` 与 `/devices/:deviceId/sessions` | router 中已存在对应 route | 符合 |
| 旧 agent dashboard | 删除页面与 route | 文件删除、路由不再出现 `agent-dashboard` | 符合 |
| Sessions 左上 Logo | `/agent/sessions` 与 `/devices/:deviceId/sessions` 左上展示 `TB` 蓝块链接 | 已通过共用侧栏 `WorkspaceSessionSidebar.vue` 实现 | 符合 |
| Sessions 右上入口 | 两个会话页面右上角不展示额外 Home / 账号入口 | 已从 `SessionWorkbench.vue` 移除右上角按钮组 | 符合 |
| Primary 入口按钮 | Home agent 打开工作区、Cloud Dashboard 进入工作台使用 primary | 已更新对应按钮样式 | 符合 |
| 组件库 | 不新增其他前端组件库 | `package.json` 未新增组件库依赖 | 符合 |

## Acceptance checklist

- [x] Home 页 agent 模式保持当前本机工作台和工作区列表入口。
- [x] cloud Home 是产品入口首页，不是管理后台，也不是企业/项目介绍页。
- [x] cloud Home 约 1200px 内容区，不使用整页大卡片承载。
- [x] cloud Home 使用生成主视觉素材，未生造正式营销文案。
- [x] cloud Home 未登录展示登录入口，已登录展示账号菜单。
- [x] 账号菜单包含修改密码和退出登录入口。
- [x] cloud Home 提供 Dashboard 入口。
- [x] cloud Home 提供 Agent 入口，cloud-only 新开 Agent 页面，hybrid SPA 切换。
- [x] `/dashboard` route 存在并要求 Cloud 登录。
- [x] `/devices/:deviceId/sessions` route 存在并复用 Cloud sessions 页面能力。
- [x] `/agent/dashboard` route 和页面已删除。
- [x] Agent 断开云端使用独立 `POST /agent-api/cloud/disconnect`，不是同一路径 HTTP 谓词分支。
- [x] Agent 断开云端后本机 `auth/me` 不再返回 `cloud_session`。
- [x] Agent 断开云端后本机设备身份保留。
- [x] Agent 断开云端不调用 Cloud 侧解绑/删除设备接口。
- [x] Agent 断开云端后前端按钮回到“连接云端”。
- [x] `/agent/sessions` 与 `/devices/:deviceId/sessions` 左上角展示 `TB` 蓝块 Logo 链接。
- [x] 两个会话页面右上角不展示额外入口按钮。
- [x] Home agent 的“打开工作区”按钮使用 primary 样式。
- [x] Cloud Dashboard 在线设备的“进入工作台”按钮使用 primary 样式。
- [x] 未引入新的前端组件库。

## Command results

### Targeted Go tests

```text
go test ./internal/agent/application/user ./internal/agent/api/handler ./internal/agent/application/bootstrap
ok   gitee.com/leoninew/TermBridge-go/internal/agent/application/user      (cached)
ok   gitee.com/leoninew/TermBridge-go/internal/agent/api/handler           (cached)
ok   gitee.com/leoninew/TermBridge-go/internal/agent/application/bootstrap (cached)
```

结果：通过。

### Web type check

```text
npm --prefix web run typecheck

> termbridge-web@0.1.0 typecheck
> vue-tsc --noEmit
```

结果：通过。

### Web lint

```text
npm --prefix web run lint

> termbridge-web@0.1.0 lint
> eslint src --cache
```

结果：通过。

### Web tests

```text
npm --prefix web test

> termbridge-web@0.1.0 test
> vitest run --passWithNoTests

Test Files  12 passed (12)
Tests       60 passed (60)
```

结果：通过。

### Sessions UI follow-up checks

追加 `/agent/sessions` 与 `/devices/:deviceId/sessions` 左上 Logo、右上入口移除和 primary 按钮调整后，重新运行：

```text
npm --prefix web run typecheck
npm --prefix web run lint
npm --prefix web test
```

结果：均通过；Web tests 仍为 12 个文件、60 个测试通过。

## Missed or expanded scope

- 新增了 Agent 侧断开云端连接接口，这是用户在实现过程中追加且已写入 Requirement 的范围变更；该接口仅清理本机连接状态，不属于 Cloud 侧设备管理或解绑接口。
- 新增 `.golangci.yml` 属于当前 diff 中的工程配置变更；本次验证未运行 `golangci-lint`，仅记录其存在和 diff 范围。
- 本次验证以静态 diff、路由核对、类型检查、lint、单元测试为主；未再次使用浏览器自动化截图复核 Cloud Home 视觉。

## Risks

1. Cloud Home 正式标题、入口说明和宣传文案仍为 TODO，占位符合当前需求，但上线前需要补真实文案。
2. Cloud Home 主视觉为当前生成候选图，视觉最终接受度仍依赖用户人工确认。
3. Connector 停止行为通过上下文取消实现，已由代码路径和目标测试覆盖本地状态语义；未进行真实 Cloud WebSocket 端到端断开测试。
4. 当前工作区存在较多暂存改动，验证按当前暂存区整体 diff 记录；若后续继续追加新需求，需要重新核对最终 diff。

## Incomplete items

- 无阻塞项。
- 未运行浏览器视觉自动化复核；如用户需要，可单独进入视觉验收。

## Conclusion

本轮实现与 `docs/requirement/20260708-home-cloud-mode-frontend.md` 对齐。核心验收项包括 Cloud Home 产品入口、Dashboard 路由、Cloud sessions 路由、旧 agent dashboard 删除，以及 Agent 侧本地断开云端连接能力。目标 Go 测试、Web 类型检查、Web lint 和 Web 测试均通过。实现可交付人工验收。
