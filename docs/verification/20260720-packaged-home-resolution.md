# 打包运行时用户目录解析 — 验证

最后修改时间: 2026-07-20 09:47:58

Review status: Accepted

## Requirement alignment

按 `docs/requirement/20260720-packaged-home-resolution.md` 核对：

| 需求点 | 结果 |
| --- | --- |
| `~` 不再直接调用 `os.UserHomeDir()` | 已实现：`resolveConfigPath` 改用 `resolveConfigHomeDir` |
| package launcher 不解释 `.env.preflite` | 已实现：`termbridge.cmd` / `termbridge.sh` 仅设置 `TERMBRIDGE_ENV=preflite`，由 `config.Load` 读取 profile |
| Cygwin `source` 不得提前展开 `~` | 已实现：移除 shell profile sourcing；新增 launcher environment regression test |
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
5. `scripts/package/termbridge.cmd` / `scripts/package/termbridge.sh`
   - 删除重复 profile parser；launcher 仅选择 `preflite` environment。
6. `scripts/package/termbridge_test.sh`
   - 用 Cygwin-style `HOME=/c/Users/...` fixture 确认 shell launcher 不会导出 profile 值。
7. `cmd/termbridge/app/app_test.go`
   - 更新陈旧的 prod/test package assertions 为当前 preflite profile 合约。
8. 本文档。

## Expected vs actual changed files

### 预期本需求范围

- `docs/requirement/20260720-packaged-home-resolution.md`
- `docs/verification/20260720-packaged-home-resolution.md`
- `internal/shared/infrastructure/config/config.go`
- `internal/shared/infrastructure/config/config_test.go`
- `internal/agent/infrastructure/gitexec/repository_test.go`
- `scripts/package/termbridge.cmd`
- `scripts/package/termbridge.sh`
- `scripts/package/termbridge_test.sh`
- `cmd/termbridge/app/app_test.go`

### 实际观察

配置 home resolver、Requirement 和 gitexec fixture 已位于当前 `HEAD`。本次实际变更移除两个 launcher 的重复 `.env.preflite` 解析，增加 shell launcher regression test，并将旧 prod/test package assertions 改为当前 preflite profile 合约。

当前 `git diff --check` 通过；本轮 focused tests 未产生范围外格式化改动。

## Acceptance criteria checklist

- [x] `resolveConfigPath` 不直接用 `os.UserHomeDir()` 展开 `~`
- [x] launcher 不解析 `.env.preflite`，应用是唯一 dotenv parser
- [x] Cygwin-style `HOME=/c/Users/...` 下 launcher 不会导出已展开的 profile 值
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

### Launcher 与相关配置测试（通过）

```text
sh scripts/package/termbridge_test.sh
# PASS

go test ./cmd/termbridge/app ./internal/shared/infrastructure/config
# PASS
```

shell fixture 在 `HOME=/c/Users/cygwin-user`、`USERPROFILE=C:\\Users\\native-user` 下执行 `termbridge.sh`；伪 `termbridge` 仅收到 `TERMBRIDGE_ENV=preflite`，没有收到 runtime state、SQLite、log 的 profile environment variables。由此证明 launcher 不会把 `~/.termbridge` 提前展开为 `/c/...`。Go 覆盖继续验证运行时三类路径、平台优先级、Windows `USERPROFILE`、Windows `HOMEDRIVE` + `HOMEPATH`、缺失变量报错。

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

- 未修改 `scripts/package/.env.preflite`；profile 继续以 literal `~/.termbridge` 声明持久化位置。
- 修改 package 启动脚本的范围仅限删除重复 dotenv 解析，符合新增 Requirement 边界。
- 未修改终端会话 CWD 的独立 `~` 展开，符合 Non-goal。

## Risks

1. `HOMEDRIVE` 与 `HOMEPATH` 仅在均非空时组合；不完整环境会继续回退到 `HOME` 或返回错误。
2. 没有在真实 Windows 打包 ZIP 中手工启动 Agent；shell fixture 证明了 pre-Go 边界，但仍需在 D: 盘 Cygwin 中确认真实 binary 写入位置。

## Incomplete items

1. 端到端手工验收：从 D: 盘 Windows ZIP 以 Cygwin `bash termbridge.sh` 启动，确认 state、SQLite、logs 位于 `%USERPROFILE%\\.termbridge`，且不存在 `D:\\c\\Users\\wangm25`。

## Conclusion

**本需求范围内验证通过。**

launcher 已停止重复 dotenv 加载；相关 shell fixture 和 `cmd/termbridge/app` / config Go tests 均通过。Cygwin-style `HOME` 不再能在应用启动前将 profile 的 literal `~/.termbridge` 变为 `/c/...`，随后 Windows config resolver 会使用原生 `USERPROFILE` 并生成 `C:\\Users\\wangm25\\.termbridge`。仍需执行真实 D: 盘 Windows ZIP + Cygwin 启动验收。
