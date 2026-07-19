<template>
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
            <SquareTerminal class="size-4 shrink-0 text-[var(--color-text-subtle)]" aria-hidden="true" />
            <span
              class="truncate"
              :class="lifecycleStateClassName(sessionLifecycleState(tab.workspaceId, tab.sessionId))"
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
        :class="sessionIconButtonClass"
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
            :class="sessionMenuItemInteractiveClass"
            @select="emit('closeTerminalTabs')"
          >
            <CircleStop class="size-4 shrink-0 text-[var(--color-text-subtle)]" aria-hidden="true" />
            {{ t('workbench.closeTerminalTabs') }}
          </DropdownMenuItem>
          <DropdownMenuItem
            :disabled="!hasBackgroundRunningSessions"
            :title="t('workbench.closeBackgroundSessionsDescription')"
            :class="sessionMenuItemInteractiveClass"
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
            <SquareTerminal class="size-4 shrink-0 text-[var(--color-text-subtle)]" aria-hidden="true" />
            <span
              class="min-w-0 flex-1 truncate"
              :class="lifecycleStateClassName(sessionLifecycleState(tab.workspaceId, tab.sessionId))"
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
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import { CircleStop, MoreHorizontal, PanelLeft, SquareTerminal, X } from '@lucide/vue'
  import {
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuPortal,
    DropdownMenuRoot,
    DropdownMenuTrigger,
    TabsList,
    TabsTrigger,
  } from 'reka-ui'
  import { VueDraggable } from 'vue-draggable-plus'
  import { lifecycleStateClassName } from '../../features/sessions/lifecycleState'
  import type { OpenSessionTab } from '../../store/workbench'
    import {
    sessionIconButtonClass,
    sessionMenuItemInteractiveClass,
  } from './sessionUi'

  withDefaults(
    defineProps<{
      openedTabs: OpenSessionTab[]
      activeSessionId: string | null
      hasTerminalTabs: boolean
      hasBackgroundRunningSessions: boolean
      sessionTitle: (workspaceId: string, sessionId: string) => string
      sessionLifecycleState: (workspaceId: string, sessionId: string) => string
      showSidebarToggle?: boolean
      disableTabReorder?: boolean
    }>(),
    {
      showSidebarToggle: false,
      disableTabReorder: false,
    },
  )

  const emit = defineEmits<{
    activateTab: [sessionId: string]
    closeTab: [workspaceId: string, sessionId: string]
    closeTerminalTabs: []
    openCloseBackgroundSessionsDrawer: []
    reorderTabs: [tabs: OpenSessionTab[]]
    toggleSidebar: []
  }>()

  const { t } = useI18n()
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
