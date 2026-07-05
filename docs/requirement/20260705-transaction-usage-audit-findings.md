# 事务边界重构需求 / Transaction Boundary Refactor Requirement

最后修改时间: 2026-07-05 20:01:33

Review status: Accepted / 接受

## Background

本文件只记录 **事务管理问题与事务边界重构需求**。OAuth2 / cloud OAuth / 设备上报相关问题在 `docs/requirement/20260705-agent-cloud-oauth-device-binding.md` 及对应会话中处理，本文不展开，避免两个工作流互相干扰。

当前代码存在两个持久化边界：

- **agent 侧**：`internal/agent/...` 拥有本地 runtime / workspace / session / state 持久化。
- **cloud 侧**：`internal/cloud/...` 拥有用户、认证码、邮件发送日志等 cloud DB 表；workspace / session / runtime API 在 cloud 侧只做代理转发。

前一轮审计已经识别出 cloud C1-C4 与 agent A1-A5。后续实现讨论明确了一点：

> 正确的长期实践不应继续把业务事务拆成多个 repository 方法各自 `BeginTx` / `Commit`，也不应通过不断新增 repository 组合方法来承载业务用例；事务边界应上移到 application service / domain service / unit of work 层。

因此本文从单纯 audit findings 更新为 transaction boundary / unit of work 重构需求。

## Goal

1. 引入明确的 transaction boundary / unit of work 机制。
2. 将跨多个持久化动作的业务事务边界放在 application service / domain service 层。
3. Repository 保留为持久化端口，提供可在同一 tx 内执行的细粒度数据操作，而不是由每个 repository 方法自行决定完整业务事务边界。
4. 修复 cloud C1-C4 与 agent A1-A5 暴露的业务中间态问题。
5. 保持 OAuth2 / device binding / device report 工作流隔离，不在本文或本任务中处理。

## Non-goal

- 不处理 OAuth2 / cloud OAuth / 设备上报问题。
- 不引入分布式事务。
- 不试图把邮件发送等外部副作用纳入 DB transaction。
- 不把所有单 SQL repository 方法都改成显式 tx 方法；只有被业务用例组合调用、会形成中间态的路径需要纳入 unit of work。
- 不要求一次性重写所有 repository；优先覆盖 C1-C4 / A1-A5 涉及的路径。

## Target architecture

### 1. Transaction boundary 所在层级

目标结构：

```text
Application Service / Domain Service / Use Case
  Begin unit of work
    repository operation A using same tx
    repository operation B using same tx
    repository operation C using same tx
  Commit / Rollback
```

避免继续扩散以下结构：

```text
repo.A() // internally begin + commit
repo.B() // internally begin + commit
repo.C() // internally begin + commit
```

也避免长期依赖以下临时结构：

```text
repo.UseCaseSpecificCombinedMethod()
```

例如：

- `MarkEmailVerifiedAndCodeUsed`
- `UpdatePasswordAndMarkCodeUsed`
- `RecordCodeDelivery`
- `SaveProcessState`
- `SaveExitState`
- `SaveSessionExitState`

这类方法可以作为短期修复手段，但长期会让 repository 承载业务用例语义，导致 repository 变厚、职责漂移。

### 2. Repository 目标职责

Repository 应提供两类能力：

1. **单表/聚合内数据操作**：例如 `MarkEmailVerified`、`MarkCodeUsed`、`UpdatePassword`、`UpdateLastLogin`。
2. **tx-aware 执行能力**：同一组 repository 操作可以绑定到同一个 `*sql.Tx` 或抽象 `DBTX`。

推荐方向之一：

```go
type DBTX interface {
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type TxManager interface {
    WithinTx(ctx context.Context, fn func(ctx context.Context, repos Repositories) error) error
}
```

或者：

```go
func (r *Repository) WithTx(tx *sql.Tx) *Repository
```

核心要求不是具体接口名，而是：

- application / domain 层决定业务事务范围；
- repository 层只执行当前 tx / db handle 上的数据操作；
- 不再靠多个 repository 方法独立 commit 串接业务流程。

### 3. 外部副作用边界

邮件发送、PTY 进程启动、history 文件写入等外部副作用不能被 DB transaction 原子包住。处理原则：

1. DB 内部相关写入必须保持同一 transaction。
2. 外部副作用与 DB 的一致性如果必须保证，需要 outbox / pending state / compensation 设计，而不是假装 DB transaction 可以覆盖外部系统。
3. 本轮需求只要求明确记录该边界，不强制引入 outbox；若实现时发现邮件发送一致性无法满足，应记录为风险或拆出后续任务。

## Assessment summary

### 当前问题性质

