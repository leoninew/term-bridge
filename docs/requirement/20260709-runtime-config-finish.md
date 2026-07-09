# runtime config 与 Home 入口收尾
最后修改时间: 2026-07-09 11:10:35

Flow mode: light / 轻量模式
Stage: Requirement / 需求
Review status: Accepted

## Background

本需求合并以下三份已接受 requirement：

- `20260708-runtime-config-injection.md`
- `20260708-home-cloud-mode-frontend.md`
- `20260709-runtime-config-finish.md`

前置工作已经完成 Home 页 agent/cloud 模式重构、Cloud Dashboard / device sessions 路由调整，以及前端 runtime config 收敛。当前统一记录最终边界：前端配置由 `runtimeConfig` store 统一解析和校验；静态入口差异由 Vite mode 与对应 `web/.env.<mode>` 在构建期注入；Home 页只承担 agent/cloud 入口职责，不继续扩展成业务状态页或管理后台。

## Goal

- `web/src/config.ts` 只保留类型定义，不承担读取 env、构建 URL、fallback 或默认值逻辑。
- `web/src/store/runtimeConfig.ts` 成为前端运行时配置唯一解析、校验和状态边界。
- `runtimeConfig` store 统一读取 `window.__CONFIG__` 或 `import.meta.env`，初始化阶段完成配置解析；缺失字段、非法 URL、非法 `agent.mode`、空 OAuth scopes 等配置错误直接 fail fast。
- 使用后端风格前端环境变量名：
  - `TERMBRIDGE_AGENT__MODE`
  - `TERMBRIDGE_AGENT__PUBLIC_URL`
  - `TERMBRIDGE_AGENT__API_BASE_URL`
  - `TERMBRIDGE_AGENT__OAUTH__CLIENT_ID`
  - `TERMBRIDGE_AGENT__OAUTH__REDIRECT_URL`
  - `TERMBRIDGE_AGENT__OAUTH__SCOPES`
  - `TERMBRIDGE_CLOUD__PUBLIC_URL`
  - `TERMBRIDGE_CLOUD__API_BASE_URL`
- `agent.mode` 是只读部署/能力配置，取值为 `agent | cloud | hybrid`。
- 页面当前模式由 `runtimeConfig.view.mode` 表达，取值仅为 `agent | cloud`；只允许在 `agent.mode === 'hybrid'` 时切换。
- `gateway` store 只负责 token、认证状态、cloud session 和设备列表，不再派生或持有运行模式、runtime target、current device 等模式职责。
- 前端构建入口收敛为三类：
  - `web/.env.development`：本地联调入口，`TERMBRIDGE_AGENT__MODE=hybrid`。
  - `web/.env.agent`：Agent 制品入口，`TERMBRIDGE_AGENT__MODE=agent`。
  - `web/.env.cloud`：Cloud 镜像入口，`TERMBRIDGE_AGENT__MODE=cloud`。
- 不再使用 `web/.env.production` 作为前端制品配置；Docker runtime env 只影响 Go 后端配置，不改写已经构建进 JS 的前端公开配置。
- Docker 镜像默认提供 Cloud 静态入口，因此镜像构建使用 `build:cloud`。
- Home 页 agent 模式保持本机工作台入口，包括工作区列表、连接/断开 Cloud、进入本机 session，以及仅 hybrid 可见的“切换到 Cloud 模式”。
- Home 页 cloud 模式作为产品入口首页，不展示业务状态卡片，不展示用户私有设备列表，不为首页入口拉取设备信息；设备列表属于 `/dashboard`。
- Cloud Home 保留核心入口：Cloud 登录/进入工作台或设备入口、Agent 入口；不再保留独立“打开设备 Dashboard”重复按钮。
- Cloud Dashboard 路径为 `/dashboard`，route name 为 `cloud-dashboard`，要求 Cloud 登录。
- Cloud device sessions 路径为 `/devices/:deviceId/sessions`，route name 为 `cloud-sessions`。
- 删除独立 agent dashboard 页面和 `/agent/dashboard` 用户可访问入口。
- Agent 模式提供“断开云端连接”能力：通过独立 `POST /agent-api/cloud/disconnect` 清理本机 `cloud_session` / `cloud_binding` 并停止当前 Cloud connector；该动作不是解绑，不删除 Cloud 账号下的设备绑定。
- Agent sessions 路径为 `/sessions`，route name 仍为 `agent-sessions`，不保留 `/agent/sessions` 兼容旧路径。
- `/sessions` 与 `/devices/:deviceId/sessions` 左上角展示 `TB` Logo 链接回对应 Home；两个会话页面右上角不展示额外 Home / 账号入口按钮。

