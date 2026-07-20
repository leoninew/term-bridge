<template>
  <AuthPageFrame :title="t('cloud.registerTitle')" content-class="text-center" @submit="submit">
    <AuthTextField
      v-model="email"
      :label="t('cloud.email')"
      type="email"
      autocomplete="email"
      required
      :disabled="submitAction.running"
    />
    <AuthTextField
      v-model="password"
      :label="t('cloud.password')"
      type="password"
      autocomplete="new-password"
      required
      :disabled="submitAction.running"
    />
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
        :disabled="turnstileAction.running"
        @click="loadTurnstileSiteKey"
      >
        {{ t('common.retry') }}
      </button>
    </div>
    <button
      type="submit"
      :class="authPrimaryButtonClass"
      :disabled="submitAction.running || externalAction.running || !turnstileToken"
    >
      {{ submitAction.running ? t('common.creating') : t('cloud.register') }}
    </button>

    <template #after>
      <AuthExternalProviders
        :provider-ids="runtimeConfig.config.cloud.externalAuthProviderIds"
        :disabled="submitAction.running || externalAction.running"
        :busy="externalAction.running"
        show-icons
        @select="registerWithExternalProvider"
      />
    </template>

    <template #footer>
      {{ t('cloud.alreadyHaveAccount') }}
      <AuthInlineLink :to="{ name: 'cloud-login' }">
        {{ t('cloud.signIn') }}
      </AuthInlineLink>
    </template>
  </AuthPageFrame>
</template>

<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import AuthExternalProviders from '../../components/cloud/AuthExternalProviders.vue'
  import AuthInlineLink from '../../components/cloud/AuthInlineLink.vue'
  import AuthPageFrame from '../../components/cloud/AuthPageFrame.vue'
  import AuthTextField from '../../components/cloud/AuthTextField.vue'
  import { authPrimaryButtonClass } from '../../components/cloud/authUi'
  import TurnstileChallenge from '../../components/cloud/TurnstileChallenge.vue'
  import { useAsyncAction } from '../../composable/useAsyncAction'
  import { authExternalUrl, authRegister, authTurnstileSiteKey } from '../../features/cloud/api'
  import { useNotificationsStore } from '../../store/notifications'
  import { useRuntimeConfigStore } from '../../store/runtimeConfig'

  const { t } = useI18n()
  const router = useRouter()
  const notifications = useNotificationsStore()
  const runtimeConfig = useRuntimeConfigStore()
  const email = ref('')
  const password = ref('')
  const turnstileSiteKey = ref('')
  const turnstileToken = ref('')
  const turnstile = ref<InstanceType<typeof TurnstileChallenge> | null>(null)
  const turnstileAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('cloud.registerTitle'), err),
  })
  const submitAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('cloud.registerFailed'), err),
  })
  const externalAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('cloud.externalLoginFailed'), err),
  })

  onMounted(loadTurnstileSiteKey)

  async function loadTurnstileSiteKey() {
    await turnstileAction.run(async () => {
      turnstileSiteKey.value = await authTurnstileSiteKey()
    })
  }

  async function submit() {
    if (submitAction.running || externalAction.running) {
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
    try {
      await submitAction.run(
        async () => {
          await authRegister(email.value, password.value, turnstileToken.value)
          notifications.pushToast('success', t('cloud.registerTitle'), t('cloud.registerSucceeded'))
          await router.push({ name: 'cloud-verify-email', query: { email: email.value } })
        },
        { rethrow: true },
      )
    } catch {
      // notify handled by submitAction.onError
    } finally {
      turnstileToken.value = ''
      turnstile.value?.reset()
    }
  }

  async function registerWithExternalProvider(providerId: string) {
    if (submitAction.running || externalAction.running) {
      return
    }
    await externalAction.run(async () => {
      window.location.href = await authExternalUrl(providerId)
    })
  }
</script>
