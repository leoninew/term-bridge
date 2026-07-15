<template>
  <AppPageShell
    main-class="flex min-h-0 items-center justify-center px-4 py-10 text-sm sm:px-6 sm:py-12"
  >
    <div class="w-full max-w-[26rem] text-center">
      <form
        class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-6 text-left shadow-[0_12px_40px_rgb(0_0_0_/_0.18)] sm:p-8"
        @submit.prevent="submit"
      >
        <h1
          class="mb-6 text-center text-2xl font-semibold leading-8 tracking-tight text-[var(--color-text-strong)] sm:text-[1.75rem]"
        >
          {{ t('cloud.registerTitle') }}
        </h1>
        <div class="space-y-5">
          <label class="flex flex-col gap-2">
            <span class="text-sm font-medium text-[var(--color-text)]">{{ t('cloud.email') }}</span>
            <input
              v-model="email"
              type="email"
              autocomplete="email"
              required
              :disabled="submitting"
              class="h-10 w-full rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-control-bg)] px-3.5 text-sm text-[var(--color-text)] outline-none transition-colors focus:border-[var(--color-primary-border)] focus:ring-2 focus:ring-[var(--color-primary-border)]/20"
            />
          </label>
          <label class="flex flex-col gap-2">
            <span class="text-sm font-medium text-[var(--color-text)]">{{
              t('cloud.password')
            }}</span>
            <input
              v-model="password"
              type="password"
              autocomplete="new-password"
              required
              :disabled="submitting"
              class="h-10 w-full rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-control-bg)] px-3.5 text-sm text-[var(--color-text)] outline-none transition-colors focus:border-[var(--color-primary-border)] focus:ring-2 focus:ring-[var(--color-primary-border)]/20"
            />
          </label>
          <TurnstileChallenge
            v-if="turnstileSiteKey"
            ref="turnstile"
            :site-key="turnstileSiteKey"
            @token="turnstileToken = $event"
            @reset="turnstileToken = ''"
          />
          <div v-else>
            <p class="text-xs text-[var(--color-danger-text)]" role="alert">
              {{ t('cloud.humanVerificationUnavailable') }}
            </p>
            <button
              type="button"
              class="mt-2 text-xs text-[var(--color-primary-border)] transition-opacity hover:opacity-80"
              :disabled="loadingTurnstile"
              @click="loadTurnstileSiteKey"
            >
              {{ t('common.retry') }}
            </button>
          </div>
          <button
            type="submit"
            class="button button-primary h-10 w-full text-sm"
            :disabled="submitting || externalSubmitting || !turnstileToken"
          >
            {{ submitting ? t('common.creating') : t('cloud.register') }}
          </button>
        </div>

        <template v-if="runtimeConfig.config.cloud.externalAuthProviderIds.length">
          <div class="my-6 flex items-center gap-3 text-xs text-[var(--color-text-muted)]">
            <div class="h-px flex-1 bg-[var(--color-border)]" />
            <span class="font-medium uppercase tracking-wide">{{ t('cloud.or') }}</span>
            <div class="h-px flex-1 bg-[var(--color-border)]" />
          </div>
          <div class="space-y-3">
            <button
              v-for="providerId in runtimeConfig.config.cloud.externalAuthProviderIds"
              :key="providerId"
              type="button"
              class="button button-provider h-10 w-full text-sm"
              :disabled="submitting || externalSubmitting"
              @click="registerWithExternalProvider(providerId)"
            >
              {{ externalSubmitting ? t('cloud.signingIn') : providerLabel(providerId) }}
            </button>
          </div>
        </template>
      </form>

      <p class="mt-6 text-sm leading-6 text-[var(--color-text-muted)]">
        {{ t('cloud.alreadyHaveAccount') }}
        <RouterLink
          class="text-[var(--color-primary-border)] outline-none transition-opacity hover:opacity-80 focus-visible:rounded focus-visible:ring-2 focus-visible:ring-[var(--color-primary-border)]/40"
          :to="{ name: 'cloud-login' }"
        >
          {{ t('cloud.signIn') }}
        </RouterLink>
      </p>
    </div>
  </AppPageShell>
</template>

<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter, RouterLink } from 'vue-router'
  import TurnstileChallenge from '../../components/cloud/TurnstileChallenge.vue'
  import AppPageShell from '../../components/layout/AppPageShell.vue'
  import { authExternalUrl, authRegister, authTurnstileSiteKey } from '../../features/cloud/api'
  import { useNotificationsStore } from '../../store/notifications'
  import { useRuntimeConfigStore } from '../../store/runtimeConfig'

  const { t } = useI18n()
  const router = useRouter()
  const notifications = useNotificationsStore()
  const runtimeConfig = useRuntimeConfigStore()
  const email = ref('')
  const password = ref('')
  const submitting = ref(false)
  const externalSubmitting = ref(false)
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
    if (submitting.value) {
      return
    }
    if (!email.value) {
      notifications.pushToast('error', t('cloud.registerFailed'), t('message.emailRequired'))
      return
    }
    if (!password.value) {
      notifications.pushToast('error', t('cloud.registerFailed'), t('message.passwordRequired'))
      return
    }
    if (!turnstileToken.value) {
      notifications.pushToast(
        'error',
        t('cloud.registerFailed'),
        t('cloud.humanVerificationRequired'),
      )
      return
    }
    submitting.value = true
    try {
      await authRegister(email.value, password.value, turnstileToken.value)
      notifications.pushToast('success', t('cloud.registerTitle'), t('cloud.registerSucceeded'))
      await router.push({ name: 'cloud-verify-email', query: { email: email.value } })
    } catch (err) {
      notifications.notifyError(t('cloud.registerFailed'), err)
    } finally {
      submitting.value = false
      turnstileToken.value = ''
      turnstile.value?.reset()
    }
  }

  async function registerWithExternalProvider(providerId: string) {
    if (externalSubmitting.value) {
      return
    }
    externalSubmitting.value = true
    try {
      window.location.href = await authExternalUrl(providerId)
    } catch (err) {
      notifications.notifyError(t('cloud.externalLoginFailed'), err)
    } finally {
      externalSubmitting.value = false
    }
  }

  function providerLabel(providerId: string) {
    return providerId === 'github' ? t('cloud.continueWithGitHub') : t('cloud.continueWithGoogle')
  }
</script>
