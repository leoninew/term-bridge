# 基于 Ed25519 公钥指纹的设备身份验证 / Device Identity Public-Key Fingerprint Verification
最后修改时间: 2026-07-20 10:46:04

Review status: Accepted

## Requirement alignment

- 已实现 `hex(SHA-256(raw_ed25519_public_key)[:16])`，输出固定为 32 位小写 hex。
- Agent 先加载/生成 Ed25519 密钥，再派生 ID；已有 `device.json` ID 与密钥派生值不一致时失败且不改写。
- Cloud 注册严格校验 canonical ID、canonical standard Base64 Ed25519 公钥和二者对应关系。
- 同 ID + 同公钥允许同用户幂等和多用户共享 binding；同 ID + 不同公钥返回 409 且不覆盖已有 key。
- 删除共享设备时，仅最后一个 user binding 被删除才断开 route。
- tunnel 验签前再次校验 header device ID 与保存公钥的派生 ID 一致。

## Spec alignment

- `internal/shared/common/security/device_identity.go` 提供唯一的派生、格式校验及 canonical Base64 公钥解析实现。
- 未修改 proto、Cloud schema、OAuth 参数或前端交互。
- 命名遵循项目缩写规则：本次新增/修改的内部标识符使用 `Id`，未引入 `ID`、`userID`、`expectedID` 或 `StreamID` 形式。

## Actual diff summary

- 新增 shared security helper 与测试，覆盖精确派生向量、ID 格式、公钥长度及 Base64 canonical 约束。
- 本地 device lifecycle 改为 key-first，移除随机 device ID 路径并增加 identity/key 一致性校验。
- Cloud repository 增加 identity 错误类型、不可覆盖 key、共享 binding 与最后 binding 删除结果；SQLite/MySQL 均使用 no-overwrite insert 语句。
- Cloud handler 将非法 identity 映射到 400，将 key conflict 映射到 409；仅最后 binding 删除时断开设备 route。
- tunnel 验证增加 ID/key 防御性一致性检查。
- 更新 Cloud tunnel/relay/terminal/workspace 测试的 fixture ID，使其对应固定测试公钥的 canonical 32 位 ID。

## Expected vs actual changed files

| 预期范围 | 实际情况 |
|---|---|
| local device identity | `internal/agent/application/user/device.go` 与测试已修改 |
| shared identity helper | 新增 `internal/shared/common/security/device_identity.go` 与测试 |
| Cloud registration/repository | `internal/cloud/repository/user/device/repository.go`、handler interface/server 及测试已修改 |
| tunnel consistency | `internal/cloud/api/handler/tunnel_auth.go` 已修改 |
| shared deletion behavior | repository、handler 与 Cloud binding 测试已修改 |
| requirement/spec/verification 文档 | 三个阶段文档均已创建或更新 |

## Acceptance checklist

- [x] 新 ID 为 raw Ed25519 public key 的 SHA-256 前 16 bytes 小写 hex。
- [x] ID 长度固定为 32 字符，适合 URL 与现有数据库字段。
- [x] 本地 identity 与私钥不匹配时失败且不重写。
- [x] Cloud 拒绝无效 ID、公钥编码/长度或未占用 ID 的 ID-key 不匹配。
- [x] 已存在 ID 的不同 key 被拒绝，且原 key 保留。
- [x] 相同 ID + 相同 key 可以由两个 Cloud 用户创建共享 binding。
- [x] 单一共享用户解绑不会断开设备 route；最后 binding 删除会断开。
- [x] canonical ID 与 key 的 tunnel 签名验证通过。
- [x] 项目内部缩写在本次任务变更中遵循 `Id` 风格。

## Command results

```text
go test ./internal/shared/common/security ./internal/agent/application/user ./internal/cloud/api/handler ./internal/cloud/repository/user/device
PASS

go test ./cmd/... ./internal/...
PASS

task check
PASS（Vue typecheck、frontend lint/format、golangci-lint fmt/run 均通过）

gofmt -w <本次任务相关 Go 文件>
PASS

git diff --check
PASS
```

## Scope deviation

无产品范围扩展。本次仅处理设备身份、Cloud 注册/共享绑定、tunnel ID-key 验证、相关删除语义、测试与阶段文档。未引入 `machineid`，未修改 schema/proto，未实现历史兼容、密钥轮换、恢复、吊销或前端交互。

## Risks and incomplete items

1. 不支持历史随机 ID 或旧 Cloud records；部署前需要保证目标环境无须兼容旧数据。
2. 完整复制 stateDir 会复制私钥与 ID，克隆实例被视为同一设备并可能竞争 route。
3. 未实现显式 key rotation/recovery；私钥丢失不能经普通设备上报替换。
4. MySQL no-overwrite insert SQL 已纳入实现与全项目编译测试；本次没有连接独立 MySQL 实例执行集成测试。

## Conclusion

当前实现满足已接受的 identity、注册安全、共享 binding、解绑与 tunnel 验证需求。自动化 Go 测试全量通过，格式化和 diff whitespace 检查通过。
