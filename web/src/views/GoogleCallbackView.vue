<template>
  <ToastProvider>
    <section
      class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
    >
      {{ t('gateway.googleSigningIn') }}
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
  import { authGoogleCallback } from '../features/gateway/api'
  import { useGatewayStore } from '../store/gateway'
  import { useNotificationsStore } from '../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const gateway = useGatewayStore()
  const notifications = useNotificationsStore()

  onMounted(async () => {
    const code = typeof route.query.code === 'string' ? route.query.code : ''
    const state = typeof route.query.state === 'string' ? route.query.state : ''
    if (!code || !state) {
      notifications.notifyError(t('gateway.googleLoginFailed'), new Error('missing code or state'))
      await router.replace({ name: 'login' })
      return
    }
    try {
      const response = await authGoogleCallback(code, state)
      gateway.setToken(response.access_token)
      await router.replace({ name: 'sessions' })
    } catch (err) {
      notifications.notifyError(t('gateway.googleLoginFailed'), err)
      await router.replace({ name: 'login' })
    }
  })
</script>
