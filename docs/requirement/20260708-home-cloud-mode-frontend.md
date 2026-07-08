# Home 页 Cloud 模式前端界面需求
最后修改时间: 2026-07-08 17:55:25

Flow mode: light / 轻量模式
Stage: Requirement / 需求
Review status: Accepted

## Background

上一轮已经完成 Home 页 agent 模式的纯前端改版：统一顶部工具栏、居中卡片、语言/主题 icon button、云端连接状态 icon，以及工作区列表卡片。本轮继续做 Home 页 cloud 模式的纯前端重实现，保持与 agent 模式一致的视觉结构和交互密度。

项目技术约束保持不变：使用现有 `reka-ui`、Tailwind v4、`@lucide/vue`，不引入其他前端组件库。需要使用 reka-ui 时参考 `docs/reka-llms.txt`。

本轮调整后的信息架构为：agent 模式 Home 保持本机工作台；cloud 模式 Home 作为产品入口首页，不是管理后台，也不是企业/项目介绍页，不展示用户私有设备数据；cloud Dashboard 作为认证后的用户设备列表页面。由于本地 Agent 前端没有 `/cloud/...` 这些页面路径，cloud 模式页面路径使用无 `/cloud` 前缀的形式：`/dashboard` 与 `/devices/:deviceId/sessions`。

## Goal

1. Home 页 agent 模式保持当前已实现界面，不回退本机工作台、工作区列表、连接云端和切换 Cloud 模式入口。
2. Home 页 cloud 模式改为产品入口首页，不是管理后台，也不是企业/项目介绍页；在 1440×900 画布下使用约 1200px 内容区，尽量不截断、不滚动，不用大卡片承载整页内容。
3. Home 页 agent 模式右上角保持现有展示：主题、语言和云端连接状态。
4. Home 页 cloud 模式右上角展示主题、语言和账号区域：
   - 未登录时展示登录按钮，点击进入现有登录界面；
   - 已登录时展示用户名和向下箭头；
   - 已登录账号区域可展开下拉菜单，菜单项为“修改密码”和“退出登录”。
5. cloud 模式 Home 以产品入口和核心操作入口为主，不使用“项目/产品介绍”这类企业介绍式标题，不用大段说明文字撑页面；允许使用 `/image-gen` 生成主视觉素材，若生成失败则交付可复用的生成提示词。
6. cloud 模式 Home 可以尝试使用生成出的主视觉图片，但如果生成素材过于复杂或 AI 味明显，则不落地为正式页面素材；改用克制的临时矢量占位，页面文案不得生造具体产品承诺，涉及尚未确认的宣传文案、项目说明和资产说明仍使用明确 TODO 占位，后续再补充真实内容。
7. cloud 模式 Home 需要提供 Dashboard 入口：未登录时点击进入登录并在登录后回到 Dashboard；已登录时点击进入 Dashboard。
8. cloud 模式 Home 需要提供 Agent 入口：
   - `TERMBRIDGE_FRONTEND_MODE=hybrid` 时文案为“切换到 Agent 模式”，在 SPA 内切换；
   - `TERMBRIDGE_FRONTEND_MODE=cloud` 时文案为“打开 Agent 页面”，新开 Agent 页面。
9. 引入 cloud Dashboard 页面，路径为 `/dashboard`，route name 保持 `cloud-dashboard`，要求 Cloud 登录。
10. cloud Dashboard 展示当前用户相关的设备列表；每个设备列表项需要展示：
   - 设备名称；
   - 在线/离线连接状态；
   - 连接时间或最近在线时间；
   - 右侧进入按钮。
11. 点击设备右侧进入按钮时，选取该设备并进入设备会话页面，路径为 `/devices/:deviceId/sessions`，route name 保持 `cloud-sessions`。
12. 删除 agent dashboard 页面和路由，不再保留 `/agent/dashboard` 作为 agent dashboard 页面入口。
13. Cloud Home / Cloud Dashboard 不新增后端接口；设备列表、登录态、当前用户、退出登录、修改密码优先复用现有前端 API/store 能力。Agent 侧新增本地断开 Cloud 连接接口仅用于清理本机连接状态，不属于 Cloud 侧设备管理接口。
14. 保持字体字号规则与上一轮一致，遵循 `D:\SourceCodes\mywork\TermBridge\docs\guides\frontend-font-size-consistency.md`。
15. Agent 模式需要提供“断开云端连接”能力给前端使用：该动作通过独立的 `POST /agent-api/cloud/disconnect` 表达，只断开当前本机 Agent 与 Cloud 的本地连接状态，清理本机 `cloud_session` / `cloud_binding`，并停止当前 Cloud connector；它不是解绑，不调用 Cloud 侧设备删除接口，不删除 Cloud 账号下的设备绑定。
16. `/agent/sessions` 与 `/devices/:deviceId/sessions` 左上角需要展示适配当前侧栏头部高度的 Logo 链接，形式只保留蓝色块 + `TB`，点击回到对应 Home；两个会话页面右上角不再展示 Home / 账号类入口按钮。
17. Home agent 的“打开工作区”按钮和 Cloud Dashboard 的在线设备“进入工作台”按钮使用 primary 样式。

