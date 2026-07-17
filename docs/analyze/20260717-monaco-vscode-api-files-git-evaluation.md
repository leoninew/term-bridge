# 基于 monaco-vscode-api 的文件与 Git 前端评估

最后修改时间: 2026-07-17 19:30:00

> 本文记录将 workspace 目录视图（`/files`）与 Git 视图（`/git`）从前端自研 Vue + Monaco 实现，切换为 `@codingame/monaco-vscode-api` Workbench 路线的技术评估与接口契约。  
> 结论面向当前产品判断：**继续自研 Explorer/SCM chrome 的打磨成本超出预期**；在可按 Workbench 需要提供后端 API 的前提下，采用 monaco-vscode-api 重做文件与 Git 前端是更可交付的路线。

## 1. 背景与问题

### 1.1 现状

TermBridge 已具备 workspace-scoped 文件与 Git 能力骨架：

| 层 | 现状 |
|----|------|
| 路由 | local `/workspaces/:workspaceId/files`、`/git`；cloud `/devices/:deviceId/workspaces/:workspaceId/files`、`/git` |
| 前端 store | `web/src/store/fileWorkbench.ts`（目录懒加载、文档 dirty/conflict、Git status/diff/summary、mutation） |
| 前端 runtime | `web/src/features/files/runtime.ts`（local/cloud HTTP 适配） |
| UI | `WorkspaceFileTree`、`WorkspaceFileWorkbench`、`GitChangesPanel`、`GitDiffEditor`、Monaco standalone |
| 后端 | Agent workspace file API + `gitexec` + Cloud 转发 |

编辑器底座是官方 `monaco-editor` standalone，**不是** VS Code Workbench。目录树、标签栏、Git 面板、状态刷新与冲突语义均由 Vue 自研维护。

### 1.2 痛点

1. 自研 Explorer + 编辑器 chrome + SCM 面板接近“迷你 IDE”，UI/交互长尾（树、多选、拖拽、tab、diff、分支、确认分级）打磨时间远超预期。
2. 视觉对齐 VS Code（见 `docs/requirement/20260716-vscode-workbench-styling.md`）只能缓解观感，不能消除状态机与交互完备性成本。
3. 若继续按组件堆叠补齐 VS Code 级文件/Git 体验，完成度容易长期停在“能用但不像 IDE / 不像完成品”。

### 1.3 评估对象

