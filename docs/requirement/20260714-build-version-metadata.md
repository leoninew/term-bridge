# 构建版本元数据需求
最后修改时间: 2026-07-14 10:51:40

## Review status

Accepted

## Flow mode / Stage

轻量模式 / light；需求 / Requirement 已接受。该文档补充记录本次已经实施的版本元数据变更。

## Background

项目此前的 Go 二进制版本变量只能使用默认值或由调用方手工传入。首页项目版本、CLI 与发布产物需要共享同一份可追溯的构建身份，且版本信息不能被运行时 YAML 或 `.env` 配置伪造或覆盖。

## Goal

1. 将 Version、Commit、BuildTime 通过 Go linker flags 注入 `termbridge` 二进制，并使本地构建、portable package 与 Docker 构建使用同一套元数据。
2. 提供 `termbridge version` 子命令，输出二进制的版本、短 commit、构建时间和运行时信息，不启动 Agent 或 Cloud 服务。
3. 按 Git 常见实践自动派生构建版本：release tag、tag 后提交、无 tag 的开发提交以及脏工作区均可区分。
4. 使首页 local dashboard 读取服务端注入的 Go Version，而不是前端 package 版本或占位文案。

## Non-goal

1. 不将构建版本作为 Viper/YAML/运行时 `.env` 配置项。
2. 不修改后端业务 API 或认证逻辑。
3. 不创建或移动 Git tag，不执行 Git 提交、推送或发布。
4. 不把脏工作区构建标记为正式 release。

## User scenarios

1. 开发者执行 `task build`、`task package` 或 `task docker` 时，产物携带当前 Git commit 与 UTC 构建时间。
2. 开发者在无 release tag 的提交上构建时，版本带有 `dev-<提交数>-g<短hash>`；有本地修改时追加 `-dirty`。
3. 发布提交位于有效 `vX.Y.Z`（可含 prerelease）tag 时，产物使用该 tag；tag 后提交带 commit 距离和短 hash。
4. 用户运行 `termbridge version` 时，只获得版本文本；local dashboard 显示服务端二进制 Version。

## Acceptance

- [x] `Taskfile.yml` 为 build、package、docker 统一传递 Version、Commit、BuildTime。
- [x] Commit 固定从当前 Git HEAD 计算为 12 位短 hash。
- [x] BuildTime 默认使用 UTC `YYYYMMDD-HHMMSS`。
- [x] Git 版本派生覆盖精确 tag、tag 后提交、无 tag、dirty 和显式版本覆盖。
- [x] `termbridge version` 是正式版本查询子命令，且不接受无关参数。
- [x] Dockerfile 与 Dockerfile.cn 接收并编译相同的构建元数据。
- [x] 浏览器 runtime config 与 local dashboard 显示 Go 二进制 Version。
- [x] `.env.example` 和 `configs/config.yaml` 明确 build metadata 不是运行时配置。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

1. Version 由 `scripts/build-version.sh` 统一从 Git 派生；`TERMBRIDGE_BUILD_VERSION` 仅作为 CI 或源码归档无法读取 Git 元数据时的受控覆盖。
2. Commit 不允许环境变量覆盖，固定使用 `git rev-parse --short=12 HEAD`，确保 Version 与 Commit 不会相互背离。
3. 构建时间可以使用 `TERMBRIDGE_BUILD_TIME` 覆盖，否则由 UTC 当前时间生成。
4. 有未提交文件的自动派生版本追加 `-dirty`。
5. 版本信息的唯一运行时来源是 Go 二进制 `internal/shared/common/utils/version`；服务端将 Version 注入 `window.__CONFIG__`。

## Risk

1. Git 历史中当前尚无 release tag，因此当前开发构建会使用 `dev-<提交数>-g<短hash>-dirty` 形式；首次发布前需建立不可变 SemVer tag 规则。
2. Docker daemon 在本机不可用，Dockerfile 的 build-arg 传递已通过 Taskfile dry-run 与静态审查确认，但未运行镜像级验证。
3. 工作区还存在本任务之外的前端布局改动，以及 `web/.env.development` 已有尾随空白；该问题不属于版本元数据逻辑。

## User review notes

- 2026-07-14：用户要求首页项目版本优先使用 Go 后端运行时版本，而不是独立前端版本。
- 2026-07-14：用户要求版本查询使用 `version` 子命令，不使用 `--version`，并要求验证使用 `go run`，避免误启动服务或调用无扩展名 Windows 文件。
- 2026-07-14：用户要求 Commit 固定读取 Git HEAD 短 hash，BuildTime 使用 UTC 时间。
- 2026-07-14：用户要求按主流 Git tag 实践自动派生 `TERMBRIDGE_BUILD_VERSION`。
