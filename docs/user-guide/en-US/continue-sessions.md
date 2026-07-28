---
id: continue-sessions
order: 4
title: Continue Sessions
description: Find a running Session and return to existing work in the browser.
next: manage-workspaces
---

# Continue existing Sessions in the browser {#continue-sessions}

Once a Session exists, you do not need to re-enter its command to return to the same task. Open the existing Session under its Workspace to continue reading output and interacting with it.

## Find the right Session {#find-a-session}

- **Local mode:** open the local workbench and enter Sessions.
- **Cloud mode:** open Dashboard, choose a Device, then open that Device's Sessions.
- Find the project in the Workspace tree, then select the appropriate Session.

The Session name, Workspace, and running state help identify the work you want to resume.

## Handle multiple tasks {#multiple-sessions}

A Workspace can have multiple Sessions. A common pattern is to keep one primary CLI Session and open a Shell separately for tests, logs, or Git commands.

```text
my-app Workspace
├── Implement login · Cloud CLI
├── Run tests · Shell
└── Review changes · Code XCLI
```

Each Session runs independently. Return to an existing Session when you need its CLI context; create another Session when you need an independent task.

## Return after closing the browser {#return-later}

Closing a browser page does not end a Session. Return to TermBridge, locate the same Device and Workspace, then open that Session again.

## Next {#continue-next}

Read [Manage projects with Workspaces](#manage-workspaces) to keep projects, Sessions, and supporting tools clear.
