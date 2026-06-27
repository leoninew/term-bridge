<template>
  <ToastProvider>
    <LoginPanel
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
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { ToastProvider } from 'reka-ui'
  import LoginPanel from '../components/session/LoginPanel.vue'
  import ToastHost from '../components/session/ToastHost.vue'
  import { authGoogleURL } from '../features/gateway/api'
  import { useGatewayStore } from '../store/gateway'
  import { useNotificationsStore } from '../store/notifications'

  const { t } = useI18n()
  const router = useRouter()
  const gateway = useGatewayStore()
  const notifications = useNotificationsStore()
  const googleLoggingIn = ref(false)

  async function login() {
    try {
      await gateway.login()
      await router.replace({ name: 'sessions' })
    } catch (err) {
      notifications.notifyError(t('gateway.loginFailed'), err)
    }
  }

  async function loginWithGoogle() {
    if (googleLoggingIn.value) {
      return
    }
    googleLoggingIn.value = true
    try {
      window.location.href = await authGoogleURL()
    } catch (err) {
      googleLoggingIn.value = false
      notifications.notifyError(t('gateway.googleLoginFailed'), err)
    }
  }
</script>
