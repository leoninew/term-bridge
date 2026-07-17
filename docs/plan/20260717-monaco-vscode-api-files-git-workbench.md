# Plan / 计划：基于 monaco-vscode-api 的文件与 Git 工作台

最后修改时间: 2026-07-17 19:30:00

Review status: Draft

## Flow mode / Stage

标准模式 / standard；计划 / Plan（Draft）。Requirement 为 Draft 时本计划仅作实施基线草案；Requirement Accepted 且用户批准计划后进入 Implementation。

## Requirement basis

- Requirement：[`docs/requirement/20260717-monaco-vscode-api-files-git-workbench.md`](../requirement/20260717-monaco-vscode-api-files-git-workbench.md)
- 评估：[`docs/analyze/20260717-monaco-vscode-api-files-git-evaluation.md`](../analyze/20260717-monaco-vscode-api-files-git-evaluation.md)
- 后端能力：可保留 `workspacefile` / `gitexec` 等**实现内核**，但对外 API（路径、proto、DTO）按 FileSystemProvider / SCM 接口**重定义**；不保留旧 API 适配层

## Implementation approach

```text
Vue 壳（鉴权 / sessions / device·workspace 上下文）
  -> 懒加载 `/workspaces/:workspaceId/code`（cloud 带 device 前缀）
       -> @codingame/monaco-vscode-api workbench initialize（进程内一次）
       -> TermBridge FileSystemProvider  --HTTP/WS--> Agent/Cloud FS
       -> TermBridge SCM Provider        --HTTP-----> Agent/Cloud Git
  -> 既有 xterm Session 保持独立
```

原则：

1. **先 Spike 后拆旧**：Phase 0 未通过前不删除现有 Vue 文件/Git 主路径。
2. **契约源 = VS Code Provider 接口**：以 `FileSystemProvider` 与 SCM Provider（Git）的方法/类型/错误语义为唯一对外契约；先锁定该契约再改 Agent/Cloud/前端。
3. **改造而非桥接**：旧 file/git 路由、proto、DTO、前端 runtime **直接改成**契约形态（含路径与字段）；禁止长期 adapter（旧 shape → Provider shape）。
4. **单一 Workbench 实例**：切换 workspace 用 rebind root + 取消在途请求；失败则整页 reload。
5. **安全底线保留在 Agent**：workspace root 解析、路径规范化、不回传本机绝对路径/秘密；不因改用 SCM UI 而暴露 generic git argv。

## Implementation steps

### 0. Spike（门禁，3–5 天）

1. 在 `web` 中引入 `@codingame/monaco-vscode-api`、editor-api 别名与最小 workbench/explorer/scm/theme 依赖。
2. 新建 `/workspaces/:workspaceId/code`（及 cloud 对称路径）全屏 Workbench 页：`initialize` 一次、基础 worker/CSS 配置；Spike 阶段可先挂路由再完善鉴权。
3. 用内存或只读假 FS 验证 Explorer 打开文件；用假 SCM 验证变更列表与打开 diff。
4. 再对接**一个** local workspace 的真实 `stat/readdir/read`（应用**新** FS 契约的最小实现，不经旧 list/content 翻译层）验证延迟可接受。
5. 记录：构建配置改动、初始 chunk 体积、首屏时间、workspace 切换候选策略、阻塞问题。
6. **Go/No-Go**：不通过则停止，保留 Vue 实现并回写评估文档。

### 1. 按 FileSystemProvider 重定义 FS 后端

1. 对照 VS Code `FileSystemProvider` / `FileSystemError` 定稿 proto 与 HTTP：操作名与语义对齐 `stat`、`readDirectory`、`readFile`、`writeFile`、`createDirectory`、`delete`、`rename`（及 `watch`）；错误码对齐 Provider 可消费集合。
2. **替换**旧 workspace file 对外 API（路径与 DTO），而不是在其上加兼容 shim；`workspacefile` 存储实现可内改外换。
3. `read/write` 字节语义；冲突策略按 Spec 写入契约本身（非前端二次解释）。
4. 实现 `watch`（优先）或文档化刷新协议；Cloud 对称 route + tunnel + device ownership，**同一数据结构**。
5. Go 测试：CRUD、边界路径、冲突、超大/二进制、越界拒绝——断言新契约字段，不测旧 DTO。

