---
id: start-cli-sessions
order: 3
title: Start CLI Sessions
description: Create Cloud CLI and Code XCLI Sessions with a direct command or Shortcut.
next: continue-sessions
---

# Start Cloud CLI / Code XCLI Sessions {#start-cli-sessions}

TermBridge offers one Session creation flow, not dedicated launchers for Cloud CLI or Code XCLI. Keep using the original commands you already installed and signed in to.

## Choose project context {#choose-workspace}

In the Sessions workbench, choose an existing Workspace or enter a project directory when creating a Session. That directory decides where the CLI starts and becomes the Workspace context for the Session.

> Note: The current product initializes a Workspace while creating a Session. Do not treat it as a separate Create Workspace form.

## Use Cloud CLI {#start-cloud-cli}

1. Click New session;
2. confirm the project directory and Session name;
3. select Direct command;
4. enter your Cloud CLI command, such as `claude`;
5. create the Session and continue the normal CLI interaction in the browser terminal.

```powershell
termbridge --cwd D:\projects\my-app exec -- claude
```

## Use Code XCLI {#start-code-xcli}

Code XCLI uses the exact same Session creation UI:

1. Create a Session in the right project directory;
2. select Direct command;
3. enter a Code XCLI command that already works in the device terminal;
4. create the Session and continue in the browser terminal.

```powershell
termbridge --cwd D:\projects\my-app exec -- codex
```

## Keep your own arguments {#keep-your-arguments}

You can keep using familiar CLI arguments. TermBridge puts a command in a Session; it does not interpret or manage a third-party CLI's arguments, account, or approval flow.

```powershell
termbridge --cwd D:\projects\my-app exec -- claude --model <model>
```

If you use a command often, save it as a Shortcut and choose it next time you create a Session.

## Next {#start-cli-next}

After the Session starts, learn how to [Continue existing Sessions](#continue-sessions).
