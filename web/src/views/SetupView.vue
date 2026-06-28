<template>
  <ToastProvider>
    <section
      class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text)]"
    >
      <div
        v-if="state === 'missing-token'"
        class="w-full max-w-md rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-xl"
      >
        <h1 class="text-lg font-semibold text-[var(--color-text-strong)]">
          {{ t('gateway.setupMissingTokenTitle') }}
        </h1>
        <p class="mt-2 text-[var(--color-text-muted)]">
          {{ t('gateway.setupMissingTokenDescription') }}
        </p>
        <p
          class="mt-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-control-bg)] p-3 text-xs text-[var(--color-text-muted)]"
        >
          {{ t('gateway.setupMissingTokenHint') }}
        </p>
        <RouterLink
          class="mt-4 inline-flex h-9 items-center rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-3 text-[var(--color-text)] hover:bg-[var(--color-control-hover)]"
          :to="loginRoute"
        >
          {{ t('gateway.setupBackToLogin') }}
        </RouterLink>
      </div>
      <div v-else class="text-[var(--color-text-muted)]">
        {{ statusText }}
      </div>
    </section>
    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { RouterLink, useRoute, useRouter } from 'vue-router'
  import { ToastProvider } from 'reka-ui'
  import ToastHost from '../components/session/ToastHost.vue'
  import { authSetupComplete } from '../features/gateway/api'
  import { useGatewayStore } from '../store/gateway'
  import { useNotificationsStore } from '../store/notifications'

  type SetupState = 'signing-in' | 'missing-token' | 'failed'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const gateway = useGatewayStore()
  const notifications = useNotificationsStore()
  const state = ref<SetupState>('signing-in')

  const loginRoute = computed(() => {
    const redirect = safeRedirect(
      typeof route.query.redirect === 'string' ? route.query.redirect : '',
    )
    return redirect ? { name: 'login', query: { redirect } } : { name: 'login' }
  })

  const statusText = computed(() =>
    state.value === 'failed' ? t('gateway.setupFailed') : t('gateway.setupSigningIn'),
  )

  onMounted(async () => {
    const setupToken = typeof route.query.token === 'string' ? route.query.token : ''
    window.history.replaceState({}, document.title, '/setup')
    if (!setupToken) {
      state.value = 'missing-token'
      return
    }
    try {
      const response = await authSetupComplete(setupToken)
      gateway.setToken(response.access_token)
      await router.replace(redirectAfterSetup())
    } catch (err) {
      state.value = 'failed'
      notifications.notifyError(t('gateway.setupFailed'), err)
    }
  })

  function redirectAfterSetup() {
    const redirect = safeRedirect(
      typeof route.query.redirect === 'string' ? route.query.redirect : '',
    )
    return redirect || { name: 'sessions' }
  }

  function safeRedirect(value: string | null) {
    return value && value.startsWith('/') && !value.startsWith('//') ? value : ''
  }
</script>
