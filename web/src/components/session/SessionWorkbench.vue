<template>
  <section
    ref="workbench"
    class="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-[var(--color-panel-bg)]"
  >
    <TabsRoot
      :model-value="activeSessionId ?? undefined"
      class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
      @update:model-value="emit('activateTab', String($event))"
    >
      <SessionTabStrip
        :opened-tabs="openedTabs"
        :active-session-id="activeSessionId"
        :has-terminal-tabs="hasTerminalTabs"
        :has-background-running-sessions="hasBackgroundRunningSessions"
        :session-title="sessionTitle"
        :session-lifecycle-state="sessionLifecycleState"
        :show-sidebar-toggle="showSidebarToggle"
        :disable-tab-reorder="disableTabReorder"
        @activate-tab="emit('activateTab', $event)"
        @close-tab="(workspaceId, sessionId) => emit('closeTab', workspaceId, sessionId)"
        @close-terminal-tabs="emit('closeTerminalTabs')"
        @open-close-background-sessions-drawer="emit('openCloseBackgroundSessionsDrawer')"
        @reorder-tabs="emit('reorderTabs', $event)"
        @toggle-sidebar="emit('toggleSidebar')"
      />

      <PageStatus
        v-if="loading && openedTabs.length === 0"
        class="flex min-h-0 flex-1 items-center justify-center p-6"
        :loading="true"
        :loading-text="t('workbench.loadingContent')"
      >
        <template #loading>
          <div class="text-[var(--color-text-muted)]" role="status">
            {{ t('workbench.loadingContent') }}
          </div>
        </template>
      </PageStatus>

      <TerminalPane
        v-else-if="activeSession && activeTab"
        :session="activeSession"
        :tab="activeTab"
        :ws-url="terminalWsUrl"
        @terminal-state="emit('terminalState', $event)"
        @terminal-error="emit('terminalError', $event)"
      />

      <section
        v-else-if="openedTabs.length === 0"
        class="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 p-6 text-center text-[var(--color-text-muted)]"
      >
        <h3 class="text-lg font-semibold text-[var(--color-text)]">
          {{ t('workbench.noTabTitle') }}
        </h3>
        <p>{{ t('workbench.noTabDescription') }}</p>
        <div class="flex flex-wrap items-center justify-center gap-3">
          <button type="button" :class="sessionPanelTextActionClass" @click="emit('openCreate')">
            <SquareTerminal class="size-3.5" />
            {{ t('workbench.newSession') }}
          </button>
          <RouterLink :to="shortcutsRoute" :class="sessionPanelTextActionClass">
            <Keyboard class="size-3.5" />
            {{ t('workbench.manageShortcuts') }}
          </RouterLink>
        </div>
      </section>
    </TabsRoot>

    <SessionStatusBar
      :session="activeSession"
      :device="currentDevice"
      :show-cloud-connection="showCloudConnection"
    />
  </section>
</template>

<script setup lang="ts">
  import { useTemplateRef, watch } from 'vue'
  import { RouterLink } from 'vue-router'
  import { useI18n } from 'vue-i18n'
  import { Keyboard, SquareTerminal } from '@lucide/vue'
  import { TabsRoot } from 'reka-ui'
  import PageStatus from '../layout/PageStatus.vue'
  import SessionStatusBar from './SessionStatusBar.vue'
  import SessionTabStrip from './SessionTabStrip.vue'
  import TerminalPane from './TerminalPane.vue'
  import { sessionPanelTextActionClass } from './sessionUi'
  import type { CloudSessionSummary } from '../../gen/proto/termbridge/cloud/v1/session'
  import type { DeviceSummary } from '../../gen/proto/termbridge/cloud/v1/device'
  import type { SessionSummary } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { ServerControlMessage } from '../../gen/proto/termbridge/agent/v1/terminal'
  import type { OpenSessionTab } from '../../store/workbench'

  withDefaults(
    defineProps<{
      openedTabs: OpenSessionTab[]
      activeSessionId: string | null
      activeTab: OpenSessionTab | null
      activeSession: SessionSummary | null
      currentDevice: DeviceSummary | CloudSessionSummary | null
      terminalWsUrl: string | null
      hasTerminalTabs: boolean
      hasBackgroundRunningSessions: boolean
      sessionTitle: (workspaceId: string, sessionId: string) => string
      sessionLifecycleState: (workspaceId: string, sessionId: string) => string
      loading?: boolean
      showSidebarToggle?: boolean
      disableTabReorder?: boolean
      showCloudConnection?: boolean
      shortcutsRoute: { name: string; params?: Record<string, string> }
    }>(),
    {
      loading: false,
      showSidebarToggle: false,
      disableTabReorder: false,
      showCloudConnection: false,
    },
  )

  const emit = defineEmits<{
    activateTab: [sessionId: string]
    closeTab: [workspaceId: string, sessionId: string]
    closeTerminalTabs: []
    openCloseBackgroundSessionsDrawer: []
    reorderTabs: [tabs: OpenSessionTab[]]
    openCreate: []
    toggleSidebar: []
    workbench: [element: HTMLElement | null]
    terminalState: [message: ServerControlMessage]
    terminalError: [message: string]
  }>()

  const { t } = useI18n()
  const workbench = useTemplateRef<HTMLElement>('workbench')
  watch(workbench, (element) => emit('workbench', element), { immediate: true })
</script>
