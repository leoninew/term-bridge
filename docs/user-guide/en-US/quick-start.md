---
id: quick-start
order: 1
title: Quick start
description: Download from GitHub Releases, start TermBridge, and run your first CLI Session.
next: understand-termbridge
---

# Quick start: run your first CLI Session {#quick-start}

This guide starts with an official package and ends with your first Cloud CLI or Code XCLI Session in the browser. Afterwards, you can learn the TermBridge model or save a frequent command as a Shortcut.

## Before you begin {#prerequisites}

Prepare a device that will run your project and CLI, and make sure that:

1. Cloud CLI or Code XCLI is installed;
2. the CLI is already signed in from the device terminal;
3. you have a project directory to work in;
4. you can open the local workbench in a browser.

:::screenshot cli-ready:::

## Download from GitHub Releases {#download}

Open [TermBridge Releases](https://github.com/leoninew/TermBridge-go/releases/latest), select the latest non-prerelease version, then download the x64 ZIP for your system from **Assets**:

| System | Package |
| --- | --- |
| Windows x64 | `TermBridge-windows-x64.zip` |
| Linux x64 | `TermBridge-linux-x64.zip` |
| macOS Intel x64 | `TermBridge-macos-x64.zip` |

The current release does not provide ARM64 packages. Extract the ZIP somewhere you can keep it.

:::screenshot release-assets:::

:::screenshot package-folder:::

## Start the local workbench {#start-termbridge}

Use the launcher for your extracted package:

```powershell
# Windows
.\termbridge.cmd
```

```bash
# Linux / macOS
./termbridge.sh
```

The launcher starts the local Agent and attempts to open `http://localhost:9030`. Keep that terminal window running, then open the local workbench in your browser.

:::screenshot agent-started:::

## Create your first Session {#first-session}

1. Enter the **Workspace** view;
2. click New session;
3. enter the project directory and a Session name;
4. choose Direct command;
5. enter a CLI command that already works in the device terminal;
6. create the Session and confirm that the CLI starts in the browser terminal.

To start a command from a fixed project directory, the current CLI contract is:

```powershell
termbridge --cwd D:\projects\my-app exec -- claude
```

`--cwd` sets the working directory; everything after `exec --` is the original command to run. It runs a CLI in a Session; it is not how to start the TermBridge product itself.

:::screenshot workspace-entry:::

:::screenshot create-session:::

:::screenshot browser-terminal:::

## Next {#quick-start-next}

Read [Understand TermBridge](#understand-termbridge) to learn how Device, Workspace, Session, and Shortcut relate, or go directly to [Start CLI Sessions](#start-cli-sessions).
