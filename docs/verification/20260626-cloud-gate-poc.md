# Cloud Gate 最小 PoC 验证
最后修改时间: 2026-06-26 20:56:13

Review status: Accepted

## Flow mode / Stage

严格模式 / strict；验证 / Verification。

依据：

- Requirement: `docs/requirement/20260626-cloud-gate-poc.md`
- Spec: `docs/spec/20260626-cloud-gate-poc.md`
- Plan: `docs/plan/20260626-cloud-gate-poc.md`

## Verification scope

本次 Verification 按用户在实现过程中的最新修正收口：

1. 本轮以 Docker 镜像内前后端一体为目标：Go server 提供 `/api` 与 `web/dist` 静态资源。
2. 本地开发继续前后端分离：Go backend 默认 `http://127.0.0.1:9010`，Vite dev server 默认 `http://127.0.0.1:9011`，Vite dev proxy 固定将 `/api` 转发到 `http://127.0.0.1:9010`。
3. 暂时不要求连接其他远程 Agent；跨网络 Cloud Gate + 外部 Local Agent 真实路径不作为本轮自动验证完成项。
4. 用户已明确不需要容器运行 smoke；因此只验证 Docker image build，不启动容器检查 `/api/health` 或 `/sessions`。
5. 用户已要求撤销新增日志脱敏实现；本轮不新增 `safelog` / query redaction。
6. 用户已要求删除 `legacyAgent` 兼容实现；`EnsureLocalIdentity` 不再读取历史 `runtime.state_dir/device.json`。

## Requirement alignment

| Requirement area | Verification result | Notes |
|---|---:|---|
| Dockerfile 构建 web 资源与 Go binary | 通过 | 默认 Dockerfile build 已完成。 |
| Browser `/sessions` 不依赖 Vite dev server | 通过 | 新增 `web.static_dir`，Go HTTP server 可提供 SPA fallback 与 assets。 |
| `/api` 不被 SPA fallback 吞掉 | 通过 | `internal/transport/http/server` 测试覆盖 `/api/health` 与 `/api/missing`。 |
| PoC 固定 `admin/admin` | 通过 | config 默认 auth 固定为 `DefaultAuthUsername/DefaultAuthPassword`。 |
| 本地 `serve` 生成/读取 device id/name 并上报 | 部分通过 | `EnsureLocalIdentity` 生成/写入 `agent.device_id/device_name`；Agent hello 既有路径继续上报。未做真实跨网络 Agent 验证。 |
| 远程 session create / attach / management | 部分通过 | 既有 Gateway API / Agent tunnel 自动化测试通过；用户最新收口为暂不连接其他远程，因此未做跨网络远程路径验证。 |
| Ctrl+C input path | 未手工验证 | 自动化未覆盖真实 PTY 前台进程 Ctrl+C；用户后续如需要可手工验证。 |
| Claude Code / Codex TUI | 未验证 | 按需求由用户自行验证。 |
| 日志不泄露 token/password | 未作为本轮新增实现验证 | 用户明确撤销新增日志脱敏，不再作为本轮实现项；历史文档中相关安全要求应在后续安全需求中单独处理。 |

## Spec alignment

| Spec decision | Verification result | Notes |
|---|---:|---|
| 同源 HTTPS + 外部 TLS 终止 | 部分通过 | Docker/runtime 侧保持内部 HTTP；未做实际 HTTPS reverse proxy 验证。 |
| `gate.listen_url` 与 public URL 分离 | 通过 | Dockerfile 使用内部 listen URL；README / `.env.example` 保持 external HTTPS 示例。 |
| 复用现有 Browser / Agent tunnel 认证 | 通过 | 本轮固定 `admin/admin`，未新增 agent secret。 |
| Web 静态资源由容器内 TermBridge 服务提供 | 通过 | Go server 支持 `web.static_dir`，Dockerfile 复制 `web/dist`。 |
| 远程 session 管理纳入 PoC | 部分通过 | 自动化覆盖现有 tunnel/session/terminal 能力；真实远程 Agent 路径按用户最新口径暂不验证。 |
| Ctrl+C 作为 interrupt 定义 | 未手工验证 | 自动化未覆盖真实 Ctrl+C 前台进程。 |
| Claude Code / Codex TUI 用户自行验证 | 未验证 | 等待用户反馈。 |

## Plan alignment

