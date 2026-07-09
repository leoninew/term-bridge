# portable package 运行时云端对接收口验证
最后修改时间: 2026-07-09 14:33:34

Flow mode: light / 轻量模式
Stage: Verification / 验证
Review status: Accepted

## Requirement alignment

按 `docs/requirement/20260709-package-runtime-cloud-connection.md` 核对：

1. portable package 已明确为本地 Agent 包，`task package` 使用 `yarn build:local`。
2. package 不再复制已删除的 `configs/config.production.yaml`。
3. package 启动脚本使用 `TERMBRIDGE_ENV=local`，不再使用 production 环境名。
4. package 根目录会包含 `.env.local` 运行时配置模板和 `.env.example` 参考文件。
5. package `.env.local` 承担 Go 后端运行时配置，包括本地静态目录、本机 public URL、Cloud public/API URL 和 Agent OAuth client secret。
6. 前端构建输入 `web/.env.local` 与 package 运行时根目录 `.env.local` 语义已在 README 中区分。

## Spec alignment

不适用。light / 轻量模式未创建单独 Spec / 规格文档，按 Requirement / 需求核对。

## Plan alignment

不适用。light / 轻量模式未创建单独 Plan / 计划文档，按 Requirement / 需求和用户追加指令核对。

## Actual diff summary

- `Taskfile.yml`：`package` 任务改为 `yarn build:local`；删除 `configs/config.production.yaml` 复制；新增复制 `.env.example` 和 `scripts/package/.env.local`。
- `scripts/package/.env.local`：新增 portable package 运行时配置模板。
- `scripts/package/start.cmd` / `start.sh`：运行环境改为 `local`，加载 `.env.local`，使用 `TERMBRIDGE_LOCAL__PUBLIC_URL` 打开浏览器，然后启动 `termbridge.exe agent`。
- `cmd/termbridge/app/app_test.go`：新增 package 构建入口和运行时脚本语义测试。
- `README.md` 和 requirement 文档：补充 portable package 构建与运行时云端对接边界。

## Expected vs actual changed files

| 范围 | 预期 | 实际 | 结论 |
| --- | --- | --- | --- |
| package 前端入口 | 本地 Agent 包使用 `build:local` | `Taskfile.yml` 已改为 `cd web && yarn build:local` | 符合 |
| 删除 production 运行时依赖 | 不复制已删除的 `config.production.yaml` | `Taskfile.yml` 不再复制该文件 | 符合 |
| package 运行时 env | 包内提供根目录 `.env.local` | `scripts/package/.env.local` 会复制到 package 根目录 | 符合 |
| 启动环境名 | `TERMBRIDGE_ENV=local` | `start.cmd` / `start.sh` 均设置 local | 符合 |
| 云端对接配置 | package `.env.local` 包含 Cloud URL 与 OAuth client secret | 模板已包含 Cloud public/API URL 和 local OAuth client id/secret/redirect/scopes | 符合 |
| secret 边界 | client secret 不进前端构建输入 | 仅 package runtime `.env.local` 包含 `TERMBRIDGE_LOCAL__OAUTH__CLIENT_SECRET` | 符合 |

## Acceptance checklist

- [x] `Taskfile.yml` 的 `package` 任务使用 `yarn build:local`。
- [x] `Taskfile.yml` 不再复制 `configs/config.production.yaml`。
- [x] `Taskfile.yml` 将 `scripts/package/.env.local` 复制为 package 根目录 `.env.local`。
- [x] `Taskfile.yml` 保留 `.env.example` 参考文件复制。
- [x] `scripts/package/start.cmd` 设置 `TERMBRIDGE_ENV=local`。
- [x] `scripts/package/start.sh` 设置 `TERMBRIDGE_ENV=local`。
- [x] 启动脚本从 `.env.local` 读取 `TERMBRIDGE_LOCAL__PUBLIC_URL`。
- [x] `scripts/package/.env.local` 包含本地静态目录、本机地址、Cloud 地址和 OAuth client 配置。
- [x] 相关 Go 测试通过。
- [x] `task package` 可实际构建 Windows portable zip。

## Command results

### App package config tests

```text
gofmt -w "cmd/termbridge/app/app_test.go" && go test ./cmd/termbridge/app -run "TestFrontendStaticEntryConfigUsesModeSpecificBuildInputs|TestPortablePackageBuildsLocalFrontendAndRuntimeEntry"
ok  	gitee.com/leoninew/TermBridge-go/cmd/termbridge/app	2.041s
```

结果：通过。

### Static grep check

```text
Grep configs/config.production.yaml|TERMBRIDGE_ENV=production|cd web && yarn build$
No current package script or startup script uses the removed production runtime path.
```

结果：符合。输出中仅剩测试断言、历史文档或本次 requirement 对旧路径的说明。

### Portable package build

```text
task package
```

关键输出：

```text
task: [package] cd web && yarn build:local
$ vue-tsc --noEmit && vite build
✓ built in 522ms
task: [package] env GOOS=windows GOARCH=amd64 go build -o bin/package/TermBridge/termbridge.exe ./cmd/termbridge
task: [package] cp .env.example bin/package/TermBridge/.env.example
task: [package] cp scripts/package/.env.local bin/package/TermBridge/.env.local
task: [package] cd bin/package && zip -r ../TermBridge-windows-x64.zip TermBridge
```

结果：通过，已生成 `bin/TermBridge-windows-x64.zip`。

构建期间 Vite / Rolldown 对 `node_modules/@vueuse/core/dist/index.js` 输出 `INVALID_ANNOTATION` warning，提示第三方包中的 `/* #__PURE__ */` 注释位置无法解释并被忽略；构建最终 `✓ built`，不是本轮代码错误。

## Missed or expanded scope

- 用户追加指出不能只考虑打包，还要考虑运行时如何对接云端；已新增 package runtime `.env.local` 模板和启动脚本加载逻辑。
- 本轮未实现后端 runtime config 注入，因此前端公开 Cloud 地址仍来自 `web/.env.local` 构建期固化。
- 本轮未真实连接外部 Cloud 验证 OAuth 授权闭环；仅验证 package 构建、启动配置边界和脚本语义。

## Risks

1. 发布前必须把 `scripts/package/.env.local` 中的 Cloud 域名和 OAuth client secret 替换成目标 Cloud 环境的真实值。
2. package 根目录 `.env.local` 和前端构建输入 `web/.env.local` 中的 Cloud 地址必须保持一致，否则浏览器入口与 Agent 后端连接目标会漂移。
3. Cloud 后端 `cloud.oauth.clients[]` 必须登记与 package `.env.local` 一致的 client id、client secret、redirect URL 和 scopes，否则连接云端会在 OAuth 阶段失败。
4. `task package` 生成了 ignored 构建产物 `bin/` 与 `web/dist/`；如不需要保留 zip，可后续清理。

## Incomplete items

- 未做真实 Cloud OAuth 对接验收。
- 未做双击 `start.cmd` 的人工启动验收。

## Conclusion

本轮修复了 portable package 仍依赖 production 运行时配置的问题。package 现在使用本地前端入口构建，不再复制已删除的 `config.production.yaml`，启动时选择 `TERMBRIDGE_ENV=local` 并加载包根目录 `.env.local`。运行时 `.env.local` 已覆盖本地静态目录、本机访问地址、Cloud 地址和 Agent OAuth client secret，能够支撑本地 Agent 包对接云端。相关 Go 测试和 `task package` 均通过。
