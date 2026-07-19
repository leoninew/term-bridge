# Plan / 计划：活跃会话终端 Keep-Alive

最后修改时间: 2026-07-19 18:20:00

Review status: Accepted

## Flow mode / Stage

标准模式 / standard；计划 / Plan（Accepted）；实现 / Implementation 已完成；验证 / Verification 进行中（自动化通过，手测 Draft）。Requirement 已 Accepted。

## Requirement basis

- Requirement（Accepted）：[docs/requirement/20260719-terminal-tab-keepalive.md](../requirement/20260719-terminal-tab-keepalive.md)
- 关联（不做）：[docs/requirement/20260719-terminal-resource-quota.md](../requirement/20260719-terminal-resource-quota.md)

### 已锁定策略

| 项 | 值 |
| --- | --- |
| Hot set | 最近激活的 **4** 个 running opened tab |
| Hot 内 | 保活 xterm + WS，**继续收输出**；切回无冷启动、无长 boot overlay |
| 离开 hot set | **立即关 WS**；保留冻结 xterm **30s** 后销毁 |
| 30s 内回到 hot | 可复用 xterm，**重新 connect/attach/replay** |
| 关闭 tab/页面 | 立即清理，不等待 30s |
| stopped | history 只读，不进 hot set |
| 配置 | L1 默认 + L2 RuntimeConfig 下发 |
| 配额硬顶 | **不在本计划** |

## Implementation approach

```text
SessionWorkbench
  现在: 只挂 active TerminalPane → 切 tab 全量 unmount/dispose
  目标: 多实例缓存 + hot set 调度

  openedTabs (running)
       |
       v
  TerminalKeepAliveController
       |- 维护 lastActivatedAt / hot set (N=maxHotTerminals)
       |- hot:  mount/show + WS 保持
       |- cold: close WS, schedule dispose in disposeDelayMs
       |- closed tab / unmount page: force dispose

  SessionWorkbench 渲染:
       对 cache 内每个 live session 保留 TerminalPane/TerminalView
       用隐藏容器切换可见性（避免 v-if 销毁）
       active: visible + focus + fit
       inactive hot: hidden + 仍收输出
       inactive cold: 无 WS；xterm 冻结至 timeout
```

原则：

1. **前端为主**：调度、缓存、延迟销毁全在 web。
2. **xterm/WS 不进 Pinia 权威状态**；可用 composable/模块挂在 workbench 生命周期。
3. **不改终端协议**；attach/replay/resize 语义保持。
4. **尺寸**：隐藏后切回必须 fit + 必要时 resize control。
5. **配置**：L1 常量兜底；L2 从 RuntimeConfig 读取并 clamp。

## Implementation steps

### 1. 配置：L1 默认 + L2 下发

1. 前端新增 keep-alive 配置模块（如 web/src/features/sessions/terminalKeepAliveConfig.ts）：
   - 默认 maxHotTerminals=4，disposeDelayMs=30000
   - clamp：maxHotTerminals in [1,16]，disposeDelayMs in [0,600000]
   - 从 runtime config 可选字段读取，非法回退默认
2. Go：
   - configs/config.yaml / .env.example 增加 	erminal.keepalive.max_hot_terminals / dispose_delay_ms
   - TerminalConfig 与 loader 绑定
   - rowserdto.RuntimeConfig 增加可选 	erminal.keepAlive
   - BuildBrowserRuntimeConfig 投影字段
3. 前端 config.ts + 
untimeConfig parse：可选 terminal 段，缺省不报错
4. 单测：clamp、缺省、非法值回退；Go runtime config 投影测试（跟随现有模式）

### 2. Hot set 调度核心

1. 新增 composable/controller（如 web/src/features/sessions/useTerminalKeepAlive.ts）：
   - 	ouch(sessionId)：刷新激活时间
   - 
econcile(openedRunningIds, activeId, limits)：计算 hot set
   - 离开 hot：closeSocket + scheduleDispose(delay)
   - 进入 hot：取消 dispose；无实例则创建；有冻结 xterm 无 WS 则重连
   - dispose(sessionId) / disposeAll()
2. 与 openedTabs / ctiveSessionId / lifecycle 同步：
   - activate → touch + reconcile
   - close tab → 立即 dispose
   - running → stopped：退出 live keep-alive（转 history）
3. debug 日志：hot hit / evict / close-ws / schedule-dispose / dispose / reconnect / effective config（	erminalDebug）

### 3. Workbench 多实例挂载

