# 工作区文件管理与 Git 差异查看规格
最后修改时间: 2026-07-15 10:17:55

Review status: Draft

## Requirement basis

本规格依据 [`docs/requirement/20260714-shared-workspace-text-view.md`](../requirement/20260714-shared-workspace-text-view.md) 的工作区/会话背景，以及用户在 2026-07-15 确认的实现方向：

- 使用 **Vue 自定义目录树 + Monaco Editor + Agent File API**。
- 文件工作台从 workspace 行操作区显式打开，覆盖现有右侧会话区域；关闭后恢复 `SessionWorkbench`、session tabs 与后台运行的 terminal session。
- 实现完整目录和文件管理：创建文件/目录、读取、编辑、保存、重命名、同 workspace 内移动、删除和显式覆盖。
- 引入最小的只读 Git API，并使用 Monaco `DiffEditor` 查看 Git 差异。
- 不实现 extension、终端、任务、调试、Git 写操作、变更监听、轮询、自动刷新或自动合并。
- local 与 cloud 共用 typed contract；Cloud 经现有 Agent tunnel 转发文件/Git 请求，不新增通用 HTTP/WebSocket relay，也不缓存文件或 Git 内容。

此前 code-server、OpenVSCode Server、`codingame/monaco-vscode-api` full workbench 只作为 alternatives 保留，不属于本期实现。

## Overview

```text
Workspace action: Open files
  └─ Vue file workbench overlay
       ├─ custom lazy directory tree + context actions
       ├─ custom document tabs / dirty state
       ├─ Monaco Editor
       └─ Git changes panel + Monaco DiffEditor
            │
            ├─ Local: Browser → Agent File/Git API → Workspace.Path root
            └─ Cloud: Browser → Cloud device route → typed Agent tunnel
                                           → Agent File/Git API → Workspace.Path root
```

`Workspace.Path` 是 Agent 服务端加载的内部根目录，浏览器只传 `workspaceId` 与 `/` 分隔的逻辑 relative path。现有 workspace/session tree 继续只表示 workspace → session，不混入真实文件节点。

## Scope and non-goals

### Included

1. 独立文件工作台、目录懒加载、文件 tabs、dirty 状态和 Monaco 文本编辑。
2. 完整单项文件管理：创建文件、创建目录、编辑/保存已有文件、重命名、同 workspace move、删除空目录、显式确认递归删除、显式确认覆盖。
3. 文本读取/写入版本前置条件、冲突处理、打开 dirty 文档保护和 Monaco model 生命周期管理。
4. workspace 级 Git status 及文件级 staged、unstaged、untracked 文本 diff。
5. Monaco `DiffEditor` 只读呈现 Git original/modified 内容。
6. local/cloud typed API、Cloud device ownership 最小校验、Agent tunnel 传输及完整回归测试。
7. 用户显式触发的目录刷新、Git 刷新和 diff 请求。

### Excluded

- `@codingame/monaco-vscode-*` service override、VS Code Workbench、FileSystemProvider、extension/extension host。
- code-server、OpenVSCode Server、IDE process、iframe、IDE proxy。
- 终端、任务、调试、DAP、任何 terminal/session 生命周期变更。
- Git add/restore/stage/unstage/commit/amend/branch/merge/rebase/stash/push/pull/fetch/clone、历史、blame 或远端状态。
- 文件/目录变更监听、fsnotify、浏览器 polling、WebSocket/SSE 推送、自动刷新、后台预取和自动合并。
- 跨 workspace/device 的复制或移动、批量操作、回收站、撤销/重做、上传/下载、二进制预览、全文搜索、协同编辑和共享 ACL。

## File and directory contract

新增 `proto/termbridge/agent/v1/file.proto`，并在 `proto/termbridge/shared/v1/tunnel.proto` 加对应 typed request/response payload。必须使用既有 Go/TypeScript protobuf 生成链路，禁止手写 `web/src/gen/proto` 产物。

### Core types

| Type | Required fields / meaning |
|---|---|
| `FileEntry` | `path`、`name`、`kind`（file/directory）、`size`、`modified_at`、opaque `revision`、可选 `has_children`。 |
| `FileList` | 直接 children、`truncated`；不递归返回整棵树。 |
| `FileConflict` | `type`（revision mismatch/already exists/type mismatch）、当前 entry/current revision。 |
| `MutationResult` | source/destination entry、affected count、受影响路径前缀等明确结果。 |

`path` 是相对 workspace root 的逻辑路径，使用 `/`；root 只在 list 操作中允许空 path。客户端不得解析或生成 `revision`。

