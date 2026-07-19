<template>
  <!-- Live keep-alive panes are stacked outside TabsContent to avoid unmount on tab switch. -->
  <section
    v-if="session && tab"
    class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-[var(--color-panel-bg)]"
  >
    <TerminalView
      v-if="session.lifecycle_state === 'running'"
      :key="session.id"
      :ws-url="wsUrl"
      :session-id="session.id"
      :connection-enabled="connectionEnabled"
      :active="active"
      @state="emit('terminalState', $event)"
      @terminal-error="emit('terminalError', $event)"
    />

    <section v-else class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <div
        v-if="tab.historyLoading"
        class="flex flex-1 items-center justify-center text-[var(--color-text-muted)]"
      >
        {{ t('workbench.loadingHistory') }}
      </div>
      <div
        v-else-if="tab.historyError"
        class="rounded-md border border-[var(--color-danger-border)] bg-[var(--color-danger-bg)] px-2 py-1.5 text-[var(--color-danger-text)]"
      >
        {{ tab.historyError }}
      </div>
      <HistoryTerminalView
        v-else-if="tab.historyText"
        :key="`${session.id}-history`"
        :history="tab.historyText"
      />
      <div v-else class="flex flex-1 items-center justify-center text-[var(--color-text-muted)]">
        {{ t('workbench.noHistory') }}
      </div>
    </section>
  </section>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import HistoryTerminalView from '../terminal/HistoryTerminalView.vue'
  import TerminalView from '../terminal/TerminalView.vue'
  import type { SessionSummary } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { ServerControlMessage } from '../../gen/proto/termbridge/agent/v1/terminal'
  import type { OpenSessionTab } from '../../store/workbench'

  withDefaults(
    defineProps<{
      session: SessionSummary | null
      tab: OpenSessionTab | null
      wsUrl: string | null
      connectionEnabled?: boolean
      active?: boolean
    }>(),
    {
      connectionEnabled: true,
      active: true,
    },
  )

  const emit = defineEmits<{
    terminalState: [message: ServerControlMessage]
    terminalError: [message: string]
  }>()

  const { t } = useI18n()
</script>