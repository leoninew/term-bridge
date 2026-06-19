# Workspace / Session / Terminal 后端接口需求
最后修改时间: 2026-06-19 22:34:45

Review status: Accepted

## Background

前端正在重构为 workspace/session/terminal 的产品模型，需要后端提供稳定接口支撑左侧两级树和右侧多 tab terminal 主界面。

当前已有部分能力：workspace 列表、session 新建、session 列表、session 查询、running session close、history 读取、WebSocket attach/live terminal。缺口集中在目录树接口、session 命名与删除、workspace 删除、workspace 排序，以及按 workspace 查询 sessions。

## Core concepts

### Workspace / 工作区 / 目录

- 一个用户可以有多个 workspace。
- workspace 对应一个目录路径，是 session 的一级分组。
- 前端左侧树的一级节点是 workspace。
- 后端存储仍以 workspace 目录作为一级结构。
- 本需求中“目录”和 workspace 指同一概念，接口命名优先使用 `workspace`。

### Session / 会话

- 每个 workspace 可以有多个 session。
- session 是用户启动的一次命令/终端任务。
- session 需要有用户可编辑的名称。
- session 生命周期至少包括 running / stopped 等状态。
- 前端左侧树的二级节点是 session。

### Terminal / 终端

- 每个 session 有一个 terminal。
- terminal 是 session 的实时 PTY 展示和输入通道。
- 选中 session 后，右侧主界面展示对应 terminal。
- 主界面使用 tab 形式，支持多个已打开 session 之间切换。
- stopped session 不再有 live PTY，只能查看历史输出。

## Goals

1. 明确后端 workspace/session/terminal 的接口边界，为前端重构提供稳定契约。
2. 支持目录 + 会话两级结构，满足左侧 tree 展示。
3. 支持右侧多 tab terminal 所需的 session 查询、attach、close、history 能力。
4. 在现有存储结构上补齐 session name、workspace 删除、session 删除、workspace 排序等能力。
5. 保持本地优先模型，不引入正式用户体系；认证留给后续用户体系需求。

## Non-goals

1. 本轮不设计正式用户体系、权限模型、多用户隔离。
2. 本轮不实现云端同步、远程 workspace、多人协作。
3. 本轮不要求 server 重启后重新 attach 已丢失 PTY 的 running session。
4. 本轮不重构前端 UI，只定义和实现后端接口契约。
5. 本轮不删除 workspace 对应的真实文件系统目录。

## User scenarios

### 1. 新建会话

用户在某个 workspace 下提供：

- session 名称，必填
- 工作目录
- 命令

后端创建 session，启动实际 PTY，并按 workspace/session 两级结构持久化 metadata、state、history 等文件。

### 2. 编辑会话

用户可以修改 session 名称。修改名称不影响 session ID、workspace、历史输出或正在运行的 PTY。

### 3. 关闭会话

running session 可以关闭实际 PTY。关闭后 session record、history 和 metadata 仍保留。

### 4. 删除会话

已经关闭的 session 可以删除。删除 session 会删除后端保存的 session metadata、state、history、process、exit 等 session 目录内容，但不删除 workspace 对应的真实文件系统目录。

### 5. 目录列表

前端需要一次读取目录 + 会话两级结构，用于渲染左侧树：

- workspace 节点
- workspace 下的 session 子节点
- session 的状态、名称、命令、更新时间等摘要

### 6. 删除目录

workspace 可以删除，但必须满足安全约束：

- workspace 下没有 session；或
- workspace 下没有 running session。

删除 workspace 只删除 TermBridge 的 workspace metadata/state 目录，不删除用户真实文件系统目录。

如果 workspace 下存在 stopped sessions，允许删除 workspace，并级联删除这些 stopped sessions 的 TermBridge metadata/history；但只要存在 running session，就必须拒绝删除 workspace。

### 7. 排序目录

后端需要为前端拖拽排序提供支持。前端拖拽完成后提交有序的 `workspace_id` 列表，后端持久化排序字段，后续目录列表按该顺序返回。

### 8. 会话列表

