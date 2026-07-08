import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { readLocalStorageValue, removeLocalStorageValue, writeLocalStorageValue } from './storage'
import { authLoginViaAgent, authMe } from '../features/agent/api'
import { authLogin, authMeViaCloud, listDevices } from '../features/cloud/api'
import { useAppModeStore } from './appMode'
import type { CloudSessionSummary } from '../gen/proto/termbridge/cloud/v1/session'
import type { DeviceSummary } from '../gen/proto/termbridge/cloud/v1/device'
import type { TokenResp, User as UserInfo } from '../gen/proto/termbridge/cloud/v1/auth'
import type { RuntimeTarget } from '../features/runtimeTarget'
import type { ApiTarget } from '../config'

const AGENT_TOKEN_KEY = 'termbridge_agent_token'
const CLOUD_TOKEN_KEY = 'termbridge_cloud_token'

type InitializeAuthOptions = {
  force?: boolean
}

export const useGatewayStore = defineStore('gateway', () => {
  const agentToken = ref<string | null>(readLocalStorageValue(AGENT_TOKEN_KEY))
  const cloudToken = ref<string | null>(readLocalStorageValue(CLOUD_TOKEN_KEY))
  const authInitialized = ref(false)
  const authenticated = ref(false)
  const loggingIn = ref(false)
  const usernameInput = ref('')
  const passwordInput = ref('')
  const user = ref<UserInfo | null>(null)
  const cloudSession = ref<CloudSessionSummary | null>(null)
  const devices = ref<DeviceSummary[]>([])
  const selectedDeviceId = ref('')
  const token = computed(() => {
    const appMode = useAppModeStore()
    return tokenForTarget(appMode.effectiveMode)
  })
  const runtimeTarget = computed<RuntimeTarget | null>(() => {
    const appMode = useAppModeStore()
    if (appMode.effectiveMode === 'agent') {
      return { mode: 'agent' }
    }
    return selectedDeviceId.value ? { mode: 'cloud', deviceId: selectedDeviceId.value } : null
  })
  const currentDevice = computed<DeviceSummary | CloudSessionSummary | null>(() => {
    const appMode = useAppModeStore()
    if (appMode.effectiveMode === 'agent') {
      return devices.value[0] ?? cloudSession.value
    }
    return devices.value.find((device) => device.id === selectedDeviceId.value) ?? null
  })
  let initializedToken: string | null | undefined
  let initializedMode: 'agent' | 'cloud' | undefined

  function setAgentToken(newToken: string) {
    agentToken.value = newToken
    writeLocalStorageValue(AGENT_TOKEN_KEY, newToken)
    resetAuthState()
  }

  function setCloudToken(newToken: string) {
    cloudToken.value = newToken
    writeLocalStorageValue(CLOUD_TOKEN_KEY, newToken)
    resetAuthState()
    resetDeviceState()
  }

  function clearAgentToken() {
    agentToken.value = null
    removeLocalStorageValue(AGENT_TOKEN_KEY)
    resetAuthState()
  }

  function clearCloudToken() {
    cloudToken.value = null
    removeLocalStorageValue(CLOUD_TOKEN_KEY)
    resetAuthState()
    resetDeviceState()
  }

  function clearTokenForTarget(target: ApiTarget) {
    if (target === 'agent') {
      clearAgentToken()
      return
    }
    clearCloudToken()
  }

  function tokenForTarget(target: ApiTarget): string | null {
    return target === 'agent' ? agentToken.value : cloudToken.value
  }

  async function ensureAgentToken() {
    if (agentToken.value) {
      return
    }
    const response: TokenResp = await authLoginViaAgent()
    setAgentToken(response.access_token)
  }

  async function initializeAuth(options: InitializeAuthOptions = {}) {
    const appMode = useAppModeStore()
    const mode = appMode.effectiveMode
    if (mode === 'agent') {
      await ensureAgentToken()
    }
    const currentToken = tokenForTarget(mode)
    if (
      !options.force &&
      authInitialized.value &&
      initializedToken === currentToken &&
      initializedMode === mode
    ) {
      return
    }
    try {
      const me = mode === 'cloud' ? await authMeViaCloud() : await authMe()
      authenticated.value = me.authenticated
      user.value = me.user ?? null
      cloudSession.value = me.cloud_session ?? null
      usernameInput.value = me.user?.email || usernameInput.value
      initializedToken = currentToken
      initializedMode = mode
    } catch (err) {
      authenticated.value = false
      user.value = null
      cloudSession.value = null
      initializedToken = undefined
      initializedMode = undefined
      throw err
    } finally {
      authInitialized.value = true
    }
  }

  async function login() {
    if (loggingIn.value) {
      return
    }
    loggingIn.value = true
    try {
      const response: TokenResp = await authLogin(usernameInput.value, passwordInput.value)
      setCloudToken(response.access_token)
      passwordInput.value = ''
    } finally {
      loggingIn.value = false
    }
  }

  async function loadDevices() {
    devices.value = await listDevices()
    if (devices.value.some((device) => device.id === selectedDeviceId.value && device.online)) {
      return
    }
    const onlineDevices = devices.value.filter((device) => device.online)
    selectedDeviceId.value = onlineDevices.length === 1 ? onlineDevices[0].id : ''
  }

  function selectDeviceId(deviceId: string) {
    const device = devices.value.find((candidate) => candidate.id === deviceId)
    if (!device?.online || deviceId === selectedDeviceId.value) {
      return false
    }
    selectedDeviceId.value = deviceId
    return true
  }

  function setCloudSession(summary: CloudSessionSummary | null) {
    cloudSession.value = summary
  }

  function resetAuthState() {
    authInitialized.value = false
    authenticated.value = false
    user.value = null
    cloudSession.value = null
    initializedToken = undefined
    initializedMode = undefined
  }

  function resetDeviceState() {
    devices.value = []
    selectedDeviceId.value = ''
  }

  return {
    token,
    agentToken,
    cloudToken,
    authInitialized,
    authenticated,
    loggingIn,
    usernameInput,
    passwordInput,
    user,
    cloudSession,
    devices,
    selectedDeviceId,
    runtimeTarget,
    currentDevice,
    setAgentToken,
    setCloudToken,
    clearAgentToken,
    clearCloudToken,
    clearTokenForTarget,
    tokenForTarget,
    ensureAgentToken,
    initializeAuth,
    login,
    loadDevices,
    selectDeviceId,
    setCloudSession,
  }
})
