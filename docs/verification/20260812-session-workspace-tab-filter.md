# 会话目录 Tab 联动验证
最后修改时间: 2026-08-12 17:14:45

Review status: Accepted

## Requirement Alignment

已按 `docs/requirement/20260812-session-workspace-tab-filter.md` 完成核对：点选左侧会话后，右侧 tab 按 workspace 筛选；tab 标题只显示会话名；关闭、批量关闭、排序、运行会话打开和后台会话关闭均限定在当前 workspace。

## Spec Alignment

不适用。此功能使用 SpecFlow light 流程，未创建独立 spec。

## Plan Alignment

不适用。此功能使用 SpecFlow light 流程，按 requirement 实现。

## Actual Diff Summary

- `SessionsPageShell` 维护当前 workspace 筛选状态，并将 tab 与批量操作投影到该 workspace。
- `SessionWorkbench` 分离可见 tab 与全量已打开 tab，使目录筛选不影响运行终端 keep-alive；当前目录无 tab 时显示空状态覆盖层。
- `useWorkbenchStore.closeTabs` 支持以可见 tab 集合计算关闭后的相邻活动 tab。
- tab 标题改为会话名，移除目录名称。
- 添加关闭过滤 tab 时的回退行为测试。

## Expected And Actual Files

| 预期文件 | 实际结果 |
| --- | --- |
| `web/src/components/session/SessionsPageShell.vue` | 已修改 |
| `web/src/components/session/SessionWorkbench.vue` | 已修改 |
| `web/src/store/workbench.ts` | 已修改 |
| `web/src/store/workbench.test.ts` | 已修改 |
| `web/src/store/workspaceSessions.ts` | 已修改 |
| `docs/requirement/20260812-session-workspace-tab-filter.md` | 已修改 |
| `docs/verification/20260812-session-workspace-tab-filter.md` | 已创建 |

## Acceptance Checklist

- [x] 左侧会话节点选择同步筛选右侧 tab。
- [x] tab、菜单和批量操作只处理当前 workspace。
- [x] 关闭活动 tab 时不切换到其他 workspace。
- [x] 其他 workspace 的运行终端继续参与 keep-alive。
- [x] tab 标题不重复展示目录名。
- [x] 删除当前 workspace 时清空筛选状态。

## Test Results

- `task check`：通过。包含 Vue 类型检查、ESLint、Prettier、Go 格式化与 golangci-lint。
- `yarn test -- --run src/store/workbench.test.ts src/store/workspaceSessions.test.ts src/features/sessions/tabManagement.test.ts`：通过，3 个文件、26 个用例。

## Scope

实现范围与需求一致，无额外产品代码范围扩展。

## Risks And Incomplete Items

- 未进行真实浏览器多 workspace 切换的手工 UI 验收；关键状态转换已由单元测试和静态检查覆盖。
- 用户环境的 Chocolatey Yarn 包登记仍需在管理员终端中清理，不属于仓库提交内容，也不影响当前 Yarn 调用路径。

## Conclusion

验证通过，可交付。
