<template>
  <DialogRoot :open="open" @update:open="emit('update:open', $event)">
    <DialogPortal>
      <DialogOverlay class="dialog-overlay shortcut-dialog-overlay" />
      <DialogContent class="dialog-content shortcut-dialog-content">
        <DialogTitle class="dialog-title">
          {{ shortcut ? t('shortcut.editTitle') : t('shortcut.createTitle') }}
        </DialogTitle>

        <form class="dialog-form shortcut-editor-form" @submit.prevent="submit">
          <label>
            <span>{{ t('dialog.name') }}</span>
            <input v-model="name" :disabled="saving" autocomplete="off" />
          </label>
          <label>
            <span>{{ t('dialog.command') }}</span>
            <textarea
              v-model="command"
              class="shortcut-command-input"
              :disabled="saving"
              rows="1"
              spellcheck="false"
            />
          </label>
          <label>
            <span>{{ t('shortcut.tagsField') }}</span>
            <TagsInputRoot
              v-model="tags"
              class="flex min-h-9 w-full flex-wrap items-center gap-1 rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] px-2 py-1 transition-colors focus-within:border-[var(--color-border-strong)] focus-within:shadow-[0_0_0_3px_var(--color-surface-muted)] data-[disabled]:cursor-not-allowed data-[disabled]:opacity-70"
              :disabled="saving"
              add-on-blur
              add-on-paste
            >
              <TagsInputItem
                v-for="tag in tags"
                :key="tag"
                :value="tag"
                class="inline-flex max-w-full shrink-0 items-center gap-1 rounded bg-[var(--color-control-hover)] py-0.5 pl-1.5 pr-0.5 text-xs text-[var(--color-text-muted)] data-[state=active]:bg-[var(--color-control-active)]"
              >
                <TagsInputItemText class="min-w-0 truncate" />
                <TagsInputItemDelete
                  class="inline-flex size-4 shrink-0 items-center justify-center rounded text-[var(--color-text-subtle)] outline-none hover:bg-[var(--color-control-active)] hover:text-[var(--color-text)] focus-visible:bg-[var(--color-control-active)] focus-visible:text-[var(--color-text)]"
                  :aria-label="`${t('common.delete')} ${tag}`"
                >
                  <X class="size-3" aria-hidden="true" />
                </TagsInputItemDelete>
              </TagsInputItem>
              <TagsInputInput
                class="!min-h-6 !w-auto min-w-24 flex-1 basis-24 !border-0 !bg-transparent !p-0 !text-[var(--color-text)] !shadow-none outline-none placeholder:text-[var(--color-text-subtle)] focus:!border-0 focus:!shadow-none"
                :placeholder="t('shortcut.tagsPlaceholder')"
                autocomplete="off"
              />
            </TagsInputRoot>
          </label>
          <div class="flex items-center gap-2">
            <CheckboxRoot
              id="shortcut-enabled"
              v-model="enabled"
              class="inline-flex size-4 shrink-0 items-center justify-center rounded border border-[var(--color-border-strong)] text-[var(--color-text)] outline-none hover:bg-[var(--color-control-hover)] focus-visible:shadow-[0_0_0_3px_var(--color-surface-muted)] data-[state=checked]:border-[var(--color-primary-border)] data-[state=checked]:bg-[var(--color-primary-border)] data-[state=checked]:text-white disabled:cursor-not-allowed disabled:opacity-70"
              :disabled="saving"
            >
              <CheckboxIndicator class="inline-flex items-center justify-center">
                <Check class="size-3" aria-hidden="true" />
              </CheckboxIndicator>
            </CheckboxRoot>
            <label for="shortcut-enabled" class="text-sm font-medium text-[var(--color-text)]">
              {{ t('shortcut.enabledField') }}
            </label>
          </div>
          <label>
            <span>{{ t('shortcut.descriptionField') }}</span>
            <textarea v-model="description" :disabled="saving" rows="1" />
          </label>

          <div class="dialog-actions">
            <DialogClose as-child>
              <button
                type="button"
                class="button button-secondary bg-[var(--color-surface-raised)]"
                :disabled="saving"
              >
                {{ t('common.cancel') }}
              </button>
            </DialogClose>
            <button type="submit" class="button button-primary" :disabled="saving">
              {{ t('common.confirm') }}
            </button>
          </div>
        </form>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup lang="ts">
  import { Check, X } from '@lucide/vue'
  import { ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import {
    CheckboxIndicator,
    CheckboxRoot,
    DialogClose,
    DialogContent,
    DialogOverlay,
    DialogPortal,
    DialogRoot,
    DialogTitle,
    TagsInputInput,
    TagsInputItem,
    TagsInputItemDelete,
    TagsInputItemText,
    TagsInputRoot,
  } from 'reka-ui'
  import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'

  const props = defineProps<{ open: boolean; shortcut: Shortcut | null; saving: boolean }>()
  const emit = defineEmits<{
    'update:open': [open: boolean]
    submit: [
      value: {
        name: string
        command: string
        description?: string
        icon: string
        enabled: boolean
        tags: string[]
      },
    ]
  }>()
  const { t } = useI18n()
  const name = ref('')
  const command = ref('')
  const description = ref('')
  const tags = ref<string[]>([])
  const enabled = ref(true)

  watch(
    () => props.open,
    (open) => {
      if (!open) {
        return
      }
      name.value = props.shortcut?.name ?? ''
      command.value = props.shortcut?.command ?? ''
      description.value = props.shortcut?.description ?? ''
      tags.value = props.shortcut?.tags ?? []
      enabled.value = props.shortcut?.enabled ?? true
    },
  )

  function submit() {
    emit('submit', {
      name: name.value.trim(),
      command: command.value,
      description: description.value,
      icon: '',
      tags: tags.value,
      enabled: enabled.value,
    })
  }
</script>
