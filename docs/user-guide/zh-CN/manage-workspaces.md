---
id: manage-workspaces
order: 5
title: 用 Workspace 管理项目
description: 让项目目录成为 CLI、Shell、代码和 Git 工作的共同上下文。
next: manage-shortcuts
---

# 用 Workspace 管理项目 {#manage-workspaces}

Workspace 对应一个项目目录。创建 Session 时选择的工作目录决定命令在哪里执行，也让相关 Session、代码和 Git 操作围绕同一个项目组织。

## 选择正确的项目目录 {#choose-project-directory}

在新建 Session 前确认目录是本次任务所在的项目。使用正确目录后，CLI 可以看到该项目的文件、配置和 Git 仓库上下文。

```text
D:\projects\my-app
└── Workspace：my-app
    └── 新建的 Session 从这里运行
```

## 一个项目的推荐布局 {#recommended-layout}

先为一个项目保留一个主要 CLI Session；需要并行任务时，再增加独立 Session：

```text
my-app Workspace
├── Cloud CLI：实现功能
├── Code XCLI：审查改动
└── Shell：运行测试和 Git 命令
```

这样可以让不同任务相互独立，又不会丢失它们属于同一项目的关系。

## 使用辅助工作台 {#workspace-tools}

在需要时，你也可以从 Workspace 进入 Code、文件或 Git 工作台。这些能力用于补充 CLI 工作流；开始和继续任务时，仍优先通过 Session 工作台组织终端会话。

## 下一步 {#workspace-next}

如果经常重复输入相同命令，继续阅读[快捷方式：定义与管理常用命令](#manage-shortcuts)。
