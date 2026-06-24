import { beforeEach, describe, expect, it, vi } from 'vitest'
import { readStorageValue, removeStorageValue, writeStorageValue } from './storage'

function storageMock(overrides: Partial<Storage> = {}): Storage {
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
    ...overrides,
  }
}

describe('storage helpers', () => {
  let localStorage: Storage

  beforeEach(() => {
    localStorage = storageMock()
    vi.stubGlobal('window', { localStorage })
  })

  it('reads existing values', () => {
    localStorage.setItem('termbridge.test', 'value')

    expect(readStorageValue('termbridge.test')).toBe('value')
  })

  it('returns null for missing values', () => {
    expect(readStorageValue('termbridge.missing')).toBeNull()
  })

  it('writes and removes values', () => {
    writeStorageValue('termbridge.test', 'value')
    expect(localStorage.getItem('termbridge.test')).toBe('value')

    removeStorageValue('termbridge.test')
    expect(localStorage.getItem('termbridge.test')).toBeNull()
  })

  it('does not throw when localStorage is unavailable', () => {
    vi.stubGlobal('window', {
      localStorage: storageMock({
        getItem: vi.fn(() => {
          throw new Error('blocked')
        }),
        setItem: vi.fn(() => {
          throw new Error('blocked')
        }),
        removeItem: vi.fn(() => {
          throw new Error('blocked')
        }),
      }),
    })

    expect(readStorageValue('termbridge.test')).toBeNull()
    expect(() => writeStorageValue('termbridge.test', 'value')).not.toThrow()
    expect(() => removeStorageValue('termbridge.test')).not.toThrow()
  })
})
