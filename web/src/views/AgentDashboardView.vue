<template>
  <ToastProvider>
    <section class="min-h-screen bg-[var(--color-app-bg)] p-6 text-[var(--color-text)]">
      <div class="mx-auto flex w-full max-w-5xl flex-col gap-6">
        <header class="flex items-center justify-between gap-3">
          <h1 class="text-xl font-semibold text-[var(--color-text-strong)]">
            {{ t('dashboard.agentTitle') }}
          </h1>
        </header>

        <section class="grid gap-3 sm:grid-cols-2">
          <RouterLink
            class="group flex items-center gap-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-xl transition hover:border-blue-500/60 hover:bg-[var(--color-control-hover)]"
            :to="{ name: 'agent-sessions' }"
          >
            <span
              class="flex size-11 shrink-0 items-center justify-center rounded-lg bg-blue-500/10 text-blue-500"
            >
              <Monitor class="size-5" />
            </span>
            <span
              class="min-w-0 flex-1 truncate text-base font-medium text-[var(--color-text-strong)]"
            >
              {{ t('dashboard.agentModeAction') }}
            </span>
          </RouterLink>

          <button
            type="button"
            :disabled="!gateway.capabilities?.cloud_oauth_enabled"
            class="group flex items-center gap-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 text-left shadow-xl transition hover:border-blue-500/60 hover:bg-[var(--color-control-hover)] disabled:cursor-not-allowed disabled:opacity-60"
            @click="startCloudMode"
          >
            <span
              class="flex size-11 shrink-0 items-center justify-center rounded-lg bg-blue-500/10 text-blue-500"
            >
              <Cloud class="size-5" />
            </span>
            <span class="min-w-0 flex-1">
              <span class="block truncate text-base font-medium text-[var(--color-text-strong)]">
                {{ t('dashboard.cloudModeAction') }}
              </span>
              <span
                v-if="gateway.cloudSession"
                class="mt-1 block truncate text-xs text-[var(--color-text-muted)]"
              >
                {{ gateway.cloudSession.gate_url }}
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
  import { RouterLink } from 'vue-router'
  import { Cloud, Monitor } from '@lucide/vue'
  import { useI18n } from 'vue-i18n'
  import { ToastProvider } from 'reka-ui'
  import ToastHost from '../components/session/ToastHost.vue'
  import { cloudOAuthStartURL } from '../features/agent/api'
  import { useGatewayStore } from '../store/gateway'

  const { t } = useI18n()
  const gateway = useGatewayStore()

  function startCloudMode() {
    if (!gateway.capabilities?.cloud_oauth_enabled) {
      return
    }
    window.location.href = cloudOAuthStartURL()
  }
</script>
