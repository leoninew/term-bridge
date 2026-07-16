# VS Code 工作台样式对齐

最后修改时间: 2026-07-16 13:05:05

Review status: Accepted

## Flow mode / Stage

轻量模式 / light；规格 / Spec（Accepted）。实现 / Implementation 待用户启动。

> 用户要求进入 Spec / 规格阶段；本需求 Review status 同步为 Accepted。

## Background

当前工作区侧栏、目录树、文件标签栏和文本编辑区域已具备文件工作台结构，但整体仍偏向圆角卡片式 Web 应用：

- 目录树与会话行主要靠边框、圆角表达 hover / selection；
- 文件标签为带间隔的独立卡片（pill / card），与编辑器画布割裂；
- 文件工作台目前叠在 `/sessions` 会话区内（带阴影 drawer / 同页 UI 切换），与会话视图耦合，而非独立路由视图；
- 工作台表面蓝灰层级与分栏线偏多，密度低于常见桌面 IDE Explorer + 编辑器 chrome。

本需求只记录**视觉层级、信息密度、颜色、边界、图标分区与交互状态**的对齐工作，不改变文件、会话、Git、Monaco 或运行时功能语义。

“接近 VS Code”指 **Explorer 列表节奏 + 编辑器 chrome 的平面、紧凑、连续表面**，不是像素级复刻某一 VS Code 版本，也不引入 code-server / VS Code Online。

## Goal

1. 将**文件工作台（目录树 + 文件 tab + 编辑器区域）**调整为更接近 VS Code Explorer 与编辑器工作区的平面、紧凑、连续表面风格。
2. 让文件 tab、目录树、工作区/会话侧栏、工具栏与编辑器画布共享一致的状态语言：默认、hover、selected/active、focus-visible、drag、danger。
3. 使当前打开文件、活跃标签与活跃会话具备清晰但克制的持久视觉定位；目录树 active 必须以现有文件工作台状态为准（如 `activeDocument`），不在组件内复制或猜测。
4. 收敛工作台相关色阶、面板边框与阴影；蓝色优先用于焦点、选中或明确语义状态，常规表面保持中性。
5. **图标分区**：工作区视图继续使用 `@lucide/vue`；目录树视图全面改用 `@vscode/codicons`（见 Decisions）。

## Non-goal

1. 不修改目录读写、文件 tab、会话、终端、Git、Monaco 生命周期、请求或 API 行为。
2. 不将产品改造成完整 VS Code Online / code-server 工作台。
3. 不重做全站按钮体系、对话框或与工作台无关的页面视觉；不改变全局 `.button-secondary` 的产品语义（另有独立需求）。
4. **不在全前端清除 Lucide**：Dashboard、Shortcuts、终端、工作区侧栏等现有 `@lucide/vue` 入口不在本任务强制迁移。
5. 不将“接近 VS Code”理解为逐像素复制；保留 TermBridge 产品标识、主题系统与可访问性要求。
6. 不新增文件/会话业务能力、数据模型或运行时接口。

## Scope

| 区域 | 本任务是否改样式 | 图标 |
|------|------------------|------|
| 工作区/会话侧栏（工作区视图） | 是（平面列表、hover/active/drag） | 保留 `@lucide/vue` |
| 目录树面板及其工具栏、树行操作（目录树视图） | 是（Explorer 节奏、active 文件） | 全面 `@vscode/codicons` |
| 文件 tab strip + 编辑器 chrome / 提示条 | 是（连续 tab、表面层级） | 目录树视图外的编辑器 chrome 图标策略：tab close 等若落在文件工作台编辑器 chrome，可采用 Codicon 以与树一致；**不得**借此改写工作区侧栏 Lucide |
| 文件工作台进入/退出呈现 | 是（**路由切换**，不再在 `/sessions` 内做 drawer/pane UI 转换） | — |
| 全站其他页面图标 | 否 | 仍可用 Lucide |

## User scenarios

