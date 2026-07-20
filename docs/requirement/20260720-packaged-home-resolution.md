# 打包运行时用户目录解析

最后修改时间: 2026-07-20 08:35:37

Review status: Accepted

## Mode

light / 轻量模式

## Background

提交 `91798176` 将打包 PrefLite 运行时的状态、SQLite 和日志位置设为 `~/.termbridge`，以避免升级覆盖安装目录内的持久化数据。

配置层此前通过 `os.UserHomeDir()` 展开 `~`。该 API 在 Windows 使用 OS 特定的 home 查找（例如 `%USERPROFILE%`），可能与打包/Cygwin 进程显式注入的 `HOME` 不一致，导致运行时写入错误用户目录。

## Goal

1. 配置路径中的 `~` 基于进程环境变量解析用户目录。
2. Windows 优先使用原生用户目录变量 `USERPROFILE`，再回退 `HOMEDRIVE` 与 `HOMEPATH`，最后才使用 `HOME`；非 Windows 使用 `HOME`。
3. 使用 `filepath.Join` 将 home 与 `.termbridge` 及其子路径拼接为平台原生路径。
4. 为运行时 state、SQLite、日志三类 `~/.termbridge` 路径建立回归覆盖。

## Non-goal

1. 不改变 `scripts/package/.env.preflite` 中已正确声明的 `~/.termbridge` 配置。
2. 不在 `termbridge.cmd` 或 `termbridge.sh` 中复制 home 查找逻辑。
3. 不调整终端会话 CWD 的独立 `~` 展开逻辑；其不在本次打包配置加载链路内。
4. 不修改相对路径、绝对路径或 cwd 相对路径的既有语义。

## User scenarios

### Scenario 1：打包 Windows/Cygwin 环境

Windows 进程同时具有 Cygwin 格式的 `HOME=/c/Users/...` 与原生 `USERPROFILE=C:\\Users\\...` 时，`~/.termbridge` 解析到 `<USERPROFILE>/.termbridge`，并用于 state、SQLite、日志。

### Scenario 2：无 HOME 的 Windows 环境

`HOME` 缺失时，配置依次使用 `USERPROFILE`，或 `HOMEDRIVE` 与 `HOMEPATH` 的组合；若所有来源都不可用，配置加载返回明确错误。

## Acceptance

1. `resolveConfigPath` 不再直接通过 `os.UserHomeDir()` 处理 `~`。
2. Windows 下 `HOME` 与 `USERPROFILE` 冲突时，state、SQLite、日志均使用 `USERPROFILE/.termbridge`。
3. Windows 的 `USERPROFILE` 与 `HOMEDRIVE` + `HOMEPATH` 回退路径、以及非 Windows 的 `HOME` 路径有单元测试覆盖。
4. 缺失全部支持的环境变量时，home 解析返回错误。
5. 现有 cwd 相对和绝对路径测试继续通过。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

1. 流程模式：light / 轻量模式。
2. 用户要求“记录问题，继续实现”，因此本 Requirement 直接标记为 `Accepted` 并进入 Implementation / 实现。
3. 发现 Windows Go 二进制在 Cygwin 下接收的 `HOME` 可为 `/c/Users/...`，不能直接作为 Windows `filepath` 根目录；因此优先级调整为：Windows：`USERPROFILE` → `HOMEDRIVE` + `HOMEPATH` → `HOME`；非 Windows：`HOME`。

## Risk

1. `HOMEDRIVE` 与 `HOMEPATH` 是否能组成有效路径受宿主环境影响；仅在两者均非空时使用，否则显式报错而非猜测路径。
2. 终端会话 CWD 的 home 展开仍遵循其当前独立行为；本轮不将其静默纳入配置回归修复范围。
