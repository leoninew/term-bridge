---
id: understand-termbridge
order: 2
title: Understand TermBridge
description: Learn the workflow from a device to a CLI Session through four concepts.
next: start-cli-sessions
---

# Understand TermBridge {#understand-termbridge}

TermBridge helps you manage CLI Sessions in the right project and on the right device. You only need four concepts to begin.

## Four concepts {#four-concepts}

| Item | What it is | When you use it |
| --- | --- | --- |
| Device | The computer or server that runs your projects and CLIs | Choose it first when working through Cloud |
| Workspace | A project working directory | Provides project context for a Session |
| Session | One running CLI or Shell | Continue current work or start an independent task |
| Shortcut | One reusable command definition | Fill a frequent command while creating a new Session |

## From Device to Session {#device-workspace-session}

```text
Device
└── Workspace (project directory)
    ├── Session: Cloud CLI
    ├── Session: Code XCLI
    └── Session: Shell
```

The local workbench uses the current device directly. Through Cloud, open Dashboard, choose a Device, then open its Sessions workbench.

## How Shortcuts fit in {#shortcut-to-session}

```text
Shortcut: one frequent command
          │
          ├── choose while creating → Session A
          └── choose again          → Session B
```

A Shortcut is not a terminal and does not restore an old Session. It only reuses a command while creating a new Session. To continue existing work, open the existing Session.

## Next {#understand-next}

Continue with [Start Cloud CLI / Code XCLI Sessions](#start-cli-sessions) and apply the model to a real task.
