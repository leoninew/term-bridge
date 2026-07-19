# 终端与连接资源配额（按用户 / 管理员动态调配）
最后修改时间: 2026-07-19 17:25:41

Review status: Accepted

## Flow mode / Stage

标准模式 / standard；当前阶段：需求 / Requirement（Accepted）；进入计划 / Plan → 实现

## Related

- 关联体验优化：`docs/requirement/20260719-terminal-tab-keepalive.md`（tab keep-alive，**独立交付**，不依赖本配额系统完成）。
- 本需求回答：前端策略 vs 后端硬顶、如何按用户配额限制、管理员如何动态调配，避免客户端篡改拖垮服务。

## Background

终端相关资源分多层：

1. **Running session / PTY**：真正重的进程成本；与是否 attach 无关。
2. **Browser terminal attach / WS**：keep-alive 会让同时 attach 数上升。
3. **前端 keep-alive 参数**（`maxHotTerminals` / `disposeDelayMs`）：由浏览器执行，可被本机用户修改。
4. **BrowserRuntimeConfig**：服务端注入的部署默认值，属于**公开 bootstrap**，不是安全边界。

现状缺口：

- 没有按用户（或设备/主体）的 **可管理资源配额**。
- 若仅靠前端 clamp 或 RuntimeConfig，无法阻止恶意/误配客户端同时挂大量 attach。
- Cloud / 多用户场景需要管理员能按主体调配额，而不是改全局常量或发前端版。

## Goal

1. 建立清晰的 **前后端控制分层**：
   - 前端：体验策略与默认建议（可被用户改，不信任）。
   - 后端：强制配额与拒绝策略（信任边界）。
2. 后端为 **每个配额主体（默认：用户）** 维护资源配额，用于限制终端相关并发与（可选）其它连接类资源。
3. 支持 **管理员动态调配** 配额（创建/查询/更新），变更后对后续新占用生效；策略需定义是否影响已有连接。
4. 配额拒绝时返回稳定业务错误码与安全文案，便于前端提示。
5. 与 keep-alive 协作：keep-alive 可以尝试多路 attach，但最终以后端配额为准。

## Non-goal

1. 不在本需求中实现 tab keep-alive UI 行为（见关联文档）。
2. 不做完整通用「多维配额中台」（计费、账单、按 API 的全站 rate limit 平台化）。
3. 首版不强制做「已连接连接的实时挤下线」高级调度（可列为可选/后续），但要定义默认策略。
4. 不把配额密钥或管理员接口暴露给浏览器任意调用；管理接口需鉴权与角色。
5. 不改变 session 单 browser writer 的既有语义（每 session 仍最多一个 browser attach writer，除非另立需求）。
6. 不在本需求中重做 OAuth/用户体系；复用现有 Cloud/Agent 身份模型能识别的主体。

## Control plane（前后端控制策略）

### 分层模型

| 层 | 名称 | 信任 | 作用 |
| --- | --- | --- | --- |
| L1 | 前端代码默认 | 不信任 | 无配置时的体验默认 |
| L2 | BrowserRuntimeConfig / 部署 yaml | 半信任（可被客户端改） | 部署默认建议：如 keep-alive 4/30s |
| L3 | **后端资源配额（本需求）** | **信任** | 按用户（主体）强制限制；管理员可调 |
| L4 | 全局/节点保护上限 | 信任 | 防止单节点过载的总闸（可选，与用户配额同时生效取更严） |

规则：

- **生效值** = 各层约束的交集（取更严）。
- 客户端上报或本地改大 **不能** 提高 L3/L4。
- L2 仅影响“客户端想怎么做”；L3 决定“服务器允不允许”。

### 职责划分

**前端**

- 读取 L1/L2 决定 keep-alive 等 UX 行为。
- 可展示当前配额/剩余（若 API 提供），但不依赖前端守法。
- 收到配额拒绝错误时友好提示（不要死循环重连打爆）。

**后端（Agent 与/或 Cloud）**

