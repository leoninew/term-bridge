import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { applyDocumentTheme, resolveInitialTheme, useThemeStore } from './theme'

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

function documentMock() {
  const classes = new Set<string>()
  return {
    documentElement: {
      dataset: {} as Record<string, string>,
      classList: {
        contains: vi.fn((name: string) => classes.has(name)),
        toggle: vi.fn((name: string, force?: boolean) => {
          if (force) {
            classes.add(name)
            return true
          }
          classes.delete(name)
          return false
        }),
      },
    },
  }
}

describe('theme store', () => {
  let localStorage: Storage
  let mockedDocument: ReturnType<typeof documentMock>

  beforeEach(() => {
    localStorage = storageMock()
    mockedDocument = documentMock()
    vi.stubGlobal('window', { localStorage })
    vi.stubGlobal('document', mockedDocument)
    setActivePinia(createPinia())
  })

  it('defaults to dark without cached theme', () => {
    expect(resolveInitialTheme()).toBe('dark')

    const store = useThemeStore()

    expect(store.theme).toBe('dark')
    expect(mockedDocument.documentElement.dataset.theme).toBe('dark')
    expect(mockedDocument.documentElement.classList.contains('dark')).toBe(true)
  })

  it('restores a valid cached theme', () => {
    localStorage.setItem('termbridge.theme', 'light')

    const store = useThemeStore()

    expect(store.theme).toBe('light')
    expect(mockedDocument.documentElement.dataset.theme).toBe('light')
    expect(mockedDocument.documentElement.classList.contains('dark')).toBe(false)
  })

  it('falls back to dark for an invalid cached theme', () => {
    localStorage.setItem('termbridge.theme', 'system')

    const store = useThemeStore()

    expect(store.theme).toBe('dark')
    expect(mockedDocument.documentElement.dataset.theme).toBe('dark')
    expect(mockedDocument.documentElement.classList.contains('dark')).toBe(true)
  })

  it('persists theme changes and updates the document marker', () => {
    const store = useThemeStore()

    store.setTheme('light')

    expect(store.theme).toBe('light')
    expect(localStorage.getItem('termbridge.theme')).toBe('light')
    expect(mockedDocument.documentElement.dataset.theme).toBe('light')
    expect(mockedDocument.documentElement.classList.contains('dark')).toBe(false)

    store.setTheme('dark')

    expect(store.theme).toBe('dark')
    expect(localStorage.getItem('termbridge.theme')).toBe('dark')
    expect(mockedDocument.documentElement.dataset.theme).toBe('dark')
    expect(mockedDocument.documentElement.classList.contains('dark')).toBe(true)
  })

  it('can apply a theme without creating a store', () => {
    applyDocumentTheme('light')

    expect(mockedDocument.documentElement.dataset.theme).toBe('light')
    expect(mockedDocument.documentElement.classList.contains('dark')).toBe(false)
  })
})
