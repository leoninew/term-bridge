<template>
  <AppPageShell
    main-class="flex flex-col overflow-y-auto px-3 py-4 text-sm sm:px-5 sm:py-6 md:px-6 md:py-8"
  >
    <template #actions>
      <CloudAccountMenu
        :authenticated="cloudAuth.authenticated"
        :user-display-name="cloudUserDisplayName"
        :user-email="cloudAuth.user?.email ?? ''"
        @login="openCloudLogin"
        @logout="logoutCloud"
      />
    </template>

    <div class="mx-auto flex w-full max-w-5xl flex-1 flex-col justify-center gap-4 sm:gap-5">
      <section
        class="relative overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl"
      >
        <div
          class="pointer-events-none absolute -right-16 -top-20 size-64 rounded-full bg-blue-500/10 blur-3xl"
          aria-hidden="true"
        />
        <div
          class="pointer-events-none absolute -bottom-24 -left-10 size-56 rounded-full bg-cyan-400/5 blur-3xl"
          aria-hidden="true"
        />

        <div class="relative flex flex-col gap-5 p-4 sm:gap-6 sm:p-6 lg:p-8">
            <p
              v-if="projectVersionLabel"
              class="absolute right-4 top-4 text-xs text-[var(--color-text-subtle)] sm:right-6 sm:top-6 lg:right-8 lg:top-8"
            >
              {{ t('dashboard.cloudCurrentVersion', { version: projectVersionLabel }) }}
            </p>
            <div class="pr-24 sm:pr-28">
              <h1
                class="text-2xl font-semibold leading-tight tracking-tight text-[var(--color-text-strong)] sm:text-3xl"
              >
                {{ t('dashboard.localHomeTitle') }}
              </h1>
              <p
                class="mt-3 max-w-xl text-sm leading-6 text-[var(--color-text-muted)] sm:text-[15px] sm:leading-7"
              >
                {{ t('dashboard.localLandingCopy') }}
              </p>
            </div>

            <div class="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
              <RouterLink
                :to="{ name: 'local-sessions' }"
                class="button button-primary !rounded-xl h-11 w-full justify-center gap-1.5 px-4 text-sm sm:h-9 sm:w-auto sm:px-3"
              >
                {{ t('dashboard.localOpenWorkbench') }}
                <ArrowRight class="size-3.5" />
              </RouterLink>
              <button
                type="button"
                class="button button-secondary !rounded-xl h-11 w-full justify-center gap-1.5 px-4 text-sm sm:h-9 sm:w-auto sm:px-3"
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
                <Unplug v-if="cloudSession.cloudSession" class="size-3.5" />
                <Plug v-else class="size-3.5" />
                {{ cloudConnectionActionLabel }}
              </button>
              <button
                type="button"
                class="button button-secondary !rounded-xl h-11 w-full justify-center gap-1.5 px-4 text-sm sm:h-9 sm:w-auto sm:px-3"
                @click="openCloudPage"
              >
                {{ t('dashboard.openCloudPage') }}
              </button>
            </div>

            <div class="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-[var(--color-text-muted)]">
              <span class="inline-flex min-w-0 items-center gap-1.5">
                <Monitor class="size-3.5 shrink-0 text-[var(--color-text-subtle)]" />
                <span class="truncate">{{ deviceName || t('dashboard.localDeviceUnavailable') }}</span>
              </span>
              <span class="inline-flex min-w-0 items-center gap-1.5">
                <span
                  class="size-2 shrink-0 rounded-full"
                  :class="cloudSession.cloudSession ? 'bg-green-500' : 'bg-[var(--color-text-subtle)]'"
                />
                <span class="truncate">
                  {{
                    cloudSession.cloudSession
                      ? t('dashboard.localCloudConnected')
                      : t('dashboard.localCloudDisconnected')
                  }}
                </span>
              </span>
            </div>
        </div>
      </section>

      <section class="grid grid-cols-1 gap-4 md:grid-cols-2 md:gap-5">
        <section
          class="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl"
        >
          <div
            class="flex flex-col gap-2 border-b border-[var(--color-border)] px-3 py-3 sm:flex-row sm:items-center sm:justify-between sm:gap-3 sm:px-4 sm:py-3.5"
          >
            <h2 class="text-base font-semibold text-[var(--color-text-strong)] sm:text-lg">
              {{ t('dashboard.localWorkspaceListTitle') }}
            </h2>
            <RouterLink
              :to="{ name: 'local-sessions' }"
              class="inline-flex h-10 w-fit items-center gap-1.5 rounded-md px-2.5 text-sm text-[var(--color-text-muted)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)] sm:h-8 sm:px-1.5"
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
              <div class="px-3 py-4 text-sm text-[var(--color-text-muted)] sm:px-4 sm:py-5">
                {{ t('dashboard.loadingWorkspaces') }}
              </div>
            </template>
            <template #error>
              <div class="px-3 py-4 text-sm text-[var(--color-danger-text)] sm:px-4 sm:py-5">
                {{ workspaceError }}
              </div>
            </template>
            <template #empty>
              <div class="px-3 py-4 text-sm text-[var(--color-text-muted)] sm:px-4 sm:py-5">
                {{ t('dashboard.emptyWorkspaces') }}
              </div>
            </template>
            <ul class="divide-y divide-[var(--color-border)]">
              <li v-for="workspace in workspaces" :key="workspace.id">
                <RouterLink
                  :to="{ name: 'local-sessions' }"
                  class="flex min-w-0 items-center justify-between gap-2 px-3 py-3 outline-none transition-colors hover:bg-[var(--color-control-hover)] focus-visible:bg-[var(--color-control-hover)] sm:gap-3 sm:px-4 sm:py-3.5"
                >
                  <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-2">
                      <Folder class="size-4 shrink-0 text-[var(--color-text-subtle)]" />
                      <span class="truncate text-sm text-[var(--color-text-strong)]">
                        {{ workspace.name }}
                      </span>
                    </div>
                    <div
                      class="mt-1 flex min-w-0 flex-col gap-0.5 pl-6 text-sm text-[var(--color-text-muted)] sm:flex-row sm:items-center sm:gap-3"
                    >
                      <span class="truncate">{{ workspace.path }}</span>
                      <span class="shrink-0 text-xs sm:text-sm">
                        {{ workspaceUpdatedAt(workspace.updated_at) }}
                      </span>
                    </div>
                  </div>
                  <ArrowRight
                    class="size-5 shrink-0 text-[var(--color-text-subtle)]"
                    aria-hidden="true"
                  />
                </RouterLink>
              </li>
            </ul>
          </PageStatus>
        </section>

        <section
          class="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl"
        >
          <div
            class="flex flex-col gap-2 border-b border-[var(--color-border)] px-3 py-3 sm:flex-row sm:items-center sm:justify-between sm:gap-3 sm:px-4 sm:py-3.5"
          >
            <h2 class="text-base font-semibold text-[var(--color-text-strong)] sm:text-lg">
              {{ t('shortcut.title') }}
            </h2>
            <RouterLink
              :to="{ name: 'local-shortcuts' }"
              class="inline-flex h-10 w-fit items-center gap-1.5 rounded-md px-2.5 text-sm text-[var(--color-text-muted)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)] sm:h-8 sm:px-1.5"
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
              <div class="px-3 py-4 text-sm text-[var(--color-text-muted)] sm:px-4 sm:py-5">
                {{ t('shortcut.loading') }}
              </div>
            </template>
            <template #error>
              <div class="px-3 py-4 text-sm text-[var(--color-danger-text)] sm:px-4 sm:py-5">
                {{ shortcutError }}
              </div>
            </template>
            <template #empty>
              <div class="px-3 py-4 text-sm text-[var(--color-text-muted)] sm:px-4 sm:py-5">
                {{ t('shortcut.empty') }}
              </div>
            </template>
            <ul class="divide-y divide-[var(--color-border)]">
              <li v-for="shortcut in shortcuts" :key="shortcut.id">
                <RouterLink
                  :to="{ name: 'local-shortcuts' }"
                  class="flex min-w-0 items-center justify-between gap-2 px-3 py-3 outline-none transition-colors hover:bg-[var(--color-control-hover)] focus-visible:bg-[var(--color-control-hover)] sm:gap-3 sm:px-4 sm:py-3.5"
                >
                  <div class="min-w-0 flex-1">
                    <p class="truncate text-sm font-semibold text-[var(--color-text-strong)]">
                      {{ shortcut.name }}
                    </p>
                    <p class="mt-1 truncate text-sm text-[var(--color-text-muted)]">
                      {{ shortcut.command }}
                    </p>
                  </div>
                  <ArrowRight
                    class="size-5 shrink-0 text-[var(--color-text-subtle)]"
                    aria-hidden="true"
                  />
                </RouterLink>
              </li>
            </ul>
          </PageStatus>
        </section>
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
    Plug,
    Unplug,
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
  const projectVersionLabel = computed(() => runtimeConfig.config.version.trim())
  const deviceName = computed(() => localDevice.value?.name || localDevice.value?.id || '')

  async function loadLocalHome() {
    workspaceError.value = ''
    shortcutError.value = ''
    try {
      const me = await agentStatus()
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
