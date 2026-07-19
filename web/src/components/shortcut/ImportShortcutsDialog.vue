<template>
  <DialogRoot :open="open" @update:open="emit('update:open', $event)">
    <DialogPortal>
      <DialogOverlay class="dialog-overlay" />
      <DialogContent class="dialog-content w-[min(448px,calc(100vw-32px))]">
        <div class="dialog-header">
          <DialogTitle class="dialog-title">
            {{ t('shortcut.importConfirmTitle') }}
          </DialogTitle>
          <DialogDescription class="dialog-description">
            {{ t('shortcut.importConfirmDescription', { count, name }) }}
          </DialogDescription>
        </div>

        <div class="dialog-actions">
          <DialogClose as-child>
            <button type="button" class="button button-secondary" :disabled="importing">
              {{ t('common.cancel') }}
            </button>
          </DialogClose>
          <button
            type="button"
            class="button button-primary"
            :disabled="importing || count === 0"
            @click="emit('confirm')"
          >
            {{ importing ? t('shortcut.importing') : t('shortcut.import') }}
          </button>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import {
    DialogClose,
    DialogContent,
    DialogDescription,
    DialogOverlay,
    DialogPortal,
    DialogRoot,
    DialogTitle,
  } from 'reka-ui'

  defineProps<{ open: boolean; count: number; name: string; importing: boolean }>()
  const emit = defineEmits<{ 'update:open': [open: boolean]; confirm: [] }>()
  const { t } = useI18n()
</script>
