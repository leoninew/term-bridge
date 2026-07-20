import { defineStore } from 'pinia'
import { ref } from 'vue'
import { readLocalStorageValue, writeLocalStorageValue } from './storage'

export const themes = ['light', 'dark'] as const
export type AppTheme = (typeof themes)[number]

const themeStorageKey = 'termbridge.theme'
const defaultTheme: AppTheme = 'dark'

export function isAppTheme(value: unknown): value is AppTheme {
  return typeof value === 'string' && themes.includes(value as AppTheme)
}

export function resolveInitialTheme(): AppTheme {
  const savedTheme = readLocalStorageValue(themeStorageKey)
  return isAppTheme(savedTheme) ? savedTheme : defaultTheme
}

export function applyDocumentTheme(theme: AppTheme) {
  if (typeof document === 'undefined') {
    return
  }
  document.documentElement.dataset.theme = theme
  document.documentElement.classList.toggle('dark', theme === 'dark')
}

export const useThemeStore = defineStore('theme', () => {
  const theme = ref<AppTheme>(resolveInitialTheme())

  function applyTheme() {
    applyDocumentTheme(theme.value)
  }

  function setTheme(value: AppTheme) {
    theme.value = value
    writeLocalStorageValue(themeStorageKey, value)
    applyTheme()
  }

  applyTheme()

  return {
    theme,
    setTheme,
    applyTheme,
  }
})
