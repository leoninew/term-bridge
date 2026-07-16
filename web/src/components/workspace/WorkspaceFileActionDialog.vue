<template>
  <DialogRoot :open="open" @update:open="emit('update:open', $event)">
    <DialogPortal>
      <DialogOverlay class="dialog-overlay" />
      <DialogContent class="dialog-content">
        <DialogTitle class="dialog-title">{{ title }}</DialogTitle>
        <p class="dialog-description">{{ description }}</p>
        <form v-if="requiresValue" class="dialog-form" @submit.prevent="emit('confirm', value)">
          <label>
            {{ label }}
            <input v-model="value" :placeholder="placeholder" autofocus />
          </label>
          <div class="dialog-actions">
            <DialogClose as-child
              ><button type="button" class="button button-secondary">
                {{ t('common.cancel') }}
              </button></DialogClose
            >
            <button type="submit" class="button button-primary" :disabled="!value.trim()">
              {{ t('common.confirm') }}
            </button>
          </div>
        </form>
        <div v-else class="dialog-actions">
          <DialogClose as-child
            ><button type="button" class="button button-secondary">
              {{ t('common.cancel') }}
            </button></DialogClose
          >
          <button
            type="button"
            class="button"
            :class="danger ? 'button-danger' : 'button-primary'"
            @click="emit('confirm', '')"
          >
            {{ t('common.confirm') }}
          </button>
        </div>
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

  const props = defineProps<{
    open: boolean
    title: string
    description: string
    label: string
    placeholder: string
    requiresValue: boolean
    danger?: boolean
    initialValue?: string
  }>()
  const emit = defineEmits<{
    'update:open': [open: boolean]
    confirm: [value: string]
  }>()
  const { t } = useI18n()
  const value = ref('')
  watch(
    () => props.open,
    (open) => {
      if (open) value.value = props.initialValue ?? ''
    },
    { immediate: true },
  )
</script>
