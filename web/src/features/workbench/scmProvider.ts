import {
  ScmCommand,
  ScmOperationState,
  ScmState,
  type ScmResource,
  type ScmResourceGroup,
  type ScmStatusResp,
} from '../../gen/proto/termbridge/agent/v1/git'
import type { WorkbenchScmApi } from './api'
import { ScmCommandId, TERM_BRIDGE_SCM_PROVIDER_ID } from './scmExtensionManifest'
import { parseScmOriginalUri, SCM_ORIGINAL_SCHEME, scmOriginalUri, workspacePathToUri } from './uri'

type VscodeApi = typeof import('vscode')
type ScmGroupId = 'staged' | 'changes' | 'untracked'

type ScmResourceTarget = {
  path: string
  groupId: ScmGroupId
}

type ScmResourceStateArgument = {
  resourceUri: { path: string }
  contextValue?: unknown
}

type ScmResourceGroupArgument = {
  resourceStates: unknown[]
}

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
    TERM_BRIDGE_SCM_PROVIDER_ID,
    'Git',
    workspaceFolderUri,
  )
  sourceControl.inputBox.placeholder = 'Message (Ctrl+Enter to commit)'
  sourceControl.acceptInputCommand = {
    command: ScmCommandId.commit,
    title: 'Commit',
  }
  sourceControl.actionButton = {
    command: {
      command: ScmCommandId.commit,
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

  const groupById: Record<ScmGroupId, typeof staged> = {
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

  // Trailing single-flight: callers during an active refresh coalesce, then one
  // follow-up round runs so the active request cannot publish an old snapshot.
  let refreshInflight: Promise<void> | null = null
  let refreshPending = false

  async function refresh(): Promise<void> {
    if (refreshInflight) {
      refreshPending = true
      return refreshInflight
    }
    refreshInflight = (async () => {
      do {
        refreshPending = false
        await doRefresh()
      } while (refreshPending)
    })().finally(() => {
      refreshInflight = null
    })
    return refreshInflight
  }

  async function refreshAfterMutation(): Promise<void> {
    await refresh()
  }

  async function doRefresh(): Promise<void> {
    let status: ScmStatusResp
    try {
      status = await api.status()
    } catch (error) {
      console.error('[termbridge-scm] status failed', error)
      sourceControl.count = 0
      sourceControl.statusBarCommands = [
        {
          command: ScmCommandId.refresh,
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
          command: ScmCommandId.refresh,
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
        command: ScmCommandId.refresh,
        title: repositoryLabel,
        tooltip: 'Refresh SCM',
      },
    ]
    sourceControl.inputBox.placeholder =
      count === 0 ? 'No changes (edit a file to see them here)' : 'Message (Ctrl+Enter to commit)'

    for (const group of status.groups ?? []) {
      if (!isScmGroupId(group.id)) {
        continue
      }
      groupById[group.id].resourceStates = (group.resources ?? []).map((resource) =>
        toResourceState(vscode, {
          ...resource,
          group_id: isScmGroupId(resource.group_id) ? resource.group_id : group.id,
        }),
      )
    }

    for (const [id, target] of Object.entries(groupById) as [ScmGroupId, typeof staged][]) {
      if (!(status.groups ?? []).some((group) => group.id === id)) {
        target.resourceStates = []
      }
    }
  }

  function registerCommand(
    id: string,
    callback: (...args: unknown[]) => unknown,
  ): { dispose: () => void } {
    try {
      return vscode.commands.registerCommand(id, callback)
    } catch (error) {
      // HMR / remount can re-register the same id; keep going so refresh still runs.
      console.warn(`[termbridge-scm] command already registered: ${id}`, error)
      return { dispose() {} }
    }
  }

  async function mutateSelectedResources(
    command: ScmCommand,
    operation: string,
    allowedGroups: readonly ScmGroupId[],
    args: unknown[],
  ): Promise<void> {
    const targets = resourceTargets(args, allowedGroups)
    if (targets.length === 0) {
      await vscode.window.showInformationMessage(
        `No changes are available to ${operation.toLowerCase()}.`,
      )
      return
    }

    if (command === ScmCommand.SCM_COMMAND_DISCARD) {
      const confirmed = await confirmDiscard(vscode, targets)
      if (!confirmed) {
        return
      }
    }

    let completed = 0
    let failedTarget: ScmResourceTarget | undefined
    let failure: unknown
    try {
      for (const target of targets) {
        failedTarget = target
        await runMutation(api, command, target.path, target.groupId)
        completed += 1
      }
    } catch (error) {
      failure = error
    } finally {
      try {
        await refreshAfterMutation()
      } catch (error) {
        console.error('[termbridge-scm] refresh after mutation failed', error)
      }
    }

    if (failure && failedTarget) {
      const reason = mutationFailureMessage(failure)
      await vscode.window.showErrorMessage(
        `${operation} stopped after ${completed} of ${targets.length} files. Failed at ${failedTarget.path}: ${reason}`,
      )
      return
    }

    await vscode.window.showInformationMessage(
      `${operation} completed for ${completed} ${fileCountLabel(completed)}.`,
    )
  }

  disposables.push(
    registerCommand(ScmCommandId.refresh, () => refresh()),
    registerCommand(ScmCommandId.open, async (...args: unknown[]) => {
      const [uri, groupId] = args
      if (isUriArgument(uri)) {
        await openScmResource(vscode, uri, isScmGroupId(groupId) ? groupId : 'changes')
        return
      }
      for (const target of resourceTargets(args, ['staged', 'changes', 'untracked'])) {
        await openScmResource(vscode, workspacePathToUri(vscode, target.path), target.groupId)
      }
    }),
    registerCommand(ScmCommandId.stage, (...args: unknown[]) =>
      mutateSelectedResources(ScmCommand.SCM_COMMAND_STAGE, 'Stage', ['changes'], args),
    ),
    registerCommand(ScmCommandId.add, (...args: unknown[]) =>
      mutateSelectedResources(ScmCommand.SCM_COMMAND_STAGE, 'Add', ['untracked'], args),
    ),
    registerCommand(ScmCommandId.unstage, (...args: unknown[]) =>
      mutateSelectedResources(ScmCommand.SCM_COMMAND_UNSTAGE, 'Unstage', ['staged'], args),
    ),
    registerCommand(ScmCommandId.discard, (...args: unknown[]) =>
      mutateSelectedResources(
        ScmCommand.SCM_COMMAND_DISCARD,
        'Discard',
        ['changes', 'untracked'],
        args,
      ),
    ),
    registerCommand(ScmCommandId.stageAllChanges, (...args: unknown[]) =>
      mutateSelectedResources(ScmCommand.SCM_COMMAND_STAGE, 'Stage', ['changes'], args),
    ),
    registerCommand(ScmCommandId.addAll, (...args: unknown[]) =>
      mutateSelectedResources(ScmCommand.SCM_COMMAND_STAGE, 'Add', ['untracked'], args),
    ),
    registerCommand(ScmCommandId.unstageAll, (...args: unknown[]) =>
      mutateSelectedResources(ScmCommand.SCM_COMMAND_UNSTAGE, 'Unstage', ['staged'], args),
    ),
    registerCommand(ScmCommandId.discardAllChanges, (...args: unknown[]) =>
      mutateSelectedResources(ScmCommand.SCM_COMMAND_DISCARD, 'Discard', ['changes'], args),
    ),
    registerCommand(ScmCommandId.discardAllUntracked, (...args: unknown[]) =>
      mutateSelectedResources(ScmCommand.SCM_COMMAND_DISCARD, 'Discard', ['untracked'], args),
    ),
    registerCommand(ScmCommandId.commit, async () => {
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
  groupId: ScmGroupId,
): Promise<void> {
  const path = uriPath(modifiedUri)
  const originalUri = scmOriginalUri(vscode, path, groupId)
  const title = diffTitle(path, groupId)
  await vscode.commands.executeCommand('vscode.diff', originalUri, modifiedUri, title)
}

function diffTitle(path: string, groupId: ScmGroupId): string {
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
  const groupId = isScmGroupId(resource.group_id) ? resource.group_id : 'changes'
  const openCommand = {
    title: 'Open Diff',
    command: ScmCommandId.open,
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

function resourceTargets(
  args: unknown[],
  allowedGroups: readonly ScmGroupId[],
): ScmResourceTarget[] {
  const allowed = new Set(allowedGroups)
  const targets = new Map<string, ScmResourceTarget>()

  for (const argument of args) {
    for (const resourceState of resourceStates(argument)) {
      const target = resourceTarget(resourceState)
      if (!target || !allowed.has(target.groupId)) {
        continue
      }
      targets.set(`${target.groupId}:${target.path}`, target)
    }
  }

  return [...targets.values()].sort(
    (left, right) =>
      left.path.localeCompare(right.path) || left.groupId.localeCompare(right.groupId),
  )
}

function resourceStates(argument: unknown): ScmResourceStateArgument[] {
  if (isScmResourceStateArgument(argument)) {
    return [argument]
  }
  if (!isScmResourceGroupArgument(argument)) {
    return []
  }
  return argument.resourceStates.filter(isScmResourceStateArgument)
}

function resourceTarget(resourceState: ScmResourceStateArgument): ScmResourceTarget | undefined {
  const groupId = resourceState.contextValue
  const path = resourceState.resourceUri.path.replace(/^\/+/, '')
  if (!isScmGroupId(groupId) || !path) {
    return undefined
  }
  return { path, groupId }
}

function isScmResourceStateArgument(value: unknown): value is ScmResourceStateArgument {
  if (!isRecord(value) || !isRecord(value.resourceUri)) {
    return false
  }
  return typeof value.resourceUri.path === 'string'
}

function isUriArgument(value: unknown): value is import('vscode').Uri {
  return isRecord(value) && typeof value.path === 'string'
}

function isScmResourceGroupArgument(value: unknown): value is ScmResourceGroupArgument {
  return isRecord(value) && Array.isArray(value.resourceStates)
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object'
}

function isScmGroupId(value: unknown): value is ScmGroupId {
  return value === 'staged' || value === 'changes' || value === 'untracked'
}

async function confirmDiscard(vscode: VscodeApi, targets: ScmResourceTarget[]): Promise<boolean> {
  const changedCount = targets.filter((target) => target.groupId === 'changes').length
  const untrackedCount = targets.filter((target) => target.groupId === 'untracked').length
  const details: string[] = []
  if (changedCount > 0) {
    details.push(
      `Restore changes in ${changedCount} ${fileCountLabel(changedCount)} to their staged version.`,
    )
  }
  if (untrackedCount > 0) {
    details.push(
      `Permanently delete ${untrackedCount} untracked ${fileCountLabel(untrackedCount)}.`,
    )
  }
  details.push('This cannot be undone.')

  const confirmed = await vscode.window.showWarningMessage(
    `Discard changes in ${targets.length} ${fileCountLabel(targets.length)}?`,
    { modal: true, detail: details.join(' ') },
    'Discard',
  )
  return confirmed === 'Discard'
}

function uriPath(uri: import('vscode').Uri): string {
  return uri.path.replace(/^\/+/, '')
}

class ScmMutationError extends Error {}

async function runMutation(
  api: WorkbenchScmApi,
  command: ScmCommand,
  path: string,
  groupId: ScmGroupId,
): Promise<void> {
  let result
  try {
    result = await api.execute({
      command,
      path,
      group_id: groupId,
      message: '',
      branch_name: '',
    })
  } catch {
    throw new ScmMutationError('Git operation could not be confirmed. Refresh before retrying.')
  }
  if (result.operation_state !== ScmOperationState.SCM_OPERATION_STATE_OK) {
    throw new ScmMutationError(safeMutationFailureMessage(result.operation_state))
  }
}

function safeMutationFailureMessage(state: ScmOperationState): string {
  switch (state) {
    case ScmOperationState.SCM_OPERATION_STATE_CONFLICT:
      return 'Git status changed. Refresh before retrying.'
    case ScmOperationState.SCM_OPERATION_STATE_UNAVAILABLE:
      return 'Git is unavailable. Refresh before retrying.'
    default:
      return 'Git operation failed. Refresh before retrying.'
  }
}

function mutationFailureMessage(error: unknown): string {
  if (error instanceof ScmMutationError) {
    return error.message
  }
  return 'Git operation failed. Refresh before retrying.'
}

function fileCountLabel(count: number): string {
  return count === 1 ? 'file' : 'files'
}

function countResources(groups: ScmResourceGroup[]): number {
  return groups.reduce((sum, group) => sum + (group.resources?.length ?? 0), 0)
}
