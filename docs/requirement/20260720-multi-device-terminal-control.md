# 多设备终端会话观察与控制权转移 / Multi-Device Terminal Observation and Control Transfer
最后修改时间: 2026-07-20 13:45:24

Review status: Accepted

## Related

- `docs/requirement/20260719-terminal-resource-quota.md`（Accepted）：每用户 concurrent attach 配额；**本需求修订其「每 session 最多一个 live browser attach / 409」表述**，改为「每 session 最多一个 controller，允许多 attach 观察」。
- `docs/requirement/20260719-terminal-tab-keepalive.md`：前端 hot terminal 保活；多端同 session 时每个 live attach 仍占配额。
- `docs/requirement/20260719-cloud-session-mcp-boundary.md`：Browser / MCP 共用单写入口径；MCP attach 若落地，与本需求共用同一 controller/observer 规则，不另开双写例外。
- `docs/requirement/20260716-cloud-terminal-control-relay.md`：Cloud terminal control relay 与 tunnel 帧路径。
- 既有 Agent runtime：`SessionRuntime` 已支持多 client 输出 fan-out；缺口是 Cloud 层单 writer 拦截与输入/PTY resize 的 controller 门禁。

## Background

当前 Cloud 与 Agent 的 terminal attach 路径均以 `workspaceId/sessionId` 为键维护单一 browser writer。第一个浏览器 attach 成功后，同一 session 的第二个浏览器连接会被拒绝为 HTTP `409 conflict`。这使用户无法在手机和平板等多个浏览器上同时打开同一设备、同一 workspace、同一 session。

现有 `workspaceId` 与 `sessionId` 使用全局唯一 ID 生成机制；本需求不改变它们的唯一性假设，也不需要将 `deviceId` 加入 session key。Cloud 的 attach quota 仍按 Cloud 用户聚合计算，默认上限为 8 个存活的 browser terminal attach。

实现现状补充：

- Cloud / Agent local API 在 accept 前用 in-process `writers` 拒绝第二 attach（409）。
- Agent `SessionRuntime` 可维护多个 client 并 fan-out 输出，但任意 client 当前均可 `WriteInput` / `Resize`，尚无 controller 角色门禁。
- Agent 在 `TerminalAttach` 携带 cols/rows 时会在 attach 路径上立即 resize PTY；本需求必须把该路径纳入「仅 controller 可改 PTY」约束。

## Goal

1. 支持多个浏览器同时 attach 同一设备上的同一 workspace/session，并实时接收该 session 的终端输出。
2. 同一时刻只允许一个已连接客户端拥有终端输入和 PTY resize 控制权，避免输入、粘贴和窗口尺寸相互竞争。
3. 后续连接默认作为只读观察者；仅其明确用户手势输入、粘贴或点击显式「接管」入口时，前端显示确认模态窗，由用户决定是否接管会话。
4. observer 的本地 xterm 可适应其容器尺寸，但不得触发接管模态；前端应尽量不向服务端发送 PTY resize，**服务端对非 controller 的 resize 必须静默忽略且不改 PTY**。
5. 用户确认接管后，服务端原子地将控制权转移给请求方；原控制者保留连接和终端输出，但失去输入与 PTY resize 权限。
6. 服务端必须成为控制权校验边界，不能仅依赖前端模态窗阻止未授权输入；**Agent 为最终 PTY 写门禁**，Cloud 路径应 early reject 且不转发非 controller 写操作。

## Non-goal

1. 不允许多个客户端同时向同一 PTY 写输入。
2. 不因另一个客户端打开或观察 session 而自动断开当前控制者。
3. 不修改 `workspaceId`、`sessionId` 的生成机制或为现有 session key 追加 `deviceId`。
4. 不改变 Cloud device identity、用户设备 binding 或 tunnel 身份认证模型。
5. 不在本需求中实现跨 Cloud 实例的分布式控制权协调；当前 in-process terminal route 模型的部署边界保持不变。
6. 不在本需求中改变现有 attach quota 的主体、默认值或管理员配额管理接口。
7. 不在本需求中新增 controller/observer 之外的 ACL、owner 或共享权限模型。
8. 不设计 Browser 与 MCP（或其他集成）对同一 session 的双写协作。

