# 活跃会话终端 Keep-Alive 验证

最后修改时间: 2026-07-19 17:03:40

Review status: Draft

## Flow mode / Stage

标准模式 / standard；验证 / Verification。

## Requirement alignment

验证对象：

- [docs/requirement/20260719-terminal-tab-keepalive.md](../requirement/20260719-terminal-tab-keepalive.md)（Accepted）
- 关联但不实现：[docs/requirement/20260719-terminal-resource-quota.md](../requirement/20260719-terminal-resource-quota.md)

对照 Goal / Keep-alive policy / Acceptance：

| 需求要点 | 对齐结果 |
| --- | --- |
| 最近 4 个激活 running tab 保活 xterm + WS 并继续收输出 | 对齐：`computeHotSessionIds` + `connectionEnabled=isHot` |
| 非 hot 立即关 WS；xterm 30s 延迟销毁 | 对齐：`connectionEnabled=false` 关 socket；controller `scheduleDispose` |
| 关 tab / 页面立即清理 | 对齐：`remove` / `disposeAll` + `onBeforeUnmount` |
| stopped 走 history，不进 hot set | 对齐：仅 `lifecycle_state===running` 进入 live keep-alive |
| L1 默认 + L2 RuntimeConfig | 对齐：前端 clamp 默认；yaml/env → Go → `BrowserRuntimeConfig.terminal.keepAlive` |
| 不改协议 / 不迁 Pinia / 不做配额 API | 对齐：diff 无 proto/protocol；xterm 仍组件局部；无配额实现 |
| debug 可观测 | 对齐：`keepalive.*` + TerminalView socket debug（`termbridge.terminalDebug`） |

## Spec alignment

标准模式无独立 Spec；按 Requirement + Plan 核对。

## Plan alignment

验证对象：

- [docs/plan/20260719-terminal-tab-keepalive.md](../plan/20260719-terminal-tab-keepalive.md)（Accepted）

| Plan 步骤 | 结果 |
| --- | --- |
| 1. L1 默认 + L2 下发 | 完成 |
| 2. Hot set 调度核心 | 完成（`terminalKeepAlive.ts`，非 `useTerminalKeepAlive.ts` 命名，等价） |
| 3. Workbench 多实例挂载 | 完成；绕开 TabsContent |
| 4. 关闭竞态与清理 | 完成：disconnect 保留 xterm；unmount disposeAll |
| 5. 自动化测试 | 完成 focused 单测 |
| 5. 浏览器手测 | **未完成**（见 Incomplete） |
| 6. 文档 / Verification | 本文件 |

## Actual diff summary

### 本 feature 代码与配置（相对 HEAD，忽略无关暂存）

| 文件 | 变更 |
| --- | --- |
| `web/src/features/sessions/terminalKeepAliveConfig.ts` | **新增** L1 默认与 clamp |
| `web/src/features/sessions/terminalKeepAlive.ts` | **新增** hot set / dispose 调度 |
| `web/src/features/sessions/terminalKeepAlive.test.ts` | **新增** 单元测试 |
| `web/src/components/session/SessionWorkbench.vue` | 多 live pane + keep-alive reconcile |
| `web/src/components/session/TerminalPane.vue` | 透传 `connectionEnabled` / `active`；去掉 TabsContent |
| `web/src/components/terminal/TerminalView.vue` | 冻结/重连、overlay、fit/focus |
| `web/src/components/session/SessionsPageShell.vue` | `resolveSession` / `resolveWsUrl` |
| `web/src/config.ts` / `web/src/store/runtimeConfig.ts` | 解析 `terminal.keepAlive` |
| `web/src/store/runtimeConfig.test.ts` | 默认 keepAlive 断言 |
| `configs/config.yaml` / `.env.example` | keepalive 默认/注释 |
| `internal/shared/dto/browser/runtime_config.go` | optional `terminal.keepAlive` |
| `internal/shared/infrastructure/config/*` | 加载、normalize、validate、投影与测试 |
| `docs/requirement|plan/20260719-terminal-tab-keepalive.md` | 过程文档 |
| `docs/requirement/20260719-terminal-resource-quota.md` | **仅需求草稿**（本 feature 不实现） |

统计（feature 相关 tracked 改动约）：14 files changed，+383 / -78（另含 untracked 前端模块 + 过程文档）。

### 工作区无关改动（勿并入本提交）

已暂存但与 keep-alive **无关**：

- `SessionStatusBar.vue`
- `ShortcutCard.vue` / `ShortcutEditorDialog.vue` / `ShortcutsPageShell.vue`
- `styles.css`

## Expected vs actual changed files

| 预期（Plan） | 实际 |
| --- | --- |
| 配置 L2：yaml/env/Go DTO/loader/runtime 投影 | 已改 |
| 前端 config + runtimeConfig parse | 已改 |
| 调度模块（useTerminalKeepAlive 或等价） | 已新增 `terminalKeepAlive*.ts` |
| SessionWorkbench / TerminalPane / TerminalView | 已改 |
| 对应测试 | 已新增/更新 |
| 终端协议 / Pinia xterm / 配额 API | **未改**（符合） |
| agent/cloud 专用 loader 文件 | 复用 shared config 加载链，无独立重复实现（可接受） |