### Operations

| Operation | Request core | Required semantics |
|---|---|---|
| List directory | `workspace_id`, `path` | 返回目录直接 children，目录优先稳定排序；按需展开。 |
| Read text | `workspace_id`, `path` | 返回 UTF-8 text、entry、revision；目录/二进制/超限返回 typed unavailable。 |
| Create file | `workspace_id`, `path`, `text`, `overwrite`, destination revision | 父目录必须存在；默认拒绝重名；显式覆盖仅限普通文本文件。 |
| Create directory | `workspace_id`, `path`, `allow_existing` | 父目录必须存在；不隐式创建多级父目录。 |
| Write text | `workspace_id`, `path`, `text`, `expected_revision`, `force` | 写已有文本文件；revision 必填；force 必须来自二次确认。 |
| Rename | `workspace_id`, `path`, `new_name`, source/destination revisions | 只在同父目录改名；`new_name` 只能是一段名称。 |
| Move | `workspace_id`, `source_path`, `destination_path`, source/destination revisions | 只允许同 workspace；目标父目录必须存在；禁止移动目录到自身后代。 |
| Delete | `workspace_id`, `path`, `expected_revision`, `recursive` | root 不可删除；删除非空目录需 `recursive=true` 和 UI 明确确认。 |

推荐 Local HTTP 资源后缀：

```text
GET  /api/workspaces/{workspaceId}/files/tree?path=
GET  /api/workspaces/{workspaceId}/files/content?path=
POST /api/workspaces/{workspaceId}/files
POST /api/workspaces/{workspaceId}/directories
PUT  /api/workspaces/{workspaceId}/files/content
POST /api/workspaces/{workspaceId}/entries:rename
POST /api/workspaces/{workspaceId}/entries:move
DELETE /api/workspaces/{workspaceId}/entries?path=&expected_revision=&recursive=
```

Cloud 沿用：`/api/devices/{deviceId}/workspaces/{workspaceId}/...`。

### Text and revision rules

- Monaco 仅打开 UTF-8 普通文本；空文件合法。
- 无效 UTF-8、二进制或超过读取/写入上限的文件可在 tree 中显示，但不可创建 Monaco model。
- 大小、目录项数、递归删除预检数量、操作 timeout 均由集中 `file.Config` 提供，不在 handler 内硬编码。
- `revision` 是服务端 opaque token；读取、保存、rename、move、delete、overwrite 都在服务端重新验证对应 revision。
- 写入与覆盖在同一受限 root 中以原子替换完成；冲突绝不静默覆盖磁盘内容。

## Agent file service and root boundary

新增独立 file domain/application/infrastructure，不将文件 I/O 塞入 terminal registry：

```text
internal/agent/model/task/file/
internal/agent/application/task/file/
internal/agent/infrastructure/storage/workspacefile/
```

Agent File Service 按 `workspaceId` 从当前 device state store 加载持久化 `Workspace.Path`，再通过 root-confined filesystem adapter 操作真实文件系统。

以下是本期不可后置的正确性边界：

1. 只接受逻辑相对路径；拒绝 absolute path、盘符、UNC、NUL、`.`、`..`、空 path segment、反斜杠歧义及规范化后语义改变的输入。
2. workspace root 必须存在、是目录且自身不作为 destructive target；`path=""` 只能用于列 root。
3. 使用 root-confined 文件操作防御 TOCTOU、symlink、Windows junction/reparse point 逃逸；不得以 `filepath.Join`、`Clean`、`Rel` 或字符串前缀比较作为唯一保护。
4. 路径及其父路径包含 link/reparse point 时 fail closed；普通文件和目录以外的特殊节点不可读写或递归。
5. 目录 move/rename/delete 不得影响 workspace metadata，也不触发现有 `DeleteWorkspace` 语义。
6. 错误与日志不得泄露 Agent absolute path、链接目标或文件正文。

多用户 workspace ACL、审计、配额、回收站和协作可后置；但 Cloud 文件/Git 请求最少必须验证当前 user 拥有目标 device。

## File workbench UX

### State and components

新增可序列化 Pinia `fileWorkbench` state：

- runtime target + active workspace；
- 按 workspace/path 缓存的目录 children 与展开状态；
- opened documents、active document、base revision、draft/original text、dirty/loading/saving/conflict/error；
- 选择的 Git change/layer、Git status 与 diff loading/error；
- 不存 Monaco editor/model 实例。

建议组件：