## Non-goal

- 不保留旧变量名、旧扁平配置结构或兼容 fallback 链。
- 不恢复 `appMode.ts`，也不新增单独的 `runtimeMode.ts` / `XXMode.ts`。
- 不把 `agent.mode` 加入 Go 后端共享配置模型；它是前端构建入口语义。
- 不做后端 HTML runtime config 注入，不替换 `index.html`，不在后端拼接或兜底前端配置。
- 不向浏览器注入任何 secret，包括 `TERMBRIDGE_AGENT__OAUTH__CLIENT_SECRET`。
- 不让 `gateway` 恢复模式派生职责。
- 不新增 Cloud 侧解绑、删除设备或设备管理接口。
- 不重新设计登录、注册、忘记密码、OAuth callback 等认证页面。
- 不重新设计 cloud sessions 页面；设备进入后仍使用已有 sessions 页面组件。
- 不在 cloud Home 展示用户私有设备列表或业务状态卡片。
- 不保留独立 agent dashboard 页面作为用户可访问入口。
- 不把 Docker runtime env 解释为可以改变已构建前端 JS 的配置来源。

## User scenarios

- 作为本地开发用户，我通过 `web/.env.development` 使用 `hybrid`，在同一个 Vite dev server 中联调 agent/cloud 前端。
- 作为 Agent 制品构建用户，我使用 `build:agent` 得到 `agent.mode=agent` 的静态入口。
- 作为 Cloud 镜像部署用户，我使用 `build:cloud` 得到 `agent.mode=cloud` 的静态入口。
- 作为维护者，我希望配置错误在初始化阶段立即失败，而不是靠隐式 fallback 继续运行。
- 作为维护者，我希望页面模式切换与部署能力配置分离，避免 `agent.mode` 被误当作当前 UI 状态。
- 作为 Agent 入口用户，我在非 `hybrid` 构建下连接 Cloud 后，不应看到“切换到 Cloud 模式”按钮。
- 作为本地联调用户，我在 `agent.mode=hybrid` 时可以看到“切换到 Cloud 模式”按钮，并在 Agent / Cloud 页面模式之间切换。
- 作为 Cloud 入口用户，我在 Home 页只看到入口操作；设备列表和设备状态在 `/dashboard` 中处理。
- 作为 Cloud 用户，我点击在线设备进入按钮后进入 `/devices/:deviceId/sessions`。
- 作为 Agent 用户，我断开 Cloud 后，本机不再展示已连接状态，Cloud connector 停止连接；Cloud 账号下的设备绑定仍保留。

## Acceptance

