<template>
  <aside class="flex h-full min-w-0 flex-col overflow-hidden bg-[#070b12]">
    <header class="flex h-11 shrink-0 items-center border-b border-slate-800/80 bg-[#0a0f18] px-2">
      <div class="flex w-full items-center gap-1.5">
        <label class="relative min-w-0 flex-1">
          <span class="sr-only">{{ t('sidebar.searchSessions') }}</span>
          <Search class="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-slate-500" />
          <input
            v-model="searchQuery"
            type="search"
            :placeholder="t('sidebar.searchPlaceholder')"
            class="h-8 w-full rounded-md border border-slate-800 bg-slate-950/80 py-1.5 pl-8 pr-2 text-sm text-slate-200 outline-none placeholder:text-slate-600"
          />
        </label>
        <button
          v-if="props.allowMutations"
          type="button"
          class="flex size-8 shrink-0 items-center justify-center rounded-md border border-slate-700 bg-slate-900 text-slate-200 hover:bg-slate-800"
          :aria-label="t('sidebar.newSession')"
          @click="emit('newSession')"
        >
          <Plus class="size-4" />
        </button>
      </div>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto p-1.5">
      <div v-if="workspaceTree.length === 0" class="rounded-md border border-dashed border-slate-800 bg-slate-950/60 p-2 text-sm text-slate-500">
        {{ t('sidebar.emptyWorkspaces') }}
      </div>
      <div v-else-if="treeItems.length === 0" class="rounded-md border border-dashed border-slate-800 bg-slate-950/60 p-2 text-sm text-slate-500">
        {{ normalizedSearchQuery ? t('sidebar.noSessionsMatch', { query: searchQuery }) : t('sidebar.noActiveSessions') }}
      </div>

      <VueDraggable
        v-else
        v-model="draggableTreeItems"
        tag="div"
        class="flex flex-col"
        item-key="value"
        handle=".workspace-drag-handle"
        :animation="150"
        :disabled="Boolean(normalizedSearchQuery) || !props.allowMutations"
        @end="reorderDraggedWorkspaces"
      >
        <div v-for="workspace in draggableTreeItems" :key="workspace.value" class="min-w-0">
          <div
            role="button"
            tabindex="0"
            class="workspace-drag-handle group flex h-7 w-full min-w-0 items-center gap-1 rounded-md border border-transparent px-1 py-0.5 text-left text-slate-300 hover:border-slate-800 hover:bg-slate-900/40"
            :style="{ paddingLeft: '6px' }"
            @click="toggleWorkspace(workspace.value)"
            @keydown.enter.prevent="toggleWorkspace(workspace.value)"
            @keydown.space.prevent="toggleWorkspace(workspace.value)"
          >
            <FolderOpen v-if="workspaceExpanded(workspace.value)" class="size-4 shrink-0 text-slate-500" aria-hidden="true" />
            <Folder v-else class="size-4 shrink-0 text-slate-500" aria-hidden="true" />
            <span class="min-w-0 flex-1 truncate text-sm font-semibold">{{ workspace.workspace.name }}</span>
            <button
              v-if="props.allowMutations"
              type="button"
              class="flex size-5 shrink-0 items-center justify-center rounded text-slate-500 opacity-0 hover:bg-slate-800 hover:text-red-200 group-hover:opacity-100"
              :aria-label="t('sidebar.removeWorkspaceAria', { name: workspace.workspace.name })"
              @click.stop="emit('removeWorkspace', workspace.workspace)"
            >
              <Trash2 class="size-3.5" />
            </button>
          </div>

          <template v-if="workspaceExpanded(workspace.value)">
            <div
              v-for="session in workspace.children"
              :key="session.value"
              role="button"
              tabindex="0"
              class="group flex h-7 w-full min-w-0 items-center gap-1 rounded-md border px-1 py-0.5 text-left transition"
              :class="activeSessionId === session.session.id
                ? 'border-slate-700 bg-slate-900 text-slate-100'
                : 'border-transparent text-slate-400 hover:border-slate-800 hover:bg-slate-900/50 hover:text-slate-200'"
              :style="{ paddingLeft: '38px' }"
              @click="selectSession(session.session)"
              @keydown.enter.prevent="selectSession(session.session)"
              @keydown.space.prevent="selectSession(session.session)"
            >
              <SquareTerminal class="size-4 shrink-0 text-slate-500" aria-hidden="true" />
              <span class="min-w-0 flex-1 truncate text-sm">{{ session.session.name || session.session.command }}</span>
              <span v-if="props.allowMutations" class="flex shrink-0 items-center gap-0.5 opacity-0 group-hover:opacity-100">
                <button
                  type="button"
                  class="flex size-5 items-center justify-center rounded text-slate-500 hover:bg-slate-800 hover:text-slate-100"
                  :aria-label="t('sidebar.renameSessionAria', { name: session.session.name || session.session.command })"
                  @click.stop="emit('renameSession', session.session)"
                >
                  <Pencil class="size-3.5" />
                </button>
                <button
                  v-if="canDeleteSession(session.session)"
                  type="button"
                  class="flex size-5 items-center justify-center rounded text-slate-500 hover:bg-slate-800 hover:text-red-200"
                  :aria-label="t('sidebar.deleteSessionAria', { name: session.session.name || session.session.command })"
                  @click.stop="emit('deleteSession', session.session)"
                >
                  <Trash2 class="size-3.5" />
                </button>
              </span>
            </div>
          </template>
        </div>
      </VueDraggable>
    </div>

    <footer class="flex h-8 shrink-0 items-center border-t border-slate-800/80 bg-[#0a0f18] px-2 text-sm text-slate-500">
      <DropdownMenuRoot>
        <DropdownMenuTrigger
          class="flex h-6 items-center gap-1.5 rounded px-1.5 text-slate-500 outline-none hover:bg-slate-900 hover:text-slate-200 focus:bg-slate-900 focus:text-slate-200"
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
            class="z-50 min-w-44 rounded-md border border-slate-800 bg-slate-950 p-1 text-sm text-slate-200 shadow-xl"
          >
            <DropdownMenuSub>
              <DropdownMenuSubTrigger class="flex cursor-pointer items-center justify-between rounded px-2 py-1.5 outline-none hover:bg-slate-800 focus:bg-slate-800">
                <span class="inline-flex items-center gap-2">{{ t('common.language') }}</span>
                <ChevronRight class="size-4 text-slate-500" />
              </DropdownMenuSubTrigger>
              <DropdownMenuPortal>
                <DropdownMenuSubContent
                  :side-offset="8"
                  class="z-50 min-w-36 rounded-md border border-slate-800 bg-slate-950 p-1 text-sm text-slate-200 shadow-xl"
                >
                  <DropdownMenuRadioGroup :model-value="locale" @update:model-value="changeLocale">
                    <DropdownMenuRadioItem
                      v-for="item in localeOptions"
                      :key="item.value"
                      :value="item.value"
                      class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none hover:bg-slate-800 focus:bg-slate-800"
                    >
                      <Check :class="locale === item.value ? 'opacity-100' : 'opacity-0'" class="size-4 text-blue-500" />
                      {{ item.label }}
                    </DropdownMenuRadioItem>
                  </DropdownMenuRadioGroup>
                </DropdownMenuSubContent>
              </DropdownMenuPortal>
            </DropdownMenuSub>
          </DropdownMenuContent>
        </DropdownMenuPortal>
      </DropdownMenuRoot>
    </footer>
  </aside>
