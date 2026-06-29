import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../router', () => ({
  router: {
    currentRoute: { value: { name: 'sessions', fullPath: '/sessions' } },
    push: vi.fn(),
  },
}))

import { cloudConnectAuthorize, cloudConnectStartURL } from './api'
import { apiClient } from '../api/client'

describe('gateway api', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('uses the product cloud connect start route', () => {
    expect(cloudConnectStartURL()).toBe('/cloud/connect/start')
  })

  it('uses redirect_uri when authorizing cloud connect', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValueOnce({
      data: { redirect_url: 'http://127.0.0.1/callback?code=code-1&state=state-1' },
    })

    await expect(cloudConnectAuthorize('http://127.0.0.1/callback', 'state-1')).resolves.toBe(
      'http://127.0.0.1/callback?code=code-1&state=state-1',
    )

    expect(get).toHaveBeenCalledWith('/api/cloud-connect/authorize', {
      params: { redirect_uri: 'http://127.0.0.1/callback', state: 'state-1' },
    })
  })

  it('lists devices from the gateway', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValueOnce({ data: [] })

    const { listDevices } = await import('./api')
    await expect(listDevices()).resolves.toEqual([])

    expect(get).toHaveBeenCalledWith('/api/devices')
  })
})
