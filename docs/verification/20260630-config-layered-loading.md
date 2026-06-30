# Config Layered Loading Verification
最后修改时间: 2026-06-30 15:57:32

## Review status

Accepted

## Flow mode / Stage

轻量模式 / light；验证 / Verification。

## Requirement alignment

依据 `docs/requirement/20260630-config-layered-loading.md` 核对：

| Requirement / Acceptance | Verification result |
| --- | --- |
| 默认配置文件为 `configs/config.yaml`，作为只读 baseline 提交 | 已实现。`configs/config.yaml` 为新的默认配置文件；旧 `.termbridge.default.yaml` 在当前工作区中删除。 |
| 未指定环境名时优先级为 `configs/config.yaml < .env < OS env` | 已实现并由 `internal/infrastructure/config/config_test.go` 覆盖默认加载、`.env` 覆盖和 OS env 优先级。 |
| 指定 `TERMBRIDGE_ENV=<env>` 时加载 `configs/config.<env>.yaml` | 已实现并由 config tests 覆盖 env config merge。 |
| 指定环境名时加载 `.env.<env>`，缺失允许继续，格式错误返回 config error | 已实现并由 config tests 覆盖 `.env.<env>`、missing env file、invalid env file。 |
| 环境名只从 OS env 读取，不通过 `.env` / `.env.<env>` 决定 | 已实现并由 config tests 覆盖 `TERMBRIDGE_ENV` OS-only。 |
| `.env` / `.env.<env>` 不覆盖已有 OS env | 已实现，使用 `gotenv.Load` 加载并保留 OS env 优先级；由 config tests 覆盖。 |
| `Config.Base` 表示 YAML-only baseline，不受 `.env` 与 OS env 影响 | 已实现并由 config tests 覆盖 base isolation。 |
| 运行时 `Config` 包含 `.env` 与 OS env 最终覆盖值 | 已实现并由 config tests 覆盖运行时覆盖值。 |
| unknown YAML key 和 unknown 环境变量被忽略 | 已实现；未实现 rejectUnknown；由 `TestLoadIgnoresUnknownConfigAndEnvironmentKeys` 覆盖。 |
| `agent.device_id` / `agent.device_name` 写入当前生效 `.env` 或 `.env.<env>`，不写回 baseline | 已实现并由 identity write / env-specific write / existing env upsert tests 覆盖。 |
| Google OAuth、Resend、cloud mode 配置完整性在 `config.Load` 初始化阶段集中校验 | 已实现；业务层保留 enablement 判断，不散落完整性校验；由 integration/cloud mode config tests 覆盖。 |
| 测试覆盖加载路径、覆盖顺序、base 隔离、缺失文件、非法 env、unknown key、active env 写入 | 已覆盖。目标包测试与全量 backend Go tests 通过。 |
| README、`configs/config.yaml`、`.env.example`、Dockerfile、Dockerfile.cn 更新说明 | 已更新配置优先级、环境名、JWT secret key 约束与 Docker copy source。 |

## Spec alignment

不适用。当前流程为轻量模式 / light，本需求未创建独立 Spec 文档；按 Requirement 核对。

## Plan alignment

不适用。当前流程为轻量模式 / light，本需求未创建独立 Plan 文档；按 Requirement 与实际实现核对。

## Actual diff summary

本次实际改动主要分为四组：

1. 配置文件入口与交付文件
   - 删除旧正式入口 `.termbridge.default.yaml`。
   - 新增 `configs/config.yaml` 作为提交到版本库的 baseline。
   - 更新 `.gitignore` / `.dockerignore`，保留 baseline，忽略环境专属配置与本地 `.env`。
   - 更新 `Dockerfile` / `Dockerfile.cn`，复制 `configs/`。

2. 配置加载与初始化校验
   - `internal/infrastructure/config/config.go` 改为分层加载 `configs/config.yaml`、可选 `configs/config.<env>.yaml`、`.env` / `.env.<env>` 与 OS env。
   - `TERMBRIDGE_ENV` 只从 OS env 决定，不允许 `.env` 反向改变加载目标。
   - 新增 `Config.Base` 保存 YAML-only baseline。
   - 取消 unknown key reject 行为；未知 YAML key / env var 被忽略。
   - 集中校验 Google OAuth、Resend、cloud mode 与 JWT key 约束。
   - `EnsureLocalIdentity` 将运行中生成的 agent identity upsert 到当前生效 env file。

3. JWT key hardening（用户后续要求的同批安全约束）
   - 新增 `internal/infrastructure/security/key.go` / `key_test.go`，严格解析 base64 编码的固定长度 key。
   - `jwt.secret_key` 初始化阶段要求 base64 解码后为 32 bytes；普通明文 secret 被拒绝。
   - `gatewayapi/auth.TokenService` 改为持有 decoded `[]byte` key，仍使用 HS256 / HMAC-SHA256。
   - `gatewayapi.Config.JWTSecret` 改为 `[]byte`，避免在业务路径继续传递原始配置字符串。

4. 文档、CLI 与测试
   - README、`.env.example`、`configs/config.yaml` 注释更新配置优先级、环境名规则和 JWT key 生成约束。
   - CLI help 更新为 `configs/config.yaml` / `TERMBRIDGE_ENV` 模型。
   - config/app/cli/gateway/auth tests 更新到新配置入口与 strict JWT key 模型。

## Expected vs actual changed files

