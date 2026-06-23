# 停止态会话重新运行规格
最后修改时间: 2026-06-23 18:44:58

Review status: Accepted

## Requirement basis

基于需求文档：`docs/requirement/20260623-session-rerun.md`。

此前 Spec 已接受；用户追加存储结构收敛要求后，本 Spec 重新进入 Draft，等待当前版本 review。

关键需求：

1. 严格模式 / strict。
2. stopped / failed 会话支持“重新运行”。
3. rerun 复用同一个 `session.id`，不创建新 session record。
4. rerun 前由 runtime / registry 在本地重命名旧 `history.log`。
5. `workspace.json` 作为 workspace + sessions 当前事实的聚合存储。
6. 合并收拾 `process.json` / `session.json` / `state.json` / `exit.json`，不再将它们作为当前 session 事实来源。
7. 不向后兼容旧碎片文件，不迁移已有碎片文件。
8. 归档不需要独立接口，不需要云端 / gateway / agent API 感知。
9. rerun 使用原始持久化 command record，不从展示字符串反解析。
10. 启动、关闭、删除是同步操作。
11. 状态集合只有 `running` / `stopped` / `failed`，不进行 `starting` / `stopping` 历史兼容。
12. create failure 固定 failed，并返回明确状态供前端组织 UI。
13. rerun 成功进入 running，rerun 失败固定进入 failed。
14. 新 terminal 不回放旧 history。

## Overview

新增一个显式 rerun 能力，跨越 Gateway、Agent tunnel、Terminal registry、runtime 和前端 UI。

后端提供：

```http
POST /api/devices/:deviceId/sessions/:sessionId/rerun
```

该接口同步启动同一个 session 的新 PTY runtime。它不同于 create：

- create 新建 session record 和新的 `session.id`。
- rerun 复用已有 session record 和同一个 `session.id`。

本轮同时收敛本地存储结构：

- `workspace.json` 保存 workspace 以及其 children sessions 的当前事实。
- `history.log` 作为大体积 terminal 输出继续独立存放。
- `session.json`、`state.json`、`process.json`、`exit.json` 不再作为当前事实文件继续扩展。
- 不读取、不迁移、不兼容已有碎片文件。

rerun 的核心顺序：

1. 从 `workspace.json` 读取 session 当前事实。
2. 校验 session 当前状态为 stopped / failed。
3. 校验没有 live runtime。
4. 校验 cwd、command、terminal size。
5. 在本地 session 目录中重命名旧 `history.log`。
6. 将当前 run 的小型 metadata 归入 `workspace.json` 的 archive/current run 结构。
7. 创建新的 history writer。
8. 启动新的 PTY runtime。
9. 将新的 process record 保存到 `workspace.json` 当前 session 节点。
10. 保存 lifecycle state 为 running。
11. 注册 runtime。
12. 返回同一个 session id。

如果步骤 5 后失败，状态固定写入 failed，且已完成的 history 归档不回滚。

## State model

目标状态集合：

```text
running
stopped
failed
```

不进行历史兼容：

- 移除 `starting` / `stopping` 作为合法业务状态。
- 新旧代码路径都不应继续依赖 `starting` / `stopping`。

状态机：

```text
create ok ------------------------------> running
create failure -------------------------> failed
running -- natural exit ok / close ok --> stopped
running -- runtime/wait failure --------> failed
stopped -- rerun ok --------------------> running
failed  -- rerun ok --------------------> running
stopped -- rerun failure ---------------> failed
failed  -- rerun failure ---------------> failed
stopped -- delete ----------------------> deleted
failed  -- delete ----------------------> deleted
```

`deleted` 不是 session lifecycle state，只代表同步删除记录。

## Storage model

### Target file layout

```text
.termbridge/
  workspaces/
    <workspace_id>/
      workspace.json
      sessions/
        <session_id>/
          history.log
          history.<archive_id>.log
```

`workspace.json` 是聚合根：

- workspace metadata
- sessions children
- 每个 session 的当前 metadata
- 每个 session 的当前 lifecycle state
- 每个 session 的 current run metadata
- 每个 session 的 archived run metadata

`history.log` 继续是独立文件，因为 terminal 输出可能较大，不适合塞进 `workspace.json`。

### Session node target shape

`workspace.json` 中的 session node 建议从当前结构扩展为：

```json
{
  "session_id": "...",
  "name": "默认",
  "launch_cwd": "D:\\work",
  "command": {
    "executable": "...",
    "command": "go",
    "args": ["test", "./..."],
    "env_strategy": "inherit",
    "env_count": 123
  },
  "history": {
    "path": "history.log",
    "max_lines": 2000,
    "max_bytes": 1048576,
    "max_line_bytes": 4096,
    "truncated": false
  },
  "state": {
    "state": "stopped",
    "reason": "user_process_exited",
    "updated_at": "2026-06-23T08:33:18Z"
  },
  "current_run": {
    "process": {},
    "exit": {}
  },
  "archived_runs": [
    {
      "archive_id": "20260623T083318Z",
      "history_path": "history.20260623T083318Z.log",
      "process": {},
      "exit": {}
    }
  ],
  "created_at": "...",
  "updated_at": "..."
}
```

