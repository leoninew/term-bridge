---
id: launch-from-shortcuts
order: 7
title: 使用快捷方式开启会话
description: 在新建 Session 时选择已启用的 Shortcut，开始一项新的独立工作。
---

# 使用快捷方式快速开启会话 {#launch-from-shortcuts}

Shortcut 不在管理页直接启动。正确流程是在目标 Device 的 Sessions 工作台中创建新 Session，并把已启用 Shortcut 作为命令来源。

## 从 Shortcut 创建 Session {#create-from-shortcut}

1. 进入目标 Device 的 Sessions 工作台；
2. 点击“新建会话”；
3. 选择或输入本次任务的 Workspace / 工作目录；
4. 在命令来源中选择 Shortcut；
5. 搜索并选择一个已启用的 Shortcut；
6. 确认 Session 名称后创建；
7. 在浏览器终端开始新任务。

:::screenshot shortcut-command-source:::

## Shortcut 和 Session 的关系 {#shortcut-session-relationship}

```text
Shortcut：my-app · Cloud CLI
        │
        ├── 创建 → Session：实现登录页
        ├── 创建 → Session：排查构建失败
        └── 创建 → Session：重构 API
```

同一个 Shortcut 可以多次用于创建互相独立的 Session。若要回到“实现登录页”，请在 Workspace 的 Session 树中打开那个已有 Session，而不是再次选择 Shortcut。

## 一个日常场景 {#daily-scenario}

- 早上：用 `my-app · Cloud CLI` 创建功能开发 Session；
- 需要验证时：用 `my-app · Test shell` 创建独立测试 Session；
- 下午：打开早上创建的开发 Session，继续原来的 CLI 上下文；
- 开始新审查任务时：创建新的 `my-app · Review` Session。

你已经完成核心工作流：下载 TermBridge、运行 CLI Session、继续已有工作，并通过 Shortcut 复用常用命令。
