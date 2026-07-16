# Spec / 规格：VS Code 工作台样式对齐

最后修改时间: 2026-07-16 13:05:05

## Review status / 审查状态

Accepted

## Mode / 模式

轻量模式 / light（用户要求补充 Spec / 规格；本阶段产出技术方案，仍不写产品代码，直至进入实现）

## Requirement basis / 需求依据

基于 `docs/requirement/20260716-vscode-workbench-styling.md`（已 Accepted，并已按用户澄清修订）。

本 Spec 把需求中的视觉目标与**导航模型**落到现有 Vue 组件、路由、全局 CSS、图标依赖上；**不改变**文件、会话、Git、Monaco 或运行时 API 业务语义。

**澄清（2026-07-16）**：目录/文件工作台通过 **路由切换**进入与退出，**不再**在 `/sessions`（及 cloud sessions）内做 drawer / pane / overlay 同页 UI 转换。此前「pane replacement + closed 契约」方案作废，由路由导航取代。

## Overview / 概览

在现有文件工作台结构上做**呈现 + 导航**重构：

1. **Explorer 列表节奏**：目录树 / Git 变更行去掉描边卡片，改为低对比背景 hover + 持久 selected（active 文件）。
2. **连续编辑器 tab strip**：文件标签去掉 pill/card 间距与四边框，活跃 tab 与编辑器 canvas 同系连续。
3. **平面侧栏**：工作区/会话侧栏行弱化圆角描边，保留 Lucide。
4. **图标分区**：目录树视图全面 `@vscode/codicons`；工作区侧栏继续 `@lucide/vue`；文件 tab close 可用 Codicon。
5. **路由化文件工作台**：打开文件/目录视图 → 导航到独立路由；返回会话 → 导航回 sessions 路由；拆除同页叠层状态机（`fileDrawerOpen` / `inert` / `closed` 主路径）。
6. **色阶与边界**：工作台相关表面偏中性；蓝色优先用于 focus / selected；token 变更优先限制在工作台选择器内。

验收取向：信息密度与交互层级接近 VS Code Explorer + 编辑器 chrome，**非像素级复刻**。

## Design decisions / 设计决策

### D1. 改动重心：路由导航 + 全局 CSS + 少量组件 class/图标

| 决策 | 说明 |
|------|------|
| 导航 | 文件工作台为独立页面路由，不是 sessions 内的绝对定位叠层 |
| 样式主战场 | `web/src/styles.css` 中 `.file-workbench-*` 及侧栏相关工具类；删除或停用 drawer 叠层专用样式 |
| 组件内 Tailwind | `WorkspaceSessionSidebar.vue` 行级平面化 |
| 不换 UI 库 | 继续 reka-ui Tabs、Monaco、Pinia |
| Token | 优先复用 `--color-*`；缺省时新增工作台专用 token；避免全局误伤非工作台页 |

### D2. 状态语言（共享）

| 状态 | 视觉规则 |
|------|----------|
| default | 透明/中性；无描边卡片；无强阴影 |
| hover | 仅低对比背景；不靠 border 表达 hover |
| selected / active | 持久背景，与 hover 可区分 |
| focus-visible | `outline` + `outline-offset` |
| drag | 可短暂 elevation；松手后平面 |
| danger | 危险 action 用 danger token |

目录树 **selected**：行 path === `fileWorkbench.activeDocument.path`。不在树内复制 active 状态。

### D3. 文件 Tab strip

| 现有 | 目标 |
|------|------|
| gap + 独立 border/radius 卡片 | 连续 strip，`gap: 0`；无 pill 四边框 |
| active 描边卡片 | active 与 editor canvas 同系背景连续 |
| close 用 Lucide `X` | 可改为 Codicon `close` |
| 高度偏松 | 略减（约 35–40px 量级），保持可点 |

不改 reka-ui Tabs 的 activate/close 与 dirty 语义。

### D4. 目录树 Explorer + Codicon

| 现有 | 目标 |
|------|------|
| 行描边 + 圆角 | 无行边框；hover/selected 仅背景 |
| 无 selected 样式 | `.file-workbench-tree-row--active` 绑定 `activeDocument` |
| 全树 Lucide | 工具栏 + 行图标 + 行内 action 全面 Codicon |

依赖：`@vscode/codicons` + 入口引入 font CSS；轻量 `Codicon` 封装仅用于目录树视图与允许的编辑器 chrome。

Codicon 映射建议：`chevron-right/down`、`loading`、`folder`/`folder-opened`、`file`、`new-file`/`new-folder`、`refresh`、`arrow-left`、`edit`、`trash` 等。

### D5. 工作区/会话侧栏平面化

范围：`WorkspaceSessionSidebar` 列表行 + drag ghost/chosen/dragging。图标 **保留 Lucide**。

