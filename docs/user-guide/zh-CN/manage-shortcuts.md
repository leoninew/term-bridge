---
id: manage-shortcuts
order: 6
title: 管理常用命令
description: 用 Shortcut 保存、整理和维护常用的单条命令。
next: launch-from-shortcuts
---

# 快捷方式：定义与管理常用命令 {#manage-shortcuts}

Shortcut 是一条可复用的命令定义。它不是键盘快捷键、命令组、脚本编排器或正在运行的终端。

## Shortcut 保存什么 {#shortcut-fields}

每个 Shortcut 可以包含：

| 字段 | 作用 |
| --- | --- |
| 名称 | 让你在新建 Session 时快速识别命令 |
| 命令 | 要复用的单条原始命令 |
| 描述 | 可选的用途说明 |
| 标签 | 用于整理和筛选 |
| 启用状态 | 控制它是否可在新建 Session 中选择 |

Shortcut **不**保存 Device、Workspace 或工作目录。创建 Session 时再选择本次要使用的 Device 和项目目录。

## 创建和整理 {#create-and-organize}

在快捷方式管理页可以创建、编辑、复制、启用或停用、按标签筛选、调整排序、导入导出和删除命令定义。建议按“项目名 · 任务名”命名：

```text
my-app · Cloud CLI
my-app · Test shell
my-app · Review
```

即使多个 Shortcut 服务同一项目，它们仍是彼此独立的单条命令。

:::screenshot shortcut-management:::

## 启用和停用 {#enable-and-disable}

停用不会删除 Shortcut，也不影响已经创建的 Session。被停用的项仍可在管理页查看、编辑或复制，但不会出现在新建 Session 的 Shortcut 选择器中。

## 编辑不会改写历史 Session {#shortcut-snapshots}

用 Shortcut 创建 Session 后，Session 会保留当时的命令和 Shortcut 快照。以后编辑 Shortcut 只会影响新建的 Session，不会改写已经在运行或历史中的会话。

## 下一步 {#shortcut-next}

继续阅读[使用快捷方式快速开启会话](#launch-from-shortcuts)，了解它在新建 Session 中的正确位置。