前端可以根据 workspace id 读取该 workspace 下的 session 列表。该接口用于按需展开树节点或刷新单个 workspace 的 session 子列表。

## Acceptance criteria

1. 后端概念清晰区分 workspace、session、terminal。
2. session model 支持用户可编辑名称。
3. 新建 session 必须传入 session name、cwd、command、terminal size。
4. 新建 session 后按 workspace/session 两级结构存储。
5. 支持修改 session name。
6. 支持关闭 running session 对应 PTY。
7. 支持删除 stopped session，且不能删除 running session。
8. 支持读取 workspace tree：workspace + sessions 两级结构。
9. 支持根据 `workspace_id` 读取 session list。
10. 支持删除 workspace，且不会删除真实文件系统目录。
11. 删除 workspace 时必须保护 running sessions。
12. 支持 workspace 排序持久化，返回列表时顺序稳定。
13. 现有 WebSocket terminal attach、history replay、session close 能力不回退。
14. 接口错误需要区分 usage/config/runtime 类错误，便于前端展示。
15. 相关 Go 测试覆盖新增 store、registry、webserver 行为。

## Existing capability snapshot

### 已有

- `GET /api/workspaces`：workspace 平铺列表。
- `GET /api/sessions`：session 平铺列表。
- `POST /api/sessions`：创建 session 并启动 PTY。
- `GET /api/sessions/{id}`：读取单个 session summary。
- `POST /api/sessions/{id}/close`：关闭 live running session。
- `GET /api/sessions/{id}/history`：读取 session history。
- `GET /api/sessions/{id}/ws`：attach live terminal。
- workspace/session 已按 state root 下 workspace/session 两级目录存储。

### 缺口

- session 没有 name 字段。
- 没有 session rename/update 接口。
- 没有 session delete 接口。
- 没有 workspace delete 接口。
- 没有 workspace order 持久化字段和排序更新接口。
- 没有 workspace tree 接口。
- 没有按 workspace id/key 查询 sessions 的接口。
- close session 当前只支持 live runtime；对非 live running/stale state 的处理需要进一步明确。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- 当前流程采用 strict / 严格模式。
- 本需求是新的后端接口需求，不修改既有 xterm hardening 过程文档。
- 用户已更正：第 5 项是“目录列表：目录 + 会话两级结构”；第 8 项是“会话列表：根据目录 id 读取会话列表”。
- 本轮聚焦后端接口；前端正在重构，不以当前前端页面作为需求来源。
- 正式用户体系不在本轮实现范围内。
- workspace 删除语义已确定：没有 running session 时允许删除，并级联删除 stopped sessions 的 TermBridge metadata/history；不删除真实文件系统目录。
- 外部 API 使用 `workspace_id` 作为 workspace 标识；`workspace_key` 仅作为内部存储路径标识或兼容字段。
- 新建 session 必须提供 name，不做默认名称生成。
- workspace 排序接口接收前端拖拽后的有序 `workspace_id` 列表。
- workspace tree 接口一次包含全部 workspace 及其 sessions，并以 workspace 节点 `children` 表达树型结构。
- 常规 session metadata 操作写入 `workspace.json`，读取 workspace tree 和 workspace sessions 时直接读取 `workspace.json`。

## Risk

1. 如果 session name、workspace delete、workspace order 直接叠加在旧 model 上而不统一接口语义，前端树和 tab 状态会出现不一致。
2. 如果 workspace 删除语义不清，可能误删 session 历史或留下 orphan state。
3. 如果 workspace id/key 混用，前端后续会难以维护。
4. 如果 close/delete 不区分 running/stopped，会出现 PTY 泄漏或正在运行任务被误删。
5. 如果排序只在前端保存，后端列表顺序不稳定会导致拖拽结果丢失。

## User review notes

Requirement 草稿已根据用户描述、更正和 5 项决策更新：workspace 删除级联 stopped sessions、外部 API 使用 `workspace_id`、新建 session 必须提供名称、排序提交有序 `workspace_id` 列表、workspace tree 包含全部 sessions。

暂无需要用户确认的未决事项。
