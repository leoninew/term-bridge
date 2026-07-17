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
      <div
        class="flex h-11 shrink-0 items-center gap-2 border-b border-[var(--color-border)] bg-[var(--color-panel-header)] px-2"
      >
        <button
          v-if="showSidebarToggle"
          type="button"
          class="button button-secondary button-icon shrink-0"
          :aria-label="t('workbench.openSidebarAria')"
          :title="t('workbench.openSidebarAria')"
          @click="emit('toggleSidebar')"
        >
          <PanelLeft class="size-4" />
        </button>
        <TabsList as-child>
          <VueDraggable
            :model-value="openedTabs"
            tag="div"
            class="tab-strip flex min-w-0 flex-1 gap-1.5 overflow-x-auto"
            ghost-class="tab-sortable-ghost"
            chosen-class="tab-sortable-chosen"
            drag-class="tab-sortable-dragging"
            :animation="150"
            handle=".tab-drag-handle"
            item-key="sessionId"
            :disabled="disableTabReorder"
            @update:model-value="emit('reorderTabs', $event)"
          >
            <div
              v-for="tab in openedTabs"
              :key="tab.sessionId"
              class="group relative flex max-w-56 shrink-0 items-center rounded-md border px-0.5 text-sm transition"
              :class="
                activeSessionId === tab.sessionId
                  ? 'border-[var(--color-border-strong)] bg-[var(--color-control-active)] text-[var(--color-text-strong)]'
                  : 'border-[var(--color-border)] bg-[var(--color-surface-muted)] text-[var(--color-text-muted)] hover:border-[var(--color-border-strong)] hover:bg-[var(--color-control-hover)]'
              "
            >
              <TabsTrigger
                :value="tab.sessionId"
                class="tab-drag-handle flex min-w-0 cursor-pointer items-center gap-1.5 rounded-md px-2 py-1.5 outline-none"
              >
                <SessionSourceIcon
                  :command-source="sessionCommandSource(tab.workspaceId, tab.sessionId)"
                  :label="sessionSourceLabel(tab.workspaceId, tab.sessionId)"
                />
                <span
                  class="truncate"
                  :class="
                    lifecycleStateClassName(sessionLifecycleState(tab.workspaceId, tab.sessionId))
                  "
                >
                  {{ sessionTitle(tab.workspaceId, tab.sessionId) }}
                </span>
              </TabsTrigger>
              <button
                type="button"
                class="rounded-md p-1 text-[var(--color-text-muted)] hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)]"
                :aria-label="
                  t('workbench.closeTabAria', {
                    name: sessionTitle(tab.workspaceId, tab.sessionId),
                  })
                "
                :title="
                  t('workbench.closeTabAria', {
                    name: sessionTitle(tab.workspaceId, tab.sessionId),
                  })
                "
                @click.stop="emit('closeTab', tab.workspaceId, tab.sessionId)"
              >
                <X class="size-3.5" />
              </button>
            </div>
          </VueDraggable>
        </TabsList>
        <DropdownMenuRoot>
          <DropdownMenuTrigger
            class="inline-flex size-8 shrink-0 items-center justify-center rounded-md text-[var(--color-text-subtle)] outline-none hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus:bg-[var(--color-control-hover)] focus:text-[var(--color-text)]"
            :aria-label="t('workbench.tabMenuAria')"
            :title="t('workbench.tabMenuAria')"
          >
            <MoreHorizontal class="size-4" aria-hidden="true" />
          </DropdownMenuTrigger>
          <DropdownMenuPortal>
            <DropdownMenuContent
              align="end"
              :side-offset="8"
              class="z-50 max-h-80 w-72 overflow-y-auto rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] p-1 text-sm text-[var(--color-text)] shadow-xl"
            >
              <DropdownMenuItem
                :disabled="!hasTerminalTabs"
                :title="t('workbench.closeTerminalTabsDescription')"
                class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none data-[disabled]:cursor-not-allowed data-[disabled]:opacity-50 data-[highlighted]:bg-[var(--color-control-hover)]"
                @select="emit('closeTerminalTabs')"
              >
                <CircleStop
                  class="size-4 shrink-0 text-[var(--color-text-subtle)]"
                  aria-hidden="true"
                />
                {{ t('workbench.closeTerminalTabs') }}
              </DropdownMenuItem>
              <DropdownMenuItem
                :disabled="!hasBackgroundRunningSessions"
                :title="t('workbench.closeBackgroundSessionsDescription')"
                class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none data-[disabled]:cursor-not-allowed data-[disabled]:opacity-50 data-[highlighted]:bg-[var(--color-control-hover)]"
                @select="emit('openCloseBackgroundSessionsDrawer')"
              >
                <SquareTerminal
                  class="size-4 shrink-0 text-[var(--color-text-subtle)]"
                  aria-hidden="true"
                />
                {{ t('workbench.closeBackgroundSessions') }}
              </DropdownMenuItem>
              <div class="my-1 border-t border-[var(--color-border)]" role="separator" />
              <p class="px-2 py-1 text-xs text-[var(--color-text-subtle)]">
                {{ t('workbench.openTabs') }}
              </p>
              <DropdownMenuItem
                v-for="tab in openedTabs"
                :key="tab.sessionId"
                class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none data-[highlighted]:bg-[var(--color-control-hover)]"
                @select="emit('activateTab', tab.sessionId)"
              >
                <SessionSourceIcon
                  :command-source="sessionCommandSource(tab.workspaceId, tab.sessionId)"
                  :label="sessionSourceLabel(tab.workspaceId, tab.sessionId)"
                />
                <span
                  class="min-w-0 flex-1 truncate"
                  :class="
                    lifecycleStateClassName(sessionLifecycleState(tab.workspaceId, tab.sessionId))
                  "
                >
                  {{ sessionTitle(tab.workspaceId, tab.sessionId) }}
                </span>
                <span
                  v-if="activeSessionId === tab.sessionId"
                  class="shrink-0 text-xs text-[var(--color-text-subtle)]"
                >
                  {{ t('workbench.activeTab') }}
                </span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenuPortal>
        </DropdownMenuRoot>
      </div>

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
        class="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 p-6 text-center text-[var(--color-text-muted)]"
      >
        <h3 class="text-lg font-semibold text-[var(--color-text)]">
          {{ t('workbench.noTabTitle') }}
        </h3>
        <p>{{ t('workbench.noTabDescription') }}</p>
        <button
          type="button"
          class="button button-secondary min-h-8 px-2.5 py-1.5 text-sm"
          @click="emit('openCreate')"
        >
          {{ t('workbench.newSession') }}
        </button>
      </section>
    </TabsRoot>

    <SessionStatusBar :session="activeSession" :device="currentDevice" />
  </section>
