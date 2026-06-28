<template>
  <ToastProvider>
    <section class="flex min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6">
      <div
        class="w-full max-w-lg rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-6 text-[var(--color-text)] shadow-xl"
      >
        <h1 class="text-xl font-semibold text-[var(--color-text-strong)]">
          {{ t('gateway.connectTitle') }}
        </h1>
        <p class="mt-2 text-sm text-[var(--color-text-muted)]">
          {{ t('gateway.connectDescription') }}
        </p>
        <p
          v-if="statusMessage"
          class="mt-4 rounded border border-[var(--color-border)] bg-[var(--color-surface-muted)] p-3 text-sm text-[var(--color-text-muted)]"
        >
          {{ statusMessage }}
        </p>
        <button
          type="button"
          :disabled="connecting"
          class="mt-6 inline-flex h-9 items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-medium text-white hover:bg-blue-500 disabled:cursor-not-allowed disabled:opacity-60"
          @click="connectCloud"
        >
          {{ connecting ? t('gateway.connectingCloud') : t('gateway.connectCloud') }}
        </button>
      </div>
    </section>
    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { computed, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute } from 'vue-router'
  import { ToastProvider } from 'reka-ui'
  import ToastHost from '../components/session/ToastHost.vue'
  import { cloudBindingStart } from '../features/gateway/api'
  import { useNotificationsStore } from '../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const notifications = useNotificationsStore()
  const connecting = ref(false)

  const statusMessage = computed(() =>
    route.query.status === 'success' ? t('gateway.connectSucceeded') : '',
  )

  async function connectCloud() {
    if (connecting.value) {
      return
    }
    connecting.value = true
    try {
      window.location.href = await cloudBindingStart()
    } catch (err) {
      connecting.value = false
      notifications.notifyError(t('gateway.connectFailed'), err)
    }
  }
</script>
