import { defineStore } from 'pinia'
import { ref } from 'vue'
import { readLocalStorageValue, removeLocalStorageValue, writeLocalStorageValue } from './storage'
import type { RuntimeMode } from './runtimeConfig'

const LOCAL_TOKEN_KEY = 'termbridge_local_token'
const CLOUD_TOKEN_KEY = 'termbridge_cloud_token'

export const useAuthTokensStore = defineStore('authTokens', () => {
  const localToken = ref<string | null>(readLocalStorageValue(LOCAL_TOKEN_KEY))
  const cloudToken = ref<string | null>(readLocalStorageValue(CLOUD_TOKEN_KEY))

  function setLocalToken(newToken: string) {
    localToken.value = newToken
    writeLocalStorageValue(LOCAL_TOKEN_KEY, newToken)
  }

  function setCloudToken(newToken: string) {
    cloudToken.value = newToken
    writeLocalStorageValue(CLOUD_TOKEN_KEY, newToken)
  }

  function clearLocalToken() {
    localToken.value = null
    removeLocalStorageValue(LOCAL_TOKEN_KEY)
  }

  function clearCloudToken() {
    cloudToken.value = null
    removeLocalStorageValue(CLOUD_TOKEN_KEY)
  }

  function clearTokenForTarget(target: RuntimeMode) {
    if (target === 'local') {
      clearLocalToken()
      return
    }
    clearCloudToken()
  }

  function tokenForTarget(target: RuntimeMode): string | null {
    return target === 'local' ? localToken.value : cloudToken.value
  }

  return {
    localToken,
    cloudToken,
    setLocalToken,
    setCloudToken,
    clearLocalToken,
    clearCloudToken,
    clearTokenForTarget,
    tokenForTarget,
  }
})
