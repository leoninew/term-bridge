import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  authenticatedCloudLoginRedirect,
  cloudLoginRedirectKey,
  consumeCloudLoginRedirect,
  safeCloudLoginRedirect,
  storeCloudLoginRedirect,
} from './loginRedirect'

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

describe('Cloud login redirect', () => {
  beforeEach(() => {
    vi.stubGlobal('window', { localStorage: storageMock() })
  })

  it('preserves the complete OAuth authorization path once', () => {
    const redirect =
      '/oauth2/authorize?response_type=code&client_id=termbridge-agent&redirect_uri=http%3A%2F%2Flocalhost%3A9030%2Foauth%2Fcallback&scope=openid%20email%20profile&state=state-1'

    storeCloudLoginRedirect(redirect)

    expect(window.localStorage.getItem(cloudLoginRedirectKey)).toBe(redirect)
    expect(consumeCloudLoginRedirect()).toBe(redirect)
    expect(consumeCloudLoginRedirect()).toBe('')
  })

  it('restores redirects only for a server-authenticated Cloud user', () => {
    const redirect = '/oauth2/authorize?state=state-1'

    expect(authenticatedCloudLoginRedirect(false, redirect)).toBe('')
    expect(authenticatedCloudLoginRedirect(true, redirect)).toBe(redirect)
  })

  it('rejects unsafe redirect targets', () => {
    expect(safeCloudLoginRedirect('https://attacker.example')).toBe('')
    expect(safeCloudLoginRedirect('//attacker.example')).toBe('')

    storeCloudLoginRedirect('https://attacker.example')

    expect(window.localStorage.getItem(cloudLoginRedirectKey)).toBeNull()
  })
})