1. 用户打开工作区目录后，目录树呈连续 Explorer 列表；当前已打开文件在指针离开后仍可识别（持久 selected，且可与 hover 区分）。
2. 用户切换多个文件时，标签栏为连续编辑器 tab strip；活跃 tab 与编辑器画布表面连续，而非一组互不相关的卡片。
3. 用户浏览工作区与会话侧栏时，通过低对比度 hover、平面选中态与一致的键盘焦点识别层级，而不是一串圆角描边卡片；侧栏图标仍为现有 Lucide 风格。
4. 用户在目录树中使用工具栏与行内操作时，图标为 Codicon，控件规则统一：静止透明、hover 低对比底、focus-visible 仅 outline。
5. 用户从工作区打开目录/文件视图时，**导航到独立文件工作台路由**（离开会话页的同页叠层切换）；返回时回到会话路由，会话区按正常页面进入可交互，不再依赖 drawer closed/inert 过渡契约。
6. 用户在深色或浅色主题下使用时，树、tab、编辑器与分栏边界层级清楚，代码编辑区仍是主视觉画布。

## Acceptance

### 视觉与布局

- [ ] 文件标签栏不再使用独立 pill/card 式间隔、大圆角与完整四边框；改为连续、紧凑的 tab strip。活跃 tab 背景与编辑器 canvas 形成可感知的连续表面（同系背景或仅底边/顶边区分，而非整块描边卡片）。
- [ ] 目录树行默认无描边卡片感；hover / selected 以低对比度背景区分；selected（当前活跃文档）在指针离开后仍保持，且与纯 hover 可区分。树容器留白与行高偏紧凑 Explorer，而非大间距卡片列表。
- [ ] 活跃文档 selected 状态绑定现有文件工作台 active 文档状态，不引入第二套“当前文件”推断。
- [ ] 工作区与会话侧栏行、活跃态、拖拽态改为平面列表表达：避免“强边框 + 强填充 + 投影”同时出现；拖拽 ghost/chosen 可保留必要轮廓，但不得回到厚卡片堆叠。
- [ ] 文件工作台常规表面、分栏线、面板边界为中性层级；工作台区域内不必要的明显 `box-shadow`（focus ring 除外）被移除或显著弱化；页面级工作台无悬浮 drawer 阴影层。
- [ ] Monaco 画布、活跃 tab、目录树、侧栏在 dark / light 下均可辨识；编辑区为主要画布。
- [ ] 空态、加载、冲突、错误、警告保持紧凑、低干扰，不以额外大卡片破坏编辑器表面。

### 图标

- [ ] 依赖中可引入 `@vscode/codicons`（含开发/构建可用的 CSS 或 font 资源路径）。
- [ ] **目录树视图**（至少：`WorkspaceFileTree` 工具栏、展开/折叠、文件/文件夹类型、行内操作）全面使用 Codicon，不再在该视图内新增 Lucide 引用；替换后保留原 action、loading、disabled、`aria-label` / `title` 语义。
- [ ] **工作区视图**（`WorkspaceSessionSidebar` 及会话行操作）继续使用 `@lucide/vue`，本任务不要求迁移。
- [ ] 目录树内紧凑图标按钮统一：默认透明边框/底、hover 低对比背景、focus-visible 以 outline 为主；不改变既有 action 含义。

### 文件工作台进入 / 退出（路由切换，非 /sessions 内 UI 转换）

- [ ] 打开目录/文件工作台时通过 **Vue Router 导航**进入独立路由，而不是在 `/sessions`（或 cloud sessions）页面内用 drawer/pane/overlay 做同页 UI 转换。
- [ ] 会话页离开后不再长期挂着叠层文件工作台；返回会话路由即恢复会话工作台，主路径不再依赖 `inert` + drawer `closed` 过渡编排。
- [ ] 文件工作台页提供明确返回会话的入口（现有 back 语义），返回目标为对应 local/cloud sessions 路由。
- [ ] local 与 cloud 均有对应文件工作台路由；workspace 标识出现在路径或等价可恢复的路由参数中。
- [ ] 浏览器前进/后退与应用内返回行为一致、可预期；不出现“路由已变但旧叠层仍挡操作”的状态。
- [ ] 旧 drawer `transitionend`/`transform` closed 契约不再作为打开/关闭主机制（随同页叠层模式拆除或仅作遗留清理）。

