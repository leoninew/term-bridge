# Session 交互式 Shell 环境隔离

最后修改时间: 2026-07-14 13:17:55

Review status: Accepted

## Flow mode / Stage

轻量模式 / light；需求 / Requirement 已接受；实现 / Implementation 与验证 / Verification 已完成。

## Background / 背景

用户在 Browser workbench 用 `bash`（或本机等价 shell，如 `cygwin` 包装启动的 bash）新建交互式会话后，输入一条失败命令（例如错误的 `find` 参数），整条 Session 会立刻变成 `stopped`。

根因排查结果：

1. TermBridge Session 生命周期正确绑定在“用户根进程”上：根进程退出 → `state=stopped` / `reason=user_process_exited`。
2. 创建 session runtime 时，PTY 进程环境来自 `os.Environ()`，完整继承 Agent 进程环境。
3. 当 Agent 从已开启 `set -e` / `set -u` 的父 shell 启动时，环境中常带有：

```text
SHELLOPTS=braceexpand:errexit:hashall:igncr:interactive-comments:nounset
```

4. bash 启动时读取 `SHELLOPTS`，交互式 shell 也会启用 `errexit`（等价 `set -e`）。
5. 任意非 0 退出的用户命令（`false`、非法 `find` 等）会让 bash 直接退出，Session 随之 stop。

本地用项目 PTY 复现：

- 保留 `SHELLOPTS`：`false` / 非法 `find` → bash 立即退出 code=1
- 剥离 `SHELLOPTS` 后：同样命令 → bash 继续存活并打印后续输出

因此问题不是“非法命令被 TermBridge 判停”，而是 **交互式 shell 错误继承了 Agent 父 shell 的 shell option 环境变量**。

## Goal / 目标

1. 新建或 rerun 的终端 Session 在启动用户进程时，不再把会影响交互式 shell 语义的父 shell option 环境变量原样传给 PTY 进程，至少剥离 `SHELLOPTS` 与 `BASHOPTS`。
2. 在 Agent 自身环境带有 `errexit`/`nounset` 的情况下，交互式 `bash` Session 执行失败命令后仍保持 `running`，直到用户主动 exit/close 或进程自然结束。
3. 不改变 Session = 用户根进程 的生命周期语义：根进程真正退出后，Session 仍应进入 `stopped` / `failed` 等既有终态。
4. 用可自动化测试覆盖“继承 SHELLOPTS 时失败命令不应直接杀死交互 shell”的行为，或至少覆盖环境过滤逻辑。

## Non-goal / 非目标

1. 不把失败命令本身标记为 Session failed；普通命令非 0 仍只是 shell 内结果，除非根进程退出。
2. 不修改前端 Session 状态展示、close/rerun 交互，除非为实现后端修复所必需。
3. 不重写用户 `.bashrc` / `.bash_profile`；用户主动配置的 `set -e` 仍可在 shell 内生效。
4. 不拦截或改写用户输入命令，也不对“非法命令”做产品层特殊判定。
5. 不做完整 shell 环境净化框架（例如全面过滤 PATH/TERM/locale）；本轮只处理 shell option 继承导致的交互语义污染。
6. 不改变一次性命令 Session（command 本身就是 `find ...` 这类会结束的进程）跑完后 stop 的既有行为。

## User scenarios / 用户场景

1. 用户从带 `set -e` 的 Cygwin/bash 启动 Agent，再在 workbench 新建 command=`bash` 的 Session；输入 `false` 或错误 `find` 后，终端仍回到 prompt，Session 保持 `running`。
2. 用户在同一交互 shell 中继续输入后续命令，可正常执行；主动 `exit` 或关闭 Session 后，Session 进入 `stopped`。
3. 用户新建一次性命令 Session（例如 command 为 `go test ./...`），命令结束后 Session 仍按既有语义 stop。
4. 开发者运行相关自动化测试时，可验证环境过滤与/或“失败命令不退出交互 shell”行为。

## Acceptance / 验收标准

- [ ] Session 启动路径（create / rerun）在构造 `ProcessSpec.Env` 时剥离 `SHELLOPTS` 与 `BASHOPTS`。
- [ ] 在 Agent 环境包含 `SHELLOPTS=...errexit...` 时，交互式 bash Session 执行失败命令后根进程仍存活，Session 保持 `running`。
- [ ] 根进程真正退出（`exit`、close、进程崩溃）后，Session 仍正确进入终态并记录 exit。
- [ ] 一次性命令 Session 结束后的 stop 行为不变。
- [ ] 有自动化测试覆盖环境过滤或交互 shell 不因失败命令退出的关键行为。
- [ ] 相关 Go 测试通过；如改动触及既有 runner/registry 测试，一并保持绿色。

## Open questions / 待定问题

1. 过滤范围是否只做 `SHELLOPTS`/`BASHOPTS`，还是顺带剥离其他明显“父 shell 状态”变量（例如部分环境下的 `SHELLOPTS` 变体）？
   - 默认建议：本轮仅 `SHELLOPTS` + `BASHOPTS`。
2. 过滤应放在哪一层？
   - 选项 A：`terminal.Registry` 构造 env 时过滤（靠近 session 产品语义）
   - 选项 B：`process.NewSpec` / PTY Manager 启动前统一过滤（所有 PTY 启动一致）
   - 默认建议：B 或靠近 `ProcessSpec` 的公共构造点，避免 create/rerun/CLI exec 路径遗漏。
3. CLI `termbridge exec -- bash` 是否纳入同一修复？
   - 默认建议：是，凡走 PTY/`ProcessSpec.Env = os.Environ()` 的用户进程启动都应一致。

## Decisions / 决策

1. 问题定性为环境隔离缺陷，不是“非法命令导致 Session 被业务层 stop”。
2. 修复目标是阻止父 shell option 污染交互式用户 shell；不是禁止用户在自己的 shell 配置中启用 `set -e`。
3. light / 轻量模式推进：Requirement → Implementation → Verification。
4. 2026-07-14 用户“开始实现”：接受默认决策——仅剥离 `SHELLOPTS`/`BASHOPTS`；落点为 `ProcessSpec` 构造 + PTY Manager 启动统一过滤；CLI exec 一并覆盖。

## Risk / 假设

1. 假设：本机复现路径（Cygwin bash + `SHELLOPTS` 含 `errexit`/`nounset`）代表真实用户启动 Agent 的常见方式之一。
2. 若某些脚本依赖从父环境继承 `SHELLOPTS`，剥离后行为会变化；对交互终端与一次性命令，通常更接近“干净登录 shell”预期。
3. Windows ConPTY / Cygwin 仍可能存在其他异常退出路径；本需求只覆盖 shell option 继承导致的失败命令退出。
4. 测试若真实拉起 bash，需考虑 Windows/Linux CI 可用性；必要时将“过滤逻辑”做成纯函数单测，交互行为用可跳过的集成测。

## User review notes / 用户审查记录

- 2026-07-14：用户报告 bash 新建会话中敲非法 `find` 后 Session 直接 stop，并确认会话由 bash 启动。
- 2026-07-14：本地复现确认根因是继承 `SHELLOPTS=...errexit...`；剥离后失败命令不再杀死 bash。
- 2026-07-14：用户要求用 SpecFlow light 记录并开始该任务。
- 2026-07-14：用户要求开始实现 / Implementation。
