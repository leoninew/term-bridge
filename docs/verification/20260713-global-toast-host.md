# 全局 Toast 挂载验证
最后修改时间: 2026-07-13 21:45:16

Review status: Accepted

## Requirement alignment

按 `docs/requirement/20260713-global-toast-host.md` 核对：

- `App.vue` 在 `RouterView` 外挂载唯一 `ToastProvider`，并在同一 provider 内挂载唯一 `ToastHost`。
- 原本在首页、会话 shell、快捷方式 shell、登录、Dashboard 与 OAuth callback 页面中的局部 ToastProvider / ToastHost 已移除。
- 复用既有 Pinia 通知队列、ToastHost 的自动关闭与手动关闭逻辑以及全局样式，未修改通知 API、文案或数据模型。
- 注册页无需新增自身宿主，即可由根部 ToastHost 渲染已有 `notifyError()`。

## Spec alignment

不适用：light / 轻量模式未创建独立 Spec。

## Plan alignment

按 Requirement 核对：已将唯一 provider 与 host 提升到根组件；局部挂载已清理，同时保留 Dashboard 的 Dialog 与会话 shell 的 Splitter 等无关 Reka UI 组件。

## Actual diff summary

- `web/src/App.vue`
  - 新增根部 `ToastProvider` 与唯一 `ToastHost`，使宿主不随路由切换卸载。
- `web/src/components/session/SessionsPageShell.vue`
- `web/src/components/shortcut/ShortcutsPageShell.vue`
- `web/src/views/HomeView.vue`
- `web/src/views/cloud/LoginView.vue`
- `web/src/views/cloud/DashboardView.vue`
- `web/src/views/cloud/GoogleCallbackView.vue`
- `web/src/views/cloud/OAuthAuthorizeView.vue`
- `web/src/views/local/OAuthCallbackView.vue`
  - 移除重复局部 ToastProvider、ToastHost 与无用 import，不改变各页面通知调用。
- `docs/requirement/20260713-global-toast-host.md`
  - 新增 light Requirement 记录。

## Expected versus actual changed files

| Expected file | Actual status | Notes |
|---|---|---|
| `web/src/App.vue` | Changed | 唯一全局 Toast 根节点。 |
| 原本局部挂载 Toast 的页面与 shell | Changed | 重复 provider/host 已清理。 |
| `web/src/components/session/ToastHost.vue` | Unchanged | 复用已有渲染、关闭和 i18n 逻辑。 |
| `web/src/store/notifications.ts` | Unchanged | 复用既有全局 Pinia 队列。 |
| `web/src/views/cloud/RegisterView.vue` | Unchanged | 既有通知调用由根部 host 覆盖。 |

## Acceptance checklist

- [x] 应用根部只有一个 ToastProvider 与 ToastHost。
- [x] 所有既有局部 ToastProvider / ToastHost 引用均已清理。
- [x] 现有通知 API、自动关闭、关闭控件、样式和 i18n 未改变。
- [x] Typecheck、lint 和完整 web Vitest 套件通过。
- [x] 浏览器首页因工作区 API 404 入队错误通知后，页面显示一个 error toast。
- [x] 浏览器跳转至注册页并触发 Turnstile 请求网络错误后，注册页显示一个 error toast，确认此前无局部宿主的路径由根部 host 渲染。
- [ ] 真实可用 runtime API 的 OAuth callback 失败后跨 `router.replace()` 持续显示通知，当前环境没有可完成的 OAuth runtime。

## Test results

| Command | Result |
|---|---|
| `yarn --cwd web typecheck` | Passed. |
| `yarn --cwd web lint` | Passed. |
| `yarn --cwd web test` | Passed: 14 files, 84 tests. |
| `git diff --check` | Passed: no whitespace errors reported. |
| `pomelo-pw run C:/Users/wangm25/.claude/plans/verify-global-toast.yaml` | Passed: home and registration browser flows captured. |

## Runtime evidence

- 首页截图显示工作区 API `404` 触发的 “加载工作区失败 / API error contract mismatch (404)” error toast：`C:\Users\wangm25\.claude\plans\verify-global-toast-output\home-error-toast.png`。
- 注册页截图显示 Turnstile key 请求失败触发的 “创建账号 / Network Error” error toast：`C:\Users\wangm25\.claude\plans\verify-global-toast-output\register-error-toast.png`。
- 两个页面均由相同根部宿主渲染；源代码搜索确认 `ToastProvider` 与 `ToastHost` 只出现在 `App.vue`。

## Scope deviations

无。现有工作树中与会话 tab、侧边栏拖拽等相关的并行修改不属于本任务，未改动或纳入本验证结论。

## Risks

- 当前浏览器环境的 runtime API 返回 404 / 网络错误，无法对真实 OAuth 回调链路执行端到端验证。
- 当前项目没有 Vue DOM component mount 测试基础设施；全局唯一 viewport 的证明由源代码搜索和浏览器页面观察提供。

## Incomplete items

- 在具备真实 OAuth runtime 的环境中，仍需观察 callback 失败后执行 `router.replace()` 时 Toast 在目标路由持续显示。

## Reverification

2026-07-13 21:45 已重新执行 `typecheck`、`lint`、完整 Vitest、Toast 引用搜索与 Pomelo 浏览器 flow；结果保持一致：14 个测试文件、84 个测试通过，ToastProvider / ToastHost 只存在于 `App.vue`，首页和注册页均再次捕获到 error toast。

## Conclusion

全局 Toast 挂载已完成。静态检查和完整 web 测试通过；浏览器已实际观察到首页与此前无局部宿主的注册页均能显示 error toast。OAuth callback 的跨跳转运行时验证受当前 API 环境限制，保留为后续验收项。
