# Workbench Vite HMR 全局服务初始化稳定性验证

最后修改时间: 2026-07-17 17:54:30

Review status: Draft

## Flow mode / Stage

轻量模式 / light；验证 / Verification。

## Requirement alignment

按 [`20260717-workbench-hmr-initialization.md`](../requirement/20260717-workbench-hmr-initialization.md) 核对。Requirement 已接受；light 模式无独立 Spec / Plan，本次直接按 Requirement 验证。

实现将 HMR retain、init terminal 边界与 reload 判定抽到 `bootstrapHmrState.ts`，并在 `bootstrap.ts` 中接入库侧 `servicesInitialized` 作为真相源。

## Spec alignment

不适用（light / 轻量模式无 Spec）。

## Plan alignment

不适用（light / 轻量模式无 Plan；按 Requirement 实现）。

## Actual diff summary

本次 HMR 相关暂存交付包含：

| 文件 | 实际变更 |
| --- | --- |
| `docs/requirement/20260717-workbench-hmr-initialization.md` | 记录问题背景、目标、决策（含 `servicesInitialized` 真相源与 bootstrap self full-reload）。 |
| `docs/verification/20260717-workbench-hmr-initialization.md` | 本验证记录。 |
| `web/src/features/workbench/bootstrapHmrState.ts` | 新增 `retainHmrState`、`startWorkbenchInitialization`、`shouldReloadWorkbenchPage`、`isServicesAlreadyInitializedError`。 |
| `web/src/features/workbench/bootstrapHmrState.test.ts` | 覆盖 retain、pre-init 重试、post-init terminal、库已 init 的 lost-state、reload 判定与错误识别。 |
| `web/src/features/workbench/bootstrap.ts` | 接入 HMR retained state、`servicesInitialized` 探测、lost-state / missing-host reload、bootstrap `hot.accept` full reload、`stateVersion` 校验。 |
| `web/src/components/workbench/CodeWorkbenchShell.vue` | 路由卸载时 `disposeMountedWorkbench()`，只释放 workspace 级 subscriptions/disposables。 |

## Expected vs actual changed files

| 预期范围 | 实际结果 |
| --- | --- |
| HMR state transfer helper + tests | 已新增并暂存。 |
| bootstrap 接入 retain / reload / terminal | 已修改并暂存。 |
| shell unmount 只释放 workspace 级资源 | 已修改并暂存。 |
| Requirement / Verification 文档 | 已新增/更新并暂存。 |
| 后端 / proto / 非 Workbench 模块 | 未纳入。 |
| workspace watch / external file freshness 其他改动 | 未纳入本次暂存；但 `bootstrap.ts` 当前文件内容同时含 `subscribeWorkspaceChanges` 与 `mountAttempt` 等相邻生命周期逻辑，难以从 HMR 修复中物理拆分。 |

## Acceptance checklist

- [x] Workbench bootstrap state 可通过 `import.meta.hot.data` transfer（provider / initPromise / vscodeApiPromise / mounted 等）。
- [x] `initializeMonacoService()` 路径在同一页面至多成功进入一次；state 丢失时走 reload / terminal，不二次 init。
- [x] HMR dispose 只保存 state，不销毁 process-global services。
- [x] pending init promise 可被后续 mount 复用。
- [x] pre-Monaco 失败可重试；`monacoInitializationStarted` 或 `servicesInitialized` 后的失败 terminal。
- [x] host 缺失、workspace 变化、lost-state 触发 reload。
- [x] 增加有业务语义的 HMR helper 测试并通过。
- [ ] 开发服务器人工 smoke：打开 Code 页面后触发 HMR，确认无 `Services are already initialized`，Explorer/SCM 仍可用。

## Test results

| 命令 | 结果 |
| --- | --- |
| `npm test -- src/features/workbench/bootstrapHmrState.test.ts`（`web/`） | 通过：1 file / 7 tests |
| `npx vue-tsc --noEmit`（`web/`，实现阶段） | 通过 |
| 开发服务器人工 HMR smoke | 未执行（需人工） |
| 全量 web lint/format/build | 未作为本任务必跑项；实现阶段未发现 typecheck 失败 |

## Missed or expanded scope

- **扩展**：`bootstrap.ts` 同时包含 `mountAttempt`/`mountGeneration` 并发控制与 `subscribeWorkspaceChanges` 集成。它们服务同一 mount 生命周期，但严格说超出“仅 HMR retain”的最小描述；暂存整文件以保持可运行集成，并在此标明。
- **未纳入**：`platformFileSystemProvider*`、workspace watch 后端、其他 workbench API 改动，以及 logger 按日滚动等无关变更。

## Risks

- 人工 HMR smoke 未跑，真实 Vite invalidation 路径仍可能有边界情况。
- bootstrap self `hot.accept` full reload 会降低改 `bootstrap.ts` 时的 HMR 细粒度，但比二次 init 崩溃更稳。
- 若后续需要把 workspace subscription 单独提交，需从 `bootstrap.ts` 再拆 hunk 或二次整理。

## Incomplete items

- 开发服务器人工 smoke 仍待用户本地确认。
- Verification 文档在人工 smoke 完成后可将 Review status 升为 `Accepted`。

## Conclusion

自动化侧：HMR helper 与 typecheck 通过，代码路径已按 Requirement 把“库侧已 init / 本地 state 丢失”收敛到 reload 或 terminal，不再二次调用 `initialize()`。

交付边界：本次暂存聚焦 bootstrapHmrState 与其 bootstrap/shell 接入；`bootstrap.ts` 内相邻生命周期逻辑一并进入暂存是已知扩展范围。建议先完成本次提交，workspace freshness 等其它改动继续单独暂存/提交。
