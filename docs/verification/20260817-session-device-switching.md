# 会话页设备切换验证
最后修改时间: 2026-08-17 12:41:48

Review status: Draft

## Requirement Alignment / 需求对齐

- 状态栏只要有当前 PC 名称即提供设备菜单入口，不再使用 `showDeviceSwitcher` 以云端连接状态控制入口。
- 本机模式通过 `localApiClient` 请求本地 Agent 的 `GET /api/cloud/devices`；浏览器不直接调用 Cloud API，也不依赖 CORS。
- 设备列表由 Agent 返回，前端直接绑定，不补当前设备、不去重、不排序或附加“本机”标记。
- 本机持有 Cloud token 时，菜单打开会通过 Agent 取得设备列表；未持有 token 时菜单仍可打开，但列表为空。
- 本机切换到其他在线设备使用 Cloud 工作台整页导航；Cloud 工作台内继续使用现有同源设备列表和路由切换。
- `ListDevicesResp` 保持为 Agent 的 Cloud client、service 和 handler 的响应类型；`DeviceSummary` 只作为列表项。
- 本机 session 缺失时，选择设备会先调用本地 Agent 恢复 Cloud session，恢复失败时显示连接错误。

## Spec Alignment / 规格对齐

不适用。此任务采用 standard / 标准模式，未创建独立规格文档。

## Plan Alignment / 计划对齐

- Agent Cloud service、Cloud HTTP client 与本地 `/api/cloud/devices` 代理端点已实现。
- 后端测试覆盖成功转发、缺少 bearer token 时返回 `401` 且不向 Cloud 出站，以及 Cloud 上游失败时返回 `502`。
- 状态栏展示、事件转发和 `SessionsPageShell` 的本机/Cloud 切换策略已完成。
- 计划中的本机前端请求约束已满足；本机代码只使用本地 API 封装。
- 计划中的真实多设备手工验证尚未完成。

## Actual Diff Summary / 实际差异摘要

- Agent 增加设备列表 Cloud API 能力和经 bearer token 授权的本地代理端点。
- `ListDevicesResp` 在 Agent 层完整透传，避免将 `DeviceSummary` 作为响应重新包装。
- 本机 Web API 增加该本地端点调用；会话页把当前本机设备用于状态栏名称展示。
- 状态栏设备名始终作为菜单触发器；菜单直接渲染后端/Cloud 提供的设备列表，离线和当前设备保持不可选。
- Cloud 工作台继续支持从已进入的设备切换至其他在线设备。
- 本机未建立 Cloud session 时，切换目标设备前会先恢复 session，避免静默无操作。
- 补充中英文文案，并修复一条与现有会话编辑行为不一致的断言。

## Expected And Actual Files / 预期与实际文件

| 类别 | 预期 | 实际 |
| --- | --- | --- |
| Agent Cloud 代理 | service、client、handler 与测试 | `internal/agent/application/user/cloud_service.go`、`internal/agent/infrastructure/cloudapi/client.go`、`internal/agent/api/handler/server.go` 及对应测试 |
| 本机设备列表调用 | `web/src/features/local/api.ts` | 已修改 |
| 会话页设备菜单 | StatusBar、Workbench、PageShell | 三个组件均已修改 |
| 文案 | `web/src/i18n.ts` | 已修改 |
| 过程文档 | requirement、plan、verification | 三份文档已创建或更新 |

未发现超出计划的功能性范围；`internal/agent/api/handler/errors_test.go` 的改动是修复过期断言，使其与既有会话编辑规则一致。

## Acceptance Checklist / 验收清单

- [x] 有当前 PC 名称时，设备名始终可打开菜单。
- [x] 前端不以云端连接状态控制菜单入口。
- [x] 前端原样绑定后端/Cloud 返回的设备列表，不合成本机项。
- [x] 本机设备列表请求仅访问本地 Agent 路径。
- [x] 缺少 token 时 Agent 返回 `401`，并且不访问 Cloud。
- [x] Cloud 上游失败时 Agent 返回 `502`。
- [x] `ListDevicesResp` 在 Agent 边界保持为响应类型，`DeviceSummary` 只作为列表项。
- [x] 本机缺少 Cloud session 时，切换前先恢复 session。
- [x] 当前设备和离线设备不可切换，其他在线设备可切换。
- [x] Cloud 工作台保留连续设备切换路径。
- [x] 后端测试、前端类型检查、lint、格式检查和单测通过。
- [ ] 在真实多设备环境完成本机 -> PC A -> PC B 的手工验收。

## Test Results / 测试结果

| 命令 | 结果 |
| --- | --- |
| `go test ./cmd/... ./internal/agent/...` | 通过 |
| `yarn --cwd web typecheck` | 通过 |
| `yarn --cwd web lint` | 通过 |
| `yarn --cwd web format` | 通过 |
| `yarn --cwd web test` | 通过，29 个文件、163 项测试 |
| `git -c core.whitespace=cr-at-eol diff --check HEAD` | 通过 |

## Scope / 范围

未修改设备绑定、OAuth、Cloud CORS、远端会话代理或数据库结构。浏览器到 Cloud 的直连保持不在本任务范围内。

## Risks And Incomplete Items / 风险与未完成项

- 尚未在真实多设备和实际 Cloud 登录态下验证跨文档跳转后的终端 attach 与连续切换。
- 本机切换到 Cloud 工作台仍依赖既有 `public_url`、Cloud 登录态和路由配置；本任务未改变这些前置条件。

## Conclusion / 结论

自动化验证通过，代码与 requirement、plan 对齐。由于真实多设备手工验收尚未完成，验证记录维持 `Draft`。