## User scenarios

1. 用户在手机上控制一个正在运行的终端后，在平板打开同一 session；平板 attach **成功**（不再因已有 writer 而 409），立即以只读方式显示已有输出与后续实时输出，手机不受影响；平板在首包中获知自己为 observer。
2. 用户在平板明确键入字符或粘贴内容；界面提示“此会话正在另一设备上控制”，提供取消和接管会话选项。
3. 平板因旋转、分屏、keep-alive 或容器尺寸变化而触发本地 xterm layout resize；它只适应本地显示，不弹出接管模态，也不改变 PTY 尺寸（包括不通过 attach 初始 size 改变 PTY）。
4. 用户取消接管；平板继续只读观察，手机仍可输入和 PTY resize；取消所对应的本次输入序列被静默丢弃，不重复弹窗，且服务端不缓冲这些输入。
5. 用户下一次新的明确输入或粘贴手势时，平板再次显示接管确认；状态栏应常驻显示只读状态，并提供显式“接管”入口。
6. 用户确认接管；平板经服务端 takeover 成为唯一 controller，获得输入和 PTY resize 权限；手机收到 control lost（或等效）并继续只读查看输出。
7. 旧前端、恶意客户端或并发请求直接发送 input/resize/attach size；服务端拒绝非控制者写入，**不得到达 PTY**。
8. 当前 controller 断开后，若仍有其它 live attach，则 **自动**将控制权交给按确定性规则选出的剩余客户端（无需再确认）；仅当无剩余 attach 时 session 无 controller。
9. 两个浏览器几乎同时 attach 同一 session：同一锁边界内仅一人成为 controller，其余为 observer。
10. 两个 observer 几乎同时 takeover：至多一个成功，失败方保持 observer 并可观测失败原因。
11. 多个 observer 同时连接时，每个 live browser attach 仍计入所属 Cloud 用户的 attach quota；配额超限保持明确的 HTTP 429 与稳定错误码语义。同 session 多端观察会占用其它 session 的 live 名额，不豁免。

## Acceptance

- [ ] 同一 `workspaceId/sessionId` 支持一个 controller 和多个 observer 同时在线；所有已 attach 客户端收到同一 session 的实时终端输出。
- [ ] 第二及以后的 attach **成功**，不再因「已有 active writer」返回 HTTP 409；首个在同一原子边界内成功进入 live 的 attach 成为 controller，其余为 observer；attach 本身不使既有 controller 断开或降级。
- [ ] attach 成功后，客户端可从服务端获知自身角色（`controller` | `observer`，或等效 `can_control`）；协议帧名由 Spec 定义。
- [ ] 非 controller 的 binary input、PTY resize 控制帧、**以及 attach 初始 size（query 或 attach payload）** 均不得改变 PTY。
- [ ] 写路径门禁：非 controller 操作**不得到达 PTY**。Agent 为最终门禁（local 与 tunnel 共用）；Cloud 路径应对非 controller 写操作 early reject 且不转发 tunnel。
- [ ] observer 的本地 xterm layout resize 不触发接管模态；前端尽量不发送 PTY resize；服务端对非 controller resize **必须静默忽略**。
- [ ] 前端仅在 observer 明确输入、粘贴或点击显式“接管”入口时显示接管确认模态窗；旋转、分屏、keep-alive 和其他布局变化不得触发模态。
- [ ] 接管必须经由可识别的服务端动作（例如显式 `take_control` 或 Spec 定义的等效请求）；确认前 observer 发出的 input **丢弃且不缓冲**。
- [ ] 用户取消后，当前输入序列静默保持只读且不重复弹窗；下一次新的明确输入或粘贴手势再次提示；状态栏常驻显示只读状态并提供接管入口。
- [ ] 用户确认后，控制权转移对并发请求具有原子性；新 controller 生效后，旧 controller 被服务端标记为 observer，不能继续写入或发送 PTY resize。
- [ ] 旧 controller 收到 control lost（或等效）通知；新 controller 收到 control granted（或等效）确认；双方 UI 可据此更新角色与只读状态。
- [ ] controller 断开且仍有其它 live attach 时，服务端 **自动**选举其一为新 controller 并通知各方；新 controller 无需再次确认即可输入与 PTY resize。
- [ ] 仅当断开后无剩余 live attach 时 session 处于无 controller；新 attach 再按首连选举成为 controller。
- [ ] Cloud 与 Agent local attach 路径遵循相同的 controller/observer 语义。
- [ ] Browser 与未来 MCP/其它集成若 attach 同一 session，共用同一 controller 规则；不存在双写例外。
- [ ] attach quota 与现有 `workspaceId/sessionId` 唯一性语义不退化；**单 session 双写保护**改为由 controller 门禁强制执行（而非拒绝第二 attach）。
- [ ] 正式修订 `20260719-terminal-resource-quota` 中「每 session 最多一个 browser writer / 409 不变」的表述，使之指向本需求的 controller 模型。
- [ ] WebSocket 建连/控制失败按可识别原因呈现（如 `terminal_attach_quota_exceeded`、设备离线、鉴权失败、`not_controller` 等）；同一 `sessionId + failure reason` 不得连续刷重复 toast。
- [ ] 自动化测试覆盖：多 attach 输出广播、observer 输入/resize/attach-size 拒绝、确认接管、并发 attach 选举、controller 断开后自动移交、quota 交互。

