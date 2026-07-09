<template>
  <section
    class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text)]"
  >
    <form
      class="w-full max-w-sm rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-xl"
      @submit.prevent="submit"
    >
      <h1 class="text-lg font-semibold text-[var(--color-text-strong)]">
        {{ t('cloud.verifyEmailTitle') }}
      </h1>
      <input
        v-model="email"
        type="email"
        class="mt-4 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 outline-none"
        :placeholder="t('cloud.email')"
      />
      <input
        v-model="code"
        class="mt-3 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 uppercase outline-none"
        maxlength="6"
        placeholder="ABC123"
      />
      <button class="mt-4 h-9 w-full rounded-md border border-blue-700 bg-blue-600 text-slate-50">
        {{ t('cloud.verifyEmail') }}
      </button>
      <button
        type="button"
        class="mt-2 h-9 w-full rounded-md border border-[var(--color-border)]"
        @click="resend"
      >
        {{ t('cloud.resendCode') }}
      </button>
    </form>
  </section>
</template>
<script setup lang="ts">
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { authResendVerification, authVerifyEmail } from '../../features/cloud/api'
  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const email = ref(String(route.query.email ?? ''))
  const code = ref('')
  async function submit() {
    await authVerifyEmail(email.value, code.value)
    await router.push({ name: 'cloud-login' })
  }
  async function resend() {
    await authResendVerification(email.value)
  }
</script>
