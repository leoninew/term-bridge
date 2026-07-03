import { afterEach, describe, expect, it, vi } from 'vitest'
import { buildApiUrl, buildApiWebSocketUrl, getApiBaseUrl } from './config'

describe('runtime config', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('uses same-origin API paths when runtime api base URL is empty', () => {
    vi.stubGlobal('window', { __CONFIG__: {} })

    expect(getApiBaseUrl()).toBe('')
    expect(buildApiUrl('/api/health')).toBe('/api/health')
    expect(buildApiWebSocketUrl('/api/sessions/session-1/ws')).toBe('/api/sessions/session-1/ws')
  })

  it('uses runtime API base URL and trims a trailing slash', () => {
    vi.stubGlobal('window', { __CONFIG__: { apiBaseUrl: 'https://api.example.com/' } })

    expect(getApiBaseUrl()).toBe('https://api.example.com')
    expect(buildApiUrl('/api/health')).toBe('https://api.example.com/api/health')
  })

  it('uses separate agent and cloud API base URLs', () => {
    vi.stubGlobal('window', {
      __CONFIG__: { agentApiBaseUrl: '/agent-api/', cloudApiBaseUrl: '/cloud-api/' },
    })

    expect(getApiBaseUrl('agent')).toBe('/agent-api')
    expect(getApiBaseUrl('cloud')).toBe('/cloud-api')
    expect(buildApiUrl('/api/health', 'agent')).toBe('/agent-api/api/health')
    expect(buildApiUrl('/api/devices', 'cloud')).toBe('/cloud-api/api/devices')
  })

  it('builds websocket URLs from runtime API base URL', () => {
    vi.stubGlobal('window', { __CONFIG__: { apiBaseUrl: 'https://api.example.com' } })

    expect(buildApiWebSocketUrl('/api/sessions/session-1/ws?token=t1')).toBe(
      'wss://api.example.com/api/sessions/session-1/ws?token=t1',
    )
  })

  it('converts http API base URL to ws websocket URL', () => {
    vi.stubGlobal('window', { __CONFIG__: { apiBaseUrl: 'http://127.0.0.1:9030' } })

    expect(buildApiWebSocketUrl('/api/sessions/session-1/ws')).toBe(
      'ws://127.0.0.1:9030/api/sessions/session-1/ws',
    )
  })

  it('keeps path-based websocket URLs for Vite dev proxy', () => {
    vi.stubGlobal('window', { __CONFIG__: { cloudApiBaseUrl: '/cloud-api' } })

    expect(buildApiWebSocketUrl('/api/devices/device-1/sessions/session-1/ws', 'cloud')).toBe(
      '/cloud-api/api/devices/device-1/sessions/session-1/ws',
    )
  })
})
