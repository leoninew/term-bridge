<template>
  <TabsRoot
    :model-value="activeDocument?.key ?? undefined"
    activation-mode="manual"
    class="file-workbench-tabs-root"
    @update:model-value="activateDocument"
  >
    <TabsList class="file-workbench-tabs" :aria-label="t('files.openDocuments')">
      <div v-for="document in documents" :key="document.key" class="file-workbench-tab-shell">
        <TabsTrigger :value="document.key" class="file-workbench-tab">
          <span class="min-w-0 flex-1 truncate">{{ entryName(document.path) }}</span>
          <span v-if="document.dirty" class="file-workbench-tab-dirty" aria-hidden="true">●</span>
          <span v-if="document.loading || document.saving" class="text-xs" aria-hidden="true"
            >…</span
          >
        </TabsTrigger>
        <button
          type="button"
          class="file-workbench-tab-close"
          :aria-label="t('files.closeDocumentAria', { name: entryName(document.path) })"
          :title="t('files.closeDocumentAria', { name: entryName(document.path) })"
          @click.stop="emit('close', document.path)"
        >
          <X class="size-3.5" aria-hidden="true" />
        </button>
      </div>
    </TabsList>
    <TabsContent v-if="activeDocument" :value="activeDocument.key" class="file-workbench-tab-panel">
      <slot />
    </TabsContent>
    <slot v-else />
  </TabsRoot>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import { X } from '@lucide/vue'
  import { TabsContent, TabsList, TabsRoot, TabsTrigger } from 'reka-ui'
  import { entryName } from '../../features/files/workbenchUi'
  import type { FileDocumentState } from '../../store/fileWorkbench'

  const props = defineProps<{
    documents: FileDocumentState[]
    activeDocument: FileDocumentState | null
  }>()
  const emit = defineEmits<{
    activate: [path: string]
    close: [path: string]
  }>()

  const { t } = useI18n()

  function activateDocument(key: string | number) {
    const document = props.documents.find((item) => item.key === key)
    if (document) emit('activate', document.path)
  }
</script>