| Plan step | Verification result | Notes |
|---|---:|---|
| Step 1 固化 PoC 认证并保留 device 上报 | 通过 | 固定 `admin/admin`；删除随机 auth 与 legacy `device.json` 读取；保留 agent identity 写回。 |
| Step 2 生产 Web 静态资源由 TermBridge 服务提供 | 通过 | `web.static_dir` + static handler + SPA fallback 测试通过。 |
| Step 3 route unavailable terminal 错误 | 通过 | Agent disconnect 向 Browser 发送结构化 terminal error。 |
| Step 4 Dockerfile / `.dockerignore` | 通过 | Docker image build 通过；未按用户要求启动容器 smoke。 |
| Step 5 最小配置示例，不新增部署说明文档 | 通过 | 未新增 deploy 文档；`.env.example` / README 更新。 |
| Step 6 远程 session 管理与 terminal 链路验证 | 部分通过 | 自动化链路通过；真实远程 / 跨网络 / 容器运行 smoke 未验证，符合用户最新收口。 |

## Actual diff summary

当前工作区包含以下变更类型：

1. SpecFlow 文档
   - 新增 `docs/requirement/20260626-cloud-gate-poc.md`
   - 新增 `docs/spec/20260626-cloud-gate-poc.md`
   - 新增 `docs/plan/20260626-cloud-gate-poc.md`
   - 新增本文件 `docs/verification/20260626-cloud-gate-poc.md`
2. 容器交付
   - 新增 `Dockerfile`
   - 新增 `Dockerfile.cn`
   - 新增 `.dockerignore`
3. 配置与说明
   - 修改 `.termbridge.default.yaml`
   - 修改 `.env.example`
   - 修改 `README.md`
   - 修改 `justfile`
4. Config / app bootstrap
   - 修改 `internal/infrastructure/config/config.go`
   - 修改 `internal/infrastructure/config/config_test.go`
   - 修改 `internal/app/app.go`
   - 修改 `internal/app/app_test.go`
5. HTTP static serving
   - 修改 `internal/transport/http/server/server.go`
   - 修改 `internal/transport/http/server/server_test.go`
6. Terminal route unavailable
   - 修改 `internal/transport/http/gatewayapi/route.go`
   - 修改 `internal/transport/http/gatewayapi/terminal_test.go`
7. Web dev/build
   - 修改 `web/vite.config.ts`
   - 修改 `web/src/features/sessions/useTerminalSocket.test.ts`
8. 需用户注意的额外 diff
   - `web/src/components/workspace/WorkspaceSessionSidebar.vue`
   - `web/src/store/workspaceSessions.ts`

## Expected vs actual changed files

| File / area | Expected by plan | Actually changed | Result |
|---|---:|---:|---|
| Requirement / Spec / Plan docs | Yes | Yes | Match |
| Verification doc | Yes | Yes | Match |
| `Dockerfile` / `.dockerignore` | Yes | Yes | Match |
| `Dockerfile.cn` | No | Yes | Extra; appears to be China mirror variant. Review before committing. |
| `.termbridge.default.yaml` / `.env.example` | Yes | Yes | Match |
| `README.md` | Yes | Yes | Match |
| `justfile` | No | Yes | Acceptable follow-up for `127.0.0.1` consistency. |
| `internal/infrastructure/config/*` | Yes | Yes | Match |
| `internal/app/*` | Partly | Yes | App passes `web.static_dir`; tests updated for `127.0.0.1`. |
| `internal/transport/http/server/*` | Yes | Yes | Match |
| `internal/transport/http/gatewayapi/route.go` / `terminal_test.go` | Yes | Yes | Match |
| requestlog / gateway error logging | Earlier expected, later removed | No current code changes | Correct per user request “不要加戏”。 |
| `web/vite.config.ts` | Yes | Yes | Match; now fixed dev proxy to `127.0.0.1:9010`. |
| `web/src/components/workspace/WorkspaceSessionSidebar.vue` | No | Yes | Extra / unrelated to Cloud Gate PoC; should be reviewed or split. |
| `web/src/store/workspaceSessions.ts` | No | Yes | Extra formatting-only diff; should be reviewed or split. |

## Acceptance checklist

### Container / static serving

- [x] Dockerfile builds web resources and Go binary.
- [x] Docker image build completes with default `Dockerfile`.
- [x] Go server supports serving `web/dist` through `web.static_dir`.
- [x] SPA fallback serves `/`, `/sessions`, and other non-file routes.
- [x] `/api` remains API-only and is not swallowed by SPA fallback.
- [x] Production `yarn build` does not require `web/.env` or `VITE_TERMBRIDGE_BACKEND`.
- [ ] Container runtime smoke (`/api/health`, `/sessions`) not run; user said no need to verify container runtime.

### Local development

- [x] Local dev addresses are unified to `127.0.0.1` in current implementation files.
- [x] Vite dev server binds `127.0.0.1:9011`.
- [x] Vite dev proxy forwards `/api` and WebSocket upgrade to `http://127.0.0.1:9010`.
- [x] README and default allowed origin use `http://127.0.0.1:9011`.

### Auth / device identity

