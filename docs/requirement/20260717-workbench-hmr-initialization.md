# Workbench Vite HMR 全局服务初始化稳定性

最后修改时间: 2026-07-17 17:54:30

Review status: Accepted

## Flow mode / Stage

轻量模式 / light；需求 / Requirement（Accepted）。

## Background

在开发服务器中打开 `/workspaces/:workspaceId/code` 后，Vite HMR 更新 Workbench 相关模块可能重新求值 `web/src/features/workbench/bootstrap.ts`。该模块内的初始化状态原先仅存于模块作用域，而 `@codingame/monaco-vscode-api` 的服务容器在同一浏览器页面内保持全局存活。

HMR 替换会重置模块局部的 `initPromise`、filesystem provider、VS Code API promise 和挂载状态；新的模块实例再次调用 `initializeMonacoService()` / `registerCustomProvider()` 时，已初始化的服务容器抛出：

```text
Services are already initialized
```

仅靠 `import.meta.hot.data` 保留 state 不够：当 dispose/data 丢失、state 结构校验失败，或初始化失败边界与库侧 `servicesInitialized` 不一致时，仍会二次 init 并抛错。

## Goal

1. 使 Workbench bootstrap 在 Vite HMR 前后尽量保持同一份运行时状态，复用已初始化的 Monaco/VS Code services、filesystem provider、extension API 与 SCM controller。
2. 当本地 HMR state 与库侧全局服务状态分叉时，强制完整页面 reload，而不是再次调用 `initialize()`。
3. 将初始化失败边界与 `@codingame/monaco-vscode-api` 的 `servicesInitialized` 对齐，避免非法重试。

## Non-goal

- 不修改 `@codingame/monaco-vscode-api`、Vite 或浏览器的第三方源码。
- 不增加第二套 Workbench、provider、extension host 或 SCM controller。
- 不将 route 卸载改为销毁 process-global Workbench services。
- 不修改既有 SCM Git action、菜单或其已确认的刷新语义。
- 不尝试让 service override、extension manifest 等本身不可安全热重建的全局配置在不刷新页面的情况下重新生效。
- 不实现可 teardown/recreate 的 Monaco service lifecycle。

## User scenarios

1. 开发者打开一个 workspace 的 Code 页面，Workbench 正常完成首次初始化。
2. Vite 因依赖更新触发 HMR；若 state 成功 transfer，页面不出现 `Services are already initialized`，现有 Workbench 继续可用。
3. HMR 在 Workbench 初始化尚未完成时发生；替换后的模块等待同一个 pending initialization，而不发起第二次全局服务初始化。
4. HMR 后同一 workspace 的 shell 重新挂载时，filesystem provider 重新绑定当前 runtime API，SCM 能继续刷新，不产生重复 command/provider 注册。
5. workspace identity 改变、Workbench host 已不存在、或本地 state 丢失但库侧 services 已初始化时，完整页面 reload，而不是尝试二次 init 或迁移全局 Workbench DOM。
6. 修改 `bootstrap.ts` 自身时，开发态直接 full reload，避免 partial HMR 与全局服务冲突。

## Acceptance

- [x] `bootstrap.ts` 的 Workbench 生命周期状态可跨 Vite HMR transfer，至少包括 provider 实例、provider 注册状态、初始化 promise、VS Code API promise 与 mounted Workbench/SCM 状态。
- [x] HMR 触发前后，`initializeMonacoService()` 在同一浏览器页面中至多执行一次；本地 state 丢失时通过 reload 恢复，不得再以二次 init 方式触发 `Services are already initialized`。
- [x] HMR dispose 仅保存 state，不销毁 Workbench、SCM、extension host、filesystem provider 或 route 外仍需保留的 process-global 服务。
- [x] 初始化中的 pending promise 可跨 HMR 继承；新模块必须等待该 promise。
- [x] 未触及 Monaco 全局初始化前的失败可在后续 mount 重试；已经开始全局初始化后的失败、或库侧 `servicesInitialized === true` 时的失败不得页内重试。
- [x] 同 workspace remount 仍只重新绑定 runtime API 并刷新 SCM；workspace 变化、host 缺失、state 丢失的页面 reload 行为明确。
- [x] 增加 HMR state transfer、reload 判定、失败边界测试。
- [ ] 开发服务器人工 smoke test 验证：对已打开的 Code 页面触发 HMR 后，无服务重复初始化或重复注册错误，Explorer 与 SCM refresh 仍工作。

## Open questions

不适用。

## Decisions

1. 使用 Vite 原生 `import.meta.hot.data` 传递命名空间化的 Workbench bootstrap state；不使用 `window` 或无边界的 `globalThis` 状态。
2. state 必须保留 provider 对象 identity，而非只保留已注册布尔值，因为全局文件服务仍绑定首次注册的 provider 实例。
3. HMR dispose 不调用 `disposeMountedWorkbench()`；路由卸载只释放当前 workspace 的 subscriptions/disposables，继续保留 process-global 服务。
4. `bootstrap.ts` 对自身模块使用 `import.meta.hot.accept` 并 full reload；依赖模块更新仍尽量走 state transfer。
5. 库侧 `servicesInitialized` 是真相源：本地 `initPromise` 丢失且 services 已 init 时强制 reload / terminal reject，禁止再次 `initialize()` / `registerCustomProvider()`。
6. 以 `stateVersion` 与 provider 实例类型校验 HMR retained state；校验失败视为 state 丢失。
7. 初始化逻辑抽到 `bootstrapHmrState.ts`，便于单测 retain / retry / terminal / reload 判定。
8. 发生全局初始化后的失败时，记录不可页内恢复状态；禁止无限重试。

## Risk

- HMR 无法安全重建既有 service override、extension manifest 和已注册 extension 的全局配置；修改这些配置后开发者仍可能需要完整浏览器刷新。
- 若 HMR 替换时 Vue 同时移除了 Workbench host，无法安全搬迁全局 Workbench DOM；继续使用 reload fallback。
- 若只持久化 promise/flag 而遗漏 provider 或 mounted 状态，会产生旧服务绑定旧 provider、重复 SCM subscriptions 或 command registration 失败，因此 state 必须整体 transfer。
- `bootstrap.ts` 当前工作区还包含 workspace change subscription、mountAttempt 并发控制等相邻生命周期改动，与 HMR 修复耦合在同一文件；暂存时需在 verification 中标明范围。
- 人工开发服务器 smoke 仍需人工确认。

## Related documents

- Workbench bootstrap：`web/src/features/workbench/bootstrap.ts`
- HMR helper：`web/src/features/workbench/bootstrapHmrState.ts`
- Workbench shell：`web/src/components/workbench/CodeWorkbenchShell.vue`
- 验证：[`docs/verification/20260717-workbench-hmr-initialization.md`](../verification/20260717-workbench-hmr-initialization.md)

## User review notes

- 2026-07-17：用户报告 `/workspaces/:workspaceId/code` 在开发服务器热加载后可能出现 `Services are already initialized`，并要求修正。
- 2026-07-17：用户要求以轻量模式 / light 记录该问题，同时继续实施修正。
- 2026-07-17：分析确认根因是库侧全局服务与本地 HMR state 双源真相；用户采纳 A/C/D 建议（`servicesInitialized` 兜底 reload、terminal 边界对齐、强化 retain）。
- 2026-07-17：用户要求暂存 bootstrapHmrState 相关变更，并用 SpecFlow 记录本任务。
