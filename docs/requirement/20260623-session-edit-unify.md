# Session 编辑操作统一

最后修改时间: 2026-06-23 15:45:00

## Background

前端存在大量 `rename` 语义：`renameSession`、`renameSelectedSession`、`renameSessionAria`、`renameSessionTitle`、`renameSessionFailed`、`sessionRenamed`、`renaming` 等。但后端 PATCH API 只支持修改名称字段，"rename" 暗示可以重命名（改名），而实际业务是"编辑会话"（编辑名称）。两者语义不一致，且前端到处挂着 `rename` 字样，不符合"使用正确的方式"原则。

## Goal

1. 将前端所有 "rename session" 概念统一为 "edit session"。用户视角是"编辑会话"，不是"重命名会话"。
2. 所有会话和 workspace 操作按钮（编辑、停止、重新运行、删除、移除工作区）点击后显示 loading spinner 并防重入。

## Non-goal

- 不修改后端 API 或后端错误消息
- 不添加编辑其他字段的能力（命令、目录等）
- 不改变现有编辑弹窗的交互结构（仍是 Dialog + 表单）
- 不改 sidebar 中其他按钮（stop、rerun、delete）的行为逻辑

## User scenarios

1. 用户 hover 会话节点，看到铅笔图标（编辑按钮），点击后弹出编辑名称的对话框
2. 用户提交编辑，成功 toast 显示"会话已编辑"
3. 用户失败时 toast 显示"编辑会话失败"
4. 无障碍标签 aria-label 使用"编辑 {name} 会话"
5. 所有操作按钮（编辑、停止、重新运行、删除、移除工作区）点击后按钮进入 loading 状态（spinner），防止重复点击

## Acceptance

- [ ] 前端所有 `rename` 相关变量、函数、事件名、i18n key 统一改为 `edit` 语义
- [ ] 按钮点击后显示 loading spinner，防重入
- [ ] 弹窗标题变为"编辑会话"
- [ ] 弹窗描述删除"当前后端 API 仅支持修改会话名称"等技术解释——这是给用户看的，不是产品应该描述的
- [ ] Toast 消息改为"会话已编辑" / "编辑会话失败"
- [ ] 无障碍标签改为"编辑 {name} 会话"
- [ ] 后端 API 调用不变（PATCH /sessions/{id}）
- [ ] 无残留 `rename` 字样（除后端错误消息外）
- [ ] 所有操作按钮（编辑、停止、重新运行、删除、移除工作区）均有 loading spinner 和防重入保护

## Open questions

- 无

## Decisions

- 统一使用 `edit` 语义，不用 `rename`
- 后端错误消息 "Rename session failed" 不修改（不属于前端代码）
- 弹窗描述不出现任何技术解释（如后端 API 限制），只面向用户

## Review status: Accepted

## Risk

- 如果其他地方引用了 `renameSession` 事件名或 i18n key，需要同步更新
- 需确保所有 `rename` 前缀的变量/函数/模板绑定都被替换
- 所有操作按钮都需要加 loading 状态管理，涉及多个函数和 ref

## User review notes

用户确认进入实现，需求已接受。
