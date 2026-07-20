# 基于 Ed25519 公钥指纹的设备身份规格 / Device Identity Public-Key Fingerprint Specification
最后修改时间: 2026-07-20 10:46:04

Review status: Accepted

## Requirement basis

依据已接受的 [`20260720-device-public-key-identity`](../requirement/20260720-device-public-key-identity.md) 需求，本次不考虑历史数据、迁移或兼容。canonical `device_id` 必须与 tunnel 验签使用的 Ed25519 公钥形成可验证绑定，并保持 32 位 URL 友好格式。

## Canonical identity contract

```text
device_id = hex(SHA-256(raw_ed25519_public_key)[:16])
```

- 输入是原始 32-byte Ed25519 public key，不能是 PEM 或 Base64 文本。
- 输出是恰好 32 个小写 hex 字符。
- `public_key` 继续为标准 padded Base64 编码的原始公钥；私钥只保存在 Agent 本地并用于签名。
- 共享 helper `internal/shared/common/security/device_identity.go` 是唯一派生及严格解析入口。
- 本项目内部缩写使用 `Id`，因此 helper 与局部变量采用 `DeviceId`、`userId`、`expectedId` 等形式；协议名与算法名称保持 `Ed25519`、`SHA-256`、`Base64`。

## Local identity lifecycle

1. `LoadOrCreateDevice` 先加载或生成 `private_key.pem`，由私钥推导公钥。
2. 从公钥派生 canonical ID。
3. 缺少 `device.json` 时，将派生 ID、默认设备名及时间戳原子写入身份文件。
4. 存在 `device.json` 时，其 ID 必须与当前私钥公钥的派生 ID 完全一致；否则启动失败，不重写 identity 或密钥。
5. `Device.PublicKey` 使用 canonical standard Base64，维持 Cloud report 和 tunnel 验签的既有接口。

## Cloud registration state machine

`POST /api/devices/current` 保持现有 `{id,name,public_key}` proto contract，不新增 schema 或请求字段。

1. 校验用户 ID、设备名、32 位小写 hex ID。
2. 严格解析 `public_key`：必须为 canonical 标准 padded Base64，解码后恰为 32-byte Ed25519 公钥。
3. 从解码后的原始公钥派生 expected ID。
4. 当请求 ID 与 expected ID 不一致：
   - 若该 ID 不存在，返回 400 `bad_request`，不写入；
   - 若该 ID 已有记录，返回 409 `conflict`，不写入。该处理让截断指纹碰撞或恶意同 ID 不同 key 均无法覆盖既有认证公钥。
5. 请求 ID 与 expected ID 一致时，在单一事务内检查现有 `devices.public_key`：
   - 无记录：创建 device，并创建当前用户 binding；
   - 相同公钥：更新全局名称及更新时间，并幂等创建/更新时间当前用户 binding；另一用户的相同 key 上报创建共享 binding；
   - 不同公钥：返回 409 `conflict`，事务回滚，不更新 device 元数据或绑定。

## Shared binding and deletion

- 相同 ID 与相同公钥唯一确定一台共享设备；多个 Cloud 用户可存在 `(user_id, device_id)` binding。
- 删除只移除当前用户 binding。
- repository 返回是否删除了最后一个 binding；Handler 仅在最后一个 binding 删除后关闭 tunnel route、标记设备离线并删除 `devices` 行。
- 仍有其他 binding 时，设备记录和在线路由保持可用。

## Tunnel verification

1. Cloud 从 tunnel header 的 device ID 查询保存的 Base64 公钥。
2. 解码并检查 Ed25519 公钥长度。
3. 从 raw public key 重新派生 ID，必须与 tunnel header 的 ID 一致。
4. 仅通过 ID/key 一致性检查后，执行现有 Ed25519 signed-request 验证和 Hello ID 一致性检查。

## HTTP errors

| 条件 | HTTP | code | 安全文案 |
|---|---:|---|---|
| 格式不合法、非 canonical Base64、错误公钥长度、未占用 ID 与公钥指纹不匹配 | 400 | `bad_request` | `Device report is invalid.` |
| 既有 ID 与不同公钥（包括公钥指纹不匹配） | 409 | `conflict` | `Device identity is already bound to a different public key.` |
| 未预期数据库或事务错误 | 500 | `internal_error` | 既有通用内部错误 |

错误响应、日志安全文案与 details 不得包含私钥、完整公钥或中间哈希值。

## Affected components

- `internal/shared/common/security/device_identity.go`：指纹派生、ID 格式和 canonical Base64 公钥解析。
- `internal/agent/application/user/device.go`：本地密钥优先的 identity 生命周期与一致性校验。
- `internal/cloud/repository/user/device/repository.go`：注册事务、不可覆盖公钥、共享 binding、最后 binding 删除结果。
- `internal/cloud/api/handler/server.go`：400/409 映射及按最后 binding 清理 route。
- `internal/cloud/api/handler/tunnel_auth.go`：认证边界再次验证 ID/key 对应关系。

## Scope boundaries and risks

- 不支持旧随机 ID 或既有 Cloud device records；必须在无历史数据环境部署。
- 不实现 key rotation、恢复、撤销、审计与显式设备重置。
- 完整复制 stateDir 会复制私钥和 ID，克隆实例属于同一设备并竞争 route，不属于支持部署方式。
- 128-bit 截断指纹只承担低碰撞路由键职责；完整 Ed25519 公钥与签名仍承担认证。碰撞场景会触发不同 key 冲突拒绝。

## User review notes

- 用户确认：共享设备解绑仅影响当前用户；只有最后一个 binding 删除才清理设备及在线路由。
- 用户确认：任一持有同一私钥的实例可刷新全局设备名称。
- 用户要求：仅处理本任务相关变更，避免扩大影响面。
