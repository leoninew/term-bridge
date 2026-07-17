# Workbench SCM 资源 Git 操作入口

最后修改时间: 2026-07-17 16:16:54

Review status: Accepted

## Flow mode / Stage

轻量模式 / light；需求 / Requirement（Accepted，用户已授权进入 Implementation / 实现）。

## Background

TermBridge 已通过 `@codingame/monaco-vscode-api` Workbench 和自定义 `termbridge-git` SCM Provider 展示 `Staged Changes`、`Changes`、`Untracked` 三类变更，并支持状态读取、diff 和后端 Git mutation 契约。

当前 SCM 文件及目录节点缺少 Stage/Add、Unstage、Discard 等常规 Git 操作入口。调查确认：`monaco-vscode-api` 的 SCM Workbench 已提供资源、目录与资源分组的行内操作和右键菜单槽位，但 TermBridge 尚未在宿主扩展 manifest 中为自定义 Provider 贡献对应命令与菜单；现有 mutation command handler 的参数模型也尚不能正确处理 SCM 菜单传入的资源状态、目录展开及多选资源。

## Goal

在不接入官方 VS Code Git 扩展、不改变 Agent/Cloud Git 执行边界的前提下，为 `termbridge-git` SCM Provider 的变更资源补齐符合 VS Code SCM 交互习惯的 Git 操作入口：

- 对普通未暂存变更提供 Stage 与 Discard；
- 对 untracked 变更提供 Add 与 Discard（删除未跟踪文件）；
- 对 staged 变更提供 Unstage；
- 对目录节点提供其后代资源的相应批量操作；
- 对资源分组提供 Stage All、Unstage All、Discard All 等完整的全量操作入口；
- 操作完成或失败后保持 SCM 状态、错误反馈和刷新结果一致，批量操作遇到首个失败即停止。

## Non-goal

- 不接入、移植或依赖浏览器端不可用的官方 `vscode.git` 扩展及其本地 Repository 模型。
- 不重做 SCM 视图、目录树、右键菜单或行内 action UI；应复用 `monaco-vscode-api` 已有 SCM contribution 机制。
- 不在本任务扩展 Git 功能范围到 fetch、pull、push、merge、rebase、cherry-pick、stash 或远程仓库管理。
- 不以本任务为由重构既有文件系统 Provider、会话/终端流程或 workspace 授权模型。
- 不要求一期引入新的后端 bulk Git mutation 协议；若现有单路径协议可由前端资源批处理安全实现，则优先沿用。

## User scenarios

1. 用户打开 Source Control 视图，在 `Changes` 中悬停或右键一个已修改文件，可执行 **Stage Changes** 或 **Discard Changes**；操作后的文件移入 `Staged Changes` 或从变更列表恢复。
2. 用户在 `Untracked` 中对文件执行 **Add**，文件进入 `Staged Changes`；执行 **Discard** 时，经确认后删除该未跟踪文件。
3. 用户在 `Staged Changes` 中对文件执行 **Unstage Changes**，文件回到未暂存变更。
4. 用户右键 SCM 资源树中的目录，可对目录内所有适用变更执行相应 Stage、Unstage 或 Discard 操作；多选资源时行为一致。
5. 用户从分组标题执行 Stage All、Unstage All 或 Discard All；批量操作在首个失败时停止，展示已完成范围和失败原因，并刷新服务端权威状态。
6. 操作入口只作用于 `termbridge-git` Provider，不影响未来可能注册的其他 SCM Provider。

## Acceptance

- [ ] 宿主扩展 manifest 为 `termbridge-git` 注册所需 commands，并在 `scm/resourceState/context`、`scm/resourceFolder/context`、`scm/resourceGroup/context` 贡献相应 Git action，交付文件、目录和分组的完整操作入口。
- [ ] 菜单可见性以 `scmProvider == termbridge-git` 及资源/分组状态为条件：`changes`、`untracked` 显示 Stage/Add 与 Discard，`staged` 显示 Unstage；无关 Provider 或状态不显示这些操作；文件和目录节点同时交付原生 hover 行内 action 与右键上下文菜单入口。
- [ ] 文件资源、目录资源和多选资源的 mutation handler 能正确接收 Workbench 传入的 `SourceControlResourceState` 集合，并使用每个资源的 `resourceUri` 和所属 group 生成请求。
- [ ] `Untracked` 的 Add 和 Discard 分别以 `untracked` group 调用后端，保留后端的 stage 与删除未跟踪文件语义；不得错误复用 `changes` group。
- [ ] 每次批量操作对各资源使用当前已支持的单路径 Git mutation；最终刷新一次服务端权威 SCM status。批量操作遇到首个失败即停止，已完成结果保留，失败项及已执行数量可识别，并在刷新后反映真实状态。
- [ ] Discard、删除 untracked 文件及 Discard All 均有一次明确的批量确认交互，显示受影响文件数量；取消确认不得发起 Git mutation。
- [ ] 操作成功、失败和刷新异常向用户提供可理解的 Workbench 反馈，且不泄露 Agent 本地绝对路径、命令行细节或敏感错误内容。
- [ ] 对菜单 contribution 可见性、resource-state 参数转换、untracked group 语义、目录/多选批处理、确认取消及部分失败刷新增加有业务语义的测试。
- [ ] 不增加官方 Git extension 依赖，也不改变既有 Agent/Cloud workspace 授权边界。

