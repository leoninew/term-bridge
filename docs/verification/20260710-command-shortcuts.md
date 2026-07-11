# 命令快捷方式验证

最后修改时间: 2026-07-10 23:40:28

Flow mode: standard / 标准模式
Stage: Verification / 验证
Review status: Draft

## Requirement alignment

按 `docs/requirement/20260710-command-shortcuts.md` 核对：

1. Agent 数据库新增设备作用域的快捷方式模型、SQLite/MySQL migration 与 repository；快捷方式包含稳定 ID、名称、原始命令、可空描述、创建/更新时间。
2. Local 与 Cloud 设备路径均通过既有 runtime/tunnel 边界提供快捷方式 CRUD；没有新增 Cloud shortcut 表或绕过设备路由访问 Agent 数据。
3. 快捷方式卡片直接展示名称、完整命令和非空描述；创建、编辑与删除确认均使用模态窗，未新增独立详情模态窗。
4. 新建会话默认来源是快捷方式；所选快捷方式只复制当时的原始命令文本，用户可切换到直接命令，Session 不保存 shortcut ID。
5. Session 编辑仅在精确 `stopped` 状态可用；名称和命令可更新，cwd 只读，更新不会启动或 rerun 会话。
6. 快捷方式后续编辑或硬删除不会追溯修改既有 Session 或 SessionRun 命令快照。

实现与需求的主要行为对齐；但前端格式检查失败，且快捷方式管理界面/API 缺少独立自动化测试，故本验证文档保持 Draft，不能将质量验收标记为已接受。

## Spec alignment

不适用。standard / 标准模式按已接受 Requirement 与 Plan 核对，未创建独立 Spec / 规格文档。

## Plan alignment

### 已落实

- Proto：新增 `shortcut.proto`，Session patch 增加可选 `command`，Tunnel oneof 追加快捷方式 CRUD 请求/响应；`task proto` 已重新生成 Go/TypeScript 绑定。
- 数据迁移：新增 SQLite/MySQL `shortcuts` 表与设备/更新时间索引；migrator 保留旧运行时三表的迁移前完整性校验，Goose 运行后额外验证 `shortcuts` 存在，避免旧完整数据库在升级前被误拒绝。
- 设备隔离：repository 的查询、更新和删除均以当前 `device_id` 约束；快捷方式名称不加唯一限制，删除是硬删除。
- 会话编辑：`UpdateStoppedSession` 在 SQL 条件更新中同时约束原 `updated_at` 和 `current_state='stopped'`，避免 rerun 竞争下的延迟编辑覆盖。
- Runtime：Local handler、Agent connector、Tunnel JSON 转换、Cloud runtime endpoint 与 Cloud device route 均已接入四项快捷方式操作。
- 前端：本地/Cloud 快捷方式路由和共享页面壳、设置入口、卡片和 CRUD 模态窗已实现；新建/编辑会话使用共享命令来源字段。

### 偏离或未充分覆盖

1. Plan 预计的 `web/src/composable/useShortcuts.ts` 未创建；当前快捷方式加载与 mutation 状态收敛在 `ShortcutsPageShell.vue`。这不改变产品行为，但与预计文件清单不同。
2. Plan 要求的快捷方式 API/composable/组件测试尚未实现。当前 web 测试只覆盖 `useCreateSessionDraft` 的快捷方式来源和原始命令拷贝。
3. Plan 包含 MySQL 方言连接/迁移冒烟；本次环境仅运行 SQLite migrator 测试，未提供 MySQL 实例。
4. Plan 的浏览器手工流（两种模式操作、离线设备、真实表单交互）未执行。

## Actual diff summary

本功能实际涉及：

- `proto/termbridge/agent/v1/shortcut.proto`、Session/Tunnel proto 与生成绑定。
- `migrations/agent/sqlite/202607100001_command_shortcuts.sql`、`migrations/agent/mysql/202607100001_command_shortcuts.sql`。
- Agent shortcut model、service、设备作用域 DbStore、migration 完整性保护与 stopped-only Session 更新。
- Agent Local HTTP runtime frame 映射、Agent tunnel dispatch、Tunnel response JSON 转换、Cloud relay/device route。
- Local/Cloud 前端 API、快捷方式路由和页面、卡片与 CRUD 模态窗、共享 Session 表单字段、会话侧边栏入口和 stopped-only 编辑保护。
- 中英文文案及定向 Go/draft 测试。

