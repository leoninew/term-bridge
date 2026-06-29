import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useGatewayStore } from './gateway'

const mocks = vi.hoisted(() => ({
  authMe: vi.fn(),
  listDevices: vi.fn(),
}))

vi.mock('../features/gateway/api', () => ({
  authLogin: vi.fn(),
  authMe: mocks.authMe,
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
    vi.stubGlobal('window', { localStorage: storageMock() })
    setActivePinia(createPinia())
  })

  it('loads local mode capabilities without requiring a token', async () => {
    mocks.authMe.mockResolvedValueOnce({
      authenticated: false,
      capabilities: {
        mode: 'local',
        providers: [],
        password_reset_enabled: false,
        email_verification_enabled: false,
        account_auth_enabled: false,
        cloud_connect_enabled: false,
      },
    })
    const store = useGatewayStore()

    await store.initializeAuth()

    expect(store.authenticated).toBe(false)
    expect(store.capabilities?.mode).toBe('local')
    expect(mocks.authMe).toHaveBeenCalledTimes(1)
  })

  it('keeps dashboard in no-device state when cloud user has no devices', async () => {
    mocks.listDevices.mockResolvedValueOnce([])
    const store = useGatewayStore()

    await store.loadDevices()

    expect(store.devices).toEqual([])
    expect(store.selectedDeviceId).toBe('')
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
    const store = useGatewayStore()

    await store.loadDevices()

    expect(store.selectedDeviceId).toBe('dev-1')
  })
})
