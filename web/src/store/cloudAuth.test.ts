import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useCloudAuthStore } from './cloudAuth'
import { useCloudDevicesStore } from './cloudDevices'
import { useCloudSessionStore } from './cloudSession'
import type { BrowserRuntimeConfig } from '../config'

const mocks = vi.hoisted(() => ({
  fetchCloudIdentityViaLocalApi: vi.fn(),
  fetchCloudIdentityViaCloudApi: vi.fn(),
  listDevices: vi.fn(),
  deleteDevice: vi.fn(),
}))

vi.mock('../features/local/api', () => ({
  fetchCloudIdentityViaLocalApi: mocks.fetchCloudIdentityViaLocalApi,
}))

vi.mock('../features/cloud/api', () => ({
  fetchCloudIdentityViaCloudApi: mocks.fetchCloudIdentityViaCloudApi,
  listDevices: mocks.listDevices,
  deleteDevice: mocks.deleteDevice,
}))

function runtimeConfig(mode: 'local' | 'cloud' | 'hybrid' = 'cloud'): BrowserRuntimeConfig {
  return {
    local: {
      mode,
      publicUrl: 'http://localhost:9030',
      apiBasePath: '/api',
      cloudOAuth: {
        clientId: 'termbridge-agent',
        redirectUrl: 'http://localhost:9030/oauth/callback',
        scopes: ['openid'],
      },
    },
    cloud: {
      publicUrl: 'https://cloud.example.test',
      apiBaseUrl: 'https://cloud.example.test/api',
      externalAuthProviderIds: [],
    },
  }
}

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

function stubBrowser(mode: 'local' | 'cloud' | 'hybrid' = 'cloud') {
  vi.stubGlobal('window', {
    __CONFIG__: runtimeConfig(mode),
    localStorage: storageMock(),
    sessionStorage: storageMock(),
  })
}

describe('cloud auth store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    stubBrowser()
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('stores the Cloud credential for Cloud API requests', () => {
    const cloudAuth = useCloudAuthStore()

    cloudAuth.setToken('cloud-token')

    expect(cloudAuth.cloudToken).toBe('cloud-token')
    expect(window.localStorage.getItem('termbridge_cloud_token')).toBe('cloud-token')
  })

  it('clears an unverified stored Cloud token', async () => {
    window.localStorage.setItem('termbridge_cloud_token', 'stale-cloud-token')
    mocks.fetchCloudIdentityViaCloudApi.mockResolvedValueOnce({ authenticated: false })
    const cloudAuth = useCloudAuthStore()

    await cloudAuth.initialize()

    expect(cloudAuth.authenticated).toBe(false)
    expect(cloudAuth.cloudToken).toBeNull()
    expect(window.localStorage.getItem('termbridge_cloud_token')).toBeNull()
    expect(mocks.fetchCloudIdentityViaCloudApi).toHaveBeenCalledTimes(1)
  })

  it('loads Cloud identity through the Local API relay in Local view', async () => {
    stubBrowser('local')
    window.localStorage.setItem('termbridge_cloud_token', 'verified-cloud-token')
    mocks.fetchCloudIdentityViaLocalApi.mockResolvedValueOnce({
      authenticated: true,
      user: {
        id: 'user-1',
        email: 'user@example.test',
        display_name: 'User',
        provider: 'email',
        email_verified: true,
      },
    })
    const cloudAuth = useCloudAuthStore()

    await cloudAuth.initialize()

    expect(cloudAuth.authenticated).toBe(true)
    expect(cloudAuth.user?.email).toBe('user@example.test')
    expect(cloudAuth.cloudToken).toBe('verified-cloud-token')
    expect(mocks.fetchCloudIdentityViaLocalApi).toHaveBeenCalledWith('verified-cloud-token')
    expect(mocks.fetchCloudIdentityViaCloudApi).not.toHaveBeenCalled()
  })

  it('does not call an identity transport without a Cloud credential in Local view', async () => {
    stubBrowser('local')
    const cloudAuth = useCloudAuthStore()

    await cloudAuth.initialize()

    expect(cloudAuth.authenticated).toBe(false)
    expect(mocks.fetchCloudIdentityViaLocalApi).not.toHaveBeenCalled()
    expect(mocks.fetchCloudIdentityViaCloudApi).not.toHaveBeenCalled()
  })

  it('soft-fails cloud identity load in Local view when cloud is unavailable', async () => {
    stubBrowser('local')
    window.localStorage.setItem('termbridge_cloud_token', 'cloud-token')
    mocks.fetchCloudIdentityViaLocalApi.mockRejectedValueOnce(new Error('cloud unavailable'))
    const cloudAuth = useCloudAuthStore()

    await expect(cloudAuth.initialize()).resolves.toBeUndefined()

    expect(cloudAuth.authenticated).toBe(false)
    expect(cloudAuth.user).toBeNull()
    expect(cloudAuth.cloudToken).toBe('cloud-token')
    expect(mocks.fetchCloudIdentityViaLocalApi).toHaveBeenCalledWith('cloud-token')
  })

  it('keeps a verified Cloud token and loads Cloud account state', async () => {
    window.localStorage.setItem('termbridge_cloud_token', 'verified-cloud-token')
    mocks.fetchCloudIdentityViaCloudApi.mockResolvedValueOnce({
      authenticated: true,
      user: {
        id: 'user-1',
        email: 'user@example.test',
        display_name: 'User',
        provider: 'email',
        email_verified: true,
      },
    })
    const cloudAuth = useCloudAuthStore()

    await cloudAuth.initialize()

    expect(cloudAuth.authenticated).toBe(true)
    expect(cloudAuth.user?.email).toBe('user@example.test')
    expect(cloudAuth.cloudToken).toBe('verified-cloud-token')
    expect(window.localStorage.getItem('termbridge_cloud_token')).toBe('verified-cloud-token')
    expect(mocks.fetchCloudIdentityViaCloudApi).toHaveBeenCalledTimes(1)
  })
})

