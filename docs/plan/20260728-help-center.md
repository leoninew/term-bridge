# 前端 `/doc` 帮助中心与 Markdown 内容源实施计划

最后修改时间: 2026-08-19 11:07:07

Review status: Accepted

## Flow mode / Stage

标准模式 / standard；计划 / Plan（Accepted）；用户要求开始实现 / Implementation。

Requirement: `docs/requirement/20260728-help-center.md`

## Implementation strategy

交付由同一仓库维护的两层组成：

1. `docs/user-guide/zh-CN/` 与 `docs/user-guide/en-US/`：可在 Git 中审阅、彼此镜像的 Markdown 正文；
2. 公开、匿名可访问的 Vue `/doc` 单页阅读体验：将这些 Markdown 在 Vite 构建时以 raw module 打入前端包，通过受控 renderer 呈现为具有侧栏、页面目录、Hash 锚点、语言切换和截图占位的文档中心。

文档正文不加入 `web/src/i18n.ts`。现有 Vue I18n 继续仅服务产品 UI；文档阅读页的章节、标题、段落、步骤、图注均来自 Markdown 文件。文档页面不会请求用户、Device、Workspace、Session 或 Shortcut API，因此可安全地在未登录和三种运行模式下访问。

## Content model

### Content directory and mirrored locale structure

新增以下目录，并保证每个 locale 都有完全相同的文件集合和排序：

```text
docs/user-guide/
├── zh-CN/
│   ├── index.md
│   ├── quick-start.md
│   ├── understand-termbridge.md
│   ├── start-cli-sessions.md
│   ├── continue-sessions.md
│   ├── manage-workspaces.md
│   ├── manage-shortcuts.md
│   └── launch-from-shortcuts.md
└── en-US/
    ├── index.md
    ├── quick-start.md
    ├── understand-termbridge.md
    ├── start-cli-sessions.md
    ├── continue-sessions.md
    ├── manage-workspaces.md
    ├── manage-shortcuts.md
    └── launch-from-shortcuts.md
```

`index.md` 是 `/doc` 的阅读入口与文档首页；其余七个文件对应需求中约定的七个核心章节。每篇文件通过 frontmatter 提供稳定 `id`、`order`、`title`、`description` 与 `next`，其中 ID 与文件 slug 均为语言无关值。语言切换、侧栏导航、首页行动入口、文章“下一步”和截图引用都只引用这些稳定 ID，禁止从翻译标题拼接 URL 或锚点。

所有需要被链接、出现在右侧 TOC 的标题使用 Markdown 显式属性定义稳定 ID，例如 `## 下载制品 {#download}`。文档结构在两种语言中保持相同的标题 ID、步骤数量、`next` 关系和 screenshot ID；文字内容独立翻译。

### Markdown renderer and safety boundary

在 `web/` 增加小型 Markdown 渲染依赖：`markdown-it`、`markdown-it-anchor`、`markdown-it-attrs` 与 `dompurify`（及对应 TypeScript 类型，如依赖本身未携带）。

新增 `web/src/features/docs/`：

- `catalog.ts`：文档语言、稳定章节 ID、Release URL、实际 Release Asset 名称、已知 screenshot ID 与导航顺序的唯一元数据；
- `content.ts`：用 `import.meta.glob('../../../docs/user-guide/*/*.md', { query: '?raw', import: 'default', eager: true })` 加载 Markdown；对 frontmatter、语言、ID、顺序和章节镜像关系作运行时断言；
- `markdown.ts`：初始化仅允许受控 Markdown 的 renderer，禁用 Markdown 内原始 HTML，渲染链接与代码块；对最终 HTML 使用 DOMPurify 净化；
- `anchors.ts`：根据 hash 在内容渲染后定位章节，补偿固定 Header 的 `scroll-margin-top`，处理首次进入、同页 hash 改变、刷新和前进/后退；
- 对应 Vitest 单元测试，校验中英文文件集、frontmatter、导航 ID、内部链接目标、截图 ID 与标题锚点的一致性。

修改 `web/vite.config.ts`，只为开发服务器明确允许读取仓库根目录下的 `docs/user-guide/`；生产构建仍将 raw Markdown 打入 bundle，不在浏览器运行时读取文件系统或请求外部 Markdown。不得使用 `v-html` 直接渲染未净化文本，也不得允许 Markdown 内嵌任意 HTML。

