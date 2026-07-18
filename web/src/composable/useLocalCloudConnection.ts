import { computed, proxyRefs } from 'vue'
import { useAsyncAction, type RunAsyncActionOptions } from './useAsyncAction'
import {
  applyCloudConnectionTime,
  clearCloudConnectionTime,
  cloudOAuthConfigured,
  hasCloudConnectionTime,
  markCloudConnected,
  startCloudOAuth,
} from '../features/cloud/oauth'
import { agentStatus, connectCloudWithToken, disconnectCloud } from '../features/local/api'
import type { DeviceSummary } from '../gen/proto/termbridge/cloud/v1/device'
import type { CloudSessionSummary } from '../gen/proto/termbridge/cloud/v1/session'
import { useCloudAuthStore } from '../store/cloudAuth'
import { useCloudSessionStore } from '../store/cloudSession'

export type LocalCloudHydrateResult = {
  device: DeviceSummary | null
  reportedSession: CloudSessionSummary | null
}

export type LocalCloudConnectResult =
  | { ok: true; kind: 'connected' }
  | { ok: true; kind: 'oauth_redirect' }
  | { ok: false; kind: 'busy' | 'not_configured' | 'failed'; error?: unknown }

export function useLocalCloudConnection() {
  const cloudAuth = useCloudAuthStore()
  const cloudSessionStore = useCloudSessionStore()
  const cloudAction = useAsyncAction()

  const cloudSession = computed(() => cloudSessionStore.cloudSession)
  const connected = computed(() => !!cloudSessionStore.cloudSession)
  const connecting = computed(() => cloudAction.running)
  const oauthConfigured = computed(() => cloudOAuthConfigured())

  function setCloudConnection(summary: CloudSessionSummary | null) {
    cloudSessionStore.setCloudSession(summary)
  }

  function clearLocalCloudSession() {
    cloudSessionStore.reset()
  }

  async function reconcileCloudConnection(reportedSession: CloudSessionSummary | null) {
    if (hasCloudConnectionTime()) {
      const connectedSession = applyCloudConnectionTime(reportedSession)
      if (connectedSession) {
        setCloudConnection(connectedSession)
        return
      }
      clearCloudConnectionTime()
    }
    if (await connectStoredCloudToken()) {
      return
    }
    setCloudConnection(null)
  }

  async function connectStoredCloudToken(): Promise<boolean> {
    const cloudToken = cloudAuth.cloudToken
    if (!cloudToken || cloudAction.running) {
      return false
    }
    const result = await cloudAction.run(async () => {
      const response = await connectCloudWithToken(cloudToken)
      const connectedSession = markCloudConnected(response.cloud_session)
      if (!connectedSession) {
        throw new Error('Cloud connect response missing cloud_session')
      }
      setCloudConnection(connectedSession)
    })
    return result.ok
  }

  async function hydrateFromAgent(): Promise<LocalCloudHydrateResult> {
    const me = await agentStatus()
    const reportedSession = me.cloud_session ?? null
    await reconcileCloudConnection(reportedSession)
    return {
      device: me.device ?? null,
      reportedSession,
    }
  }

  async function disconnectSession() {
    await disconnectCloud()
    clearCloudConnectionTime()
    setCloudConnection(null)
  }

  async function disconnectDevice(options: RunAsyncActionOptions = {}) {
    if (cloudAction.running) {
      return { ok: false as const, kind: 'busy' as const }
    }
    const result = await cloudAction.run(async () => {
      await disconnectSession()
    }, options)
    return result.ok
      ? { ok: true as const, kind: 'disconnected' as const }
      : { ok: false as const, kind: 'failed' as const, error: result.cause }
  }

  async function connectDevice(
    options: RunAsyncActionOptions = {},
  ): Promise<LocalCloudConnectResult> {
    if (cloudAction.running) {
      return { ok: false, kind: 'busy' }
    }
    if (cloudAuth.cloudToken) {
      const result = await cloudAction.run(async () => {
        const response = await connectCloudWithToken(cloudAuth.cloudToken!)
        const connectedSession = markCloudConnected(response.cloud_session)
        if (!connectedSession) {
          throw new Error('Cloud connect response missing cloud_session')
        }
        setCloudConnection(connectedSession)
      }, options)
      return result.ok
        ? { ok: true, kind: 'connected' }
        : { ok: false, kind: 'failed', error: result.cause }
    }
    if (!cloudOAuthConfigured()) {
      return { ok: false, kind: 'not_configured' }
    }
    startCloudOAuth('/')
    return { ok: true, kind: 'oauth_redirect' }
  }

  return proxyRefs({
    cloudSession,
    connected,
    connecting,
    oauthConfigured,
    hydrateFromAgent,
    reconcileCloudConnection,
    connectDevice,
    disconnectDevice,
    disconnectSession,
    clearLocalCloudSession,
    setCloudConnection,
  })
}
