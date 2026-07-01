# Windows EXE Static Directory Zip Packaging Plan
最后修改时间: 2026-07-01 13:21:25

## Review status

Accepted

## Flow mode / Stage

标准模式 / standard；计划 / Plan 已接受，当前进入实现 / Implementation。

## Requirement basis

读取并依据：

- `docs/requirement/20260701-windows-zip-packaging.md`

已接受需求要求：

1. 目标分发形态为 Windows x64 zip 包。
2. zip 包采用 `termbridge.exe + 静态目录`，不做单 exe embedded assets。
3. zip 包需要可双击启动并自动打开浏览器。
4. zip 包包含 `start.cmd` 和 `start.sh`。
5. 分发配置复用 `configs/config.yaml`。
6. 需要在 `Taskfile.yml` 添加 `package` task 完成该能力。

## Package shape

目标 zip：

```text
bin/TermBridge-windows-x64.zip
```

zip 内目录：

```text
TermBridge/
├── termbridge.exe
├── web/
│   ├── index.html
│   └── assets/
├── configs/
│   └── config.yaml
├── .env.local
├── .env.example
├── start.cmd
└── start.sh
```

说明：

- `termbridge.exe`：Windows x64 Go binary，入口仍为现有 `cmd/termbridge`。
- `web/`：由 `web/yarn build` 生成的 `web/dist` 复制而来。
- `configs/config.yaml`：直接复用仓库中的默认配置文件，满足用户确认的配置复用要求。
- `.env.example`：直接复制仓库根目录已有文件，不修改，只作为用户参考；不在 `scripts/package` 下维护第二份示例。
- `.env.local`：直接复制 `scripts/package/.env.local`，只包含 zip 分发运行必须的最小配置。
- `start.cmd`：Windows 双击入口，只负责切换 cwd、指定 `TERMBRIDGE_ENV=local`、从 `.env.local` 读取浏览器 URL、延迟打开浏览器、启动 `termbridge.exe serve`。
- `start.sh`：便携 shell 启动入口，主要用于 Cygwin/Git Bash/MSYS/类 Unix shell 调试；指定 `TERMBRIDGE_ENV=local` 并从 `.env.local` 读取浏览器 URL，本轮验证重点仍是 Windows x64。

## Runtime configuration strategy

由于用户要求复用 `configs/config.yaml`，且该文件当前默认：

- `web.static_dir: ""`
- `agent.listen_url: http://127.0.0.1:9030`
- `agent.public_url: http://localhost:9031`
- `gate.browser.allowed_origins` 包含 Vite dev server `9031`

分发包不直接改写 `configs/config.yaml`，也不修改根目录 `.env.example`。打包时原样复制根目录 `.env.example` 作为参考文件，同时直接复制 `scripts/package/.env.local` 作为 zip 包内实际运行配置。`scripts/package/.env.local` 只启用 zip 分发运行必须项：

```text
TERMBRIDGE_WEB__STATIC_DIR=web
TERMBRIDGE_AGENT__LISTEN_URL=http://127.0.0.1:9033
TERMBRIDGE_AGENT__PUBLIC_URL=http://127.0.0.1:9033
TERMBRIDGE_GATE__BROWSER__ALLOWED_ORIGINS=http://127.0.0.1:9033,http://localhost:9033
```

理由：

1. `start.cmd` / `start.sh` 只指定 OS env `TERMBRIDGE_ENV=local`，让配置层选择 `configs/config.local.yaml` 与 `.env.local`；端口、URL、静态目录等用户可编辑项只放在 `.env.local`。
2. 根目录 `.env.example` 原样进入 zip，保持参考文档属性；`scripts/package/.env.local` 是分发包实际运行配置模板。
3. `start.cmd` / `start.sh` 不再堆大量 `set` / `export`，启动脚本只负责选择 local 环境、加载 `.env.local` 和启动服务。
4. `web.static_dir` 的相对路径会按 cwd 展开；启动脚本先切换到自身目录，`web` 会解析为 zip 解压目录下的 `web/`。
5. 前后端同源部署时，浏览器 URL、API、WebSocket 使用同一后端端口，不再依赖 Vite dev server `9031`。
6. `TERMBRIDGE_AGENT__LISTEN_URL` 和 `TERMBRIDGE_AGENT__PUBLIC_URL` 同时放在 package `.env.local` 中，确保改端口时只改 `.env.local`，启动脚本不需要改。
7. `agent.connect_url` 为空时已有 fallback 到 `agent.listen_url` 的语义，因此第一版不在 `.env.local` 里重复启用。
8. `.env.local` 不写入 `TERMBRIDGE_AGENT__DEVICE_ID`、`TERMBRIDGE_AGENT__DEVICE_NAME`、`TERMBRIDGE_JWT__SECRET_KEY` 等机器私有或首次启动应生成的运行状态；这些生成项在 `TERMBRIDGE_ENV=local` 时写入 `configs/config.local.yaml`。
9. 生成配置的代码路径按当前环境名泛化为 `configs/config.<env>.yaml`，local 只是打包启动脚本选择的环境名，不在生成逻辑里写死。