打开文件入口：由「同页 `open-files` 触发 drawer」改为 **router.push 到文件工作台路由**（携带 workspace id）。

### D6. 文件工作台路由化（替代 pane / drawer / closed）

#### 目标导航模型

```text
Sessions 路由                          Files 工作台路由
┌─────────────────────┐   open-files   ┌──────────────────────────┐
│ Sidebar (Lucide)    │ ─────────────► │ File tree (Codicon)      │
│ Session workbench   │                │ Tabs + Monaco + Git      │
└─────────────────────┘ ◄───────────── └──────────────────────────┘
                         back / router
```

#### 建议路由（实现可微调命名，须 local + cloud 对称）

| 模式 | Sessions（已有） | Files 工作台（新增） |
|------|------------------|----------------------|
| local | `/sessions` (`local-sessions`) | `/workspaces/:workspaceId/files`（建议名 `local-workspace-files`） |
| cloud | `/devices/:deviceId/sessions` | `/devices/:deviceId/workspaces/:workspaceId/files`（建议名 `cloud-workspace-files`） |

约束：

- `workspaceId` 必须可从路由恢复，以支持刷新深链。
- `meta.mode` 与现有 local/cloud 守卫一致。
- 返回：`router.push` 到对应 sessions 路由（保留 deviceId）。
- 可选：query 记录 `from` 或返回 focus 目标；**不要求**复刻旧 trigger 元素 focus（页面级导航后焦点落在返回控件或主区域即可，需可访问）。

#### 从 SessionsPageShell 拆除的主路径

现有同页状态机（应移除或降为非主路径并清理）：

- `fileDrawerOpen` / `fileViewActive` / `fileDrawerWorkspace`
- `WorkspaceFileDrawer` 叠在 `SessionWorkbench` 上
- `session-workbench-stage--files-open` + `inert`
- `sessions-shell--files-open` 侧栏 collapse
- `@closed` → `finishClosingWorkspaceFiles` + transform `transitionend`

打开入口改为：

```ts
// 概念：侧栏 open-files
router.push(
  mode === 'local'
    ? { name: 'local-workspace-files', params: { workspaceId } }
    : { name: 'cloud-workspace-files', params: { deviceId, workspaceId } },
)
```

#### 页面组合

新建 view（或复用抽取后的 page shell），例如：

- `web/src/views/local/WorkspaceFilesView.vue`
- `web/src/views/cloud/WorkspaceFilesView.vue`

或单一 `WorkspaceFilesView` + props/runtimeTarget 工厂（与 SessionsView 对称）。

页面内挂载现有能力（可继续用 `WorkspaceFileDrawer` 的**内容布局**，但：

- 去掉 absolute drawer / open prop 驱动的离场动画主路径；
- 或重构为 `WorkspaceFileWorkbench` 纯页面布局组件（**推荐命名澄清**，避免 drawer 语义残留）；
- 不再 emit 页面级 `closed` 作为离开会话的完成信号；离开 = 路由返回。

`fileWorkbench` store 仍按 workspace 打开文档；进入路由时 `openWorkspace(target, workspaceId)`，离开时可保留 store 缓存或按现有策略清理（**不改变**文档 dirty 业务规则；若离开即销毁实例，须确认 dirty 文档策略与现状一致——默认：保持 store 单例行为，与现网一致，避免静默丢 dirty）。

#### 与「样式任务」的边界

路由化属于本需求「进入/退出呈现」的修订范围，**允许**改 router 与 SessionsPageShell 导航接线；仍 **禁止**改文件/Git API 与 Monaco 生命周期语义。

### D7. 编辑器 chrome / 提示条

冲突/警告/错误条更紧凑；工具栏/status 与树 header 节奏对齐；Monaco 为主画布。

### D8. Git 面板

与树同行平面列表样式；属文件工作台页内时图标优先 Codicon。

## Affected files / components / 受影响文件 / 组件

### 预期修改

| 路径 | 变更类型 |
|------|----------|
| `web/src/router/index.ts` | 新增 local/cloud 文件工作台路由 |
| `web/src/views/local/*` / `web/src/views/cloud/*` | 新增 WorkspaceFiles 视图（或共享 view） |
| `web/src/components/session/SessionsPageShell.vue` | 移除叠层 drawer 主路径；`open-files` 改为 router 导航 |
| `web/src/components/workspace/WorkspaceSessionSidebar.vue` | 列表平面化；open-files 仍 emit，由 shell/view 导航 |
| `web/src/components/workspace/WorkspaceFileDrawer.vue` | 改为页面布局组件或剥离 drawer open/closed；内容复用 |
| `web/src/components/workspace/WorkspaceFileTree.vue` | Codicon + active 行；back → 路由返回会话 |
| `web/src/components/workspace/WorkspaceEditorTabs.vue` | strip 样式；close 可选 Codicon |
| `web/src/components/workspace/GitChangesPanel.vue` | 样式/图标对齐 |
| `web/src/styles.css` | 树/tab 平面化；删除或停用 drawer 叠层阴影/transform 专样式 |
| `web/package.json` | `@vscode/codicons` |
| 新建 Codicon 封装 | 目录树/编辑器 chrome |
| 相关 `*.test.ts` | 导航与组件拆叠层；删除「仅 transform 才 closed」主路径用例 |