### Screenshot placeholder contract

新增 `web/src/components/docs/DocScreenshotPlaceholder.vue` 与 `web/src/features/docs/screenshots.ts`。组件通过稳定 screenshot ID 查询元数据，始终渲染：

- 明确可见的“截图待补充 / Screenshot planned”状态；
- 本地化的截图标题；
- “计划展示什么”说明；
- 即使没有图片也独立可读的图注；
- 未来图片的 `alt`、固定宽高比和资源路径 slot。

首版只登记并呈现九个 P0 ID：`cli-ready`、`release-assets`、`package-folder`、`agent-started`、`workspace-entry`、`create-session`、`browser-terminal`、`shortcut-management`、`shortcut-command-source`。不生成伪造 UI、伪造 Release Assets、Token 或终端输出。

未来真实截图统一放入 `web/public/doc-assets/<screenshot-id>.<ext>`；组件仅在资源清单显式声明资源已入库时显示 `<img>`，否则继续显示占位，避免损坏图片。图片使用压缩的 WebP/PNG、预设响应式容器和脱敏检查；中英文若图片包含文字，分别在同一 screenshot ID 的语言资源下声明。

## User-facing documentation content

### 1. `quick-start`

按真实 portable Release 流程写出 5–10 分钟主线：

1. 准备一台已安装、已登录、可在终端运行 Cloud CLI 或 Code XCLI 的 Device；
2. 前往 `https://github.com/leoninew/TermBridge-go/releases/latest`，选择最新非 prerelease Release；
3. 仅列出当前实际发布的 x64 资产：`TermBridge-windows-x64.zip`、`TermBridge-linux-x64.zip`、`TermBridge-macos-x64.zip`；不声称 arm64、安装器、自动升级或其他平台支持；
4. 解压后按平台启动：Windows 运行 `termbridge.cmd`，Linux/macOS 运行 `./termbridge.sh`；脚本会启动本地 Agent 并打开 `http://localhost:9030`；
5. 打开本地工作台，在 Session 工作台中创建第一个项目会话；
6. 选择直接命令并用真实 CLI contract 示例解释 `termbridge --cwd <项目目录> exec -- <命令>` 的语义，明确这用于运行 CLI 命令，不替代启动 TermBridge 产品的 launcher；
7. 在浏览器终端确认 CLI 启动，再引导用户继续已有会话或创建快捷方式。

Release 链接、平台 Asset 名称和 launcher 说明来自 `.github/workflows/release.yml`、`Taskfile.yml` 与 `scripts/package/termbridge.{cmd,sh}`；不得复用 README 中面向贡献者的 Task 开发命令作为最终用户流程。

### 2–5. 核心概念、CLI 会话、已有会话与 Workspace

- `understand-termbridge` 只用 Device → Workspace → Session 和 Shortcut → 新 Session 两个关系图建立心智模型；不展开 Runtime、PTY 或云端协议。
- `start-cli-sessions` 为 Cloud CLI 与 Code XCLI 提供不同命令示例，但 UI 流程均严格对应真实界面：选择/输入工作目录 → 新建 Session → 选择 Direct command 或已启用 Shortcut → 启动。不可称为专用 CLI 集成、自动安装或自动登录。
- `continue-sessions` 说明 Local 入口和 Cloud 的真实路径 Dashboard → Device → Sessions；已有 Session 从 Workspace/Session 树进入，不能用同一 Shortcut “恢复”。
- `manage-workspaces` 说明 Workspace 是 Session 的项目目录上下文；不得声称已提供独立 Workspace 创建表单。当前 UI 会在创建 Session 时初始化 Workspace，因此文案应相应描述。

### 6–7. Shortcut 管理与从 Shortcut 创建会话

- `manage-shortcuts` 只描述当前 `Shortcut` 数据模型：名称、单条命令、可选描述、标签、启用状态。它不保存 Device、Workspace 或 cwd，不是键盘快捷键、命令组、脚本编排器或运行中终端。
- 管理页真实能力包括创建、编辑、复制命令、启用/停用、标签/关键字筛选、排序、导入导出和删除；没有“在 Shortcut 卡片上直接运行”的操作。
- `launch-from-shortcuts` 写明正确 Happy Path：先进入目标 Device 的 Sessions 工作台，选择/输入此 Session 的 Workspace/cwd，在新建 Session 的命令来源中选择已启用 Shortcut，然后创建新 Session。已禁用 Shortcut 留在管理页但不能被会话选择器使用。
- 解释同一 Shortcut 可反复创建独立 Session；新会话会保存命令和 Shortcut 名称/ID 快照，所以后来编辑 Shortcut 不会改写既有会话。

