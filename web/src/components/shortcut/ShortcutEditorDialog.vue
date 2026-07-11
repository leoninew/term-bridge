<template>
  <DialogRoot :open="open" @update:open="emit('update:open', $event)">
    <DialogPortal>
      <DialogOverlay class="dialog-overlay" />
      <DialogContent class="dialog-content shortcut-dialog-content">
        <div class="dialog-header">
          <DialogTitle class="dialog-title">
            {{ shortcut ? t('shortcut.editTitle') : t('shortcut.createTitle') }}
          </DialogTitle>
          <p class="dialog-description">{{ t('shortcut.description') }}</p>
        </div>

        <form class="dialog-form shortcut-editor-form" @submit.prevent="submit">
          <label>
            <span>{{ t('dialog.name') }}</span>
            <input v-model="name" :disabled="saving" autocomplete="off" />
          </label>
          <label>
            <span>{{ t('dialog.command') }}</span>
            <textarea v-model="command" :disabled="saving" rows="4" spellcheck="false" />
          </label>
          <label>
            <span>{{ t('shortcut.descriptionField') }}</span>
            <textarea v-model="description" :disabled="saving" rows="3" />
          </label>

          <div class="dialog-actions">
            <DialogClose as-child>
              <button type="button" class="button button-secondary" :disabled="saving">
                {{ t('common.cancel') }}
              </button>
            </DialogClose>
            <button type="submit" class="button button-primary" :disabled="saving">
              {{ saving ? t('common.editing') : shortcut ? t('common.edit') : t('common.create') }}
            </button>
          </div>
        </form>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
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

  const props = defineProps<{ open: boolean; shortcut: Shortcut | null; saving: boolean }>()
  const emit = defineEmits<{
    'update:open': [open: boolean]
    submit: [value: { name: string; command: string; description?: string }]
  }>()
  const { t } = useI18n()
  const name = ref('')
  const command = ref('')
  const description = ref('')

  watch(
    () => props.open,
    (open) => {
      if (!open) {
        return
      }
      name.value = props.shortcut?.name ?? ''
      command.value = props.shortcut?.command ?? ''
      description.value = props.shortcut?.description ?? ''
    },
  )

  function submit() {
    emit('submit', { name: name.value.trim(), command: command.value, description: description.value })
  }
</script>
