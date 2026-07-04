<template>
  <SessionsPageShell
    :runtime-target="runtimeTarget"
    :runtime-api="runtimeApi"
    dashboard-route-name="agent-dashboard"
    login-redirect="/agent/sessions"
    :start-cloud-o-auth-url="cloudOAuthStartURL"
    :logout="authLogout"
  />
</template>

<script setup lang="ts">
  import SessionsPageShell from '../components/session/SessionsPageShell.vue'
  import {
    authLogout,
    closeSession,
    cloudOAuthStartURL,
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
  } from '../features/agent/api'
  import type { SessionRuntimeApi } from '../features/sessions/runtime'
  import type { RuntimeTarget } from '../features/runtimeTarget'

  const runtimeTarget: RuntimeTarget = { mode: 'agent' }
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
