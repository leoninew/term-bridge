# 本地首页快捷方式卡片验证

最后修改时间: 2026-07-11 16:23:36

Review status: Accepted

## Requirement alignment

按 `docs/requirement/20260711-local-home-shortcuts.md` 核对：

- 本地首页新增“快捷方式 / Shortcuts”卡片，标题右侧“更多 / More”使用既有 `local-shortcuts` 路由。
- 首页在前端分别截取前五个工作区和前四个快捷方式；列表 API 与后端契约保持全量、无分页。
- 快捷方式卡片宽屏四列、窄屏自动降列；首页每项名称和命令均水平居中，命令为常规 `text-sm` 文本。
- `/shortcuts` 管理页宽屏四列，命令正文不使用等宽字体。
- 快捷方式与工作区加载状态、错误状态和数据状态相互隔离。
- 中文与英文均提供“更多 / More”文案。

## Spec and plan alignment

轻量模式 / light 未创建 Spec 或 Plan 文档；按已接受的 Requirement 核对，不适用。

## Actual diff summary

预期功能相关改动：

- `web/src/components/dashboard/LocalHome.vue`
  - 新增快捷方式预览卡片、更多入口、独立加载状态和错误状态。
  - 对工作区和快捷方式结果分别执行 `slice(0, 5)` 与 `slice(0, 4)`。
- `web/src/components/shortcut/ShortcutCard.vue`
  - 统一紧凑标题与命令文本的排版，移除命令等宽字体。
- `web/src/components/shortcut/ShortcutsPageShell.vue`
  - 调整为响应式四列网格。
- `web/src/i18n.ts`
  - 新增“更多 / More”翻译。
- `docs/requirement/20260711-local-home-shortcuts.md`
  - 记录已接受的轻量需求及最终前端限量决策。

验证开始时工作树仅包含上述功能改动与需求文档；未发现与该功能无关的未暂存改动。`git diff --check` 通过。

## Acceptance checklist

- [x] 本地首页出现“快捷方式 / Shortcuts”卡片。
- [x] “更多 / More”使用 `local-shortcuts` 路由。
- [x] 工作区最多显示五项，快捷方式最多显示四项，且 API 契约未增加分页。
- [x] 快捷方式预览在宽屏四列、窄屏降列，网格内容水平居中。
- [x] 首页命令预览使用常规 `text-sm`，不使用等宽字体。
- [x] `/shortcuts` 使用响应式四列网格，命令正文不使用等宽字体。
- [x] 快捷方式、工作区分别处理加载、空、失败与有数据状态。
- [x] 快捷方式和工作区在身份初始化后并发独立加载。
- [x] 中英文“更多 / More”文案存在。

## Test results

| 检查 | 结果 |
| --- | --- |
| `go test ./cmd/... ./internal/...` | 通过 |
| `yarn --cwd web prettier --check src/components/dashboard/LocalHome.vue src/components/shortcut/ShortcutCard.vue src/components/shortcut/ShortcutsPageShell.vue src/i18n.ts` | 通过 |
| `yarn --cwd web typecheck` | 通过 |
| `yarn --cwd web lint` | 通过 |
| `yarn --cwd web test` | 通过，12 个文件、62 项测试 |
| `yarn --cwd web build:local` | 通过 |
| `git diff --check` | 通过 |

`build:local` 输出了来自 `@vueuse/core` 的既有 Rolldown `INVALID_ANNOTATION` 警告；构建成功完成，未阻断产物生成。

## Scope deviations

无功能范围扩展。验证过程中对两个预先存在格式不一致的快捷方式组件执行了 Prettier 格式化，未改变行为。

## Risks and incomplete items

- 未配置针对 `LocalHome.vue` 的组件级自动化测试；已通过类型检查、lint、现有前端测试、构建和人工 diff 核对覆盖。
- 未进行浏览器手工交互验收；建议在本地拥有超过五个工作区、四个快捷方式的数据时确认首页数量与“更多”跳转。

## Conclusion

验证通过。实现与已接受的轻量需求一致，可进入交付审阅。
