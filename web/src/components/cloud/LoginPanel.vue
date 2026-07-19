<template>
  <AuthPageFrame
    :title="t('cloud.signInTitle')"
    content-class="text-center"
    @submit="emit('submit')"
  >
    <AuthTextField
      :model-value="username"
      :label="t('cloud.username')"
      type="email"
      autocomplete="username"
      required
      @update:model-value="emit('update:username', $event)"
    />
    <AuthTextField
      :model-value="password"
      type="password"
      autocomplete="current-password"
      required
      @update:model-value="emit('update:password', $event)"
    >
      <template #label>
        <span>{{ t('cloud.password') }}</span>
        <AuthInlineLink :to="{ name: 'cloud-forgot-password' }" link-class="font-normal">
          {{ t('cloud.forgotPassword') }}
        </AuthInlineLink>
      </template>
    </AuthTextField>
    <slot />
    <button
      type="submit"
      :class="authPrimaryButtonClass"
      :disabled="loggingIn || externalLoggingIn || !turnstileReady"
    >
      {{ loggingIn ? t('cloud.signingIn') : t('cloud.signIn') }}
    </button>

    <template #after>
      <AuthExternalProviders
        :provider-ids="externalAuthProviderIds"
        :disabled="loggingIn || externalLoggingIn"
        :busy="externalLoggingIn"
        show-icons
        @select="emit('external', $event)"
      />
    </template>

    <template #footer>
      {{ t('cloud.newToTermBridge') }}
      <AuthInlineLink :to="{ name: 'cloud-register' }">
        {{ t('cloud.createAccount') }}
      </AuthInlineLink>
    </template>
  </AuthPageFrame>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import AuthExternalProviders from './AuthExternalProviders.vue'
  import AuthInlineLink from './AuthInlineLink.vue'
  import AuthPageFrame from './AuthPageFrame.vue'
  import AuthTextField from './AuthTextField.vue'
  import { authPrimaryButtonClass } from './authUi'

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
</script>