- `config.ts` 不包含运行时读取、构建 URL、fallback 或默认值逻辑，只包含类型。
- 全局 `import.meta.env` 和 `window.__CONFIG__` 读取集中在 `runtimeConfig` store。
- `runtimeConfig` store 对缺失字段、非法 URL、非法 `agent.mode`、空 OAuth scopes 等配置错误直接抛错。
- 前端不再出现 `TERMBRIDGE_FRONTEND_MODE`、`both`、`FrontendMode`、旧 `ApiTarget` 等旧语义。
- 前端 UI 当前模式通过 `runtimeConfig.view.mode` 读取，通过 `runtimeConfig.switchMode(mode)` 切换。
- 非 `hybrid` 部署下，`switchMode` 拒绝切换到与 `agent.mode` 不一致的页面模式。
- `gateway.ts` 不导出 `runtimeTarget`、`currentDevice` 或模式派生 token computed。
- `web/.env.development` 中 `TERMBRIDGE_AGENT__MODE=hybrid`。
- `web/.env.agent` 中 `TERMBRIDGE_AGENT__MODE=agent`。
- `web/.env.cloud` 中 `TERMBRIDGE_AGENT__MODE=cloud`。
- `web/.env.production` 不再作为前端制品配置保留。
- `web/package.json` 提供明确产品线构建入口：`build:agent` 与 `build:cloud`。
- Dockerfile 和 Dockerfile.cn 使用 `yarn build:cloud` 构建镜像内置前端。
- `web/index.html` 保留后端注入占位和空 `window.__CONFIG__` 初始化；空对象不应阻断 build-time env。
- 后端静态服务不修改 `index.html`，继续原样返回构建产物。
- 非 `hybrid` 的 Agent-only 入口不显示“切换到 Cloud 模式”按钮。
- `hybrid` 模式的本地页面模式切换按钮保持可见可用。
- Cloud Home 不展示“账号会话 / 设备在线状态 / 工作台路由”等业务状态卡片。
- Cloud Home 不展示用户私有设备列表，不拉取设备列表，不为了首页入口直跳设备会话列表。
- Cloud Home 不保留独立“打开设备 Dashboard”重复按钮；进入设备或设备列表的入口以主 CTA/明确入口表达。
- Cloud Home 保留 Agent 入口：hybrid 下 SPA 切换到 Agent 模式；cloud-only 下新开 Agent 页面。
- `/dashboard` route 存在，要求 Cloud 登录，未登录访问会跳转到登录并带 redirect。
- `/dashboard` 展示设备列表，设备项包含名称、在线/离线状态、连接时间或最近在线时间、右侧进入按钮。
- 在线设备进入按钮可选取设备并跳转到 `/devices/:deviceId/sessions`；离线设备不可进入。
- `/devices/:deviceId/sessions` route 存在并复用 cloud sessions 页面能力。
- `/agent/dashboard` route 和 agent dashboard 页面被删除；相关跳转、鉴权兜底、OAuth2 回跳更新为 Home 页或其他明确目标。
- Agent 模式断开云端连接后，本机 `auth/me` 不再返回 `cloud_session`，本机设备身份仍保留，Cloud 侧不收到解绑/删除设备请求。
- `/sessions` 与 `/devices/:deviceId/sessions` 左上角展示 `TB` Logo 链接；两个会话页面右上角不展示额外 Home / 账号入口按钮。
- 卡片标题右上角操作使用融入标题区的 text action / link 样式，不使用边框、背景或蓝色实心按钮；设备列表项右侧进入工作台保留无边框箭头图标。
- 根目录 `.env` / `.env.<env>` 是后端/本机运行覆盖文件，默认忽略；可从 `.env.example` 复制维护。
- 前端 `web/.env.*` 与根目录 `.env.*` 职责分离：前者是构建输入，后者是本机运行覆盖。
- 相关前端 typecheck、前端测试、后端配置测试和后端静态服务测试通过。

## Open questions

- 暂无阻塞实现的问题。后续如需要把 Cloud 最终公网域名从 Docker runtime env 注入到已构建前端，需要另行设计 `window.__CONFIG__` 注入或等价机制；本轮不实现。

## Decisions

