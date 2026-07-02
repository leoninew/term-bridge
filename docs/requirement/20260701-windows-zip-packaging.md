# Windows EXE Static Directory Zip Packaging Requirement
最后修改时间: 2026-07-02 13:32:16

## Review status

Accepted

## Flow mode / Stage

标准模式 / standard；需求 / Requirement 已接受，当前进入计划 / Plan。

## Background

用户希望分析并推进 TermBridge-go 的 Windows 可分发应用打包方案。项目当前已经具备：

- Go CLI 入口 `cmd/termbridge/main.go`，支持 `termbridge serve` 和 `termbridge exec -- <command>` 等命令。
- Go 后端 HTTP server 可通过 `server.static_dir` 寄宿静态页面。
- Vue/Vite 前端可构建为静态资源目录。
- 后端已有 PTY/runtime/session 体系，可运行其他 CLI 命令并通过 Web terminal 使用。

本任务在标准模式 / standard 下记录需求、可用方案和方案评估边界；用户指定优先按“打包成 exe + 静态目录”的 zip 包形式进行评估，并要求后续在 `Taskfile.yml` 添加 `task package` 命令完成该能力。

## Goal

1. 明确 TermBridge-go 在 Windows x64 上以 zip 包形式分发的目标和约束。
2. 记录可用 Windows 应用分发方案，并对比其适配度、复杂度和风险。
3. 将首选评估与实现方向限定为：

   ```text
   zip 包
   ├── termbridge.exe
   ├── web/ 或 dist/ 静态资源目录
   ├── configs/config.yaml
   ├── .env.local
   ├── .env.example
   ├── start.cmd
   └── start.sh
   ```

4. 评估并实现该 zip 包形式满足：
   - 可以运行其他 CLI 命令。
   - 可以本地寄宿并提供页面服务。
   - 用户不需要安装 Go、Node、yarn 即可运行。
   - Windows 用户可双击启动入口并自动打开浏览器。
5. 在 `Taskfile.yml` 添加 `package` task，用于构建 Windows x64 zip 分发包。
6. 为后续 Implementation / 实现和 Verification / 验证准备实施步骤和验证策略。

## Non-goal

1. 本任务不制作安装器、不引入 Electron/Tauri/Wails/WebView2 桌面壳。
2. 本任务不做单 exe 内嵌静态资源实现；该方案仅作为可选方案记录。
3. 本任务不设计自动更新、代码签名、Windows Defender 信誉、企业部署策略。
4. 本任务仅评估并实现 Windows x64 zip 包；不覆盖 Windows arm64。
5. 本任务不执行 git 写操作。

## User scenarios

1. 作为 Windows x64 用户，我可以下载一个 zip 包，解压后双击 `start.cmd`，自动打开浏览器并访问本地 TermBridge 页面。
2. 作为本地用户，我可以在浏览器中访问 TermBridge 本地页面，页面由同一个 Go 后端或明确配置的本地后端提供。
3. 作为 CLI 用户，我可以通过页面创建终端 session，运行本机可用的 CLI 命令，例如 `pwsh`、`cmd`、`git`、`claude` 或其他 PATH 中的命令。
4. 作为维护者，我可以运行 `task package` 复现分发包构建流程，并明确知道哪些文件应该进入 zip、哪些文件不能进入 zip。
5. 作为调试者，我可以通过日志、控制台输出或文档说明定位端口占用、静态目录缺失、配置错误等启动失败原因。
6. 作为非 Windows 主机上的维护者，我可以查看或运行随包提供的 `start.sh` 作为便携启动入口；但本轮分发目标和验证重点仍是 Windows x64。

## Available packaging options

### Option A: exe + static directory zip package

形式：

