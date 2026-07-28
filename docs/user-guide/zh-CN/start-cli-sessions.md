---
id: start-cli-sessions
order: 3
title: 启动 CLI 会话
description: 在 Workspace 中使用直接命令或 Shortcut 创建 Cloud CLI 与 Code XCLI Session。
next: continue-sessions
---

# 启动 Cloud CLI / Code XCLI 会话 {#start-cli-sessions}

TermBridge 提供的是统一的 Session 创建流程，而不是 Cloud CLI 或 Code XCLI 的专用启动器。你继续使用已经安装、已经登录的原始命令。

## 选择项目上下文 {#choose-workspace}

进入 Sessions 工作台后，选择已有 Workspace，或在新建 Session 时输入项目目录。这个目录决定 CLI 从哪里运行，并会成为该 Session 的 Workspace 上下文。

> 注意：当前产品会在创建 Session 时初始化 Workspace；不要把它理解为一个独立的“创建 Workspace”表单。

## 使用 Cloud CLI {#start-cloud-cli}

1. 点击“新建会话”；
2. 确认项目目录和 Session 名称；
3. 选择“直接命令”；
4. 输入你的 Cloud CLI 命令，例如 `claude`；
5. 创建后，在浏览器终端完成原有 CLI 的交互。

```powershell
termbridge --cwd D:\projects\my-app exec -- claude
```

## 使用 Code XCLI {#start-code-xcli}

Code XCLI 使用完全相同的 Session 创建界面：

1. 在正确的项目目录创建 Session；
2. 选择“直接命令”；
3. 输入已经可在设备终端运行的 Code XCLI 命令；
4. 创建后在浏览器终端继续交互。

```powershell
termbridge --cwd D:\projects\my-app exec -- codex
```

## 保留自己的参数 {#keep-your-arguments}

你可以继续使用自己熟悉的 CLI 参数。TermBridge 负责把命令放进 Session，不会替你解释或管理第三方 CLI 的参数、账号或审批流程。

```powershell
termbridge --cwd D:\projects\my-app exec -- claude --model <model>
```

如果某条命令会反复使用，下一次可先将它保存为 Shortcut，再在新建 Session 中选择。

## 下一步 {#start-cli-next}

会话开始后，学习如何[在浏览器中继续已有会话](#continue-sessions)。