</template>

<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { Check, ChevronRight, Folder, FolderOpen, Pencil, Plus, Search, Settings, SquareTerminal, Trash2 } from '@lucide/vue'
  import {
    DropdownMenuContent,
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
  import type { SessionSummary, WorkspaceSummary, WorkspaceTreeSummary } from '../../protocol/terminal'

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
    allowMutations?: boolean
  }>()

  const emit = defineEmits<{
    select: [session: SessionSummary]
    refresh: []
    newSession: []
    renameSession: [session: SessionSummary]
    deleteSession: [session: SessionSummary]
    removeWorkspace: [workspace: WorkspaceSummary]
    unsupportedDirectoryDelete: [workspace: WorkspaceSummary]
    reorderWorkspaces: [workspaceIds: string[]]
  }>()

  const { t, locale } = useI18n()
  const searchQuery = ref('')
  const expandedKeys = ref<string[]>([])
  const initializedExpandedKeys = new Set<string>()
  const draggableTreeItems = ref<WorkspaceTreeItem[]>([])

  const normalizedSearchQuery = computed(() => searchQuery.value.trim().toLowerCase())
  const localeOptions = computed(() => locales.map((value) => ({ value, label: localeLabels[value] })))

  function changeLocale(value: unknown) {
    if (typeof value === 'string' && locales.includes(value as AppLocale)) {
      setLocale(value as AppLocale)
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
      .filter((item) => !normalizedSearchQuery.value || item.children.length > 0 || workspaceMatchesSearch(item.workspace))
  })

  watch(
    treeItems,
    (items) => {
      draggableTreeItems.value = items
      const nextExpanded = new Set(expandedKeys.value)
      for (const item of items) {
        if (!initializedExpandedKeys.has(item.value) && item.children.some((child) => isActiveSession(child.session))) {
          initializedExpandedKeys.add(item.value)
          nextExpanded.add(item.value)
        }
      }
      expandedKeys.value = Array.from(nextExpanded)
    },
    { immediate: true },
  )

  function reorderDraggedWorkspaces() {
    if (normalizedSearchQuery.value || !props.allowMutations) {
      return
    }
    emit('reorderWorkspaces', draggableTreeItems.value.map((item) => item.workspace.id))
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

  function isActiveSession(session: SessionSummary) {
    return ['starting', 'running', 'stopping'].includes(session.lifecycle_state)
  }

  function canDeleteSession(session: SessionSummary) {
    return ['stopped', 'failed'].includes(session.lifecycle_state)
  }

  function sessionsFor(workspace: WorkspaceTreeSummary): SessionSummary[] {
    return workspace.children
      .map((session) => ({
        ...session,
        workspace_id: session.workspace_id ?? workspace.id,
        workspace_key: session.workspace_key ?? workspace.key,
      }))
      .filter((session) => !normalizedSearchQuery.value || sessionMatchesSearch(session, workspace))
  }

  function workspaceMatchesSearch(workspace: WorkspaceSummary) {
    const haystack = [workspace.id, workspace.key, workspace.name, workspace.path]
      .join(' ')
      .toLowerCase()
    return haystack.includes(normalizedSearchQuery.value)
  }

  function sessionMatchesSearch(session: SessionSummary, workspace: WorkspaceSummary) {
    const haystack = [
      session.id,
      session.name,
      session.command,
      session.cwd,
      session.lifecycle_state,
      session.attachment_state ?? '',
      workspace.name,
      workspace.path,
    ]
      .join(' ')
      .toLowerCase()
    return haystack.includes(normalizedSearchQuery.value)
  }
</script>
