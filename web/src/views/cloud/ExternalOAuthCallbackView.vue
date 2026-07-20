<template>
  <AuthStatusScreen
    :status="status"
    :title="title"
    :description="description"
    :footer-to="status === 'error' ? { name: 'cloud-login' } : undefined"
    :footer-label="status === 'error' ? t('cloud.backToLogin') : ''"
  >
    <template #icon>
      <GoogleIcon v-if="provider === 'google'" class="size-10" />
      <GithubIcon v-else-if="provider === 'github'" class="size-10 text-[var(--color-text)]" />
    </template>
  </AuthStatusScreen>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import GithubIcon from '../../components/branding/GithubIcon.vue'
  import GoogleIcon from '../../components/branding/GoogleIcon.vue'
  import AuthStatusScreen from '../../components/cloud/AuthStatusScreen.vue'
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
  const status = ref<'loading' | 'error'>('loading')

  const title = computed(() =>
    status.value === 'error'
      ? t('cloud.oauthCallbackFailed')
      : props.provider === 'google'
        ? t('cloud.googleSigningIn')
        : t('cloud.externalSigningIn'),
  )
  const description = computed(() =>
    status.value === 'error'
      ? t('cloud.oauthCallbackFailedHint')
      : t('cloud.externalSigningInHint'),
  )

  function redirectAfterLogin() {
    return consumeCloudLoginRedirect() || { name: 'cloud-dashboard' }
  }

  function failAndStay(err: unknown) {
    status.value = 'error'
    notifications.notifyError(t('cloud.externalLoginFailed'), err)
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
      failAndStay(new Error('invalid provider callback'))
      return
    }
    try {
      const response = await authExternalCallback(providerId, code, state)
      cloudAuth.setToken(response.access_token)
      await cloudAuth.initialize()
      if (cloudAuth.authenticated) {
        await router.replace(redirectAfterLogin())
        return
      }
      failAndStay(new Error('not authenticated after callback'))
    } catch (err) {
      failAndStay(err)
    }
  })
</script>
