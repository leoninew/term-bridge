<template>
  <AlertDialogRoot :open="open" @update:open="onOpenChange">
    <AlertDialogPortal>
      <AlertDialogOverlay class="dialog-overlay" />
      <AlertDialogContent class="dialog-content">
        <div class="dialog-header">
          <AlertDialogTitle class="dialog-title">{{
            t('dialog.deleteDeviceTitle')
          }}</AlertDialogTitle>
          <AlertDialogDescription class="dialog-description">
            {{
              t('dialog.deleteDeviceDescription', {
                name: device?.name || t('dashboard.devicesTitle'),
              })
            }}
          </AlertDialogDescription>
        </div>
        <div class="dialog-actions">
          <AlertDialogCancel as-child>
            <button type="button" class="button button-secondary" :disabled="deleting">
              {{ t('common.cancel') }}
            </button>
          </AlertDialogCancel>
          <button
            type="button"
            class="button button-danger"
            :disabled="!canDelete"
            @click="emit('confirm')"
          >
            {{ deleting ? t('common.deleting') : t('common.delete') }}
          </button>
        </div>
      </AlertDialogContent>
    </AlertDialogPortal>
  </AlertDialogRoot>
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import {
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogOverlay,
    AlertDialogPortal,
    AlertDialogRoot,
    AlertDialogTitle,
  } from 'reka-ui'
  import type { DeviceSummary } from '../../gen/proto/termbridge/cloud/v1/device'

  const props = defineProps<{
    open: boolean
    device: DeviceSummary | null
    deleting: boolean
  }>()

  const emit = defineEmits<{
    'update:open': [open: boolean]
    confirm: []
  }>()

  const { t } = useI18n()

  const canDelete = computed(() => !!props.device && !props.device.online && !props.deleting)

  function onOpenChange(nextOpen: boolean) {
    if (props.deleting && !nextOpen) {
      return
    }
    emit('update:open', nextOpen)
  }
</script>
