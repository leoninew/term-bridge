<template>
  <aside class="sidebar">
    <header class="sidebar-header">
      <div>
        <span class="eyebrow">Workspace / Session</span>
        <h1>TermBridge</h1>
      </div>
      <button type="button" @click="emit('refresh')">Refresh</button>
    </header>

    <div v-if="workspaces.length === 0" class="empty-state">
      No workspaces yet. Create a session to initialize one.
    </div>

    <section v-for="workspace in workspaces" :key="workspace.key" class="workspace-group">
      <h2>{{ workspace.name }}</h2>
      <p :title="workspace.path">{{ workspace.path }}</p>
      <button
        v-for="session in sessionsFor(workspace.key)"
        :key="session.id"
        type="button"
        class="session-item"
        :class="{ selected: selectedSessionId === session.id }"
        @click="emit('select', session)"
      >
        <span class="session-command">{{ session.command }}</span>
        <span class="session-meta">
          {{ session.lifecycle_state }}
          <template v-if="session.attachment_state"> · {{ session.attachment_state }}</template>
          <template v-if="session.exit_code !== undefined">
            · exit {{ session.exit_code }}</template
          >
        </span>
      </button>
    </section>
  </aside>
</template>

<script setup lang="ts">
  import type { SessionSummary, WorkspaceSummary } from '../../protocol/terminal'

  const props = defineProps<{
    workspaces: WorkspaceSummary[]
    sessions: SessionSummary[]
    selectedSessionId: string | null
  }>()

  const emit = defineEmits<{
    select: [session: SessionSummary]
    refresh: []
  }>()

  function sessionsFor(workspaceKey: string) {
    return props.sessions.filter((session) => session.workspace_key === workspaceKey)
  }
</script>