## Start script behavior

### start.cmd

随包模板位于：

- `scripts/package/start.cmd`

计划内容要点：

```bat
@echo off
setlocal
cd /d "%~dp0"
set "TERMBRIDGE_ENV=local"

if not exist ".env.local" (
  echo .env.local is missing
  exit /b 1
)

for /f "usebackq eol=# tokens=1,* delims==" %%A in (".env.local") do (
  if not "%%A"=="" set "%%A=%%B"
)

if not defined TERMBRIDGE_AGENT__PUBLIC_URL (
  echo TERMBRIDGE_AGENT__PUBLIC_URL is not set in .env.local
  exit /b 1
)

start "" /min cmd /c "timeout /t 2 /nobreak >nul & start %TERMBRIDGE_AGENT__PUBLIC_URL%"
termbridge.exe serve
```

行为：

1. 双击后工作目录切换到 zip 解压目录中的 `TermBridge/`。
2. 设置 `TERMBRIDGE_ENV=local`，使配置层选择 `configs/config.local.yaml` 和 `.env.local`。
3. 从 `.env.local` 读取 `TERMBRIDGE_AGENT__PUBLIC_URL` 作为浏览器打开地址；端口/URL 的修改点只在 `.env.local`。
4. 后台启动一个短延迟 `cmd`，用默认浏览器打开该 URL。
5. 前台运行 `termbridge.exe serve`，控制台窗口保留用于查看日志和错误。
6. 运行期用户配置来自 zip 包内 `.env.local`；启动生成项写入 `configs/config.local.yaml`。

已知限制：

- 2 秒固定等待不等于严格 health check；若首次启动生成密钥或迁移较慢，浏览器可能先看到连接失败，刷新后恢复。后续可增强为 health polling，但本轮优先保持脚本简单。

### start.sh

随包模板位于：

- `scripts/package/start.sh`

说明：

- `start.sh` 不改变本轮 Windows x64 目标；它是用户要求的随包辅助入口。
- 从 `.env.local` 读取 `TERMBRIDGE_AGENT__PUBLIC_URL` 作为浏览器打开地址；端口/URL 的修改点只在 `.env.local`。
- 在 Cygwin 上优先尝试调用 `cygstart` 打开浏览器。
- 在 Linux 桌面环境上尝试调用 `xdg-open`，在 macOS 上尝试调用 `open`。
- 与 `start.cmd` 一样，运行期配置来自 zip 包内 `.env.local`。

## Taskfile implementation plan

修改 `Taskfile.yml`：新增 `package` task，并按现有 Taskfile 的命令风格直接组织构建步骤，不新增 PowerShell package 脚本。

`package` task 行为：

1. 清理旧 package staging：`bin/package/TermBridge` 和 `bin/TermBridge-windows-x64.zip`。
2. 创建目录：
   - `bin/package/TermBridge/web`
   - `bin/package/TermBridge/configs`
3. 构建前端：
   - `cd web && yarn build`
4. 构建 Windows x64 binary：
   - `env GOOS=windows GOARCH=amd64 go build -o bin/package/TermBridge/termbridge.exe ./cmd/termbridge`
5. 复制静态资源：
   - `web/dist/*` -> `bin/package/TermBridge/web/`
6. 复制配置和环境文件：
   - `configs/config.yaml` -> `bin/package/TermBridge/configs/config.yaml`
   - 根目录 `.env.example` -> `bin/package/TermBridge/.env.example`，原样复制，不修改。
   - `scripts/package/.env.local` -> `bin/package/TermBridge/.env.local`，作为分发包实际运行配置。
7. 复制启动模板：
   - `scripts/package/start.cmd` -> `bin/package/TermBridge/start.cmd`
   - `scripts/package/start.sh` -> `bin/package/TermBridge/start.sh`
8. 打包 zip：
   - `cd bin/package && zip -r ../TermBridge-windows-x64.zip TermBridge`

## Files to change

预计修改：

1. `Taskfile.yml`
   - 新增 `package` task。

2. `scripts/package/.env.local`
   - 新增 zip 分发包最小运行配置模板。

3. `scripts/package/start.cmd`
   - 新增 Windows 双击启动模板。

4. `scripts/package/start.sh`
   - 新增便携 shell 启动模板。

5. `docs/plan/20260701-windows-zip-packaging.md`
   - 更新计划与实际实现策略一致。

不预计修改产品 Go/Vue 代码。

## Verification plan

进入 Verification / 验证阶段后，建议执行：

1. 构建分发包：

   ```powershell
   task package
   ```

2. 检查 zip 文件存在：

   ```powershell
   Test-Path bin/TermBridge-windows-x64.zip
   ```