## Non-goal

1. 不引入 reka-ui、Tailwind v4、`@lucide/vue` 之外的新前端组件库。
2. 不新增 Cloud 侧解绑、删除设备或设备管理接口；Agent 断开云端连接不得复用 Cloud 侧解绑语义。
3. 不重新设计登录、注册、忘记密码、OAuth callback 等认证页面。
4. 不重新设计 cloud 会话页面；设备进入后仍使用已有 cloud sessions 页面组件。
5. 不在本轮实现新的设备管理能力，例如添加设备、删除设备、设备重命名或排序。
6. 不在 cloud Home 展示用户私有设备列表。
7. 不保留独立 agent dashboard 页面作为用户可访问入口。

## User scenarios

1. 用户进入 Home 页并处于 cloud 模式但未登录时，可以看到产品/项目宣传内容、登录按钮和 Dashboard 入口；点击 Dashboard 入口进入登录，登录后回到 Dashboard。
2. 用户进入 Home 页并处于 cloud 模式且已登录时，可以在右上角看到用户名和向下箭头；点击账号区域后看到“修改密码”和“退出登录”菜单项。
3. 已登录 cloud 用户可以从 Home 页 CTA 进入 Dashboard，在 Dashboard 中看到当前账号关联的设备列表，快速判断设备在线状态和连接/最近在线时间。
4. 用户点击 Dashboard 中在线设备右侧进入按钮后，系统选取该设备并进入该设备的会话页面 `/devices/:deviceId/sessions`。
5. 离线设备保留在 Dashboard 列表中用于确认状态，但进入按钮不可用或展示离线态，不进入会话。
6. 用户访问 Home 页时，根据当前 agent/cloud 模式看到对应 Home 界面，而不是被自动带到历史 agent dashboard 页面。
7. 在 hybrid 前端中，用户可以从 cloud Home 的 Agent 入口 SPA 切换回 agent Home；在 cloud-only 前端中，该入口新开 Agent 页面。
8. Agent 模式已经连接 Cloud 时，用户点击“断开云端”后，本机不再展示已连接状态，Cloud connector 停止连接；Cloud 账号下的设备绑定仍保留，后续重新连接时可继续复用 Cloud 侧绑定语义。
9. 用户进入 `/agent/sessions` 或 `/devices/:deviceId/sessions` 时，可通过左上角 `TB` 蓝块返回对应 Home；右上角不再提供额外入口按钮。

## Acceptance

1. Home 页 agent 模式保持当前实现和行为。
2. Home 页 cloud 模式是产品入口首页，不是管理后台，也不是企业/项目介绍页，不加载或展示当前用户设备列表；主视觉优先尝试使用 `/image-gen` 生成素材，若生成失败则交付提示词；若生成结果过于复杂或 AI 味明显，则不用作页面主图，改为临时矢量占位；尚未确认的宣传文案和项目说明使用明确 TODO 占位，不生造具体内容。
3. cloud Home 右上角未登录时展示登录按钮，点击跳转到现有 `cloud-login`。
4. cloud Home 右上角已登录时展示用户名和向下箭头，并通过 reka-ui dropdown 展示“修改密码”和“退出登录”；账号按钮使用与顶部其他按钮一致的轻量样式。
5. “退出登录”复用现有 cloud logout 能力和 token 清理逻辑；退出后回到未登录状态或登录页，不残留已登录用户信息。
6. “修改密码”复用现有 `authChangePassword` 能力；如果现有设置页不足以承载，本轮可用弹窗表单承载，不新增后端契约。
7. cloud Home 存在 Dashboard 入口；未登录点击时走登录并回到 Dashboard，已登录点击时直接进入 `/dashboard`。
8. cloud Home 存在 Agent 入口：hybrid 下展示“切换到 Agent 模式”并 SPA 切换，cloud-only 下展示“打开 Agent 页面”并新开 Agent 页面。
9. `/dashboard` route 存在，要求 Cloud 登录，未登录访问会跳转到登录并带 redirect。
10. `/dashboard` 展示设备列表，设备项包含名称、在线/离线状态、连接时间或最近在线时间、右侧进入按钮。
11. 在线设备进入按钮可选取设备并跳转到 `/devices/:deviceId/sessions`；离线设备不可进入。
12. `/devices/:deviceId/sessions` route 存在并复用现有 cloud sessions 页面能力。
13. `/agent/dashboard` route 和 agent dashboard 页面被删除；相关跳转、鉴权兜底、OAuth2 回跳更新为 Home 页或其他明确目标。
14. 实现不引入新的前端组件库。
15. 字号和加粗规则符合参考指南：页面根使用 `text-sm`，标题使用 `text-lg font-semibold`，非标题文本不加粗且不引入额外字号层级。
16. 本轮完成后需要进入 Verification / 验证阶段时，验证需覆盖类型检查、实际 diff 范围、路由路径变更、Dashboard 认证要求和 agent/cloud 两种 Home 模式入口行为。
17. Agent 模式断开云端连接后，本机 `auth/me` 不再返回 `cloud_session`，本机设备身份仍保留，Cloud 侧不收到解绑/删除设备请求，前端按钮回到“连接云端”入口。
18. `/agent/sessions` 与 `/devices/:deviceId/sessions` 左上角展示 `TB` 蓝块 Logo 链接，尺寸适配侧栏当前 `h-11` 头部高度；两个会话页面右上角不展示额外 Home / 账号入口按钮。
19. Home agent 的“打开工作区”按钮和 Cloud Dashboard 的在线设备“进入工作台”按钮使用 primary 样式。

