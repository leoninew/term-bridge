import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAppModeStore } from './appMode'

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

describe('app mode store', () => {
  let localStorage: Storage
  let sessionStorage: Storage

  beforeEach(() => {
    localStorage = storageMock()
    sessionStorage = storageMock()
    vi.stubGlobal('window', {
      __CONFIG__: { frontendMode: 'hybrid' },
      localStorage,
      sessionStorage,
    })
    setActivePinia(createPinia())
  })

  it('restores active mode from sessionStorage', () => {
    localStorage.setItem('termbridge.active_mode', 'agent')
    sessionStorage.setItem('termbridge.active_mode', 'cloud')

    const store = useAppModeStore()

    expect(store.activeMode).toBe('cloud')
  })

  it('persists active mode to sessionStorage', () => {
    const store = useAppModeStore()

    expect(store.setActiveMode('cloud')).toBe(true)

    expect(sessionStorage.getItem('termbridge.active_mode')).toBe('cloud')
    expect(localStorage.getItem('termbridge.active_mode')).toBeNull()
  })
})
