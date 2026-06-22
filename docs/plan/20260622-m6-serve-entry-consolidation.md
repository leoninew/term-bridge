# M6.1 Serve 统一入口与文档收口计划
最后修改时间: 2026-06-22 17:01:33

Review status: Accepted

## Basis

- Requirement: `docs/requirement/20260622-m6-serve-entry-consolidation.md`，Review status: Accepted
- M6 requirement: `docs/requirement/20260620-m6-gateway-web-terminal-mvp.md`，Review status: Accepted
- M6 plan: `docs/plan/20260620-m6-gateway-web-terminal-mvp.md`，Review status: Accepted
- Spec: `docs/spec/20260622-m6-serve-entry-consolidation.md`，Review status: Accepted
- M6 verification: `docs/verification/20260620-m6-gateway-web-terminal-mvp.md`，Review status: Accepted
- Roadmap: `docs/plan/20260617-roadmap-refresh.md`
- Current README: `README.md`
- Current justfile: `justfile`
- Current CLI: `internal/cli/cli.go`
- Current config loader: `internal/config/config.go`

M6.1 使用严格模式 / strict。本计划基于已接受的 Requirement 和 Spec，收口 M6 Gateway/Agent 入口、配置、文档和验收状态，不进入 M7，不扩展正式用户系统，不重写 tunnel protocol，不引入第二套前端。

## User decisions carried into plan

1. Gateway/Agent 对外收口为统一 `termbridge serve` 启动入口。
2. 开发阶段入口为 `just serve`。
3. 启动后同时具备 Gateway service 和 Agent connector 能力；用不用是配置、路由和访问路径问题，不再是两个用户模式。
4. Agent 连接哪个 Gateway 是配置问题。
5. Gateway/Agent 运行参数不通过 CLI 参数或环境变量作为正式入口传递，全部收口到 Viper 配置及配置覆盖机制。
6. 删除所有 M6 专用 just target。
7. 删除 `--seed-session-command`。
8. 前端只有一个，通过不同路由区分 local/Gateway 访问面。
9. M6 verification 在 M6.1 完成后更新为 `Accepted`，同时保留剩余人工验证项。
10. README 不提及临时 `admin/admin`。

## Implementation steps

### Step 1. 引入 `serve` CLI command shape

目标：把对外正式入口从 `gateway` / `agent` 两个 command 收口为 `serve`。

计划：

1. 在 `internal/cli/cli.go` 中新增/替换 command kind：`serve`。
2. 删除独立 `gateway` / `agent` command parsing。
3. 删除 Gateway/Agent 运行参数 flag：
   - `--host`
   - `--port`
   - `--open`
   - `--dev`
   - `--gateway-url`
   - `--device-name`
   - `--seed-session-command`
4. 更新 top-level usage：展示 `termbridge serve`，不再展示 `termbridge gateway` / `termbridge agent`。
5. 新增 `PrintServeUsage` 或调整现有 usage，使 help 说明：
   - `serve` 启动统一 Gateway/Agent service。
   - Gateway listen、Agent upstream、device name 等由配置控制。
6. 更新 CLI tests，覆盖：
   - `serve` 可解析。
   - `gateway` / `agent` 不再作为 command。
   - 运行参数 flag 不再被接受。
   - help 不出现被删除的 command/flag。

### Step 2. 扩展配置结构并建立配置驱动入口

目标：用 Viper 配置和配置覆盖机制承载 Gateway service 与 Agent connector 的运行参数。

计划：

1. 在 `internal/config/config.go` 增加配置结构：

```go
type GatewayConfig struct {
    Host string
    Port int
    Open bool
    Dev bool
}

type AgentConfig struct {
    GatewayURL string
    DeviceName string
}
```

2. 在 `Config` 中加入：

```go
Gateway GatewayConfig
Agent AgentConfig
```

3. 在 `.termbridge.default.yaml` 增加默认配置，建议键名：

```yaml
gateway:
  host: 127.0.0.1
  port: 9010
  open: false
  dev: false

agent:
  gateway_url: http://127.0.0.1:9010
  device_name: local-dev
```

4. 将新 key 加入 unknown key allowlist。
5. 加载配置时填充 `cfg.Gateway` 和 `cfg.Agent`。
6. 增加必要校验：
   - Gateway port 必须在 0..65535。
   - Gateway host 可为空或非空的边界按现有 server 行为确定；若现有 server 要求非空则校验非空。
   - Agent gateway URL 必须非空；格式校验可先保持最小，只做 trim 非空。
   - Agent device name trim 后可为空，空值沿用现有 device identity fallback。
