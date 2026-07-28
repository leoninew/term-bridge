---
id: manage-workspaces
order: 5
title: Manage projects with Workspaces
description: Use a project directory as shared context for CLI, Shell, code, and Git work.
next: manage-shortcuts
---

# Manage projects with Workspaces {#manage-workspaces}

A Workspace represents a project directory. The working directory chosen while creating a Session decides where the command runs and keeps related Sessions, code, and Git work around the same project.

## Choose the right project directory {#choose-project-directory}

Before creating a Session, confirm that the directory is the project for this task. With the right directory, a CLI can see the project's files, configuration, and Git repository context.

```text
D:\projects\my-app
└── Workspace: my-app
    └── New Sessions run here
```

## A recommended project layout {#recommended-layout}

Keep one primary CLI Session for a project, then add independent Sessions for parallel work as needed:

```text
my-app Workspace
├── Cloud CLI: implement a feature
├── Code XCLI: review changes
└── Shell: run tests and Git commands
```

This keeps tasks independent without losing the fact that they belong to the same project.

## Use supporting workbenches {#workspace-tools}

When needed, a Workspace can also open Code, file, or Git workbenches. These tools complement the CLI workflow; use the Sessions workbench first to start and continue terminal work.

## Next {#workspace-next}

If you repeatedly enter the same command, continue to [Manage frequent commands](#manage-shortcuts).
