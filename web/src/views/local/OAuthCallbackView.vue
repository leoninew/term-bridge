<template>
  <ToastProvider>
    <section
      class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
    >
      {{ t('cloud.checkingAuth') }}
    </section>
    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { onMounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { ToastProvider } from 'reka-ui'
  import ToastHost from '../../components/session/ToastHost.vue'
  import { assertCloudOAuthState, consumeCloudOAuthRedirect } from '../../features/cloud/oauth'
  import { exchangeOAuthCode } from '../../features/local/api'
  import { useAuthTokensStore } from '../../store/authTokens'
  import { useLocalAuthStore } from '../../store/localAuth'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const authTokens = useAuthTokensStore()
  const localAuth = useLocalAuthStore()
  const notifications = useNotificationsStore()

  onMounted(async () => {
    const code = typeof route.query.code === 'string' ? route.query.code : ''
    const state = typeof route.query.state === 'string' ? route.query.state : ''
    if (!code || !state) {
      notifications.notifyError(
        t('dashboard.cloudConnectionFailed'),
        new Error('missing code or state'),
      )
      await router.replace({ name: 'home' })
      return
    }
    try {
      assertCloudOAuthState(state)
      await localAuth.ensureToken()
      const accessToken = await exchangeOAuthCode(code)
      authTokens.setCloudToken(accessToken)
      await router.replace(consumeCloudOAuthRedirect())
    } catch (err) {
      notifications.notifyError(t('dashboard.cloudConnectionFailed'), err)
      await router.replace({ name: 'home' })
    }
  })
</script>
