<template>
  <ToastProvider>
    <section
      v-if="!cloudAuth.authInitialized"
      class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
    >
      {{ t('cloud.checkingAuth') }}
    </section>
    <section
      v-else
      class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
    >
      {{ t('dashboard.signInWithOAuth') }}
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
  import { cloudOAuthAuthorize } from '../../features/cloud/api'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const cloudAuth = useCloudAuthStore()
  const notifications = useNotificationsStore()

  onMounted(async () => {
    await cloudAuth.initializeAuth()
    if (!cloudAuth.authenticated) {
      await router.replace({ name: 'cloud-login', query: { redirect: route.fullPath } })
      return
    }
    try {
      window.location.href = await cloudOAuthAuthorize(route.fullPath)
    } catch (err) {
      notifications.notifyError(t('cloud.loginFailed'), err)
      await router.replace({ name: 'home' })
    }
  })
</script>