| 编号 | 当前问题 | 根因 | 目标修复方式 | 严重度 |
| --- | --- | --- | --- | --- |
| C1 | `sendCode` 认证码、邮件结果、email log 跨多个独立步骤 | service 用例没有统一 tx；邮件是外部副作用 | auth use case transaction + 明确外部副作用边界 | high |
| C2 | `VerifyEmail` 用户状态与验证码 used 标记分两次写 | 两个 repository 方法各自独立写 | service 层 unit of work 内调用 `MarkEmailVerified` + `MarkCodeUsed` | high |
| C3 | `ConfirmPasswordReset` 密码更新与验证码 used 标记分两次写 | 两个 repository 方法各自独立写 | service 层 unit of work 内调用 `UpdatePassword` + `MarkCodeUsed` | high |
| C4 | `Login` 忽略 `last_login_at` 写失败 | audit 写入失败被 `_ =` 吞掉 | 明确业务语义：要么登录依赖 audit 写成功，要么记录为 best-effort；不能静默不透明 | medium |
| A1 | runtime start 事件拆成 process 与 running state 多次 commit | lifecycle use case 没有统一 tx | agent runtime unit of work：process + state 同 tx | medium |
| A2 | runtime exit 拆成 history metadata、exit、final state 多段持久化 | lifecycle use case 没有统一 tx | agent runtime unit of work：session metadata + exit + final state 同 tx | medium |
| A3 | CLI exec 生命周期多次独立持久化 | lifecycle use case 没有统一 tx | CLI runtime use case 复用 agent runtime unit of work | medium |
| A4 | `UpdateSession` 读-改-写存在 lost update | 读写不在同一 tx，且无 CAS/version | tx 内读写 + version / updated_at CAS | medium |
| A5 | `ensureCurrentRunTx` 依赖 `sessions.current_state` 准确性 | 上游 partial commit 会污染 current state | 修复 A1/A2/A3 后减少滞后来源；必要时补充状态修复/校验 | low |

### 对短期组合 repository 方法的评估

短期组合 repository 方法可以快速压制中间态，但不是长期目标：

- 优点：改动小，容易验证，能快速降低数据不一致风险。
- 缺点：repository 开始表达业务用例，后续每个业务流程都可能新增一个专用组合方法。
- 结论：如当前代码已有这类方法，应视为 tactical fix；后续重构时应迁移到 unit of work，并尽量回收或降级为 tx-aware helper。

## Cloud side requirement

### C1. `sendCode` 需要 use-case transaction 与外部副作用边界

涉及路径：

- `internal/cloud/application/user/auth/service.go`
- `Register`
- `ResendVerification`
- `RequestPasswordReset`
- `sendCode`
- `internal/cloud/repository/user/auth/repository.go`
- `auth_codes`
- `email_delivery_logs`

目标要求：

1. 认证码失效、新认证码创建、邮件发送结果日志写入必须形成清晰一致的业务流程。
2. DB 内部写入应在 service/domain 层的 unit of work 内完成。
3. 邮件发送是外部副作用，不能简单包进 DB transaction；实现必须明确选择：
   - 先 DB 记录 pending，再发送邮件，再记录 delivery result；或
   - 使用 outbox / async delivery；或
   - 明确接受发送成功但 DB 后续失败的风险，并记录补偿策略。
4. 不允许继续 `_ = SaveEmailLog` / `_ = InvalidateCode` 这类静默吞错。

### C2. `VerifyEmail` 需要 service 层 unit of work

涉及路径：

- `internal/cloud/application/user/auth/service.go:138-153`
- `internal/cloud/repository/user/auth/repository.go`
- `users`
- `auth_codes`

目标要求：

1. `verifyCode` 后，`users.status/email_verified_at` 与 `auth_codes.used_at` 必须在同一 unit of work 内提交。
2. Repository 不应长期保留 use-case specific 的 `MarkEmailVerifiedAndCodeUsed` 作为主设计。
3. 目标形态应是 application service 开启 tx，并在同一 tx-bound repository 上调用：
   - `MarkEmailVerified`
   - `MarkCodeUsed`

### C3. `ConfirmPasswordReset` 需要 service 层 unit of work

涉及路径：

- `internal/cloud/application/user/auth/service.go:220-242`
- `internal/cloud/repository/user/auth/repository.go`
- `user_identities`
- `auth_codes`

目标要求：

1. `UpdatePassword` 与 `MarkCodeUsed` 必须同一 unit of work 提交。
2. 不允许密码已更新但 reset code 未 used 的中间态。
3. Repository 不应长期保留 use-case specific 的 `UpdatePasswordAndMarkCodeUsed` 作为主设计。

### C4. `Login` 的 `last_login_at` 写入语义必须显式

涉及路径：

- `internal/cloud/application/user/auth/service.go:155-175`
- `internal/cloud/repository/user/auth/repository.go`
- `users.last_login_at`

目标要求：

1. 不能继续 `_ = UpdateLastLogin` 静默吞错。
2. 必须明确选择业务语义：
   - **强一致审计**：`UpdateLastLogin` 失败则不签发 token；或
   - **best-effort 审计**：允许登录成功，但必须有结构化日志、指标或后续补偿，不允许静默丢失。
