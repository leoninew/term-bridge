<template>
  <AppPageShell main-class="shortcut-page-main">
    <section class="shortcut-page bg-[var(--color-app-bg)] text-[var(--color-text)]">
      <section class="shortcut-page-content mx-auto flex w-full max-w-[1440px] flex-col">
        <header class="shortcut-page-header">
          <div class="shortcut-toolbar-left">
            <div class="shortcut-title-group">
              <h1 class="shortcut-page-title">
                {{ t('shortcut.title') }}
              </h1>
            </div>

            <div class="shortcut-toolbar-filters">
              <input
                v-model="filterQuery"
                class="shortcut-search-input"
                :placeholder="t('shortcut.filterPlaceholder')"
                type="search"
              />

              <div
                v-if="tags.length"
                class="shortcut-tag-segments"
                role="group"
                :aria-label="t('shortcut.filterTags')"
              >
                <span
                  class="shortcut-tag-segment"
                  :class="{ 'shortcut-tag-segment-active': selectedTags.length === 0 }"
                  role="button"
                  tabindex="0"
                  :aria-pressed="selectedTags.length === 0"
                  @click="clearTags"
                  @keydown.enter.prevent="clearTags"
                  @keydown.space.prevent="clearTags"
                >
                  {{ t('shortcut.allTags') }}
                </span>
                <span
                  v-for="tag in tags"
                  :key="tag"
                  class="shortcut-tag-segment"
                  :class="{ 'shortcut-tag-segment-active': selectedTags.includes(tag) }"
                  role="button"
                  tabindex="0"
                  :aria-pressed="selectedTags.includes(tag)"
                  @click="toggleTag(tag)"
                  @keydown.enter.prevent="toggleTag(tag)"
                  @keydown.space.prevent="toggleTag(tag)"
                >
                  {{ tag }}
                </span>
              </div>
            </div>
          </div>

          <div class="shortcut-toolbar-right">
            <button
              type="button"
              class="button button-secondary shortcut-select-button inline-flex items-center gap-1"
              :class="{ 'shortcut-select-button-active': selectionMode }"
              :aria-pressed="selectionMode"
              :title="selectionMode ? t('shortcut.selectAllTip') : undefined"
              :aria-keyshortcuts="selectionMode ? 'Control+A Meta+A' : undefined"
              :disabled="busy"
              @click="toggleSelectionMode"
            >
              <CheckSquare class="size-4" aria-hidden="true" />
              {{ selectionMode ? t('shortcut.exitSelect') : t('shortcut.select') }}
            </button>
            <button
              v-if="selectedCount > 0"
              type="button"
              class="button button-secondary shortcut-export-button inline-flex items-center gap-1"
              :disabled="busy"
              @click="exportSelected"
            >
              <Download class="size-4" aria-hidden="true" />
              {{ t('shortcut.export') }}
            </button>
            <button
              v-if="selectedCount > 0"
              type="button"
              class="button button-danger shortcut-batch-delete-button inline-flex items-center gap-1"
              :disabled="busy"
              @click="openBatchDelete"
            >
              <Trash2 class="size-4" aria-hidden="true" />
              {{ t('shortcut.batchDelete') }}
            </button>
            <button
              type="button"
              class="button button-secondary shortcut-import-button inline-flex items-center gap-1"
              :disabled="busy"
              @click="openImportPicker"
            >
              <Upload class="size-4" aria-hidden="true" />
              {{ t('shortcut.import') }}
            </button>
            <button
              type="button"
              class="button button-primary shortcut-create-button inline-flex items-center gap-1"
              :disabled="busy"
              @click="openCreate"
            >
              <Plus class="size-4" aria-hidden="true" />
              {{ t('shortcut.createTitle') }}
            </button>
            <button
              type="button"
              class="button button-secondary shortcut-return-button"
              :disabled="busy"
              @click="props.returnToWorkspace"
            >
              {{ t('common.returnToWorkspace') }}
            </button>
            <input
              ref="importInput"
              class="shortcut-import-input"
              type="file"
              accept="application/json,.json"
              @change="onImportFileChange"
            />
          </div>
        </header>

        <PageStatus
          class="shortcut-page-status"
          :loading="loading"
          :error="loadError"
          :empty="!loading && !loadError && shortcuts.length === 0"
          :loading-text="t('shortcut.loading')"
          :empty-text="t('shortcut.empty')"
        >
          <template #loading>
            <p class="shortcut-loading text-center text-sm text-[var(--color-text-muted)]">
              {{ t('shortcut.loading') }}
            </p>
          </template>
          <template #error>
            <section
              class="shortcut-empty border border-dashed border-[var(--color-border-strong)] bg-[var(--color-surface)] text-center text-sm text-[var(--color-danger-text)]"
            >
              {{ loadError }}
            </section>
          </template>
          <template #empty>
            <section
              class="shortcut-empty border border-dashed border-[var(--color-border-strong)] bg-[var(--color-surface)] text-center text-sm text-[var(--color-text-muted)]"
            >
              {{ t('shortcut.empty') }}
            </section>
          </template>

          <VueDraggable
            v-if="!filtering"
            v-model="shortcuts"
            tag="div"
            class="shortcut-grid"
            item-key="id"
            :animation="150"
            :disabled="reordering || selectionMode"
            :filter="'button, [data-no-drag]'"
            :prevent-on-filter="false"
            ghost-class="shortcut-sortable-ghost"
            chosen-class="shortcut-sortable-chosen"
            drag-class="shortcut-sortable-dragging"
            @start="rememberOrder"
            @end="persistOrder"
          >
            <ShortcutCard
              v-for="shortcut in shortcuts"
              :key="shortcut.id"
              :shortcut="shortcut"
              :selection-mode="selectionMode"
              :selected="selectedIds.has(shortcut.id)"
              :class="!reordering && !selectionMode ? 'shortcut-card-sortable' : ''"
              @edit="openEdit"
              @delete="openDelete"
              @copy="copyCommand"
              @toggle-enabled="toggleEnabled"
              @toggle="toggleSelected"
            />
          </VueDraggable>

          <div v-else-if="filteredShortcuts.length" class="shortcut-grid">
            <ShortcutCard
              v-for="shortcut in filteredShortcuts"
              :key="shortcut.id"
              :shortcut="shortcut"
              :selection-mode="selectionMode"
              :selected="selectedIds.has(shortcut.id)"
              @edit="openEdit"
              @delete="openDelete"
              @copy="copyCommand"
              @toggle-enabled="toggleEnabled"
              @toggle="toggleSelected"
            />
          </div>

          <section
            v-else
            class="shortcut-empty border border-dashed border-[var(--color-border-strong)] bg-[var(--color-surface)] text-center text-sm text-[var(--color-text-muted)]"
          >
            {{ t('shortcut.noFilterMatches') }}
          </section>
        </PageStatus>
      </section>
    </section>
  </AppPageShell>

  <ShortcutEditorDialog
    :open="editorOpen"
    :shortcut="selected"
    :saving="saving"
    @update:open="editorOpen = $event"
    @submit="save"
  />
  <DeleteShortcutDialog
    :open="deleteOpen"
    :shortcut="selected"
    :deleting="deleting"
    @update:open="deleteOpen = $event"
    @confirm="remove"
  />
  <BatchDeleteShortcutsDialog
    :open="batchDeleteOpen"
    :count="selectedCount"
    :deleting="batchDeleting"
    @update:open="batchDeleteOpen = $event"
    @confirm="removeSelected"
  />
  <ImportShortcutsDialog
    :open="importOpen"
    :count="pendingImportItems.length"
    :name="pendingImportName"
    :importing="importing"
    @update:open="onImportDialogOpenChange"
    @confirm="confirmImport"
  />
