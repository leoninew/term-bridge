# 面向 Cloud CLI 与 Code XCLI 用户的帮助中心

最后修改时间: 2026-07-28 08:32:30

Review status: Accepted

## Flow mode / Stage

标准模式 / standard；需求 / Requirement（Accepted）；用户要求进入计划 / Plan。

## Background

TermBridge 已有云端帮助路由和占位页面，但尚未提供面向最终用户的完整使用文档。现有仓库中的 README、设计、需求、计划和验证记录主要服务于开发与内部交付，不能直接构成最终用户帮助内容。

目标读者已经能够在自己的 Device 上使用 Cloud CLI 或 Code XCLI，希望通过 TermBridge 围绕项目目录创建、进入和复用 CLI 会话。用户的主流程不是学习终端、项目开发环境或部署架构，而是：下载正式制品、启动 TermBridge、打开项目、运行 CLI 会话、从浏览器继续工作，并将高频命令沉淀为快捷方式。

项目通过 GitHub Actions 构建并发布制品，GitHub Release 是用户下载 TermBridge 的正式来源。帮助中心需要将“去哪里下载、下载哪个制品、下载后如何开始”纳入快速开始闭环。

## Goal

1. 在 Web 前端交付一个公开可访问、可发现、章节化的大型 `/doc` 文档中心，采用常见开源文档导航形态，优先服务 Cloud CLI 与 Code XCLI 用户；它不是仓库 Markdown 导航，也不依附于现有 `/help` 占位页。
2. 以 Happy Path 为主线提供有内容深度的使用文档，而不是以安全、性能、内部架构或故障排查作为主导航重点。
3. 将快速开始组织为完整闭环：从 GitHub Release 下载制品，到启动 TermBridge、打开 Workspace、创建并在浏览器中使用第一个 CLI Session。
4. 在首版交付下面七个核心章节，并让章节之间有明确的下一步跳转：
   1. 快速开始：下载、启动并运行第一个 CLI 会话；
   2. 认识 TermBridge；
   3. 启动 Cloud CLI / Code XCLI 会话；
   4. 在浏览器中继续已有会话；
   5. 用 Workspace 管理项目；
   6. 快捷方式：定义与管理常用命令；
   7. 使用快捷方式快速开启会话。
5. 把快捷方式作为独立的高频工作流：用户可维护一组各自对应**单条命令**的可复用命令定义，并在新建 Session 时选择其中一项；快捷方式不绑定 Device、Workspace 或工作目录，也不是正在运行的终端、命令组或已有 Session。
6. 为关键使用步骤预留统一的截图占位，使真实截图可以在后续视觉与功能稳定后直接替换，不改变正文结构。
7. 同时交付仓库中可审阅的中英文 Markdown 文档源与前端 `/doc` 阅读页；长篇文档内容不写入 Vue I18n，中文与英文的章节、操作步骤、链接和截图图注通过独立 Markdown 内容保持一致，并在 `/doc` 中可切换阅读。

## Non-goal

1. 本期不建设独立的外部文档站、VitePress、MDX/CMS、版本化文档站或 SEO 策略；但产品内 `/doc` 必须读取或构建自仓库维护的 Markdown 文档源。
2. 不把项目源码开发、Go/Node/Yarn/Task 开发环境安装流程作为最终用户快速开始主线。
3. 不将安全、性能、部署架构、协议细节、复杂故障排查或已知限制设为帮助中心主导航章节；确有必要时仅在相关步骤中以简短提示说明。
4. 不为 Cloud CLI、Code XCLI 或其他第三方 CLI 重写其完整命令参数、认证或产品文档；TermBridge 只说明如何在其 Session 中启动用户已有的命令。
5. 不承诺所有交互式 CLI、全屏 TUI、复杂终端控制序列或长时会话场景均具备完全相同的表现。
6. 本期不制作真实截图、录屏或插画；只交付可替换的截图占位和图注规范。
7. 不在本需求中修改 GitHub Actions、Release 制品构建、后端 CLI contract、Device/Session/Shortcut 数据模型或权限模型。

## User scenarios

### 1. 从 Release 开始首次使用

作为已有 Cloud CLI 或 Code XCLI 的用户，我可以在帮助中心“快速开始”中找到 GitHub Releases 下载入口，知道应选择与当前系统匹配的 Release Asset，解压并启动 TermBridge，然后完成第一个 CLI Session。

### 2. 启动已有的 CLI

