# 快捷方式自定义图标、状态与标签验证
最后修改时间: 2026-07-14 22:45:05

Review status: Draft

## Requirement alignment

依据 `docs/requirement/20260714-shortcut-customization.md` 核对。

- `icon`、`enabled`、`tags` 与 response-only 的 `last_used_at` 已贯通 protobuf、领域模型、SQLite/MySQL 迁移、仓储和快捷方式 API；`enabled` 使用 `optional bool`，缺失时按启用处理；`last_used_at` 以 NULL/零值表示未使用且不接受客户端写入。
- 快捷方式创建、编辑、卡片禁用状态、标签展示、管理页标签归纳与过滤均已实现；过滤时改用非拖拽列表，未向完整排序 API 提交子集。
- 创建和编辑会话前端仅传入启用快捷方式；后端在快捷方式来源的创建、编辑路径中再次读取并拒绝不存在或禁用项。
- 发现两个未满足验收项，见“未完成项”。因此本验证未确认交付。

## Spec alignment

不适用。当前为 light / 轻量模式，按 Requirement / 需求核对。

## Plan alignment

不适用。当前为 light / 轻量模式，按 Requirement / 需求核对。

## Actual diff summary

本次工作区改动包括：

- 新增 SQLite/MySQL `shortcuts` 元数据迁移，增加 `icon`、`enabled`、`tags_json` 与可空 `last_used_at` 列；最后使用时间不在本期写入。
- protobuf 及生成的 Go/TypeScript 类型增加 `icon`、具 presence 的 `enabled`、`tags` 与只读 `last_used_at` 响应字段。
- 快捷方式领域模型和仓储完成可选文本、默认启用、标签去空白及精确去重，以及 JSON 持久化。
- 终端会话注册表增加快捷方式读取依赖，在创建和编辑会话时校验快捷方式来源仍存在且启用；Agent bootstrap 注入同一 `DbStore`。
- 管理页增加标签 store、标签过滤工具条、标签卡片样式、启用开关及禁用展示。
- 会话创建/编辑界面向下游只传递 `enabled !== false` 的快捷方式。

`docs/design/ee45340b-6b72-4267-83cd-d5e628103f5f.png` 同时处于新增状态，但不属于本需求验收范围。

## Expected vs actual changed files

| 预期范围 | 实际情况 |
| --- | --- |
| `proto/` 与生成代码 | 已修改 `shortcut.proto`、Go 与 TypeScript 生成文件 |
| `migrations/agent/{sqlite,mysql}/` | 已新增同编号元数据迁移 |
| 快捷方式模型、服务、仓储与相关测试 | 已修改并新增关键覆盖 |
| 终端会话校验与 bootstrap 注入 | 已修改 `registry.go`、`server.go` 及测试 |
| 快捷方式管理和会话选择前端 | 已修改对应 Vue 组件、样式、i18n，新增标签 store 与样式模块 |
| Requirement 文档 | 已新增并保持 `Accepted` |
| Verification 文档 | 本文档新增 |
| 设计图片 | 工作区存在无关新增文件，未作为本需求内容验证 |

## Acceptance criteria checklist

- [x] protobuf、API、领域模型、SQLite/MySQL 迁移和仓储读写均传递新增字段，且为向后兼容字段；`last_used_at` 只随响应返回，NULL/零值代表未使用，普通快捷方式编辑不会改写它。
- [x] 数据库默认值与服务创建逻辑均将既有/新增快捷方式视为启用。
- [x] `icon` 与标签空白值按可选元数据处理；标签去除首尾空白、空值和精确重复值。
- [ ] `icon` 仅为持久化占位而不进入编辑交互：编辑器提交时固定发送 `icon: ''`，会覆写已有图标占位值，见“未完成项”。
- [x] 编辑器可维护启用状态和标签，卡片显示标签与禁用状态。
- [x] 管理页支持标签过滤，过滤状态下不启用拖拽排序。
- [x] 新建和编辑会话只接收启用快捷方式，禁用项不展示且不作为默认选择。
- [ ] 会话快捷方式选择器搜索只匹配名称与命令，未匹配标签，见“未完成项”。
- [x] 后端创建和编辑会话拒绝禁用或不存在的快捷方式来源；启用项可用。
- [x] 本地 Agent bootstrap 将同一 `DbStore` 同时注入快捷方式服务和会话注册表；新增字段通过 protobuf 传输，Agent tunnel 与 Cloud relay 的固定 `last_used_at` 响应值保持完整。
- [x] 已运行关键回归测试和完整项目测试。

