# M4 Local Product Surface 需求
最后修改时间: 2026-06-18 12:48:50

Review status: Draft

## Background

M1 已建立 Go CLI skeleton，M2 已完成真实 PTY Command Runner MVP，M3 已引入 `Workspace` / `Session` runtime model、文件系统持久化、bounded history、`termbridge exec` / `workspace` / `session` 基础命令和默认配置文件加载机制。

Roadmap 中 M4 的定位是：

```text
M4: Local Product Surface: CLI first, GUI optional container
```

也就是在不引入正式 Web、不引入 Gateway、不提前建设 daemon 的前提下，把本地 CLI 产品面打磨到可以日常使用，并为未来 GUI container 提供清晰边界。M4 的核心不是再扩展 runtime 深层模型，而是把 M1-M3 已具备的能力变成稳定、清晰、可理解、可诊断的本地产品体验。

当前已确认的上游路线约束：

1. 本地阶段 `termbridge` CLI 是唯一一等入口。
2. M3 后执行命令入口为：

   ```text
   termbridge [options] exec [exec options] -- <command...>
   ```

3. `Workspace` 与 `Session` 是显式领域概念，一个 Workspace 管理多个 Session。
4. 本地状态持久化在 effective cwd 下的 `.termbridge/<workspace>/<session>`。
5. 默认配置来自打包的 `termbridge.default.yaml`，用户通过 `.termbridge.yaml` 覆盖。
6. GUI 若进入 M4 范围，也只能是 CLI/runtime 的容器，不能拥有独立 PTY、Process、History、Workspace lifecycle。
7. Web terminal / Gateway 不属于 M4 正式产品面。

## Goal

M4 的目标是定义并实现本地产品面，使用户可以仅通过 CLI 完成本地主要使用，并清楚理解配置、执行、状态查询、历史/日志定位和故障排查路径。

M4 应聚焦：

1. CLI help / usage / error message 的产品化打磨。
2. 默认配置与用户配置的可发现性和可解释性。
3. Workspace / Session list 的可用性提升，例如字段清晰、排序合理、状态可理解、路径可定位。
4. 日志和 history 的可发现性：用户能从 CLI 输出或命令中找到相关文件。
5. recent / last / inspect 类本地诊断能力的取舍和命名分析，避免和现有 `workspace` / `session` 命令混乱。
6. 常见运行路径的日常使用体验，例如 `claude` / `codex` / `pwsh` / `npm run dev`。
7. GUI optional container 的边界需求分析：如果 M4 选择进入 GUI，只定义容器能力和 runtime 复用边界，不实现第二套 Agent。
8. 为 M5 hardening 留出清晰边界：M4 只把本地产品面做清楚，不承诺完成所有复杂进程树、长时间压力、真实 Agent TUI hardening。

## Non-goal

M4 不包括：

1. 不实现 Gateway、Gate auth、Device Registry、Workspace Registry 或跨设备访问。
2. 不把 local Web UI 作为正式本地产品面。
3. 不实现正式 Web terminal。
4. 不实现常驻 daemon 作为本地一等入口。
5. 不实现 detach / reattach 产品化。
6. 不实现 multi-attach 产品化。
7. 不让 GUI 独立拥有 PTY lifecycle、Process lifecycle、History lifecycle、Interrupt / kill strategy 或 Workspace ownership。
8. 不完成 M5 级 runtime hardening，例如复杂进程树 cleanup、长期压力测试、backpressure 全面治理、真实 Claude Code / Codex 长时间交互稳定性证明。
9. 不迁移存储到 SQLite；M4 仍基于 M3 的文件系统 runtime model。
10. 不执行 `git add`、`git commit`、`git push` 或其他 git 写操作。

## User scenarios

### 场景一：用户查看如何使用 TermBridge

作为新用户，我运行：

```text
termbridge --help
termbridge exec --help
```

我应能明确看到：

- 如何运行命令。
- 根级 options 与子命令 options 的作用域。
- 如何指定 cwd。
- 如何查看 workspace / session。
- 配置文件读取位置。
- 常见示例。

帮助信息不能继续展示已经废弃的 M2 旧入口：

```text
termbridge [options] -- <command...>
```

除非是作为错误迁移提示出现。

### 场景二：用户运行常见本地命令

作为本地用户，我希望以下命令路径清晰可用：

