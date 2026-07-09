import { defineStore } from 'pinia'
import { ref } from 'vue'
import { authLoginViaLocal, authMe } from '../features/local/api'
import { useAuthTokensStore } from './authTokens'
import type { LocalAuthLoginResp, User as UserInfo } from '../gen/proto/termbridge/cloud/v1/auth'
import type { CloudSessionSummary } from '../gen/proto/termbridge/cloud/v1/session'

type InitializeAuthOptions = {
  force?: boolean
}

export const useLocalAuthStore = defineStore('localAuth', () => {
  const authInitialized = ref(false)
  const authenticated = ref(false)
  const user = ref<UserInfo | null>(null)
  const cloudSession = ref<CloudSessionSummary | null>(null)
  let initializedToken: string | null | undefined

  async function ensureToken() {
    const tokens = useAuthTokensStore()
    if (tokens.localToken) {
      return
    }
    const response: LocalAuthLoginResp = await authLoginViaLocal()
    tokens.setLocalToken(response.access_token)
    reset()
  }

  async function initializeAuth(options: InitializeAuthOptions = {}) {
    await ensureToken()
    const tokens = useAuthTokensStore()
    if (!options.force && authInitialized.value && initializedToken === tokens.localToken) {
      return
    }
    try {
      const me = await authMe()
      authenticated.value = me.authenticated
      user.value = me.user ?? null
      cloudSession.value = me.cloud_session ?? null
      initializedToken = tokens.localToken
    } catch (err) {
      authenticated.value = false
      user.value = null
      cloudSession.value = null
      initializedToken = undefined
      throw err
    } finally {
      authInitialized.value = true
    }
  }

  function setCloudSession(summary: CloudSessionSummary | null) {
    cloudSession.value = summary
  }

  function clearToken() {
    useAuthTokensStore().clearLocalToken()
    reset()
  }

  function reset() {
    authInitialized.value = false
    authenticated.value = false
    user.value = null
    cloudSession.value = null
    initializedToken = undefined
  }

  return {
    authInitialized,
    authenticated,
    user,
    cloudSession,
    ensureToken,
    initializeAuth,
    setCloudSession,
    clearToken,
    reset,
  }
})
