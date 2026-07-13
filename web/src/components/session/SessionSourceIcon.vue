<template>
  <component
    :is="commandSource === 'shortcut' ? Keyboard : Command"
    :class="[sizeClass, 'shrink-0', 'text-[var(--color-text-subtle)]']"
    :aria-hidden="label ? undefined : 'true'"
    :aria-label="label"
    :title="label"
  />
</template>

<script setup lang="ts">
  import { Command, Keyboard } from '@lucide/vue'
  withDefaults(
    defineProps<{
      commandSource: string
      sizeClass?: string
      label?: string
    }>(),
    {
      sizeClass: 'size-4',
      label: undefined,
    },
  )
</script>