1. 改造 SessionWorkbench.vue / TerminalPane：
   - 不再只挂 active 一个 live pane
   - 对 cache 中 running session 渲染多个实例
   - 可见性用 CSS 隐藏，避免会卸载的 -if
2. TerminalView 小改：
   - props 如 connectionEnabled / ctive
   - connectionEnabled=false：关 WS，保留 xterm（冻结）
   - connectionEnabled=true：保持或 connect
   - hidden→visible：fit + focus；尺寸变化发 resize
   - 已连接复用：showBootOverlay=false
3. 缓存 key = session.id；禁止跨 session 复用
4. stopped 保持 HistoryTerminalView；从 live 缓存移除

### 4. 关闭竞态与资源清理

1. 快速切 tab：同 session close→open 串行，避免 409
2. workbench unmount / 路由离开：disposeAll，清定时器
3. dispose 顺序：disposed 标记 → close socket → dispose xterm
4. 确认单 session 仍单 browser writer

### 5. 测试与手测

**自动化**

1. hot set：上限 4、active 必在、LRU 挤出
2. 延迟销毁：evict 后 delay dispose；re-touch 取消
3. 配置 clamp / 默认
4. 可测时：connectionEnabled 切换不销毁实例（mock）

**手测**

1. 2 running tab 来回切：秒开、无长 connecting、输出连续
2. 5 running tab：挤出最旧；立即无 live 输出；30s 内点回重连 replay；超时冷启动
3. 关 tab：立即释放
4. session 停止 → history，无残留 live WS
5. resize 后切回行列正确
6. 主题切换多实例正确
7. local/cloud 各抽测（环境允许时）

**命令**

- web：
pm test / 	ypecheck / lint（以 package scripts 为准）
- Go：runtime config / config load 相关 go test

### 6. 文档与收尾

1. .env.example / config.yaml 注释补 keepalive
2. 不自动 git commit
3. 实现完停在 Implementation，等用户要求再 Verification

## Files to change（预期）

| 区域 | 路径（代表性） |
| --- | --- |
| 配置 L2 | configs/config.yaml、.env.example、internal/shared/dto/browser/runtime_config.go、internal/shared/infrastructure/config/browser_runtime_config.go、agent/cloud config load |
| 前端配置 | web/src/config.ts、web/src/store/runtimeConfig.ts、新建 	erminalKeepAliveConfig.ts |
| 调度 | 新建 web/src/features/sessions/useTerminalKeepAlive.ts（或等价） |
| UI | SessionWorkbench.vue、TerminalPane.vue、TerminalView.vue |
| 测试 | 对应 *.test.ts；Go browser runtime config tests |

**预期不改**

- 终端协议 / subprotocol
- Pinia 存 xterm
- 资源配额 API
- history 存储格式

## Verification plan

1. Requirement acceptance 逐条
2. diff 范围检查（不越界配额/协议）
3. 自动化 + 手测记录
4. 风险：隐藏 fit、close 竞态、配置回退

## Blockers

无。配额需求不阻塞。

## Assumptions

1. 若 reka-ui TabsContent 卸载非 active 内容，则绕开它自管面板容器，tab strip 仍用现有 trigger。
2. 多 TerminalView 各有独立 socket，最多约 4 条 live WS。
3. L2 字段可选，旧注入无 terminal 段仍用 L1。
4. 同 session 不会双实例双连。

## Risks

1. 隐藏零尺寸 fit 错误 → 可见时强制 fit/resize
2. 冻结待销毁实例在 30s 内额外占内存；reconcile 避免无界堆积
3. TabsContent 卸载行为与缓存冲突
4. 快速切换 write-after-close 噪音
5. L2 跨 Go/web 的向后兼容

## Rollback

1. 可选：maxHotTerminals=1 且 disposeDelayMs=0 接近旧行为
2. 或 revert 本 feature 提交；不影响 PTY 数据

## User review notes

- 2026-07-19：Requirement 按推荐 Accepted，进入 Plan。
- 2026-07-19：用户确认「开始实现」→ Plan Accepted，进入 Implementation。

## 实现状态

- 配置 L1/L2（Go config + BrowserRuntimeConfig + 前端 parse/clamp）已落地
- Keep-alive 调度核心与单测已落地
- SessionWorkbench 多实例挂载 + TerminalView connectionEnabled 已落地
- 前端 typecheck / vitest 全绿；Go config 测试通过
- 待用户确认后进入 Verification
