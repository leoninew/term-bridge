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

      <div
        v-else-if="openedTabs.length > 0"
        class="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
      >
        <div
          v-for="pane in livePanes"
          :key="pane.session.id"
          class="absolute inset-0 flex min-h-0 min-w-0 flex-col overflow-hidden"
          :class="pane.active ? 'z-10' : 'pointer-events-none invisible z-0'"
          :aria-hidden="pane.active ? 'false' : 'true'"
        >
          <TerminalPane
            :session="pane.session"
            :tab="pane.tab"
            :ws-url="pane.wsUrl"
            :connection-enabled="pane.connectionEnabled"
            :active="pane.active"
            @terminal-state="emit('terminalState', $event)"
            @terminal-error="emit('terminalError', $event)"
          />
        </div>

        <div
          v-if="activeStoppedPane"
          class="absolute inset-0 z-10 flex min-h-0 min-w-0 flex-col overflow-hidden"
        >
          <TerminalPane
            :session="activeStoppedPane.session"
            :tab="activeStoppedPane.tab"
            :ws-url="null"
            :connection-enabled="false"
            :active="true"
            @terminal-state="emit('terminalState', $event)"
            @terminal-error="emit('terminalError', $event)"
          />
        </div>
      </div>

      <section
        v-else
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
  import { computed, onBeforeUnmount, useTemplateRef, watch } from 'vue'
  import { RouterLink } from 'vue-router'
  import { useI18n } from 'vue-i18n'
  import { Keyboard, SquareTerminal } from '@lucide/vue'
  import { TabsRoot } from 'reka-ui'
  import PageStatus from '../layout/PageStatus.vue'
  import SessionStatusBar from './SessionStatusBar.vue'
  import SessionTabStrip from './SessionTabStrip.vue'
  import TerminalPane from './TerminalPane.vue'
  import { sessionPanelTextActionClass } from './sessionUi'
  import { createTerminalKeepAliveController } from '../../features/sessions/terminalKeepAlive'
  import { useRuntimeConfigStore } from '../../store/runtimeConfig'
  import type { CloudSessionSummary } from '../../gen/proto/termbridge/cloud/v1/session'
  import type { DeviceSummary } from '../../gen/proto/termbridge/cloud/v1/device'
  import type { SessionSummary } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { ServerControlMessage } from '../../gen/proto/termbridge/agent/v1/terminal'
  import type { OpenSessionTab } from '../../store/workbench'

  const props = withDefaults(
    defineProps<{
      openedTabs: OpenSessionTab[]
      activeSessionId: string | null
      activeTab: OpenSessionTab | null
      activeSession: SessionSummary | null
      currentDevice: DeviceSummary | CloudSessionSummary | null
      resolveSession: (workspaceId: string, sessionId: string) => SessionSummary | null
      resolveWsUrl: (workspaceId: string, sessionId: string) => string | null
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
  const runtimeConfig = useRuntimeConfigStore()
  const keepAlive = createTerminalKeepAliveController({
    config: runtimeConfig.config.terminal.keepAlive,
  })

  const workbench = useTemplateRef<HTMLElement>('workbench')
  watch(workbench, (element) => emit('workbench', element), { immediate: true })

  const openedRunningSessionIds = computed(() =>
    props.openedTabs
      .filter((tab) => props.sessionLifecycleState(tab.workspaceId, tab.sessionId) === 'running')
      .map((tab) => tab.sessionId),
  )

  watch(
    [openedRunningSessionIds, () => props.activeSessionId],
    ([runningIds, activeId]) => {
      keepAlive.reconcile(runningIds, activeId)
    },
    { immediate: true },
  )

  watch(
    () => props.openedTabs.map((tab) => tab.sessionId).join(','),
    () => {
      const opened = new Set(props.openedTabs.map((tab) => tab.sessionId))
      for (const sessionId of [...keepAlive.mountedSessionIds.value]) {
        if (!opened.has(sessionId)) {
          keepAlive.remove(sessionId)
        }
      }
    },
  )

  const livePanes = computed(() => {
    const mountedIds = keepAlive.mountedSessionIds.value
    const hotIds = new Set(keepAlive.hotSessionIds.value)
    const panes: Array<{
      session: SessionSummary
      tab: OpenSessionTab
      wsUrl: string | null
      connectionEnabled: boolean
      active: boolean
    }> = []

    for (const sessionId of mountedIds) {
      const tab = props.openedTabs.find((item) => item.sessionId === sessionId)
      if (!tab) {
        continue
      }
      const session = props.resolveSession(tab.workspaceId, tab.sessionId)
      if (!session || session.lifecycle_state !== 'running') {
        continue
      }
      panes.push({
        session,
        tab,
        wsUrl: props.resolveWsUrl(tab.workspaceId, tab.sessionId),
        connectionEnabled: hotIds.has(sessionId),
        active: props.activeSessionId === sessionId,
      })
    }

    return panes
  })

  const activeStoppedPane = computed(() => {
    if (!props.activeSession || !props.activeTab) {
      return null
    }
    if (props.activeSession.lifecycle_state === 'running') {
      return null
    }
    return {
      session: props.activeSession,
      tab: props.activeTab,
    }
  })

  onBeforeUnmount(() => {
    keepAlive.disposeAll()
  })
</script>