具体 Go 类型命名可在实现时按现有 domain package 收敛，但职责边界必须清楚：当前 session 事实来自 `workspace.json`，不是来自多个碎片文件。

### Files no longer used as current truth

以下文件不再作为当前 session 事实来源：

```text
session.json
state.json
process.json
exit.json
```

处理原则：

1. 新代码不再写入这些文件。
2. 新代码不再读取这些文件。
3. 新代码不从这些文件迁移数据到 `workspace.json`。
4. 新代码不为这些文件提供 fallback。
5. 不做长期历史兼容。
6. 旧文件如果留在磁盘上，只是未使用遗留文件，不是新结构的数据来源。

### Exit record

`exit.json` 本轮一并收敛：

- `exit` 是小型结构化 run metadata，和 process/state 一样保存在 `workspace.json` 的 current/archived run 中。
- rerun 前不生成新的 `exit.<archive_id>.json`。
- 新代码不写当前 `exit.json`，也不读旧 `exit.json`。
- history 归档文件名是旧 run 输出的唯一独立文件入口。

## Archive model

### File model

每次 rerun 前，将当前运行输出在本地 session 目录中重命名为归档文件。

文件命名：

```text
history.<archive_id>.log
```

`archive_id` 规则：

- 使用 UTC 时间戳作为基础，例如 `20260623T083318Z`。
- 文件名跨平台安全，不包含冒号。
- 同一秒冲突时追加序号或更高精度，不能覆盖已有归档。

旧 run 的 state/process/exit metadata 不通过独立 JSON 文件归档，而是进入 `workspace.json` 的 `archived_runs`。

### Scope

归档是 runtime / registry 的本地文件与 `workspace.json` 整理：

1. 不新增归档列表 API。
2. 不新增归档 history API。
3. 不通知云端 / gateway。
4. 不经过 agent tunnel。
5. 不要求前端展示或读取归档。

## Compatibility and migration

本轮明确不做兼容和迁移：

1. 不读取旧 `session.json` / `state.json` / `process.json` / `exit.json`。
2. 不把旧碎片文件折叠进 `workspace.json`。
3. 不为旧碎片文件提供 fallback。
4. 不写迁移脚本。
5. 不维护新旧结构双写。
6. 不主动删除旧碎片文件；除非用户后续单独要求清理，否则本功能只停止使用它们。

## Design decisions

### 1. Rerun 是显式动作，不是自动恢复

状态机允许 `stopped -> running` / `failed -> running` 只服务于用户触发的 rerun。

列表刷新、进程状态校验、attach、history 读取、delete 判断不能根据该状态机自动复活终态 session。

### 2. `workspace.json` 是 session 当前事实聚合根

当前代码已经把 session 大部分 metadata 写入 `workspace.json` children。继续保留 `session.json` / `state.json` / `process.json` / `exit.json` 会造成事实源分裂。

本轮将当前事实收敛到 `workspace.json`：

- session metadata 不再另存 `session.json`。
- lifecycle state 不再另存 `state.json`。
- process record 不再另存 `process.json`。
- exit record 不再另存 `exit.json`。

### 3. 旧运行输出本地重命名归档

rerun 前 runtime / registry 将旧 `history.log` 重命名为 archived run 输出。不存在的文件不报错，存在的文件不可覆盖。

不引入 store 级归档接口，也不通过 gateway / agent 暴露归档。

### 4. 后端复用原始 command record

前端 `SessionSummary.command` 是展示字符串，不适合作为 rerun 的权威输入。rerun 应由后端读取 `workspace.json` 中 session 节点保存的原始 command record：

- `Command`
- `Args`
- 必要时重新 resolve executable

### 5. 同步操作语义

- create：同步返回 running 或 failed。
- close：同步等待 close/wait 完成，返回 stopped 或 failed/error。
- delete：同步删除 stopped/failed 记录。
- rerun：同步返回 running 或 failed。

UI 不展示 starting/stopping。

## Affected components

### Backend domain

- `internal/domain/session/state.go`
  - 收敛状态集合。
  - 移除 starting/stopping 业务状态。
  - 允许终态到 running 的显式 rerun 转换。
- `internal/domain/session/session.go`
  - 作为业务视图类型保留，但当前持久化来源应来自 workspace session node。
- `internal/domain/workspace/workspace.go`
  - 扩展 `SessionNode`，承载 history/state/current_run/archived_runs。

### State repository

- `internal/infrastructure/repository/state/store.go`
  - `SaveSession` / `LoadSession` 只读写 `workspace.json`。
  - `SaveState` / `LoadState` 改为读写 `workspace.json` 中 session node state，或被更明确的 workspace-session 更新方法替代。
  - `SaveProcess` / `LoadProcess` 改为读写 `workspace.json` 中 current run process，或被更明确的 workspace-session 更新方法替代。
  - `SaveExit` / `LoadExit` 改为读写 `workspace.json` 中 current run exit，或被更明确的 workspace-session 更新方法替代。
  - `ListSessionsByWorkspaceId` 不再逐 session 读取 state/exit 碎片文件。
  - 不读取、不迁移、不 fallback 旧碎片文件。
  - 不新增归档 API。
  - 不新增归档列表 / 读取能力。

