# 事务边界重构验证 / Transaction Boundary Refactor Verification

最后修改时间: 2026-07-05 20:09:10

Review status: Accepted / 接受

## Requirement alignment

按 `docs/requirement/20260705-transaction-usage-audit-findings.md` 验证。本次验证只覆盖 transaction boundary / unit of work，不覆盖 OAuth2 / cloud OAuth / device report。

## Spec alignment

不适用。当前任务使用轻量模式 / light，没有单独 spec 文档；按 requirement / 需求核对。

## Plan alignment

不适用。当前任务使用轻量模式 / light，没有单独 plan 文档；按 requirement / 需求核对。

## Actual diff summary

本次事务边界相关改动集中在以下文件：

- `docs/requirement/20260705-transaction-usage-audit-findings.md`
- `internal/cloud/application/user/auth/service.go`
- `internal/cloud/repository/user/auth/repository.go`
- `internal/cloud/application/user/auth/service_test.go`
- `internal/agent/repository/task/state/db_store.go`
- `internal/agent/repository/task/state/store.go`
- `internal/agent/repository/task/state/db_store_test.go`
- `internal/agent/application/task/terminal/registry.go`
- `internal/agent/application/task/terminal/runtime.go`
- `internal/agent/application/bootstrap/commands.go`

`git diff HEAD --stat` 对上述事务相关路径的统计：

```text
 .../20260705-transaction-usage-audit-findings.md   | 315 +++++++++++++++++++++
 internal/agent/application/bootstrap/commands.go   |  68 ++---
 .../agent/application/task/terminal/registry.go    |  11 +-
 .../agent/application/task/terminal/runtime.go     |  18 +-
 internal/agent/repository/task/state/db_store.go   | 233 +++++++++++----
 .../agent/repository/task/state/db_store_test.go   |  78 ++++-
 internal/agent/repository/task/state/store.go      |  52 ++++
 internal/cloud/application/user/auth/service.go    | 120 ++++----
 .../cloud/application/user/auth/service_test.go    | 224 +++++++++++++++
 internal/cloud/repository/user/auth/repository.go  | 278 +++++++++++++-----
 10 files changed, 1160 insertions(+), 237 deletions(-)
```

## Expected vs actual changed files

| Area | Expected | Actual | Result |
| --- | --- | --- | --- |
| Requirement | Update transaction boundary requirement | Updated and marked Accepted for verification entry | Pass |
| Cloud auth service | Move C1-C3 transaction boundary into service / use-case layer; make C4 explicit | `VerifyEmail` / `ConfirmPasswordReset` / `sendCode` use tx-bound repository; `Login` returns `UpdateLastLogin` error | Pass |
| Cloud auth repository | Remove use-case-specific combined methods as primary design; avoid db/q compatibility field | root `Repository` holds `*sql.DB`; tx-bound `TxRepository` holds `*sql.Tx`; no `q` / `inTx` / `WithTx` compatibility layer remains | Pass |
| Agent DB store | Fix A1-A5 transaction consistency without `db/q/inTx` compatibility layer | `DbStore` no longer has `q` / `inTx`; lifecycle write methods use explicit DB transactions | Pass |
| Terminal runtime / registry | Stop splitting lifecycle writes through optional fallback paths | `RuntimeStore` requires transaction lifecycle methods and terminal code calls them directly | Pass |
| Bootstrap CLI path | Reuse lifecycle transaction methods | CLI lifecycle path calls process/state and exit/state combined methods | Pass |
| Tests | Add focused coverage for transaction behavior and run relevant Go tests | Focused transaction tests passed; bootstrap package check also passed on rerun | Pass |

## Acceptance checklist

- [x] 文档只覆盖 transaction boundary / unit of work；未混入 OAuth2 / cloud OAuth / device report 实现。
- [x] cloud C1-C4 均有实现覆盖：
  - C1：`sendCode` 的 code 创建 / failed invalidation / email log DB 写入在 service transaction boundary 下完成；邮件发送作为外部副作用仍在 DB transaction 外显式处理。
  - C2：`VerifyEmail` 在同一 transaction 中更新用户 verified 状态并标记 code used。
  - C3：`ConfirmPasswordReset` 在同一 transaction 中更新密码并标记 code used。
  - C4：`Login` 不再静默吞掉 `UpdateLastLogin` 错误，采用强一致审计语义。
