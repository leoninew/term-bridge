<template>
  <section
    class="file-workbench-drawer"
    :class="{ 'file-workbench-drawer--open': open }"
    :aria-hidden="open ? undefined : 'true'"
    :inert="!open"
    role="region"
    :aria-label="t('files.drawerTitle', { name: workspace.name })"
    @keydown.esc="close"
    @transitionend="handleTransitionEnd"
  >
    <div class="file-workbench-layout">
      <WorkspaceFileTree
        :api="api"
        :workspace-name="workspace.name"
        @action="openAction"
        @back="close"
      />
      <main class="file-workbench-editor-panel">
        <WorkspaceEditorTabs
          :documents="store.documents"
          :active-document="store.activeDocument"
          @activate="store.setActiveDocument"
          @close="requestCloseDocument"
        >
          <WorkspaceTextEditor
            :target="store.target"
            :workspace-id="store.workspaceId"
            :document="store.activeDocument"
            @draft="store.setDocumentDraft"
            @save="saveDocument"
          />
          <div v-if="store.activeDocument?.conflict" class="file-workbench-conflict-actions">
            <button
              type="button"
              class="button button-secondary"
              @click="requestReload(store.activeDocument.path)"
            >
              {{ t('files.reloadDiscard') }}
            </button>
            <button type="button" class="button button-secondary" @click="copyDraft">
              {{ t('files.copyDraft') }}
            </button>
            <button type="button" class="button button-danger" @click="requestForceSave">
              {{ t('files.forceSave') }}
            </button>
          </div>
        </WorkspaceEditorTabs>
        <footer class="file-workbench-status-bar" role="status">
          <span class="min-w-0 flex-1 truncate">{{ editorStatusPrimary }}</span>
          <span
            v-if="editorStatusSecondary"
            class="min-w-0 shrink truncate text-[var(--color-text-subtle)]"
          >
            {{ editorStatusSecondary }}
          </span>
        </footer>
      </main>
    </div>

    <WorkspaceFileActionDialog
      :open="action !== null"
      :title="actionTitle"
      :description="actionDescription"
      :label="actionLabel"
      :placeholder="actionPlaceholder"
      :requires-value="actionRequiresValue"
      :danger="
        action?.type === 'delete' ||
        action?.type === 'delete-recursive' ||
        action?.type === 'overwrite-file' ||
        action?.type === 'force-save'
      "
      :initial-value="actionInitialValue"
      @update:open="closeAction"
      @confirm="performAction"
    />
  </section>
</template>

