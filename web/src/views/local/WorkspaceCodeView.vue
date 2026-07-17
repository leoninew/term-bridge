<template>
  <CodeWorkbenchShell
    :runtime-target="runtimeTarget"
    :workspace-id="workspaceId"
    :sessions-route="{ name: 'local-sessions' }"
  />
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useRoute } from 'vue-router'
  import CodeWorkbenchShell from '../../components/workbench/CodeWorkbenchShell.vue'
  import type { RuntimeTarget } from '../../features/runtimeTarget'

  const route = useRoute()
  const workspaceId = computed(() => String(route.params.workspaceId ?? ''))
  const runtimeTarget: RuntimeTarget = { mode: 'local' }
</script>