## Open questions

1. 读取历史与实时输出之间的顺序、回放边界及慢 observer 背压策略需要在计划阶段结合现有 terminal relay 行为确认（沿用有界 per-client 队列，慢 observer 不得阻塞 PTY 或 controller）。
2. 角色通知与 takeover 的具体协议形状（新 control type vs 扩展现有 `state`/`started` 字段）在 Spec 阶段确定；需求只要求语义可测。
3. 自动移交或主动 takeover 后，新 controller 是否应立即用其当前本地尺寸执行一次 PTY resize：建议「是」（前端在 granted/auto_granted 后发 resize），细节由 Spec/实现确认。

## Decisions

1. 多端打开同一 session 的默认行为是“可观察、不可控制”，而不是直接 HTTP 409 或自动踢掉已有连接。
2. **废止**「每 session 最多一个 live browser attach」；**保留**「每 session 同一时刻最多一个 controller（唯一可写）」。
3. 输入权与 PTY resize 权绑定到同一个 controller，且同一时刻只能由一个 attach stream 持有。
4. 接管必须由用户明确确认；前端模态窗只承担交互确认，服务端对每个会改变终端状态的控制路径执行 controller 校验。
5. observer 的本地 xterm 允许因容器变化 layout resize；服务端对非 controller 的 PTY resize **必须静默忽略**；前端尽量不发送。
6. 非 controller 的 attach 初始 size **不得**改变 PTY；仅 controller 可在 attach 时或之后 resize。
7. 仅明确用户手势的输入、粘贴，以及显式“接管”按钮触发接管确认；旋转、分屏、keep-alive 等非用户控制意图不得触发。
8. 接管后旧 controller 降为 observer，而不是被默认断开，保留会话连续观察能力。
9. controller 断开后，若仍有其它 live attach，则 **自动**将控制权交给按确定性规则选出的剩余客户端；仅无剩余 attach 时进入无 controller。主动 `take_control`（确认/按钮）仍用于「有人控制时」的移交。
10. observer 取消接管后，本次输入序列静默只读并丢弃且不缓冲；下一次新的明确输入或粘贴手势再次显示确认模态；状态栏常驻只读状态并提供接管入口。
11. 对共享 device binding 的 session，任何已通过既有设备/session 访问授权的用户都可接管；本需求不新增额外角色或授权层。
12. session key 继续使用现有全局唯一的 `workspaceId + sessionId`；不追加 `deviceId`。
13. attach quota 继续按每个存活 browser attach 计数，不因同一 session 的 observer/controller 关系而豁免。
14. **控制权权威**：Agent 为最终 PTY 写门禁；Cloud 同步维护/执行 controller 校验以便 early reject 与角色通知。两端语义一致。
15. Browser / MCP / 其它集成 attach 共用同一 controller 池与规则。
16. WebSocket 建连与控制失败必须按可识别的服务端原因呈现；同一 session 的相同失败不得连续创建重复 toast，需按 `sessionId + failure reason` 合并或限频。

