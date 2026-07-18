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
          {{ t('cloud.changePassword') }}
        </h1>
        <div class="space-y-5">
          <label class="flex flex-col gap-2">
            <span class="text-sm font-medium text-[var(--color-text)]">
              {{ t('cloud.currentPassword') }}
            </span>
            <input
              v-model="currentPassword"
              type="password"
              autocomplete="current-password"
              required
              :disabled="submitAction.running"
              class="h-10 w-full rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-control-bg)] px-3.5 text-sm text-[var(--color-text)] outline-none transition-colors focus:border-[var(--color-primary-border)] focus:ring-2 focus:ring-[var(--color-primary-border)]/20"
            />
          </label>
          <label class="flex flex-col gap-2">
            <span class="text-sm font-medium text-[var(--color-text)]">
              {{ t('cloud.newPassword') }}
            </span>
            <input
              v-model="newPassword"
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
            {{ submitAction.running ? t('cloud.changingPassword') : t('cloud.changePassword') }}
          </button>
        </div>
      </form>
    </div>
  </AppPageShell>
</template>

<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import AppPageShell from '../../components/layout/AppPageShell.vue'
  import { useAsyncAction } from '../../composable/useAsyncAction'
  import { authChangePassword, authLogout } from '../../features/cloud/api'
  import { useCloudAuthStore } from '../../store/cloudAuth'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const router = useRouter()
  const cloudAuth = useCloudAuthStore()
  const notifications = useNotificationsStore()
  const currentPassword = ref('')
  const newPassword = ref('')
  const submitAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('cloud.changePasswordFailed'), err),
  })

  onMounted(async () => {
    await cloudAuth.initialize()
    if (!cloudAuth.authenticated || cloudAuth.user?.provider !== 'email') {
      await router.replace({ name: 'home' })
    }
  })

  async function submit() {
    if (submitAction.running) {
      return
    }
    if (!currentPassword.value) {
      notifications.pushToast(
        'error',
        t('cloud.changePasswordFailed'),
        t('message.currentPasswordRequired'),
      )
      return
    }
    if (!newPassword.value) {
      notifications.pushToast(
        'error',
        t('cloud.changePasswordFailed'),
        t('message.newPasswordRequired'),
      )
      return
    }
    if (currentPassword.value === newPassword.value) {
      notifications.pushToast(
        'error',
        t('cloud.changePasswordFailed'),
        t('message.passwordUnchanged'),
      )
      return
    }
    await submitAction.run(async () => {
      await authChangePassword(currentPassword.value, newPassword.value)
      currentPassword.value = ''
      newPassword.value = ''
      try {
        await authLogout()
      } catch {
        // ignore logout API errors; clear local state anyway
      }
      cloudAuth.clearToken()
      notifications.pushToast('success', t('cloud.changePasswordSucceeded'), '')
      await router.replace({ name: 'cloud-login' })
    })
  }
</script>