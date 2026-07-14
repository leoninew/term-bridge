# 构建版本元数据验证
最后修改时间: 2026-07-14 10:51:40

## Review status

Accepted

## Flow mode / Stage

轻量模式 / light；验证 / Verification。本次为已完成版本相关工作的补充记录。

## Requirement alignment

- Requirement 文档：`docs/requirement/20260714-build-version-metadata.md`
- Requirement status：`Accepted`

对齐结论：构建元数据已统一由 Git 派生并注入 Go 二进制；CLI、浏览器 runtime config 与 local 首页均使用 Go Version。构建 metadata 与运行时配置严格分离。

## Spec alignment

不适用。轻量模式 / light 未创建单独 Spec 文档。

## Plan alignment

不适用。轻量模式 / light 按 Requirement 实施。

## Actual diff summary

- `internal/shared/common/utils/version/version.go`：复用 Version、Commit、BuildTime 变量作为 linker 注入目标。
- `Taskfile.yml`：为 build、package、docker 增加统一 `GO_LDFLAGS`；Commit 固定读取当前 Git HEAD 12 位短 hash，BuildTime 默认 UTC；所有 build 入口使用 Git 派生 Version。
- `scripts/build-version.sh`：新增 Git 版本解析器，支持 release tag、tag 后提交、无 tag 开发版本、dirty 标识和受控版本覆盖。
- `scripts/build-version_test.sh`：验证无 tag、精确 tag、tag 后提交、dirty 工作区与显式版本覆盖。
- `Dockerfile`、`Dockerfile.cn`：Go build stage 使用 Version、Commit、BuildTime build args 编译二进制。
- `cmd/termbridge/cli/cli.go`、`cmd/termbridge/cli/cli_test.go`：将版本查询改为 `termbridge version` 子命令，并验证 Version/Commit/BuildTime 输出。
- `internal/shared/dto/browser/runtime_config.go`、`internal/shared/infrastructure/config/browser_runtime_config.go`、`web/src/config.ts`、`web/src/store/runtimeConfig.ts`：服务端将 Go Version 注入浏览器 runtime config；开发环境可使用 Vite version 值，缺失时页面版本内容为空。
- `web/src/components/dashboard/LocalHome.vue`：项目版本卡显示 runtime config Version。
- `.env.example`、`configs/config.yaml`、`README.md`：记录构建元数据来源、Git tag/dirty 语义，以及其非运行时配置边界。

## Expected vs actual changed files

预期涉及构建脚本、Dockerfile、Go version/CLI、browser runtime config、local 首页和相关文档/测试。实际改动覆盖上述范围；未引入后端业务 API 或运行时 YAML 配置键。

## Acceptance criteria checklist

- [x] build、package、docker 复用同一 linker metadata 结构。
- [x] Commit 固定来自 `git rev-parse --short=12 HEAD`。
- [x] BuildTime 使用 UTC `YYYYMMDD-HHMMSS` 默认格式。
- [x] Version 可根据 Git tag / commit 距离 / 无 tag / dirty 状态区分构建。
- [x] `termbridge version` 输出 Version、Commit 与 BuildTime，且不启动服务。
- [x] local dashboard 通过服务端 runtime config 显示 Go Version。
- [x] build metadata 未被加入 YAML/Viper/portable runtime profile。
- [x] helper 测试覆盖 Git 版本派生的主要场景。

## Command results

通过：

```text
sh scripts/build-version_test.sh
  → passed（无 tag、精确 tag、tag 后提交、dirty、显式覆盖）

go test ./cmd/termbridge/cli/...
  → passed

go test ./cmd/termbridge/app/...
  → passed

go test ./internal/shared/infrastructure/config/... ./internal/shared/api/...
  → passed

yarn --cwd web test --run src/store/runtimeConfig.test.ts
  → 1 test file / 8 tests passed

yarn --cwd web typecheck
  → passed

task --dry --verbose build/package/docker
  → 统一展开 Git-derived Version、12 位 Commit、UTC BuildTime

go run -ldflags "…" ./cmd/termbridge version
  → 输出指定 Version、Commit、BuildTime；未启动服务
```

`git diff --check`：版本相关 Go、Taskfile、script 改动通过；全工作区仍报告 `web/.env.development` 的既有尾随空白，未由本任务修改。

## Missed or expanded scope

范围扩展：增加了 browser runtime config 和 local 首页实际显示路径，确保构建版本对用户可见；未改变任何业务 API。

## Risks

1. 本机 Docker daemon 不可用，未执行真实 Docker image build/run；Docker build-arg 与 linker 命令仅通过静态审查和 Task dry-run 确认。
2. 当前 Git 历史没有 release tag，当前自动版本为开发形式；发布流程仍需采用不可变 `vX.Y.Z` tag。
3. `TERMBRIDGE_BUILD_VERSION` 允许 CI/源码归档覆盖，因此发布 CI 需将该变量限制在受控发布环境，不能用于掩盖脏工作区产物。

## Incomplete items

- Docker daemon 可用后，需分别构建 `Dockerfile` 与 `Dockerfile.cn`，执行容器 `termbridge version`，确认 metadata 与 Taskfile 一致。
- 建立实际 release CI 后，应在 tag 触发器中验证 SemVer tag、全量 fetch tags、portable package checksum 与镜像发布策略。

## Conclusion

结论：**已满足本次构建版本元数据的代码与脚本目标；Docker 运行态验证待本机 Docker daemon 可用后补充。**

Git 派生的 Version、固定 Git HEAD Commit、UTC BuildTime 已在编译期统一注入。`termbridge version`、服务端 browser runtime config 和 local 首页使用同一 Go Version；运行时配置文件不能覆盖已编译二进制身份。
