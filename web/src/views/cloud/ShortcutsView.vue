<template><ShortcutsPageShell :api="api" /></template>
<script setup lang="ts">
  import { computed } from 'vue'
  import { useRoute } from 'vue-router'
  import ShortcutsPageShell from '../../components/shortcut/ShortcutsPageShell.vue'
  import { createShortcut, deleteShortcut, listShortcuts, updateShortcut } from '../../features/cloud/api'
  import type { RuntimeTarget } from '../../features/runtimeTarget'
  const route = useRoute(); const target = computed<RuntimeTarget>(() => ({ mode: 'cloud', deviceId: String(route.params.deviceId ?? '') }))
  const api = computed(() => ({ listShortcuts: () => listShortcuts(target.value), createShortcut: (request: Parameters<typeof createShortcut>[1]) => createShortcut(target.value, request), updateShortcut: (id: string, request: Parameters<typeof updateShortcut>[2]) => updateShortcut(target.value, id, request), deleteShortcut: (id: string) => deleteShortcut(target.value, id) }))
</script>
