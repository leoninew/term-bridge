<template>
  <AuthStatusScreen
    v-if="checkingAuth"
    status="loading"
    :title="t('cloud.checkingAuth')"
    :description="t('cloud.externalSigningInHint')"
  />

  <LoginPanel
    v-else
    :username="username"
    :password="password"
    :logging-in="loginAction.running"
    :external-logging-in="externalAction.running"
    :external-auth-provider-ids="runtimeConfig.config.cloud.externalAuthProviderIds"
    :turnstile-ready="!!turnstileToken"
    @update:username="username = $event"
    @update:password="password = $event"
    @submit="login"
    @external="loginWithExternalProvider"
  >
    <TurnstileChallenge
      v-if="turnstileSiteKey"
      ref="turnstile"
      :site-key="turnstileSiteKey"
      @token="turnstileToken = $event"
      @reset="turnstileToken = ''"
    />
  </LoginPanel>
</template>

<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import AuthStatusScreen from '../../components/cloud/AuthStatusScreen.vue'
  import LoginPanel from '../../components/cloud/LoginPanel.vue'
  import TurnstileChallenge from '../../components/cloud/TurnstileChallenge.vue'
  import { useAsyncAction } from '../../composable/useAsyncAction'
  import {
    authExternalUrl,
    authLogin,
    authLoginCSRFToken,
    authTurnstileSiteKey,
  } from '../../features/cloud/api'
  import {
    authenticatedCloudLoginRedirect,
    storeCloudLoginRedirect,
  } from '../../features/cloud/loginRedirect'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useNotificationsStore } from '../../store/notifications'
  import { useRuntimeConfigStore } from '../../store/runtimeConfig'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const cloudAuth = useCloudAuthStore()
  const notifications = useNotificationsStore()
  const runtimeConfig = useRuntimeConfigStore()
  const checkingAuth = ref(true)
  const username = ref('')
  const password = ref('')
  const turnstileSiteKey = ref('')
  const turnstileToken = ref('')
  const turnstile = ref<InstanceType<typeof TurnstileChallenge> | null>(null)
  const loginAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('cloud.loginFailed'), err),
  })
  const externalAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('cloud.externalLoginFailed'), err),
  })

  onMounted(async () => {
    try {
      ;[turnstileSiteKey.value] = await Promise.all([
        authTurnstileSiteKey(),
        cloudAuth.initialize(),
      ])
      if (cloudAuth.authenticated) {
        await router.replace(redirectAfterLogin())
      }
    } catch (err) {
      notifications.notifyError(t('cloud.loginFailed'), err)
    } finally {
      checkingAuth.value = false
    }
  })

  async function login() {
    if (loginAction.running || externalAction.running) {
      return
    }
    if (!username.value) {
      notifications.pushToast('error', t('cloud.loginFailed'), t('message.emailRequired'))
      return
    }
    if (!password.value) {
      notifications.pushToast('error', t('cloud.loginFailed'), t('message.passwordRequired'))
      return
    }
    if (!turnstileToken.value) {
      notifications.notifyError(
        t('cloud.loginFailed'),
        new Error(t('cloud.humanVerificationRequired')),
      )
      return
    }
    try {
      await loginAction.run(
        async () => {
          const csrfToken = await authLoginCSRFToken()
          const response = await authLogin(
            username.value,
            password.value,
            turnstileToken.value,
            csrfToken,
          )
          password.value = ''
          cloudAuth.setToken(response.access_token)
          await cloudAuth.initialize()
          if (cloudAuth.authenticated) {
            await router.replace(redirectAfterLogin())
          }
        },
        { rethrow: true },
      )
    } catch {
      // notify handled by loginAction.onError
    } finally {
      turnstileToken.value = ''
      turnstile.value?.reset()
    }
  }

  function redirectAfterLogin() {
    const redirect = authenticatedCloudLoginRedirect(
      cloudAuth.authenticated,
      typeof route.query.redirect === 'string' ? route.query.redirect : '',
    )
    return redirect || { name: 'cloud-dashboard' }
  }

  async function loginWithExternalProvider(providerId: string) {
    if (loginAction.running || externalAction.running) {
      return
    }
    await externalAction.run(async () => {
      storeCloudLoginRedirect(typeof route.query.redirect === 'string' ? route.query.redirect : '')
      window.location.href = await authExternalUrl(providerId)
    })
  }
</script>
