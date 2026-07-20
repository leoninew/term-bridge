<template>
  <AppPageShell :main-class="authPageMainClass">
    <div :class="[authPageContentClass, 'text-center']">
      <section :class="authFormCardClass">
        <h1 :class="authFormTitleClass">{{ t('cloud.settingsTitle') }}</h1>
        <p class="text-sm leading-6 text-[var(--color-text-muted)]">
          {{ t('cloud.settingsPlaceholder') }}
        </p>
      </section>
    </div>
  </AppPageShell>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import {
    authFormCardClass,
    authFormTitleClass,
    authPageContentClass,
    authPageMainClass,
  } from '../../components/cloud/authUi'
  import AppPageShell from '../../components/layout/AppPageShell.vue'

  const { t } = useI18n()
</script>