### 2. 前端 FileSystemProvider + FS MVP 路由

1. 实现 `TermBridgeFileSystemProvider`：方法体 essentially 调新 FS API，**无 DTO 翻译表**。
2. Workbench 打开时绑定当前 workspace URI。
3. 接入保存/dirty；错误直接映射 `FileSystemError` / 后端契约码到通知。
4. sessions → `/code`；cloud 对称路径。
5. 测试：Provider 与后端字段同构；无“旧字段别名”层。

### 3. 按 SCM Provider 重定义 Git 后端 + 前端

1. 对照 `vscode.scm`（SourceControl、ResourceGroup、ResourceState、inputBox、命令）定稿 Git HTTP/proto：**替换**旧 `git.proto` 对外形状，而非 bridge。
2. host 内 `createSourceControl`：groups/states 字段与后端响应同构或仅做 URI 拼装。
3. stage/unstage/discard/commit 等命令的请求体以 SCM 操作为准设计；确认类 UX 用 Workbench dialog。
4. mutation 后 refresh；与 FS watch 联动。
5. 分支能力若进 MVP，同样走新契约。
6. 删除或停用旧 Git client/DTO；测试只覆盖新契约。

### 4. 生命周期、cloud 对称与硬化

1. workspace/device 切换：abort in-flight、清空 models/SCM、rebind 或 reload。
2. 核对 Cloud 全路径与超时/offline 文案。
3. 性能：大目录截断、大文件拒绝、避免无节流 watch 风暴。
4. i18n：Workbench 外的入口/错误；Workbench 内可先用英文/默认语言包（若启用 language pack 需单独评估）。

### 5. 迁移与清理

1. 主入口切到 `/code` 后，移除旧 `/files`、`/git` UI，并**移除旧 file/git API 客户端与并行 schema**（服务端旧路由删除或一次性替换，不做双写双读）。
2. 删除或归档：旧 Vue 组件、`fileWorkbench` 中旧 DTO 状态机、旧 runtime 适配代码。
3. 更新 requirement/analyze 交叉引用与 README/roadmap（若有条目）。
4. Session/xterm 回归：打开工作台再返回 sessions，终端仍可用。

### 6. 实现期检查

1. 相关 Go tests、web test/typecheck/lint/format/build。
2. 手测清单：local 打开/编辑/保存/CRUD、SCM stage/commit、cloud 授权路径、切换 workspace。
3. 不自动 git commit；Verification 文档待用户要求验证阶段再写。

## Critical files

| Area | Representative paths |
| --- | --- |
| 评估/需求/计划 | `docs/analyze/20260717-monaco-vscode-api-files-git-evaluation.md`、`docs/requirement/20260717-monaco-vscode-api-files-git-workbench.md`、本文件 |
| 前端 Workbench 壳 | 新建如 `web/src/features/workbench/*`、`web/src/views/**/WorkspaceCodeView.vue`、`web/src/router/index.ts`（注册 `/workspaces/:workspaceId/code` 与 cloud 对称路径） |
| FS/SCM Provider | 新建如 `web/src/features/workbench/fsProvider.ts`、`scmProvider.ts`、`bootstrap.ts` |
| 前端 HTTP | 新建 `web/src/features/workbench/api.ts`（契约同构 client）；旧 `features/files/runtime.ts` 删除而非 adapter 化 |
| 旧 UI/API（待删） | 旧 Vue 文件/Git 组件、`fileWorkbench` 旧状态、旧 file/git DTO 生成物 |
| 后端 FS | `workspacefile` 实现可留；**对外** proto/HTTP 按 FileSystemProvider 重定义；handler/tunnel 同步替换 |
| 后端 Git | `gitexec` 实现可留；**对外** proto/HTTP 按 SCM/Git 工作台契约重定义 |
| Cloud | `internal/cloud/api/handler/file_git_routes*.go`、tunnel dispatch |
| 构建 | `web/package.json`、`web/vite.config.*` |

