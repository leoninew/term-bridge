<template>
  <section class="min-h-screen bg-[var(--color-app-bg)] text-sm text-[var(--color-text)]">
    <header
      class="flex h-16 items-center justify-between border-b border-[var(--color-border)] bg-[var(--color-panel-header)] px-6"
    >
      <RouterLink to="/" class="flex items-center gap-3">
        <img :src="logoDataUrl" alt="TermBridge" class="size-10 rounded-xl shadow-lg" />
        <p class="text-lg font-semibold text-[var(--color-text-strong)]">TermBridge</p>
      </RouterLink>

      <div class="flex items-center gap-3">
        <CloudAccountConnectionMenu :connection="cloudSession" />
        <DisplayControls />
      </div>
    </header>

    <main class="flex min-h-[calc(100vh-4rem)] items-center justify-center p-6">
      <div class="mx-auto flex w-full max-w-4xl flex-col gap-5">
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
                :disabled="connectingCloud || (!cloudSession && !cloudConnectEnabled)"
                :title="cloudConnectionActionTitle"
                @click="toggleCloudConnection"
              >
                <Unplug v-if="cloudSession" class="size-3.5 text-[var(--color-text-subtle)]" />
                <Plug v-else class="size-3.5 text-[var(--color-text-subtle)]" />
                {{ cloudConnectionActionLabel }}
              </button>
              <button
                v-if="cloudModeSwitchVisible"
                type="button"
                class="inline-flex h-8 items-center gap-1.5 rounded-md px-1.5 text-sm text-[var(--color-text-muted)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)] disabled:cursor-not-allowed disabled:text-[var(--color-text-subtle)]"
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
      </div>
    </main>
  </section>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ArrowRight, Folder, FolderOpen, Monitor, Package, Plug, Unplug, User } from '@lucide/vue'
  import { RouterLink, useRouter } from 'vue-router'
  import CloudAccountConnectionMenu from './CloudAccountConnectionMenu.vue'
  import DisplayControls from './DisplayControls.vue'
  import { authMe, disconnectCloud, listWorkspaces } from '../../features/local/api'
  import { cloudOAuthConfigured, startCloudOAuth } from '../../features/cloud/oauth'
  import type { Workspace } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { DeviceSummary } from '../../gen/proto/termbridge/cloud/v1/device'
  import type { CloudSessionSummary } from '../../gen/proto/termbridge/cloud/v1/session'
  import { useLocalAuthStore } from '../../store/localAuth'
  import { useRuntimeConfigStore } from '../../store/runtimeConfig'
  import { useNotificationsStore } from '../../store/notifications'

  const logoDataUrl =
    'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI0MCIgaGVpZ2h0PSI0MCIgdmlld0JveD0iMCAwIDQwIDQwIj48cmVjdCB3aWR0aD0iNDAiIGhlaWdodD0iNDAiIHJ4PSIxMCIgZmlsbD0iIzI1NjNlYiIvPjx0ZXh0IHg9IjIwIiB5PSIyNSIgZm9udC1zaXplPSIxNCIgZm9udC1mYW1pbHk9IkFyaWFsLCBzYW5zLXNlcmlmIiBmaWxsPSJ3aGl0ZSIgZm9udC13ZWlnaHQ9IjcwMCIgdGV4dC1hbmNob3I9Im1pZGRsZSI+VEI8L3RleHQ+PC9zdmc+'

  const { t } = useI18n()
  const router = useRouter()
  const runtimeConfig = useRuntimeConfigStore()
  const localAuth = useLocalAuthStore()
  const notifications = useNotificationsStore()
  const connectingCloud = ref(false)
  const localUser = ref('')
  const localDevice = ref<DeviceSummary | null>(null)
  const cloudSession = ref<CloudSessionSummary | null>(null)
  const workspaces = ref<Workspace[]>([])
  const workspacesLoading = ref(false)
  const workspaceError = ref('')

  const cloudConnectEnabled = computed(() => cloudOAuthConfigured())
  const cloudConnectionActionLabel = computed(() =>
    cloudSession.value ? t('dashboard.disconnectCloudAccount') : t('dashboard.connectCloudAccount'),
  )
  const cloudConnectionActionTitle = computed(() =>
    cloudSession.value || cloudConnectEnabled.value ? '' : t('dashboard.cloudConnectionNotConfigured'),
  )
  const localUserLabel = computed(() => localUser.value || t('dashboard.todoLocalUser'))
  const projectVersionLabel = computed(() => t('dashboard.todoProjectVersion'))
  const deviceName = computed(() => localDevice.value?.name || localDevice.value?.id || '')
  const cloudModeSwitchVisible = computed(() => runtimeConfig.config.local.mode === 'hybrid')

  async function loadLocalHome() {
    workspacesLoading.value = true
    workspaceError.value = ''
    try {
      await localAuth.ensureToken()
      const [me, workspaceResponse] = await Promise.all([authMe(), listWorkspaces()])
      localUser.value = me.user?.display_name || me.user?.email || ''
      localDevice.value = me.device ?? null
      cloudSession.value = me.cloud_session ?? null
      localAuth.setCloudSession(cloudSession.value)
      workspaces.value = workspaceResponse.data
    } catch (err) {
      workspaceError.value = t('dashboard.loadWorkspacesFailed')
      notifications.notifyError(t('dashboard.loadWorkspacesFailed'), err)
    } finally {
      workspacesLoading.value = false
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
        localAuth.setCloudSession(null)
      } catch (err) {
        notifications.notifyError(t('dashboard.cloudDisconnectionFailed'), err)
      } finally {
        connectingCloud.value = false
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

  async function switchToCloudMode() {
    if (!runtimeConfig.switchMode('cloud')) {
      return
    }
    await router.push({ name: 'home' })
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
