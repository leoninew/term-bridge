import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../router', () => ({
  router: {
    currentRoute: { value: { name: 'sessions', fullPath: '/sessions' } },
    push: vi.fn(),
  },
}))

import { cloudOAuthAuthorize, cloudOAuthStart, cloudOAuthStartURL } from './api'
import { apiClient } from '../api/client'

describe('gateway api', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('uses the product cloud oauth start route', () => {
    expect(cloudOAuthStartURL()).toBe('/cloud/oauth/start')
  })

  it('requests the backend OAuth start URL', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValueOnce({
      data: { authorize_url: 'http://termbridge.lvh.me/oauth2/authorize?state=state-1' },
    })

    await expect(cloudOAuthStart('/dashboard')).resolves.toBe(
      'http://termbridge.lvh.me/oauth2/authorize?state=state-1',
    )

    expect(get).toHaveBeenCalledWith('/api/cloud-oauth/start', {
      params: { redirect: '/dashboard' },
    })
  })

  it('uses client_id and redirect_uri when authorizing cloud oauth', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValueOnce({
      data: { redirect_url: 'http://localhost:9031/callback?code=code-1&state=state-1' },
    })

    await expect(
      cloudOAuthAuthorize('termbridge-local', 'http://localhost:9031/callback', 'state-1'),
    ).resolves.toBe('http://localhost:9031/callback?code=code-1&state=state-1')

    expect(get).toHaveBeenCalledWith('/api/cloud-oauth/authorize', {
      params: {
        client_id: 'termbridge-local',
        redirect_uri: 'http://localhost:9031/callback',
        state: 'state-1',
      },
    })
  })

  it('lists devices from the gateway', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValueOnce({ data: [] })

    const { listDevices } = await import('./api')
    await expect(listDevices()).resolves.toEqual([])

    expect(get).toHaveBeenCalledWith('/api/devices')
  })
})
