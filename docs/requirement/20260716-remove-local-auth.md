# 移除本地模式认证
最后修改时间: 2026-07-16 12:23:16

Review status: Accepted

## Background

Local 模式的浏览器与 Local Agent 处于同一既有 loopback 边界。浏览器直接使用 Local API；Cloud 账户凭据只用于需要 Agent 调用 Cloud 的操作。

## Goal

Local 模式浏览器直接调用 Local Agent API，浏览器仅管理 Cloud token。需要 Cloud 凭据的 Local API 通过请求头 `Authorization: Bearer <cloud_token>` 传递；Local Agent 调用 Cloud API 时，将该 token 原样作为上游 `Authorization: Bearer <cloud_token>` 转发。

## Non-goal

- 不改 loopback 监听约束；该约束由配置负责。
- 不调整 CORS Origin allowlist 策略。允许为既有客户端头（如 `X-Request-ID`）补充 CORS `Access-Control-Allow-Headers`，不改变 Origin 校验语义。
- 不改 Cloud 自身的认证、JWT、OAuth authorize、OAuth token exchange 或 Cloud API 契约。
- 不持久化 Cloud token 到 Agent 侧存储。

## User scenarios

- 本地页面加载工作区、会话、文件和 Git 数据时，直接请求 Local Agent（不带 Authorization）。
- 本地 OAuth callback 将 authorization code 直接提交给 Local Agent（body 仅含 code）。
- 本地页面连接 Cloud 或查询 Cloud 账户时，向 Local Agent 发送带 `Authorization: Bearer <cloud_token>` 的请求；Agent 以该 token 调用 Cloud。
- Local 模式退出时，重置本地 UI/session 状态。
- Local 视图需要展示 Cloud 账户信息时，浏览器仅向 `GET /api/cloud/auth/me` 显式附加 Cloud bearer；Agent 原样转发到 Cloud `GET /api/auth/me`。该链路只加载 Cloud identity，不产生 Local Auth。

## Acceptance

- Local Agent 的本地业务路由直接处理浏览器请求，不依赖本地 JWT / local bearer。
- 浏览器只保存 Cloud token。
- 普通 Local API client 不附加 `Authorization`；需要 Cloud 凭据的 Local 接口（`GET /api/cloud/auth/me`、`POST /api/cloud/connect`）由调用方显式附加 Cloud bearer。
- Cloud API client 仅附加 Cloud bearer。
- `POST /api/cloud/oauth/exchange` 直接处理本地前端请求，body 仅含 authorization code，不携带 Cloud token。
- Local Agent 的 Cloud client 使用 `<cloud.api_base_url>` 和 `Authorization: Bearer <cloud_token>`。
- Cloud 模式认证行为与 Cloud JWT 配置保持不变；portable Agent profile 不携带 Cloud JWT。

## Decisions

- 用户明确要求直接实施，因此本 Requirement 视为 Accepted。
- Local Agent 默认 loopback 安全边界和 CORS Origin 策略不在本次范围。
- Cloud 凭据在 Local Agent 入口使用 `Authorization` 头而非 JSON body，与 Agent → Cloud 上游头语义对齐。
- `CloudConnectReq` 不再承载 `cloud_token` 字段（可为空 message）；token 仅经 Authorization 传递。
- Cloud identity 初始化与 Local/Cloud transport 选择必须以 Cloud 为语义主体：Local 仅作为 `/cloud/auth/me` relay，不命名或建模为 Local Auth。

## Risk

- Local Agent API 的调用边界由既有 loopback 配置承担。
- 共享配置中的 JWT 继续服务 Cloud；Cloud JWT 配置与 Cloud 认证实现不在本次范围。