# M5 Runtime Hardening 需求
最后修改时间: 2026-06-20 17:35:59

Review status: Accepted

## Background

M4 closeout 已完成文档和可自动化 product-surface 测试收口。项目当前进入 M5 Runtime Hardening 阶段，目标是把 CLI command runner、Session / Workspace runtime 和 local Web/workbench 从“功能可用”推进到“可靠可验证”。

M5 需要承接 M2、M2.5、M3、M4 遗留的可靠性风险：

- Claude Code / Codex 真实 TUI 交互尚未充分验证。
- Ctrl+C / interrupt / close / kill process tree 在真实命令中的行为需要明确。
- 多 session、长时间运行、大量输出、高频 resize 和 slow client backpressure 需要验证和加固。
- Web terminal 已有待处理风险：client queue 满 detach、history 写盘性能、readLoop 无背压、高频 copyBytes 分配。
- Session / Workspace metadata 与真实进程状态的一致性需要检查。

## Goal

完成 M5 Runtime Hardening：

1. 加固 process cleanup 和 process tree cleanup，并提供可重复验证方式。
2. 明确 Ctrl+C / interrupt / close / kill 的实际行为和升级策略。
3. 验证 Claude Code / Codex 真实 TUI 交互，而不只验证 `--version`。
4. 处理或明确 Web terminal 大输出、slow client backpressure、history flush、copyBytes 高频分配等风险。
5. 验证 session/workspace metadata、state、process、exit、history 与真实 runtime 状态一致。
6. 建立 M5 可重复运行的自动化测试、检查命令和人工验证清单。
7. 保持 CLI-first 和 runtime ownership：Web/workbench 只作为辅助入口，不拥有 PTY / Process / Workspace runtime。

## Non-goal

本阶段不做以下事项：

1. 不进入 Gateway / M6 正式远程 Web 产品实现。
2. 不实现多用户 auth、device registry、agent tunnel 或 remote pairing。
3. 不新增 GUI runtime，也不让 GUI/Web 独立拥有 PTY 或 Process lifecycle。
4. 不改变正式 CLI 入口 `termbridge exec -- <command...>`。
5. 不实现显式 `recent` 命令或新 recent UX。
6. 不做 release packaging / installer / upgrade strategy。

## User scenarios

1. 作为本地 CLI 用户，我可以通过 `termbridge exec -- <command...>` 长时间运行 shell、Claude Code 或 Codex，并能可靠退出。
2. 作为终端用户，我按 Ctrl+C 时能中断前台命令；如果命令不退出，TermBridge 有明确升级策略并能报告结果。
3. 作为 Web/workbench 用户，我可以 attach 会话、查看输出、resize、detach/close，并且大量输出或慢客户端不会拖垮 runtime。
4. 作为维护者，我可以通过测试和人工验证清单判断 M5 是否满足进入 Gateway/M6 的条件。
5. 作为调试者，我可以通过 session/workspace metadata、process record、exit record、history 和 logs 判断真实状态。

## Acceptance

M5 完成需要满足：

1. CLI command runner 长时间运行可用，并有自动化或人工验证记录。
2. Ctrl+C soft interrupt、close PTY、kill process tree 的边界和升级行为明确。
3. Claude Code / Codex 真实交互有验证记录，包括启动、输入、输出、退出、中断。
4. 20+ command/session 或同等压力场景有验证记录。
5. 大量 output 不导致 runtime 内存无限增长或无说明的 client 丢失。
6. 高频 resize 不导致协议错误、panic 或 runtime 卡死。
7. process cleanup / process tree cleanup 有测试或人工验证记录；失败时有可观测报告。
8. Web terminal backpressure 策略明确，slow client 不拖死全局 PTY reader。
9. history 有明确上限和写盘策略，大输出下 I/O 行为可接受。
10. session/workspace metadata 与真实进程状态一致，异常退出能正确反映 state、process、exit 和 history。
11. 可自动化项有测试覆盖；不可稳定自动化的交互项有 manual verification checklist。
12. M5 通过后，可以进入 M6 Gateway Web Terminal MVP。

## Open questions

1. Windows process tree cleanup 的实现方式是否使用系统 API、job object、进程枚举，还是先增强现有 go-pty adapter 的 fallback？
2. Web terminal backpressure 策略采用丢弃、限速、断开慢客户端、分层 ring buffer，还是其他方案？
3. history 写盘策略采用 debounce/batch、append-only、bounded segment，还是维持当前 writer 并加限制？
4. 20+ session 压测的最小可接受命令组合是什么：shell、短命令、长输出命令、Claude/Codex 混合？
5. Claude Code / Codex 人工验证是否依赖本机已安装环境；如果缺失，是否允许记录为环境限制并使用 shell/TUI 替代验证？

## Decisions

1. M5 聚焦 runtime hardening，不进入 Gateway/M6。
2. CLI 入口保持 `termbridge exec -- <command...>`。
3. 可自动化检查优先写测试；真实 TUI、Ctrl+C、长时间运行和多终端交互保留人工验证记录。
4. Web/workbench hardening 只处理本地辅助入口的可靠性，不改变 runtime ownership。
5. M4 中不实现的显式 `recent` UX 不进入 M5 范围。

## Risk

1. Claude Code / Codex 的真实交互可能依赖本机环境和账号状态，自动化程度有限。
2. Windows Ctrl+C / process tree cleanup 行为可能因 shell、ConPTY、子进程模型不同而不一致。
3. backpressure 策略若过度复杂，可能影响现有 Web/workbench 简洁性。
4. 大输出和长时间运行测试可能暴露较多 runtime 设计问题，需要拆分子任务推进。
5. 如果 M5 未完成就进入 M6，Gateway 会放大 runtime、WebSocket 和 backpressure 问题。

## User review notes

用户已确认进入 M5，并补充采用严格模式 / strict。后续流程为 Requirement → Spec → Plan → Implementation → Verification。

仍需在 Spec / Plan 阶段收敛：

1. Windows process tree cleanup 是否作为 M5 必做，还是先作为验证项确认现状。
2. Web terminal backpressure 是优先修复项还是先做压测和风险记录。
3. Claude Code / Codex 真实验证是否可依赖当前开发机环境。
