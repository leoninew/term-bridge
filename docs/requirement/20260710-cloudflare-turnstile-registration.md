# Cloudflare Turnstile 登录与注册保护
最后修改时间: 2026-07-10 22:09:57

Review status: Accepted

## Background

Cloud 的邮箱密码注册入口目前只提交 `email` 与 `password`，后端直接创建待验证账户并发送邮箱验证码；登录入口同样只提交凭据并签发 JWT。两个公开端点均缺少人机验证，容易受到自动化批量注册、邮件投递滥用与凭据填充攻击影响。

本次引入 Cloudflare Turnstile，为 Cloud 邮箱密码登录和注册增加前端挑战与后端服务端校验。注册页的视觉层次参考 `D:\SourceCodes\mywork\typing-island\frontend\src\views\Register.vue` 中“邮箱输入框 + 操作区”的分组和反馈方式，但保持 TermBridge 现有主题变量、组件风格与注册流程，不复制其用户名、邮箱验证码发送、确认密码或 DaisyUI 样式。

此外，Cloud 登录表单新增 CSRF token 获取与随登录请求提交的机制；后端将对公开登录端点验证 CSRF token。现有仓库尚未存在 CSRF 实现或 hostname 校验抽象，因此本次会以独立、集中配置的能力接入，而不是复用不存在的既有实现。

## Goal

1. Cloud 邮箱密码登录和注册均必须携带有效的 Cloudflare Turnstile token；Cloud 后端完成服务端校验成功后，登录才可签发 JWT，注册才可创建账户或触发验证邮件。
2. Cloud 登录表单实现 CSRF token 获取和提交；后端对该公开登录请求验证 CSRF token，并使 CSRF 校验与 Turnstile 校验均先于凭据认证和 JWT 签发。
3. 登录和注册页展示 Turnstile 控件；注册页在既有简洁卡片布局内参考 typing-island 做适度美化，为验证区域提供明确标签、辅助说明、与表单一致的边框/间距及可理解的失败反馈。
4. 开发环境开箱即用 Cloudflare 官方 Turnstile 测试凭据，无需开发者另行申请或注入密钥。
5. 默认/生产配置仅保留 Turnstile 配置占位，不提交真实生产 site key 或 secret key；浏览器只接收 site key，secret key 只保留在 Cloud 后端配置中。
6. 生产环境缺失或不完整的 Turnstile 配置必须在配置校验阶段失败，阻止 Cloud 启动；不得通过业务 Handler 判断后静默放行。
7. Turnstile 状态与 token 失效时，登录和注册按钮不能发起请求；后端仍以服务端校验作为最终安全边界。
8. 对生产 Siteverify 成功响应实施 hostname 校验，且 hostname 必须与集中配置的预期 Cloud 对外域名精确匹配。

## Non-goal

- 不为 Google OAuth 注册/登录增加 Turnstile 或 CSRF token。
- 不改变邮箱验证、重发验证码、改密或密码重置的业务流程。
- 不照搬 typing-island 的用户名、邮箱验证码发送、确认密码、DaisyUI 样式或文案。
- 不扩大 CSRF 防护到本次登录端点以外的 API；后续若采用 Cookie 会话或新增浏览器可诱导的状态变更端点，应作为独立安全工作评估。
- 不在本次工作中接入 Cloudflare Analytics、Bot Management、WAF 规则或其他 Cloudflare 产品。
- 不持久化 Turnstile token、验证响应或用户 IP；token 仅随一次注册请求传输并用于即时校验。

## User scenarios

1. 开发者以 development 配置启动 Cloud 和 Web，登录与注册页自动加载 Cloudflare 官方测试 Turnstile 控件；完成挑战后可正常提交对应表单。
2. 访客填写登录或注册凭据但尚未完成 Turnstile：对应提交按钮不可用，页面提示需先完成人机验证。
3. 访客登录时，页面先获取 CSRF token，并与邮箱、密码、Turnstile token 一并提交；后端依次完成 CSRF 与 Siteverify 校验后，才进入既有凭据认证并签发 JWT。
4. 访客完成注册挑战后提交：前端将 token 和邮箱、密码一并发送；后端向 Turnstile Siteverify API 校验成功并通过 hostname 校验后，按既有逻辑创建待验证用户并发送验证邮件。
5. Turnstile 前端加载失败、挑战错误、token 过期或被重置：页面清除失效 token，阻止提交，并展示可理解的重试提示。
6. 攻击者绕过浏览器直接调用登录或注册 API、提交空 token、伪造 token、无效 token 或错误 CSRF token：Cloud 后端拒绝请求；不得签发 JWT、创建用户或发送验证邮件。
7. 生产部署者未替换默认占位配置：配置加载/校验直接失败，Cloud 不启动；不得在未启用服务端校验时静默放行登录或注册。
8. 生产部署中 Siteverify 返回的 hostname 与配置的 Cloud 对外 hostname 不匹配：后端拒绝登录或注册，不执行后续认证或账户创建。

## Acceptance

