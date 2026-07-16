<template>
  <section
    class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
  >
    {{ t('cloud.checkingAuth') }}
  </section>
</template>

<script setup lang="ts">
  import { onMounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { assertCloudOAuthState, consumeCloudOAuthRedirect } from '../../features/cloud/oauth'
  import { exchangeOAuthCode } from '../../features/local/api'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const cloudAuth = useCloudAuthStore()
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
      const accessToken = await exchangeOAuthCode(code)
      cloudAuth.setToken(accessToken)
      await cloudAuth.initialize()
      await router.replace(consumeCloudOAuthRedirect())
    } catch (err) {
      notifications.notifyError(t('dashboard.cloudConnectionFailed'), err)
      await router.replace({ name: 'home' })
    }
  })
</script>
