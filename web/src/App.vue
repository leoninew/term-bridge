<template>
  <main class="app-shell">
    <WorkspaceSessionSidebar
      :workspaces="workspaces"
      :sessions="sessions"
      :selected-session-id="selectedSession?.id ?? null"
      @select="selectSession"
      @refresh="refresh"
    />

    <section class="main-pane">
      <header class="topbar">
        <div>
          <span class="eyebrow">M2.5 Web Terminal</span>
          <h2>Gate-ready local terminal prototype</h2>
        </div>
        <span v-if="loading" class="status-pill">Loading…</span>
      </header>

      <form class="new-session" @submit.prevent="startSession">
        <label>
          <span>Cwd</span>
          <input v-model="cwd" placeholder="empty = backend cwd" />
        </label>
        <label>
          <span>Command</span>
          <input v-model="commandText" placeholder="pwsh -NoLogo" />
        </label>
        <button type="submit">New session</button>
      </form>

      <p v-if="error" class="error-banner">{{ error }}</p>

      <section v-if="selectedSession" class="session-summary">
        <div><strong>Session</strong> {{ selectedSession.id }}</div>
        <div><strong>Cwd</strong> {{ selectedSession.cwd }}</div>
        <div><strong>Command</strong> {{ selectedSession.command }}</div>
        <div>
          <strong>State</strong> {{ selectedSession.lifecycle_state }}
          <template v-if="selectedSession.attachment_state">
            / {{ selectedSession.attachment_state }}
          </template>
        </div>
      </section>

      <TerminalView
        v-if="selectedCanAttach"
        :key="selectedSession?.id"
        :ws-url="wsUrl"
        :session-id="selectedSession?.id ?? null"
        @state="handleTerminalState"
      />

      <section v-else class="history-panel">
        <h3>History / metadata</h3>
        <pre>{{
          historyText ||
          'Select a stopped session to inspect bounded history, or create a new session.'
        }}</pre>
      </section>
    </section>
  </main>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import TerminalView from './components/terminal/TerminalView.vue'
  import WorkspaceSessionSidebar from './components/workspace/WorkspaceSessionSidebar.vue'
  import type { ServerControlMessage, SessionSummary, WorkspaceSummary } from './protocol/terminal'
  import { commandFromText } from './protocol/terminal'
  import { createSession, listSessions, readHistory } from './features/sessions/api'
  import { listWorkspaces } from './features/workspaces/api'

  const workspaces = ref<WorkspaceSummary[]>([])
  const sessions = ref<SessionSummary[]>([])
  const selectedSession = ref<SessionSummary | null>(null)
  const wsUrl = ref<string | null>(null)
  const historyText = ref('')
  const commandText = ref('pwsh -NoLogo')
  const cwd = ref('')
  const error = ref<string | null>(null)
  const loading = ref(false)

  const selectedCanAttach = computed(() => selectedSession.value?.lifecycle_state === 'running')

  async function refresh() {
    loading.value = true
    error.value = null
    try {
      const [nextWorkspaces, nextSessions] = await Promise.all([listWorkspaces(), listSessions()])
      workspaces.value = nextWorkspaces
      sessions.value = nextSessions
      if (selectedSession.value) {
        selectedSession.value =
          nextSessions.find((session) => session.id === selectedSession.value?.id) ??
          selectedSession.value
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : String(err)
    } finally {
      loading.value = false
    }
  }

  async function startSession() {
    const command = commandFromText(commandText.value)
    if (command.length === 0) {
      error.value = 'command is required'
      return
    }
    error.value = null
    const created = await createSession({ cwd: cwd.value, command, cols: 120, rows: 32 })
    wsUrl.value = created.ws_url
    await refresh()
    selectedSession.value =
      sessions.value.find((session) => session.id === created.session_id) ?? null
  }

  async function selectSession(session: SessionSummary) {
    selectedSession.value = session
    wsUrl.value = session.lifecycle_state === 'running' ? `/api/sessions/${session.id}/ws` : null
    historyText.value = ''
    if (session.lifecycle_state !== 'running') {
      historyText.value = await readHistory(session.id)
    }
  }

  function handleTerminalState(message: ServerControlMessage) {
    if (message.type === 'state' || message.type === 'exited') {
      void refresh()
    }
  }

  onMounted(() => {
    void refresh()
  })
</script>
