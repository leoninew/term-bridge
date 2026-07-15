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
          {{ t('cloud.forgotPassword') }}
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
          <button
            type="submit"
            class="button button-primary h-10 w-full text-sm"
            :disabled="submitting"
          >
            {{ t('cloud.sendResetCode') }}
          </button>
        </div>
      </form>

      <p class="mt-6 text-sm leading-6 text-[var(--color-text-muted)]">
        <RouterLink
          class="text-[var(--color-primary-border)] outline-none transition-opacity hover:opacity-80 focus-visible:rounded focus-visible:ring-2 focus-visible:ring-[var(--color-primary-border)]/40"
          :to="{ name: 'cloud-login' }"
        >
          {{ t('cloud.backToLogin') }}
        </RouterLink>
      </p>
    </div>
  </AppPageShell>
</template>

<script setup lang="ts">
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter, RouterLink } from 'vue-router'
  import AppPageShell from '../../components/layout/AppPageShell.vue'
  import { authPasswordResetRequest } from '../../features/cloud/api'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const router = useRouter()
  const notifications = useNotificationsStore()
  const email = ref('')
  const submitting = ref(false)

  async function submit() {
    if (submitting.value) {
      return
    }
    if (!email.value) {
      notifications.pushToast('error', t('cloud.sendResetCodeFailed'), t('message.emailRequired'))
      return
    }
    submitting.value = true
    try {
      await authPasswordResetRequest(email.value)
      notifications.pushToast('success', t('cloud.forgotPassword'), t('cloud.resetCodeSent'))
      await router.push({ name: 'cloud-reset-password', query: { email: email.value } })
    } catch (err) {
      notifications.notifyError(t('cloud.sendResetCodeFailed'), err)
    } finally {
      submitting.value = false
    }
  }
</script>
