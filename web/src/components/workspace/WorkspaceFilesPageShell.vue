<template>
  <div
    class="file-workbench-shell flex h-screen min-h-screen overflow-hidden bg-[var(--color-app-bg)] text-sm text-[var(--color-text)]"
  >
    <PageStatus
      class="flex min-h-0 w-full flex-1 flex-col"
      :loading="!ready"
      :error="ready && !workspace ? missingMessage : null"
      :loading-text="t('files.loadingWorkspace')"
    >
      <template #loading>
        <div class="file-workbench-missing" role="status">
          <p class="file-workbench-empty">{{ t('files.loadingWorkspace') }}</p>
        </div>
      </template>
      <template #error>
        <div class="file-workbench-missing" role="status">
          <p class="file-workbench-error">{{ missingMessage }}</p>
          <button type="button" class="button button-secondary" @click="goBackToSessions">
            {{ t('files.backToSessions') }}
          </button>
        </div>
      </template>

      <WorkspaceFileWorkbench
        v-if="workspace"
        :workspace="workspace"
        :target="runtimeTarget"
        :api="fileRuntimeApi"
        @back="goBackToSessions"
      />
    </PageStatus>
  </div>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { createFileGitRuntimeApi } from '../../features/files/runtime'
  import type { RuntimeTarget } from '../../features/runtimeTarget'
  import type { SessionRuntimeApi } from '../../features/sessions/runtime'
  import type { Workspace as WorkspaceSummary } from '../../gen/proto/termbridge/agent/v1/workspace'
  import { useNotificationsStore } from '../../store/notifications'
  import { useWorkspaceSessionsStore } from '../../store/workspaceSessions'
  import PageStatus from '../layout/PageStatus.vue'
  import WorkspaceFileWorkbench from './WorkspaceFileWorkbench.vue'

  const props = defineProps<{
    runtimeTarget: RuntimeTarget
    runtimeApi: SessionRuntimeApi
    workspaceId: string
    sessionsRoute: { name: string; params?: Record<string, string> }
  }>()

  const { t } = useI18n()
  const router = useRouter()
  const workspaceSessions = useWorkspaceSessionsStore()
  const notifications = useNotificationsStore()
  const ready = ref(false)
  const loadError = ref<string | null>(null)

  const fileRuntimeApi = computed(() => createFileGitRuntimeApi(props.runtimeTarget))
  const workspace = computed<WorkspaceSummary | null>(() => {
    return workspaceSessions.workspaceById(props.workspaceId)
  })
  const missingMessage = computed(
    () => loadError.value || t('files.workspaceNotFound', { id: props.workspaceId }),
  )

  async function ensureWorkspaceLoaded() {
    loadError.value = null
    if (workspaceSessions.workspaceById(props.workspaceId)) {
      ready.value = true
      return
    }
    try {
      await workspaceSessions.refresh(props.runtimeTarget, props.runtimeApi)
    } catch (err) {
      loadError.value = err instanceof Error ? err.message : t('toast.refreshFailed')
      notifications.notifyError(t('toast.refreshFailed'), err)
    } finally {
      ready.value = true
    }
  }

  function goBackToSessions() {
    void router.push(props.sessionsRoute)
  }

  onMounted(() => {
    void ensureWorkspaceLoaded()
  })

  watch(
    () => [props.workspaceId, props.runtimeTarget] as const,
    () => {
      ready.value = false
      void ensureWorkspaceLoaded()
    },
  )
</script>
