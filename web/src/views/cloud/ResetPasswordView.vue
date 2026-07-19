<template>
  <AuthPageFrame :title="t('cloud.resetPassword')" @submit="submit">
    <AuthTextField
      v-model="email"
      :label="t('cloud.email')"
      type="email"
      autocomplete="email"
      required
      :disabled="submitAction.running"
    />
    <AuthTextField
      v-model="code"
      :label="t('cloud.code')"
      maxlength="6"
      autocomplete="one-time-code"
      inputmode="text"
      pattern="[A-Za-z0-9]{6}"
      required
      :disabled="submitAction.running"
      placeholder="ABC123"
      input-class="uppercase"
    />
    <AuthTextField
      v-model="password"
      :label="t('cloud.newPassword')"
      type="password"
      autocomplete="new-password"
      required
      :disabled="submitAction.running"
    />
    <button type="submit" :class="authPrimaryButtonClass" :disabled="submitAction.running">
      {{ t('cloud.resetPassword') }}
    </button>
  </AuthPageFrame>
</template>

<script setup lang="ts">
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import AuthPageFrame from '../../components/cloud/AuthPageFrame.vue'
  import AuthTextField from '../../components/cloud/AuthTextField.vue'
  import { authPrimaryButtonClass } from '../../components/cloud/authUi'
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
