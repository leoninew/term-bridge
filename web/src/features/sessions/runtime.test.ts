import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { BrowserRuntimeConfig } from '../../config'

vi.mock('../../router', () => ({
  router: {
    currentRoute: { value: { name: 'local-sessions', fullPath: '/sessions' } },
    push: vi.fn(),
  },
}))

import { terminalWsUrl } from './runtime'

function storageMock(): Storage {
  const values = new Map<string, string>()
  return {
    get length() {
      return values.size
    },
    clear: vi.fn(() => values.clear()),
    getItem: vi.fn((key: string) => values.get(key) ?? null),
    key: vi.fn((index: number) => Array.from(values.keys())[index] ?? null),
    removeItem: vi.fn((key: string) => values.delete(key)),
    setItem: vi.fn((key: string, value: string) => values.set(key, value)),
  }
}

function runtimeConfig(overrides: BrowserRuntimeConfig = {}): BrowserRuntimeConfig {
  return {
    local: {
      mode: 'hybrid',
      publicUrl: 'http://localhost:9030',
      apiBasePath: '/local-api',
      cloudOAuth: {
        clientId: 'termbridge-agent',
        redirectUrl: 'http://localhost:9030/oauth/callback',
        scopes: ['openid', 'email', 'profile'],
      },
      ...overrides.local,
    },
    cloud: {
      publicUrl: 'http://localhost:9030',
      apiBaseUrl: 'https://cloud.example.test/api',
      ...overrides.cloud,
    },
  }
}

function stubBrowser(config: BrowserRuntimeConfig) {
  vi.stubGlobal('window', {
    __CONFIG__: config,
    localStorage: storageMock(),
    sessionStorage: storageMock(),
  })
}

describe('sessions runtime helpers', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    stubBrowser(runtimeConfig())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('builds same-origin terminal websocket path from local API config', () => {
    expect(terminalWsUrl({ mode: 'local' }, 'workspace 1', 'session 1')).toBe(
      '/local-api/workspaces/workspace%201/sessions/session%201/ws',
    )
  })

  it('builds split-origin terminal websocket URL with token and size', () => {
    stubBrowser(runtimeConfig({ cloud: { apiBaseUrl: 'https://cloud.example.com/api' } }))
    setActivePinia(createPinia())

    expect(
      terminalWsUrl(
        { mode: 'cloud', deviceId: 'device/1' },
        'workspace-1',
        'session-1',
        'token-1',
        { cols: 500, rows: 0 },
      ),
    ).toBe(
      'wss://cloud.example.com/api/devices/device%2F1/workspaces/workspace-1/sessions/session-1/ws?token=token-1&cols=500&rows=1',
    )
  })
})