- 在资源占用点强制检查配额（至少：terminal attach；可选：running session 数、其它 WS）。
- 记录占用与释放，保证连接断开后配额归还。
- 提供管理员查询/调整配额的 API（Cloud 侧为主；Agent 本地模式见 Open questions）。

### 与 keep-alive 的关系

```text
前端想保活 N 路
  → 发起最多 N 条 attach
  → 每条 attach 后端检查用户配额
  → 超限：拒绝该 attach（keep-alive 降级为更少 live 连接）
```

keep-alive 需求 **可先上线**；本配额上线后自动成为其上界。

## Quota model（配额模型）

### 主体（Subject）

默认主体：**用户（user id）**。

可选扩展（本需求需在 Open questions 确认优先级）：

- device / agent 实例
- organization / tenant
- 本地无登录场景的 `local` 伪主体

### 首版建议配额项

| 配额键 | 含义 | 占用点 | 释放点 | 默认值（待确认） |
| --- | --- | --- | --- |
| `terminal.concurrent_attaches` | 同时 browser 终端 attach 数 | terminal WS attach 成功 | WS/client detach 或异常关闭 | 8 |
| `terminal.concurrent_running_sessions`（可选 P1） | 同时 running session/PTY 数 | create/start session | session stop/exit | 例如 20 |
| `terminal.keep_alive_max_hot`（可选） | 服务端认可的 keep-alive 建议/上限，可下发到客户端 | 配置下发 | n/a | 4 |

说明：

- **P0 必须**：`terminal.concurrent_attaches`（直接回应“调大保活打挂服务器”）。
- running sessions 配额能限制更根本的进程成本，建议 P1。
- keep-alive 数字本身不必强行做成配额项；用 attach 并发即可卡住。

### 配额数据

每个主体一份配额记录，至少包含：

- `subject_type` / `subject_id`
- 各 quota key 的 `limit`
- `updated_at` / `updated_by`（管理员）
- 可选：`note`、生效时间窗口

运行时占用：

- 内存计数 + 连接生命周期绑定（首选，低延迟）
- 必要时落库仅存 **limit 配置**；占用不必强行持久化（进程重启后以实际连接重建）。

### 默认与回退

1. 主体无自定义配额 → 使用系统默认模板。
2. 管理员更新 → 覆盖该主体对应 key。
3. 删除自定义 → 回退默认模板。
4. 非法/缺失配置 → 安全默认（偏严或沿用代码默认，需在实现中固定一种并测）。

### 超限行为

- 新占用：拒绝，返回稳定 code（例如 `quota_exceeded` / `terminal_attach_quota_exceeded`）+ 安全 message。
- 已有占用：默认 **不主动断开**（避免管理员降配时瞬间踢光）；可选后续支持“drain/强制回收”。
- 前端：提示并停止对该资源的重试风暴。

## Admin dynamic allocation（管理员动态调配）

### 角色与入口（方向）

- **Cloud 管理员**（或具备 admin 角色的用户）通过管理 API/未来管理台调整某用户配额。
- 普通用户只读自己的配额摘要（可选）。
- Agent 纯本地模式：见 Open questions（本地配置模板 vs 无多用户）。

### 管理能力（验收级）

1. 查询某用户当前 limit 与当前 usage（至少 attaches）。
2. 更新某用户单个或多个 quota limit。
3. 重置为系统默认。
4. 列出/审计最近变更（最低：日志；更好：审计表，可 P1）。

### 动态生效

- **升配**：立即允许新的占用。
- **降配**：
  - 默认：仅影响新占用；已有连接保留直到自然断开。
  - 可选（非必须）：提供 `enforce=true` 触发超额回收（后续）。

## User scenarios

### S1. 普通用户正常使用 keep-alive

1. 用户默认配额 attach=8，前端 hot=4。
2. 打开 4 个 running tab 保活，全部 attach 成功。

### S2. 客户端篡改调大保活

1. 用户把前端 `maxHotTerminals` 改成 99。
2. 尝试同时 attach 超过配额。
3. 超出部分被后端拒绝；已附着连接不受影响。
4. 服务器占用不超过该用户 limit。

