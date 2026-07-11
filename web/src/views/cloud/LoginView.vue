<template>
  <ToastProvider>
    <section
      v-if="!cloudAuth.authInitialized"
      class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
    >
      {{ t('cloudAuth.checkingAuth') }}
    </section>

    <LoginPanel
      v-else
      :username="cloudAuth.usernameInput"
      :password="cloudAuth.passwordInput"
      :logging-in="cloudAuth.loggingIn"
      :google-logging-in="googleLoggingIn"
      :turnstile-ready="!!turnstileToken"
      @update:username="cloudAuth.usernameInput = $event"
      @update:password="cloudAuth.passwordInput = $event"
      @submit="login"
      @google="loginWithGoogle"
    >
      <TurnstileChallenge
        v-if="turnstileSiteKey"
        ref="turnstile"
        :site-key="turnstileSiteKey"
        @token="turnstileToken = $event"
        @reset="turnstileToken = ''"
      />
    </LoginPanel>

    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { ToastProvider } from 'reka-ui'
  import LoginPanel from '../../components/session/LoginPanel.vue'
  import ToastHost from '../../components/session/ToastHost.vue'
  import TurnstileChallenge from '../../components/cloud/TurnstileChallenge.vue'
  import { authGoogleUrl, authLoginCSRFToken, authTurnstileSiteKey } from '../../features/cloud/api'
  import {
    authenticatedCloudLoginRedirect,
    storeCloudLoginRedirect,
  } from '../../features/cloud/loginRedirect'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const cloudAuth = useCloudAuthStore()
  const notifications = useNotificationsStore()
  const googleLoggingIn = ref(false)
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
      notifications.notifyError(t('cloudAuth.loginFailed'), err)
    }
  })

  async function login() {
    if (!turnstileToken.value) {
      notifications.notifyError(
        t('cloudAuth.loginFailed'),
        new Error(t('cloud.humanVerificationRequired')),
      )
      return
    }
    try {
      const csrfToken = await authLoginCSRFToken()
      await cloudAuth.login(turnstileToken.value, csrfToken)
      await cloudAuth.initializeAuth({ force: true })
      if (cloudAuth.authenticated) {
        await router.replace(redirectAfterLogin())
      }
    } catch (err) {
      notifications.notifyError(t('cloudAuth.loginFailed'), err)
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

  async function loginWithGoogle() {
    if (googleLoggingIn.value) {
      return
    }
    googleLoggingIn.value = true
    try {
      storeCloudLoginRedirect(typeof route.query.redirect === 'string' ? route.query.redirect : '')
      window.location.href = await authGoogleUrl()
    } catch (err) {
      googleLoggingIn.value = false
      notifications.notifyError(t('cloudAuth.googleLoginFailed'), err)
    }
  }
</script>
