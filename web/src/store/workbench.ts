import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { SessionSummary } from '../protocol/terminal'
import { readHistory } from '../features/sessions/api'
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
  const openedTabs = ref<OpenSessionTab[]>([])
  const activeSessionId = ref<string | null>(null)
  const createSessionFormOpen = ref(false)

  const activeTab = computed(
    () => openedTabs.value.find((tab) => tab.sessionId === activeSessionId.value) ?? null,
  )

  function resetForSourceChange() {
    createSessionFormOpen.value = false
    openedTabs.value = []
    activeSessionId.value = null
  }

  async function openSession(target: RuntimeTarget | null, session: SessionSummary) {
    createSessionFormOpen.value = false
    ensureTab(session)
    setActiveSession(session.id)
    return ensureHistoryLoaded(target, session)
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
    sessionId: string,
    sessionResolver: (workspaceId: string, sessionId: string) => SessionSummary | null,
  ) {
    createSessionFormOpen.value = false
    setActiveSession(sessionId)
    const tab = tabFor(sessionId)
    const session = tab ? sessionResolver(tab.workspaceId, tab.sessionId) : null
    return session ? ensureHistoryLoaded(target, session) : null
  }

  function closeTab(
    _workspaceId: string,
    sessionId: string,
    sessionResolver: (workspaceId: string, sessionId: string) => SessionSummary | null,
  ) {
    const closingIndex = openedTabs.value.findIndex((tab) => tab.sessionId === sessionId)
    if (closingIndex === -1) {
      return null
    }
    openedTabs.value.splice(closingIndex, 1)
    if (activeSessionId.value !== sessionId) {
      return null
    }
    const nextTab = openedTabs.value[Math.min(closingIndex, openedTabs.value.length - 1)] ?? null
    activeSessionId.value = nextTab?.sessionId ?? null
    return nextTab ? sessionResolver(nextTab.workspaceId, nextTab.sessionId) : null
  }

  async function ensureHistoryLoaded(target: RuntimeTarget | null, session: SessionSummary) {
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
      const response = await readHistory(target, session.workspace_id, session.id)
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
    return ensureHistoryLoaded(target, session)
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

  function openCreateSessionForm() {
    createSessionFormOpen.value = true
  }

  function closeCreateSessionForm() {
    createSessionFormOpen.value = false
  }

  function tabFor(sessionId: string) {
    return openedTabs.value.find((tab) => tab.sessionId === sessionId) ?? null
  }

  return {
    openedTabs,
    activeSessionId,
    activeTab,
    createSessionFormOpen,
    resetForSourceChange,
    openSession,
    ensureTab,
    setActiveSession,
    activateSession,
    closeTab,
    ensureHistoryLoaded,
    ensureActiveHistoryLoaded,
    resetTabHistory,
    closeRemovedSessions,
    openCreateSessionForm,
    closeCreateSessionForm,
    tabFor,
  }
})
