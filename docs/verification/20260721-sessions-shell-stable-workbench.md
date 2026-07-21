# 会话壳布局切换时保持终端 Workbench 实例 — 验证

最后修改时间: 2026-07-21 21:39:41

Review status: Draft

## Flow mode / Stage

轻量模式 / light；验证 / Verification

## Requirement alignment

验证对象：

- [docs/requirement/20260721-sessions-shell-stable-workbench.md](../requirement/20260721-sessions-shell-stable-workbench.md)（Accepted）

| 需求要点 | 对齐结果 |
| --- | --- |
| 窄/宽切换时同一 `SessionWorkbench` 实例保持挂载 | **对齐**：去掉 `v-if isNarrow` / `v-else` 双 workbench；终端 panel 内唯一挂载 |
| 布局切换不主动关闭/重建终端 WS | **结构对齐**：`isNarrow` 只切换侧栏 chrome；workbench 不随分支卸载。真机 DevTools 未在本机跑通 |
| controller 连接未断时不因壳切换降为 observer | **结构对齐**（依赖上条）；真机横竖屏手测未完成 |
| 宽屏 Splitter / 窄屏 overlay 行为不回退 | **代码对齐**：宽屏 panel+handle；窄屏 backdrop+drawer 接线保留 |
| 不改后端 multi-attach / take_control | **对齐**：diff 无 Go/proto |
| 不做 sticky controller / 断点滞回 | **对齐**：未引入 |
| Decision 6 优先：workbench 留在终端侧 SplitterPanel | **已采用**；未触发「提到 Splitter 外」兜底 |

## Spec alignment

不适用（轻量模式无独立 Spec）。按 requirement / 需求核对。

## Plan alignment

不适用（轻量模式无 Plan）。实现依据 requirement Decisions 1–6。

## Actual diff summary

相对 `develop` 工作区（本 feature）：

| 路径 | 变更 |
| --- | --- |
| `web/src/components/session/SessionsPageShell.vue` | 单一 `SessionWorkbench` 宿主；`SplitterGroup` 常驻；窄屏 overlay 侧栏外置；宽屏侧栏 `v-if="!isNarrow && !sidebarCollapsed"` |
| `docs/requirement/20260721-sessions-shell-stable-workbench.md` | 过程文档（新增） |
| `docs/verification/20260721-sessions-shell-stable-workbench.md` | 本验证文档 |

统计（代码）：约 1 个 Vue 文件，`+`/`-` 量级 ~34/36 行（含注释与 Prettier 整形）。

无关工作区改动：无其它 tracked 文件混入本 feature。

## Expected vs actual changed files

| 预期（Requirement） | 实际 |
| --- | --- |
| 主改 `SessionsPageShell.vue` | 是 |
| 必要时样式微调 | **未改** styles（现有 class 足够） |
| 不改 agent/cloud 控制权状态机 | 未改 |
| 不改 TerminalView / keep-alive / FAB | 未改 |

## Acceptance criteria checklist

- [x] `SessionWorkbench` 在模板中仅一处挂载（代码检索：template 内单节点 `#terminal-workbench` 内）
- [x] `isNarrow` 不再包住独立 workbench 分支（已改为侧栏 chrome 条件）
- [x] 布局切换路径上不再因双实例 remount 主动销毁 workbench（静态结构）
- [ ] 真机/浏览器：跨 768 横竖屏旋转时 live terminal WS **不** close+新建（DevTools）— **未跑**
- [ ] 真机：手机 controller 旋转后不出现接管条（连接未被系统杀掉时）— **未跑**
- [x] 窄屏 overlay / backdrop / 选 session 收起接线仍在
- [x] 宽屏 Splitter 侧栏 + resize handle + 折叠条件仍在
- [x] 无后端协议变更

## Test / command results

| 命令 | 结果 |
| --- | --- |
| `web/`: `npm run typecheck` | **通过** |
| `web/`: `npm run test -- src/composable/useSessionsLayoutMode.test.ts` | **通过**（2 tests） |
| `web/`: `npx eslint src/components/session/SessionsPageShell.vue` | **通过** |
| `web/`: `npx prettier ...SessionsPageShell.vue --check` | **通过**（验证阶段已 `--write` 整形） |
| `web/`: `npm run lint`（全量） | **失败（无关）**：`TerminalScrollFabs.vue` 中 `PointerEvent` `no-undef` ×3；非本 feature 引入 |
| 真机横竖屏 / DevTools WS | **未执行** |

## Missed or expanded scope

- **未扩展**：无 sticky controller、无断点滞回、无后端改动。
- **未完成手测**：Acceptance 中依赖设备/浏览器的两条仍 open。
- **范围偏差**：无。

## Risks

1. reka-ui 在侧栏 `SplitterPanel` 增减时若仍重建终端子树，则 Decision 6 兜底尚未启用；与既有「宽屏折叠侧栏」同路径，风险中等偏低。
2. 系统/浏览器真断 WS 仍会重选 controller（需求 Decision 5，可接受）。
3. 全量 `npm run lint` 当前被无关 `TerminalScrollFabs` 错误挡住；合并本 feature 时勿与该文件混修除非另开任务。

## Incomplete items

1. 真机或桌面 DevTools 模拟跨 768 旋转：确认 WS 连接 id 不变、无 observer 接管条。
2. 可选：多端同 session（电脑 observer + 手机 controller）下旋转手机，确认电脑不会被 auto-grant（连接未断时）。

## Conclusion

**有条件通过（static / CI 层）**：实现与 requirement 的结构目标一致，类型检查与相关单测通过，diff 范围干净。

**交付前建议**：用户做一次横竖屏手测勾选剩余 Acceptance；若手测失败且证实 Splitter 子树被拆，再按 Decision 6 把 workbench 提到 Splitter 外。

未授权 git 写操作；下方 commit message 仅为建议。