3. 检查 zip 内容包含：

   ```text
   TermBridge/termbridge.exe
   TermBridge/web/index.html
   TermBridge/configs/config.yaml
   TermBridge/.env.local
   TermBridge/.env.example
   TermBridge/start.cmd
   TermBridge/start.sh
   ```

4. 解压到临时目录，检查可启动：

   ```powershell
   Expand-Archive bin/TermBridge-windows-x64.zip <temp-dir>
   cd <temp-dir>/TermBridge
   .\termbridge.exe --version
   ```

5. 启动服务并验证静态页面：

   ```powershell
   .\start.cmd
   ```

   另一个终端或后台检查：

   ```powershell
   Invoke-WebRequest http://127.0.0.1:9030/api/health
   Invoke-WebRequest http://127.0.0.1:9030/
   ```

6. 如时间和环境允许，通过页面或 API 验证最小 CLI session。若自动化成本过高，在 verification 文档中记录未自动验证原因和人工验证建议。

## Rollback

如果实现后需要回退：

1. 删除 `Taskfile.yml` 中新增的 `package` task。
2. 删除 `scripts/package/start.cmd` 和 `scripts/package/start.sh`。
3. 删除生成的本地构建产物：
   - `bin/package/`
   - `bin/TermBridge-windows-x64.zip`
4. 保留或删除本 feature 的 docs 文档由用户决定；不要自动删除过程文档。

## Risks and mitigations

1. 风险：`start.cmd` 使用固定延迟打开浏览器，可能早于服务 ready。
   - 缓解：第一版接受刷新即可；后续可改为 health polling。

2. 风险：复用 `configs/config.yaml` 导致默认 `web.static_dir` 为空。
   - 缓解：package 时复制 zip 内 `.env.local`，启用 `TERMBRIDGE_WEB__STATIC_DIR=web`。

3. 风险：直接双击 `termbridge.exe` 仍不会自动打开浏览器。
   - 缓解：交付入口定义为双击 `start.cmd`；zip 包内保留 exe 作为 CLI/debug 入口。

4. 风险：第三方 CLI 不在用户 PATH 中。
   - 缓解：这是外部依赖；TermBridge 只能运行用户环境可发现的命令，文档/错误提示后续补充。

5. 风险：本地服务拥有运行命令能力。
   - 缓解：默认 `configs/config.yaml` 的 `agent.listen_url` 仍是 `127.0.0.1:9030`；更强认证/Host/Origin 策略作为后续安全增强。

6. 风险：`task package` 依赖当前开发环境中可用的 POSIX/Cygwin 工具，例如 `rm`、`mkdir`、`cp`、`chmod`、`env`、`zip`。
   - 缓解：这是对当前 Windows + Cygwin 环境的显式复用；Verification 阶段需要实际运行 `task package` 确认工具链可用。

## Assumptions

1. `yarn build` 产物目录为 `web/dist`。
2. 当前 Windows 主机具备 Go、Yarn、Task CLI，以及 Cygwin/POSIX 命令工具链。
3. 当前 Go dependencies 支持 `GOOS=windows GOARCH=amd64` 构建，且在 Windows 主机构建时不涉及跨 OS cgo 问题。
4. 用户接受第一版 zip 的双击入口是 `start.cmd`，不是直接双击 `termbridge.exe`。
5. 本轮 package 能力不包含安装器、签名和自动更新。

## User review notes

- 用户确认进入 Plan / 计划阶段。
- 用户补充要求：在 `Taskfile.yml` 添加 `task package` 命令完成该能力。
- 用户确认：`开始实现`，Plan / 计划状态更新为 `Accepted`。
- 用户反馈不接受在 `Taskfile.yml` 内塞大段脚本内容，也不接受额外新增 package PowerShell 脚本；实现改为 Taskfile 直接使用现有 Cygwin/POSIX 能力完成组装。
- 用户指出根目录已有 `.env.example`，不应再写一份；实现改为只复制根目录 `.env.example`，不在 `scripts/package` 下维护重复示例。
- 用户提出在 package 目录维护最小 `.env` 内容，打包时直接复制为分发包 `.env`；根目录 `.env.example` 也原样复制进包内，不修改。实现先采纳该方案，并简化 `start.cmd` / `start.sh`。
- 用户进一步修正为 local 环境方案：启动脚本设置 `TERMBRIDGE_ENV=local`，分发运行配置从 `.env` 改名为 `.env.local`，启动生成项写入 `configs/config.<env>.yaml`，local 场景即 `configs/config.local.yaml`，但生成逻辑不能写死 local。
- 用户指出端口/URL 的修改点应该只在 `.env.local`，不应硬编码在启动脚本；实现改为 `start.cmd` / `start.sh` 加载 `.env.local`，并从 `TERMBRIDGE_AGENT__PUBLIC_URL` 打开浏览器。
