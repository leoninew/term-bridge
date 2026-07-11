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
            class="h-8 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] py-1.5 pl-8 pr-2 text-sm text-[var(--color-text)] outline-none placeholder:text-[var(--color-text-subtle)]"
          />
        </label>
        <button
          type="button"
          class="flex size-8 shrink-0 items-center justify-center rounded-md border border-[var(--color-border-strong)] bg-[var(--color-control-active)] text-[var(--color-text)] hover:bg-[var(--color-control-hover)]"
          :aria-label="t('sidebar.newSession')"
          :title="t('sidebar.newSession')"
          @click="emit('newSession')"
        >
          <Plus class="size-4" />
        </button>
      </div>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto p-1.5">
      <div
        v-if="workspaceTree.length === 0"
        class="rounded-md border border-dashed border-[var(--color-border)] bg-[var(--color-surface-muted)] p-2 text-sm text-[var(--color-text-muted)]"
      >
        {{ t('sidebar.emptyWorkspaces') }}
      </div>
      <div
        v-else-if="treeItems.length === 0"
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
        v-model="draggableTreeItems"
        tag="div"
        class="flex flex-col"
        item-key="value"
        handle=".workspace-drag-handle"
        :animation="150"
        :disabled="Boolean(normalizedSearchQuery)"
        @end="reorderDraggedWorkspaces"
      >
        <div v-for="workspace in draggableTreeItems" :key="workspace.value" class="min-w-0">
          <div
            role="button"
            tabindex="0"
            class="workspace-drag-handle group flex h-7 w-full min-w-0 items-center gap-1 rounded-md border border-transparent px-1 py-0.5 text-left text-[var(--color-text)] hover:border-[var(--color-border)] hover:bg-[var(--color-control-hover)]"
            :style="{ paddingLeft: '6px' }"
            @click="toggleWorkspace(workspace.value)"
            @keydown.enter.prevent="toggleWorkspace(workspace.value)"
            @keydown.space.prevent="toggleWorkspace(workspace.value)"
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
            <span
              class="flex shrink-0 items-center gap-0.5 opacity-0 transition group-hover:opacity-100 focus-within:opacity-100"
            >
              <button
                type="button"
                class="flex size-5 shrink-0 items-center justify-center rounded text-[var(--color-text-subtle)] hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text-strong)]"
                :aria-label="
                  t('sidebar.newSessionInWorkspaceAria', { name: workspace.workspace.name })
                "
                :title="t('sidebar.newSessionInWorkspaceAria', { name: workspace.workspace.name })"
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
                class="flex size-5 shrink-0 items-center justify-center rounded text-[var(--color-text-subtle)] hover:bg-[var(--color-control-hover)] hover:text-red-500"
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
            v-model="workspace.children"
            tag="div"
            class="flex flex-col"
            item-key="value"
            :animation="150"
            :disabled="Boolean(normalizedSearchQuery)"
            @end="reorderDraggedSessions(workspace)"
          >
            <div
              v-for="session in workspace.children"
              :key="session.value"
              role="button"
              tabindex="0"
              class="group flex h-7 w-full min-w-0 cursor-pointer items-center gap-1 rounded-md border px-1 py-0.5 text-left transition"
              :class="
                isActiveSessionSelection(session.session.id)
                  ? 'border-[var(--color-border-strong)] bg-[var(--color-control-active)] text-[var(--color-text-strong)]'
                  : 'border-transparent text-[var(--color-text-muted)] hover:border-[var(--color-border)] hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)]'
              "
              :style="{ paddingLeft: '38px' }"
              @click="selectSession(session.session)"
              @keydown.enter.prevent="selectSession(session.session)"
              @keydown.space.prevent="selectSession(session.session)"
            >
              <SquareTerminal
                class="size-4 shrink-0 text-[var(--color-text-subtle)]"
                aria-hidden="true"
              />
              <span class="min-w-0 flex-1 truncate text-sm">{{
                session.session.name || session.session.command
              }}</span>
              <span
                v-if="isEditableSession(session.session)"
                class="flex shrink-0 items-center gap-0.5 opacity-0 transition group-hover:opacity-100 focus-within:opacity-100"
              >
                <button
                  type="button"
                  class="flex size-5 items-center justify-center rounded text-[var(--color-text-subtle)] hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text-strong)]"
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
                v-if="session.session.lifecycle_state === 'running'"
                type="button"
                :disabled="props.stoppingSessionId === session.session.id"
                class="flex size-5 items-center justify-center rounded text-[var(--color-text-subtle)] hover:bg-[var(--color-control-hover)] hover:text-red-500"
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
                :disabled="props.rerunningSessionId === session.session.id"
                class="flex size-5 items-center justify-center rounded text-[var(--color-text-subtle)] hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text-strong)]"
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
                v-if="['stopped', 'failed'].includes(session.session.lifecycle_state)"
                type="button"
                :disabled="props.deletingSessionId === session.session.id"
                class="flex size-5 items-center justify-center rounded text-[var(--color-text-subtle)] hover:bg-[var(--color-control-hover)] hover:text-red-500"
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
    </div>

    <footer
      class="flex h-8 shrink-0 items-center gap-2 border-t border-[var(--color-border)] bg-[var(--color-panel-header)] px-2 text-sm text-[var(--color-text-muted)]"
    >
      <DropdownMenuRoot>
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
            <DropdownMenuItem
              class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
              @select="emit('openDashboard')"
            >
              <LayoutDashboard class="size-4 text-[var(--color-text-subtle)]" />
              {{ t('dashboard.home') }}
            </DropdownMenuItem>
            <DropdownMenuItem
              class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
              @select="emit('openShortcuts')"
            >
              <Command class="size-4 text-[var(--color-text-subtle)]" />
              {{ t('shortcut.title') }}
            </DropdownMenuItem>
            <DropdownMenuItem v-if="props.helpHref" as-child>
              <a
                :href="props.helpHref"
                class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
              >
                <CircleHelp class="size-4 text-[var(--color-text-subtle)]" />
                {{ t('common.help') }}
              </a>
            </DropdownMenuItem>
            <DropdownMenuItem v-else as-child>
              <RouterLink
                :to="{ name: 'cloud-help' }"
                class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]"
              >
                <CircleHelp class="size-4 text-[var(--color-text-subtle)]" />
                {{ t('common.help') }}
              </RouterLink>
            </DropdownMenuItem>
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
                  <DropdownMenuRadioGroup :model-value="locale" @update:model-value="changeLocale">
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
    </footer>
  </aside>