- [x] Default Browser login and Agent tunnel credential are fixed to `admin/admin`.
- [x] `EnsureLocalIdentity` no longer generates random auth password.
- [x] `EnsureLocalIdentity` no longer reads legacy `device.json` auth or agent identity.
- [x] Missing `agent.device_id` / `agent.device_name` are generated and written to `.termbridge.yaml`.
- [x] Agent hello path continues to use configured/generated device id/name.

### Terminal / route unavailable

- [x] Agent disconnect sends structured terminal protocol error.
- [x] Error code is `device_disconnected`.
- [x] Existing route unavailable API returns structured `device_offline` error.

### Session management

- [x] Existing Gateway API / Agent tunnel automated tests pass.
- [ ] Real cross-network Cloud Gate + separate Local Agent path not verified; user latest scope says no remote connection requirement for now.
- [ ] Claude Code / Codex real TUI not verified; user-owned verification.

## Test results

| Command | Result | Notes |
|---|---:|---|
| `go test ./cmd/... ./internal/...` | Pass | Final full Go test run passed. |
| `go vet ./cmd/... ./internal/...` | Pass | No output. |
| `cd web && yarn test` | Pass | 8 test files, 36 tests passed. |
| `cd web && yarn typecheck` | Pass | `vue-tsc --noEmit` passed. |
| `cd web && yarn lint` | Pass | `eslint .` passed. |
| `cd web && yarn build` | Pass with warnings | Rolldown `INVALID_ANNOTATION` warnings from `@vueuse/core`; chunk size warning for `SessionsView`; build succeeded. |
| `docker build -t termbridge-cloud-gate-poc .` | Pass | Default Docker image build completed. |
| Container smoke run | Skipped | Started once, then stopped after user said no container verification needed; no smoke result recorded. |

## Missed or expanded scope

1. **日志脱敏 scope removed**
   - Earlier draft/spec had query token log redaction wording.
   - User explicitly said不需要、不要加戏，并要求撤销。
   - Current implementation no longer contains `safelog` / `RedactRawQuery`.

2. **Legacy identity compatibility removed**
   - User explicitly said删除实现、无须兼容。
   - Current implementation no longer reads legacy `runtime.state_dir/device.json`.

3. **Real remote Agent path not verified**
   - Earlier requirement/spec described Cloud Gate + Local Agent outbound path.
   - User later clarified current image work暂时没有连接其他远程的需求。
   - Verification records this as not verified, not as a failure of current image-oriented scope.

4. **Container runtime smoke skipped**
   - Docker image build passed.
   - Container was not smoke-tested because user explicitly said无须验证容器。

5. **Extra web diffs present**
   - `web/src/components/workspace/WorkspaceSessionSidebar.vue` changes cursor style from `cursor-move` to `cursor-pointer`.
   - `web/src/store/workspaceSessions.ts` contains formatting-only multiline change.
   - These are outside Cloud Gate PoC scope and should be reviewed before commit.

## Risks

1. **PoC fixed credential risk**
   - `admin/admin` is only acceptable as temporary PoC behavior.
   - Formal user system and device binding remain future work.

2. **No runtime container smoke**
   - Docker build success proves image assembly, but not runtime `/sessions` or `/api/health` behavior inside a container.

3. **No HTTPS reverse proxy validation**
   - TLS termination and WebSocket upgrade are documented assumptions, not validated in this run.

4. **No cross-network remote Agent validation**
   - Current verification does not prove Local Agent can connect from another machine/network.

5. **Unrelated staged/working diffs risk**
   - Extra web diffs should be split or explicitly accepted before a final commit.

6. **Historical docs still mention localhost or older auth/device behavior**
   - Current implementation docs were updated where relevant.
   - Older verification/spec documents intentionally remain historical records and were not rewritten.

## Incomplete items

- [ ] User verification for Claude Code / Codex real TUI.
- [ ] Optional future container runtime smoke if needed.
- [ ] Optional future real Cloud Gate + separate Local Agent / HTTPS reverse proxy verification.
- [ ] Decide whether to keep, split, or revert unrelated `WorkspaceSessionSidebar.vue` and `workspaceSessions.ts` diffs.
- [ ] Decide whether `Dockerfile.cn` belongs in the same commit as the default Cloud Gate PoC Dockerfile.

## Conclusion

当前实现满足用户最新收口后的核心目标：本地开发统一为 `127.0.0.1` 前后端分离，镜像构建时将 web 资源和 Go binary 打包进同一镜像，Go server 可通过 `web.static_dir` 提供前端静态资源并保持 `/api` 路由优先，PoC auth 固定为 `admin/admin`，device id/name 由当前配置生成/写入并用于 Agent hello，terminal route unavailable 改为结构化错误。

验证结论：自动化测试、前端检查、前端 production build、Go vet 和 Docker build 均通过。容器运行 smoke、真实 HTTPS reverse proxy、跨网络 Agent 和 Claude Code / Codex TUI 未验证，按用户最新指令和需求边界记录为未完成/后续项。
