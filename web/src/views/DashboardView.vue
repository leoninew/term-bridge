<template>
  <ToastProvider>
    <section class="min-h-screen bg-[var(--color-app-bg)] p-6 text-[var(--color-text)]">
      <div class="mx-auto flex w-full max-w-5xl flex-col gap-6">
        <header class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <p class="text-sm text-[var(--color-text-muted)]">
              {{ t(isLocalMode ? 'dashboard.localKicker' : 'dashboard.kicker') }}
            </p>
            <h1 class="mt-1 text-2xl font-semibold text-[var(--color-text-strong)]">
              {{ t(isLocalMode ? 'dashboard.localTitle' : 'dashboard.title') }}
            </h1>
            <p class="mt-2 max-w-2xl text-sm text-[var(--color-text-muted)]">
              {{ t(isLocalMode ? 'dashboard.localDescription' : 'dashboard.description') }}
            </p>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <button
              type="button"
              :disabled="loading"
              class="inline-flex h-9 items-center justify-center rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-3 text-sm text-[var(--color-text)] hover:bg-[var(--color-control-hover)] disabled:cursor-not-allowed disabled:opacity-60"
              @click="refreshDevices"
            >
              {{ loading ? t('dashboard.refreshing') : t('gateway.refreshDevices') }}
            </button>
            <RouterLink
              v-if="isLocalMode"
              class="inline-flex h-9 items-center justify-center rounded-md bg-blue-600 px-3 text-sm font-medium text-white hover:bg-blue-500"
              :to="{ name: 'sessions' }"
            >
              {{ t('dashboard.openLocalWorkbench') }}
            </RouterLink>
            <button
              v-else
              type="button"
              class="inline-flex h-9 items-center justify-center rounded-md bg-blue-600 px-3 text-sm font-medium text-white hover:bg-blue-500"
              @click="showConnectGuide = true"
            >
              {{ t('dashboard.addDevice') }}
            </button>
          </div>
        </header>

        <section
          v-if="isLocalMode"
          class="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-6 shadow-xl"
        >
          <h2 class="text-lg font-semibold text-[var(--color-text-strong)]">
            {{ t('dashboard.localDeviceTitle') }}
          </h2>
          <p class="mt-2 max-w-2xl text-sm text-[var(--color-text-muted)]">
            {{ t('dashboard.localDeviceDescription') }}
          </p>
          <div class="mt-5 flex flex-wrap gap-2">
            <RouterLink
              class="inline-flex h-9 items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-medium text-white hover:bg-blue-500"
              :to="{ name: 'sessions' }"
            >
              {{ t('dashboard.openLocalWorkbench') }}
            </RouterLink>
            <button
              v-if="gateway.capabilities?.cloud_connect_enabled"
              type="button"
              class="inline-flex h-9 items-center justify-center rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-4 text-sm text-[var(--color-text)] hover:bg-[var(--color-control-hover)]"
              @click="connectCloud"
            >
              {{ t('dashboard.connectCloudAccount') }}
            </button>
          </div>
        </section>

        <template v-else>
          <section
            v-if="gateway.devices.length === 0"
            class="rounded-xl border border-dashed border-[var(--color-border)] bg-[var(--color-surface)] p-6 shadow-xl"
          >
            <h2 class="text-lg font-semibold text-[var(--color-text-strong)]">
              {{ t('dashboard.emptyTitle') }}
            </h2>
            <p class="mt-2 max-w-2xl text-sm text-[var(--color-text-muted)]">
              {{ t('dashboard.emptyDescription') }}
            </p>
            <button
              type="button"
              class="mt-5 inline-flex h-9 items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-medium text-white hover:bg-blue-500"
              @click="showConnectGuide = true"
            >
              {{ t('dashboard.addDevice') }}
            </button>
          </section>

          <section
            v-else
            class="overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl"
          >
            <div class="border-b border-[var(--color-border)] p-4">
              <h2 class="text-lg font-semibold text-[var(--color-text-strong)]">
                {{ t('dashboard.devicesTitle') }}
              </h2>
              <p class="mt-1 text-sm text-[var(--color-text-muted)]">
                {{ t('dashboard.devicesDescription') }}
              </p>
            </div>
            <ul class="divide-y divide-[var(--color-border)]">
              <li
                v-for="device in gateway.devices"
                :key="device.id"
                class="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between"
              >
                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="truncate font-medium text-[var(--color-text-strong)]">
                      {{ device.name }}
                    </span>
                    <span
                      class="rounded-full px-2 py-0.5 text-xs"
                      :class="
                        device.online
                          ? 'bg-green-500/10 text-green-500'
                          : 'bg-[var(--color-surface-muted)] text-[var(--color-text-muted)]'
                      "
                    >
                      {{ device.online ? t('gateway.online') : t('gateway.offline') }}
                    </span>
                  </div>
                  <p class="mt-1 text-xs text-[var(--color-text-muted)]">
                    {{ deviceActivity(device) }}
                  </p>
                </div>
                <button
                  type="button"
                  :disabled="!device.online"
                  class="inline-flex h-9 items-center justify-center rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-3 text-sm text-[var(--color-text)] hover:bg-[var(--color-control-hover)] disabled:cursor-not-allowed disabled:opacity-60"
                  @click="openWorkbench(device.id)"
                >
                  {{ device.online ? t('dashboard.openWorkbench') : t('dashboard.offlineAction') }}
                </button>
              </li>
            </ul>
          </section>

          <section
            v-if="showConnectGuide"
            class="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-xl"
          >
            <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-[var(--color-text-strong)]">
                  {{ t('dashboard.connectGuideTitle') }}
                </h2>
                <p class="mt-2 max-w-2xl text-sm text-[var(--color-text-muted)]">
                  {{ t('dashboard.connectGuideDescription') }}
                </p>
              </div>
              <button
                type="button"
                class="inline-flex h-8 shrink-0 items-center justify-center rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-3 text-sm text-[var(--color-text)] hover:bg-[var(--color-control-hover)]"
                @click="showConnectGuide = false"
              >
                {{ t('common.close') }}
              </button>
            </div>
            <ol class="mt-4 list-decimal space-y-2 pl-5 text-sm text-[var(--color-text-muted)]">
              <li>{{ t('dashboard.connectStepServe') }}</li>
              <li>{{ t('dashboard.connectStepCloud') }}</li>
              <li>{{ t('dashboard.connectStepRefresh') }}</li>
            </ol>
            <div class="mt-4">
              <button
                type="button"
                :disabled="loading"
                class="inline-flex h-9 items-center justify-center rounded-md bg-blue-600 px-3 text-sm font-medium text-white hover:bg-blue-500 disabled:cursor-not-allowed disabled:opacity-60"
                @click="refreshDevices"
              >
                {{ t('gateway.refreshDevices') }}
              </button>
            </div>
          </section>
        </template>
      </div>
    </section>
    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { RouterLink, useRouter } from 'vue-router'
  import { useI18n } from 'vue-i18n'
  import { ToastProvider } from 'reka-ui'
  import ToastHost from '../components/session/ToastHost.vue'
  import { cloudConnectStartURL, type DeviceSummary } from '../features/gateway/api'
  import { useGatewayStore } from '../store/gateway'
  import { useNotificationsStore } from '../store/notifications'

  const { t } = useI18n()
  const router = useRouter()
  const gateway = useGatewayStore()
  const notifications = useNotificationsStore()
  const loading = ref(false)
  const showConnectGuide = ref(false)
  const isLocalMode = computed(() => gateway.capabilities?.mode === 'local')

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
    await router.push({ name: 'sessions' })
  }

  function connectCloud() {
    window.location.href = cloudConnectStartURL()
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
