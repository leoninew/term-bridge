import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { WorkspaceTreeNode as WorkspaceTreeSummary } from '../gen/proto/termbridge/agent/v1/workspace'
import type { ApiResult, SessionRuntimeApi } from '../features/sessions/runtime'
import { useWorkspaceSessionsStore } from './workspaceSessions'

const runtimeApi = {
  listWorkspaceTree: vi.fn(),
  updateWorkspaceOrder: vi.fn(),
  updateSessionOrder: vi.fn(),
} as unknown as SessionRuntimeApi

const target = { mode: 'cloud' as const, deviceId: 'device-1' }

function deferred<T>() {
  let resolve: (value: T) => void
  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve
  })
  return { promise, resolve: resolve! }
}

const workspaceTree: WorkspaceTreeSummary[] = [
  {
    id: 'workspace-1',
    name: 'Workspace One',
    path: '/work/one',
    updated_at: '2026-06-24T00:00:00Z',
    children: [
      {
        id: 'session-1',
        workspace_id: 'workspace-1',
        name: 'Shell',
        command: 'bash',
        cwd: '/work/one',
        lifecycle_state: 'running',
        attachment_state: '',
        command_source: '',
        shortcut_id_snapshot: '',
        shortcut_name_snapshot: '',
        updated_at: '2026-06-24T00:00:00Z',
      },
      {
        id: 'session-2',
        workspace_id: 'workspace-1',
        name: 'Build',
        command: 'npm run build',
        cwd: '/work/one',
        lifecycle_state: 'stopped',
        attachment_state: '',
        command_source: '',
        shortcut_id_snapshot: '',
        shortcut_name_snapshot: '',
        updated_at: '2026-06-24T00:00:01Z',
      },
    ],
  },
  {
    id: 'workspace-2',
    name: 'Workspace Two',
    path: '/work/two',
    updated_at: '2026-06-24T00:00:00Z',
    children: [
      {
        id: 'session-3',
        workspace_id: 'workspace-2',
        name: 'Tests',
        command: 'npm test',
        cwd: '/work/two',
        lifecycle_state: 'running',
        attachment_state: '',
        command_source: '',
        shortcut_id_snapshot: '',
        shortcut_name_snapshot: '',
        updated_at: '2026-06-24T00:00:00Z',
      },
      {
        id: 'session-4',
        workspace_id: 'workspace-2',
        name: 'Logs',
        command: 'tail -f app.log',
        cwd: '/work/two',
        lifecycle_state: 'stopped',
        attachment_state: '',
        command_source: '',
        shortcut_id_snapshot: '',
        shortcut_name_snapshot: '',
        updated_at: '2026-06-24T00:00:01Z',
      },
    ],
  },
]

