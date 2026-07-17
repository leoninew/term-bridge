# Workbench SCM 资源 Git 操作入口验证

最后修改时间: 2026-07-17 17:26:53

Review status: Accepted

## Flow mode / Stage

轻量模式 / light；验证 / Verification。

## Requirement alignment

按 [`20260717-workbench-scm-resource-actions.md`](../requirement/20260717-workbench-scm-resource-actions.md) 核对。Requirement 已接受；light 模式无独立 Spec / Plan，本次直接按 Requirement 验证。

实现通过 `@codingame/monaco-vscode-api` 的 extension manifest、SCM menu contribution、native dialogs 和 notifications 交付，不接入官方 `vscode.git` 扩展，也未增加 Vue/DOM 自绘 SCM UI。

## Actual diff summary

暂存交付包含以下文件：

| 文件 | 实际变更 |
| --- | --- |
| `docs/requirement/20260717-workbench-scm-resource-actions.md` | 记录范围、批量语义、原生 UI 约束以及随后确认的 hover 行内 action 范围。 |
| `web/src/features/workbench/scmExtensionManifest.ts` | 声明 `termbridge-git` 的 SCM commands、文件/目录/分组的右键菜单，及文件/目录的 `inline@...` hover actions。 |
| `web/src/features/workbench/bootstrap.ts` | 仅暂存 system extension 使用 Workbench SCM manifest 的 integration hunk。 |
| `web/src/features/workbench/scmProvider.ts` | 将 Workbench resource state / group / folder 参数归一化为保留 group 的单路径 Git mutation；支持 Open Changes、一次 Discard 确认、首错停止和最终刷新。 |
| `web/src/features/workbench/scmExtensionManifest.test.ts` | 验证 manifest action surface、provider 限制、状态条件、inline actions 与图标。 |
| `web/src/features/workbench/scmProvider.test.ts` | 验证 mutation group、批量首错停止、刷新、Open Changes、Discard 确认与取消语义。 |

暂存区统计：6 files changed，1037 insertions，94 deletions。

## Expected vs actual changed files

| 预期范围 | 实际结果 |
| --- | --- |
| SCM Requirement | 已新增并暂存。 |
| Workbench extension manifest | 已新增并暂存。 |
| SCM command adapter | 已修改并暂存。 |
| SCM focused tests | 已新增并暂存。 |
| Bootstrap manifest 注册 | 已暂存对应 import 和 `registerExtension(...)` hunk。 |
| HMR、workspace subscription、filesystem provider 等其他 Workbench 改动 | 未纳入本次 SCM 暂存范围；`bootstrap.ts` 保留未暂存 hunk，未混入该交付。 |
| 后端 Git API / proto / Agent/Cloud authorization | 未修改。 |

## Acceptance checklist

- [x] Manifest 为 `termbridge-git` 声明 commands，并在 resource state、folder、group context menu 提供相应 Git action。
- [x] `scmProvider == termbridge-git` 显式限制所有 action；按 `changes`、`untracked`、`staged` 过滤适用命令。
- [x] 文件、目录、分组和多选通过 resource state / resource group 归一化为目标资源；目录路径不直接作为 mutation path。
- [x] Add 和 Discard 保留 `untracked` group，未错误复用 `changes`。
- [x] 批处理按稳定排序逐项执行，首个失败停止，最终刷新 SCM status；不声称后端不存在的原子回滚。
- [x] Discard 使用一次 Workbench 原生 modal 确认；取消时不发 mutation 或刷新。
- [x] 失败消息以安全 operation state 映射提供反馈，不透传后端原始 message。
- [x] 文件和目录有原生 `inline@...` hover actions，右键普通 group 入口保留；分组标题只提供右键全量操作。
- [x] Open Changes 兼容原有文件点击 `[Uri, groupId]` 与 Workbench 菜单转发的 resource state 集合。
- [x] 未增加官方 Git extension 依赖，未改动 Agent/Cloud Git authorization 边界。

## Test results

| 检查 | 结果 |
| --- | --- |
| `git diff --cached --check` | 通过。 |
| Focused Vitest：`scmExtensionManifest.test.ts`、`scmProvider.test.ts` | 通过：2 test files、11 tests。 |
| Focused ESLint：四个 SCM implementation/test 文件 | 通过。 |
| Focused Prettier：四个 SCM implementation/test 文件 | 通过。 |
| `vite build` | 通过。存在既有 `optimizeDeps.esbuildOptions` deprecation、`node:fs/promises` browser externalization 和大 chunk warnings，均未阻断构建。 |
| `vue-tsc --noEmit` | 未通过，阻塞于当前 SCM 暂存范围外的 `platformFileSystemProvider.ts`：`IStat` 不接受 `etag`（4 处）。 |

## Manual validation

未完成真实浏览器 SCM smoke test。应在 Workbench 中验证：

1. `Changes`、`Untracked`、`Staged Changes` 的文件 hover 显示正确的原生图标 action；
2. 同一文件/目录的右键菜单仍存在，且无关 SCM provider 不显示 TermBridge action；
3. 目录 hover / right-click 的 Stage、Add、Unstage、Discard 能处理其后代资源；
4. Open Changes 打开文件 diff；目录 Open Changes 依稳定顺序打开后代资源 diff；
5. Discard 的一次 modal、取消、首错停止和刷新后的资源状态符合预期。

## Scope deviation and risks

- 无意外的已暂存范围扩张；`bootstrap.ts` 已使用 partial staging 隔离 SCM manifest integration，HMR/workspace 的工作区改动仍未暂存。
- 后端为单路径 mutation contract，目录/分组批次不能原子化，也不会 rollback 已成功的前置项；这是 Requirement 已接受的语义。
- 真实浏览器交互尚未 smoke test，因此 hover 的视觉呈现和 Workbench runtime 参数转发仍需人工确认。
- 全量 typecheck 当前被不相关的 `platformFileSystemProvider.ts` 类型错误阻塞；本次未为了绿灯修改无关文件。

## Conclusion

代码级、focused unit、lint、format、暂存区 whitespace 和 production build 验证通过；本次 SCM 交付满足 Requirement 的静态和行为级测试证据。交付前仍保留两项人工/环境验证：真实浏览器 SCM hover/right-click smoke test，以及修复或隔离现有 `platformFileSystemProvider.ts` 全量 typecheck 错误。
