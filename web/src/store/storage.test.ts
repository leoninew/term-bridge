import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  readLocalStorageValue,
  readSessionStorageValue,
  removeLocalStorageValue,
  removeSessionStorageValue,
  writeLocalStorageValue,
  writeSessionStorageValue,
} from './storage'

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

describe('storage helpers', () => {
  let localStorage: Storage
  let sessionStorage: Storage

  beforeEach(() => {
    localStorage = storageMock()
    sessionStorage = storageMock()
    vi.stubGlobal('window', { localStorage, sessionStorage })
  })

  it('reads existing localStorage values', () => {
    localStorage.setItem('termbridge.test', 'value')

    expect(readLocalStorageValue('termbridge.test')).toBe('value')
  })

  it('returns null for missing localStorage values', () => {
    expect(readLocalStorageValue('termbridge.missing')).toBeNull()
  })

  it('writes and removes localStorage values', () => {
    writeLocalStorageValue('termbridge.test', 'value')
    expect(localStorage.getItem('termbridge.test')).toBe('value')

    removeLocalStorageValue('termbridge.test')
    expect(localStorage.getItem('termbridge.test')).toBeNull()
  })

  it('reads existing sessionStorage values', () => {
    sessionStorage.setItem('termbridge.test', 'value')

    expect(readSessionStorageValue('termbridge.test')).toBe('value')
  })

  it('returns null for missing sessionStorage values', () => {
    expect(readSessionStorageValue('termbridge.missing')).toBeNull()
  })

  it('writes and removes sessionStorage values', () => {
    writeSessionStorageValue('termbridge.test', 'value')
    expect(sessionStorage.getItem('termbridge.test')).toBe('value')

    removeSessionStorageValue('termbridge.test')
    expect(sessionStorage.getItem('termbridge.test')).toBeNull()
  })
})
