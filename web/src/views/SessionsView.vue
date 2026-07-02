<template>
  <ToastProvider>
    <section
      v-if="!gateway.authInitialized"
      class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
    >
      {{ t('gateway.checkingAuth') }}
    </section>

    <SplitterGroup
      v-else-if="gateway.capabilities?.mode === 'local' || gateway.authenticated"
      direction="horizontal"
      class="flex h-screen min-h-screen overflow-hidden bg-[var(--color-app-bg)] text-sm text-[var(--color-text)]"
    >
      <SplitterPanel id="workspace-sidebar" :default-size="20" :min-size="15" :max-size="25">
        <WorkspaceSessionSidebar
          :workspace-tree="workspaceSessions.workspaceTree"
          :active-session-id="workbench.activeSessionId"
          :stopping-session-id="stoppingSessionId"
          :rerunning-session-id="rerunningSessionId"
          :deleting-session-id="deletingSessionId"
          :removing-workspace-id="removingWorkspaceId"
          @select="openSessionTab"
          @refresh="refresh"
          @new-session="openCreateSessionForm"
          @edit-session="dialogs.openEditDialog"
          @stop-session="stopSessionFromSidebar"
          @rerun-session="rerunSessionFromSidebar"
          @delete-session="openDeleteSessionDialog"
          @remove-workspace="dialogs.openRemoveWorkspaceDialog"
          @unsupported-directory-delete="explainUnsupportedDirectoryDelete"
          @reorder-workspaces="reorderWorkspaces"
          @reorder-sessions="reorderSessions"
          @logout="logout"
          @open-dashboard="openDashboard"
        />
      </SplitterPanel>

      <SplitterResizeHandle
        class="group flex w-1 shrink-0 cursor-col-resize items-stretch justify-center bg-[var(--color-app-bg)] outline-none"
      >
        <span
          class="w-px bg-[var(--color-border)] transition group-hover:bg-[var(--color-border-strong)]"
        />
      </SplitterResizeHandle>

      <SplitterPanel id="terminal-workbench" :min-size="55">
        <SessionWorkbench
          :opened-tabs="workbench.openedTabs"
          :active-session-id="workbench.activeSessionId"
          :active-tab="workbench.activeTab"
          :active-session="activeSession"
          :current-device="gateway.currentDevice"
          :authenticated="gateway.authenticated"
          :user-display-name="userDisplayName"
          :user-email="gateway.user?.email ?? ''"
          :user-action-label="userActionLabel"
          :user-action-disabled="userActionDisabled"
          :create-session-form-open="workbench.createSessionFormOpen"
          :create-cwd="createDraft.cwd"
          :create-name="createDraft.sessionName"
          :create-command="createDraft.commandText"
          :creating-session="creatingSession"
          :terminal-ws-url="activeTerminalWsUrl"
          :session-title="sessionTitle"
          @activate-tab="activateOpenedTab"
          @close-tab="closeTab"
          @reorder-tabs="workbench.openedTabs = $event"
          @open-create="() => openCreateSessionForm()"
          @submit-create="startSession"
          @cancel-create="cancelCreateSession"
          @update:create-cwd="createDraft.cwd = $event"
          @update:create-name="createDraft.sessionName = $event"
          @update:create-command="createDraft.commandText = $event"
          @create-workbench="createSessionWorkbench = $event"
          @terminal-state="handleTerminalState"
          @terminal-error="handleTerminalError"
          @open-dashboard="openDashboard"
          @open-user-auth="openUserAuth"
        />
      </SplitterPanel>
    </SplitterGroup>

    <EditSessionDialog
      :open="dialogs.editDialogOpen"
      :session="dialogs.selectedSession"
      :editing="editingSession"
      @update:open="dialogs.editDialogOpen = $event"
      @submit="editSelectedSession"
    />

    <DeleteSessionDialog
      :open="dialogs.deleteSessionDialogOpen"
      :session="dialogs.selectedSession"
      :deleting="!!deletingSessionId"
      @update:open="dialogs.deleteSessionDialogOpen = $event"
      @confirm="deleteSelectedSession"
    />

    <RemoveWorkspaceDialog
      :open="dialogs.removeWorkspaceDialogOpen"
      :workspace="dialogs.selectedWorkspace"
      :removing="!!removingWorkspaceId"
      @update:open="dialogs.removeWorkspaceDialogOpen = $event"
      @confirm="removeSelectedWorkspace"
    />

    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { SplitterGroup, SplitterPanel, SplitterResizeHandle, ToastProvider } from 'reka-ui'
  import DeleteSessionDialog from '../components/session/DeleteSessionDialog.vue'
  import EditSessionDialog from '../components/session/EditSessionDialog.vue'
  import RemoveWorkspaceDialog from '../components/session/RemoveWorkspaceDialog.vue'
  import SessionWorkbench from '../components/session/SessionWorkbench.vue'
  import ToastHost from '../components/session/ToastHost.vue'
  import WorkspaceSessionSidebar from '../components/workspace/WorkspaceSessionSidebar.vue'
  import { logTerminalDiagnostic } from '../components/terminal/diagnostics'
  import { useCreateSessionDraft } from '../composable/useCreateSessionDraft'
  import { useSessionDialogs } from '../composable/useSessionDialogs'
  import { useTerminalSize } from '../composable/useTerminalSize'
  import {
    closeSession,
    createSession,
    deleteSession,
    getSession,
    rerunSession,
    terminalWsUrl,
    updateSession,
  } from '../features/sessions/api'
  import { authLogout, cloudOAuthStartURL } from '../features/gateway/api'
  import { deleteWorkspace } from '../features/workspaces/api'
  import type { ServerControlMessage, SessionSummary, WorkspaceSummary } from '../protocol/terminal'
  import { useGatewayStore } from '../store/gateway'
  import { useNotificationsStore } from '../store/notifications'
  import { useWorkbenchStore } from '../store/workbench'
  import { useWorkspaceSessionsStore } from '../store/workspaceSessions'

  const { t } = useI18n()
  const router = useRouter()
  const gateway = useGatewayStore()
  const workspaceSessions = useWorkspaceSessionsStore()
  const workbench = useWorkbenchStore()
  const notifications = useNotificationsStore()
  const dialogs = useSessionDialogs()
  const createDraft = useCreateSessionDraft()

  const createSessionWorkbench = ref<HTMLElement | null>(null)
  const { measureInitialTerminalSize } = useTerminalSize(createSessionWorkbench)

  const creatingSession = ref(false)
  const editingSession = ref(false)
  const stoppingSessionId = ref<string | null>(null)
  const rerunningSessionId = ref<string | null>(null)
  const deletingSessionId = ref<string | null>(null)
  const removingWorkspaceId = ref<string | null>(null)

  const isLocalMode = computed(() => gateway.capabilities?.mode === 'local')
  const userActionDisabled = computed(
    () => isLocalMode.value && !gateway.capabilities?.cloud_oauth_enabled,
  )
  const userActionLabel = computed(() =>
    isLocalMode.value ? t('dashboard.signInWithOAuth') : t('dashboard.signIn'),
  )
  const userDisplayName = computed(
    () => gateway.user?.display_name || gateway.user?.email || t('dashboard.signedIn'),
  )

  const activeSession = computed(() => {
    const tab = workbench.activeTab
    return tab ? workspaceSessions.sessionById(tab.workspaceId, tab.sessionId) : null
  })

  const activeTerminalWsUrl = computed(() => {
    const session = activeSession.value
    const target = gateway.runtimeTarget
    if (!session || !target) {
      return null
    }
    return terminalWsUrl(target, session.workspace_id, session.id, gateway.token ?? undefined)
  })

  async function refresh() {
    const target = gateway.runtimeTarget
    if (!target) {
      return
    }
    try {
      await workspaceSessions.refresh(target)
      await notifyHistoryError(
        await workbench.ensureActiveHistoryLoaded(target, workspaceSessions.sessionById),
      )
    } catch (err) {
      notifications.notifyError(t('toast.refreshFailed'), err)
    }
  }

  async function openDashboard() {
    await router.push({ name: 'dashboard' })
  }

  async function openUserAuth() {
    if (gateway.authenticated) {
      return
    }
    if (!isLocalMode.value) {
      await router.push({ name: 'login', query: { redirect: '/sessions' } })
      return
    }
    if (!gateway.capabilities?.cloud_oauth_enabled) {
      return
    }
    window.location.href = cloudOAuthStartURL()
  }

  async function logout() {
    try {
      await authLogout()
    } catch {
      // ignore logout API errors — clear local state anyway
    }
    gateway.clearToken()
    gateway.passwordInput = ''
    workspaceSessions.reset()
    workbench.resetForSourceChange()
    await router.replace({ name: 'login' })
  }

  async function startSession() {
    if (creatingSession.value) {
      return
    }
    const draft = createDraft.validate()
    if (draft.error) {
      notifyCreateSessionValidationError(draft.error)
      return
    }
    const target = gateway.runtimeTarget
    if (!target) {
      return
    }
    creatingSession.value = true
    try {
      const size = measureInitialTerminalSize()
      logTerminalDiagnostic('session.create.request', {
        name: draft.value.name,
        cwd: draft.value.cwd,
        command: draft.value.command[0],
        args: draft.value.command.length - 1,
        cols: size.cols,
        rows: size.rows,
      })
      const created = await createSession(target, draft.value.workspaceId, {
        ...(draft.value.workspaceId ? { workspace_id: draft.value.workspaceId } : {}),
        name: draft.value.name,
        cwd: draft.value.cwd,
        command: draft.value.command,
        cols: size.cols,
        rows: size.rows,
      })
      logTerminalDiagnostic('session.create.response', {
        sessionId: created.session_id,
        workspaceId: created.workspace_id,
        state: created.state,
      })
      const session = await getSession(target, created.workspace_id, created.session_id)
      logTerminalDiagnostic('session.create.summary', {
        sessionId: session.id,
        lifecycleState: session.lifecycle_state,
        attachmentState: session.attachment_state,
        exitCode: session.exit_code,
      })
      if (!workspaceSessions.upsertSession(session)) {
        await refresh()
      }
      createDraft.reset(undefined, t('dialog.defaultSessionName'))
      workbench.closeCreateSessionForm()
      await openSessionTab(session)
      notifications.pushToast('success', t('toast.sessionCreated'), session.name)
    } catch (err) {
      notifications.notifyError(t('toast.createSessionFailed'), err)
      await refresh()
    } finally {
      creatingSession.value = false
    }
  }

  async function editSelectedSession(name: string) {
    const target = gateway.runtimeTarget
    if (!dialogs.selectedSession || editingSession.value || !target) {
      return
    }
    if (!name) {
      notifications.pushToast('error', t('toast.editSessionFailed'), t('message.nameRequired'))
      return
    }
    editingSession.value = true
    try {
      const updated = await updateSession(
        target,
        dialogs.selectedSession.workspace_id,
        dialogs.selectedSession.id,
        { name },
      )
      workspaceSessions.updateSession(updated)
      dialogs.selectedSession = updated
      dialogs.editDialogOpen = false
      notifications.pushToast('success', t('toast.sessionEdited'), name)
    } catch (err) {
      notifications.notifyError(t('toast.editSessionFailed'), err)
    } finally {
      editingSession.value = false
    }
  }

  async function deleteSelectedSession() {
    const session = dialogs.selectedSession
    const target = gateway.runtimeTarget
    if (!session || deletingSessionId.value || isActiveLifecycle(session) || !target) {
      return
    }
    deletingSessionId.value = session.id
    try {
      await deleteSession(target, session.workspace_id, session.id)
      workspaceSessions.removeSession(session.workspace_id, session.id)
      const nextSession = workbench.closeTab(
        session.workspace_id,
        session.id,
        workspaceSessions.sessionById,
      )
      if (nextSession) {
        await notifyHistoryError(await workbench.ensureHistoryLoaded(target, nextSession))
      }
      dialogs.clearSelectedSession()
      dialogs.deleteSessionDialogOpen = false
      notifications.pushToast('success', t('toast.sessionDeleted'), session.name)
    } catch (err) {
      notifications.notifyError(t('toast.deleteSessionFailed'), err)
    } finally {
      deletingSessionId.value = null
    }
  }

  async function removeSelectedWorkspace() {
    const workspace = dialogs.selectedWorkspace
    const target = gateway.runtimeTarget
    if (!workspace || removingWorkspaceId.value || !target) {
      return
    }
    removingWorkspaceId.value = workspace.id
    try {
      await deleteWorkspace(target, workspace.id)
      const removedSessions = workspaceSessions.removeWorkspace(workspace.id)
      workbench.closeRemovedSessions(removedSessions)
      dialogs.clearSelectedWorkspace()
      dialogs.removeWorkspaceDialogOpen = false
      notifications.pushToast(
        'success',
        t('toast.workspaceRemoved'),
        t('message.workspaceRemoved', { name: workspace.name }),
      )
    } catch (err) {
      notifications.notifyError(t('toast.removeWorkspaceFailed'), err)
    } finally {
      removingWorkspaceId.value = null
    }
  }

  async function reorderWorkspaces(workspaceIds: string[]) {
    try {
      await workspaceSessions.reorderWorkspaces(gateway.runtimeTarget, workspaceIds)
    } catch (err) {
      notifications.notifyError(t('toast.updateWorkspaceOrderFailed'), err)
      await refresh()
    }
  }

  async function reorderSessions(workspaceId: string, sessionIds: string[]) {
    try {
      await workspaceSessions.reorderSessions(gateway.runtimeTarget, workspaceId, sessionIds)
    } catch (err) {
      notifications.notifyError(t('toast.updateSessionOrderFailed'), err)
      await refresh()
    }
  }

  async function openSessionTab(session: SessionSummary) {
    await notifyHistoryError(await workbench.openSession(gateway.runtimeTarget, session))
  }

  async function activateOpenedTab(sessionId: string) {
    await notifyHistoryError(
      await workbench.activateSession(
        gateway.runtimeTarget,
        sessionId,
        workspaceSessions.sessionById,
      ),
    )
  }

  async function closeTab(workspaceId: string, sessionId: string) {
    const nextSession = workbench.closeTab(workspaceId, sessionId, workspaceSessions.sessionById)
    if (nextSession) {
      await notifyHistoryError(
        await workbench.ensureHistoryLoaded(gateway.runtimeTarget, nextSession),
      )
    }
  }

  function openCreateSessionForm(workspace?: WorkspaceSummary) {
    logTerminalDiagnostic('session.form.open', {
      workspaceId: workspace?.id,
      workspacePath: workspace?.path,
      activeSessionId: workbench.activeSessionId,
    })
    createDraft.reset(workspace, t('dialog.defaultSessionName'))
    workbench.openCreateSessionForm()
  }

  function cancelCreateSession() {
    logTerminalDiagnostic('session.form.cancel', { activeSessionId: workbench.activeSessionId })
    workbench.closeCreateSessionForm()
  }

  async function stopSessionFromSidebar(session: SessionSummary) {
    const target = gateway.runtimeTarget
    if (!isStoppableLifecycle(session) || stoppingSessionId.value || !target) {
      return
    }
    stoppingSessionId.value = session.id
    try {
      const updated = await closeSession(target, session.workspace_id, session.id)
      workspaceSessions.updateSession(updated)
      await notifyHistoryError(await workbench.ensureHistoryLoaded(target, updated))
    } catch (err) {
      notifications.notifyError(t('toast.stopSessionFailed'), err)
    } finally {
      stoppingSessionId.value = null
    }
  }

  async function rerunSessionFromSidebar(session: SessionSummary) {
    const target = gateway.runtimeTarget
    if (!canRerunLifecycle(session) || rerunningSessionId.value || !target) {
      return
    }
    rerunningSessionId.value = session.id
    try {
      const size = measureInitialTerminalSize()
      const response = await rerunSession(target, session.workspace_id, session.id, size)
      const updated = await getSession(target, response.workspace_id, response.session_id)
      workspaceSessions.updateSession(updated)
      workbench.resetTabHistory(updated.workspace_id, updated.id)
      await openSessionTab(updated)
      notifications.pushToast('success', t('toast.sessionRerun'), updated.name)
    } catch (err) {
      notifications.notifyError(t('toast.rerunSessionFailed'), err)
      await refreshSessionAfterRerunFailure(session)
    } finally {
      rerunningSessionId.value = null
    }
  }

  async function refreshSessionAfterRerunFailure(session: SessionSummary) {
    const target = gateway.runtimeTarget
    if (!target) {
      return
    }
    workbench.resetTabHistory(session.workspace_id, session.id)
    try {
      const updated = await getSession(target, session.workspace_id, session.id)
      workspaceSessions.updateSession(updated)
      await notifyHistoryError(await workbench.ensureHistoryLoaded(target, updated))
    } catch {
      await refresh()
    }
  }

  function openDeleteSessionDialog(session: SessionSummary) {
    dialogs.openDeleteSessionDialog(session)
    if (isActiveLifecycle(session)) {
      notifications.pushToast(
        'info',
        t('toast.sessionCannotBeDeletedYet'),
        t('message.closeSessionBeforeDelete'),
      )
    }
  }

  function explainUnsupportedDirectoryDelete(workspace: WorkspaceSummary) {
    dialogs.selectedWorkspace = workspace
    notifications.pushToast(
      'info',
      t('toast.directoryDeleteUnsupported'),
      t('message.directoryDeleteUnsupported'),
    )
  }

  function sessionTitle(workspaceId: string, sessionId: string) {
    const session = workspaceSessions.sessionById(workspaceId, sessionId)
    return session ? workspaceSessions.sessionTitle(session) : sessionId.slice(0, 8)
  }

  function isActiveLifecycle(session: SessionSummary) {
    return session.lifecycle_state === 'running'
  }

  function isStoppableLifecycle(session: SessionSummary) {
    return session.lifecycle_state === 'running'
  }

  function canRerunLifecycle(session: SessionSummary) {
    return ['stopped', 'failed'].includes(session.lifecycle_state)
  }

  function handleTerminalState(message: ServerControlMessage) {
    if (message.type === 'error') {
      notifications.pushToast(
        'error',
        t('toast.terminalError', { code: message.code }),
        message.message,
      )
    }
    if (message.type === 'state' || message.type === 'exited') {
      void refreshActiveSession()
    }
  }

  async function refreshActiveSession(session = activeSession.value) {
    const target = gateway.runtimeTarget
    if (!session || !target) {
      return
    }
    try {
      const updated = await getSession(target, session.workspace_id, session.id)
      workspaceSessions.updateSession(updated)
      await notifyHistoryError(await workbench.ensureHistoryLoaded(target, updated))
    } catch (err) {
      notifications.notifyError(t('toast.refreshFailed'), err)
    }
  }

  function handleTerminalError(message: string) {
    notifications.pushToast('error', t('toast.terminalConnectionFailed'), message)
  }

  function notifyCreateSessionValidationError(
    error: 'name-required' | 'cwd-required' | 'command-required',
  ) {
    const messageKey = {
      'name-required': 'message.sessionNameRequired',
      'cwd-required': 'message.workingDirectoryRequired',
      'command-required': 'message.commandRequired',
    }[error]
    notifications.pushToast('error', t('toast.createSessionFailed'), t(messageKey))
  }

  async function notifyHistoryError(message: string | null) {
    if (message) {
      notifications.pushToast('error', t('toast.readHistoryFailed'), message)
    }
  }

  onMounted(async () => {
    try {
      await gateway.loadDevices()
      await refresh()
    } catch (err) {
      notifications.notifyError(t('toast.refreshFailed'), err)
    }
  })
</script>
