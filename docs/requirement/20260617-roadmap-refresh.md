# Roadmap 刷新需求
最后修改时间: 2026-06-17 15:05:21

Review status: Accepted

## Background

TermBridge-go 已完成 Windows PTY 技术可行性验证，阶段性结论是：

```text
Go + github.com/aymanbagabas/go-pty
```

可以作为 Windows Agent Runtime 的 PTY 技术方向。

README 已明确新项目不延续原 TermBridge 的 `ttyd + tmux` 架构，而是由 TermBridge 自主管理运行时：

```text
Workspace
  ↓
PTY
  ↓
Process
```

后续讨论进一步收敛了本地交付形态：本地阶段不应把 Web 或常驻 daemon 暴露为一等使用方式，而应把 `termbridge` CLI 作为唯一一等入口。用户进入目录后直接运行 `termbridge <命令>`，或通过 `termbridge [options] -- <command...>` 指定目录、环境和命令。

GUI 可以作为可选产品壳，但必须依赖 CLI / CLI runtime，不应自己实现 PTY、Process、Workspace lifecycle 或 History，否则就等于另做一套 Agent。

Web terminal 技术风险不能推迟到 Gateway 阶段才第一次验证；但 Web 只作为提前 technical spike / dev harness，不作为本地正式交付形态。Gateway 阶段再把 Web 作为正式跨设备产品入口。

## Goal

刷新项目路线文档，使后续开发围绕以下方向推进：

1. 当前 PTY 技术方向采用 Go + `github.com/aymanbagabas/go-pty`。
2. 本地阶段只有一种一等使用方式：`termbridge` CLI。
3. 标准本地命令形态为：

   ```text
   termbridge [options] -- <command...>
   ```

   例如：

   ```text
   termbridge -- claude
   termbridge --cwd D:\project -- codex
   termbridge --cwd D:\project -- pwsh
   termbridge --cwd D:\project -- npm run dev
   ```

4. 用户愿意运行什么命令，TermBridge 就在 PTY runtime 中直接运行该命令。
5. `Ctrl+C` 在 CLI 使用中首先表示用户要停止当前命令：先 soft interrupt，必要时再升级 close / kill process tree。
6. GUI 是 CLI 的可选容器，不拥有 runtime。
7. Web terminal 在 Gateway 前提前验证，但不作为本地正式产品交付。
8. 里程碑计划使用 `docs/plan/20260617-roadmap-refresh.md` 承接，不再恢复独立 `docs/roadmap/project-milestones.md`。

## Non-goal

本次不包括：

1. 不实现 Agent Runtime、CLI runner、Workspace Manager、GUI 或 Gateway。
2. 不修改 `cmd/pty-spike` / `cmd/pty-testprogram` 代码。
3. 不运行新的 PTY spike 或 Web terminal spike。
4. 不创建 ADR，除非后续单独要求。
5. 不恢复 `docs/roadmap/project-milestones.md`。
6. 不删除 `docs/spec/20260617-go-pty-feasibility.md`。
7. 不创建 verification 文档，除非后续明确进入 Verification / 验证阶段。
8. 不执行 `git add`、`git commit`、`git push` 或其他 git 写操作。

## User scenarios

### 场景一：用户在当前目录运行命令

作为本地用户，我希望进入项目目录后直接运行：

```text
termbridge -- claude
```

TermBridge 在当前目录启动 PTY runtime，并运行我指定的命令。

### 场景二：用户指定 cwd 运行命令

作为本地用户，我希望不切换 shell 当前目录，也能运行：

```text
termbridge --cwd D:\SourceCodes\mywork\TermBridge-go -- codex
```

TermBridge 在指定目录启动命令。

### 场景三：用户用 Ctrl+C 停止命令

作为 CLI 用户，我希望 `Ctrl+C` 表示停止当前运行的命令，而不是 detach 到后台。TermBridge 应先尝试 interrupt foreground process；如果命令没有退出，应提示或升级到 close / kill process tree。

### 场景四：GUI 作为容器

作为未来 GUI 用户，我希望 GUI 只是更友好的启动器和 terminal 容器，底层仍然调用或复用 `termbridge` CLI / runtime 能力。GUI 不应另做一套 PTY 和 Workspace lifecycle。