## Frontend implementation

### Route, shell and discoverability

1. 在 `web/src/router/index.ts` 新增 `{ path: '/doc', name: 'docs', component: DocView, meta: { mode: 'hybrid' } }`。不加入 Cloud login flow，也不调用 `runtimeConfig.switchMode`，以确保 local / cloud / hybrid 配置、未登录、带 hash 的直达 URL 都不会重定向。
2. 保留现有 `/help` 路由和占位页面，不将其重命名或迁移为 `/doc`；本任务的公共文档入口是 `/doc`。
3. 在 `AppHeader.vue` 新增可访问的 Docs 链接（桌面与移动同一入口），固定跳转 `/doc`；在 `LocalHome.vue` 和 `CloudHome.vue` 的适当 CTA/页脚添加“文档 / Docs”入口，保证首次用户不必先登录或进入工作台即可找到文档。
4. CTA 按匿名边界设计：文档内“快速开始”均为 hash；“创建 CLI 会话”“继续已有会话”“管理快捷方式”等产品动作，在匿名状态只返回首页或明确的登录/设备入口，绝不生成缺少 `deviceId` 的 Cloud 路由。首版优先采取保守的公开入口链接，不读取鉴权状态以做深链接。

### Document view and responsive layout

新增 `web/src/views/DocView.vue` 与 `web/src/components/docs/`：

- `DocsShell.vue`：沿用 `AppPageShell` 和设计 token，不复用现有 auth card；实现大屏三栏（左侧章节导航、正文、右侧当前页 TOC）与窄屏可折叠目录；
- `DocsSidebar.vue`：使用 catalog 的七章顺序，支持高亮当前 hash；
- `DocsLanguageSwitch.vue`：使用 query 参数 `?lang=zh-CN|en-US`（默认使用现有浏览器/产品 locale 仅作为初始推断，切换状态由 URL 决定），保留当前 hash，语言缺少对应锚点时回退文档首页；不将正文写入 Vue I18n；
- `DocsArticle.vue`：按 catalog 顺序渲染首页与所有七章 Markdown，使用 sanitizer 后 HTML、插入截图占位、提供文章末尾“下一步”导航；
- `DocsToc.vue`：从实际渲染标题生成二级目录，只展示 catalog 规定的标题层级，并通过共享 hash 常量跳转；
- `DocsCallout.vue`、`DocScreenshotPlaceholder.vue`：承载 Markdown 约定的提示块和 P0 截图位置。

全页使用稳定 hash（如 `#quick-start`、`#download`、`#manage-shortcuts`），而非本地化标题生成 hash。为标题、正文锚点和 header 设置 scroll margin，确保固定 `AppHeader` 不遮挡定位标题。语言切换始终保留 hash；路由 hash、浏览器导航和页面首次挂载后均走同一个滚动函数。

### Styling and accessibility

在 `web/src/components/docs/docs.css` 或同等 scoped/style token 模块中定义文档阅读样式，保持现有浅色/深色主题变量：最大正文宽度、代码块横向滚动、表格移动端滚动、链接焦点态、至少 44px 的移动端目录/语言切换触达区、图片和占位的最大宽度与不溢出约束。所有图示、代码块和外部链接有可读文本，导航设置 `aria-current`，正文容器设置文档语义与正确的 heading 层级。

## Files to change

- Routing/navigation: `web/src/router/index.ts`, `web/src/components/layout/AppHeader.vue`, `web/src/components/dashboard/LocalHome.vue`, `web/src/components/dashboard/CloudHome.vue`。
- Document UI: `web/src/views/DocView.vue`, `web/src/components/docs/{DocsShell,DocsSidebar,DocsLanguageSwitch,DocsArticle,DocsToc,DocsCallout,DocScreenshotPlaceholder}.vue` 及对应样式文件。
- Document runtime: `web/src/features/docs/{catalog,content,markdown,anchors,screenshots}.ts` 及单元测试。
- Content source: `docs/user-guide/{zh-CN,en-US}/` 的 `index.md` 和七篇章节 Markdown。
- Tooling: `web/package.json`, `web/yarn.lock`, `web/vite.config.ts`（引入 renderer、开发时允许受控的仓库文档目录）。
- Documentation process: `docs/requirement/20260728-help-center.md`, `docs/plan/20260728-help-center.md`，以及后续 verification 文档。