3. 若选择强一致审计，`UpdateLastLogin` 应发生在 token 签发前，失败直接返回错误。
4. 若选择 best-effort，应在 requirement / risk 中显式说明审计字段不参与登录成功判定。

## Agent side requirement

### A1. runtime start 事件需要统一 unit of work

涉及路径：

- `internal/agent/application/task/terminal/registry.go`
- `internal/agent/repository/task/state/db_store.go`
- `session_runs.process_json`
- `session_runs.state`
- `sessions.current_state`

目标要求：

1. process record 与 running state 必须同一 DB transaction 提交。
2. application 层表达“process started”这个 lifecycle event。
3. repository 层只负责在当前 tx 内写 process / state 字段。

### A2. runtime exit 事件需要统一 unit of work

涉及路径：

- `internal/agent/application/task/terminal/runtime.go`
- `internal/agent/repository/task/state/db_store.go`
- `sessions.history_json`
- `session_runs.exit_json`
- `session_runs.state`
- `sessions.current_state`

目标要求：

1. exit record 与 final state 必须同一 DB transaction 提交。
2. history truncated metadata 如果参与本次退出事件，也应并入同一 unit of work。
3. 不允许 exit 已写但 `sessions.current_state` 未推进的中间态。

### A3. CLI exec 生命周期应复用 runtime unit of work

涉及路径：

- `internal/agent/application/bootstrap/commands.go`
- `internal/agent/repository/task/state/db_store.go`

目标要求：

1. CLI exec 的 process started 与 state running 使用同一 unit of work。
2. CLI exec 的 exit record、final state、history metadata 使用同一 unit of work。
3. CLI 路径不应复制一套与 terminal runtime 不一致的事务脚本。

### A4. `UpdateSession` 需要 tx 内读写与并发冲突检测

涉及路径：

- `internal/agent/repository/task/state/db_store.go`
- `internal/agent/application/task/terminal/registry.go`

目标要求：

1. `UpdateSession` 不能继续 `LoadSession -> in-memory update -> SaveSession` 的无事务读改写。
2. 读写应发生在同一 transaction 内。
3. 必须使用 version / updated_at CAS / row lock 等机制检测 lost update。
4. 并发冲突不能静默覆盖，应返回明确错误。

### A5. `ensureCurrentRunTx` 依赖 state 准确性的风险需要通过上游事务修复降低

涉及路径：

- `internal/agent/repository/task/state/db_store.go`

目标要求：

1. `ensureCurrentRunTx` 自身继续保持 repository tx 内一致。
2. 优先通过修复 A1/A2/A3 避免 `sessions.current_state` 滞后。
3. 如果后续仍发现 current run 判断受历史脏状态影响，应增加状态修复、校验或 recovery 机制；不要在本轮无依据地复杂化 `ensureCurrentRunTx`。

## Acceptance

1. 文档只覆盖 transaction boundary / unit of work；不混入 OAuth2 / cloud OAuth / device report。
2. cloud C1-C4 与 agent A1-A5 均有目标事务边界描述。
3. 明确 repository 组合方法只是 tactical fix，不是长期目标架构。
4. 明确 application service / domain service / unit of work 是业务事务边界所在层。
5. 明确外部副作用不能由 DB transaction 直接保证原子性。
6. 对 C4 给出强一致审计与 best-effort 审计两种业务语义选择，并要求实现时显式选择。
7. 对 A4 要求 tx 内读写与 lost update 检测。
8. 不要求本文件直接给出完整代码实现细节，但实现阶段必须能据此拆分 repository tx-aware 能力和 service/domain unit of work。

## Risks / Assumptions

1. 引入 unit of work 会改变 repository API，可能影响较多调用方。
2. 如果短期修复已经新增了 use-case specific repository 方法，后续重构需要清理或降级这些方法，避免形成长期双轨。
3. 邮件发送与 DB 状态之间仍存在外部副作用一致性问题；如果业务要求严格一致，需要单独设计 outbox 或 compensation。
4. Agent runtime 同时涉及 DB、PTY 进程、history 文件，DB transaction 只能覆盖 DB 状态，不能回滚已启动的进程或已写入的 history 文件。
5. 当前工作区存在 OAuth/device 相关未完成改动，验证 transaction boundary 时需要避免被无关编译错误干扰。

## User review notes

- 用户明确要求 OAuth2 相关问题在另一个会话处理，本文和本任务只处理事务问题。
- 用户指出 agent 侧 5 个问题此前未被改动，后续实现必须覆盖 A1-A5。
- 用户进一步确认：正确实践应引入 transaction boundary / unit of work，而不是在 repository 层做过细粒度、用例组合式事务控制。
- 本次更新将需求评估从“列出事务问题”提升为“以 unit of work 为目标架构的事务边界重构需求”。
