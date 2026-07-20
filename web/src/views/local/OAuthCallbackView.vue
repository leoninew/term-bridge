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
  import { assertCloudOAuthState, consumeCloudOAuthRedirect } from '../../features/cloud/oauth'
  import { exchangeOAuthCode } from '../../features/local/api'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const cloudAuth = useCloudAuthStore()
  const notifications = useNotificationsStore()
  const status = ref<'loading' | 'error'>('loading')

  const title = computed(() =>
    status.value === 'error' ? t('cloud.localOAuthFailed') : t('cloud.localOAuthConnecting'),
  )
  const description = computed(() =>
    status.value === 'error'
      ? t('cloud.localOAuthFailedHint')
      : t('cloud.localOAuthConnectingHint'),
  )

  function failAndStay(err: unknown) {
    status.value = 'error'
    notifications.notifyError(t('dashboard.cloudConnectionFailed'), err)
  }

  onMounted(async () => {
    const code = typeof route.query.code === 'string' ? route.query.code : ''
    const state = typeof route.query.state === 'string' ? route.query.state : ''
    if (!code || !state) {
      failAndStay(new Error('missing code or state'))
      return
    }
    try {
      assertCloudOAuthState(state)
      const accessToken = await exchangeOAuthCode(code)
      cloudAuth.setToken(accessToken)
      await cloudAuth.initialize()
      await router.replace(consumeCloudOAuthRedirect())
    } catch (err) {
      failAndStay(err)
    }
  })
</script>
