import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  ScmCommand,
  ScmOperationState,
  ScmState,
  type ScmStatusResp,
} from '../../gen/proto/termbridge/agent/v1/git'
import type { WorkbenchScmApi } from './api'
import { ScmCommandId } from './scmExtensionManifest'
import { registerTermBridgeScm } from './scmProvider'

type ResourceState = {
  resourceUri: Uri
  contextValue?: string
}

type Uri = {
  scheme: string
  authority: string
  path: string
}

type SourceControlFake = {
  inputBox: {
    placeholder: string
    value: string
  }
  acceptInputCommand?: unknown
  actionButton?: unknown
  count: number
  statusBarCommands: unknown[]
  createResourceGroup: ReturnType<typeof vi.fn>
  dispose: ReturnType<typeof vi.fn>
}

const availableStatus = (groups: ScmStatusResp['groups'] = []): ScmStatusResp => ({
  state: ScmState.SCM_STATE_AVAILABLE,
  groups,
  count: groups.reduce((sum, group) => sum + group.resources.length, 0),
  message: '',
})

function resource(path: string, groupId: 'staged' | 'changes' | 'untracked') {
  return {
    path,
    original_path: '',
    group_id: groupId,
    decorations: undefined,
    resource_state: 1,
  }
}

function createVscodeFake() {
  const commands = new Map<string, (...args: unknown[]) => unknown>()
  const groups = new Map<string, { resourceStates: ResourceState[]; hideWhenEmpty?: boolean }>()
  const sourceControl: SourceControlFake = {
    inputBox: { placeholder: '', value: '' },
    count: 0,
    statusBarCommands: [],
    createResourceGroup: vi.fn((id: string) => {
      const group = { resourceStates: [] as ResourceState[] }
      groups.set(id, group)
      return group
    }),
    dispose: vi.fn(),
  }
  const warning = vi.fn<(...args: unknown[]) => Promise<string | undefined>>(async () => undefined)
  const error = vi.fn<(...args: unknown[]) => Promise<void>>(async () => undefined)
  const information = vi.fn<(...args: unknown[]) => Promise<void>>(async () => undefined)
  const vscode = {
    scm: {
      createSourceControl: vi.fn(() => sourceControl),
    },
    workspace: {
      registerTextDocumentContentProvider: vi.fn(() => ({ dispose: vi.fn() })),
    },
    commands: {
      registerCommand: vi.fn((id: string, callback: (...args: unknown[]) => unknown) => {
        commands.set(id, callback)
        return { dispose: vi.fn() }
      }),
      executeCommand: vi.fn(),
    },
    window: {
      showWarningMessage: warning,
      showErrorMessage: error,
      showInformationMessage: information,
    },
    Uri: {
      from: vi.fn((parts: Partial<Uri>) => ({
        scheme: parts.scheme ?? '',
        authority: parts.authority ?? '',
        path: parts.path ?? '',
      })),
    },
  }
  return { vscode, commands, groups, sourceControl, warning, error, information }
}

function createApi(status: ScmStatusResp, execute = vi.fn()) {
  const statusMock = vi.fn(async () => status)
  return {
    status: statusMock,
    repository: vi.fn(async () => ({
      state: ScmState.SCM_STATE_AVAILABLE,
      current_branch: 'main',
      local_branches: [],
      history: [],
      message: '',
    })),
    originalContent: vi.fn(),
    execute,
  } as unknown as WorkbenchScmApi & { status: typeof statusMock }
}

async function registeredProvider(status: ScmStatusResp, execute = vi.fn()) {
  const fake = createVscodeFake()
  const api = createApi(status, execute)
  await registerTermBridgeScm(fake.vscode as unknown as typeof import('vscode'), api, {
    scheme: 'file',
    authority: '',
    path: '/',
  } as import('vscode').Uri)
  return { ...fake, api, execute }
}

beforeEach(() => {
  vi.restoreAllMocks()
})

