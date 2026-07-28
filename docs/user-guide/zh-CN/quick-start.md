---
id: quick-start
order: 1
title: 快速开始
description: 从 GitHub Release 下载、启动并运行第一个 CLI Session。
next: understand-termbridge
---

# 快速开始：运行第一个 CLI Session {#quick-start}

本章带你从正式制品开始，在浏览器中运行第一个 Cloud CLI 或 Code XCLI Session。完成后，你可以继续阅读“认识 TermBridge”，或将常用命令保存为 Shortcut。

## 开始前准备 {#prerequisites}

准备一台要运行项目和 CLI 的设备，并确认：

1. Cloud CLI 或 Code XCLI 已安装；
2. 该 CLI 已在设备终端完成登录；
3. 你有一个要使用的项目目录；
4. 可以使用浏览器打开本地工作台。

:::screenshot cli-ready:::

## 从 GitHub Release 下载 {#download}

前往 [TermBridge Releases](https://github.com/leoninew/TermBridge-go/releases/latest)，选择最新的非预发布版本，在 **Assets** 中下载与你的系统匹配的 x64 ZIP：

| 系统 | 制品 |
| --- | --- |
| Windows x64 | `TermBridge-windows-x64.zip` |
| Linux x64 | `TermBridge-linux-x64.zip` |
| macOS Intel x64 | `TermBridge-macos-x64.zip` |

当前 Release 不提供 ARM64 安装包。下载后解压到方便长期保留的位置。

:::screenshot release-assets:::

:::screenshot package-folder:::

## 启动本地工作台 {#start-termbridge}

在解压目录中使用对应启动入口：

```powershell
# Windows
.\termbridge.cmd
```

```bash
# Linux / macOS
./termbridge.sh
```

启动脚本会运行本地 Agent 并尝试打开 `http://localhost:9030`。保持该终端窗口运行，然后在浏览器中进入本地工作台。

:::screenshot agent-started:::

## 创建第一个 Session {#first-session}

1. 进入 **工作区**；
2. 点击“新建会话”；
3. 输入项目目录和 Session 名称；
4. 选择直接命令；
5. 输入已能在设备终端运行的 CLI 命令；
6. 创建 Session，并在浏览器终端确认 CLI 已启动。

如果你希望从固定项目目录启动命令，可以使用当前 CLI contract：

```powershell
termbridge --cwd D:\projects\my-app exec -- claude
```

`--cwd` 指定工作目录；`exec --` 后面是要运行的原始命令。它用于在 Session 中运行 CLI，不是启动 TermBridge 本身的方式。

:::screenshot workspace-entry:::

:::screenshot create-session:::

:::screenshot browser-terminal:::

## 下一步 {#quick-start-next}

现在你可以[认识 TermBridge](#understand-termbridge)，了解 Device、Workspace、Session 和 Shortcut 的关系；也可以直接学习如何[启动 CLI 会话](#start-cli-sessions)。
