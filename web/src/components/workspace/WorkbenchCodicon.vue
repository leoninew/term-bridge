<template>
  <span
    class="codicon"
    :class="[iconClass, spin ? 'codicon-modifier-spin' : null, className]"
    aria-hidden="true"
  />
</template>

<script setup lang="ts">
  import { computed } from 'vue'

  const props = withDefaults(
    defineProps<{
      name: string
      spin?: boolean
      className?: string
    }>(),
    {
      spin: false,
      className: '',
    },
  )

  const iconClass = computed(() => `codicon-${props.name}`)
</script>
