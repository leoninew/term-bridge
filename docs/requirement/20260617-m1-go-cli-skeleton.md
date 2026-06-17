# M1 Go CLI Skeleton 需求
最后修改时间: 2026-06-17 16:15:22

Review status: Accepted

## Background

TermBridge-go 已确认采用：

```text
Go + github.com/aymanbagabas/go-pty
```

作为 Windows PTY 技术方向。当前路线已收敛为 CLI-first：本地阶段只有一种一等使用入口：

```text
termbridge [options] -- <command...>
```

GUI 是 CLI 的容器；Gateway 阶段才把 Web 作为正式产品入口。

上一轮已经有一部分 M1 代码雏形：

```text
cmd/termbridge/main.go
internal/cli/cli.go
internal/cli/cli_test.go
internal/version/version.go
```

这些已有实现可以继续使用，但 M1 不能只停留在“能解析参数”。在继续实现前，需要先明确 Go CLI Skeleton 的基础技术选择，包括：

```text
CLI contract
configuration
logging
error model
exit code
version/build info
testing boundary
package boundary
```

否则后续进入 M2 PTY Command Runner 时，会把日志、配置、异常、退出码等基础问题混入 PTY runtime 实现，导致技术债。

## Goal

M1 的目标是建立 `termbridge` CLI 的稳定工程骨架，为 M2 PTY Command Runner 提供清晰边界。

M1 完成后应明确并实现：

1. `termbridge [options] -- <command...>` 的 CLI 合同。
2. `--help` / `--version` / `--cwd` 等基础选项。
3. 配置来源、优先级和 fail-fast 规则。
4. 日志格式、输出位置、默认级别和用户可配置方式。
5. 错误分类、用户可读错误信息、debug 信息边界。
6. CLI exit code 约定。
7. version / build info 输出格式。
8. Go package 边界，避免把 CLI、config、logging、runtime 混在一起。
9. 最小测试策略和本地检查命令。

M1 不要求真正通过 PTY 运行用户命令；`termbridge -- <command...>` 可以继续返回“runner not implemented yet”，但必须在行为、错误和日志上符合 M1 约定。

## Non-goal

本需求不包括：

1. 不实现 PTY Command Runner。
2. 不接入 `go-pty` 到正式 runtime。
3. 不实现 Ctrl+C interrupt / close / kill process tree。
4. 不实现 Session / Workspace 模型。
5. 不实现 GUI。
6. 不实现 Web terminal spike。
7. 不实现 Gateway。
8. 不执行 git 写操作。
9. 不创建 verification 文档，除非后续明确进入 Verification / 验证阶段。

## User scenarios

### 场景一：查看帮助

作为用户，我希望运行：

```text
termbridge --help
```

看到清晰的命令格式、选项说明和示例，尤其明确：

```text
termbridge [options] -- <command...>
```

### 场景二：查看版本

作为用户，我希望运行：

```text
termbridge --version
```

看到版本、Go runtime、OS/arch 和必要的 build 信息。

### 场景三：指定 cwd 但暂不运行 PTY

作为用户，我希望运行：

```text
termbridge --cwd D:\project -- claude
```

M1 阶段即使还不运行 PTY，也应完成参数解析、cwd 校验、错误处理和明确提示。

### 场景四：错误输入能 fail fast

作为用户，如果我运行：

```text
termbridge claude
```

或：

```text
termbridge --cwd D:\not-exist -- claude
```

TermBridge 应明确指出错误，不应静默猜测、吞掉参数或继续执行不确定行为。

### 场景五：开发者排查问题

作为开发者，我希望 CLI 有一致的日志和错误模型。用户默认看到简洁错误；调试时可以通过配置或选项看到更多上下文。

## Acceptance

M1 完成后应满足：

