# HTTP 静态资源访问日志过滤 — 验证
最后修改时间: 2026-07-09 16:35:00

Flow mode: light / 轻量模式
Stage: Verification / 验证

## 需求对齐

对照 `docs/requirement/20260709-http-assets-logging-filter.md`（Accepted）：

- 默认过滤 `/assets/` 开头、配置扩展名结尾、最终 HTTP status 为 `200` 的 access log — ✓ 实现按最终 status==200 决定是否跳过。
- 配置开关，默认启用 — ✓ `SkipAssetEnabled` 默认 `true`（`configs/config.yaml`）。
- 开关关闭后恢复现有 access log — ✓ `SkipAssetEnabled=false` 时所有请求走正常日志路径。
- `/assets/` 下非 `200` 请求仍输出日志 — ✓ `deferStartedLog && status==200` 之外补打 started/completed。
- 非 `/assets/` 路径即使扩展名相同也不过滤 — ✓ `isSkippableAssetPath` 先校验 `assetsPathPrefix`。
- API 路径不受影响 — ✓ `/local-api/...` 不走资产匹配。
- 不改变 handler 执行、响应状态码、响应体、静态文件服务 — ✓ 候选资产路径仍经 `LoggingResponseWriter` + `next.ServeHTTP`，仅日志输出被跳过。
- 无兼容/适配层 — ✓ 配置键仅保留 `log.http.skip_asset_*`，旧的 `skip_assets_200_*` 不读取、不回退；`requestlog` 包删除本地 `Config` struct，直接用 `sharedconfig.LogHTTPConfig`；agent/cloud bootstrap 删除本地 `LogHTTPConfig`，复用 shared。

## 规格对齐

不适用（轻量模式，未创建 spec.md）。按 requirement / 需求核对。

## 计划对齐

不适用（轻量模式，未创建 plan.md）。

## 实际 Diff 摘要

预期 vs 实际改动文件一致，未扩展到范围外：

- `internal/shared/infrastructure/config/config.go` — `LogHTTPConfig` 增加 `SkipAssetEnabled`/`SkipAssetExtensions`；`buildConfig` 加载 `log.http.skip_asset_*`；`configKeys()` 注册新键；`normalizeLogHTTPConfig` 合并原 body-limit 规范化 + 扩展名规范化（小写、补点、去重、去空）。
- `internal/shared/api/middleware/requestlog/logging.go` — 候选资产路径延迟 `request started`，handler 返回后按 `status==200` 决定是否整体跳过；新增 `isSkippableAssetPath`、`assetExtensionSet`；删除本地 `Config` struct。
- `internal/agent/application/bootstrap/config.go`、`internal/cloud/application/bootstrap/config.go` — 删除本地 `LogHTTPConfig`，`Config.LogHTTP` 改用 `sharedconfig.LogHTTPConfig`。
- `internal/agent/application/bootstrap/router.go`、`internal/cloud/application/bootstrap/router.go` — `requestlog.Middleware(logger, cfg.LogHTTP)` 直接传 shared config。
- `configs/config.yaml`、`bin/package/TermBridge/configs/config.yaml`、`.env.example` — 增加 `skip_asset_enabled` / `skip_asset_extensions` 默认值。
- `internal/agent/application/bootstrap/router_test.go`、`internal/shared/api/middleware/requestlog/logging_test.go`、`internal/shared/infrastructure/config/config_test.go` — 字段名/测试 helper 同步。
- `cmd/termbridge/app/app.go` — `agentConfig`/`cloudConfig` 直接传 `cfg.LogHTTP`，去掉重复字段拷贝。
- `docs/requirement/20260709-http-assets-logging-filter.md` — 需求文档。

## 验收清单

| 验收项 | 结果 |
| --- | --- |
| `/assets/` + 配置扩展名 + 最终 `200` 不输出 access log | ✓ `TestMiddlewareSkipsConfiguredAssetSuccessLogs`（含 `/assets/theme.css?v=1` query 场景） |
| 扩展名集合覆盖需求列出的 18 种 | ✓ `wantDefaultSkipAssetExtensions` + yaml 默认值 |
| 配置开关默认启用 | ✓ `TestLoadDefaults` 断言 `SkipAssetEnabled==true` |
| 开关关闭后恢复日志 | ✓ `TestMiddlewareKeepsAssetLogsWhenSkipDisabled` |
| `/assets/` 非 `200` 仍输出日志 | ✓ `TestMiddlewareKeepsFailedAssetAndAPILogs`（`/assets/missing.js` → 404 保留） |
| 非 `/assets/` 路径不过滤 | ✓ `TestMiddlewareKeepsFailedAssetAndAPILogs`（`/local-api/health.js` → 保留） |
| API 路径不受影响 | ✓ 同上 |
| handler 执行/响应不变 | ✓ `TestMiddlewareSkipsConfiguredAssetSuccessLogs` 断言 recorder.Code/Body 不变 |
| 扩展名配置生效 | ✓ `TestMiddlewareUsesConfiguredAssetExtensions`（`.js` 保留、`.css` 跳过） |
| 扩展名规范化（大小写、补点、去重、去空） | ✓ `TestLoadDotEnvOverridesDefaultYAMLAndBaseIgnoresEnv`（`js,CSS,,.webp` → `.js,.css,.webp`） |

## 命令结果

- `go test ./internal/shared/api/middleware/requestlog/... ./internal/shared/infrastructure/config/... ./internal/agent/application/bootstrap/... ./internal/cloud/application/bootstrap/... ./cmd/termbridge/app/...` — 全部 `ok`。
- `go vet ./...` — 干净。
- `gofmt -l internal/ cmd/` — 未列出本次改动的文件；列出的 `cloud_binding_test.go` / `server.go` / `cors_test.go` 均为本次未触碰的历史文件，与本变更无关。
- `git diff --check` — 见下方"未完成项"。

## 范围偏差

无。未扩展到 API 行为、前端 runtime、Docker、SPA fallback 或 PomeloOrbit-go 其它实现。

## 风险

- 候选资产路径的 `request started` 必须延迟到 handler 返回后；若 status==200 则整体跳过，否则补打 started/complemented。当前实现正确处理该分支。
- 配置键为中断性变更：旧的 `TERMBRIDGE_LOG__HTTP__SKIP_ASSETS_200_*` 不再被读取。无兼容层（按用户要求）。

## 未完成项

- `git diff --check` 报告 `.env.example:29-31` 与 `configs/config.yaml:22-41` 存在 trailing whitespace。Read 工具显示这些行在 `true`/`分隔。`/扩展名列表之后无可见空格，疑似 Windows git autocrlf 对本次新增行产生的 CR 标记，非真实空格。未做猜测性编辑（避免违反 exact-string 编辑规则）；如确认是真实尾部空格，需在提交前清理。
- 工作树存在无关未跟踪文件 `docs/requirement/20260709-docker-frontend-runtime-config.md`，不属于本次变更，未处理。

## 结论

实现与 Accepted 需求一致，验收清单全部通过，测试/vet/gofmt（本次改动文件）干净。可交付，建议先确认并清理 `git diff --check` 报告的尾部空白后再提交。
