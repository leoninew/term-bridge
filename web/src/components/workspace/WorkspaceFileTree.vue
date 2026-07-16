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
          <WorkbenchCodicon name="arrow-left" class-name="file-workbench-codicon" />
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
          <WorkbenchCodicon name="new-folder" class-name="file-workbench-codicon" />
        </button>
        <button
          type="button"
          class="button button-secondary button-icon file-workbench-small-button"
          :aria-label="t('files.newFile')"
          :title="t('files.newFile')"
          @click="emit('action', { type: 'create-file', path: '' })"
        >
          <WorkbenchCodicon name="new-file" class-name="file-workbench-codicon" />
        </button>
        <button
          type="button"
          class="button button-secondary button-icon file-workbench-small-button"
          :disabled="root?.loading"
          :aria-label="t('files.refreshTree')"
          :title="t('files.refreshTree')"
          @click="refreshRoot"
        >
          <WorkbenchCodicon
            :name="root?.loading ? 'loading' : 'refresh'"
            :spin="Boolean(root?.loading)"
            class-name="file-workbench-codicon"
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
        :aria-selected="!item.directory && isActiveFile(item.entry.path) ? true : undefined"
      >
        <div
          class="file-workbench-tree-row"
          :class="{
            'file-workbench-tree-row--active': !item.directory && isActiveFile(item.entry.path),
          }"
          :style="{ paddingLeft: `${8 + item.depth * 16}px` }"
          tabindex="0"
          @click="activate(item.entry)"
          @keydown.enter.prevent="activate(item.entry)"
          @keydown.space.prevent="activate(item.entry)"
        >
          <WorkbenchCodicon
            v-if="item.directory"
            :name="item.loading ? 'loading' : item.expanded ? 'chevron-down' : 'chevron-right'"
            :spin="item.loading"
            class-name="file-workbench-codicon file-workbench-tree-twistie"
          />
          <span v-else class="file-workbench-tree-twistie-spacer" />
          <WorkbenchCodicon
            :name="item.directory ? (item.expanded ? 'folder-opened' : 'folder') : 'file'"
            class-name="file-workbench-codicon file-workbench-tree-icon"
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
              <WorkbenchCodicon name="new-file" class-name="file-workbench-codicon" />
            </button>
            <button
              v-if="item.directory"
              type="button"
              class="file-workbench-tree-action"
              :aria-label="t('files.newDirectory')"
              :title="t('files.newDirectory')"
              @click="emit('action', { type: 'create-directory', path: item.entry.path })"
            >
              <WorkbenchCodicon name="new-folder" class-name="file-workbench-codicon" />
            </button>
            <button
              type="button"
              class="file-workbench-tree-action"
              :aria-label="t('files.rename')"
              :title="t('files.rename')"
              @click="emit('action', { type: 'rename', entry: item.entry })"
            >
              <WorkbenchCodicon name="edit" class-name="file-workbench-codicon" />
            </button>
            <button
              type="button"
              class="file-workbench-tree-action"
              :aria-label="t('files.move')"
              :title="t('files.move')"
              @click="emit('action', { type: 'move', entry: item.entry })"
            >
              <WorkbenchCodicon name="files" class-name="file-workbench-codicon" />
            </button>
            <button
              type="button"
              class="file-workbench-tree-action workspace-tree-node-action-danger"
              :aria-label="t('files.delete')"
              :title="t('files.delete')"
              @click="emit('action', { type: 'delete', entry: item.entry })"
            >
              <WorkbenchCodicon name="trash" class-name="file-workbench-codicon" />
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
  import type { FileEntry } from '../../gen/proto/termbridge/agent/v1/file'
  import type { FileGitRuntimeApi } from '../../features/files/runtime'
  import { isDirectory } from '../../features/files/workbenchUi'
  import { useFileWorkbenchStore } from '../../store/fileWorkbench'
  import WorkbenchCodicon from './WorkbenchCodicon.vue'

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

  function isActiveFile(path: string) {
    return store.activeDocument?.path === path
  }

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
