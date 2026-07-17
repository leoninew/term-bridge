<template>
  <SplitterGroup
    v-if="isLocalMode || cloudAuth.authenticated"
    direction="horizontal"
    class="sessions-shell flex h-screen min-h-screen overflow-hidden bg-[var(--color-app-bg)] text-sm text-[var(--color-text)]"
  >
    <SplitterPanel
      id="workspace-sidebar"
      class="workspace-sidebar-panel"
      collapsible
      :collapsed-size="0"
      :default-size="20"
      :min-size="18"
      :max-size="24"
    >
      <WorkspaceSessionSidebar
        :workspace-tree="workspaceSessions.workspaceTree"
        :active-session-id="workbench.activeSessionId"
        :active-workspace="activeWorkspaceForViewSwitch"
        :stopping-session-id="stoppingSessionId"
        :rerunning-session-id="rerunningSessionId"
        :deleting-session-id="deletingSessionId"
        :removing-workspace-id="removingWorkspaceId"
        :loading="workspaceSessions.loading"
        :load-error="workspaceTreeError"
        :help-href="helpHref"
        :home-route-name="props.homeRouteName"
        @select="openSessionTab"
        @refresh="refresh"
        @new-session="openCreateSessionForm"
        @copy-session="openCopiedSessionForm"
        @edit-session="openEditSessionDialog"
        @stop-session="stopSessionFromSidebar"
        @rerun-session="rerunSessionFromSidebar"
        @delete-session="openDeleteSessionDialog"
        @remove-workspace="dialogs.openRemoveWorkspaceDialog"
        @open-files="openWorkspaceFiles"
        @open-git="openWorkspaceGit"
        @unsupported-directory-delete="explainUnsupportedDirectoryDelete"
        @reorder-workspaces="reorderWorkspaces"
        @reorder-sessions="reorderSessions"
        @logout="handleLogout"
        @open-dashboard="openDashboard"
        @open-shortcuts="openShortcuts"
      />
    </SplitterPanel>

    <SplitterResizeHandle
      class="sessions-resize-handle group flex w-1 shrink-0 cursor-col-resize items-stretch justify-center bg-[var(--color-app-bg)] outline-none"
    >
      <span
        class="w-px bg-[var(--color-border)] transition group-hover:bg-[var(--color-border-strong)]"
      />
    </SplitterResizeHandle>

    <SplitterPanel id="terminal-workbench" :min-size="55">
      <div class="relative h-full min-h-0 min-w-0">
        <SessionWorkbench
          class="session-workbench-stage"
          :opened-tabs="workbench.openedTabs"
          :active-session-id="workbench.activeSessionId"
          :active-tab="workbench.activeTab"
          :active-session="activeSession"
          :current-device="workbenchDevice"
          :terminal-ws-url="activeTerminalWsUrl"
          :loading="workspaceSessions.loading && workbench.openedTabs.length === 0"
          :has-terminal-tabs="terminalTabs.length > 0"
          :has-background-running-sessions="hasRunningSessions"
          :session-title="sessionTitle"
          :session-lifecycle-state="sessionLifecycleState"
          :session-command-source="sessionCommandSource"
          :session-source-label="sessionSourceLabel"
          @activate-tab="activateOpenedTab"
          @close-tab="closeTab"
          @close-terminal-tabs="closeTerminalTabs"
          @open-close-background-sessions-drawer="openCloseBackgroundSessionsDrawer"
          @reorder-tabs="workbench.openedTabs = $event"
          @open-create="() => openCreateSessionForm()"
          @workbench="terminalWorkbench = $event"
          @terminal-state="handleTerminalState"
          @terminal-error="handleTerminalError"
        />
      </div>
    </SplitterPanel>
  </SplitterGroup>

  <CreateSessionDialog
    :open="dialogs.createSessionDialogOpen"
    :cwd="createDraft.cwd"
    :name="createDraft.sessionName"
    :command="createDraft.commandText"
    :command-source="createDraft.commandSource"
    :selected-shortcut-id="createDraft.selectedShortcutId"
    :selected-shortcut-name="createDraft.selectedShortcutName"
    :shortcuts="enabledShortcuts"
    :creating="createAction.running"
    @update:open="dialogs.createSessionDialogOpen = $event"
    @update:cwd="createDraft.cwd = $event"
    @update:name="createDraft.sessionName = $event"
    @update:command="createDraft.commandText = $event"
    @update:command-source="createDraft.selectCommandSource($event, enabledShortcuts)"
    @update:selected-shortcut-id="selectCreateShortcut"
    @submit="startSession"
  />

  <EditSessionDialog
    :open="dialogs.editDialogOpen"
    :session="dialogs.selectedSession"
    :shortcuts="enabledShortcuts"
    :editing="editAction.running"
    @update:open="dialogs.editDialogOpen = $event"
    @submit="editSelectedSession"
  />

  <DeleteSessionDialog
    :open="dialogs.deleteSessionDialogOpen"
    :session="dialogs.selectedSession"
    :deleting="!!deletingSessionId || deleteAction.running"
    @update:open="dialogs.deleteSessionDialogOpen = $event"
    @confirm="deleteSelectedSession"
  />

  <RemoveWorkspaceDialog
    :open="dialogs.removeWorkspaceDialogOpen"
    :workspace="dialogs.selectedWorkspace"
    :removing="!!removingWorkspaceId || removeWorkspaceAction.running"
    @update:open="dialogs.removeWorkspaceDialogOpen = $event"
    @confirm="removeSelectedWorkspace"
  />

  <CloseBackgroundSessionsDrawer
    :open="closeBackgroundSessionsDrawerOpen"
    :workspace-tree="sessionWorkspaceTree"
    :opened-tabs="workbench.openedTabs"
    :closing="closingBackgroundSessions"
    @update:open="closeBackgroundSessionsDrawerOpen = $event"
    @confirm="closeBackgroundSessions"
  />
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { SplitterGroup, SplitterPanel, SplitterResizeHandle } from 'reka-ui'
  import CloseBackgroundSessionsDrawer from './CloseBackgroundSessionsDrawer.vue'
  import CreateSessionDialog from './CreateSessionDialog.vue'
  import DeleteSessionDialog from './DeleteSessionDialog.vue'
  import EditSessionDialog from './EditSessionDialog.vue'
  import RemoveWorkspaceDialog from './RemoveWorkspaceDialog.vue'
  import SessionWorkbench from './SessionWorkbench.vue'
  import WorkspaceSessionSidebar from '../workspace/WorkspaceSessionSidebar.vue'
  import { terminalDebug } from '../terminal/diagnostics'
  import { useAsyncAction } from '../../composable/useAsyncAction'
  import { useCreateSessionDraft } from '../../composable/useCreateSessionDraft'
  import { useSessionDialogs } from '../../composable/useSessionDialogs'
  import { useTerminalSize } from '../../composable/useTerminalSize'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useCloudDevicesStore } from '../../store/cloudDevices'
  import { useCloudSessionStore } from '../../store/cloudSession'
  import { useRuntimeConfigStore } from '../../store/runtimeConfig'
  import {
    closeSessionsSerially,
    normalizedSessionWorkspaceTree,
    selectedSessionWorkspaceTreeTargets,
    terminalTabTargets,
    type SessionIdentity,
  } from '../../features/sessions/tabManagement'
  import {
    terminalWsUrl,
    type SessionRuntimeApi,
    type ShortcutRuntimeApi,
  } from '../../features/sessions/runtime'
  import type { RuntimeTarget } from '../../features/runtimeTarget'
  import type { CloudSessionSummary } from '../../gen/proto/termbridge/cloud/v1/session'
  import type { DeviceSummary } from '../../gen/proto/termbridge/cloud/v1/device'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'
  import type {
    SessionSummary,
    Workspace as WorkspaceSummary,
  } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { ServerControlMessage } from '../../gen/proto/termbridge/agent/v1/terminal'
  import { useNotificationsStore } from '../../store/notifications'
  import { useWorkbenchStore } from '../../store/workbench'
  import { useWorkspaceSessionsStore } from '../../store/workspaceSessions'

  const props = defineProps<{
    runtimeTarget: RuntimeTarget
    runtimeApi: SessionRuntimeApi & ShortcutRuntimeApi
    homeRouteName: string
    loginRedirect: string
    currentDevice?: DeviceSummary | CloudSessionSummary | null
    logout?: () => Promise<void>
  }>()

  const { t } = useI18n()
  const router = useRouter()
  const cloudAuth = useCloudAuthStore()
  const cloudDevices = useCloudDevicesStore()
  const cloudSession = useCloudSessionStore()
  const runtimeConfig = useRuntimeConfigStore()
  const workspaceSessions = useWorkspaceSessionsStore()
  const workbench = useWorkbenchStore()
  const notifications = useNotificationsStore()
  const dialogs = useSessionDialogs()
  const createDraft = useCreateSessionDraft()

  const terminalWorkbench = ref<HTMLElement | null>(null)
  const { measureInitialTerminalSize } = useTerminalSize(terminalWorkbench)

  const shortcuts = ref<Shortcut[]>([])
  const enabledShortcuts = computed(() =>
    shortcuts.value.filter((shortcut) => shortcut.enabled !== false),
  )
  const stoppingSessionId = ref<string | null>(null)
  const rerunningSessionId = ref<string | null>(null)
  const deletingSessionId = ref<string | null>(null)
  const removingWorkspaceId = ref<string | null>(null)
  const closeBackgroundSessionsDrawerOpen = ref(false)
  const workspaceTreeError = ref<string | null>(null)
  const refreshAction = useAsyncAction({
    onError: (err) => {
      workspaceTreeError.value = t('toast.refreshFailed')
      notifications.notifyError(t('toast.refreshFailed'), err)
    },
  })
  const createAction = useAsyncAction({
    onError: (err) => {
      notifications.notifyError(t('toast.createSessionFailed'), err)
      void refresh()
    },
  })
  const editAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('toast.editSessionFailed'), err),
  })
  const deleteAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('toast.deleteSessionFailed'), err),
  })
  const removeWorkspaceAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('toast.removeWorkspaceFailed'), err),
  })
  const stopAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('toast.stopSessionFailed'), err),
  })
  const rerunAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('toast.rerunSessionFailed'), err),
  })
  const closeBackgroundAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('toast.closeBackgroundSessionsFailed'), err),
  })
  const closingBackgroundSessions = computed(() => closeBackgroundAction.running)
  const isLocalMode = computed(() => props.runtimeTarget.mode === 'local')
  const workbenchDevice = computed(() => props.currentDevice ?? cloudSession.cloudSession)
  const helpHref = computed(() =>
    isLocalMode.value ? new URL('/help', runtimeConfig.config.cloud.publicUrl).toString() : '',
  )

  const activeSession = computed(() => {
    const tab = workbench.activeTab
    return tab ? workspaceSessions.sessionById(tab.workspaceId, tab.sessionId) : null
  })

  const activeWorkspaceForViewSwitch = computed(() => {
    const tab = workbench.activeTab
    if (!tab) return null
    return workspaceSessions.workspaceById(tab.workspaceId)
  })

  const activeTerminalWsUrl = computed(() => {
    const session = activeSession.value
    if (!session) {
      return null
    }
    return terminalWsUrl(
      props.runtimeTarget,
      session.workspace_id,
      session.id,
      props.runtimeTarget.mode === 'cloud' ? (cloudAuth.cloudToken ?? undefined) : undefined,
    )
  })

  const terminalTabs = computed(() =>
    terminalTabTargets(workbench.openedTabs, workspaceSessions.sessionById),
  )
  const sessionWorkspaceTree = computed(() =>
    normalizedSessionWorkspaceTree(workspaceSessions.workspaceTree),
  )
  const hasRunningSessions = computed(() =>
    sessionWorkspaceTree.value.some((workspace) =>
      workspace.children.some((session) => session.lifecycle_state === 'running'),
    ),
  )

  async function refresh() {
    workspaceTreeError.value = null
    await refreshAction.run(async () => {
      await workspaceSessions.refresh(props.runtimeTarget, props.runtimeApi)
      await notifyHistoryError(
        await workbench.ensureActiveHistoryLoaded(
          props.runtimeTarget,
          props.runtimeApi,
          workspaceSessions.sessionById,
        ),
      )
    })
  }

  async function openWorkspaceFiles(workspace: WorkspaceSummary) {
    if (props.runtimeTarget.mode === 'local') {
      await router.push({
        name: 'local-workspace-code',
        params: { workspaceId: workspace.id },
      })
      return
    }
    await router.push({
      name: 'cloud-workspace-code',
      params: {
        deviceId: props.runtimeTarget.deviceId,
        workspaceId: workspace.id,
      },
    })
  }

  async function openWorkspaceGit(workspace: WorkspaceSummary) {
    if (props.runtimeTarget.mode === 'local') {
      await router.push({
        name: 'local-workspace-code',
        params: { workspaceId: workspace.id },
      })
      return
    }
    await router.push({
      name: 'cloud-workspace-code',
      params: {
        deviceId: props.runtimeTarget.deviceId,
        workspaceId: workspace.id,
      },
    })
  }

  async function openDashboard() {
    await router.push({ name: props.homeRouteName })
  }

  async function openShortcuts() {
    await router.push(
      props.runtimeTarget.mode === 'local'
        ? { name: 'local-shortcuts' }
        : { name: 'cloud-shortcuts', params: { deviceId: props.runtimeTarget.deviceId } },
    )
  }

  function selectCreateShortcut(shortcutId: string | null) {
    createDraft.selectShortcut(enabledShortcuts.value.find((value) => value.id === shortcutId))
  }

  function openEditSessionDialog(session: SessionSummary) {
    if (!isEditableLifecycle(session)) {
      return
    }
    dialogs.openEditDialog(session)
  }

  async function refreshShortcuts() {
    shortcuts.value = await props.runtimeApi.listShortcuts()
  }

  async function handleLogout() {
    if (props.logout) {
      try {
        await props.logout()
      } catch {
        // Clear local state even if Cloud logout fails.
      }
    }
    if (isLocalMode.value) {
      cloudSession.reset()
    } else {
      cloudAuth.clearToken()
    }
    workspaceSessions.reset()
    dialogs.createSessionDialogOpen = false
    dialogs.editDialogOpen = false
    dialogs.clearSelectedSession()
    dialogs.removeWorkspaceDialogOpen = false
    dialogs.clearSelectedWorkspace()
    closeBackgroundSessionsDrawerOpen.value = false
    workbench.resetForSourceChange()
    await router.replace(
      isLocalMode.value ? { name: props.homeRouteName } : { name: 'cloud-login' },
    )
  }

  async function startSession() {
    if (createAction.running) {
      return
    }
    const draft = createDraft.validate()
    if (draft.error) {
      notifyCreateSessionValidationError(draft.error)
      return
    }
    await createAction.run(async () => {
      const size = measureInitialTerminalSize()
      terminalDebug('session.create.request', {
        name: draft.value.name,
        cwd: draft.value.cwd,
        commandLength: draft.value.commandText.length,
        cols: size.cols,
        rows: size.rows,
      })
      const created = await props.runtimeApi.createSession(draft.value.workspaceId, {
        workspace_id: draft.value.workspaceId ?? '',
        name: draft.value.name,
        cwd: draft.value.cwd,
        command: [draft.value.commandText],
        cols: size.cols,
        rows: size.rows,
        command_source: draft.value.commandSource,
        shortcut_id_snapshot: draft.value.shortcutIdSnapshot,
        shortcut_name_snapshot: draft.value.shortcutNameSnapshot,
      })
      terminalDebug('session.create.response', {
        sessionId: created.session_id,
        workspaceId: created.workspace_id,
        state: created.state,
      })
      const session = await props.runtimeApi.getSession(created.workspace_id, created.session_id)
      terminalDebug('session.create.summary', {
        sessionId: session.id,
        lifecycleState: session.lifecycle_state,
        attachmentState: session.attachment_state,
        exitCode: session.exit_code,
      })
      if (!workspaceSessions.upsertSession(session)) {
        await refresh()
      }
      createDraft.reset(undefined, t('dialog.defaultSessionName'), enabledShortcuts.value)
      dialogs.createSessionDialogOpen = false
      await openSessionTab(session)
      notifications.pushToast('success', t('toast.sessionCreated'), session.name)
    })
  }

  async function editSelectedSession(payload: { name: string; command: string }) {
    const session = dialogs.selectedSession
    if (!session || !isEditableLifecycle(session) || editAction.running) {
      return
    }
    if (!payload.name) {
      notifications.pushToast('error', t('toast.editSessionFailed'), t('message.nameRequired'))
      return
    }
    if (!payload.command.trim()) {
      notifications.pushToast('error', t('toast.editSessionFailed'), t('message.commandRequired'))
      return
    }
    await editAction.run(async () => {
      const updated = await props.runtimeApi.updateSession(
        session.workspace_id,
        session.id,
        payload,
      )
      workspaceSessions.updateSession(updated)
      dialogs.selectedSession = updated
      dialogs.editDialogOpen = false
      notifications.pushToast('success', t('toast.sessionEdited'), payload.name)
    })
  }

  async function deleteSelectedSession() {
    const session = dialogs.selectedSession
    if (!session || deletingSessionId.value || deleteAction.running || isActiveLifecycle(session)) {
      return
    }
    deletingSessionId.value = session.id
    try {
      await deleteAction.run(
        async () => {
          await props.runtimeApi.deleteSession(session.workspace_id, session.id)
          workspaceSessions.removeSession(session.workspace_id, session.id)
          const nextSession = workbench.closeTab(
            session.workspace_id,
            session.id,
            workspaceSessions.sessionById,
          )
          if (nextSession) {
            await notifyHistoryError(
              await workbench.ensureHistoryLoaded(
                props.runtimeTarget,
                props.runtimeApi,
                nextSession,
              ),
            )
          }
          dialogs.clearSelectedSession()
          dialogs.deleteSessionDialogOpen = false
          notifications.pushToast('success', t('toast.sessionDeleted'), session.name)
        },
        { rethrow: true },
      )
    } catch {
      // notify handled by deleteAction.onError
    } finally {
      deletingSessionId.value = null
    }
  }

  async function removeSelectedWorkspace() {
    const workspace = dialogs.selectedWorkspace
    if (!workspace || removingWorkspaceId.value || removeWorkspaceAction.running) {
      return
    }
    removingWorkspaceId.value = workspace.id
    try {
      await removeWorkspaceAction.run(
        async () => {
          await props.runtimeApi.deleteWorkspace(workspace.id)
          const removedSessions = workspaceSessions.removeWorkspace(workspace.id)
          workbench.closeRemovedSessions(removedSessions)
          dialogs.clearSelectedWorkspace()
          dialogs.removeWorkspaceDialogOpen = false
          notifications.pushToast(
            'success',
            t('toast.workspaceRemoved'),
            t('message.workspaceRemoved', { name: workspace.name }),
          )
        },
        { rethrow: true },
      )
    } catch {
      // notify handled by removeWorkspaceAction.onError
    } finally {
      removingWorkspaceId.value = null
    }
  }

  async function reorderWorkspaces(workspaceIds: string[]) {
    try {
      await workspaceSessions.reorderWorkspaces(props.runtimeTarget, props.runtimeApi, workspaceIds)
    } catch (err) {
      notifications.notifyError(t('toast.updateWorkspaceOrderFailed'), err)
      await refresh()
    }
  }

  async function reorderSessions(workspaceId: string, sessionIds: string[]) {
    try {
      await workspaceSessions.reorderSessions(
        props.runtimeTarget,
        props.runtimeApi,
        workspaceId,
        sessionIds,
      )
    } catch (err) {
      notifications.notifyError(t('toast.updateSessionOrderFailed'), err)
      await refresh()
    }
  }

  async function openSessionTab(session: SessionSummary) {
    await notifyHistoryError(
      await workbench.openSession(props.runtimeTarget, props.runtimeApi, session),
    )
  }

  async function activateOpenedTab(sessionId: string) {
    await notifyHistoryError(
      await workbench.activateSession(
        props.runtimeTarget,
        props.runtimeApi,
        sessionId,
        workspaceSessions.sessionById,
      ),
    )
  }

  async function closeTab(_workspaceId: string, sessionId: string) {
    await closeTabs([sessionId])
  }

  async function closeTerminalTabs() {
    await closeTabs(terminalTabs.value.map((tab) => tab.sessionId))
  }

  function openCloseBackgroundSessionsDrawer() {
    if (!hasRunningSessions.value || closingBackgroundSessions.value) {
      return
    }
    closeBackgroundSessionsDrawerOpen.value = true
  }

  async function closeBackgroundSessions(selectedSessions: SessionIdentity[]) {
    if (closeBackgroundAction.running) {
      return
    }

    const sessions = selectedSessionWorkspaceTreeTargets(
      sessionWorkspaceTree.value,
      selectedSessions,
    )
    if (sessions.length === 0) {
      return
    }

    let completed = false
    try {
      await closeBackgroundAction.run(
        async () => {
          await closeSessionsSerially(
            sessions,
            props.runtimeApi.closeSession,
            workspaceSessions.updateSession,
          )
          completed = true
        },
        { rethrow: true },
      )
    } catch {
      // notify handled by closeBackgroundAction.onError
    } finally {
      await refresh()
      if (completed) {
        closeBackgroundSessionsDrawerOpen.value = false
      }
    }
  }

  async function closeTabs(sessionIds: string[]) {
    const nextSession = workbench.closeTabs(sessionIds, workspaceSessions.sessionById)
    if (nextSession) {
      await notifyHistoryError(
        await workbench.ensureHistoryLoaded(props.runtimeTarget, props.runtimeApi, nextSession),
      )
    }
  }

  function openCreateSessionForm(workspace?: WorkspaceSummary) {
    terminalDebug('session.form.open', {
      workspaceId: workspace?.id,
      workspacePath: workspace?.path,
      activeSessionId: activeSession.value?.id ?? null,
    })
    createDraft.reset(workspace, t('dialog.defaultSessionName'), enabledShortcuts.value)
    dialogs.openCreateSessionDialog()
  }

  function openCopiedSessionForm(session: SessionSummary) {
    const workspace = workspaceSessions.workspaceById(session.workspace_id)
    if (!workspace) {
      return
    }
    terminalDebug('session.form.copy', {
      sessionId: session.id,
      workspaceId: workspace.id,
    })
    createDraft.populateFromSession(
      session,
      workspace,
      workspaceSessions.sessions.map((existingSession) => existingSession.name),
      enabledShortcuts.value,
    )
    dialogs.openCreateSessionDialog()
  }

  async function stopSessionFromSidebar(session: SessionSummary) {
    if (!isStoppableLifecycle(session) || stoppingSessionId.value || stopAction.running) {
      return
    }
    stoppingSessionId.value = session.id
    try {
      await stopAction.run(
        async () => {
          const updated = await props.runtimeApi.closeSession(session.workspace_id, session.id)
          workspaceSessions.updateSession(updated)
          await notifyHistoryError(
            await workbench.ensureHistoryLoaded(props.runtimeTarget, props.runtimeApi, updated),
          )
        },
        { rethrow: true },
      )
    } catch {
      // notify handled by stopAction.onError
    } finally {
      stoppingSessionId.value = null
    }
  }

  async function rerunSessionFromSidebar(session: SessionSummary) {
    if (!canRerunLifecycle(session) || rerunningSessionId.value || rerunAction.running) {
      return
    }
    rerunningSessionId.value = session.id
    try {
      await rerunAction.run(
        async () => {
          const size = measureInitialTerminalSize()
          const response = await props.runtimeApi.rerunSession(
            session.workspace_id,
            session.id,
            size,
          )
          const updated = await props.runtimeApi.getSession(
            response.workspace_id,
            response.session_id,
          )
          workspaceSessions.updateSession(updated)
          workbench.resetTabHistory(updated.workspace_id, updated.id)
          await openSessionTab(updated)
          notifications.pushToast('success', t('toast.sessionRerun'), updated.name)
        },
        {
          rethrow: true,
          onError: (err) => {
            notifications.notifyError(t('toast.rerunSessionFailed'), err)
            void refreshSessionAfterRerunFailure(session)
          },
        },
      )
    } catch {
      // notify/refresh handled by rerunAction onError
    } finally {
      rerunningSessionId.value = null
    }
  }

  async function refreshSessionAfterRerunFailure(session: SessionSummary) {
    workbench.resetTabHistory(session.workspace_id, session.id)
    try {
      const updated = await props.runtimeApi.getSession(session.workspace_id, session.id)
      workspaceSessions.updateSession(updated)
      await notifyHistoryError(
        await workbench.ensureHistoryLoaded(props.runtimeTarget, props.runtimeApi, updated),
      )
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

  function sessionLifecycleState(workspaceId: string, sessionId: string) {
    return workspaceSessions.sessionById(workspaceId, sessionId)?.lifecycle_state ?? ''
  }

  function sessionCommandSource(workspaceId: string, sessionId: string) {
    return workspaceSessions.sessionById(workspaceId, sessionId)?.command_source ?? ''
  }

  function sessionSourceLabel(workspaceId: string, sessionId: string) {
    return sessionCommandSource(workspaceId, sessionId) === 'shortcut'
      ? t('dialog.shortcut')
      : t('workbench.launchCommand')
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

  function isEditableLifecycle(session: SessionSummary) {
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
    if (!session) {
      return
    }
    try {
      const updated = await props.runtimeApi.getSession(session.workspace_id, session.id)
      workspaceSessions.updateSession(updated)
      await notifyHistoryError(
        await workbench.ensureHistoryLoaded(props.runtimeTarget, props.runtimeApi, updated),
      )
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
      if (props.runtimeTarget.mode === 'cloud') {
        await cloudDevices.loadDevices()
      }
      await Promise.all([refresh(), refreshShortcuts()])
    } catch (err) {
      notifications.notifyError(t('toast.refreshFailed'), err)
    }
  })
</script>