<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import type { Workspace as WorkspaceSummary } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { FileEntry } from '../../gen/proto/termbridge/agent/v1/file'
  import type { FileGitRuntimeApi } from '../../features/files/runtime'
  import type { RuntimeTarget } from '../../features/runtimeTarget'
  import {
    entryName,
    entryPath,
    isDirectory,
    isFile,
    parentPath,
  } from '../../features/files/workbenchUi'
  import { useFileWorkbenchStore } from '../../store/fileWorkbench'
  import { useNotificationsStore } from '../../store/notifications'
  import WorkspaceEditorTabs from './WorkspaceEditorTabs.vue'
  import WorkspaceFileActionDialog from './WorkspaceFileActionDialog.vue'
  import WorkspaceFileTree, { type FileTreeAction } from './WorkspaceFileTree.vue'
  import WorkspaceTextEditor from './WorkspaceTextEditor.vue'

  type DrawerAction =
    | FileTreeAction
    | { type: 'overwrite-file'; entry: FileEntry }
    | { type: 'delete-recursive'; entry: FileEntry }
    | { type: 'close-document'; path: string }
    | { type: 'reload'; path: string }
    | { type: 'force-save'; path: string }

  const props = defineProps<{
    open: boolean
    workspace: WorkspaceSummary
    target: RuntimeTarget
    api: FileGitRuntimeApi
  }>()

  const emit = defineEmits<{
    close: []
    closed: []
  }>()

  const { t } = useI18n()
  const store = useFileWorkbenchStore()
  const notifications = useNotificationsStore()
  const action = ref<DrawerAction | null>(null)

  const actionTitle = computed(() => {
    if (!action.value) return ''
    if (action.value.type === 'create-file') return t('files.newFile')
    if (action.value.type === 'create-directory') return t('files.newDirectory')
    if (action.value.type === 'rename') return t('files.rename')
    if (action.value.type === 'move') return t('files.move')
    if (action.value.type === 'delete' || action.value.type === 'delete-recursive')
      return t('files.delete')
    if (action.value.type === 'overwrite-file') return t('files.overwriteFile')
    if (action.value.type === 'close-document') return t('files.closeDirtyDocument')
    if (action.value.type === 'reload') return t('files.reloadDiscard')
    return t('files.forceSave')
  })
  const actionDescription = computed(() => {
    if (!action.value) return ''
    if (action.value.type === 'overwrite-file')
      return t('files.overwriteDescription', { path: action.value.entry.path })
    if (action.value.type === 'delete-recursive')
      return t('files.recursiveDeleteDescription', { path: action.value.entry.path })
    if ('entry' in action.value) return action.value.entry.path
    if ('path' in action.value) return action.value.path
    return ''
  })
  const actionLabel = computed(() => {
    if (!action.value) return ''
    if (action.value.type === 'rename') return t('files.name')
    if (action.value.type === 'move') return t('files.destinationDirectory')
    if (action.value.type === 'create-file' || action.value.type === 'create-directory')
      return t('files.name')
    return ''
  })
  const actionPlaceholder = computed(() => {
    if (action.value?.type === 'move') return t('files.destinationDirectoryPlaceholder')
    return t('files.namePlaceholder')
  })
  const actionInitialValue = computed(() =>
    action.value?.type === 'rename' ? entryName(action.value.entry.path) : '',
  )
  const actionRequiresValue = computed(() =>
    ['create-file', 'create-directory', 'rename', 'move'].includes(action.value?.type ?? ''),
  )

  const editorStatusPrimary = computed(() => {
    const document = store.activeDocument
    if (!document) return t('files.noDocument')
    if (document.loading) return t('files.loadingFile')
    if (document.saving) return t('files.saving')
    if (document.conflict) return t('files.revisionConflict')
    if (document.error) return document.error
    if (document.dirty) return t('files.editorStatusDirty')
    return t('files.editorStatusReady')
  })

  const editorStatusSecondary = computed(() => {
    const document = store.activeDocument
    if (!document) return ''
    return document.path
  })

  watch(
    () => [props.open, props.workspace.id, props.target] as const,
    async ([open]) => {
      if (!open) return
      store.openWorkspace(props.target, props.workspace.id)
      await store.ensureDirectoryLoaded(props.api, '')
    },
    { immediate: true, deep: false },
  )

  function close() {
    if (action.value) return
    store.cancelReadonlyRequests()
    emit('close')
  }

  function handleTransitionEnd(event: TransitionEvent) {
    if (!props.open && event.target === event.currentTarget && event.propertyName === 'transform') {
      emit('closed')
    }
  }

  function closeAction(open: boolean) {
    if (!open) action.value = null
  }

  function openAction(next: FileTreeAction) {
    action.value = next
  }

  function requestCloseDocument(path: string) {
    const document = store.documentFor(path)
    if (!document || (!document.dirty && !document.saving)) {
      store.closeDocument(path)
      return
    }
    action.value = { type: 'close-document', path }
  }

  function requestReload(path: string) {
    const document = store.documentFor(path)
    if (document?.dirty || document?.conflict) {
      action.value = { type: 'reload', path }
      return
    }
    void reloadDocument(path)
  }

  function requestForceSave() {
    if (store.activeDocument) action.value = { type: 'force-save', path: store.activeDocument.path }
  }

  async function saveDocument(path: string, force: boolean) {
    const message = await store.saveDocument(props.api, path, force)
    if (message && message !== 'revision_conflict') {
      notifications.pushToast('error', t('toast.fileSaveFailed'), message)
    }
  }

  async function reloadDocument(path: string) {
    const message = await store.reloadDocumentDiscardingDraft(props.api, path)
    if (message) notifications.pushToast('error', t('toast.fileLoadFailed'), message)
  }

  async function copyDraft() {
    const document = store.activeDocument
    if (!document) return
    try {
      await navigator.clipboard.writeText(document.draftText)
      notifications.pushToast('success', t('toast.fileDraftCopied'))
    } catch (error) {
      notifications.notifyError(t('toast.copyFileDraftFailed'), error)
    }
  }

  function findEntry(path: string): FileEntry | null {
    for (const directory of store.directories) {
      if (directory.directory?.path === path) return directory.directory
      const entry = directory.items.find((item) => item.path === path)
      if (entry) return entry
    }
    return null
  }

  function directoryRevision(path: string): string | null {
    return store.directoryFor(path)?.directory?.revision ?? findEntry(path)?.revision ?? null
  }

  async function performAction(value: string) {
    const next = action.value
    if (!next) return
    try {
      const message = await applyAction(next, value)
      if (message) notifications.pushToast('error', t('toast.fileMutationFailed'), message)
    } catch (error) {
      notifications.notifyError(t('toast.fileMutationFailed'), error)
    }
    if (action.value === next) action.value = null
  }

  async function applyAction(next: DrawerAction, value: string): Promise<string | null> {
    let message: string | null = null
    if (next.type === 'create-file' || next.type === 'create-directory') {
      const path = entryPath(next.path, value.trim())
      const parentRevision = directoryRevision(next.path)
      if (!parentRevision) {
        message = t('files.refreshDirectoryBeforeMutation')
      } else if (next.type === 'create-file') {
        const existing = findEntry(path)
        if (existing && isFile(existing)) {
          if (store.hasDirtyDocumentUnderPath(path)) {
            message = t('files.dirtyOperationBlocked')
          } else {
            action.value = { type: 'overwrite-file', entry: existing }
            return null
          }
        } else {
          await store.createFile(props.api, {
            path,
            text: '',
            overwrite: false,
            expected_parent_revision: parentRevision,
            expected_destination_revision: '',
          })
        }
      } else {
        await store.createDirectory(props.api, {
          path,
          allow_existing: false,
          expected_parent_revision: parentRevision,
        })
      }
    } else if (next.type === 'rename') {
      const parent = parentPath(next.entry.path)
      const parentRevision = directoryRevision(parent)
      if (!parentRevision) {
        message = t('files.refreshDirectoryBeforeMutation')
      } else {
        await store.renameEntry(props.api, {
          path: next.entry.path,
          new_name: value.trim(),
          expected_source_revision: next.entry.revision,
          expected_parent_revision: parentRevision,
          expected_destination_revision: '',
        })
      }
    } else if (next.type === 'move') {
      const targetDirectory = value.trim().replace(/^\/+|\/+$/g, '')
      const targetRevision = directoryRevision(targetDirectory)
      const sourceParentRevision = directoryRevision(parentPath(next.entry.path))
      if (!targetRevision || !sourceParentRevision) {
        message = t('files.refreshDirectoryBeforeMutation')
      } else {
        await store.moveEntry(props.api, {
          source_path: next.entry.path,
          destination_path: entryPath(targetDirectory, entryName(next.entry.path)),
          expected_source_revision: next.entry.revision,
          expected_source_parent_revision: sourceParentRevision,
          expected_destination_parent_revision: targetRevision,
          expected_destination_revision: '',
        })
      }
    } else if (next.type === 'overwrite-file') {
      const parentRevision = directoryRevision(parentPath(next.entry.path))
      if (!parentRevision) {
        message = t('files.refreshDirectoryBeforeMutation')
      } else if (store.hasDirtyDocumentUnderPath(next.entry.path)) {
        message = t('files.dirtyOperationBlocked')
      } else {
        await store.createFile(props.api, {
          path: next.entry.path,
          text: '',
          overwrite: true,
          expected_parent_revision: parentRevision,
          expected_destination_revision: next.entry.revision,
        })
      }
    } else if (next.type === 'delete' || next.type === 'delete-recursive') {
      const parentRevision = directoryRevision(parentPath(next.entry.path))
      if (!parentRevision) {
        message = t('files.refreshDirectoryBeforeMutation')
      } else if (store.hasDirtyDocumentUnderPath(next.entry.path)) {
        message = t('files.dirtyOperationBlocked')
      } else {
        try {
          await store.deleteEntry(props.api, {
            path: next.entry.path,
            expected_revision: next.entry.revision,
            expected_parent_revision: parentRevision,
            recursive: next.type === 'delete-recursive',
          })
        } catch (error) {
          if (
            next.type === 'delete' &&
            isDirectory(next.entry) &&
            typeof error === 'object' &&
            error !== null &&
            'code' in error &&
            (error as { code?: unknown }).code === 'directory_not_empty'
          ) {
            action.value = { type: 'delete-recursive', entry: next.entry }
            return null
          }
          throw error
        }
      }
    } else if (next.type === 'close-document') {
      store.discardDocumentDraft(next.path)
      store.closeDocument(next.path)
    } else if (next.type === 'reload') {
      await reloadDocument(next.path)
    } else if (next.type === 'force-save') {
      await saveDocument(next.path, true)
    }
    return message
  }
</script>
