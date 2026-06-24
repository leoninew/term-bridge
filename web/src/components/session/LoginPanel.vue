<template>
  <section
    class="flex h-screen min-h-screen items-center justify-center bg-[#05070d] p-6 text-sm text-slate-200"
  >
    <form
      class="w-full max-w-sm rounded-xl border border-slate-800 bg-[#0a0f18] p-5 shadow-xl"
      @submit.prevent="emit('submit')"
    >
      <h1 class="text-lg font-semibold text-slate-100">{{ t('gateway.loginTitle') }}</h1>
      <p class="mt-1 text-slate-500">{{ t('gateway.loginDescription') }}</p>
      <label class="mt-4 block">
        <span class="text-slate-400">{{ t('gateway.username') }}</span>
        <input
          :value="username"
          class="mt-1 h-9 w-full rounded-md border border-slate-800 bg-slate-950 px-2 text-slate-100 outline-none"
          autocomplete="username"
          @input="emit('update:username', ($event.target as HTMLInputElement).value)"
        />
      </label>
      <label class="mt-3 block">
        <span class="text-slate-400">{{ t('gateway.password') }}</span>
        <input
          :value="password"
          type="password"
          class="mt-1 h-9 w-full rounded-md border border-slate-800 bg-slate-950 px-2 text-slate-100 outline-none"
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
