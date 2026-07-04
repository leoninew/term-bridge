<template>
  <ToastProvider>
    <section
      class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
    >
      {{ t('gateway.authorizingCloudConnect') }}
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
  import { cloudOAuthAuthorize } from '../features/cloud/api'
  import { useNotificationsStore } from '../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const notifications = useNotificationsStore()

  onMounted(async () => {
    const clientId = typeof route.query.client_id === 'string' ? route.query.client_id : ''
    const redirectUri = typeof route.query.redirect_uri === 'string' ? route.query.redirect_uri : ''
    const state = typeof route.query.state === 'string' ? route.query.state : ''
    if (!clientId || !redirectUri || !state) {
      notifications.notifyError(
        t('gateway.connectFailed'),
        new Error('missing client_id, redirect_uri or state'),
      )
      await router.replace({ name: 'agent-dashboard' })
      return
    }
    try {
      window.location.href = await cloudOAuthAuthorize(clientId, redirectUri, state)
    } catch (err) {
      notifications.notifyError(t('gateway.connectFailed'), err)
      await router.replace({ name: 'agent-dashboard' })
    }
  })
</script>
