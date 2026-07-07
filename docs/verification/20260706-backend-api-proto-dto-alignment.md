# 后端 API 请求响应体 proto DTO 对齐验证

最后修改时间: 2026-07-06 17:40:24

Review status: Accepted

## Requirement alignment

依据：`docs/requirement/20260706-backend-api-proto-dto-alignment.md`，状态为 `Accepted`。

本次实现与需求对齐情况：

- API 请求/响应体已从 handler 手写 DTO 收敛到生成 proto 类型：`cloudv1`、`commonv1`、`runtimev1`、`terminalv1`、`tunnelv1`。
- 已删除重复的 handler DTO 文件和 runtime DTO 文件，不保留同 shape 手写 wire DTO。
- 没有引入适配层、兼容层、re-export 或 alias DTO。
- 如果 proto 已能表达现有契约，业务代码直接引用生成类型。
- 时间字段处理从 handler `dto.go` 中提取到共享 proto 时间工具 `prototime`，避免用 DTO 文件承载字段转换。
- agent handler 内部保存 `cloudv1.CloudSessionSummary` 时，protobuf message 的防御性拷贝直接写在状态所有者方法内，不再单独建立 helper 层。

## Spec alignment

不适用。当前流程为轻量模式 / light，没有独立 Spec 阶段；按 Requirement 核对。

## Plan alignment

不适用。当前流程为轻量模式 / light，没有独立 Plan 阶段；按 Requirement 和实现结果核对。

## Actual diff summary

实际变更范围：

- 新增需求文档：
  - `docs/requirement/20260706-backend-api-proto-dto-alignment.md`
- 删除冗余 DTO / wire 类型文件：
  - `internal/agent/api/handler/auth_types.go`
  - `internal/agent/api/handler/dto.go`
  - `internal/agent/application/task/terminal/dto.go`
  - `internal/cloud/api/handler/auth_types.go`
  - `internal/cloud/api/handler/dto.go`
  - `internal/cloud/api/handler/runtime_dto.go`
- 新增共享 proto 时间工具：
  - `internal/shared/common/utils/prototime/time.go`
- 更新 agent API handler：
  - auth、health、cloud connect、runtime endpoint、terminal control message 改用生成 proto 类型。
  - 本地 cloud session 状态保存和读取直接在业务状态方法内使用 `proto.Clone` 做防御性拷贝。
- 更新 cloud API handler：
  - auth、device、runtime、tunnel、terminal relay 等请求/响应体改用生成 proto 类型。
- 更新 application/runtime 链路：
  - terminal registry、runtime access、cloud client 改为直接传递 `runtimev1.*` / `terminalv1.*`。
  - 删除 runtime tunnel response 的冗余转换 helper。
- 更新测试：
  - 测试响应解析改用生成 proto 类型和 `codec.UnmarshalProtoJSON`。
  - 保留测试局部 `testErrorResponse` 仅用于错误响应断言。
- 支撑性修复：
  - `internal/cloud/infrastructure/database/migrator.go` 修正 cloud migrations 引用，保证 cloud handler 测试使用正确迁移。

## Expected vs actual changed files

### 预期范围

Requirement 预期影响：

- `internal/agent/api/handler`
- `internal/cloud/api/handler`
- `internal/agent/application/task/terminal`
- `internal/agent/application/user`
- `internal/shared/dto/protocol/terminal`
- 相关测试

### 实际范围

实际变更与预期基本一致，并包含以下合理扩展：

- `internal/shared/common/utils/prototime/time.go`：集中处理 proto timestamp 字段转换，替代 handler `dto.go` 中的重复 helper。
- `internal/agent/application/bootstrap/server.go`：callback 类型改为 `cloudv1.CloudSessionSummary` 后的编译链路同步。
- `internal/cloud/application/user/auth/service.go`、`internal/cloud/model/user/auth/types.go`：auth service 返回生成 proto 类型后，删除 wire-ish 手写返回模型。
- `internal/cloud/infrastructure/database/migrator.go`：验证过程中发现 cloud handler 测试依赖正确迁移路径，作为支撑修复纳入本次变更。

未纳入本验证范围但当前工作区存在的文件：

