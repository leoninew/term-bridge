# Workspace 与 Session 排序需求
最后修改时间: 2026-06-26 12:05:00

Review status: Accepted

## Background

当前 workspace 节点已支持拖拽排序，但 session 节点尚不支持在所属 workspace 内拖拽排序。需要补齐会话节点排序能力，并明确后端排序数据模型，避免在实体本身加入 `order` / `sort_order` 一类字段。

## Goal

- 工作区和会话创建时，默认追加到各自父级顺序末尾。
- workspace 节点继续支持拖拽排序。
- 每个 workspace 下的 session 节点支持拖拽排序。
- session 节点拖拽不需要单独拖拽手柄，整行即可拖动。
- 不同 workspace 的 session 节点排序互不干扰。
- 后端数据不通过实体字段排序，也不通过重排实体列表表达排序；由父级维护 ID 顺序数组，例如 `workspace_ids` / `session_ids`。
- 后端返回 workspace/session 列表时，总是按父级 ID 顺序数组排序。

## Non-goal

- 不支持跨 workspace 拖动 session。
- 不新增 session 级 `order`、`sort_order` 或类似排序字段。
- 不把排序语义放到 session/workspace 实体列表物理顺序中。
- 不改变 session 运行、停止、删除、重跑等生命周期行为。
- 不改变搜索过滤的业务语义。

## User scenarios

1. 用户在左侧面板拖动 workspace，workspace 显示顺序被持久化。
2. 用户在某个 workspace 内拖动 session，只有该 workspace 内的 session 顺序变化。
3. 用户在 workspace A 内拖动 session，不影响 workspace B 的 session 顺序。
4. 用户新建 workspace，新 workspace 默认出现在 workspace 顺序末尾。
5. 用户新建 session，新 session 默认出现在所属 workspace 的 session 顺序末尾。
6. 用户刷新页面或重新拉取列表后，workspace/session 顺序保持与父级 ID 顺序数组一致。

## Acceptance

- Workspace 排序由父级 `workspace_ids` 表达，后端返回列表按该数组排序。
- Session 排序由所属 workspace 父级 `session_ids` 表达，后端返回该 workspace 的 session 列表和 workspace tree children 时按该数组排序。
- 新建 workspace/session 默认追加到对应父级顺序数组末尾。
- session 节点可在所属 workspace 内拖拽排序，且不需要拖拽手柄。
- 不同 workspace 的 session 拖拽列表独立，不发生跨 workspace 干扰。
- 搜索过滤状态下禁用拖拽，避免对过滤后的局部列表写入不完整排序。
- 前后端不再依赖 `sort_order` 字段。
- 相关 Go 测试、前端 store 测试和前端类型检查通过。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- 使用 light / 轻量模式记录本需求。
- 后端采用父级 ID 顺序数组模型：根级维护 `workspace_ids`，workspace 级维护 `session_ids`。
- API 保持 workspace order 既有入口，并新增 workspace 内 session order 入口。
- 前端 session 拖拽使用每个 workspace 独立的 `VueDraggable` 实例，不配置跨列表 group。

## Risk

- 旧数据没有 `workspace_ids` / `session_ids` 时，需要使用创建时间、名称、ID 作为 fallback 排序，并在新增项目时初始化/追加顺序，避免新项目插入到已有项目之前。
- 若排序请求只包含部分 ID，后端需要保留未包含的已有 ID 并追加在后，避免数据丢失。
- 搜索过滤下若允许拖拽会造成顺序不完整，因此必须禁用拖拽。