```text
TermBridge-windows-x64.zip
└── TermBridge/
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

适配度：高。

优点：

- 与当前架构最匹配；后端已经支持 `server.static_dir`。
- 不需要重构为桌面应用。
- 构建和验证成本最低。
- 静态资源独立于 exe，便于快速检查和替换。
- 适合作为第一版 portable 分发物。

限制和风险：

- 不是单文件分发；用户可能误删 `web/` 或 `configs/`。
- 双击体验依赖 `start.cmd`；`termbridge.exe` 本身仍是 CLI 程序。
- 需要明确 cwd、配置文件路径和 `server.static_dir` 相对路径规则。
- 分发包只包含 package 专用 `.env.local` 和根目录 `.env.example` 参考文件；不能包含开发者本机 `.env`、数据库、日志、`.termbridge` runtime state 或 node_modules。

结论：作为当前任务的首选评估和实现方案。

### Option B: single exe with embedded static assets

形式：

```text
termbridge.exe
```

适配度：中高，但需要代码改造。

优点：

- 用户只需一个 exe。
- 不会出现静态目录缺失。
- 更适合长期 portable 分发。

限制和风险：

- 需要用 Go `embed.FS` 改造 HTTP static server。
- 需要保留开发模式的外部静态目录或 Vite proxy 兼容。
- 本任务用户指定优先评估 exe + 静态目录，因此不作为本轮实施首选。

### Option C: installer package

形式：Inno Setup / WiX / MSIX 安装包。

适配度：中。

优点：

- 更符合普通 Windows 应用安装体验。
- 可创建开始菜单、桌面快捷方式、卸载器。

限制和风险：

- 比 zip 多安装器维护成本。
- 仍需要先确定 exe + 静态资源如何运行。
- 不解决核心运行与寄宿页面问题，只是分发外壳。

### Option D: desktop shell with Wails / Tauri / Electron

适配度：中低，现阶段不优先。

优点：

- 可以提供原生窗口、托盘、菜单等桌面体验。

限制和风险：

- 引入新运行模型和打包链路。
- 可能需要调整当前本地 HTTP/WebSocket 架构。
- 对“先验证 zip 包分发”来说过重。

### Option E: Windows Service

适配度：低。

原因：

- Windows Service 与用户桌面 session 隔离，不适合运行交互式 CLI / PTY。
- 用户环境变量、PATH、权限和进程交互都会更复杂。

## Selected evaluation direction

本任务按用户指定优先评估并实现：**exe + 静态目录 zip 包**。

评估和实现重点：

1. 当前代码是否已经支持通过 `termbridge.exe serve` 本地寄宿静态页面。
2. zip 包内应包含哪些最小文件。
3. 分发配置如何通过 `.env.local` 设置 `server.static_dir`、`server.listen_url`、`server.public_url` 和数据库/日志/runtime 路径。
4. 启动时 cwd 对配置和静态目录解析的影响。
5. Windows 上运行其他 CLI 命令时对 PATH、工作目录、PTY 和进程清理的要求。
6. 应排除哪些开发产物和本地状态。
7. `Taskfile.yml` 中 `package` task 如何跨平台组装 Windows x64 zip。
8. `start.cmd` 如何双击启动服务并打开浏览器。

## Acceptance

- [ ] Requirement 文档记录 Windows 分发目标、非目标、用户场景、可用方案和首选评估方向。
- [ ] Plan / 计划阶段给出 exe + 静态目录 zip 包的候选目录结构。
- [ ] Plan / 计划阶段给出构建命令草案：前端 build、Go build、zip 组装。
- [ ] Plan / 计划阶段明确哪些文件进入 zip，哪些文件必须排除。
- [ ] Plan / 计划阶段评估现有配置是否能直接支持 `server.static_dir` 指向分发静态目录。
- [ ] Plan / 计划阶段纳入 `Taskfile.yml` 新增 `package` task 的实现方案。
- [ ] Implementation / 实现阶段在 `Taskfile.yml` 添加 `package` task。
- [ ] Implementation / 实现阶段生成或纳入 `start.cmd` 和 `start.sh` 到 zip 包。
- [ ] 生成的 zip 包面向 Windows x64，包含 `termbridge.exe`、静态页面目录、`configs/config.yaml`、`.env.local`、`.env.example`、`start.cmd`、`start.sh`。
- [ ] `start.cmd` 支持双击后设置 `TERMBRIDGE_ENV=local`、加载 `.env.local`、启动本地服务并打开浏览器。
- [ ] 启动生成的 `TERMBRIDGE_JWT__SECRET_KEY` 写入 `configs/config.<env>.yaml`；local package 场景为 `configs/config.local.yaml`，且不能写入 `.env.local`；设备身份生成并读取自 `<runtime.state_dir>/agent.json`。
- [ ] Verification / 验证阶段至少验证 zip 包能被创建，且包内文件结构符合预期。
- [ ] Verification / 验证阶段如可执行，应验证包内 exe 能启动本地服务并能访问静态页面。
- [ ] Verification / 验证阶段如可执行，应验证通过页面或 API 创建/运行 CLI session 的最小路径，或记录无法自动验证的原因。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- 用户指定流程为标准模式 / standard。
- 用户要求开始任务并记录可用方案，因此创建新的 requirement 文档。
- 用户指定首选评估方向为“打包成 exe + 静态目录”的 zip 包形式。
- 用户确认 zip 包要求“双击即可打开浏览器”。
- 用户确认允许随包提供 `start.cmd` 和 `start.sh`。
- 用户确认分发配置复用 `configs/config.yaml`。
- 用户确认只评估 Windows x64。
- 用户要求进入 Plan / 计划阶段，Requirement / 需求状态更新为 `Accepted`。
- 用户追加要求：在 `Taskfile.yml` 添加 `task package` 命令完成该能力。
- 用户后续修正 package 配置策略：根目录 `.env.example` 原样复制进包内，package 运行配置使用 `.env.local`；`start.cmd` / `start.sh` 只指定 `TERMBRIDGE_ENV=local` 并加载 `.env.local`，端口/URL 修改点不写在脚本里。
- 用户后续修正启动生成配置策略：生成项写入 `configs/config.<env>.yaml`，local package 场景为 `configs/config.local.yaml`；实现必须按 `<env>` 泛化，不能写死 local。

## Risk

1. 当前配置加载以 cwd 下 `configs/config.yaml` 为默认入口；zip 分发时如果用户从不同目录启动 exe，可能导致配置路径不稳定。
2. 当前 `server.static_dir` 为空时只提供 API；zip 分发需要明确设置为静态资源目录。
3. 如果分发版前端和后端同源运行，`server.public_url` 应从开发默认的 `http://localhost:9031` 调整为本地后端地址，否则页面/回跳 URL 可能不一致。
4. 运行其他 CLI 命令依赖用户机器上的 PATH 和命令安装状态；zip 包不能保证第三方 CLI 存在。
5. Windows 交互终端、进程树清理和杀进程行为需要实机验证，不能只依赖静态分析。
6. 本地 HTTP API 具备运行用户命令的能力，分发版必须避免默认监听 `0.0.0.0`，并需要关注本地访问认证、Origin/Host 校验等安全边界。
7. `start.cmd` 自动打开浏览器需要处理服务尚未 ready 的竞态；可先用短暂等待或后台启动后打开固定 URL，后续再强化健康检查。
8. `task package` 如果交叉编译 Windows x64，必须确认依赖和 PTY 实现在当前构建主机上可交叉编译；否则需要限定在 Windows 主机运行或调整构建策略。

## User review notes

- 用户原始指令：`分析如何将本项目打包一个可以分发的 windows 应用 - 可以运行其他 cli 命令 - 可以本地寄宿、提供页面服务`。
- 用户随后指定：`/specflow 标准模式 开始任务，文档记录可用方案。然后按’打包成 exe+静态目录‘ zip 包形式 评估`。
- 用户确认未决事项：双击打开浏览器；提供 `start.cmd` 和 `start.sh`；复用 `configs/config.yaml`；只评估 x64 Windows；进入 Plan。
- 用户追加：`然后我们要在 TaskFile yaml 添加 task package 命令完成这个能力`。
