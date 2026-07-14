<template>
  <section class="shortcut-page min-h-screen bg-[var(--color-app-bg)] text-[var(--color-text)]">
    <AppHeader />

    <main class="shortcut-page-main min-h-[calc(100vh-4rem)]">
      <section class="shortcut-page-content mx-auto flex w-full max-w-[1440px] flex-col">
        <header class="shortcut-page-header flex items-center justify-between">
          <h1 class="shortcut-page-title font-semibold text-[var(--color-text-strong)]">
            {{ t('shortcut.title') }}
          </h1>
          <button
            type="button"
            class="button button-primary shortcut-create-button inline-flex shrink-0 items-center gap-1"
            @click="openCreate"
          >
            <Plus class="size-4" aria-hidden="true" />
            {{ t('common.create') }}
          </button>
        </header>

        <p v-if="loading" class="shortcut-loading text-center text-[var(--color-text-muted)]">
          {{ t('shortcut.loading') }}
        </p>

        <VueDraggable
          v-else-if="shortcuts.length"
          v-model="shortcuts"
          tag="div"
          class="shortcut-grid grid sm:grid-cols-2 lg:grid-cols-4"
          item-key="id"
          :animation="150"
          :disabled="reordering"
          :filter="'button'"
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
            @edit="openEdit"
            @delete="openDelete"
          />
        </VueDraggable>

        <section
          v-else
          class="shortcut-empty border border-dashed border-[var(--color-border-strong)] bg-[var(--color-surface)] text-center text-[var(--color-text-muted)]"
        >
          {{ t('shortcut.empty') }}
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
  import { onMounted, ref } from 'vue'
  import { VueDraggable } from 'vue-draggable-plus'
  import { useI18n } from 'vue-i18n'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'
  import type { ShortcutRuntimeApi } from '../../features/sessions/runtime'
  import { useNotificationsStore } from '../../store/notifications'
  import AppHeader from '../layout/AppHeader.vue'
  import DeleteShortcutDialog from './DeleteShortcutDialog.vue'
  import ShortcutCard from './ShortcutCard.vue'
  import ShortcutEditorDialog from './ShortcutEditorDialog.vue'

  const props = defineProps<{ api: ShortcutRuntimeApi }>()
  const { t } = useI18n()
  const notifications = useNotificationsStore()
  const shortcuts = ref<Shortcut[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const deleting = ref(false)
  const reordering = ref(false)
  const previousOrder = ref<Shortcut[]>([])
  const editorOpen = ref(false)
  const deleteOpen = ref(false)
  const selected = ref<Shortcut | null>(null)

  async function load() {
    loading.value = true
    try {
      shortcuts.value = await props.api.listShortcuts()
    } catch (err) {
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

  async function save(value: { name: string; command: string; description?: string }) {
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
