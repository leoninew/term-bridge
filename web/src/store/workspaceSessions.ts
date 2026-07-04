import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import type {
  SessionSummary,
  WorkspaceSummary,
  WorkspaceTreeSession,
  WorkspaceTreeSummary,
} from '../protocol/terminal'
import type { SessionRuntimeApi } from '../features/sessions/runtime'
import type { RuntimeTarget } from '../features/runtimeTarget'

export type RemovedSessionSummary = Pick<SessionSummary, 'id' | 'workspace_id'>

export const useWorkspaceSessionsStore = defineStore('workspaceSessions', () => {
  const workspaceTree = ref<WorkspaceTreeSummary[]>([])
  const loading = ref(false)

  const workspaces = computed<WorkspaceSummary[]>(() =>
    workspaceTree.value.map(workspaceSummaryFromTree),
  )

  const sessions = computed<SessionSummary[]>(() =>
    workspaceTree.value.flatMap((workspace) => sessionsForWorkspace(workspace)),
  )

  function reset() {
    workspaceTree.value = []
  }

  async function refresh(target: RuntimeTarget | null, runtimeApi: SessionRuntimeApi) {
    if (!target) {
      return
    }
    loading.value = true
    try {
      const response = await runtimeApi.listWorkspaceTree()
      applyWorkspaceTree(response.data)
    } finally {
      loading.value = false
    }
  }

  function applyWorkspaceTree(nextWorkspaceTree: WorkspaceTreeSummary[]) {
    workspaceTree.value = Array.isArray(nextWorkspaceTree) ? nextWorkspaceTree : []
  }

  function workspaceById(workspaceId: string): WorkspaceSummary | null {
    const workspace = workspaceTree.value.find((item) => item.id === workspaceId)
    return workspace ? workspaceSummaryFromTree(workspace) : null
  }

  function sessionById(workspaceId: string, sessionId: string): SessionSummary | null {
    const workspace = workspaceTree.value.find((item) => item.id === workspaceId)
    if (!workspace) {
      return null
    }
    const session = workspace.children.find((item) => item.id === sessionId)
    return session ? { ...session, workspace_id: session.workspace_id ?? workspace.id } : null
  }

  function sessionTitle(session: SessionSummary | null): string {
    if (!session) {
      return ''
    }
    const workspace = workspaceById(session.workspace_id)
    return workspace ? `${session.name} · ${workspace.name}` : session.name
  }

  function upsertSession(updated: SessionSummary): boolean {
    if (!workspaceTree.value.some((workspace) => workspace.id === updated.workspace_id)) {
      return false
    }
    workspaceTree.value = workspaceTree.value.map((workspace) => {
      if (workspace.id !== updated.workspace_id) {
        return workspace
      }
      const nextSession = sessionSummaryForWorkspaceTree(updated)
      const childIndex = workspace.children.findIndex((session) => session.id === updated.id)
      if (childIndex === -1) {
        return { ...workspace, children: [...workspace.children, nextSession] }
      }
      return {
        ...workspace,
        children: workspace.children.map((session) =>
          session.id === updated.id ? nextSession : session,
        ),
      }
    })
    return true
  }

  function updateSession(updated: SessionSummary) {
    if (!sessionById(updated.workspace_id, updated.id)) {
      return
    }
    upsertSession(updated)
  }

  function removeSession(workspaceId: string, sessionId: string) {
    workspaceTree.value = workspaceTree.value.map((workspace) => {
      if (workspace.id !== workspaceId) {
        return workspace
      }
      return {
        ...workspace,
        children: workspace.children.filter((session) => session.id !== sessionId),
      }
    })
  }

  function removeWorkspace(workspaceId: string): RemovedSessionSummary[] {
    const workspace = workspaceTree.value.find((item) => item.id === workspaceId)
    const removedSessions =
      workspace?.children.map((session) => ({
        id: session.id,
        workspace_id: session.workspace_id ?? workspace.id,
      })) ?? []
    workspaceTree.value = workspaceTree.value.filter((item) => item.id !== workspaceId)
    return removedSessions
  }

  async function reorderWorkspaces(
    target: RuntimeTarget | null,
    runtimeApi: SessionRuntimeApi,
    workspaceIds: string[],
  ) {
    if (!target) {
      return
    }
    const previousTree = workspaceTree.value
    workspaceTree.value = orderWorkspaceTree(previousTree, workspaceIds)
    try {
      const orderedWorkspaces = await runtimeApi.updateWorkspaceOrder(workspaceIds)
      workspaceTree.value = orderWorkspaceTree(
        workspaceTree.value,
        orderedWorkspaces.map((workspace) => workspace.id),
      )
    } catch (err) {
      workspaceTree.value = previousTree
      throw err
    }
  }

  async function reorderSessions(
    target: RuntimeTarget | null,
    runtimeApi: SessionRuntimeApi,
    workspaceId: string,
    sessionIds: string[],
  ) {
    if (!target) {
      return
    }
    const previousTree = workspaceTree.value
    workspaceTree.value = orderWorkspaceSessions(previousTree, workspaceId, sessionIds)
    try {
      const orderedSessions = await runtimeApi.updateSessionOrder(workspaceId, sessionIds)
      workspaceTree.value = replaceWorkspaceSessions(
        workspaceTree.value,
        workspaceId,
        orderedSessions,
      )
    } catch (err) {
      workspaceTree.value = previousTree
      throw err
    }
  }

  return {
    workspaceTree,
    loading,
    workspaces,
    sessions,
    reset,
    refresh,
    applyWorkspaceTree,
    workspaceById,
    sessionById,
    sessionTitle,
    upsertSession,
    updateSession,
    removeSession,
    removeWorkspace,
    reorderWorkspaces,
    reorderSessions,
  }
})

