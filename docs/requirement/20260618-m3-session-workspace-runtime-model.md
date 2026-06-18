# M3 Session / Workspace Runtime Model 需求
最后修改时间: 2026-06-18 10:56:59

Review status: Accepted

## Background

M1 已建立 `termbridge` CLI skeleton，M2 已将 `termbridge [options] -- <command...>` 从 placeholder 切换为真实 PTY Command Runner MVP。随后，早期 `cmd/pty-spike` / `cmd/pty-testprogram` 的低副作用 PTY 验证能力已收敛到 `internal/pty/gopty` 测试中，正式 runtime 不再依赖这些 spike 命令。

M3 需要在 CLI command runner 成立之后，引入可复用的 Session / Workspace Runtime Model，为后续本地产品面、GUI container、Web terminal spike、Gateway 映射和长期 Workspace Runtime Platform 打基础。

用户已进一步明确：M3 必须显式区分 `Workspace` 与 `Session`：

```text
Workspace
  ↓
Session
  ↓
PTY
  ↓
Process
```

其中：

- Workspace 将来会在 Gate 上展示为个人工作区列表。
- 一个 Workspace 管理多个 Session。
- Session 是一次具体会话，带有发起目录、命令、进程、状态、history 和日志关联。

为了给 `recent`、`list`、未来 workspace/session 操作留下 CLI 空间，M3 还需要调整当前运行命令形式，从：

```text
termbridge [options] -- <command...>
```

演进为：

```text
termbridge [options] exec -- <command...>
```

该变化仍保持 CLI-first，但不再把根命令完全让给 command runner。

## Goal

M3 的目标是建立最小但严谨的 Workspace / Session Runtime Model，让 TermBridge 从“能运行一个命令”提升为“能描述、记录、恢复识别、查询和复用运行态”。

M3 完成后应明确并实现或设计：

1. Workspace model：表达未来 Gate 上展示的个人工作区，能管理多个 Session。
2. Session model：表达一次具体会话，包含发起目录、命令、进程、状态、history 和退出记录。
3. CLI command surface：将用户命令执行入口调整为 `termbridge [options] exec -- <command...>`，为 `recent` / `list` / 后续 workspace/session 命令预留空间。
4. Runtime state model：能表达 `starting`、`running`、`stopping`、`stopped`、`failed` 等基础状态。
5. Process record：持久化与进程相关的信息，用于生命周期维护、启动恢复识别和故障排查。
6. Exit record：记录用户命令 exit code、exit reason、started/ended 时间。
7. Command record：记录 cwd、command、args、env 处理策略或摘要。
8. Filesystem persistence：开发模式下持久化到项目目录 `.termbridge/<workspace>/<session>` 结构。
9. Bounded history buffer：先按行数限制，确保至少一屏可用；再按 bytes 限制，避免超长单行导致存储或展示问题。
10. History persistence：将 session history 存储到 `.termbridge/<workspace>/<session>` 下。
11. Logs location：继续沿用当前 effective cwd 下 `logs/termbridge.log`。
12. ULID identity：workspace id 与 session id 均使用 ULID。
13. Workspace derivation：用户进入目录执行命令或通过 `--cwd` 指定目录时，该目录就是 Session 发起目录；Workspace 由该目录稳定映射生成，例如基于路径 hash 建立或复用。
14. Startup recovery scan：启动时加载 `.termbridge/<workspace>/<session>`，读取 metadata 和 process 信息，检查进程是否仍存在且匹配，并在整个生命周期中维护状态。
15. Basic workspace/session commands：先实现 `termbridge workspace` 与 `termbridge session`，分别展示目录映射的 Workspace 列表和会话列表。
16. Configurable history limits：history 默认限制采用主流实践，并支持配置化；默认策略在 Spec 阶段确定。
17. Future compatibility：虽然未来可能迁移到 SQLite，M3 仍必须先完成领域建模，不能只做临时文件堆叠。

## Non-goal

M3 不包括：

