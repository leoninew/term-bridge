import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../router', () => ({
  router: {
    currentRoute: { value: { name: 'agent-sessions', fullPath: '/agent/sessions' } },
    push: vi.fn(),
  },
}))

import { terminalWsUrl } from './runtime'

describe('sessions runtime helpers', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('builds same-origin terminal websocket path by default', () => {
    vi.stubGlobal('window', { __CONFIG__: {} })

    expect(terminalWsUrl({ mode: 'agent' }, 'workspace 1', 'session 1')).toBe(
      '/agent-api/workspaces/workspace%201/sessions/session%201/ws',
    )
  })

  it('builds split-origin terminal websocket URL with token and size', () => {
    vi.stubGlobal('window', { __CONFIG__: { cloudApiBaseUrl: 'https://cloud.example.com' } })

    expect(
      terminalWsUrl(
        { mode: 'cloud', deviceId: 'device/1' },
        'workspace-1',
        'session-1',
        'token-1',
        { cols: 500, rows: 0 },
      ),
    ).toBe(
      'wss://cloud.example.com/devices/device%2F1/workspaces/workspace-1/sessions/session-1/ws?token=token-1&cols=500&rows=1',
    )
  })
})
