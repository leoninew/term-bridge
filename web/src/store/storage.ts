function browserStorage(): Storage | null {
  if (typeof window === 'undefined') {
    return null
  }
  return window.localStorage
}

export function readStorageValue(key: string): string | null {
  try {
    return browserStorage()?.getItem(key) ?? null
  } catch {
    return null
  }
}

export function writeStorageValue(key: string, value: string) {
  try {
    browserStorage()?.setItem(key, value)
  } catch {
    // localStorage may be unavailable in restricted browser contexts.
  }
}

export function removeStorageValue(key: string) {
  try {
    browserStorage()?.removeItem(key)
  } catch {
    // localStorage may be unavailable in restricted browser contexts.
  }
}