1. 不实现完整后台 daemon。
2. 不实现 detach / reattach 产品化。
3. 不实现 multi-attach 产品化。
4. 不实现 GUI。
5. 不实现正式 Web terminal 或 Gateway。
6. 不实现 Gate 上的 Workspace Registry / Device Registry / User model。
7. 不把 storage 直接设计成最终数据库方案；未来可迁移 SQLite，但 M3 先使用文件系统持久化。
8. 不实现完整持久 terminal history 或可无限回放的 history store。
9. 不完成 M5 级别 runtime hardening，例如复杂 process tree cleanup、Agent TUI 长时间稳定性、owner-crash 恢复。
10. 不执行 git 写操作。

## User scenarios

### 场景一：用户通过 exec 正常运行命令

作为 CLI 用户，我运行：

```text
termbridge exec -- claude
termbridge exec -- pwsh -NoLogo
termbridge --cwd D:\project exec -- npm run dev
```

TermBridge 创建或识别当前项目对应的 Workspace，在该 Workspace 下创建一个新的 Session，记录发起目录、命令、进程、状态、history、exit code 和日志位置。

### 场景二：一个 Workspace 管理多个 Session

作为用户，我在同一个项目目录下多次运行不同命令：

```text
termbridge exec -- claude
termbridge exec -- codex
termbridge exec -- pwsh
```

这些 Session 应属于同一个 Workspace，但每个 Session 保留自己的启动目录、命令、状态、history 和退出记录。

### 场景三：不同发起目录形成不同 Session 上下文

作为用户，我可能在不同 cwd 下启动命令。Session 必须记录自己的发起目录，因为会话是有发起目录的。后续 GUI / Gate 展示时，用户能理解某个 Session 来源于哪个 workspace/project path。

### 场景四：用户查看 Workspace 或 Session 列表

作为用户，我希望能查看 Workspace 和 Session，例如：

```text
termbridge workspace
termbridge session
```

M3 先实现这两个命令，用于展示目录映射的 Workspace 列表和会话列表；`recent` / `list` 可作为后续命令命名优化，不作为当前已确认入口。

输出至少应能帮助我判断：

- Workspace id。
- Session id。
- Session 发起目录。
- command / args。
- 当前或最终状态。
- exit code / exit reason。
- 日志位置。

### 场景五：进程内查看所有会话

作为 happy path，TermBridge 启动终端后，进程内可以看到当前生命周期内管理的所有会话，并持续维护这些 Session 的状态。

### 场景六：进程异常退出后的恢复识别

作为维护者，如果 TermBridge 自身进程挂了，下一次启动时应加载 `.termbridge/<workspace>/<session>` 下的记录，识别已有 Session 的状态，并把无法确认仍在运行的记录标记为合理状态或进入恢复/异常识别流程。

M3 不要求恢复 attach 到原 PTY，但必须避免把陈旧 running 状态永久留成 running。

### 场景七：未来 GUI / Gate 展示 Workspace

作为未来 GUI / Gate，它会展示个人 Workspace 列表。每个 Workspace 下有多个 Session。M3 必须把领域模型先立住，使未来 GUI / Gate 不需要重新定义 Workspace / Session 关系。

## Acceptance

M3 完成后应满足：

- [ ] 存在 M3 requirement 文档，明确目标、边界、验收标准、风险和决策。
- [ ] 存在 M3 spec 文档，说明 Workspace / Session model、状态机、history、文件系统存储、启动恢复和 CLI 交互方案。
- [ ] 存在 M3 plan 文档，列出实施步骤、关键文件、验证计划和 rollback。
- [ ] Workspace 与 Session 是两个显式领域概念。
- [ ] Workspace 能管理多个 Session。
- [ ] Session 记录自己的发起目录、命令、args、进程信息、状态、history 和退出记录。
- [ ] 执行命令入口调整为 `termbridge [options] exec -- <command...>`。
- [ ] CLI 为 `recent` / `list` / 后续 workspace/session 命令预留根命令空间。
- [ ] workspace id 与 session id 使用 ULID。
- [ ] 开发模式下 session record 持久化到项目目录 `.termbridge/<workspace>/<session>`。
- [ ] `.termbridge/<workspace>/<session>` 下包含 metadata、process 相关信息、history 和必要状态文件；具体文件名在 Spec 阶段确定。
- [ ] runtime state model 至少覆盖 `starting`、`running`、`stopping`、`stopped`、`failed`。
- [ ] state transition 有明确规则，不能任意字符串散落在代码中。
- [ ] 启动时加载 `.termbridge/<workspace>/<session>`，识别既有状态。
- [ ] 生命周期中持续维护 session 状态，避免 stale running 永久残留。
- [ ] history buffer 先限制行数，保证至少一屏；再限制 bytes，避免过长单行。
- [ ] history 存储到 `.termbridge/<workspace>/<session>`。
- [ ] logs location 继续沿用 effective cwd 下 `logs/termbridge.log`。
- [ ] `termbridge workspace` 能展示目录映射的 Workspace 列表。
- [ ] `termbridge session` 能展示 Session 列表。
- [ ] M3 不把 detach / reattach / daemon / multi-attach 伪装成已完成能力。
- [ ] `go test ./...` 与项目约定检查通过。
- [ ] 不执行 git 写操作。

