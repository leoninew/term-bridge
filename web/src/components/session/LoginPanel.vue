<template>
  <section
    class="flex h-screen min-h-screen items-center justify-center bg-[var(--color-app-bg)] p-6 text-sm text-[var(--color-text)]"
  >
    <form
      class="w-full max-w-sm rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-xl"
      @submit.prevent="emit('submit')"
    >
      <h1 class="text-lg font-semibold text-[var(--color-text-strong)]">{{ t('gateway.loginTitle') }}</h1>
      <p class="mt-1 text-[var(--color-text-muted)]">{{ t('gateway.loginDescription') }}</p>
      <label class="mt-4 block">
        <span class="text-[var(--color-text)]">{{ t('gateway.username') }}</span>
        <input
          :value="username"
          class="mt-1 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-[var(--color-text)] outline-none"
          autocomplete="username"
          @input="emit('update:username', ($event.target as HTMLInputElement).value)"
        />
      </label>
      <label class="mt-3 block">
        <span class="text-[var(--color-text)]">{{ t('gateway.password') }}</span>
        <input
          :value="password"
          type="password"
          class="mt-1 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-[var(--color-text)] outline-none"
          autocomplete="current-password"
          @input="emit('update:password', ($event.target as HTMLInputElement).value)"
        />
      </label>
      <button
        type="submit"
        class="mt-4 h-9 w-full rounded-md border border-blue-700 bg-blue-600 text-slate-50 hover:bg-blue-500 disabled:opacity-60"
        :disabled="loggingIn"
      >
        {{ loggingIn ? t('gateway.signingIn') : t('gateway.signIn') }}
      </button>
    </form>
  </section>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'

  defineProps<{
    username: string
    password: string
    loggingIn: boolean
  }>()

  const emit = defineEmits<{
    'update:username': [value: string]
    'update:password': [value: string]
    submit: []
  }>()

  const { t } = useI18n()
</script>
