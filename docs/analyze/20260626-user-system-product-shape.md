# 用户系统引入前的产品形态审视

最后修改时间: 2026-06-26 21:44:24

## 1. 结论

引入用户系统后，TermBridge 的产品入口不应变成“登录表单 + 设备 ID 表单”。从用户角度，它应该变成：

```text
我是谁
  -> 我有哪些可访问的设备
    -> 我选择一台设备
      -> 我进入这台设备上的 workspace / session / terminal
```

也就是：

```text
User -> Device -> Workspace -> Session -> Terminal
```

用户系统的产品价值不是“多一个账号密码”，而是让用户清楚三件事：

1. **我是谁**：当前登录的是哪个人 / account / personal space。
2. **我能操作哪些设备**：设备是资源，不是登录凭据。
3. **我现在正在操作哪台设备上的 session**：runtime ownership 清晰。

可以将未来用户系统的产品方向收敛为一句话：

> TermBridge 的用户系统首先用于确认“谁在访问”，然后展示“这个用户有权访问哪些设备”；用户选择设备后，才进入该设备上的 workspace、session 和 terminal。Device 是被授权访问的 runtime 资源，不是登录凭据；本地 self-connected 是这一模型的单用户压缩形态。

## 2. 当前本地形态与引入用户系统后的变化

当前 TermBridge 的本地心智模型更像：

```text
我启动 termbridge serve
  -> 打开 Browser
  -> 进入 /sessions
  -> 操作本机 session
```

这是对本地开发者友好的。

但一旦引入用户系统，用户看到的产品应该升级为：

```text
打开 TermBridge
  -> 登录 / 完成首次设置
  -> 看到我的设备
  -> 选择设备
  -> 进入这个设备的 sessions
```

这里 **User 是入口，Device 是资源**。

## 3. 推荐的用户旅程

### 3.1 本地首次启动

用户第一次运行：

```text
termbridge serve
```

用户不应该看到一堆随机用户名、密码、device_id，而应该看到类似：

```text
TermBridge first-run setup

Open:
http://127.0.0.1:9010/setup?token=xxxx

This link is shown once and expires after setup.
```

浏览器中：

```text
Welcome to TermBridge

Set up your local account:
- display name
- password

This computer will be added as your first device:
- device name: Leon-PC
- status: online

[Create account and enter workbench]
```

用户理解的是：

> 我正在为这台机器上的 TermBridge 创建本地管理员账号，并把当前电脑加入我的设备列表。

而不是：

> 我需要理解 auth.username、auth.password、device_id、agent credential。

### 3.2 本地日常使用

用户之后打开 TermBridge：

```text
Sign in
  -> 如果只有一台 device，直接进入 /sessions
  -> 如果多台 device，先展示 device selector
```

单设备场景应该尽量直达：

```text
Login -> Sessions
```

多设备场景才需要强调选择：

```text
Login -> Devices -> Sessions
```

但无论是否跳过选择页，页面上都应该始终清楚表达：

```text
Current device: Leon-PC
Status: online
```

### 3.3 添加新设备

用户在 Cloud 或本地多设备形态下添加设备时，不应该手动填 raw `device_id`。

推荐产品流程：

```text
Settings / Devices
  -> Add Device
  -> TermBridge 生成一次性 pairing code / claim command
  -> 用户在目标机器运行命令
  -> 设备出现在 device list
```

用户看到的应该是：

```text
Add a device

Run this on the device you want to connect:

termbridge agent enroll --gate https://gate.example.com --code ABCD-EFGH

This code expires in 10 minutes.
```

完成后：

```text
New device connected:
- MacBook-Pro
- Windows 11
- Last seen: now

[Enter workbench]
```

用户理解的是：

> 我把一台设备加入了我的账号。

而不是：

> 我在配置 Agent 的认证材料。

### 3.4 远端 Cloud Gate 使用

Cloud 形态下用户旅程应该是：

```text
Open Cloud Gate
  -> Sign in
  -> See authorized devices
  -> Select device
  -> Enter sessions
```

也就是：

```text
Browser User
  -> authenticates to Gate
  -> selects authorized Device
  -> Gate routes to Device Agent
  -> Workspace / Session / Terminal
```

重点是：

- 未登录前不展示设备列表。
- 登录后只展示有权限访问的设备。
- 设备 offline 时可以看到，但不能误以为 session 操作失败是登录失败。
- device 不参与用户登录本身。

