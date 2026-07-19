<template>
  <AuthStatusScreen
    :status="status"
    :title="title"
    :description="description"
    :footer-to="status === 'error' ? { name: 'home' } : undefined"
    :footer-label="status === 'error' ? t('dashboard.home') : ''"
  />
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import AuthStatusScreen from '../../components/cloud/AuthStatusScreen.vue'
  import { cloudOAuthAuthorize } from '../../features/cloud/api'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const cloudAuth = useCloudAuthStore()
  const notifications = useNotificationsStore()
  const status = ref<'loading' | 'error'>('loading')

  const title = computed(() =>
    status.value === 'error' ? t('cloud.loginFailed') : t('cloud.oauthAuthorizeWorking'),
  )
  const description = computed(() =>
    status.value === 'error' ? t('cloud.oauthCallbackFailedHint') : t('cloud.oauthAuthorizeHint'),
  )

  onMounted(async () => {
    try {
      await cloudAuth.initialize()
      if (!cloudAuth.authenticated) {
        await router.replace({ name: 'cloud-login', query: { redirect: route.fullPath } })
        return
      }
      window.location.href = await cloudOAuthAuthorize(route.fullPath)
    } catch (err) {
      status.value = 'error'
      notifications.notifyError(t('cloud.loginFailed'), err)
    }
  })
</script>
