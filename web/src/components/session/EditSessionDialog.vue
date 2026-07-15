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
            :selected-shortcut-name="selectedShortcutName"
            :shortcuts="shortcuts"
            :disabled="editing"
            @update:cwd="cwd = $event"
            @update:name="name = $event"
            @update:command="updateCommand"
            @update:command-source="selectCommandSource"
            @update:selected-shortcut-id="selectShortcut"
          />
          <div class="dialog-actions">
            <DialogClose as-child
              ><button type="button" class="button button-secondary">
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
    submit: [
      payload: {
        name: string
        command: string
        command_source?: CommandSource
        shortcut_id_snapshot?: string
        shortcut_name_snapshot?: string
      },
    ]
  }>()
  const { t } = useI18n()
  const name = ref('')
  const command = ref('')
  const cwd = ref('')
  const commandSource = ref<CommandSource>('command')
  const selectedShortcutId = ref<string | null>(null)
  const selectedShortcutName = ref<string | null>(null)
  const commandChanged = ref(false)
  watch(
    () => props.open,
    (open) => {
      if (!open || !props.session) return
      name.value = props.session.name || props.session.command
      command.value = props.session.command
      cwd.value = props.session.cwd
      commandSource.value = props.session.command_source === 'shortcut' ? 'shortcut' : 'command'
      selectedShortcutId.value = props.session.shortcut_id_snapshot || null
      selectedShortcutName.value = props.session.shortcut_name_snapshot || null
      commandChanged.value = false
    },
  )
  function selectCommandSource(source: CommandSource) {
    commandSource.value = source
    commandChanged.value = true
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
    selectedShortcutName.value = shortcut.name
    command.value = shortcut.command
    commandChanged.value = true
  }

  function updateCommand(value: string) {
    command.value = value
    commandChanged.value = true
  }

  function submit() {
    emit('submit', {
      name: name.value.trim(),
      command: command.value,
      ...(commandChanged.value
        ? {
            command_source: commandSource.value,
            shortcut_id_snapshot:
              commandSource.value === 'shortcut' ? (selectedShortcutId.value ?? '') : '',
            shortcut_name_snapshot:
              commandSource.value === 'shortcut' ? (selectedShortcutName.value ?? '') : '',
          }
        : {}),
    })
  }
</script>
