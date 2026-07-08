<template>
  <ToastProvider>
    <section class="min-h-screen bg-[var(--color-app-bg)] text-sm text-[var(--color-text)]">
      <header
        class="flex h-16 items-center justify-between border-b border-[var(--color-border)] bg-[var(--color-panel-header)] px-6"
      >
        <div class="flex items-center gap-3">
          <img :src="logoDataUrl" alt="TermBridge" class="size-10 rounded-xl shadow-lg" />
          <p class="text-lg font-semibold text-[var(--color-text-strong)]">TermBridge</p>
        </div>

        <div class="flex items-center gap-3">
          <CloudAccountConnectionMenu :connection="cloudSession" />
          <DisplayControls />
        </div>
      </header>

      <main class="flex min-h-[calc(100vh-4rem)] items-center justify-center p-6">
        <div class="flex w-full max-w-4xl flex-col gap-5">
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
                  :disabled="cloudSession !== null || !cloudConnectEnabled"
                  :title="cloudConnectionActionTitle"
                  @click="connectCloud"
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
                  <Cloud class="size-3.5 text-[var(--color-text-subtle)]" />
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
                class="inline-flex h-8 items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-3 text-sm text-[var(--color-text)] outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
              >
                <FolderOpen class="size-3.5 text-[var(--color-text-subtle)]" />
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
        </div>
      </main>
    </section>
    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ArrowRight, Cloud, Folder, FolderOpen, Monitor, Package, Plug, Unplug, User } from '@lucide/vue'
  import { RouterLink, useRouter } from 'vue-router'
  import { ToastProvider } from 'reka-ui'
  import CloudAccountConnectionMenu from '../../components/dashboard/CloudAccountConnectionMenu.vue'
  import DisplayControls from '../../components/dashboard/DisplayControls.vue'
  import ToastHost from '../../components/session/ToastHost.vue'
  import { authMe, listWorkspaces } from '../../features/agent/api'
  import { cloudOAuthConfigured, startCloudOAuth } from '../../features/cloud/oauth'
  import type { Workspace } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { CloudSessionSummary } from '../../gen/proto/termbridge/cloud/v1/session'
  import { useAppModeStore } from '../../store/appMode'
  import { useGatewayStore } from '../../store/gateway'
  import { useNotificationsStore } from '../../store/notifications'

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

  const cloudConnectEnabled = computed(() => cloudOAuthConfigured())
  const cloudConnectionActionLabel = computed(() =>
    cloudSession.value ? t('dashboard.disconnectCloudAccount') : t('dashboard.connectCloudAccount'),
  )
  const cloudConnectionActionTitle = computed(() => {
    if (cloudSession.value) {
      return t('dashboard.todoDisconnectCloud')
    }
    return cloudConnectEnabled.value ? '' : t('dashboard.cloudGateNotConfigured')
  })
  const localUserLabel = computed(() => localUser.value || t('dashboard.todoLocalUser'))
  const projectVersionLabel = computed(() => t('dashboard.todoProjectVersion'))
  const deviceName = computed(
    () => cloudSession.value?.device_name || cloudSession.value?.device_id || '',
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

  function connectCloud() {
    if (connectingCloud.value) {
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
      startCloudOAuth('/agent/dashboard')
    } catch (err) {
      connectingCloud.value = false
      notifications.notifyError(t('dashboard.cloudConnectionFailed'), err)
    }
  }

  async function switchToCloudMode() {
    if (!appMode.setActiveMode('cloud')) {
      return
    }
    await gateway.initializeAuth({ force: true })
    await router.push(gateway.authenticated ? { name: 'cloud-dashboard' } : { name: 'cloud-login' })
  }

  function workspaceUpdatedAt(value: string | undefined) {
    if (!value) {
      return t('dashboard.noActivity')
    }
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
  }

  onMounted(() => {
    void loadAgentHome()
  })
</script>
