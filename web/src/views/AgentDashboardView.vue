<template>
  <ToastProvider>
    <section class="min-h-screen bg-[var(--color-app-bg)] p-6 text-[var(--color-text)]">
      <div class="mx-auto flex w-full max-w-5xl flex-col gap-6">
        <header class="flex flex-col gap-2">
          <h1 class="text-xl font-semibold text-[var(--color-text-strong)]">
            {{ t('dashboard.agentTitle') }}
          </h1>
          <p class="max-w-3xl text-sm text-[var(--color-text-muted)]">
            {{ t('dashboard.agentFlowDescription') }}
          </p>
        </header>

        <section class="grid gap-3 lg:grid-cols-3">
          <RouterLink
            class="group flex flex-col gap-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-xl transition hover:border-blue-500/60 hover:bg-[var(--color-control-hover)]"
            :to="{ name: 'agent-sessions' }"
          >
            <span
              class="flex size-11 shrink-0 items-center justify-center rounded-lg bg-blue-500/10 text-blue-500"
            >
              <Monitor class="size-5" />
            </span>
            <span class="space-y-1">
              <span class="block text-base font-medium text-[var(--color-text-strong)]">
                {{ t('dashboard.agentSessionsAction') }}
              </span>
              <span class="block text-sm text-[var(--color-text-muted)]">
                {{ t('dashboard.agentSessionsDescription') }}
              </span>
            </span>
          </RouterLink>

          <RouterLink
            v-if="!gateway.cloudToken"
            class="group flex flex-col gap-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-xl transition hover:border-blue-500/60 hover:bg-[var(--color-control-hover)]"
            :to="{ name: 'login', query: { redirect: '/agent/dashboard' } }"
          >
            <span
              class="flex size-11 shrink-0 items-center justify-center rounded-lg bg-blue-500/10 text-blue-500"
            >
              <UserRound class="size-5" />
            </span>
            <span class="space-y-1">
              <span class="block text-base font-medium text-[var(--color-text-strong)]">
                {{ t('dashboard.cloudLoginAction') }}
              </span>
              <span class="block text-sm text-[var(--color-text-muted)]">
                {{ t('dashboard.cloudLoginDescription') }}
              </span>
            </span>
          </RouterLink>
          <div
            v-else
            class="flex flex-col gap-4 rounded-xl border border-emerald-500/40 bg-[var(--color-surface)] p-5 shadow-xl"
          >
            <span
              class="flex size-11 shrink-0 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-500"
            >
              <UserCheck class="size-5" />
            </span>
            <span class="space-y-1">
              <span class="block text-base font-medium text-[var(--color-text-strong)]">
                {{ t('dashboard.cloudLoginActive') }}
              </span>
              <span class="block text-sm text-[var(--color-text-muted)]">
                {{ t('dashboard.cloudLoginActiveDescription') }}
              </span>
            </span>
          </div>

          <button
            type="button"
            class="group flex flex-col gap-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 text-left shadow-xl transition enabled:hover:border-blue-500/60 enabled:hover:bg-[var(--color-control-hover)] disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="!gateway.cloudToken || connectingCloud"
            @click="connectCloud"
          >
            <span
              class="flex size-11 shrink-0 items-center justify-center rounded-lg bg-blue-500/10 text-blue-500"
            >
              <Cloud class="size-5" />
            </span>
            <span class="space-y-1">
              <span class="block text-base font-medium text-[var(--color-text-strong)]">
                {{
                  gateway.cloudSession
                    ? t('dashboard.cloudDeviceConnected')
                    : t('dashboard.connectCloudDeviceAction')
                }}
              </span>
              <span class="block text-sm text-[var(--color-text-muted)]">
                {{ cloudConnectionDescription }}
              </span>
              <span
                v-if="gateway.cloudSession"
                class="block truncate text-xs text-[var(--color-text-subtle)]"
              >
                {{ gateway.cloudSession.gate_url }} · {{ gateway.cloudSession.device_name }}
              </span>
            </span>
          </button>
        </section>
      </div>
    </section>
    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { computed, ref } from 'vue'
  import { RouterLink } from 'vue-router'
  import { Cloud, Monitor, UserCheck, UserRound } from '@lucide/vue'
  import { useI18n } from 'vue-i18n'
  import { ToastProvider } from 'reka-ui'
  import ToastHost from '../components/session/ToastHost.vue'
  import { connectCloudWithCurrentAccount } from '../features/agent/api'
  import { useGatewayStore } from '../store/gateway'
  import { useNotificationsStore } from '../store/notifications'

  const { t } = useI18n()
  const gateway = useGatewayStore()
  const notifications = useNotificationsStore()
  const connectingCloud = ref(false)

  const cloudConnectionDescription = computed(() => {
    if (!gateway.cloudToken) {
      return t('dashboard.connectCloudDeviceRequiresLogin')
    }
    if (gateway.cloudSession) {
      return t('dashboard.connectCloudDeviceConnectedDescription')
    }
    return t('dashboard.connectCloudDeviceDescription')
  })

  async function connectCloud() {
    if (!gateway.cloudToken || connectingCloud.value) {
      return
    }
    connectingCloud.value = true
    try {
      const result = await connectCloudWithCurrentAccount(gateway.cloudToken)
      gateway.setCloudSession(result.cloud_session ?? null)
      await gateway.initializeAuth({ force: true })
    } catch (err) {
      notifications.notifyError(t('gateway.connectFailed'), err)
    } finally {
      connectingCloud.value = false
    }
  }
</script>
