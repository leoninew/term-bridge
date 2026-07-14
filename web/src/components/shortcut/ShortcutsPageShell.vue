<template>
  <section class="shortcut-page min-h-screen bg-[var(--color-app-bg)] text-[var(--color-text)]">
    <AppHeader />

    <main class="shortcut-page-main min-h-[calc(100vh-4rem)]">
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
              class="button button-primary shortcut-create-button inline-flex items-center gap-1"
              @click="openCreate"
            >
              <Plus class="size-4" aria-hidden="true" />
              {{ t('shortcut.createTitle') }}
            </button>
            <button
              type="button"
              class="button button-secondary shortcut-return-button"
              @click="props.returnToWorkspace"
            >
              {{ t('common.returnToWorkspace') }}
            </button>
          </div>
        </header>

        <p
          v-if="loading"
          class="shortcut-loading text-center text-sm text-[var(--color-text-muted)]"
        >
          {{ t('shortcut.loading') }}
        </p>

        <VueDraggable
          v-if="shortcuts.length && !filtering"
          v-model="shortcuts"
          tag="div"
          class="shortcut-grid"
          item-key="id"
          :animation="150"
          :disabled="reordering"
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
            :class="!reordering ? 'shortcut-card-sortable' : ''"
            @edit="openEdit"
            @delete="openDelete"
            @copy="copyCommand"
            @toggle-enabled="toggleEnabled"
          />
        </VueDraggable>

        <div v-else-if="filteredShortcuts.length" class="shortcut-grid">
          <ShortcutCard
            v-for="shortcut in filteredShortcuts"
            :key="shortcut.id"
            :shortcut="shortcut"
            @edit="openEdit"
            @delete="openDelete"
            @copy="copyCommand"
            @toggle-enabled="toggleEnabled"
          />
        </div>

        <section
          v-else-if="shortcuts.length === 0"
          class="shortcut-empty border border-dashed border-[var(--color-border-strong)] bg-[var(--color-surface)] text-center text-sm text-[var(--color-text-muted)]"
        >
          {{ t('shortcut.empty') }}
        </section>

        <section
          v-else
          class="shortcut-empty border border-dashed border-[var(--color-border-strong)] bg-[var(--color-surface)] text-center text-sm text-[var(--color-text-muted)]"
        >
          {{ t('shortcut.noFilterMatches') }}
        </section>
      </section>
    </main>
  </section>

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
</template>

<script setup lang="ts">
  import { Plus } from '@lucide/vue'
  import { computed, onMounted, ref } from 'vue'
  import { storeToRefs } from 'pinia'
  import { VueDraggable } from 'vue-draggable-plus'
  import { useI18n } from 'vue-i18n'
  import type { Shortcut, UpdateShortcutReq } from '../../gen/proto/termbridge/agent/v1/shortcut'
  import type { ShortcutRuntimeApi } from '../../features/sessions/runtime'
  import { useNotificationsStore } from '../../store/notifications'
  import { useShortcutTagsStore } from '../../store/shortcutTags'
  import AppHeader from '../layout/AppHeader.vue'
  import DeleteShortcutDialog from './DeleteShortcutDialog.vue'
  import ShortcutCard from './ShortcutCard.vue'
  import ShortcutEditorDialog from './ShortcutEditorDialog.vue'

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
  const loading = ref(false)
  const saving = ref(false)
  const deleting = ref(false)
  const reordering = ref(false)
  const previousOrder = ref<Shortcut[]>([])
  const editorOpen = ref(false)
  const deleteOpen = ref(false)
  const selected = ref<Shortcut | null>(null)

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

  async function load() {
    loading.value = true
    shortcutTags.setShortcuts([])
    try {
      shortcuts.value = await props.api.listShortcuts()
      shortcutTags.setShortcuts(shortcuts.value)
      selectedTags.value = selectedTags.value.filter((tag) => shortcutTags.tags.includes(tag))
    } catch (err) {
      shortcuts.value = []
      selectedTags.value = []
      notifications.notifyError(t('toast.loadShortcutsFailed'), err)
    } finally {
      loading.value = false
    }
  }

  function rememberOrder() {
    previousOrder.value = [...shortcuts.value]
  }

  async function persistOrder() {
    if (
      reordering.value ||
      previousOrder.value.length === 0 ||
      sameShortcutOrder(previousOrder.value, shortcuts.value)
    ) {
      previousOrder.value = []
      return
    }
    const previous = previousOrder.value
    reordering.value = true
    try {
      shortcuts.value = await props.api.updateShortcutOrder({
        shortcut_ids: shortcuts.value.map((shortcut) => shortcut.id),
      })
    } catch (err) {
      shortcuts.value = previous
      notifications.notifyError(t('toast.updateShortcutOrderFailed'), err)
      await load()
    } finally {
      previousOrder.value = []
      reordering.value = false
    }
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

  async function copyCommand(shortcut: Shortcut) {
    try {
      await window.navigator.clipboard.writeText(shortcut.command)
      notifications.pushToast('success', t('toast.commandCopied'), shortcut.name)
    } catch (err) {
      notifications.notifyError(t('toast.copyCommandFailed'), err)
    }
  }

  async function toggleEnabled(shortcut: Shortcut) {
    const nextEnabled = shortcut.enabled === false
    try {
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
    } catch (err) {
      notifications.notifyError(t('toast.updateShortcutFailed'), err)
    }
  }

  async function save(value: {
    name: string
    command: string
    description?: string
    icon: string
    enabled: boolean
    tags: string[]
  }) {
    if (!value.name || !value.command.trim()) {
      notifications.pushToast('error', t('toast.createShortcutFailed'), t('message.nameRequired'))
      return
    }
    saving.value = true
    try {
      if (selected.value) {
        await props.api.updateShortcut(selected.value.id, value)
        notifications.pushToast('success', t('toast.shortcutUpdated'), value.name)
      } else {
        await props.api.createShortcut(value)
        notifications.pushToast('success', t('toast.shortcutCreated'), value.name)
      }
      await load()
      editorOpen.value = false
      selected.value = null
    } catch (err) {
      notifications.notifyError(
        selected.value ? t('toast.updateShortcutFailed') : t('toast.createShortcutFailed'),
        err,
      )
    } finally {
      saving.value = false
    }
  }

  async function remove() {
    if (!selected.value || deleting.value) {
      return
    }
    deleting.value = true
    try {
      const name = selected.value.name
      await props.api.deleteShortcut(selected.value.id)
      await load()
      deleteOpen.value = false
      selected.value = null
      notifications.pushToast('success', t('toast.shortcutDeleted'), name)
    } catch (err) {
      notifications.notifyError(t('toast.deleteShortcutFailed'), err)
    } finally {
      deleting.value = false
    }
  }

  onMounted(load)
</script>
