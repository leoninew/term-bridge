# 构建版本元数据需求
最后修改时间: 2026-08-17

## Review status

Accepted

## Flow mode / Stage

轻量模式 / light；需求 / Requirement 已接受。该文档记录当前版本模型。

## Background

项目版本需要在 CLI、浏览器 runtime config 与发布产物间保持一致，且不能被运行时 YAML 或 `.env` 配置伪造或覆盖。发布版本以仓库根目录的 `VERSION` 文件记录，并由受控的版本计算命令同步到编译和前端元数据。

## Goal

1. 使用根目录 `VERSION` 记录 SemVer package 版本；`task version:apply` 从 Git 历史计算版本并同步更新 `VERSION`、Go `Version` 变量和 `web/package.json`。
2. 使 `task build`、`task package`、`task docker` 直接编译已同步的源码元数据，不再通过 linker flags 或环境变量注入版本、commit 或构建时间。
3. 提供 `termbridge version` 子命令，仅输出版本和运行时信息，不启动 Agent 或 Cloud 服务。
4. 为三个 portable package 使用带版本号的目录和 ZIP 名称，并在包内携带 `VERSION`。
5. 使首页 local dashboard 读取服务端注入的 Go Version，而不是前端 package 版本或占位文案。

## Non-goal

1. 不将版本作为 Viper/YAML/运行时 `.env` 配置项。
2. 不再向二进制暴露 commit 或构建时间，也不保留 Git tag、commit 距离或 dirty 标识作为构建版本格式。
3. 不修改后端业务 API 或认证逻辑。
4. 不创建或移动 Git tag，不执行 Git 提交、推送或发布。

## User scenarios

1. 开发者执行 `task version` 查看根据 Git 历史计算出的版本，并执行 `task version:apply` 将该版本同步到全部跟踪的消费者。
2. 开发者执行 `task build`、`task package` 或 `task docker` 时，产物使用已提交的 Go `Version`；portable package 名称为 `termbridge-v<version>-<platform>.zip`。
3. 用户运行 `termbridge version` 时，获得版本和运行时信息；local dashboard 显示服务端二进制 Version。

## Acceptance

- [x] 根目录 `VERSION` 仅保存 SemVer package 版本，并作为 portable package 命名来源。
- [x] `task version:apply` 同步更新 `VERSION`、Go `Version` 变量和 `web/package.json`。
- [x] build、package、docker 不再接收 linker metadata 或构建环境变量。
- [x] `termbridge version` 是正式版本查询子命令，且不接受无关参数。
- [x] Dockerfile 与 Dockerfile.cn 编译仓库中已同步的 Go 版本变量。
- [x] 浏览器 runtime config 与 local dashboard 显示 Go 二进制 Version。
- [x] `.env.example` 与 `configs/config.yaml` 不再记录已移除的 build metadata 配置。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

1. `scripts/version-calc.py` 负责从 Git 历史计算 SemVer；只有显式执行 `task version:apply` 才会改写跟踪的版本文件，打包时不再重新派生版本。
2. `VERSION` 是 package 命名和包内版本记录；`version:apply` 将其与 Go `Version` 变量、`web/package.json` 保持一致。
3. 版本信息的唯一运行时来源是 Go 二进制 `internal/shared/common/utils/version`；服务端将 Version 注入 `window.__CONFIG__`。
4. `termbridge version` 不再输出 commit 或构建时间。

## Risk

1. 发布 CI 使用 `VERSION` 生成制品文件名、使用 Git tag 命名 GitHub Release；当前尚未自动校验两者相等。发布前必须确认 tag 为 `v$(cat VERSION)`，否则可能生成 tag 与制品版本不一致的 Release。
2. `VERSION`、Go `Version` 与 `web/package.json` 均为跟踪文件；跳过 `task version:apply` 的手工修改可能使它们漂移。
3. Docker daemon 不可用时，仍需在可用环境中构建两个 Dockerfile 并执行 `termbridge version`，验证镜像包含已同步版本。

## User review notes

- 2026-07-14：用户要求首页项目版本优先使用 Go 后端运行时版本，而不是独立前端版本。
- 2026-07-14：用户要求版本查询使用 `version` 子命令，不使用 `--version`，并要求验证使用 `go run`，避免误启动服务或调用无扩展名 Windows 文件。
- 2026-08-17：版本模型改为由 `VERSION` 及受控的 `version:apply` 同步管理，不再将 Git commit、构建时间或 dirty 状态注入二进制。
