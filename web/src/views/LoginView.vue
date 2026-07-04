<template>
  <ToastProvider>
    <section
      v-if="!gateway.authInitialized"
      class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text-muted)]"
    >
      {{ t('gateway.checkingAuth') }}
    </section>

    <LoginPanel
      v-else
      :username="gateway.usernameInput"
      :password="gateway.passwordInput"
      :logging-in="gateway.loggingIn"
      :google-logging-in="googleLoggingIn"
      @update:username="gateway.usernameInput = $event"
      @update:password="gateway.passwordInput = $event"
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
  import LoginPanel from '../components/session/LoginPanel.vue'
  import ToastHost from '../components/session/ToastHost.vue'
  import { authGoogleURL } from '../features/cloud/api'
  import { writeStorageValue } from '../store/storage'
  import { useGatewayStore } from '../store/gateway'
  import { useNotificationsStore } from '../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const gateway = useGatewayStore()
  const notifications = useNotificationsStore()
  const googleLoggingIn = ref(false)
  const OAUTH_REDIRECT_KEY = 'termbridge.oauth_redirect'

  onMounted(async () => {
    await gateway.initializeAuth()
    if (gateway.capabilities?.mode === 'local') {
      await router.replace({ name: 'agent-dashboard' })
    }
  })

  async function login() {
    try {
      await gateway.login()
      await router.replace(redirectAfterLogin())
    } catch (err) {
      notifications.notifyError(t('gateway.loginFailed'), err)
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
        writeStorageValue(OAUTH_REDIRECT_KEY, redirect)
      }
      window.location.href = await authGoogleURL()
    } catch (err) {
      googleLoggingIn.value = false
      notifications.notifyError(t('gateway.googleLoginFailed'), err)
    }
  }
</script>