7. 更新 config tests：
   - defaults 可读。
   - `.termbridge.yaml` 可覆盖 Gateway/Agent 配置。
   - unknown key 仍拒绝。
   - invalid port / empty gateway URL 按预期报错。

### Step 3. 将 app 层统一到 serve runtime

目标：`app.Run` 通过 `CommandServe` 启动后同时具备 Gateway service 和 Agent connector 能力。

计划：

1. 在 `internal/app/app.go` 中将 command shape 从 `GatewayCommand` / `AgentCommand` 收口为 `ServeCommand`。
2. `runServe` 使用 `cfg.Gateway` 创建 Gateway server。
3. `runServe` 使用 `cfg.Agent` 创建 Agent client。
4. 启动后同时具备两类能力：
   - Gateway service listening。
   - Agent connector outbound tunnel。
5. 并发运行策略需要避免一方退出静默吞掉另一方失败：
   - 使用 `context.WithCancel` 统一生命周期。
   - 任一组件返回非 `context.Canceled` 错误时取消整体并返回错误。
   - 正常 context cancel 时整体退出。
6. Agent connector 使用本地 `webterminal.Registry` 作为 runtime access，保持 Gateway service 不直接拥有 PTY/process lifecycle。
7. 删除 seed-session wiring：不再在 serve/agent 启动中自动创建 dev session。
8. 更新 app tests：
   - serve command 使用 config 中 gateway/agent 值。
   - seed-session 字段和行为不存在。
   - 如测试中 mock `runGatewayServer` / `runAgentClient`，需覆盖统一生命周期。

### Step 4. 调整 justfile 长期入口

目标：开发入口与正式入口一致，移除 milestone 和旧模式入口。

计划：

1. 保留通用入口，所有组合命令按“前端在前、后端在后”组织：
   - `check`：单一检查入口，直接覆盖前端 typecheck/lint/format/test 和后端 Go fmt/vet/test，不通过 Go 专用 just 子入口串联。
   - `test`：前后端单元测试统一入口，先运行前端 Vitest，再运行后端 Go tests。
   - `build`
   - `exec`
   - `web`
2. 新增/保留统一后端入口：
   - `serve`: 直接通过 `go run cmd/termbridge/main.go serve` 运行 `termbridge serve`，覆盖 local Web API、Gateway service 和 Agent connector。
3. 删除旧开发入口：
   - `web-backend`
   - `gateway`
   - `agent`
   - `web-check`
   - `web-typecheck`
   - `web-lint`
   - `web-build`
   - `web-frontend`
4. 删除 M6 专用 just target：
   - `m6-frontend`
   - `m6-agent`
   - `m6-agent-with-session`
   - `m6-test`
   - `m6-race`
   - `m6-check`
   - `m6-pw-validate`
   - `m6-pw-run`
5. 不删除 `.pomelo-pw/m6-gateway-web-terminal.yaml`，因为它是验证资产，不是入口污染。

### Step 5. 核对前端单体与路由区分

目标：文档必须反映“前端只有一个，通过不同路由区分”的真实实现。

计划：

1. 核对 `web/src` 当前是否已有 route 或状态区分 local/Gateway 访问面。
2. 本任务默认不新增第二套 frontend、不新增 Gateway 专用构建。
3. 如果当前实现尚未使用显式 router route，需要在文档中谨慎表述为“同一前端承载 local/Gateway 访问面，后续通过路由继续收口”，避免虚构已实现能力。
4. 不进行大规模 UI 重构；如发现必须代码改动才能满足“不同路由区分”，先报告冲突并回到需求/计划调整。

### Step 6. 更新 README 使用路径

目标：让 README 反映当前本地和统一 serve 两条主要使用路径。

计划：

1. 保留现有本地 CLI 示例：
   - `termbridge exec -- <command...>`
   - `termbridge --cwd <dir> exec -- <command...>`
   - `termbridge session`
   - `termbridge workspace`
2. 新增 serve 使用路径：
   - 在 `.termbridge.yaml` 中覆盖 Gateway/Agent 配置。
   - 启动统一服务：`termbridge serve`。
   - 浏览器访问配置中声明的 Gateway service 地址。
3. 明确 Gateway MVP 当前支持：
   - 查看 device / workspace / session。
   - 读取 session history。
   - attach 已存在 terminal session。
4. 明确 Gateway MVP 当前不支持：
   - 通过 Gateway 创建 session。
   - 通过 Gateway 修改 workspace/session。
   - 正式用户系统和生产部署能力。
