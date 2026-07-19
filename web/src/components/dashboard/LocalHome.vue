<template>
  <AppPageShell :main-class="homePageMainClass">
    <template #actions>
      <CloudAccountMenu
        :authenticated="cloudAuth.authenticated"
        :user-display-name="cloudUserDisplayName"
        :user-email="cloudAuth.user?.email ?? ''"
        @login="openCloudLogin"
        @logout="logoutCloud"
      />
    </template>

    <div :class="homePageContentClass">
      <HomeHeroBanner :background-url="homeBgUrl" :version="projectVersionLabel">
        <div class="relative max-w-md">
          <h1
            class="text-2xl font-semibold leading-tight tracking-tight text-[var(--color-text-strong)] sm:text-3xl"
          >
            {{ t('dashboard.localHomeTitle') }}
          </h1>
          <p
            class="mt-3 text-sm leading-6 text-[var(--color-text-muted)] sm:text-[15px] sm:leading-7"
          >
            {{ t('dashboard.localLandingCopy') }}
          </p>

          <div class="mt-5 flex flex-col gap-2 sm:mt-6 sm:flex-row sm:flex-wrap sm:items-center">
            <RouterLink
              :to="{ name: 'local-sessions' }"
              :class="[homeCtaLocalClass, 'button-primary']"
            >
              {{ t('dashboard.localOpenWorkbench') }}
              <ArrowRight class="size-3.5" />
            </RouterLink>
            <button
              type="button"
              :class="[homeCtaLocalClass, 'button-secondary']"
              :disabled="
                connectingCloud ||
                (!localCloud.connected && !cloudAuth.cloudToken && !cloudConnectEnabled)
              "
              :title="cloudConnectionActionTitle"
              @click="
                localCloud.connected
                  ? disconnectLocalDeviceFromCloud()
                  : connectLocalDeviceToCloud()
              "
            >
              <Unplug v-if="localCloud.connected" class="size-3.5" />
              <Plug v-else class="size-3.5" />
              {{ cloudConnectionActionLabel }}
            </button>
            <button
              type="button"
              :class="[homeCtaLocalClass, 'button-secondary']"
              @click="openCloudPage"
            >
              {{ t('dashboard.openCloudPage') }}
            </button>
          </div>

          <div
            class="mt-5 flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-[var(--color-text-muted)] sm:mt-6"
          >
            <span class="inline-flex min-w-0 items-center gap-1.5">
              <Monitor class="size-3.5 shrink-0 text-[var(--color-text-subtle)]" />
              <span class="truncate">{{
                deviceName || t('dashboard.localDeviceUnavailable')
              }}</span>
            </span>
            <span class="inline-flex min-w-0 items-center gap-1.5">
              <span
                class="size-2 shrink-0 rounded-full"
                :class="localCloud.connected ? 'bg-green-500' : 'bg-[var(--color-text-subtle)]'"
              />
              <span class="truncate">
                {{
                  localCloud.connected
                    ? t('dashboard.localCloudConnected')
                    : t('dashboard.localCloudDisconnected')
                }}
              </span>
            </span>
          </div>
        </div>
      </HomeHeroBanner>

      <section class="grid grid-cols-1 gap-4 md:grid-cols-2 md:gap-5">
        <HomePreviewPanel
          :title="t('dashboard.localWorkspaceListTitle')"
          :loading="workspacesLoading"
          :error="workspaceError || null"
          :empty="!workspacesLoading && !workspaceError && workspaces.length === 0"
          :loading-text="t('dashboard.loadingWorkspaces')"
          :empty-text="t('dashboard.emptyWorkspaces')"
        >
          <template #action>
            <RouterLink :to="{ name: 'local-sessions' }" :class="homePanelActionClass">
              <FolderOpen class="size-3.5" />
              {{ t('dashboard.openWorkspaceList') }}
            </RouterLink>
          </template>
          <ul class="divide-y divide-[var(--color-border)]">
            <HomePreviewRow
              v-for="workspace in workspaces"
              :key="workspace.id"
              :to="{ name: 'local-sessions' }"
            >
              <div class="flex items-center gap-2">
                <Folder class="size-4 shrink-0 text-[var(--color-text-subtle)]" />
                <span class="truncate text-sm text-[var(--color-text-strong)]">
                  {{ workspace.name }}
                </span>
              </div>
              <div class="mt-1 min-w-0 pl-6 text-sm text-[var(--color-text-muted)]">
                <span class="block truncate">{{ workspace.path }}</span>
              </div>
            </HomePreviewRow>
          </ul>
        </HomePreviewPanel>

        <HomePreviewPanel
          :title="t('shortcut.title')"
          :loading="shortcutsLoading"
          :error="shortcutError || null"
          :empty="!shortcutsLoading && !shortcutError && shortcuts.length === 0"
          :loading-text="t('shortcut.loading')"
          :empty-text="t('shortcut.empty')"
        >
          <template #action>
            <RouterLink :to="{ name: 'local-shortcuts' }" :class="homePanelActionClass">
              <Keyboard class="size-3.5" />
              {{ t('common.more') }}
            </RouterLink>
          </template>
          <ul class="divide-y divide-[var(--color-border)]">
            <HomePreviewRow
              v-for="shortcut in shortcuts"
              :key="shortcut.id"
              :to="{ name: 'local-shortcuts' }"
            >
              <p class="truncate text-sm font-semibold text-[var(--color-text-strong)]">
                {{ shortcut.name }}
              </p>
              <p class="mt-1 truncate text-sm text-[var(--color-text-muted)]">
                {{ shortcut.command }}
              </p>
            </HomePreviewRow>
          </ul>
        </HomePreviewPanel>
      </section>
    </div>
  </AppPageShell>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ArrowRight, Folder, FolderOpen, Keyboard, Monitor, Plug, Unplug } from '@lucide/vue'
  import { RouterLink } from 'vue-router'
  import AppPageShell from '../layout/AppPageShell.vue'
  import homeBgUrl from '../../assets/cloud-home-bg.jpg'
  import { useAsyncAction } from '../../composable/useAsyncAction'
  import { useLocalCloudConnection } from '../../composable/useLocalCloudConnection'
  import CloudAccountMenu from './CloudAccountMenu.vue'
  import HomeHeroBanner from './HomeHeroBanner.vue'
  import HomePreviewPanel from './HomePreviewPanel.vue'
  import HomePreviewRow from './HomePreviewRow.vue'
  import {
    homeCtaLocalClass,
    homePageContentClass,
    homePageMainClass,
    homePanelActionClass,
  } from './homeUi'
  import { listShortcuts, listWorkspaces } from '../../features/local/api'
  import { startCloudOAuth } from '../../features/cloud/oauth'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'
  import type { Workspace } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { DeviceSummary } from '../../gen/proto/termbridge/cloud/v1/device'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useRuntimeConfigStore } from '../../store/runtimeConfig'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const runtimeConfig = useRuntimeConfigStore()
  const cloudAuth = useCloudAuthStore()
  const localCloud = useLocalCloudConnection()
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
  const workspacesLoading = computed(() => workspacesAction.running)
  const shortcutsLoading = computed(() => shortcutsAction.running)
  const connectingCloud = computed(() => localCloud.connecting)
  const cloudConnectEnabled = computed(() => localCloud.oauthConfigured)
  const cloudConnectionActionLabel = computed(() =>
    localCloud.connected
      ? t('dashboard.disconnectCloudAccount')
      : t('dashboard.connectCloudAccount'),
  )
  const cloudConnectionActionTitle = computed(() =>
    localCloud.connected || cloudAuth.cloudToken || cloudConnectEnabled.value
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
      const { device } = await localCloud.hydrateFromAgent()
      localDevice.value = device
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

  function openCloudLogin() {
    startCloudOAuth('/')
  }

  async function logoutCloud() {
    try {
      await localCloud.disconnectSession()
    } catch (err) {
      notifications.notifyError(t('dashboard.cloudDisconnectionFailed'), err)
    } finally {
      cloudAuth.clearToken()
    }
  }

  async function disconnectLocalDeviceFromCloud() {
    await localCloud.disconnectDevice({
      onError: (err) => notifications.notifyError(t('dashboard.cloudDisconnectionFailed'), err),
    })
  }

  async function connectLocalDeviceToCloud() {
    const result = await localCloud.connectDevice({
      onError: (err) => notifications.notifyError(t('dashboard.cloudConnectionFailed'), err),
    })
    if (!result.ok && result.kind === 'not_configured') {
      notifications.notifyError(
        t('dashboard.cloudConnectionNotConfigured'),
        new Error('Cloud OAuth is not configured'),
      )
    }
  }

  function openCloudPage() {
    const cloudDashboardURL = new URL('/dashboard', runtimeConfig.config.cloud.publicUrl)
    window.open(cloudDashboardURL, '_blank', 'noopener,noreferrer')
  }

  onMounted(() => {
    void loadLocalHome()
  })
</script>
