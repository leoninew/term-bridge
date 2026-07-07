<template>
  <DialogRoot :open="open" @update:open="emit('update:open', $event)">
    <DialogPortal>
      <DialogOverlay class="dialog-overlay" />
      <DialogContent class="dialog-content">
        <DialogTitle class="dialog-title">{{ t('dialog.editSessionTitle') }}</DialogTitle>
        <form class="dialog-form" @submit.prevent="submit">
          <label>
            <span>{{ t('dialog.name') }}</span>
            <input
              ref="editInput"
              v-model="editText"
              :placeholder="t('dialog.sessionNamePlaceholder')"
            />
          </label>
          <div class="dialog-actions">
            <DialogClose as-child>
              <button type="button" class="button button-secondary">
                {{ t('common.cancel') }}
              </button>
            </DialogClose>
            <button type="submit" class="button button-primary" :disabled="editing">
              {{ editing ? t('common.editing') : t('common.edit') }}
            </button>
          </div>
        </form>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup lang="ts">
  import { nextTick, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import {
    DialogClose,
    DialogContent,
    DialogOverlay,
    DialogPortal,
    DialogRoot,
    DialogTitle,
  } from 'reka-ui'
  import type { SessionSummary } from '../../gen/proto/termbridge/agent/v1/workspace'

  const props = defineProps<{
    open: boolean
    session: SessionSummary | null
    editing: boolean
  }>()

  const emit = defineEmits<{
    'update:open': [open: boolean]
    submit: [name: string]
  }>()

  const { t } = useI18n()
  const editText = ref('')
  const editInput = ref<{ focus: () => void; select: () => void } | null>(null)

  watch(
    () => props.open,
    (open) => {
      if (!open || !props.session) {
        return
      }
      editText.value = props.session.name || props.session.command || ''
      void nextTick(() => {
        editInput.value?.focus()
        editInput.value?.select()
      })
    },
  )

  function submit() {
    emit('submit', editText.value.trim())
  }
</script>