```text
WorkspaceFileDrawer.vue     // right-pane overlay root
WorkspaceFileTree.vue       // lazy tree + file actions
WorkspaceEditorTabs.vue     // open docs / dirty / close guard
WorkspaceTextEditor.vue     // Monaco Editor
GitChangesPanel.vue         // explicit-refresh status list
GitDiffEditor.vue           // Monaco DiffEditor, read-only
```

`WorkspaceSessionSidebar.vue` 在 workspace 操作区增加 `openFiles` event，使用 `click.stop` 保持原有行点击展开/收起。`SessionsPageShell.vue` 在右侧 panel 内挂载 overlay；关闭 Drawer 后恢复触发入口焦点，且不销毁现有 terminal workbench/session state。

### Mutation and dirty protections

- Tree context menu/hover action 必须提供键盘可达入口：新建文件、目录、重命名、移动、删除。
- destructive/overwrite dialog 必须展示 canonical relative path、覆盖/递归影响及不可撤销提示。
- mutation 以服务端 canonical response 完成后才局部更新 tree；失败时保留旧 tree、旧文档及草稿，不做永久乐观更新。
- dirty 文档改变时：

| Operation | Clean documents | Dirty documents |
|---|---|---|
| overwrite | 重新读取或关闭 | 阻止，要求保存/放弃草稿/取消 |
| delete file | 关闭 tab / dispose model | 阻止 |
| delete directory | 关闭受影响 clean tabs | 任一 dirty 后代都阻止 |
| rename/move file | 更新 canonical path / model URI | 需明确选择移动保留草稿、先保存或取消 |
| rename/move directory | 批量更新 clean children path | dirty 子文档需明确处理 |

放弃草稿只丢弃浏览器 buffer，不是覆盖磁盘授权。

### Monaco Editor

只添加 `monaco-editor`：

- 配置 Vite worker，验证 development 与 production build；
- 按扩展名映射 language ID，未知为 plaintext；
- 跟随现有 light/dark theme；
- 创建、切换、关闭/卸载时 dispose editor/model/listener；
- `Ctrl+S`/`Cmd+S` 只在 Monaco 聚焦且当前文档 dirty 时执行显式 save；
- 保存成功更新 base revision 与 original text；
- `409 revision_conflict` 后保留草稿，禁用普通 save，并提供重新读取（丢弃草稿需确认）、复制草稿、强制覆盖（二次确认）和取消。

没有 watcher 的情况下，外部文件变化不主动刷新 tree/editor；用户下一次显式 read、refresh 或 save 时才观察到更新/冲突。

## Git status and diff

Git 是只读、显式刷新能力，独立于文件写操作和 terminal。

### Contract

建议在 `agent/v1` workspace 或独立 `git.proto` 定义，且在 tunnel 添加匹配 frame：

| Operation | Request | Response |
|---|---|---|
| Git status | `workspace_id` | `state`、`changes`、安全 message |
| Git diff | `workspace_id`、`path`、`layer` | `state`、original/modified path、original/modified text、message |

`GitChange` 包含 `path`、rename/copy 时 `original_path`、index status、worktree status、unmerged、untracked 和 available layers。

`layer` 是明确 enum：

- `staged`：`HEAD → index`；
- `unstaged`：`index → worktree`；
- `untracked`：空文本 → worktree。

对同时 staged/unstaged 的 `MM` 文件，前端提供两个层选择，而不是把 `HEAD → worktree` 伪装为任一层。

Git routes：

```text
GET /api/workspaces/{workspaceId}/git/status
GET /api/workspaces/{workspaceId}/git/diff?path=&layer=

GET /api/devices/{deviceId}/workspaces/{workspaceId}/git/status
GET /api/devices/{deviceId}/workspaces/{workspaceId}/git/diff?path=&layer=
```

### Git service

新增 `internal/agent/application/task/git/`：

- 使用 `exec.CommandContext`，不经 shell、cmd、PowerShell 或 PTY；`cmd.Dir` 为 Agent 解析出的 workspace root。
- executable 来自集中 `git.Config`（默认 `git`），用 `exec.LookPath` 解析；Windows 不手动拼 `.exe`。
- 固定独立 argv、`--no-ext-diff`、`--` path terminator、`GIT_OPTIONAL_LOCKS=0`、timeout、stdout/stderr 字节上限。
- 不记录 diff 正文、原始 stderr、环境变量或绝对 repository path。
- 预检 `git rev-parse --is-inside-work-tree`；status 用 `git status --porcelain=v1 -z --untracked-files=all`，按 NUL 格式解析。
- staged 内容：`HEAD:path` 与 `:path`；unstaged：`:path` 与受限 worktree read；untracked：空文本与 worktree read。
- 二进制、无效 UTF-8、超限、unmerged、submodule、symlink、非仓库、Git 缺失均返回 typed unavailable state，不将 CLI stderr 交给浏览器。

