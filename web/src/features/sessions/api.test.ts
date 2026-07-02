import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../router', () => ({
  router: {
    currentRoute: { value: { name: 'sessions', fullPath: '/sessions' } },
    push: vi.fn(),
  },
}))

import { terminalWsUrl } from './api'

describe('sessions api', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('builds same-origin terminal websocket path by default', () => {
    vi.stubGlobal('window', { __CONFIG__: {} })

    expect(terminalWsUrl({ mode: 'local' }, 'workspace 1', 'session 1')).toBe(
      '/api/workspaces/workspace%201/sessions/session%201/ws',
    )
  })

  it('builds split-origin terminal websocket URL with token and size', () => {
    vi.stubGlobal('window', { __CONFIG__: { apiBaseUrl: 'https://api.example.com' } })

    expect(
      terminalWsUrl(
        { mode: 'cloud', deviceId: 'device/1' },
        'workspace-1',
        'session-1',
        'token-1',
        { cols: 500, rows: 0 },
      ),
    ).toBe(
      'wss://api.example.com/api/devices/device%2F1/workspaces/workspace-1/sessions/session-1/ws?token=token-1&cols=500&rows=1',
    )
  })
})
