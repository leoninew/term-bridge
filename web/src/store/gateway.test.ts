import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAppModeStore } from './appMode'
import { useGatewayStore } from './gateway'

const mocks = vi.hoisted(() => ({
  authLoginViaAgent: vi.fn(),
  authMe: vi.fn(),
  authMeViaCloud: vi.fn(),
  listDevices: vi.fn(),
}))

vi.mock('../features/agent/api', () => ({
  authLoginViaAgent: mocks.authLoginViaAgent,
  authMe: mocks.authMe,
}))

vi.mock('../features/cloud/api', () => ({
  authLogin: vi.fn(),
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

describe('gateway store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.authLoginViaAgent.mockResolvedValue({ access_token: 'agent-token', token_type: 'bearer' })
    vi.stubGlobal('window', { localStorage: storageMock(), __CONFIG__: {} })
    setActivePinia(createPinia())
  })

  it('logs into agent locally before loading agent capabilities', async () => {
    mocks.authMe.mockResolvedValueOnce({
      authenticated: false,
      capabilities: {
        providers: [],
        password_reset_enabled: false,
        email_verification_enabled: false,
        account_auth_enabled: false,
        cloud_oauth_enabled: false,
      },
    })
    const store = useGatewayStore()

    await store.initializeAuth()

    expect(store.authenticated).toBe(false)
    expect(store.runtimeTarget).toEqual({ mode: 'agent' })
    expect(store.cloudSession).toBeNull()
    expect(store.agentToken).toBe('agent-token')
    expect(mocks.authLoginViaAgent).toHaveBeenCalledTimes(1)
    expect(mocks.authMe).toHaveBeenCalledTimes(1)
    expect(mocks.authMeViaCloud).not.toHaveBeenCalled()
  })

  it('keeps agent cloud session separate from browser authentication', async () => {
    mocks.authMe.mockResolvedValueOnce({
      authenticated: false,
      capabilities: {
        providers: [],
        password_reset_enabled: false,
        email_verification_enabled: false,
        account_auth_enabled: false,
        cloud_oauth_enabled: true,
      },
      cloud_session: {
        gate_url: 'https://cloud.example.test',
        device_id: 'dev-1',
        device_name: 'agent-device',
        connected_at: '2026-06-29T10:00:00Z',
      },
    })
    const store = useGatewayStore()

    await store.initializeAuth()

    expect(store.authenticated).toBe(false)
    expect(store.cloudSession?.gate_url).toBe('https://cloud.example.test')
    expect(store.cloudSession?.device_name).toBe('agent-device')
    expect(store.currentDevice).toEqual(store.cloudSession)

    store.clearAgentToken()

    expect(store.cloudSession).toBeNull()
  })

  it('uses cloud auth endpoint when active mode is cloud', async () => {
    vi.stubGlobal('window', { localStorage: storageMock(), __CONFIG__: { frontendMode: 'cloud' } })
    const appMode = useAppModeStore()
    appMode.setActiveMode('cloud')
    mocks.authMeViaCloud.mockResolvedValueOnce({
      authenticated: true,
      user: {
        id: 'user-1',
        email: 'user@example.test',
        display_name: 'User',
        provider: 'email',
        email_verified: true,
      },
      capabilities: {
        providers: ['email'],
        password_reset_enabled: true,
        email_verification_enabled: true,
        account_auth_enabled: true,
        cloud_oauth_enabled: false,
      },
    })
    const store = useGatewayStore()

    await store.initializeAuth()

    expect(store.authenticated).toBe(true)
    expect(store.user?.email).toBe('user@example.test')
    expect(mocks.authLoginViaAgent).not.toHaveBeenCalled()
    expect(mocks.authMeViaCloud).toHaveBeenCalledTimes(1)
    expect(mocks.authMe).not.toHaveBeenCalled()
  })

  it('stores agent and cloud tokens separately', async () => {
    const store = useGatewayStore()

    store.setAgentToken('agent-token')
    store.setCloudToken('cloud-token')

    expect(store.tokenForTarget('agent')).toBe('agent-token')
    expect(store.tokenForTarget('cloud')).toBe('cloud-token')
    expect(window.localStorage.getItem('termbridge_agent_token')).toBe('agent-token')
    expect(window.localStorage.getItem('termbridge_cloud_token')).toBe('cloud-token')
  })

  it('keeps dashboard in no-device state when cloud user has no devices', async () => {
    mocks.listDevices.mockResolvedValueOnce([])
    const store = useGatewayStore()

    await store.loadDevices()

    expect(store.devices).toEqual([])
    expect(store.selectedDeviceId).toBe('')
  })

  it('auto-selects the only online device for dashboard workbench entry', async () => {
    vi.stubGlobal('window', { localStorage: storageMock(), __CONFIG__: { frontendMode: 'cloud' } })
    const appMode = useAppModeStore()
    appMode.setActiveMode('cloud')
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
    const store = useGatewayStore()

    await store.loadDevices()

    expect(store.selectedDeviceId).toBe('dev-1')
    expect(store.runtimeTarget).toEqual({ mode: 'cloud', deviceId: 'dev-1' })
    expect(store.currentDevice).toEqual(store.devices[0])
  })
})