工作树同时包含与本功能无关的 OAuth、Turnstile/CSRF、Cloud auth/config、请求日志和相关文档变更；它们没有计入本功能范围或本验证结论。

## Expected vs actual changed files

| 范围 | 预期 | 实际 | 结论 |
| --- | --- | --- | --- |
| Agent 数据模型与 migration | Agent 本地 `shortcuts` 表、SQLite/MySQL、设备范围 | 新增 model、repository 和两份 migration | 符合 |
| 迁移升级 | 旧完整 runtime schema 可升级 | 迁移后验证 `shortcuts`，并有 SQLite 升级模拟测试 | 符合 |
| 命令语义 | 原始命令文本无 argv 拆分/重组 | model、service、draft、frame 测试均直接保留 raw text | 符合 |
| stopped Session 编辑 | 仅 stopped；cwd 只读；不启动 | UI 与 Registry 双重限制，repository 加状态/时间条件 | 符合 |
| Local/Cloud runtime | 复用认证、device tunnel、offline 语义 | Local routes、Agent dispatch、Cloud device route/relay 均接入 | 符合 |
| 快捷方式管理 UI | 卡片、CRUD 模态窗、设置入口 | 已新增页面和组件 | 符合 |
| 前端状态测试 | API、composable、组件关键交互测试 | 仅 `useCreateSessionDraft` 测试新增覆盖 | 不充分 |
| `useShortcuts.ts` | Plan 预计的状态 composable | 未创建，逻辑位于页面壳 | 范围偏离但不阻断行为 |

## Acceptance checklist

### 数据与迁移

- [x] 快捷方式模型具备 ID、名称、原始命令、可空描述、时间与设备归属。
- [x] SQLite 与 MySQL migration 已新增；repository 按当前设备范围操作。
- [x] 名称/命令为空被拒绝；空描述被标准化为缺省。
- [x] 命令按原始文本保存和返回。
- [x] SQLite migrator 覆盖新库建表和完整旧 runtime schema 升级。
- [ ] 未在可用 MySQL 实例上执行 migration/CRUD 冒烟。

### 快捷方式管理界面

- [x] 会话设置菜单提供快捷方式入口，Local 与 Cloud 有对应路由。
- [x] 卡片展示名称、命令和非空描述。
- [x] 创建、编辑和删除确认均通过模态窗；没有独立详情模态窗。
- [x] CRUD 成功后重新加载列表；失败保留当前列表并显示 toast 错误。
- [x] 空状态显示创建入口。
- [ ] 未执行浏览器级手工交互验收。
- [ ] 未新增快捷方式页面/API/组件自动化测试。

### 新建与编辑会话

- [x] 新建 draft 默认 `shortcut` 来源，可切换直接命令。
- [x] 所选快捷方式复制原始命令文本，提交仍只发送命令文本。
- [x] 空名称、空 cwd 或空命令无法通过创建 draft 校验。
- [x] stopped-only 编辑在侧边栏、页面壳和后端 Registry 均受限制；failed/running 被拒绝。
- [x] 编辑表单 cwd 为只读，更新请求只发送 name/command。
- [x] update 不启动/重启会话；repository 状态条件保护 rerun 竞争。
- [ ] 未新增“编辑后 rerun 使用新 run 快照、旧 run 保持不变”的端到端自动化测试；实现依赖现有 rerun snapshot 路径与定向 session 更新测试。

### 兼容与质量

- [x] 全量 Go 与 web 测试通过。
- [x] `task proto` 通过，生成绑定未手工修改。
- [x] `yarn --cwd web typecheck` 与 `yarn --cwd web lint` 通过。
- [x] `git diff --check` 与快捷方式 Go 文件 `gofmt -l` 通过。
- [x] Local/Cloud frontend production build 通过。
- [ ] `yarn --cwd web format` 失败：11 个前端文件不符合 Prettier 格式，其中包含快捷方式页面/组件及共享 Session 表单文件。
- [ ] `./bin/golangci-lint run ./cmd/... ./internal/...` 失败，报告两个工作树问题：`internal/cloud/api/handler/auth_security.go:60` 未检查 `response.Body.Close()` 返回值，以及 `internal/agent/api/handler/errors.go:102` 的未使用方法。两项均不属于快捷方式实现，但阻断全仓静态检查。

## Command results

### Proto 与全量测试

```text
task proto && task test
```

结果：通过。

- `task proto` 完成 Go 与 web Proto 生成。
- Web Vitest：12 个测试文件、62 个测试通过。
- `go test ./cmd/... ./internal/...` 通过；所有有测试包为 `ok`。

