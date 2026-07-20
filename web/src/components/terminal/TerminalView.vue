<template>
  <section class="terminal-shell relative">
    <div ref="terminalElement" class="terminal-container" />

    <div
      v-if="showBootOverlay"
      class="pointer-events-none absolute inset-0 z-10 flex flex-col items-center justify-center gap-3 bg-[color-mix(in_srgb,var(--color-terminal-bg)_88%,transparent)] text-sm text-[var(--color-text-muted)] backdrop-blur-[1px] transition-opacity duration-200"
      role="status"
      aria-live="polite"
    >
      <Loader2 class="size-6 animate-spin text-[var(--color-text-subtle)]" aria-hidden="true" />
      <p class="px-4 text-center">{{ bootOverlayLabel }}</p>
    </div>

    <p v-if="replaying" class="terminal-message warning">{{ t('workbench.replayingHistory') }}</p>
    <p v-if="socket.error.value" class="terminal-message error">{{ socket.error.value }}</p>
  </section>
</template>

<script setup lang="ts">
  import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { Loader2 } from '@lucide/vue'
  import { clampTerminalSize } from '../../protocol/terminal'
  import type { ServerControlMessage } from '../../gen/proto/termbridge/agent/v1/terminal'
  import { terminalDebug } from './diagnostics'
  import { createXterm } from './useXterm'
  import { useTerminalSocket } from '../../features/sessions/useTerminalSocket'
  import { useThemeStore } from '../../store/theme'

  const props = withDefaults(
    defineProps<{
      wsUrl: string | null
      sessionId: string | null
      /** When false, close WS but keep xterm (frozen cold pane). */
      connectionEnabled?: boolean
      /** Visible/focused live pane. */
      active?: boolean
    }>(),
    {
      connectionEnabled: true,
      active: true,
    },
  )

  const emit = defineEmits<{
    state: [message: ServerControlMessage]
    terminalError: [message: string]
  }>()

  const { t } = useI18n()
  const terminalElement = ref<HTMLElement | null>(null)
  const themeStore = useThemeStore()
  const replaying = ref(false)
  const hasTrustedSize = ref(false)
  const sessionStarted = ref(false)
  let xterm: ReturnType<typeof createXterm> | null = null
  let lastTerminalSize: { cols: number; rows: number } | null = null
  let connectAttemptedForUrl: string | null = null
  let focusFrame: number | null = null
  let focusTimer: number | null = null

  const showBootOverlay = computed(() => {
    if (!props.active || !props.connectionEnabled) {
      return false
    }
    if (socket.error.value) {
      return false
    }
    if (!props.wsUrl) {
      return false
    }
    return !sessionStarted.value
  })

  const bootOverlayLabel = computed(() => {
    if (!hasTrustedSize.value) {
      return t('workbench.preparingTerminal')
    }
    if (socket.status.value !== 'connected') {
      return t('workbench.connectingTerminal')
    }
    return t('workbench.attachingTerminal')
  })

  const socket = useTerminalSocket(
    (data) => xterm?.write(data),
    (message) => {
      emit('state', message)
      if (message.type === 'started') {
        sessionStarted.value = true
        terminalDebug('socket.control.started', {
          sessionId: props.sessionId,
          lastTerminalSize,
          terminalCols: xterm?.terminal.cols,
          terminalRows: xterm?.terminal.rows,
        })
        xterm?.fit('started')
        if (props.active) {
          scheduleTerminalFocus('started')
        }
      }
      if (message.type === 'replay_started') {
        replaying.value = true
      }
      if (message.type === 'replay_finished') {
        replaying.value = false
        if (message.truncated) {
          console.warn(
            '[termbridge] terminal history replay truncated; only recent output is shown',
            { sessionId: props.sessionId },
          )
        }
      }
      if (message.type === 'error') {
        sessionStarted.value = true
        terminalDebug(
          'socket.control.error',
          {
            sessionId: props.sessionId,
            code: message.code,
            message: message.message,
          },
          { level: 'error' },
        )
        xterm?.terminal.writeln(`\r\n[termbridge] ${terminalErrorText(message)}`)
      }
      if (message.type === 'exited') {
        sessionStarted.value = true
        xterm?.terminal.writeln(`\r\n[termbridge] process exited with code ${message.exit_code}`)
      }
    },
    (message) => {
      sessionStarted.value = true
      emit('terminalError', message)
    },
  )

  function clearScheduledTerminalFocus() {
    if (focusFrame !== null) {
      window.cancelAnimationFrame(focusFrame)
      focusFrame = null
    }
    if (focusTimer !== null) {
      window.clearTimeout(focusTimer)
      focusTimer = null
    }
  }

  function focusTerminal(reason: string) {
    if (!xterm || !props.active) {
      return
    }
    terminalDebug('terminal-view.focus', {
      sessionId: props.sessionId,
      reason,
      sessionStarted: sessionStarted.value,
    })
    xterm.focus()
  }

  function scheduleTerminalFocus(reason: string) {
    if (!props.active) {
      return
    }
    clearScheduledTerminalFocus()
    focusFrame = window.requestAnimationFrame(() => {
      focusFrame = null
      focusTerminal(`${reason}:raf`)
      focusTimer = window.setTimeout(() => {
        focusTimer = null
        focusTerminal(`${reason}:settled`)
      }, 50)
    })
  }

  function disconnect(reason: string) {
    terminalDebug('socket.disconnect', {
      sessionId: props.sessionId,
      reason,
      socketStatus: socket.status.value,
    })
    connectAttemptedForUrl = null
    socket.close()
  }

  function connect(reason = 'manual') {
    if (!props.connectionEnabled) {
      terminalDebug('socket.attach.skipped-disabled', {
        sessionId: props.sessionId,
        reason,
      })
      return
    }
    if (!props.wsUrl) {
      return
    }
    if (!lastTerminalSize) {
      terminalDebug('socket.attach.wait-size', {
        sessionId: props.sessionId,
        reason,
        wsUrl: props.wsUrl,
        socketStatus: socket.status.value,
      })
      return
    }

    const url = new URL(props.wsUrl, window.location.href)
    url.searchParams.set('cols', String(lastTerminalSize.cols))
    url.searchParams.set('rows', String(lastTerminalSize.rows))
    const wsUrl = url.toString()

    if (
      connectAttemptedForUrl === wsUrl &&
      (socket.status.value === 'connecting' || socket.status.value === 'connected')
    ) {
      terminalDebug('socket.attach.skip-duplicate', {
        sessionId: props.sessionId,
        reason,
        cols: lastTerminalSize.cols,
        rows: lastTerminalSize.rows,
        socketStatus: socket.status.value,
      })
      return
    }

    connectAttemptedForUrl = wsUrl
    terminalDebug('socket.attach-size', {
      sessionId: props.sessionId,
      reason,
      cols: lastTerminalSize.cols,
      rows: lastTerminalSize.rows,
      socketStatus: socket.status.value,
    })
    socket.connect(wsUrl)
  }

  function handleTerminalSize(cols: number, rows: number, reason: string) {
    const previous = lastTerminalSize
    lastTerminalSize = clampTerminalSize({ cols, rows })
    hasTrustedSize.value = true
    terminalDebug('terminal-view.size', {
      sessionId: props.sessionId,
      reason,
      previous,
      cols: lastTerminalSize.cols,
      rows: lastTerminalSize.rows,
      socketStatus: socket.status.value,
      connectionEnabled: props.connectionEnabled,
      active: props.active,
    })

    if (!props.connectionEnabled) {
      return
    }

    if (socket.status.value === 'connected') {
      terminalDebug('socket.control.resize-send', {
        sessionId: props.sessionId,
        previous,
        cols: lastTerminalSize.cols,
        rows: lastTerminalSize.rows,
        socketStatus: socket.status.value,
      })
      socket.sendControl({
        type: 'resize',
        cols: lastTerminalSize.cols,
        rows: lastTerminalSize.rows,
        nonce: '',
      })
      return
    }

    if (socket.status.value === 'connecting') {
      socket.sendControl({
        type: 'resize',
        cols: lastTerminalSize.cols,
        rows: lastTerminalSize.rows,
        nonce: '',
      })
      return
    }

    connect(`size-ready:${reason}`)
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
    const el = terminalElement.value
    terminalDebug('terminal-view.mount', {
      sessionId: props.sessionId,
      wsUrl: props.wsUrl,
      connectionEnabled: props.connectionEnabled,
      active: props.active,
      hasElement: Boolean(el),
      clientWidth: el?.clientWidth,
      clientHeight: el?.clientHeight,
    })
    xterm = createXterm(
      (data) => {
        if (props.connectionEnabled) {
          socket.sendInput(data)
        }
      },
      (data) => {
        if (props.connectionEnabled) {
          socket.sendBinary(data)
        }
      },
      (cols, rows) => handleTerminalSize(cols, rows, 'xterm-onResize'),
      { source: 'live', sessionId: props.sessionId },
      themeStore.theme,
    )
    if (terminalElement.value) {
      xterm.open(terminalElement.value)
      if (props.active) {
        scheduleTerminalFocus('open')
      }
    } else {
      terminalDebug(
        'terminal-view.mount.missing-element',
        { sessionId: props.sessionId },
        { level: 'error' },
      )
    }
    connect('after-open')
  })

  watch(
    () => props.wsUrl,
    () => {
      if (!props.connectionEnabled) {
        return
      }
      connectAttemptedForUrl = null
      sessionStarted.value = false
      replaying.value = false
      connect('wsUrl-changed')
      void nextTick(() => scheduleTerminalFocus('wsUrl-changed'))
    },
  )

  watch(
    () => props.connectionEnabled,
    (enabled, wasEnabled) => {
      if (enabled === wasEnabled) {
        return
      }
      if (!enabled) {
        disconnect('connection-disabled')
        return
      }
      connectAttemptedForUrl = null
      sessionStarted.value = false
      replaying.value = false
      connect('connection-enabled')
      if (props.active) {
        void nextTick(() => {
          xterm?.fit('connection-enabled')
          scheduleTerminalFocus('connection-enabled')
        })
      }
    },
  )

  watch(
    () => props.active,
    (active) => {
      if (!active) {
        return
      }
      void nextTick(() => {
        xterm?.fit('activated')
        scheduleTerminalFocus('session-activated')
      })
    },
  )

  watch(
    () => props.sessionId,
    () => {
      if (props.active) {
        void nextTick(() => scheduleTerminalFocus('session-id-changed'))
      }
    },
  )

  watch(
    () => themeStore.theme,
    (theme) => xterm?.setTheme(theme),
  )

  onBeforeUnmount(() => {
    clearScheduledTerminalFocus()
    socket.close()
    xterm?.dispose()
  })
</script>
