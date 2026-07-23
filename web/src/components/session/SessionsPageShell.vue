<template>
  <div
    v-if="isLocalMode || cloudAuth.authenticated"
    class="sessions-shell flex h-full overflow-hidden bg-[var(--color-app-bg)] text-sm text-[var(--color-text)]"
    :class="{ 'sessions-shell-narrow': isNarrow }"
  >
    <!--
      Single SessionWorkbench host: isNarrow only changes sidebar chrome.
      Avoid v-if/v-else remount of workbench on orientation/breakpoint flips
      (that closed the terminal WS and dropped multi-attach control).
    -->
    <SplitterGroup direction="horizontal" class="flex min-h-0 min-w-0 flex-1">
      <SplitterPanel
        v-if="!isNarrow && !sidebarCollapsed"
        id="workspace-sidebar"
        class="workspace-sidebar-panel"
        :default-size="18"
        :min-size="12"
        :max-size="24"
      >
        <WorkspaceSessionSidebar
          v-bind="sidebarBind"
          @collapse="collapseSidebar"
          @select="openSessionTab"
          v-on="sidebarListeners"
        />
      </SplitterPanel>

      <SplitterResizeHandle
        v-if="!isNarrow && !sidebarCollapsed"
        class="sessions-resize-handle group flex w-1 shrink-0 cursor-col-resize items-stretch justify-center bg-[var(--color-app-bg)] outline-none"
      >
        <span
          class="w-px bg-[var(--color-border)] transition group-hover:bg-[var(--color-border-strong)]"
        />
      </SplitterResizeHandle>

      <SplitterPanel id="terminal-workbench" :min-size="isNarrow || sidebarCollapsed ? 100 : 55">
        <div class="relative flex h-full min-h-0 min-w-0 flex-1 flex-col">
          <SessionWorkbench
            v-bind="workbenchBind"
            class="session-workbench-stage h-full min-h-0 min-w-0"
            :show-sidebar-toggle="sidebarCollapsed"
            @toggle-sidebar="expandSidebar"
            v-on="workbenchListeners"
          />
        </div>
      </SplitterPanel>
    </SplitterGroup>

    <template v-if="isNarrow">
      <div
        v-if="!sidebarCollapsed"
        class="fixed inset-0 z-[35] bg-black/45"
        aria-hidden="true"
        @click="collapseSidebar"
      />
      <div
        class="sessions-mobile-sidebar fixed inset-x-auto top-[env(safe-area-inset-top,0px)] bottom-[env(safe-area-inset-bottom,0px)] left-0 z-40 flex w-[min(320px,88vw)] flex-col border-r border-[var(--color-border)] bg-[var(--color-sidebar-bg)] shadow-[12px_0_32px_rgb(0_0_0/0.28)] [&>*]:h-full [&>*]:min-h-0"
        :data-state="sidebarCollapsed ? 'closed' : 'open'"
        :aria-hidden="sidebarCollapsed ? 'true' : 'false'"
      >
        <WorkspaceSessionSidebar
          v-bind="sidebarBind"
          @collapse="collapseSidebar"
          @select="selectSessionFromMobileSidebar"
          v-on="sidebarListeners"
        />
      </div>
    </template>
  </div>

  <CreateSessionDialog
    :open="dialogs.createSessionDialogOpen"
    :cwd="createDraft.cwd"
    :cwd-hidden="!!createDraft.workspace"
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
  import { computed, onMounted, ref, watch } from 'vue'
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
  import { useLocalCloudConnection } from '../../composable/useLocalCloudConnection'
  import { useSessionDialogs } from '../../composable/useSessionDialogs'
  import { useTerminalSize } from '../../composable/useTerminalSize'
  import { useSessionsLayoutMode } from '../../composable/useSessionsLayoutMode'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useCloudDevicesStore } from '../../store/cloudDevices'
  import { useCloudSessionStore } from '../../store/cloudSession'
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
  }>()

  const { t } = useI18n()
  const router = useRouter()
  const cloudAuth = useCloudAuthStore()
  const cloudDevices = useCloudDevicesStore()
  const cloudSession = useCloudSessionStore()
  const localCloud = useLocalCloudConnection()
  const workspaceSessions = useWorkspaceSessionsStore()
  const workbench = useWorkbenchStore()
  const notifications = useNotificationsStore()
  const dialogs = useSessionDialogs()
  const createDraft = useCreateSessionDraft()

  const terminalWorkbench = ref<HTMLElement | null>(null)
  const { measureInitialTerminalSize } = useTerminalSize(terminalWorkbench)
  const { isNarrow, disableReorder } = useSessionsLayoutMode()
  // Narrow starts collapsed (overlay closed); wide starts expanded.
  const sidebarCollapsed = ref(false)

  function collapseSidebar() {
    sidebarCollapsed.value = true
  }

  function expandSidebar() {
    sidebarCollapsed.value = false
  }

  function selectSessionFromMobileSidebar(session: SessionSummary) {
    void openSessionTab(session)
    collapseSidebar()
  }

  watch(
    isNarrow,
    (narrow, wasNarrow) => {
      if (narrow === wasNarrow) return
      // Entering narrow: close overlay. Leaving narrow: restore side panel.
      sidebarCollapsed.value = narrow
    },
    { immediate: true },
  )

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

  const activeSession = computed(() => {
    const tab = workbench.activeTab
    return tab ? workspaceSessions.sessionById(tab.workspaceId, tab.sessionId) : null
  })

  const activeWorkspaceForViewSwitch = computed(() => {
    const tab = workbench.activeTab
    if (!tab) return null
    return workspaceSessions.workspaceById(tab.workspaceId)
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

  function openWorkspaceFiles(workspace: WorkspaceSummary) {
    const route =
      props.runtimeTarget.mode === 'local'
        ? {
            name: 'local-workspace-code',
            params: { workspaceId: workspace.id },
          }
        : {
            name: 'cloud-workspace-code',
            params: {
              deviceId: props.runtimeTarget.deviceId,
              workspaceId: workspace.id,
            },
          }
    const { href } = router.resolve(route)
    window.open(href, '_blank', 'noopener,noreferrer')
  }

  const shortcutsRoute = computed(() =>
    props.runtimeTarget.mode === 'local'
      ? { name: 'local-shortcuts' }
      : { name: 'cloud-shortcuts', params: { deviceId: props.runtimeTarget.deviceId } },
  )

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

  async function editSelectedSession(payload: {
    name: string
    command?: string
    command_source?: string
    shortcut_id_snapshot?: string
    shortcut_name_snapshot?: string
  }) {
    const session = dialogs.selectedSession
    if (!session || !isEditableLifecycle(session) || editAction.running) {
      return
    }
    if (!payload.name) {
      notifications.pushToast('error', t('toast.editSessionFailed'), t('message.nameRequired'))
      return
    }
    const nameOnly = isActiveLifecycle(session)
    if (!nameOnly && !payload.command?.trim()) {
      notifications.pushToast('error', t('toast.editSessionFailed'), t('message.commandRequired'))
      return
    }
    await editAction.run(async () => {
      const request = nameOnly
        ? { name: payload.name }
        : {
            name: payload.name,
            command: payload.command,
            ...(payload.command_source !== undefined
              ? {
                  command_source: payload.command_source,
                  shortcut_id_snapshot: payload.shortcut_id_snapshot,
                  shortcut_name_snapshot: payload.shortcut_name_snapshot,
                }
              : {}),
          }
      const updated = await props.runtimeApi.updateSession(
        session.workspace_id,
        session.id,
        request,
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
          // Keep terminal strip in sync: closed running sessions drop their open tabs.
          await closeTabs(sessions.map((session) => session.id))
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
      workspaceSessions.sessions
        .filter((existingSession) => existingSession.workspace_id === session.workspace_id)
        .map((existingSession) => existingSession.name),
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
          // Closing a running session from the sidebar should drop its terminal tab.
          await closeTabs([session.id])
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
    return ['running', 'stopped', 'failed'].includes(session.lifecycle_state)
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

  const terminalErrorToastAt = new Map<string, number>()
  function handleTerminalError(message: string) {
    const sessionKey = workbench.activeSessionId ?? ''
    const key = sessionKey + '|' + message
    const now = Date.now()
    const prev = terminalErrorToastAt.get(key) ?? 0
    if (now - prev < 10000) {
      return
    }
    terminalErrorToastAt.set(key, now)
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

  const sidebarBind = computed(() => ({
    workspaceTree: workspaceSessions.workspaceTree,
    activeSessionId: workbench.activeSessionId,
    activeWorkspace: activeWorkspaceForViewSwitch.value,
    stoppingSessionId: stoppingSessionId.value,
    rerunningSessionId: rerunningSessionId.value,
    deletingSessionId: deletingSessionId.value,
    removingWorkspaceId: removingWorkspaceId.value,
    loading: workspaceSessions.loading,
    loadError: workspaceTreeError.value,
    homeRouteName: props.homeRouteName,
    disableReorder: disableReorder.value,
  }))

  const sidebarListeners = {
    refresh,
    newSession: openCreateSessionForm,
    copySession: openCopiedSessionForm,
    editSession: openEditSessionDialog,
    stopSession: stopSessionFromSidebar,
    rerunSession: rerunSessionFromSidebar,
    deleteSession: openDeleteSessionDialog,
    removeWorkspace: dialogs.openRemoveWorkspaceDialog,
    openFiles: openWorkspaceFiles,
    unsupportedDirectoryDelete: explainUnsupportedDirectoryDelete,
    reorderWorkspaces,
    reorderSessions,
  }

  function resolveSession(workspaceId: string, sessionId: string) {
    return workspaceSessions.sessionById(workspaceId, sessionId)
  }

  function resolveTerminalWsUrl(workspaceId: string, sessionId: string) {
    return terminalWsUrl(
      props.runtimeTarget,
      workspaceId,
      sessionId,
      props.runtimeTarget.mode === 'cloud' ? (cloudAuth.cloudToken ?? undefined) : undefined,
    )
  }

  const workbenchBind = computed(() => ({
    openedTabs: workbench.openedTabs,
    activeSessionId: workbench.activeSessionId,
    activeTab: workbench.activeTab,
    activeSession: activeSession.value,
    currentDevice: workbenchDevice.value,
    resolveSession,
    resolveWsUrl: resolveTerminalWsUrl,
    loading: workspaceSessions.loading && workbench.openedTabs.length === 0,
    hasTerminalTabs: terminalTabs.value.length > 0,
    hasBackgroundRunningSessions: hasRunningSessions.value,
    sessionTitle,
    sessionLifecycleState,
    disableTabReorder: disableReorder.value,
    showCloudConnection: isLocalMode.value,
    shortcutsRoute: shortcutsRoute.value,
  }))

  const workbenchListeners = {
    activateTab: activateOpenedTab,
    closeTab,
    closeTerminalTabs,
    openCloseBackgroundSessionsDrawer,
    reorderTabs: (tabs: typeof workbench.openedTabs) => {
      workbench.openedTabs = tabs
    },
    openCreate: () => openCreateSessionForm(),
    workbench: (element: HTMLElement | null) => {
      terminalWorkbench.value = element
    },
    terminalState: handleTerminalState,
    terminalError: handleTerminalError,
  }

  onMounted(async () => {
    try {
      if (isLocalMode.value) {
        try {
          await localCloud.hydrateFromAgent()
        } catch (err) {
          notifications.notifyError(t('dashboard.cloudConnectionFailed'), err)
        }
      } else {
        await cloudDevices.loadDevices()
      }
      await Promise.all([refresh(), refreshShortcuts()])
    } catch (err) {
      notifications.notifyError(t('toast.refreshFailed'), err)
    }
  })
</script>