- [x] agent A1-A5 均有实现覆盖：
  - A1：runtime start 的 process record 与 running state 通过同一 DB transaction 写入。
  - A2：runtime exit 的 exit record、final state、可选 history metadata 通过同一 DB transaction 写入。
  - A3：CLI exec 生命周期复用相同 lifecycle persistence 方法。
  - A4：`UpdateSession` 在 transaction 内读改写，并通过 `updated_at` CAS 检测 lost update。
  - A5：通过 A1/A2/A3 的同 tx 写入降低 `sessions.current_state` 滞后来源。
- [x] 不再保留被拒绝的 `Repository{db, q, inTx}` / `DbStore{db, q, inTx}` 兼容字段设计。
- [x] 不再保留 terminal runtime 的 optional saver fallback；application contract 明确要求事务性 lifecycle 方法。
- [x] 外部副作用边界已保留为风险：DB transaction 不覆盖邮件发送、PTY 启动或 history 文件写入。

## Command results

### Focused transaction tests

命令：

```text
go test ./internal/cloud/application/user/auth ./internal/agent/repository/task/state ./internal/agent/application/task/terminal
```

结果：

```text
ok   termbridge-go/internal/cloud/application/user/auth   (cached)
ok   termbridge-go/internal/agent/repository/task/state   (cached)
ok   termbridge-go/internal/agent/application/task/terminal (cached)
```

### Bootstrap package focused check

命令：

```text
go test ./internal/agent/application/bootstrap
```

结果：通过。

```text
ok   termbridge-go/internal/agent/application/bootstrap (cached)
```

判断：CLI exec 生命周期相关 package 当前可编译并通过测试；A3 验证不再被 bootstrap package build failure 阻塞。

## Scope deviation

当前工作区存在大量 OAuth/device/acronym casing 相关改动。验证时已将事务相关路径单独列出，并运行聚焦测试，避免把无关 build failure 混入事务边界验收。

`git status --short` 显示工作区包含大量非本任务文件变更；因此不能把全仓状态视为本次 transaction boundary 的独立 diff。

## Risks

1. `sendCode` 仍然无法用 DB transaction 覆盖邮件发送这个外部副作用；当前实现选择在邮件发送后，将 code / invalidation / email log 作为 DB 内部一致写入。若业务要求“邮件发送成功但 DB 写失败”也可恢复，需要后续 outbox 或 compensation 设计。
2. Agent runtime 的 PTY 启动和 history 文件写入仍然不受 DB transaction 回滚保护；本轮只保证 DB 内部 lifecycle 状态一致。
3. `SaveProcessState` / `SaveExitState` / `SaveSessionExitState` 仍位于 store 接口上，虽然用于表达 lifecycle persistence contract，但 requirement 中曾指出这类方法不应长期变成 repository 承载业务语义的终点；后续若继续演进，可进一步抽出 application-level unit of work / lifecycle repository 端口。
4. 工作区存在与本任务无关的未完成改动，尤其 acronym casing 和 OAuth/device 相关文件；提交前需要拆分或确保对应任务一起完成。

## Incomplete items

- 未运行全仓 `go test ./...`，因为当前工作区存在大量非本任务改动；本次只验证事务边界相关 focused packages。
- 未处理 OAuth2 / cloud OAuth / device report，这是本 requirement 明确 non-goal。
- 未引入 outbox / compensation，这是本 requirement 明确不强制的外部副作用后续设计。

## Conclusion

事务边界重构的聚焦验证通过：cloud auth、agent runtime/state 与 CLI bootstrap 相关 transaction paths 已按 requirement 调整并通过相关 focused tests。当前工作区仍存在大量非本任务改动，因此提交前应继续拆分或确认交付边界。
