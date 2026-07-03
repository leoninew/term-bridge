# 本地开发与 agent/cloud 部署拓扑需求

最后修改时间: 2026-07-03 19:27:13

Review status: Accepted

## Background

当前 agent/cloud 已拆分为两个业务入口：

```text
termbridge agent
termbridge cloud
```

本地开发需要同时联调 agent 与 cloud，但最终交付场景又不同：agent 在用户本机运行，cloud 在云端部署运行。需要明确本地开发端口、前端代理路由、最终运行拓扑，以及哪些配置只属于开发联调，避免把本地 Vite proxy 路径误写成产品部署默认语义。

已讨论的本地开发期望端口与路由：

```text
web 本地开发端口:   9030
agent 本地开发端口: 9031，前端路由 /agent-api
cloud 本地开发端口: 9032，前端路由 /cloud-api
```

同时需要考虑最终真实运行：

```text
agent: 用户本机运行
cloud: 云端部署运行
```

## Goal

1. 明确本地开发联调拓扑：一个 Vite/Web dev server 同时代理 agent 与 cloud 两个后端。
2. 明确最终运行拓扑：agent 本机运行、cloud 云端运行，两者不是共用同一个 Web 进程。
3. 将 `/agent-api` 与 `/cloud-api` 定义为本地开发代理前缀，而不是最终产品 API 路径。
4. 让配置默认值、开发配置、文档和测试能区分：
   - 本地开发联调配置。
   - agent 本机运行配置。
   - cloud 云端部署配置。
5. 避免在最终部署或打包场景中误依赖 Vite dev proxy。

## Non-goal

1. 本需求不实现新的 cloud 生产部署脚本、Kubernetes、systemd 或反向代理配置。
2. 本需求不要求把 agent 与 cloud 拆成两个 repository 或两个前端项目。
3. 本需求不要求重构所有前端 API 调用，只要求明确本地联调路径和最终部署语义；如果现有前端单一 API base 无法同时表达 agent/cloud，后续实现中可在最小范围内调整。
4. 本需求不处理历史 docs 中已过时的端口或 `server.*` 记录，除非这些文档仍作为当前开发说明使用。
5. 本需求不要求一次性完成完整 E2E 自动化验证。

## User scenarios

### Scenario 1: 开发者本地同时联调 agent 与 cloud

开发者启动三个进程：

```text
web:   http://localhost:9030
agent: http://127.0.0.1:9031
cloud: http://127.0.0.1:9032
```

浏览器只打开一个 Web dev server：

```text
http://localhost:9030
```

前端请求通过 Vite dev proxy 分流：

```text
/agent-api/* -> agent backend
/cloud-api/* -> cloud backend
```

### Scenario 2: 用户本机运行 agent

用户本机运行 `termbridge agent`，agent 提供本机 Web/API/runtime 能力。此时不依赖 Vite dev server，前端与 agent API 推荐同源：

```text
agent Web/API: http://localhost:<agent-port>
API path:      /api/*
api_base_url:  ""
```

### Scenario 3: 用户访问云端 cloud

cloud 部署在云端并提供 cloud Web/API。默认推荐 cloud Web 与 cloud API 同源：

```text
cloud Web/API: https://cloud.example.com
API path:      /api/*
api_base_url:  ""
```

如果后续云端 Web 与 API 分域部署，可以通过 `cloud.api_base_url` 配置绝对 API origin。

### Scenario 4: agent 连接 cloud 授权回跳

本地开发时，浏览器回跳应该回到 Web dev server：

```text
http://localhost:9030/cloud/oauth/callback
```

最终本机 agent 运行时，浏览器回跳应该回到 agent 的 public URL：

```text
<agent.public_url>/cloud/oauth/callback
```

## Acceptance

1. 本地开发默认拓扑在当前开发文档或配置示例中清晰表达：

   ```text
   web:   localhost:9030
   agent: 127.0.0.1:9031
   cloud: 127.0.0.1:9032
   ```

2. Vite dev server 监听 `localhost:9030`。
3. Vite dev proxy 支持：

   ```text
   /agent-api -> http://127.0.0.1:9031
   /cloud-api -> http://127.0.0.1:9032
   ```

4. 本地开发时，前端能通过 `/agent-api` 调用 agent API，通过 `/cloud-api` 调用 cloud API。
5. `/agent-api` 与 `/cloud-api` 不被描述为最终产品 API path，只作为本地开发代理前缀。
6. agent 本机运行的最终/打包语义保持同源 `/api`，不要求用户运行 Vite。
7. cloud 云端部署的默认语义保持同源 `/api`；只有分域部署时才使用绝对 `cloud.api_base_url`。
8. `configs/config.yaml`、`.env.example`、README、Taskfile 或 package 脚本中与端口/API base 相关的说明不应互相矛盾。
9. 若前端当前只有单一 `apiBaseUrl`，实现阶段必须明确哪些请求默认走 agent、哪些请求走 cloud；不能通过隐式全局切换导致同一页面里 agent/cloud 请求混淆。
10. 相关配置加载测试需要覆盖 agent/cloud 端口与 `expose_errors` 的 role-specific 配置。

## Open questions

1. `configs/config.yaml` 是否应保持最终运行 baseline（同源 `/api`），并新增 `configs/config.local.yaml` 表达本地联调端口与 `/agent-api`、`/cloud-api`？
2. 本地 Vite 开发时，前端默认 API base 是否设为 `/agent-api`，同时为 cloud API 新增单独 client/base？
3. cloud OAuth 的本地开发回跳是否统一使用 `http://localhost:9030/cloud/oauth/callback`？
4. 打包版 agent 默认端口是否继续沿用 package `.env.local` 的独立端口，还是同步调整到 9031？

## Decisions

1. 本地开发默认只启动一个 Web/Vite dev server，不启动两个 Web dev server。
2. `/agent-api` 和 `/cloud-api` 是本地开发代理前缀，不是最终部署 API path。
3. 最终真实运行中，agent 和 cloud 是两个运行实例；它们可以复用同一套前端代码/构建产物，但不共享同一个 Web 进程。
4. agent 本机运行与 cloud 同源部署时，`api_base_url` 应允许为空并表示同源 `/api`。

## Risk

1. 如果直接把 `/agent-api`、`/cloud-api` 写入通用 baseline 配置，可能污染最终部署和打包语义。
2. 如果前端继续只有一个全局 `apiBaseUrl`，同时联调 agent/cloud 时容易把 cloud 请求错误打到 agent，或反之。
3. OAuth 回跳涉及 browser route、agent local API 和 cloud API 三方协作，端口和 base URL 不一致时容易出现回跳地址注册不匹配。
4. 当前工作区已有大量 agent/cloud 拆分改动；实现阶段需要避免把本需求与无关历史改动混在一起解释。

## User review notes

- 用户确认本地开发端口期望：web 9030、agent 9031、cloud 9032。
- 用户确认本地开发前端代理路由期望：agent 使用 `/agent-api`，cloud 使用 `/cloud-api`。
- 用户提醒需要同时考虑最终运行场景：agent 本地运行，cloud 云端部署运行。

## 需要用户关注的未决事项

1. 是否同意新增 `configs/config.local.yaml` 或等价开发配置来承载本地联调端口和 `/agent-api`、`/cloud-api`，而让 `configs/config.yaml` 更偏最终运行 baseline？
2. 是否同意前端引入 agent/cloud 两个 API base 的区分，而不是继续只用一个全局 `apiBaseUrl`？
3. 打包版 agent 的默认端口是否应使用 9031，还是保持已有 package 端口策略？
