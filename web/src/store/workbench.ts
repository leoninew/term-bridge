import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { SessionSummary } from '../gen/proto/termbridge/agent/v1/workspace'
import type { SessionRuntimeApi } from '../features/sessions/runtime'
import type { RuntimeTarget } from '../features/runtimeTarget'
import { errorMessage } from './notifications'

export type OpenSessionTab = {
  workspaceId: string
  sessionId: string
  historyText: string
  historyLoaded: boolean
  historyLoading: boolean
  historyError: string | null
}

export type RemovedSession = Pick<SessionSummary, 'id' | 'workspace_id'>

export const useWorkbenchStore = defineStore('workbench', () => {
  // Session IDs are globally unique ULIDs; TabsRoot, TabsTrigger, and TerminalPane must share sessionId values.
  const openedTabs = ref<OpenSessionTab[]>([])
  const activeSessionId = ref<string | null>(null)

  const activeTab = computed(
    () => openedTabs.value.find((tab) => tab.sessionId === activeSessionId.value) ?? null,
  )

  function resetForSourceChange() {
    openedTabs.value = []
    activeSessionId.value = null
  }

  async function openSession(
    target: RuntimeTarget | null,
    runtimeApi: SessionRuntimeApi,
    session: SessionSummary,
  ) {
    ensureTab(session)
    setActiveSession(session.id)
    return ensureHistoryLoaded(target, runtimeApi, session)
  }

  function ensureTab(session: SessionSummary) {
    if (openedTabs.value.some((tab) => tab.sessionId === session.id)) {
      return
    }
    openedTabs.value.push({
      workspaceId: session.workspace_id,
      sessionId: session.id,
      historyText: '',
      historyLoaded: false,
      historyLoading: false,
      historyError: null,
    })
  }

  function setActiveSession(sessionId: string) {
    activeSessionId.value = sessionId
  }

  async function activateSession(
    target: RuntimeTarget | null,
    runtimeApi: SessionRuntimeApi,
    sessionId: string,
    sessionResolver: (workspaceId: string, sessionId: string) => SessionSummary | null,
  ) {
    setActiveSession(sessionId)
    const tab = tabFor(sessionId)
    const session = tab ? sessionResolver(tab.workspaceId, tab.sessionId) : null
    return session ? ensureHistoryLoaded(target, runtimeApi, session) : null
  }

  function closeTab(
    _workspaceId: string,
    sessionId: string,
    sessionResolver: (workspaceId: string, sessionId: string) => SessionSummary | null,
  ) {
    return closeTabs([sessionId], sessionResolver)
  }

  function closeTabs(
    sessionIds: string[],
    sessionResolver: (workspaceId: string, sessionId: string) => SessionSummary | null,
  ) {
    const closingSessionIds = new Set(sessionIds)
    const activeIndex = openedTabs.value.findIndex((tab) => tab.sessionId === activeSessionId.value)
    const activeTabIsClosing =
      activeIndex !== -1 && closingSessionIds.has(openedTabs.value[activeIndex].sessionId)
    const nextTab = activeTabIsClosing
      ? (openedTabs.value
          .slice(activeIndex + 1)
          .find((tab) => !closingSessionIds.has(tab.sessionId)) ??
        openedTabs.value
          .slice(0, activeIndex)
          .reverse()
          .find((tab) => !closingSessionIds.has(tab.sessionId)) ??
        null)
      : null
    const remainingTabs = openedTabs.value.filter((tab) => !closingSessionIds.has(tab.sessionId))

    if (remainingTabs.length === openedTabs.value.length) {
      return null
    }

    openedTabs.value = remainingTabs
    if (!activeTabIsClosing) {
      return null
    }

    activeSessionId.value = nextTab?.sessionId ?? null
    return nextTab ? sessionResolver(nextTab.workspaceId, nextTab.sessionId) : null
  }

  async function ensureHistoryLoaded(
    target: RuntimeTarget | null,
    runtimeApi: SessionRuntimeApi,
    session: SessionSummary,
  ) {
    const tab = tabFor(session.id)
    if (
      !tab ||
      !target ||
      session.lifecycle_state === 'running' ||
      tab.historyLoaded ||
      tab.historyLoading
    ) {
      return null
    }
    tab.historyLoading = true
    tab.historyError = null
    try {
      const response = await runtimeApi.readHistory(session.workspace_id, session.id)
      tab.historyText = response.data
      tab.historyLoaded = true
      return null
    } catch (err) {
      const message = errorMessage(err)
      tab.historyError = message
      return message
    } finally {
      tab.historyLoading = false
    }
  }

  async function ensureActiveHistoryLoaded(
    target: RuntimeTarget | null,
    runtimeApi: SessionRuntimeApi,
    sessionResolver: (workspaceId: string, sessionId: string) => SessionSummary | null,
  ) {
    const tab = activeTab.value
    if (!tab) {
      return null
    }
    const session = sessionResolver(tab.workspaceId, tab.sessionId)
    if (!session) {
      return null
    }
    return ensureHistoryLoaded(target, runtimeApi, session)
  }

  function resetTabHistory(workspaceId: string, sessionId: string) {
    const tab = tabFor(sessionId)
    if (!tab || tab.workspaceId !== workspaceId) {
      return
    }
    tab.historyText = ''
    tab.historyLoaded = false
    tab.historyLoading = false
    tab.historyError = null
  }

  function closeRemovedSessions(removedSessions: RemovedSession[]) {
    const removedSessionIds = new Set(removedSessions.map((session) => session.id))
    openedTabs.value = openedTabs.value.filter((tab) => !removedSessionIds.has(tab.sessionId))
    if (activeSessionId.value && removedSessionIds.has(activeSessionId.value)) {
      activeSessionId.value = openedTabs.value[0]?.sessionId ?? null
    }
  }

  function tabFor(sessionId: string) {
    return openedTabs.value.find((tab) => tab.sessionId === sessionId) ?? null
  }

  return {
    openedTabs,
    activeSessionId,
    activeTab,
    resetForSourceChange,
    openSession,
    ensureTab,
    setActiveSession,
    activateSession,
    closeTab,
    closeTabs,
    ensureHistoryLoaded,
    ensureActiveHistoryLoaded,
    resetTabHistory,
    closeRemovedSessions,
    tabFor,
  }
})