### Terminal registry/runtime

- `internal/application/terminal/registry.go`
  - 增加 `RerunSession`。
  - 在 rerun 前执行本地 history 文件重命名。
  - 更新 `workspace.json` 中 archived/current run metadata。
  - 抽取或复用启动 runtime helper。
  - create / rerun 均同步产出 running 或 failed。
- `internal/application/terminal/runtime.go`
  - close 不写 stopping。
  - wait loop 继续负责最终 stopped/failed，但写入目标是 `workspace.json` session node。

### Agent tunnel

- `internal/application/agent/runtime_access.go`
- `internal/application/agent/client.go`
  - 增加 `rerun_session`。
  - 不增加归档相关 tunnel method。

### Gateway API

- `internal/transport/http/gatewayapi/server.go`
  - 新增 `/rerun` route。
  - 不新增归档相关 route。

### Frontend

- `web/src/protocol/terminal.ts`
- `web/src/features/sessions/api.ts`
- `web/src/components/workspace/WorkspaceSessionSidebar.vue`
- `web/src/views/SessionsView.vue`
- `web/src/i18n.ts`
  - 增加 rerun API、按钮、handler、文案。
  - UI 状态判断收敛为 running/stopped/failed。
  - 不增加归档 UI。

## Interfaces

### Rerun HTTP

```http
POST /api/devices/:deviceId/sessions/:sessionId/rerun
Content-Type: application/json

{
  "cols": 120,
  "rows": 32
}
```

成功响应：

```json
{
  "session_id": "same-session-id",
  "workspace_id": "workspace-id",
  "state": "running",
  "ws_url": "/api/sessions/same-session-id/ws"
}
```

失败响应必须让前端可识别 failed 状态；如果沿用错误响应体，应同时保证随后 `get_session` 返回 failed。

### Agent tunnel methods

- `rerun_session`

不新增归档相关 methods。

### Frontend API

```ts
export type RerunSessionRequest = {
  cols: number
  rows: number
}
```

## Technical questions

1. create failure 如何保证前端拿到 failed：Plan 需要实现 create 失败后返回明确错误并保证 `get_session` 可返回 failed，或 create 响应直接包含 failed summary。
2. `SaveState` / `SaveProcess` / `SaveExit` 是否保留方法名作为内部 facade：Plan 倾向先保留方法名但改写内部实现，避免一次性扩大调用方改动；不保留对应碎片文件。

## Risks

1. 收敛状态集合会触及后端、前端、测试多个位置，可能暴露旧 starting/stopping 依赖。
2. create failure 固定 failed 可能要求调整现有 create 错误路径，避免“创建失败但前端没有 session 状态”的空洞。
3. 复用同一个 session id 会让当前列表只呈现最新运行状态；旧运行输出只能在本地文件系统通过归档 history 追溯。
4. 归档不走云端 / API，远程 UI 不展示旧归档输出；这是本轮明确范围。
5. 离线设备不能 rerun。
6. `workspace.json` 变成更高频写入文件，必须保证原子替换和错误处理清晰。
7. 移除碎片文件事实源会影响现有测试和 recovery 逻辑，需要按新结构重写断言。
8. 不迁移旧碎片文件会让旧数据在新代码路径下不可见；这是用户明确接受的边界。

## Alternatives

### A. 新建 session id

优点：历史天然隔离，状态机简单。
缺点：不满足用户明确的“复用 id / 复用 session”。已拒绝。

### B. 清空旧 history/exit/process

优点：实现简单。
缺点：丢失旧输出和退出记录。已拒绝，改为 history 文件重命名 + workspace 内部 archived run metadata。

### C. 暴露归档 API / UI

优点：远程 UI 可查看多次运行历史。
缺点：超出当前意图；用户已明确归档只是本地整理，且不需要云端知晓。已拒绝。

### D. 继续保留 session/state/process/exit 多文件结构

优点：改动小。
缺点：与用户追加要求相反；当前 `workspace.json` 已含 session 大部分信息，继续碎片化会扩大一致性风险。已拒绝。

### E. 迁移旧碎片文件到 workspace.json

优点：旧数据保留。
缺点：用户已明确“不向后兼容，不迁移已有文件”。已拒绝。

## User review notes

- 用户要求切换严格模式 / strict。
- 用户要求状态机相关纳入文档。
- 用户确认 create failure 固定 failed，且要返回明确状态。
- 用户确认不进行历史兼容。
- 用户确认归档是文件整理，不需要归档接口。
- 用户确认归档不需要云端知晓。
- 用户追加：`workspace.json` 内部有 session 大部分信息，合并收拾 `process.json` / `session.json` / `state.json`，重新组织结构并在这里 review，不需要这么碎片化的存储。
- 用户强调：这个存储设计在 Plan 阶段就要规划好。
- 用户确认：不向后兼容，不迁移已有文件，`exit.json` 也要收拾。
- 本 Spec 因新增存储重组范围重新置为 Draft，待用户 review。
