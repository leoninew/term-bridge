<template>
  <AppPageShell :main-class="authPageMainClass">
    <div :class="[authPageContentClass, contentClass]">
      <form :class="authFormCardClass" @submit.prevent="emit('submit')">
        <h1 :class="authFormTitleClass">{{ title }}</h1>
        <div class="space-y-5">
          <slot />
        </div>
        <slot name="after" />
      </form>
      <div v-if="$slots.footer" :class="authFooterClass">
        <slot name="footer" />
      </div>
    </div>
  </AppPageShell>
</template>

<script setup lang="ts">
  import AppPageShell from '../layout/AppPageShell.vue'
  import {
    authFooterClass,
    authFormCardClass,
    authFormTitleClass,
    authPageContentClass,
    authPageMainClass,
  } from './authUi'

  withDefaults(
    defineProps<{
      title: string
      contentClass?: string
    }>(),
    { contentClass: '' },
  )

  const emit = defineEmits<{
    submit: []
  }>()
</script>
