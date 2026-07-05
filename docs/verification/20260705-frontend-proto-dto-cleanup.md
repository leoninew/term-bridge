# 前端 Proto DTO 清理验证 / Frontend Proto DTO Cleanup Verification

最后修改时间: 2026-07-05 23:05:59

Review status: Accepted / 接受

## 当前阶段

轻量模式 / light，验证 / Verification。

## Requirement alignment / 需求对齐

依据 `docs/requirement/20260705-frontend-proto-dto-cleanup.md` 验证：

- R1 Cloud HTTP DTO must come from generated proto：已对齐。
  - `proto/termbridge/cloud/v1/cloud.proto` 补齐前端 cloud API 使用的 request / response body。
  - 前端 cloud API、gateway、dashboard/session 组件的 DTO 类型改为直接从 `web/src/gen/proto/termbridge/cloud/v1/cloud` import。
  - `web/src/features/types.ts` 不再承载 cloud API DTO。
- R2 Runtime HTTP DTO must come from generated proto：已对齐。
  - `proto/termbridge/runtime/v1/runtime.proto` 中 body-only request 已拆出，不再把 URL path 参数混入 `RerunSessionReq`、`UpdateSessionReq`、`UpdateSessionOrderReq`。
  - tunnel 需要 path 参数时使用 wrapper message：`RerunWorkspaceSessionReq`、`UpdateWorkspaceSessionReq`、`WorkspaceSessionOrderReq`、`ReadWorkspaceSessionHistoryReq`。
  - 前端 runtime API DTO 类型改为直接从 `web/src/gen/proto/termbridge/runtime/v1/runtime` import。
- R3 Terminal WebSocket control protocol must be proto-defined：已对齐。
  - 新增 `proto/termbridge/terminal/v1/terminal.proto`。
  - 生成 Go / TypeScript terminal control message。
  - wire format 仍保持 JSON over WebSocket + string `type`，未引入 gRPC 或 binary protobuf。
- R4 Enums and time fields must not cause DTO wrappers：已对齐。
  - 未新增 `Omit<Proto...>` wrapper。
  - timestamp / state 字段按 generated proto type 使用，业务处理留在业务逻辑。
- R5 Error details remain flexible：已对齐。
  - `web/src/features/api/client.ts` 改用 generated `common.ErrorResp`。
  - `details` 保持业务处自行收窄。
- R6 Backend hand-written JSON structs are migration debt：部分符合本轮边界。
  - 本轮未完整迁移后端 handler DTO，这是 requirement 明确的 non-goal。
  - 与本轮 terminal control / tunnel frame 相关的 Go 类型已同步到 generated proto。

## Spec / Plan alignment

不适用。该任务使用轻量模式 / light，只有 Requirement → Implementation → Verification。

## Actual diff summary / 实际变更摘要

本轮 proto DTO 清理相关主线：

1. Proto contract
   - 修改 `proto/termbridge/cloud/v1/cloud.proto`，补齐 cloud API request / response body message。
   - 修改 `proto/termbridge/runtime/v1/runtime.proto`，区分 HTTP body-only request 与 tunnel wrapper request。
   - 修改 `proto/termbridge/tunnel/v1/tunnel.proto`，让 tunnel frame 使用 wrapper request。
   - 新增 `proto/termbridge/terminal/v1/terminal.proto`。
2. Generated outputs
   - 重新生成 Go proto：`internal/shared/dto/proto/termbridge/...`。
   - 重新生成 frontend proto：`web/src/gen/proto/...`。
3. Frontend DTO cleanup
   - API clients 和 UI/store/composable 消费点改为直接 import `web/src/gen/proto/...` generated DTO。
   - 移除 DTO re-export 中转，避免 `features/types.ts`、`features/sessions/runtime.ts`、`protocol/terminal.ts` 再 import/export generated DTO。
   - `web/src/protocol/terminal.ts` 仅保留 terminal helper、JSON encode/decode 与校验逻辑。
4. Terminal control
   - Go terminal protocol 改用 generated terminal proto message。
   - generated Go proto message 使用 pointer 传递，避免 copylocks。
   - 前端 terminal control message 构造补齐 generated proto 必填字段。
5. Runtime relay / tunnel
   - agent/cloud runtime endpoint 和 relay 处理同步到新的 wrapper proto message。

## Expected vs actual changed files / 预期与实际改动对比

### 与本需求匹配的改动

