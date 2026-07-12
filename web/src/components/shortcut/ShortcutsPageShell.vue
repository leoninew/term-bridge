<template>
  <ToastProvider>
    <section class="shortcut-page min-h-screen bg-[var(--color-app-bg)] text-[var(--color-text)]">
      <AppHeader />

      <main class="shortcut-page-main min-h-[calc(100vh-4rem)]">
        <section class="shortcut-page-content mx-auto flex w-full max-w-[1200px] flex-col">
          <header class="shortcut-page-header flex items-center justify-between">
            <h1 class="shortcut-page-title font-semibold text-[var(--color-text-strong)]">
              {{ t('shortcut.title') }}
            </h1>
            <button
              type="button"
              class="button button-primary shortcut-create-button shrink-0"
              @click="openCreate"
            >
              {{ t('shortcut.createTitle') }}
            </button>
          </header>

          <p v-if="loading" class="shortcut-loading text-center text-[var(--color-text-muted)]">
            {{ t('shortcut.loading') }}
          </p>

          <div
            v-else-if="shortcuts.length"
            class="shortcut-grid grid sm:grid-cols-2 lg:grid-cols-4"
          >
            <ShortcutCard
              v-for="shortcut in shortcuts"
              :key="shortcut.id"
              :shortcut="shortcut"
              @edit="openEdit"
              @delete="openDelete"
            />
          </div>

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
    <ToastHost />
  </ToastProvider>
</template>

<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ToastProvider } from 'reka-ui'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'
  import type { ShortcutRuntimeApi } from '../../features/sessions/runtime'
  import { useNotificationsStore } from '../../store/notifications'
  import AppHeader from '../layout/AppHeader.vue'
  import ToastHost from '../session/ToastHost.vue'
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
