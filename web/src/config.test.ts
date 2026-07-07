import { afterEach, describe, expect, it, vi } from 'vitest'
import { buildApiUrl, buildApiWebSocketUrl, getApiBaseUrl, getFrontendMode, runtimeConfig } from './config'

describe('runtime config', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.unstubAllEnvs()
  })

  it('uses explicit agent and cloud API paths by default', () => {
    vi.stubGlobal('window', { __CONFIG__: {} })

    expect(getApiBaseUrl('agent')).toBe('/agent-api')
    expect(getApiBaseUrl('cloud')).toBe('/cloud-api')
    expect(buildApiUrl('/health', 'agent')).toBe('/agent-api/health')
    expect(buildApiWebSocketUrl('/sessions/session-1/ws', 'agent')).toBe(
      '/agent-api/sessions/session-1/ws',
    )
  })

  it('uses runtime target API base URLs and trims trailing slashes', () => {
    vi.stubGlobal('window', {
      __CONFIG__: {
        agentApiBaseUrl: 'https://agent.example.com/',
        cloudApiBaseUrl: 'https://cloud.example.com/',
      },
    })

    expect(getApiBaseUrl('agent')).toBe('https://agent.example.com')
    expect(getApiBaseUrl('cloud')).toBe('https://cloud.example.com')
    expect(buildApiUrl('/health', 'agent')).toBe('https://agent.example.com/health')
    expect(buildApiUrl('/devices', 'cloud')).toBe('https://cloud.example.com/devices')
  })

  it('uses separate agent and cloud API base URLs', () => {
    vi.stubGlobal('window', {
      __CONFIG__: { agentApiBaseUrl: '/agent-api/', cloudApiBaseUrl: '/cloud-api/' },
    })

    expect(getApiBaseUrl('agent')).toBe('/agent-api')
    expect(getApiBaseUrl('cloud')).toBe('/cloud-api')
    expect(buildApiUrl('/health', 'agent')).toBe('/agent-api/health')
    expect(buildApiUrl('/devices', 'cloud')).toBe('/cloud-api/devices')
  })

  it('builds websocket URLs from target runtime API base URL', () => {
    vi.stubGlobal('window', { __CONFIG__: { agentApiBaseUrl: 'https://agent.example.com' } })

    expect(buildApiWebSocketUrl('/sessions/session-1/ws?token=t1', 'agent')).toBe(
      'wss://agent.example.com/sessions/session-1/ws?token=t1',
    )
  })

  it('converts http API base URL to ws websocket URL', () => {
    vi.stubGlobal('window', { __CONFIG__: { agentApiBaseUrl: 'http://127.0.0.1:9030' } })

    expect(buildApiWebSocketUrl('/sessions/session-1/ws', 'agent')).toBe(
      'ws://127.0.0.1:9030/sessions/session-1/ws',
    )
  })

  it('keeps path-based websocket URLs for Vite dev proxy', () => {
    vi.stubGlobal('window', { __CONFIG__: { cloudApiBaseUrl: '/cloud-api' } })

    expect(buildApiWebSocketUrl('/devices/device-1/sessions/session-1/ws', 'cloud')).toBe(
      '/cloud-api/devices/device-1/sessions/session-1/ws',
    )
  })

  it('reads frontend mode from runtime config', () => {
    vi.stubGlobal('window', { __CONFIG__: { frontendMode: 'hybrid' } })

    expect(getFrontendMode()).toBe('hybrid')
  })

  it('falls back to agent mode when frontend mode is invalid', () => {
    vi.stubGlobal('window', { __CONFIG__: { frontendMode: 'invalid' } })

    expect(getFrontendMode()).toBe('agent')
  })

  it('uses Vite Cloud OAuth public config when runtime config is absent', () => {
    vi.stubGlobal('window', { __CONFIG__: {} })
    vi.stubEnv('VITE_CLOUD_OAUTH_CLIENT_ID', 'termbridge-agent')
    vi.stubEnv('VITE_CLOUD_OAUTH_REDIRECT_URL', 'http://localhost:9030/agent/oauth/callback')
    vi.stubEnv('VITE_CLOUD_OAUTH_SCOPES', 'openid,email,profile')

    expect(runtimeConfig.cloudOAuth).toEqual({
      clientId: 'termbridge-agent',
      redirectUrl: 'http://localhost:9030/agent/oauth/callback',
      scopes: ['openid', 'email', 'profile'],
    })
  })
})
