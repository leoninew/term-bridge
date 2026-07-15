<template>
  <AppPageShell
    main-class="flex min-h-0 items-center justify-center px-4 py-10 text-sm sm:px-6 sm:py-12"
  >
    <div class="w-full max-w-[26rem] text-center">
      <form
        class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-6 text-left shadow-[0_12px_40px_rgb(0_0_0_/_0.18)] sm:p-8"
        @submit.prevent="emit('submit')"
      >
        <h1
          class="mb-6 text-center text-2xl font-semibold leading-8 tracking-tight text-[var(--color-text-strong)] sm:text-[1.75rem]"
        >
          {{ t('cloud.loginTitle') }}
        </h1>
        <div class="space-y-5">
          <label class="flex flex-col gap-2">
            <span class="block text-sm font-medium text-[var(--color-text)]">{{
              t('cloud.username')
            }}</span>
            <input
              :value="username"
              class="h-10 w-full rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-control-bg)] px-3.5 text-sm text-[var(--color-text)] outline-none transition-colors focus:border-[var(--color-primary-border)] focus:ring-2 focus:ring-[var(--color-primary-border)]/20"
              autocomplete="username"
              type="email"
              required
              @input="emit('update:username', ($event.target as HTMLInputElement).value)"
            />
          </label>
          <label class="flex flex-col gap-2">
            <span
              class="flex items-center justify-between gap-3 text-sm font-medium text-[var(--color-text)]"
            >
              {{ t('cloud.password') }}
              <RouterLink
                class="font-normal text-[var(--color-primary-border)] outline-none transition-opacity hover:opacity-80 focus-visible:rounded focus-visible:ring-2 focus-visible:ring-[var(--color-primary-border)]/40"
                :to="{ name: 'cloud-forgot-password' }"
              >
                {{ t('cloud.forgotPassword') }}
              </RouterLink>
            </span>
            <input
              :value="password"
              type="password"
              class="h-10 w-full rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-control-bg)] px-3.5 text-sm text-[var(--color-text)] outline-none transition-colors focus:border-[var(--color-primary-border)] focus:ring-2 focus:ring-[var(--color-primary-border)]/20"
              autocomplete="current-password"
              required
              @input="emit('update:password', ($event.target as HTMLInputElement).value)"
            />
          </label>
          <slot />
          <button
            type="submit"
            class="button button-primary h-10 w-full text-sm"
            :disabled="loggingIn || externalLoggingIn || !turnstileReady"
          >
            {{ loggingIn ? t('cloud.signingIn') : t('cloud.signIn') }}
          </button>
        </div>

        <template v-if="externalAuthProviderIds.length">
          <div class="my-6 flex items-center gap-3 text-xs text-[var(--color-text-muted)]">
            <div class="h-px flex-1 bg-[var(--color-border)]" />
            <span class="font-medium uppercase tracking-wide">{{ t('cloud.or') }}</span>
            <div class="h-px flex-1 bg-[var(--color-border)]" />
          </div>
          <div class="space-y-3">
            <button
              v-for="providerId in externalAuthProviderIds"
              :key="providerId"
              type="button"
              class="button button-provider h-10 w-full gap-2 text-sm"
              :disabled="loggingIn || externalLoggingIn"
              @click="emit('external', providerId)"
            >
              <img
                v-if="!externalLoggingIn"
                :src="providerIcon(providerId)"
                class="size-4 shrink-0"
                aria-hidden="true"
              />
              <span>{{
                externalLoggingIn ? t('cloud.signingIn') : providerLabel(providerId)
              }}</span>
            </button>
          </div>
        </template>
      </form>

      <p class="mt-6 text-sm leading-6 text-[var(--color-text-muted)]">
        {{ t('cloud.newToTermBridge') }}
        <RouterLink
          class="text-[var(--color-primary-border)] outline-none transition-opacity hover:opacity-80 focus-visible:rounded focus-visible:ring-2 focus-visible:ring-[var(--color-primary-border)]/40"
          :to="{ name: 'cloud-register' }"
        >
          {{ t('cloud.createAccount') }}
        </RouterLink>
      </p>
    </div>
  </AppPageShell>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import { RouterLink } from 'vue-router'
  import githubProviderIcon from '../../assets/github-provider.svg'
  import googleProviderIcon from '../../assets/google-provider.svg'
  import AppPageShell from '../layout/AppPageShell.vue'

  defineProps<{
    username: string
    password: string
    loggingIn: boolean
    externalLoggingIn: boolean
    externalAuthProviderIds: string[]
    turnstileReady: boolean
  }>()

  const emit = defineEmits<{
    'update:username': [value: string]
    'update:password': [value: string]
    submit: []
    external: [providerId: string]
  }>()

  const { t } = useI18n()

  function providerIcon(providerId: string) {
    return providerId === 'github' ? githubProviderIcon : googleProviderIcon
  }

  function providerLabel(providerId: string) {
    return providerId === 'github' ? t('cloud.continueWithGitHub') : t('cloud.continueWithGoogle')
  }
</script>