- `docs/requirement/20260706-frontend-non-session-rewrite.md`：未暂存的新文件，属于独立需求文档，不计入本次后端 proto DTO 验证结论。

## Acceptance checklist

- [x] `HealthResp` 使用 `commonv1.HealthResp`。
- [x] auth 请求/响应使用 `cloudv1.*` 生成类型。
- [x] device、cloud session、cloud connect 请求/响应使用 `cloudv1.*` 生成类型。
- [x] runtime 请求/响应使用 `runtimev1.*` 生成类型。
- [x] terminal control message 直接使用 `terminalv1.ClientControlMessage` / `terminalv1.ServerControlMessage`。
- [x] 删除 handler 中同 shape 手写 DTO 文件。
- [x] 删除 runtime DTO 文件中与 `runtimev1.*` 重复的请求模型。
- [x] 不保留 `type X = xxxv1.X` alias / re-export。
- [x] 不新增手写请求/响应体绕过 proto。
- [x] 测试解析不依赖旧业务 DTO；必要解析改用生成 proto 或测试局部结构。
- [x] timestamp 转换集中为共享 proto 时间工具，不继续放在 handler `dto.go`。
- [x] implementation 后执行全量 Go 测试。

## Verification commands

### 结构扫描

1. handler DTO 文件扫描：

```text
Glob: internal/**/api/handler/*dto*.go
Result: No files found
```

2. proto alias / re-export 扫描：

```text
Grep: type\s+\w+\s*=\s*\w+v1\.
Result: No matches found
```

3. 旧 timestamp helper 残留扫描：

```text
Grep: func\s+(timestampFromTime|timeFromTimestamp)|timestampFromTime|timeFromTimestamp
Result: No matches found
```

4. 手写请求/响应体扫描：

```text
Grep: type\s+\w+(Req|Resp|Request|Response)\s+struct
Result: only generated proto files and internal/cloud/api/handler/server_test.go:testErrorResponse
```

`testErrorResponse` 是测试局部错误响应断言结构，不是 live API DTO。

### Go 测试

执行命令：

```text
go test ./cmd/... ./internal/...
```

结果：通过。

关键包结果包括：

```text
ok   termbridge/cmd/termbridge/app
ok   termbridge/cmd/termbridge/cli
ok   termbridge/internal/agent/api/handler
ok   termbridge/internal/agent/application/bootstrap
ok   termbridge/internal/agent/application/task/terminal
ok   termbridge/internal/agent/application/user
ok   termbridge/internal/cloud/api/handler
ok   termbridge/internal/cloud/application/user/auth
ok   termbridge/internal/cloud/infrastructure/database
ok   termbridge/internal/shared/common/utils/codec
ok   termbridge/internal/shared/dto/protocol/terminal
```

`internal/shared/common/utils/prototime` 无测试文件，但已随全量 Go 测试参与编译。

## Scope deviation

没有发现违反 Requirement 的范围偏离。

已识别的范围扩展：

- 新增 `prototime` 共享工具，是为了移除 handler `dto.go` 中的字段处理残留，属于需求内的清理性扩展。
- `migrator.go` 的迁移路径修正是为了让 cloud handler 测试覆盖真实数据库迁移行为，属于验证支撑修复。

## Risks and notes

- 编辑器可能继续对已删除文件显示 stale 诊断，例如 `internal/agent/api/handler/dto.go` 或 `internal/agent/application/task/terminal/dto.go` 的 “No packages found for open file”。磁盘和结构扫描均确认这些文件已删除，Go 测试通过；关闭对应 tab 或 reload gopls 即可。
- Go 静态分析仍可能提示 `rangeint` 或 `testingcontext` 等风格建议；这些不是本次 proto DTO 对齐的功能缺陷，也不影响测试结果。
- 当前工作区存在未暂存的 `docs/requirement/20260706-frontend-non-session-rewrite.md`，与本验证主题无关，提交前应单独处理或拆分。
- 本次没有执行前端 typecheck，因为变更范围未修改前端代码或生成 TS 契约。

## Incomplete items

无。

## Conclusion

验证通过。后端 API 请求/响应体已按 Accepted Requirement 收敛到生成 proto 类型，冗余 DTO、alias/re-export、handler `dto.go` 和重复 timestamp helper 已清理；全量 Go 测试通过。