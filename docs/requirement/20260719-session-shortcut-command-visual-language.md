# 会话 / 快捷方式 / 命令 视觉语言统一
最后修改时间: 2026-07-19 11:49:36

Review status: Accepted

## Background

`/sessions` 及相关表面多处需要表达：

- **会话**实体（树节点、tab、空态入口等）
- 会话的 **启动方式**（`command_source`：快捷方式 / 命令）
- **快捷方式**品类入口（管理页、首页面板、空态「管理快捷方式」）
- **目录**（工作区树节点）

当前问题：

1. 会话行/tab 用 `SessionSourceIcon`（Keyboard/Command）表达启动方式，会话本身缺少稳定的实体图标。
2. `Command` 既表示「命令」启动方式，又被用作「管理快捷方式」入口，语义过载。
3. 文案分裂：「启动命令」vs「直接命令」等。
4. 图标映射与 label 逻辑分散。

目录（Folder / FolderOpen）与用户确认 **保持现状**。

## Goal

约定并落地 **目录 / 会话 / 快捷方式 / 命令** 的统一视觉语言（图标 + 用语），使：

1. 会话实体有固定图标，不与启动方式图标混用。
2. 启动方式仅在「选择/展示启动方式」的语境出现，分为快捷方式与命令。
3. 实现侧启动方式 icon/文案映射单一来源。
4. 目录树节点图标与交互保持现状。

### 约定（已按用户确认修订）

#### 用语

| 概念 | 中文 | English | 含义 |
| --- | --- | --- | --- |
| 目录 | 目录 / 工作区 | Workspace（树节点） | 树形父节点，组织会话 |
| 会话 | 会话 | Session | 一次可打开、可运行的记录；列表/tab 上的实体 |
| 启动方式 | 启动方式 | Launch method | 会话如何启动（对应 `command_source`） |
| 快捷方式 | 快捷方式 | Shortcut | 启动方式之一：`command_source === 'shortcut'`；亦指快捷方式品类/管理对象 |
| 命令 | 命令 | Command | 启动方式之一：直接输入的命令（`command_source === 'command'` 及非 shortcut 兜底） |

说明：

- 不再用「直接命令」「启动命令」作为启动方式类型的对外主文案；统一为 **命令**（必要时 UI 短标签可与 i18n key 对齐为 `command` / `shortcut`）。
- 「启动方式」是会话的属性分类说法，不是第四种实体图标。

#### 图标

| 概念 | Lucide | 使用场景 |
| --- | --- | --- |
| **目录** | `Folder` / `FolderOpen` | 工作区树父节点；**保持现状** |
| **会话** | `SquareTerminal` | 树会话节点、已打开 tab、以及其它「会话实体」列表项；**节点上不展示启动方式图标** |
| **快捷方式**（启动方式 / 品类） | `Keyboard` | 新建/编辑中的启动方式切换；需要明示启动方式的元信息区（如终端底部状态栏）；快捷方式**入口与标题** |
| **命令**（启动方式） | `Terminal` | 新建/编辑中的启动方式切换；需要明示启动方式的元信息区；**禁止**用作快捷方式入口 |
| **新建目录**（动作） | `Plus` | 树/工具区新建工作区等；**保持 Plus** |
| **新建会话**（动作） | `SquareTerminal` | 空态「新建会话」等；**继续用 SquareTerminal**（与会话实体图标一致） |

#### 表面规则

| 表面 | 图标行为 |
| --- | --- |
| 左侧树 · 目录节点 | `Folder` / `FolderOpen`（现状） |
| 左侧树 · 会话节点 | **仅** `SquareTerminal`；**不**展示 Keyboard/Command |
| 已打开会话 tab | **仅** `SquareTerminal`；**不**展示启动方式图标 |
| 终端底部状态栏 | 可保留启动方式图标 + 快捷方式名 / 命令文本（元信息，非「会话节点」） |
| 新建 / 编辑会话 | 启动方式切换：`Keyboard` / `Command`，文案为快捷方式 / 命令 |
| 关闭后台会话抽屉 | 会话行按「会话实体」处理：用 `SquareTerminal`，不展示启动方式图标（与树/tab 一致） |
| 无 tab 空态 · 新建会话 | `SquareTerminal` |
| 无 tab 空态 · 管理快捷方式 | `Keyboard`（不得用 `Command`） |
| 本地首页快捷方式「更多」 | `Keyboard` |
| 快捷方式管理页 | **只改入口与标题** 使用 `Keyboard`；卡片列表不强制加 Keyboard |

#### 实现约定

