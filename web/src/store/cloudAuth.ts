import { defineStore } from 'pinia'
import { ref } from 'vue'
import { authLogin, authMeViaCloud } from '../features/cloud/api'
import { useAuthTokensStore } from './authTokens'
import { useCloudDevicesStore } from './cloudDevices'
import type { TokenResp, User as UserInfo } from '../gen/proto/termbridge/cloud/v1/auth'

type InitializeAuthOptions = {
  force?: boolean
}

export const useCloudAuthStore = defineStore('cloudAuth', () => {
  const authInitialized = ref(false)
  const authenticated = ref(false)
  const loggingIn = ref(false)
  const usernameInput = ref('')
  const passwordInput = ref('')
  const user = ref<UserInfo | null>(null)
  let initializedToken: string | null | undefined

  async function initializeAuth(options: InitializeAuthOptions = {}) {
    const tokens = useAuthTokensStore()
    if (!options.force && authInitialized.value && initializedToken === tokens.cloudToken) {
      return
    }
    try {
      const me = await authMeViaCloud()
      authenticated.value = me.authenticated
      user.value = me.user ?? null
      usernameInput.value = me.user?.email || usernameInput.value
      initializedToken = tokens.cloudToken
    } catch (err) {
      authenticated.value = false
      user.value = null
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

  function setToken(newToken: string) {
    useAuthTokensStore().setCloudToken(newToken)
    reset()
    useCloudDevicesStore().reset()
  }

  function clearToken() {
    useAuthTokensStore().clearCloudToken()
    reset()
    useCloudDevicesStore().reset()
  }

  function reset() {
    authInitialized.value = false
    authenticated.value = false
    user.value = null
    initializedToken = undefined
  }

  return {
    authInitialized,
    authenticated,
    loggingIn,
    usernameInput,
    passwordInput,
    user,
    initializeAuth,
    login,
    setToken,
    clearToken,
    reset,
  }
})
