<template>
  <AppPageShell
    main-class="flex items-start justify-center px-4 py-5 text-sm sm:items-center sm:px-6 sm:py-8 lg:px-8 2xl:px-12"
  >
    <template #actions>
      <CloudAccountMenu
        :authenticated="cloudAuth.authenticated"
        :user-display-name="userDisplayName"
        :user-email="cloudAuth.user?.email ?? ''"
        :can-change-password="cloudAuth.user?.provider === 'email'"
        @login="openCloudLogin"
        @logout="logoutCloud"
        @change-password="openChangePassword"
      />
    </template>
    <div class="mx-auto flex w-full max-w-[1200px] flex-col gap-5">
      <section
        class="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl"
      >
        <div
          class="flex flex-col gap-3 border-b border-[var(--color-border)] px-4 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-5"
        >
          <h1 class="text-base font-semibold text-[var(--color-text-strong)] sm:text-lg">
            {{ t('dashboard.devicesTitle') }}
          </h1>
          <div class="flex flex-wrap items-center gap-1 sm:gap-2">
            <RouterLink
              :to="{ name: 'home' }"
              class="inline-flex h-10 items-center gap-1.5 rounded-md px-2.5 text-sm text-[var(--color-text-muted)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)] sm:h-8 sm:px-1.5"
            >
              {{ t('dashboard.home') }}
            </RouterLink>
            <button
              type="button"
              class="inline-flex h-10 items-center gap-1.5 rounded-md px-2.5 text-sm text-[var(--color-text-muted)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)] disabled:cursor-not-allowed disabled:text-[var(--color-text-muted)] sm:h-8 sm:px-1.5"
              :disabled="devicesAction.running"
              @click="refreshDevices"
            >
              <RefreshCw class="size-3.5" :class="devicesAction.running ? 'animate-spin' : ''" />
              {{
                devicesAction.running ? t('dashboard.refreshing') : t('dashboard.refreshDevices')
              }}
            </button>
          </div>
        </div>

        <PageStatus
          class="min-w-0"
          :loading="devicesAction.running"
          :error="deviceError || null"
          :empty="!devicesAction.running && !deviceError && cloudDevices.devices.length === 0"
          :loading-text="t('dashboard.loadingDevices')"
          :empty-text="t('dashboard.emptyTitle')"
        >
          <template #loading>
            <div class="p-4 text-sm text-[var(--color-text-muted)] sm:p-5">
              {{ t('dashboard.loadingDevices') }}
            </div>
          </template>
          <template #error>
            <div class="p-4 text-sm text-[var(--color-danger-text)] sm:p-5">{{ deviceError }}</div>
          </template>
          <template #empty>
            <div class="p-4 text-sm text-[var(--color-text-muted)] sm:p-5">
              {{ t('dashboard.emptyTitle') }}
            </div>
          </template>
          <ul class="divide-y divide-[var(--color-border)]">
            <li
              v-for="device in cloudDevices.devices"
              :key="device.id"
              class="flex min-w-0 items-center justify-between gap-3 px-4 py-3.5 sm:gap-4 sm:px-5 sm:py-4"
            >
              <div class="min-w-0 flex-1">
                <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
                  <span
                    class="size-2 shrink-0 rounded-full"
                    :class="device.online ? 'bg-green-500' : 'bg-[var(--color-text-subtle)]'"
                  />
                  <Monitor class="size-4 shrink-0 text-[var(--color-text-subtle)]" />
                  <span class="min-w-0 truncate text-sm text-[var(--color-text-strong)]">
                    {{ device.name }}
                  </span>
                  <span class="shrink-0 text-xs text-[var(--color-text-muted)] sm:text-sm">
                    {{ device.online ? t('cloud.online') : t('cloud.offline') }}
                  </span>
                </div>
                <p
                  class="mt-1 truncate pl-6 text-xs text-[var(--color-text-muted)] sm:pl-8 sm:text-sm"
                >
                  {{ deviceActivity(device) }}
                </p>
              </div>
              <div class="flex shrink-0 items-center gap-1">
                <button
                  v-if="!device.online"
                  type="button"
                  class="inline-flex size-10 items-center justify-center rounded-md text-[var(--color-text-muted)] outline-none hover:bg-[var(--color-surface-muted)] hover:text-[var(--color-danger-text)] focus:bg-[var(--color-surface-muted)] focus:text-[var(--color-danger-text)] disabled:cursor-not-allowed disabled:opacity-60 sm:size-8"
                  :disabled="deletingDeviceId === device.id || deleteDeviceAction.running"
                  :aria-label="t('dashboard.deleteDeviceAria', { name: device.name })"
                  :title="t('dashboard.deleteOfflineDevice')"
                  @click.stop="openDeleteDevice(device)"
                >
                  <Trash2 class="size-4" />
                </button>
                <button
                  type="button"
                  class="inline-flex size-10 items-center justify-center rounded-md outline-none disabled:cursor-not-allowed sm:size-8"
                  :disabled="!device.online"
                  :aria-label="t('dashboard.openWorkbench')"
                  @click="openCloudSessions(device.id)"
                >
                  <ArrowRight
                    class="size-5"
                    :class="
                      device.online
                        ? 'text-[var(--color-text-subtle)]'
                        : 'text-[var(--color-text-muted)]'
                    "
                  />
                </button>
              </div>
            </li>
          </ul>
        </PageStatus>
      </section>
    </div>
  </AppPageShell>

  <DeleteDeviceDialog
    :open="deleteDeviceDialogOpen"
    :device="selectedDevice"
    :deleting="!!deletingDeviceId || deleteDeviceAction.running"
    @update:open="setDeleteDeviceDialogOpen"
    @confirm="deleteSelectedDevice"
  />
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ArrowRight, Monitor, RefreshCw, Trash2 } from '@lucide/vue'
  import { RouterLink, useRouter } from 'vue-router'
  import AppPageShell from '../../components/layout/AppPageShell.vue'
  import PageStatus from '../../components/layout/PageStatus.vue'
  import { useAsyncAction } from '../../composable/useAsyncAction'
  import CloudAccountMenu from '../../components/dashboard/CloudAccountMenu.vue'
  import DeleteDeviceDialog from '../../components/dashboard/DeleteDeviceDialog.vue'
  import { authLogout } from '../../features/cloud/api'
  import type { DeviceSummary } from '../../gen/proto/termbridge/cloud/v1/device'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useCloudDevicesStore } from '../../store/cloudDevices'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const router = useRouter()
  const cloudAuth = useCloudAuthStore()
  const cloudDevices = useCloudDevicesStore()
  const notifications = useNotificationsStore()
  const deviceError = ref('')
  const deleteDeviceDialogOpen = ref(false)
  const selectedDevice = ref<DeviceSummary | null>(null)
  const deletingDeviceId = ref<string | null>(null)
  const devicesAction = useAsyncAction({
    onError: (err) => {
      deviceError.value = t('dashboard.loadDevicesFailed')
      notifications.notifyError(t('dashboard.loadDevicesFailed'), err)
    },
  })
  const deleteDeviceAction = useAsyncAction({
    onError: (err) => {
      notifications.notifyError(t('toast.deleteDeviceFailed'), err)
    },
  })

  const userDisplayName = computed(
    () => cloudAuth.user?.display_name || cloudAuth.user?.email || t('dashboard.signedIn'),
  )

  async function refreshDevices() {
    if (devicesAction.running) {
      return
    }
    deviceError.value = ''
    await devicesAction.run(async () => {
      await cloudDevices.loadDevices()
    })
  }

  async function openCloudLogin() {
    await router.push({ name: 'cloud-login', query: { redirect: '/dashboard' } })
  }

  async function logoutCloud() {
    try {
      await authLogout()
    } catch {
      // ignore logout API errors — clear local state anyway
    }
    cloudAuth.clearToken()
    await router.replace({ name: 'home' })
  }

  async function openChangePassword() {
    await router.push({ name: 'cloud-change-password' })
  }

  async function openCloudSessions(deviceId: string) {
    if (cloudDevices.selectedDeviceId !== deviceId && !cloudDevices.selectDeviceId(deviceId)) {
      return
    }
    await router.push({ name: 'cloud-sessions', params: { deviceId } })
  }

  function openDeleteDevice(device: DeviceSummary) {
    if (device.online || deletingDeviceId.value || deleteDeviceAction.running) {
      return
    }
    selectedDevice.value = device
    deleteDeviceDialogOpen.value = true
  }

  function setDeleteDeviceDialogOpen(open: boolean) {
    deleteDeviceDialogOpen.value = open
    // Keep selectedDevice while confirming: AlertDialogAction closes the dialog
    // before the parent confirm handler finishes reading the device.
    if (!open && !deletingDeviceId.value && !deleteDeviceAction.running) {
      selectedDevice.value = null
    }
  }

  async function deleteSelectedDevice() {
    const device = selectedDevice.value
    if (!device || device.online || deletingDeviceId.value || deleteDeviceAction.running) {
      return
    }
    // Capture first so dialog close from AlertDialogAction cannot clear the target.
    const target = device
    deletingDeviceId.value = target.id
    try {
      await deleteDeviceAction.run(
        async () => {
          const removed = await cloudDevices.removeDevice(target.id)
          if (!removed) {
            throw new Error(t('dialog.deleteOnlineDeviceBlocked'))
          }
          deleteDeviceDialogOpen.value = false
          selectedDevice.value = null
          notifications.pushToast(
            'success',
            t('toast.deviceDeleted'),
            t('message.deviceDeleted', { name: target.name }),
          )
        },
        { rethrow: true },
      )
    } catch {
      // notify handled by deleteDeviceAction.onError
    } finally {
      deletingDeviceId.value = null
      if (!deleteDeviceDialogOpen.value) {
        selectedDevice.value = null
      }
    }
  }

  function deviceActivity(device: DeviceSummary) {
    if (device.online && device.connected_at) {
      return t('dashboard.connectedAt', { value: formatTime(device.connected_at) })
    }
    if (device.last_seen) {
      return t('dashboard.lastSeenAt', { value: formatTime(device.last_seen) })
    }
    return t('dashboard.noActivity')
  }

  function formatTime(value: string) {
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
  }

  onMounted(() => {
    void refreshDevices()
  })
</script>
