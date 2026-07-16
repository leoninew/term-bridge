<template>
  <section class="file-workbench-diff" :aria-label="t('files.gitDiffTitle')">
    <div v-if="loading" class="file-workbench-empty file-workbench-empty-center" role="status">
      {{ t('files.loadingDiff') }}
    </div>
    <p v-else-if="error" class="file-workbench-error" role="status">{{ error }}</p>
    <div
      v-else-if="!diff || diff.state !== GitState.GIT_STATE_AVAILABLE"
      class="file-workbench-empty file-workbench-empty-center"
    >
      {{ diff?.message || t('files.selectGitChange') }}
    </div>
    <div v-else ref="host" class="file-workbench-monaco-host" />
  </section>
</template>

<script setup lang="ts">
  import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { GitState, type GitDiffResp } from '../../gen/proto/termbridge/agent/v1/git'
  import type { RuntimeTarget } from '../../features/runtimeTarget'
  import {
    applyMonacoTheme,
    diffEditorOptions,
    fileUri,
    languageForPath,
    monaco,
  } from '../../features/files/monaco'
  import { useThemeStore } from '../../store/theme'

  const props = defineProps<{
    target: RuntimeTarget | null
    workspaceId: string | null
    diff: GitDiffResp | null
    loading: boolean
    error: string | null
  }>()

  const { t } = useI18n()
  const theme = useThemeStore()
  const host = ref<HTMLElement | null>(null)
  let editor: monaco.editor.IStandaloneDiffEditor | null = null
  let original: monaco.editor.ITextModel | null = null
  let modified: monaco.editor.ITextModel | null = null
  let resizeObserver: ResizeObserver | null = null

  async function mountDiff() {
    disposeDiff()
    const diff = props.diff
    if (
      !host.value ||
      !diff ||
      diff.state !== GitState.GIT_STATE_AVAILABLE ||
      !props.target ||
      !props.workspaceId
    ) {
      return
    }
    applyMonacoTheme(theme.theme)
    const language = languageForPath(diff.modified_path || diff.original_path)
    original = monaco.editor.createModel(
      diff.original_text,
      language,
      fileUri(
        props.target,
        props.workspaceId,
        diff.original_path || diff.modified_path,
        'original',
      ),
    )
    modified = monaco.editor.createModel(
      diff.modified_text,
      language,
      fileUri(
        props.target,
        props.workspaceId,
        diff.modified_path || diff.original_path,
        'modified',
      ),
    )
    editor = monaco.editor.createDiffEditor(host.value, {
      ...diffEditorOptions,
      theme: theme.theme === 'dark' ? 'termbridge-dark' : 'termbridge-light',
    })
    editor.setModel({ original, modified })
    resizeObserver = new ResizeObserver(() => editor?.layout())
    resizeObserver.observe(host.value)
    editor.layout()
  }

  function disposeDiff() {
    resizeObserver?.disconnect()
    resizeObserver = null
    editor?.dispose()
    editor = null
    original?.dispose()
    original = null
    modified?.dispose()
    modified = null
  }

  watch(
    () => [props.diff, props.target, props.workspaceId] as const,
    async () => {
      await nextTick()
      await mountDiff()
    },
    { immediate: true, deep: false },
  )
  watch(
    () => theme.theme,
    (value) => applyMonacoTheme(value),
  )
  onBeforeUnmount(disposeDiff)
</script>
