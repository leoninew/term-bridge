# 基于 Ed25519 公钥指纹的设备身份 / Device Identity from Ed25519 Public-Key Fingerprint
最后修改时间: 2026-07-20 10:34:58

Review status: Accepted

## Background

当前 Agent 在 `<runtime.state_dir>/device.json` 缺失时生成 128-bit 随机 `device_id`，并独立生成、持久化 Ed25519 密钥对。`device_id` 同时用于本地 runtime 数据归属、Cloud `devices` 全局主键、tunnel 路由索引，以及 tunnel 签名请求中的设备声明；Ed25519 私钥用于证明对该声明的控制权。

现有模型中，Cloud 设备上报可对已有全局 `device_id` 更新保存的公钥。仅依赖一个可提交的 ID 不能建立 ID 与认证公钥的密码学对应关系，且可能允许未经授权的公钥覆盖。

本需求将新安装的 canonical `device_id` 定义为本地 Ed25519 公钥的加密指纹，使设备标识与用于 tunnel 身份证明的密钥形成可验证绑定。该变更替代此前评估的 `machineid` 方案；不引入 OS 机器指纹作为 canonical device identity。

## Goal

1. 新建本地设备身份时，先创建或读取 Ed25519 密钥对，再从公钥确定性派生 `device_id`。
2. `device_id` 使用公开、稳定、无歧义的短指纹表示：`hex(SHA-256(ed25519_public_key)[:16])`。它是 128-bit 截断哈希的 32 位小写 hex，不暴露私钥、不依赖 OS 机器标识，并保持当前 URL 与数据库 ID 的长度。
3. Cloud 设备注册/上报校验提交的 `device_id` 必须与提交的 Ed25519 公钥指纹一致。
4. Cloud 对已存在的 `device_id` 默认拒绝不同公钥的普通上报，禁止静默覆盖设备认证公钥或跨用户接管同一设备身份。
5. 保持现有 tunnel 签名协议的“ID 用于查找、公钥用于验签”职责分离；Cloud 校验完成后，二者具有密码学绑定。

## Non-goal

1. 不引入 `github.com/denisbrodbeck/machineid`，不使用 raw 或 protected OS machine ID 作为 canonical `device_id`。
2. 不在本需求中实现历史随机 ID、既有本地 runtime 数据或既有 Cloud device 记录的迁移与兼容；本次变更按无历史数据前提设计。
3. 不在本需求中实现完整密钥轮换、设备恢复、吊销或审计产品流程。
4. 不改变设备显示名的 hostname 生成规则。
5. 不将私钥、原始 OS 机器标识或新的设备认证信息放入 OAuth URL、OAuth state/code 或普通用户输入表单。

## User scenarios

1. 作为首次启动 Agent 的用户，系统创建 Ed25519 密钥对，并自动生成可长期复用的、由该公钥派生的设备 ID；我无需输入设备 ID 或机器标识。
2. 作为重启或升级 Agent 的用户，只要保留 stateDir，系统读取既有 `device.json` 与私钥，设备 ID、公钥和已有 Cloud 设备关系均保持不变。
3. 作为 Cloud 用户，我连接自己的新设备时，Cloud 只接受 ID 与公钥指纹匹配的设备上报。
4. 作为另一个 Cloud 用户，当我上报与既有记录相同 `device_id` 且相同公钥的设备时，Cloud 将其认定为同一设备并允许新增共享绑定。
5. 作为既有设备所有者，持有不同私钥的安装实例不能通过重复提交相同 `device_id` 静默替换 Cloud 公钥。
6. 作为运维人员，复制完整 stateDir 仍被视为复制同一设备身份；复制 stateDir 不属于支持的多设备部署方式。

## Acceptance