## Open questions

需要在 Spec / 规格阶段继续细化，但当前已有方向的问题：

1. Workspace 的 human-readable name 如何生成？可能来自目录名、配置文件或后续用户命名。
2. `.termbridge/<workspace>/<session>` 中具体文件布局仍需设计；其中 `<workspace>` 是会话发起目录的稳定映射，`<session>` 暂定为 Session ULID。
3. 进程信息记录到什么粒度？至少应考虑 pid、command、cwd、started_at、exit 信息；是否记录 process tree 信息留到 Spec 阶段。
4. 启动恢复时如何判断历史 running session 是否仍然 alive？当前方向是读取 metadata 和 process 信息，检查进程是否仍存在且匹配；具体匹配字段留到 Spec 阶段确定。
5. history 的默认行数和 bytes 限制采用主流实践并配置化；具体默认值需要在 Spec 阶段确定。
6. `termbridge workspace` / `termbridge session` 的输出格式和是否支持过滤仍需设计。
7. `termbridge [options] exec [options] -- <command...>` 中 options 的作用域需要按主流 CLI 实践审视：根级 `[options]` 作用于 `termbridge`，子命令级 `[options]` 作用于 `exec`。
8. 旧的 `termbridge [options] -- <command...>` 不保留兼容期，M3 直接切换到 `exec` 形式；Spec 阶段需同步测试和文档。

如果用户要求继续推进，这些事项可进入 Spec / 规格阶段，不阻塞需求草稿形成。

## Decisions

当前已明确：

1. 当前流程为 strict / 严格模式。
2. 当前阶段为 Requirement / 需求。
3. M3 是 `Session / Workspace Runtime Model`，不是 GUI、Gateway 或 M5 hardening。
4. Workspace 与 Session 必须显式区分。
5. Workspace 将来会在 Gate 上展示为个人工作区列表。
6. 每个 Workspace 管理多个 Session。
7. Session 有发起目录；发起目录是 Session 的核心 metadata。
8. 命令执行入口改为 `termbridge [options] exec -- <command...>`，为其他命令留下空间。
9. session record 默认持久化到文件系统。
10. 开发模式持久化路径为项目目录 `.termbridge/<workspace>/<session>`。
11. `.termbridge/<workspace>/<session>` 需要包含进程等相关信息。
12. history buffer 先限制行数，确保一屏；再限制 bytes，避免过长单行。
13. history 存储到 `.termbridge/<workspace>/<session>`。
14. logs location 继续沿用当前 effective cwd 下 `logs/termbridge.log`。
15. workspace id 和 session id 使用 ULID。
16. `--session-id` 不是对外用户入口；session id 属于内部持久化和领域建模机制。
17. 未来可能迁移到 SQLite，但 M3 仍要先完成领域建模。
18. 需要支持查询/识别 running session 场景：happy path 中进程内能看到所有会话；进程挂掉后，下一次启动需要加载 `.termbridge/<workspace>/<session>` 并识别状态。
19. GUI 未来必须复用 CLI/runtime model，不能自己拥有 PTY / Process / Workspace lifecycle / History。
20. Web / Gateway 不进入 M3；M3 只为其建立可复用模型边界。
21. Workspace 由 Session 发起目录稳定映射生成；用户进入目录执行命令或使用 `--cwd` 指定目录时，该目录就是 Session 发起目录。
22. Workspace 目录名可用 hash 类似方式稳定生成，但 Workspace id 仍使用 ULID；二者关系需在 Spec 阶段建模。
23. `.termbridge/<workspace>/<session>` 中 `<workspace>` 是目录映射，`<session>` 暂定为 Session ULID。
24. 启动恢复通过读取 metadata 和 process 信息，检查进程是否仍存在且匹配，作为判断 session 是否 alive 的基础。
25. history 默认限制使用主流实践并配置化。
26. M3 先实现 `termbridge workspace` 和 `termbridge session`，展示目录和会话列表。
27. CLI options 作用域遵循主流实践：`termbridge [options] exec [options] -- <command...>` 中根级 options 作用于 `termbridge`，exec 后 options 作用于 `exec`。
28. 旧入口 `termbridge [options] -- <command...>` 直接改掉，不做兼容。
29. 重副作用验证场景，例如真实 Agent TUI、owner-crash、descendant-cleanup、multi-attach、复杂 process tree cleanup，继续归入 M5 / hardening 或后续人工验证，不作为 M3 核心验收。

