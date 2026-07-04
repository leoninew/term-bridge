import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { SessionSummary } from '../protocol/terminal'
import type { SessionRuntimeApi } from '../features/sessions/runtime'
import { useWorkbenchStore } from './workbench'

const target = { mode: 'cloud' as const, deviceId: 'device-1' }
const runtimeApi = {
  readHistory: vi.fn(),
} as unknown as SessionRuntimeApi

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
    vi.mocked(runtimeApi.readHistory).mockResolvedValueOnce({
      data: 'history text',
      offline: false,
    })

    await expect(store.openSession(target, runtimeApi, session())).resolves.toBeNull()
    await expect(store.openSession(target, runtimeApi, session())).resolves.toBeNull()

    expect(store.openedTabs).toHaveLength(1)
    expect(store.activeSessionId).toBe('session-1')
    expect(store.activeTab?.historyText).toBe('history text')
    expect(runtimeApi.readHistory).toHaveBeenCalledTimes(1)
    expect(runtimeApi.readHistory).toHaveBeenCalledWith('workspace-1', 'session-1')
  })

  it('does not read history for running sessions', async () => {
    const store = useWorkbenchStore()

    await store.openSession(target, runtimeApi, session({ lifecycle_state: 'running' }))

    expect(store.openedTabs).toHaveLength(1)
    expect(runtimeApi.readHistory).not.toHaveBeenCalled()
  })

  it('returns history load errors for callers to present', async () => {
    const store = useWorkbenchStore()

    vi.mocked(runtimeApi.readHistory).mockRejectedValueOnce(new Error('history failed'))

    await expect(store.openSession(target, runtimeApi, session())).resolves.toBe('history failed')
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

    await store.openSession(target, runtimeApi, first)
    await store.openSession(target, runtimeApi, second)
    await store.activateSession(target, runtimeApi, first.id, resolver)

    expect(store.activeSessionId).toBe(first.id)
    expect(store.activeTab?.workspaceId).toBe(first.workspace_id)

    expect(store.closeTab(first.workspace_id, first.id, resolver)).toEqual(second)
    expect(store.activeSessionId).toBe(second.id)
  })

  it('closes tabs for removed workspace sessions', async () => {
    const store = useWorkbenchStore()

    await store.openSession(target, runtimeApi, session())
    await store.openSession(
      target,
      runtimeApi,
      session({ id: 'session-2', workspace_id: 'workspace-2' }),
    )

    store.closeRemovedSessions([{ id: 'session-1', workspace_id: 'workspace-1' }])

    expect(store.openedTabs.map((tab) => tab.sessionId)).toEqual(['session-2'])
    expect(store.activeSessionId).toBe('session-2')
  })
})