## Verification plan

1. **Content unit tests**：加载两种语言 Markdown，验证每种语言均包含首页与七章；frontmatter ID/order/next、稳定 heading ID、内部章节链接、P0 screenshot ID 和 Release URL/Asset 常量全部匹配 catalog；任意镜像缺失或不一致时失败。
2. **Renderer unit tests**：验证 Markdown 代码块、外部链接、标题锚点和 screenshot directive 的输出；原始 script/事件属性/危险 URL 不能进入最终 HTML；未知 screenshot ID 明确失败或渲染开发期错误。
3. **Route/anchor tests**：验证 `/doc` 在 local / cloud / hybrid 运行配置及未认证状态下不重定向；`/doc?lang=en-US#quick-start` 正确选择语言并保持/定位 hash；切换语言保留可映射锚点；无映射时回退首页。
4. **Component tests**：覆盖侧栏导航、TOC、语言切换、截图占位九项登记、外部 Release 链接属性与小屏不溢出的关键 class/DOM 语义。
5. **Project checks**：运行 `yarn --cwd web typecheck`、`yarn --cwd web test`、`yarn --cwd web lint`、`yarn --cwd web format` 与 `yarn --cwd web build`。
6. **Browser verification**：通过 Pomelo PW 在未登录 Local/Cloud/Hybrid 入口直接访问 `/doc` 和带 hash 的 `/doc?lang=en-US#quick-start`，确认不跳首页或登录；截图检查桌面与窄屏侧栏、语言切换、Release 下载区、所有占位块、长代码块和 TOC。真实截图到位后另行执行脱敏、断图和响应式检查。
7. **Release fact check**：在验证阶段重新打开 `https://github.com/leoninew/TermBridge-go/releases/latest`，确认 Release Assets 仍包含文档列出的三个 x64 ZIP；如实际线上 Release 改变，更新 catalog 与 Markdown 后再发布。

## Risks and constraints

- Markdown 位于 `web/` 之外会依赖 Vite raw import 与开发文件系统 allowlist；实现必须限制到仓库 `docs/user-guide/`，不扩大任意磁盘访问。
- 新增 renderer 依赖和 `v-html` 是新的 HTML 边界；必须保持 raw HTML 禁用并进行 DOMPurify 净化，且只构建时导入仓库受控内容。
- 单页长文档若未实现 hash 定位，用户会在语言切换、刷新或分享链接时丢失位置；该行为属于功能验收而非样式增强。
- 当前本地 UI 截图可能包含用户项目名、路径和命令，因此仅作为实施参考，不能直接并入正式资产；首版用语义占位。
- Release 目前仅有 x64 ZIP；帮助文案需随发布矩阵改变而更新，不能长期硬编码为永不变化的支持承诺。

## Rollback

文档页只新增公开静态前端路由、内容和导航入口，不变更 API、认证、运行时或数据模型。若出现阻断性渲染问题，可移除 `/doc` 导航入口与路由，恢复现有产品路径；Markdown 与静态资源可保留供后续修复，不影响 Agent/Cloud/Session 功能。

## User review notes

- 用户已接受帮助中心匿名可访问的判断。
- 用户更正：交付必须是前端内的大型 `/doc` 页面及章节，而不是 `/help` 或仅仓库 Markdown 导航。
- 用户进一步确认：仓库仍需维护 Markdown 文档源，`/doc` 需提供中文/英文切换，长文正文不使用 Vue I18n。
- 已实际核实最新 GitHub Release `v0.106.2` 与三个 x64 ZIP Assets，并使用 Pomelo PW 抓取 Release 页面和当前本地首页作为实施参考；这些参考截图不作为正式用户文档素材。
- 用户于 2026-07-28 要求开始实现，并重申八篇文档必须以唯一 `/doc` 路由呈现；此前部分本地 UI 截图因页面加载未完成而为空，首版不使用这些截图，并在后续真实截图流程中等待关键内容稳定后再采集。
