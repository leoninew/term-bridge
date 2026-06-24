# SessionsView Complexity Decomposition Requirement
最后修改时间: 2026-06-24 13:03:36

## Review status

Accepted

## Flow mode / Stage

严格模式 / strict；需求 / Requirement 已接受，当前进入规格 / Spec。

## Background

`web/src/views/SessionsView.vue` 当前不是单纯“组件偏长”，而是事实上的 frontend application controller。它同时承载：

- auth 初始化、登录表单、登录态判断；
- device 列表加载、当前 device 选择、source change reset；
- workspace tree 拉取、workspace/session projection、本地增删改同步；
- session create/edit/delete/stop/rerun orchestration；
- session tabs、active tab、history loading/cache/error；
- terminal attach URL 组装、terminal state/error 事件处理；
- create/edit/delete/remove dialog 与 form state；
- toast queue 与 error presentation；
- create session 默认值、command parsing、terminal 初始尺寸测量。

这导致页面组件同时承担 layout、domain state、application use case、transport coordination、UI feedback 多层职责，长期会形成以下风险：

1. workspace-scoped session API contract 已经要求 `(device_id, workspace_id, session_id)` 定位；前端交互流程是先选择 device，再获取该 device 下的 `workspaceTree`，因此 device 是页面上下文。用户确认两个 workspace 下不会出现相同 `session.id`，所以 UI/internal active key 可使用裸 `session.id`，但 API/domain mutation 仍应显式携带 `workspace_id + session.id`。
2. `workspaceTree` 已经包含展示 workspace 和 session 所需的足够信息；继续维护 `workspaceTree`、`workspaces`、`sessions` 多份可变状态会制造不必要同步风险。
3. workbench tab 与 history loading 混在页面内，使 terminal/history 生命周期难以独立验证。
4. dialog/form/defaults/toast 横切关注点散落在主视图，任何新行为都会继续扩大 `SessionsView.vue`。
5. 继续按“先抽一点 auth/device”的思路只能减少局部行数，不能解决会话定位、domain state 和 orchestration 边界不一致的问题。

本任务采纳资深开发视角：不按 MVP 或低风险表层切片处理，而是以完整职责分解为目标，建立长期一致的 frontend architecture boundary。实现仍可按阶段落地和验证，但需求目标必须是彻底分解，而不是“够用即可”。

## Goal

1. 将 `SessionsView.vue` 从 application controller 降级为 composition shell：主要负责页面布局、组件组合和少量顶层事件转发。
2. 建立清晰的 frontend application boundary，至少拆分以下职责：
   - Gateway/Auth/Device state；
   - Workspace/Session domain state；
   - 会话定位与 tab/history 查找规则；
   - Workbench tabs/history/active session state；
   - Terminal pane coordination boundary；
   - Create session draft/defaults/validation；
   - Edit/delete/remove dialogs 或 dialog state；
   - Toast/notification queue。
