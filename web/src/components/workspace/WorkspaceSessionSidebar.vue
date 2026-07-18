<template>
  <aside
    class="flex h-full min-w-0 flex-col overflow-hidden bg-[var(--color-sidebar-bg)] text-[var(--color-text)]"
  >
    <header
      class="flex h-11 shrink-0 items-center border-b border-[var(--color-border)] bg-[var(--color-panel-header)] px-2"
    >
      <div class="flex w-full items-center gap-1.5">
        <RouterLink
          :to="{ name: props.homeRouteName }"
          class="flex size-8 shrink-0 items-center justify-center rounded-md bg-blue-600 text-sm font-semibold text-white shadow-sm hover:bg-blue-500 focus:bg-blue-500 focus:outline-none"
          aria-label="TermBridge"
          title="TermBridge"
        >
          TB
        </RouterLink>
        <label class="relative min-w-0 flex-1">
          <span class="sr-only">{{ t('sidebar.searchSessions') }}</span>
          <Search
            class="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-[var(--color-text-subtle)]"
          />
          <input
            v-model="searchQuery"
            type="search"
            :placeholder="t('sidebar.searchPlaceholder')"
            class="h-7 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] py-1 pl-8 pr-2 text-sm text-[var(--color-text)] outline-none placeholder:text-[var(--color-text-subtle)]"
          />
        </label>
        <button
          type="button"
          class="button button-secondary button-icon file-workbench-small-button"
          :aria-label="t('sidebar.newSession')"
          :title="t('sidebar.newSession')"
          @click="emit('newSession')"
        >
          <Plus class="size-3.5" />
        </button>
        <button
          v-if="showClose"
          type="button"
          class="button button-secondary button-icon file-workbench-small-button"
          :aria-label="t('common.close')"
          :title="t('common.close')"
          @click="emit('close')"
        >
          <X class="size-3.5" />
        </button>
      </div>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto p-1.5">
      <PageStatus
        class="min-h-full"
        :loading="loading && workspaceTree.length === 0 && !loadError"
        :error="loadError || null"
        :empty="!loading && !loadError && workspaceTree.length === 0"
        :loading-text="t('dashboard.loadingWorkspaces')"
        :empty-text="t('sidebar.emptyWorkspaces')"
      >
        <template #loading>
          <div
            class="rounded-md border border-dashed border-[var(--color-border)] bg-[var(--color-surface-muted)] p-2 text-sm text-[var(--color-text-muted)]"
            role="status"
          >
            {{ t('dashboard.loadingWorkspaces') }}
          </div>
        </template>
        <template #error>
          <div
            class="rounded-md border border-dashed border-[var(--color-border)] bg-[var(--color-surface-muted)] p-2 text-sm text-[var(--color-danger-text)]"
            role="alert"
          >
            {{ loadError }}
          </div>
        </template>
        <template #empty>
          <div
            class="rounded-md border border-dashed border-[var(--color-border)] bg-[var(--color-surface-muted)] p-2 text-sm text-[var(--color-text-muted)]"
          >
            {{ t('sidebar.emptyWorkspaces') }}
          </div>
        </template>

        <div
          v-if="treeItems.length === 0"
          class="rounded-md border border-dashed border-[var(--color-border)] bg-[var(--color-surface-muted)] p-2 text-sm text-[var(--color-text-muted)]"
        >
          {{
            normalizedSearchQuery
              ? t('sidebar.noSessionsMatch', { query: searchQuery })
              : t('sidebar.noActiveSessions')
          }}
        </div>

        <VueDraggable
          v-else
          :model-value="treeItems"
          tag="div"
          class="flex flex-col"
          item-key="value"
          handle=".workspace-drag-handle"
          draggable=".workspace-sortable-item"
          :filter="workspaceSortableFilter"
          :prevent-on-filter="false"
          ghost-class="workspace-sortable-ghost"
          chosen-class="workspace-sortable-chosen"
          drag-class="workspace-sortable-dragging"
          :animation="150"
          :disabled="Boolean(normalizedSearchQuery) || disableReorder"
          @update:model-value="updateWorkspaceOrder"
        >
          <div
            v-for="workspace in treeItems"
            :key="workspace.value"
            class="workspace-sortable-item min-w-0"
          >
            <div
              role="button"
              tabindex="0"
              class="workspace-drag-handle group flex min-h-10 w-full min-w-0 items-center gap-1 border-0 bg-transparent px-1 py-1 text-left text-[var(--color-text)] hover:bg-[var(--color-control-hover)] sm:min-h-7 sm:py-0.5"
              :class="!normalizedSearchQuery ? 'cursor-pointer' : ''"
              :style="{ paddingLeft: '6px' }"
              @click="handleWorkspaceClick($event, workspace.value)"
              @keydown.enter="handleWorkspaceKeydown($event, workspace.value)"
              @keydown.space="handleWorkspaceKeydown($event, workspace.value)"
            >
              <FolderOpen
                v-if="workspaceExpanded(workspace.value)"
                class="size-4 shrink-0 text-[var(--color-text-subtle)]"
                aria-hidden="true"
              />
              <Folder
                v-else
                class="size-4 shrink-0 text-[var(--color-text-subtle)]"
                aria-hidden="true"
              />
              <span class="min-w-0 flex-1 truncate text-sm font-semibold">{{
                workspace.workspace.name
              }}</span>
              <span class="workspace-tree-node-actions flex shrink-0 items-center gap-0.5">
                <button
                  type="button"
                  class="workspace-tree-node-action"
                  :aria-label="t('sidebar.openFilesAria', { name: workspace.workspace.name })"
                  :title="t('sidebar.openFilesAria', { name: workspace.workspace.name })"
                  @click.stop="emit('openFiles', workspace.workspace, $event.currentTarget)"
                >
                  <FileCode2 class="size-3.5" />
                </button>
                <button
                  type="button"
                  class="workspace-tree-node-action"
                  :aria-label="
                    t('sidebar.newSessionInWorkspaceAria', { name: workspace.workspace.name })
                  "
                  :title="
                    t('sidebar.newSessionInWorkspaceAria', { name: workspace.workspace.name })
                  "
                  @click.stop="emit('newSession', workspace.workspace)"
                >
                  <Plus class="size-3.5" />
                </button>
                <button
                  type="button"
                  :disabled="
                    !canRemoveWorkspace(workspace) ||
                    props.removingWorkspaceId === workspace.workspace.id
                  "
                  class="workspace-tree-node-action workspace-tree-node-action-danger"
                  :aria-label="removeWorkspaceLabel(workspace)"
                  :title="removeWorkspaceLabel(workspace)"
                  @click.stop="emit('removeWorkspace', workspace.workspace)"
                >
                  <Loader2
                    v-if="props.removingWorkspaceId === workspace.workspace.id"
                    class="size-3.5 animate-spin"
                  />
                  <Trash2 v-else class="size-3.5" />
                </button>
              </span>
            </div>

            <VueDraggable
              v-if="workspaceExpanded(workspace.value)"
              :model-value="workspace.children"
              tag="div"
              class="session-sortable-list flex flex-col"
              item-key="value"
              draggable=".session-sortable-item"
              :group="sessionSortableGroup(workspace.workspace.id)"
              :filter="sessionSortableFilter"
              :prevent-on-filter="false"
              ghost-class="session-sortable-ghost"
              chosen-class="session-sortable-chosen"
              drag-class="session-sortable-dragging"
              :animation="150"
              :disabled="Boolean(normalizedSearchQuery) || disableReorder"
              @start="startSessionDrag"
              @update:model-value="updateSessionOrder(workspace, $event)"
              @end="finishSessionDrag"
            >
              <div
                v-for="session in workspace.children"
                :key="session.value"
                role="button"
                tabindex="0"
                class="session-sortable-item group flex min-h-10 w-full min-w-0 items-center gap-1 border-0 px-1 py-1 text-left transition sm:min-h-7 sm:py-0.5"
                :class="[
                  isActiveSessionSelection(session.session.id)
                    ? 'bg-[var(--color-control-active)] text-[var(--color-text-strong)]'
                    : 'bg-transparent text-[var(--color-text-muted)] hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)]',
                  'cursor-pointer',
                ]"
                :style="{ paddingLeft: '24px' }"
                @click="handleSessionClick($event, session.session)"
                @keydown.enter="handleSessionKeydown($event, session.session)"
                @keydown.space="handleSessionKeydown($event, session.session)"
              >
                <SessionSourceIcon
                  :command-source="session.session.command_source"
                  :label="sessionSourceLabel(session.session)"
                />
                <span class="min-w-0 flex-1 truncate text-sm">{{
                  session.session.name || session.session.command
                }}</span>
                <span
                  v-if="session.session.lifecycle_state === 'running'"
                  class="flex size-5 shrink-0 items-center justify-center"
                >
                  <button
                    type="button"
                    class="workspace-tree-node-action"
                    :aria-label="
                      t('sidebar.copySessionAria', {
                        name: session.session.name || session.session.command,
                      })
                    "
                    :title="
                      t('sidebar.copySessionAria', {
                        name: session.session.name || session.session.command,
                      })
                    "
                    @click.stop="emit('copySession', session.session)"
                  >
                    <Copy class="size-3.5" />
                  </button>
                </span>
                <span
                  v-if="session.session.lifecycle_state === 'running'"
                  class="flex size-5 shrink-0 items-center justify-center"
                >
                  <CircleDot
                    class="size-3.5"
                    :class="lifecycleStateClassName(session.session.lifecycle_state)"
                    aria-hidden="true"
                  />
                </span>
                <span
                  v-if="isEditableSession(session.session)"
                  class="workspace-tree-node-actions flex shrink-0 items-center gap-0.5"
                >
                  <button
                    type="button"
                    class="workspace-tree-node-action"
                    :aria-label="
                      t('sidebar.editSessionAria', {
                        name: session.session.name || session.session.command,
                      })
                    "
                    :title="
                      t('sidebar.editSessionAria', {
                        name: session.session.name || session.session.command,
                      })
                    "
                    @click.stop="emit('editSession', session.session)"
                  >
                    <Pencil class="size-3.5" />
                  </button>
                </span>
                <button
                  v-if="['stopped', 'failed'].includes(session.session.lifecycle_state)"
                  type="button"
                  :disabled="props.rerunningSessionId === session.session.id"
                  class="workspace-tree-node-action"
                  :aria-label="
                    t('sidebar.rerunSessionAria', {
                      name: session.session.name || session.session.command,
                    })
                  "
                  :title="
                    t('sidebar.rerunSessionAria', {
                      name: session.session.name || session.session.command,
                    })
                  "
                  @click.stop="emit('rerunSession', session.session)"
                >
                  <Loader2
                    v-if="props.rerunningSessionId === session.session.id"
                    class="size-3.5 animate-spin"
                  />
                  <RotateCcw v-else class="size-3.5" />
                </button>
                <button
                  v-if="session.session.lifecycle_state === 'running'"
                  type="button"
                  :disabled="props.stoppingSessionId === session.session.id"
                  class="workspace-tree-node-action workspace-tree-node-action-danger"
                  :aria-label="
                    t('sidebar.stopSessionAria', {
                      name: session.session.name || session.session.command,
                    })
                  "
                  :title="
                    t('sidebar.stopSessionAria', {
                      name: session.session.name || session.session.command,
                    })
                  "
                  @click.stop="emit('stopSession', session.session)"
                >
                  <Loader2
                    v-if="props.stoppingSessionId === session.session.id"
                    class="size-3.5 animate-spin"
                  />
                  <CircleStop v-else class="size-3.5" />
                </button>
                <button
                  v-if="['stopped', 'failed'].includes(session.session.lifecycle_state)"
                  type="button"
                  :disabled="props.deletingSessionId === session.session.id"
                  class="workspace-tree-node-action workspace-tree-node-action-danger"
                  :aria-label="
                    t('sidebar.deleteSessionAria', {
                      name: session.session.name || session.session.command,
                    })
                  "
                  :title="
                    t('sidebar.deleteSessionAria', {
                      name: session.session.name || session.session.command,
                    })
                  "
                  @click.stop="emit('deleteSession', session.session)"
                >
                  <Loader2
                    v-if="props.deletingSessionId === session.session.id"
                    class="size-3.5 animate-spin"
                  />
                  <Trash2 v-else class="size-3.5" />
                </button>
              </div>
            </VueDraggable>
          </div>
        </VueDraggable>
      </PageStatus>
    </div>

    <footer
      class="flex h-8 shrink-0 items-center gap-2 border-t border-[var(--color-border)] bg-[var(--color-panel-header)] px-2 text-sm text-[var(--color-text-muted)]"
    >
      <div class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden">
        <DropdownMenuRoot :modal="false">
          <DropdownMenuTrigger
            class="flex h-6 items-center gap-1.5 rounded px-1.5 text-[var(--color-text-muted)] outline-none hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus:bg-[var(--color-control-hover)] focus:text-[var(--color-text)]"
            :aria-label="t('common.settings')"
            :title="t('common.settings')"
          >
            <Settings class="size-4" />
            <span>{{ t('common.settings') }}</span>
          </DropdownMenuTrigger>
          <DropdownMenuPortal>
            <DropdownMenuContent
              side="top"
              align="start"
              :side-offset="8"
              class="z-50 min-w-44 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] p-1 text-sm text-[var(--color-text)] shadow-xl"
            >
              <DropdownMenuSub>
                <DropdownMenuSubTrigger
                  class="flex cursor-pointer items-center justify-between rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
                >
                  <span class="inline-flex items-center gap-2">
                    <Languages class="size-4 text-[var(--color-text-subtle)]" />
                    {{ t('common.language') }}
                  </span>
                  <ChevronRight class="size-4 text-[var(--color-text-subtle)]" />
                </DropdownMenuSubTrigger>
                <DropdownMenuPortal>
                  <DropdownMenuSubContent
                    :side-offset="8"
                    class="z-50 min-w-36 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] p-1 text-sm text-[var(--color-text)] shadow-xl"
                  >
                    <DropdownMenuRadioGroup
                      :model-value="locale"
                      @update:model-value="changeLocale"
                    >
                      <DropdownMenuRadioItem
                        v-for="item in localeOptions"
                        :key="item.value"
                        :value="item.value"
                        class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
                      >
                        <Check
                          :class="locale === item.value ? 'opacity-100' : 'opacity-0'"
                          class="size-4 text-blue-500"
                        />
                        {{ item.label }}
                      </DropdownMenuRadioItem>
                    </DropdownMenuRadioGroup>
                  </DropdownMenuSubContent>
                </DropdownMenuPortal>
              </DropdownMenuSub>
              <DropdownMenuSub>
                <DropdownMenuSubTrigger
                  class="flex cursor-pointer items-center justify-between rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
                >
                  <span class="inline-flex items-center gap-2">
                    <Sun class="size-4 text-[var(--color-text-subtle)]" />
                    {{ t('common.theme') }}
                  </span>
                  <ChevronRight class="size-4 text-[var(--color-text-subtle)]" />
                </DropdownMenuSubTrigger>
                <DropdownMenuPortal>
                  <DropdownMenuSubContent
                    :side-offset="8"
                    class="z-50 min-w-36 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] p-1 text-sm text-[var(--color-text)] shadow-xl"
                  >
                    <DropdownMenuRadioGroup
                      :model-value="themeStore.theme"
                      @update:model-value="changeTheme"
                    >
                      <DropdownMenuRadioItem
                        v-for="item in themeOptions"
                        :key="item.value"
                        :value="item.value"
                        class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
                      >
                        <Check
                          :class="themeStore.theme === item.value ? 'opacity-100' : 'opacity-0'"
                          class="size-4 text-blue-500"
                        />
                        {{ item.label }}
                      </DropdownMenuRadioItem>
                    </DropdownMenuRadioGroup>
                  </DropdownMenuSubContent>
                </DropdownMenuPortal>
              </DropdownMenuSub>
              <DropdownMenuItem
                class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
                @select="emit('logout')"
              >
                <LogOut class="size-4 text-[var(--color-text-subtle)]" />
                {{ t('cloud.logout') }}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenuPortal>
        </DropdownMenuRoot>
      </div>

      <button
        type="button"
        class="flex size-6 shrink-0 items-center justify-center rounded text-[var(--color-text-muted)] outline-none hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus-visible:bg-[var(--color-control-hover)] focus-visible:text-[var(--color-text)] disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-[var(--color-text-muted)]"
        :disabled="!props.activeWorkspace"
        :aria-label="switchToFilesAria"
        :title="switchToFilesAria"
        @click="switchToFiles"
      >
        <FileCode2 class="size-4" />
      </button>
    </footer>
  </aside>
