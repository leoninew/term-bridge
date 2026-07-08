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
          <DisplayControls />
          <CloudAccountMenu
            :authenticated="gateway.authenticated"
            :user-display-name="userDisplayName"
            :user-email="gateway.user?.email ?? ''"
            @login="openCloudLogin"
            @logout="logoutCloud"
            @change-password="changePasswordDialogOpen = true"
          />
        </div>
      </header>

      <main class="flex min-h-[calc(100vh-4rem)] items-center justify-center p-6">
        <div class="flex w-full max-w-4xl flex-col gap-5">
          <section
            class="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl"
          >
            <div class="flex items-center justify-between border-b border-[var(--color-border)] px-5 py-4">
              <h1 class="text-lg font-semibold text-[var(--color-text-strong)]">
                {{ t('dashboard.devicesTitle') }}
              </h1>
              <button
                type="button"
                class="inline-flex h-8 items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-3 text-sm text-[var(--color-text)] outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)] disabled:cursor-not-allowed disabled:text-[var(--color-text-muted)]"
                :disabled="loading"
                @click="refreshDevices"
              >
                <RefreshCw class="size-3.5 text-[var(--color-text-subtle)]" :class="loading ? 'animate-spin' : ''" />
                {{ loading ? t('dashboard.refreshing') : t('dashboard.refreshDevices') }}
              </button>
            </div>

            <div v-if="loading" class="p-5 text-sm text-[var(--color-text-muted)]">
              {{ t('dashboard.loadingDevices') }}
            </div>
            <div v-else-if="deviceError" class="p-5 text-sm text-[var(--color-danger-text)]">
              {{ deviceError }}
            </div>
            <div v-else-if="gateway.devices.length === 0" class="p-5 text-sm text-[var(--color-text-muted)]">
              {{ t('dashboard.emptyTitle') }}
            </div>
            <ul v-else class="divide-y divide-[var(--color-border)]">
              <li
                v-for="device in gateway.devices"
                :key="device.id"
                class="flex min-w-0 items-center justify-between gap-4 px-5 py-4"
              >
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span
                      class="size-2 shrink-0 rounded-full"
                      :class="device.online ? 'bg-green-500' : 'bg-[var(--color-text-subtle)]'"
                    />
                    <Monitor class="size-4 shrink-0 text-[var(--color-text-subtle)]" />
                    <span class="truncate text-sm text-[var(--color-text-strong)]">
                      {{ device.name }}
                    </span>
                    <span class="shrink-0 text-sm text-[var(--color-text-muted)]">
                      {{ device.online ? t('gateway.online') : t('gateway.offline') }}
                    </span>
                  </div>
                  <p class="mt-1 truncate pl-8 text-sm text-[var(--color-text-muted)]">
                    {{ deviceActivity(device) }}
                  </p>
                </div>
                <button
                  type="button"
                  class="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-md border border-blue-700 bg-blue-600 px-3 text-sm text-slate-50 outline-none hover:bg-blue-500 focus:bg-blue-500 disabled:cursor-not-allowed disabled:border-[var(--color-border)] disabled:bg-[var(--color-control-bg)] disabled:text-[var(--color-text-muted)]"
                  :disabled="!device.online"
                  @click="openCloudSessions(device.id)"
                >
                  {{ device.online ? t('dashboard.openWorkbench') : t('dashboard.offlineAction') }}
                  <ArrowRight class="size-3.5" :class="device.online ? 'text-slate-100' : 'text-[var(--color-text-subtle)]'" />
                </button>
              </li>
            </ul>
          </section>
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
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ArrowRight, Monitor, RefreshCw } from '@lucide/vue'
  import { RouterLink, useRouter } from 'vue-router'
  import {
    DialogClose,
    DialogContent,
    DialogOverlay,
    DialogPortal,
    DialogRoot,
    DialogTitle,
    ToastProvider,
  } from 'reka-ui'
  import CloudAccountMenu from '../../components/dashboard/CloudAccountMenu.vue'
  import DisplayControls from '../../components/dashboard/DisplayControls.vue'
  import ToastHost from '../../components/session/ToastHost.vue'
  import { authChangePassword, authLogout } from '../../features/cloud/api'
  import type { DeviceSummary } from '../../gen/proto/termbridge/cloud/v1/device'
  import { useGatewayStore } from '../../store/gateway'
  import { useNotificationsStore } from '../../store/notifications'

  const logoDataUrl =
    'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI0MCIgaGVpZ2h0PSI0MCIgdmlld0JveD0iMCAwIDQwIDQwIj48cmVjdCB3aWR0aD0iNDAiIGhlaWdodD0iNDAiIHJ4PSIxMCIgZmlsbD0iIzI1NjNlYiIvPjx0ZXh0IHg9IjIwIiB5PSIyNSIgZm9udC1zaXplPSIxNCIgZm9udC1mYW1pbHk9IkFyaWFsLCBzYW5zLXNlcmlmIiBmaWxsPSJ3aGl0ZSIgZm9udC13ZWlnaHQ9IjcwMCIgdGV4dC1hbmNob3I9Im1pZGRsZSI+VEI8L3RleHQ+PC9zdmc+'

  const { t } = useI18n()
  const router = useRouter()
  const gateway = useGatewayStore()
  const notifications = useNotificationsStore()
  const loading = ref(false)
  const deviceError = ref('')
  const changePasswordDialogOpen = ref(false)
  const changingPassword = ref(false)
  const currentPassword = ref('')
  const newPassword = ref('')

  const userDisplayName = computed(
    () => gateway.user?.display_name || gateway.user?.email || t('dashboard.signedIn'),
  )

  async function refreshDevices() {
    if (loading.value) {
      return
    }
    loading.value = true
    deviceError.value = ''
    try {
      await gateway.loadDevices()
    } catch (err) {
      deviceError.value = t('dashboard.loadDevicesFailed')
      notifications.notifyError(t('dashboard.loadDevicesFailed'), err)
    } finally {
      loading.value = false
    }
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

  async function openCloudSessions(deviceId: string) {
    if (gateway.selectedDeviceId !== deviceId && !gateway.selectDeviceId(deviceId)) {
      return
    }
    await router.push({ name: 'cloud-sessions', params: { deviceId } })
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
