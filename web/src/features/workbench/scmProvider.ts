import {
  ScmCommand,
  ScmOperationState,
  ScmState,
  type ScmResource,
  type ScmResourceGroup,
  type ScmStatusResp,
} from '../../gen/proto/termbridge/agent/v1/git'
import type { WorkbenchScmApi } from './api'
import {
  parseScmOriginalUri,
  SCM_ORIGINAL_SCHEME,
  scmOriginalUri,
  workspacePathToUri,
} from './uri'

type VscodeApi = typeof import('vscode')

const CMD_REFRESH = 'termbridge.scm.refresh'
const CMD_OPEN = 'termbridge.scm.open'
const CMD_STAGE = 'termbridge.scm.stage'
const CMD_UNSTAGE = 'termbridge.scm.unstage'
const CMD_DISCARD = 'termbridge.scm.discard'
const CMD_COMMIT = 'termbridge.scm.commit'

const textDecoder = new TextDecoder()

export type ScmController = {
  refresh: () => Promise<void>
  dispose: () => void
}

export async function registerTermBridgeScm(
  vscode: VscodeApi,
  api: WorkbenchScmApi,
  workspaceFolderUri: import('vscode').Uri,
): Promise<ScmController> {
  const disposables: { dispose: () => void }[] = []

  const sourceControl = vscode.scm.createSourceControl(
    'termbridge-git',
    'Git',
    workspaceFolderUri,
  )
  sourceControl.inputBox.placeholder = 'Message (Ctrl+Enter to commit)'
  sourceControl.acceptInputCommand = {
    command: CMD_COMMIT,
    title: 'Commit',
  }
  sourceControl.actionButton = {
    command: {
      command: CMD_COMMIT,
      title: 'Commit',
    },
    enabled: true,
  }

  // Keep groups visible even when empty so a clean tree does not look "broken".
  const staged = sourceControl.createResourceGroup('staged', 'Staged Changes')
  staged.hideWhenEmpty = false
  const changes = sourceControl.createResourceGroup('changes', 'Changes')
  changes.hideWhenEmpty = false
  const untracked = sourceControl.createResourceGroup('untracked', 'Untracked')
  untracked.hideWhenEmpty = false

  const groupById: Record<string, typeof staged> = {
    staged,
    changes,
    untracked,
  }

  disposables.push(
    vscode.workspace.registerTextDocumentContentProvider(SCM_ORIGINAL_SCHEME, {
      async provideTextDocumentContent(uri: import('vscode').Uri): Promise<string> {
        const { path, groupId } = parseScmOriginalUri(uri)
        if (!path) {
          return ''
        }
        try {
          const result = await api.originalContent(path, groupId)
          return textDecoder.decode(result.content)
        } catch {
          // Missing original (e.g. untracked / new file) → empty left side.
          return ''
        }
      },
    }),
  )

  async function refresh(): Promise<void> {
    let status: ScmStatusResp
    try {
      status = await api.status()
    } catch (error) {
      console.error('[termbridge-scm] status failed', error)
      sourceControl.count = 0
      sourceControl.statusBarCommands = [
        {
          command: CMD_REFRESH,
          title: '$(error) Git unavailable',
          tooltip: error instanceof Error ? error.message : 'Git unavailable',
        },
      ]
      staged.resourceStates = []
      changes.resourceStates = []
      untracked.resourceStates = []
      sourceControl.inputBox.placeholder = 'Git unavailable'
      return
    }

    if (status.state !== ScmState.SCM_STATE_AVAILABLE) {
      sourceControl.count = 0
      sourceControl.statusBarCommands = [
        {
          command: CMD_REFRESH,
          title: status.message || 'Git unavailable',
        },
      ]
      staged.resourceStates = []
      changes.resourceStates = []
      untracked.resourceStates = []
      sourceControl.inputBox.placeholder = status.message || 'Git unavailable'
      return
    }

    let repositoryLabel = '$(git-branch) Git'
    try {
      const repository = await api.repository()
      if (repository.current_branch) {
        repositoryLabel = `$(git-branch) ${repository.current_branch}`
      }
    } catch {
      // optional
    }

    const count = status.count ?? countResources(status.groups ?? [])
    sourceControl.count = count
    sourceControl.statusBarCommands = [
      {
        command: CMD_REFRESH,
        title: repositoryLabel,
        tooltip: 'Refresh SCM',
      },
    ]
    sourceControl.inputBox.placeholder =
      count === 0
        ? 'No changes (edit a file to see them here)'
        : 'Message (Ctrl+Enter to commit)'

    for (const group of status.groups ?? []) {
      const target = groupById[group.id]
      if (!target) {
        continue
      }
      target.resourceStates = (group.resources ?? []).map((resource) =>
        toResourceState(vscode, {
          ...resource,
          group_id: resource.group_id || group.id,
        }),
      )
    }

    for (const [id, target] of Object.entries(groupById)) {
      if (!(status.groups ?? []).some((group) => group.id === id)) {
        target.resourceStates = []
      }
    }
  }

  function registerCommand(
    id: string,
    callback: (...args: any[]) => unknown,
  ): { dispose: () => void } {
    try {
      return vscode.commands.registerCommand(id, callback)
    } catch (error) {
      // HMR / remount can re-register the same id; keep going so refresh still runs.
      console.warn(`[termbridge-scm] command already registered: ${id}`, error)
      return { dispose() {} }
    }
  }

  disposables.push(
    registerCommand(CMD_REFRESH, () => refresh()),
    registerCommand(
      CMD_OPEN,
      async (uri?: import('vscode').Uri, groupId?: string) => {
        if (!uri) {
          return
        }
        await openScmResource(vscode, uri, groupId || 'changes')
      },
    ),
    registerCommand(
      CMD_STAGE,
      async (resourceUri?: import('vscode').Uri) => {
        if (!resourceUri) {
          return
        }
        await runMutation(api, ScmCommand.SCM_COMMAND_STAGE, uriPath(resourceUri), 'changes')
        await refresh()
      },
    ),
    registerCommand(
      CMD_UNSTAGE,
      async (resourceUri?: import('vscode').Uri) => {
        if (!resourceUri) {
          return
        }
        await runMutation(api, ScmCommand.SCM_COMMAND_UNSTAGE, uriPath(resourceUri), 'staged')
        await refresh()
      },
    ),
    registerCommand(
      CMD_DISCARD,
      async (resourceUri?: import('vscode').Uri) => {
        if (!resourceUri) {
          return
        }
        const confirmed = await vscode.window.showWarningMessage(
          `Discard changes in ${uriPath(resourceUri)}?`,
          { modal: true },
          'Discard',
        )
        if (confirmed !== 'Discard') {
          return
        }
        await runMutation(api, ScmCommand.SCM_COMMAND_DISCARD, uriPath(resourceUri), 'changes')
        await refresh()
      },
    ),
    registerCommand(CMD_COMMIT, async () => {
      const message = sourceControl.inputBox.value.trim()
      if (!message) {
        await vscode.window.showWarningMessage('Commit message is required.')
        return
      }
      const result = await api.execute({
        command: ScmCommand.SCM_COMMAND_COMMIT,
        message,
        path: '',
        group_id: '',
        branch_name: '',
      })
      if (result.operation_state !== ScmOperationState.SCM_OPERATION_STATE_OK) {
        await vscode.window.showErrorMessage(result.message || 'Commit failed.')
        return
      }
      sourceControl.inputBox.value = ''
      await refresh()
    }),
  )

  disposables.push(sourceControl)
  await refresh()

  return {
    refresh,
    dispose() {
      for (const disposable of disposables.splice(0)) {
        disposable.dispose()
      }
    },
  }
}