- `proto/termbridge/cloud/v1/cloud.proto`
- `proto/termbridge/runtime/v1/runtime.proto`
- `proto/termbridge/tunnel/v1/tunnel.proto`
- `proto/termbridge/terminal/v1/terminal.proto`
- `internal/shared/dto/proto/termbridge/...`
- `internal/shared/dto/protocol/terminal/...`
- `internal/agent/api/handler/runtime_endpoint.go`
- `internal/agent/api/handler/server.go`
- `internal/agent/application/task/terminal/...`
- `internal/agent/application/user/cloud_client.go`
- `internal/cloud/api/handler/runtime_endpoint.go`
- `internal/cloud/api/handler/server.go`
- `internal/cloud/api/handler/route.go`
- `web/src/gen/proto/...`
- `web/src/features/agent/api.ts`
- `web/src/features/cloud/api.ts`
- `web/src/features/api/client.ts`
- `web/src/features/sessions/runtime.ts`
- `web/src/features/sessions/useTerminalSocket.ts`
- `web/src/protocol/terminal.ts`
- frontend components / stores / composables that previously consumed handwritten DTOs.

### 工作区存在但不归属于本需求的改动

当前 worktree 同时存在其它任务改动，例如 OAuth/device binding/config/router/i18n 相关文件。验证时只把它们作为范围风险记录，不把这些文件视为本需求交付内容。

## Acceptance checklist / 验收清单

- [x] `proto/termbridge/cloud/v1/cloud.proto` 覆盖前端 cloud API 使用的请求/响应体。
- [x] `proto/termbridge/runtime/v1/runtime.proto` 的 request message 表达 HTTP body contract，不混入 path 参数。
- [x] Terminal WebSocket control message 被 proto 定义覆盖，并生成 Go / TypeScript 类型。
- [x] `task proto` 执行成功。
- [x] 前端 API client 不再引用 `web/src/features/types.ts` 中的手写 API DTO。
- [x] `web/src/protocol/terminal.ts` 不再定义 runtime HTTP API DTO，只保留 terminal helper / WebSocket 编解码校验逻辑。
- [x] 前端 API client 泛型直接使用 `web/src/gen/proto/...` generated types。
- [x] 未发现 `Omit<Proto...>` 用于构造 API DTO wrapper。
- [x] 未新增 adapter / wrapper / view-model 转换层来掩盖 proto 与实际 HTTP contract 的差异。
- [x] 相关 frontend typecheck / lint / unit tests / Go tests 通过。
- [x] 项目完整 `task check` 通过。

## Command results / 命令结果

已执行并通过：

- `task proto`
  - 通过；buf dep update 提示无 configured dependencies，是当前配置下的 warning。
- `yarn --cwd web typecheck`
  - 通过。
- `yarn --cwd web test`
  - 通过；11 个 test files，55 个 tests。
- `yarn --cwd web lint`
  - 通过。
- `yarn --cwd web prettier --check <本需求相关 touched frontend files>`
  - 通过。
- `go test ./cmd/... ./internal/...`
  - 通过。
- `gofmt -l cmd internal`
  - 通过；无输出。
- DTO cleanup 搜索检查
  - `features/types` DTO import：无匹配。
  - DTO re-export `export type { ... }`：无匹配。
  - `Omit<`：无匹配。

追加验证：

- `task check`
  - 通过。
  - 覆盖 `web` typecheck、format、lint，以及 `golangci-lint fmt` / `golangci-lint run`。
  - `golangci-lint run` 结果：0 issues。

## Scope deviation / 范围偏差

- 本轮实现没有处理 transaction/OAuth/device binding 的业务逻辑。
- 当前工作区包含其它任务改动，因此完整工作区 diff 不能直接视为本需求 diff。
- 为了让 generated proto 类型在 TypeScript 下成立，terminal client control message 构造补齐了 proto3 scalar 默认字段（例如 `cols: 0`、`rows: 0`、`nonce: ''`）。wire format 仍是 JSON，业务语义不变。
- Go terminal proto message 改为 pointer 传递是 generated proto 类型的必要约束，避免 copylocks，不改变协议语义。

## Risks / 风险

1. 后端 handler 的其它手写 JSON struct 仍是后续迁移债务；本轮仅保证前端 API DTO 不再另起一套。
2. 当前 worktree 混有其它任务变更；提交前应由用户自行分拣 staged/unstaged 范围，避免把 unrelated change 合入同一个提交。
3. 完整 `task check` 已在追加验证中通过；当前无已知质量门禁阻塞。

## Incomplete items / 未完成项

- 本需求内无未完成项。
- 当前无已知非本需求质量门禁阻塞项。

## Conclusion / 结论

前端 Proto DTO 清理需求已完成并通过本需求相关验证。前端 API DTO 的来源已收敛到 `web/src/gen/proto/...`，缺失 request / response body 已补 proto，terminal WebSocket control protocol 已 proto-defined，runtime request body 与 path 参数已分离。

追加验证中完整项目 `task check` 已通过；当前无已知质量门禁阻塞。
