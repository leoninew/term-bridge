<template>
  <SessionsPageShell
    :runtime-target="runtimeTarget"
    :runtime-api="runtimeApi"
    home-route-name="home"
    login-redirect="/sessions"
    :logout="authLogout"
  />
</template>

<script setup lang="ts">
  import SessionsPageShell from '../../components/session/SessionsPageShell.vue'
  import {
    authLogout,
    closeSession,
    createSession,
    createShortcut,
    deleteSession,
    deleteShortcut,
    deleteWorkspace,
    getSession,
    listShortcuts,
    listWorkspaceTree,
    readHistory,
    rerunSession,
    updateSession,
    updateSessionOrder,
    updateShortcut,
    updateWorkspaceOrder,
  } from '../../features/local/api'
  import type { SessionRuntimeApi, ShortcutRuntimeApi } from '../../features/sessions/runtime'
  import type { RuntimeTarget } from '../../features/runtimeTarget'

  const runtimeTarget: RuntimeTarget = { mode: 'local' }
  const runtimeApi: SessionRuntimeApi & ShortcutRuntimeApi = {
    createSession,
    createShortcut,
    getSession,
    updateSession,
    deleteSession,
    deleteShortcut,
    readHistory,
    closeSession,
    rerunSession,
    updateSessionOrder,
    updateShortcut,
    listShortcuts,
    listWorkspaceTree,
    updateWorkspaceOrder,
    deleteWorkspace,
  }
</script>
