<template>
  <ToastProvider>
    <section class="min-h-screen bg-[var(--color-app-bg)] text-sm text-[var(--color-text)]">
      <header
        class="flex h-16 items-center justify-between border-b border-[var(--color-border)] bg-[var(--color-panel-header)] px-6"
      >
        <RouterLink to="/" class="flex items-center gap-3">
          <img :src="logoDataUrl" alt="TermBridge" class="size-10 rounded-xl shadow-lg" />
          <p class="text-lg font-semibold text-[var(--color-text-strong)]">TermBridge</p>
        </RouterLink>

        <div class="flex items-center gap-3">
          <template v-if="appMode.effectiveMode === 'agent'">
            <CloudAccountConnectionMenu :connection="cloudSession" />
            <DisplayControls />
          </template>
          <template v-else>
            <DisplayControls />
            <CloudAccountMenu
              :authenticated="gateway.authenticated"
              :user-display-name="cloudUserDisplayName"
              :user-email="gateway.user?.email ?? ''"
              @login="openCloudLogin"
              @logout="logoutCloud"
              @change-password="changePasswordDialogOpen = true"
            />
          </template>
        </div>
      </header>

      <main
        class="min-h-[calc(100vh-4rem)] p-6"
        :class="appMode.effectiveMode === 'agent' ? 'flex items-center justify-center' : 'pt-10'"
      >
        <div
          class="mx-auto flex w-full flex-col gap-5"
          :class="appMode.effectiveMode === 'agent' ? 'max-w-4xl' : 'max-w-[1200px]'"
        >
          <template v-if="appMode.effectiveMode === 'agent'">
            <section
              class="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl"
            >
              <div class="flex items-center justify-between border-b border-[var(--color-border)] px-5 py-4">
                <h1 class="text-lg font-semibold text-[var(--color-text-strong)]">
                  {{ t('dashboard.agentHomeTitle') }}
                </h1>
                <div class="flex items-center gap-2">
                  <button
                    type="button"
                    class="inline-flex h-8 items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-3 text-sm text-[var(--color-text)] outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)] disabled:cursor-not-allowed disabled:text-[var(--color-text-muted)]"
                    :disabled="connectingCloud || (!cloudSession && !cloudConnectEnabled)"
                    :title="cloudConnectionActionTitle"
                    @click="toggleCloudConnection"
                  >
                    <Unplug
                      v-if="cloudSession"
                      class="size-3.5 text-[var(--color-text-subtle)]"
                    />
                    <Plug v-else class="size-3.5 text-[var(--color-text-subtle)]" />
                    {{ cloudConnectionActionLabel }}
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-8 items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-3 text-sm text-[var(--color-text)] outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)] disabled:cursor-not-allowed disabled:text-[var(--color-text-muted)]"
                    :disabled="!cloudSession"
                    :title="cloudSession ? '' : t('dashboard.cloudAccountNotConnected')"
                    @click="switchToCloudMode"
                  >
                    {{ t('dashboard.switchToCloudMode') }}
                  </button>
                </div>
              </div>

              <div class="grid gap-3 p-5 sm:grid-cols-3">
                <div
                  class="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-muted)] p-4"
                >
                  <div class="flex items-center gap-3">
                    <span
                      class="flex size-10 shrink-0 items-center justify-center rounded-lg bg-blue-500/10 text-blue-500"
                    >
                      <Monitor class="size-5" />
                    </span>
                    <div class="min-w-0">
                      <p class="text-sm text-[var(--color-text-strong)]">
                        {{ t('dashboard.cloudConnectionDevice') }}
                      </p>
                      <p class="mt-1 truncate text-sm text-[var(--color-text-muted)]">
                        {{ deviceName || t('dashboard.todoLocalUser') }}
                      </p>
                    </div>
                  </div>
                </div>

                <div
                  class="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-muted)] p-4"
                >
                  <div class="flex items-center gap-3">
                    <span
                      class="flex size-10 shrink-0 items-center justify-center rounded-lg bg-blue-500/10 text-blue-500"
                    >
                      <User class="size-5" />
                    </span>
                    <div class="min-w-0">
                      <p class="text-sm text-[var(--color-text-strong)]">
                        {{ t('dashboard.localUser') }}
                      </p>
                      <p class="mt-1 truncate text-sm text-[var(--color-text-muted)]">
                        {{ localUserLabel }}
                      </p>
                    </div>
                  </div>
                </div>

                <div
                  class="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-muted)] p-4"
                >
                  <div class="flex items-center gap-3">
                    <span
                      class="flex size-10 shrink-0 items-center justify-center rounded-lg bg-blue-500/10 text-blue-500"
                    >
                      <Package class="size-5" />
                    </span>
                    <div class="min-w-0">
                      <p class="text-sm text-[var(--color-text-strong)]">
                        {{ t('dashboard.projectVersion') }}
                      </p>
                      <p class="mt-1 truncate text-sm text-[var(--color-text-muted)]">
                        {{ projectVersionLabel }}
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            <section
              class="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl"
            >
              <div class="flex items-center justify-between border-b border-[var(--color-border)] px-5 py-4">
                <h2 class="text-lg font-semibold text-[var(--color-text-strong)]">
                  {{ t('dashboard.agentWorkspaceListTitle') }}
                </h2>
                <RouterLink
                  :to="{ name: 'agent-sessions' }"
                  class="inline-flex h-8 items-center gap-1.5 rounded-md border border-blue-700 bg-blue-600 px-3 text-sm text-slate-50 outline-none hover:bg-blue-500 focus:bg-blue-500"
                >
                  <FolderOpen class="size-3.5 text-slate-100" />
                  {{ t('dashboard.openWorkspaceList') }}
                </RouterLink>
              </div>

              <div v-if="workspacesLoading" class="p-5 text-sm text-[var(--color-text-muted)]">
                {{ t('dashboard.loadingWorkspaces') }}
              </div>
              <div v-else-if="workspaceError" class="p-5 text-sm text-[var(--color-danger-text)]">
                {{ workspaceError }}
              </div>
              <div v-else-if="workspaces.length === 0" class="p-5 text-sm text-[var(--color-text-muted)]">
                {{ t('dashboard.emptyWorkspaces') }}
              </div>
              <ul v-else class="divide-y divide-[var(--color-border)]">
                <li
                  v-for="workspace in workspaces"
                  :key="workspace.id"
                  class="flex min-w-0 items-center justify-between gap-4 px-5 py-4"
                >
                  <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-2">
                      <Folder class="size-4 shrink-0 text-[var(--color-text-subtle)]" />
                      <span class="truncate text-sm text-[var(--color-text-strong)]">
                        {{ workspace.name }}
                      </span>
                    </div>
                    <div class="mt-1 flex min-w-0 items-center gap-3 pl-6 text-sm text-[var(--color-text-muted)]">
                      <span class="truncate">{{ workspace.path }}</span>
                      <span class="shrink-0">
                        {{ workspaceUpdatedAt(workspace.updated_at) }}
                      </span>
                    </div>
                  </div>
                  <ArrowRight
                    class="size-5 shrink-0 text-[var(--color-text-subtle)]"
                    aria-hidden="true"
                  />
                </li>
              </ul>
            </section>
          </template>

          <template v-else>
            <section class="grid min-h-[680px] content-center gap-10 lg:grid-cols-[minmax(0,440px)_minmax(0,1fr)]">
              <div class="flex flex-col justify-center gap-6">
                <div>
                  <p class="text-sm text-[var(--color-text-muted)]">
                    {{ t('dashboard.cloudHomeTitle') }}
                  </p>
                  <h1 class="mt-3 text-lg font-semibold text-[var(--color-text-strong)]">
                    {{ t('dashboard.cloudLandingTitle') }}
                  </h1>
                  <p class="mt-3 max-w-md text-sm text-[var(--color-text-muted)]">
                    {{ t('dashboard.cloudLandingCopyTodo') }}
                  </p>
                </div>

                <div class="flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    class="inline-flex h-8 items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-3 text-sm text-[var(--color-text)] outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
                    @click="openCloudDashboard"
                  >
                    {{ t('dashboard.openCloudDashboard') }}
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-8 items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-3 text-sm text-[var(--color-text)] outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
                    @click="openAgentEntry"
                  >
                    {{ agentEntryLabel }}
                  </button>
                </div>

                <div class="grid gap-3 border-t border-[var(--color-border)] pt-5 sm:grid-cols-3 lg:grid-cols-1">
                  <div>
                    <p class="text-sm text-[var(--color-text-strong)]">
                      {{ t('dashboard.cloudLandingSlotPrimary') }}
                    </p>
                    <p class="mt-1 text-sm text-[var(--color-text-muted)]">
                      {{ t('dashboard.cloudLandingSlotPrimaryTodo') }}
                    </p>
                  </div>
                  <div>
                    <p class="text-sm text-[var(--color-text-strong)]">
                      {{ t('dashboard.cloudLandingSlotSecondary') }}
                    </p>
                    <p class="mt-1 text-sm text-[var(--color-text-muted)]">
                      {{ t('dashboard.cloudLandingSlotSecondaryTodo') }}
                    </p>
                  </div>
                  <div>
                    <p class="text-sm text-[var(--color-text-strong)]">
                      {{ t('dashboard.cloudLandingSlotTertiary') }}
                    </p>
                    <p class="mt-1 text-sm text-[var(--color-text-muted)]">
                      {{ t('dashboard.cloudLandingSlotTertiaryTodo') }}
                    </p>
                  </div>
                </div>
              </div>

              <div
                class="flex min-h-[520px] items-center"
                :aria-label="t('dashboard.cloudHeroImageAlt')"
              >
                <img
                  :src="cloudHomeHeroUrl"
                  :alt="t('dashboard.cloudHeroImageAlt')"
                  class="w-full rounded-2xl object-cover shadow-2xl shadow-blue-950/30"
                />
              </div>
            </section>
          </template>
        </div>
      </main>
    </section>

    <DialogRoot :open="changePasswordDialogOpen" @update:open="changePasswordDialogOpen = $event">
      <DialogPortal>
        <DialogOverlay class="dialog-overlay" />
        <DialogContent class="dialog-content">
          <DialogTitle class="dialog-title">{{ t('gateway.changePassword') }}</DialogTitle>
          <form class="mt-4 space-y-3" @submit.prevent="submitChangePassword">
            <input
              v-model="currentPassword"
              type="password"
              class="h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-sm text-[var(--color-text)] outline-none"
              :placeholder="t('gateway.currentPassword')"
            />
            <input
              v-model="newPassword"
              type="password"
              class="h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-sm text-[var(--color-text)] outline-none"
              :placeholder="t('gateway.newPassword')"
            />
            <div class="flex justify-end gap-2 pt-2">
              <DialogClose as-child>
                <button
                  type="button"
                  class="h-8 rounded-md border border-[var(--color-border)] px-3 text-sm text-[var(--color-text)]"
                >
                  {{ t('common.cancel') }}
                </button>
              </DialogClose>
              <button
                type="submit"
                class="h-8 rounded-md border border-blue-700 bg-blue-600 px-3 text-sm text-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="changingPassword"
              >
                {{ changingPassword ? t('gateway.changingPassword') : t('gateway.changePassword') }}
              </button>
            </div>
          </form>
        </DialogContent>
      </DialogPortal>
    </DialogRoot>

    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import {
    ArrowRight,
    Folder,
    FolderOpen,
    Monitor,
    Package,
    Plug,
    Unplug,
    User,
  } from '@lucide/vue'
  import { RouterLink, useRouter } from 'vue-router'
  import cloudHomeHeroUrl from '../assets/cloud-home-hero-candidate.png'
  import { buildAgentPageUrl } from '../config'
  import {
    DialogClose,
    DialogContent,
    DialogOverlay,
    DialogPortal,
    DialogRoot,
    DialogTitle,
    ToastProvider,
  } from 'reka-ui'
  import CloudAccountConnectionMenu from '../components/dashboard/CloudAccountConnectionMenu.vue'
  import CloudAccountMenu from '../components/dashboard/CloudAccountMenu.vue'
  import DisplayControls from '../components/dashboard/DisplayControls.vue'
  import ToastHost from '../components/session/ToastHost.vue'
  import { authMe, disconnectCloud, listWorkspaces } from '../features/agent/api'
  import { authChangePassword, authLogout } from '../features/cloud/api'
  import { cloudOAuthConfigured, startCloudOAuth } from '../features/cloud/oauth'
  import type { Workspace } from '../gen/proto/termbridge/agent/v1/workspace'
  import type { CloudSessionSummary } from '../gen/proto/termbridge/cloud/v1/session'
  import { useAppModeStore } from '../store/appMode'
  import { useGatewayStore } from '../store/gateway'
  import { useNotificationsStore } from '../store/notifications'

  const logoDataUrl =
    'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI0MCIgaGVpZ2h0PSI0MCIgdmlld0JveD0iMCAwIDQwIDQwIj48cmVjdCB3aWR0aD0iNDAiIGhlaWdodD0iNDAiIHJ4PSIxMCIgZmlsbD0iIzI1NjNlYiIvPjx0ZXh0IHg9IjIwIiB5PSIyNSIgZm9udC1zaXplPSIxNCIgZm9udC1mYW1pbHk9IkFyaWFsLCBzYW5zLXNlcmlmIiBmaWxsPSJ3aGl0ZSIgZm9udC13ZWlnaHQ9IjcwMCIgdGV4dC1hbmNob3I9Im1pZGRsZSI+VEI8L3RleHQ+PC9zdmc+'

  const { t } = useI18n()
  const router = useRouter()
  const appMode = useAppModeStore()
  const gateway = useGatewayStore()
  const notifications = useNotificationsStore()
  const connectingCloud = ref(false)
  const localUser = ref('')
  const cloudSession = ref<CloudSessionSummary | null>(null)
  const workspaces = ref<Workspace[]>([])
  const workspacesLoading = ref(false)
  const workspaceError = ref('')
  const changePasswordDialogOpen = ref(false)
  const changingPassword = ref(false)
  const currentPassword = ref('')
  const newPassword = ref('')

  const cloudConnectEnabled = computed(() => cloudOAuthConfigured())
  const cloudConnectionActionLabel = computed(() =>
    cloudSession.value ? t('dashboard.disconnectCloudAccount') : t('dashboard.connectCloudAccount'),
  )
  const cloudConnectionActionTitle = computed(() =>
    cloudSession.value || cloudConnectEnabled.value ? '' : t('dashboard.cloudGateNotConfigured'),
  )
  const localUserLabel = computed(() => localUser.value || t('dashboard.todoLocalUser'))
  const projectVersionLabel = computed(() => t('dashboard.todoProjectVersion'))
  const deviceName = computed(
    () => cloudSession.value?.device_name || cloudSession.value?.device_id || '',
  )
  const cloudUserDisplayName = computed(
    () => gateway.user?.display_name || gateway.user?.email || t('dashboard.signedIn'),
  )
  const agentEntryLabel = computed(() =>
    appMode.frontendMode === 'cloud' ? t('dashboard.openAgentPage') : t('dashboard.switchToAgentMode'),
  )

  async function loadAgentHome() {
    workspacesLoading.value = true
    workspaceError.value = ''
    try {
      await gateway.ensureAgentToken()
      const [me, workspaceResponse] = await Promise.all([authMe(), listWorkspaces()])
      localUser.value = me.user?.display_name || me.user?.email || ''
      cloudSession.value = me.cloud_session ?? null
      workspaces.value = workspaceResponse.data
    } catch (err) {
      workspaceError.value = t('dashboard.loadWorkspacesFailed')
      notifications.notifyError(t('dashboard.loadWorkspacesFailed'), err)
    } finally {
      workspacesLoading.value = false
    }
  }

  async function loadCloudHome() {
    try {
      await gateway.initializeAuth({ force: true })
    } catch (err) {
      notifications.notifyError(t('gateway.loginFailed'), err)
    }
  }

  async function toggleCloudConnection() {
    if (connectingCloud.value) {
      return
    }
    if (cloudSession.value) {
      connectingCloud.value = true
      try {
        await disconnectCloud()
        cloudSession.value = null
      } catch (err) {
        notifications.notifyError(t('dashboard.cloudDisconnectionFailed'), err)
      } finally {
        connectingCloud.value = false
      }
      return
    }
    if (!cloudOAuthConfigured()) {
      notifications.notifyError(
        t('dashboard.cloudGateNotConfigured'),
        new Error('Cloud OAuth is not configured'),
      )
      return
    }
    connectingCloud.value = true
    try {
      startCloudOAuth('/')
    } catch (err) {
      connectingCloud.value = false
      notifications.notifyError(t('dashboard.cloudConnectionFailed'), err)
    }
  }

  async function switchToCloudMode() {
    if (!appMode.setActiveMode('cloud')) {
      return
    }
    await router.push({ name: 'home' })
  }

  async function openAgentEntry() {
    if (appMode.frontendMode === 'cloud') {
      window.open(buildAgentPageUrl('/'), '_blank', 'noopener,noreferrer')
      return
    }
    if (!appMode.setActiveMode('agent')) {
      return
    }
    await router.push({ name: 'home' })
  }

  async function openCloudLogin() {
    await router.push({ name: 'cloud-login', query: { redirect: '/dashboard' } })
  }

  async function openCloudDashboard() {
    await router.push({ name: 'cloud-dashboard' })
  }

  async function logoutCloud() {
    try {
      await authLogout()
    } catch {
      // ignore logout API errors — clear local state anyway
    }
    gateway.clearCloudToken()
    await router.replace({ name: 'home' })
  }

  async function submitChangePassword() {
    if (changingPassword.value) {
      return
    }
    if (!currentPassword.value) {
      notifications.pushToast('error', t('gateway.changePasswordFailed'), t('message.currentPasswordRequired'))
      return
    }
    if (!newPassword.value) {
      notifications.pushToast('error', t('gateway.changePasswordFailed'), t('message.newPasswordRequired'))
      return
    }
    changingPassword.value = true
    try {
      await authChangePassword(currentPassword.value, newPassword.value)
      currentPassword.value = ''
      newPassword.value = ''
      changePasswordDialogOpen.value = false
      notifications.pushToast('success', t('gateway.changePasswordSucceeded'), '')
    } catch (err) {
      notifications.notifyError(t('gateway.changePasswordFailed'), err)
    } finally {
      changingPassword.value = false
    }
  }

  function workspaceUpdatedAt(value: string | undefined) {
    if (!value) {
      return t('dashboard.noActivity')
    }
    return formatTime(value)
  }

  function formatTime(value: string) {
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
  }

  async function loadCurrentHome() {
    if (appMode.effectiveMode === 'agent') {
      await loadAgentHome()
      return
    }
    await loadCloudHome()
  }

  watch(
    () => appMode.effectiveMode,
    () => {
      void loadCurrentHome()
    },
  )

  onMounted(() => {
    void loadCurrentHome()
  })
</script>
