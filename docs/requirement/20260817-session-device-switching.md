# 会话页设备切换
最后修改时间: 2026-08-17 13:16:07

Review status: Accepted

## Background

本机 `/sessions` 已显示当前 PC 名称和云端连接状态，但已连接云端的用户不能从该位置进入其他设备。首次进入其他设备的 Cloud 工作台后，原实现又丢失设备选择入口，无法连续切换设备。

本机页面的浏览器请求必须只发送到本地 Agent；由 Agent 代表浏览器访问 Cloud。浏览器不能从本机工作台直接调用 Cloud API 或依赖 CORS 放行。

## Goal

- 本机 `/sessions` 只要存在当前 PC 名称，右下角设备名就可打开设备菜单。
- 菜单展示账号下的设备状态，允许进入其他在线设备的会话工作台。
- 进入 Cloud 工作台后，右下角设备菜单仍可使用，可继续切换在线设备。
- 本机模式下的设备列表请求固定走本地 Agent 代理。

## Non-goal

- 不改变设备绑定、删除、配对或凭据轮换流程。
- 不把 Cloud 的完整工作区、会话或终端 API 代理到本地 Agent。
- 不为本机页面新增 Cloud CORS allowlist 或跨域 API 调用。
- 不改变离线设备的可见性；离线设备只展示状态，不可进入工作台。

## User scenarios

1. 用户在已连接 Cloud 的本机 `/sessions` 点击右下角当前 PC 名称，选择另一台在线 PC，进入其 Cloud 会话工作台。
2. 用户到达该在线 PC 的 Cloud 会话工作台后，再次点击右下角设备名，继续切换到第三台在线设备。
3. 用户看到当前设备的选中标记；离线设备不可选择。
4. Cloud 凭据缺失或失效时，本机页面不向 Cloud 发起浏览器请求，设备列表加载失败有可读错误提示。
5. 用户在 Cloud 工作台的设备菜单中能看到原本机设备的“本地”标记，并点击它回到本地 `/sessions` 地址。
6. 用户在 Cloud 工作台切换到另一台远端设备后，页面不复用上一台设备的终端、工作区或会话列表。

## Acceptance

- 设备名始终是可访问的菜单触发器；云端连接状态不控制该入口。
- 菜单不合成、重排或标注当前 PC，只绑定 Agent 或 Cloud 返回的设备列表；未连接 Cloud 且没有凭据时列表为空。
- `GET /api/cloud/devices` 由 Agent 验证 bearer token 并请求 Cloud 的设备列表；浏览器在本机模式只调用该本地路径。
- Agent 未收到 token 时返回 `401`，且不向 Cloud 出站请求。
- Agent 的 Cloud client、service 与 handler 使用 `ListDevicesResp` 传递完整列表响应；`DeviceSummary` 只作为 `items` 元素。
- 当前设备和离线设备不能被选择；其他在线设备可被选择。
- 本机已有设备列表但 Cloud session 缺失时，选择其他设备会先经本地 Agent 恢复 Cloud session；恢复失败时向用户显示错误。
- 菜单中与发起本机一致的设备显示“本地”标记，但前端不改变后端列表顺序。
- 从 Cloud 工作台选择原本机设备时，浏览器进入 `local.publicUrl` 下的 `/sessions`，而非 Cloud 设备路由。
- Cloud 设备路由变更时，旧终端标签会关闭，工作区、会话和快捷方式会针对目标设备重新加载。
- 目标设备的数据按顺序加载：先工作区和会话，再快捷方式；整个过程显示 loading 并禁止重复切换。
- Cloud 会话工作台保留同一菜单，并能在在线设备间连续切换。
- 前后端相关测试、类型检查、lint 和格式检查通过。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- 设备列表使用现有 `ListDevicesResp` 协议，不新增 protobuf 类型。
- Agent 新增窄范围 `/api/cloud/devices` 代理端点，而不是让本机浏览器直连 Cloud。
- `ListDevicesResp` 是 Cloud client、application service 和 handler 的返回契约；不在中间层拆成 `DeviceSummary` 列表后重新包装。
- `SessionsPageShell` 根据 runtime target 选择设备列表来源和切换行为；前端原样绑定设备列表，不补充当前设备、不去重、不排序；`SessionStatusBar` 只呈现菜单并发出事件。
- 本机缺少 `public_url` 时，设备切换先通过既有本地 Cloud connect 流程恢复 session，再进入目标 Cloud 工作台。
- 本机进入 Cloud 工作台时通过路由 query 传递本机设备 ID；Cloud 内设备切换保留该 ID，用于显示“本地”及返回本地地址。
- `SessionsPageShell` 监听 runtime target 变化，在 Cloud 设备切换后重置工作台和工作区 store 再刷新数据。
- `SessionsPageShell` 的目标设备刷新不并行请求；切换状态同时控制工作台 loading 和状态栏设备按钮的 spinner/禁用状态。
- 从本机切到 Cloud 是整页导航；在 Cloud 工作台内的后续切换使用该页面已有的同源设备状态。

## User review notes

- 用户明确指出：所有本机页面请求只能请求本地，不能直接请求 Cloud。
- 用户要求：进入其他 PC 后仍必须能够继续切换设备。
- 用户要求：设备菜单入口不依赖云端连接状态；设备列表由后端输出，前端只绑定。

## Risk

- 从本机进入 Cloud 工作台依赖既有的 Cloud 登录态和路由配置；本任务不改变跨站登录或 token 迁移行为。
- 尚未完成用户的真实多设备手工验收；自动化检查仅覆盖接口、类型和静态行为。