### 明确不改（除非阻塞）

- `fileWorkbench` 文档/dirty/保存业务逻辑（仅只读 active 做样式）
- 文件/Git API、proto
- Dashboard / Shortcuts / 终端 Lucide
- 全局 `.button-secondary` 语义

## Data model / interfaces / 数据模型 / 接口

**无后端/proto 变更。**

### 路由参数

```ts
// local
params: { workspaceId: string }
// cloud
params: { deviceId: string; workspaceId: string }
```

### 废弃的页面级契约（主路径）

```ts
// 不再作为 sessions ↔ files 切换主机制
emit('closed')  // drawer 离场完成
fileDrawerOpen / fileViewActive / inert 叠层编排
```

### 保留的组件内契约

- tab activate/close
- tree action / open file
- dialog 确认流（脏关闭、删除等）

### 目录树 active（展示）

```ts
const isActiveFile = (path: string) => store.activeDocument?.path === path
```

## Open technical questions / 待定技术问题

**无阻塞用户决策。** 实现默认：

| # | 问题 | 默认 |
|---|------|------|
| 1 | 精确 path 字符串 | 上文建议 path；若与现有命名惯例冲突可微调，但须对称 local/cloud |
| 2 | Drawer 组件是重命名还是掏空 | 推荐抽出 `WorkspaceFileWorkbench` 页面布局；旧 drawer 测试改为页面/布局测试 |
| 3 | 离开 files 路由是否清空 documents | **默认保持现有 store 生命周期**，不借机改 dirty 语义 |
| 4 | 深链 workspace 不存在 | 提示错误并提供回 sessions 链接（与现有错误展示风格一致） |
| 5 | Codicon 全量 CSS | 官方全量，构建验证 |

## Risks and trade-offs / 风险与权衡

| 风险 | 缓解 |
|------|------|
| 双轨：路由已上但 drawer 残留 | 实现时一次拆除 SessionsPageShell 叠层主路径 |
| 刷新深链丢 workspace | workspaceId 进 path；进入时 `openWorkspace` |
| dirty 文档跨路由 | 保持 pinia store，不因路由卸载误清 |
| local/cloud 路由遗漏 | 成对添加与守卫 meta |
| 焦点不再回到原按钮 | 页面级导航可接受；back 按钮可聚焦 |
| 样式任务变导航任务，diff 变大 | 范围已由用户澄清纳入；仍不扩 API |
| 双图标系统 | 需求已接受分区 |

## Alternatives considered / 已考虑替代方案

| 方案 | 结论 |
|------|------|
| `/sessions` 内 pane replacement + closed | **否决**（用户澄清：改路由） |
| 全站 Codicon | 否决 |
| code-server / VS Code Workbench | 否决 |
| 仅样式不改导航 | 否决（与澄清冲突） |
| query-only `?files=1` 同页切换 | 否决：仍是同页 UI 转换；要求独立路由视图 |

## Implementation sketch / 实现草图

1. 新增 local/cloud files 路由 + view 壳，挂载工作台布局。
2. `open-files` / back 改为 router 导航；拆除 shell 叠层状态机与 drawer 专样式。
3. 依赖 Codicon + 封装；树图标与 active 行。
4. `styles.css`：tab/tree/侧栏平面化与密度。
5. Git 面板与 chrome 提示紧凑化。
6. 更新/替换 drawer closed 测试；补路由导航相关测试；dark/light 目检。

## Acceptance mapping / 验收映射

| 需求验收主题 | Spec 落点 |
|--------------|-----------|
| 连续 tab strip | D3 |
| 树 Explorer + 持久 selected | D2/D4 |
| 侧栏平面 + Lucide | D5 |
| 目录树 Codicon | D4 |
| **路由进入/退出 files** | **D6** |
| 色阶/边界 | D1 |
| 无业务 API 回归 | store/API 不动 |

## User review notes / 用户审查记录

- 2026-07-16：用户要求进入 Spec；Requirement 同步 Accepted。
- 2026-07-16：用户澄清——目录视图 **切换路由**，不再在 `/sessions` 内做 UI 转换；Spec 以路由化替代 pane/closed 方案。
- 2026-07-16：用户**采纳** Spec / 规格；Review status → Accepted。
