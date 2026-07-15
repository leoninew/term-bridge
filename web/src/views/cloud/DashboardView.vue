<template>
  <AppPageShell main-class="flex items-center justify-center px-8 py-8 text-sm 2xl:px-12">
    <template #actions>
      <CloudAccountMenu
        :authenticated="cloudAuth.authenticated"
        :user-display-name="userDisplayName"
        :user-email="cloudAuth.user?.email ?? ''"
        :show-change-password="true"
        @login="openCloudLogin"
        @logout="logoutCloud"
        @change-password="openChangePassword"
      />
    </template>
    <div class="flex w-full max-w-[1200px] flex-col gap-5">
      <section
        class="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl"
      >
        <div
          class="flex items-center justify-between border-b border-[var(--color-border)] px-5 py-4"
        >
          <h1 class="text-lg font-semibold text-[var(--color-text-strong)]">
            {{ t('dashboard.devicesTitle') }}
          </h1>
          <div class="flex items-center gap-2">
            <RouterLink
              :to="{ name: 'home' }"
              class="inline-flex h-8 items-center gap-1.5 rounded-md px-1.5 text-sm text-[var(--color-text-muted)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)]"
            >
              {{ t('dashboard.home') }}
            </RouterLink>
            <button
              type="button"
              class="inline-flex h-8 items-center gap-1.5 rounded-md px-1.5 text-sm text-[var(--color-text-muted)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)] disabled:cursor-not-allowed disabled:text-[var(--color-text-muted)]"
              :disabled="loading"
              @click="refreshDevices"
            >
              <RefreshCw class="size-3.5" :class="loading ? 'animate-spin' : ''" />
              {{ loading ? t('dashboard.refreshing') : t('dashboard.refreshDevices') }}
            </button>
          </div>
        </div>

        <div v-if="loading" class="p-5 text-sm text-[var(--color-text-muted)]">
          {{ t('dashboard.loadingDevices') }}
        </div>
        <div v-else-if="deviceError" class="p-5 text-sm text-[var(--color-danger-text)]">
          {{ deviceError }}
        </div>
        <div
          v-else-if="cloudDevices.devices.length === 0"
          class="p-5 text-sm text-[var(--color-text-muted)]"
        >
          {{ t('dashboard.emptyTitle') }}
        </div>
        <ul v-else class="divide-y divide-[var(--color-border)]">
          <li
            v-for="device in cloudDevices.devices"
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
                  {{ device.online ? t('cloud.online') : t('cloud.offline') }}
                </span>
              </div>
              <p class="mt-1 truncate pl-8 text-sm text-[var(--color-text-muted)]">
                {{ deviceActivity(device) }}
              </p>
            </div>
            <button
              type="button"
              class="inline-flex size-8 shrink-0 items-center justify-center rounded-md outline-none disabled:cursor-not-allowed"
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
          </li>
        </ul>
      </section>
    </div>
  </AppPageShell>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ArrowRight, Monitor, RefreshCw } from '@lucide/vue'
  import { RouterLink, useRouter } from 'vue-router'
  import AppPageShell from '../../components/layout/AppPageShell.vue'
  import CloudAccountMenu from '../../components/dashboard/CloudAccountMenu.vue'
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
  const loading = ref(false)
  const deviceError = ref('')

  const userDisplayName = computed(
    () => cloudAuth.user?.display_name || cloudAuth.user?.email || t('dashboard.signedIn'),
  )

  async function refreshDevices() {
    if (loading.value) {
      return
    }
    loading.value = true
    deviceError.value = ''
    try {
      await cloudDevices.loadDevices()
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