## Risk

### 风险一：过早引入 daemon 破坏 CLI-first 心智

M3 需要支持启动时加载和识别状态，但这不等于必须实现常驻 daemon。若为了 running 查询过早引入 daemon，会扩大范围并改变本地使用心智。

### 风险二：Workspace / Session 生命周期边界不清

Workspace 是未来个人工作区列表中的稳定对象，Session 是具体运行会话。如果创建/复用规则不清，会导致 `.termbridge` 目录膨胀、recent/list 混乱或 Gate 映射困难。

### 风险三：CLI 迁移破坏既有 M2 使用方式

从 `termbridge [options] -- <command...>` 改为 `termbridge [options] exec [options] -- <command...>` 是有意为根命令扩展留空间，但会影响已有测试、文档和用户习惯。已决策不做兼容，因此 Spec 阶段必须同步更新测试、文档和错误提示。

### 风险四：文件系统持久化演变成隐式数据库

`.termbridge/<workspace>/<session>` 适合开发模式和领域建模，但如果缺乏 schema、原子写、错误处理和迁移策略，后续迁移 SQLite 会困难。

### 风险五：history 过度或不足

完整 terminal history、回放、滚动、断线恢复都容易膨胀；但只保留太少又无法支撑 GUI/Web 预览。M3 需要用行数 + bytes 的边界明确取舍。

### 风险六：stale running 状态误导用户

进程挂掉后，持久化记录可能仍显示 running。M3 必须有启动扫描和状态识别策略，避免永久 stale running。

### 风险七：日志位置与 session record 分离

日志继续在 effective cwd 下 `logs/termbridge.log`，而 session record 在 `.termbridge/<workspace>/<session>`。两者分离时必须在 session metadata 中记录 log path，否则排查体验会变差。

## User review notes

用户已确认上一阶段 PTY 测试收敛交付，并要求开始梳理 M3 阶段任务。随后用户明确要求使用严格模式。

用户进一步确认 M3 关键决策：Workspace 与 Session 需要区分；Workspace 将来会在 Gate 上展示为个人工作区列表；每个 Workspace 管理多个 Session；Session 有发起目录；命令入口改为 `termbridge [options] exec -- <command...>`；session record 持久化到项目目录 `.termbridge/<workspace>/<session>`；history 使用行数与 bytes 双限制并存储到 session 目录；日志继续使用 effective cwd 下 `logs/termbridge.log`；workspace id 和 session id 使用 ULID；session id 是内部持久化机制；未来可能迁移 SQLite 但当前要先领域建模；启动时需要加载 `.termbridge/<workspace>/<session>` 并识别状态，生命周期中持续维护状态。

用户继续确认：Workspace 由用户进入目录执行命令或 `--cwd` 指定的会话发起目录稳定映射生成，可使用 hash 类似方式；`.termbridge/<workspace>/<session>` 中 `<workspace>` 是目录映射，`<session>` 暂定为会话 id；恢复时读取 metadata 和 process 信息，检查进程是否仍存在且匹配；history 默认限制使用主流实践并配置化；先实现 `termbridge workspace` 和 `termbridge session` 展示目录和会话列表；options 作用域遵循 `termbridge [options] exec [options] -- <command...>` 的主流 CLI 实践；旧入口 `termbridge [options] -- <command...>` 直接改掉，不做兼容。