5. 不在 README 中公开临时 `admin/admin`。

### Step 7. 更新设计文档 Gateway/Agent 合并能力边界

目标：让 `docs/design.md` 与统一 serve 模型一致。

计划：

1. 更新最后修改时间。
2. 补充当前 serve 链路：

```text
termbridge serve
  ├─ Gateway service: Browser/API/WS entry
  └─ Agent connector: outbound tunnel + local runtime adapter
       ↓
     local runtime -> PTY -> process
```

3. 明确 Gateway service 职责：auth、device registry、routing、relay。
4. 明确 Agent connector 职责：outbound tunnel、本地 runtime adapter、workspace/session/history/terminal access。
5. 明确两者同进程/同入口启动后同时可用，但是否连接远端、访问哪个 Gateway、使用哪条路由由配置和访问路径决定。
6. 明确 Gateway service 不拥有 PTY/process lifecycle。
7. 明确前端只有一个，通过不同路由或访问面区分 local/Gateway 能力。

### Step 8. 更新 roadmap 中 M6/M6.1 状态

目标：让 roadmap 显示 M6 工程实现后由 M6.1 收口为 serve 统一入口。

计划：

1. 更新 `docs/plan/20260617-roadmap-refresh.md` 最后修改时间。
2. 在 milestone overview 或 M6 章节中补充：
   - M6 Gateway Web Terminal MVP 工程链路已实现。
   - M6.1 收口为 `termbridge serve` / `just serve`。
   - M6 verification 将在 M6.1 后进入 Accepted。
   - M7 之前仍保留 Agent reconnect、20+ sessions、真实 Claude/Codex TUI、Windows 真机验证等人工验证项。
3. 不重写整个 roadmap，只做与 M6.1 相关的局部同步。

### Step 9. 整理 docs/todo.md

目标：消除与 M5/M6 verification 冲突的过期 TODO。

计划：

1. 将原有 webterminal 待处理项重新分类：
   - 已由 M5 处理或缓解：large output slow client detach/backpressure、history 并发/flush、copyBytes per-client 重复 copy 等。
   - 仍需人工验证：20+ sessions、真实 Claude/Codex TUI、Windows 真机 cleanup、Agent reconnect。
   - 仍未解决：如果文档核对后仍存在未处理项，单独保留。
2. 保留历史修复记录，但避免“待处理”章节继续列出已处理项。
3. 对 M5/M6 verification 文档建立引用，说明事实来源。

### Step 10. 更新 M6 verification 并标记 Accepted

目标：让 M6 verification 与 M6.1 收口结果一致。

计划：

1. 更新 `docs/verification/20260620-m6-gateway-web-terminal-mvp.md` 最后修改时间。
2. 将 `Review status: Draft` 改为 `Accepted`。
3. 增加 M6.1 serve entry/docs consolidation 说明：
   - 对外入口收口为 `termbridge serve` / `just serve`。
   - Gateway/Agent 运行参数进入配置。
   - M6 专用 just target 已删除。
   - README/design/roadmap/todo 已同步。
   - 前端保持单体，通过路由/访问面区分 local/Gateway。
4. 保留未完成项，不把它们从文档中删除：
   - Agent reconnect。
   - 20+ sessions。
   - 真实 Claude/Codex TUI。
   - Windows 真机验证。
5. 如 command results 中列出被删除的 `just m6-*`，改写为历史验证结果或移到说明中，避免新读者误以为这些 target 仍存在。

### Step 11. 创建 M6.1 verification 文档

目标：对照本 requirement、spec 和 plan 记录实际改动、验证结果和剩余风险。

计划：

1. 创建 `docs/verification/20260622-m6-serve-entry-consolidation.md`。
2. 按 standard verification 维度记录：
   - requirement alignment。
   - plan alignment。
   - actual diff summary。
   - expected vs actual changed files。
   - acceptance criteria checklist。
   - command results。
   - missed or expanded scope。
   - risks。
   - incomplete items。
   - conclusion。
3. Verification 阶段再运行检查并写入结果；Plan 阶段不运行验证。

## Files to change

预计修改：

1. `README.md`
2. `.termbridge.default.yaml`
3. `justfile`
4. `internal/config/config.go`
5. `internal/config/config_test.go`
6. `internal/cli/cli.go`
7. `internal/cli/cli_test.go`
8. `internal/app/app.go`
9. `internal/app/app_test.go`
10. `docs/design.md`
11. `docs/plan/20260617-roadmap-refresh.md`
12. `docs/todo.md`
13. `docs/verification/20260620-m6-gateway-web-terminal-mvp.md`
14. `docs/verification/20260622-m6-serve-entry-consolidation.md`（Verification 阶段创建）

