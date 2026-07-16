<template>
  <section class="file-workbench-git-panel" :aria-label="t('files.gitTitle')">
    <header class="file-workbench-panel-header">
      <h3>{{ t('files.gitTitle') }}</h3>
      <button
        type="button"
        class="button button-secondary file-workbench-toolbar-button"
        :disabled="store.gitStatusLoading"
        @click="refresh"
      >
        {{ store.gitStatusLoading ? t('files.refreshing') : t('files.refreshGit') }}
      </button>
    </header>
    <p v-if="store.gitStatusStale" class="file-workbench-note" role="status">
      {{ t('files.gitStale') }}
    </p>
    <p v-if="store.gitStatusError" class="file-workbench-error" role="status">
      {{ store.gitStatusError }}
    </p>
    <p v-else-if="!store.gitStatus" class="file-workbench-empty">{{ t('files.gitNotLoaded') }}</p>
    <p
      v-else-if="store.gitStatus.state !== GitState.GIT_STATE_AVAILABLE"
      class="file-workbench-empty"
    >
      {{ store.gitStatus.message || t(`files.gitState.${gitStateLabel(store.gitStatus.state)}`) }}
    </p>
    <ul v-else class="file-workbench-git-list">
      <li v-if="store.gitStatus.changes.length === 0" class="file-workbench-empty">
        {{ t('files.noGitChanges') }}
      </li>
      <li
        v-for="change in store.gitStatus.changes"
        :key="change.path"
        class="file-workbench-git-change"
      >
        <div class="min-w-0 flex-1">
          <p class="truncate font-mono text-xs text-[var(--color-text)]">{{ change.path }}</p>
          <p v-if="change.original_path" class="truncate text-xs text-[var(--color-text-subtle)]">
            {{ change.original_path }}
          </p>
        </div>
        <div class="flex shrink-0 gap-1">
          <button
            v-for="layer in change.available_layers"
            :key="layer"
            type="button"
            class="file-workbench-git-layer"
            :class="{
              'file-workbench-git-layer-active':
                store.gitSelection?.path === change.path && store.gitSelection.layer === layer,
            }"
            @click="select(change.path, layer)"
          >
            {{ t(`files.gitLayer.${gitLayerLabel(layer)}`) }}
          </button>
        </div>
      </li>
    </ul>
    <GitDiffEditor
      :target="store.target"
      :workspace-id="store.workspaceId"
      :diff="store.gitDiff"
      :loading="store.gitDiffLoading"
      :error="store.gitDiffError"
    />
  </section>
</template>

<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import { GitState, type GitLayer } from '../../gen/proto/termbridge/agent/v1/git'
  import type { FileGitRuntimeApi } from '../../features/files/runtime'
  import { gitLayerLabel, gitStateLabel } from '../../features/files/workbenchUi'
  import { useFileWorkbenchStore } from '../../store/fileWorkbench'
  import GitDiffEditor from './GitDiffEditor.vue'

  const props = defineProps<{
    api: FileGitRuntimeApi
  }>()

  const { t } = useI18n()
  const store = useFileWorkbenchStore()

  async function refresh() {
    await store.refreshGitStatus(props.api)
  }

  async function select(path: string, layer: GitLayer) {
    if (store.selectGitChange(path, layer)) {
      await store.refreshGitDiff(props.api)
    }
  }
</script>
