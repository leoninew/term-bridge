# 前端页面加载与异常处理统一

最后修改时间: 2026-07-16 21:20:00

Review status: Accepted

## Flow mode / Stage

标准模式 / standard；需求 / Requirement 已接受。用户要求使用 SpecFlow 记录并修正“各页面加载与异常处理不统一”的问题，本轮直接进入实现 / Implementation。

## Background

当前前端通知层已基本统一：

- `App.vue` 全局挂载 `ToastProvider` / `ToastHost`
- 异步失败普遍通过 `useNotificationsStore().notifyError()` / `pushToast()` 反馈

但页面加载、页面错误、空态与表单提交状态仍由各页面/壳层各自实现，存在这些不一致：

1. 状态命名分散：`loading`、`submitting`、`checkingAuth`、`creatingSession`、`ready`、`workspacesLoading` 等。
2. 错误展示策略不统一：有的仅 toast，有的行内 error + toast，有的失败后 redirect。
3. 加载 UI 不统一：Dashboard / Shortcuts 有文案；`WorkspaceFilesPageShell` 加载中空白；Sessions 几乎不展示整页 loading。
4. 缺少共享 composable / 页面状态组件，表单页重复 `submitting + try/catch/finally + notifyError` 模板。
5. `AppPageShell` 只负责布局，不承接 loading / error / empty 状态槽。

这会让用户在不同路由感知到不同的等待与失败反馈，也增加后续页面继续复制局部状态逻辑的成本。

## Goal

1. 建立前端统一的异步状态约定：页面查询态与表单提交态分层处理。
2. 提供可复用的异步 action 工具与页面状态展示入口，减少各页重复实现。
3. 修正当前最明显的不一致缺口，使关键页面至少具备：
   - 可感知 loading
   - 可感知 error 或 toast
   - 有数据列表时具备 empty 态
4. 保持现有全局 toast 作为统一通知通道，不另建第二套通知系统。

## Non-goal

1. 不重写 Sessions / File workbench / Git 的业务状态机。
2. 不引入 React Query 风格全局缓存层，也不替换 Pinia store 的数据源职责。
3. 不统一所有按钮文案、i18n key 或视觉皮肤。
4. 不把终端 WebSocket 连接态改造成通用 PageState。
5. 不在本轮把所有历史页面一次性重构完；优先补共享能力 + 代表性缺口修正。

## User scenarios

1. 用户打开有列表数据的页面（Dashboard 设备、Shortcuts、LocalHome 工作区/快捷方式）时，看到明确 loading 文案，失败时看到错误反馈，空列表时看到 empty 文案。
2. 用户提交登录、改密、重置密码等表单时，按钮进入提交中状态，成功/失败通过 toast 反馈，重复点击被防抖。
3. 用户打开文件工作台时，在 workspace 尚未确认前看到加载态，而不是整页空白；加载失败或 workspace 不存在时看到可操作错误/缺失态。
4. 开发者新增普通查询页时，可复用共享 composable / 状态组件，而不是再复制一套 loading/error 局部变量。

## Acceptance

- [x] 新增共享异步状态能力（至少包含 action 执行封装与页面状态展示入口）。
- [x] 约定：
  - 表单/命令型操作：`submitting` + toast（必要时局部 disabled）
  - 页面/列表查询：`loading` + 行内 error/empty + toast（失败可同时 toast）
- [x] `WorkspaceFilesPageShell` 加载中不再空白，至少展示 loading 文案。
- [x] 列表页已对齐：Shortcuts、Dashboard 设备列表、LocalHome 工作区/快捷方式
- [x] 表单页已接入 useAsyncAction：Login/Register/Forgot/Reset/ChangePassword/VerifyEmail
- [x] 现有全局 toast 通道保持可用；不新增第二套通知宿主。
- [x] 相关前端 typecheck / lint / 关键测试通过。

## Open questions

暂无阻塞实现的未决事项。Sessions 整页 loading 是否补齐，本轮先不强行改造，避免牵动会话工作台交互。

## Decisions

1. 采用标准模式 / standard：记录 requirement + plan，不强制单独 spec。
2. 复用 `useNotificationsStore` 与 `errorMessage`，共享层只补状态与 UI 组织，不改 toast 数据模型。
3. 共享能力优先放在 `web/src/composable` 与轻量展示组件，而不是塞进 `AppPageShell` 强制所有页面改壳。
4. 本轮修正优先级：
   1. 共享 composable / 状态组件
   2. `WorkspaceFilesPageShell` 空白加载
   3. 表单页样板接入
   4. 列表页对齐示范
5. Sessions 操作级状态（creating/deleting/stop/rerun）继续保留，不强制改成通用 page query 状态。

## Risk

- 过度抽象会让简单页面更难读；共享 API 必须保持小而明确。
- 部分页面错误目前只 toast，改为行内 error 时要避免重复打扰。
- `WorkspaceFilesPageShell` 增加 loading 文案后，要保持现有 missing workspace 返回会话能力。
- 表单页接入共享 action 时需保留现有校验 toast 文案与 redirect 行为。

## User review notes

- 用户先要求检查各页面是否有统一加载和异常处理。
- 用户随后要求使用 SpecFlow 记录并修正该问题。

## Implementation notes / 实现备注

- Sessions / Files 的加载态按左右 panel 分离：
  - 左侧树：工作区/会话树、目录树 loading/error/empty
  - 右侧内容：会话内容 bootstrap、文件编辑器 loading/error/empty
- 不把整页单一 loading 覆盖左右两个 panel。
