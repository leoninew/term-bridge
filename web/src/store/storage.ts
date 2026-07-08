function browserLocalStorage(): Storage | null {
  if (typeof window === 'undefined') {
    return null
  }
  return window.localStorage
}

function browserSessionStorage(): Storage | null {
  if (typeof window === 'undefined') {
    return null
  }
  return window.sessionStorage
}

export function readLocalStorageValue(key: string): string | null {
  return browserLocalStorage()?.getItem(key) ?? null
}

export function writeLocalStorageValue(key: string, value: string) {
  browserLocalStorage()?.setItem(key, value)
}

export function removeLocalStorageValue(key: string) {
  browserLocalStorage()?.removeItem(key)
}

export function readSessionStorageValue(key: string): string | null {
  return browserSessionStorage()?.getItem(key) ?? null
}

export function writeSessionStorageValue(key: string, value: string) {
  browserSessionStorage()?.setItem(key, value)
}

export function removeSessionStorageValue(key: string) {
  browserSessionStorage()?.removeItem(key)
}