### 回归

- [ ] 文件树展开/打开文件、文件 tab 切换与关闭、会话侧栏选择/拖拽、键盘焦点、工具栏 action、主题切换不出现功能回归。
- [ ] 组件/单测覆盖行为契约；视觉层级以浏览器 dark/light 人工确认（测试无法单独证明“像 VS Code”）。

## Open questions

暂无需要用户确认的未决事项。下列方向已确认：

- 工作台样式一次覆盖 Tab、目录树、侧栏平面化、色阶/边界、目录树 Codicon；文件工作台以**独立路由**呈现（替代 pane replacement / 同页转换）。
- 图标分区：工作区视图 Lucide，目录树视图 Codicon。
- 文件工作台进入/退出：**路由切换**（不再 `/sessions` 内 drawer/pane 转换）；旧 closed/inert 叠层契约不再作为主路径。

## Decisions

- 采用轻量模式 / light：限定为已有工作台组件的样式与呈现重构，不新增业务能力或 API。
- 一次覆盖：编辑器 Tab、文件树、工作区/会话侧栏平面化、工作台色阶/边界、**目录树**图标 Codicon、**文件工作台路由化**、编辑器提示紧凑化；不拆成仅 Tab 或仅树的局部需求。
- **图标策略（分区，非全站）**：
  - 工作区视图：继续 `@lucide/vue`；
  - 目录树视图：引入并全面使用 `@vscode/codicons`；
  - 允许短期内双图标系统共存；全站清除 Lucide 不在本需求范围。
- **文件工作台呈现**：独立路由进入/退出，**不再**在 `/sessions` 内做 drawer/pane UI 转换；返回会话用路由导航。旧叠层 open/close/inert/closed 过渡契约废弃为主路径。
- 继续沿用现有 CSS token、Vue 组件与 Monaco 集成；不通过大规模换 UI 库解决一致性。
- 验收取向：更接近 VS Code 的信息密度与交互层级，而非像素级复刻。

## Risk

- Tab、树、侧栏样式改动可能影响 keyboard focus、active 与窄屏溢出；实现须按交互路径回归。
- 目录树 active-file 必须读文件工作台 active 文档状态，避免与 tab 激活逻辑漂移。
- 全局颜色 token 被多页面复用；新增或收紧 token 时应优先限制在工作台选择器内，或评估非工作台页面影响。
- 路由化后需处理：local/cloud 双路由、刷新深链打开、返回会话、以及从 sessions 入口迁出时拆除叠层状态机，避免双轨（路由 + 残留 drawer）并存。
- `@vscode/codicons` 的依赖与资源加载需在开发与部署构建中可用；目录树替换时需保留 loading（如 spin）、disabled 与可访问名称。
- 双图标系统会增加心智负担，但本任务刻意限定范围以控制回归面；后续若全站统一图标应另立需求。
- “像 VS Code”依赖实机主题检查；自动化测试主要保证行为与契约，不替代视觉验收。

## User review notes

- 2026-07-16：用户要求审视向 VS Code 靠齐仍需进行的工作，并明确范围是样式。
- 2026-07-16：用户要求以轻量模式 / light 记录该任务。
- 2026-07-16：用户确认一次覆盖全部已列出的 VS Code 工作台样式范围。
- 2026-07-16：用户曾要求全面引入 `@vscode/codicons` 并替换 Lucide；后修订为**分区策略**：工作区视图保留 `@lucide/vue`，目录树视图全面使用 `@vscode/codicons`。
- 2026-07-16：用户曾确认 drawer → pane replacement 与 closed 契约；**后澄清修订**：目录/文件视图改为**切换路由**，不再在 /sessions 内做 UI 转换；叠层 closed/inert 主路径废弃。
- 2026-07-16：根据评审反馈改进表述准确性：明确范围表、验收可检查项、Non-goal 与全站图标脱钩、closed 契约与防过期要求。
- 2026-07-16：用户澄清——目录视图通过**路由切换**进入，不再在 /sessions 内进行 drawer/pane UI 转换。
- 2026-07-16：用户采纳 Spec / 规格（路由化文件工作台 + 样式对齐方案）。