## Risk

1. 去掉 Cloud 独占 `writers` 后，依赖 Agent 多 client 输出与每 stream 独立 attach 生命周期；输入门禁必须新建，不能假设现有 multi-client 已安全。
2. 多个 observer 会增加 terminal 输出扇出、缓存与慢消费者背压压力；必须保留每客户端隔离和有界队列策略，不能由慢 observer 阻塞 controller 或 PTY。
3. 输入、resize、detach、attach 初始 size 与接管并发时容易出现短暂双控制或错误释放；服务端状态机必须在同一锁/原子边界内维护 controller 与 observer 集合及选举。
4. 前端 WebSocket `onerror` 目前对所有握手失败显示“可能配额超限”的泛化文案；本需求实现时应使 `not_controller`、quota、设备离线等与其它连接失败原因可被明确区分。
5. 本需求修订 `20260719-terminal-resource-quota` 的单 attach/409 表述；须保持每用户配额、HTTP 429 与降配不主动踢已有连接的原则不变。
6. 同 session 多端同时观察会占用 attach 配额 headroom，可能与 keep-alive 多 tab 叠加触发 429；产品预期需在实现/UI 中可理解。
7. 自动移交或主动 takeover 后，新 controller 应有清晰的一次 PTY resize 路径；observer 本地 layout 与 PTY 尺寸不一致时只读可接受。

## User review notes

- 用户提出在手机和平板同时打开同一设备、同一 workspace/session 的需求。
- 用户否决“设备 B 一连接就直接断开设备 A”的默认策略。
- 用户确认的交互方向：允许设备连接；当 observer 尝试输入时弹出模态窗，让用户确认是否接管会话。
- 用户确认当前 `workspaceId` 与 `sessionId` 都是唯一的；不需要为 session key 增加 `deviceId`。
- 用户更新（2026-07-20）：controller **断开**后改为 **自动**让仍在线的某一端成为 controller（不再保持无 controller 等确认）。
- 用户采纳：observer 的 layout resize 不触发接管；仅 controller 可改变 PTY 尺寸，observer 的服务端 resize 被静默忽略。
- 用户采纳：仅明确用户手势输入、粘贴或显式接管按钮触发确认；旋转、分屏和 keep-alive 布局变化不得弹窗。
- 用户采纳：observer 取消接管后，本次输入序列静默只读；下一次新的明确输入或粘贴手势再次提示，并通过常驻只读状态和接管入口减少误触。
- 用户采纳：共享 device binding 下，既有设备/session 访问授权通过的用户均可接管；不新增额外角色授权。
- 用户采纳：WebSocket 建连错误需按实际失败原因展示；相同 session 和原因的重复失败不得刷屏。
- 审查采纳：废止每 session 单 live attach/409，改为单 controller + 多 observer；Agent 最终写门禁；attach 初始 size 纳入只读约束；attach 成功下发 role；Related 与配额文档交叉修订。

## Implementation scope note

- **2026-07-20**：实现按 [docs/spec/20260720-multi-device-terminal-control.md](../spec/20260720-multi-device-terminal-control.md) 的 **V1 语义收缩** 交付。
- V1 产品句：同一 session 可多端同时看；任意时刻只有一端能打字和改 PTY 尺寸；另一端点「接管」或确认后成为唯一控制端；**当前 controller 断开且仍有其它 live attach 时自动移交控制权**；**不做**严格并发 CAS 与 attach preflight。
- 与上文 Acceptance 中「并发 takeover 至多一个成功」「upgrade 失败可识别原因」等加严项冲突时，**以 V1 Spec 为准**；其余主路径（多观察、单写、确认接管、服务端门禁、配额）不变。