可能修改：

1. `internal/agent/*`：如果 seed-session command wiring 或 Agent command shape 有专门实现且不再被使用，则同步删除。
2. `internal/gateway/*`：如果 constructor/config naming 需要适配 serve 组合启动，可做最小调整。
3. `web/src/*`：仅当当前前端路由现实与“单前端、不同路由区分”的文档要求冲突且用户确认后修改。

预计不修改：

1. Tunnel frame protocol。
2. Gateway auth 行为。
3. `.pomelo-pw/m6-gateway-web-terminal.yaml`。

## Verification plan

Implementation 阶段完成后，进入 Verification 阶段时执行：

1. `just --list`
   - 确认 `serve` 存在。
   - 确认 `gateway` / `agent` 平级 just target 已删除。
   - 确认 `m6-*` target 已删除。
2. `go test ./internal/config ./internal/cli ./internal/app ./internal/agent`
   - 覆盖配置、CLI/app/agent command shape 回归。
3. `go test ./internal/tunnel ./internal/gatewayauth ./internal/gateway ./internal/agent ./internal/cli ./internal/app`
   - 作为删除 `m6-test` target 后的等价底层测试命令记录。
4. `just build`
   - 确认 Go binary 和前端生产构建都被 build 入口覆盖。
5. 文档一致性核对：
   - README 不出现 `admin/admin`。
   - README 出现 `termbridge serve` 和配置示例。
   - README 不推荐 `termbridge gateway` / `termbridge agent`。
   - justfile 不出现 `m6-` target 名。
   - justfile 不出现平级 `gateway` / `agent` target。
   - CLI help 不出现 `gateway` / `agent` command 或 Gateway/Agent 运行参数 flag。
   - M6 verification 为 `Accepted` 且保留未完成项。

## Assumptions

1. 删除 M6 专用 just target 不等于删除底层验证能力；verification 文档只记录当前可执行的通用替代命令，不保留历史入口作为兼容路径。
2. 删除 `--seed-session-command` 后，不再保留通过 CLI 自动 seed dev session 的正式或开发入口。
3. `termbridge serve` 可以作为统一正式入口承载 Gateway service 和 Agent connector。
4. Agent connector 是否连接哪个 Gateway 完全由配置决定。
5. 前端当前实现可以支持“单前端、不同路由或访问面区分”的文档表述；Implementation 阶段需要核对现实。
6. README 不写 `admin/admin` 不影响 M6.1 入口收口；登录细节后续由正式 auth 或开发者上下文处理。

## Risks

1. 删除 `gateway` / `agent` CLI 和 just target 会影响已有脚本或个人验证路径；本任务不做向后兼容，verification 文档只记录当前 `serve` 与通用检查命令。
2. 统一 serve 同时启动 Gateway service 和 Agent connector 后，生命周期和错误传播比单独进程更复杂，必须在 app 层测试覆盖基本失败路径。
3. 删除 `--seed-session-command` 可能影响此前用于 Gateway attach 验证的 seed-session 快捷路径；需要在 verification 中记录后续验证应通过已有 session 或单独手动启动 session 完成。
4. M6 verification 改为 Accepted 后，剩余人工验证项可能被忽略；文档必须显式保留 incomplete items。
5. README 不提及临时账号后，新用户可能知道如何启动 serve 但不知道如何登录；这是用户已确认的取舍，后续正式 auth 文档需要补齐。
6. 前端路由区分如果只写文档但实现不一致，会继续造成入口认知混乱；Implementation 阶段需要核对当前前端路由现实。

## Rollback plan

如 M6.1 实施后发现入口收口方向不合适：

1. 文档改动可通过 git diff 精确回退相关段落。
2. justfile 中被删除的 `m6-*` target、`gateway` target 或 `agent` target 不作为兼容路径恢复；若后续确有需要，应重新设计明确的内部验证工具。
3. `gateway` / `agent` CLI command shape 和运行参数 flag 不作为兼容路径恢复。
4. `--seed-session-command` 相关 CLI/app/agent wiring 不作为兼容路径恢复；后续验证应通过已有 session 或新的显式验证方案完成。
5. M6 verification 的 `Review status` 可在后续文档修订中重新降为 Draft，但应记录原因。

## User review notes

本计划已根据用户最新决策改为 `serve` 统一入口模型。若用户要求开始实现，则视为接受本计划，先将 `Review status` 更新为 `Accepted`，再进入 Implementation / 实现阶段。
