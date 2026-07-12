# 命令快捷方式验证

最后修改时间: 2026-07-12 20:59:11

Flow mode: standard / 标准模式
Stage: Verification / 验证
Review status: Accepted

## Requirement alignment

按 `docs/requirement/20260710-command-shortcuts.md` 核对：

1. 快捷方式仍是 Agent 数据库中的设备本地数据；Cloud 仅经既有设备 tunnel 访问，不新增 Cloud shortcut 表或跨设备同步。
2. 快捷方式仍以原始命令文本保存和返回，CRUD、会话创建与停止会话编辑语义未改变。
3. 卡片页面现支持按设备持久化的拖拽排序：直接拖动卡片主体，不添加拖拽手柄；编辑、删除按钮维持独立操作。
4. 新建快捷方式追加到末尾，编辑内容不移动卡片；刷新或从 Cloud 访问同一设备时，服务端顺序保持一致。
5. 排序请求必须是当前设备完整、无重复的 ID 排列；缺失、重复、未知或跨设备 ID 均拒绝且不写入。

## Spec alignment

不适用。standard / 标准模式未创建独立 Spec / 规格文档，按已接受 Requirement 与 Plan 核对。

## Plan alignment

### 已落实

- SQLite/MySQL 新增 `202607120001_shortcut_ordering.sql`，引入 `sort_order`、按此前 `updated_at DESC, id ASC` 规则回填并建立设备排序索引。
- MySQL migration 使用兼容旧版本的存储过程游标回填，且针对之前可能的中断状态使用幂等列/索引创建；不依赖 MySQL 8 CTE。
- `DbStore` 列表按 `sort_order ASC, id ASC` 返回；创建追加到末尾，更新内容不改位置。
- MySQL 创建快捷方式使用 `SERIALIZABLE` 事务和 `FOR UPDATE`，避免同设备并发追加出现重复位置。
- 新增 `UpdateShortcutOrderReq/Resp` 及 tunnel oneof；Local HTTP、Agent runtime access、Agent tunnel、Cloud device endpoint 和 relay response allowlist 均已接通。
- 前端 `VueDraggable` 以卡片表面拖拽，不配置 drag handle；`button` filter 保留编辑、删除点击；失败时恢复快照、toast 提示并回读服务端顺序，无实际移动不发请求。
- Go 测试覆盖顺序创建/更新语义、非法排列零写入、跨设备拒绝、service 错误分类、Local frame、Agent tunnel dispatch 与 Cloud relay response。

### 范围偏离

无阻断性偏离。原需求中“不实现排序”的边界已由用户后续明确需求取代，Requirement 与 Plan 已同步更新。未新增 `useShortcuts.ts`，排序状态仍归属 `ShortcutsPageShell.vue`，符合现有页面壳的 CRUD 状态管理方式。

## Actual diff summary

本轮快捷方式排序涉及：

- shortcut/tunnel Proto 与 Go、TypeScript generated bindings；
- SQLite/MySQL 排序 migration 与 Agent migrator 升级测试；
- Agent shortcut repository/service/runtime/HTTP、Cloud runtime/route/relay 与业务测试；
- Local/Cloud web API adapters、Session runtime adapters、快捷方式卡片网格与中英文错误文案；
- Requirement、Plan 与本验证文档。

此前工作树已有的 Cloud、认证、会话表单和视觉样式变更不因本排序验证而归因；本结论仅覆盖快捷方式排序及其必要接入路径。

## Expected vs actual changed files

| 范围 | 预期 | 实际 | 结论 |
| --- | --- | --- | --- |
| 数据迁移 | 两种方言增加持久化排序并回填 | 新增 SQLite/MySQL ordering migration | 符合 |
| 设备隔离 | 按当前设备排序、拒绝跨设备 ID | Repository 在事务内读取并精确校验当前设备集合 | 符合 |
| 并发追加 | 新建追加到末尾且不重复位置 | MySQL 使用串行化事务与锁定查询 | 符合 |
| Local/Cloud 传输 | 统一排序 API 和 tunnel 往返 | Local endpoint、Agent dispatch、Cloud route/relay 完整接入 | 符合 |
| 拖拽 UX | 卡片主体拖动、无手柄、按钮可用 | `VueDraggable` 无 handle，过滤 `button` | 符合 |
| 失败行为 | 回滚、错误提示、重读规范顺序 | 使用 drag 前快照、toast 与 `load()` | 符合 |
| 质量验证 | 定向及全量检查 | Go/web 全量检查与双模式构建均已执行 | 符合 |

## Acceptance checklist

### 排序数据与迁移

- [x] SQLite/MySQL 均有 `sort_order` migration、回填和索引。
- [x] 列表按持久化顺序返回，ID 作为稳定兜底。
- [x] 新建快捷方式追加到末尾，编辑不改变顺序。
- [x] 非完整、重复、未知、跨设备排列被拒绝，拒绝后顺序不变。
- [x] MySQL 并发创建使用可串行化事务保护排序分配。
- [x] 旧完整 runtime schema 升级测试验证 `sort_order` 已建立。

### Local 与 Cloud 传输

- [x] Proto、Local runtime frame、Agent runtime dispatch、tunnel response JSON 均支持排序。
- [x] Cloud device route 与 relay response allowlist 支持排序返回，避免请求成功后超时。
- [x] Local/Cloud TypeScript API 和会话/快捷方式页面 adapter 均提供 `updateShortcutOrder`。

### 前端交互

- [x] 直接拖拽快捷方式卡片主体排序，无专用拖拽手柄。
- [x] 编辑、删除 button 不触发拖拽。
- [x] 成功后使用服务端返回的规范顺序。
- [x] 失败后恢复拖动前快照、展示本地化错误并重新加载。
- [x] 无实际位移时不发送排序请求。

## Command results

```text
go test ./...
go vet ./...
yarn --cwd web typecheck
yarn --cwd web lint
yarn --cwd web format
yarn --cwd web test
yarn --cwd web build:local
yarn --cwd web build:cloud
git diff --check
```

结果：全部通过。

- Go：`go test ./...` 和 `go vet ./...` 通过；其中覆盖 agent/cloud runtime、repository、migration 与 relay 的定向测试均为 `ok`。
- Web：typecheck、ESLint、Prettier、Vitest（12 files / 62 tests）均通过。
- Production build：local、cloud 两种 Vite 构建均通过。
- 构建过程仍输出 `@vueuse/core` 的 Rolldown `INVALID_ANNOTATION` 第三方依赖 warning，但不影响产物生成，非本功能引入。
- `git diff --check` 无空白错误。

## Risks

1. 本地环境没有真实 MySQL 实例，因此 MySQL migration 以 SQL 兼容性审查、代码路径和 SQLite migrator 测试验证；上线前仍应在目标 MySQL 版本执行 migration 烟测。
2. 本轮未执行真实浏览器拖拽手工流；自动化验证覆盖 API/transport/repository 和前端静态检查，建议发布前在 Local 与 Cloud 各完成一次卡片拖拽、刷新与失败回滚人工验收。
3. Vite build 的第三方 `@vueuse/core` annotation warning 仍存在，但两个模式均成功构建；需由依赖升级或构建链单独处理。

## Incomplete items

无实现或自动化检查阻塞项。

## Conclusion

快捷方式排序已完成并通过验证：排序按设备持久化，Local 与 Cloud 使用相同运行时链路，非法排列零写入，MySQL 并发追加有事务保护；前端直接拖动卡片主体且不添加拖拽手柄，编辑/删除操作不受影响，失败状态可恢复。验证结论为 **Accepted**。