3. 前端会话定位必须与 workspace-scoped contract 和当前页面上下文对齐：device 由“先选设备，再加载对应 `workspaceTree`”的页面上下文提供；用户确认两个 workspace 下不会出现相同 `session.id`。因此 API 调用、domain mutation、history 读取等 workspace-scoped 路径必须使用已有 `SessionSummary.workspace_id` 和 `SessionSummary.id`；UI/internal active key、tab value、pending id 等 presentation 状态可以使用裸 `session.id`，但该用法依赖 session id 全局唯一不变量。
4. Workspace/session domain state 以 `workspaceTree` 作为单一事实源；`workspaces`、`sessions` 等平铺数据若仍需要，只能作为 computed projection 或局部只读派生结果，不得作为第二份可变事实源。
5. Workbench tabs/history 必须独立于页面组件：打开/关闭/激活 tab、history loading/cache/error、rerun 后 history invalidation、workspace/session 删除后的 tab 清理，都应集中在 workbench 层或明确的 application state 层。
6. Terminal rendering/xterm internals 不作为本任务重写目标，但 `SessionsView.vue` 不应长期负责 terminal URL 拼接、running/history pane 切换和 terminal event 到 domain refresh 的细节；需要通过拆分组件建立 terminal pane coordination boundary。
7. Create session 的 default command、default cwd、default name、command text parsing、required validation 应从主视图抽离为 draft/defaults/validation 逻辑。
8. Dialog/form state 和 toast queue 应从主视图抽离或组件化，避免业务 action 与 UI feedback 在 `SessionsView.vue` 中继续交织。
9. `web/src/store` 与 `web/src/composable` 各司其职：store 承载需要跨组件共享或代表应用/domain 生命周期的状态；composable 承载可复用交互逻辑、派生逻辑或局部副作用封装。具体模块归属在 Spec / Plan 中按职责确定。
10. 保持现有用户可见行为不变；本任务是架构分解和职责收敛，不引入新产品功能。
11. 遵守全局约束：不无声改代码；不以“编译能过”为交付底线；不做未经授权的 git 写操作。

## Non-goal

1. 不改变后端 API contract；本任务聚焦 frontend architecture decomposition。
2. 不重写 xterm rendering、terminal socket protocol、`TerminalView` 内部终端渲染细节，除非 Spec / Plan 阶段证明需要为了边界隔离做最小适配。
3. 不引入新的状态管理框架；项目已经注册 Pinia，若需要全局/feature store，应优先使用现有 Pinia 能力。
4. 不改变登录、设备选择、workspace tree、session tabs、terminal attach、history 展示、session create/edit/delete/stop/rerun、workspace remove/reorder、offline readonly 的用户可见行为。
5. 不把错误吞掉或无限容忍异常状态；对缺失 workspace/session、非法 command/form 状态应在合理边界 fail-fast 或显式返回 validation error。
6. 不做“只抽一两个函数减少行数”的表层整理；拆分必须服务清晰边界和长期维护。
7. 不执行 git add / commit / push / checkout / stash / reset / rebase / merge 等 git 写操作。

## User scenarios

1. 作为维护者，我希望打开 `SessionsView.vue` 时看到的是页面结构和子模块组合，而不是完整应用状态机。
2. 作为后续开发者，我希望新增 session 操作时只修改 session domain/workbench/dialog 对应模块，而不是继续扩大主视图。
3. 作为前端调用方，我希望所有 API/domain session 操作都明确带上 workspace，并依赖当前 selected device 上下文；UI/internal active key 可以使用裸 `session.id`，因为用户确认 session id 在当前 device/workspace tree 内全局唯一。
4. 作为验证者，我希望 workspace/session state 以 `workspaceTree` 为明确单一事实源，可以审查某个 mutation 是否只进入一个边界，而不是同时追踪 tree、flat workspaces、flat sessions 多份可变副本。
5. 作为用户，我在登录、选择设备、打开 session、查看 running terminal、查看历史、创建/编辑/删除/停止/重跑 session、移除/排序 workspace 时，行为和文案不应因为架构分解发生非预期变化。
6. 作为后续维护者，我希望 terminal rendering 内部可以保持稳定，同时 terminal pane 的外部 orchestration 不再绑死在 `SessionsView.vue`。
7. 作为代码审查者，我希望 diff 能按照架构边界拆分，验证每个新模块的职责、输入、输出和失败语义。

## Acceptance

