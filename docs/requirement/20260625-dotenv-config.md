# Dotenv Config Requirement
最后修改时间: 2026-06-25 13:05:24

## Review status

Accepted

## Flow mode / Stage

轻量模式 / light；需求 / Requirement 已接受，当前进入实现 / Implementation。

## Background

用户要求参考 `D:\SourceCodes\mywork\pomelo-orbit\backend-go` 的配置加载实现，为 TermBridge-go 增加 `.env` 支持，策略保持一致，并明确“不向后兼容”。

参考实现的核心行为是：

```text
config.defaults.yaml
  < config.local.yaml
  < .env
  < OS env
```

其中 `.env` 通过 `godotenv.Load(path)` 写入进程环境，再由 Viper `BindEnv` 将指定配置 key 映射到环境变量；因为使用 `Load` 而不是 `Overload`，`.env` 不覆盖 Docker / systemd / Kubernetes / CI / shell 已注入的同名 OS env。

TermBridge-go 当前配置加载只包含：

```text
.termbridge.default.yaml
  < .termbridge.yaml
```

当前实现没有 `.env`、没有 Viper env binding、没有 OS env 覆盖配置。配置层已存在 unknown key 拒绝机制，应继续保留。

用户补充澄清：需要检查实现中是否存在“配置文件相关”的直接 env 文件或系统环境变量读取；配置值必须基于 Viper 提供，不应分散通过 `os.Getenv` 读取。读取计算机名、当前用户、子进程继承环境变量等非配置来源用途不属于这个限制。

## Goal

1. 为 TermBridge-go 配置加载增加 `.env` 支持。
2. 配置优先级调整为：

   ```text
   .termbridge.default.yaml
     < .termbridge.yaml
     < .env
     < OS env
   ```

3. `.env` 文件位于有效工作目录 `cwd` 下，即与 `.termbridge.yaml` 同级。
4. `.env` 不存在时不报错；`.env` 存在但格式非法时加载失败并返回 config error。
5. `.env` 不覆盖已存在的 OS env；OS env 仍具有最高优先级。
6. 所有“配置项来自环境变量”的读取都必须通过 Viper binding 进入统一 config loader，不允许在业务代码或配置解析逻辑中分散 `os.Getenv` 读取配置。
7. 继续保留 `.termbridge.default.yaml < .termbridge.yaml` 的 YAML merge 语义和 unknown key 校验。
8. 不为旧配置 key 或旧配置路径做向后兼容迁移。

## Non-goal

1. 不恢复已删除的旧配置结构，例如 `serve.*`、`web.error.debug`、`web.error`、`agent.server_url`、`runtime.env.denylist`。
2. 不改变 `.termbridge/device.json` 的职责；它仍只保存本地临时登录凭据 `auth.username/password`。
3. 不把 agent identity 从 `.termbridge.yaml` 移回 `.termbridge/device.json`。
4. 不改变 terminal runtime 对子进程环境变量的继承行为；`os.Environ()` 用于启动子进程不是配置读取。
5. 不禁止读取机器名、当前用户等非配置默认值来源；例如默认 device name、默认 username 的生成仍可使用系统 API。
6. 不引入环境变量 denylist / allowlist runtime feature。
7. 不执行 git 写操作。

## User scenarios

1. 作为本地开发者，我可以在项目根目录写 `.env` 覆盖本地调试配置，而不必修改 `.termbridge.yaml`。
2. 作为部署者，我可以通过 OS env 覆盖 `.env` 中的同名配置，保证 Docker / systemd / Kubernetes / CI 注入值优先。
3. 作为维护者，我可以在一个地方看到所有支持 env 覆盖的配置 key，而不是在代码中搜索分散的 `os.Getenv`。
4. 作为调试者，如果 `.env` 格式写错，启动应明确失败，而不是静默忽略导致配置不符合预期。

## Acceptance

- [ ] 配置加载优先级为 `.termbridge.default.yaml < .termbridge.yaml < .env < OS env`。
- [ ] `.env` 路径基于 `config.Options.Cwd` 解析，和 `.termbridge.yaml` 同目录。
- [ ] `.env` 不存在时 `config.Load` 正常继续。
- [ ] `.env` 存在但格式非法时 `config.Load` 返回 config error，错误上下文能指向 env file 读取失败。
- [ ] `.env` 使用不覆盖已有 OS env 的加载策略；OS env 对同名配置永远优先于 `.env`。
- [ ] Env binding 由 Viper 统一完成，配置项不通过分散的 `os.Getenv` 读取。
- [ ] 支持当前配置模型中的可配置 key，至少包括：
  - `log.level`
  - `log.format`
  - `log.dir`
  - `log.http.request_body_limit`
  - `log.http.response_body_limit`
  - `history.max_lines`
  - `history.max_bytes`
  - `history.max_line_bytes`
  - `runtime.state_dir`
  - `gate.listen_url`
  - `gate.browser.allowed_origins`
  - `gate.api.expose_errors`
  - `agent.connect_url`
  - `agent.device_id`
  - `agent.device_name`