作为已在 Device 上完成 CLI 安装和登录的用户，我可以在对应 Workspace 中以自己的原始命令启动 Cloud CLI 或 Code XCLI，无需重新安装、迁移或重新认证该 CLI。

### 3. 从浏览器继续工作

作为正在处理项目任务的用户，我可以从 Session 列表打开已有 Session，在浏览器中继续查看输出和进行交互，而不必重新理解该会话属于哪个项目或设备。

### 4. 围绕项目组织工作

作为需要同时处理多个项目或任务的用户，我可以理解 Device、Workspace、Session 和 Shortcut 的关系，并围绕正确的 Workspace 创建 Cloud CLI、Code XCLI 或 Shell Session。

### 5. 管理快捷方式

作为高频用户，我可以为常用任务和 CLI 参数创建、查看、编辑、启用、停用和整理快捷方式，明确每一项保存的是单条可复用命令定义。

### 6. 通过快捷方式开启新会话

作为高频用户，我可以选择一个快捷方式快速创建新的 Session，并知道若要继续已开始的工作，应打开已有 Session 而不是再次启动快捷方式。

## Content and information architecture

### Help home

帮助首页使用文档式布局，至少提供：

- 清晰的产品价值说明：让 Cloud CLI 与 Code XCLI 会话可在浏览器中继续工作；
- 三个优先行动入口：`快速开始`、`创建 CLI 会话`、`从快捷方式启动`；
- “最常用”链接：创建第一个 CLI Session、继续已有会话、管理 Workspace、管理快捷方式；
- 按章节浏览的左侧导航；
- 当前章节内标题目录，长页面可快速跳转。

### Chapter 1: 快速开始：下载、启动并运行第一个 CLI 会话

该章以 5–10 分钟的 Happy Path 为目标，按以下顺序组织：

1. 开始前准备：设备、可运行且已登录的 Cloud CLI 或 Code XCLI、项目目录和浏览器；
2. 从 GitHub Release 下载：提供正式 Releases 链接、说明选择最新稳定版本与匹配操作系统的 Asset；Release Asset 文件名和平台清单必须以实际发布产物为准；
3. 解压与启动 TermBridge：按正式制品提供的实际启动方式编写，不使用项目开发命令；
4. 打开或创建 Workspace：选择包含目标项目的工作目录；
5. 在 Workspace 中创建第一个 Cloud CLI 或 Code XCLI Session；
6. 在浏览器终端确认 CLI 已启动，并说明下一步可继续已有会话或创建快捷方式。

命令示例应以当前已验证的 CLI contract 为准，可使用 `termbridge --cwd <项目目录> exec -- <命令>` 解释工作目录与原始命令关系；不得将内部开发命令或高风险第三方 CLI 参数作为默认示例。

### Chapter 2: 认识 TermBridge

以一个从项目到会话的真实故事解释四个用户概念：

- Device：运行 CLI 与项目文件的电脑或服务器；
- Workspace：项目所在工作目录；
- Session：一次正在运行的 CLI 或 Shell；
- Shortcut：可复用的一组命令定义。

使用“Device → Workspace → Session”和“Shortcut → 新 Session”的简图说明关系，避免展开 Runtime、PTY、Agent、Cloud 协议等内部概念。

### Chapter 3: 启动 Cloud CLI / Code XCLI 会话

用两个平行但结构一致的操作路径，说明如何在当前 Workspace 中启动 Cloud CLI 和 Code XCLI。每条路径包含：选择 Workspace、创建 Session、确认工作目录、填写或选择原始命令、启动并确认终端。另设“使用自己的启动参数”小节，说明用户可保留既有命令与参数，但不收录第三方 CLI 的完整参数参考。

### Chapter 4: 在浏览器中继续已有会话

说明如何在正确的 Device 与 Workspace 下找到 Session、打开正在运行的会话、同时处理多个 Session，以及离开浏览器后再次回到已有工作。需要以用户可理解的方式突出“快捷方式用于创建新工作；Session 用于继续已有工作”。

### Chapter 5: 用 Workspace 管理项目

说明 Workspace 对应项目目录、Session 从该目录运行，以及如何选择或添加项目 Workspace。提供一个推荐工作方式示例：同一 Workspace 下可分别运行主要 CLI Session、并行 CLI Session 和 Shell Session，文件、Code、Git 工作台作为按需进入的辅助能力。

### Chapter 6: 快捷方式：定义与管理常用命令

该章独立说明 Shortcut 的管理价值和配置方式：

