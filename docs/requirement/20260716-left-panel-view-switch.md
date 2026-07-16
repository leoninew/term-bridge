# 工作区/目录视图左栏切换入口

最后修改时间: 2026-07-16 21:21:09

Review status: Accepted

## Flow mode / Stage

轻量模式 / light；阶段：Requirement / 需求。

## Background

当前前端已有两个左右结构的主工作视图：

1. **工作区视图**（`/sessions`，local；`/devices/:deviceId/sessions`，cloud）
   - 左 panel：`WorkspaceSessionSidebar`（工作区 + 会话树）
   - 右 panel：`SessionWorkbench`（终端 tabs / PTY）
   - 进入目录视图的现有入口：每个工作区节点 hover 时的 `openFiles` 按钮（`FileCode2`），由 `SessionsPageShell.openWorkspaceFiles` 路由跳转到 files 页。

2. **目录视图**（`/workspaces/:workspaceId/files`，local；`/devices/:deviceId/workspaces/:workspaceId/files`，cloud）
   - 左 panel：`WorkspaceFileTree`（目录树 + 新建/刷新等工具）
   - 右 panel：文件 tab + Monaco 编辑区
   - 返回工作区视图的现有入口：目录树 header 左侧 `arrow-left`（`files.backToSessions`），经 `WorkspaceFilesPageShell.goBackToSessions` 路由返回 sessions。

两个视图已通过**路由切换**解耦（见 `20260716-vscode-workbench-styling` 决策），但左 panel 缺少统一、对称的视图切换图标入口：工作区侧主要是“按工作区打开文件”，目录侧是“返回箭头”，语义与位置不一致，发现成本偏高。

## Goal

1. 在两个视图左 panel 的**右下角**提供视图切换 icon 按钮：
   - **工作区视图**（`/sessions`）左 panel 右下角：展示切换到**目录视图**（`/workspaces/:workspaceId/files`）的 icon 按钮。
   - **目录视图**（`/workspaces/:workspaceId/files`）左 panel 右下角：展示切换到**工作区视图**（`/sessions`）的 icon 按钮。
2. 切换目标工作区以**当前打开的 tab 所属工作区**为准：
   - 工作区视图：以当前活动终端 tab 所属 `workspaceId` 为准。
   - 目录视图：当前 files 路由本身已绑定 `workspaceId`，返回工作区视图始终可用（不依赖是否已打开文件 tab）。
3. **没有活动 tab 时，切换按钮不可用（disabled）**。
4. 切换继续走现有路由导航，不回到 `/sessions` 内 drawer/pane UI 切换。
5. local / cloud 双路由均可用，行为一致。
6. **本阶段不实现跨路由 tab 状态保存/恢复**（因两视图路由分离，跨页 tab 状态先不做）。
7. 图标分区策略与既有决策一致：工作区侧栏继续 Lucide；目录树侧栏可继续 Codicon（或与现有按钮风格一致）。
8. 保持可访问性：`aria-label` / `title`、键盘可聚焦；disabled 时也应有明确可访问名称/提示。

## Non-goal

1. 不新增第三种主路由视图，不引入完整 VS Code Activity Bar 多视图系统。
2. 不改变文件读写、会话生命周期、终端、Git、Monaco 业务语义。
3. **不实现跨路由 tab 状态保存/恢复**（工作区 tabs 与文件 tabs 在切换后不要求保留/还原）。
4. 不移除现有 per-workspace `openFiles` 入口或目录树 header 现有返回按钮（除非后续单独收敛）。
5. 不做全站导航重构或全局图标库统一。

## User scenarios

1. 用户在工作区视图打开了某个 session tab，点击左 panel 右下角目录视图切换 icon，进入该 tab 所属 workspace 的目录视图。
2. 用户在目录视图打开了某个文件 tab，点击左 panel 右下角工作区视图切换 icon，返回工作区/会话视图。
3. 用户在工作区视图没有任何活动 tab（或无打开 tab）时，目录视图切换 icon 为 disabled。
5. 用户刷新目录视图深链后，若有活动文件 tab（或按当前路由/工作台状态可判定活动 tab），切换按钮可用；否则 disabled。

## Acceptance criteria

- [ ] 工作区视图左 panel **右下角**可见“切换到目录视图”icon 按钮。
- [ ] 目录视图左 panel **右下角**可见“切换到工作区视图”icon 按钮。
- [ ] 切换目标 workspace 取自**当前活动 tab 所属工作区**。
- [ ] 工作区视图无活动 session tab 时目录切换按钮 disabled；目录视图返回工作区按钮始终可用。
- [ ] 点击可用按钮后：
  - 工作区 → 目录：进入对应 files 路由（local/cloud 正确携带 `workspaceId` / `deviceId`）。
  - 目录 → 工作区：进入对应 sessions 路由（local/cloud 正确携带 `deviceId`）。
- [ ] **不实现**跨路由 tab 状态保存/恢复；切换后不要求恢复另一视图的 tabs。
- [ ] 按钮具备可访问名称与 hover title；disabled 状态可感知；不破坏左 panel 现有布局、footer 设置区、拖拽与滚动。
- [ ] 不引入 sessions 内 drawer/pane 作为主切换路径。

## Scope notes / 实现落点（需求级）

| 视图 | 左 panel 组件 | 建议落点 |
|------|---------------|----------|
| 工作区 `/sessions` | `WorkspaceSessionSidebar` + `SessionsPageShell` | 左 panel 右下角（footer 区域右侧）新增目录视图切换 icon；目标 `workspaceId` 来自当前活动 session tab；无活动 tab 则 disabled |
| 目录 `/workspaces/:workspaceId/files` | `WorkspaceFileTree` + `WorkspaceFilesPageShell` / `WorkspaceFileWorkbench` | 左 panel 右下角新增工作区视图切换 icon；有活动文件 tab 才可切换；无活动 tab 则 disabled；导航继续 `goBackToSessions` 一类路由返回 |

## Decisions

- 采用轻量模式 / light。
- 继续路由切换，不回退 drawer/pane。
- **按钮位置**：两个视图左 panel 的**右下角**。
- **目标工作区**：以当前打开/活动 tab 所属工作区为准。
- **无活动 tab**：切换按钮 **disabled**。
- **跨路由 tab 状态**：本阶段**不做**保存与恢复。
- 现有 per-workspace `openFiles` 与目录树 header 返回箭头：本需求不强制移除，作为并行入口保留，除非后续单独收敛。

## Risk / Assumptions

- 目录视图“活动 tab”指文件编辑区当前 `activeDocument`/活动文件 tab；若路由进入后尚未打开任何文件，按钮 disabled。
- 工作区视图“活动 tab”指 `workbench.activeTab` / `activeSessionId` 对应会话所属 workspace。
- 因不做跨路由 tab 状态保存，用户从目录返回工作区后，终端 tabs 保持工作区页自身状态（Pinia/store 若仍存活则可能保留，但本需求不新增跨路由持久化契约）。
- 双图标系统（Lucide / Codicon）短期共存，与 VS Code 样式需求一致。

## User review notes

- 2026-07-16：用户要求理解 `/sessions` 与 `/files` 左右结构，并在左 panel 增加视图切换 icon。
- 2026-07-16：用户确认按钮放在左 panel **右下角**；目标工作区以**当前打开 tab 所属工作区**为准；**无活动 tab 时按钮不可用**；因路由不同，**先不实现 tab 状态保存**。

