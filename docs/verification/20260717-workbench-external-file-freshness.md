# Workbench 外部文件变更刷新与保存冲突验证
最后修改时间: 2026-07-17 18:11:20

Review status: Accepted

## Requirement alignment

依据 `docs/requirement/20260717-workbench-external-file-freshness.md` 核对。已暂存实现覆盖 Agent 工作区监听、Local/Cloud WebSocket 事件传递、Tunnel 协议、Workbench Provider 缓存失效与 ETag 保存冲突保护。

本次按用户要求采用文档、实际 diff 与自动化检查验证；未启动开发服务器、浏览器或 Cloud 实例。

## Spec alignment

不适用。本功能按已有 Requirement 直接实施，未创建独立 Spec 文档。

## Plan alignment

不适用。本功能按已有 Requirement 直接实施，未创建独立 Plan 文档。

## Actual diff summary

暂存区包含 64 个文件，涉及：

- Agent：`fsnotify` 递归监听、订阅队列溢出/监听异常的 `rescan_required` 降级、订阅关闭与工作区根路径解析。
- 文件 API：`/fs/events` Local WebSocket、文件 `Stat`、内容敏感 revision，以及既有文件写入的 ETag 前置条件。
- Cloud：浏览器 WebSocket 的设备/工作区授权、Tunnel 订阅与事件中继、断线关闭帧处理。
- 协议：`FsChangeKind`、订阅/确认/变更事件消息及对应 Go/TypeScript 生成代码，Tunnel 协议版本同步更新。
- Workbench：事件订阅与重连、序列缺口/异常时整工作区保守失效、文件/目录缓存失效、in-flight 读防旧数据回填、脏编辑器 write-base ETag 保留及缓存容量限制。
- 支撑：配置队列边界、token 查询参数日志脱敏，以及 Go 和 Web 针对性测试。

## Expected vs actual changed files

需求预期的 Agent、Cloud、Proto、前端 Provider/API、配置与测试均存在对应改动。

存在范围偏差：暂存区还混入此前/并行的 CLI、Logger、Session UI、Workbench 基础模块与 HMR 相关改动；未暂存区另有 4 个文件，包括 Cloud 路由测试、Session 组件改动和本次 `fileSystemProvider.ts` lint 修复。因此当前暂存区不是可直接代表本需求的纯净提交边界，提交前需要再次拆分。

## Acceptance checklist

- [x] 创建、修改、删除、重命名事件具备 Agent 类型化事件、Local WebSocket、Cloud Tunnel 和浏览器订阅传递路径。
- [x] Provider 对截断目录列表返回错误而不缓存为完整目录。
- [x] 外部变更会失效文件、子路径和父目录缓存，并发出 FileService 变更事件。
- [x] 路径与全局 epoch 防止事件发生前发起的旧请求写回缓存。
- [x] 外部刷新不会清空 write-base ETag，脏编辑器保存继续使用原始版本进行冲突检测。
- [x] 已有文件写入要求 ETag，过期 revision 仍以 `revision_conflict` 拒绝。
- [x] SCM 通过同一 Provider 变更回调刷新；工作区事件和浏览器外部恢复均触发保守失效。
- [x] WebSocket subscription、mounted Workbench disposable 与 Agent/Cloud 订阅关闭路径均有实现和覆盖。
- [x] Provider 内容、stat、目录和 write-base 缓存都有容量边界。

## Command results

| 命令 | 结果 | 说明 |
| --- | --- | --- |
| `go test -count=1 ./internal/agent/api/handler ./internal/agent/application/task/file ./internal/agent/infrastructure/storage/workspacefile ./internal/agent/infrastructure/workspacewatch ./internal/cloud/api/handler ./internal/shared/api/middleware/requestlog ./internal/shared/infrastructure/config` | PASS | 7 个目标 Go 包均通过，覆盖监听、缓存 revision、Local/Cloud relay、日志脱敏和配置。 |
| `yarn --cwd web test platformFileSystemProvider api` | PASS | 3 个测试文件、20 个测试通过，覆盖事件解码、重连/失效、缓存和 Provider 行为。 |
| `yarn --cwd web eslint src/features/workbench/fileSystemProvider.ts src/features/workbench/api.ts src/features/workbench/platformFileSystemProvider.ts` | PASS | 本次修复的未使用参数及 freshness 核心前端模块均通过 lint。 |
| `yarn --cwd web lint` | FAIL（范围外） | `SessionsPageShell.vue:241` 存在未使用 `useSessionsLayoutMode`，以及既有 `WorkspaceSessionSidebar.vue` 显式 emits warning；均不在 freshness 模块内。 |
| `git diff --cached --check` | PASS | 暂存差异未发现空白错误。 |

## Risks

- 未按本次用户指令启动开发服务器或浏览器，因此没有对真实 fsnotify 到浏览器页面刷新的全链路进行运行时观察。
- Cloud 中继的真实授权、设备在线状态和 Tunnel 断线重连未连接真实 Cloud/Agent 环境验证；已由目标 Go 测试覆盖协议中继路径。
- `fileSystemProvider.ts` 的 lint 修复仍未暂存，需由交付者决定是否并入相关提交。

## Incomplete items

- 需要先从当前 64 文件暂存区中拆出与本需求无关的 CLI、Logger、Session UI、HMR 等改动，形成纯净提交边界。
- 需要修复或拆分 `SessionsPageShell.vue` 的未使用导入，才可使全量 `yarn --cwd web lint` 通过。

## Conclusion

目标文件刷新、缓存失效、保存冲突保护与事件 relay 的针对性验证通过；但由于当前暂存区混入无关改动、全量 Web lint 被范围外问题阻断且未做真实服务端到浏览器运行时观察，交付结论为“功能验证通过，提交边界与全量 lint 待清理”。
