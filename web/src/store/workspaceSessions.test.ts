import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { WorkspaceTreeSummary } from '../protocol/terminal'
import { useWorkspaceSessionsStore } from './workspaceSessions'

const mocks = vi.hoisted(() => ({
  listWorkspaceTree: vi.fn(),
  updateWorkspaceOrder: vi.fn(),
}))

vi.mock('../features/workspaces/api', () => ({
  listWorkspaceTree: mocks.listWorkspaceTree,
  updateWorkspaceOrder: mocks.updateWorkspaceOrder,
}))

const workspaceTree: WorkspaceTreeSummary[] = [
  {
    id: 'workspace-1',
    name: 'Workspace One',
    path: '/work/one',
    sort_order: 1,
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
    ],
  },
  {
    id: 'workspace-2',
    name: 'Workspace Two',
    path: '/work/two',
    sort_order: 2,
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
        sort_order: 1,
        updated_at: '2026-06-24T00:00:00Z',
      },
      {
        id: 'workspace-2',
        name: 'Workspace Two',
        path: '/work/two',
        sort_order: 2,
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
    ])
    expect(store.sessionById('workspace-1', 'session-1')?.workspace_id).toBe('workspace-1')
  })

  it('upserts and removes sessions within workspaceTree', () => {
    const store = useWorkspaceSessionsStore()

    store.applyWorkspaceTree(workspaceTree)

    expect(
      store.upsertSession({
        id: 'session-2',
        workspace_id: 'workspace-1',
        name: 'New Shell',
        command: 'zsh',
        cwd: '/work/one',
        lifecycle_state: 'stopped',
        updated_at: '2026-06-24T00:00:01Z',
      }),
    ).toBe(true)
    expect(store.sessionById('workspace-1', 'session-2')?.name).toBe('New Shell')

    store.removeSession('workspace-1', 'session-2')

    expect(store.sessionById('workspace-1', 'session-2')).toBeNull()
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
    ])
    expect(store.workspaceById('workspace-1')).toBeNull()
  })

  it('rolls back optimistic workspace reorder when the API fails', async () => {
    const store = useWorkspaceSessionsStore()
    const error = new Error('failed')

    store.applyWorkspaceTree(workspaceTree)
    mocks.updateWorkspaceOrder.mockRejectedValueOnce(error)

    await expect(
      store.reorderWorkspaces('device-1', ['workspace-2', 'workspace-1']),
    ).rejects.toThrow(error)
    expect(store.workspaceTree.map((workspace) => workspace.id)).toEqual([
      'workspace-1',
      'workspace-2',
    ])
  })
})
