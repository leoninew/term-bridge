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
      @update:username="cloudAuth.usernameInput = $event"
      @update:password="cloudAuth.passwordInput = $event"
      @submit="login"
      @google="loginWithGoogle"
    />

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
  import { authGoogleUrl } from '../../features/cloud/api'
  import { useAuthTokensStore } from '../../store/authTokens'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useNotificationsStore } from '../../store/notifications'
  import { removeLocalStorageValue, writeLocalStorageValue } from '../../store/storage'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const cloudAuth = useCloudAuthStore()
  const tokens = useAuthTokensStore()
  const notifications = useNotificationsStore()
  const googleLoggingIn = ref(false)
  const cloudLoginRedirectKey = 'termbridge.cloudAuth.login_redirect'

  onMounted(async () => {
    await cloudAuth.initializeAuth()
    if (tokens.cloudToken) {
      await router.replace(redirectAfterLogin())
    }
  })

  async function login() {
    try {
      await cloudAuth.login()
      await router.replace(redirectAfterLogin())
    } catch (err) {
      notifications.notifyError(t('cloudAuth.loginFailed'), err)
    }
  }

  function redirectAfterLogin() {
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : ''
    return safeRedirect(redirect) || { name: 'cloud-dashboard' }
  }

  function safeRedirect(value: string | null) {
    return value && value.startsWith('/') && !value.startsWith('//') ? value : ''
  }

  async function loginWithGoogle() {
    if (googleLoggingIn.value) {
      return
    }
    googleLoggingIn.value = true
    try {
      const redirect = safeRedirect(
        typeof route.query.redirect === 'string' ? route.query.redirect : '',
      )
      if (redirect) {
        writeLocalStorageValue(cloudLoginRedirectKey, redirect)
      } else {
        removeLocalStorageValue(cloudLoginRedirectKey)
      }
      window.location.href = await authGoogleUrl()
    } catch (err) {
      googleLoggingIn.value = false
      notifications.notifyError(t('cloudAuth.googleLoginFailed'), err)
    }
  }
</script>