### S3. 管理员提升配额

1. 管理员将用户 A 的 `terminal.concurrent_attaches` 从 8 调到 16。
2. 用户 A 之后可以 attach 更多终端。

### S4. 管理员降低配额

1. 用户 A 当前已 attach 6，limit 从 8 改为 4。
2. 默认：6 条保持；新 attach 拒绝，直到占用降到 <4。

### S5. 连接释放归还

1. 用户关闭 tab / 断开 WS。
2. usage 下降，可再次 attach。

### S6. 与 keep-alive 并存

1. keep-alive 先上线、配额后上线：行为兼容。
2. 配额先上线、keep-alive 后上线：keep-alive 受配额自然限制。

## Acceptance

1. 存在按用户的配额模型与默认模板。
2. terminal attach 路径强制检查 `terminal.concurrent_attaches`。
3. 超限返回稳定错误码，不提升权限、不泄漏内部细节。
4. 管理员可查询/修改/重置用户配额；鉴权正确。
5. 断开连接后 usage 正确归还；无长期泄漏（异常路径需覆盖）。
6. 文档明确 L1/L2/L3 信任边界：前端配置不可抬高后端配额。
7. 与 `20260719-terminal-tab-keepalive` 无硬依赖，可独立发布。

## Open questions

### Q1. 配额主体

- A. 仅 Cloud user id
- B. Cloud user + device/agent 维度
- C. 本地 Agent 使用单一 `local` 主体 + 配置文件调配

**推荐**：Cloud 用 A 或 B；本地 Agent 用 C（yaml 默认 + 可选本地覆盖文件）。

### Q2. 管理面落点

- A. 仅 Cloud Admin API（推荐多用户）
- B. Agent 本地管理 API
- C. 两者都有

### Q3. 首版配额项范围

- P0 only attaches？
- 是否包含 running sessions？

**推荐 P0：attaches；P1：running sessions。**

### Q4. 降配是否踢现有连接

**推荐默认不踢。**

### Q5. 是否向客户端下发“有效上限”

用于前端把 hot set 自动钳到配额，减少无谓失败。  
**推荐**：提供只读 `GET` 配额摘要；keep-alive 可选用。

## Decisions

1. 前端/RuntimeConfig 只做体验与建议，**不是**防滥用边界。
2. 防服务器压力靠 **后端按用户配额**（本需求）。
3. keep-alive 需求继续独立实现，不阻塞本需求。
4. 管理员动态调配针对配额 limit，不是改全体前端发版。

## Risk

1. 占用计数泄漏导致“配额用尽假死”。
2. Cloud 与 Agent 双端计数不一致（隧道场景要明确以谁为准）。
3. 身份缺失（未登录/本地）时的主体映射错误。
4. 管理员误配过小影响可用性。
5. 范围膨胀成通用配额中台。

## Assumptions

1. 能从现有认证上下文解析稳定 user id（Cloud）；本地模式可用退化主体。
2. attach 成功/失败/断开路径可挂钩子做 acquire/release。
3. 首版以正确性与可运营性优先，不追求复杂实时调度。

## User review notes

- 2026-07-19：从 keep-alive 讨论中拆出本独立需求。
- 待用户确认主体、管理面、P0 配额项后进入 Plan。

### 关闭记录（2026-07-19 用户：开始推进实现）

| 项 | 结论 |
| --- | --- |
| Q1 主体 | Cloud：user id；本地 Agent：固定主体 local |
| Q2 管理面 | P0：Cloud Admin API + yaml 默认；本地仅 yaml 默认/覆盖 |
| Q3 配额项 | P0：	erminal.concurrent_attaches（默认 8）；running sessions 为 P1 |
| Q4 降配 | 默认不踢现有连接 |
| Q5 下发摘要 | P0 提供用户只读 GET；前端可选用；管理端 CRUD |
| 管理员鉴权 | P0：cloud.admin.user_ids / emails 配置白名单（非完整 RBAC） |