- [ ] API 调用、domain mutation、history 读取、workspace/session store 查找等 workspace-scoped 路径，直接使用已有 `SessionSummary.workspace_id` + `SessionSummary.id`；UI/internal active key、tab value、pending operation id 可使用裸 `session.id`，前提是维护用户确认的 session id 全局唯一不变量；不引入新的领域抽象名词。
- [ ] Workspace/session domain state 被迁出 `SessionsView.vue`，并以 `workspaceTree` 形成单一事实源；`SessionsView.vue` 不再直接维护 `workspaceTree`、`workspaces`、`sessions` 多份 mutable collection 的同步细节。
- [ ] Workspace/session mutation 入口集中化：refresh tree、upsert/update session、remove session、remove workspace、reorder workspaces 等逻辑进入 domain store/composable/application service，并保持 workspace-scoped semantics。
- [ ] Workbench tabs/history state 被迁出 `SessionsView.vue`：open/close/activate tab、active session resolution、history loading/cache/error、rerun history invalidation、source change reset、删除 workspace/session 后 tab 清理均由 workbench 边界处理。
- [ ] Gateway/Auth/Device state 被迁出 `SessionsView.vue`：auth initialization、login、device loading、selected device、device switch 语义有明确边界；device switch 与 workspace/workbench reset 的 orchestration 明确且可审查。
- [ ] Create session draft/defaults/validation 被迁出 `SessionsView.vue`：UA 推断 command、default cwd、default name、command parsing、name/cwd/command required validation 不再散落在主视图。
- [ ] Edit session、delete session、remove workspace 等 dialog state 被抽离为明确组件或 composable；主视图不再持有所有 dialog target/pending/form 字段的细节。
- [ ] Toast queue / notification state 被抽离；业务 action 与 toast presentation 的边界按 Vue/Pinia 主流实践处理，不把 domain state 无边界强绑定到 i18n/toast side effect。
- [ ] Terminal pane coordination 通过拆分组件形成明确边界：running terminal vs history pane 切换、terminal ws URL 获取、terminal state/error event 转换为 session refresh 或通知的逻辑不继续散落在主视图；`TerminalView` / xterm internals 不被无谓重写。
- [ ] `web/src/store` 与 `web/src/composable` 职责清晰：store 管理共享应用/domain 状态，composable 管理可复用交互逻辑、派生逻辑或局部副作用；不创建新的巨型 controller。
- [ ] `SessionsView.vue` 保留为 composition shell：模板可组合 sidebar、workbench、dialogs、toast；script 主要 wiring stores/composables/components，不包含大段 domain mutation 或 use-case orchestration。
- [ ] 所有现有用户可见行为保持一致，包括 auth gate、login、device selection、workspace tree、session tabs、terminal attach、history display、create/edit/delete/stop/rerun、workspace remove/reorder、offline readonly。
- [ ] 相关 frontend typecheck / lint / tests 通过；如现有项目缺少某类测试入口，verification 阶段必须记录原因并用其他检查补足。具体测试补充由后续 Spec / Plan 阶段自行评估。
- [ ] 不执行 git 写操作。

## Open questions

暂无需要用户确认的未决事项。以下事项已按用户反馈收敛为决策，后续 Spec / Plan 阶段只需给出技术细化：

- workspace/session domain state 采用 `workspaceTree` 单一事实源。
- device 由先选设备后的页面上下文提供；用户确认两个 workspace 下不会出现相同 `session.id`。API/domain 路径使用现有 `workspace_id + session.id`，UI/internal active key 可使用裸 `session.id`。
- `web/src/store` 与 `web/src/composable` 各司其职。
- toast/notification 边界按 Vue/Pinia 主流实践选取。
- terminal pane coordination 拆分组件。
- 测试补充由实现规划阶段自行评估。

## Decisions

