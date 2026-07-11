<template>
  <section class="min-h-screen bg-[var(--color-app-bg)] text-sm text-[var(--color-text)]">
    <AppHeader />
    <div class="flex min-h-[calc(100vh-4rem)] items-center justify-center p-6">
    <form
      class="w-full max-w-sm rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-xl"
      @submit.prevent="emit('submit')"
    >
      <h1 class="text-lg font-semibold text-[var(--color-text-strong)]">
        {{ t('cloud.loginTitle') }}
      </h1>
      <p class="mt-1 text-[var(--color-text-muted)]">{{ t('cloud.loginDescription') }}</p>
      <label class="mt-4 block">
        <span class="text-[var(--color-text)]">{{ t('cloud.username') }}</span>
        <input
          :value="username"
          class="mt-1 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-[var(--color-text)] outline-none"
          autocomplete="username"
          type="email"
          @input="emit('update:username', ($event.target as HTMLInputElement).value)"
        />
      </label>
      <label class="mt-3 block">
        <span class="text-[var(--color-text)]">{{ t('cloud.password') }}</span>
        <input
          :value="password"
          type="password"
          class="mt-1 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 text-[var(--color-text)] outline-none"
          autocomplete="current-password"
          @input="emit('update:password', ($event.target as HTMLInputElement).value)"
        />
      </label>
      <slot />
      <button
        type="submit"
        class="mt-4 h-9 w-full rounded-md border border-blue-700 bg-blue-600 text-slate-50 hover:bg-blue-500 disabled:opacity-60"
        :disabled="loggingIn || googleLoggingIn || !turnstileReady"
      >
        {{ loggingIn ? t('cloud.signingIn') : t('cloud.signIn') }}
      </button>
      <button
        type="button"
        class="mt-2 h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] text-[var(--color-text)] hover:bg-[var(--color-control-hover)] disabled:opacity-60"
        :disabled="loggingIn || googleLoggingIn"
        @click="emit('google')"
      >
        {{ googleLoggingIn ? t('cloud.signingIn') : t('cloud.continueWithGoogle') }}
      </button>
      <div class="mt-3 flex justify-between text-xs text-[var(--color-text-muted)]">
        <RouterLink class="hover:text-[var(--color-text)]" :to="{ name: 'cloud-register' }">
          {{ t('cloud.register') }}
        </RouterLink>
        <RouterLink class="hover:text-[var(--color-text)]" :to="{ name: 'cloud-forgot-password' }">
          {{ t('cloud.forgotPassword') }}
        </RouterLink>
      </div>
      </form>
    </div>
  </section>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import { RouterLink } from 'vue-router'
  import AppHeader from '../layout/AppHeader.vue'

  defineProps<{
    username: string
    password: string
    loggingIn: boolean
    googleLoggingIn: boolean
    turnstileReady: boolean
  }>()

  const emit = defineEmits<{
    'update:username': [value: string]
    'update:password': [value: string]
    submit: []
    google: []
  }>()

  const { t } = useI18n()
</script>