```text
termbridge exec -- claude
termbridge exec -- codex
termbridge exec -- pwsh -NoLogo
termbridge --cwd D:\project exec -- npm run dev
```

TermBridge 应保持 PTY/TUI 显示正确，返回用户命令 exit code，并在异常时输出可理解的 TermBridge 错误。

### 场景三：用户配置默认行为

作为用户，我希望能理解默认配置来自哪里，以及如何覆盖：

```text
.termbridge.yaml
```

用户应能发现或参考默认配置，例如：

```text
configs/termbridge.default.yaml
```

M4 应明确配置文档或 CLI 输出是否需要展示以下信息：

- effective config file path。
- 默认配置与用户覆盖关系。
- `log.dir`。
- `runtime.state_dir`。
- `history.max_lines` / `history.max_bytes` / `history.max_line_bytes`。

### 场景四：用户查看 Workspace / Session 列表

作为用户，我运行：

```text
termbridge workspace
termbridge session
```

输出应能帮助我理解：

- 当前读取的是哪个 state dir。
- Workspace 来自哪个路径。
- Session 属于哪个 Workspace。
- Session 的 command、cwd、state、exit code、updated time。
- 日志或 history 如何进一步定位。

M4 需要分析是否继续使用纯表格，是否增加可选过滤或详情命令，以及是否需要避免宽输出在窄终端中不可读。

### 场景五：用户排查一次失败的执行

作为用户，当命令运行失败或 TermBridge 自身报错时，我希望错误输出能说明：

- 是用户命令退出，还是 TermBridge runtime/config/persistence 出错。
- cwd 是什么。
- command 是什么。
- log path 在哪里。
- session id 或 session 记录如何查找。

M4 应避免把所有失败都压缩成模糊错误，也不能把用户命令非零退出误报成 TermBridge 自身错误。

### 场景六：用户想找到最近一次会话

作为日常使用者，我可能想快速找到最近一次 `claude` 或 `pwsh` 会话，查看状态、exit code、log/history 位置。

M4 需要分析是否引入：

```text
termbridge session --recent
termbridge session --last
termbridge session inspect <session-id>
termbridge recent
```

或继续只增强现有 `termbridge session` 输出。命名必须遵循 CLI-first 且为后续 Gateway/GUI 留出空间，不能为了短期方便制造语义债务。

### 场景七：GUI 作为可选容器

作为未来 GUI 用户，我希望 GUI 可以：

- 选择 cwd。
- 选择或输入 command。
- spawn / supervise `termbridge` CLI 或复用同一 CLI runtime package。
- 承载 terminal surface。
- 展示 exit code、session id、log path、history path。
- 保存 UI preference。

但 GUI 不能：

- 直接创建 PTY。
- 直接拥有 Process lifecycle。
- 直接维护 Workspace state。
- 直接实现 History buffer。
- 直接实现 interrupt / kill 策略。

M4 若不实际实现 GUI，也必须把边界记录清楚，避免后续产品面分裂。

## Acceptance

M4 Requirement / Spec / Plan 完成后应满足：

- [ ] 存在 M4 requirement 文档，明确本地产品面的目标、边界、用户场景、验收标准、风险和未决事项。
- [ ] M4 明确当前流程为 strict / 严格模式，并继续遵守 Requirement → Spec → Plan → Implementation → Verification。
- [ ] Requirement 明确 M4 是 Local Product Surface，不是 Gateway、正式 Web、daemon、detach/reattach 或 M5 hardening。
- [ ] Requirement 明确 CLI 是本地唯一一等入口。
- [ ] Requirement 明确 GUI 若进入范围，只能是 CLI/runtime container，不能拥有第二套 Agent runtime。
- [ ] Requirement 识别 M3 后 CLI 入口为 `termbridge [options] exec -- <command...>`，不回退到 M2 旧入口。
- [ ] Requirement 覆盖 CLI help、error message、config discoverability、workspace/session list、log/history discoverability、recent/inspect 命令分析等产品面问题。
- [ ] Requirement 记录哪些事项必须在 Spec 阶段设计，哪些事项留到 M5 hardening。

M4 实现完成后应满足的初步验收方向：

