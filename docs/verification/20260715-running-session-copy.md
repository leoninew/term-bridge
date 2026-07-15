# 运行中会话复制入口验证

最后修改时间: 2026-07-15 08:25:54

Review status: Accepted

## Requirement alignment

依据 `docs/requirement/20260715-running-session-copy.md` 核对。

- 左侧树仅在 `running` 会话节点渲染 copy icon，且图标位于运行状态点左侧；图标默认隐藏，仅节点 hover 或键盘聚焦时显示。既有编辑、重跑、停止和删除入口的展示规则未改变。
- copy icon 使用 `@click.stop` 发出 `copySession`，不会触发会话节点选择；`SessionsPageShell` 接入后复用既有 `CreateSessionDialog` 与创建提交路径。
- 复制草稿预填原会话的工作区、目录、原始命令及命令来源。快捷方式快照仍完整且对应当前启用快捷方式时保持快捷方式来源；不可用或不完整快照降级为直接命令并保留原命令。
- 名称通过当前已加载会话树的全部会话名称计算，从 `<原名称>_1` 起选择第一个未占用序号。

## Spec alignment

不适用。当前为 light / 轻量模式，按 Requirement / 需求核对。

## Plan alignment

不适用。当前为 light / 轻量模式，按 Requirement / 需求核对。

## Actual diff summary

本次暂存范围包括：

- `WorkspaceSessionSidebar.vue`：运行会话节点增加 hover/focus 可见的 copy icon、无障碍标签及 `copySession` 事件。
- `SessionsPageShell.vue`：将 copy 事件映射到复制预填草稿并打开既有创建会话模态窗。
- `useCreateSessionDraft.ts`：增加复制会话预填与递增名称计算；处理快捷方式快照不可用或不完整时的直接命令降级。
- `useCreateSessionDraft.test.ts`：覆盖首个可用序号、快捷方式预填和快捷方式失效时保留命令的降级规则。
- `20260715-running-session-copy.md`：本功能的 light Requirement。

`web/src/i18n.ts` 同时包含另一项云端认证工作新增的翻译文本；为避免把无关改动纳入暂存，本次 copy icon 的两条标签翻译未暂存。该文件工作树仍保留 copy icon 所需文本，因此当前运行验证可用；提交前需要与该文件所属工作一起拆分，或以交互式暂存仅选择 copy icon 翻译 hunk。

## Expected vs actual changed files

| 预期范围 | 实际情况 |
| --- | --- |
| 左树运行节点与图标行为 | 已修改 `WorkspaceSessionSidebar.vue` |
| 创建会话对话框入口和草稿预填 | 已修改 `SessionsPageShell.vue`、`useCreateSessionDraft.ts` |
| 名称/来源状态规则测试 | 已修改 `useCreateSessionDraft.test.ts` |
| 中英文 a11y 标签 | 已修改但未暂存的 `web/src/i18n.ts`；与云认证翻译混在同一工作树文件 |
| Requirement 文档 | 已新增并更新为 `Accepted` |
| Verification 文档 | 已新增本文 |
| 后端、protobuf、迁移 | 未修改，符合纯前端范围 |

## Acceptance criteria checklist

- [x] 仅运行中会话提供 copy icon，位于运行状态点左侧；默认隐藏，hover 或键盘聚焦时出现。
- [x] 点击 copy icon 不选择原会话，并打开既有新建会话模态窗。
- [x] 模态窗预填原会话的名称、目录、原始命令和可用快捷方式来源；不可用快捷方式安全降级为直接命令。
- [x] 复制名称按 `<原名称>_<n>` 从 1 起选取当前前端会话树中的首个未使用序号。
- [x] 原会话、树排序、生命周期操作及既有新建入口未被复制流程修改。
- [x] 新增状态规则测试覆盖递增名称、快捷方式预填和快捷方式失效降级。
- [x] 用户已在本地实际页面中自行验证 copy icon 到新建模态窗的交互流程；按用户要求未使用 Pomelo PW 自动化。

## Runtime verification

| 方式 | 结果 |
| --- | --- |
| 用户在本地 `/sessions` 页面手工观察并操作运行会话 copy icon | 通过；用户确认自行验证，copy 入口可打开预填的新建会话模态窗 |
| 相邻交互约束 | 通过；按用户反馈将 copy icon 调整为仅节点 hover 或键盘聚焦时显示，其他图标保持原有展示规则 |
| 自动化浏览器验证 | 未执行；用户明确要求不使用 Pomelo PW，手工验证替代 |

## Command results

以下命令在进入 Verification 前的 Implementation 阶段执行，作为静态回归证据；本验证阶段不将其作为运行时结论的替代：

| 命令 | 结果 |
| --- | --- |
| `npm --prefix web run test -- "src/composable/useCreateSessionDraft.test.ts"` | 通过：1 个测试文件、9 个测试 |
| `npm --prefix web run lint -- ...` | 通过：本功能涉及前端文件无 lint 输出 |
| `web/node_modules/.bin/vue-tsc.cmd --noEmit -p web/tsconfig.json` | 通过 |
| 针对本功能文件的 Prettier 检查 | 通过 |
| `git diff --cached --check` | 通过 |

## Scope deviations

- 按用户要求仅暂存本功能的 Requirement 和五个前端实现/测试文件；未提交。
- `web/src/i18n.ts` 与正在进行的云认证翻译改动共存，未能安全地纳入本次暂存范围；copy icon 标签文本仍在工作树中并参与当前本地运行。
- 当前工作区存在大量云认证、配置、迁移及文档改动，均未作为本功能的验证或暂存内容。

## Risks

- 复制名称仅根据当前前端已加载的会话树计算；其他浏览器标签或客户端并发创建时，仍可能产生同名候选。这符合 Requirement 的纯前端边界。
- 本次运行证据为用户手工验证，未保留自动化截图或录制；用户明确选择不使用 Pomelo PW。
- 在本功能提交前，必须将 `web/src/i18n.ts` 的两条 `copySessionAria` 翻译从云认证文本中精确拆分并暂存；否则提交内容会缺少新增标签的国际化资源。

## Incomplete items

1. 将 `web/src/i18n.ts` 中中英文 `copySessionAria` 两个 hunk 与无关云认证翻译拆分后暂存，再提交本功能。

## Conclusion

运行中的会话节点 copy 入口、hover 可见性、事件隔离、现有创建对话框复用、名称递增和命令/快捷方式预填均与 Requirement 对齐。用户已完成本地手工运行验证，并明确不使用 Pomelo PW。前端静态检查与草稿状态测试通过。由于 i18n 文件混有无关云认证改动，本功能相关的两个翻译 hunk 尚未暂存；在完成精确暂存前不应提交。本验证结论为已接受交付，但保留上述暂存整理项。
