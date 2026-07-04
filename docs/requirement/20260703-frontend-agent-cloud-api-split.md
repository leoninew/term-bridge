# 前端 agent / cloud API 模块拆分 + DTO proto 生成

最后修改时间: 2026-07-04 08:48:55

Review status: Accepted

## Background

TermBridge-go 的前端（`web/`）是一个 Vue 3 + Vite 共享 Web 应用，同时对接两个后端：

- **agent**：本地终端服务，端口 9031，Vite 开发代理路径前缀 `/agent-api`
- **cloud**：云端服务，端口 9032，Vite 开发代理路径前缀 `/cloud-api`

`web/src/features/api/client.ts` 已经分别暴露了 `agentApiClient` 和 `cloudApiClient` 两个 axios 实例。但在拆分之前，所有 API 调用函数、DTO 类型都混在 `web/src/features/gateway/api.ts` 一个文件里，既不区分调用目标，也让消费者难以判断某个请求到底打向哪个服务。

随着 agent / cloud 后端内部拆分（`internal/agent`、`internal/cloud`、`internal/shared`）完成，前端职责边界也应当对齐。

目前前端 DTO 是手写在 `web/src/features/types.ts` 和 `web/src/protocol/terminal.ts` 里的，和 `proto/termbridge/` 下的 proto 定义存在冗余和不一致风险。后端已经有 proto → Go 代码生成流水线（`buf` + `protoc-gen-go`），前端也需要准备并纳入 proto → TypeScript 生成工具链，让 proto 成为 DTO 的单一真源。

## Goal

1. **API 函数拆分**：将 `web/src/features/gateway/api.ts`（monolith 混合调用）拆成职责清晰的独立模块：
   - `web/src/features/agent/api.ts`：只调用 `agentApiClient`
   - `web/src/features/cloud/api.ts`：只调用 `cloudApiClient`
   - 删除 `web/src/features/gateway/`
   - 所有消费者按调用目标从新模块导入

2. **DTO proto 生成**：前端 DTO 类型不再手写，改为从 `proto/termbridge/*.proto` 生成 TypeScript。proto 是前后端一致性的单一真源。

3. **准备前端 proto → TypeScript 工具链**：工具链就绪是本次任务的交付内容，不是可延期风险。实现需要完成插件选型、依赖/配置接入、生成脚本接入和本地验证。

4. **拆分 agent / cloud view 页面与路由**：最大可能复用组件，但不共用路由和页面；不保留兼容页面或兼容路由。页面层不再同时 import agent 与 cloud API，避免 `closeSession as closeAgentSession` / `closeSession as closeCloudSession` 这类别名和运行时分派混入 view。

## Non-goal

- 不修改认证/授权的行为逻辑（不移除 agent 本地账号登录、不调整 `/api/auth/me` 流程）。本次是结构 + 工具链变更，不是行为变更。
- 不改变路由、视图组件、store 的业务逻辑（仅改 import 路径和类型来源）。
- 不重新组织 `web/src/features/` 下其他目录（`sessions/`、`workspaces/`、`runtimeTarget.ts`、`api/` 等）。
- 不处理 `tunnel.proto` 的运行时二进制帧编解码；只在已有 HTTP DTO / JSON DTO 有对应 proto 时复用生成类型。
- 不重构 `protocol/terminal.ts` 中的 WebSocket 控制消息（`ClientControlMessage`、`ServerControlMessage`），它们不是 HTTP DTO。

## User scenarios

开发者视角（内部质量，无外部用户可见行为变化）：

1. 新增一个调用 agent 后端的接口：去 `features/agent/api.ts` 加函数，类型从生成的 `gen/...` 导入。
2. 新增一个调用 cloud 后端的接口：去 `features/cloud/api.ts` 加函数，类型从生成的 `gen/...` 导入。
3. 需要同时用到两边接口的页面（如 `SessionsView`）：分别从 `agent/api` 和 `cloud/api` import。
4. 后端改了 proto：运行前端 proto 生成命令后，前端自动获得新的 TS 类型，编译期就能发现字段不匹配。
5. 新增 DTO：改 proto 文件 → 生成 → 前端消费，不再两份手写定义。
6. 新开发者拉下项目后，不需要手工猜工具链；按项目脚本即可重新生成 TypeScript DTO。

## Acceptance

### API 模块拆分

- `web/src/features/gateway/` 目录已删除。
- `web/src/features/agent/api.ts` 只 import `agentApiClient`，不引入 `cloudApiClient`。
- `web/src/features/cloud/api.ts` 只 import `cloudApiClient`，不引入 `agentApiClient`。
- 所有 HTTP API 函数要么归入 `features/agent/api.ts`，要么归入 `features/cloud/api.ts`；不再存在新的混合 `gateway/api` 或第三处 HTTP API 聚合模块。
- 整个 `web/src/` 下 **零** `features/gateway` 残留引用。

### DTO proto 生成工具链

- 存在明确的前端 proto 生成配置（例如 `web/buf.gen.yaml` 或等效配置），能从项目根目录下的 `proto/termbridge/` 生成 TypeScript。
- 所需生成插件/依赖写入项目依赖或项目内工具链配置，不能只依赖开发者机器上的全局安装。
- 存在可复用脚本（例如 `cd web && yarn proto` / `yarn gen:proto`，或根 `task proto` 同时生成 Go + TS）用于重新生成前端 DTO。
- 生成的 TS 文件放在 `web/src/gen/`（或等效目录），生成路径稳定，且可被 `tsconfig` / Vite 正常解析。
- `buf generate` 或等效前端生成命令在当前环境中实际跑通，无报错。

### DTO 来源

