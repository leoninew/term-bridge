<template>
  <DialogRoot :open="open" @update:open="emit('update:open', $event)"
    ><DialogPortal
      ><DialogOverlay class="dialog-overlay" /><DialogContent class="dialog-content"
        ><DialogTitle class="dialog-title">{{ t('dialog.editSessionTitle') }}</DialogTitle>
        <form class="dialog-form" @submit.prevent="submit">
          <SessionFormFields
            :cwd="cwd"
            cwd-readonly
            :name="name"
            :command="command"
            :command-source="commandSource"
            :selected-shortcut-id="selectedShortcutId"
            :shortcuts="shortcuts"
            :disabled="editing"
            @update:cwd="cwd = $event"
            @update:name="name = $event"
            @update:command="command = $event"
            @update:command-source="selectCommandSource"
            @update:selected-shortcut-id="selectShortcut"
          />
          <div class="dialog-actions">
            <DialogClose as-child
              ><button
                type="button"
                class="button button-secondary bg-[var(--color-surface-raised)]"
              >
                {{ t('common.cancel') }}
              </button></DialogClose
            ><button type="submit" class="button button-primary" :disabled="editing">
              {{ t('common.confirm') }}
            </button>
          </div>
        </form></DialogContent
      ></DialogPortal
    ></DialogRoot
  >
</template>
<script setup lang="ts">
  import { ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import {
    DialogClose,
    DialogContent,
    DialogOverlay,
    DialogPortal,
    DialogRoot,
    DialogTitle,
  } from 'reka-ui'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'
  import type { SessionSummary } from '../../gen/proto/termbridge/agent/v1/workspace'
  import type { CommandSource } from '../../composable/useCreateSessionDraft'
  import SessionFormFields from './SessionFormFields.vue'
  const props = defineProps<{
    open: boolean
    session: SessionSummary | null
    shortcuts: Shortcut[]
    editing: boolean
  }>()
  const emit = defineEmits<{
    'update:open': [open: boolean]
    submit: [payload: { name: string; command: string }]
  }>()
  const { t } = useI18n()
  const name = ref('')
  const command = ref('')
  const cwd = ref('')
  const commandSource = ref<CommandSource>('command')
  const selectedShortcutId = ref<string | null>(null)
  watch(
    () => props.open,
    (open) => {
      if (!open || !props.session) return
      name.value = props.session.name || props.session.command
      command.value = props.session.command
      cwd.value = props.session.cwd
      commandSource.value = 'command'
      selectedShortcutId.value = null
    },
  )
  function selectCommandSource(source: CommandSource) {
    commandSource.value = source
    if (source === 'command') return

    selectShortcut(
      props.shortcuts.find((shortcut) => shortcut.id === selectedShortcutId.value)?.id ??
        props.shortcuts[0]?.id ??
        null,
    )
  }

  function selectShortcut(id: string | null) {
    const shortcut = props.shortcuts.find((value) => value.id === id)
    if (!shortcut) return
    selectedShortcutId.value = shortcut.id
    command.value = shortcut.command
  }
  function submit() {
    emit('submit', { name: name.value.trim(), command: command.value })
  }
</script>