- 本任务保持严格模式 / strict，因为它涉及前端应用架构边界、跨 feature state、terminal/history 行为和 workspace-scoped session 操作。
- 采纳资深开发分析：不再把本任务定义为“低风险切片”或“先抽 auth/device 的 MVP”；需求目标改为彻底职责分解。
- 实现可以为了审查和回归控制分阶段提交/验证，但阶段目标必须服务完整架构分解，不接受“只减少行数、不建立边界”的交付。
- 前端必须正视 workspace-scoped session contract；API 调用、domain mutation 和 history 读取不得丢失 workspace 维度。
- 因用户操作流是先选择 device，再获取对应 `workspaceTree`，device 是当前页面上下文；用户确认两个 workspace 下不会出现相同 `session.id`。会话 API/domain 定位使用现有 `SessionSummary.workspace_id + SessionSummary.id`，UI/internal active key 可使用裸 `session.id`，不额外发明 `SessionRef` / `SessionKey` 这类领域术语。
- Workspace/session domain state 以 `workspaceTree` 为单一事实源；`workspaces`、`sessions` 若存在，应为 computed projection 或只读派生，不作为可变事实源。
- Terminal rendering/xterm internals 不是本任务重写对象；terminal pane orchestration boundary 通过拆分组件实现。
- Pinia 已在项目注册，使用 Pinia 不视为引入新框架；`web/src/store` 和 `web/src/composable` 按职责分工：共享应用/domain 状态进 store，可复用逻辑和局部副作用进 composable。
- Toast/notification 设计在 Spec 阶段按 Vue/Pinia 主流实践细化，原则是边界清晰、可测试、不制造无边界 side effect。
- 测试覆盖由后续 Spec / Plan 阶段自行评估，不再要求用户预先指定测试类型。
- 本任务只更新当前新需求文档，不修改历史相邻 SpecFlow 文档，除非后续用户明确要求。

## Risk

1. 分解彻底会触碰多个边界，若 Spec / Plan 不先固定会话定位、source-of-truth 和 orchestration 规则，实施阶段容易出现半拆分、双状态或行为漂移。
2. 会话 API/domain 定位不包含 deviceId 的前提是页面上下文严格遵守“先选 device，再加载对应 workspaceTree”；UI/internal active key 使用裸 `session.id` 的前提是 session id 在当前 device/workspace tree 内全局唯一；device switch 必须完整 reset workspace/workbench，避免跨 device 残留 tab/history。
3. Store/composable 可能把耦合从 `SessionsView.vue` 平移到新的大 store；需要用职责边界限制每个模块的输入输出，而不是创建“另一个巨型 controller”。
4. History loading 和 terminal pane coordination 与用户可见行为强相关；虽然不重写 xterm internals，但边界迁移仍可能造成 running/history 切换、rerun history invalidation、terminal state refresh 的回归。
5. Toast/i18n 与 domain action 的边界若处理不当，会造成 store 难测试、side effect 难追踪。
6. 现有前端测试可能不足，verification 不能只依赖 typecheck；需要尽可能增加纯函数/store 层测试，或记录缺口并给出手动验证路径。
7. 大范围 frontend refactor 可能与并行功能开发冲突；Plan 阶段需要明确文件边界和回滚策略。

## User review notes

- 初始用户指令：严格模式，新任务；`SessionsView.vue` 现在承担太多职责，建议下一阶段降低复杂度，原始建议顺序为 auth/device、new session defaults、dialog form、workspace/session 数据流，并提醒 terminal attach/history 不建议第一步动。
- 用户后续要求：读取全局 `~/.claude/CLAUDE.md`，无视“低风险”要求，从资深开发视角分析和提出建议。
- 分析结论：核心问题不是 auth/device 行数，而是 `SessionsView.vue` 混合 application controller、多份 domain state、裸 `session.id` 定位、workbench/history、terminal coordination、dialog/form/toast 等多层职责。
- 用户进一步要求：严格模式采纳建议，不要 MVP 思路，分解彻底。
- 用户对未决事项的反馈：`workspaceTree` 已经有足够信息展示工作区和会话；当前流程是先选设备再得到对应 `workspaceTree`，上下文自洽；两个 workspace 下不会出现相同 `session.id`；`web/src/store` 和 `web/src/composable` 每司其职；toast/notification 按主流实践选取；terminal coordination 拆分组件；测试由 agent 自行评估。
- 用户纠正：不要发明 `SessionRef` / `SessionKey` 等过多术语；会话定位应直接使用已有字段表达。
