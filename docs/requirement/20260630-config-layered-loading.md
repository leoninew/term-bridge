# Config Layered Loading Requirement
最后修改时间: 2026-07-01 13:21:25

## Review status

Accepted

## Flow mode / Stage

轻量模式 / light；需求 / Requirement 已接受，当前进入实现 / Implementation。

## Background

用户要求采纳后端配置分层加载最佳实践，对比对象为：

- `D:\SourceCodes\mywork\best-practices\docs\patterns\backend-api\config-layered-loading.cn.操作手册.md`
- `D:\SourceCodes\mywork\best-practices\docs\patterns\backend-api\config-layered-loading.cn.md`

TermBridge-go 需要从原先的 `.termbridge.*.yaml` 配置文件名演进到更清晰的 `configs/config*.yaml` 配置目录模型，同时保留 `.env` / OS env 作为敏感值与部署覆盖入口。

## Goal

1. 默认配置文件改为 `configs/config.yaml`，作为提交到版本库的只读 baseline。
2. 运行环境差异配置写入 `configs/config.<env>.yaml`，环境名只允许从 OS env `TERMBRIDGE_ENV` 读取。
3. 未指定环境名时加载顺序为：

   ```text
   configs/config.yaml
     < .env
     < OS env
   ```

4. 指定环境名时加载顺序为：

   ```text
   configs/config.yaml
     < configs/config.<env>.yaml
     < .env.<env>
     < OS env
   ```

5. 指定环境名时只加载 `.env.<env>`；缺失允许继续，不回退加载 `.env`，避免环境切换时混入默认本地敏感覆盖。
6. 新增 `Config.Base` 或等价字段，表示只经过 YAML 层合并后的 baseline，不受 `.env` 与 OS env 污染。
7. 配置合法性必须在初始化阶段集中校验；业务代码只消费已合法的 typed config，不在业务路径、client 构造或请求处理时散落必填项、格式、范围或跨字段一致性校验。
8. `.env` / `.env.<env>` 不覆盖已有 OS env。
9. 不认识的 YAML key 和环境变量直接忽略，不实现 unknown key 拒绝逻辑。
10. 业务运行中变化或首次启动自动生成的本机状态配置，特别是 `jwt.secret_key`、`agent.device_id`、`agent.device_name`，不得写回 `configs/config.yaml`。
11. 未指定环境名时，兼容写入当前生效 `.env`；指定 `TERMBRIDGE_ENV=<env>` 时，生成配置写入 `configs/config.<env>.yaml`，不写入 `.env.<env>`。
12. 加载配置文件时必须通过现有日志系统逐个打印实际加载的文件；只记录本轮真实读取的 `configs/config.yaml`、可选 `configs/config.<env>.yaml`、可选 `.env` / `.env.<env>`，不能把启动过程中刚生成但未读取的文件伪装成已加载。
13. 同步 README、默认配置注释、`.env.example`、Docker 构建输入和配置测试。

## Non-goal

1. 不保留 `.termbridge.default.yaml` / `.termbridge.yaml` 作为新的正式配置入口。
2. 不允许 `.env` 或 `.env.<env>` 覆盖已有 OS env。
3. 不新增旧配置 key 兼容层或旧文件迁移逻辑。
4. 不实现 unknown / rejectUnknown 配置拒绝逻辑。
5. 不让业务逻辑修改默认配置 baseline。
6. 不执行 git 写操作。

## User scenarios

1. 作为本地开发者，我不设置环境名时，可以使用 `configs/config.yaml` 与 `.env` 获得默认本地配置。
2. 作为维护者，我可以设置 OS env `TERMBRIDGE_ENV=cloud`，让程序合并 `configs/config.cloud.yaml` 并加载 `.env.cloud`。
3. 作为部署者，我可以用 OS env 覆盖 `.env.<env>` 中的值，保证 Docker/systemd/Kubernetes/CI/shell 注入值优先。
4. 作为配置展示功能的后续开发者，我可以通过 `Config.Base` 获取 YAML baseline，而不是被 `.env` 或 OS env 覆盖后的运行时值。
5. 作为本地用户，我不设置环境名时首次启动 `termbridge serve`，自动生成的 JWT secret 与 device identity 会写入 `.env`，不会污染默认配置文件。
6. 作为 portable zip 用户，启动脚本设置 `TERMBRIDGE_ENV=local` 后，程序加载 `.env.local` 作为用户可编辑运行配置；首次启动自动生成的 JWT secret 与 device identity 写入 `configs/config.local.yaml`，不改写 `.env.local`。

## Acceptance

