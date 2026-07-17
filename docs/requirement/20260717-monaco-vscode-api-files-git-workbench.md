# 基于 monaco-vscode-api 的文件与 Git 工作台

最后修改时间: 2026-07-17 19:30:00

Review status: Draft

## Flow mode / Stage

标准模式 / standard；需求 / Requirement（Draft，待用户审阅后 Accepted 再进入 Spec/Plan 执行）。

评估依据：[`docs/analyze/20260717-monaco-vscode-api-files-git-evaluation.md`](../analyze/20260717-monaco-vscode-api-files-git-evaluation.md)

## Background

TermBridge 已有 workspace-scoped 文件工作台与 Git 审阅/操作能力骨架：

- 路由：local `/workspaces/:workspaceId/files`、`/git`；cloud `/devices/:deviceId/workspaces/:workspaceId/files`、`/git`
- 前端：Vue 自研目录树、标签栏、Monaco standalone 编辑器、Git changes panel、`fileWorkbench` store
- 后端：Agent workspace file API、`gitexec`、Cloud device 转发

继续自研 Explorer + 编辑器 chrome + SCM 面板的打磨成本超出预期，完成度难以收敛到“可当作工作台交付”的体验。评估结论是：在后端可按 VS Code 契约提供 API、且明确不做扩展市场的前提下，采用 `@codingame/monaco-vscode-api` 的 **纯浏览器 Workbench + 自定义 FileSystemProvider + 自定义 SCM Provider** 重做文件与 Git 前端。

本需求把该结论收成可验收的产品与工程范围。它替换的是**文件与 Git 前端呈现与交互底座**，不把 TermBridge 变成完整 VS Code Online / code-server 产品，也不用库自带 Terminal 替换现有 Session/xterm 模型。

## Goal

1. 用户在 local 或 Cloud 的 workspace 上下文中，打开基于 monaco-vscode-api Workbench 的**文件与 Git 工作台**，获得接近 VS Code 的 Explorer、编辑器 tab/dirty/save 与 SCM 视图体验。
2. Workbench 通过自定义 **FileSystemProvider** 读写当前 workspace 根下的文件与目录；通过自定义 **SCM Provider** 展示与操作 Git 变更（status、diff、stage/unstage、discard、commit，以及约定范围内的分支能力）。
3. Agent/Cloud 的文件与 Git **HTTP/proto/路径/数据结构以 monaco-vscode-api 所需的 `FileSystemProvider` 与 SCM Provider（Git）接口为准**直接实现或改造；**不做**旧 TermBridge DTO ↔ VS Code 接口的适配层/桥接层。local 与 cloud 仍经既有 device ownership / runtime 转发模型工作。
4. `/sessions` 仍是会话入口；Workbench 使用**独立全屏路由**：
   - local：`/workspaces/:workspaceId/code`
   - cloud：`/devices/:deviceId/workspaces/:workspaceId/code`
   不与 session tab 状态机耦合；不复用 `/files` 或 `/git` 路径。
5. 旧 Vue 文件树 / Git 面板在 Workbench MVP 可用后退出主路径，避免双实现长期并行。

## Non-goal

1. 不提供扩展市场、任意 VSIX 安装、官方 Git 扩展依赖、LSP/语言服务扩展包作为交付范围。
2. 不把 VS Code Server / code-server / remote agent 作为本需求主路径（可在 Spike 中对照，不作为验收依赖）。
3. 不使用 monaco-vscode-api 自带 Terminal 替换或映射 TermBridge Session/history/rerun/writer lock。
4. 不实现 debug、测试资源管理器、notebook、远程 fetch/pull/push/clone、merge/rebase/cherry-pick/stash 等完整 IDE Git 能力（除非后续单独立项）。
5. 不要求工作区全文搜索（Ctrl+Shift+F）在一期可用。
6. 不改变 device ownership 授权模型，不新增 workspace/repository 级 ACL。
7. 不要求像素级复刻某一 VS Code 版本的非文件/Git 区域（欢迎页、账号、settings UI 全量等）。

## User scenarios

1. 用户在 local `/sessions` 选择 workspace 后打开代码工作台，进入 `/workspaces/:workspaceId/code`；Explorer 展示 workspace 根目录树，可展开、打开文本文件、编辑、保存。
2. 用户在 Cloud `/devices/:deviceId/sessions` 打开同一能力，进入 `/devices/:deviceId/workspaces/:workspaceId/code`；路由始终带正确 `deviceId`，请求经 Cloud 转发到所属 Agent。
3. 用户新建文件/目录、重命名、移动、删除条目；树与已打开编辑器状态保持一致（在 watch 或显式刷新策略下可恢复一致）。
4. 用户打开 SCM 视图，看到 staged / unstaged / untracked（或等价分组），选择变更打开 diff；可 stage/unstage，在确认后 discard 或删除 untracked，填写 message 后 commit。
5. 用户查看当前分支信息；在约定范围内创建本地分支或在 worktree 允许时切换本地分支（细节与后端 Git 能力对齐）。
6. 用户从工作台返回 sessions；切换 device/workspace 时不会把文件内容或 Git 状态串到错误上下文。
7. 遇到非仓库、Git 不可用、device offline、binary/too-large、冲突保存、权限失败时，得到可理解恢复路径（重试/刷新/返回），不泄露本机绝对路径或敏感诊断。