## Verification plan

### Spike

- Workbench 可启动；Explorer 可见；至少一次远程 read；SCM 至少静态列表 + 一次 diff。
- 记录 bundle 体积与明显阻塞（CSS/worker/initialize）。

### FS

- real workspace：readdir/stat/read/write/mkdir/rename/delete；越界 path 拒绝；冲突可复现；too-large/binary 策略符合契约。
- watch 或刷新策略下，外部改文件后树可恢复一致。

### Git

- real temporary git repo：status 分组、diff、stage/unstage、discard 确认、commit、失败态（not repository、unavailable）。
- 保存文件后 SCM refresh。

### 产品路径

- local 与 cloud 入口、返回 sessions、切换 workspace/device 无串态。
- 旧主路径已移除或不再可达。
- Session 终端回归。

### 工程

- 约定范围内 `go test`、`yarn --cwd web test|typecheck|lint|format|build`；失败如实录入。

## Risks and rollback

| 风险 | 缓解 |
|------|------|
| Spike 失败 | 不删旧实现；文档标记 No-Go |
| 契约源漂移 | 以 VS Code Provider 接口文档/类型为源，禁止另起“TermBridge 专用中间 DTO” |
| 偷偷做 bridge | Code review 拒绝“旧字段 rename 层”；新旧 API 不双轨长期并存 |
| initialize 一次 | 禁止多实例；切换策略写测试 |
| 双实现 | Phase 3 完成定义含删除旧入口与旧 schema |
| proto/tunnel 不对称 | local/cloud 同一契约表 |

回滚：还原本特征相关前后端与依赖变更；保留评估文档。不对用户仓库做破坏性 git 操作。

## Suggested sequencing (calendar)

| 周 | 焦点 |
|----|------|
| W0 | Phase 0 Spike + Go/No-Go |
| W1–W2 | 按 FileSystemProvider 重定义 FS API + Agent/Cloud + 薄 Provider |
| W3 | CRUD/冲突/watch + `/code` 产品化；开始拆除旧 file API |
| W4 | 按 SCM 重定义 Git API + stage/commit/refresh；拆除旧 git DTO |
| W5 | 分支（若纳入）、cloud 硬化、切换生命周期 |
| W6 | 删旧 UI、回归、文档收口 |

（单人约 5–8 周；双人可压缩 Spike 后并行 FS 后端与 Workbench 壳。）

## User review notes

- 2026-07-17：随 Requirement 草案一并起草；待用户审阅 Requirement 后确认 Open questions，再将 Review status 更新为 Accepted 并开始 Phase 0。
- 2026-07-17：路由定为独立全屏 `/workspaces/:workspaceId/code`（cloud：`/devices/:deviceId/workspaces/:workspaceId/code`）。
- 2026-07-17：契约以 FileSystemProvider / SCM Provider 为准；旧实现直接改造，不做适配/桥接。




## Implementation status (2026-07-17)

- Backend FS/SCM wire contract live: /fs/*, /scm/*, tunnel frames, proto regenerated.
- Frontend Phase 0 path started:
  - Routes: local /workspaces/:workspaceId/code, cloud /devices/:deviceId/workspaces/:workspaceId/code
  - Sessions Files/Git entry navigates to /code
  - Legacy /files /git views redirect to /code
  - @codingame/monaco-vscode-api workbench bootstrap + TermBridgeFileSystemProvider + SCM provider
  - Client: web/src/features/workbench/* (no old DTO bridge)
  - Legacy Vue Explorer/SCM chrome removed (broken against new protos)
- Spike remaining: runtime smoke on real Agent workspace (Explorer readdir/read, SCM list/diff/commit)

