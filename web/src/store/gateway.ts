import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  authLogin,
  authMe,
  listDevices,
  type AuthCapabilities,
  type DeviceSummary,
  type TokenResp,
  type UserInfo,
} from '../features/gateway/api'

const TOKEN_KEY = 'termbridge_gateway_token'

type InitializeAuthOptions = {
  force?: boolean
}

export const useGatewayStore = defineStore('gateway', () => {
  const token = ref<string | null>(localStorage.getItem(TOKEN_KEY))
  const authInitialized = ref(false)
  const authenticated = ref(false)
  const loggingIn = ref(false)
  const usernameInput = ref('')
  const passwordInput = ref('')
  const user = ref<UserInfo | null>(null)
  const capabilities = ref<AuthCapabilities | null>(null)
  const devices = ref<DeviceSummary[]>([])
  const selectedDeviceId = ref('')
  let initializedToken: string | null | undefined

  function setToken(newToken: string) {
    token.value = newToken
    localStorage.setItem(TOKEN_KEY, newToken)
    resetAuthState()
    resetDeviceState()
  }

  function clearToken() {
    token.value = null
    localStorage.removeItem(TOKEN_KEY)
    resetAuthState()
    resetDeviceState()
  }

  async function initializeAuth(options: InitializeAuthOptions = {}) {
    if (!token.value) {
      authenticated.value = false
      user.value = null
      capabilities.value = null
      authInitialized.value = true
      initializedToken = null
      return
    }
    if (!options.force && authInitialized.value && initializedToken === token.value) {
      return
    }
    try {
      const me = await authMe()
      authenticated.value = me.authenticated
      user.value = me.user ?? null
      capabilities.value = me.capabilities ?? null
      usernameInput.value = me.user?.email || usernameInput.value
      initializedToken = token.value
    } catch (err) {
      authenticated.value = false
      user.value = null
      capabilities.value = null
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
    if (devices.value.some((device) => device.id === selectedDeviceId.value)) {
      return
    }
    selectedDeviceId.value = devices.value.length === 1 ? devices.value[0].id : ''
  }

  function selectDeviceId(deviceId: string) {
    if (deviceId === selectedDeviceId.value) {
      return false
    }
    selectedDeviceId.value = deviceId
    return true
  }

  function resetAuthState() {
    authInitialized.value = false
    authenticated.value = false
    user.value = null
    capabilities.value = null
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
    devices,
    selectedDeviceId,
    setToken,
    clearToken,
    initializeAuth,
    login,
    loadDevices,
    selectDeviceId,
  }
})
