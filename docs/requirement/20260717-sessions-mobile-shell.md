# 会话界面移动端友好（Phase 1）

最后修改时间: 2026-07-17 18:14:00

Review status: Accepted

## Flow mode / Stage

轻量模式 / light；需求 / Requirement（Accepted，用户授权实现）。

## Background

`/sessions` 与 Cloud 设备会话页采用桌面优先的左右 Splitter：侧栏 + 终端 Workbench。窄屏下侧栏挤占终端宽度，树节点操作依赖 hover，拖拽排序与触控冲突，手机上难用。

## Goal

Phase 1：在不改变后端协议与桌面主路径的前提下，让手机/窄屏可以：

1. 默认全宽终端，侧栏以 overlay drawer 打开/关闭。
2. 不依赖 hover 即可发现并使用侧栏操作。
3. 列表行与图标按钮具备更可用的触控命中区。
4. 窄屏/粗指针下禁用 workspace、session、tab 拖拽排序。

## Non-goal

- 不做 list/session 双路由重构（Phase 2）。
- 不做 xterm 软键盘快捷条 / visualViewport 适配（Phase 2）。
- 不改 Code/Files/Git workbench 的独立布局。
- 不改会话创建/编辑的业务校验与 API。

## User scenarios

1. 手机竖屏打开会话页：直接看到终端区域；点菜单打开侧栏；选 session 后侧栏关闭并显示该 session。
2. 侧栏内可新建、停止、删除等，无需 hover。
3. 桌面宽度恢复后回到可拖拽 Splitter 双栏，行为与现网一致。

## Acceptance

- [x] `max-width: 768px` 使用 overlay 侧栏，终端默认全宽。
- [x] 窄屏有明确入口打开/关闭侧栏。
- [x] 选中 session 后自动关闭移动侧栏。
- [x] 窄屏或 coarse pointer 下禁用拖拽排序。
- [x] 侧栏操作在触屏上可见（非 hover-only）。
- [x] 桌面布局与现有 Splitter 行为不回退。

## Open questions

不适用。

## Decisions

1. 断点：`768px`。
2. 移动侧栏：fixed overlay + backdrop，不用永久占宽的 Splitter。
3. 触控可见：CSS `(hover: none)` / 窄屏下 action 常显，并略增命中区。
4. 拖拽：`disableReorder = isNarrow || isCoarsePointer`。

## Risk

- Splitter 与 overlay 双布局可能有少量属性重复，需保持事件接线一致。
- 横屏平板处于断点边界时可能在两种模式间切换，可接受。