最小 status 支持 staged/unstaged modified、added、deleted、renamed、copied、untracked 和 unmerged；ignored 不显示。Git API 不缓存，不做离线回退。

### DiffEditor UX

- 仅打开 Git 面板、点击 Refresh、选择 change 或切换 layer 时请求 Git API；无 timer、watch、poll 或 focus/network 自动 refresh。
- `GitChangesPanel` 显示 status，并让 `MM` 项选择 staged/unstaged。
- `GitDiffEditor` 用 `monaco.editor.createDiffEditor`，两个显式 model，`originalEditable=false`、`readOnly=true`。
- add/delete 一侧为空；rename 使用 original/new path label；binary/too_large/unmerged/not_repository/git_unavailable 显示 inline unavailable state，不创建空的可编辑 diff。
- 用 request sequence 或 `AbortController` 防止旧 diff 覆盖新选择；切 workspace/关闭 Drawer 时忽略或取消 in-flight 请求。

## Local/cloud transport

### Agent runtime

`RuntimeAccess` 组合独立 File Service 与 Git Service。Agent bootstrap 构造 `file.Config`、`git.Config` 和服务；state store 仅用于 workspace metadata/root lookup。

Agent local handler、runtime endpoint、Agent cloud client、request dispatcher、response mapper、runtime request/response 判别都新增 File + Git typed operations。文件/Git 内容不得走 terminal WebSocket 或 history API。

### Cloud runtime

Cloud device workspace route 新增 files/content/entries/directories/git 分支，转发现有 typed Agent tunnel。每个文件/Git请求在建立 tunnel endpoint 前用当前 claims user ID 执行 `UserOwnsDevice(deviceId)` 或等效验证；失败不可枚举且不触发 Agent 请求。

Cloud 不缓存 directory、file text、Git status 或 diff；device offline 返回 `device_offline`。预期业务 conflict/unsupported state 不得被 Cloud 泛化为 `502 upstream_error`。

## Error contract

继续使用现有结构化 `ErrorResp`，保留 request ID；details 是 typed、安全的诊断，不泄露绝对路径或内容。稳定 codes 至少覆盖：

```text
invalid_file_path          invalid_operation
workspace_not_found        workspace_root_unavailable
path_outside_workspace     symlink_unsupported
file_not_found             file_not_text
file_too_large             already_exists
revision_conflict          type_mismatch
directory_not_empty        root_protected
file_busy                  device_offline
```

Git unavailable state 优先作为成功 response 中的 typed state（not repository、git unavailable、binary、too large、unmerged、layer unavailable），transport/tunnel/internal failure 才走结构化 error。

## Delivery tasks

### G1 — Proto, generated types and error taxonomy

- 新增 file 与 Git typed messages，扩展 tunnel oneof；保持字段号稳定。
- 确认/扩展 Go/TS generation entry，禁止人工编辑 generated code。
- 定义 revision、mutation result、Git status/diff layer/state 与安全 error details。

### G2 — Agent root-confined file service

- 实现 root resolver、relative path validator、link/reparse guard、list/read/create/write/rename/move/delete。
- 实现文本/大小限制、revision 与原子 mutation。
- 覆盖完整文件管理、root protection、TOCTOU/link、recursive delete 预检、Windows 路径及错误脱敏测试。

### G3 — Agent Git read-only service

- 实现集中的 git config、safe process runner、NUL porcelain parser、status/diff layer 内容读取。
- 覆盖 staged/unstaged/MM/add/delete/rename/untracked/unmerged、UTF-8/binary/size、Git unavailable/timeout 与 Windows command lookup。

### G4 — Agent API, RuntimeAccess and typed tunnel

- 将 File/Git services 注入 bootstrap、RuntimeAccess 与 agent dispatcher。
- 增加 local routes、local frame factory、cloud client mapping、response JSON mapping。
- 保证业务 conflict/error/state 经 tunnel 保留语义，回归 workspace/session/history/terminal。

### G5 — Cloud route and ownership gate

- 加 device-scoped files/entries/directories/git routes。
- 在 File/Git route 前做 `UserOwnsDevice`；无 owner 不触发 tunnel。
- 禁用 offline cache，测试 owned/unowned/offline/tunnel response。