- 使用 light / 轻量模式处理本组收尾任务。
- 前端运行时配置以嵌套 `RuntimeConfig` 为唯一结构。
- 不做兼容适配，配置错误直接 fail fast。
- `client_secret` 仅供 Agent 后端使用，不进入浏览器运行时配置。
- `gateway` 与 `runtimeConfig` 职责分离：前者认证/设备，后者配置/页面模式。
- 移除后端复杂配置注入，使用前端 `build:agent` / `build:cloud` 和对应 `web/.env.<mode>` 完成静态入口配置。
- `web/.env.development` 使用 `hybrid` 是仓库内明确的本地联调入口，不再要求 development 入口避免 `hybrid`。
- “切换到 Cloud 模式”是开发环境 agent/cloud 共用一个地址时的解决方式，预期当且仅当 `TERMBRIDGE_AGENT__MODE=hybrid` 时可见可用。
- Cloud Home 定位为产品入口首页，不是管理后台，也不是企业/项目介绍页；业务数据进入 `/dashboard`。
- Cloud Dashboard 路径使用 `/dashboard`，设备会话路径使用 `/devices/:deviceId/sessions`，不使用 `/cloud/...` 前缀。
- `/agent/dashboard` 彻底删除路由，不做旧路径兼容 redirect。
- Agent 断开 Cloud 是本机连接状态清理，不是 Cloud 侧解绑或删除设备。
- Root `.env.*` 继续忽略；`web/.env.development`、`web/.env.agent`、`web/.env.cloud` 作为前端构建输入提交；`web/.env.production` 删除并忽略。

## Risk

- Vite build-time 配置会固化进 JS；镜像必须使用正确入口构建，不能依赖 Docker runtime env 改写已构建前端配置。
- 如果制品构建配置中重新出现 `hybrid`，Cloud/Agent 专一入口语义会被破坏。
- 如果 store 误把空 `window.__CONFIG__` 当成有效注入对象，会导致构建期 env 被绕过并 fail fast。
- 如果继续允许业务模块直接读取 env 或 `window.__CONFIG__`，会破坏集中配置边界。
- Cloud Home 入口应保持轻量；如果重新加入设备状态或账号状态卡片，会让 Home 承担业务页面职责。
- 根目录 `.env.production`、`.env.development` 可能包含真实密钥，不能纳入版本管理。
- `TERMBRIDGE_ENV` 来自 OS env，shell、CI、systemd 或启动脚本中残留值会改变后端配置加载目标。

## User review notes

- 用户要求使用 `/specflow light` 记录和推进 runtime config 收尾任务。
- 用户明确：`appMode.ts` 删除，不继续修；不需要额外 `XXMode.ts`；`runtimeConfig` 和 store 承担配置/页面模式职责。
- 用户明确：`agent.mode` 是只读配置，页面切换使用单独 `view.mode`。
- 用户明确：`gateway.ts` 的职责重叠要消除。
- 用户明确：Cloud 静态入口缺配置不是通过后端曲线解决，而是前端入口配置问题。
- 用户明确：既然已有 `build:agent` / `build:cloud`，移除后端复杂配置注入，前端构建期注入即可。
- 用户明确：`index.html` 后端注入占位和空 `window.__CONFIG__` 保留，当前用不上但不删除。
- 用户确认：“切换到 Cloud 模式”是开发环境 agent/cloud 共用一个地址的解决方式，预期当且仅当 `TERMBRIDGE_AGENT__MODE=hybrid` 时可见可用。
- 用户要求移除 Cloud 模式首页“打开设备 Dashboard”相关重复内容，但保留进入设备/工作台的入口。
- 用户要求 Cloud Home 不承担业务功能，移除“账号会话 / 设备在线状态 / 工作台路由”卡片。
- 用户采纳 env mode 收敛建议：开发环境为 hybrid，Agent 制品与 Cloud 镜像使用独立构建入口，不再使用 `web/.env.production`。
- 用户纠正：项目现有本地端口是 `localhost:9030`，不能擅自改为 `localhost:3000`。
- 用户要求 README 整体简化，不需要过多配置细节。
- 用户要求审视根目录 `.env.xx`、开发环境 Air 启动、该删除的删除、该忽略的忽略。