</template>

<script setup lang="ts">
  import { useTemplateRef, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import {
    PanelLeft,
    CircleStop, MoreHorizontal, SquareTerminal, X
  } from '@lucide/vue'
  import {
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuPortal,
    DropdownMenuRoot,
    DropdownMenuTrigger,
    TabsList,
    TabsRoot,
    TabsTrigger,
  } from 'reka-ui'
  import { VueDraggable } from 'vue-draggable-plus'
  import { lifecycleStateClassName } from '../../features/sessions/lifecycleState'
  import SessionSourceIcon from './SessionSourceIcon.vue'
  import PageStatus from '../layout/PageStatus.vue'
  import SessionStatusBar from './SessionStatusBar.vue'
  import TerminalPane from './TerminalPane.vue'
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
      sessionCommandSource: (workspaceId: string, sessionId: string) => string
      sessionSourceLabel: (workspaceId: string, sessionId: string) => string
      loading?: boolean
      showSidebarToggle?: boolean
      disableTabReorder?: boolean
    }>(),
    { loading: false, showSidebarToggle: false, disableTabReorder: false },
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

<style scoped>
  .tab-sortable-ghost {
    border-color: var(--color-border-strong) !important;
    border-style: dashed;
    background: var(--color-surface-muted) !important;
    color: var(--color-text-subtle) !important;
    opacity: 0.72;
  }

  .tab-sortable-chosen {
    border-color: var(--color-border-strong) !important;
    background: var(--color-control-hover) !important;
    box-shadow: 0 0 0 1px var(--color-border-strong);
    cursor: pointer;
  }

  .tab-sortable-dragging {
    border-color: var(--color-border-strong) !important;
    background: var(--color-control-active) !important;
    box-shadow: 0 8px 20px color-mix(in srgb, var(--color-text) 20%, transparent);
    cursor: pointer;
    opacity: 0.96;
  }
</style>
