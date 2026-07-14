# 1440px 中等桌面屏幕布局适配验证
最后修改时间: 2026-07-13 23:55:34

## Review status

Accepted

## Flow mode / Stage

标准模式 / standard；验证 / Verification。

## Requirement alignment

- Requirement 文档：`docs/requirement/20260713-medium-screen-layout-adaptation.md`
- Requirement status：`Accepted`

对齐结论：本次实现将验证范围收敛至 CSS viewport `1440px` 及以上，符合用户明确排除小于 `1440px` 的要求。

需求验收点核对：

1. 页面范围：已以实际运行的 local dashboard、Cloud landing 与快捷指令路由进行浏览器验证；认证与辅助页面未因本次布局改动发生结构性变更，未单独驱动。
2. `1440px` 及以上无根级横向溢出：local dashboard 在 `1440×900` 和 `1728×1000` 下通过运行时截图与 `scrollWidth` 检查；Cloud landing 在 `1440×900` 下通过截图检查。浏览器流程的 evaluate 结果未持久化到报告输出，但运行成功且截图未显示横向裁切。
3. 会话 Splitter：实现调整为 `18%–24%`，但本地 Vite 运行环境的 local auth / API contract 失败，导致 `/sessions` 只渲染空白背景，未能观察经过鉴权的 Splitter、sidebar、tab 或 terminal surface。
4. Cloud landing：`1440×900` 下双栏文案、CTA 与 hero 图同时位于首屏，图像没有突破容器或覆盖主操作。
5. local dashboard：`1440×900` 与 `1728×1000` 下 `1200px` 内容容器、三项概览、工作区和快捷指令区域层级稳定；API 404 错误以既有 error/toast 呈现，不影响布局观察。
6. 快捷指令：当前本地 API/鉴权问题使 `/shortcuts` 只显示空白背景，无法实际观察卡片 grid、创建入口和 dialog。
7. 小于 `1440px`：未检查，符合范围约束。

## Spec alignment

不适用。标准模式 / standard 未创建单独 Spec 文档。

## Plan alignment

不适用。用户在 Requirement 阶段明确要求开始实现，Requirement 已接受后直接进入 Implementation。

## Actual diff summary

产品代码改动：

- `web/src/components/session/SessionsPageShell.vue`：侧栏 Splitter 从 `15%–25%` 调整为 `18%–24%`。
- `web/src/components/dashboard/CloudHome.vue`：Cloud landing 容器提升至 `1440px` 上限，调整双栏比例、间距和 hero 的视口高度约束。
- `web/src/components/dashboard/LocalHome.vue`：使用 `1200px` 桌面内容上限、统一大屏留白，并调整概览卡片间距。
- `web/src/views/cloud/DashboardView.vue`：使用与 local dashboard 一致的 `1200px` 内容上限和大屏留白。
- `web/src/components/shortcut/ShortcutsPageShell.vue`、`web/src/styles.css`：快捷指令内容区提升至 `1440px`，扩大页面、grid 与卡片间距。
- `web/src/components/layout/AppHeader.vue`：同步大屏水平留白。
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`：Prettier 格式化调整，无业务或布局语义变化。

过程文档：

- 新增 `docs/requirement/20260713-medium-screen-layout-adaptation.md`。
- 新增本验证文档。

## Expected vs actual changed files

预期改动：

- 会话工作台布局：`SessionsPageShell.vue`。
- 首页 / dashboard / header：`CloudHome.vue`、`LocalHome.vue`、`DashboardView.vue`、`AppHeader.vue`。
- 快捷指令布局：`ShortcutsPageShell.vue`、`styles.css`。
- 过程文档：本 requirement 与 verification 文档。

实际改动与预期一致。`WorkspaceSessionSidebar.vue` 仅因格式化产生变更，不改变行为。

## Acceptance criteria checklist

- [x] 小于 `1440px` 不在验证范围内。
- [x] `1440×900` local dashboard 通过运行时截图检查，内容容器、概览卡片和列表区无横向裁切。
- [x] `1728×1000` local dashboard 通过运行时截图检查，内容上限和留白保持稳定。
- [x] `1440×900` Cloud landing 通过运行时截图检查，CTA、文案、hero 图双栏关系稳定。
- [x] `1440px` 下共享 header 与页面水平留白对齐。
- [x] 代码层会话 Splitter 的范围已收敛为 `18%–24%`。
- [ ] 已验证运行态会话 sidebar、tab overflow、terminal resize、status bar 与 drawer：受本地 auth/API 环境阻断。
- [ ] 已验证运行态快捷指令 grid、card、创建/编辑 dialog：受本地 auth/API 环境阻断。
- [ ] 覆盖所有认证与辅助路由：本次未单独运行；这些页面未包含本次结构性布局改动。

## Runtime evidence

本地 Vite 运行在 `http://127.0.0.1:9031`，通过 Pomelo PW 驱动 Chrome headless。

