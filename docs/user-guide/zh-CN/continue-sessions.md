---
id: continue-sessions
order: 4
title: 在浏览器中继续会话
description: 找到正在运行的 Session，并在浏览器中返回已有工作。
next: manage-workspaces
---

# 在浏览器中继续已有会话 {#continue-sessions}

创建 Session 后，不必为了回到同一任务重复输入命令。打开对应 Workspace 下的已有 Session，即可继续查看输出和进行交互。

## 找到正确的会话 {#find-a-session}

- **本地模式**：打开本地工作台，进入 Sessions；
- **云端模式**：先进入 Dashboard，选择 Device，再进入该设备的 Sessions；
- 在 Workspace 树中找到项目，再选择对应的 Session。

Session 名称、所在 Workspace 和运行状态共同帮助你判断应回到哪项工作。

## 同时处理多个任务 {#multiple-sessions}

同一个 Workspace 可以有多个 Session。常见方式是保留一个主要 CLI Session，同时另开一个 Shell 来运行测试、查看日志或执行 Git 命令。

```text
my-app Workspace
├── 实现登录页 · Cloud CLI
├── 运行测试 · Shell
└── 审查改动 · Code XCLI
```

每个 Session 独立运行。需要保存 CLI 上下文时，返回已有 Session；需要开始另一项独立任务时，再创建新 Session。

## 浏览器关闭后再回来 {#return-later}

关闭浏览器页面不等于结束 Session。回到 TermBridge 后，从同一个 Device 和 Workspace 找到该 Session，即可再次打开它。

## 下一步 {#continue-next}

继续阅读[用 Workspace 管理项目](#manage-workspaces)，让项目、会话和辅助工具保持清晰。
