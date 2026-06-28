<template>
  <ToastProvider>
    <section
      class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
    >
      {{ t('gateway.authorizingDeviceBinding') }}
    </section>
    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { onMounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { ToastProvider } from 'reka-ui'
  import ToastHost from '../components/session/ToastHost.vue'
  import { cloudBindingAuthorize } from '../features/gateway/api'
  import { useNotificationsStore } from '../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const notifications = useNotificationsStore()

  onMounted(async () => {
    const callback = typeof route.query.callback === 'string' ? route.query.callback : ''
    const state = typeof route.query.state === 'string' ? route.query.state : ''
    if (!callback || !state) {
      notifications.notifyError(t('gateway.connectFailed'), new Error('missing callback or state'))
      await router.replace({ name: 'sessions' })
      return
    }
    try {
      window.location.href = await cloudBindingAuthorize(callback, state)
    } catch (err) {
      notifications.notifyError(t('gateway.connectFailed'), err)
      await router.replace({ name: 'sessions' })
    }
  })
</script>
