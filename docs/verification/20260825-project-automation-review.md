# term-bridge Taskfile 自动化改进验证

最后修改时间: 2026-08-26

Review status: Accepted

## Requirement alignment

已按 `docs/requirement/20260825-project-automation-review.md` 收敛 Taskfile：

- 工具安装从 `install` 拆到 `deps`，`buf`、`protoc-gen-go`、`golangci-lint` 和 `air` 均固定版本并安装到项目 `bin/`。
- `protoc-gen-go` 不再使用 `@latest`；proto、Air 和 linter 任务均从 `./bin` 调用。
- `deps` 使用 `GOTOOLCHAIN=local`，避免固定版本工具在安装时自动下载 Go 工具链。
- `check` 默认只读且离线；前端使用 `lint`/`format`，Go 使用 `golangci-lint fmt --diff` 和带 `--modules-download-mode=readonly --timeout` 的 `run`，只有 `task check -- --fix` 才启用修复。
- `check` 与 `test` 均覆盖 `./cmd/... ./internal/... ./migrations/...`，生成 proto 文件由最小化路径排除规则处理。
- `task test -- --cov` 只增加 Go `-cover -coverprofile=coverage.out` 参数，不改变测试 package 范围。
- portable 构建统一收敛到 `release`，CI 改为使用 `task deps` 和 `task release`。

## Spec alignment

不适用。本任务按 light / 轻量模式执行，未创建独立 Spec 文档。

## Plan alignment

不适用。本任务按 light / 轻量模式依据 Requirement 实施，未创建独立 Plan 文档。

## Actual diff summary

- `Taskfile.yml`：新增固定工具版本、`deps`、用户级 `install`、`proto:deps`；以离线、只读边界统一 check/test package 范围；加入 `--fix`、`--cov` 透传；统一 Air、release 和构建环境变量入口。
- `.github/workflows/release.yml`：CI 使用仓库 Taskfile 的依赖和 release 入口。
- `.gitignore`：忽略 `coverage.out`。
- `README.md`、`README.en.md`：将依赖安装入口更新为 `task deps`，补充 CLI 安装入口。
- `docs/requirement/20260825-project-automation-review.md`：标记为 `Accepted`。
- `docs/verification/20260825-project-automation-review.md`：记录本次验证结果。

## Expected vs actual changed files

本任务预期变更为上述自动化、文档和 CI 文件，实际均已覆盖。工作区另有 `docs/design/design.md` 修改和 `docs/design/cloud-client-communication.md` 新增；它们在本任务开始前已存在，不属于本次 Taskfile 自动化实现，但按用户要求会随全部工作区变更一起暂存和提交。

## Acceptance checklist

- [x] `task --list` 可解析，默认任务只显示任务列表。
- [x] `task deps` 下载 Go/Web 依赖，并使用 `GOTOOLCHAIN=local` 安装固定版本工具到 `bin/`；不执行 `go mod tidy`。
- [x] `task install` 仅安装用户级 `termbridge` CLI。
- [x] `task check` 在离线环境中以只读模式检查 `./cmd/... ./internal/... ./migrations/...`。
- [x] `task check -- --fix` 展开前端和 Go 修复命令。
- [x] `task test` 覆盖前端、`cmd`、`internal` 和 `migrations` 单元测试。
- [x] `task test -- --cov` 生成 `coverage.out`，且测试范围不变。
- [x] `task release` 负责构建三平台 portable release packages，不隐式上传或发布。

## Test results

| Command | Result |
| --- | --- |
| `task --list` | PASS |
| `task --dry check` | PASS；Go 展开为离线、只读的 `./cmd/... ./internal/... ./migrations/...`，前端为只读 lint/format |
| `task --dry check -- --fix` | PASS；展开 `lint:fix`、`format:fix`、`golangci-lint run --fix` |
| `task --dry test -- --cov` | PASS；展开 `go test -cover -coverprofile=coverage.out ./cmd/... ./internal/... ./migrations/...` |
| `task deps` | PASS；Yarn lock 校验和四个固定版本 Go 工具安装成功 |
| `task check` | PASS；前端 lint、format、typecheck 和 Go lint 均通过，golangci-lint 为 0 issues |
| `task test` | PASS；Web 32 个测试文件 / 168 个测试，Go 全部 package 通过 |
| `task test -- --cov` | PASS；覆盖率文件生成成功，Go 全部 package 通过 |
| `task release` | PASS；生成 Windows、Linux、macOS x64 portable zip |

## Missed or expanded scope

- 未修改 proto 生成结果；`proto:deps` 仅显式更新 Buf module 依赖，`proto` 只负责生成。
- 未运行常驻的 `task dev:agent`、`task dev:cloud` 或 `task dev:web`，因为它们会启动开发服务器，不属于本次自动化定义的必要验证。
- release 构建输出保留在被忽略的 `dist/`，覆盖率输出 `coverage.out` 已加入忽略规则。

## Risks and incomplete items

- portable release 命令继续沿用仓库已有的 POSIX/Cygwin `rm`、`cp`、`zip` 和 `chmod` 工具约定；本次在当前 Windows + Cygwin 环境实际验证通过，未扩展为纯 PowerShell 实现。
- Vite 构建继续报告既有的大 chunk warning；构建成功，不属于本次 Taskfile 变更范围。

## Conclusion

Requirement 中的自动化审查项已完成，Taskfile 的依赖、质量检查、测试覆盖和 release 入口均已按当前规范验证通过。结论为 `Accepted`。
