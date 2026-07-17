# Explorer 文件树搜索无效

最后修改时间: 2026-07-17 21:20:00

- Review status: Accepted
- Mode: light
- Feature: workbench-explorer-tree-search

## Background

Code workbench 左侧 Explorer（文件目录树）顶部的 Type to search / filter 文本框可打开，但输入后不产生过滤或高亮结果。

对比 `D:\SourceCodes\opensource\monaco-vscode-api` demo：

- demo 工作区使用 `file://` scheme，并注册 `@codingame/monaco-vscode-search-service-override`
- TermBridge 使用自定义 scheme `tb`，且未安装/注册 search service override

## Goal

- Explorer 树顶部 search 文本框可对当前工作区文件/目录名进行 filter/highlight
- 与 demo 行为一致：依赖 `ISearchService.fileSearch` 返回匹配资源

## Non-goal

- 不实现完整 Search 面板全文检索（Ctrl+Shift+F）的完备体验（可预留 provider 接口，textSearch 可先空实现）
- 不改为 `file://` 工作区 scheme
- 不修改 node_modules 源文件（用运行时 patch + 自有 provider）

## User scenarios

1. 打开 `/code` 工作台，侧栏 Explorer 可见文件树
2. 点击或快捷键打开树顶部 search 输入框
3. 输入文件名片段后，树过滤或高亮匹配项
4. 清空后恢复完整树

## Acceptance

- [ ] 已注册 search service override，使 `ISearchService` 可用
- [ ] 为 `tb` scheme 注册 `file` 类型 search result provider
- [ ] ExplorerFindProvider 对 `tb` 不再被 scheme 硬过滤拒绝
- [ ] 输入关键字后树结果随关键字变化（filter 或 highlight 模式）

## Open questions

- 不适用（根因与修复路径已明确）

## Decisions

1. 工作区保持 `tb` scheme
2. 安装并启用 `@codingame/monaco-vscode-search-service-override`
3. 自实现 `TermBridgeFileSearchProvider`（基于 FileService 遍历 + glob 匹配）
4. 运行时 patch `ExplorerFindProvider.searchSupportsScheme`，允许 `tb` 在已注册 provider 时通过

## Risk

- 官方 `WorkspaceSearchProvider` 硬编码只索引 `file` scheme，不能直接复用
- Explorer 源码硬编码只允许 `file`/`vscodeRemote`；若不 patch，即使注册 provider 仍无根可搜
- 大工作区全量遍历可能有性能开销（限制 maxResults / 深度）