- [ ] 对缺少 `device.json` 的新 stateDir，系统生成或加载 Ed25519 私钥后，以其公钥派生 `device_id`；派生算法固定为 `hex(SHA-256(publicKey)[:16])`，其中 `publicKey` 是原始 Ed25519 公钥 32-byte 值，输出为 32 位小写 hex；格式、输入字节和编码方式有单一实现与测试。
- [ ] 新生成的 ID 与对应公钥指纹完全一致；私钥不进入 `device.json`、Cloud 请求体、日志或错误消息。
- [ ] 已存在且合法的 `device.json` 仍直接读取其中 ID，不会因升级被改写为公钥指纹，也不会触发新的 ID 生成。
- [ ] Cloud 首次注册仅接受合法非空字段、可解析的 Ed25519 公钥，以及与该公钥指纹一致的 `device_id`。
- [ ] 同一 `device_id` 且同一公钥的重复上报保持幂等，可更新允许更新的非认证元数据与既有绑定时间；来自另一 Cloud 用户的同 ID、同公钥上报被认定为同一设备，并允许新增共享 `user_devices` 绑定。
- [ ] 同一 `device_id` 但不同公钥的普通设备上报被拒绝；Cloud 不更新已有 `devices.public_key`，不新增跨用户绑定。
- [ ] Agent 以新 ID 与私钥建立 tunnel 时，既有签名、Cloud 公钥查找、验签和 Hello-ID 一致性检查仍能通过。
- [ ] 覆盖新身份派生、ID-公钥不匹配拒绝、同 ID 不同公钥拒绝、同 ID 同公钥的同用户幂等上报与跨用户共享绑定，以及 tunnel 正常认证的自动化测试。

## Open questions

1. 显式密钥轮换的授权协议、恢复与设备重置流程不在范围内；在其落地前，密钥丢失或损坏的处理策略需作为运营风险保留。

## Decisions

1. canonical `device_id` 采用 Ed25519 public-key fingerprint，而不是随机 ID 或 `machineid`。
2. 固定算法为 `hex(SHA-256(publicKey)[:16])`：输入为用于 Cloud 上报与 tunnel 验签的原始 Ed25519 公钥 32-byte 值，取 SHA-256 输出的前 16 bytes，并进行小写 hex 编码，最终 ID 长度固定为 32 字符。该长度与历史随机 ID 一致，适合 URL route；禁止存在多个派生实现。
3. 本次按无历史数据前提设计，不实现随机 ID 或既有 Cloud device 记录的迁移与兼容。
4. Cloud ID-公钥校验和不同公钥覆盖拒绝纳入同一任务，不拆分为后续安全加固。
5. 同一 ID 与同一公钥唯一确定同一设备；允许多个 Cloud 用户通过 `user_devices` 共享绑定该设备。
6. 当前 Ed25519 密钥对继续承担认证职责；公钥指纹仅作为设备标识符，不替代私钥签名认证。

## Risk

1. **克隆风险**：复制完整 stateDir 会复制私钥及 ID，克隆实例可作为同一设备认证并竞争在线 tunnel route；需明确禁止将完整 stateDir 用于复制出独立设备。
2. **密钥不可恢复风险**：没有显式 key rotation/recovery 前，私钥丢失或损坏会导致设备无法继续证明既有身份；不能通过普通上报静默换钥绕过。
3. **共享绑定风险**：同一 ID 与公钥允许多用户绑定；设备删除、解绑、访问授权和在线路由对共享用户的影响必须沿用或在规格阶段明确现有 `user_devices` 语义。

## User review notes

- 用户确认：新安装采用 Ed25519 公钥指纹作为 canonical `device_id`。
- 用户确认：Cloud ID-公钥一致性校验与禁止未授权公钥覆盖纳入同一任务。
- 用户确认：不考虑已有数据；本次不实现历史随机 ID 或既有 Cloud 设备记录的兼容与迁移。
- 用户确认：相同 ID 且相同公钥即认定为同一设备，允许多用户共享 `user_devices` 绑定。
- 用户确认：新 ID 使用 `SHA-256(publicKey)` 前 16 bytes 的 32 位小写 hex 编码，以保持 URL 与历史 ID 长度。
