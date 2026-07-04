import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { WorkspaceTreeSummary } from '../protocol/terminal'
import type { SessionRuntimeApi } from '../features/sessions/runtime'
import { useWorkspaceSessionsStore } from './workspaceSessions'

const runtimeApi = {
  listWorkspaceTree: vi.fn(),
  updateWorkspaceOrder: vi.fn(),
  updateSessionOrder: vi.fn(),
} as unknown as SessionRuntimeApi

const target = { mode: 'cloud' as const, deviceId: 'device-1' }

const workspaceTree: WorkspaceTreeSummary[] = [
  {
    id: 'workspace-1',
    name: 'Workspace One',
    path: '/work/one',
    updated_at: '2026-06-24T00:00:00Z',
    children: [
      {
        id: 'session-1',
        name: 'Shell',
        command: 'bash',
        cwd: '/work/one',
        lifecycle_state: 'running',
        updated_at: '2026-06-24T00:00:00Z',
      },
      {
        id: 'session-2',
        name: 'Build',
        command: 'npm run build',
        cwd: '/work/one',
        lifecycle_state: 'stopped',
        updated_at: '2026-06-24T00:00:01Z',
      },
    ],
  },
  {
    id: 'workspace-2',
    name: 'Workspace Two',
    path: '/work/two',
    updated_at: '2026-06-24T00:00:00Z',
    children: [],
  },
]

describe('useWorkspaceSessionsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
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
    expect(store.sessions).toEqual([
      {
        id: 'session-1',
        workspace_id: 'workspace-1',
        name: 'Shell',
        command: 'bash',
        cwd: '/work/one',
        lifecycle_state: 'running',
        updated_at: '2026-06-24T00:00:00Z',
      },
      {
        id: 'session-2',
        workspace_id: 'workspace-1',
        name: 'Build',
        command: 'npm run build',
        cwd: '/work/one',
        lifecycle_state: 'stopped',
        updated_at: '2026-06-24T00:00:01Z',
      },
    ])
    expect(store.sessionById('workspace-1', 'session-1')?.workspace_id).toBe('workspace-1')
  })

  it('upserts and removes sessions within workspaceTree', () => {
    const store = useWorkspaceSessionsStore()

    store.applyWorkspaceTree(workspaceTree)

    expect(
      store.upsertSession({
        id: 'session-3',
        workspace_id: 'workspace-1',
        name: 'New Shell',
        command: 'zsh',
        cwd: '/work/one',
        lifecycle_state: 'stopped',
        updated_at: '2026-06-24T00:00:01Z',
      }),
    ).toBe(true)
    expect(store.sessionById('workspace-1', 'session-3')?.name).toBe('New Shell')

    store.removeSession('workspace-1', 'session-3')

    expect(store.sessionById('workspace-1', 'session-3')).toBeNull()
    expect(
      store.upsertSession({
        id: 'session-3',
        workspace_id: 'missing-workspace',
        name: 'Missing',
        command: 'bash',
        cwd: '/tmp',
        lifecycle_state: 'running',
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

  it('reorders sessions within one workspace', async () => {
    const store = useWorkspaceSessionsStore()

    store.applyWorkspaceTree(workspaceTree)
    vi.mocked(runtimeApi.updateSessionOrder).mockResolvedValueOnce([
      workspaceTree[0].children[1],
      workspaceTree[0].children[0],
    ])

    await store.reorderSessions(target, runtimeApi, 'workspace-1', ['session-2', 'session-1'])

    expect(runtimeApi.updateSessionOrder).toHaveBeenCalledWith('workspace-1', [
      'session-2',
      'session-1',
    ])
    expect(store.workspaceTree[0].children.map((session) => session.id)).toEqual([
      'session-2',
      'session-1',
    ])
    expect(store.workspaceTree[1].children).toEqual([])
  })

  it('rolls back optimistic session reorder when the API fails', async () => {
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
  })
})
