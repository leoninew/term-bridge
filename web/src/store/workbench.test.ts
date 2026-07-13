import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { SessionSummary } from '../gen/proto/termbridge/agent/v1/workspace'
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
    attachment_state: '',
    command_source: '',
    shortcut_id_snapshot: '',
    shortcut_name_snapshot: '',
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

  it('closes multiple tabs atomically and selects the next surviving tab', async () => {
    const store = useWorkbenchStore()
    const first = session()
    const second = session({ id: 'session-2', workspace_id: 'workspace-2' })
    const third = session({ id: 'session-3', workspace_id: 'workspace-3' })
    const fourth = session({ id: 'session-4', workspace_id: 'workspace-4' })
    const sessions = [first, second, third, fourth]
    const resolver = (workspaceId: string, sessionId: string) =>
      sessions.find((item) => item.workspace_id === workspaceId && item.id === sessionId) ?? null

    for (const item of sessions) {
      await store.openSession(null, runtimeApi, item)
    }
    store.setActiveSession(second.id)

    expect(store.closeTabs([second.id, third.id], resolver)).toEqual(fourth)
    expect(store.openedTabs.map((tab) => tab.sessionId)).toEqual([first.id, fourth.id])
    expect(store.activeSessionId).toBe(fourth.id)
  })

  it('preserves the active tab for background closes and falls back left when needed', async () => {
    const store = useWorkbenchStore()
    const first = session()
    const second = session({ id: 'session-2', workspace_id: 'workspace-2' })
    const third = session({ id: 'session-3', workspace_id: 'workspace-3' })
    const sessions = [first, second, third]
    const resolver = (workspaceId: string, sessionId: string) =>
      sessions.find((item) => item.workspace_id === workspaceId && item.id === sessionId) ?? null

    for (const item of sessions) {
      await store.openSession(null, runtimeApi, item)
    }
    store.setActiveSession(second.id)

    expect(store.closeTabs([third.id], resolver)).toBeNull()
    expect(store.activeSessionId).toBe(second.id)
    expect(store.closeTabs([second.id], resolver)).toEqual(first)
    expect(store.activeSessionId).toBe(first.id)
    expect(store.closeTabs([first.id], resolver)).toEqual(null)
    expect(store.openedTabs).toEqual([])
    expect(store.activeSessionId).toBeNull()
  })

  it('leaves tab state unchanged when bulk targets are not open', async () => {
    const store = useWorkbenchStore()
    const first = session()
    const resolver = vi.fn(() => first)

    await store.openSession(null, runtimeApi, first)

    expect(store.closeTabs(['missing'], resolver)).toBeNull()
    expect(store.openedTabs.map((tab) => tab.sessionId)).toEqual([first.id])
    expect(store.activeSessionId).toBe(first.id)
    expect(resolver).not.toHaveBeenCalled()
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
