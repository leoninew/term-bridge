# Terminal resize hardening 需求
最后修改时间: 2026-06-25 20:00:49

Review status: Accepted

Flow mode: light / 轻量模式
Stage: Requirement / 需求

## Background

在审视终端 resize 链路后，当前实现已经覆盖了 Web xterm 初始尺寸测量、WebSocket OPEN 前 resize 缓存补发、后端 runtime 初始尺寸与 PTY 启动尺寸对齐等关键路径。但仍存在若干轻量级 hardening 点：resize 失败后本地状态可能先行推进、启动前 PTY resize 错误被忽略、Gateway/Agent resize 错误静默吞掉、resize 上限过宽，以及 reattach 阶段未使用已有 attach cols/rows 能力。

本任务聚焦修复这些高确定性风险，不重做 terminal 架构。

## Goal

1. 保证 resize 失败时 runtime / runner 的本地尺寸状态不会与真实 PTY 状态永久不一致。
2. 保证 PTY 启动前的初始 resize 失败不会被静默忽略。
3. 提升 Gateway / Agent resize 转发失败时的前后端可观察性，异常路径必须打印日志，避免完全静默失败。
4. 在本轮完成 reattach 初始尺寸处理，利用已有 `TerminalAttachReq.Cols/Rows` 能力或最小兼容扩展，降低 reattach 时旧尺寸先输出的风险。
5. 在本轮收紧 resize 上限，使协议范围更贴近真实 xterm / PTY 能力。
6. 为上述行为补充或调整针对性测试。

## Non-goal

1. 不重写 Web terminal、xterm 封装或 Gateway tunnel 架构。
2. 不引入新的前端状态管理或新的 terminal 协议大版本。
3. 不解决所有历史 replay / 多客户端协同问题。
4. 不修改 git 历史、不自动提交、不推送。
5. 不为了单次验证修改 `justfile` 或项目通用脚本。

## User scenarios

1. 用户在 Web terminal 中运行全屏 TUI，首次尺寸和后续 resize 应尽量与真实浏览器 xterm 保持一致。
2. 用户拖动浏览器/分栏导致 resize，如果某次 PTY resize 失败，系统应能重试或至少不记录虚假的成功尺寸。
3. 用户在 detached 后调整窗口再 reattach，系统必须尽量避免先以旧 PTY 尺寸 attach 并输出旧布局。
4. 维护者排查 resize 问题时，应能从前端浏览器诊断日志、Gateway 日志和 Agent 日志看出 resize 转发或 PTY resize 是否失败。
5. 异常客户端发送过大 resize 时，应被协议校验拒绝，而不是传入 PTY backend。

## Acceptance

1. `SessionRuntime.resize()` 不应在 PTY resize 失败后把 `current` 永久更新为失败尺寸；相同尺寸的后续 resize 应可重试。
2. CLI runner 的 `watchResize()` 不应在 `session.Resize(next)` 失败后推进 `last`，应允许后续 tick 重试同一尺寸。
3. `gopty.Manager.Start()` 对启动前初始 PTY resize 错误不再完全忽略；应返回错误或至少记录明确失败。优先选择不启动错误尺寸进程。
4. Gateway / Agent 中 resize 转发或执行失败不应完全静默；前端浏览器诊断日志、Gateway 日志和 Agent 日志应覆盖异常路径。
5. reattach 初始尺寸必须本轮完成：应使用已有协议字段或最小兼容扩展，把浏览器当前尺寸带到 attach 阶段，并保持旧客户端兼容。
6. resize 上限必须本轮收紧；新的上限应写入协议常量和测试，并与前端 fallback clamp 保持一致。
7. 新增或更新测试覆盖 resize 失败状态一致性、启动初始 resize 失败、reattach 初始尺寸、上限收紧、以及关键转发错误路径；现有 pending resize 和协议边界测试不得回退。
8. 变更范围应集中在 terminal resize 相关文件和对应测试，不引入无关 UI 或配置变更。

## Open questions

无。用户已确认：reattach 初始尺寸必须本轮完成；resize 上限必须本轮收紧；Gateway / Agent resize 失败时前后端都需要打印异常日志。

## Decisions

1. 本任务采用 light / 轻量模式。
2. 先修高确定性后端/runner 状态一致性问题；reattach 初始尺寸必须本轮完成。
3. resize 上限必须本轮收紧，并同步协议常量、前端 clamp 和测试期望。
4. Gateway / Agent resize 失败时前后端都需要打印异常日志。
5. 不使用 compound-engineering agent；按用户要求由当前 agent 直接完成。

## Risk

1. 初始 PTY resize 失败改为返回错误后，某些环境中原本“勉强可启动”的 session 可能变为创建失败；但这比启动错误尺寸的全屏 TUI 更可诊断。
2. reattach 初始尺寸需要调整 browser WS handshake，可能引入 attach 延迟或复杂度；需避免阻塞正常 attach，并保持旧客户端兼容。
3. resize 上限收紧可能拒绝极端大屏或异常布局请求；需选择足够覆盖真实浏览器使用的上限，并在日志/错误中可诊断。
4. 错误日志增加需要避免高频 resize 场景刷屏。
5. 测试 fake PTY / fake tunnel 如果过度耦合内部实现，可能导致测试脆弱；应优先验证行为而非细节。

## User review notes

- 2026-06-25：用户确认 reattach 初始尺寸必须本轮完成。
- 2026-06-25：用户确认 resize 上限必须本轮收紧。
- 2026-06-25：用户确认 Gateway / Agent resize 失败时前后端都要打印异常日志。
- 2026-06-25：用户要求开始实现，Requirement 视为 Accepted。
