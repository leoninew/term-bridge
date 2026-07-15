<template>
  <section class="flex min-h-screen flex-col bg-[var(--color-app-bg)] text-[var(--color-text)]">
    <AppHeader>
      <template v-if="$slots.actions" #actions>
        <slot name="actions" />
      </template>
    </AppHeader>
    <main :class="['min-h-0 flex-1', mainClass]">
      <slot />
    </main>
  </section>
</template>

<script setup lang="ts">
  import AppHeader from './AppHeader.vue'

  withDefaults(
    defineProps<{
      mainClass?: string
    }>(),
    { mainClass: '' },
  )
</script>