- 将 `SessionSourceIcon` **重命名**为与「启动方式」对齐的名称（推荐 `LaunchMethodIcon` 或 `CommandSourceIcon`；实现时二选一，默认倾向 `LaunchMethodIcon` 以贴合「启动方式」用语）。
- 启动方式 icon 映射与 label 映射各只定义一处；`SessionCommandInput` 复用，禁止第二套 hardcode。
- 空 / 未知 `command_source`：启动方式展示按 **命令** 处理。
- 展示态 i18n：启动方式类型统一为「快捷方式 / Shortcut」与「命令 / Command」；清理会话节点上对 source 图标的依赖。

## Non-goal

- 不改后端 `command_source` 字段语义与 API。
- 不改目录展开/折叠、拖拽、无 running 默认折叠等行为（除非实现时发现与图标无关的既有回归）。
- 不改快捷方式 CRUD / 导入导出业务。
- 不在每张 ShortcutCard 上强制加 `Keyboard`。
- 不引入新图标库。
- 不重做会话页信息架构。

## User scenarios

1. 用户在左侧树看到会话：图标为 `SquareTerminal`，无 Keyboard/Command；点开后行为与现网一致。
2. 用户查看已打开 tab：图标为 `SquareTerminal`，不显示启动方式图标；标题仍为会话名。
3. 用户看终端底部：可知当前会话状态，并以启动方式图标 + 快捷方式名或命令文本了解如何启动。
4. 用户新建/编辑会话：在「启动方式」切换快捷方式与命令，图标分别为 Keyboard 与 Command，文案一致。
5. 用户点空态「管理快捷方式」或首页快捷方式更多、进入管理页标题区：看到 `Keyboard`，不会与「命令」混淆。
6. 用户操作目录节点：Folder/FolderOpen 与新建目录 Plus 与改前一致；新建会话动作为 SquareTerminal。

## Acceptance

- [ ] 用语：对外主文案使用「启动方式 / 快捷方式 / 命令」；会话节点不再用启动方式图标暗示类型。
- [ ] 树会话节点、会话 tab、关后台抽屉中的会话行：实体图标为 `SquareTerminal`，**不**渲染启动方式图标。
- [ ] 目录节点仍为 `Folder` / `FolderOpen`；新建目录动作仍为 `Plus`；新建会话动作仍为 `SquareTerminal`。
- [ ] 新建/编辑启动方式切换：`Keyboard` = 快捷方式，`Command` = 命令；文案与 i18n 一致（中英）。
- [ ] 终端底部等元信息区若展示启动方式，图标与文案遵循同一映射。
- [ ] 空态「管理快捷方式」、本地首页快捷方式「更多」、快捷方式管理页入口与标题：使用 `Keyboard`，不用 `Command`。
- [ ] ShortcutCard 列表不强制加启动方式/品类小图标。
- [ ] `SessionSourceIcon` 已重命名；全仓库引用更新；映射与 label 单一来源。
- [ ] 空 / 未知 `command_source` 按命令展示。
- [ ] 不破坏会话创建/编辑、快捷方式管理、目录展开等主路径。

## Open questions

无。用户已确认决策。

## Decisions

- 流程模式：**轻量 / light**。
- **目录**保持 `Folder` / `FolderOpen`；**新建目录**继续 `Plus`。
- **会话**实体图标统一为 **`SquareTerminal`**；**会话节点（树 / tab / 同类列表）不展示启动方式图标**。
- 启动方式用语：属性叫 **启动方式**，取值 **快捷方式** | **命令**（不再主推「直接命令」「启动命令」作为类型名）。
- **快捷方式**图标 `Keyboard`；**命令**图标 `Terminal`；`Terminal` 表示命令启动方式；禁止用键盘类图标表示命令。
- **新建会话**继续 `SquareTerminal`。
- 组件 **重命名**（取代 `SessionSourceIcon`）。
- 快捷方式管理页 **只改入口与标题**，不加在每张卡片上。
- 终端底部状态栏保留启动方式元信息展示（非会话节点）。

## Risk

- Lucide `Command` 为 ⌘ 键图形，与 `Keyboard` 难区分；命令启动方式改用 `Terminal`。

- 去掉树/tab 上的启动方式图标后，用户扫一眼列表时较难区分 shortcut 与 command 会话；可依赖名称、底部状态栏与编辑态补偿。
- 文案从「直接命令 / 启动命令」收到「命令」属于可见文案变更，需中英 i18n 一并替换相关 key 的用户可见字符串。
- 重命名组件时注意全仓 import 与测试引用。

## User review notes

- 用户确认：会话用 `SquareTerminal`，会话节点不展示启动方式图标。
- 用户确认：应使用「启动方式」说法，启动方式分快捷方式或命令。
- 用户确认：组件重命名。
- 用户确认：快捷方式只改入口和标题。
- 用户确认：新建目录继续 Plus，新建会话继续 SquareTerminal。