</template>

<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { RouterLink } from 'vue-router'
  import {
    Check,
    ChevronRight,
    CircleHelp,
    Command,
    CircleStop,
    Folder,
    FolderOpen,
    Languages,
    LayoutDashboard,
    Loader2,
    LogOut,
    Pencil,
    Plus,
    RotateCcw,
    Search,
    Settings,
    SquareTerminal,
    Sun,
    Trash2,
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
  import { localeLabels, locales, setLocale, type AppLocale } from '../../i18n'
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

  const props = defineProps<{
    workspaceTree: WorkspaceTreeSummary[]
    activeSessionId: string | null
    stoppingSessionId: string | null
    rerunningSessionId: string | null
    deletingSessionId: string | null
    removingWorkspaceId: string | null
    helpHref?: string
    homeRouteName: string
  }>()

  const emit = defineEmits<{
    select: [session: SessionSummary]
    refresh: []
    newSession: [workspace?: WorkspaceSummary]
    editSession: [session: SessionSummary]
    stopSession: [session: SessionSummary]
    rerunSession: [session: SessionSummary]
    deleteSession: [session: SessionSummary]
    removeWorkspace: [workspace: WorkspaceSummary]
    unsupportedDirectoryDelete: [workspace: WorkspaceSummary]
    reorderWorkspaces: [workspaceIds: string[]]
    reorderSessions: [workspaceId: string, sessionIds: string[]]
    logout: []
    openDashboard: []
    openShortcuts: []
  }>()

  const { t, locale } = useI18n()
  const themeStore = useThemeStore()
  const searchQuery = ref('')
  const expandedKeys = ref<string[]>([])
  const initializedExpandedKeys = new Set<string>()
  const draggableTreeItems = ref<WorkspaceTreeItem[]>([])

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

  watch(
    treeItems,
    (items) => {
      draggableTreeItems.value = items
      const nextExpanded = new Set(expandedKeys.value)
      for (const item of items) {
        if (
          !initializedExpandedKeys.has(item.value) &&
          item.children.some((child) => isActiveSession(child.session))
        ) {
          initializedExpandedKeys.add(item.value)
          nextExpanded.add(item.value)
        }
      }
      expandedKeys.value = Array.from(nextExpanded)
    },
    { immediate: true },
  )

  function reorderDraggedWorkspaces() {
    if (normalizedSearchQuery.value) {
      return
    }
    emit(
      'reorderWorkspaces',
      draggableTreeItems.value.map((item) => item.workspace.id),
    )
  }

  function reorderDraggedSessions(workspace: WorkspaceTreeItem) {
    if (normalizedSearchQuery.value) {
      return
    }
    emit(
      'reorderSessions',
      workspace.workspace.id,
      workspace.children.map((item) => item.session.id),
    )
  }

  function toggleWorkspace(workspaceValue: string) {
    const nextExpanded = new Set(expandedKeys.value)
    if (nextExpanded.has(workspaceValue)) {
      nextExpanded.delete(workspaceValue)
    } else {
      nextExpanded.add(workspaceValue)
    }
    expandedKeys.value = Array.from(nextExpanded)
  }

  function workspaceExpanded(workspaceValue: string) {
    return expandedKeys.value.includes(workspaceValue)
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
