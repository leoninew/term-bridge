<template>
  <section
    class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text)]"
  >
    <form
      class="w-full max-w-sm rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-xl"
      @submit.prevent="submit"
    >
      <h1 class="text-lg font-semibold text-[var(--color-text-strong)]">
        {{ t('gateway.registerTitle') }}
      </h1>
      <label class="mt-4 block"
        ><span>{{ t('gateway.email') }}</span
        ><input
          v-model="email"
          type="email"
          class="mt-1 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 outline-none"
      /></label>
      <label class="mt-3 block"
        ><span>{{ t('gateway.password') }}</span
        ><input
          v-model="password"
          type="password"
          class="mt-1 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 outline-none"
      /></label>
      <button
        class="mt-4 h-9 w-full rounded-md border border-blue-700 bg-blue-600 text-slate-50 disabled:opacity-60"
        :disabled="submitting || googleSubmitting"
      >
        {{ submitting ? t('common.creating') : t('gateway.register') }}
      </button>
      <button
        type="button"
        class="mt-2 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] text-[var(--color-text)] hover:bg-[var(--color-control-hover)] disabled:opacity-60"
        :disabled="submitting || googleSubmitting"
        @click="registerWithGoogle"
      >
        {{ googleSubmitting ? t('gateway.signingIn') : t('gateway.continueWithGoogle') }}
      </button>
      <RouterLink
        class="mt-3 block text-xs text-[var(--color-text-muted)]"
        :to="{ name: 'login' }"
        >{{ t('gateway.backToLogin') }}</RouterLink
      >
    </form>
  </section>
</template>
<script setup lang="ts">
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter, RouterLink } from 'vue-router'
  import { authGoogleURL, authRegister } from '../features/gateway/api'
  const { t } = useI18n()
  const router = useRouter()
  const email = ref('')
  const password = ref('')
  const submitting = ref(false)
  const googleSubmitting = ref(false)
  async function submit() {
    submitting.value = true
    try {
      await authRegister(email.value, password.value)
      await router.push({ name: 'verify-email', query: { email: email.value } })
    } finally {
      submitting.value = false
    }
  }

  async function registerWithGoogle() {
    if (googleSubmitting.value) {
      return
    }
    googleSubmitting.value = true
    try {
      window.location.href = await authGoogleURL()
    } finally {
      googleSubmitting.value = false
    }
  }
</script>