- 每个快捷方式是一条可重复使用的命令定义，而不是已运行的 Session；
- 解释实际可用配置项：名称、命令、描述、标签和启用状态；明确它不保存 Device、Workspace 或工作目录；
- 用“项目名 · 任务名”展示推荐命名；
- 给出按项目或任务整理多个快捷方式的示例；
- 说明编辑快捷方式只影响以后新建的 Session，不改变已经运行的 Session；
- 说明在目标 Device 的 Session 工作台中，先选择 Workspace/工作目录，再在命令来源中选择已启用 Shortcut；快捷方式管理页不提供直接“运行/启动”按钮；
- 仅描述当前 UI 已实现且经核对的实际能力；用一组同前缀的单命令 Shortcut 表达多命令工作流，不虚构命令组执行能力。

### Chapter 7: 使用快捷方式快速开启会话

该章聚焦在目标 Device 的 Session 工作台中使用快捷方式：选择或输入 Workspace/工作目录、打开新建 Session、选择已启用 Shortcut 作为命令来源、创建新 Session、进入浏览器终端。使用关系图明确一个 Shortcut 可以反复创建多个彼此独立的 Session；以“功能开发、测试、审查”的日常场景解释什么时候使用 Shortcut，什么时候回到已有 Session。

## Screenshot placeholders

帮助页首版应为下列 P0 位置提供统一的截图占位组件或结构化占位块：

1. 本机终端中已可运行 Cloud CLI 或 Code XCLI；
2. GitHub Releases 页面中最新稳定版本和 Assets 下载区域；
3. 解压后的 TermBridge 制品目录；
4. TermBridge 启动成功及本地访问地址；
5. Workspace 列表或创建 Workspace 入口；
6. 新建 CLI Session 的界面；
7. 浏览器 Web Terminal 中运行的 CLI Session；
8. Shortcut 列表与创建/编辑入口；
9. 从 Shortcut 创建新 Session 的操作位置。

每个占位必须包括截图标题、计划展示内容和图注；真实截图替换后仍保留具有独立含义的图注。占位文案应避免把示例视作已交付截图。真实截图不得展示 Token、账户信息、私有项目内容、敏感终端输出或用户个人路径。

## Acceptance

- [ ] 帮助中心具备可发现的入口和文档式导航，可从首屏进入快速开始、CLI 会话与快捷方式工作流。
- [ ] 帮助中心首版具备 Goal 中列出的七个完整章节，并按约定主线互相链接。
- [ ] “快速开始”明确指向 GitHub Releases 作为下载来源，说明如何选择实际存在的平台制品，并覆盖下载、启动、Workspace、首次 CLI Session 和浏览器确认闭环。
- [ ] 帮助正文面向已能使用 Cloud CLI 或 Code XCLI 的用户，优先说明实际使用工作流，不以项目开发环境搭建作为主路径。
- [ ] 内容清晰区分 Device、Workspace、Session 与 Shortcut；明确 Shortcut 创建新 Session、已有 Session 用于继续已有工作。
- [ ] 快捷方式管理章节覆盖实际 UI 支持的创建、查看、编辑、启用/停用和组织行为；准确表明单条命令、非 Device/Workspace/cwd 绑定、无管理页直接启动按钮，不引入未实现的命令组语义。
- [ ] 七个章节包含有意义的正文、操作步骤、示例、预期结果或下一步，而不是目录级占位文本。
- [ ] P0 截图位置都采用统一占位与图注，后续可替换真实截图而不重写文章结构。
- [ ] 仓库中存在可审阅的中文与英文 Markdown 文档源；`/doc` 读取或构建自该内容而非 Vue I18n，且可切换中文/英文。
- [ ] 中文和英文帮助内容的章节结构、链接目的、操作步骤及截图图注一致。
- [ ] 现有本地与云端主要业务路径、Session、Workspace 和 Shortcut 行为不因帮助中心改动而回退。

## Open questions

1. **`/doc` 页面可访问范围。** 已确认：新增 `meta.mode: 'hybrid'` 的公开 `/doc` 路由，使其绕过 Cloud 登录校验与 Local/Cloud 模式重定向；现有 `/help` 占位路由不作为文档中心承载页。
2. **Releases 正式 URL 与 Release Asset 命名。** 已核实：文档使用 `https://github.com/leoninew/TermBridge-go/releases`，当前发布 `TermBridge-windows-x64.zip`、`TermBridge-linux-x64.zip`、`TermBridge-macos-x64.zip`；文案仍以 Release 中实际列出的 Assets 为准，不把未来平台写成既有承诺。
3. **截图资产位置与替换流程。** 默认采用前端静态资源目录，截图使用固定 slug 命名并在 Markdown 中引用；占位块和图注不依赖图片存在。真实图片替换时只新增/替换同名文件，不改变文章结构。
4. **文档内容呈现模型。** 已确认：仓库维护中英文 Markdown 文档源，前端 `/doc` 以受控 Markdown 渲染方式读取，语言切换不使用 Vue I18n。具体解析依赖、内容目录、锚点生成和桌面/移动导航行为在 Plan 阶段确定。

