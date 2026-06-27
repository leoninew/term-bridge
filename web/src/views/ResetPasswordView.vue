<template>
  <section
    class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text)]"
  >
    <form
      class="w-full max-w-sm rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-xl"
      @submit.prevent="submit"
    >
      <h1 class="text-lg font-semibold text-[var(--color-text-strong)]">
        {{ t('gateway.resetPassword') }}
      </h1>
      <input
        v-model="email"
        type="email"
        class="mt-4 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 outline-none"
        :placeholder="t('gateway.email')"
      />
      <input
        v-model="code"
        class="mt-3 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 uppercase outline-none"
        maxlength="6"
        placeholder="ABC123"
      />
      <input
        v-model="password"
        type="password"
        class="mt-3 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 outline-none"
        :placeholder="t('gateway.newPassword')"
      />
      <button class="mt-4 h-9 w-full rounded-md border border-blue-700 bg-blue-600 text-slate-50">
        {{ t('gateway.resetPassword') }}
      </button>
    </form>
  </section>
</template>
<script setup lang="ts">
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { authPasswordResetConfirm } from '../features/gateway/api'
  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const email = ref(String(route.query.email ?? ''))
  const code = ref('')
  const password = ref('')
  async function submit() {
    await authPasswordResetConfirm(email.value, code.value, password.value)
    await router.push({ name: 'login' })
  }
</script>
