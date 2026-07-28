---
id: understand-termbridge
order: 2
title: 认识 TermBridge
description: 用四个概念理解从设备到 CLI 会话的工作流。
next: start-cli-sessions
---

# 认识 TermBridge {#understand-termbridge}

TermBridge 的核心是让你在正确的项目和设备中管理 CLI 会话。你只需要先认识四个对象。

## 四个对象 {#four-concepts}

| 对象 | 它是什么 | 你在什么时候使用 |
| --- | --- | --- |
| Device | 运行项目和 CLI 的电脑或服务器 | 云端访问时，先选择要工作的设备 |
| Workspace | 项目所在的工作目录 | 为 Session 提供项目上下文 |
| Session | 一次正在运行的 CLI 或 Shell | 继续当前任务或启动新的独立任务 |
| Shortcut | 一条可复用的命令定义 | 快速填入常用命令以创建新 Session |

## 从设备到会话 {#device-workspace-session}

```text
Device
└── Workspace（项目目录）
    ├── Session：Cloud CLI
    ├── Session：Code XCLI
    └── Session：Shell
```

本地工作台直接使用当前设备。通过云端访问时，先进入 Dashboard，选择 Device，再进入该设备的 Sessions 工作台。

## Shortcut 如何参与 {#shortcut-to-session}

```text
Shortcut：一条常用命令
        │
        ├── 在新建 Session 中选择 → Session A
        └── 再次选择同一项       → Session B
```

Shortcut 不是终端，也不恢复旧会话。它只帮助你在创建新 Session 时复用命令；要继续已有工作，请打开已有 Session。

## 接下来 {#understand-next}

继续阅读[启动 Cloud CLI / Code XCLI 会话](#start-cli-sessions)，把这个模型用于第一个实际任务。