| File / path | Expected? | Notes |
| --- | --- | --- |
| `.dockerignore` | Yes | 忽略 `.env*` 与环境专属 config，保留 baseline。 |
| `.env.example` | Yes | 更新分层优先级、env 命名、JWT 32-byte base64 key 说明。 |
| `.gitignore` | Yes | 忽略环境专属 config 与本地 env 文件。 |
| `.termbridge.default.yaml` | Yes | 旧正式配置入口删除。 |
| `configs/config.yaml` | Yes | 新 baseline 配置文件。 |
| `Dockerfile`, `Dockerfile.cn` | Yes | 镜像构建复制 `configs/`。 |
| `README.md` | Yes | 更新配置模型与 JWT key 约束说明。 |
| `docs/requirement/20260630-config-layered-loading.md` | Yes | 轻量模式 requirement 文档。 |
| `docs/verification/20260630-config-layered-loading.md` | Yes | 本验证文档。 |
| `internal/app/app.go`, `internal/app/app_test.go` | Yes | app wiring 使用新配置字段、decoded JWT key 与新测试 fixture。 |
| `internal/application/auth/service.go` | Yes | Google auth 业务层只判断 enablement。 |
| `internal/infrastructure/config/config.go`, `config_test.go` | Yes | 分层加载、集中校验、identity env 写入与测试。 |
| `internal/infrastructure/email/resend.go` | Yes | Resend 业务层只判断 enablement。 |
| `internal/infrastructure/security/` | Expanded scope | 用户后续要求参照 Pomelo strict key 约束，用途不变；作为同批安全约束纳入验证。 |
| `internal/transport/cli/cli.go`, `cli_test.go` | Yes | CLI 文案和测试 fixture 更新。 |
| `internal/transport/http/gatewayapi/auth/*` | Expanded scope | strict JWT key hardening；JWT 用途保持 HS256。 |
| `internal/transport/http/gatewayapi/server*.go`, `cloud_binding_test.go`, `tunnel_test.go` | Expanded scope | `JWTSecret` 从 string 改为 decoded `[]byte` 后同步调用点与测试。 |

最新 `git status --short` 未显示此前提到的 `Taskfile.yml` / `scripts/run.py` 为当前未提交改动；本验证未将它们纳入交付范围。

## Acceptance checklist

- [x] 默认配置文件改为 `configs/config.yaml`。
- [x] 未指定环境名时按 `configs/config.yaml < .env < OS env` 加载。
- [x] 指定 `TERMBRIDGE_ENV=<env>` 时按 `configs/config.yaml < configs/config.<env>.yaml < .env.<env> < OS env` 加载。
- [x] `TERMBRIDGE_ENV` 只从 OS env 读取。
- [x] `.env` / `.env.<env>` 不覆盖已有 OS env。
- [x] `Config.Base` 提供 YAML-only baseline。
- [x] unknown YAML key / env var 被忽略。
- [x] 运行中生成的 agent identity 写入 active env file。
- [x] 配置合法性集中在初始化阶段校验。
- [x] 文档、示例与 Docker 构建输入同步。
- [x] 用户后续要求的 strict JWT key 约束已纳入：只接受 base64 编码 32-byte key，JWT 用途不变。

## Command results

| Command | Result |
| --- | --- |
| `go test ./internal/infrastructure/security ./internal/infrastructure/config ./internal/transport/http/gatewayapi/auth ./internal/transport/http/gatewayapi ./internal/app ./internal/transport/cli` | Passed |
| `go test ./cmd/... ./internal/...` | Passed |
| `go vet ./cmd/... ./internal/...` | Passed，命令无输出 |

## Scope deviation

- `internal/infrastructure/security/` 与 JWT auth/gateway 相关改动不在最初 config layered loading requirement 中，但来自用户后续明确指令“使用类似的实现和约束，用途不变”，用于对齐 Pomelo commit `d30598ec` 的 strict key 思路；本验证视为同批交付的安全 hardening 扩展。
- 历史过程文档中仍可能存在 `.termbridge.default.yaml` 或 `jwt.secret_key=<secret>` 等历史描述；未主动迁移历史文档，避免篡改旧阶段记录。
- 未执行前端 `yarn` 系列检查，因为本次后续变更集中在后端配置、文档、Dockerfile 与 Go 测试；若需要发布级全量验证，可再运行 `task check`。

## Risks

1. `jwt.secret_key` 现在拒绝明文 secret；现有本地或部署环境需要将旧值迁移为 base64 编码的 32-byte 随机 key。
2. 旧 `.termbridge.default.yaml` / `.termbridge.yaml` 不再是正式入口；已有本地配置需要手动迁移到 `configs/config.<env>.yaml` 或 `.env` / `.env.<env>`。
3. `TERMBRIDGE_ENV` 来自 OS env，shell、CI 或 systemd 中残留值会改变配置加载目标；README 和 `.env.example` 已说明。
4. `go test` 输出为 cached，表示当前 Go 工具链判断相关包未变更且缓存有效；已额外运行 `go vet`。

## Incomplete items

- 未运行前端 typecheck/lint/test。
- 未执行 Docker build。
- 未执行 git add / commit / push，符合用户“未经许可禁止 git 写操作”的限制。

## Conclusion

验证通过。当前实现与轻量模式 Requirement 对齐，并纳入用户后续明确要求的 strict JWT key hardening。后端相关 Go tests 与 `go vet` 均通过。发布或提交前建议确认是否需要额外运行 `task check` / Docker build，以及确认旧环境中的 `TERMBRIDGE_JWT__SECRET_KEY` 已迁移为 base64 编码 32-byte key。