### 快捷方式定向 Go 测试

```text
go test ./internal/agent/model/task/shortcut ./internal/agent/repository/task/state ./internal/agent/infrastructure/database ./internal/agent/application/task/shortcut ./internal/agent/application/task/terminal ./internal/agent/application/user ./internal/agent/api/handler ./internal/cloud/api/handler ./internal/shared/dto/protocol/tunnel
```

结果：通过。覆盖快捷方式 model/service/repository/migration、stopped Session 编辑、Agent runtime dispatch、Local frame 映射和 Cloud relay。

### 前端定向测试

```text
yarn --cwd web test useCreateSessionDraft.test.ts
```

结果：通过，1 个测试文件、4 个测试。覆盖新建 draft 默认快捷方式来源、原始命令无损拷贝、切换直接命令与输入校验。

### 前端静态检查与构建

```text
yarn --cwd web typecheck
yarn --cwd web lint
yarn --cwd web build:local
yarn --cwd web build:cloud
```

结果：均通过。

两个构建均输出来自 `node_modules/@vueuse/core/dist/index.js` 的 Rolldown `INVALID_ANNOTATION` warning；产物成功生成，属于第三方依赖注释警告，非本轮功能代码错误。

### 格式与静态分析

```text
git diff --check
gofmt -l <shortcut-related Go files>
yarn --cwd web format
./bin/golangci-lint run ./cmd/... ./internal/...
```

- `git diff --check`：通过。
- `gofmt -l`：无输出，目标 Go 文件格式正确。
- `yarn --cwd web format`：失败。Prettier 报告 11 个格式不符合文件，包括 `CreateSessionPanel.vue`、`EditSessionDialog.vue`、`SessionFormFields.vue`、`SessionsPageShell.vue`、全部四个快捷方式组件、`WorkspaceSessionSidebar.vue` 与两个 `ShortcutsView.vue`。
- `golangci-lint`：失败，两个报告均在非快捷方式工作树改动中，详见验收清单。

## Missed or expanded scope

- 没有引入快捷方式跨设备同步、Cloud 数据表、导入导出、模板变量或自动执行，范围没有超出已接受需求。
- 没有新增计划中预计的 `useShortcuts.ts`；页面壳自行维护状态。
- 当前前端新增文件以压缩的单行模板/脚本风格写入，虽可通过 typecheck 与 ESLint，但导致 Prettier 失败且降低可维护性。

## Risks

1. 前端格式不合规会阻断执行项目完整 `task check` 或依赖 Prettier 的 CI；应在提交前统一格式化快捷方式和共享 Session Vue 文件，并复查实际 diff。
2. `golangci-lint` 的两个失败来自本功能外的未提交 auth/security 与旧 handler 代码；完整工作树当前不能声称静态分析全绿，提交时应拆分或先修复相应问题。
3. 缺少快捷方式 API、页面和模态窗组件测试，后续 UI 重构可能破坏 Cloud URL、失败提示或重复提交保护而不被 web 单测发现。
4. 缺少 MySQL 实例迁移冒烟与浏览器手工流；SQLite 单测和构建不能完全替代这两类验证。

## Incomplete items

1. 格式化 11 个前端文件并重新运行 `yarn --cwd web format`。
2. 为 Local/Cloud shortcut URL、CRUD reload/error 行为、编辑表单只读 cwd 与 stopped-only 入口补充前端测试。
3. 在可用 MySQL 环境执行 migration 与设备范围 CRUD 冒烟。
4. 执行浏览器手工验收：快捷方式 CRUD、空列表切换直接命令、同设备 Local/Cloud 可见性、设备切换隔离、running/failed 编辑拒绝、Cloud 离线错误。
5. 单独处理或从本功能提交中剥离全仓 `golangci-lint` 的两项非快捷方式失败。

## Conclusion

快捷方式的核心实现、迁移、设备隔离、Local/Cloud tunnel 访问、原始命令语义和 stopped-only Session 编辑均已通过全量测试、定向测试、类型检查、ESLint 与双模式生产构建验证。未发现由快捷方式后端契约或运行时链路导致的测试失败。

但前端 Prettier 格式检查失败，快捷方式页面/API/组件测试不足，且未执行 MySQL 与浏览器手工冒烟；此外完整 Go 静态分析受两项无关工作树问题阻断。因此当前交付结论为**功能实现可用但质量验证未完成**，本验证文档维持 Draft，待上述 incomplete items 收敛后再更新为 Accepted。
