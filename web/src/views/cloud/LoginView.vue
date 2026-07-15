<template>
  <section
    v-if="!cloudAuth.authInitialized"
    class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
  >
    {{ t('cloud.checkingAuth') }}
  </section>

  <LoginPanel
    v-else
    :username="cloudAuth.usernameInput"
    :password="cloudAuth.passwordInput"
    :logging-in="cloudAuth.loggingIn || loginSubmitting"
    :external-logging-in="externalLoggingIn"
    :external-auth-provider-ids="runtimeConfig.config.cloud.externalAuthProviderIds"
    :turnstile-ready="!!turnstileToken"
    @update:username="cloudAuth.usernameInput = $event"
    @update:password="cloudAuth.passwordInput = $event"
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
  import LoginPanel from '../../components/session/LoginPanel.vue'
  import TurnstileChallenge from '../../components/cloud/TurnstileChallenge.vue'
  import {
    authExternalUrl,
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
  const externalLoggingIn = ref(false)
  const loginSubmitting = ref(false)
  const turnstileSiteKey = ref('')
  const turnstileToken = ref('')
  const turnstile = ref<InstanceType<typeof TurnstileChallenge> | null>(null)

  onMounted(async () => {
    try {
      ;[turnstileSiteKey.value] = await Promise.all([
        authTurnstileSiteKey(),
        cloudAuth.initializeAuth(),
      ])
      if (cloudAuth.authenticated) {
        await router.replace(redirectAfterLogin())
      }
    } catch (err) {
      notifications.notifyError(t('cloud.loginFailed'), err)
    }
  })

  async function login() {
    if (loginSubmitting.value || cloudAuth.loggingIn) {
      return
    }
    if (!cloudAuth.usernameInput) {
      notifications.pushToast('error', t('cloud.loginFailed'), t('message.emailRequired'))
      return
    }
    if (!cloudAuth.passwordInput) {
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
    loginSubmitting.value = true
    try {
      const csrfToken = await authLoginCSRFToken()
      await cloudAuth.login(turnstileToken.value, csrfToken)
      await cloudAuth.initializeAuth({ force: true })
      if (cloudAuth.authenticated) {
        await router.replace(redirectAfterLogin())
      }
    } catch (err) {
      notifications.notifyError(t('cloud.loginFailed'), err)
    } finally {
      loginSubmitting.value = false
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
    if (externalLoggingIn.value) {
      return
    }
    externalLoggingIn.value = true
    try {
      storeCloudLoginRedirect(typeof route.query.redirect === 'string' ? route.query.redirect : '')
      window.location.href = await authExternalUrl(providerId)
    } catch (err) {
      externalLoggingIn.value = false
      notifications.notifyError(t('cloud.externalLoginFailed'), err)
    }
  }
</script>
