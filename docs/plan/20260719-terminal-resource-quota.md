# Plan / 计划：终端资源配额

最后修改时间: 2026-07-19 17:25:41

Review status: Accepted

## Flow mode / Stage

标准模式 / standard；计划 / Plan（Accepted）；进入实现 / Implementation。

## Requirement basis

- [docs/requirement/20260719-terminal-resource-quota.md](../requirement/20260719-terminal-resource-quota.md)
- 关联 keep-alive（独立）：[docs/requirement/20260719-terminal-tab-keepalive.md](../requirement/20260719-terminal-tab-keepalive.md)

## Locked P0 scope

| 项 | 值 |
| --- | --- |
| 配额项 | 	erminal.concurrent_attaches 默认 **8** |
| 主体 | Cloud = JWT sub（user id）；Agent local = local |
| 占用点 | browser terminal WS attach 成功前 acquire |
| 释放点 | WS 结束 / detach / Accept 失败后的 defer |
| 超限 | HTTP **429**，code=	erminal_attach_quota_exceeded |
| 降配 | 不踢已有连接 |
| 管理 | Cloud：查询/更新/重置用户 limit；配置 admin 白名单 |
| 用户只读 | GET 自己的 limit + usage |
| 非本 P0 | running sessions 配额、强制挤下线、完整 RBAC、计费中台 |

## Implementation approach

`	ext
shared/application/quota.AttachCounter
  TryAcquire(subject, limit) / Release(subject) / Usage(subject)

Agent:
  bridgeTerminalStream → subject=local, limit=config.terminal.quota.concurrent_attaches

Cloud:
  handleTerminalWS → subject=claims.Sub
  limit = user_quota_limits override || default
  admin whitelist from cloud.admin.user_ids/emails
  repo + migrations for overrides
  routes: GET /api/me/quota, GET|PUT|DELETE /api/admin/users/{id}/quota

Frontend:
  map 429 code → user-visible error; no reconnect storm
`

## Steps

1. shared AttachCounter + errors + unit tests
2. config: terminal.quota.concurrent_attaches + cloud.admin.*
3. Agent wire + 429 mapping
4. Cloud migration + repository + wire enforce
5. Cloud user/admin JSON APIs + admin auth
6. Frontend error handling
7. focused tests

## Files (expected)

- internal/shared/application/quota/*
- internal/shared/infrastructure/config/*, configs/config.yaml, .env.example
- internal/agent/api/handler/*, agent bootstrap
- internal/cloud/api/handler/*, cloud bootstrap, repository, migrations
- web terminal error path (minimal)

## Risks

- usage leak if missing Release → always defer
- Cloud vs Agent double-count not an issue: each process counts its own browser attaches
- admin whitelist misconfig leaves no admin → log + fail closed on admin routes

## Rollback

- set concurrent_attaches very high or disable by large default; or revert feature
