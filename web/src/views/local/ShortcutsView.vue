<template><ShortcutsPageShell :api="api" /></template>
<script setup lang="ts">
  import ShortcutsPageShell from '../../components/shortcut/ShortcutsPageShell.vue'
  import {
    createShortcut,
    deleteShortcut,
    listShortcuts,
    updateShortcut,
    updateShortcutOrder,
  } from '../../features/local/api'
  const api = { listShortcuts, createShortcut, updateShortcut, updateShortcutOrder, deleteShortcut }
</script>