## Test results

| 命令 | 结果 |
| --- | --- |
| `go test ./internal/agent/model/task/shortcut ./internal/agent/application/task/shortcut ./internal/agent/repository/task/state ./internal/agent/infrastructure/database ./internal/agent/application/user ./internal/cloud/api/handler` | 通过；覆盖 `last_used_at` 的 NULL、保留、protobuf 映射与本地/云端中继透传 |
| `yarn --cwd web test --run src/store/shortcutTags.test.ts src/composable/useCreateSessionDraft.test.ts` | 通过，2 个文件、8 个测试 |
| `yarn --cwd web typecheck` | 通过 |
| `yarn --cwd web lint` | 通过 |
| `task test` | 通过；Web 15 个文件、82 个测试，后端 `./cmd/... ./internal/...` 全部通过 |
| `task build && yarn --cwd web build` | 通过；Vite 仅报告既有的单 chunk 超过 500 kB 警告 |
| changed Go files 的 `gofmt -d` 检查 | 通过 |
| `git diff --check` | 失败：`ShortcutCard.vue:165` 与 `ShortcutsPageShell.vue:365` 存在行尾空白 |
| 针对本次快捷方式前端文件的 Prettier 检查 | 失败：`ShortcutCard.vue`、`ShortcutsPageShell.vue`、`tagStyle.ts`、`styles.css` 未通过格式检查 |
| 全量 `yarn --cwd web format` | 失败：除上述文件外，另有 6 个工作区前端文件未通过格式检查；其中是否为本次引入尚未拆分 |

## Scope deviations

- 未发现功能范围扩展。
- 当前工作区含不在本需求范围内的新增设计图片；该文件未影响编译、测试或本次功能核对。
- 构建产生的 `bin/termbridge` 与 `web/dist` 未出现在 Git 状态中。

## Risks

- 选择器标签搜索缺失会导致标签“可展示并参与搜索”的用户场景无法实现，标签较多时难以检索。
- 编辑器每次保存都提交空 `icon` 值，会清除未来客户端或迁移已保存的图标占位数据，破坏该字段的持久化兼容性。
- 当前 diff 存在行尾空白和 Prettier 失败，未满足项目格式质量门槛；本次 `last_used_at` 的 Go、proto、迁移与测试文件均通过 `gofmt`，格式失败来自既有前端工作区改动。

## Incomplete items

1. `web/src/components/session/SessionCommandInput.vue:183-185` 的 `shortcutSearchText` 仅拼接 `shortcut.name` 和 `shortcut.command`；应在不改变禁用过滤前提下纳入 `shortcut.tags`。
2. `web/src/components/shortcut/ShortcutEditorDialog.vue:151-159` 在没有图标编辑需求时仍固定提交 `icon: ''`；应避免将 `icon` 写入编辑器提交载荷，或保留已有值，避免修改其他客户端持久化的占位数据。
3. 修复并复查本次改动引入的格式问题，至少包括 `web/src/components/shortcut/ShortcutCard.vue`、`web/src/components/shortcut/ShortcutsPageShell.vue`、`web/src/components/shortcut/tagStyle.ts` 和 `web/src/styles.css`。

## Conclusion

功能主路径的后端持久化、状态约束、`last_used_at` 的 response-only 透传、前端过滤和完整测试/构建均通过；`last_used_at` 未在本期采集、写入、展示、排序或筛选。仍有两个需求偏差及格式检查失败，因此验证结论维持未确认交付；完成“未完成项”后应重新运行相关前端测试、格式检查、`task test` 与构建。
