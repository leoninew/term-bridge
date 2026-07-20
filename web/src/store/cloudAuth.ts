import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchCloudIdentityViaCloudApi } from '../features/cloud/api'
import { fetchCloudIdentityViaLocalApi } from '../features/local/api'
import { readLocalStorageValue, removeLocalStorageValue, writeLocalStorageValue } from './storage'
import { useRuntimeConfigStore } from './runtimeConfig'
import { useCloudDevicesStore } from './cloudDevices'
import type { AuthMeResp, User as UserInfo } from '../gen/proto/termbridge/cloud/v1/auth'

const CLOUD_TOKEN_KEY = 'termbridge_cloud_token'

export const useCloudAuthStore = defineStore('cloudAuth', () => {
  const cloudToken = ref<string | null>(readLocalStorageValue(CLOUD_TOKEN_KEY))
  const authenticated = ref(false)
  const user = ref<UserInfo | null>(null)

  function setCloudToken(newToken: string) {
    cloudToken.value = newToken
    writeLocalStorageValue(CLOUD_TOKEN_KEY, newToken)
  }

  function clearCloudToken() {
    cloudToken.value = null
    removeLocalStorageValue(CLOUD_TOKEN_KEY)
  }

  async function initialize() {
    try {
      const identity = await loadCloudIdentity(cloudToken.value)
      authenticated.value = identity.authenticated
      user.value = identity.user ?? null
      if (!identity.authenticated) {
        clearCloudToken()
      }
    } catch (err) {
      authenticated.value = false
      user.value = null
      console.warn('cloud identity load failed', err)
      if (useRuntimeConfigStore().config.local.mode === 'cloud') {
        throw err
      }
    }
  }

  async function loadCloudIdentity(token: string | null): Promise<AuthMeResp> {
    if (!token) {
      return {
        authenticated: false,
        user: undefined,
        cloud_session: undefined,
        device: undefined,
      }
    }
    if (useRuntimeConfigStore().config.local.mode !== 'cloud') {
      return fetchCloudIdentityViaLocalApi(token)
    }
    return fetchCloudIdentityViaCloudApi()
  }

  function setToken(newToken: string) {
    setCloudToken(newToken)
    resetSession()
    useCloudDevicesStore().reset()
  }

  function clearToken() {
    clearCloudToken()
    resetSession()
    useCloudDevicesStore().reset()
  }

  function resetSession() {
    authenticated.value = false
    user.value = null
  }

  return {
    cloudToken,
    authenticated,
    user,
    initialize,
    setToken,
    clearToken,
  }
})
