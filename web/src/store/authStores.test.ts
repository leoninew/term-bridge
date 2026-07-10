import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthTokensStore } from './authTokens'
import { useCloudAuthStore } from './cloudAuth'
import { useCloudDevicesStore } from './cloudDevices'
import { useLocalAuthStore } from './localAuth'

const mocks = vi.hoisted(() => ({
  authLogin: vi.fn(),
  authLoginViaLocal: vi.fn(),
  authMe: vi.fn(),
  authMeViaCloud: vi.fn(),
  listDevices: vi.fn(),
}))

vi.mock('../features/local/api', () => ({
  authLoginViaLocal: mocks.authLoginViaLocal,
  authMe: mocks.authMe,
}))

vi.mock('../features/cloud/api', () => ({
  authLogin: mocks.authLogin,
  authMeViaCloud: mocks.authMeViaCloud,
  listDevices: mocks.listDevices,
}))

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

describe('auth token store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('window', {
      localStorage: storageMock(),
      sessionStorage: storageMock(),
    })
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('stores local and cloud tokens separately', () => {
    const tokens = useAuthTokensStore()

    tokens.setLocalToken('local-token')
    tokens.setCloudToken('cloud-token')

    expect(tokens.tokenForTarget('local')).toBe('local-token')
    expect(tokens.tokenForTarget('cloud')).toBe('cloud-token')
    expect(window.localStorage.getItem('termbridge_local_token')).toBe('local-token')
    expect(window.localStorage.getItem('termbridge_cloud_token')).toBe('cloud-token')
  })
})

describe('local auth store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.authLoginViaLocal.mockResolvedValue({ access_token: 'local-token', token_type: 'bearer' })
    vi.stubGlobal('window', {
      localStorage: storageMock(),
      sessionStorage: storageMock(),
    })
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('logs into local API before loading local auth state', async () => {
    mocks.authMe.mockResolvedValueOnce({
      authenticated: false,
    })
    const localAuth = useLocalAuthStore()
    const tokens = useAuthTokensStore()

    await localAuth.initializeAuth()

    expect(localAuth.authenticated).toBe(false)
    expect(localAuth.cloudSession).toBeNull()
    expect(tokens.localToken).toBe('local-token')
    expect(mocks.authLoginViaLocal).toHaveBeenCalledTimes(1)
    expect(mocks.authMe).toHaveBeenCalledTimes(1)
    expect(mocks.authMeViaCloud).not.toHaveBeenCalled()
  })

  it('reuses an existing local token without issuing another local login', async () => {
    const tokens = useAuthTokensStore()
    tokens.setLocalToken('existing-local-token')
    const localAuth = useLocalAuthStore()

    await localAuth.ensureToken()

    expect(tokens.localToken).toBe('existing-local-token')
    expect(mocks.authLoginViaLocal).not.toHaveBeenCalled()
  })

  it('keeps local cloud session separate from browser authentication', async () => {
    mocks.authMe.mockResolvedValueOnce({
      authenticated: false,
      cloud_session: {
        public_url: 'https://cloud.example.test',
        device_id: 'dev-1',
        device_name: 'local-device',
        connected_at: '2026-06-29T10:00:00Z',
      },
    })
    const localAuth = useLocalAuthStore()

    await localAuth.initializeAuth()

    expect(localAuth.authenticated).toBe(false)
    expect(localAuth.cloudSession?.public_url).toBe('https://cloud.example.test')
    expect(localAuth.cloudSession?.device_name).toBe('local-device')

    localAuth.clearToken()

    expect(localAuth.cloudSession).toBeNull()
  })
})

describe('cloud auth store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('window', {
      localStorage: storageMock(),
      sessionStorage: storageMock(),
    })
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('clears an unverified stored cloud token', async () => {
    window.localStorage.setItem('termbridge_cloud_token', 'stale-cloud-token')
    mocks.authMeViaCloud.mockResolvedValueOnce({ authenticated: false })
    const tokens = useAuthTokensStore()
    const cloudAuth = useCloudAuthStore()

    await cloudAuth.initializeAuth()

    expect(cloudAuth.authInitialized).toBe(true)
    expect(cloudAuth.authenticated).toBe(false)
    expect(tokens.cloudToken).toBeNull()
    expect(window.localStorage.getItem('termbridge_cloud_token')).toBeNull()
    expect(mocks.authMeViaCloud).toHaveBeenCalledTimes(1)
  })

  it('keeps a server-verified cloud token and loads browser account state', async () => {
    window.localStorage.setItem('termbridge_cloud_token', 'verified-cloud-token')
    mocks.authMeViaCloud.mockResolvedValueOnce({
      authenticated: true,
      user: {
        id: 'user-1',
        email: 'user@example.test',
        display_name: 'User',
        provider: 'email',
        email_verified: true,
      },
    })
    const tokens = useAuthTokensStore()
    const cloudAuth = useCloudAuthStore()

    await cloudAuth.initializeAuth()

    expect(cloudAuth.authenticated).toBe(true)
    expect(cloudAuth.user?.email).toBe('user@example.test')
    expect(tokens.cloudToken).toBe('verified-cloud-token')
    expect(window.localStorage.getItem('termbridge_cloud_token')).toBe('verified-cloud-token')
    expect(mocks.authLoginViaLocal).not.toHaveBeenCalled()
    expect(mocks.authMeViaCloud).toHaveBeenCalledTimes(1)
    expect(mocks.authMe).not.toHaveBeenCalled()
  })
})

describe('cloud devices store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('window', {
      localStorage: storageMock(),
      sessionStorage: storageMock(),
    })
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('keeps dashboard in no-device state when cloud user has no devices', async () => {
    mocks.listDevices.mockResolvedValueOnce([])
    const cloudDevices = useCloudDevicesStore()

    await cloudDevices.loadDevices()

    expect(cloudDevices.devices).toEqual([])
    expect(cloudDevices.selectedDeviceId).toBe('')
  })

  it('auto-selects the only online device for dashboard workbench entry', async () => {
    mocks.listDevices.mockResolvedValueOnce([
      {
        id: 'dev-1',
        name: 'Laptop',
        online: true,
        status: 'online',
        connected_at: '2026-06-28T10:00:00Z',
        last_seen: '2026-06-28T10:00:00Z',
      },
    ])
    const cloudDevices = useCloudDevicesStore()

    await cloudDevices.loadDevices()

    expect(cloudDevices.selectedDeviceId).toBe('dev-1')
  })
})
