<template>
  <section class="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-[#090d14]">
    <TabsRoot
      :model-value="activeSessionId ?? undefined"
      class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
      @update:model-value="emit('activateTab', String($event))"
    >
      <div class="flex h-11 shrink-0 items-center border-b border-slate-800/80 bg-[#0a0f18] px-2">
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
                  ? 'border-slate-700 bg-slate-900 text-slate-50'
                  : 'border-slate-800 bg-slate-950/70 text-slate-400 hover:border-slate-700 hover:bg-slate-900/80'
              "
            >
              <TabsTrigger
                :value="tab.sessionId"
                class="tab-drag-handle flex min-w-0 items-center gap-1.5 rounded-md px-2 py-1.5 outline-none"
              >
                <SquareTerminal class="size-4 shrink-0 text-slate-500" aria-hidden="true" />
                <span class="truncate">{{ sessionTitle(tab.workspaceId, tab.sessionId) }}</span>
              </TabsTrigger>
              <button
                type="button"
                class="rounded-md p-1 text-slate-500 hover:bg-slate-800 hover:text-slate-200"
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
        class="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 p-6 text-center text-slate-500"
      >
        <h3 class="text-lg font-semibold text-slate-300">{{ t('workbench.noTabTitle') }}</h3>
        <p>
          {{
            !selectedDeviceId
              ? t('gateway.selectDevicePlaceholder')
              : t('workbench.noTabDescription')
          }}
        </p>
        <button
          type="button"
          class="rounded-md border border-slate-700 bg-slate-900 px-2.5 py-1.5 text-slate-100 hover:bg-slate-800"
          @click="emit('openCreate')"
        >
          {{ t('workbench.newSession') }}
        </button>
      </section>
    </TabsRoot>

    <footer
      class="flex h-8 shrink-0 items-center gap-1.5 overflow-hidden border-t border-slate-800/80 bg-[#0a0f18] px-3 text-sm text-slate-500"
    >
      <template v-if="activeSession">
        <span>{{ t('workbench.status') }}</span>
        <span class="text-slate-200">{{ activeLifecycleLabel }}</span>
        <span class="text-slate-700">·</span>
        <span>{{ t('workbench.command') }}</span>
        <span class="min-w-0 truncate text-slate-200">{{ activeSession.command }}</span>
      </template>
      <span v-else>{{ t('workbench.noActiveSession') }}</span>
    </footer>
  </section>
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { SquareTerminal, X } from '@lucide/vue'
  import { TabsList, TabsRoot, TabsTrigger } from 'reka-ui'
  import { VueDraggable } from 'vue-draggable-plus'
  import CreateSessionPanel from './CreateSessionPanel.vue'
  import TerminalPane from './TerminalPane.vue'
  import type { ServerControlMessage, SessionSummary } from '../../protocol/terminal'
  import type { OpenSessionTab } from '../../store/workbench'

  const props = defineProps<{
    openedTabs: OpenSessionTab[]
    activeSessionId: string | null
    activeTab: OpenSessionTab | null
    activeSession: SessionSummary | null
    selectedDeviceId: string
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
  }>()

  const { t } = useI18n()

  const activeLifecycleLabel = computed(() => {
    const state = props.activeSession?.lifecycle_state
    return state ? state.charAt(0).toUpperCase() + state.slice(1) : ''
  })
</script>
