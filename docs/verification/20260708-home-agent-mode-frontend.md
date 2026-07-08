# Home 页 Agent 模式前端界面验证
最后修改时间: 2026-07-08 15:45:00

Flow mode: light / 轻量模式
Stage: Verification / 验证
Review status: Accepted

## Requirement alignment

按 `docs/requirement/20260708-home-agent-mode-frontend.md` 核对：

1. Home 页 agent 模式顶部工具栏已实现：左侧 base64 LOGO + TermBridge 标题，右侧远程连接状态 icon、语言切换 icon、主题切换 icon。
2. 顶部工具栏连接状态只展示动态 icon：已连接为绿色 Cloud，未连接为红色 CloudOff；已连接 hover title 包含连接时间；不承载 cloud 模式切换入口。
3. 语言和主题切换使用 icon button，单击直接切换，不展示 dropdown。
4. 页面主体居中展示上下两个卡片。
5. 上方卡片标题为“本机工作台”，内容展示本机设备、当前用户、项目版本三列。
6. “连接云端”按钮保留已有 OAuth2 认证逻辑：未连接时使用 Plug icon，点击走 `startCloudOAuth`；已连接时同一按钮切换为“断开云端”并使用 Unplug icon，因断开接口待实现，本轮展示禁用态和 TODO title。
7. “切换到 Cloud 模式”入口位于“本机工作台”卡片右上角，仅在已连接云端时可用。
8. 下方卡片通过已有 `listWorkspaces()` 接口展示真实工作区列表项，不使用 TODO 工作区占位。
9. 工作区列表项右侧展示进入含义 icon，不绑定交互。
10. “打开工作区”按钮位于“工作区列表”卡片右上角，使用 FolderOpen icon，跳转到已有 `/agent/sessions` 路由。
11. Home 页 `/` 已改为真实 route，不再自动跳转到 agent/cloud 其他页面。
12. 右下角全局 agent/cloud 模式切换元素已移除。
13. 工作区列表下方 cloud 模式预留说明文案已移除。
14. 字号按参考指南收敛：标题使用 `text-lg font-semibold`，非标题使用 `text-sm` 且不加粗。

## Spec alignment

不适用。light / 轻量模式未创建单独 Spec / 规格文档，按 Requirement / 需求核对。

## Plan alignment

不适用。light / 轻量模式未创建单独 Plan / 计划文档，按 Requirement / 需求和实现过程核对。

## Actual diff summary

实际涉及文件：

- `docs/requirement/20260708-home-agent-mode-frontend.md`
  - 更新需求、验收、决策，记录最终交互：顶部状态 icon 纯展示，显式 Cloud 入口在“本机工作台”卡片右上角。
- `docs/verification/20260708-home-agent-mode-frontend.md`
  - 新增本验证文档。
- `web/src/App.vue`
  - 移除全局 `ModeSwitcher` 渲染。
- `web/src/components/dashboard/CloudAccountConnectionMenu.vue`
  - 简化为纯状态 icon button：已连接 Cloud / 未连接 CloudOff。
- `web/src/components/dashboard/DisplayControls.vue`
  - 新增/调整语言与主题 icon-only 单击切换按钮，并去除顶部工具栏大边框。
- `web/src/i18n.ts`
  - 新增 Home 页文案、工作区入口文案、Cloud 模式切换文案、断开云端 TODO 文案。
  - 清理不再使用的 cloud 预留或 dropdown 相关文案。
- `web/src/router/index.ts`
  - `/` 改为真实 Home route，并加入 auth whitelist，避免自动跳转到 agent/cloud 其他页面。
- `web/src/views/agent/DashboardView.vue`
  - 重构 Home agent 模式页面：顶部工具栏、本机工作台卡片、工作区列表卡片、真实工作区加载、显式 Cloud/Workspace 入口、字号规范收敛。

## Expected vs actual changed files

