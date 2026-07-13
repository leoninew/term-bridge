# 全局 Toast 挂载
最后修改时间: 2026-07-13 21:04:12

Review status: Accepted

## Background

当前通知状态已由 Pinia 的 `useNotificationsStore()` 全局保存，但 `ToastProvider` 和 `ToastHost` 分散在页面与页面 shell 内。路由切换会卸载当前 Toast 宿主，可能使仍在展示期的通知中断；同时云端注册页会调用 `notifyError()`，却没有自身的 Toast 宿主，导致通知无法渲染。

## Goal

在应用根部提供唯一、持续存在的 ToastProvider 与 ToastHost，使所有路由共享同一个通知渲染入口，并保持通知跨路由切换可见。

## Non-goal

- 不修改 `useNotificationsStore()` 的通知数据结构、去重策略、时长或调用 API。
- 不调整 Toast 的文案、i18n 内容、视觉样式或位置。
- 不为本次挂载迁移新增 Vue 组件测试基础设施。

## User scenarios

1. 用户在会话、快捷方式、登录或云端页面触发错误通知时，应用仅显示一组 Toast。
2. Toast 显示期间发生路由跳转时，通知继续显示直到自动关闭或用户手动关闭。
3. 云端注册页发生已有的 Turnstile key 加载错误时，用户能看到通知。
4. OAuth 回调失败后跳转到目标路由时，错误通知仍可见。

## Acceptance

- `App.vue` 在 RouterView 之上挂载唯一 `ToastProvider`，并在同一 provider 内挂载唯一 `ToastHost`。
- 原有页面和页面 shell 不再各自挂载 `ToastProvider` 或 `ToastHost`。
- 全应用只存在一个 Toast viewport，现有通知创建、自动关闭、手动关闭和样式保持可用。
- 原本无本地宿主的注册页通知能通过根部宿主显示。
- Typecheck、lint 与现有 web 测试通过；浏览器运行时验证记录实际可达的路由和环境限制。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- 使用 `App.vue` 作为全局宿主边界；它在路由切换期间持续存在。
- 复用现有 `web/src/store/notifications.ts` 和 `web/src/components/session/ToastHost.vue`，不创建新的全局 notification API。
- 页面级 Toast wrapper 全部移除，避免多 viewport、重复提示与重复 live-region 宣告。

## Risk

- 移除大型页面 shell 的外层 provider 标签时必须保持模板结构与无关 Reka UI imports 完整。
- OAuth 回调的错误提示依赖根部宿主在 `router.replace()` 前后持续挂载，需要浏览器环境验证。

## User review notes

- 用户于 2026-07-13 要求“把 toast 改为全局挂载”。
- 用户于 2026-07-13 要求“继续实现, 同时 /specflow light 记录本任务”，本 Requirement 已补录并接受。