</template>

<script setup lang="ts">
  import { CheckSquare, Download, Plus, Trash2, Upload } from '@lucide/vue'
  import { computed, onMounted, onUnmounted, ref } from 'vue'
  import { storeToRefs } from 'pinia'
  import { VueDraggable } from 'vue-draggable-plus'
  import { useI18n } from 'vue-i18n'
  import type { Shortcut, UpdateShortcutReq } from '../../gen/proto/termbridge/agent/v1/shortcut'
  import type { ShortcutRuntimeApi } from '../../features/sessions/runtime'
  import { useAsyncAction } from '../../composable/useAsyncAction'
  import { useNotificationsStore } from '../../store/notifications'
  import { useShortcutTagsStore } from '../../store/shortcutTags'
  import AppPageShell from '../layout/AppPageShell.vue'
  import PageStatus from '../layout/PageStatus.vue'
  import BatchDeleteShortcutsDialog from './BatchDeleteShortcutsDialog.vue'
  import ImportShortcutsDialog from './ImportShortcutsDialog.vue'
  import type { ShortcutExportItem } from './shortcutTransfer'
  import DeleteShortcutDialog from './DeleteShortcutDialog.vue'
  import ShortcutCard from './ShortcutCard.vue'
  import ShortcutEditorDialog from './ShortcutEditorDialog.vue'
  import {
    downloadJsonFile,
    formatShortcutExportFilename,
    parseShortcutImportJson,
    toCreateShortcutRequest,
    toShortcutExportItem,
  } from './shortcutTransfer'

  const props = defineProps<{
    api: ShortcutRuntimeApi
    returnToWorkspace: () => void | Promise<void>
  }>()
  const { t } = useI18n()
  const notifications = useNotificationsStore()
  const shortcutTags = useShortcutTagsStore()
  const { tags } = storeToRefs(shortcutTags)
  const shortcuts = ref<Shortcut[]>([])
  const filterQuery = ref('')
  const selectedTags = ref<string[]>([])
  const selectionMode = ref(false)
  const selectedIds = ref<Set<string>>(new Set())
  const filtering = computed(() => selectedTags.value.length > 0 || filterQuery.value.trim() !== '')
  const filteredShortcuts = computed(() => {
    const query = filterQuery.value.trim().toLocaleLowerCase()
    return shortcuts.value.filter((shortcut) => {
      const matchesTags = selectedTags.value.every((tag) => shortcut.tags?.includes(tag))
      const matchesQuery =
        query === '' ||
        shortcut.name.toLocaleLowerCase().includes(query) ||
        shortcut.command.toLocaleLowerCase().includes(query)
      return matchesTags && matchesQuery
    })
  })
  const selectedCount = computed(() => selectedIds.value.size)
  const loadError = ref<string | null>(null)
  const previousOrder = ref<Shortcut[]>([])
  const editorOpen = ref(false)
  const deleteOpen = ref(false)
  const batchDeleteOpen = ref(false)
  const importOpen = ref(false)
  const pendingImportItems = ref<ShortcutExportItem[]>([])
  const pendingImportName = ref('')
  const selected = ref<Shortcut | null>(null)
  const importInput = ref<HTMLInputElement | null>(null)
  const loadAction = useAsyncAction({
    onError: (err) => {
      loadError.value = t('toast.loadShortcutsFailed')
      notifications.notifyError(t('toast.loadShortcutsFailed'), err)
    },
  })
  const saveAction = useAsyncAction()
  const deleteAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('toast.deleteShortcutFailed'), err),
  })
  const batchDeleteAction = useAsyncAction()
  const importAction = useAsyncAction()
  const reorderAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('toast.updateShortcutOrderFailed'), err),
  })
  const toggleAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('toast.updateShortcutFailed'), err),
  })
  const copyAction = useAsyncAction({
    onError: (err) => notifications.notifyError(t('toast.copyCommandFailed'), err),
  })
  const loading = computed(() => loadAction.running)
  const saving = computed(() => saveAction.running)
  const deleting = computed(() => deleteAction.running)
  const batchDeleting = computed(() => batchDeleteAction.running)
  const importing = computed(() => importAction.running)
  const reordering = computed(() => reorderAction.running)
  const busy = computed(
    () =>
      saving.value ||
      deleting.value ||
      batchDeleting.value ||
      importing.value ||
      reordering.value ||
      toggleAction.running ||
      copyAction.running,
  )

  function clearTags() {
    selectedTags.value = []
  }

  function toggleTag(tag: string) {
    if (selectedTags.value.includes(tag)) {
      selectedTags.value = selectedTags.value.filter((value) => value !== tag)
      return
    }
    selectedTags.value = [...selectedTags.value, tag]
  }

  function clearSelected() {
    selectedIds.value = new Set()
  }

  function toggleSelectionMode() {
    if (selectionMode.value) {
      selectionMode.value = false
      clearSelected()
      return
    }
    selectionMode.value = true
  }

  function toggleSelected(shortcut: Shortcut) {
    if (!selectionMode.value) {
      return
    }
    const next = new Set(selectedIds.value)
    if (next.has(shortcut.id)) {
      next.delete(shortcut.id)
    } else {
      next.add(shortcut.id)
    }
    selectedIds.value = next
  }

  function selectedShortcutsInOrder(): Shortcut[] {
    return shortcuts.value.filter((shortcut) => selectedIds.value.has(shortcut.id))
  }

  function visibleShortcuts(): Shortcut[] {
    return filtering.value ? filteredShortcuts.value : shortcuts.value
  }

  function selectAllVisible() {
    if (!selectionMode.value || busy.value) {
      return
    }
    selectedIds.value = new Set(visibleShortcuts().map((shortcut) => shortcut.id))
  }

  function isEditableTarget(target: EventTarget | null) {
    if (!(target instanceof HTMLElement)) {
      return false
    }
    if (target.isContentEditable) {
      return true
    }
    const tag = target.tagName
    return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT'
  }

  function onSelectionKeydown(event: KeyboardEvent) {
    if (!selectionMode.value || busy.value) {
      return
    }
    if (!(event.ctrlKey || event.metaKey) || event.key.toLowerCase() !== 'a') {
      return
    }
    if (isEditableTarget(event.target)) {
      return
    }
    event.preventDefault()
    selectAllVisible()
  }

  async function load() {
    loadError.value = null
    shortcutTags.setShortcuts([])
    const result = await loadAction.run(async () => {
      shortcuts.value = await props.api.listShortcuts()
      shortcutTags.setShortcuts(shortcuts.value)
      selectedTags.value = selectedTags.value.filter((tag) => shortcutTags.tags.includes(tag))
      const availableIds = new Set(shortcuts.value.map((shortcut) => shortcut.id))
      selectedIds.value = new Set(
        [...selectedIds.value].filter((id) => availableIds.has(id)),
      )
    })
    if (!result.ok) {
      shortcuts.value = []
      selectedTags.value = []
      clearSelected()
    }
  }

  function rememberOrder() {
    previousOrder.value = [...shortcuts.value]
  }

  async function persistOrder() {
    if (
      reorderAction.running ||
      selectionMode.value ||
      previousOrder.value.length === 0 ||
      sameShortcutOrder(previousOrder.value, shortcuts.value)
    ) {
      previousOrder.value = []
      return
    }
    const previous = previousOrder.value
    const result = await reorderAction.run(async () => {
      shortcuts.value = await props.api.updateShortcutOrder({
        shortcut_ids: shortcuts.value.map((shortcut) => shortcut.id),
      })
    })
    if (!result.ok) {
      shortcuts.value = previous
      await load()
    }
    previousOrder.value = []
  }

  function sameShortcutOrder(left: Shortcut[], right: Shortcut[]) {
    return (
      left.length === right.length &&
      left.every((shortcut, index) => shortcut.id === right[index]?.id)
    )
  }

  function openCreate() {
    selected.value = null
    editorOpen.value = true
  }

  function openEdit(shortcut: Shortcut) {
    selected.value = shortcut
    editorOpen.value = true
  }

  function openDelete(shortcut: Shortcut) {
    selected.value = shortcut
    deleteOpen.value = true
  }

  function openBatchDelete() {
    if (selectedCount.value === 0 || busy.value) {
      return
    }
    batchDeleteOpen.value = true
  }

  async function copyCommand(shortcut: Shortcut) {
    await copyAction.run(async () => {
      await window.navigator.clipboard.writeText(shortcut.command)
      notifications.pushToast('success', t('toast.commandCopied'), shortcut.name)
    })
  }

  async function toggleEnabled(shortcut: Shortcut) {
    if (toggleAction.running) {
      return
    }
    const nextEnabled = shortcut.enabled === false
    await toggleAction.run(async () => {
      const updated = await props.api.updateShortcut(shortcut.id, {
        enabled: nextEnabled,
      } as UpdateShortcutReq)
      shortcuts.value = shortcuts.value.map((item) => (item.id === updated.id ? updated : item))
      shortcutTags.setShortcuts(shortcuts.value)
      notifications.pushToast(
        'success',
        nextEnabled ? t('toast.shortcutEnabled') : t('toast.shortcutDisabled'),
        updated.name,
      )
    })
  }

  async function save(value: {
    name: string
    command: string
    description?: string
    icon: string
    enabled: boolean
    tags: string[]
  }) {
    if (saveAction.running) {
      return
    }
    if (!value.name || !value.command.trim()) {
      notifications.pushToast('error', t('toast.createShortcutFailed'), t('message.nameRequired'))
      return
    }
    const editing = selected.value
    await saveAction.run(
      async () => {
        if (editing) {
          await props.api.updateShortcut(editing.id, value)
          notifications.pushToast('success', t('toast.shortcutUpdated'), value.name)
        } else {
          await props.api.createShortcut(value)
          notifications.pushToast('success', t('toast.shortcutCreated'), value.name)
        }
        await load()
        editorOpen.value = false
        selected.value = null
      },
      {
        onError: (err) =>
          notifications.notifyError(
            editing ? t('toast.updateShortcutFailed') : t('toast.createShortcutFailed'),
            err,
          ),
      },
    )
  }

  async function remove() {
    if (!selected.value || deleteAction.running) {
      return
    }
    const current = selected.value
    await deleteAction.run(async () => {
      const name = current.name
      await props.api.deleteShortcut(current.id)
      selectedIds.value = new Set([...selectedIds.value].filter((id) => id !== current.id))
      await load()
      deleteOpen.value = false
      selected.value = null
      notifications.pushToast('success', t('toast.shortcutDeleted'), name)
    })
  }

  async function removeSelected() {
    if (selectedCount.value === 0 || batchDeleteAction.running) {
      return
    }
    const targets = selectedShortcutsInOrder()
    if (targets.length === 0) {
      batchDeleteOpen.value = false
      return
    }
    await batchDeleteAction.run(async () => {
      let deleted = 0
      for (const shortcut of targets) {
        try {
          await props.api.deleteShortcut(shortcut.id)
          deleted += 1
        } catch (err) {
          await load()
          clearSelected()
          selectionMode.value = false
          batchDeleteOpen.value = false
          notifications.notifyError(
            t('toast.batchDeleteShortcutsPartial', {
              deleted,
              total: targets.length,
              name: shortcut.name,
            }),
            err,
          )
          return
        }
      }
      await load()
      clearSelected()
      selectionMode.value = false
      batchDeleteOpen.value = false
      notifications.pushToast(
        'success',
        t('toast.batchDeleteShortcutsSucceeded', { count: deleted }),
      )
    })
  }

  function exportSelected() {
    if (selectedCount.value === 0 || busy.value) {
      return
    }
    const items = selectedShortcutsInOrder().map(toShortcutExportItem)
    downloadJsonFile(formatShortcutExportFilename(), items)
    clearSelected()
    selectionMode.value = false
    notifications.pushToast('success', t('toast.exportShortcutsSucceeded', { count: items.length }))
  }

  function openImportPicker() {
    if (busy.value) {
      return
    }
    const input = importInput.value
    if (!input) {
      return
    }
    input.value = ''
    input.click()
  }

  function clearPendingImport() {
    pendingImportItems.value = []
    pendingImportName.value = ''
  }

  function onImportDialogOpenChange(open: boolean) {
    if (!open && importAction.running) {
      importOpen.value = true
      return
    }
    importOpen.value = open
    if (!open && !importAction.running) {
      clearPendingImport()
    }
  }

  async function onImportFileChange(event: Event) {
    const input = event.target as HTMLInputElement
    const file = input.files?.[0]
    input.value = ''
    if (!file || importAction.running || busy.value) {
      return
    }
    let text: string
    try {
      text = await file.text()
    } catch (err) {
      notifications.notifyError(t('toast.importShortcutsFailed'), err)
      return
    }
    const parsed = parseShortcutImportJson(text)
    if (!parsed.ok) {
      const detail =
        parsed.error === 'invalid_json'
          ? t('shortcut.importInvalidJson')
          : parsed.error === 'not_array'
            ? t('shortcut.importNotArray')
            : t('shortcut.importInvalidItem', { index: (parsed.index ?? 0) + 1 })
      notifications.pushToast('error', t('toast.importShortcutsFailed'), detail)
      return
    }
    if (parsed.items.length === 0) {
      notifications.pushToast('error', t('toast.importShortcutsFailed'), t('shortcut.importEmpty'))
      return
    }
    pendingImportItems.value = parsed.items
    pendingImportName.value = file.name
    importOpen.value = true
  }

  async function confirmImport() {
    if (pendingImportItems.value.length === 0 || importAction.running) {
      return
    }
    const items = [...pendingImportItems.value]
    const fileName = pendingImportName.value
    await importAction.run(async () => {
      let created = 0
      for (const item of items) {
        try {
          await props.api.createShortcut(toCreateShortcutRequest(item))
          created += 1
        } catch (err) {
          await load()
          clearPendingImport()
          importOpen.value = false
          notifications.notifyError(
            t('toast.importShortcutsPartial', {
              created,
              total: items.length,
              name: item.name,
            }),
            err,
          )
          return
        }
      }
      await load()
      clearPendingImport()
      importOpen.value = false
      notifications.pushToast(
        'success',
        t('toast.importShortcutsSucceeded', { count: created }),
        fileName,
      )
    })
  }

  onMounted(() => {
    window.addEventListener('keydown', onSelectionKeydown)
    void load()
  })

  onUnmounted(() => {
    window.removeEventListener('keydown', onSelectionKeydown)
  })
</script>
