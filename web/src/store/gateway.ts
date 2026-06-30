import { defineStore } from 'pinia'
import { ref } from 'vue'
import { readStorageValue, removeStorageValue, writeStorageValue } from './storage'
import {
  authLogin,
  authMe,
  listDevices,
  type AuthCapabilities,
  type CloudSessionSummary,
  type DeviceSummary,
  type TokenResp,
  type UserInfo,
} from '../features/gateway/api'

const TOKEN_KEY = 'termbridge_gateway_token'

type InitializeAuthOptions = {
  force?: boolean
}

export const useGatewayStore = defineStore('gateway', () => {
  const token = ref<string | null>(readStorageValue(TOKEN_KEY))
  const authInitialized = ref(false)
  const authenticated = ref(false)
  const loggingIn = ref(false)
  const usernameInput = ref('')
  const passwordInput = ref('')
  const user = ref<UserInfo | null>(null)
  const capabilities = ref<AuthCapabilities | null>(null)
  const cloudSession = ref<CloudSessionSummary | null>(null)
  const devices = ref<DeviceSummary[]>([])
  const selectedDeviceId = ref('')
  let initializedToken: string | null | undefined

  function setToken(newToken: string) {
    token.value = newToken
    writeStorageValue(TOKEN_KEY, newToken)
    resetAuthState()
    resetDeviceState()
  }

  function clearToken() {
    token.value = null
    removeStorageValue(TOKEN_KEY)
    resetAuthState()
    resetDeviceState()
  }

  async function initializeAuth(options: InitializeAuthOptions = {}) {
    if (!options.force && authInitialized.value && initializedToken === token.value) {
      return
    }
    try {
      const me = await authMe()
      authenticated.value = me.authenticated
      user.value = me.user ?? null
      capabilities.value = me.capabilities ?? null
      cloudSession.value = me.cloud_session ?? null
      usernameInput.value = me.user?.email || usernameInput.value
      initializedToken = token.value
    } catch (err) {
      authenticated.value = false
      user.value = null
      capabilities.value = null
      cloudSession.value = null
      initializedToken = undefined
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
      setToken(response.access_token)
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
    capabilities.value = null
    cloudSession.value = null
    initializedToken = undefined
  }

  function resetDeviceState() {
    devices.value = []
    selectedDeviceId.value = ''
  }

  return {
    token,
    authInitialized,
    authenticated,
    loggingIn,
    usernameInput,
    passwordInput,
    user,
    capabilities,
    cloudSession,
    devices,
    selectedDeviceId,
    setToken,
    clearToken,
    initializeAuth,
    login,
    loadDevices,
    selectDeviceId,
    setCloudSession,
  }
})
