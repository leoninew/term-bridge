import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  authLogin,
  authMe,
  listDevices,
  type DeviceSummary,
  type TokenResp,
} from '../features/gateway/api'

const TOKEN_KEY = 'termbridge_gateway_token'

export const useGatewayStore = defineStore('gateway', () => {
  const token = ref<string | null>(localStorage.getItem(TOKEN_KEY))
  const authInitialized = ref(false)
  const authenticated = ref(false)
  const loggingIn = ref(false)
  const usernameInput = ref('')
  const passwordInput = ref('')
  const devices = ref<DeviceSummary[]>([])
  const selectedDeviceId = ref('')

  function setToken(newToken: string) {
    token.value = newToken
    localStorage.setItem(TOKEN_KEY, newToken)
  }

  function clearToken() {
    token.value = null
    localStorage.removeItem(TOKEN_KEY)
  }

  async function initializeAuth() {
    try {
      const me = await authMe()
      authenticated.value = me.authenticated
      usernameInput.value = me.username || usernameInput.value
      if (me.authenticated) {
        await loadDevices()
      }
    } catch (err) {
      authenticated.value = false
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
      authenticated.value = true
      await loadDevices()
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

  return {
    token,
    authInitialized,
    authenticated,
    loggingIn,
    usernameInput,
    passwordInput,
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
