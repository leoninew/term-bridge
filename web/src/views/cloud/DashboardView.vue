<template>
  <AppPageShell :main-class="dashboardPageMainClass">
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

    <div :class="dashboardPageContentClass">
      <HomePreviewPanel
        heading="h1"
        :title="t('dashboard.devicesTitle')"
        :loading="devicesAction.running"
        :error="deviceError || null"
        :empty="!devicesAction.running && !deviceError && cloudDevices.devices.length === 0"
        :loading-text="t('dashboard.loadingDevices')"
        :empty-text="t('dashboard.emptyTitle')"
      >
        <template #action>
          <div class="flex flex-wrap items-center gap-1 sm:gap-2">
            <RouterLink :to="{ name: 'home' }" :class="homePanelActionClass">
              {{ t('dashboard.home') }}
            </RouterLink>
            <button
              type="button"
              :class="[
                homePanelActionClass,
                'disabled:cursor-not-allowed disabled:text-[var(--color-text-muted)]',
              ]"
              :disabled="devicesAction.running"
              @click="refreshDevices"
            >
              <RefreshCw class="size-3.5" :class="devicesAction.running ? 'animate-spin' : ''" />
              {{
                devicesAction.running ? t('dashboard.refreshing') : t('dashboard.refreshDevices')
              }}
            </button>
          </div>
        </template>

        <ul class="divide-y divide-[var(--color-border)]">
          <DeviceListItem
            v-for="device in cloudDevices.devices"
            :key="device.id"
            :device="device"
            :deleting="deletingDeviceId === device.id || deleteDeviceAction.running"
            @delete="openDeleteDevice"
            @open="(item) => openCloudSessions(item.id)"
          />
        </ul>
      </HomePreviewPanel>
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
  import { RefreshCw } from '@lucide/vue'
  import { RouterLink, useRouter } from 'vue-router'
  import AppPageShell from '../../components/layout/AppPageShell.vue'
  import { useAsyncAction } from '../../composable/useAsyncAction'
  import CloudAccountMenu from '../../components/dashboard/CloudAccountMenu.vue'
  import DeleteDeviceDialog from '../../components/dashboard/DeleteDeviceDialog.vue'
  import DeviceListItem from '../../components/dashboard/DeviceListItem.vue'
  import HomePreviewPanel from '../../components/dashboard/HomePreviewPanel.vue'
  import {
    dashboardPageContentClass,
    dashboardPageMainClass,
    homePanelActionClass,
  } from '../../components/dashboard/homeUi'
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
      // ignore logout API errors ? clear local state anyway
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

  onMounted(() => {
    void refreshDevices()
  })
</script>