## Decisions

1. 使用标准模式 / standard：本任务跨前端信息架构、路由可达性、国际化内容、截图占位和 Release 分发说明，需要 Requirement 与 Plan 两个阶段后再实现。
2. 首版以 Happy Path 为信息架构主线；安全、性能、架构和排障不占主导航，相关提示仅在操作语境中简短呈现。
3. GitHub Releases 是最终用户下载制品的正式来源；帮助页必须明确提供下载引导。
4. 快捷方式是帮助中心的独立核心章节，而非“会话管理”的附录。
5. 首版页面内使用截图占位锁定视觉叙事，真实截图作为后续可替换资源补齐。
6. 将文档中心实现为产品内的公开 `/doc` 页面，沿用既有 Vue 与 Vue Router；`/doc` 使用 `hybrid` 路由模式，不要求 Cloud 登录；现有 `/help` 路由不承担此功能。
7. 文档正文以仓库中的中英文 Markdown 作为单一内容来源，使用受控的前端 Markdown 渲染/构建管线呈现；不将长文写入 Vue I18n。

## Risk

1. Release 制品的实际平台覆盖、文件名、启动入口与帮助文案不一致会直接阻断快速开始，必须在实施和验证时以 GitHub Actions 与实际 Release/构建产物核对。
2. 历史 `docs/` 可能包含已过时的命令和流程；用户帮助应以当前 CLI、当前 UI 和 Release 行为为准。
3. 若 `/doc` 被错误标为 Cloud 路由、加入登录白名单之外，或依赖 Local/Cloud 当前模式，安装前用户将无法访问快速开始；必须保持 `hybrid` 且不引入认证依赖。
4. 快捷方式的实际数据模型和 UI 支持范围可能与“命令组”的用户表达不同；文案必须遵从真实行为，避免把多个 Shortcut 或命令来源选择器误写为单个快捷方式的多命令编排。
5. 中英文 Markdown 若不采用固定镜像目录、稳定 slug、章节校验和统一链接策略，容易造成语言切换后的章节或锚点漂移；实现需将内容源、语言映射与导航元数据作为明确结构维护。
6. 截图最终替换若没有统一尺寸、标注和脱敏约定，容易造成移动端布局不稳定或泄漏示例环境信息。

## User review notes

- 用户提出需要为本项目提供类似开源项目 Doc 导航的帮助文档页，重点是使用与快速开始。
- 用户明确目标读者主要是 Cloud CLI 或 Code XCLI 用户。
- 用户要求内容聚焦 Happy Path，而不是将安全、性能等内容作为重点。
- 用户明确快捷方式管理是核心能力：快捷方式用于定义一系列命令并快速开启新会话。
- 用户补充：项目通过 GitHub Actions 分发制品并在 GitHub Release 制作制品；快速开始必须说明下载来源。
- 用户确认截图可以先以占位形式进入首版。
- 用户于 2026-07-28 要求使用 `/specflow` 开始此“前端 + 文档”任务。
- 用户于 2026-07-28 接受帮助中心应匿名可访问的判断，并要求继续核实其余实现事实；正式 GitHub 仓库为 `https://github.com/leoninew/TermBridge-go`，可使用 `/pomelo-pw` 组织截图例证。
- 用户于 2026-07-28 更正交付目标：需要前端内大型公开 `/doc` 章节页，不以 `/help` 或仅仓库 Markdown 导航替代。
- 用户于 2026-07-28 进一步明确：仍需维护 Markdown 文档源，`/doc` 支持中文/英文切换；长篇文档正文不使用 Vue I18n。
- 计划阶段核实后修正：每个 Shortcut 仅保存单条命令及名称、描述、标签、启用状态；不绑定 Device、Workspace 或 cwd。用户从目标 Device 的 Session 工作台中选定 Workspace/工作目录后，在新建 Session 的命令来源选择器里使用已启用 Shortcut。