</template>

<script setup lang="ts">
  import { computed, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { RouterLink } from 'vue-router'
  import {
    Check,
    ChevronRight,
    CircleDot,
    CircleStop,
    Copy,
    FileCode2,
    Folder,
    FolderOpen,
    Languages,
    Loader2,
    LogOut,
    Pencil,
    Plus,
    RotateCcw,
    Search,
    Settings,
    Sun,
    Trash2,
    X,
  } from '@lucide/vue'
  import {
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuPortal,
    DropdownMenuRadioGroup,
    DropdownMenuRadioItem,
    DropdownMenuRoot,
    DropdownMenuSub,
    DropdownMenuSubContent,
    DropdownMenuSubTrigger,
    DropdownMenuTrigger,
  } from 'reka-ui'
  import { VueDraggable } from 'vue-draggable-plus'
  import type { SortableEvent } from 'sortablejs'
  import { lifecycleStateClassName } from '../../features/sessions/lifecycleState'
  import { localeLabels, locales, setLocale, type AppLocale } from '../../i18n'
  import SessionSourceIcon from '../session/SessionSourceIcon.vue'
  import PageStatus from '../layout/PageStatus.vue'
  import { themes, useThemeStore, type AppTheme } from '../../store/theme'
  import type {
    SessionSummary,
    Workspace as WorkspaceSummary,
    WorkspaceTreeNode as WorkspaceTreeSummary,
  } from '../../gen/proto/termbridge/agent/v1/workspace'

  type WorkspaceTreeItem = {
    kind: 'workspace'
    value: string
    workspace: WorkspaceSummary
    children: SessionTreeItem[]
  }

  type SessionTreeItem = {
    kind: 'session'
    value: string
    session: SessionSummary
  }

  type SidebarPointerEvent = {
    target: unknown
  }

  type SidebarKeyboardEvent = SidebarPointerEvent & {
    preventDefault: () => void
  }

  type InteractiveEventTarget = {
    closest: (selectors: string) => unknown
  }

  const props = defineProps<{
    workspaceTree: WorkspaceTreeSummary[]
    activeSessionId: string | null
    activeWorkspace: WorkspaceSummary | null
    stoppingSessionId: string | null
    rerunningSessionId: string | null
    deletingSessionId: string | null
    removingWorkspaceId: string | null
    loading?: boolean
    loadError?: string | null
    homeRouteName: string
    disableReorder?: boolean
    showClose?: boolean
  }>()

  const emit = defineEmits<{
    close: []
    select: [session: SessionSummary]
    refresh: []
    newSession: [workspace?: WorkspaceSummary]
    copySession: [session: SessionSummary]
    editSession: [session: SessionSummary]
    stopSession: [session: SessionSummary]
    rerunSession: [session: SessionSummary]
    deleteSession: [session: SessionSummary]
    removeWorkspace: [workspace: WorkspaceSummary]
    openFiles: [workspace: WorkspaceSummary, trigger: EventTarget | null]
    unsupportedDirectoryDelete: [workspace: WorkspaceSummary]
    reorderWorkspaces: [workspaceIds: string[]]
    reorderSessions: [workspaceId: string, sessionIds: string[]]
    logout: []
  }>()

  const { t, locale } = useI18n()
  const themeStore = useThemeStore()
  const searchQuery = ref('')
  const workspaceExpansionOverrides = ref<Record<string, boolean>>({})
  const suppressNextSessionClick = ref(false)
  let sessionClickSuppressionTimer: ReturnType<typeof window.setTimeout> | null = null

  const interactiveSortableFilter =
    "button, a, input, textarea, select, [contenteditable='true'], [role='menuitem']"
  const workspaceSortableFilter = `${interactiveSortableFilter}, .session-sortable-list, .session-sortable-list *`
  const sessionSortableFilter = interactiveSortableFilter

  const normalizedSearchQuery = computed(() => searchQuery.value.trim().toLowerCase())
  const localeOptions = computed(() =>
    locales.map((value) => ({ value, label: localeLabels[value] })),
  )
  const themeOptions = computed(() =>
    themes.map((value) => ({
      value,
      label: t(`theme.${value}`),
    })),
  )
  const switchToFilesAria = computed(() => {
    if (!props.activeWorkspace) return t('sidebar.switchToFilesDisabled')
    return t('sidebar.switchToFilesAria', { name: props.activeWorkspace.name })
  })
  function switchToFiles() {
    if (!props.activeWorkspace) return
    emit('openFiles', props.activeWorkspace, null)
  }

  function changeLocale(value: unknown) {
    if (typeof value === 'string' && locales.includes(value as AppLocale)) {
      setLocale(value as AppLocale)
    }
  }

  function changeTheme(value: unknown) {
    if (typeof value === 'string' && themes.includes(value as AppTheme)) {
      themeStore.setTheme(value as AppTheme)
    }
  }

  const treeItems = computed<WorkspaceTreeItem[]>(() => {
    return props.workspaceTree
      .map((workspace) => {
        const children = sessionsFor(workspace).map((session) => ({
          kind: 'session' as const,
          value: `session:${session.id}`,
          session,
        }))
        return {
          kind: 'workspace' as const,
          value: `workspace:${workspace.id}`,
          workspace,
          children,
        }
      })
      .filter(
        (item) =>
          !normalizedSearchQuery.value ||
          item.children.length > 0 ||
          workspaceMatchesSearch(item.workspace),
      )
  })

  function updateWorkspaceOrder(items: WorkspaceTreeItem[]) {
    if (normalizedSearchQuery.value || itemsHaveSameValues(items, treeItems.value)) {
      return
    }
    emit(
      'reorderWorkspaces',
      items.map((item) => item.workspace.id),
    )
  }

  function startSessionDrag() {
    clearSessionClickSuppression()
  }

  function updateSessionOrder(workspace: WorkspaceTreeItem, items: SessionTreeItem[]) {
    if (normalizedSearchQuery.value || itemsHaveSameValues(items, workspace.children)) {
      return
    }
    emit(
      'reorderSessions',
      workspace.workspace.id,
      items.map((item) => item.session.id),
    )
  }

  function finishSessionDrag(event: SortableEvent) {
    if (dragPositionChanged(event)) {
      suppressSessionClick()
    }
  }

  function itemsHaveSameValues<T extends { value: string }>(left: T[], right: T[]) {
    return (
      left.length === right.length &&
      left.every((item, index) => item.value === right[index]?.value)
    )
  }

  function dragPositionChanged(event: SortableEvent) {
    return (
      typeof event.oldIndex === 'number' &&
      typeof event.newIndex === 'number' &&
      event.oldIndex !== event.newIndex
    )
  }

  function suppressSessionClick() {
    suppressNextSessionClick.value = true
    if (sessionClickSuppressionTimer) {
      window.clearTimeout(sessionClickSuppressionTimer)
    }
    sessionClickSuppressionTimer = window.setTimeout(clearSessionClickSuppression, 0)
  }

  function clearSessionClickSuppression() {
    suppressNextSessionClick.value = false
    if (sessionClickSuppressionTimer) {
      window.clearTimeout(sessionClickSuppressionTimer)
      sessionClickSuppressionTimer = null
    }
  }

  function sessionSortableGroup(workspaceId: string) {
    return {
      name: `workspace-session-order:${workspaceId}`,
      pull: false,
      put: false,
    }
  }

  function handleWorkspaceClick(event: SidebarPointerEvent, workspaceValue: string) {
    if (eventTargetsInteractiveControl(event)) {
      return
    }
    toggleWorkspace(workspaceValue)
  }

  function handleWorkspaceKeydown(event: SidebarKeyboardEvent, workspaceValue: string) {
    if (eventTargetsInteractiveControl(event)) {
      return
    }
    event.preventDefault()
    toggleWorkspace(workspaceValue)
  }

  function toggleWorkspace(workspaceValue: string) {
    workspaceExpansionOverrides.value = {
      ...workspaceExpansionOverrides.value,
      [workspaceValue]: !workspaceExpanded(workspaceValue),
    }
  }

  function workspaceExpanded(workspaceValue: string) {
    const override = workspaceExpansionOverrides.value[workspaceValue]
    if (override !== undefined) {
      return override
    }
    return props.workspaceTree.some(
      (workspace) =>
        `workspace:${workspace.id}` === workspaceValue &&
        workspace.children.some((session) => isActiveSession(session)),
    )
  }

  function handleSessionClick(event: SidebarPointerEvent, session: SessionSummary) {
    if (suppressNextSessionClick.value) {
      suppressNextSessionClick.value = false
      return
    }
    if (eventTargetsInteractiveControl(event)) {
      return
    }
    selectSession(session)
  }

  function handleSessionKeydown(event: SidebarKeyboardEvent, session: SessionSummary) {
    if (eventTargetsInteractiveControl(event)) {
      return
    }
    event.preventDefault()
    selectSession(session)
  }

  function eventTargetsInteractiveControl(event: SidebarPointerEvent) {
    return (
      typeof event.target === 'object' &&
      event.target !== null &&
      'closest' in event.target &&
      typeof event.target.closest === 'function' &&
      (event.target as InteractiveEventTarget).closest(interactiveSortableFilter) !== null
    )
  }

  function selectSession(session: SessionSummary) {
    emit('select', session)
  }

  function isActiveSessionSelection(sessionId: string) {
    return props.activeSessionId === sessionId
  }

  function isActiveSession(session: SessionSummary) {
    return session.lifecycle_state === 'running'
  }

  function isEditableSession(session: SessionSummary) {
    return ['stopped', 'failed'].includes(session.lifecycle_state)
  }

  function sessionSourceLabel(session: SessionSummary) {
    return session.command_source === 'shortcut'
      ? t('dialog.shortcut')
      : t('workbench.launchCommand')
  }

  function canRemoveWorkspace(workspace: WorkspaceTreeItem) {
    return workspace.children.every((child) => !isActiveSession(child.session))
  }

  function removeWorkspaceLabel(workspace: WorkspaceTreeItem) {
    if (canRemoveWorkspace(workspace)) {
      return t('sidebar.removeWorkspaceAria', { name: workspace.workspace.name })
    }
    return t('sidebar.removeWorkspaceDisabledAria', { name: workspace.workspace.name })
  }

  function sessionsFor(workspace: WorkspaceTreeSummary): SessionSummary[] {
    return workspace.children
      .map((session) => ({
        ...session,
        workspace_id: session.workspace_id ?? workspace.id,
      }))
      .filter((session) => !normalizedSearchQuery.value || sessionMatchesSearch(session, workspace))
  }

  function workspaceMatchesSearch(workspace: WorkspaceSummary) {
    return workspace.name.toLowerCase().includes(normalizedSearchQuery.value)
  }

  function sessionMatchesSearch(session: SessionSummary, workspace: WorkspaceSummary) {
    return (
      session.name.toLowerCase().includes(normalizedSearchQuery.value) ||
      workspace.name.toLowerCase().includes(normalizedSearchQuery.value)
    )
  }
</script>

<style scoped>
  .workspace-sortable-ghost > .workspace-drag-handle,
  .session-sortable-ghost {
    background: var(--color-surface-muted) !important;
    color: var(--color-text-subtle) !important;
    opacity: 0.72;
  }

  .workspace-sortable-chosen > .workspace-drag-handle,
  .session-sortable-chosen {
    background: var(--color-control-hover) !important;
    cursor: pointer;
  }

  .workspace-sortable-dragging > .workspace-drag-handle,
  .session-sortable-dragging {
    background: var(--color-control-active) !important;
    box-shadow: 0 4px 12px color-mix(in srgb, var(--color-text) 12%, transparent);
    cursor: pointer;
    opacity: 0.96;
  }
</style>