## Acceptance

### 工作台与路由

- [ ] local 提供可深链路由 `/workspaces/:workspaceId/code`；cloud 提供 `/devices/:deviceId/workspaces/:workspaceId/code`。
- [ ] 该路由为独立全屏 Workbench，不复用 `/files`、`/git`；SCM 在 Workbench 内打开，无需独立 `/git` 产品路由。
- [ ] 从 sessions 的 workspace 入口可进入 Workbench；返回 sessions 不破坏既有 session/terminal 状态。
- [ ] Workbench 为独立全屏页，不在 `/sessions` 内以 drawer/overlay 承载完整 Explorer+SCM。
- [ ] monaco-vscode-api 以 Workbench service override 初始化；不依赖 extension gallery。
- [ ] 包体与首屏：Workbench 路由级懒加载；产品可接受的加载说明或 skeleton（具体阈值 Spike 后写入 Spec）。

### 文件系统

- [ ] 后端 FS API 的方法集、参数、错误语义以 VS Code `FileSystemProvider` 为准（至少：`stat`、`readDirectory`、`readFile`、`writeFile`、`createDirectory`、`delete`、`rename`；建议 `watch`）。
- [ ] 前端 `FileSystemProvider` 为**薄封装**：URI/workspace 绑定与传输，**不**把旧 `file.proto`/list-content DTO 翻译成 VS Code 形状。
- [ ] 旧文件 API 的路径、字段名、结构定义允许**直接改成** Provider 所需形态；不为兼容旧前端而保留并行 schema。
- [ ] 读写以字节语义对接；文本编辑由 Workbench/Monaco 负责编码展示。
- [ ] 保存支持 create/overwrite；冲突策略在 Spec 中按 Provider/`FileSystemError` 语义定义并可恢复。
- [ ] 目录截断、binary、too-large 有明确 UX，不出现假空文件当成功。
- [ ] **变更感知**：`watch`（WS/SSE/长轮询）或文档化的等价刷新策略。
- [ ] 路径始终为 workspace-scoped logical path；后端解析 root，不信任客户端绝对路径。

### Git / SCM

- [ ] 后端 Git API 的资源模型、命令与 diff/original 内容接口以 VS Code **SCM Provider API**（及本产品选用的 Git 命令面）为准设计；**不**保留“旧 `git.proto` layer DTO → SCM resourceState”的长期桥接。
- [ ] 自定义 SCM Provider 填充 Source Control 视图；不安装官方 Git 扩展。
- [ ] 前端 SCM 代码直接消费新契约；旧 Git status/diff/mutation 形状可删除或整体替换，而非 adapter 兼容。
- [ ] 能展示当前变更列表并打开 diff；binary/too-large/unmerged 等显示不可用而非伪造 diff。
- [ ] 支持 stage、unstage；discard / delete untracked 需确认。
- [ ] 支持 commit（message 由 UI 收集；author/signing/hooks/凭据仍仅由 Agent 环境处理）。
- [ ] 文件保存或 mutation 后 SCM 可 refresh 到服务端权威 status。
- [ ] 分支 summary / create / switch 若纳入本期，契约同样以 SCM/Git 工作台需要为准；否则 Spec 列为二期并隐藏入口。

### local / cloud 与一致性

- [ ] local 与 cloud 使用同一套 **Provider 对齐** 的 API 契约（路径与数据结构一致）；cloud 保留 device ownership 与 offline 语义。
- [ ] target/workspace/device 切换取消或忽略过期响应，不串数据。
- [ ] Workbench 全局 initialize 约束下，切换 workspace 有明确 rebind 或 reload 策略且可测。

### 迁移与清理

- [ ] Workbench 成为文件+Git 主路径后，旧 Vue `WorkspaceFileTree` / `GitChangesPanel` 等主路径入口移除或降级，文档标注废弃。
- [ ] 既有终端 Session 流程回归通过。

### 验证证据

- [ ] Phase 0 Spike 记录（打包、FS 往返、SCM diff、包体）通过或否决有书面结论。
- [ ] 自动化或脚本化覆盖：打开/保存文件、基础 CRUD、status/diff、stage/commit 的 local 路径；cloud 至少授权/转发与关键回归。
- [ ] 前端 typecheck/lint/test 与相关 Go tests 在实现期可运行并通过约定集合。

