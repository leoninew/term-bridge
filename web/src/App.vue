<template>
  <ToastProvider>
    <section
      v-if="gatewayRoute && gatewayAvailable && !gatewayAuthenticated"
      class="flex h-screen min-h-screen items-center justify-center bg-[#05070d] p-6 text-sm text-slate-200"
    >
      <form
        class="w-full max-w-sm rounded-xl border border-slate-800 bg-[#0a0f18] p-5 shadow-xl"
        @submit.prevent="loginGateway"
      >
        <h1 class="text-lg font-semibold text-slate-100">{{ t('gateway.loginTitle') }}</h1>
        <p class="mt-1 text-slate-500">{{ t('gateway.loginDescription') }}</p>
        <label class="mt-4 block">
          <span class="text-slate-400">{{ t('gateway.username') }}</span>
          <input
            v-model="gatewayUsernameInput"
            class="mt-1 h-9 w-full rounded-md border border-slate-800 bg-slate-950 px-2 text-slate-100 outline-none"
            autocomplete="username"
          />
        </label>
        <label class="mt-3 block">
          <span class="text-slate-400">{{ t('gateway.password') }}</span>
          <input
            v-model="gatewayPasswordInput"
            type="password"
            class="mt-1 h-9 w-full rounded-md border border-slate-800 bg-slate-950 px-2 text-slate-100 outline-none"
            autocomplete="current-password"
          />
        </label>
        <button
          type="submit"
          class="mt-4 h-9 w-full rounded-md border border-blue-700 bg-blue-600 text-slate-50 hover:bg-blue-500 disabled:opacity-60"
          :disabled="gatewayLoggingIn"
        >
          {{ gatewayLoggingIn ? t('gateway.signingIn') : t('gateway.signIn') }}
        </button>
      </form>
    </section>

    <SplitterGroup
      v-else
      direction="horizontal"
      class="flex h-screen min-h-screen overflow-hidden bg-[#05070d] text-sm text-slate-200"
    >
      <SplitterPanel id="workspace-sidebar" :default-size="22" :min-size="16" :max-size="35">
        <div
          v-if="gatewayRoute && gatewayAvailable && gatewayAuthenticated"
          class="border-b border-slate-800/80 bg-[#0a0f18] p-2 text-sm"
        >
          <div class="flex items-center justify-between gap-2">
            <span class="font-medium text-slate-200">{{ t('gateway.devices') }}</span>
            <button
              type="button"
              class="rounded px-1.5 py-0.5 text-slate-500 hover:bg-slate-900 hover:text-slate-200"
              @click="logoutGateway"
            >
              {{ t('gateway.logout') }}
            </button>
          </div>
          <select
            v-model="selectedGatewayDeviceId"
            class="mt-2 h-8 w-full rounded-md border border-slate-800 bg-slate-950 px-2 text-slate-100 outline-none"
            @change="selectGatewayDevice"
          >
            <option value="">{{ t('gateway.localWorkbench') }}</option>
            <option v-for="device in gatewayDevices" :key="device.id" :value="device.id">
              {{ device.name }} · {{ device.online ? t('gateway.online') : t('gateway.offline') }}
            </option>
          </select>
          <p v-if="isGatewayBackend" class="mt-1.5 text-xs text-slate-500">
            {{ t('gateway.attachOnly') }}
          </p>
        </div>
        <WorkspaceSessionSidebar
          :workspace-tree="workspaceTree"
          :active-session-id="activeTabId"
          :allow-mutations="allowMutations"
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

      <SplitterResizeHandle
        class="group flex w-1 shrink-0 cursor-col-resize items-stretch justify-center bg-[#05070d] outline-none"
      >
        <span class="w-px bg-slate-800 transition group-hover:bg-slate-700" />
      </SplitterResizeHandle>

      <SplitterPanel id="terminal-workbench" :min-size="55">
        <section class="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-[#090d14]">
          <TabsRoot
            :model-value="activeTabId ?? undefined"
            class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
            @update:model-value="activateOpenedTab"
          >
            <div
              class="flex h-11 shrink-0 items-center border-b border-slate-800/80 bg-[#0a0f18] px-2"
            >
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
                    :class="
                      activeTabId === tab.sessionId
                        ? 'border-slate-700 bg-slate-900 text-slate-50'
                        : 'border-slate-800 bg-slate-950/70 text-slate-400 hover:border-slate-700 hover:bg-slate-900/80'
                    "
                  >
                    <TabsTrigger
                      :value="tab.sessionId"
                      class="tab-drag-handle flex min-w-0 items-center gap-1.5 rounded-md px-2 py-1.5 outline-none"
                    >
                      <SquareTerminal class="size-4 shrink-0 text-slate-500" aria-hidden="true" />
                      <span class="truncate">{{ sessionTitle(tab.sessionId) }}</span>
                    </TabsTrigger>
                    <button
                      type="button"
                      class="rounded-md p-1 text-slate-500 hover:bg-slate-800 hover:text-slate-200"
                      :aria-label="
                        t('workbench.closeTabAria', { name: sessionTitle(tab.sessionId) })
                      "
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
                :ws-url="terminalWsUrl(activeSession.id)"
                :session-id="activeSession.id"
                @state="handleTerminalState"
                @terminal-error="handleTerminalError"
              />

              <section v-else class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
                <div
                  v-if="activeTab.historyLoading"
                  class="flex flex-1 items-center justify-center text-slate-500"
                >
                  {{ t('workbench.loadingHistory') }}
                </div>
                <div
                  v-else-if="activeTab.historyError"
                  class="rounded-md border border-slate-800 bg-slate-950/80 px-2 py-1.5 text-red-100"
                >
                  {{ activeTab.historyError }}
                </div>
                <HistoryTerminalView
                  v-else-if="activeTab.historyText"
                  :key="`${activeSession.id}-history`"
                  :history="activeTab.historyText"
                />
                <div v-else class="flex flex-1 items-center justify-center text-slate-500">
                  {{ t('workbench.noHistory') }}
                </div>
              </section>
            </TabsContent>

            <section
              v-if="openedTabs.length === 0"
              class="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 p-6 text-center text-slate-500"
            >
              <h3 class="text-lg font-semibold text-slate-300">{{ t('workbench.noTabTitle') }}</h3>
              <p>
                {{
                  isGatewayBackend
                    ? t('gateway.selectExistingSession')
                    : t('workbench.noTabDescription')
                }}
              </p>
              <button
                v-if="allowMutations"
                type="button"
                class="rounded-md border border-slate-700 bg-slate-900 px-2.5 py-1.5 text-slate-100 hover:bg-slate-800"
                @click="openCreateDialog"
              >
                {{ t('workbench.newSession') }}
              </button>
            </section>
          </TabsRoot>

          <footer
            class="flex h-8 shrink-0 items-center gap-1.5 overflow-hidden border-t border-slate-800/80 bg-[#0a0f18] px-3 text-sm text-slate-500"
          >
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
              <input
                ref="sessionNameInput"
                v-model="sessionName"
                :placeholder="t('dialog.sessionNamePlaceholder')"
              />
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
                <button type="button" class="button button-secondary">
                  {{ t('common.cancel') }}
                </button>
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
              <input
                ref="renameInput"
                v-model="renameText"
                :placeholder="t('dialog.sessionNamePlaceholder')"
              />
            </label>
            <div class="dialog-actions">
              <DialogClose as-child>
                <button type="button" class="button button-secondary">
                  {{ t('common.cancel') }}
                </button>
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
          <AlertDialogTitle class="dialog-title">{{
            t('dialog.deleteSessionTitle')
          }}</AlertDialogTitle>
          <AlertDialogDescription class="dialog-description">
            <template v-if="selectedSession && isActiveLifecycle(selectedSession)">
              {{ t('dialog.deleteActiveSessionDescription') }}
            </template>
            <template v-else>
              {{
                t('dialog.deleteSessionDescription', {
                  name: selectedSession
                    ? sessionDisplayName(selectedSession)
                    : t('dialog.fallbackSession'),
                })
              }}
            </template>
          </AlertDialogDescription>
          <div class="dialog-actions">
            <AlertDialogCancel as-child>
              <button type="button" class="button button-secondary">
                {{ t('common.cancel') }}
              </button>
            </AlertDialogCancel>
            <AlertDialogAction as-child>
              <button
                type="button"
                class="button button-danger"
                :disabled="
                  !selectedSession || isActiveLifecycle(selectedSession) || deletingSession
                "
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
          <AlertDialogTitle class="dialog-title">{{
            t('dialog.removeWorkspaceTitle')
          }}</AlertDialogTitle>
          <AlertDialogDescription class="dialog-description">
            {{ t('dialog.removeWorkspaceDescription') }}
          </AlertDialogDescription>
          <div v-if="selectedWorkspace" class="dialog-note">
            {{ selectedWorkspace.name }} — {{ selectedWorkspace.path }}
          </div>
          <div class="dialog-actions">
            <AlertDialogCancel as-child>
              <button type="button" class="button button-secondary">
                {{ t('common.cancel') }}
              </button>
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
    getSession,
    readHistory,
    updateSession,
  } from './features/sessions/api'
  import {
    gatewayLogin,
    gatewayLogout,
    gatewayMe,
    listGatewayDevices,
    listGatewayWorkspaceTree,
    readGatewayHistory,
    type GatewayDeviceSummary,
  } from './features/gateway/api'
  import {
    deleteWorkspace,
    listWorkspaceTree,
    updateWorkspaceOrder,
  } from './features/workspaces/api'

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
  const gatewayRoute = window.location.pathname.startsWith('/gateway')
  const gatewayAvailable = ref(false)
  const gatewayAuthenticated = ref(false)
  const gatewayLoggingIn = ref(false)
  const gatewayUsernameInput = ref('admin')
  const gatewayPasswordInput = ref('admin')
  const gatewayDevices = ref<GatewayDeviceSummary[]>([])
  const selectedGatewayDeviceId = ref('')
  const sessionNameInput = ref<{ focus: () => void; select: () => void } | null>(null)
  const renameInput = ref<{ focus: () => void; select: () => void } | null>(null)
  let toastId = 0

  const activeSession = computed(
    () => sessions.value.find((session) => session.id === activeTabId.value) ?? null,
  )

  const activeTab = computed(
    () => openedTabs.value.find((tab) => tab.sessionId === activeTabId.value) ?? null,
  )

  const activeLifecycleLabel = computed(() => {
    const state = activeSession.value?.lifecycle_state
    return state ? state.charAt(0).toUpperCase() + state.slice(1) : ''
  })

  const selectedGatewayDevice = computed(
    () =>
      gatewayDevices.value.find((device) => device.id === selectedGatewayDeviceId.value) ?? null,
  )

  const activeBackend = computed(() => {
    const gatewayDeviceId = selectedGatewayDeviceId.value
    if (!gatewayDeviceId) {
      return {
        kind: 'local' as const,
        allowMutations: true,
        async loadWorkspaceTree() {
          return listWorkspaceTree()
        },
        async readHistory(sessionId: string) {
          return readHistory(sessionId)
        },
        terminalWsUrl(sessionId: string) {
          return `/api/sessions/${encodeURIComponent(sessionId)}/ws`
        },
      }
    }
    return {
      kind: 'gateway' as const,
      allowMutations: false,
      async loadWorkspaceTree() {
        gatewayDevices.value = await listGatewayDevices()
        const device = selectedGatewayDevice.value
        if (!device || !device.online) {
          throw new Error(t('gateway.routeUnavailable'))
        }
        return listGatewayWorkspaceTree(gatewayDeviceId)
      },
      async readHistory(sessionId: string) {
        return readGatewayHistory(gatewayDeviceId, sessionId)
      },
      terminalWsUrl(sessionId: string) {
        return `/api/gateway/devices/${encodeURIComponent(gatewayDeviceId)}/sessions/${encodeURIComponent(sessionId)}/ws`
      },
    }
  })

  const allowMutations = computed(() => activeBackend.value.allowMutations)
  const isGatewayBackend = computed(() => activeBackend.value.kind === 'gateway')

  async function refresh() {
    loading.value = true
    try {
      applyWorkspaceTree(await activeBackend.value.loadWorkspaceTree())
      if (activeTabId.value) {
        await ensureHistoryLoaded(activeTabId.value)
      }
    } catch (err) {
      notifyError(t('toast.refreshFailed'), err)
    } finally {
      loading.value = false
    }
  }

  async function loginGateway() {
    if (gatewayLoggingIn.value) {
      return
    }
    gatewayLoggingIn.value = true
    try {
      await gatewayLogin(gatewayUsernameInput.value, gatewayPasswordInput.value)
      gatewayAuthenticated.value = true
      gatewayDevices.value = await listGatewayDevices()
      selectedGatewayDeviceId.value = gatewayDevices.value.find((device) => device.online)?.id ?? ''
      await resetWorkbenchForSourceChange()
      await refresh()
    } catch (err) {
      notifyError(t('gateway.loginFailed'), err)
    } finally {
      gatewayLoggingIn.value = false
    }
  }

  async function logoutGateway() {
    try {
      await gatewayLogout()
    } catch (err) {
      notifyError(t('gateway.logoutFailed'), err)
    }
    gatewayAuthenticated.value = false
    gatewayDevices.value = []
    selectedGatewayDeviceId.value = ''
    await resetWorkbenchForSourceChange()
    await refresh()
  }

  async function selectGatewayDevice() {
    await resetWorkbenchForSourceChange()
    await refresh()
  }

  async function resetWorkbenchForSourceChange() {
    openedTabs.value = []
    activeTabId.value = null
    applyWorkspaceTree([])
  }

  async function startSession() {
    if (!ensureMutationAllowed()) {
      return
    }
    if (creatingSession.value) {
      return
    }
    const name = sessionName.value.trim()
    const trimmedCwd = cwd.value.trim()
    const command = commandFromText(commandText.value)
    if (!name) {
      pushToast('error', t('toast.createSessionFailed'), t('message.sessionNameRequired'))
      return
    }
    if (!trimmedCwd) {
      pushToast('error', t('toast.createSessionFailed'), t('message.workingDirectoryRequired'))
      return
    }
    if (command.length === 0) {
      pushToast('error', t('toast.createSessionFailed'), t('message.commandRequired'))
      return
    }
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
      const session = await getSession(created.session_id)
      if (!upsertSessionInState(session)) {
        await refresh()
      }
      createDialogOpen.value = false
      await openSessionTab(session)
      pushToast('success', t('toast.sessionCreated'), sessionDisplayName(session))
    } catch (err) {
      notifyError(t('toast.createSessionFailed'), err)
    } finally {
      creatingSession.value = false
    }
  }

  async function renameSelectedSession() {
    if (!ensureMutationAllowed()) {
      return
    }
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
      notifyError(t('toast.renameSessionFailed'), err)
    } finally {
      renamingSession.value = false
    }
  }

  async function deleteSelectedSession() {
    if (!ensureMutationAllowed()) {
      return
    }
    if (
      !selectedSession.value ||
      deletingSession.value ||
      isActiveLifecycle(selectedSession.value)
    ) {
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
      notifyError(t('toast.deleteSessionFailed'), err)
    } finally {
      deletingSession.value = false
    }
  }

  async function removeSelectedWorkspace() {
    if (!ensureMutationAllowed()) {
      return
    }
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
      pushToast(
        'success',
        t('toast.workspaceRemoved'),
        t('message.workspaceRemoved', { name: workspace.name }),
      )
    } catch (err) {
      notifyError(t('toast.removeWorkspaceFailed'), err)
    } finally {
      removingWorkspace.value = false
    }
  }

  async function reorderWorkspaces(workspaceIds: string[]) {
    if (!ensureMutationAllowed()) {
      return
    }
    const previousTree = workspaceTree.value
    workspaceTree.value = orderWorkspaceTree(previousTree, workspaceIds)
    try {
      const orderedWorkspaces = await updateWorkspaceOrder(workspaceIds)
      workspaceTree.value = orderWorkspaceTree(
        workspaceTree.value,
        orderedWorkspaces.map((workspace) => workspace.id),
      )
      workspaces.value = orderedWorkspaces
    } catch (err) {
      notifyError(t('toast.updateWorkspaceOrderFailed'), err)
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
    if (
      !session ||
      !tab ||
      session.lifecycle_state === 'running' ||
      tab.historyLoaded ||
      tab.historyLoading
    ) {
      return
    }
    tab.historyLoading = true
    tab.historyError = null
    try {
      tab.historyText = await activeBackend.value.readHistory(sessionId)
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
    if (!ensureMutationAllowed()) {
      return
    }
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
    if (!ensureMutationAllowed()) {
      return
    }
    selectedSession.value = session
    renameText.value = session.name || session.command || ''
    renameDialogOpen.value = true
    void nextTick(() => {
      renameInput.value?.focus()
      renameInput.value?.select()
    })
  }

  function openDeleteSessionDialog(session: SessionSummary) {
    if (!ensureMutationAllowed()) {
      return
    }
    selectedSession.value = session
    deleteSessionDialogOpen.value = true
    if (isActiveLifecycle(session)) {
      pushToast('info', t('toast.sessionCannotBeDeletedYet'), t('message.closeSessionBeforeDelete'))
    }
  }

  function openRemoveWorkspaceDialog(workspace: WorkspaceSummary) {
    if (!ensureMutationAllowed()) {
      return
    }
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

  function upsertSessionInState(updated: SessionSummary): boolean {
    if (!workspaceTree.value.some((workspace) => workspace.id === updated.workspace_id)) {
      return false
    }

    const existingIndex = sessions.value.findIndex((session) => session.id === updated.id)
    if (existingIndex === -1) {
      sessions.value = [...sessions.value, updated]
    } else {
      sessions.value = sessions.value.map((session) =>
        session.id === updated.id ? updated : session,
      )
    }

    workspaceTree.value = workspaceTree.value.map((workspace) => {
      if (workspace.id !== updated.workspace_id) {
        return workspace
      }
      const nextSession = sessionSummaryForWorkspaceTree(updated)
      const childIndex = workspace.children.findIndex((session) => session.id === updated.id)
      if (childIndex === -1) {
        return { ...workspace, children: [...workspace.children, nextSession] }
      }
      return {
        ...workspace,
        children: workspace.children.map((session) =>
          session.id === updated.id ? nextSession : session,
        ),
      }
    })
    return true
  }

  function updateSessionInState(updated: SessionSummary) {
    if (!sessions.value.some((session) => session.id === updated.id)) {
      return
    }
    upsertSessionInState(updated)
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
    if (
      activeSession.value?.workspace_id === workspaceId ||
      (activeTabId.value && removedSessionIds.has(activeTabId.value))
    ) {
      activeTabId.value = openedTabs.value[0]?.sessionId ?? null
    }
  }

  function sessionSummaryForWorkspaceTree(session: SessionSummary) {
    return {
      id: session.id,
      name: session.name,
      command: session.command,
      cwd: session.cwd,
      lifecycle_state: session.lifecycle_state,
      attachment_state: session.attachment_state,
      exit_code: session.exit_code,
      updated_at: session.updated_at,
      log_path: session.log_path,
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

  function terminalWsUrl(sessionId: string) {
    return activeBackend.value.terminalWsUrl(sessionId)
  }

  function ensureMutationAllowed() {
    if (allowMutations.value) {
      return true
    }
    pushToast('info', t('gateway.attachOnlyTitle'), t('gateway.attachOnly'))
    return false
  }

  function handleTerminalState(message: ServerControlMessage) {
    if (message.type === 'error') {
      pushToast('error', t('toast.terminalError', { code: message.code }), message.message)
    }
    if (message.type === 'state' || message.type === 'exited') {
      void refreshActiveSession()
    }
  }

  async function refreshActiveSession() {
    const sessionId = activeTabId.value
    if (!sessionId || isGatewayBackend.value) {
      return
    }
    try {
      const updated = await getSession(sessionId)
      updateSessionInState(updated)
      await ensureHistoryLoaded(sessionId)
    } catch (err) {
      notifyError(t('toast.refreshFailed'), err)
    }
  }

  function handleTerminalError(message: string) {
    pushToast('error', t('toast.terminalConnectionFailed'), message)
  }

  function pushToast(kind: ToastKind, title: string, description?: string) {
    toasts.value.push({ id: ++toastId, kind, title, description })
  }

  function notifyError(title: string, err: unknown) {
    pushToast('error', title, errorMessage(err))
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

  async function initializeGateway() {
    if (!gatewayRoute) {
      gatewayAvailable.value = false
      gatewayAuthenticated.value = false
      return
    }
    try {
      const me = await gatewayMe()
      gatewayAvailable.value = true
      gatewayAuthenticated.value = me.authenticated
      if (me.authenticated) {
        gatewayDevices.value = await listGatewayDevices()
      }
    } catch {
      gatewayAvailable.value = false
      gatewayAuthenticated.value = false
    }
  }

  onMounted(async () => {
    await initializeGateway()
    if (!gatewayAvailable.value || gatewayAuthenticated.value) {
      await refresh()
    }
  })
</script>