describe('useWorkspaceSessionsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('keeps the latest workspace refresh when an earlier response finishes last', async () => {
    const store = useWorkspaceSessionsStore()
    const first = deferred<ApiResult<WorkspaceTreeSummary[]>>()
    const second = deferred<ApiResult<WorkspaceTreeSummary[]>>()
    vi.mocked(runtimeApi.listWorkspaceTree)
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise)

    const firstRefresh = store.refresh({ mode: 'cloud', deviceId: 'device-a' }, runtimeApi)
    const secondRefresh = store.refresh({ mode: 'cloud', deviceId: 'device-b' }, runtimeApi)
    second.resolve({ data: [workspaceTree[1]!], offline: false })
    await secondRefresh
    first.resolve({ data: [workspaceTree[0]!], offline: false })
    await firstRefresh

    expect(store.workspaceTree.map((workspace) => workspace.id)).toEqual(['workspace-2'])
    expect(store.loading).toBe(false)
  })

  it('projects workspaces and sessions from workspaceTree', () => {
    const store = useWorkspaceSessionsStore()

    store.applyWorkspaceTree(workspaceTree)

    expect(store.workspaces).toEqual([
      {
        id: 'workspace-1',
        name: 'Workspace One',
        path: '/work/one',
        updated_at: '2026-06-24T00:00:00Z',
      },
      {
        id: 'workspace-2',
        name: 'Workspace Two',
        path: '/work/two',
        updated_at: '2026-06-24T00:00:00Z',
      },
    ])
    expect(store.sessions.map((session) => session.id)).toEqual([
      'session-1',
      'session-2',
      'session-3',
      'session-4',
    ])
    expect(store.sessionById('workspace-1', 'session-1')?.workspace_id).toBe('workspace-1')
  })

  it('upserts and removes sessions within workspaceTree', () => {
    const store = useWorkspaceSessionsStore()

    store.applyWorkspaceTree(workspaceTree)

    expect(
      store.upsertSession({
        id: 'session-5',
        workspace_id: 'workspace-1',
        name: 'New Shell',
        command: 'zsh',
        cwd: '/work/one',
        lifecycle_state: 'stopped',
        attachment_state: '',
        command_source: '',
        shortcut_id_snapshot: '',
        shortcut_name_snapshot: '',
        updated_at: '2026-06-24T00:00:01Z',
      }),
    ).toBe(true)
    expect(store.sessionById('workspace-1', 'session-5')?.name).toBe('New Shell')

    store.removeSession('workspace-1', 'session-5')

    expect(store.sessionById('workspace-1', 'session-5')).toBeNull()
    expect(
      store.upsertSession({
        id: 'session-5',
        workspace_id: 'missing-workspace',
        name: 'Missing',
        command: 'bash',
        cwd: '/tmp',
        lifecycle_state: 'running',
        attachment_state: '',
        command_source: '',
        shortcut_id_snapshot: '',
        shortcut_name_snapshot: '',
        updated_at: '2026-06-24T00:00:02Z',
      }),
    ).toBe(false)
  })

  it('returns removed sessions when removing a workspace', () => {
    const store = useWorkspaceSessionsStore()

    store.applyWorkspaceTree(workspaceTree)

    expect(store.removeWorkspace('workspace-1')).toEqual([
      { id: 'session-1', workspace_id: 'workspace-1' },
      { id: 'session-2', workspace_id: 'workspace-1' },
    ])
    expect(store.workspaceById('workspace-1')).toBeNull()
  })

  it('rolls back optimistic workspace reorder when the API fails', async () => {
    const store = useWorkspaceSessionsStore()
    const error = new Error('failed')

    store.applyWorkspaceTree(workspaceTree)
    vi.mocked(runtimeApi.updateWorkspaceOrder).mockRejectedValueOnce(error)

    await expect(
      store.reorderWorkspaces(target, runtimeApi, ['workspace-2', 'workspace-1']),
    ).rejects.toThrow(error)
    expect(store.workspaceTree.map((workspace) => workspace.id)).toEqual([
      'workspace-1',
      'workspace-2',
    ])
  })

  it('reorders one workspace with the server order while preserving another workspace', async () => {
    const store = useWorkspaceSessionsStore()

    store.applyWorkspaceTree(workspaceTree)
    vi.mocked(runtimeApi.updateSessionOrder).mockResolvedValueOnce([
      workspaceTree[0].children[0],
      workspaceTree[0].children[1],
    ])

    await store.reorderSessions(target, runtimeApi, 'workspace-1', ['session-2', 'session-1'])

    expect(runtimeApi.updateSessionOrder).toHaveBeenCalledWith('workspace-1', [
      'session-2',
      'session-1',
    ])
    expect(store.workspaceTree[0].children.map((session) => session.id)).toEqual([
      'session-1',
      'session-2',
    ])
    expect(store.workspaceTree[1].children.map((session) => session.id)).toEqual([
      'session-3',
      'session-4',
    ])
  })

  it('reorders a later workspace without changing an earlier workspace', async () => {
    const store = useWorkspaceSessionsStore()

    store.applyWorkspaceTree(workspaceTree)
    vi.mocked(runtimeApi.updateSessionOrder).mockResolvedValueOnce([
      workspaceTree[1].children[1],
      workspaceTree[1].children[0],
    ])

    await store.reorderSessions(target, runtimeApi, 'workspace-2', ['session-4', 'session-3'])

    expect(runtimeApi.updateSessionOrder).toHaveBeenCalledWith('workspace-2', [
      'session-4',
      'session-3',
    ])
    expect(store.workspaceTree[0].children.map((session) => session.id)).toEqual([
      'session-1',
      'session-2',
    ])
    expect(store.workspaceTree[1].children.map((session) => session.id)).toEqual([
      'session-4',
      'session-3',
    ])
  })

  it('persists consecutive reorders from one workspace as distinct requests', async () => {
    const store = useWorkspaceSessionsStore()

    store.applyWorkspaceTree(workspaceTree)
    vi.mocked(runtimeApi.updateSessionOrder)
      .mockResolvedValueOnce([workspaceTree[0].children[1], workspaceTree[0].children[0]])
      .mockResolvedValueOnce([workspaceTree[0].children[0], workspaceTree[0].children[1]])

    await store.reorderSessions(target, runtimeApi, 'workspace-1', ['session-2', 'session-1'])
    await store.reorderSessions(target, runtimeApi, 'workspace-1', ['session-1', 'session-2'])

    expect(runtimeApi.updateSessionOrder).toHaveBeenNthCalledWith(1, 'workspace-1', [
      'session-2',
      'session-1',
    ])
    expect(runtimeApi.updateSessionOrder).toHaveBeenNthCalledWith(2, 'workspace-1', [
      'session-1',
      'session-2',
    ])
    expect(store.workspaceTree[0].children.map((session) => session.id)).toEqual([
      'session-1',
      'session-2',
    ])
    expect(store.workspaceTree[1].children.map((session) => session.id)).toEqual([
      'session-3',
      'session-4',
    ])
  })

  it('restores every workspace order when session reorder persistence fails', async () => {
    const store = useWorkspaceSessionsStore()
    const error = new Error('failed')

    store.applyWorkspaceTree(workspaceTree)
    vi.mocked(runtimeApi.updateSessionOrder).mockRejectedValueOnce(error)

    await expect(
      store.reorderSessions(target, runtimeApi, 'workspace-1', ['session-2', 'session-1']),
    ).rejects.toThrow(error)
    expect(store.workspaceTree[0].children.map((session) => session.id)).toEqual([
      'session-1',
      'session-2',
    ])
    expect(store.workspaceTree[1].children.map((session) => session.id)).toEqual([
      'session-3',
      'session-4',
    ])
  })

  it('does not reorder sessions when no runtime target is selected', async () => {
    const store = useWorkspaceSessionsStore()

    store.applyWorkspaceTree(workspaceTree)

    await store.reorderSessions(null, runtimeApi, 'workspace-1', ['session-2', 'session-1'])

    expect(runtimeApi.updateSessionOrder).not.toHaveBeenCalled()
    expect(store.workspaceTree[0].children.map((session) => session.id)).toEqual([
      'session-1',
      'session-2',
    ])
    expect(store.workspaceTree[1].children.map((session) => session.id)).toEqual([
      'session-3',
      'session-4',
    ])
  })
})
