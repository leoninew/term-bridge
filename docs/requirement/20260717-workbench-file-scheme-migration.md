# Workbench 工作区 URI 迁到 file://

最后修改时间: 2026-07-17 15:34:00

- Review status: Accepted
- Mode: light
- Feature: workbench-file-scheme-migration

## Background

Code workbench 当前使用自定义 scheme `tb` 承载远程工作区文件。这导致：

- Explorer 树搜索硬编码只认 `file` / `vscodeRemote`
- monaco-vscode-api search-service-override 默认只为 `file` 注册 provider
- 需要 runtime patch + 自建 search provider 才能让树搜索工作

对比 `monaco-vscode-api` demo：工作区使用 `file://`，经 `registerFileSystemOverlay` 挂自定义 FS。

产品侧 `tb` **不是硬性要求**；逻辑路径仍由后端 workspace root 解析，不暴露本机绝对路径。

## Goal

- 将 workbench 工作区 URI 从 `tb://` 迁到 `file://`
- 用 `registerFileSystemOverlay` 挂载现有 `TermBridgePlatformFileSystemProvider`（或等价适配）
- Explorer 树搜索走官方 search 能力，移除对 `ExplorerFindProvider` 的 scheme patch（及可删除的 tb-only search provider）
- SCM / 打开文件 / 保存 / diff 在 `file://` 下保持可用

## Non-goal

- 不改后端 FS/Git API 协议（仍为 workspace-scoped logical path）
- 不实现 HTML File System Access 本机目录挂载作为主路径
- 不在本需求内做多 workspace 同页并行
- 不强制改名 `tb-scm`（可保留 virtual original scheme，或后续再统一）

## User scenarios

1. 打开 `/code`，Explorer 以 `file:///`（或约定 root）展示树，可打开/保存文件
2. Explorer 顶部 search 输入文件名片段后 filter/highlight，无需自定义 scheme patch
3. SCM 状态/diff 仍可打开（左端 original 可用 `tb-scm` 或等价 virtual scheme）
4. 切换 workspace 仍安全（现有 reload 策略可保留）

## Acceptance

- [ ] `workspaceRootUri` / path 转换使用 `file` scheme（约定 root path 文档化）
- [ ] FS 通过 `registerFileSystemOverlay`（或官方推荐方式）挂到 `file`，不再 `registerCustomProvider('tb')` 作为主路径
- [ ] Explorer 树搜索在无 `searchSupportsScheme` patch 时可用
- [ ] 打开文本文件、保存、创建/重命名/删除在 smoke 路径可用
- [ ] SCM list + open diff 可用
- [ ] 删除或停用 tb-only scheme patch；自建 provider 仅在仍需要时保留并说明
- [ ] 相关单测更新并通过；`task check` 通过

## Open questions

- 不适用（实现中已决策）

## Decisions

1. root 使用 `file:///`（`monaco.Uri.file('/')`）
2. FS 通过 `registerFileSystemOverlay(1, provider)` 挂到 `file`，并启用 `getFilesServiceOverride()`
3. 内部 API 路径继续用 URI `path`（logical）
4. `tb-scm` 保留为 SCM original virtual scheme
5. 删除 `tb` 专用 Explorer scheme patch 与自定义 fileSearchProvider；依赖官方 search-service-override

## Risk

- `file` overlay 与默认 memory provider 优先级冲突 → 用正 priority 盖住默认层
- 既有测试/硬编码 `scheme: 'tb'` 遗漏 → 全局检索清理
- SCM original URI 与工作区 URI scheme 不一致需继续可解析
- 大改动面：uri.ts、bootstrap、platform FS 事件、scmProvider、测试
