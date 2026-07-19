<template>
  <AuthPageFrame :title="t('cloud.changePassword')" @submit="submit">
    <AuthTextField
      v-model="currentPassword"
      :label="t('cloud.currentPassword')"
      type="password"
      autocomplete="current-password"
      required
      :disabled="submitAction.running"
    />
    <AuthTextField
      v-model="newPassword"
      :label="t('cloud.newPassword')"
      type="password"
      autocomplete="new-password"
      required
      :disabled="submitAction.running"
    />
    <button type="submit" :class="authPrimaryButtonClass" :disabled="submitAction.running">
      {{ submitAction.running ? t('cloud.changingPassword') : t('cloud.changePassword') }}
    </button>
  </AuthPageFrame>
</template>

<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import AuthPageFrame from '../../components/cloud/AuthPageFrame.vue'
  import AuthTextField from '../../components/cloud/AuthTextField.vue'
  import { authPrimaryButtonClass } from '../../components/cloud/authUi'
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