## Open questions

暂无阻塞实现的问题。以下事项按用户决策进入实现：

1. “修改密码”菜单项检查现有注册、登录、修改密码相关实现；已经就绪则复用，存在缺口或现有实现不合理则在实现结果中说明。
2. `/agent/dashboard` 彻底删除路由，不做旧路径兼容 redirect。
3. cloud Dashboard 路径使用 `/dashboard`，设备会话路径使用 `/devices/:deviceId/sessions`，不使用 `/cloud/...` 前缀。

## Decisions

1. 本轮继续采用 light / 轻量模式。
2. 本轮是纯前端任务，不新增后端接口，不改后端契约。
3. 技术栈边界固定为 reka-ui + Tailwind v4 + `@lucide/vue`。
4. cloud Home 定位为产品入口首页，不是管理后台，也不是企业/项目介绍页；cloud Dashboard 定位为认证后的用户设备列表页。
5. cloud 模式优先复用现有 `gateway` store 的登录态、用户信息、设备列表、设备选择和 cloud sessions 页面。
6. Home 页作为 agent/cloud 的模式入口；cloud-only 前端不能 SPA 切换到 agent 模式时，通过新页面打开 Agent 页面。
7. 已有 agent 模式 Home 设计不应被重写成另一套风格；cloud 模式需要复用其视觉语言和组件组织。
8. 设备时间展示沿用现有语义：在线且有 `connected_at` 时展示连接时间；否则有 `last_seen` 时展示最近在线时间；都缺失时展示无连接记录。

## Risk

1. 路由路径从 `/cloud/dashboard`、`/cloud/devices/:deviceId/sessions` 调整为 `/dashboard`、`/devices/:deviceId/sessions`，需要同步登录回跳、鉴权守卫、会话页 login redirect 和旧引用。
2. “修改密码”现有 Settings 页当前不足以承载完整流程；本轮用弹窗表单会增加前端状态和错误处理复杂度。
3. cloud Home 不展示设备列表后，需要确保 Dashboard CTA 明确，避免已登录用户找不到设备入口。
4. agent 模式已完成验收的 Home 页不应因 cloud Home/Dashboard 调整产生视觉/行为回退。
5. cloud Home 页面不应被限制在居中的小区域；在 1440×900 桌面视口使用约 1200px 内容区，尽量不截断、不滚动，且不使用整页大卡片作为承载。

## User review notes

用户采纳信息架构调整：agent 模式 Home 保持当前实现；cloud 模式 Home 提供 Dashboard 入口；cloud 模式引入认证后的 Dashboard 展示用户设备列表；cloud 页面路径不使用 `/cloud` 前缀，`/cloud/dashboard` 改为 `/dashboard`，`/cloud/devices/:deviceId/sessions` 改为 `/devices/:deviceId/sessions`。用户补充要求：产品相关页面会涉及图片和文案，本轮不要生造，先用明确 TODO 占位，后续再补。用户进一步要求 cloud Home 不是管理后台，也不是企业/项目介绍页，应作为产品入口首页，并尝试使用 `/image-gen` 生成所需素材；如果生成失败，则展示提示词供用户自行生成。经 Pomelo PW 检查后，用户指出页面被放在中间小区域且生成主图过于复杂、AI 味明显；后续又明确要求 1440×900 画布、约 1200px 内容区、尽量不截断和滚动、不使用整页卡片、不使用“项目/产品介绍”这类标题或大段介绍文字。用户新增 Agent 侧断开云端连接需求，并强调不是解绑；实现应使用独立 `POST /agent-api/cloud/disconnect`，不要通过同一路由的不同 HTTP 谓词表达条件分支。用户随后要求 `/agent/sessions` 和 `/devices/:deviceId/sessions` 左上角展示适配高度的 `TB` 蓝块 Logo 链接，移除两个会话页面右上角入口，并将 Home agent / Cloud Dashboard 的打开工作区或进入工作台按钮改为 primary。Requirement / 需求视为 Accepted，继续 Implementation / 实现。
