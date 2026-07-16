<template>
  <AppPageShell main-class="flex items-center justify-center px-8 py-8 text-sm 2xl:px-12">
    <template #actions>
      <CloudAccountMenu
        :authenticated="cloudAuth.authenticated"
        :user-display-name="cloudUserDisplayName"
        :user-email="cloudAuth.user?.email ?? ''"
        @login="openCloudLogin"
        @logout="logoutCloud"
      />
    </template>
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
                connectingCloud ||
                (!cloudSession.cloudSession && !cloudAuth.cloudToken && !cloudConnectEnabled)
              "
              :title="cloudConnectionActionTitle"
              @click="
                cloudSession.cloudSession
                  ? disconnectLocalDeviceFromCloud()
                  : connectLocalDeviceToCloud()
              "
            >
              <Unplug
                v-if="cloudSession.cloudSession"
                class="size-3.5 text-[var(--color-text-subtle)]"
              />
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

        <PageStatus
          class="min-w-0"
          :loading="workspacesLoading"
          :error="workspaceError || null"
          :empty="!workspacesLoading && !workspaceError && workspaces.length === 0"
          :loading-text="t('dashboard.loadingWorkspaces')"
          :empty-text="t('dashboard.emptyWorkspaces')"
        >
          <template #loading>
            <div class="p-5 text-sm text-[var(--color-text-muted)]">
              {{ t('dashboard.loadingWorkspaces') }}
            </div>
          </template>
          <template #error>
            <div class="p-5 text-sm text-[var(--color-danger-text)]">{{ workspaceError }}</div>
          </template>
          <template #empty>
            <div class="p-5 text-sm text-[var(--color-text-muted)]">
              {{ t('dashboard.emptyWorkspaces') }}
            </div>
          </template>
          <ul class="divide-y divide-[var(--color-border)]">
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
        </PageStatus>
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

        <PageStatus
          class="min-w-0"
          :loading="shortcutsLoading"
          :error="shortcutError || null"
          :empty="!shortcutsLoading && !shortcutError && shortcuts.length === 0"
          :loading-text="t('shortcut.loading')"
          :empty-text="t('shortcut.empty')"
        >
          <template #loading>
            <div class="p-5 text-sm text-[var(--color-text-muted)]">
              {{ t('shortcut.loading') }}
            </div>
          </template>
          <template #error>
            <div class="p-5 text-sm text-[var(--color-danger-text)]">{{ shortcutError }}</div>
          </template>
          <template #empty>
            <div class="p-5 text-sm text-[var(--color-text-muted)]">{{ t('shortcut.empty') }}</div>
          </template>
          <ul class="grid gap-px bg-[var(--color-border)] sm:grid-cols-2 lg:grid-cols-4">
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
        </PageStatus>
      </section>
    </div>
  </AppPageShell>
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
  import AppPageShell from '../layout/AppPageShell.vue'
  import PageStatus from '../layout/PageStatus.vue'
  import { useAsyncAction } from '../../composable/useAsyncAction'
  import CloudAccountMenu from './CloudAccountMenu.vue'
  import {
    agentStatus,
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
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useCloudSessionStore } from '../../store/cloudSession'
  import { useRuntimeConfigStore } from '../../store/runtimeConfig'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const runtimeConfig = useRuntimeConfigStore()
  const cloudAuth = useCloudAuthStore()
  const cloudSession = useCloudSessionStore()
  const notifications = useNotificationsStore()
  const localUser = ref('')
  const localDevice = ref<DeviceSummary | null>(null)
  const workspaces = ref<Workspace[]>([])
  const workspaceError = ref('')
  const shortcuts = ref<Shortcut[]>([])
  const shortcutError = ref('')
  const workspacesAction = useAsyncAction({
    onError: (err) => {
      workspaceError.value = t('dashboard.loadWorkspacesFailed')
      notifications.notifyError(t('dashboard.loadWorkspacesFailed'), err)
    },
  })
  const shortcutsAction = useAsyncAction({
    onError: (err) => {
      shortcutError.value = t('toast.loadShortcutsFailed')
      notifications.notifyError(t('toast.loadShortcutsFailed'), err)
    },
  })
  const cloudAction = useAsyncAction()
  const workspacesLoading = computed(() => workspacesAction.running)
  const shortcutsLoading = computed(() => shortcutsAction.running)
  const connectingCloud = computed(() => cloudAction.running)

  const cloudConnectEnabled = computed(() => cloudOAuthConfigured())
  const cloudConnectionActionLabel = computed(() =>
    cloudSession.cloudSession
      ? t('dashboard.disconnectCloudAccount')
      : t('dashboard.connectCloudAccount'),
  )
  const cloudConnectionActionTitle = computed(() =>
    cloudSession.cloudSession || cloudAuth.cloudToken || cloudConnectEnabled.value
      ? ''
      : t('dashboard.cloudConnectionNotConfigured'),
  )
  const cloudUserDisplayName = computed(
    () => cloudAuth.user?.display_name || cloudAuth.user?.email || t('dashboard.signedIn'),
  )
  const localUserLabel = computed(() => localUser.value || t('dashboard.todoLocalUser'))
  const projectVersionLabel = computed(() => runtimeConfig.config.version)
  const deviceName = computed(() => localDevice.value?.name || localDevice.value?.id || '')

  async function loadLocalHome() {
    workspaceError.value = ''
    shortcutError.value = ''
    try {
      const me = await agentStatus()
      localUser.value = me.user?.display_name || me.user?.email || ''
      localDevice.value = me.device ?? null
      try {
        await reconcileCloudConnection(me.cloud_session ?? null)
      } catch (err) {
        notifications.notifyError(t('dashboard.cloudConnectionFailed'), err)
      }
      try {
        await cloudAuth.initialize()
      } catch (err) {
        console.warn('cloud identity load failed', err)
      }
    } catch (err) {
      workspaceError.value = t('dashboard.loadWorkspacesFailed')
      notifications.notifyError(t('dashboard.loadWorkspacesFailed'), err)
      return
    }

    await Promise.all([loadWorkspaces(), loadShortcuts()])
  }

  async function loadWorkspaces() {
    workspaceError.value = ''
    await workspacesAction.run(async () => {
      const workspaceResponse = await listWorkspaces()
      workspaces.value = workspaceResponse.data.slice(0, 4)
    })
  }

  async function loadShortcuts() {
    shortcutError.value = ''
    await shortcutsAction.run(async () => {
      shortcuts.value = (await listShortcuts()).slice(0, 4)
    })
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
    const cloudToken = cloudAuth.cloudToken
    if (!cloudToken || cloudAction.running) {
      return false
    }
    const result = await cloudAction.run(async () => {
      const response = await connectCloudWithToken(cloudToken)
      const connectedSession = markCloudConnected(response.cloud_session)
      if (!connectedSession) {
        throw new Error('Cloud connect response missing cloud_session')
      }
      setCloudConnection(connectedSession)
    })
    return result.ok
  }

  function setCloudConnection(summary: CloudSessionSummary | null) {
    cloudSession.setCloudSession(summary)
  }

  function openCloudLogin() {
    startCloudOAuth('/')
  }

  async function logoutCloud() {
    try {
      await disconnectCloudSession()
    } catch (err) {
      notifications.notifyError(t('dashboard.cloudDisconnectionFailed'), err)
    } finally {
      cloudAuth.clearToken()
    }
  }

  async function disconnectCloudSession() {
    await disconnectCloud()
    clearCloudConnectionTime()
    setCloudConnection(null)
  }

  async function disconnectLocalDeviceFromCloud() {
    if (cloudAction.running) {
      return
    }
    await cloudAction.run(
      async () => {
        await disconnectCloudSession()
      },
      {
        onError: (err) => notifications.notifyError(t('dashboard.cloudDisconnectionFailed'), err),
      },
    )
  }

  async function connectLocalDeviceToCloud() {
    if (cloudAction.running) {
      return
    }
    if (cloudAuth.cloudToken) {
      const result = await cloudAction.run(
        async () => {
          const response = await connectCloudWithToken(cloudAuth.cloudToken!)
          const connectedSession = markCloudConnected(response.cloud_session)
          if (!connectedSession) {
            throw new Error('Cloud connect response missing cloud_session')
          }
          setCloudConnection(connectedSession)
        },
        {
          onError: (err) => notifications.notifyError(t('dashboard.cloudConnectionFailed'), err),
        },
      )
      void result
      return
    }
    if (!cloudOAuthConfigured()) {
      notifications.notifyError(
        t('dashboard.cloudConnectionNotConfigured'),
        new Error('Cloud OAuth is not configured'),
      )
      return
    }
    try {
      startCloudOAuth('/')
    } catch (err) {
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
