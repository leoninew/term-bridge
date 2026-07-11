<template>
  <section class="min-h-screen bg-[var(--color-app-bg)] text-sm text-[var(--color-text)]">
    <AppHeader />
    <div class="flex min-h-[calc(100vh-4rem)] items-center justify-center p-6">
      <form
        class="w-full max-w-sm rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-xl"
        @submit.prevent="submit"
      >
        <h1 class="text-lg font-semibold text-[var(--color-text-strong)]">
          {{ t('cloud.registerTitle') }}
        </h1>
        <label class="mt-4 block"
          ><span>{{ t('cloud.email') }}</span
          ><input
            v-model="email"
            type="email"
            class="mt-1 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 outline-none"
        /></label>
        <label class="mt-3 block"
          ><span>{{ t('cloud.password') }}</span
          ><input
            v-model="password"
            type="password"
            class="mt-1 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 outline-none"
        /></label>
        <TurnstileChallenge
          v-if="turnstileSiteKey"
          ref="turnstile"
          :site-key="turnstileSiteKey"
          @token="turnstileToken = $event"
          @reset="turnstileToken = ''"
        />
        <div v-else class="mt-3">
          <p class="text-xs text-[var(--color-danger-text)]" role="alert">
            {{ t('cloud.humanVerificationUnavailable') }}
          </p>
          <button
            type="button"
            class="mt-2 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
            :disabled="loadingTurnstile"
            @click="loadTurnstileSiteKey"
          >
            {{ t('common.retry') }}
          </button>
        </div>
        <button
          class="mt-4 h-9 w-full rounded-md border border-blue-700 bg-blue-600 text-slate-50 disabled:opacity-60"
          :disabled="submitting || googleSubmitting || !turnstileToken"
        >
          {{ submitting ? t('common.creating') : t('cloud.register') }}
        </button>
        <button
          type="button"
          class="mt-2 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] text-[var(--color-text)] hover:bg-[var(--color-control-hover)] disabled:opacity-60"
          :disabled="submitting || googleSubmitting"
          @click="registerWithGoogle"
        >
          {{ googleSubmitting ? t('cloud.signingIn') : t('cloud.continueWithGoogle') }}
        </button>
        <RouterLink
          class="mt-3 block text-xs text-[var(--color-text-muted)]"
          :to="{ name: 'cloud-login' }"
          >{{ t('cloud.backToLogin') }}</RouterLink
        >
      </form>
    </div>
  </section>
</template>
<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter, RouterLink } from 'vue-router'
  import { useNotificationsStore } from '../../store/notifications'
  import AppHeader from '../../components/layout/AppHeader.vue'
  import TurnstileChallenge from '../../components/cloud/TurnstileChallenge.vue'
  import { authGoogleUrl, authRegister, authTurnstileSiteKey } from '../../features/cloud/api'
  const { t } = useI18n()
  const router = useRouter()
  const notifications = useNotificationsStore()
  const email = ref('')
  const password = ref('')
  const submitting = ref(false)
  const googleSubmitting = ref(false)
  const turnstileSiteKey = ref('')
  const turnstileToken = ref('')
  const loadingTurnstile = ref(false)
  const turnstile = ref<InstanceType<typeof TurnstileChallenge> | null>(null)

  onMounted(loadTurnstileSiteKey)

  async function loadTurnstileSiteKey() {
    if (loadingTurnstile.value) {
      return
    }
    loadingTurnstile.value = true
    try {
      turnstileSiteKey.value = await authTurnstileSiteKey()
    } catch (err) {
      notifications.notifyError(t('cloud.registerTitle'), err)
    } finally {
      loadingTurnstile.value = false
    }
  }

  async function submit() {
    if (!turnstileToken.value) {
      return
    }
    submitting.value = true
    try {
      await authRegister(email.value, password.value, turnstileToken.value)
      await router.push({ name: 'cloud-verify-email', query: { email: email.value } })
    } finally {
      submitting.value = false
      turnstileToken.value = ''
      turnstile.value?.reset()
    }
  }

  async function registerWithGoogle() {
    if (googleSubmitting.value) {
      return
    }
    googleSubmitting.value = true
    try {
      window.location.href = await authGoogleUrl()
    } finally {
      googleSubmitting.value = false
    }
  }
</script>
