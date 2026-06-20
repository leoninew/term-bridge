<template>
  <ToastProvider>
    <SplitterGroup direction="horizontal" class="flex h-screen min-h-screen overflow-hidden bg-[#05070d] text-sm text-slate-200">
      <SplitterPanel id="workspace-sidebar" :default-size="22" :min-size="16" :max-size="35">
        <WorkspaceSessionSidebar
          :workspace-tree="workspaceTree"
          :active-session-id="activeTabId"
          @select="openSessionTab"
          @refresh="refresh"
          @new-session="openCreateDialog"
          @rename-session="openRenameDialog"
          @delete-session="openDeleteSessionDialog"
          @remove-workspace="openRemoveWorkspaceDialog"
          @unsupported-directory-delete="explainUnsupportedDirectoryDelete"
          @reorder-workspaces="reorderWorkspaces"
        />
      </SplitterPanel>

      <SplitterResizeHandle class="group flex w-1 shrink-0 cursor-col-resize items-stretch justify-center bg-[#05070d] outline-none">
        <span class="w-px bg-slate-800 transition group-hover:bg-slate-700" />
      </SplitterResizeHandle>

      <SplitterPanel id="terminal-workbench" :min-size="55">
        <section class="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-[#090d14]">
        <p v-if="error" class="mx-3 mt-3 rounded-md border border-slate-800 bg-slate-950/80 px-2 py-1.5 text-red-100">
          {{ error }}
        </p>

        <TabsRoot
          :model-value="activeTabId ?? undefined"
          class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
          @update:model-value="activateOpenedTab"
        >
          <div class="flex h-11 shrink-0 items-center border-b border-slate-800/80 bg-[#0a0f18] px-2">
            <TabsList as-child>
              <VueDraggable
                v-model="openedTabs"
                tag="div"
                class="flex min-w-0 flex-1 gap-1.5 overflow-x-auto"
                :animation="150"
                handle=".tab-drag-handle"
                item-key="sessionId"
              >
                <div
                  v-for="tab in openedTabs"
                  :key="tab.sessionId"
                  class="group relative flex max-w-56 shrink-0 items-center rounded-md border px-0.5 text-sm transition"
                  :class="activeTabId === tab.sessionId
                    ? 'border-slate-700 bg-slate-900 text-slate-50'
                    : 'border-slate-800 bg-slate-950/70 text-slate-400 hover:border-slate-700 hover:bg-slate-900/80'"
                >
                  <TabsTrigger :value="tab.sessionId" class="tab-drag-handle flex min-w-0 items-center gap-1.5 rounded-md px-2 py-1.5 outline-none">
                    <SquareTerminal class="size-4 shrink-0 text-slate-500" aria-hidden="true" />
                    <span class="truncate">{{ sessionTitle(tab.sessionId) }}</span>
                  </TabsTrigger>
                  <button
                    type="button"
                    class="rounded-md p-1 text-slate-500 hover:bg-slate-800 hover:text-slate-200"
                    :aria-label="t('workbench.closeTabAria', { name: sessionTitle(tab.sessionId) })"
                    @click.stop="closeTab(tab.sessionId)"
                  >
                    <X class="size-3.5" />
                  </button>
                </div>
              </VueDraggable>
            </TabsList>
          </div>

          <TabsContent
            v-if="activeSession && activeTab"
            :key="activeSession.id"
            :value="activeSession.id"
            class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-[#090d14] p-2"
          >
            <TerminalView
              v-if="activeSession.lifecycle_state === 'running'"
              :key="activeSession.id"
              :ws-url="`/api/sessions/${activeSession.id}/ws`"
              :session-id="activeSession.id"
              @state="handleTerminalState"
              @terminal-error="handleTerminalError"
            />

            <section v-else class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
              <div v-if="activeTab.historyLoading" class="flex flex-1 items-center justify-center text-slate-500">{{ t('workbench.loadingHistory') }}</div>
              <div v-else-if="activeTab.historyError" class="rounded-md border border-slate-800 bg-slate-950/80 px-2 py-1.5 text-red-100">
                {{ activeTab.historyError }}
              </div>
              <HistoryTerminalView
                v-else-if="activeTab.historyText"
                :key="`${activeSession.id}-history`"
                :history="activeTab.historyText"
              />
              <div v-else class="flex flex-1 items-center justify-center text-slate-500">{{ t('workbench.noHistory') }}</div>
            </section>
          </TabsContent>

          <section v-if="openedTabs.length === 0" class="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 p-6 text-center text-slate-500">
            <h3 class="text-lg font-semibold text-slate-300">{{ t('workbench.noTabTitle') }}</h3>
            <p>{{ t('workbench.noTabDescription') }}</p>
            <button type="button" class="rounded-md border border-slate-700 bg-slate-900 px-2.5 py-1.5 text-slate-100 hover:bg-slate-800" @click="openCreateDialog">
              {{ t('workbench.newSession') }}
            </button>
          </section>
        </TabsRoot>

        <footer class="flex h-8 shrink-0 items-center gap-1.5 overflow-hidden border-t border-slate-800/80 bg-[#0a0f18] px-3 text-sm text-slate-500">
          <template v-if="activeSession">
            <span>{{ t('workbench.status') }}</span>
            <span class="text-slate-200">{{ activeLifecycleLabel }}</span>
            <span class="text-slate-700">·</span>
            <span>{{ t('workbench.command') }}</span>
            <span class="min-w-0 truncate text-slate-200">{{ activeSession.command }}</span>
          </template>
          <span v-else>{{ t('workbench.noActiveSession') }}</span>
        </footer>
        </section>
      </SplitterPanel>
    </SplitterGroup>

    <DialogRoot v-model:open="createDialogOpen">
      <DialogPortal>
        <DialogOverlay class="dialog-overlay" />
        <DialogContent class="dialog-content">
          <DialogTitle class="dialog-title">{{ t('dialog.newSessionTitle') }}</DialogTitle>
          <DialogDescription class="dialog-description">
            {{ t('dialog.newSessionDescription') }}
          </DialogDescription>
          <form class="dialog-form" @submit.prevent="startSession">
            <label>
              <span>{{ t('dialog.name') }}</span>
              <input ref="sessionNameInput" v-model="sessionName" :placeholder="t('dialog.sessionNamePlaceholder')" />
            </label>
            <label>
              <span>{{ t('dialog.cwd') }}</span>
              <input v-model="cwd" :placeholder="t('dialog.workingDirectoryPlaceholder')" />
            </label>
            <label>
              <span>{{ t('dialog.command') }}</span>
              <input v-model="commandText" :placeholder="t('dialog.commandPlaceholder')" />
            </label>
            <div class="dialog-actions">
              <DialogClose as-child>
                <button type="button" class="button button-secondary">{{ t('common.cancel') }}</button>
              </DialogClose>
              <button type="submit" class="button button-primary" :disabled="creatingSession">
                {{ creatingSession ? t('common.creating') : t('common.create') }}
              </button>
            </div>
          </form>
        </DialogContent>
      </DialogPortal>
    </DialogRoot>

    <DialogRoot v-model:open="renameDialogOpen">
      <DialogPortal>
        <DialogOverlay class="dialog-overlay" />
        <DialogContent class="dialog-content">
          <DialogTitle class="dialog-title">{{ t('dialog.renameSessionTitle') }}</DialogTitle>
          <DialogDescription class="dialog-description">
            {{ t('dialog.renameSessionDescription') }}
          </DialogDescription>
          <form class="dialog-form" @submit.prevent="renameSelectedSession">
            <label>
              <span>{{ t('dialog.name') }}</span>
              <input ref="renameInput" v-model="renameText" :placeholder="t('dialog.sessionNamePlaceholder')" />
            </label>
            <div class="dialog-actions">
              <DialogClose as-child>
                <button type="button" class="button button-secondary">{{ t('common.cancel') }}</button>
              </DialogClose>
              <button type="submit" class="button button-primary" :disabled="renamingSession">
                {{ renamingSession ? t('common.renaming') : t('common.rename') }}
              </button>
            </div>
          </form>
        </DialogContent>
      </DialogPortal>
    </DialogRoot>

    <AlertDialogRoot v-model:open="deleteSessionDialogOpen">
      <AlertDialogPortal>
        <AlertDialogOverlay class="dialog-overlay" />
        <AlertDialogContent class="dialog-content">
          <AlertDialogTitle class="dialog-title">{{ t('dialog.deleteSessionTitle') }}</AlertDialogTitle>
          <AlertDialogDescription class="dialog-description">
            <template v-if="selectedSession && isActiveLifecycle(selectedSession)">
              {{ t('dialog.deleteActiveSessionDescription') }}
            </template>
            <template v-else>
              {{ t('dialog.deleteSessionDescription', { name: selectedSession ? sessionDisplayName(selectedSession) : t('dialog.fallbackSession') }) }}
            </template>
          </AlertDialogDescription>
          <div class="dialog-actions">
            <AlertDialogCancel as-child>
              <button type="button" class="button button-secondary">{{ t('common.cancel') }}</button>
            </AlertDialogCancel>
            <AlertDialogAction as-child>
              <button
                type="button"
                class="button button-danger"
                :disabled="!selectedSession || isActiveLifecycle(selectedSession) || deletingSession"
                @click="deleteSelectedSession"
              >
                {{ deletingSession ? t('common.deleting') : t('common.delete') }}
              </button>
            </AlertDialogAction>
          </div>
        </AlertDialogContent>
      </AlertDialogPortal>
    </AlertDialogRoot>

    <AlertDialogRoot v-model:open="removeWorkspaceDialogOpen">
      <AlertDialogPortal>
        <AlertDialogOverlay class="dialog-overlay" />
        <AlertDialogContent class="dialog-content">
          <AlertDialogTitle class="dialog-title">{{ t('dialog.removeWorkspaceTitle') }}</AlertDialogTitle>
          <AlertDialogDescription class="dialog-description">
            {{ t('dialog.removeWorkspaceDescription') }}
          </AlertDialogDescription>
          <div v-if="selectedWorkspace" class="dialog-note">
            {{ selectedWorkspace.name }} — {{ selectedWorkspace.path }}
          </div>
          <div class="dialog-actions">
            <AlertDialogCancel as-child>
              <button type="button" class="button button-secondary">{{ t('common.cancel') }}</button>
            </AlertDialogCancel>
            <AlertDialogAction as-child>
              <button
                type="button"
                class="button button-danger"
                :disabled="!selectedWorkspace || removingWorkspace"
                @click="removeSelectedWorkspace"
              >
                {{ removingWorkspace ? t('common.removing') : t('common.removeWorkspace') }}
              </button>
            </AlertDialogAction>
          </div>
        </AlertDialogContent>
      </AlertDialogPortal>
    </AlertDialogRoot>

    <ToastRoot
      v-for="toast in toasts"
      :key="toast.id"
      class="toast-root"
      :class="`toast-${toast.kind}`"
      :duration="5000"
      @update:open="(open) => !open && dismissToast(toast.id)"
    >
      <ToastTitle class="toast-title">{{ toast.title }}</ToastTitle>
      <ToastDescription v-if="toast.description" class="toast-description">
        {{ toast.description }}
      </ToastDescription>
      <ToastClose class="toast-close">×</ToastClose>
    </ToastRoot>
    <ToastViewport class="toast-viewport" />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { computed, nextTick, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { SquareTerminal, X } from '@lucide/vue'
  import { VueDraggable } from 'vue-draggable-plus'
  import {
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogOverlay,
    AlertDialogPortal,
    AlertDialogRoot,
    AlertDialogTitle,
    DialogClose,
    DialogContent,
    DialogDescription,
    DialogOverlay,
    DialogPortal,
    DialogRoot,
    DialogTitle,
    SplitterGroup,
    SplitterPanel,
    SplitterResizeHandle,
    TabsContent,
    TabsList,
    TabsRoot,
    TabsTrigger,
    ToastClose,
    ToastDescription,
    ToastProvider,
    ToastRoot,
    ToastTitle,
    ToastViewport,
  } from 'reka-ui'
  import HistoryTerminalView from './components/terminal/HistoryTerminalView.vue'
  import TerminalView from './components/terminal/TerminalView.vue'
  import WorkspaceSessionSidebar from './components/workspace/WorkspaceSessionSidebar.vue'
  import type {
    ServerControlMessage,
    SessionSummary,
    WorkspaceSummary,
    WorkspaceTreeSummary,
  } from './protocol/terminal'
  import { commandFromText } from './protocol/terminal'
  import {
    createSession,
    deleteSession,
    readHistory,
    updateSession,
  } from './features/sessions/api'
  import { deleteWorkspace, listWorkspaceTree, updateWorkspaceOrder } from './features/workspaces/api'

  type OpenSessionTab = {
    sessionId: string
    historyText: string
    historyLoaded: boolean
    historyLoading: boolean
    historyError: string | null
  }

  type ToastKind = 'success' | 'error' | 'info'
  type AppToast = { id: number; kind: ToastKind; title: string; description?: string }

  const { t } = useI18n()
  const workspaceTree = ref<WorkspaceTreeSummary[]>([])
  const workspaces = ref<WorkspaceSummary[]>([])
  const sessions = ref<SessionSummary[]>([])
  const openedTabs = ref<OpenSessionTab[]>([])
  const activeTabId = ref<string | null>(null)
  const sessionName = ref('')
  const commandText = ref('zsh')
  const cwd = ref('')
  const error = ref<string | null>(null)
  const loading = ref(false)
  const creatingSession = ref(false)
  const renamingSession = ref(false)
  const deletingSession = ref(false)
  const removingWorkspace = ref(false)
  const createDialogOpen = ref(false)
  const renameDialogOpen = ref(false)
  const deleteSessionDialogOpen = ref(false)
  const removeWorkspaceDialogOpen = ref(false)
  const selectedSession = ref<SessionSummary | null>(null)
  const selectedWorkspace = ref<WorkspaceSummary | null>(null)
  const renameText = ref('')
  const toasts = ref<AppToast[]>([])
  const sessionNameInput = ref<{ focus: () => void; select: () => void } | null>(null)
  const renameInput = ref<{ focus: () => void; select: () => void } | null>(null)
  let toastId = 0

  const activeSession = computed(() =>
    sessions.value.find((session) => session.id === activeTabId.value) ?? null,
  )

  const activeTab = computed(() =>
    openedTabs.value.find((tab) => tab.sessionId === activeTabId.value) ?? null,
  )

  const activeLifecycleLabel = computed(() => {
    const state = activeSession.value?.lifecycle_state
    return state ? state.charAt(0).toUpperCase() + state.slice(1) : ''
  })

  async function refresh() {
    loading.value = true
    error.value = null
    try {
      const workspaceTree = await listWorkspaceTree()
      applyWorkspaceTree(workspaceTree)
      if (activeTabId.value) {
        await ensureHistoryLoaded(activeTabId.value)
      }
    } catch (err) {
      const message = errorMessage(err)
      error.value = message
      pushToast('error', t('toast.refreshFailed'), message)
    } finally {
      loading.value = false
    }
  }

  async function startSession() {
    if (creatingSession.value) {
      return
    }
    const name = sessionName.value.trim()
    const trimmedCwd = cwd.value.trim()
    const command = commandFromText(commandText.value)
    if (!name) {
      error.value = t('message.sessionNameRequired')
      pushToast('error', t('toast.createSessionFailed'), error.value)
      return
    }
    if (!trimmedCwd) {
      error.value = t('message.workingDirectoryRequired')
      pushToast('error', t('toast.createSessionFailed'), error.value)
      return
    }
    if (command.length === 0) {
      error.value = t('message.commandRequired')
      pushToast('error', t('toast.createSessionFailed'), error.value)
      return
    }
    error.value = null
    creatingSession.value = true
    try {
      const size = estimateTerminalSize()
      const created = await createSession({
        name,
        cwd: trimmedCwd,
        command,
        cols: size.cols,
        rows: size.rows,
      })
      await refresh()
      const session = sessions.value.find((item) => item.id === created.session_id)
      if (!session) {
        throw new Error(t('message.createdSessionMissing'))
      }
      createDialogOpen.value = false
      await openSessionTab(session)
      pushToast('success', t('toast.sessionCreated'), sessionDisplayName(session))
    } catch (err) {
      const message = errorMessage(err)
      error.value = message
      pushToast('error', t('toast.createSessionFailed'), message)
    } finally {
      creatingSession.value = false
    }
  }

  async function renameSelectedSession() {
    if (!selectedSession.value || renamingSession.value) {
      return
    }
    const name = renameText.value.trim()
    if (!name) {
      pushToast('error', t('toast.renameSessionFailed'), t('message.nameRequired'))
      return
    }
    renamingSession.value = true
    try {
      const updated = await updateSession(selectedSession.value.id, { name })
      updateSessionInState(updated)
      selectedSession.value = updated
      renameDialogOpen.value = false
      pushToast('success', t('toast.sessionRenamed'), name)
    } catch (err) {
      pushToast('error', t('toast.renameSessionFailed'), errorMessage(err))
    } finally {
      renamingSession.value = false
    }
  }

  async function deleteSelectedSession() {
    if (!selectedSession.value || deletingSession.value || isActiveLifecycle(selectedSession.value)) {
      return
    }
    const session = selectedSession.value
    deletingSession.value = true
    try {
      await deleteSession(session.id)
      removeSessionFromState(session.id)
      closeTab(session.id)
      selectedSession.value = null
      deleteSessionDialogOpen.value = false
      pushToast('success', t('toast.sessionDeleted'), sessionDisplayName(session))
    } catch (err) {
      pushToast('error', t('toast.deleteSessionFailed'), errorMessage(err))
    } finally {
      deletingSession.value = false
    }
  }

  async function removeSelectedWorkspace() {
    if (!selectedWorkspace.value || removingWorkspace.value) {
      return
    }
    const workspace = selectedWorkspace.value
    removingWorkspace.value = true
    try {
      await deleteWorkspace(workspace.id)
      removeWorkspaceFromState(workspace.id)
      selectedWorkspace.value = null
      removeWorkspaceDialogOpen.value = false
      pushToast('success', t('toast.workspaceRemoved'), t('message.workspaceRemoved', { name: workspace.name }))
    } catch (err) {
      pushToast('error', t('toast.removeWorkspaceFailed'), errorMessage(err))
    } finally {
      removingWorkspace.value = false
    }
  }

  async function reorderWorkspaces(workspaceIds: string[]) {
    const previousTree = workspaceTree.value
    workspaceTree.value = orderWorkspaceTree(previousTree, workspaceIds)
    try {
      const orderedWorkspaces = await updateWorkspaceOrder(workspaceIds)
      workspaceTree.value = orderWorkspaceTree(workspaceTree.value, orderedWorkspaces.map((workspace) => workspace.id))
      workspaces.value = orderedWorkspaces
    } catch (err) {
      pushToast('error', t('toast.updateWorkspaceOrderFailed'), errorMessage(err))
      workspaceTree.value = previousTree
      workspaces.value = previousTree.map(workspaceSummaryFromTree)
      await refresh()
    }
  }

  async function openSessionTab(session: SessionSummary) {
    if (!openedTabs.value.some((tab) => tab.sessionId === session.id)) {
      openedTabs.value.push({
        sessionId: session.id,
        historyText: '',
        historyLoaded: false,
        historyLoading: false,
        historyError: null,
      })
    }
    setActiveTab(session.id)
    await ensureHistoryLoaded(session.id)
  }

  function setActiveTab(sessionId: string) {
    activeTabId.value = sessionId
  }

  function activateOpenedTab(value: string | number) {
    const sessionId = String(value)
    setActiveTab(sessionId)
    void ensureHistoryLoaded(sessionId)
  }

  function closeTab(sessionId: string) {
    const closingIndex = openedTabs.value.findIndex((tab) => tab.sessionId === sessionId)
    if (closingIndex === -1) {
      return
    }
    openedTabs.value.splice(closingIndex, 1)
    if (activeTabId.value !== sessionId) {
      return
    }
    activeTabId.value =
      openedTabs.value[Math.min(closingIndex, openedTabs.value.length - 1)]?.sessionId ?? null
    if (activeTabId.value) {
      void ensureHistoryLoaded(activeTabId.value)
    }
  }

  async function ensureHistoryLoaded(sessionId: string) {
    const session = sessionFor(sessionId)
    const tab = openedTabs.value.find((item) => item.sessionId === sessionId)
    if (!session || !tab || session.lifecycle_state === 'running' || tab.historyLoaded || tab.historyLoading) {
      return
    }
    tab.historyLoading = true
    tab.historyError = null
    try {
      tab.historyText = await readHistory(sessionId)
      tab.historyLoaded = true
    } catch (err) {
      const message = errorMessage(err)
      tab.historyError = message
      pushToast('error', t('toast.readHistoryFailed'), message)
    } finally {
      tab.historyLoading = false
    }
  }

  function openCreateDialog() {
    createDialogOpen.value = true
    if (!sessionName.value.trim()) {
      sessionName.value = commandText.value.trim()
    }
    void nextTick(() => {
      sessionNameInput.value?.focus()
      sessionNameInput.value?.select()
    })
  }

  function openRenameDialog(session: SessionSummary) {
    selectedSession.value = session
    renameText.value = session.name || session.command || ''
    renameDialogOpen.value = true
    void nextTick(() => {
      renameInput.value?.focus()
      renameInput.value?.select()
    })
  }

  function openDeleteSessionDialog(session: SessionSummary) {
    selectedSession.value = session
    deleteSessionDialogOpen.value = true
    if (isActiveLifecycle(session)) {
      pushToast('info', t('toast.sessionCannotBeDeletedYet'), t('message.closeSessionBeforeDelete'))
    }
  }

  function openRemoveWorkspaceDialog(workspace: WorkspaceSummary) {
    selectedWorkspace.value = workspace
    removeWorkspaceDialogOpen.value = true
  }

  function explainUnsupportedDirectoryDelete(workspace: WorkspaceSummary) {
    selectedWorkspace.value = workspace
    pushToast(
      'info',
      t('toast.directoryDeleteUnsupported'),
      t('message.directoryDeleteUnsupported'),
    )
  }

  function applyWorkspaceTree(nextWorkspaceTree: WorkspaceTreeSummary[]) {
    workspaceTree.value = nextWorkspaceTree
    workspaces.value = nextWorkspaceTree.map(workspaceSummaryFromTree)
    sessions.value = nextWorkspaceTree.flatMap((workspace) =>
      workspace.children.map((session) => ({
        ...session,
        workspace_id: session.workspace_id ?? workspace.id,
        workspace_key: session.workspace_key ?? workspace.key,
      })),
    )
  }

  function updateSessionInState(updated: SessionSummary) {
    sessions.value = sessions.value.map((session) => (session.id === updated.id ? updated : session))
    workspaceTree.value = workspaceTree.value.map((workspace) => ({
      ...workspace,
      children: workspace.children.map((session) => {
        if (session.id !== updated.id) {
          return session
        }
        return {
          id: updated.id,
          name: updated.name,
          command: updated.command,
          cwd: updated.cwd,
          lifecycle_state: updated.lifecycle_state,
          attachment_state: updated.attachment_state,
          exit_code: updated.exit_code,
          updated_at: updated.updated_at,
          log_path: updated.log_path,
        }
      }),
    }))
  }

  function removeSessionFromState(sessionId: string) {
    sessions.value = sessions.value.filter((session) => session.id !== sessionId)
    workspaceTree.value = workspaceTree.value.map((workspace) => ({
      ...workspace,
      children: workspace.children.filter((session) => session.id !== sessionId),
    }))
  }

  function removeWorkspaceFromState(workspaceId: string) {
    const removedSessionIds = new Set(
      workspaceTree.value
        .find((workspace) => workspace.id === workspaceId)
        ?.children.map((session) => session.id) ?? [],
    )
    workspaceTree.value = workspaceTree.value.filter((workspace) => workspace.id !== workspaceId)
    workspaces.value = workspaces.value.filter((workspace) => workspace.id !== workspaceId)
    sessions.value = sessions.value.filter((session) => session.workspace_id !== workspaceId)
    openedTabs.value = openedTabs.value.filter((tab) => !removedSessionIds.has(tab.sessionId))
    if (activeSession.value?.workspace_id === workspaceId || (activeTabId.value && removedSessionIds.has(activeTabId.value))) {
      activeTabId.value = openedTabs.value[0]?.sessionId ?? null
    }
  }

  function workspaceSummaryFromTree(workspace: WorkspaceTreeSummary): WorkspaceSummary {
    return {
      id: workspace.id,
      key: workspace.key,
      name: workspace.name,
      path: workspace.path,
      sort_order: workspace.sort_order,
      updated_at: workspace.updated_at,
    }
  }

  function orderWorkspaceTree(tree: WorkspaceTreeSummary[], workspaceIds: string[]) {
    const order = new Map(workspaceIds.map((workspaceId, index) => [workspaceId, index]))
    return [...tree].sort((left, right) => {
      const leftOrder = order.get(left.id) ?? Number.MAX_SAFE_INTEGER
      const rightOrder = order.get(right.id) ?? Number.MAX_SAFE_INTEGER
      return leftOrder - rightOrder
    })
  }

  function sessionFor(sessionId: string) {
    return sessions.value.find((session) => session.id === sessionId) ?? null
  }

  function sessionTitle(sessionId: string) {
    const session = sessionFor(sessionId)
    return session ? sessionDisplayName(session) : sessionId.slice(0, 8)
  }

  function sessionDisplayName(session: SessionSummary) {
    return session.name || session.command || session.id.slice(0, 8)
  }

  function isActiveLifecycle(session: SessionSummary) {
    return ['running', 'starting', 'stopping'].includes(session.lifecycle_state)
  }

  function handleTerminalState(message: ServerControlMessage) {
    if (message.type === 'error') {
      pushToast('error', t('toast.terminalError', { code: message.code }), message.message)
    }
    if (message.type === 'state' || message.type === 'exited') {
      void refresh()
    }
  }

  function handleTerminalError(message: string) {
    pushToast('error', t('toast.terminalConnectionFailed'), message)
  }

  function pushToast(kind: ToastKind, title: string, description?: string) {
    toasts.value.push({ id: ++toastId, kind, title, description })
  }

  function dismissToast(id: number) {
    toasts.value = toasts.value.filter((toast) => toast.id !== id)
  }

  function errorMessage(err: unknown) {
    return err instanceof Error ? err.message : String(err)
  }

  function estimateTerminalSize(): { cols: number; rows: number } {
    const cols = Math.max(80, Math.min(10000, Math.floor((window.innerWidth - 360) / 9)))
    const rows = Math.max(24, Math.min(10000, Math.floor((window.innerHeight - 180) / 18)))
    return { cols, rows }
  }

  watch(commandText, (nextCommand) => {
    if (!sessionName.value.trim()) {
      sessionName.value = nextCommand.trim()
    }
  })

  onMounted(async () => {
    await refresh()
  })
</script>