## Open questions

暂无需要用户确认的未决事项。

实现前需确认当前 `monaco-vscode-api` 版本的 SCM menu contribution 是否能通过 `group` / `when` 仅进入上下文菜单而不同时渲染行内 action；若框架没有这一显示控制，则采用已支持的菜单分组与 presentation 配置实现“右键优先”，并将实际表现记录在实现结果中。

## Decisions

1. 复用 `@codingame/monaco-vscode-api` SCM Workbench 的 `contributes.menus` 与 command 机制，不自研 Vue action 菜单。
2. 保留自定义 `termbridge-git` SCM Provider 与 TermBridge 远程 Git API；不接入官方 Git extension。
3. 文件、目录和资源分组的常规 Git 操作均为本期交付范围：Stage/Add、Unstage、Discard 及其对应全量操作。
4. 菜单的资源与目录操作 handler 以 variadic `SourceControlResourceState` 为参数模型，而非仅接收单个 `Uri`。
5. 目录、多选与分组全量操作由 Workbench 提供资源集合，前端适配层按资源顺序执行既有单路径 mutation；遇到首个失败立即停止，在整批结束后刷新。
6. Discard、删除 untracked 和 Discard All 均使用一次总确认，明确显示受影响文件数。
7. 文件和目录节点交付 SCM 原生 hover 行内 action，并保留既有右键上下文菜单；资源分组标题仍只提供右键上下文菜单。
8. UI 必须完全使用 `@codingame/monaco-vscode-api` 已有 Workbench、SCM menu、dialog 与 notification 能力；不得自绘 Vue/DOM UI，原生能力不满足时暂停并与用户沟通决策。
9. menu `when` 条件必须显式限定 `scmProvider == termbridge-git`；根据资源状态使用 `scmResourceState` 或根据分组使用 `scmResourceGroup`，不沿用官方 Git extension 的 `scmProvider == git` 条件。
10. hover 行内 action 包含 Open Changes：文件打开其 diff；目录由 Workbench 传入后代资源集合并依稳定顺序逐个打开 diff。

## Risk

- 当前后端单路径 mutation 使大量目录/分组操作不能保证原子性；前端必须避免把目录路径直接作为 Git path 发送，并向用户清晰呈现部分失败。
- SCM 菜单调度传入资源状态或分组上下文，若 handler 仍按 `Uri` 处理，将导致运行时操作失败；实现前需用 upstream demo/VS Code API 的参数语义校验。
- `Untracked` 操作若错误标记为 `changes`，会触发后端状态校验失败或执行错误 discard 语义。
- hover 行内 action 需要与右键菜单复用相同 command adapter；Open Changes 必须兼容既有文件点击的静态 URI 参数与 Workbench 自动转发的 resource-state 参数。
- `monaco-vscode-api` SCM 的 `inline` group 会原生渲染为 hover action bar；普通 group 保留为右键菜单入口。上游版本升级时需复验该 presentation 行为。
- 上游 `monaco-vscode-api` 与 vendored VS Code 版本升级时，SCM context key、proposal 或菜单表现可能变化；测试需覆盖可见性和 command dispatch 语义。

## Related documents

- 主工作台需求：[`docs/requirement/20260717-monaco-vscode-api-files-git-workbench.md`](20260717-monaco-vscode-api-files-git-workbench.md)
- 技术评估：[`docs/analyze/20260717-monaco-vscode-api-files-git-evaluation.md`](../analyze/20260717-monaco-vscode-api-files-git-evaluation.md)
- 现有实现：`web/src/features/workbench/scmProvider.ts`、`web/src/features/workbench/bootstrap.ts`
- 上游源码与 demo：`D:/SourceCodes/opensource/monaco-vscode-api`

## User review notes

- 2026-07-17：用户要求记录此任务。调查结论：Workbench SCM 的菜单、资源树、目录节点与 command dispatch 为 monaco-vscode-api 已有能力；TermBridge 需自行贡献菜单、实现 remote Git command adapter，并正确处理 resource-state、目录、多选与 untracked group 语义。
- 2026-07-17：用户确认本期交付文件、目录、分组全量操作；批量操作在首个失败时停止；Discard 采用一次总确认；不实现 SCM 行内操作入口，仅提供右键上下文菜单。
- 2026-07-17：用户接受 Requirement 并授权进入 Implementation。UI 必须完全复用 `@codingame/monaco-vscode-api` 能力；若原生能力无法满足“仅右键”的约束，暂停实现并沟通，不自绘任何 UI。
- 2026-07-17：用户调整范围，要求为 SCM 文件和目录节点添加原生 hover 行内操作。该能力由 `inline` menu group 支持；保留右键菜单，不自绘 UI。
