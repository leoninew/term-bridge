<template>
  <section class="terminal-shell">
    <div ref="terminalElement" class="terminal-container" />
    <p v-if="replaying" class="terminal-message warning">Replaying bounded history…</p>
    <p v-if="socket.error.value" class="terminal-message error">{{ socket.error.value }}</p>
  </section>
</template>

<script setup lang="ts">
  import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { clampTerminalSize } from '../../protocol/terminal'
  import type { ServerControlMessage } from '../../gen/proto/termbridge/agent/v1/terminal'
  import { logTerminalDiagnostic, logTerminalDiagnosticError } from './diagnostics'
  import { createXterm } from './useXterm'
  import { useTerminalSocket } from '../../features/sessions/useTerminalSocket'
  import { useThemeStore } from '../../store/theme'

  const props = defineProps<{
    wsUrl: string | null
    sessionId: string | null
  }>()

  const emit = defineEmits<{
    state: [message: ServerControlMessage]
    terminalError: [message: string]
  }>()

  const terminalElement = ref<HTMLElement | null>(null)
  const themeStore = useThemeStore()
  const replaying = ref(false)
  let xterm: ReturnType<typeof createXterm> | null = null
  let lastTerminalSize: { cols: number; rows: number } | null = null

  const socket = useTerminalSocket(
    (data) => xterm?.write(data),
    (message) => {
      emit('state', message)
      if (message.type === 'started') {
        xterm?.fit()
      }
      if (message.type === 'replay_started') {
        replaying.value = true
      }
      if (message.type === 'replay_finished') {
        replaying.value = false
        if (message.truncated) {
          xterm?.terminal.writeln('\r\n[termbridge] 当前仅显示最近终端历史，较早输出已截断。')
        }
      }
      if (message.type === 'error') {
        logTerminalDiagnosticError('socket.control.error', {
          sessionId: props.sessionId,
          code: message.code,
          message: message.message,
        })
        xterm?.terminal.writeln(`\r\n[termbridge] ${terminalErrorText(message)}`)
      }
      if (message.type === 'exited') {
        xterm?.terminal.writeln(`\r\n[termbridge] process exited with code ${message.exit_code}`)
      }
    },
    (message) => emit('terminalError', message),
  )

  function connect() {
    if (!props.wsUrl) {
      return
    }
    const url = new URL(props.wsUrl, window.location.href)
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
    if (lastTerminalSize) {
      url.searchParams.set('cols', String(lastTerminalSize.cols))
      url.searchParams.set('rows', String(lastTerminalSize.rows))
    } else {
      logTerminalDiagnosticError('socket.attach-size.missing', { sessionId: props.sessionId })
    }
    logTerminalDiagnostic('socket.attach-size', {
      sessionId: props.sessionId,
      cols: lastTerminalSize?.cols,
      rows: lastTerminalSize?.rows,
    })
    socket.connect(url.toString())
  }

  function terminalErrorText(message: ServerControlMessage): string {
    if (message.code === 'terminal_stream_error' && message.message === 'client queue full') {
      return '当前浏览器连接消费终端输出过慢，已断开以保护会话。重新连接可继续查看最新输出。'
    }
    return `${message.code}: ${safeTerminalText(message.message)}`
  }

  function safeTerminalText(value: string): string {
    return Array.from(value)
      .filter((char) => {
        const code = char.charCodeAt(0)
        return code >= 0x20 && code !== 0x7f && code !== 0x9b
      })
      .join('')
  }

  onMounted(() => {
    xterm = createXterm(
      (data) => socket.sendInput(data),
      (data) => socket.sendBinary(data),
      (cols, rows) => {
        lastTerminalSize = clampTerminalSize({ cols, rows })
        socket.sendControl({
          type: 'resize',
          cols: lastTerminalSize.cols,
          rows: lastTerminalSize.rows,
          nonce: '',
        })
      },
      { source: 'live', sessionId: props.sessionId },
      themeStore.theme,
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

  watch(
    () => themeStore.theme,
    (theme) => xterm?.setTheme(theme),
  )

  onBeforeUnmount(() => {
    socket.close()
    xterm?.dispose()
  })
</script>
