<template>
  <AppPageShell
    main-class="flex min-h-0 items-center justify-center px-4 py-10 text-sm sm:px-6 sm:py-12"
  >
    <div class="w-full max-w-[26rem]">
      <form
        class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-6 shadow-[0_12px_40px_rgb(0_0_0_/_0.18)] sm:p-8"
        @submit.prevent="submit"
      >
        <h1
          class="mb-6 text-center text-2xl font-semibold leading-8 tracking-tight text-[var(--color-text-strong)] sm:text-[1.75rem]"
        >
          {{ t('cloud.resetPassword') }}
        </h1>
        <div class="space-y-5">
          <label class="flex flex-col gap-2">
            <span class="text-sm font-medium text-[var(--color-text)]">{{ t('cloud.email') }}</span>
            <input
              v-model="email"
              type="email"
              autocomplete="email"
              required
              :disabled="submitAction.running"
              class="h-10 w-full rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-control-bg)] px-3.5 text-sm text-[var(--color-text)] outline-none transition-colors focus:border-[var(--color-primary-border)] focus:ring-2 focus:ring-[var(--color-primary-border)]/20"
            />
          </label>
          <label class="flex flex-col gap-2">
            <span class="text-sm font-medium text-[var(--color-text)]">{{ t('cloud.code') }}</span>
            <input
              v-model="code"
              maxlength="6"
              autocomplete="one-time-code"
              inputmode="text"
              pattern="[A-Za-z0-9]{6}"
              required
              :disabled="submitAction.running"
              placeholder="ABC123"
              class="h-10 w-full rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-control-bg)] px-3.5 text-sm uppercase text-[var(--color-text)] outline-none transition-colors focus:border-[var(--color-primary-border)] focus:ring-2 focus:ring-[var(--color-primary-border)]/20"
            />
          </label>
          <label class="flex flex-col gap-2">
            <span class="text-sm font-medium text-[var(--color-text)]">
              {{ t('cloud.newPassword') }}
            </span>
            <input
              v-model="password"
              type="password"
              autocomplete="new-password"
              required
              :disabled="submitAction.running"
              class="h-10 w-full rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-control-bg)] px-3.5 text-sm text-[var(--color-text)] outline-none transition-colors focus:border-[var(--color-primary-border)] focus:ring-2 focus:ring-[var(--color-primary-border)]/20"
            />
          </label>
          <button
            type="submit"
            class="button button-primary h-10 w-full text-sm"
            :disabled="submitAction.running"
          >
            {{ t('cloud.resetPassword') }}
          </button>
        </div>
      </form>
    </div>
  </AppPageShell>
</template>

<script setup lang="ts">
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import AppPageShell from '../../components/layout/AppPageShell.vue'
  import { useAsyncAction } from '../../composable/useAsyncAction'
  import { authPasswordResetConfirm } from '../../features/cloud/api'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const notifications = useNotificationsStore()
  const email = ref(String(route.query.email ?? ''))
  const code = ref('')
  const password = ref('')
  const submitAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('cloud.resetPasswordFailed'), err),
  })

  async function submit() {
    if (submitAction.running) {
      return
    }
    if (!email.value) {
      notifications.pushToast('error', t('cloud.resetPasswordFailed'), t('message.emailRequired'))
      return
    }
    if (!code.value) {
      notifications.pushToast(
        'error',
        t('cloud.resetPasswordFailed'),
        t('message.verificationCodeRequired'),
      )
      return
    }
    if (code.value.length !== 6) {
      notifications.pushToast(
        'error',
        t('cloud.resetPasswordFailed'),
        t('message.verificationCodeInvalid'),
      )
      return
    }
    if (!password.value) {
      notifications.pushToast(
        'error',
        t('cloud.resetPasswordFailed'),
        t('message.newPasswordRequired'),
      )
      return
    }
    await submitAction.run(async () => {
      await authPasswordResetConfirm(email.value, code.value, password.value)
      notifications.pushToast(
        'success',
        t('cloud.resetPassword'),
        t('cloud.resetPasswordSucceeded'),
      )
      await router.push({ name: 'cloud-login' })
    })
  }
</script>
