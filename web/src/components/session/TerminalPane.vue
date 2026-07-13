<template>
  <!-- Keep this aligned with SessionWorkbench TabsTrigger: sessionId is the globally unique Tab value. -->
  <TabsContent
    v-if="session && tab"
    :key="session.id"
    :value="tab.sessionId"
    class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-[var(--color-panel-bg)] p-2"
  >
    <TerminalView
      v-if="session.lifecycle_state === 'running'"
      :key="session.id"
      :ws-url="wsUrl"
      :session-id="session.id"
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
  </TabsContent>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import { TabsContent } from 'reka-ui'
  import HistoryTerminalView from '../terminal/HistoryTerminalView.vue'
  import TerminalView from '../terminal/TerminalView.vue'
  import type { SessionSummary } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { ServerControlMessage } from '../../gen/proto/termbridge/agent/v1/terminal'
  import type { OpenSessionTab } from '../../store/workbench'

  defineProps<{
    session: SessionSummary | null
    tab: OpenSessionTab | null
    wsUrl: string | null
  }>()

  const emit = defineEmits<{
    terminalState: [message: ServerControlMessage]
    terminalError: [message: string]
  }>()

  const { t } = useI18n()
</script>