async function openScmResource(
  vscode: VscodeApi,
  modifiedUri: import('vscode').Uri,
  groupId: string,
): Promise<void> {
  const path = uriPath(modifiedUri)
  const originalUri = scmOriginalUri(vscode, path, groupId)
  const title = diffTitle(path, groupId)
  await vscode.commands.executeCommand('vscode.diff', originalUri, modifiedUri, title)
}

function diffTitle(path: string, groupId: string): string {
  switch (groupId) {
    case 'staged':
      return `${path} (Index)`
    case 'untracked':
      return `${path} (Untracked)`
    default:
      return `${path} (Working Tree)`
  }
}

function toResourceState(vscode: VscodeApi, resource: ScmResource) {
  const uri = workspacePathToUri(vscode, resource.path)
  const groupId = resource.group_id || 'changes'
  const openCommand = {
    title: 'Open Diff',
    command: CMD_OPEN,
    arguments: [uri, groupId],
  }

  return {
    resourceUri: uri,
    command: openCommand,
    decorations: resource.decorations
      ? {
          strikeThrough: resource.decorations.strike_through,
          faded: resource.decorations.faded,
          tooltip: resource.decorations.tooltip || undefined,
          letter: resource.decorations.letter || undefined,
        }
      : undefined,
    contextValue: groupId,
  }
}

function uriPath(uri: import('vscode').Uri): string {
  return uri.path.replace(/^\/+/, '')
}

async function runMutation(
  api: WorkbenchScmApi,
  command: ScmCommand,
  path: string,
  groupId: string,
): Promise<void> {
  const result = await api.execute({
    command,
    path,
    group_id: groupId,
    message: '',
    branch_name: '',
  })
  if (result.operation_state !== ScmOperationState.SCM_OPERATION_STATE_OK) {
    throw new Error(result.message || 'SCM operation failed')
  }
}

function countResources(groups: ScmResourceGroup[]): number {
  return groups.reduce((sum, group) => sum + (group.resources?.length ?? 0), 0)
}

