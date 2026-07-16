<template>
  <section class="file-workbench-tree-panel" :aria-label="t('files.treeTitle')">
    <header class="file-workbench-panel-header">
      <div class="file-workbench-panel-title">
        <button
          type="button"
          class="button button-secondary button-icon file-workbench-small-button"
          :aria-label="t('files.backToSessions')"
          :title="t('files.backToSessions')"
          @click="emit('back')"
        >
          <ArrowLeft class="size-3.5" aria-hidden="true" />
        </button>
        <h3 class="truncate">{{ workspaceName }}</h3>
      </div>
      <span class="flex shrink-0 gap-1">
        <button
          type="button"
          class="button button-secondary button-icon file-workbench-small-button"
          :aria-label="t('files.newDirectory')"
          :title="t('files.newDirectory')"
          @click="emit('action', { type: 'create-directory', path: '' })"
        >
          <FolderPlus class="size-3.5" aria-hidden="true" />
        </button>
        <button
          type="button"
          class="button button-secondary button-icon file-workbench-small-button"
          :aria-label="t('files.newFile')"
          :title="t('files.newFile')"
          @click="emit('action', { type: 'create-file', path: '' })"
        >
          <FilePlus2 class="size-3.5" aria-hidden="true" />
        </button>
        <button
          type="button"
          class="button button-secondary button-icon file-workbench-small-button"
          :disabled="root?.loading"
          :aria-label="t('files.refreshTree')"
          :title="t('files.refreshTree')"
          @click="refreshRoot"
        >
          <RefreshCw
            class="size-3.5"
            :class="{ 'animate-spin': root?.loading }"
            aria-hidden="true"
          />
        </button>
      </span>
    </header>
    <p v-if="root?.error" class="file-workbench-error" role="status">{{ root.error }}</p>
    <p v-if="root?.truncated" class="file-workbench-note" role="status">
      {{ t('files.treeTruncated') }}
    </p>
    <p v-if="root?.loading && !root.loaded" class="file-workbench-note" role="status">
      {{ t('files.loadingTree') }}
    </p>
    <ul class="file-workbench-tree" role="tree" :aria-label="t('files.treeTitle')">
      <li
        v-for="item in visibleItems"
        :key="item.entry.path"
        role="treeitem"
        :aria-expanded="item.directory ? item.expanded : undefined"
      >
        <div
          class="file-workbench-tree-row"
          :style="{ paddingLeft: `${8 + item.depth * 16}px` }"
          tabindex="0"
          @click="activate(item.entry)"
          @keydown.enter.prevent="activate(item.entry)"
          @keydown.space.prevent="activate(item.entry)"
        >
          <component
            :is="item.loading ? Loader2 : item.expanded ? ChevronDown : ChevronRight"
            v-if="item.directory"
            class="size-3.5"
            :class="{ 'animate-spin': item.loading }"
            aria-hidden="true"
          />
          <span v-else class="w-3.5" />
          <component
            :is="item.directory ? (item.expanded ? FolderOpen : Folder) : File"
            class="size-4 shrink-0"
            aria-hidden="true"
          />
          <span class="min-w-0 flex-1 truncate">{{ item.entry.name }}</span>
          <span class="file-workbench-tree-menu" @click.stop>
            <button
              v-if="item.directory"
              type="button"
              class="file-workbench-tree-action"
              :aria-label="t('files.newFile')"
              :title="t('files.newFile')"
              @click="emit('action', { type: 'create-file', path: item.entry.path })"
            >
              <FilePlus2 class="size-3.5" aria-hidden="true" />
            </button>
            <button
              v-if="item.directory"
              type="button"
              class="file-workbench-tree-action"
              :aria-label="t('files.newDirectory')"
              :title="t('files.newDirectory')"
              @click="emit('action', { type: 'create-directory', path: item.entry.path })"
            >
              <FolderPlus class="size-3.5" aria-hidden="true" />
            </button>
            <button
              type="button"
              class="file-workbench-tree-action"
              :aria-label="t('files.rename')"
              :title="t('files.rename')"
              @click="emit('action', { type: 'rename', entry: item.entry })"
            >
              <Pencil class="size-3.5" aria-hidden="true" />
            </button>
            <button
              type="button"
              class="file-workbench-tree-action"
              :aria-label="t('files.move')"
              :title="t('files.move')"
              @click="emit('action', { type: 'move', entry: item.entry })"
            >
              <FolderInput class="size-3.5" aria-hidden="true" />
            </button>
            <button
              type="button"
              class="file-workbench-tree-action workspace-tree-node-action-danger"
              :aria-label="t('files.delete')"
              :title="t('files.delete')"
              @click="emit('action', { type: 'delete', entry: item.entry })"
            >
              <Trash2 class="size-3.5" aria-hidden="true" />
            </button>
          </span>
        </div>
      </li>
    </ul>
    <footer class="file-workbench-status-bar" role="status">
      <span class="min-w-0 flex-1 truncate">{{ statusPrimary }}</span>
      <span v-if="statusSecondary" class="min-w-0 shrink truncate text-[var(--color-text-subtle)]">
        {{ statusSecondary }}
      </span>
    </footer>
  </section>
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import {
    ArrowLeft,
    ChevronDown,
    ChevronRight,
    File,
    FilePlus2,
    Folder,
    FolderInput,
    FolderOpen,
    FolderPlus,
    Loader2,
    Pencil,
    RefreshCw,
    Trash2,
  } from '@lucide/vue'
  import type { FileEntry } from '../../gen/proto/termbridge/agent/v1/file'
  import type { FileGitRuntimeApi } from '../../features/files/runtime'
  import { isDirectory } from '../../features/files/workbenchUi'
  import { useFileWorkbenchStore } from '../../store/fileWorkbench'

  export type FileTreeAction =
    | { type: 'create-file'; path: string }
    | { type: 'create-directory'; path: string }
    | { type: 'rename'; entry: FileEntry }
    | { type: 'move'; entry: FileEntry }
    | { type: 'delete'; entry: FileEntry }

  type VisibleTreeItem = {
    entry: FileEntry
    depth: number
    directory: boolean
    expanded: boolean
    loading: boolean
  }

  const props = defineProps<{
    api: FileGitRuntimeApi
    workspaceName: string
  }>()
  const emit = defineEmits<{
    action: [action: FileTreeAction]
    back: []
  }>()
  const { t } = useI18n()
  const store = useFileWorkbenchStore()
  const root = computed(() => store.directoryFor(''))
  const rootItemCount = computed(() => root.value?.items.length ?? 0)
  const statusPrimary = computed(() => {
    if (root.value?.error) return root.value.error
    if (root.value?.loading && !root.value.loaded) return t('files.loadingTree')
    if (!root.value?.loaded) return t('files.treeStatusIdle')
    if (root.value.truncated) {
      return t('files.treeStatusTruncated', { count: rootItemCount.value })
    }
    if (rootItemCount.value === 0) return t('files.treeStatusEmpty')
    return t('files.treeStatusReady', { count: rootItemCount.value })
  })
  const statusSecondary = computed(() => {
    const active = store.activeDocument
    if (!active) return ''
    return active.dirty
      ? t('files.treeStatusActiveDirty', { path: active.path })
      : t('files.treeStatusActive', { path: active.path })
  })

  const visibleItems = computed(() => {
    const items: VisibleTreeItem[] = []
    appendChildren('', 0, items)
    return items
  })

  function appendChildren(path: string, depth: number, items: VisibleTreeItem[]) {
    const directory = store.directoryFor(path)
    for (const entry of directory?.items ?? []) {
      const directoryState = isDirectory(entry) ? store.directoryFor(entry.path) : null
      const item = {
        entry,
        depth,
        directory: isDirectory(entry),
        expanded: directoryState?.expanded ?? false,
        loading: directoryState?.loading ?? false,
      }
      items.push(item)
      if (item.directory && item.expanded) appendChildren(entry.path, depth + 1, items)
    }
  }

  async function refreshRoot() {
    await store.refreshDirectory(props.api, '')
  }

  async function activate(entry: FileEntry) {
    if (!isDirectory(entry)) {
      await store.openDocument(props.api, entry.path)
      return
    }
    const expanded = store.toggleDirectoryExpanded(entry.path)
    if (expanded) await store.ensureDirectoryLoaded(props.api, entry.path)
  }
</script>