- [ ] `termbridge --help` 与 `termbridge exec --help` 清楚说明当前命令模型。
- [ ] 旧入口 `termbridge -- <command...>` 的错误提示能引导用户使用 `termbridge exec -- <command...>`。
- [ ] CLI 错误能区分 usage/config/runtime/user command exit 的语义。
- [ ] 用户能发现 effective config、默认配置和用户配置覆盖规则。
- [ ] 用户能通过 CLI 找到 session 对应的 log/history/state 位置。
- [ ] `termbridge workspace` / `termbridge session` 的输出对日常排查足够可读。
- [ ] 如新增 recent/inspect/logs/config 类命令，命名和参数边界清晰，并与 `workspace` / `session` 模型一致。
- [ ] 不破坏 M3 runtime model 和已存在的 `exec` / `workspace` / `session` 行为。
- [ ] 相关 Go 测试和项目约定检查通过。
- [ ] 不执行 git 写操作。

## Open questions

这些问题需要在 Spec / 规格阶段继续细化，当前不阻塞 Requirement 草稿形成：

1. M4 是否实际实现 GUI container，还是只完成 CLI product surface 并记录 GUI 边界？
2. 是否需要新增 `termbridge config` 或 `termbridge config show`，用于展示 effective config 与默认/用户配置来源？
3. 是否需要新增 `termbridge session inspect <session-id>`，还是先增强 `termbridge session` 表格输出？
4. recent 能力应该作为 `termbridge session --recent`、`termbridge session --last`，还是独立 `termbridge recent`？
5. 是否需要 `--json` 输出？M3 曾倾向于稳定文本表格，并建议 GUI/Gate 复用 runtime package 而非解析 CLI 表格；M4 需要重新评估本地诊断和脚本化需求。
6. logs/history 如何暴露最合适：在 session list 中显示路径、提供 inspect、还是提供 `termbridge logs` 命令？
7. 宽表格在窄终端中如何处理：保留完整列、自动截断、分两行展示，还是提供 detail 命令？
8. 当前默认配置文件 `configs/termbridge.default.yaml` 是否需要被 README 或 CLI help 明确引用？
9. 是否需要 shell completion，还是作为 optional deliverable 留到后续？
10. 是否需要针对 Claude Code / Codex TUI 的人工 checklist 纳入 M4，还是全部留到 M5 hardening？
11. M4 是否应更新 README 使用方式，使其与 M3 的 `exec` 入口一致？

如果用户要求继续推进，这些事项可进入 Spec / 规格阶段，不要求在 Requirement 阶段一次性定死。

## Decisions

当前已明确：

1. 当前流程为 strict / 严格模式。
2. 当前阶段为 Requirement / 需求。
3. 本次是 M4 相关任务分析，默认创建新的 M4 requirement 文档。
4. M4 主题为 `Local Product Surface: CLI first, GUI optional container`。
5. CLI 是本地唯一一等产品入口。
6. M4 不回退到 M2 的 `termbridge [options] -- <command...>`，以 M3 的 `termbridge [options] exec -- <command...>` 为基础。
7. GUI 若纳入 M4，也只能是 CLI/runtime container，不能拥有独立 runtime。
8. Web terminal / Gateway 不进入 M4 正式产品面。
9. M5 hardening 关注的复杂可靠性问题不应被 M4 伪装为已完成。
10. 默认配置和用户配置覆盖是 M4 产品面的一部分，用户应能理解和发现。

## Risk

### 风险一：M4 被 GUI 或 Web 范围膨胀吞掉

M4 的核心是本地产品面，尤其是 CLI-first。如果过早实现 GUI 或 local Web，容易偏离已经确认的路线，并制造第二套 runtime。

### 风险二：CLI 命令面继续漂移

M3 已把执行入口切到 `exec`。M4 如果在 help、README、错误提示或新增命令中混用旧入口，会让用户心智混乱，也会影响后续 GUI/Gate 复用。

### 风险三：列表命令变成不可维护的临时表格

`workspace` / `session` 输出如果只为当前终端宽度临时拼接，后续 recent、inspect、GUI、Gate 复用都会变困难。M4 需要明确表格与领域视图边界。

### 风险四：把 M5 hardening 提前承诺为 M4 完成

Claude Code / Codex 长时间真实交互、复杂 Ctrl+C、子孙进程 cleanup、backpressure、压力测试都属于 M5 重点。M4 可以改善日常体验和可诊断性，但不能把这些可靠性风险标成已完全解决。

### 风险五：配置可发现性不足

默认配置已经从代码常量迁移为 YAML，但如果 README/help/diagnostic 输出没有同步，用户仍然不知道默认值来自哪里、如何覆盖、如何确认 effective config。

## User review notes

待用户 review。