| 文件 | 预期 | 实际 | 结论 |
| --- | --- | --- | --- |
| `docs/requirement/20260708-home-agent-mode-frontend.md` | 记录需求调整 | 已更新 | 符合 |
| `docs/verification/20260708-home-agent-mode-frontend.md` | 验证阶段文档 | 已创建 | 符合 |
| `web/src/App.vue` | 移除右下角全局切换元素 | 已移除 `ModeSwitcher` | 符合 |
| `web/src/components/dashboard/CloudAccountConnectionMenu.vue` | 顶部云端状态 icon | 已实现 | 符合 |
| `web/src/components/dashboard/DisplayControls.vue` | 语言/主题 icon button | 已实现 | 符合 |
| `web/src/i18n.ts` | 文案增删 | 已更新 | 符合 |
| `web/src/router/index.ts` | Home 不自动跳转 | 已更新 | 符合 |
| `web/src/views/agent/DashboardView.vue` | 页面主体实现 | 已更新 | 符合 |

## Acceptance checklist

- [x] Home 页 agent 模式存在顶部工具栏。
- [x] 左上展示 base64 占位 LOGO。
- [x] 右上展示远程连接状态 icon，已连接/未连接动态切换。
- [x] 右上复用语言切换与主题切换，均为 icon button，单击切换，不展示 dropdown。
- [x] 顶部工具栏按钮无大边框。
- [x] 主体区域居中，包含上下两个卡片。
- [x] 上方卡片标题为“本机工作台”。
- [x] 上方卡片展示本机设备、当前用户、项目版本三列。
- [x] “连接云端”按钮保留 OAuth2 认证逻辑，未连接时可用且使用 Plug icon。
- [x] 已连接时连接按钮切换为“断开云端”禁用态，并使用 Unplug icon，断开接口明确待实现。
- [x] “切换到 Cloud 模式”入口只在已连接云端时可用。
- [x] 下方卡片通过已有工作区接口展示真实工作区列表项。
- [x] 工作区列表项右侧进入 icon 不绑定交互。
- [x] “打开工作区”按钮使用 FolderOpen icon 并跳转 `/agent/sessions`。
- [x] `/` Home 页不会自动跳转到 agent/cloud 其他页面。
- [x] 右下角全局 agent/cloud 切换元素已移除。
- [x] 工作区列表下方 cloud 模式预留说明文案已移除。
- [x] 未引入新的前端组件库。
- [x] 字号规范符合参考指南：标题 `text-lg font-semibold`，非标题 `text-sm` 且不加粗。

## Command results

### Type check

```text
npm --prefix web run typecheck

> termbridge-web@0.1.0 typecheck
> vue-tsc --noEmit
```

结果：通过。

### Font consistency search

检查命令语义等价于参考指南建议的模式，范围为本次新增/修改的 Home 页面与 dashboard 子组件：

```text
text-(xs|base|xl|2xl|\[[^\]]+\])|font-(medium|semibold|bold|extrabold)
```

结果：仅命中允许项：

- 页面根 `text-sm` 兜底；
- 标题 `text-lg font-semibold`；
- 普通文本 `text-sm`；
- 设计 token 类如 `text-[var(--color-...)]`。

未发现非标题 `font-medium`、`font-semibold`、`font-bold`，未发现 `text-xs`、`text-base`、`text-xl`、`text-2xl`。

## Missed or expanded scope

- 未新增后端接口，符合范围。
- 未实现断开云端接口，仅按需求展示禁用态和 TODO title，符合“接口待实现”。
- 未为工作区列表项右侧进入 icon 添加交互，符合范围。
- “打开工作区”按钮跳转到已有 `/agent/sessions`，未新增路由。

## Risks

1. `项目版本`仍为 TODO 占位，因为当前未接入版本信息来源。
2. 本机设备信息使用 `cloud_session.device_name || device_id`；未连接云端时仍使用 TODO 占位，后续如有独立本机设备接口可替换。
3. 断开云端接口待实现，当前“断开云端”按钮为禁用态，不会产生实际断开行为。
4. 本验证以类型检查和静态 diff/样式规则核对为主，未在浏览器中执行视觉或 OAuth2 端到端流程。

## Incomplete items

- 断开云端接口未实现，按钮仅展示禁用态。
- 项目版本真实数据源未接入。

## Conclusion

本轮 Home 页 agent 模式纯前端实现与轻量模式 Requirement 对齐。类型检查通过；字号规范按参考指南核对通过；未发现超出本轮范围的新增后端接口或新组件库依赖。实现可进入用户验收。
