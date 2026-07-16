<template>
  <TabsRoot
    :model-value="activeDocument?.key ?? undefined"
    activation-mode="manual"
    class="file-workbench-tabs-root"
    @update:model-value="activateDocument"
  >
    <div class="file-workbench-tab-toolbar">
      <TabsList as-child>
        <div class="file-workbench-tabs" :aria-label="t('files.openDocuments')">
          <div v-for="document in documents" :key="document.key" class="file-workbench-tab-shell">
            <TabsTrigger :value="document.key" class="file-workbench-tab">
              <span class="file-workbench-tab-status" aria-hidden="true">
                <span v-if="document.dirty" class="file-workbench-tab-dirty">●</span>
                <WorkbenchCodicon
                  v-else-if="document.loading || document.saving"
                  name="loading"
                  :spin="true"
                  class-name="file-workbench-tab-loading"
                />
              </span>
              <span
                :ref="(element) => setTabTitleElement(document.key, element)"
                class="file-workbench-tab-title"
                :title="
                  truncatedDocumentKeys.has(document.key) ? entryName(document.path) : undefined
                "
              >
                {{ entryName(document.path) }}
              </span>
              <span class="file-workbench-tab-status" aria-hidden="true" />
            </TabsTrigger>
            <button
              type="button"
              class="file-workbench-tab-close"
              :disabled="document.saving"
              :aria-label="
                document.saving
                  ? t('files.closeDocumentSavingAria', { name: entryName(document.path) })
                  : t('files.closeDocumentAria', { name: entryName(document.path) })
              "
              :title="
                document.saving
                  ? t('files.closeDocumentSavingAria', { name: entryName(document.path) })
                  : t('files.closeDocumentAria', { name: entryName(document.path) })
              "
              @click.stop="emit('close', document.path)"
            >
              <WorkbenchCodicon name="close" class-name="file-workbench-codicon" />
            </button>
          </div>
        </div>
      </TabsList>
      <DropdownMenuRoot>
        <DropdownMenuTrigger
          class="file-workbench-tab-menu-trigger"
          :aria-label="t('files.tabMenuAria')"
          :title="t('files.tabMenuAria')"
        >
          <WorkbenchCodicon name="ellipsis" class-name="file-workbench-codicon" />
        </DropdownMenuTrigger>
        <DropdownMenuPortal>
          <DropdownMenuContent align="end" :side-offset="6" class="file-workbench-tab-menu-content">
            <DropdownMenuItem
              :disabled="!canCloseAll"
              :title="t('files.closeAllDocumentsDescription')"
              class="file-workbench-tab-menu-item"
              @click="emit('closeAll')"
            >
              {{ t('files.closeAllDocuments') }}
            </DropdownMenuItem>
            <DropdownMenuItem
              :disabled="!canCloseOthers"
              :title="t('files.closeOtherDocumentsDescription')"
              class="file-workbench-tab-menu-item"
              @click="emit('closeOthers')"
            >
              {{ t('files.closeOtherDocuments') }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenuPortal>
      </DropdownMenuRoot>
    </div>
    <TabsContent v-if="activeDocument" :value="activeDocument.key" class="file-workbench-tab-panel">
      <slot />
    </TabsContent>
    <slot v-else />
  </TabsRoot>
</template>

<script setup lang="ts">
  import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import {
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuPortal,
    DropdownMenuRoot,
    DropdownMenuTrigger,
    TabsContent,
    TabsList,
    TabsRoot,
    TabsTrigger,
  } from 'reka-ui'
  import WorkbenchCodicon from './WorkbenchCodicon.vue'
  import { entryName } from '../../features/files/workbenchUi'
  import type { FileDocumentState } from '../../store/fileWorkbench'

  const props = defineProps<{
    documents: FileDocumentState[]
    activeDocument: FileDocumentState | null
  }>()
  const emit = defineEmits<{
    activate: [path: string]
    close: [path: string]
    closeAll: []
    closeOthers: []
  }>()

  const { t } = useI18n()
  const tabTitleElements = new Map<string, HTMLElement>()
  const truncatedDocumentKeys = ref(new Set<string>())
  let tabTitleObserver: ResizeObserver | null = null

  const canCloseAll = computed(
    () => props.documents.length > 0 && !props.documents.some((document) => document.saving),
  )
  const canCloseOthers = computed(
    () =>
      props.documents.length > 1 &&
      props.activeDocument !== null &&
      !props.documents.some(
        (document) => document.path !== props.activeDocument?.path && document.saving,
      ),
  )

  function activateDocument(key: string | number) {
    const document = props.documents.find((item) => item.key === key)
    if (document) emit('activate', document.path)
  }

  function setTabTitleElement(key: string, element: unknown) {
    const previous = tabTitleElements.get(key)
    if (previous) tabTitleObserver?.unobserve(previous)
    if (!isHtmlElement(element)) {
      tabTitleElements.delete(key)
      setTruncated(key, false)
      return
    }
    tabTitleElements.set(key, element)
    tabTitleObserver?.observe(element)
    measureTabTitle(key)
  }

  function isHtmlElement(element: unknown): element is HTMLElement {
    return typeof element === 'object' && element !== null && 'scrollWidth' in element
  }

  function measureTabTitle(key: string) {
    const element = tabTitleElements.get(key)
    setTruncated(key, Boolean(element && element.scrollWidth > element.clientWidth))
  }

  function measureAllTabTitles() {
    for (const key of tabTitleElements.keys()) {
      measureTabTitle(key)
    }
  }

  function setTruncated(key: string, truncated: boolean) {
    if (truncatedDocumentKeys.value.has(key) === truncated) return
    const next = new Set(truncatedDocumentKeys.value)
    if (truncated) {
      next.add(key)
    } else {
      next.delete(key)
    }
    truncatedDocumentKeys.value = next
  }

  onMounted(() => {
    if (typeof ResizeObserver === 'undefined') {
      measureAllTabTitles()
      return
    }
    tabTitleObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const key = [...tabTitleElements.entries()].find(
          ([, element]) => element === entry.target,
        )?.[0]
        if (key) measureTabTitle(key)
      }
    })
    for (const element of tabTitleElements.values()) {
      tabTitleObserver.observe(element)
    }
    measureAllTabTitles()
  })

  watch(
    () => props.documents.map((document) => document.key),
    async () => {
      await nextTick()
      measureAllTabTitles()
    },
    { flush: 'post' },
  )

  onBeforeUnmount(() => {
    tabTitleObserver?.disconnect()
    tabTitleObserver = null
  })
</script>
