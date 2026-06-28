import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../router', () => ({
  router: {
    currentRoute: { value: { name: 'sessions', fullPath: '/sessions' } },
    push: vi.fn(),
  },
}))

import { authSetupStatus } from './api'
import { apiClient } from '../api/client'

describe('gateway api', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('reads local setup status from the gateway', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValueOnce({ data: { available: true } })

    await expect(authSetupStatus()).resolves.toEqual({ available: true })

    expect(get).toHaveBeenCalledWith('/api/auth/setup/status')
  })

  it('returns unavailable setup status', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValueOnce({ data: { available: false } })

    await expect(authSetupStatus()).resolves.toEqual({ available: false })
  })
})