## Scope by phase

| Phase | 交付 | 是否本需求 MVP 必须 |
|-------|------|---------------------|
| 0 Spike | Workbench 起页、假/真 FS 读、假/真 SCM 列表+diff、体积与切换策略 | 是（门禁） |
| 1 FS MVP | 远程 FS Provider + CRUD/保存/冲突 + `/code` 路由 | 是 |
| 2 SCM MVP | status/diff/stage/unstage/discard/commit + refresh | 是 |
| 2b 分支 | summary/create/switch | 默认是（若后端已具备）；否则 Spec 可降为二期 |
| 3 产品化 | cloud 对称、切换硬化、删除旧 UI 主路径、E2E | 是 |
| 二期 | 全文搜索、语言高亮包、multi-diff、remote git、VS Code Server | 否 |

## Open questions

1. FS 冲突策略选 revision token、etag 还是 mtime+size？
2. watch 一期用 WebSocket、SSE 还是带退避的轮询？
3. 分支 create/switch 是否强制纳入 MVP，还是可降二期？
4. 旧 Vue `/files`、`/git` 代码删除的时间点：与 Phase 3 绑定还是并行保留一个版本周期？

## Decisions

1. 前端底座采用 `@codingame/monaco-vscode-api` **Workbench** 模式，而非继续自研 Explorer/SCM chrome。
2. Workbench 使用独立全屏路由：local `/workspaces/:workspaceId/code`，cloud `/devices/:deviceId/workspaces/:workspaceId/code`；不复用 `/files`/`/git`。Git/SCM 仅在该工作台内呈现。
3. 文件与 Git 分别通过 **自定义 FileSystemProvider** 与 **自定义 SCM Provider** 接入；host 侧 `registerExtension(LocalProcess)` 仅用于注册 API，不引入扩展市场。
4. 主路径不使用 VS Code Server remote agent；不使用库 Terminal 替换 TermBridge Session。
5. **接口源**：以 CodinGame/monaco-vscode-api（VS Code）`FileSystemProvider` 与 SCM Provider（Git）所需接口为唯一契约源。Agent/Cloud 的路径、proto/DTO、错误码按该契约**直接改造或重定义**。
6. **禁止适配/桥接层**：不实现“旧 file/git API → Provider”的翻译层；旧实现可以改路径与数据结构，前端 Provider 只做传输与 workspace 绑定。
7. “严格受控 Git UI 语义”让位于 SCM 标准体验；后端仍可保留 workspace root 约束与安全日志边界（实现细节，不另造 UI DTO）。
8. Phase 0 Spike 为硬门禁：失败则不拆除现有 Vue 实现。迁移期内旧 `/files`、`/git` 可并存，主入口改为 `/code` 后按 Phase 3 移除旧主路径与旧 API。

## Risk

- **Watch 缺失**会导致 Explorer/SCM 体验回退到手动刷新；必须在 Spec 中固化方案。
- **全局 initialize** 使多 workspace/device 切换易泄漏状态；需要单一 Workbench 实例策略与测试。
- **包体与性能**可能影响门户观感；需懒加载与体积基线。
- **双实现并行**会拖垮维护；Phase 3 必须有删除旧主路径的明确完成定义。
- **与既有 requirement 漂移**：只读 Git 页、受控 Git 工作流、VS Code 样式对齐等文档需在 Accepted 后标注关系（替代 / 吸收 / 仍有效后端部分）。

## Related documents

- 评估：[`docs/analyze/20260717-monaco-vscode-api-files-git-evaluation.md`](../analyze/20260717-monaco-vscode-api-files-git-evaluation.md)
- 计划（草案）：[`docs/plan/20260717-monaco-vscode-api-files-git-workbench.md`](../plan/20260717-monaco-vscode-api-files-git-workbench.md)
- 前序：`docs/requirement/20260714-shared-workspace-text-view.md`、`docs/requirement/20260716-workspace-git-review-view.md`、`docs/requirement/20260717-controlled-git-workflow.md`、`docs/requirement/20260716-vscode-workbench-styling.md`

## User review notes

- 2026-07-17：基于评估讨论起草本需求；前提为“受控 Git 不作前端架构硬约束、可按需提供 API、只要文件+Git、不做扩展”。待用户审阅后更新 Review status。
- 2026-07-17：用户澄清 Workbench 使用独立全屏路由：local `/workspaces/:workspaceId/code`，cloud对称 `/devices/:deviceId/workspaces/:workspaceId/code`。
- 2026-07-17：用户澄清：以 monaco-vscode-api 所需 FileSystemProvider / SCM Provider（Git）接口为准；旧实现可直接改造成目标路径与数据结构，**不要适配或桥接**。