- [ ] 存在 M1 requirement 文档，明确 CLI、配置、日志、错误、exit code、测试边界。
- [ ] `cmd/termbridge` 是唯一正式 CLI entrypoint。
- [ ] `termbridge --help` 输出 CLI 合同、选项和示例。
- [ ] `termbridge --version` 输出版本与 runtime 信息。
- [ ] `termbridge [options] -- <command...>` 能解析 command 与 args，且不吞掉用户参数。
- [ ] `--cwd` 默认使用当前目录；显式传入时必须校验目录存在。
- [ ] 缺少 `--`、缺少 command、非法 cwd、未知 TermBridge 选项时 fail fast。
- [ ] 使用 `viper` 建立配置边界：明确配置来源、优先级、`.termbridge.yaml` 自动发现和 fail-fast 规则；不使用 `--config <path>`。
- [ ] 使用 `log/slog` 建立日志边界：支持主流日志级别/格式配置，开发阶段写入 `logs/` 目录。
- [ ] 建立错误模型：区分 usage error、config error、internal error、future runtime error。
- [ ] 建立 exit code 约定。
- [ ] 建立 package 边界，至少覆盖 `internal/cli`、`internal/app`、`internal/config`、`internal/logging`、`internal/version`，必要时包含 `internal/errors`。
- [ ] 提供 `configs/termbridge.example.yaml`，明确 M1 支持的配置 schema。
- [ ] 有最小单元测试覆盖 CLI parsing、cwd 校验、usage error、version/help 行为、Run exit code、unknown config key、日志不污染 stdout 并写入 log file。
- [ ] `go test ./...` 通过。

## Technical decisions

### CLI contract

M1 以以下形式作为规范合同：

```text
termbridge [options] -- <command...>
```

M1 不支持或不鼓励隐式形式：

```text
termbridge <command...>
```

该语法糖可以后续评估，但不能影响当前 `--` 分隔合同。

### Configuration

M1 使用 `viper` 作为配置库，并采用 fail-fast 规则。

配置来源优先级：

```text
environment variables > selected config file > defaults
```

CLI 只暴露 `--cwd` 和命令透传相关选项；日志等细粒度配置不通过 CLI flags 暴露。

M1 最小配置项：

```text
log level
log format
log directory
```

建议环境变量前缀：

```text
TERMBRIDGE_
```

建议开发阶段默认日志目录：

```text
logs/
```

M1 不使用 `--config <path>`。配置文件采用 YAML，并按以下顺序自动发现：

```text
1. effective cwd/.termbridge.yaml
2. <home>/.termbridge.yaml
3. defaults
```

其中 effective cwd 定义为：如果用户传入 `--cwd`，则使用 `--cwd` 指定目录；否则使用当前进程工作目录。

只有文件不存在时才继续查找下一个位置；如果文件存在但读取、解析、校验失败，则立即 fail fast。M1 不做 local + home 配置 merge，只选择第一个存在的配置文件。

fail-fast 规则：

```text
配置文件存在但无法读取 -> 失败
配置文件存在但无法解析 -> 失败
配置项非法 -> 失败
未知关键配置项 -> 失败或明确报错，不能静默忽略
log directory 无法创建或不可写 -> 失败
log level / log format 非法 -> 失败
```

M1 允许没有配置文件，以默认值启动；但只要发现配置文件或用户通过环境变量提供配置项，就必须严格校验。

### Logging

M1 使用 Go 标准库 `log/slog`，按主流实践建立结构化日志边界。

默认行为：

```text
普通用户输出: stdout / stderr 保持简洁
日志实现: log/slog
日志默认级别: info
日志默认格式: text
开发阶段日志目录: logs/
```

M1 支持主流日志配置，但这些配置通过 `.termbridge.yaml` 或环境变量提供，不作为 CLI flags 暴露：

```text
log.level: debug|info|warn|error
log.format: text|json
log.dir: logs
```

日志输出建议：

```text
1. 用户可读错误仍输出到 stderr。
2. 结构化运行日志写入 log directory 下的文件。
3. debug/info/warn/error 使用统一 slog handler。
4. json format 用于后续机器解析和问题上报。
5. text format 用于开发阶段直接阅读。
```

M1 阶段默认写入：

```text
logs/termbridge.log
```

如果日志目录无法创建或不可写，必须 fail fast，不降级为静默无日志。

### Error model

建议定义错误类型或错误分类：

```text
UsageError      -> 用户命令行输入错误
ConfigError     -> 配置读取或校验错误
InternalError   -> TermBridge 自身不可恢复错误
RuntimeError    -> M2 起用户命令运行错误
```

M1 重点覆盖前三类。

用户默认看到简洁错误；debug 日志中可以包含 wrapped error 细节。

### Exit code

建议 M1 约定：

