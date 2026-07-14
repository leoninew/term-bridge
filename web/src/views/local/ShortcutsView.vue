<template><ShortcutsPageShell :api="api" :return-to-workspace="returnToWorkspace" /></template>
<script setup lang="ts">
  import { useRouter } from 'vue-router'
  import ShortcutsPageShell from '../../components/shortcut/ShortcutsPageShell.vue'
  import {
    createShortcut,
    deleteShortcut,
    listShortcuts,
    updateShortcut,
    updateShortcutOrder,
  } from '../../features/local/api'

  const router = useRouter()
  const api = { listShortcuts, createShortcut, updateShortcut, updateShortcutOrder, deleteShortcut }

  async function returnToWorkspace() {
    await router.push({ name: 'local-sessions' })
  }
</script>