### G6 — Web API, store and Monaco workbench

- 新增 FileRuntimeApi、GitRuntimeApi、local/cloud adapters 和 Pinia fileWorkbench store。
- 增加文件 tree/actions/dialogs/tabs/Monaco editor/Git panel/DiffEditor。
- 实现 dirty guards、局部 tree refresh、revision conflict、explicit Git refresh，以及 Monaco resource disposal。
- 新增 i18n、a11y、Vite worker/theme 及 component/store tests。

## Verification plan

### Backend and protocol

- protobuf lint/breaking 和实际 Go/TS generation；
- File Service：路径、root/link、create/write/overwrite/rename/move/delete/recursive/dirty-related server conflicts、revision 和 atomicity；
- Git Service：NUL porcelain、所有支持状态、diff direction、timeout/binary/nonrepo；
- Agent/local/cloud/tunnel：route mapping、request ID、owned/unowned/offline、typed errors/states 和无 cache；
- 相关 Go package 测试后执行项目约定的全量回归。

### Frontend and browser

- Vitest：URL/adapter、tree 局部 mutation、document dirty/revision/conflict、Git explicit refresh/layer/race；
- typecheck、lint、format、production build；验证 Monaco worker；
- 浏览器：Local 与 Cloud owned+online 的创建、移动、rename、覆盖、递归 delete、文本保存、冲突、Git status/diff；验证 unowned/offline 拒绝；验证无自动刷新且关闭 Drawer 后 terminal tabs/xterm 不回归。

## Backend delivery evidence — 2026-07-15

The backend-only G1–G5 scope is implemented. The browser workbench remains deferred to G6.

- Added generated File/Git protobuf contracts and typed tunnel frames; tunnel protocol is versioned at `3` so older Agents cannot receive unsupported File/Git requests.
- Added Agent workspace-root-confined File service and storage adapter with logical-path validation, root and intermediate link/special-entry rejection, opaque revisions, bounded UTF-8 text operations, and explicit recursive deletion.
- Added read-only Git status/diff service using fixed `exec.CommandContext` arguments. Nested workspaces expose only workspace-relative repository changes; Git object reads are translated back to the validated repository-relative prefix.
- Added Agent local routes, runtime dispatch, Cloud forwarding, Cloud ownership gating before tunnel lookup, offline behavior without File/Git cache fallback, and preservation of safe File conflict details across the tunnel.
- Reviewed containment, protocol compatibility, error propagation, and Git semantics; applied the verified findings before final verification.

Verification completed:

```text
go test ./...
# PASS
```

Focused checks also passed for the File store, Git adapter, Agent/Cloud handlers, runtime client, and shared configuration. `git diff --check` passed. No frontend, Monaco, or File workbench implementation was included in this delivery.

## Risks and deferred work

- 目录/文件完整管理的 destructive 操作必须以服务端 root boundary、revision 和前端 dirty guard 三层共同保证；任一层缺失都会形成数据丢失风险。
- 无 watcher 下，状态可能在用户显式刷新前陈旧；本期接受该行为，以保存/结构操作的 revision conflict 防止静默覆盖。
- Git status 与 diff 并非原子快照；本期依赖显式 refresh，不引入 Git lock/watch/snapshot service。
- Git 不提供跨 workspace 可见性；若 workspace 是 repository 子目录，Agent 必须限制返回 path 仍在 workspace policy 范围内。
- Git 写操作、自动刷新、搜索、协作、二进制/下载与共享 ACL 均需单独需求，禁止在本期顺带扩张。

## Alternatives

### Monaco VSCode API full workbench

延后。它可提供 VS Code Explorer/SCM/command UI，但会引入 FileSystemProvider、`termbridge://` URI、workbench initialization、worker/asset pipeline、版本锁定和扩展生态；当前不做 extension/terminal，其复杂度高于 Vue 自定义 UI。

### code-server / OpenVSCode Server

拒绝本期。它们带来 IDE process lifecycle、token、代理/iframe/独立窗口、Windows 发布验证和完整 terminal/extension runtime，和“文件管理 + 只读 Git diff”不匹配。

## User review notes

- 2026-07-15：用户确认 Vue 自定义目录树、Monaco Editor、Agent File API，并要求重新分析任务。
- 2026-07-15：用户确认不做扩展或终端。
- 2026-07-15：用户要求完整目录和文件管理，并确认引入基本 Git API 与 Monaco DiffEditor 的差异查看；不引入变更监听或自动刷新。
