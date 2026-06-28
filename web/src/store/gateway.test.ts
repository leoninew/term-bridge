import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useGatewayStore } from './gateway'

const mocks = vi.hoisted(() => ({
  authSetupStatus: vi.fn(),
}))

vi.mock('../features/gateway/api', () => ({
  authLogin: vi.fn(),
  authMe: vi.fn(),
  authSetupStatus: mocks.authSetupStatus,
  listDevices: vi.fn(),
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

  it('checks and caches available local setup status without a token', async () => {
    mocks.authSetupStatus.mockResolvedValueOnce({ available: true })
    const store = useGatewayStore()

    await expect(store.checkSetupStatus()).resolves.toBe(true)
    await expect(store.checkSetupStatus()).resolves.toBe(true)

    expect(store.setupAvailable).toBe(true)
    expect(store.authenticated).toBe(false)
    expect(mocks.authSetupStatus).toHaveBeenCalledTimes(1)
  })

  it('records unavailable local setup status', async () => {
    mocks.authSetupStatus.mockResolvedValueOnce({ available: false })
    const store = useGatewayStore()

    await expect(store.checkSetupStatus()).resolves.toBe(false)

    expect(store.setupAvailable).toBe(false)
  })

  it('falls back to unavailable when setup status cannot be read', async () => {
    mocks.authSetupStatus.mockRejectedValueOnce(new Error('offline'))
    const store = useGatewayStore()

    await expect(store.checkSetupStatus()).resolves.toBe(false)

    expect(store.setupAvailable).toBe(false)
  })

  it('can force refresh cached setup status', async () => {
    mocks.authSetupStatus
      .mockResolvedValueOnce({ available: true })
      .mockResolvedValueOnce({ available: false })
    const store = useGatewayStore()

    await expect(store.checkSetupStatus()).resolves.toBe(true)
    await expect(store.checkSetupStatus({ force: true })).resolves.toBe(false)

    expect(store.setupAvailable).toBe(false)
    expect(mocks.authSetupStatus).toHaveBeenCalledTimes(2)
  })
})
