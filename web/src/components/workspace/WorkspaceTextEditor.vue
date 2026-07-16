<template>
  <section class="file-workbench-editor" :aria-label="t('files.editorTitle')">
    <p v-if="document?.conflict" class="file-workbench-warning" role="status">
      {{ t('files.revisionConflict') }}
    </p>
    <PageStatus
      class="min-h-0 flex-1"
      :loading="Boolean(document?.loading)"
      :error="document?.error || null"
      :empty="!document"
      :loading-text="t('files.loadingFile')"
      :empty-text="t('files.noDocument')"
    >
      <template #loading>
        <div class="file-workbench-empty file-workbench-empty-center" role="status">
          {{ t('files.loadingFile') }}
        </div>
      </template>
      <template #error>
        <p class="file-workbench-error" role="status">{{ document?.error }}</p>
      </template>
      <template #empty>
        <div class="file-workbench-empty file-workbench-empty-center">
          {{ t('files.noDocument') }}
        </div>
      </template>
      <div ref="host" class="file-workbench-monaco-host" />
    </PageStatus>
  </section>
</template>

<script setup lang="ts">
  import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
  import PageStatus from '../layout/PageStatus.vue'
  import { useI18n } from 'vue-i18n'
  import type { FileDocumentState } from '../../store/fileWorkbench'
  import { useThemeStore } from '../../store/theme'
  import type { RuntimeTarget } from '../../features/runtimeTarget'
  import {
    applyMonacoTheme,
    editorOptions,
    fileUri,
    languageForPath,
    monaco,
  } from '../../features/files/monaco'

  const props = defineProps<{
    target: RuntimeTarget | null
    workspaceId: string | null
    document: FileDocumentState | null
  }>()

  const emit = defineEmits<{
    draft: [path: string, text: string]
    save: [path: string, force: boolean]
  }>()

  const { t } = useI18n()
  const theme = useThemeStore()
  const host = ref<HTMLElement | null>(null)
  let editor: monaco.editor.IStandaloneCodeEditor | null = null
  let model: monaco.editor.ITextModel | null = null
  let contentListener: monaco.IDisposable | null = null
  let resizeObserver: ResizeObserver | null = null
  let syncing = false

  async function mountEditor() {
    disposeEditor()
    const document = props.document
    if (
      !host.value ||
      !document ||
      document.loading ||
      !document.entry ||
      !props.target ||
      !props.workspaceId
    ) {
      return
    }
    applyMonacoTheme(theme.theme)
    model = monaco.editor.createModel(
      document.draftText,
      languageForPath(document.path),
      fileUri(props.target, props.workspaceId, document.path),
    )
    editor = monaco.editor.create(host.value, {
      ...editorOptions,
      model,
      theme: theme.theme === 'dark' ? 'termbridge-dark' : 'termbridge-light',
    })
    contentListener = model.onDidChangeContent(() => {
      if (!syncing && props.document) {
        emit('draft', props.document.path, model?.getValue() ?? '')
      }
    })
    editor.addAction({
      id: 'termbridge.save-file',
      label: t('files.save'),
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS],
      run: () => {
        if (props.document?.dirty && !props.document.conflict) {
          emit('save', props.document.path, false)
        }
      },
    })
    resizeObserver = new ResizeObserver(() => editor?.layout())
    resizeObserver.observe(host.value)
    editor.layout()
  }

  function syncEditor() {
    if (!model || !props.document || model.getValue() === props.document.draftText) {
      return
    }
    syncing = true
    model.setValue(props.document.draftText)
    syncing = false
  }

  function disposeEditor() {
    resizeObserver?.disconnect()
    resizeObserver = null
    contentListener?.dispose()
    contentListener = null
    editor?.dispose()
    editor = null
    model?.dispose()
    model = null
  }

  watch(
    () =>
      [
        props.document?.key,
        props.document?.entry?.revision,
        props.target,
        props.workspaceId,
      ] as const,
    async () => {
      await nextTick()
      await mountEditor()
    },
    { immediate: true, deep: false },
  )
  watch(() => props.document?.draftText, syncEditor)
  watch(
    () => theme.theme,
    (value) => applyMonacoTheme(value),
  )
  onBeforeUnmount(disposeEditor)
</script>