## 4. 产品信息架构建议

引入用户系统后，产品表面可以分成四块。

### 4.1 Setup / Login

负责回答：

```text
你是谁？
```

状态：

- first-run setup
- local password login
- future OAuth / OIDC login
- logout
- session expired

不要在这里混入 device_id。

### 4.2 Device Workbench Entry

负责回答：

```text
你要操作哪台设备？
```

内容：

- device name
- device status
- last seen
- platform / hostname
- current selected device
- unavailable reason

设备状态应该用户可理解：

```text
Online
Offline
Connecting
Unauthorized
Enrollment required
```

### 4.3 Sessions Workbench

负责回答：

```text
这台设备上有哪些 workspace / session / terminal？
```

这部分是现有 `/sessions` 的主体价值，应该尽量保持不变。

用户系统不应该把 `/sessions` 搞复杂，而是给它提供一个清晰的上层上下文：

```text
Signed in as Leon
Current device: Home-PC
```

### 4.4 Account / Devices Settings

负责：

- 修改本地账号密码。
- 查看已绑定设备。
- 重命名设备。
- revoke device。
- 添加设备。
- 查看 device credential 状态，但不要暴露 token 明文。

这部分可以后置，不必第一阶段做完整。

## 5. 最小可接受的用户系统

现在不建议一上来做完整 SaaS 用户系统。

建议第一阶段只做 **Personal / Local Owner 模型**。

### 5.1 第一阶段最小模型

```text
Personal Space
  -> Local User / Owner
      -> Devices
          -> Workspaces
              -> Sessions
```

用户界面不一定暴露 `tenant`、`org`、`realm` 这些词。

内部可以预留：

```text
realm_id / personal_space_id
user_id
device_id
```

但用户看到的是：

```text
My devices
My sessions
```

## 6. 本地和 Cloud 应该是同一心智模型

不要为本地和 Cloud 设计两套完全不同产品。

应该是同一个模型的两种压缩程度。

### 6.1 本地 self-connected

```text
User
  -> local device
      -> workspace
          -> session
```

用户感知：

> 我在本机使用 TermBridge。

### 6.2 Cloud Gate

```text
User
  -> remote-accessible devices
      -> workspace
          -> session
```

用户感知：

> 我登录云端 Gate，然后选择我要访问的设备。

二者差异在部署，不在产品模型。

## 7. 明确不要做的形态

### 7.1 不要让登录表单要求 device_id

错误形态：

```text
Username
Password
Device ID
[Login]
```

问题：

- `device_id` 不是用户友好字段。
- `device_id` 不是 secret。
- 用户会以为 device 是认证因子。
- 登录失败、设备不存在、设备无权限、设备离线会混在一起。
- 将来 OAuth / SSO 时这个形态完全不自然。

### 7.2 不要让 Agent 继续使用 Browser password

短期 PoC 可以固定 `admin/admin`，但长期产品不能让：

```text
Browser user password == Agent tunnel credential
```

长期应该是：

```text
Browser User Credential
  !=
Agent Device Credential
  !=
Enrollment Token
```

也就是三种身份材料：

| 类型 | 表示什么 | 生命周期 |
| --- | --- | --- |
| User credential | 人登录 | 用户控制，可重置 |
| Device credential | 设备连接 Gate | 可撤销、可轮换 |
| Enrollment token | 绑定设备的一次性材料 | 短期、一次性 |

### 7.3 不要第一阶段暴露完整 Org / RBAC

用户系统第一阶段不要直接做成企业后台。

不建议一开始就暴露：

- Tenant 管理
- Org 切换
- RBAC
- Team invite
- Audit log
- SSO 管理
- 多角色权限矩阵

这些是未来 Cloud / SaaS 的能力，但不是第一阶段用户系统的最小产品形态。

## 8. 建议阶段拆分

### Phase 1：Local Owner / First-run Setup

目标：替代 `admin/admin` 这种 PoC 入口。

用户结果：

```text
首次启动时创建本地 owner。
本机 device 自动加入。
用户登录后进入自己的 workbench。
```

范围：

- first-run setup token / setup URL
- local user / owner
- password hash
- session cookie / browser login
- 当前 device 自动 claim
- 单用户 personal space
- `/sessions` 继续作为主工作台

不做：

