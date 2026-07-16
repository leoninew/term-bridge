# 前端页面加载与异常处理统一

最后修改时间: 2026-07-16 21:20:00

Review status: Accepted

## Flow mode / Stage

标准模式 / standard；基于 `docs/requirement/20260716-frontend-page-loading-error-handling.md`。

## Requirement basis / 需求依据

Requirement 已记录并通过用户“记录和修正问题”授权进入实现。

## Spec basis / 规格依据

不适用（standard 模式不强制独立 Spec）。

## Implementation steps / 实现步骤

1. 新增共享异步 action composable：封装 `running/error/run`，可选 toast 失败通知。
2. 新增轻量页面状态组件：统一展示 loading / error / empty / content。
3. 修正 `WorkspaceFilesPageShell` 加载空白，改为明确 loading 态。
4. 选择 1 个表单页接入共享 action（优先 `ForgotPasswordView` 或同结构 auth 表单）。
5. 选择 1 个列表页对齐共享状态展示（优先 `ShortcutsPageShell`）。
6. 补充必要单测，并运行相关 web typecheck/lint/test。

## Files to change / 需修改文件

- `web/src/composable/useAsyncAction.ts`（新增）
- `web/src/composable/useAsyncAction.test.ts`（新增）
- `web/src/components/layout/PageStatus.vue`（新增）
- `web/src/components/workspace/WorkspaceFilesPageShell.vue`
- `web/src/views/cloud/ForgotPasswordView.vue` 或同级 auth 表单页
- `web/src/components/shortcut/ShortcutsPageShell.vue`
- `docs/requirement/20260716-frontend-page-loading-error-handling.md`
- `docs/plan/20260716-frontend-page-loading-error-handling.md`

## Verification plan / 验证计划

1. 单测覆盖 `useAsyncAction` 成功、失败、重复执行保护。
2. 前端 typecheck / lint / 相关 vitest。
3. 人工核对：
   - WorkspaceFiles 加载中显示文案
   - Shortcuts 列表 loading/empty/error 一致
   - 表单提交失败仍 toast，且按钮 disabled

## Risks / blockers / assumptions / 风险 / 阻塞项 / 假设

- Sessions 工作台暂不纳入整页 loading 改造，避免扩大会话交互风险。
- 假设现有 toast 全局宿主已足够，不需要再改 `App.vue`。
- 若列表页已有完善局部状态，共享组件只做展示对齐，不强制迁移数据源。

## Rollback / recovery / 回滚 / 恢复

- 可单独回退新增 composable/组件，并恢复被改页面的本地 loading/error 变量。
- 不影响后端 API 与通知 store 数据结构。

## User review notes / 用户审查记录

- 用户要求 SpecFlow 记录并修正统一加载/异常处理问题。

## Implementation notes / 实现备注

- 已扩展接入 auth 表单：Login/Register/Forgot/Reset/ChangePassword/VerifyEmail。
- 已扩展接入列表页：Dashboard 设备列表、LocalHome 工作区/快捷方式。
- Sessions 工作台整页 loading 仍不在本轮范围。

- 后续扩展：Sessions/Files 左右 panel 分别接入 PageStatus。
