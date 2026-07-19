<template>
  <Keyboard
    v-if="isShortcut"
    :class="[sizeClass, 'shrink-0']"
    :aria-hidden="label ? undefined : 'true'"
    :aria-label="label"
    :title="label"
  />
  <Terminal
    v-else
    :class="[sizeClass, 'shrink-0']"
    :aria-hidden="label ? undefined : 'true'"
    :aria-label="label"
    :title="label"
  />
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { Keyboard, Terminal } from '@lucide/vue'
  import { isShortcutLaunchMethod } from './launchMethod'

  const props = withDefaults(
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

  const isShortcut = computed(() => isShortcutLaunchMethod(props.commandSource))
</script>
