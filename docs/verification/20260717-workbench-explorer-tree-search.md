# Explorer 文件树搜索验证

最后修改时间: 2026-07-17 15:34:36

- Review status: Draft
- Mode: light
- Feature: workbench-explorer-tree-search
- Linked requirement: docs/requirement/20260717-workbench-explorer-tree-search.md

## 需求对齐

- 目标：Explorer 顶部 Type to search/filter 对 `tb` 工作区生效。
- 实现路径与 requirement Decisions 一致：
  1. 启用 `@codingame/monaco-vscode-search-service-override`
  2. 为 `tb` 注册 file search provider
  3. patch `ExplorerFindProvider.searchSupportsScheme` 允许已注册 provider 的 `tb`
- Non-goal 保持：不强制完整 Ctrl+Shift+F 全文检索；不改工作区 scheme（本需求范围）。

## Spec / Plan 对齐

- 不适用（light 模式，按 requirement 实现）。

## 实际 diff 摘要（本 feature 相关）

- 新增 `fileSearchMatch.ts` / `fileSearchMatch.test.ts`：遍历与 glob 匹配纯逻辑。
- 新增 `fileSearchProvider.ts`：`TermBridgeFileSearchProvider` + scheme patch + 注册入口。
- 修改 `bootstrap.ts`：接入 search service override、patch、注册 provider。
- 修改 `vite.config.ts`：alias 解析 `explorerViewer.js`，修复 Rolldown deep import。
- 修改 `package.json` / `yarn.lock`：增加 search-service-override 依赖。
- 新增 requirement 文档。

## 预期 vs 实际改动文件

| 预期 | 实际 | 说明 |
|------|------|------|
| search service 依赖 | 有 | package.json / yarn.lock |
| bootstrap 接入 | 有 | getSearchServiceOverride + register |
| tb file search provider | 有 | fileSearchProvider.ts |
| Explorer scheme 兼容 | 有 | patchExplorerFindSchemeSupport |
| 纯逻辑单测 | 有 | fileSearchMatch.test.ts |
| vite resolve | 有 | alias for explorerViewer |

暂存区另含 `task check` 触发的 format 与其他未完成 UI 改动（sessions/dashboard/titlebar 等），**不属于本 feature 验收范围**，拆 commit 时建议分离。

## 验收清单

- [x] 已注册 search service override
- [x] 为 `tb` 注册 file 类型 search result provider
- [x] ExplorerFindProvider 对 `tb` 可通过 scheme 检查（patch）
- [ ] 浏览器：输入关键字后树结果变化（需本地打开 `/code` 手工确认）
- [x] `task check` 通过（typecheck / lint:fix / format:fix / golangci-lint）
- [x] 单测：`fileSearchMatch` 2 例通过

## 命令结果

```text
task check
- yarn --cwd web typecheck  OK
- yarn --cwd web lint:fix  OK
- yarn --cwd web format:fix OK
- golangci-lint fmt/run     0 issues

cd web && yarn test src/features/workbench/fileSearchMatch.test.ts
- Test Files  1 passed
- Tests       2 passed
```

## 范围偏差

- 为通过构建，增加了 vite alias；属于实现约束，未扩大产品范围。
- 暂存区混入其他 format/UI 文件：交付时应拆分或单独说明，避免与本 fix 绑死。

## 风险

1. patch 依赖 `ExplorerFindProvider` 内部 API；monaco-vscode-api 升级可能破坏。
2. 官方 WorkspaceSearchProvider 仍只服务 `file`；本 fix 用自有 walk provider，大仓库有遍历上限（depth/entries/maxResults）。
3. 浏览器手工验收尚未在本机完成。
4. 中长期更干净方案是工作区迁 `file://`（新需求）。

## 未完成项

- [ ] `/code` 页面手工确认 filter/highlight
- [ ] 拆分无关暂存改动（可选）

## 结论

实现与 requirement 主线对齐，自动化检查通过；**功能闭环差浏览器验收**。  
建议：手工确认树搜索后将本 verification 标为 Accepted，再单独提交 workbench search fix（勿与 sessions UI format 混 commit）。

## 后续

同日已启动 `workbench-file-scheme-migration`：工作区改为 `file://` + `registerFileSystemOverlay`，官方 search-service-override 接管 Explorer 搜索。
`tb` 专用 fileSearchProvider / scheme patch 已删除；本 verification 的实现作为迁移前中间态保留文档。
