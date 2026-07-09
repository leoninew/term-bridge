<template>
  <SessionsPageShell
    :runtime-target="runtimeTarget"
    :runtime-api="runtimeApi"
    home-route-name="home"
    :login-redirect="loginRedirect"
    :current-device="currentDevice"
    :logout="authLogout"
  />
</template>

<script setup lang="ts">
  import { computed } from 'vue'
  import { useRoute } from 'vue-router'
  import SessionsPageShell from '../../components/session/SessionsPageShell.vue'
  import { useCloudDevicesStore } from '../../store/cloudDevices'
  import {
    authLogout,
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
  } from '../../features/cloud/api'
  import type { SessionRuntimeApi } from '../../features/sessions/runtime'
  import type { RuntimeTarget } from '../../features/runtimeTarget'
  import type {
    CreateSessionReq,
    RerunSessionReq,
    UpdateSessionReq,
  } from '../../gen/proto/termbridge/agent/v1/session'

  const route = useRoute()
  const cloudDevices = useCloudDevicesStore()
  const deviceId = computed(() => String(route.params.deviceId ?? ''))
  const runtimeTarget = computed<RuntimeTarget>(() => ({ mode: 'cloud', deviceId: deviceId.value }))
  const loginRedirect = computed(() => `/devices/${encodeURIComponent(deviceId.value)}/sessions`)
  const currentDevice = computed(
    () => cloudDevices.devices.find((device) => device.id === deviceId.value) ?? null,
  )
  const runtimeApi = computed<SessionRuntimeApi>(() => ({
    createSession(workspaceId: string | null, request: CreateSessionReq) {
      return createSession(runtimeTarget.value, workspaceId, request)
    },
    getSession(workspaceId: string, sessionId: string) {
      return getSession(runtimeTarget.value, workspaceId, sessionId)
    },
    updateSession(workspaceId: string, sessionId: string, request: UpdateSessionReq) {
      return updateSession(runtimeTarget.value, workspaceId, sessionId, request)
    },
    deleteSession(workspaceId: string, sessionId: string) {
      return deleteSession(runtimeTarget.value, workspaceId, sessionId)
    },
    readHistory(workspaceId: string, sessionId: string) {
      return readHistory(runtimeTarget.value, workspaceId, sessionId)
    },
    closeSession(workspaceId: string, sessionId: string) {
      return closeSession(runtimeTarget.value, workspaceId, sessionId)
    },
    rerunSession(workspaceId: string, sessionId: string, request: RerunSessionReq) {
      return rerunSession(runtimeTarget.value, workspaceId, sessionId, request)
    },
    updateSessionOrder(workspaceId: string, sessionIds: string[]) {
      return updateSessionOrder(runtimeTarget.value, workspaceId, sessionIds)
    },
    listWorkspaceTree() {
      return listWorkspaceTree(runtimeTarget.value)
    },
    updateWorkspaceOrder(workspaceIds: string[]) {
      return updateWorkspaceOrder(runtimeTarget.value, workspaceIds)
    },
    deleteWorkspace(workspaceId: string) {
      return deleteWorkspace(runtimeTarget.value, workspaceId)
    },
  }))
</script>
