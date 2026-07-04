<template>
  <ToastProvider>
    <section
      class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
    >
      {{ t('gateway.connectingCloud') }}
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
  import { cloudOAuthStart } from '../features/agent/api'
  import { useNotificationsStore } from '../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const notifications = useNotificationsStore()

  onMounted(async () => {
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : undefined
    try {
      window.location.replace(await cloudOAuthStart(redirect))
    } catch (err) {
      notifications.notifyError(t('gateway.connectFailed'), err)
      await router.replace({ name: 'agent-dashboard' })
    }
  })
</script>
