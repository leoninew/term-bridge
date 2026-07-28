---
id: manage-shortcuts
order: 6
title: Manage frequent commands
description: Save, organize, and maintain frequent single commands with Shortcuts.
next: launch-from-shortcuts
---

# Shortcuts: define and manage frequent commands {#manage-shortcuts}

A Shortcut is one reusable command definition. It is not a keyboard shortcut, command group, script orchestrator, or running terminal.

## What a Shortcut stores {#shortcut-fields}

Each Shortcut can include:

| Field | Purpose |
| --- | --- |
| Name | Recognize the command quickly while creating a Session |
| Command | One original command to reuse |
| Description | An optional explanation of its purpose |
| Tags | Organize and filter items |
| Enabled state | Control whether it can be selected in a new Session |

A Shortcut does **not** store a Device, Workspace, or working directory. Choose the Device and project directory when you create a Session.

## Create and organize {#create-and-organize}

The Shortcuts page lets you create, edit, copy, enable or disable, filter by tags, reorder, import, export, and delete command definitions. A useful naming pattern is `project · task`:

```text
my-app · Cloud CLI
my-app · Test shell
my-app · Review
```

Even when several Shortcuts serve one project, each is still one independent command.

:::screenshot shortcut-management:::

## Enable and disable {#enable-and-disable}

Disabling does not delete a Shortcut and does not affect existing Sessions. Disabled items remain visible, editable, and copyable on the management page, but no longer appear in the Shortcut picker for new Sessions.

## Editing does not rewrite historical Sessions {#shortcut-snapshots}

When a Session is created from a Shortcut, it keeps the command and Shortcut snapshot from that time. Editing the Shortcut later affects new Sessions only; it does not rewrite a running or historical Session.

## Next {#shortcut-next}

Continue with [Launch a Session from a Shortcut](#launch-from-shortcuts) to learn where Shortcuts belong in the new Session flow.