- 库：[CodinGame/monaco-vscode-api](https://github.com/CodinGame/monaco-vscode-api)（MIT）
- npm：`@codingame/monaco-vscode-api` 及 service-override 分包
- 能力本质：把 VS Code 服务层/Workbench 贡献点拆成可在浏览器集成的包，而不仅是 Monaco 文本控件

---

## 2. 决策前提

本评估接受以下前提（与早期“严格受控 Git / 禁止 IDE 化”的部分需求表述不同，以后续产品判断为准）：

1. **“受控 Git”不再作为前端架构硬约束**。后端仍可保留安全实现细节，但不要求前端自建与 VS Code SCM 不等价的专用 UI 语义层。
2. **后端 API 以 monaco-vscode-api 所需 `FileSystemProvider` / SCM Provider 接口为源**；旧路径与数据结构**直接改造成**目标形态，**不做**旧 DTO ↔ Provider 适配/桥接。
3. **前端范围只含文件 + Git**（Explorer、编辑器、SCM）。
4. **明确不做**：扩展市场、任意第三方扩展安装、LSP/语言服务扩展、调试、库自带 Terminal 替换 TermBridge Session、VS Code Server remote agent 作为主路径。

说明：host 侧通过 `registerExtension(..., LocalProcess)` 注册 `FileSystemProvider` / SCM Provider **不属于扩展市场**，是使用 VS Code API 的正规方式；demo 的 SCM 即此模式。

---

## 3. 结论

### 3.1 主结论

**建议采用 `@codingame/monaco-vscode-api` 作为文件与 Git 前端工作台底座**，路线为：

```text
纯浏览器 Workbench
  + 自定义 FileSystemProvider（对接 TermBridge Agent/Cloud）
  + 自定义 SCM Provider（对接 TermBridge Git API）
```

不建议继续把主要工期投入自研 `WorkspaceFileTree` / `GitChangesPanel` 等 IDE chrome。

### 3.2 不建议的路线

| 路线 | 原因 |
|------|------|
| 继续全面自研 Vue Explorer + SCM | UI 长尾不可控，与“像 VS Code 工作台”目标不匹配 |
| monaco-vscode-api + 官方 Git 扩展 + Node extension host | 与“不做扩展”冲突，部署与 Windows/Agent 复杂度高 |
| monaco-vscode-api remote agent + VS Code Server | 接近 code-server/完整 IDE 产品线，版本锁与进程运维成本高，超出本评估范围 |
| 仅升级 Monaco 服务（theme/textmate）但不换 Explorer/SCM | 对完成度帮助有限，无法解决主痛点 |

### 3.3 工期结构变化

| 维度 | 自研 Vue + Monaco | monaco-vscode-api Workbench |
|------|-------------------|-----------------------------|
| Explorer / tab / layout / SCM chrome | 长期打磨 | 基本由 Workbench 提供 |
| dirty / 多 tab / 基础命令 | 自研堆叠 | Working copy + Workbench 自带 |
| 远程 FS 语义 / 冲突 / 大文件 | 已有一半，仍需打磨 | 必须实现 VS Code `FileSystemProvider` 契约 |
| Git UI | 已有 panel，体验长尾 | SCM 视图免费；后端按 SCM 重定义 API，前端薄 Provider |
| Vue 集成 | 自然 | Workbench 占页、全局 initialize 一次 |
| 包体 | 小 | 明显增大 |
| 测试 | 组件单测友好 | 偏集成 / E2E |

**换底座省的是 UI 与 chrome 状态机；不省远程 FS/Git 做对的时间。**  
工程主体从“做 IDE 皮肤”变为“实现 VS Code 期望的 FS/SCM 后端契约 + 接入 Workbench”。

粗估（1 人，local 优先，不含扩展市场/LSP）：

| 阶段 | 时间 |
|------|------|
| Spike：Workbench 起页 + 内存/假 FS + 假 SCM | 3–5 天 |
| 远程 FS Provider + 保存/树/CRUD | 1.5–2.5 周 |
| SCM 接真 Git API + diff/commit | 1–2 周 |
| 路由/鉴权/cloud/workspace 切换 | 1–2 周 |
| 硬化（大文件、冲突、断线、刷新） | 1–2 周+ |

**可用 MVP：约 5–8 周**（前提是 watch 与 write 冲突语义可交付）。

---

## 4. 目标架构

### 4.1 运行时结构

```text
Browser
  Vue 壳（鉴权、/sessions 入口、device/workspace 上下文）
    -> monaco-vscode-api Workbench（独立全屏路由）
         -> FileSystemProvider  --HTTP/WS--> Agent/Cloud FS API
         -> SCM Provider        --HTTP-----> Agent/Cloud Git API
  既有 xterm Session 工作台保持独立，不使用库自带 Terminal 替换 Session 模型
```

### 4.2 最小 service-override 集合

| 能力 | 包 / 服务 | MVP |
|------|-----------|-----|
| 整页布局 | `@codingame/monaco-vscode-workbench-service-override` | 必须 |
| 文件系统 | files（默认依赖） | 必须 |
| Explorer | `@codingame/monaco-vscode-explorer-service-override` | 必须 |
| SCM | `@codingame/monaco-vscode-scm-service-override` | 必须 |
| Model / dirty / save | model + workbench working copy | 必须 |
| 主题 | theme + theme-defaults default extension | 建议 |
| 配置 / 对话框 / 通知 | configuration / dialogs / notifications | 建议 |
| 快捷键 | keybindings | 建议 |
| 工作区搜索 | search | 二期（无 remote search 时能力弱） |
| Terminal / Debug / Extension Gallery | 对应 override | 不做 |
| Remote agent | remote-agent | 不做（主路径） |

`workbench` 与 standalone `editor` / 部分 `views` 模式互斥；文件+Git 目标应直接选 **Workbench**。

### 4.3 与现有路由的关系

**已决定：**

1. Workbench 使用**独立全屏路由**：
   - local：`/workspaces/:workspaceId/code`
   - cloud：`/devices/:deviceId/workspaces/:workspaceId/code`
2. **不复用**现有 `/files`、`/git` 路径；Git/SCM 只在 `/code` Workbench 内呈现。
3. `/sessions` 仅作为入口，不在 session 页内嵌 Workbench drawer。
4. 旧 Vue `/files`、`/git` 在 `/code` MVP 可用后标记废弃并删除（可短暂 redirect 到 `/code`），避免双实现并行过久。

### 4.4 workspace / device 生命周期

VS Code 服务设计为**进程内 initialize 一次**，难以按 Vue 组件随意卸载。

约束：

1. 同一 Browser tab 内切换 device/workspace 时，优先 **rebind workspace folder + 重置 Provider 根**，必要时整页 reload / sandbox 模式。
2. 不得假设多个并行 Workbench 实例互不干扰。
3. Cloud 请求必须始终携带当前 `deviceId` 与 `workspaceId`，切换后取消或忽略在途请求。

---

## 5. 后端 API 契约

**原则（已决定）：** 契约源是 VS Code / monaco-vscode-api 的 `FileSystemProvider` 与 SCM Provider（Git）接口。Agent/Cloud 对外 API 按该接口的方法、参数、错误与资源模型**直接定义或整体替换旧 API**。前端 Provider 只做 scheme/URI 与传输，**禁止**“旧 `file.proto`/`git.proto` → VS Code 形状”的适配层。

以下为应对齐的能力面（HTTP 路径名以实现时的 Provider 方法为准，不要求兼容旧 `/files/*`、`/git/*` 形状）。

### 5.1 FileSystemProvider

#### 5.1.1 必须方法

| 方法 | 用途 | 备注 |
|------|------|------|
| `stat(path)` | 类型、mtime、size、权限 | 驱动树图标与是否目录 |
| `readDirectory(path)` | 展开目录 | 需稳定排序；超大目录要有上限/截断策略 |
| `readFile(path)` | 打开编辑 | **字节**（`Uint8Array`/base64），不要仅 text |
| `writeFile(path, content, options)` | 保存 | 支持 create / overwrite；冲突可映射 etag/revision |
| `createDirectory(path)` | 新建目录 | |
| `delete(path, { recursive })` | 删除 | 目录递归语义需明确 |
| `rename(oldPath, newPath)` | 重命名/移动 | 跨目录移动常见 |

#### 5.1.2 强烈建议

| 能力 | 原因 |
|------|------|
| `watch`（WS / SSE / 长轮询） | 无变更事件时 Explorer/SCM 只能手动 Refresh，体验会明显回退 |
| 冲突 token（etag / revision / mtime+size） | 对应保存冲突与多端编辑 |
| 大文件策略 | too-large、只读预览或拒绝打开的稳定错误码 |
| 二进制策略 | 编辑器不可预览时的明确错误，而不是损坏文本 |

#### 5.1.3 建议 HTTP 形态（示意）

```text
GET    /api/workspaces/{workspaceId}/fs/stat?path=
GET    /api/workspaces/{workspaceId}/fs/readdir?path=
GET    /api/workspaces/{workspaceId}/fs/read?path=
PUT    /api/workspaces/{workspaceId}/fs/write?path=
POST   /api/workspaces/{workspaceId}/fs/mkdir
DELETE /api/workspaces/{workspaceId}/fs/delete?path=&recursive=
POST   /api/workspaces/{workspaceId}/fs/rename
GET/WS /api/workspaces/{workspaceId}/fs/watch
```

Cloud 侧保持现有 device 前缀模式：

```text
/api/devices/{deviceId}/workspaces/{workspaceId}/fs/...
```

#### 5.1.4 对旧文件 API 的处理

| 旧实现 | 处理 |
|--------|------|
| `agent/v1/file` 文本 read/write、revision、tree list 等 | **整体按 FileSystemProvider 重定义**（字节、`FileStat`、readdir、错误码）；不保留并行旧 schema |
| 无 watch | 在新契约中实现，而非外挂 |
| workspace logical path / root 约束 | 可保留为服务端实现规则，体现在新 API 的 path 语义中 |
| 前端 `runtime.ts` 旧 DTO | 删除；新 client 与契约同构 |

结论：不是“适配旧 API”，而是**改造成 Provider 所需 API**。

### 5.2 SCM Provider（Git）

`scm-service-override` 只提供 **SCM UI 与 API**，不提供真 Git。  
在“不做扩展”前提下，必须用 **自定义 SCM Provider** 填充：

- `vscode.scm.createSourceControl`
- resource groups（如 Staged / Changes / Untracked）
- input box / action button（commit）
- 资源命令（stage / unstage / discard / open diff）
- 可选 quickDiff original content provider

#### 5.2.1 后端能力

| API | SCM 用途 | 现有能力 |
|-----|----------|----------|
| `status` | 填充 resource groups | `git/status` 已接近 |
| `diff` 或 `show(path, side)` | 打开 diff；gutter quickDiff 需 original | `git/diff` 已有 layer 语义 |
| `stage` / `unstage` | 资源操作 | `git/path-mutation` |
| `discard` / `delete untracked` | 危险操作 | `git/path-mutation` |
| `commit(message)` | SCM input | `git/commit` |
| `summary`（branch/history） | 标题与分支 UI | `git/summary` |
| `createBranch` / `switchBranch` | 分支操作 | 已有路由 |
| 变更通知或显式 refresh 点 | 刷新 SCM | 需约定；可与 FS watch 联动 |

#### 5.2.2 实现原则（非映射层）

1. 后端 status/groups 的数据结构按 SCM `ResourceGroup` / `ResourceState` 需要设计（含 path、decorations、命令参数）；**不**先产出旧 `GitChange`+`GitLayer` 再翻译。
2. diff / original content 接口直接服务 Workbench diff 与 quickDiff。
3. mutation 请求体按 SCM 命令设计；成功后返回可直接刷新 SCM 的 status。
4. 一期可不做 remote 与复杂 merge/rebase。

#### 5.2.3 对旧 Git API 的处理

| 旧实现 | 处理 |
|--------|------|
| `git/status|diff|path-mutation|commit|branches` 等 | **按 SCM Provider 能力重定义**路径与 DTO；`gitexec` 内核可留 |
| 旧 layer 枚举与前端 store 形状 | 删除或一次性替换，不做兼容 adapter |
| 前端 Git runtime/store | 新 client 同构消费；旧代码随 `/git` UI 移除 |

---

## 6. 前端集成要点

### 6.1 依赖与别名

典型安装（版本以当时 npm 为准）：

```bash
npm install @codingame/monaco-vscode-api
npm install vscode@npm:@codingame/monaco-vscode-extension-api
npm install monaco-editor@npm:@codingame/monaco-vscode-editor-api
```

并按需安装 workbench / explorer / scm / theme 等 service-override 包。

### 6.2 初始化约束

1. `initialize` 只能调用一次，且须在创建编辑器/Workbench 之前。
2. Worker、CSS（含 shadow-dom 方案）、Vite 配置成本高于当前 `monaco-editor` standalone。
3. 建议 Workbench 容器全屏；可用 shadow-dom 隔离与 TermBridge 全局样式冲突（库标注 beta）。

### 6.3 与 Vue / Pinia 的边界

| 仍由 Vue 负责 | 交给 Workbench |
|---------------|----------------|
| 登录态、cloud/local mode | Explorer |
| `/sessions` 与入口导航 | 编辑器 tab / dirty / save |
| device/workspace 选择上下文 | SCM 视图 |
| 全局 toast 中与门户相关的通知（可选双轨） | 命令面板中的文件/Git 命令（可选） |

`fileWorkbench` store 的 UI 状态机与旧 DTO **整体废弃**；新建与 Provider 契约同构的薄 client，不做错误/字段翻译层（仅 URI 与 transport）。

### 6.4 可删除 / 可保留资产

| 资产 | 建议 |
|------|------|
| `WorkspaceFileTree*.vue` | Workbench MVP 后删除 |
| `WorkspaceFileWorkbench*.vue` / `WorkspaceEditorTabs*.vue` / `WorkspaceTextEditor*.vue` | 删除或降为非主路径 |
| `GitChangesPanel*.vue` / `GitDiffEditor*.vue` / `WorkspaceGitPageShell*.vue` | 删除或降为非主路径 |
| `features/files/monaco.ts` standalone 封装 | 被 editor-api 别名替代 |
| `features/files/runtime.ts` / 旧 proto 客户端 | **删除**；新建同构 client |
| `store/fileWorkbench.ts` | 随旧 UI 删除，不 bridge |
| 后端 `gitexec` / `workspacefile` | **实现可留，对外 API 按 Provider 重定义** |

---

## 7. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| 无 `watch` | 树与 SCM 陈旧 | MVP 必含 watch 或可接受的短周期刷新策略 |
| 全局 initialize | workspace 切换泄漏/错绑 | 单 Workbench、严格 rebind、必要时 reload |
| 包体增大 | 门户首屏变慢 | 路由级懒加载 Workbench chunk；不做 default language 全家桶 |
| VS Code 版本跟车 | 升级成本 | 锁定 monaco-vscode-api 主版本；升级单独排期 |
| 双实现并存 | 维护加倍 | 设废弃截止日期，禁止新功能写入旧 Vue 文件/Git UI |
| 搜索能力弱 | Ctrl+Shift+F 差 | 一期砍全文搜索；二期再做 search provider |
| Windows 路径/大小写 | FS 语义坑 | Provider 与 Agent 统一 logical path 规则 |
| 与 TermBridge Session 混淆 | 用户以为 IDE terminal 即 Session | 产品文案与 UI 入口分离；不用库 Terminal 替换 Session |

---

## 8. 分阶段落地

### Phase 0 — Spike（通过/失败门）

目标：验证技术闭环，不接完整产品壳。

- [ ] Vite 集成 workbench service override，全屏起页
- [ ] 自定义 scheme 的 `FileSystemProvider` 能 readdir/read/write 到**一个** local workspace
- [ ] 自定义 SCM 能显示 status 并打开至少一种 diff
- [ ] 测量初始 JS/CSS 体积与首屏可交互时间
- [ ] 记录 workspace 切换策略（rebind vs reload）

**失败则停**：无法稳定打包 workbench、或 FS 往返延迟使 Explorer 不可用、或无法在不引入扩展市场的情况下完成 SCM diff。

### Phase 1 — FS MVP

- [ ] 定稿 FS HTTP/WS 契约并实现 Agent 侧
- [ ] Cloud 转发与 device 鉴权
- [ ] Provider：树、打开、保存、创建、重命名、删除
- [ ] 冲突与 too-large/binary 错误映射
- [ ] `/workspaces/:workspaceId/code`（及 cloud 对称路径）挂载 Workbench；sessions 入口可打开

### Phase 2 — Git / SCM MVP

- [ ] SCM groups + refresh
- [ ] stage/unstage/discard/commit
- [ ] diff 编辑器
- [ ] 文件保存后标记 SCM stale 并支持 refresh
- [ ] （可选）branch summary / switch

### Phase 3 — 产品化

- [ ] local/cloud 行为一致
- [ ] workspace/device 切换硬化
- [ ] 删除旧 Vue 文件/Git 主路径
- [ ] 基础 E2E：打开文件、保存、stage、commit
- [ ] 文档与 roadmap 同步

### 明确二期

- 工作区全文搜索
- 语法高亮 default language packs（按需）
- multi-diff、timeline、blame
- remote sync（fetch/pull/push）
- 完整 VS Code Server / code-server 路线（若战略需要，单独立项）

---

## 9. 与既有文档的关系

| 文档 | 关系 |
|------|------|
| `docs/requirement/20260714-shared-workspace-text-view.md` | 文件工作台起源；实现底座拟切换 |
| `docs/requirement/20260716-workspace-git-review-view.md` | 独立 `/git` 只读审阅；Workbench 下由 SCM 视图吸收 |
| `docs/requirement/20260717-controlled-git-workflow.md` | 受控 mutation 语义仍可在后端保留；前端不再自建完整 SCM chrome |
| `docs/requirement/20260716-vscode-workbench-styling.md` | 自研样式对齐需求；若采用本路线，该需求大部分被 Workbench 外观替代 |
| 完整 IDE / code-server 类需求 | 仍是另一条产品线；本评估不合并为同一交付 |

若产品确认本路线，应另开 **Requirement / Spec** 将 Phase 0–2 正式立项，并更新 roadmap 中文件与 Git 前端条目。

已起草（Draft）：

- Requirement：[docs/requirement/20260717-monaco-vscode-api-files-git-workbench.md](../requirement/20260717-monaco-vscode-api-files-git-workbench.md)
- Plan：[docs/plan/20260717-monaco-vscode-api-files-git-workbench.md](../plan/20260717-monaco-vscode-api-files-git-workbench.md)

---

## 10. 决策摘要

1. **做**：用 monaco-vscode-api Workbench 重做文件 + Git 前端。
2. **提供**：Agent/Cloud 侧以 FileSystemProvider / SCM Provider 为源**直接定义** FS 与 Git API（含 watch）；旧 API 改造替换，**不桥接**。
3. **不做**：扩展市场、官方 Git 扩展依赖、remote VS Code Server 主路径、用库 Terminal 替换 TermBridge Session。
4. **先 Spike 后投入**：Phase 0 不过，不拆除现有 Vue 实现。
5. **工程中心**：`FileSystemProvider` + `SCM Provider` + workspace 生命周期；不是再写树组件。

---

## 11. 参考

- https://github.com/CodinGame/monaco-vscode-api
- https://github.com/CodinGame/monaco-vscode-api/wiki/List-of-service-overrides
- https://github.com/CodinGame/monaco-vscode-api/wiki/How-to-install-and-use-VSCode-server-with-monaco‐vscode‐api（仅对照，非本路线主路径）
- Demo SCM 自定义 Provider：`demo/src/features/scm.ts`（upstream）
- 当前实现入口：`web/src/store/fileWorkbench.ts`、`web/src/features/files/runtime.ts`、`web/src/components/workspace/*`




