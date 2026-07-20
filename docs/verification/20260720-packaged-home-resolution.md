# 打包运行时用户目录解析 — 验证

最后修改时间: 2026-07-20 08:49:35

Review status: Accepted

## Requirement alignment

按 `docs/requirement/20260720-packaged-home-resolution.md` 核对：

| 需求点 | 结果 |
| --- | --- |
| `~` 不再直接调用 `os.UserHomeDir()` | 已实现：`resolveConfigPath` 改用 `resolveConfigHomeDir` |
| Windows 原生 home 优先 | 已实现：`USERPROFILE` → `HOMEDRIVE` + `HOMEPATH` → `HOME` |
| 非 Windows 使用 `HOME` | 已实现：Windows 分支外使用 `HOME` |
| 以标准路径拼接 `.termbridge` | 已保持：`filepath.Join(home, path[2:])` |
| state、SQLite、日志共同覆盖 | 已覆盖：`TestLoadResolvesHomeRelativeRuntimePaths` |
| 缺失 home 环境变量报错 | 已覆盖：`TestResolveConfigHomeDirRequiresEnvironmentHome` |
| cwd 相对与绝对路径语义 | `TestResolveConfigPath` 通过 |

## Spec alignment

不适用。light / 轻量模式无独立 Spec。

## Plan alignment

不适用。light / 轻量模式无独立 Plan；按 Requirement 直接实现。

## Actual diff summary

本需求预期代码与过程文档变更：

1. `internal/shared/infrastructure/config/config.go`
   - 新增按平台决定优先级的 `resolveConfigHomeDir`。
   - Windows 避免把 Cygwin 格式的 `/c/Users/...` `HOME` 当成 Windows 路径根目录。
2. `internal/shared/infrastructure/config/config_test.go`
   - 覆盖平台优先级、`USERPROFILE` 回退、`HOMEDRIVE` + `HOMEPATH` 回退、无 home 报错，以及 state/SQLite/log 的集成加载路径。
3. `docs/requirement/20260720-packaged-home-resolution.md`
   - 记录 Cygwin 与 PowerShell `HOME` 格式差异和最终优先级。
4. `internal/agent/infrastructure/gitexec/repository_test.go`
   - 将 Windows 不可靠的 Unix `pre-commit` hook fixture 改为跨平台的 Git commit failure wrapper。
5. 本文档。

## Expected vs actual changed files

### 预期本需求范围

- `docs/requirement/20260720-packaged-home-resolution.md`
- `docs/verification/20260720-packaged-home-resolution.md`
- `internal/shared/infrastructure/config/config.go`
- `internal/shared/infrastructure/config/config_test.go`
- `internal/agent/infrastructure/gitexec/repository_test.go`

### 实际观察

配置 home 解析及 Requirement 已位于当前 `HEAD`（`84bea1c fix(config): resolve packaged home paths from environment`）。本次暂存区仅包含：修正后的 gitexec 测试和本文档；`git diff --cached --check` 通过。

执行 `task check` 的自动修复命令后，工作区仍有 29 个与本需求无关的前端文件和 browser runtime config 文件改动。它们不属于本需求实现，未将其纳入暂存区，也未对其执行还原操作。

## Acceptance criteria checklist

- [x] `resolveConfigPath` 不直接用 `os.UserHomeDir()` 展开 `~`
- [x] Windows `HOME=/c/Users/...` 与 `USERPROFILE=C:\\Users\\...` 冲突时选择 `USERPROFILE`
- [x] state、SQLite、日志解析到同一 native home 下的 `.termbridge`
- [x] `USERPROFILE` 回退有测试覆盖
- [x] `HOMEDRIVE` + `HOMEPATH` 回退有测试覆盖
- [x] home 来源全部缺失时返回错误
- [x] 相对和绝对路径既有行为通过测试
- [x] gitexec 的跨平台 commit 失败映射测试通过
- [x] `go test ./internal/...` 通过
- [x] `task check` 通过

## Test results

### 项目规定检查（通过）

```text
task check
# yarn --cwd web typecheck                 PASS
# yarn --cwd web lint:fix                  PASS
# yarn --cwd web format:fix                PASS
# ./bin/golangci-lint fmt ./cmd/... ./internal/...  PASS
# ./bin/golangci-lint run ./cmd/... ./internal/...  0 issues
```

### 相关配置测试（通过）

```text
go test -run 'TestLoadResolvesHomeRelativeRuntimePaths|TestResolveConfigHomeDir' -v ./internal/shared/infrastructure/config
# PASS
```

覆盖通过：运行时三类路径、平台优先级、Windows `USERPROFILE`、Windows `HOMEDRIVE` + `HOMEPATH`、缺失变量报错。

### 本机环境核对（通过）

```text
Cygwin HOME=/c/Users/wangm25
Windows USERPROFILE=C:\Users\wangm25
Expected state=C:\Users\wangm25\.termbridge
```

实现会在 Windows 目标上选择 `USERPROFILE`，因此该场景落到预期 state 路径。

### 全量内部 Go 测试（通过）

```text
go test ./internal/...
# PASS
```

验证中发现 `TestCommitFailureOmitsPreCommitStatus` 假设 Unix `chmod +x` 能在 Windows 使无扩展名 shell hook 可执行，导致 Git for Windows 跳过该 hook、commit 错误地在测试中成功。该测试已改为平台无关的 Git wrapper：仅 `commit` 子命令确定性返回非零，其余前置 status/scope 命令委托真实 Git。修复后：

```text
go test -count=1 ./internal/agent/infrastructure/gitexec
# PASS
```

## Missed or expanded scope

- 未修改 `scripts/package/.env.preflite` 或 package 启动脚本，符合 Non-goal。
- 未修改终端会话 CWD 的独立 `~` 展开，符合 Non-goal。
- `task check` 产生的无关格式化改动不属于本需求，未处理。

## Risks

1. `HOMEDRIVE` 与 `HOMEPATH` 仅在均非空时组合；不完整环境会继续回退到 `HOME` 或返回错误。
2. 没有在真实 Windows 打包 ZIP 中手工启动 Agent；本机已核对 Cygwin 与 Windows 环境变量值，并通过 Windows 目标单元测试逻辑覆盖。
3. 当前工作区包含 `task check` 自动修改的无关文件；提交前需由用户决定是否保留、单独提交或还原。

## Incomplete items

1. 可选的端到端手工验收：从 Windows ZIP 的 `termbridge.cmd` 启动，在 Cygwin 注入 `/c/...` 的 `HOME` 后确认实际生成目录为 `%USERPROFILE%\\.termbridge`。
2. 需处理 `task check` 造成的无关工作区改动，避免与本需求混合提交。

## Conclusion

**本需求范围内验证通过。**

`task check` 与 `go test ./internal/...` 已通过；相关配置测试及本机 Cygwin/PowerShell 环境核对均确认 Windows 会使用原生 `USERPROFILE`，从而将 `~/.termbridge` 解析为 `C:\\Users\\wangm25\\.termbridge`。gitexec 的 Windows 兼容测试夹具已修正。工作区仍有 `task check` 自动产生的无关格式化改动，已明确隔离并记录。