- `web/src/features/types.ts` 中定义的、已有 proto 对应消息的类型（`DeviceSummary`、`CloudSessionSummary`、`AuthMeResp`、`TokenResp`、`CloudOAuthCallbackResp`、`CloudOAuthStartResp`、`CloudOAuthAuthorizeResp`）删掉，改为引用生成的 TS。
- `web/src/protocol/terminal.ts` 中与 HTTP DTO 重复的类型，如果 proto 已覆盖，也改为引用生成类型。
- 前端仍有少量 proto 未覆盖的类型（`Capabilities`、`UserInfo`、`GoogleAuthURLResp`、`ListResp`、`PaginatedResp`）可作为局部补充类型保留，但需要注明原因和后续迁移条件。

### 验证

- `cd web && yarn vitest run` 全部测试通过。
- `cd web && yarn vue-tsc --noEmit` 类型检查无错误。
- 前端 proto 生成命令能成功生成 TS 且无报错。
- 行为无回归：agent 发起的请求仍走 `agentApiClient`，cloud 发起的请求仍走 `cloudApiClient`。

## Open questions

1. **proto → TS 生成工具选型由实现阶段验证后确定**。候选方案：
   - `buf.build/bufbuild/es`（`protoc-gen-es`，ES Modules，优先尝试）
   - `buf.build/community/stephenh-ts-proto`（`ts-proto`，TS 类型导出能力强）
   - 本地 npm 插件方案（如 `@protobuf-ts/plugin`），用于远程 buf plugin 不适合当前环境时的落地方案

   该问题不是是否要做，而是选择哪条可维护、可在本项目当前环境跑通的路径。实现阶段需要验证并落定，不允许把工具链准备延期到后续任务。

2. **proto 覆盖度缺口**：当前 proto 未覆盖：
   - `Capabilities` 消息（agent 和 cloud 各自有 Go 定义，proto 没有）
   - `User` 消息缺少 `provider` 字段（Go `UserView` 有该字段）
   - `AuthMeResp` 缺少 `capabilities` 字段（Go `AuthMeResp` 有该字段）
   - `GoogleAuthURLResp`（前端 google auth URL 响应，cloud 存在 Go 定义但 proto 未映射成 HTTP 消息）
   - `ListResp` / `PaginatedResp`（通用分页包装，proto 里用的是 `ListDevicesResp` 这类具体消息），runtime proto 用具体 `ListWorkspacesResp` 等形式

   本次目标是先使用现有 proto 作为 DTO 生成来源；缺口类型可以暂时保留手写，但必须明确标注原因。是否补 proto 是后续独立需求，不应阻塞本次工具链准备和已有 proto DTO 迁移。

## Decisions

### API 函数归属

- `authMe`（打向 agent 的 `/api/auth/me`）→ `features/agent/api.ts`。
- `authMeViaCloud`（打向 cloud 的 `/api/auth/me`）→ `features/cloud/api.ts`。
- `cloudOAuthStartURL` / `cloudOAuthStart` / `cloudOAuthCallback`（调用 agent 后端 `/api/cloud-oauth/start`、`/api/cloud-oauth/callback`）→ `features/agent/api.ts`。
- `cloudOAuthAuthorize`（调用 cloud 后端 `/api/cloud-oauth/authorize`）→ `features/cloud/api.ts`。
- `authLogout` 在 agent 和 cloud 模块中各保留一份（不同 axios 实例，打不同后端），导入方按需选择。
- 其他 HTTP API 函数按实际 axios client 归属：使用 `agentApiClient` 就进 agent 模块，使用 `cloudApiClient` 就进 cloud 模块；不允许再建立混合 gateway API 模块。

### DTO 来源

- 有 proto 对应的消息（`DeviceSummary`、`CloudSessionSummary`、`AuthMeResp`、`TokenResp`、`CloudOAuthCallbackResp`、`CloudOAuthStartResp`、`CloudOAuthAuthorizeResp`）→ proto 生成。
- 没有 proto 对应的消息（`Capabilities`、`UserInfo`、`GoogleAuthURLResp`）→ 暂时保留手写，并在类型注释里标注 "proto 未覆盖，待补充 proto 定义后迁移"。
- 通用包装 `ListResp<T>` → runtime proto 里已经用具体消息（`ListDevicesResp`、`ListWorkspacesResp`）表达，不单独生成泛型包装；前端可以手写一个轻量 `ListResp<T>` 工具类型组合，或优先使用具体生成响应类型。

### Proto 工具链

- 本次实现必须准备可运行的前端 proto → TS 工具链，包含依赖、配置、脚本和生成物。
- 优先选择与 Vite / TS ESM 兼容、生成物可读、维护成本低的方案。
- 如果优先方案在当前环境无法跑通，必须在同一任务内切换到可跑通方案，而不是把工具链准备列为风险延期。

### Proto 缺口处理

**不在本次需求中修改 proto 文件**。proto 是前后端共用契约，改动要走 proto review。本次先把生成流水线搭起来，用 proto 当前能覆盖的部分做迁移；缺口类型先保留手写并打 TODO，后续单独开需求补 proto。

## Risk

- **proto 覆盖度不够**：强切生成会导致新类型无处安放。已通过"缺口保留手写 + 打 TODO"策略规避；但缺口本身仍表示后续需要补齐 proto 契约。
- **消费者漏改**：`grep -r "features/gateway"` 必须为零，否则运行时会模块找不到。已用该命令作为验证手段。
- **类型兼容**：生成的 TS 类型中的字段名、可选性、Timestamp 表示、JSON 序列化约定必须和当前 HTTP JSON 响应一致，否则会引入隐蔽错误。需要跑完整 vitest 和 vue-tsc 兜住。
- **构建性能**：生成文件体量如果很大，可能拖慢 vue-tsc / vite 冷启。初步观察 proto 数量有限（4 个文件，约 30 条 message），暂时可控。
