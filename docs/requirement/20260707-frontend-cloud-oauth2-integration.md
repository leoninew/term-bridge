# 前端接入 Cloud OAuth2
最后修改时间: 2026-07-07 17:47:32

Review status: Accepted

## Background

Agent Dashboard 的“登录 Cloud 账号”应是一个前端 OAuth2 接入问题：Agent 前端作为 Cloud OAuth2 的接入方，跳转到 Cloud OAuth2 authorization endpoint；Cloud 作为 OAuth2 Authorization Server，内部自行处理 Cloud 登录和 Google provider。此前实现方向误把问题扩展为 Agent backend OAuth start/callback、PKCE、cookie session、额外 proto 等后端重写，这是错误范围。

## Goal

- 按前端 OAuth2 接入方式重做 Agent 登录 Cloud 账号流程。
- Agent Dashboard 不跳本机 `/login?redirect=/agent/dashboard`。
- Agent Dashboard 不直接调用 Google provider API，不认识 Google provider。
- Agent 前端只面向 Cloud OAuth2 authorization endpoint。
- OAuth2 callback 回到 Agent 前端后，前端校验 state 并把 authorization code 交给 Agent 后端；Agent 后端使用 `golang.org/x/oauth2` 和本地 `agent.oauth.*` confidential client 配置换取 Cloud token，再上报当前设备。
- 清理此前错误方向产生的无效改动，包括不必要的 proto / generated contract / backend OAuth start-callback 残留。

## Non-goal

- 不新增 Agent backend `/agent-api/cloud-oauth/start`。
- 不新增 Agent backend `/agent-api/cloud-oauth/callback`。
- 不引入 PKCE，除非后续明确要求或现有 Cloud OAuth2 实现已经要求。
- 不新增 Cloud browser cookie session 作为本任务的前置设计。
- 不把 Google provider 暴露给 Agent Dashboard。
- 不把 device id/name/public_key 放入 OAuth2 authorize/code/state/provider callback。
- 不持久化 Cloud access token 到 Agent 本地 device state。
- 不恢复 `GateURL` / `gate_url` 语义。
- 不执行任何 git 写操作。

## User scenarios

1. 用户在 Agent Dashboard 点击“登录 Cloud 账号”。
2. Agent 前端构造 Cloud OAuth2 authorization URL 并让浏览器跳转到 Cloud。
3. Cloud 负责用户登录、Google provider 以及 OAuth2 授权码流程。
4. OAuth2 完成后，Agent 前端 callback 获得 authorization code。
5. Agent 前端调用 Agent 本地设备连接 API，把 code 交给 Agent 后端。
6. Agent 后端使用 `golang.org/x/oauth2` 调 Cloud token endpoint 换取 Cloud token。
7. Agent 后端使用 Cloud token 调 Cloud `/cloud-api/devices/current` 上报当前设备。
8. Agent Dashboard 展示 Cloud 连接摘要。

## Acceptance

1. Agent Dashboard Cloud 登录入口由前端 OAuth2 接入逻辑驱动，不再跳本机 `/login?redirect=/agent/dashboard`。
2. Agent Dashboard 不直接调用 `/cloud-api/auth/google`、`authGoogleUrl()`、`startGoogleLogin()` 等 Google provider 入口。
3. 不新增或保留 Agent backend OAuth start/callback 路由作为本任务方案。
4. 前端 OAuth2 相关实现不通过新增 proto DTO 承载标准 OAuth2 redirect/token 协议。
5. 已有错误新增的 proto / generated code 如无业务必要应清理。
6. Agent 后端设备上报仍发生在 token 获取之后；前端只传 authorization code，Agent 后端使用 Cloud token 调用 Cloud device API。
7. OAuth2 state/code/authorize/provider callback 不携带 device id/name/public_key。
8. Cloud token 不写入 Agent 本地 device state。
9. 使用 `public_url` / `PublicUrl` 语义，不恢复 `gate_url` / `GateURL`。
10. 实现过程中不执行 git 写操作，只使用文件读写和允许的只读检查。

## Open questions

- 需要读取当前 Cloud OAuth2 既有实现，确认 Cloud `/oauth2/authorize` 与 token endpoint 的现状、参数和回调地址约定。
- 需要确认现有前端是否已有可复用的 OAuth2 client helper；如果没有，应实现最小前端 helper，而不是引入后端 flow controller。

## Decisions

- 任务方向明确为“前端 OAuth2 接入 Cloud”，不是“Agent backend 作为 OAuth2 flow controller”。
- 不再继续此前 `/agent-api/cloud-oauth/start`、`/agent-api/cloud-oauth/callback`、PKCE、cookie session 的实现假设。
- 本任务使用文件读写完成，禁止 git 写操作。

## Risk

- 当前工作区已有较多历史变更，实施前需要通过文件读取定位并区分：哪些是正确的 `public_url` / 前端归属改动，哪些是错误 OAuth2 方向残留。
- 如果 Cloud OAuth2 authorization/token endpoint 本身已被删除或破坏，可能需要先恢复 Cloud 作为 Authorization Server 的既有协议能力，但仍不应把 Agent backend 改成 flow controller。
- 如果前端 token exchange 涉及 client secret，则说明现有 Cloud OAuth2 client 类型与“前端接认证库”不匹配，需要暴露为设计风险，而不是擅自改为 PKCE 或后端 callback。