function sessionsForWorkspace(workspace: WorkspaceTreeSummary): SessionSummary[] {
  return workspace.children.map((session) => ({
    ...session,
    workspace_id: session.workspace_id ?? workspace.id,
  }))
}

function sessionSummaryForWorkspaceTree(session: SessionSummary) {
  return {
    id: session.id,
    name: session.name,
    command: session.command,
    cwd: session.cwd,
    lifecycle_state: session.lifecycle_state,
    attachment_state: session.attachment_state,
    exit_code: session.exit_code,
    updated_at: session.updated_at,
  }
}

function workspaceSummaryFromTree(workspace: WorkspaceTreeSummary): WorkspaceSummary {
  return {
    id: workspace.id,
    name: workspace.name,
    path: workspace.path,
    updated_at: workspace.updated_at,
  }
}

function orderWorkspaceTree(tree: WorkspaceTreeSummary[], workspaceIds: string[]) {
  const order = new Map(workspaceIds.map((workspaceId, index) => [workspaceId, index]))
  return [...tree].sort((left, right) => {
    const leftOrder = order.get(left.id) ?? Number.MAX_SAFE_INTEGER
    const rightOrder = order.get(right.id) ?? Number.MAX_SAFE_INTEGER
    return leftOrder - rightOrder
  })
}

function orderWorkspaceSessions(
  tree: WorkspaceTreeSummary[],
  workspaceId: string,
  sessionIds: string[],
) {
  return tree.map((workspace) => {
    if (workspace.id !== workspaceId) {
      return workspace
    }
    const order = new Map(sessionIds.map((sessionId, index) => [sessionId, index]))
    return {
      ...workspace,
      children: [...workspace.children].sort((left, right) => {
        const leftOrder = order.get(left.id) ?? Number.MAX_SAFE_INTEGER
        const rightOrder = order.get(right.id) ?? Number.MAX_SAFE_INTEGER
        return leftOrder - rightOrder
      }),
    }
  })
}

function replaceWorkspaceSessions(
  tree: WorkspaceTreeSummary[],
  workspaceId: string,
  sessions: WorkspaceTreeSession[],
) {
  return tree.map((workspace) =>
    workspace.id === workspaceId ? { ...workspace, children: sessions } : workspace,
  )
}