### 场景五：Web 风险提前验证

作为维护者，我希望在 Gateway 前通过 dev-only Web terminal spike 验证 xterm.js、browser input、resize、backpressure 和 WebSocket protocol，避免 Gateway 阶段才首次暴露 Web terminal 风险。

## Acceptance

完成后应满足：

- [ ] `docs/requirement/20260617-roadmap-refresh.md` 记录 CLI-first、GUI container、Web spike before Gateway 的路线决策。
- [ ] `docs/spec/20260617-roadmap-refresh.md` 说明这些决策如何影响里程碑和架构边界。
- [ ] `docs/plan/20260617-roadmap-refresh.md` 将里程碑调整为 CLI-first 路线，至少包含 M1 Go CLI Skeleton、M2 PTY Command Runner MVP、M2.5 Web Terminal Technical Spike、M3 Session / Workspace Runtime Model、M4 Local Product Surface、M6 Gateway Web Terminal MVP。
- [ ] `docs/spec/20260617-go-pty-feasibility.md` 的后续路线描述与 CLI-first 决策一致。
- [ ] 文档明确：本地正式交付不以 local Web 为主，Gateway 阶段才把 Web 作为正式入口。
- [ ] 文档明确：GUI 是 CLI 的容器，不拥有 PTY / Process / Workspace lifecycle / History。
- [ ] 文档明确：`Ctrl+C` 在本地 CLI 中是停止当前命令的用户意图，不能等同于 detach。
- [ ] 保留 Go PTY feasibility 文档。
- [ ] 不恢复 `docs/roadmap/project-milestones.md`。
- [ ] 不执行 git 写操作。

## Open questions

当前无阻塞文档更新的问题。

后续仍需单独决策的问题：

1. `termbridge <命令>` 是否作为 `termbridge -- <命令>` 的语法糖支持，还是只支持 `--` 分隔形式。
2. `Ctrl+C` 升级策略的具体交互：是否第一次 soft interrupt，第二次 hard stop，或使用超时自动升级。
3. GUI 容器调用 CLI 的具体方式：spawn CLI、local IPC，还是复用同一 Go runtime package。
4. Web terminal spike 放在 M2.5 独立阶段，还是并入 M3/M5 gate。

## Decisions

已确认：

1. 本地阶段 `termbridge` CLI 是唯一一等使用入口。
2. 标准运行模型是 `termbridge [options] -- <command...>`。
3. 默认 cwd 是当前目录；可以通过 `--cwd` 指定目录。
4. 用户指定什么命令，TermBridge 就直接在 PTY runtime 中运行该命令。
5. `Ctrl+C` 表示停止当前命令，不能当作 detach。
6. GUI 是 CLI 的容器，不拥有 runtime。
7. Web terminal 风险需要在 Gateway 前做技术验证。
8. Gateway 阶段再把 Web 作为正式产品入口。

## Risk

### 风险一：CLI 语法过度设计

本地阶段应优先保证 `termbridge [options] -- <command...>` 稳定，不要过早设计复杂 workspace 子命令体系。

### 风险二：GUI 变成另一套 Agent

如果 GUI 直接管理 PTY、Process、History 或 Workspace lifecycle，就会和 CLI runtime 分裂。GUI 必须作为容器或薄壳。

### 风险三：Web 验证过晚

如果等到 Gateway 才首次验证 xterm.js、browser input 和 WebSocket backpressure，会与 auth、routing、tunnel reconnect 等问题叠加，定位困难。因此需要提前 Web technical spike。

### 风险四：Ctrl+C 语义不严谨

Windows 下不同 shell / Agent CLI 的 Ctrl+C 行为不同。正式实现必须有 soft interrupt、close、kill process tree 的分层策略，不要无限容忍错误。

## User review notes

用户明确确认：CLI 应该是本地阶段的一等公民；本地只应有一种使用方式，即进入目录运行 `termbridge <命令>`，或 `termbridge [options] -- <command...>`；GUI 也应该是 CLI 的容器，否则等于另做 Agent。