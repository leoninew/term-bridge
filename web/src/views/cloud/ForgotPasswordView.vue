<template>
  <AuthPageFrame :title="t('cloud.forgotPassword')" content-class="text-center" @submit="submit">
    <AuthTextField
      v-model="email"
      :label="t('cloud.email')"
      type="email"
      autocomplete="email"
      required
      :disabled="submitAction.running"
    />
    <button type="submit" :class="authPrimaryButtonClass" :disabled="submitAction.running">
      {{ t('cloud.sendResetCode') }}
    </button>

    <template #footer>
      <AuthInlineLink :to="{ name: 'cloud-login' }">
        {{ t('cloud.backToLogin') }}
      </AuthInlineLink>
    </template>
  </AuthPageFrame>
</template>

<script setup lang="ts">
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import AuthInlineLink from '../../components/cloud/AuthInlineLink.vue'
  import AuthPageFrame from '../../components/cloud/AuthPageFrame.vue'
  import AuthTextField from '../../components/cloud/AuthTextField.vue'
  import { authPrimaryButtonClass } from '../../components/cloud/authUi'
  import { useAsyncAction } from '../../composable/useAsyncAction'
  import { authPasswordResetRequest } from '../../features/cloud/api'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const router = useRouter()
  const notifications = useNotificationsStore()
  const email = ref('')
  const submitAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('cloud.sendResetCodeFailed'), err),
  })

  async function submit() {
    if (submitAction.running) {
      return
    }
    if (!email.value) {
      notifications.pushToast('error', t('cloud.sendResetCodeFailed'), t('message.emailRequired'))
      return
    }
    await submitAction.run(async () => {
      await authPasswordResetRequest(email.value)
      notifications.pushToast('success', t('cloud.forgotPassword'), t('cloud.resetCodeSent'))
      await router.push({ name: 'cloud-reset-password', query: { email: email.value } })
    })
  }
</script>