- [ ] Env 变量命名规则稳定、可文档化；建议采用 `TERMBRIDGE_` 前缀，并将配置 key 中的 `.` 转换为 `__`，例如：
  - `gate.listen_url` -> `TERMBRIDGE_GATE__LISTEN_URL`
  - `agent.connect_url` -> `TERMBRIDGE_AGENT__CONNECT_URL`
  - `log.http.request_body_limit` -> `TERMBRIDGE_LOG__HTTP__REQUEST_BODY_LIMIT`
- [ ] 列表型配置 `gate.browser.allowed_origins` 使用逗号分隔 env 字符串表达，并由测试覆盖，例如：`TERMBRIDGE_GATE__BROWSER__ALLOWED_ORIGINS=http://localhost:9011,http://127.0.0.1:9011`。
- [ ] 继续拒绝 YAML 中 unknown config key；env binding 不应绕过当前配置模型引入未声明配置项。
- [ ] 不向后兼容旧 key；旧 env 名或旧 YAML key 不被支持，出现旧 YAML key 时仍应失败。
- [ ] 测试覆盖：默认配置、local YAML 覆盖、`.env` 覆盖 YAML、OS env 覆盖 `.env`、缺失 `.env`、非法 `.env`、至少一个 bool/int/string/list env override。
- [ ] 新增 `.env.example`，说明配置优先级、env 命名规则和常用配置项。
- [ ] README 和默认示例文档说明 `.env` 优先级和 env 命名规则。
- [ ] 不执行 git 写操作。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- 使用轻量模式 / light：变更范围集中在配置加载、测试和文档，但仍需要记录需求和完整验证。
- 参考 `pomelo-orbit/backend-go` 的优先级策略：default YAML < local YAML < `.env` < OS env。
- `.env` 采用不覆盖 OS env 的语义；也就是应使用与 `godotenv.Load` 等价的行为，而不是 `Overload`。
- Env 变量命名采用 `TERMBRIDGE_` 前缀，并将配置 key 中的 `.` 转换为 `__`。
- `gate.browser.allowed_origins` 采用逗号分隔 env 字符串表达，例如：`TERMBRIDGE_GATE__BROWSER__ALLOWED_ORIGINS=http://localhost:9011,http://127.0.0.1:9011`。
- 新增 `.env.example`，用于说明优先级、命名规则和常用配置项。
- 不向后兼容旧配置 key；当前配置模型是唯一目标。
- 配置来源必须集中经过 Viper；不要在业务代码中通过 `os.Getenv` 读取配置值。
- 非配置用途的系统读取不受限制：机器名、当前用户、子进程环境继承等仍可保留。

## Risk

1. Env override 会让配置来源从纯 YAML 变为 YAML + env，调试时需要清楚展示优先级，否则容易误判 `.termbridge.yaml` 未生效。
2. Viper 对 slice、bool、int 的 env 解析行为需要通过测试锁定，尤其是 `gate.browser.allowed_origins`。
3. OS env 最高优先级可能导致用户 shell 中残留变量影响本地启动；README / `.env.example` 需要明确说明。
4. 如果 env binding key 列表和 allowed YAML key 列表分离维护，后续新增配置容易遗漏其中一边；实现时应尽量复用同一份 key 列表或至少用测试防漂移。
5. `.env` 中可能包含敏感信息；日志和错误输出不能打印完整 env value。
6. 由于不向后兼容旧 key，历史文档中的旧配置示例不应作为当前行为依据；当前 README / 默认配置 / 测试是准绳。

## User review notes

- 用户原始指令：`参考该实现，添加对 .env 的支持，策略相同，不向后兼容 /specflow light 模式`。
- 已对比 `pomelo-orbit/backend-go`：其 `.env` 行为为 `config.defaults.yaml < config.local.yaml < .env < OS env`，`.env` 通过 `godotenv.Load` 加载，不覆盖已有 OS env，再由 Viper `BindEnv` 进入配置解析。
- 用户补充：`同时检查实现里有没有直接读 env 文件或系统环境变量的，我们要基于 viper 提供配置`。
- 用户进一步澄清：读计算机名、读用户等用法没有问题；限制只针对配置文件里的配置来源，配置值要基于 Viper。
- 用户确认：`gate.browser.allowed_origins` 采用逗号分隔 env 字符串表达。
- 用户确认：新增 `.env.example`。
