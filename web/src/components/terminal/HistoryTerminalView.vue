<template>
  <section class="terminal-shell">
    <div ref="terminalElement" class="terminal-container" />
  </section>
</template>

<script setup lang="ts">
  import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { createXterm } from './useXterm'

  const props = defineProps<{
    history: string
  }>()

  const terminalElement = ref<HTMLElement | null>(null)
  let xterm: ReturnType<typeof createXterm> | null = null

  function replay() {
    xterm?.terminal.clear()
    if (props.history) {
      xterm?.write(new TextEncoder().encode(props.history))
    }
  }

  onMounted(() => {
    xterm = createXterm(
      () => undefined,
      () => undefined,
      () => undefined,
      { source: 'history' },
    )
    if (terminalElement.value) {
      xterm.open(terminalElement.value)
    }
    replay()
  })

  watch(
    () => props.history,
    () => replay(),
  )

  onBeforeUnmount(() => {
    xterm?.dispose()
  })
</script>
