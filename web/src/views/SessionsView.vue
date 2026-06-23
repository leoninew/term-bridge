<template>
  <ToastProvider>
    <section
      v-if="!authInitialized"
      class="flex h-screen min-h-screen items-center justify-center bg-[#05070d] p-6 text-sm text-slate-500"
    >
      {{ t('gateway.checkingAuth') }}
    </section>

    <section
      v-else-if="!authenticated"
      class="flex h-screen min-h-screen items-center justify-center bg-[#05070d] p-6 text-sm text-slate-200"
    >
      <form
        class="w-full max-w-sm rounded-xl border border-slate-800 bg-[#0a0f18] p-5 shadow-xl"
        @submit.prevent="login"
      >
        <h1 class="text-lg font-semibold text-slate-100">{{ t('gateway.loginTitle') }}</h1>
        <p class="mt-1 text-slate-500">{{ t('gateway.loginDescription') }}</p>
        <label class="mt-4 block">
          <span class="text-slate-400">{{ t('gateway.username') }}</span>
          <input
            v-model="usernameInput"
            class="mt-1 h-9 w-full rounded-md border border-slate-800 bg-slate-950 px-2 text-slate-100 outline-none"
            autocomplete="username"
          />
        </label>
        <label class="mt-3 block">
          <span class="text-slate-400">{{ t('gateway.password') }}</span>
          <input
            v-model="passwordInput"
            type="password"
            class="mt-1 h-9 w-full rounded-md border border-slate-800 bg-slate-950 px-2 text-slate-100 outline-none"
            autocomplete="current-password"
          />
        </label>
        <button
          type="submit"
          class="mt-4 h-9 w-full rounded-md border border-blue-700 bg-blue-600 text-slate-50 hover:bg-blue-500 disabled:opacity-60"
          :disabled="loggingIn"
        >
          {{ loggingIn ? t('gateway.signingIn') : t('gateway.signIn') }}
        </button>
      </form>
    </section>

    <SplitterGroup
      v-else
      direction="horizontal"
      class="flex h-screen min-h-screen overflow-hidden bg-[#05070d] text-sm text-slate-200"
    >
      <SplitterPanel id="workspace-sidebar" :default-size="22" :min-size="16" :max-size="35">
        <WorkspaceSessionSidebar
          :workspace-tree="workspaceTree"
          :active-session-id="activeTabId"
          :allow-mutations="allowMutations"
          :devices="devices"
          :selected-device-id="selectedDeviceId"
          @select-device="selectDeviceId"
          @select="openSessionTab"
          @refresh="refresh"
          @new-session="openCreateSessionForm"
          @rename-session="openRenameDialog"
          @stop-session="stopSessionFromSidebar"
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
                      :title="t('workbench.closeTabAria', { name: sessionTitle(tab.sessionId) })"
                      @click.stop="closeTab(tab.sessionId)"
                    >
                      <X class="size-3.5" />
                    </button>
                  </div>
                </VueDraggable>
              </TabsList>
            </div>

            <section
              v-if="createSessionFormOpen"
              ref="createSessionWorkbench"
              class="flex min-h-0 min-w-0 flex-1 items-center justify-center overflow-hidden bg-[#090d14] p-6"
            >
              <form
                class="flex w-full max-w-xl flex-col gap-3 rounded-lg border border-slate-800 bg-slate-950/70 p-4 shadow-xl"
                @submit.prevent="startSession"
              >
                <div class="flex items-start gap-3 border-b border-slate-800/80 pb-3">
                  <span
                    class="flex size-9 shrink-0 items-center justify-center rounded-md border border-slate-800 bg-[#0a0f18] text-slate-400"
                  >
                    <SquareTerminal class="size-4" aria-hidden="true" />
                  </span>
                  <div class="min-w-0">
                    <h3 class="text-base font-semibold text-slate-100">
                      {{ t('dialog.newSessionTitle') }}
                    </h3>
                    <p class="mt-0.5 text-sm text-slate-500">
                      {{ t('dialog.newSessionDescription') }}
                    </p>
                  </div>
                </div>

                <label class="flex flex-col gap-1.5 text-sm text-slate-300">
                  <span>{{ t('dialog.cwd') }}</span>
                  <input
                    v-model="cwd"
                    class="h-9 rounded-md border border-slate-800 bg-[#05070d] px-2 text-slate-100 outline-none placeholder:text-slate-600 focus:border-slate-600"
                    :placeholder="t('dialog.workingDirectoryPlaceholder')"
                  />
                </label>
                <label class="flex flex-col gap-1.5 text-sm text-slate-300">
                  <span>{{ t('dialog.name') }}</span>
                  <input
                    ref="sessionNameInput"
                    v-model="sessionName"
                    class="h-9 rounded-md border border-slate-800 bg-[#05070d] px-2 text-slate-100 outline-none placeholder:text-slate-600 focus:border-slate-600"
                    :placeholder="t('dialog.sessionNamePlaceholder')"
                  />
                </label>
                <label class="flex flex-col gap-1.5 text-sm text-slate-300">
                  <span>{{ t('dialog.command') }}</span>
                  <input
                    v-model="commandText"
                    class="h-9 rounded-md border border-slate-800 bg-[#05070d] px-2 text-slate-100 outline-none placeholder:text-slate-600 focus:border-slate-600"
                    :placeholder="t('dialog.commandPlaceholder')"
                  />
                </label>
                <div class="mt-1 flex items-center justify-end gap-2">
                  <button
                    type="button"
                    class="button button-secondary"
                    :disabled="creatingSession"
                    @click="cancelCreateSession"
                  >
                    {{ t('common.cancel') }}
                  </button>
                  <button type="submit" class="button button-primary" :disabled="creatingSession">
                    {{ creatingSession ? t('common.creating') : t('common.create') }}
                  </button>
                </div>
              </form>
            </section>

            <TabsContent
              v-if="!createSessionFormOpen && activeSession && activeTab"
              :key="activeSession.id"
              :value="activeSession.id"
              class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-[#090d14] p-2"
            >
              <TerminalView
                v-if="
                  activeSession.lifecycle_state === 'running' &&
                  selectedDeviceOnline &&
                  !offlineReadonly
                "
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
              v-if="!createSessionFormOpen && openedTabs.length === 0"
              class="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 p-6 text-center text-slate-500"
            >
              <h3 class="text-lg font-semibold text-slate-300">{{ t('workbench.noTabTitle') }}</h3>
              <p>
                {{
                  !selectedDeviceId
                    ? t('gateway.selectDevicePlaceholder')
                    : allowMutations
                      ? t('workbench.noTabDescription')
                      : t('gateway.selectExistingSession')
                }}
              </p>
              <button
                v-if="allowMutations"
                type="button"
                class="rounded-md border border-slate-700 bg-slate-900 px-2.5 py-1.5 text-slate-100 hover:bg-slate-800"
                @click="() => openCreateSessionForm()"
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
                  name: selectedSession ? selectedSession.name : t('dialog.fallbackSession'),
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
      <ToastClose class="toast-close" :aria-label="t('common.close')" />
    </ToastRoot>
    <ToastViewport class="toast-viewport" />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { computed, nextTick, onMounted, ref } from 'vue'
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
  import HistoryTerminalView from '../components/terminal/HistoryTerminalView.vue'
  import TerminalView from '../components/terminal/TerminalView.vue'
  import { logTerminalDiagnostic } from '../components/terminal/diagnostics'
  import { measureXtermSize } from '../components/terminal/useXterm'
  import WorkspaceSessionSidebar from '../components/workspace/WorkspaceSessionSidebar.vue'
  import type {
    ServerControlMessage,
    SessionSummary,
    WorkspaceSummary,
    WorkspaceTreeSummary,
  } from '../protocol/terminal'
  import { commandFromText } from '../protocol/terminal'
  import {
    closeSession,
    createSession,
    deleteSession,
    getSession,
    readHistory,
    terminalWsUrl as sessionTerminalWsUrl,
    updateSession,
  } from '../features/sessions/api'
  import { authLogin, authMe, listDevices, type DeviceSummary } from '../features/gateway/api'
  import {
    deleteWorkspace,
    listWorkspaceTree,
    updateWorkspaceOrder,
  } from '../features/workspaces/api'

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
  const commandText = ref(defaultCommand())
  const cwd = ref('')
  const loading = ref(false)
  const creatingSession = ref(false)
  const renamingSession = ref(false)
  const deletingSession = ref(false)
  const removingWorkspace = ref(false)
  const createSessionFormOpen = ref(false)
  const createSessionWorkspace = ref<WorkspaceSummary | null>(null)
  const renameDialogOpen = ref(false)
  const deleteSessionDialogOpen = ref(false)
  const removeWorkspaceDialogOpen = ref(false)
  const selectedSession = ref<SessionSummary | null>(null)
  const selectedWorkspace = ref<WorkspaceSummary | null>(null)
  const renameText = ref('')
  const toasts = ref<AppToast[]>([])
  const authInitialized = ref(false)
  const authenticated = ref(false)
  const loggingIn = ref(false)
  const usernameInput = ref('')
  const passwordInput = ref('')
  const devices = ref<DeviceSummary[]>([])
  const selectedDeviceId = ref('')
  const offlineReadonly = ref(false)
  const sessionNameInput = ref<{ focus: () => void; select: () => void } | null>(null)
  const renameInput = ref<{ focus: () => void; select: () => void } | null>(null)
  const createSessionWorkbench = ref<HTMLElement | null>(null)
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

  const selectedDevice = computed(
    () => devices.value.find((device) => device.id === selectedDeviceId.value) ?? null,
  )

  const selectedDeviceOnline = computed(() => selectedDevice.value?.online ?? false)

  const allowMutations = computed(
    () => Boolean(selectedDeviceId.value) && selectedDeviceOnline.value && !offlineReadonly.value,
  )

  async function refresh() {
    if (!selectedDeviceId.value) {
      return
    }
    loading.value = true
    try {
      const response = await listWorkspaceTree(selectedDeviceId.value)
      offlineReadonly.value = response.offline || !selectedDeviceOnline.value
      applyWorkspaceTree(response.data)
      if (activeTabId.value) {
        await ensureHistoryLoaded(activeTabId.value)
      }
    } catch (err) {
      notifyError(t('toast.refreshFailed'), err)
    } finally {
      loading.value = false
    }
  }

  async function login() {
    if (loggingIn.value) {
      return
    }
    loggingIn.value = true
    try {
      await authLogin(usernameInput.value, passwordInput.value)
      authenticated.value = true
      await loadDevices()
    } catch (err) {
      notifyError(t('gateway.loginFailed'), err)
    } finally {
      loggingIn.value = false
    }
  }

  async function loadDevices() {
    try {
      devices.value = await listDevices()
      if (devices.value.some((device) => device.id === selectedDeviceId.value)) {
        await selectDevice()
        return
      }
      selectedDeviceId.value = devices.value.length === 1 ? devices.value[0].id : ''
      await selectDevice()
    } catch (err) {
      notifyError(t('toast.refreshFailed'), err)
    }
  }

  async function selectDeviceId(deviceId: string) {
    if (deviceId === selectedDeviceId.value) {
      return
    }
    selectedDeviceId.value = deviceId
    await selectDevice()
  }

  async function selectDevice() {
    offlineReadonly.value = selectedDeviceId.value !== '' && !selectedDeviceOnline.value
    await resetWorkbenchForSourceChange()
    await refresh()
  }

  async function resetWorkbenchForSourceChange() {
    createSessionFormOpen.value = false
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
      const size = measureInitialTerminalSize()
      logTerminalDiagnostic('session.create.request', {
        name,
        cwd: trimmedCwd,
        command: command[0],
        args: command.length - 1,
        cols: size.cols,
        rows: size.rows,
      })
      const workspaceId = createSessionWorkspace.value?.id ?? null
      const created = await createSession(selectedDeviceId.value, workspaceId, {
        ...(workspaceId ? { workspace_id: workspaceId } : {}),
        name,
        cwd: trimmedCwd,
        command,
        cols: size.cols,
        rows: size.rows,
      })
      logTerminalDiagnostic('session.create.response', {
        sessionId: created.session_id,
        workspaceId: created.workspace_id,
        state: created.state,
      })
      const session = await getSession(selectedDeviceId.value, created.workspace_id, created.session_id)
      logTerminalDiagnostic('session.create.summary', {
        sessionId: session.id,
        lifecycleState: session.lifecycle_state,
        attachmentState: session.attachment_state,
        exitCode: session.exit_code,
      })
      if (!upsertSessionInState(session)) {
        await refresh()
      }
      resetCreateSessionForm()
      createSessionFormOpen.value = false
      await openSessionTab(session)
      pushToast('success', t('toast.sessionCreated'), session.name)
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
      const updated = await updateSession(
        selectedDeviceId.value,
        selectedSession.value.workspace_id,
        selectedSession.value.id,
        { name },
      )
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
      await deleteSession(selectedDeviceId.value, session.workspace_id, session.id)
      removeSessionFromState(session.id)
      closeTab(session.id)
      selectedSession.value = null
      deleteSessionDialogOpen.value = false
      pushToast('success', t('toast.sessionDeleted'), session.name)
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
      await deleteWorkspace(selectedDeviceId.value, workspace.id)
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
      const orderedWorkspaces = await updateWorkspaceOrder(selectedDeviceId.value, workspaceIds)
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
    createSessionFormOpen.value = false
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
    createSessionFormOpen.value = false
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
      (session.lifecycle_state === 'running' &&
        selectedDeviceOnline.value &&
        !offlineReadonly.value) ||
      tab.historyLoaded ||
      tab.historyLoading
    ) {
      return
    }
    tab.historyLoading = true
    tab.historyError = null
    try {
      const response = await readHistory(selectedDeviceId.value, session.workspace_id, sessionId)
      offlineReadonly.value = offlineReadonly.value || response.offline
      tab.historyText = response.data
      tab.historyLoaded = true
    } catch (err) {
      const message = errorMessage(err)
      tab.historyError = message
      pushToast('error', t('toast.readHistoryFailed'), message)
    } finally {
      tab.historyLoading = false
    }
  }

  function openCreateSessionForm(workspace?: WorkspaceSummary) {
    if (!ensureMutationAllowed()) {
      return
    }
    logTerminalDiagnostic('session.form.open', {
      workspaceId: workspace?.id,
      workspacePath: workspace?.path,
      activeTabId: activeTabId.value,
    })
    resetCreateSessionForm(workspace)
    createSessionFormOpen.value = true
    void nextTick(() => {
      sessionNameInput.value?.focus()
      sessionNameInput.value?.select()
    })
  }

  function cancelCreateSession() {
    logTerminalDiagnostic('session.form.cancel', { activeTabId: activeTabId.value })
    createSessionFormOpen.value = false
  }

  function resetCreateSessionForm(workspace?: WorkspaceSummary) {
    commandText.value = defaultCommand()
    createSessionWorkspace.value = workspace ?? null
    cwd.value = workspace?.path ?? '~'
    sessionName.value = t('dialog.defaultSessionName')
  }

  function defaultCommand() {
    const userAgent = window.navigator.userAgent.toLowerCase()
    if (userAgent.includes('windows')) {
      return 'cmd'
    }
    if (userAgent.includes('mac os') || userAgent.includes('macintosh')) {
      return 'zsh'
    }
    if (userAgent.includes('linux')) {
      return 'bash'
    }
    return 'bash'
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

  async function stopSessionFromSidebar(session: SessionSummary) {
    if (!ensureMutationAllowed()) {
      return
    }
    if (!isStoppableLifecycle(session)) {
      return
    }
    try {
      const updated = await closeSession(selectedDeviceId.value, session.workspace_id, session.id)
      updateSessionInState(updated)
      await ensureHistoryLoaded(session.id)
    } catch (err) {
      notifyError(t('toast.stopSessionFailed'), err)
    }
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
    }
  }

  function workspaceSummaryFromTree(workspace: WorkspaceTreeSummary): WorkspaceSummary {
    return {
      id: workspace.id,
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
    if (!session) {
      return sessionId.slice(0, 8)
    }
    const workspaceName = workspaces.value.find((workspace) => workspace.id === session.workspace_id)?.name
    return workspaceName ? `${session.name} · ${workspaceName}` : session.name
  }


  function isActiveLifecycle(session: SessionSummary) {
    return ['running', 'starting', 'stopping'].includes(session.lifecycle_state)
  }

  function isStoppableLifecycle(session: SessionSummary) {
    return ['running', 'starting'].includes(session.lifecycle_state)
  }

  function terminalWsUrl(sessionId: string) {
    const session = sessionFor(sessionId)
    return selectedDeviceId.value && selectedDeviceOnline.value && session
      ? sessionTerminalWsUrl(selectedDeviceId.value, session.workspace_id, sessionId)
      : null
  }

  function ensureMutationAllowed() {
    if (allowMutations.value) {
      return true
    }
    pushToast('info', t('gateway.offlineReadonlyTitle'), t('gateway.offlineReadonly'))
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

  async function refreshActiveSession(sessionId = activeTabId.value) {
    if (!sessionId || !selectedDeviceId.value) {
      return
    }
    try {
      const session = sessionFor(sessionId)
      if (!session) {
        return
      }
      const updated = await getSession(selectedDeviceId.value, session.workspace_id, sessionId)
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

  function measureInitialTerminalSize(): { cols: number; rows: number } {
    const measured = measureCreateSessionWorkbench()
    if (measured) {
      return measured
    }
    const cols = Math.max(80, Math.min(10000, Math.floor((window.innerWidth - 360) / 9)))
    const rows = Math.max(24, Math.min(10000, Math.floor((window.innerHeight - 180) / 18)))
    logTerminalDiagnostic('xterm.measure.fallback', { cols, rows })
    return { cols, rows }
  }

  function measureCreateSessionWorkbench(): { cols: number; rows: number } | null {
    const workbench = createSessionWorkbench.value
    if (!workbench) {
      logTerminalDiagnostic('xterm.measure.missing-workbench')
      return null
    }
    const wrapper = document.createElement('section')
    wrapper.style.position = 'fixed'
    wrapper.style.left = '-10000px'
    wrapper.style.top = '0'
    wrapper.style.width = `${workbench.clientWidth}px`
    wrapper.style.height = `${workbench.clientHeight}px`
    wrapper.style.display = 'flex'
    wrapper.style.flexDirection = 'column'
    wrapper.style.padding = '8px'
    wrapper.style.boxSizing = 'border-box'
    wrapper.style.visibility = 'hidden'
    wrapper.style.pointerEvents = 'none'

    const shell = document.createElement('section')
    shell.className = 'terminal-shell'
    shell.style.flex = '1'

    const container = document.createElement('div')
    container.className = 'terminal-container'
    shell.appendChild(container)
    wrapper.appendChild(shell)
    document.body.appendChild(wrapper)
    try {
      return measureXtermSize(container)
    } finally {
      wrapper.remove()
    }
  }

  async function initializeAuth() {
    try {
      const me = await authMe()
      authenticated.value = me.authenticated
      usernameInput.value = me.username || usernameInput.value
      if (me.authenticated) {
        await loadDevices()
      }
    } catch (err) {
      notifyError(t('toast.refreshFailed'), err)
      authenticated.value = false
    } finally {
      authInitialized.value = true
    }
  }

  onMounted(async () => {
    await initializeAuth()
  })
</script>