describe('local agent cloud session store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    stubBrowser('local')
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('keeps the Agent Cloud-session summary as transient UI state', () => {
    const cloudSession = useCloudSessionStore()
    const summary = {
      public_url: 'https://cloud.example.test',
      device_id: 'dev-1',
      device_name: 'local-device',
      connected_at: '2026-06-29T10:00:00Z',
    }

    cloudSession.setCloudSession(summary)

    expect(cloudSession.cloudSession?.device_name).toBe('local-device')
    cloudSession.reset()
    expect(cloudSession.cloudSession).toBeNull()
  })
})

describe('cloud devices store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    stubBrowser()
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('keeps dashboard in no-device state when Cloud user has no devices', async () => {
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

  it('removes only offline devices from the dashboard list', async () => {
    mocks.listDevices.mockResolvedValueOnce([
      {
        id: 'dev-online',
        name: 'Online laptop',
        online: true,
        status: 'online',
        connected_at: '2026-06-28T10:00:00Z',
        last_seen: '2026-06-28T10:00:00Z',
      },
      {
        id: 'dev-offline',
        name: 'Offline laptop',
        online: false,
        status: 'offline',
        connected_at: '',
        last_seen: '2026-06-27T10:00:00Z',
      },
    ])
    mocks.deleteDevice.mockResolvedValueOnce(undefined)
    const cloudDevices = useCloudDevicesStore()

    await cloudDevices.loadDevices()
    cloudDevices.selectedDeviceId = 'dev-offline'

    await expect(cloudDevices.removeDevice('dev-online')).resolves.toBe(false)
    expect(mocks.deleteDevice).not.toHaveBeenCalled()

    await expect(cloudDevices.removeDevice('dev-offline')).resolves.toBe(true)
    expect(mocks.deleteDevice).toHaveBeenCalledWith('dev-offline')
    expect(cloudDevices.devices.map((device) => device.id)).toEqual(['dev-online'])
    expect(cloudDevices.selectedDeviceId).toBe('dev-online')
  })
})
