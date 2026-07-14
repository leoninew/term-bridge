<template>
  <section class="min-h-screen bg-[var(--color-app-bg)] text-sm text-[var(--color-text)]">
    <AppHeader>
      <template #actions>
        <CloudAccountConnectionMenu :connection="cloudSession" />
      </template>
    </AppHeader>

    <main class="flex min-h-[calc(100vh-4rem)] items-center justify-center px-8 py-8 2xl:px-12">
      <div class="mx-auto flex w-full max-w-[1200px] flex-col gap-5">
        <section
          class="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl"
        >
          <div
            class="flex items-center justify-between border-b border-[var(--color-border)] px-5 py-4"
          >
            <h1 class="text-lg font-semibold text-[var(--color-text-strong)]">
              {{ t('dashboard.localHomeTitle') }}
            </h1>
            <div class="flex items-center gap-2">
              <button
                type="button"
                class="inline-flex h-8 items-center gap-1.5 rounded-md px-1.5 text-sm text-[var(--color-text-muted)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)] disabled:cursor-not-allowed disabled:text-[var(--color-text-subtle)]"
                :disabled="
                  connectingCloud || (!cloudSession && !tokens.cloudToken && !cloudConnectEnabled)
                "
                :title="cloudConnectionActionTitle"
                @click="toggleCloudConnection"
              >
                <Unplug v-if="cloudSession" class="size-3.5 text-[var(--color-text-subtle)]" />
                <Plug v-else class="size-3.5 text-[var(--color-text-subtle)]" />
                {{ cloudConnectionActionLabel }}
              </button>
              <button
                type="button"
                class="inline-flex h-8 items-center gap-1.5 rounded-md px-1.5 text-sm text-[var(--color-text-muted)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)]"
                @click="openCloudPage"
              >
                {{ t('dashboard.openCloudPage') }}
              </button>
            </div>
          </div>

          <div class="grid gap-4 p-5 lg:grid-cols-3">
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
                    {{ deviceName || t('dashboard.localDeviceUnavailable') }}
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
          <div
            class="flex items-center justify-between border-b border-[var(--color-border)] px-5 py-4"
          >
            <h2 class="text-lg font-semibold text-[var(--color-text-strong)]">
              {{ t('dashboard.localWorkspaceListTitle') }}
            </h2>
            <RouterLink
              :to="{ name: 'local-sessions' }"
              class="inline-flex h-8 items-center gap-1.5 rounded-md px-1.5 text-sm text-[var(--color-text-muted)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)]"
            >
              <FolderOpen class="size-3.5" />
              {{ t('dashboard.openWorkspaceList') }}
            </RouterLink>
          </div>

          <div v-if="workspacesLoading" class="p-5 text-sm text-[var(--color-text-muted)]">
            {{ t('dashboard.loadingWorkspaces') }}
          </div>
          <div v-else-if="workspaceError" class="p-5 text-sm text-[var(--color-danger-text)]">
            {{ workspaceError }}
          </div>
          <div
            v-else-if="workspaces.length === 0"
            class="p-5 text-sm text-[var(--color-text-muted)]"
          >
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
                <div
                  class="mt-1 flex min-w-0 items-center gap-3 pl-6 text-sm text-[var(--color-text-muted)]"
                >
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

        <section
          class="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl"
        >
          <div
            class="flex items-center justify-between border-b border-[var(--color-border)] px-5 py-4"
          >
            <h2 class="text-lg font-semibold text-[var(--color-text-strong)]">
              {{ t('shortcut.title') }}
            </h2>
            <RouterLink
              :to="{ name: 'local-shortcuts' }"
              class="inline-flex h-8 items-center gap-1.5 rounded-md px-1.5 text-sm text-[var(--color-text-muted)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)]"
            >
              <Command class="size-3.5" />
              {{ t('common.more') }}
            </RouterLink>
          </div>

          <div v-if="shortcutsLoading" class="p-5 text-sm text-[var(--color-text-muted)]">
            {{ t('shortcut.loading') }}
          </div>
          <div v-else-if="shortcutError" class="p-5 text-sm text-[var(--color-danger-text)]">
            {{ shortcutError }}
          </div>
          <div
            v-else-if="shortcuts.length === 0"
            class="p-5 text-sm text-[var(--color-text-muted)]"
          >
            {{ t('shortcut.empty') }}
          </div>
          <ul v-else class="grid gap-px bg-[var(--color-border)] sm:grid-cols-2 lg:grid-cols-4">
            <li
              v-for="shortcut in shortcuts"
              :key="shortcut.id"
              class="min-w-0 bg-[var(--color-surface)] px-4 py-3 text-center"
            >
              <p class="truncate text-sm font-semibold text-[var(--color-text-strong)]">
                {{ shortcut.name }}
              </p>
              <p class="mt-1 truncate text-sm text-[var(--color-text-muted)]">
                {{ shortcut.command }}
              </p>
            </li>
          </ul>
        </section>
      </div>
    </main>
  </section>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import {
    ArrowRight,
    Command,
    Folder,
    FolderOpen,
    Monitor,
    Package,
    Plug,
    Unplug,
    User,
  } from '@lucide/vue'
  import { RouterLink } from 'vue-router'
  import AppHeader from '../layout/AppHeader.vue'
  import CloudAccountConnectionMenu from './CloudAccountConnectionMenu.vue'
  import {
    authMe,
    connectCloudWithToken,
    disconnectCloud,
    listShortcuts,
    listWorkspaces,
  } from '../../features/local/api'
  import {
    applyCloudConnectionTime,
    clearCloudConnectionTime,
    cloudOAuthConfigured,
    hasCloudConnectionTime,
    markCloudConnected,
    startCloudOAuth,
  } from '../../features/cloud/oauth'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'
  import type { Workspace } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { DeviceSummary } from '../../gen/proto/termbridge/cloud/v1/device'
  import type { CloudSessionSummary } from '../../gen/proto/termbridge/cloud/v1/session'
  import { useAuthTokensStore } from '../../store/authTokens'
  import { useLocalAuthStore } from '../../store/localAuth'
  import { useRuntimeConfigStore } from '../../store/runtimeConfig'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const runtimeConfig = useRuntimeConfigStore()
  const tokens = useAuthTokensStore()
  const localAuth = useLocalAuthStore()
  const notifications = useNotificationsStore()
  const connectingCloud = ref(false)
  const localUser = ref('')
  const localDevice = ref<DeviceSummary | null>(null)
  const cloudSession = ref<CloudSessionSummary | null>(null)
  const workspaces = ref<Workspace[]>([])
  const workspacesLoading = ref(false)
  const workspaceError = ref('')
  const shortcuts = ref<Shortcut[]>([])
  const shortcutsLoading = ref(false)
  const shortcutError = ref('')

  const cloudConnectEnabled = computed(() => cloudOAuthConfigured())
  const cloudConnectionActionLabel = computed(() =>
    cloudSession.value ? t('dashboard.disconnectCloudAccount') : t('dashboard.connectCloudAccount'),
  )
  const cloudConnectionActionTitle = computed(() =>
    cloudSession.value || tokens.cloudToken || cloudConnectEnabled.value
      ? ''
      : t('dashboard.cloudConnectionNotConfigured'),
  )
  const localUserLabel = computed(() => localUser.value || t('dashboard.todoLocalUser'))
  const projectVersionLabel = computed(() => runtimeConfig.config.version)
  const deviceName = computed(() => localDevice.value?.name || localDevice.value?.id || '')

  async function loadLocalHome() {
    workspacesLoading.value = true
    shortcutsLoading.value = true
    workspaceError.value = ''
    shortcutError.value = ''
    try {
      await localAuth.ensureToken()
      const me = await authMe()
      localUser.value = me.user?.display_name || me.user?.email || ''
      localDevice.value = me.device ?? null
      try {
        await reconcileCloudConnection(me.cloud_session ?? null)
      } catch (err) {
        notifications.notifyError(t('dashboard.cloudConnectionFailed'), err)
      }
    } catch (err) {
      workspaceError.value = t('dashboard.loadWorkspacesFailed')
      notifications.notifyError(t('dashboard.loadWorkspacesFailed'), err)
      workspacesLoading.value = false
      shortcutsLoading.value = false
      return
    }

    await loadWorkspaces()
    await loadShortcuts()
  }

  async function loadWorkspaces() {
    workspacesLoading.value = true
    workspaceError.value = ''
    try {
      const workspaceResponse = await listWorkspaces()
      workspaces.value = workspaceResponse.data.slice(0, 4)
    } catch (err) {
      workspaceError.value = t('dashboard.loadWorkspacesFailed')
      notifications.notifyError(t('dashboard.loadWorkspacesFailed'), err)
    } finally {
      workspacesLoading.value = false
    }
  }

  async function loadShortcuts() {
    shortcutsLoading.value = true
    shortcutError.value = ''
    try {
      shortcuts.value = (await listShortcuts()).slice(0, 4)
    } catch (err) {
      shortcutError.value = t('toast.loadShortcutsFailed')
      notifications.notifyError(t('toast.loadShortcutsFailed'), err)
    } finally {
      shortcutsLoading.value = false
    }
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
    const cloudToken = tokens.cloudToken
    if (!cloudToken || connectingCloud.value) {
      return false
    }
    connectingCloud.value = true
    try {
      const response = await connectCloudWithToken(cloudToken)
      const connectedSession = markCloudConnected(response.cloud_session)
      if (!connectedSession) {
        throw new Error('Cloud connect response missing cloud_session')
      }
      setCloudConnection(connectedSession)
      return true
    } finally {
      connectingCloud.value = false
    }
  }

  function setCloudConnection(summary: CloudSessionSummary | null) {
    cloudSession.value = summary
    localAuth.setCloudSession(summary)
  }

  async function toggleCloudConnection() {
    if (connectingCloud.value) {
      return
    }
    if (cloudSession.value) {
      connectingCloud.value = true
      try {
        await disconnectCloud()
        clearCloudConnectionTime()
        setCloudConnection(null)
      } catch (err) {
        notifications.notifyError(t('dashboard.cloudDisconnectionFailed'), err)
      } finally {
        connectingCloud.value = false
      }
      return
    }
    if (tokens.cloudToken) {
      try {
        await connectStoredCloudToken()
      } catch (err) {
        notifications.notifyError(t('dashboard.cloudConnectionFailed'), err)
      }
      return
    }
    if (!cloudOAuthConfigured()) {
      notifications.notifyError(
        t('dashboard.cloudConnectionNotConfigured'),
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

  function openCloudPage() {
    const cloudDashboardURL = new URL('/dashboard', runtimeConfig.config.cloud.publicUrl)
    window.open(cloudDashboardURL, '_blank', 'noopener,noreferrer')
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

  onMounted(() => {
    void loadLocalHome()
  })
</script>
