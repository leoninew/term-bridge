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
    version: 'server-version',
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
    vi.stubEnv('TERMBRIDGE_LOCAL__VERSION', 'vite-version')
    stubWindowConfig(runtimeConfig({ local: { mode: 'cloud', apiBasePath: '/local-api/' } }))

    const store = useRuntimeConfigStore()

    expect(store.config.version).toBe('server-version')
    expect(store.config.local.mode).toBe('cloud')
    expect(store.config.local.apiBasePath).toBe('/local-api')
    expect(store.config.cloud.publicUrl).toBe('http://termbridge.lvh.me')
    expect(store.config.cloud.apiBaseUrl).toBe('http://termbridge.lvh.me/cloud-api')
    expect(store.view.mode).toBe('cloud')
  })

  it('builds nested config from Vite env when no runtime config is injected', () => {
    vi.stubGlobal('window', {
      localStorage: storageMock(),
      sessionStorage: storageMock(),
    })
    vi.stubEnv('TERMBRIDGE_LOCAL__VERSION', 'vite-version')
    vi.stubEnv('TERMBRIDGE_LOCAL__MODE', 'hybrid')
    vi.stubEnv('TERMBRIDGE_LOCAL__PUBLIC_URL', 'http://localhost:9030')
    vi.stubEnv('TERMBRIDGE_LOCAL__API_BASE_PATH', '/local-api')
    vi.stubEnv('TERMBRIDGE_LOCAL__OAUTH__CLIENT_ID', 'termbridge-agent')
    vi.stubEnv('TERMBRIDGE_LOCAL__OAUTH__REDIRECT_URL', 'http://localhost:9030/oauth/callback')
    vi.stubEnv('TERMBRIDGE_LOCAL__OAUTH__SCOPES', 'openid,email,profile')
    vi.stubEnv('TERMBRIDGE_CLOUD__PUBLIC_URL', 'https://cloud.example.com/')
    vi.stubEnv('TERMBRIDGE_CLOUD__API_BASE_URL', 'https://cloud.example.com/cloud-api/')
    vi.stubEnv('TERMBRIDGE_CLOUD__EXTERNAL_AUTH_PROVIDER_IDS', 'google, github,,unsupported')

    const store = useRuntimeConfigStore()

    expect(store.config).toEqual({
      version: 'vite-version',
      local: {
        mode: 'hybrid',
        publicUrl: 'http://localhost:9030',
        apiBasePath: '/local-api',
        cloudOAuth: {
          clientId: 'termbridge-agent',
          redirectUrl: 'http://localhost:9030/oauth/callback',
          scopes: ['openid', 'email', 'profile'],
        },
      },
      cloud: {
        publicUrl: 'https://cloud.example.com',
        apiBaseUrl: 'https://cloud.example.com/cloud-api',
        externalAuthProviderIds: ['google', 'github'],
      },
    })
  })

  it('ignores empty runtime config placeholder and builds from Vite env', () => {
    vi.stubGlobal('window', {
      __CONFIG__: {},
      localStorage: storageMock(),
      sessionStorage: storageMock(),
    })
    vi.stubEnv('TERMBRIDGE_LOCAL__MODE', 'cloud')
    vi.stubEnv('TERMBRIDGE_LOCAL__PUBLIC_URL', 'http://localhost:9030')
    vi.stubEnv('TERMBRIDGE_LOCAL__API_BASE_PATH', '/local-api')
    vi.stubEnv('TERMBRIDGE_LOCAL__OAUTH__CLIENT_ID', 'termbridge-agent')
    vi.stubEnv('TERMBRIDGE_LOCAL__OAUTH__REDIRECT_URL', 'http://localhost:9030/oauth/callback')
    vi.stubEnv('TERMBRIDGE_LOCAL__OAUTH__SCOPES', 'openid,email,profile')
    vi.stubEnv('TERMBRIDGE_CLOUD__PUBLIC_URL', 'http://localhost:9030')
    vi.stubEnv('TERMBRIDGE_CLOUD__API_BASE_URL', 'https://cloud.example.com/api')

    const store = useRuntimeConfigStore()

    expect(store.config.version).toBe('')
    expect(store.config.local.mode).toBe('cloud')
    expect(store.view.mode).toBe('cloud')
  })

  it('initializes view mode from session storage only in hybrid mode', () => {
    const sessionStorage = storageMock()
    sessionStorage.setItem('termbridge.active_mode', 'cloud')
    stubWindowConfig(runtimeConfig({ local: { mode: 'hybrid' } }), sessionStorage)

    const store = useRuntimeConfigStore()

    expect(store.view.mode).toBe('cloud')
  })

  it('keeps local mode read-only and switches only the view mode in hybrid mode', () => {
    const sessionStorage = storageMock()
    stubWindowConfig(runtimeConfig({ local: { mode: 'hybrid' } }), sessionStorage)
    const store = useRuntimeConfigStore()

    expect(store.switchMode('cloud')).toBe(true)

    expect(store.config.local.mode).toBe('hybrid')
    expect(store.view.mode).toBe('cloud')
    expect(sessionStorage.getItem('termbridge.active_mode')).toBe('cloud')
  })

  it('rejects view mode changes outside hybrid mode', () => {
    stubWindowConfig(runtimeConfig({ local: { mode: 'local' } }))
    const store = useRuntimeConfigStore()

    expect(store.switchMode('cloud')).toBe(false)

    expect(store.config.local.mode).toBe('local')
    expect(store.view.mode).toBe('local')
  })

  it('fails fast when required config is missing', () => {
    stubWindowConfig({ local: { mode: 'hybrid' }, cloud: {} })

    expect(() => useRuntimeConfigStore()).toThrow(/Invalid runtime config from window\.__CONFIG__/)
  })

  it('fails fast when local mode uses an unsupported value', () => {
    stubWindowConfig(runtimeConfig({ local: { mode: 'both' as never } }))

    expect(() => useRuntimeConfigStore()).toThrow(/local\.mode must be local, cloud, or hybrid/)
  })
})
