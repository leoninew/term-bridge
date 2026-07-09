import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { BrowserRuntimeConfig } from '../config'
import { useRuntimeConfigStore } from './runtimeConfig'

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
    agent: {
      mode: 'hybrid',
      publicUrl: 'http://localhost:9030',
      apiBaseUrl: '/agent-api',
      cloudOAuth: {
        clientId: 'termbridge-agent',
        redirectUrl: 'http://localhost:9030/agent/oauth/callback',
        scopes: ['openid', 'email', 'profile'],
      },
      ...overrides.agent,
    },
    cloud: {
      publicUrl: 'http://termbridge.lvh.me',
      apiBaseUrl: 'http://termbridge.lvh.me/cloud-api',
      ...overrides.cloud,
    },
  }
}

function stubWindowConfig(config: BrowserRuntimeConfig, sessionStorage = storageMock()) {
  vi.stubGlobal('window', {
    __CONFIG__: config,
    localStorage: storageMock(),
    sessionStorage,
  })
}

describe('runtime config store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.unstubAllEnvs()
    vi.unstubAllGlobals()
  })

  it('parses nested runtime config injected by the server', () => {
    stubWindowConfig(
      runtimeConfig({ agent: { mode: 'cloud', apiBaseUrl: 'https://agent.example.com/' } }),
    )

    const store = useRuntimeConfigStore()

    expect(store.config.agent.mode).toBe('cloud')
    expect(store.config.agent.apiBaseUrl).toBe('https://agent.example.com')
    expect(store.config.cloud.publicUrl).toBe('http://termbridge.lvh.me')
    expect(store.config.cloud.apiBaseUrl).toBe('http://termbridge.lvh.me/cloud-api')
    expect(store.view.mode).toBe('cloud')
  })

  it('builds nested config from Vite env when no runtime config is injected', () => {
    vi.stubGlobal('window', {
      localStorage: storageMock(),
      sessionStorage: storageMock(),
    })
    vi.stubEnv('TERMBRIDGE_AGENT__MODE', 'hybrid')
    vi.stubEnv('TERMBRIDGE_AGENT__PUBLIC_URL', 'http://localhost:9030')
    vi.stubEnv('TERMBRIDGE_AGENT__API_BASE_URL', '/agent-api')
    vi.stubEnv('TERMBRIDGE_AGENT__OAUTH__CLIENT_ID', 'termbridge-agent')
    vi.stubEnv(
      'TERMBRIDGE_AGENT__OAUTH__REDIRECT_URL',
      'http://localhost:9030/agent/oauth/callback',
    )
    vi.stubEnv('TERMBRIDGE_AGENT__OAUTH__SCOPES', 'openid,email,profile')
    vi.stubEnv('TERMBRIDGE_CLOUD__PUBLIC_URL', 'https://cloud.example.com/')
    vi.stubEnv('TERMBRIDGE_CLOUD__API_BASE_URL', 'https://cloud.example.com/cloud-api/')

    const store = useRuntimeConfigStore()

    expect(store.config).toEqual({
      agent: {
        mode: 'hybrid',
        publicUrl: 'http://localhost:9030',
        apiBaseUrl: '/agent-api',
        cloudOAuth: {
          clientId: 'termbridge-agent',
          redirectUrl: 'http://localhost:9030/agent/oauth/callback',
          scopes: ['openid', 'email', 'profile'],
        },
      },
      cloud: {
        publicUrl: 'https://cloud.example.com',
        apiBaseUrl: 'https://cloud.example.com/cloud-api',
      },
    })
  })

  it('ignores empty runtime config placeholder and builds from Vite env', () => {
    vi.stubGlobal('window', {
      __CONFIG__: {},
      localStorage: storageMock(),
      sessionStorage: storageMock(),
    })
    vi.stubEnv('TERMBRIDGE_AGENT__MODE', 'cloud')
    vi.stubEnv('TERMBRIDGE_AGENT__PUBLIC_URL', 'http://localhost:9030')
    vi.stubEnv('TERMBRIDGE_AGENT__API_BASE_URL', '/agent-api')
    vi.stubEnv('TERMBRIDGE_AGENT__OAUTH__CLIENT_ID', 'termbridge-agent')
    vi.stubEnv(
      'TERMBRIDGE_AGENT__OAUTH__REDIRECT_URL',
      'http://localhost:9030/agent/oauth/callback',
    )
    vi.stubEnv('TERMBRIDGE_AGENT__OAUTH__SCOPES', 'openid,email,profile')
    vi.stubEnv('TERMBRIDGE_CLOUD__PUBLIC_URL', 'http://localhost:9030')
    vi.stubEnv('TERMBRIDGE_CLOUD__API_BASE_URL', '/cloud-api')

    const store = useRuntimeConfigStore()

    expect(store.config.agent.mode).toBe('cloud')
    expect(store.view.mode).toBe('cloud')
  })

  it('initializes view mode from session storage only in hybrid mode', () => {
    const sessionStorage = storageMock()
    sessionStorage.setItem('termbridge.active_mode', 'cloud')
    stubWindowConfig(runtimeConfig({ agent: { mode: 'hybrid' } }), sessionStorage)

    const store = useRuntimeConfigStore()

    expect(store.view.mode).toBe('cloud')
  })

  it('keeps agent mode read-only and switches only the view mode in hybrid mode', () => {
    const sessionStorage = storageMock()
    stubWindowConfig(runtimeConfig({ agent: { mode: 'hybrid' } }), sessionStorage)
    const store = useRuntimeConfigStore()

    expect(store.switchMode('cloud')).toBe(true)

    expect(store.config.agent.mode).toBe('hybrid')
    expect(store.view.mode).toBe('cloud')
    expect(sessionStorage.getItem('termbridge.active_mode')).toBe('cloud')
  })

  it('rejects view mode changes outside hybrid mode', () => {
    stubWindowConfig(runtimeConfig({ agent: { mode: 'agent' } }))
    const store = useRuntimeConfigStore()

    expect(store.switchMode('cloud')).toBe(false)

    expect(store.config.agent.mode).toBe('agent')
    expect(store.view.mode).toBe('agent')
  })

  it('fails fast when required config is missing', () => {
    stubWindowConfig({ agent: { mode: 'hybrid' }, cloud: {} })

    expect(() => useRuntimeConfigStore()).toThrow(/Invalid runtime config from window\.__CONFIG__/)
  })

  it('fails fast when agent mode uses an unsupported value', () => {
    stubWindowConfig(runtimeConfig({ agent: { mode: 'both' as never } }))

    expect(() => useRuntimeConfigStore()).toThrow(/agent\.mode must be agent, cloud, or hybrid/)
  })
})
