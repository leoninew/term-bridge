# 引入 Air 为后端提供热加载验证
最后修改时间: 2026-06-20 11:26:50

Review status: Accepted

## Requirement alignment

依据 `docs/requirement/20260620-backend-air-hot-reload.md` 核对。

- 为后端本地开发引入 `air` 热加载能力：已通过新增 `.air.toml` 实现。
- 引入 `justfile` 作为项目本地开发命令入口：项目已有 `justfile`，本次复用现有入口，没有新建第二套入口。
- 通过 `just` 命令启动后端热加载：现有 `backend` recipe 已改为执行 `air`。
- 保持后端构建、测试和生产运行方式不受影响：未修改 Go 业务代码、CLI 行为、配置加载逻辑或生产启动逻辑。
- 用户后续明确要求“不引入新命令”：最终实现未新增 just recipe，仅修改已有 `backend` recipe。

## Spec alignment

不适用。light / 轻量模式未创建独立 spec 文档。

## Plan alignment

不适用。light / 轻量模式未创建独立 plan 文档；按 requirement / 需求核对。

## Actual diff summary

- 新增 `.air.toml`：定义 Air 根目录、临时目录、Go build 命令、运行参数、监听扩展名和排除目录。
- 修改 `justfile`：将已有 `backend` recipe 从 `go run cmd/termbridge/main.go web --host localhost --port 9010 --dev` 改为 `air`。
- 修改 `README.md`：补充 Air 开发依赖，并说明 `just backend` 和 `just frontend` 开发入口。
- 更新 `docs/requirement/20260620-backend-air-hot-reload.md`：标记为 `Accepted`，并纳入 justfile 范围。

## Expected vs actual changed files

| File | Expected | Actual | Notes |
| --- | --- | --- | --- |
| `.air.toml` | Yes | Added | Air 后端热加载配置。 |
| `justfile` | Yes | Modified | 复用现有 `backend` recipe，未新增命令。 |
| `README.md` | Yes | Modified | 记录 Air 依赖和开发入口。 |
| `docs/requirement/20260620-backend-air-hot-reload.md` | Yes | Added | SpecFlow light requirement 文档。 |
| `.termbridge.yaml` | No | Modified in working tree | 非本轮直接编辑；当前 diff 为 `log.level: error -> info`，应由用户确认是否保留或回滚。 |

## Acceptance criteria checklist

- [x] 仓库中存在适用于后端的 Air 配置：`.air.toml` 已新增。
- [x] 仓库根目录存在 `justfile`，并提供后端热加载入口：`just backend` 执行 `air`。
- [x] Air 配置的 build/run 命令与当前 Go 后端入口匹配：build 使用 `./cmd/termbridge`，运行参数为 `web --host localhost --port 9010 --dev`，与原 `backend` recipe 保持一致。
- [x] 热加载配置避免监听明显不必要目录：排除 `.git`、`.termbridge`、`bin`、`docs`、`logs`、`web`、`tmp`、`vendor`。
- [x] 基础验证命令通过：`go test ./cmd/... ./internal/...` 和 Go build 通过。
- [x] 不引入新 just 命令：`just --list` 中没有新增 recipe。

## Command results

### `just --dry-run backend`

Result: Pass

```text
air
```

### `just --list`

Result: Pass

```text
Available recipes:
    backend
    build
    check
    clean
    exec *args
    frontend
    install
```

### `go test ./cmd/... ./internal/...`

Result: Pass

```text
?   	termbridge-go/cmd/termbridge	[no test files]
ok  	termbridge-go/internal/app	(cached)
ok  	termbridge-go/internal/cli	(cached)
ok  	termbridge-go/internal/config	(cached)
ok  	termbridge-go/internal/errors	(cached)
ok  	termbridge-go/internal/history	(cached)
ok  	termbridge-go/internal/identity	(cached)
ok  	termbridge-go/internal/logging	(cached)
ok  	termbridge-go/internal/process	(cached)
?   	termbridge-go/internal/pty	[no test files]
?   	termbridge-go/internal/pty/gopty	[no test files]
ok  	termbridge-go/internal/runner	(cached)
ok  	termbridge-go/internal/session	(cached)
ok  	termbridge-go/internal/state	(cached)
ok  	termbridge-go/internal/terminalproto	(cached)
?   	termbridge-go/internal/version	[no test files]
ok  	termbridge-go/internal/webserver	(cached)
ok  	termbridge-go/internal/webterminal	(cached)
ok  	termbridge-go/internal/workspace	(cached)
```

### `go build -o /tmp/termbridge-air-check ./cmd/termbridge`

Result: Pass

No output.

### `air -h`

Result: Pass

Air CLI 可用；当前环境中的 Air 版本检查结果为：

```text
__    _   ___
 / /\  | | | |_)
/_/--\ |_| |_| \_ v1.65.3, built with Go go1.25.11
```

## Missed or expanded scope

- 未实际长时间运行 `air` 监听进程；验证集中在配置、入口、构建和测试。原因是热加载进程是前台常驻进程，不适合在验证文档生成流程中保持运行。
- 工作区存在 `.termbridge.yaml` 的未预期改动，内容为 `log.level: error -> info`。这不属于本次 Air/justfile 需求范围，且不是本轮直接编辑的文件。

## Risks

- `air` 不是 Go module 依赖；开发者需要按 README 中的外部开发依赖自行安装。
- `.air.toml` 使用 Air 当前支持的 `args_bin` 配置。若未来 Air 版本移除该字段，需要改用当时推荐的 entrypoint/full_bin 形式。
- `.termbridge.yaml` 的本地改动若被一并提交，会扩大本次交付范围。

## Incomplete items

- 需要用户决定是否保留或回滚 `.termbridge.yaml` 的 `log.level` 改动。

## Conclusion

本次 Air 后端热加载接入已通过轻量验证：`just backend` 复用现有命令名并切换为 Air，Air build/run 参数与原后端 dev 启动入口一致，Go 测试和构建通过，且未新增 just 命令。唯一需要用户处理的是工作区中与本需求无关的 `.termbridge.yaml` 改动。