## Acceptance criteria checklist

### 1. 策略落地

- [x] 最近 N=4（可配）running opened tab：mount + WS 收输出
- [x] 非 hot：关 WS；xterm 按 disposeDelayMs 延迟销毁
- [x] 关 tab / 页面：立即清理

### 2. 切换体验

- [x] 代码路径：hot 内 `connectionEnabled` 保持 true，`showBootOverlay` 仅 active+enabled 且未 started
- [ ] **浏览器手测**：2 tab 秒开 / 无长 connecting（未跑）
- [x] 非 hot 再进：`connectionEnabled` 变 true → reconnect + reset sessionStarted（允许 replay）

### 3. 正确性

- [x] 切 tab 不 close remote session（仅浏览器 WS）
- [x] 同 session 单挂载实例（mounted set 去重）；不长期双 writer
- [x] hot 隐藏仍写 xterm（WS 仍连）
- [x] 非 hot 漏输出依赖既有 attach replay 上限（未改协议）

### 4. 尺寸与焦点

- [x] activate / connection-enabled 时 `fit` + `scheduleTerminalFocus`
- [ ] **浏览器手测**：resize 后切回行列（未跑）

### 5. 资源边界

- [x] hot ≤ maxHotTerminals（单测覆盖上限与 eviction）
- [x] dispose 定时器可取消；`disposeAll` 清理

### 6. 配置

- [x] L1 默认 4 / 30000
- [x] L2 yaml/env + BrowserRuntimeConfig 投影
- [x] clamp 1–16 / 0–600000；非法回退
- [x] 无 L3 Policy API

### 7. 可观测性

- [x] `keepalive.reconcile|evict|schedule-dispose|unmount|dispose-all`
- [x] TerminalView socket/size/disconnect debug

### 8. 回归（静态/自动化）

- [x] typecheck 全量通过
- [x] focused lint 通过
- [x] focused unit 通过
- [ ] 主题切换多实例浏览器回归（未跑）
- [ ] local/cloud 手测（未跑）

## Test results

| 检查 | 结果 |
| --- | --- |
| `cd web && npm run test -- src/features/sessions/terminalKeepAlive.test.ts src/store/runtimeConfig.test.ts` | 通过：2 files / **14 tests** |
| `cd web && npm run typecheck` | 通过 |
| focused `npm run lint`（keep-alive 相关源文件） | 通过 |
| `go test ./internal/shared/infrastructure/config/` | 通过 |
| 浏览器 E2E / 人工 smoke | **未执行** |
| 全量 vitest / 生产 build | 未作为本验证必跑项 |

### 单测覆盖要点

- clamp 默认 / 越界 / 非有限值回退
- hot set：active 优先 + MRU 上限
- eviction 后 delay dispose
- re-enter hot 取消 dispose
- runtimeConfig 缺省 terminal 段时 keepAlive 默认

## Missed or expanded scope

| 项 | 说明 |
| --- | --- |
| 资源配额需求文档 | 已新建 requirement 草稿，**无实现代码**（符合 non-goal） |
| 调度文件命名 | Plan 写 `useTerminalKeepAlive.ts`，实现为 `terminalKeepAlive.ts`（等价，无范围扩张） |
| 无关暂存 UI/styles | 工作区存在，**不属于本 feature** |
| 浏览器手测 | Plan 清单未完成 → Incomplete |

## Risks

1. 隐藏 pane fit 仍依赖切回时 `fit('activated')`；极端布局下需手测确认。
2. cold 30s 内额外 xterm 内存；有上界但高输出仍可能触达前后端队列。
3. 快速切 tab 的 write-after-close 噪音可能仍存在（应收敛，非完全消除）。
4. `__CONFIG__` 客户端可改；服务端硬顶属配额需求，本交付仅前端 clamp + 部署配置。
5. 提交时混入无关 staged 文件会导致 diff 污染。

## Incomplete items

1. 浏览器手测清单（Plan）：
   - 2 running tab 来回秒开
   - 5 tab 驱逐 + 30s 内重连 / 超时冷启动
   - 关 tab 立即释放
   - stop → history 无残留 live WS
   - resize / 主题
   - local/cloud 抽测
2. 可选：全量 `npm test` / `vite build`（本轮未跑）

## Conclusion

**代码与自动化验证通过**，实现与 Accepted Requirement/Plan 的策略、配置分层与 non-goal 对齐。

**交付判定**：

- 可按 feature 范围单独提交（排除无关 staged 文件与配额实现）。
- 浏览器 smoke **尚未完成**；建议合并前由人工完成 Incomplete 手测清单，或接受“先合代码、后补手测”风险。

Verification 状态：人工 smoke 完成前保持 `Draft`；用户确认手测通过后可将 `Review status` 改为 `Accepted`。
