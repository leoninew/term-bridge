<template>
  <aside
    class="flex h-full min-w-0 flex-col overflow-hidden bg-[var(--color-sidebar-bg)] text-[var(--color-text)]"
  >
    <WorkspaceSidebarHeader
      v-model:search-query="searchQuery"
      :home-route-name="homeRouteName"
      @new-session="emit('newSession')"
      @collapse="emit('collapse')"
    />

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
          <div :class="sidebarStatusCardClass" role="status">
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
          <div :class="sidebarStatusCardClass">{{ t('sidebar.emptyWorkspaces') }}</div>
        </template>

        <div v-if="treeItems.length === 0" :class="sidebarStatusCardClass">
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
              class="workspace-drag-handle group flex min-h-10 w-full min-w-0 items-center gap-1 border-0 bg-transparent px-1 py-1 text-left sm:min-h-8 sm:py-0.5"
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
              <span :class="treeNodeActionsClass">
                <button
                  type="button"
                  :class="treeNodeActionClass"
                  :aria-label="t('sidebar.openCodeAria')"
                  :title="t('sidebar.openCodeAria')"
                  @click.stop="emit('openFiles', workspace.workspace, $event.currentTarget)"
                >
                  <VscodeCodicon size-class="size-3.5" />
                </button>
                <button
                  type="button"
                  :class="treeNodeActionClass"
                  :aria-label="t('sidebar.newSessionInWorkspaceAria')"
                  :title="t('sidebar.newSessionInWorkspaceAria')"
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
                  :class="treeNodeActionClass"
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
              <WorkspaceTreeSessionRow
                v-for="session in workspace.children"
                :key="session.value"
                :session="session.session"
                :active="isActiveSessionSelection(session.session.id)"
                :stopping="props.stoppingSessionId === session.session.id"
                :rerunning="props.rerunningSessionId === session.session.id"
                :deleting="props.deletingSessionId === session.session.id"
                @activate="handleSessionClick($event, session.session)"
                @activate-key="handleSessionKeydown($event, session.session)"
                @copy="emit('copySession', session.session)"
                @edit="emit('editSession', session.session)"
                @stop="emit('stopSession', session.session)"
                @rerun="emit('rerunSession', session.session)"
                @delete="emit('deleteSession', session.session)"
              />
            </VueDraggable>
          </div>
        </VueDraggable>
      </PageStatus>
    </div>

    <WorkspaceSidebarFooter
      :active-workspace="activeWorkspace"
      :switch-to-files-aria="switchToFilesAria"
      @switch-to-files="switchToFiles"
    />
  </aside>
</template>

<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { Folder, FolderOpen, Loader2, Plus, Trash2 } from '@lucide/vue'
  import VscodeCodicon from '../branding/VscodeCodicon.vue'
  import { VueDraggable } from 'vue-draggable-plus'
  import type { SortableEvent } from 'sortablejs'
  import PageStatus from '../layout/PageStatus.vue'
  import type {
    SessionSummary,
    Workspace as WorkspaceSummary,
    WorkspaceTreeNode as WorkspaceTreeSummary,
  } from '../../gen/proto/termbridge/agent/v1/workspace'
  import {
    sidebarStatusCardClass,
    treeNodeActionClass,
    treeNodeActionsClass,
  } from '../session/sessionUi'
  import WorkspaceSidebarFooter from './WorkspaceSidebarFooter.vue'
  import WorkspaceSidebarHeader from './WorkspaceSidebarHeader.vue'
  import WorkspaceTreeSessionRow from './WorkspaceTreeSessionRow.vue'

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
  }>()

  const emit = defineEmits<{
    collapse: []
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
  }>()

  const { t } = useI18n()
  const searchQuery = ref('')
  const workspaceExpansionState = ref<Record<string, boolean>>({})
  const suppressNextSessionClick = ref(false)
  let sessionClickSuppressionTimer: ReturnType<typeof window.setTimeout> | null = null

  const interactiveSortableFilter =
    "button, a, input, textarea, select, [contenteditable='true'], [role='menuitem']"
  const workspaceSortableFilter = `${interactiveSortableFilter}, .session-sortable-list, .session-sortable-list *`
  const sessionSortableFilter = interactiveSortableFilter

  const normalizedSearchQuery = computed(() => searchQuery.value.trim().toLowerCase())
  const switchToFilesAria = computed(() => {
    if (!props.activeWorkspace) return t('sidebar.openCodeDisabled')
    return t('sidebar.openCodeAria')
  })
  function switchToFiles() {
    if (!props.activeWorkspace) return
    emit('openFiles', props.activeWorkspace, null)
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

  function workspaceKey(workspaceId: string) {
    return `workspace:${workspaceId}`
  }

  function hasRunningSession(workspace: WorkspaceTreeSummary) {
    return workspace.children.some((session) => isActiveSession(session))
  }

  watch(
    () => props.workspaceTree,
    (tree) => {
      const next = { ...workspaceExpansionState.value }
      let changed = false
      for (const workspace of tree) {
        const key = workspaceKey(workspace.id)
        if (next[key] === undefined) {
          next[key] = hasRunningSession(workspace)
          changed = true
        }
      }
      if (changed) {
        workspaceExpansionState.value = next
      }
    },
    { immediate: true },
  )

  function toggleWorkspace(workspaceValue: string) {
    workspaceExpansionState.value = {
      ...workspaceExpansionState.value,
      [workspaceValue]: !workspaceExpanded(workspaceValue),
    }
  }

  function workspaceExpanded(workspaceValue: string) {
    return workspaceExpansionState.value[workspaceValue] ?? false
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

  function canRemoveWorkspace(workspace: WorkspaceTreeItem) {
    return workspace.children.every((child) => !isActiveSession(child.session))
  }

  function removeWorkspaceLabel(workspace: WorkspaceTreeItem) {
    if (canRemoveWorkspace(workspace)) {
      return t('sidebar.removeWorkspaceAria')
    }
    return t('sidebar.removeWorkspaceDisabledAria')
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
