---
id: launch-from-shortcuts
order: 7
title: Launch Sessions from Shortcuts
description: Choose an enabled Shortcut while creating a Session and begin new independent work.
---

# Launch Sessions from Shortcuts {#launch-from-shortcuts}

A Shortcut does not run directly from the management page. The correct flow is to create a new Session in the target Device's Sessions workbench and choose an enabled Shortcut as the command source.

## Create a Session from a Shortcut {#create-from-shortcut}

1. Enter the target Device's Sessions workbench;
2. click New session;
3. select or enter the Workspace / working directory for this task;
4. choose Shortcut as the command source;
5. search for and select an enabled Shortcut;
6. confirm the Session name and create it;
7. begin the new task in the browser terminal.

:::screenshot shortcut-command-source:::

## Shortcut and Session relationship {#shortcut-session-relationship}

```text
Shortcut: my-app · Cloud CLI
          │
          ├── create → Session: implement login
          ├── create → Session: investigate build failure
          └── create → Session: refactor API
```

You can use the same Shortcut repeatedly to create independent Sessions. To return to Implement login, open that existing Session in the Workspace Session tree instead of choosing the Shortcut again.

## A daily example {#daily-scenario}

- Morning: create a feature Session with `my-app · Cloud CLI`.
- When verification is needed: create an independent test Session with `my-app · Test shell`.
- Afternoon: open the feature Session created in the morning and continue its CLI context.
- When a new review begins: create a new `my-app · Review` Session.

You have now completed the core workflow: download TermBridge, run a CLI Session, continue existing work, and reuse frequent commands through Shortcuts.
