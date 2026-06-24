<template>
  <ToastProvider>
    <section
      v-if="!gateway.authInitialized"
      class="flex h-screen min-h-screen items-center justify-center bg-[#05070d] p-6 text-sm text-slate-500"
    >
      {{ t('gateway.checkingAuth') }}
    </section>

    <LoginPanel
      v-else
      :username="gateway.usernameInput"
      :password="gateway.passwordInput"
      :logging-in="gateway.loggingIn"
      @update:username="gateway.usernameInput = $event"
      @update:password="gateway.passwordInput = $event"
      @submit="login"
    />

    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { onMounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { ToastProvider } from 'reka-ui'
  import LoginPanel from '../components/session/LoginPanel.vue'
  import ToastHost from '../components/session/ToastHost.vue'
  import { useGatewayStore } from '../store/gateway'
  import { useNotificationsStore } from '../store/notifications'

  const { t } = useI18n()
  const router = useRouter()
  const gateway = useGatewayStore()
  const notifications = useNotificationsStore()

  async function login() {
    try {
      await gateway.login()
      await router.replace({ name: 'sessions' })
    } catch (err) {
      notifications.notifyError(t('gateway.loginFailed'), err)
    }
  }

  onMounted(async () => {
    try {
      await gateway.initializeAuth()
      if (gateway.authenticated) {
        await router.replace({ name: 'sessions' })
      }
    } catch (err) {
      notifications.notifyError(t('toast.refreshFailed'), err)
    }
  })
</script>
