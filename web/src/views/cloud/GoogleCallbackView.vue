<template>
  <section
    class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
  >
    {{ t('cloud.googleSigningIn') }}
  </section>
</template>

<script setup lang="ts">
  import { onMounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { authGoogleCallback } from '../../features/cloud/api'
  import { consumeCloudLoginRedirect } from '../../features/cloud/loginRedirect'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const cloudAuth = useCloudAuthStore()
  const notifications = useNotificationsStore()

  function redirectAfterLogin() {
    return consumeCloudLoginRedirect() || { name: 'cloud-dashboard' }
  }

  onMounted(async () => {
    const code = typeof route.query.code === 'string' ? route.query.code : ''
    const state = typeof route.query.state === 'string' ? route.query.state : ''
    if (!code || !state) {
      notifications.notifyError(t('cloud.googleLoginFailed'), new Error('missing code or state'))
      await router.replace({ name: 'cloud-login' })
      return
    }
    try {
      const response = await authGoogleCallback(code, state)
      cloudAuth.setToken(response.access_token)
      await cloudAuth.initializeAuth({ force: true })
      if (cloudAuth.authenticated) {
        await router.replace(redirectAfterLogin())
      } else {
        await router.replace({ name: 'cloud-login' })
      }
    } catch (err) {
      notifications.notifyError(t('cloud.googleLoginFailed'), err)
      await router.replace({ name: 'cloud-login' })
    }
  })
</script>
