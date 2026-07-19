<template>
  <template v-if="providerIds.length">
    <div :class="authDividerClass">
      <div class="h-px flex-1 bg-[var(--color-border)]" />
      <span class="font-medium uppercase tracking-wide">{{ t('cloud.or') }}</span>
      <div class="h-px flex-1 bg-[var(--color-border)]" />
    </div>
    <div class="space-y-3">
      <button
        v-for="providerId in providerIds"
        :key="providerId"
        type="button"
        :class="authProviderButtonClass"
        :disabled="disabled"
        @click="emit('select', providerId)"
      >
        <component
          :is="providerIcon(providerId)"
          v-if="showIcons && !busy"
          class="size-4 shrink-0"
          aria-hidden="true"
        />
        <span>{{ busy ? t('cloud.signingIn') : providerLabel(providerId) }}</span>
      </button>
    </div>
  </template>
</template>

<script setup lang="ts">
  import type { Component } from 'vue'
  import { useI18n } from 'vue-i18n'
  import GithubIcon from '../branding/GithubIcon.vue'
  import GoogleIcon from '../branding/GoogleIcon.vue'
  import { authDividerClass, authProviderButtonClass } from './authUi'

  withDefaults(
    defineProps<{
      providerIds: string[]
      disabled?: boolean
      busy?: boolean
      showIcons?: boolean
    }>(),
    {
      disabled: false,
      busy: false,
      showIcons: false,
    },
  )

  const emit = defineEmits<{
    select: [providerId: string]
  }>()

  const { t } = useI18n()

  function providerIcon(providerId: string): Component {
    return providerId === 'github' ? GithubIcon : GoogleIcon
  }

  function providerLabel(providerId: string) {
    return providerId === 'github' ? t('cloud.continueWithGitHub') : t('cloud.continueWithGoogle')
  }
</script>