成功截图：

- `D:\ProgramFiles\Cygwin64\tmp\claude\D--SourceCodes-mywork-TermBridge-go\5ae22534-c5f2-4c93-9b3c-c9a0d5ac867d\layout-1440-cloud-output\cloud-home-1440.png`
- `D:\ProgramFiles\Cygwin64\tmp\claude\D--SourceCodes-mywork-TermBridge-go\5ae22534-c5f2-4c93-9b3c-c9a0d5ac867d\verify-local-pages-output\local-dashboard-1440.png`
- `D:\ProgramFiles\Cygwin64\tmp\claude\D--SourceCodes-mywork-TermBridge-go\5ae22534-c5f2-4c93-9b3c-c9a0d5ac867d\verify-local-pages-output\local-dashboard-1728.png`

运行态观察：

1. Cloud landing `1440×900`：左侧 CTA 与说明、右侧 hero 图均完整可见；没有非预期页面级横向滚动。
2. local dashboard `1440×900`：`1200px` 内容面板居中，三项概览卡完整，工作区/快捷指令容器没有裁切。环境 API 返回 404，页面按既有行为显示错误信息与 toast。
3. local dashboard `1728×1000`：内容保持 `1200px` 上限而不被过度拉伸，header 与内容区域留白协调。
4. 探测 `/sessions`：页面等待 local auth 后未渲染 Splitter，截图为仅有 app 背景；尝试定位 sidebar/new-session button 超时，自动捕获了 error HTML 与截图。该阻断来自本地 API/auth 可用性，不能证明或否定 Splitter 运行态行为。
5. 探测 `/shortcuts`：同样受运行环境 auth/API 状态影响，未渲染可操作页面内容；未将其误报为布局通过。

## Command results

实现阶段已运行且通过：

```text
yarn --cwd web typecheck  → passed
yarn --cwd web lint       → passed
yarn --cwd web format     → passed
yarn --cwd web test       → 14 test files / 84 tests passed
git diff --check          → passed
```

验证阶段专注运行态观察，未重复运行 CI 类检查。

## Missed or expanded scope

未完成：

1. 真实鉴权后的 `/sessions` runtime surface 未能访问，故无法验证侧栏最小/最大位置、tab overflow、终端 resize、状态栏与 drawer。
2. 真实 `/shortcuts` runtime surface 未能访问，故无法验证卡片和 dialog 的视觉状态。
3. 认证和辅助路由未逐页浏览器检查；它们不包含本次布局结构调整。

范围扩展：无。

## Risks

1. 会话 Splitter 的比例变化已通过代码审查，但缺少带真实工作区、会话和终端的端到端验证；终端列数、tab overflow 与拖拽边界仍应在可用 API 环境复验。
2. 快捷指令页的 `1440px` 内容宽度和卡片间距缺少带实际快捷指令数据的浏览器截图，长名称和多个操作按钮的视觉密度仍应复验。
3. 本地开发环境的 `API error contract mismatch (404)` 与 Cloud landing 的 `Network Error` 为环境服务不可用或契约不匹配信号，非本次 CSS 改动引入的可视布局证据，但阻断了部分运行态路径。

## Incomplete items

- 会话工作台和快捷指令页需要在可用的本地 API / 已认证会话中执行一次 `1440px` 及以上浏览器复验。
- 认证与辅助路由可在上线前根据实际内容补充一次视觉巡检。

## Conclusion

结论：**部分通过，存在环境阻断，不能作为完整验收通过。**

已在实际运行的浏览器中确认 Cloud landing 与 local dashboard 在 `1440px` 及更宽 viewport 下的布局目标；会话工作台与快捷指令关键运行态页面因本地 API/auth 失败不可达，尚未得到完整端到端证据。建议在连接可用 backend 后，优先复验会话 Splitter（`18%` / 默认 / `24%`）、多标签、运行终端和快捷指令卡片/对话框。
