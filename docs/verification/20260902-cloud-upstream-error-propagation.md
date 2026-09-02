# Cloud 上游错误传播验证 / Cloud Upstream Error Propagation Verification
最后修改时间: 2026-09-02 17:17:26

Review status: Accepted

## Flow mode / Stage

标准模式 / standard；验证 / Verification。

## Requirement alignment

已对照 [`docs/requirement/20260902-cloud-upstream-error-propagation.md`](../requirement/20260902-cloud-upstream-error-propagation.md) 验证：

- `CloudUpstreamError` 保留 `operation`、`upstreamStatus`、完整 Cloud `ErrorResp` 和 cause；`CloudCode()` 只读取 Cloud 既有 `code`。
- 已收到且可解析的 Cloud 非 `2xx` 错误直接沿用上游 HTTP status、`code`、`error`、`request_id` 与 `details`，本地 response 不增加 wrapper 或重复字段。
- 无 response、非结构化非 `2xx` body、以及 `2xx` 成功 DTO 解码失败按约定返回本地 `upstream_error` fallback；已收到的非结构化非 `2xx` 仍沿用上游 status。
- Agent HTTP 入口生成或接收的 `X-Request-ID` 写入 context；Cloud client 的统一 transport 对普通 HTTP 与 OAuth token request 都直接写入该 header。
- `200 authenticated:false` 保持 Cloud 身份探测的成功语义；JWT 在 `exp <= now` 时拒绝。
- 前端仅对使用 Cloud bearer 的本地 Cloud relay client 的 `401` 清理 Cloud 凭据；普通 local API 与 OAuth 换码不复用该行为。

## Spec alignment

不适用。该任务采用 standard 模式，未创建 Spec 文档。

## Plan alignment

Plan 的五个实施步骤均已完成：

1. 在 Agent application 层定义统一 Cloud typed error 与四个 operation 常量。
2. 普通 Cloud HTTP 和 OAuth `RetrieveError` 共用结构化错误解析路径。
3. Agent handler 使用单一 `writeCloudError` 输出路径，日志保留 operation、上游 status 和 Cloud code。
4. 前端新增显式 `localCloudApiClient`，移除按 URL 前缀推断凭据清理语义的设计。
5. JWT 到期边界改为 `exp <= now`，未修改 TTL 或引入续期策略。

## Actual diff summary

- `internal/agent/application/user/cloud_service.go`：新增 `CloudUpstreamError`，并移除 request ID 的业务方法参数。
- `internal/agent/infrastructure/cloudapi/client.go`：统一 Cloud error 解析，并用 context 驱动的 RoundTripper 透传 `X-Request-ID` 到四个 Cloud 出站操作及 OAuth。
- `internal/agent/api/handler/server.go`、`errors.go`：入口写入 request ID context，结构化 Cloud error 直接输出，日志补充 typed-error 字段。
- `internal/shared/common/requestid/context.go`：提供请求 ID context 读写工具。
- `web/src/features/api/client.ts`、`web/src/features/local/api.ts`：为本地 Cloud relay 请求使用独立 client，仅在该调用身份收到 `401` 时清理 Cloud 凭据。
- `internal/shared/common/auth/jwt.go`：修正 JWT 精确到期时的拒绝边界。
- 对 Cloud client、Agent handler、JWT 和前端 API client 增加或更新回归测试。

## Expected vs actual files

Plan 中列出的后端、前端、Requirement、Plan 文件均在实际变更中；新增 `internal/shared/common/requestid/context.go` 和 `internal/shared/common/auth/jwt_test.go` 也与 Plan 一致。

新增本 Verification 文档属于验证阶段记录。未发现范围外的代码、协议 schema、迁移、开发服务器或 Git metadata 变更。

## Acceptance checklist

- [x] 四个 Agent-to-Cloud 操作的非成功响应均产生统一 typed error。
- [x] 结构化 Cloud error 的 status、`code`、`error`、`request_id`、`details` 原样继承，且没有嵌套 wrapper。
- [x] 入站已有或由 Agent 生成的 `X-Request-ID` 均通过 context 和共享 transport 传递到 Cloud；普通请求与 OAuth 共用该机制。
- [x] Cloud `401 unauthorized` 经本地 devices route 仍为 `401 unauthorized`。
- [x] Cloud `409 conflict` 经本地 connect route 仍为 `409 conflict`。
- [x] OAuth `4xx` 与普通 Cloud HTTP response 使用同一错误传播路径。
- [x] Cloud `200 authenticated:false` 仍是成功响应；异常非 `2xx` 不再软转换为未认证成功响应。
- [x] 前端 token 清理依据明确的 client 调用身份，不依赖 URL、operation 或具体 status 改写。
- [x] 无上游 response 与成功 DTO 解码失败保持本地 `502 upstream_error` fallback；非结构化非 `2xx` 保留上游 status。
- [x] JWT 在 `exp == now` 时拒绝；TTL 未改变。

## Test results

```text
PASS  yarn --cwd web lint:fix
PASS  yarn --cwd web typecheck
PASS  task check
PASS  go test ./cmd/... ./internal/...
PASS  go test ./internal/agent/api/handler ./internal/agent/infrastructure/cloudapi ./internal/shared/common/auth
PASS  yarn --cwd web test src/features/api/client.test.ts
       1 test file, 11 tests passed
PASS  git -c core.whitespace=cr-at-eol diff --check
```

未启动、停止或重启开发服务器。

## Scope deviation

无。实现与 Plan 约束一致。

## Risks

- 本地 API 现在直接依赖 Cloud `ErrorResp` 契约的稳定性；Cloud 端应继续保持其结构化错误测试。
- request ID 一致性依赖 Cloud 服务按入站 `X-Request-ID` 生成其 response `request_id`，该行为已由 Agent 边界测试覆盖。

## Incomplete items

无。

## Conclusion

用户已确认验证通过。该变更满足已接受的 Requirement 与 Plan，可交付。