- [ ] Cloud 登录和注册请求契约包含 Turnstile token；前端、生成 DTO 与 Cloud Handler 能传递该字段。
- [ ] Cloud 登录请求契约包含 CSRF token；前端在提交登录前获取 token 并提交，Cloud 后端在凭据认证和 JWT 签发前验证它。
- [ ] Cloud 登录与注册 Handler 在调用 `authService.Login` / `authService.Register` 前完成服务端 Siteverify 校验；验证失败时不得执行凭据认证、JWT 签发、用户创建、验证码生成或邮件投递。
- [ ] 服务端验证请求使用 Cloudflare Siteverify HTTPS endpoint，且 secret key 永不出现在浏览器运行时配置、前端构建产物或 API 响应中。
- [ ] Siteverify 成功响应在生产环境必须含有与配置的预期 hostname 精确一致的 hostname；不一致或缺失时拒绝请求。开发测试凭据采用明确的开发配置规则，不将生产 hostname 约束错误施加于本地。
- [ ] 登录和注册页面仅在获得有效 Turnstile token 后允许提交；验证完成、错误、过期和重置事件均正确同步页面状态。
- [ ] 组件加载或挑战失败时，页面有可访问的文字反馈；登录或注册请求失败时不会遗留可继续提交的过期 token。
- [ ] 注册验证区域在窄屏和桌面宽度下均不溢出注册卡片，并遵从现有 `--color-*` 主题变量、边框和控件视觉语言。
- [ ] `configs/config.development.yaml` 使用 Cloudflare 官方公开测试 site key 与 test secret，使本地开发不依赖真实 Cloudflare 项目。
- [ ] `configs/config.yaml` 的生产基线只提供空的 Turnstile site key/secret key 与预期 hostname 占位和安全注释，不包含真实生产凭据。
- [ ] 配置加载对 Turnstile 的完整性、生产缺失配置和 hostname 合法性进行集中校验；生产环境缺失/不完整凭据或 hostname 时必须返回配置错误，业务 Handler 内不散落配置判断。
- [ ] 增加具备业务语义的后端测试：有效 CSRF 与有效 Turnstile 允许登录/注册；缺失/无效/验证服务失败/hostname 不匹配时不会签发 JWT、创建账户或投递邮件；配置不完整或生产策略不满足时被拒绝。
- [ ] 增加前端测试：未完成验证不可提交；登录 CSRF token 获取、Turnstile token 成功/过期/错误能驱动正确的提交状态和反馈。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- Turnstile 是 Cloud 公开邮箱密码登录与注册的附加安全校验，不替代既有邮箱验证、密码策略或 JWT 身份体系。
- 验证采用浏览器获取 token、Cloud 后端调用 Siteverify 的标准服务端校验模式；前端 token 仅用于改善交互，不能作为授权依据。
- 登录额外采用显式 CSRF token：前端先从 Cloud 取得 token，再将 token 随登录 JSON 请求提交；后端在尝试用户名密码认证前验证。尽管目前 Cloud token 经 `Authorization` header 保存而非 Cookie，该要求用于和参考登录表单的防护模式保持一致，并为未来浏览器认证演进建立明确边界。
- Turnstile 与 CSRF 的配置、token 生成/验证规则都集中在配置与专用安全组件层，Cloud Handler 只协调执行，不写内联正则或零散魔法值。
- site key 作为浏览器运行时配置的可公开字段，secret key 只由 Cloud 后端读取；CSRF token 不进入持久化存储、日志或 URL。
- development 配置直接使用 Cloudflare 官方测试凭据；默认生产基线只保留占位，真实凭据通过环境特定配置或环境变量提供。生产缺失、空白或不完整凭据（含 expected hostname）在配置校验阶段失败，阻止启动。
- 生产 Siteverify 响应的 hostname 必须精确匹配 `cloud.public_url` 解析出的 hostname；不额外引入重复 hostname 配置，从而避免配置漂移。development 的官方测试凭据通过环境识别不强制这一匹配。
- 注册页美化遵循现有 Tailwind 与 CSS 变量，不引入 DaisyUI 或 typing-island 的业务逻辑。

## Risk

- Turnstile script、CSRF token 端点或 Siteverify 服务不可用时会影响登录与注册可用性；必须提供明确错误反馈，且不能在失败时降级绕过服务端校验。
- Cloudflare test credential 仅用于 development，绝不能被 production 环境覆盖策略意外继承。
- Turnstile token 短时有效、单次使用且可能过期；前端需正确响应 expiry/error/reset，后端不得缓存或重试同一 token。
- CSRF token 的发放、过期、单次使用与前端刷新策略必须原子而明确，避免 token 重放、并发登录页打开或错误重试导致不可理解的失败。
- 新增公开运行时 site key、CSRF 端点与 proto 契约后，需要确保生成代码、服务端静态运行时配置和 Vite 开发模式保持一致。
- 生产 `cloud.public_url` 与 Turnstile dashboard 配置不一致时，合法用户将无法登录或注册；启动期 URL 校验和运行期 hostname 比对必须使用同一规范化规则。