- [ ] 默认配置文件为 `configs/config.yaml`，并作为只读 baseline 提交。
- [ ] 未指定环境名时，配置优先级为 `configs/config.yaml < .env < OS env`。
- [ ] 指定 `TERMBRIDGE_ENV=<env>` 时，可选加载 `configs/config.<env>.yaml`。
- [ ] 指定 `TERMBRIDGE_ENV=<env>` 时，加载 `.env.<env>`；缺失允许继续，格式错误返回 config error。
- [ ] 环境名只从 OS env 读取，不通过 `.env` / `.env.<env>` 决定。
- [ ] `.env` / `.env.<env>` 使用不覆盖已有 OS env 的加载策略。
- [ ] `Config.Base` 表示 YAML-only baseline，不受 `.env` 与 OS env 影响。
- [ ] 运行时 `Config` 仍包含 `.env` 与 OS env 的最终覆盖值。
- [ ] unknown YAML key 和 unknown 环境变量被忽略，不触发启动失败。
- [ ] `jwt.secret_key`、`agent.device_id` / `agent.device_name` 缺失时，不写回 `configs/config.yaml`。
- [ ] 未指定环境名时，生成配置兼容写入 `.env`。
- [ ] 指定 `TERMBRIDGE_ENV=<env>` 时，生成配置写入 `configs/config.<env>.yaml`，不写入 `.env.<env>`；portable local 模式对应 `configs/config.local.yaml`，但实现必须按 `<env>` 泛化，不能写死 local。
- [ ] `.env.local` 等环境 env 文件只作为用户可编辑覆盖入口，启动生成项不得写入其中。
- [ ] 配置加载日志进入现有日志系统，并按实际读取顺序逐行输出“加载配置文件”，不使用裸 stdout/stderr，也不记录未读取但启动时生成的文件。
- [ ] Google OAuth、Resend、cloud mode 等配置完整性在 `config.Load` 初始化阶段集中校验；业务层只判断功能启用状态或消费 typed config。
- [ ] 测试覆盖默认路径、env config 覆盖、`.env.<env>` 覆盖、OS env 优先、`Config.Base` 隔离、缺失 env config/env file、非法 env file、unknown key 忽略、无环境名生成写入 `.env`、有环境名生成写入 `configs/config.<env>.yaml` 和配置加载日志回调。
- [ ] README、`configs/config.yaml`、`.env.example`、Dockerfile、Dockerfile.cn 更新配置优先级与环境名说明。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- 用户已要求“轻量模式 直接开始”，视为 Requirement / 需求已接受并进入 Implementation / 实现。
- 用户指定配置文件名方案为 `configs/config.yaml`。
- 环境名采用 `TERMBRIDGE_ENV`，不放入 Viper 配置 key，避免从 `.env` 反向决定加载哪个 `.env.<env>`。
- 指定环境名时加载 `.env.<env>` 而不是同时加载 `.env`，避免环境敏感覆盖混杂。
- 用户最初要求业务逻辑不修改默认配置，业务中变化的配置写入生效的 `.env` 文件；后续在 portable package 场景中修正为：无环境名时兼容写入 `.env`，有环境名时生成项写入 `configs/config.<env>.yaml`，不写入 `.env.<env>`。
- 用户明确要求不实现 `rejectUnknown`；不认识的环境变量和配置直接忽略。
- 用户明确要求配置合法性在初始化阶段校验，不要在业务里到处散落校验；参考最佳实践文档也应补充该规则。
- 用户在 portable package 场景中进一步修正：启动脚本指定 `TERMBRIDGE_ENV=local`，运行配置文件改为 `.env.local`，启动生成的 `TERMBRIDGE_JWT__SECRET_KEY`、`TERMBRIDGE_AGENT__DEVICE_ID`、`TERMBRIDGE_AGENT__DEVICE_NAME` 不写入 `.env.local`，而是写入 `configs/config.<env>.yaml`；local 环境下即 `configs/config.local.yaml`。
- 用户特别指出生成配置写入路径必须按当前 `<env>` 泛化，不要写死成 `configs/config.local.yaml`。
- 用户要求配置加载日志使用现有日志系统，并且“加载一个打印一次”；不得用裸 stderr/stdout，也不得用事后文件存在性误判加载文件。

## Risk

1. 新增 `Config.Base` 会扩大 `Config` 结构；需要避免递归结构导致比较、打印或赋值问题。
2. 路径规范化会让 `Base` 中的相对路径也按 cwd 展开，应与当前运行时行为保持一致。
3. `TERMBRIDGE_ENV` 是 OS env 入口，用户 shell 中残留值可能改变配置加载文件；文档需提示。
4. 旧 `.termbridge.default.yaml` / `.termbridge.yaml` 不再作为正式入口后，历史本地配置需要用户手动迁移到 `configs/config.<env>.yaml` 或 `.env`。
5. 有环境名时首次启动生成的 `configs/config.<env>.yaml` 在本轮 `Load` 之后才写入；同一进程内使用内存中的生成值，下一次启动才会作为 env config 被加载。日志必须反映“本轮实际加载”，避免把新生成文件误报为已加载。

## User review notes

- 用户原始指令：`暂存已经提交，采纳建议`。
- 用户随后指定：`/specflow 轻量模式 直接开始`。
- 用户补充：`配置合法性在初始化阶段校验，不要在业务里到处散落校验 也检查了一下参考文档有没有这类描述，没有就加上`。
- 用户补充并确认配置文件名方案：`configs/config.yaml`。
- 用户补充：`注意业务逻辑不修改默认配置，业务中变化的配置写在生效的 .env 文件里`。
- 用户补充：`不实现 rejectUnknown 逻辑，不认识的环境变量和配置直接忽略就好`。
