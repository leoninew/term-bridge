<template>
  <section
    class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text)]"
  >
    <form
      class="w-full max-w-sm rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-xl"
      @submit.prevent="submit"
    >
      <h1 class="text-lg font-semibold text-[var(--color-text-strong)]">
        {{ t('cloud.forgotPassword') }}
      </h1>
      <input
        v-model="email"
        type="email"
        class="mt-4 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 outline-none"
        :placeholder="t('cloud.email')"
      />
      <button class="mt-4 h-9 w-full rounded-md border border-blue-700 bg-blue-600 text-slate-50">
        {{ t('cloud.sendResetCode') }}
      </button>
    </form>
  </section>
</template>
<script setup lang="ts">
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { authPasswordResetRequest } from '../../features/cloud/api'
  const { t } = useI18n()
  const router = useRouter()
  const email = ref('')
  async function submit() {
    await authPasswordResetRequest(email.value)
    await router.push({ name: 'cloud-reset-password', query: { email: email.value } })
  }
</script>
