<template>
  <section class="terminal-shell relative">
    <div ref="terminalElement" class="terminal-container" />
    <TerminalScrollFabs
      :show-top="scrollEdges.showTop"
      :show-bottom="scrollEdges.showBottom"
      @scroll-page-up="scrollPageUp"
      @scroll-page-down="scrollPageDown"
    />
  </section>
</template>

<script setup lang="ts">
  import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import TerminalScrollFabs from './TerminalScrollFabs.vue'
  import { createXterm, type TerminalScrollEdges } from './useXterm'
  import { useThemeStore } from '../../store/theme'

  const props = defineProps<{
    history: string
  }>()

  const terminalElement = ref<HTMLElement | null>(null)
  const scrollEdges = ref({ showTop: false, showBottom: false })
  const themeStore = useThemeStore()
  let xterm: ReturnType<typeof createXterm> | null = null

  function applyScrollEdges(edges: TerminalScrollEdges) {
    const fits = edges.atTop && edges.atBottom
    scrollEdges.value = {
      showTop: !fits && !edges.atTop,
      showBottom: !fits && !edges.atBottom,
    }
  }

  function replay() {
    xterm?.terminal.clear()
    if (props.history) {
      xterm?.write(new TextEncoder().encode(props.history))
    }
    // History may contain CSI ? 25 h; always re-hide after replay.
    xterm?.write(new TextEncoder().encode('\u001b[?25l'))
  }

  function scrollPageUp() {
    xterm?.scrollPageUp()
  }

  function scrollPageDown() {
    xterm?.scrollPageDown()
  }

  onMounted(() => {
    xterm = createXterm(
      () => undefined,
      () => undefined,
      () => undefined,
      { source: 'history' },
      themeStore.theme,
    )
    if (terminalElement.value) {
      xterm.open(terminalElement.value)
      xterm.setScrollEdgesListener(applyScrollEdges)
    }
    replay()
  })

  watch(
    () => props.history,
    () => replay(),
  )

  watch(
    () => themeStore.theme,
    (theme) => xterm?.setTheme(theme),
  )

  onBeforeUnmount(() => {
    xterm?.dispose()
  })
</script>