- OAuth
- 多用户
- Org
- RBAC
- remote enrollment 完整闭环

### Phase 2：Device Enrollment

目标：让设备绑定成为正式产品能力。

用户结果：

```text
我可以把另一台机器加入我的 TermBridge。
```

范围：

- Add Device
- pairing code / claim URL
- Agent enroll command
- device token
- revoke device
- device list status

### Phase 3：Cloud Identity

目标：支持远端 Gate。

用户结果：

```text
我登录 Cloud Gate，看到我的设备，从其他设备访问本地 runtime。
```

范围：

- OAuth / OIDC / SSO 或 local cloud password
- user account
- device authorization
- browser session
- secure remote Agent tunnel

### Phase 4：Org / Team / Enterprise

目标：团队和企业能力。

范围：

- Org / Tenant
- invite
- role / permission
- audit
- policy
- credential rotation policy

## 9. 对当前 open questions 的产品向建议

### 9.1 本地 first-run 是否自动创建 local admin？

建议：**不要静默自动创建长期 admin**。

可以自动生成一次性 setup token，但用户应通过 setup 页面确认：

```text
Create local owner
```

这样用户知道“我创建了一个账号”。

### 9.2 本机 device 是否自动绑定到 local admin？

建议：**自动绑定，但明确展示**。

本地 first-run 场景下，不需要让用户再跑 pairing code。否则本地体验会过重。

但 UI 应该告诉用户：

```text
This computer will be added as your first device.
```

### 9.3 Cloud enrollment 用 pairing code 还是 claim URL？

建议：第一版用 **copyable CLI command + short-lived code**。

例如：

```text
termbridge agent enroll --gate https://gate.example.com --code ABCD-EFGH
```

claim URL 可以后续补。

CLI command 对当前产品最自然，因为 Agent 在用户设备上运行。

### 9.4 Device credential 用 bearer token 还是 mTLS？

建议：第一版用 **device token**，但抽象上不要写死为普通用户密码。

mTLS 可以后续作为更高安全等级。

第一阶段更重要的是把身份域拆开：

```text
device_token != user_password
```

### 9.5 local password 放在哪里？

用户视角不关心，但产品边界上不建议继续明文放 `.termbridge.yaml`。

建议方向：

- user password 存 hash。
- device token 单独存。
- `.termbridge.yaml` 可以保留非敏感配置和 device identity。
- secret 存储后续可以演进到 OS credential manager。

### 9.6 单用户本地部署是否需要 Tenant / Org？

建议：内部可以有隐式 `personal realm`，UI 不暴露。

用户看到：

```text
My TermBridge
My devices
```

不要让本地用户理解 Tenant / Org。

### 9.7 OAuth 用户和 local bootstrap 用户如何合并？

先不要第一阶段解决。

但数据模型上要避免把 username 当主键，未来才能迁移：

```text
user_id
provider
provider_subject
realm_id
```

## 10. 用户系统引入后的验收标准

不能只看“能登录”。应该从用户路径验收。

### 10.1 登录前

- 用户知道自己是在 setup、login，还是 session expired。
- 未登录不展示 device list。
- 不要求输入 raw `device_id`。

### 10.2 登录后

- 用户能看到当前登录身份。
- 用户能看到 authorized devices。
- 用户能区分 online / offline / unavailable。
- 单设备场景可以自然进入 workbench。
- 多设备场景必须明确当前 selected device。

### 10.3 设备绑定

- 用户可以理解“添加设备”是把某台机器加入账号。
- pairing 材料短期有效。
- enrollment 完成后不再使用 pairing token。
- Agent 使用 device credential，而不是用户密码。

### 10.4 `/sessions`

- workspace / session / terminal 始终归属于 selected device。
- 切换 device 不污染状态。
- route unavailable、auth error、device offline 有不同反馈。

## 11. 与长期身份需求文档的关系

本分析基于 `docs/requirement/20260623-long-term-identity-device-enrollment.md`，并从用户角度补充产品形态判断。

该需求文档中的长期方向仍然成立：

- 长期应先有 User / Account / Tenant 认证，再有 Device 选择和授权访问。
- 当前生成凭据形式适合作为本地 first-run bootstrap shortcut，但不是长期身份架构。
- Device 通过 enrollment / pairing 绑定到用户或组织。
- 登录 UI 不要求普通用户填写 raw device id；可以在同一个入口卡片中完成登录后设备选择。
