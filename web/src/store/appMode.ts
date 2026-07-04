import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { getFrontendMode, type FrontendMode } from '../config'
import { readStorageValue, writeStorageValue } from './storage'

export type ActiveMode = 'agent' | 'cloud'
export type RouteMode = ActiveMode | 'both'

const ACTIVE_MODE_KEY = 'termbridge.active_mode'

export const useAppModeStore = defineStore('appMode', () => {
  const frontendMode = ref<FrontendMode>(getFrontendMode())
  const activeMode = ref<ActiveMode>(initialActiveMode(frontendMode.value))

  const isHybrid = computed(() => frontendMode.value === 'hybrid')
  const effectiveMode = computed<ActiveMode>(() =>
    frontendMode.value === 'hybrid' ? activeMode.value : frontendMode.value,
  )

  function allowsMode(mode: ActiveMode) {
    return frontendMode.value === 'hybrid' || frontendMode.value === mode
  }

  function allowsRouteMode(mode: RouteMode) {
    return mode === 'both' || allowsMode(mode)
  }

  function setActiveMode(mode: ActiveMode) {
    if (!allowsMode(mode)) {
      return false
    }
    activeMode.value = mode
    writeStorageValue(ACTIVE_MODE_KEY, mode)
    return true
  }

  function activateRouteMode(mode: RouteMode) {
    if (mode === 'agent' || mode === 'cloud') {
      setActiveMode(mode)
    }
  }

  function dashboardRouteName(mode: ActiveMode = effectiveMode.value) {
    return mode === 'agent' ? 'agent-dashboard' : 'cloud-dashboard'
  }

  return {
    frontendMode,
    activeMode,
    isHybrid,
    effectiveMode,
    allowsMode,
    allowsRouteMode,
    setActiveMode,
    activateRouteMode,
    dashboardRouteName,
  }
})

function initialActiveMode(frontendMode: FrontendMode): ActiveMode {
  if (frontendMode === 'agent' || frontendMode === 'cloud') {
    return frontendMode
  }
  const saved = readStorageValue(ACTIVE_MODE_KEY)
  return saved === 'cloud' ? 'cloud' : 'agent'
}
