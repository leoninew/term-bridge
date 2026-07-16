<template>
  <WorkspaceFilesPageShell
    :runtime-target="runtimeTarget"
    :runtime-api="runtimeApi"
    :workspace-id="workspaceId"
    :sessions-route="{ name: 'local-sessions' }"
  />
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useRoute } from 'vue-router'
  import WorkspaceFilesPageShell from '../../components/workspace/WorkspaceFilesPageShell.vue'
  import {
    closeSession,
    createSession,
    deleteSession,
    deleteWorkspace,
    getSession,
    listWorkspaceTree,
    readHistory,
    rerunSession,
    updateSession,
    updateSessionOrder,
    updateWorkspaceOrder,
  } from '../../features/local/api'
  import type { SessionRuntimeApi } from '../../features/sessions/runtime'
  import type { RuntimeTarget } from '../../features/runtimeTarget'

  const route = useRoute()
  const workspaceId = computed(() => String(route.params.workspaceId ?? ''))
  const runtimeTarget: RuntimeTarget = { mode: 'local' }
  const runtimeApi: SessionRuntimeApi = {
    createSession,
    getSession,
    updateSession,
    deleteSession,
    readHistory,
    closeSession,
    rerunSession,
    updateSessionOrder,
    listWorkspaceTree,
    updateWorkspaceOrder,
    deleteWorkspace,
  }
</script>
