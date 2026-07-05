<template>
  <section
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
        <TabsList as-child>
          <VueDraggable
            :model-value="openedTabs"
            tag="div"
            class="flex min-w-0 flex-1 gap-1.5 overflow-x-auto"
            :animation="150"
            handle=".tab-drag-handle"
            item-key="sessionId"
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
                class="tab-drag-handle flex min-w-0 items-center gap-1.5 rounded-md px-2 py-1.5 outline-none"
              >
                <SquareTerminal
                  class="size-4 shrink-0 text-[var(--color-text-muted)]"
                  aria-hidden="true"
                />
                <span class="truncate">{{ sessionTitle(tab.workspaceId, tab.sessionId) }}</span>
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
        <div class="ml-auto flex shrink-0 items-center gap-1.5">
          <button
            type="button"
            class="inline-flex size-8 items-center justify-center rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] text-[var(--color-text)] hover:bg-[var(--color-control-hover)]"
            :aria-label="t('dashboard.title')"
            :title="t('dashboard.title')"
            @click="emit('openDashboard')"
          >
            <LayoutDashboard class="size-4 text-[var(--color-text-subtle)]" />
          </button>
          <button
            type="button"
            :disabled="!authenticated && userActionDisabled"
            class="inline-flex size-8 items-center justify-center rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] text-[var(--color-text)] hover:bg-[var(--color-control-hover)] disabled:cursor-not-allowed disabled:opacity-60"
            :aria-label="accountLabel"
            :title="accountLabel"
            @click="emit('openUserAuth')"
          >
            <User class="size-4 text-[var(--color-text-subtle)]" />
          </button>
        </div>
      </div>

      <CreateSessionPanel
        v-if="createSessionFormOpen"
        :cwd="createCwd"
        :name="createName"
        :command="createCommand"
        :creating="creatingSession"
        @update:cwd="emit('update:createCwd', $event)"
        @update:name="emit('update:createName', $event)"
        @update:command="emit('update:createCommand', $event)"
        @workbench="emit('createWorkbench', $event)"
        @submit="emit('submitCreate')"
        @cancel="emit('cancelCreate')"
      />

      <TerminalPane
        v-if="!createSessionFormOpen && activeSession && activeTab"
        :session="activeSession"
        :tab="activeTab"
        :ws-url="terminalWsUrl"
        @terminal-state="emit('terminalState', $event)"
        @terminal-error="emit('terminalError', $event)"
      />

      <section
        v-if="!createSessionFormOpen && openedTabs.length === 0"
        class="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 p-6 text-center text-[var(--color-text-muted)]"
      >
        <h3 class="text-lg font-semibold text-[var(--color-text)]">
          {{ t('workbench.noTabTitle') }}
        </h3>
        <p>{{ t('workbench.noTabDescription') }}</p>
        <button
          type="button"
          class="rounded-md border border-[var(--color-border-strong)] bg-[var(--color-control-active)] px-2.5 py-1.5 text-[var(--color-text-strong)] hover:bg-[var(--color-control-hover)]"
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
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { LayoutDashboard, SquareTerminal, User, X } from '@lucide/vue'
  import { TabsList, TabsRoot, TabsTrigger } from 'reka-ui'
  import { VueDraggable } from 'vue-draggable-plus'
  import CreateSessionPanel from './CreateSessionPanel.vue'
  import SessionStatusBar from './SessionStatusBar.vue'
  import TerminalPane from './TerminalPane.vue'
  import type {
    CloudSessionSummary,
    DeviceSummary,
  } from '../../gen/proto/termbridge/cloud/v1/cloud'
  import type { SessionSummary } from '../../gen/proto/termbridge/runtime/v1/runtime'
  import type { ServerControlMessage } from '../../gen/proto/termbridge/terminal/v1/terminal'
  import type { OpenSessionTab } from '../../store/workbench'

  const props = defineProps<{
    openedTabs: OpenSessionTab[]
    activeSessionId: string | null
    activeTab: OpenSessionTab | null
    activeSession: SessionSummary | null
    currentDevice: DeviceSummary | CloudSessionSummary | null
    authenticated: boolean
    userDisplayName: string
    userEmail: string
    userActionLabel: string
    userActionDisabled: boolean
    createSessionFormOpen: boolean
    createCwd: string
    createName: string
    createCommand: string
    creatingSession: boolean
    terminalWsUrl: string | null
    sessionTitle: (workspaceId: string, sessionId: string) => string
  }>()

  const emit = defineEmits<{
    activateTab: [sessionId: string]
    closeTab: [workspaceId: string, sessionId: string]
    reorderTabs: [tabs: OpenSessionTab[]]
    openCreate: []
    submitCreate: []
    cancelCreate: []
    'update:createCwd': [value: string]
    'update:createName': [value: string]
    'update:createCommand': [value: string]
    createWorkbench: [element: HTMLElement | null]
    terminalState: [message: ServerControlMessage]
    terminalError: [message: string]
    openDashboard: []
    openUserAuth: []
  }>()

  const { t } = useI18n()
  const accountLabel = computed(() =>
    props.authenticated ? props.userEmail || props.userDisplayName : props.userActionLabel,
  )
</script>
