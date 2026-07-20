<template>
  <DialogRoot :open="open" @update:open="emit('update:open', $event)">
    <DialogPortal>
      <DialogOverlay class="dialog-overlay" />
      <DialogContent class="dialog-content">
        <DialogTitle class="dialog-title">{{ t('dialog.newSessionTitle') }}</DialogTitle>
        <form class="dialog-form" @submit.prevent="emit('submit')">
          <SessionFormFields
            :cwd="cwd"
            :name="name"
            :command="command"
            :command-source="commandSource"
            :selected-shortcut-id="selectedShortcutId"
            :selected-shortcut-name="selectedShortcutName"
            :shortcuts="shortcuts"
            :disabled="creating"
            @update:cwd="emit('update:cwd', $event)"
            @update:name="emit('update:name', $event)"
            @update:command="emit('update:command', $event)"
            @update:command-source="emit('update:commandSource', $event)"
            @update:selected-shortcut-id="emit('update:selectedShortcutId', $event)"
          />
          <div class="dialog-actions">
            <DialogClose as-child>
              <button type="button" class="button button-secondary" :disabled="creating">
                {{ t('common.cancel') }}
              </button>
            </DialogClose>
            <button type="submit" class="button button-primary" :disabled="creating">
              {{ t('common.confirm') }}
            </button>
          </div>
        </form>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup lang="ts">
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
  import type { CommandSource } from '../../composable/useCreateSessionDraft'
  import SessionFormFields from './SessionFormFields.vue'

  defineProps<{
    open: boolean
    cwd: string
    name: string
    command: string
    commandSource: CommandSource
    selectedShortcutId: string | null
    selectedShortcutName: string | null
    shortcuts: Shortcut[]
    creating: boolean
  }>()

  const emit = defineEmits<{
    'update:open': [open: boolean]
    'update:cwd': [value: string]
    'update:name': [value: string]
    'update:command': [value: string]
    'update:commandSource': [value: CommandSource]
    'update:selectedShortcutId': [value: string | null]
    submit: []
  }>()

  const { t } = useI18n()
</script>
