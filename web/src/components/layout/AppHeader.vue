<template>
  <header
    class="flex h-16 items-center justify-between border-b border-[var(--color-border)] bg-[var(--color-panel-header)] px-8 2xl:px-12"
  >
    <RouterLink to="/" class="flex items-center gap-3">
      <BrandMark class="size-10 rounded-xl shadow-lg" />
      <p class="text-lg font-semibold text-[var(--color-text-strong)]">TermBridge</p>
    </RouterLink>

    <div class="flex items-center gap-3">
      <DisplayControls />
      <slot name="actions" />
    </div>
  </header>
</template>

<script setup lang="ts">
  import { RouterLink } from 'vue-router'
  import BrandMark from '../branding/BrandMark.vue'
  import DisplayControls from '../dashboard/DisplayControls.vue'
</script>
