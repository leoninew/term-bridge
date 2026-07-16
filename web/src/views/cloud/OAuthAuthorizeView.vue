<template>
  <section
    v-if="checkingAuth"
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
</template>

<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { cloudOAuthAuthorize } from '../../features/cloud/api'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const cloudAuth = useCloudAuthStore()
  const notifications = useNotificationsStore()
  const checkingAuth = ref(true)

  onMounted(async () => {
    try {
      await cloudAuth.initialize()
      if (!cloudAuth.authenticated) {
        await router.replace({ name: 'cloud-login', query: { redirect: route.fullPath } })
        return
      }
      window.location.href = await cloudOAuthAuthorize(route.fullPath)
    } catch (err) {
      notifications.notifyError(t('cloud.loginFailed'), err)
      await router.replace({ name: 'home' })
    } finally {
      checkingAuth.value = false
    }
  })
</script>
