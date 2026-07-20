<template><ShortcutsPageShell :api="api" :return-to-workspace="returnToWorkspace" /></template>
<script setup lang="ts">
  import { computed } from 'vue'
  import { useRoute, useRouter } from 'vue-router'
  import ShortcutsPageShell from '../../components/shortcut/ShortcutsPageShell.vue'
  import {
    createShortcut,
    deleteShortcut,
    listShortcuts,
    updateShortcut,
    updateShortcutOrder,
  } from '../../features/cloud/api'
  import type { RuntimeTarget } from '../../features/runtimeTarget'
  const route = useRoute()
  const router = useRouter()
  const deviceId = computed(() => String(route.params.deviceId ?? ''))
  const target = computed<RuntimeTarget>(() => ({
    mode: 'cloud',
    deviceId: deviceId.value,
  }))
  const api = computed(() => ({
    listShortcuts: () => listShortcuts(target.value),
    createShortcut: (request: Parameters<typeof createShortcut>[1]) =>
      createShortcut(target.value, request),
    updateShortcut: (id: string, request: Parameters<typeof updateShortcut>[2]) =>
      updateShortcut(target.value, id, request),
    updateShortcutOrder: (request: Parameters<typeof updateShortcutOrder>[1]) =>
      updateShortcutOrder(target.value, request),
    deleteShortcut: (id: string) => deleteShortcut(target.value, id),
  }))

  async function returnToWorkspace() {
    await router.push({ name: 'cloud-sessions', params: { deviceId: deviceId.value } })
  }
</script>
