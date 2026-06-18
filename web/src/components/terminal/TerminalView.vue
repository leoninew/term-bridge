<template>
  <section class="terminal-shell">
    <header class="terminal-toolbar">
      <div>
        <span class="eyebrow">Terminal</span>
        <strong>{{ sessionId ?? 'No session selected' }}</strong>
      </div>
      <div class="terminal-actions">
        <span class="socket-status" :data-state="socket.status.value">
          {{ socket.status.value }}
        </span>
        <button type="button" :disabled="!wsUrl" @click="detach">Detach</button>
        <button type="button" class="danger" :disabled="!wsUrl" @click="closeSession">Close</button>
      </div>
    </header>
    <div ref="terminalElement" class="terminal-container" />
    <p v-if="socket.error.value" class="terminal-error">{{ socket.error.value }}</p>
  </section>
</template>

<script setup lang="ts">
  import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import type { ServerControlMessage } from '../../protocol/terminal'
  import { createXterm } from './useXterm'
  import { useTerminalSocket } from '../../features/sessions/useTerminalSocket'

  const props = defineProps<{
    wsUrl: string | null
    sessionId: string | null
  }>()

  const emit = defineEmits<{
    state: [message: ServerControlMessage]
  }>()

  const terminalElement = ref<HTMLElement | null>(null)
  let xterm: ReturnType<typeof createXterm> | null = null

  const socket = useTerminalSocket(
    (data) => xterm?.write(data),
    (message) => {
      emit('state', message)
      if (message.type === 'error') {
        xterm?.terminal.writeln(`\r\n[termbridge:${message.code}] ${message.message}`)
      }
      if (message.type === 'exited') {
        xterm?.terminal.writeln(`\r\n[termbridge] process exited with code ${message.exit_code}`)
      }
    },
  )

  function connect() {
    if (!props.wsUrl) {
      return
    }
    const url = new URL(props.wsUrl, window.location.href)
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
    socket.connect(url.toString())
  }

  function detach() {
    socket.sendControl({ type: 'detach' })
    socket.close()
  }

  function closeSession() {
    socket.sendControl({ type: 'close' })
    socket.close()
  }

  onMounted(() => {
    xterm = createXterm(
      (data) => socket.sendInput(data),
      (data) => socket.sendBinary(data),
      (cols, rows) => socket.sendControl({ type: 'resize', cols, rows }),
    )
    if (terminalElement.value) {
      xterm.open(terminalElement.value)
    }
    connect()
  })

  watch(
    () => props.wsUrl,
    () => connect(),
  )

  onBeforeUnmount(() => {
    socket.close()
    xterm?.dispose()
  })
</script>
