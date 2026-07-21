<template>
  <div
    class="flex h-11 shrink-0 items-stretch gap-1 border-b border-[var(--color-border)] bg-[var(--color-panel-header)] pr-1"
    :class="showSidebarToggle ? 'pl-2' : 'pl-0'"
  >
    <button
      v-if="showSidebarToggle"
      type="button"
      :class="[sidebarHeaderIconButtonClass, 'self-center']"
      :aria-label="t('workbench.expandSidebarAria')"
      :title="t('workbench.expandSidebarAria')"
      @click="emit('toggleSidebar')"
    >
      <UnfoldHorizontal class="size-3.5" />
    </button>

    <TabsList as-child>
      <VueDraggable
        :model-value="openedTabs"
        tag="div"
        class="tab-strip flex h-full min-w-0 flex-1 overflow-x-auto"
        ghost-class="tab-sortable-ghost"
        chosen-class="tab-sortable-chosen"
        drag-class="tab-sortable-dragging"
        :animation="150"
        handle=".tab-drag-handle"
        filter="button:not(.tab-drag-handle)"
        :prevent-on-filter="false"
        item-key="sessionId"
        :disabled="disableTabReorder"
        @update:model-value="emit('reorderTabs', $event)"
      >
        <div
          v-for="tab in openedTabs"
          :key="tab.sessionId"
          class="group relative flex min-w-22 max-w-56 shrink-0 items-stretch border-0 border-r border-[var(--color-border)] text-[var(--color-text-muted)] data-[active=true]:bg-[var(--color-panel-bg)] data-[active=true]:text-[var(--color-text-strong)] data-[active=true]:shadow-[inset_0_2px_0_var(--color-primary-border),inset_0_-1px_0_var(--color-panel-bg)] data-[active=false]:hover:bg-[var(--color-control-hover)] data-[active=false]:hover:text-[var(--color-text)]"
          :data-active="activeSessionId === tab.sessionId ? 'true' : 'false'"
        >
          <TabsTrigger
            :value="tab.sessionId"
            class="tab-drag-handle grid h-full w-full min-w-0 cursor-pointer grid-cols-[1.25rem_minmax(0,1fr)] items-center gap-1 border-0 bg-transparent px-2 pr-6 text-left text-sm leading-tight text-inherit outline-none focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-[var(--color-primary-border)]"
          >
            <span class="inline-flex size-5 items-center justify-center" aria-hidden="true">
              <SquareTerminal
                class="size-3.5"
                :class="
                  sessionLifecycleState(tab.workspaceId, tab.sessionId) === 'running'
                    ? lifecycleStateClassName('running')
                    : 'text-[var(--color-text-subtle)] group-data-[active=true]:text-[var(--color-text-muted)]'
                "
              />
            </span>
            <span class="min-w-0 truncate">
              {{ sessionTitle(tab.workspaceId, tab.sessionId) }}
            </span>
          </TabsTrigger>
          <button
            type="button"
            class="absolute top-1/2 right-0.5 z-1 inline-flex size-5.5 -translate-y-1/2 items-center justify-center rounded border-0 bg-transparent p-0 text-inherit opacity-0 outline-none hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus-visible:bg-[var(--color-control-hover)] focus-visible:text-[var(--color-text)] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-[var(--color-primary-border)] group-hover:opacity-100 group-focus-within:opacity-100 group-data-[active=true]:opacity-100 [@media(hover:none)]:opacity-100"
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
        class="self-center"
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
            <CircleStop
              class="size-4 shrink-0 text-[var(--color-text-subtle)]"
              aria-hidden="true"
            />
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
            <SquareTerminal
              class="size-4 shrink-0"
              :class="
                sessionLifecycleState(tab.workspaceId, tab.sessionId) === 'running'
                  ? lifecycleStateClassName('running')
                  : 'text-[var(--color-text-subtle)]'
              "
              aria-hidden="true"
            />
            <span class="min-w-0 flex-1 truncate">
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
  import { CircleStop, MoreHorizontal, SquareTerminal, UnfoldHorizontal, X } from '@lucide/vue'
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
    sidebarHeaderIconButtonClass,
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
  }

  .tab-sortable-dragging {
    border-color: var(--color-border-strong) !important;
    background: var(--color-control-active) !important;
    box-shadow: 0 8px 20px color-mix(in srgb, var(--color-text) 20%, transparent);
    opacity: 0.96;
  }
</style>