```text
0   success
1   internal error / not implemented / general failure
2   usage error
3   config error
```

M2 起用户命令退出码需要透传，但 M1 先记录该约束：

```text
当 command runner 实现后，用户命令退出码应尽可能成为 termbridge exit code。
TermBridge 自身错误使用保留 exit code，不与用户命令退出混淆。
```

### Version / build info

M1 版本输出至少包含：

```text
termbridge version
GOOS/GOARCH
```

M1 需要预留 build-time 注入点：

```text
git commit
build time
Go version
```

但不要依赖 git 命令在运行时获取这些信息，应通过 build flags 注入。

### Package boundary

建议 M1 包边界：

```text
cmd/termbridge       CLI process entrypoint，尽量薄
internal/cli         参数解析、stdout/stderr、exit code 映射
internal/app         应用编排薄层：配置、日志、未来 runtime 调用
internal/config      viper 配置默认值、读取、校验
internal/logging     slog 初始化和日志选项
internal/version     version / build info
internal/errors      错误分类和 exit code 映射
```

`cmd/termbridge/main.go` 不应直接承载业务逻辑。

`internal/app` 只做 orchestration 薄层，为未来 GUI / Gateway 复用预留空间；M1 不在该层引入 framework 或复杂生命周期。

## Open questions

需要用户关注的未决事项：

当前无阻塞 M1 实现的未决事项。

进入实现时采用以下 assumption：

1. 接受 exit code 约定：`0 success`、`1 general/internal`、`2 usage`、`3 config`。
2. 本轮 M1 将错误分类和 exit code 映射抽出到独立包，为未来 Gateway / GUI 复用预留边界。
3. M1 不使用 `--config <path>`。
4. 配置文件使用 YAML，按 `effective cwd/.termbridge.yaml`、`<home>/.termbridge.yaml`、defaults 的顺序发现。
5. 采纳 `internal/app` 薄层，为 GUI / Gateway 复用预留空间，但禁止框架化。
6. 补齐 build info 注入点、配置样例、unknown config key 测试、Run exit code 测试和日志不污染 stdout 的测试。

已确认：

```text
Configuration 使用 viper，采用 fail-fast 规则。
Logging 使用 log/slog，满足主流配置。
开发阶段日志输出到 logs/ 目录。
其余按主流实践执行。
```

剩余问题不阻塞 requirement 草稿形成，但在进入实现前应确认或由实现文档记录为 assumption。

## Decisions

当前已明确：

1. 本地一等入口是 `termbridge` CLI。
2. 标准命令合同是 `termbridge [options] -- <command...>`。
3. `--cwd` 是 M1 必需选项。
4. M1 不实现 PTY command runner。
5. 已有 CLI skeleton 可以继续使用，但需要补齐配置、日志、错误和 exit code 设计。
6. Configuration 使用 `viper`，采用 fail-fast 规则。
7. Logging 使用 `log/slog`，开发阶段输出到 `logs/` 目录，并通过配置文件/环境变量支持主流 level / format 配置；不暴露日志细粒度 CLI flags。
8. 不暴露 `--config <path>`；配置文件自动发现 `.termbridge.yaml`。
9. 错误分类和 exit code 映射抽出到独立包，为未来 Gateway / GUI 接入预留空间。
10. GUI 不进入 M1。
11. Web / Gateway 不进入 M1。

## Risk

### 风险一：继续直接实现导致基础边界缺失

如果不先明确日志、配置、错误和 exit code，M2 接入 PTY 后会把 runtime failure 和 CLI failure 混在一起。

### 风险二：CLI 参数透传和 TermBridge 选项混淆

`--` 分隔是当前关键合同。实现中必须严格区分 TermBridge 自身选项和用户命令参数。

### 风险三：日志污染用户命令输出

未来 `termbridge -- <command...>` 会把用户命令 stdout/stderr 转发到终端。TermBridge 自身日志必须避免污染用户程序输出，必要时只写 stderr 或 debug 文件。

### 风险四：过早引入复杂配置

M1 只需要建立边界。持久化配置和用户级配置目录可以后续实现，不应阻塞 CLI skeleton。

## User review notes

用户指出：M1 已有实现可以继续使用，但在继续实现前，Go CLI Skeleton 应该先明确日志、配置、异常等技术选择，而不是直接开干。