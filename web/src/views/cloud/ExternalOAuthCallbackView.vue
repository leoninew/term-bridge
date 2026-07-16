<template>
  <section
    class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
  >
    {{ t('cloud.externalSigningIn') }}
  </section>
</template>

<script setup lang="ts">
  import { onMounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { authExternalCallback } from '../../features/cloud/api'
  import { consumeCloudLoginRedirect } from '../../features/cloud/loginRedirect'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useNotificationsStore } from '../../store/notifications'
  import { useRuntimeConfigStore } from '../../store/runtimeConfig'

  const props = defineProps<{ provider: string }>()

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const cloudAuth = useCloudAuthStore()
  const notifications = useNotificationsStore()
  const runtimeConfig = useRuntimeConfigStore()

  function redirectAfterLogin() {
    return consumeCloudLoginRedirect() || { name: 'cloud-dashboard' }
  }

  onMounted(async () => {
    const providerId = props.provider
    const code = typeof route.query.code === 'string' ? route.query.code : ''
    const state = typeof route.query.state === 'string' ? route.query.state : ''
    if (
      !runtimeConfig.config.cloud.externalAuthProviderIds.includes(providerId) ||
      !code ||
      !state
    ) {
      notifications.notifyError(
        t('cloud.externalLoginFailed'),
        new Error('invalid provider callback'),
      )
      await router.replace({ name: 'cloud-login' })
      return
    }
    try {
      const response = await authExternalCallback(providerId, code, state)
      cloudAuth.setToken(response.access_token)
      await cloudAuth.initialize()
      if (cloudAuth.authenticated) {
        await router.replace(redirectAfterLogin())
      } else {
        await router.replace({ name: 'cloud-login' })
      }
    } catch (err) {
      notifications.notifyError(t('cloud.externalLoginFailed'), err)
      await router.replace({ name: 'cloud-login' })
    }
  })
</script>
