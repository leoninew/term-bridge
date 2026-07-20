<template>
  <AuthPageFrame :title="t('cloud.verifyEmailTitle')" @submit="submit">
    <AuthTextField
      v-model="email"
      :label="t('cloud.email')"
      type="email"
      autocomplete="email"
      required
      :disabled="busy"
    />
    <AuthTextField
      v-model="code"
      :label="t('cloud.code')"
      maxlength="6"
      autocomplete="one-time-code"
      inputmode="text"
      pattern="[A-Za-z0-9]{6}"
      required
      :disabled="busy"
      placeholder="ABC123"
      input-class="uppercase"
    />
    <button type="submit" :class="authPrimaryButtonClass" :disabled="busy">
      {{ t('cloud.verifyEmail') }}
    </button>
    <button type="button" :class="authSecondaryButtonClass" :disabled="busy" @click="resend">
      {{ t('cloud.resendCode') }}
    </button>
  </AuthPageFrame>
</template>

<script setup lang="ts">
  import { computed, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import AuthPageFrame from '../../components/cloud/AuthPageFrame.vue'
  import AuthTextField from '../../components/cloud/AuthTextField.vue'
  import { authPrimaryButtonClass, authSecondaryButtonClass } from '../../components/cloud/authUi'
  import { useAsyncAction } from '../../composable/useAsyncAction'
  import { authResendVerification, authVerifyEmail } from '../../features/cloud/api'
  import { useNotificationsStore } from '../../store/notifications'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const notifications = useNotificationsStore()
  const email = ref(String(route.query.email ?? ''))
  const code = ref('')
  const submitAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('cloud.verifyEmailFailed'), err),
  })
  const resendAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('cloud.resendVerificationFailed'), err),
  })
  const busy = computed(() => submitAction.running || resendAction.running)

  async function submit() {
    if (busy.value) {
      return
    }
    if (!email.value) {
      notifications.pushToast('error', t('cloud.verifyEmailFailed'), t('message.emailRequired'))
      return
    }
    if (!code.value) {
      notifications.pushToast(
        'error',
        t('cloud.verifyEmailFailed'),
        t('message.verificationCodeRequired'),
      )
      return
    }
    if (code.value.length !== 6) {
      notifications.pushToast(
        'error',
        t('cloud.verifyEmailFailed'),
        t('message.verificationCodeInvalid'),
      )
      return
    }
    await submitAction.run(async () => {
      await authVerifyEmail(email.value, code.value)
      notifications.pushToast(
        'success',
        t('cloud.verifyEmailTitle'),
        t('cloud.verifyEmailSucceeded'),
      )
      await router.push({ name: 'cloud-login' })
    })
  }

  async function resend() {
    if (busy.value) {
      return
    }
    if (!email.value) {
      notifications.pushToast(
        'error',
        t('cloud.resendVerificationFailed'),
        t('message.emailRequired'),
      )
      return
    }
    await resendAction.run(async () => {
      await authResendVerification(email.value)
      notifications.pushToast(
        'success',
        t('cloud.verifyEmailTitle'),
        t('cloud.verificationCodeSent'),
      )
    })
  }
</script>
