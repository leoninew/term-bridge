<template>
  <ToastProvider>
    <section
      v-if="gateway.capabilities?.mode === 'local'"
      class="flex min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6"
    >
      <div
        class="w-full max-w-lg rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-6 text-[var(--color-text)] shadow-xl"
      >
        <h1 class="text-xl font-semibold text-[var(--color-text-strong)]">
          {{ t('gateway.localLoginTitle') }}
        </h1>
        <p class="mt-2 text-sm text-[var(--color-text-muted)]">
          {{ t('gateway.localLoginDescription') }}
        </p>
        <div class="mt-6 flex flex-wrap gap-2">
          <RouterLink
            class="inline-flex h-9 items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-medium text-white hover:bg-blue-500"
            :to="{ name: 'dashboard' }"
          >
            {{ t('gateway.localLoginBackToDashboard') }}
          </RouterLink>
          <button
            v-if="gateway.capabilities?.cloud_oauth_enabled"
            type="button"
            class="inline-flex h-9 items-center justify-center rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-4 text-sm text-[var(--color-text)] hover:bg-[var(--color-control-hover)]"
            @click="connectCloud"
          >
            {{ t('gateway.localLoginConnectCloud') }}
          </button>
        </div>
      </div>
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
  import { RouterLink, useRoute, useRouter } from 'vue-router'
  import { ToastProvider } from 'reka-ui'
  import LoginPanel from '../components/session/LoginPanel.vue'
  import ToastHost from '../components/session/ToastHost.vue'
  import { authGoogleURL, cloudOAuthStartURL } from '../features/gateway/api'
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
    return safeRedirect(redirect) || { name: 'dashboard' }
  }

  function safeRedirect(value: string | null) {
    return value && value.startsWith('/') && !value.startsWith('//') ? value : ''
  }

  function connectCloud() {
    window.location.href = cloudOAuthStartURL()
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
