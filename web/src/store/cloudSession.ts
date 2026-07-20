import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { CloudSessionSummary } from '../gen/proto/termbridge/cloud/v1/session'

export const useCloudSessionStore = defineStore('cloudSession', () => {
  const cloudSession = ref<CloudSessionSummary | null>(null)

  function setCloudSession(summary: CloudSessionSummary | null) {
    cloudSession.value = summary
  }

  function reset() {
    cloudSession.value = null
  }

  return {
    cloudSession,
    setCloudSession,
    reset,
  }
})
