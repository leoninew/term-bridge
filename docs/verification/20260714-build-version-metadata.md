# 构建版本元数据验证
最后修改时间: 2026-08-19

## Review status

Accepted

## Flow mode / Stage

轻量模式 / light；验证 / Verification。本次记录当前版本模型及已执行的验证。

## Requirement alignment

- Requirement 文档：`docs/requirement/20260714-build-version-metadata.md`
- Requirement status：`Accepted`

对齐结论：发布版本由根目录 `VERSION` 记录，并通过 `task version:apply` 同步到 Go 与前端元数据。CLI、浏览器 runtime config 与 local 首页均使用 Go Version；版本不属于运行时配置。

## Spec alignment

不适用。轻量模式 / light 未创建单独 Spec 文档。

## Plan alignment

不适用。轻量模式 / light 按 Requirement 实施。

## Actual diff summary

- `VERSION`：新增作为 portable package 命名和包内记录的 SemVer 版本文件。
- `internal/shared/common/utils/version/version.go`：只保留编译进二进制的 `Version` 与运行时信息。
- `Taskfile.yml`：新增 `version`、`version:apply` 任务；build、package、docker 直接编译已同步的源码，portable package 使用版本化目录和 ZIP 名称。
- `scripts/version-calc.py`：`--apply` 同步更新 `VERSION`、Go `Version` 和 `web/package.json`；运行前工作目录干净时创建对应轻量 tag，不干净时警告并跳过 tag。
- `scripts/build-version.sh`、`scripts/build-version_test.sh`：删除旧的 Git tag、commit 距离、dirty 和环境变量覆盖构建模型。
- `Dockerfile`、`Dockerfile.cn`：移除 build args 和 linker flags。
- `cmd/termbridge/cli/cli_test.go`：验证 `termbridge version` 不再输出 commit 或构建时间。
- `.env.example`、`configs/config.yaml`：移除已经不存在的 build metadata 配置说明。
- 用户指南与前端帮助目录：改为展示带版本号的 portable package 文件名。

## Expected vs actual changed files

预期涉及版本文件、构建脚本、Dockerfile、Go version/CLI、发布命名和相关文档/测试。实际改动覆盖上述范围；未引入后端业务 API 或运行时 YAML 配置键。

## Acceptance criteria checklist

- [x] `VERSION` 保存 SemVer package 版本，并用于 portable package 命名和包内记录。
- [x] `task version:apply` 同步 `VERSION`、Go `Version` 和 `web/package.json`。
- [x] `task version:apply` 仅在运行前工作目录干净时创建 `v<version>` 轻量 tag；不干净时输出 warning 并跳过 tag。
- [x] build、package、docker 不再传递 linker metadata 或构建环境变量。
- [x] `termbridge version` 输出 Version 与运行时信息，且不启动服务。
- [x] local dashboard 通过服务端 runtime config 显示 Go Version。
- [x] 版本未被加入 YAML/Viper/portable runtime profile。
- [x] portable package 文件名为 `termbridge-v<version>-<platform>.zip`，并在包内携带 `VERSION`。

## Command results

通过：

```text
python scripts/version-calc.py --quiet
  -> version: 0.114.1

task --dry package
  -> 展开 termbridge-v0.114.1-<platform> 的目录和 ZIP 路径

go test ./cmd/... ./internal/...
  -> passed
```

`git diff --check` 当前报告 `VERSION:1` 的 CRLF 尾随空白；在提交前需要改为 LF。

## Missed or expanded scope

范围扩展：新增 `VERSION` 作为 portable package 元数据，并把 package 命名改为带版本号的文件名；未改变任何业务 API。

## Risks

1. 本机 Docker daemon 不可用，未执行真实 Docker image build/run；两个 Dockerfile 已移除 build args，但仍需验证镜像 `termbridge version` 与仓库版本一致。
2. GitHub Release 名称来自 tag，资产名称来自 `VERSION`；干净工作目录运行 `task version:apply` 会创建对应 tag。若运行前不干净而跳过 tag，发布时必须确认 tag 为 `v$(cat VERSION)`。
3. `VERSION`、Go `Version` 和 `web/package.json` 的同步依赖 `task version:apply`；应在 CI 中加入一致性检查，防止手工编辑造成漂移。

## Incomplete items

- Docker daemon 可用后，需分别构建 `Dockerfile` 与 `Dockerfile.cn`，执行容器 `termbridge version`，确认其与 `VERSION` 一致。
- 在 release CI 中验证 SemVer tag 等于 `v$(cat VERSION)`，并验证 `VERSION`、Go `Version`、`web/package.json` 一致后再上传资产。

## Conclusion

结论：**版本模型已改为 `VERSION` 加受控同步；Docker 运行态验证和 release tag 一致性校验仍待补充。**

`termbridge version`、服务端 browser runtime config 和 local 首页使用同一 Go Version；运行时配置文件不能覆盖已编译二进制身份。portable package 命名和包内记录使用 `VERSION`，发布流程必须保持它与 Git tag 一致。
