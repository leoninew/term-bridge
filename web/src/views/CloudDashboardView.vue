<template>
  <ToastProvider>
    <section class="min-h-screen bg-[var(--color-app-bg)] p-6 text-[var(--color-text)]">
      <div class="mx-auto flex w-full max-w-5xl flex-col gap-6">
        <header class="flex items-center justify-between gap-3">
          <h1 class="text-xl font-semibold text-[var(--color-text-strong)]">
            {{ t('dashboard.title') }}
          </h1>

          <div
            class="inline-flex h-9 max-w-56 items-center gap-2 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 text-sm text-[var(--color-text)]"
            :title="gateway.user?.email || userDisplayName"
          >
            <User class="size-4 shrink-0 text-[var(--color-text-subtle)]" />
            <span class="truncate">{{ userDisplayName }}</span>
          </div>
        </header>

        <section
          v-if="gateway.devices.length === 0"
          class="flex min-h-56 flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-[var(--color-border)] bg-[var(--color-surface)] p-8 text-center shadow-xl"
        >
          <Monitor class="size-8 text-[var(--color-text-subtle)]" />
          <p class="text-sm text-[var(--color-text-muted)]">{{ t('dashboard.emptyTitle') }}</p>
        </section>

        <section
          v-else
          class="overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl"
        >
          <div
            class="flex h-12 items-center justify-between border-b border-[var(--color-border)] px-4"
          >
            <div
              class="inline-flex items-center gap-2 text-sm font-medium text-[var(--color-text-strong)]"
            >
              <Monitor class="size-4 text-[var(--color-text-subtle)]" />
              {{ t('dashboard.devicesTitle') }}
            </div>
            <button
              type="button"
              :disabled="loading"
              class="inline-flex size-8 items-center justify-center rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] text-[var(--color-text)] hover:bg-[var(--color-control-hover)] disabled:cursor-not-allowed disabled:opacity-60"
              :aria-label="loading ? t('dashboard.refreshing') : t('gateway.refreshDevices')"
              :title="loading ? t('dashboard.refreshing') : t('gateway.refreshDevices')"
              @click="refreshDevices"
            >
              <RefreshCw class="size-4" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
          <ul class="divide-y divide-[var(--color-border)]">
            <li
              v-for="device in gateway.devices"
              :key="device.id"
              class="flex items-center justify-between gap-3 p-4"
            >
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span
                    class="size-2 shrink-0 rounded-full"
                    :class="device.online ? 'bg-green-500' : 'bg-[var(--color-text-subtle)]'"
                  />
                  <span class="truncate font-medium text-[var(--color-text-strong)]">
                    {{ device.name }}
                  </span>
                </div>
                <p class="mt-1 truncate pl-4 text-xs text-[var(--color-text-muted)]">
                  {{ deviceActivity(device) }}
                </p>
              </div>
              <button
                type="button"
                :disabled="!device.online"
                class="inline-flex h-8 shrink-0 items-center justify-center rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2.5 text-sm text-[var(--color-text)] hover:bg-[var(--color-control-hover)] disabled:cursor-not-allowed disabled:opacity-60"
                @click="openWorkbench(device.id)"
              >
                {{ device.online ? t('dashboard.openWorkbench') : t('dashboard.offlineAction') }}
              </button>
            </li>
          </ul>
        </section>
      </div>
    </section>
    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useRouter } from 'vue-router'
  import { Monitor, RefreshCw, User } from '@lucide/vue'
  import { useI18n } from 'vue-i18n'
  import { ToastProvider } from 'reka-ui'
  import ToastHost from '../components/session/ToastHost.vue'
  import type { DeviceSummary } from '../features/types'
  import { useGatewayStore } from '../store/gateway'
  import { useNotificationsStore } from '../store/notifications'

  const { t } = useI18n()
  const router = useRouter()
  const gateway = useGatewayStore()
  const notifications = useNotificationsStore()
  const loading = ref(false)
  const userDisplayName = computed(
    () => gateway.user?.display_name || gateway.user?.email || t('dashboard.signedIn'),
  )

  async function refreshDevices() {
    if (loading.value) {
      return
    }
    loading.value = true
    try {
      await gateway.loadDevices()
    } catch (err) {
      notifications.notifyError(t('toast.refreshFailed'), err)
    } finally {
      loading.value = false
    }
  }

  async function openWorkbench(deviceId: string) {
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
