<template>
  <AlertDialogRoot :open="open" @update:open="emit('update:open', $event)">
    <AlertDialogPortal>
      <AlertDialogOverlay class="dialog-overlay" />
      <AlertDialogContent class="dialog-content">
        <AlertDialogTitle class="dialog-title">{{ title }}</AlertDialogTitle>
        <AlertDialogDescription class="dialog-description">{{
          description
        }}</AlertDialogDescription>
        <form v-if="requiresValue" class="dialog-form" @submit.prevent="confirmValue">
          <label>
            {{ label }}
            <input v-model="value" :disabled="pending" :placeholder="placeholder" autofocus />
          </label>
          <div class="dialog-actions">
            <AlertDialogCancel as-child>
              <button type="button" class="button button-secondary" :disabled="pending">
                {{ t('common.cancel') }}
              </button>
            </AlertDialogCancel>
            <button
              type="submit"
              class="button"
              :class="buttonClass"
              :disabled="pending || !value.trim()"
            >
              {{ pending ? t('files.gitOperationRunning') : confirmLabel }}
            </button>
          </div>
        </form>
        <div v-else class="dialog-actions">
          <AlertDialogCancel as-child>
            <button type="button" class="button button-secondary" :disabled="pending">
              {{ t('common.cancel') }}
            </button>
          </AlertDialogCancel>
          <button
            type="button"
            class="button"
            :class="buttonClass"
            :disabled="pending"
            @click="emit('confirm', '')"
          >
            {{ pending ? t('files.gitOperationRunning') : confirmLabel }}
          </button>
        </div>
      </AlertDialogContent>
    </AlertDialogPortal>
  </AlertDialogRoot>
</template>

<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
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

  const props = withDefaults(
    defineProps<{
      open: boolean
      title: string
      description: string
      label?: string
      placeholder?: string
      requiresValue?: boolean
      danger?: boolean
      pending?: boolean
      confirmLabel: string
      initialValue?: string
    }>(),
    {
      label: '',
      placeholder: '',
      requiresValue: false,
      danger: false,
      pending: false,
      initialValue: '',
    },
  )
  const emit = defineEmits<{
    'update:open': [open: boolean]
    confirm: [value: string]
  }>()
  const { t } = useI18n()
  const value = ref('')
  const buttonClass = computed(() => (props.danger ? 'button-danger' : 'button-primary'))

  watch(
    () => props.open,
    (open) => {
      if (open) {
        value.value = props.initialValue
      }
    },
    { immediate: true },
  )

  function confirmValue() {
    if (!props.pending && value.value.trim()) {
      emit('confirm', value.value)
    }
  }
</script>
