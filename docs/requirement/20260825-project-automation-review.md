# term-bridge Taskfile 审查结果

最后修改时间: 2026-08-25 22:30:05

Review status: Accepted

## 原始审查结论

`FAIL`。buf 和 golangci-lint 固定写入 `bin`，但 protoc-gen-go 使用 `@latest`，Air 在开发任务中使用全局命令；check 默认修改前端和 Go 格式，测试漏掉 migrations 包。

## 证据

- `Taskfile.yml:23-35` 的 install 承担依赖和工具链，没有 deps；`Taskfile.yml:31` 使用 `protoc-gen-go@latest`。
- `Taskfile.yml:40-42` 的 proto 使用 `./bin/buf`，但 `Taskfile.yml:71、79` 使用裸 `air`。
- `Taskfile.yml:44-50` 默认执行前端修复和无 `--diff` 的 golangci-lint fmt。
- `Taskfile.yml:53-57` 的测试只扫描 `./cmd/... ./internal/...`，`go list ./...` 另有 `migrations/agent`、`migrations/cloud`，且没有 `cov=1`。
- `Taskfile.yml:86-139` 使用 build/package，没有统一 release。

## 目标规范

把工具链移到 deps 并固定 protoc 版本；所有 Air/buf/linter 调用从 `./bin` 发起；check 直接执行 format/lint/typecheck，`fix=1` 才修改；test cov=1 保持单元测试范围并覆盖真实 package；release 负责构建产物。

## 当前状态

上述原始审查项已在本次 Taskfile 改进中完成，并由 `docs/verification/20260825-project-automation-review.md` 记录的检查、测试和 release 构建验证通过。`check` 保留原有 `./cmd/... ./internal/...` 范围，migrations 仅纳入 test。
