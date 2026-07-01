# API Inventory Verification
最后修改时间: 2026-07-01 17:26:03

## Review status

Accepted

## Flow mode / Stage

轻量模式 / light；验证 / Verification。用户已明确要求进入 Verification，本文件对照 `docs/requirement/20260701-api-inventory.md` 和 `docs/spec/20260701-api-inventory.md` 核对实现结果。

## Requirement alignment

需求阶段原目标是摸底后端 API，并标记未遵从显式 `Req` / `Resp`、数组、map/dict、文本、空响应、WebSocket frame 等形态，同时补充前端说明。

本次实现超出最初“只摸底”的 requirement，但符合后续用户在 Spec / Implementation 阶段追加的治理要求：

- 列表类型统一为 `{ items: []T }`。
- 有响应内容使用 `XxxxResp`，无响应内容使用 `204 No Content`。
- 保留 `PaginatedResp[T]` 作为通用分页 response。
- 错误响应使用泛型类。
- DTO 抽取到独立 `dto.go`，不要和业务逻辑混在一起。

结论：实现与后续用户确认的治理方向一致；相比最初 requirement，从“摸底文档”推进到了“契约规范实现”。

## Spec alignment

对照 `docs/spec/20260701-api-inventory.md`：

- P0 HTTP JSON API checklist 已落地：auth、cloud OAuth、current device、health 等匿名 request / map response 已改为命名 DTO 或 `204 No Content`。
- P1 relay JSON API checklist 已落地：gateway handler 显式解码 terminal DTO，agent runtime relay 列表响应改为 `{ items: ... }`。
- P2 devices checklist 已落地：`GET /api/devices` 改为 `ListDevicesResp`；`DELETE /api/devices/{deviceId}` 保持 `204 No Content`。
- P3 WebSocket protocol 保持协议形态，不强行改为 HTTP Req/Resp；前端仍只有 terminal WebSocket 使用原生 `WebSocket`。
- P4 common contract infrastructure 已落地：后端 `APIErrorResp[T]`，前端 `ApiErrorResp<TDetails = unknown>`，不新增 `OkResp`。

结论：Spec checklist 中本轮要求实现的 HTTP/relay/common contract 项已完成；WebSocket 项按 Spec 决策保持原协议形态。

## Plan alignment

本功能未创建独立 Plan 文档；按轻量模式 / light 和用户直接要求进入 Implementation 的指令执行。验证按 Requirement + Spec + Implementation 用户补充规则核对。

## Actual diff summary

本次实现涉及以下实际改动类别：

1. 后端 gateway DTO 抽取与命名
   - 新增 `internal/transport/http/gatewayapi/dto.go`。
   - `server.go` 中匿名 request struct 和 map response 改为显式 DTO。
   - `errors.go` 使用 `APIErrorResp[any]` 写错误响应。

2. Terminal runtime DTO 抽取
   - 新增 `internal/application/terminal/dto.go`。
   - `registry.go` 移除 DTO 定义，保留 runtime 业务逻辑。
   - `UpdateSessionReq.UnmarshalJSON` 作为 DTO 自身解析逻辑保留在 DTO 文件。

3. Relay contract 规范
   - `internal/application/agent/client.go` 中 workspace/session list/order relay 返回 `{ items: ... }` 包装。
   - gateway relay 参数改用 `terminalapp.*Req` 类型，不再用匿名 struct 或 `map[string]any` 拼装核心参数。
   - workspace delete relay 成功响应改为 `204 No Content`。

4. 前端同步
   - `web/src/protocol/terminal.ts` 新增 `ListResp<T>`、`PaginatedResp<T>`，并泛型化 `ApiErrorResp<TDetails = unknown>`。
   - `web/src/features/gateway/api.ts`、`workspaces/api.ts`、`sessions/api.ts` 读取 `{ items }`。
   - `web/src/features/gateway/api.test.ts` 更新列表响应 mock。

5. 测试同步
   - gateway 相关 Go 测试更新为 `ListDevicesResp` / `{ items }` 和 `204 No Content` 预期。
   - relay fake response 更新为 `{ items: [...] }`。

## Expected vs actual changed files

### Expected

- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/errors.go`
- `internal/transport/http/gatewayapi/dto.go`
- `internal/application/terminal/registry.go`
- `internal/application/terminal/dto.go`
- `internal/application/agent/client.go`
- `web/src/protocol/terminal.ts`
- `web/src/features/gateway/api.ts`
- `web/src/features/workspaces/api.ts`
- `web/src/features/sessions/api.ts`
- gateway / frontend 相关测试
- `docs/spec/20260701-api-inventory.md`
- `docs/verification/20260701-api-inventory.md`

### Actual tracked diff reported by `git diff --name-only`

- `internal/application/agent/client.go`
- `internal/application/terminal/registry.go`
- `internal/transport/http/gatewayapi/cloud_binding_test.go`
- `internal/transport/http/gatewayapi/errors.go`
- `internal/transport/http/gatewayapi/relay_test.go`
- `internal/transport/http/gatewayapi/server.go`
- `internal/transport/http/gatewayapi/server_test.go`
- `internal/transport/http/gatewayapi/tunnel_test.go`
- `web/src/features/gateway/api.test.ts`
- `web/src/features/gateway/api.ts`
- `web/src/features/sessions/api.ts`
- `web/src/features/workspaces/api.ts`
- `web/src/protocol/terminal.ts`

### Additional untracked files from `git status --short`

- `docs/requirement/20260701-api-inventory.md`
- `docs/spec/20260701-api-inventory.md`
- `docs/verification/20260701-api-inventory.md`
- `internal/application/terminal/dto.go`
- `internal/transport/http/gatewayapi/dto.go`
- `docs/analyze/20260701-core-abstractions-and-flows.md`
- `docs/analyze/20260701-grounded-uml-sequence-flow-diagrams.md`

说明：`docs/analyze/*` 不属于本次 API inventory 契约规范的核心交付范围，但当前工作区存在这些未跟踪文件，提交前建议用户自行确认是否纳入同一变更。

## Acceptance criteria checklist

- [x] 后端 HTTP JSON 有内容响应使用命名 `XxxxResp`。
- [x] 无内容成功响应使用 `204 No Content`。
- [x] 列表响应使用 `{ items: []T }`。
- [x] `PaginatedResp[T]` 保留为通用分页 response。
- [x] 错误响应后端使用泛型 `APIErrorResp[T]`，前端使用泛型 `ApiErrorResp<TDetails = unknown>`。
- [x] DTO 定义抽出到 `dto.go`，业务逻辑文件不再承载主要 DTO 定义。
- [x] 前端 API 封装同步读取 `{ items }`。
- [x] 前端未发现业务 HTTP 调用绕过 `apiClient` 使用 `fetch` 或原始 axios。
- [x] WebSocket API 保持 protocol 形态。
- [x] 相关 Go targeted tests 通过。
- [x] 前端 typecheck / test / lint 通过。
- [ ] 全量 `go test ./...` 通过。
- [ ] 前端 `format:check` 通过。

## Command results

### Passed

- `go test ./internal/transport/http/gatewayapi ./internal/application/agent ./internal/application/terminal`
  - 结果：通过。

- `npm --prefix web run typecheck`
  - 结果：通过。

- `npm --prefix web run test`
  - 初次结果：失败，`web/src/features/gateway/api.test.ts` 仍 mock 顶层数组 `data: []`。
  - 修复后重跑结果：通过，10 个 test files / 44 个 tests 全部通过。

- `npm --prefix web run lint`
  - 结果：通过。

- 前端 raw HTTP client 搜索：`rg -n "\bfetch\s*\(|from ['\"]axios['\"]|import\s+axios|axios\." "web/src"`
  - 结果：无匹配；业务 HTTP API 未发现绕过 `apiClient` 的 `fetch` 或原始 axios 用法。

### Failed / warnings

- `go test ./...`
  - 结果：失败。
  - 失败包：`termbridge-go/internal/app`。
  - 失败测试：
    - `TestRunServeStartsUnifiedBackendAndAgentFromConfig`：`condition was not met within 1s`。
    - `TestRunServeStartsCloudConnectorAfterOAuthCompletion`：`condition was not met within 1s`。
  - 观察：本次 targeted changed packages 通过；全量失败集中在 `internal/app` 的 1s 等待条件，需进一步判断是否为现有时序/环境问题或本轮间接影响。

- `npm --prefix web run format:check`
  - 结果：失败。
  - Prettier 报告：`src/views/DashboardView.vue` 存在格式问题。
  - 观察：该文件不在本次 API inventory diff 中，当前验证未自动格式化或修改该无关文件。

- `npm --prefix web run type-check`
  - 结果：失败。
  - 原因：项目没有 `type-check` 脚本；实际脚本名为 `typecheck`。
  - 后续：已改用 `npm --prefix web run typecheck` 并通过。

## Scope deviations

1. 最初 requirement 是静态摸底，不修改产品代码；后续用户明确进入 Spec / Implementation 并追加契约规范规则，因此本次实际包含代码改造。
2. 列表响应从顶层数组改为 `{ items: []T }` 是前端可见的响应形态变化，已同步前端 API 封装和测试。
3. 多个成功响应从 `200/202 + { ok: true }` 改为 `204 No Content`，这是状态码和 body 变化，已同步相关测试；前端原本多为 `Promise<void>`，兼容该方向。
4. 未修复 `DashboardView.vue` 的 Prettier warning，因为该文件不属于本次 API 契约变更范围。
5. 未修改 `internal/app` 的全量测试超时失败，因为 targeted API 相关测试均通过，且该失败需要单独定位时序/环境原因。

## Risks

1. `{ items: []T }` 是破坏性响应形态调整；若还有未覆盖的外部调用方，需要同步迁移。
2. offline cache 存储的是 relay 返回的 raw JSON；本轮 agent runtime 已返回 `{ items }`，但历史运行中的旧 cache 如果残留顶层数组，前端新代码将无法按 `items` 读取。当前 cache 是内存态，进程重启后消失。
3. `go test ./...` 未全量通过，尽管失败集中在 `internal/app` 时序等待测试，仍建议后续单独处理或延长/稳定相关测试。
4. `format:check` 因无关文件失败，若 CI 强制格式检查，需要先处理或拆分该文件的格式问题。
5. 当前工作区存在不属于本次核心范围的未跟踪 `docs/analyze/*` 文件，提交前应确认归属。

## Incomplete items

- 全量 `go test ./...` 尚未通过：`internal/app` 两个测试 1s condition timeout。
- `npm --prefix web run format:check` 尚未通过：`web/src/views/DashboardView.vue` 格式问题。
- 未对 live backend 逐个 HTTP endpoint 发起运行时请求验证；本次验证以静态 diff、targeted tests、frontend tests/typecheck/lint 为主。

## Conclusion

本次 API inventory 契约规范实现与用户在 Spec / Implementation 阶段确认的规则整体对齐。核心后端 gateway/agent/terminal DTO、relay list wrapper、前端 API 封装和相关 targeted tests 已通过验证。

交付状态：有条件通过。核心相关检查通过，但全量 Go 测试和前端 format check 仍存在非核心或待进一步定位的问题，合并或提交前建议先决定是否单独修复这些剩余项。