describe('registerTermBridgeScm', () => {
  it('stages modified SCM resources using their changes group', async () => {
    const status = availableStatus([
      {
        id: 'changes',
        label: 'Changes',
        hide_when_empty: false,
        resources: [resource('src/a.go', 'changes')],
      },
    ])
    const execute = vi.fn(async () => ({
      operation_state: ScmOperationState.SCM_OPERATION_STATE_OK,
    }))
    const { commands, groups, api, information } = await registeredProvider(status, execute)
    const selected = groups.get('changes')?.resourceStates[0]

    await commands.get(ScmCommandId.stage)?.(selected)

    expect(execute).toHaveBeenCalledWith({
      command: ScmCommand.SCM_COMMAND_STAGE,
      path: 'src/a.go',
      group_id: 'changes',
      message: '',
      branch_name: '',
    })
    expect(api.status).toHaveBeenCalledTimes(2)
    expect(information).toHaveBeenCalledWith('Stage completed for 1 file.')
  })

  it('adds untracked resources using the untracked group', async () => {
    const status = availableStatus([
      {
        id: 'untracked',
        label: 'Untracked',
        hide_when_empty: false,
        resources: [resource('new.txt', 'untracked')],
      },
    ])
    const execute = vi.fn(async () => ({
      operation_state: ScmOperationState.SCM_OPERATION_STATE_OK,
    }))
    const { commands, groups } = await registeredProvider(status, execute)

    await commands.get(ScmCommandId.add)?.(groups.get('untracked')?.resourceStates[0])

    expect(execute).toHaveBeenCalledWith(
      expect.objectContaining({
        command: ScmCommand.SCM_COMMAND_STAGE,
        path: 'new.txt',
        group_id: 'untracked',
      }),
    )
  })

  it('opens diffs from static clicks and forwarded resource states in path order', async () => {
    const status = availableStatus([
      {
        id: 'changes',
        label: 'Changes',
        hide_when_empty: false,
        resources: [resource('src/z.go', 'changes'), resource('src/a.go', 'changes')],
      },
    ])
    const { commands, groups, vscode } = await registeredProvider(status)
    const [z, a] = groups.get('changes')?.resourceStates ?? []
    const staticUri = { scheme: 'file', authority: '', path: '/static.go' }

    await commands.get(ScmCommandId.open)?.(staticUri, 'changes')
    await commands.get(ScmCommandId.open)?.(z, a)

    expect(vscode.commands.executeCommand).toHaveBeenNthCalledWith(
      1,
      'vscode.diff',
      { scheme: 'tb-scm', authority: 'changes', path: '/static.go' },
      staticUri,
      'static.go (Working Tree)',
    )
    expect(vscode.commands.executeCommand).toHaveBeenNthCalledWith(
      2,
      'vscode.diff',
      { scheme: 'tb-scm', authority: 'changes', path: '/src/a.go' },
      { scheme: 'file', authority: '', path: '/src/a.go' },
      'src/a.go (Working Tree)',
    )
    expect(vscode.commands.executeCommand).toHaveBeenNthCalledWith(
      3,
      'vscode.diff',
      { scheme: 'tb-scm', authority: 'changes', path: '/src/z.go' },
      { scheme: 'file', authority: '', path: '/src/z.go' },
      'src/z.go (Working Tree)',
    )
  })

  it('stops a directory batch at the first Git failure and refreshes once', async () => {
    const status = availableStatus([
      {
        id: 'changes',
        label: 'Changes',
        hide_when_empty: false,
        resources: [resource('src/a.go', 'changes'), resource('src/b.go', 'changes')],
      },
    ])
    const execute = vi
      .fn()
      .mockResolvedValueOnce({ operation_state: ScmOperationState.SCM_OPERATION_STATE_OK })
      .mockResolvedValueOnce({
        operation_state: ScmOperationState.SCM_OPERATION_STATE_CONFLICT,
        message: 'Git status changed. Refresh, then retry the operation.',
      })
    const { commands, groups, api, error } = await registeredProvider(status, execute)

    await commands.get(ScmCommandId.stage)?.({
      resourceStates: groups.get('changes')?.resourceStates ?? [],
    })

    expect(execute).toHaveBeenCalledTimes(2)
    expect(execute).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({ path: 'src/a.go', group_id: 'changes' }),
    )
    expect(execute).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({ path: 'src/b.go', group_id: 'changes' }),
    )
    expect(api.status).toHaveBeenCalledTimes(2)
    expect(error).toHaveBeenCalledWith(
      'Stage stopped after 1 of 2 files. Failed at src/b.go: Git status changed. Refresh before retrying.',
    )
  })

  it('waits for an in-flight refresh before reconciling a mutation batch', async () => {
    const status = availableStatus([
      {
        id: 'changes',
        label: 'Changes',
        hide_when_empty: false,
        resources: [resource('a.go', 'changes')],
      },
    ])
    let resolveStatus: ((status: ScmStatusResp) => void) | undefined
    const api = createApi(
      status,
      vi.fn(async () => ({ operation_state: ScmOperationState.SCM_OPERATION_STATE_OK })),
    )
    api.status.mockImplementationOnce(() => Promise.resolve(status))
    const fake = createVscodeFake()
    await registerTermBridgeScm(fake.vscode as unknown as typeof import('vscode'), api, {
      scheme: 'file',
      authority: '',
      path: '/',
    } as import('vscode').Uri)
    api.status.mockImplementationOnce(
      () =>
        new Promise<ScmStatusResp>((resolve) => {
          resolveStatus = resolve
        }),
    )
    const staleRefresh = fake.commands.get(ScmCommandId.refresh)?.()
    const stage = fake.commands.get(ScmCommandId.stage)?.(
      fake.groups.get('changes')?.resourceStates[0],
    )

    resolveStatus?.(status)
    await staleRefresh
    await stage

    expect(api.status).toHaveBeenCalledTimes(3)
  })

  it('confirms one discard batch and preserves untracked delete semantics', async () => {
    const status = availableStatus([
      {
        id: 'untracked',
        label: 'Untracked',
        hide_when_empty: false,
        resources: [resource('new.txt', 'untracked'), resource('second.txt', 'untracked')],
      },
    ])
    const execute = vi.fn(async () => ({
      operation_state: ScmOperationState.SCM_OPERATION_STATE_OK,
    }))
    const { commands, groups, warning } = await registeredProvider(status, execute)
    warning.mockResolvedValue('Discard')

    await commands.get(ScmCommandId.discard)?.({
      resourceStates: groups.get('untracked')?.resourceStates ?? [],
    })

    expect(warning).toHaveBeenCalledTimes(1)
    expect(warning).toHaveBeenCalledWith(
      'Discard changes in 2 files?',
      {
        modal: true,
        detail: 'Permanently delete 2 untracked files. This cannot be undone.',
      },
      'Discard',
    )
    expect(execute).toHaveBeenCalledTimes(2)
    expect(execute).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        command: ScmCommand.SCM_COMMAND_DISCARD,
        path: 'new.txt',
        group_id: 'untracked',
      }),
    )
  })

  it('does not mutate or refresh when the discard confirmation is cancelled', async () => {
    const status = availableStatus([
      {
        id: 'changes',
        label: 'Changes',
        hide_when_empty: false,
        resources: [resource('a.go', 'changes')],
      },
    ])
    const execute = vi.fn()
    const { commands, groups, api } = await registeredProvider(status, execute)

    await commands.get(ScmCommandId.discard)?.(groups.get('changes')?.resourceStates[0])

    expect(execute).not.toHaveBeenCalled()
    expect(api.status).toHaveBeenCalledTimes(1)
  })
})
