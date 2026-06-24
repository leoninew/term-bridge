import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { SessionSummary } from '../protocol/terminal'
import { useWorkbenchStore } from './workbench'

const mocks = vi.hoisted(() => ({
  readHistory: vi.fn(),
}))

vi.mock('../features/sessions/api', () => ({
  readHistory: mocks.readHistory,
}))

function session(overrides: Partial<SessionSummary> = {}): SessionSummary {
  return {
    id: 'session-1',
    workspace_id: 'workspace-1',
    name: 'Shell',
    command: 'bash',
    cwd: '/work/one',
    lifecycle_state: 'stopped',
    updated_at: '2026-06-24T00:00:00Z',
    ...overrides,
  }
}

describe('useWorkbenchStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('opens sessions by session id and loads stopped session history once', async () => {
    const store = useWorkbenchStore()
    mocks.readHistory.mockResolvedValueOnce({ data: 'history text' })

    await expect(store.openSession('device-1', session())).resolves.toBeNull()
    await expect(store.openSession('device-1', session())).resolves.toBeNull()

    expect(store.openedTabs).toHaveLength(1)
    expect(store.activeSessionId).toBe('session-1')
    expect(store.activeTab?.historyText).toBe('history text')
    expect(mocks.readHistory).toHaveBeenCalledTimes(1)
    expect(mocks.readHistory).toHaveBeenCalledWith('device-1', 'workspace-1', 'session-1')
  })

  it('does not read history for running sessions', async () => {
    const store = useWorkbenchStore()

    await store.openSession('device-1', session({ lifecycle_state: 'running' }))

    expect(store.openedTabs).toHaveLength(1)
    expect(mocks.readHistory).not.toHaveBeenCalled()
  })

  it('returns history load errors for callers to present', async () => {
    const store = useWorkbenchStore()

    mocks.readHistory.mockRejectedValueOnce(new Error('history failed'))

    await expect(store.openSession('device-1', session())).resolves.toBe('history failed')
    expect(store.activeTab?.historyError).toBe('history failed')
  })

  it('activates and closes tabs using session id as tab value', async () => {
    const store = useWorkbenchStore()
    const first = session()
    const second = session({ id: 'session-2', workspace_id: 'workspace-2', name: 'Second' })
    const resolver = vi.fn((workspaceId: string, sessionId: string) => {
      if (workspaceId === first.workspace_id && sessionId === first.id) {
        return first
      }
      if (workspaceId === second.workspace_id && sessionId === second.id) {
        return second
      }
      return null
    })

    await store.openSession('device-1', first)
    await store.openSession('device-1', second)
    await store.activateSession('device-1', first.id, resolver)

    expect(store.activeSessionId).toBe(first.id)
    expect(store.activeTab?.workspaceId).toBe(first.workspace_id)

    expect(store.closeTab(first.workspace_id, first.id, resolver)).toEqual(second)
    expect(store.activeSessionId).toBe(second.id)
  })

  it('closes tabs for removed workspace sessions', async () => {
    const store = useWorkbenchStore()

    await store.openSession('device-1', session())
    await store.openSession('device-1', session({ id: 'session-2', workspace_id: 'workspace-2' }))

    store.closeRemovedSessions([{ id: 'session-1', workspace_id: 'workspace-1' }])

    expect(store.openedTabs.map((tab) => tab.sessionId)).toEqual(['session-2'])
    expect(store.activeSessionId).toBe('session-2')
  })
})
