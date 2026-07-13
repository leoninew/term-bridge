import type {
  SessionSummary,
  WorkspaceTreeNode,
} from '../../gen/proto/termbridge/agent/v1/workspace'
import type { OpenSessionTab } from '../../store/workbench'

export type SessionIdentity = {
  workspaceId: string
  sessionId: string
}

export function terminalTabTargets(
  openedTabs: OpenSessionTab[],
  sessionResolver: (workspaceId: string, sessionId: string) => SessionSummary | null,
): OpenSessionTab[] {
  return openedTabs.filter((tab) => {
    const lifecycleState = sessionResolver(tab.workspaceId, tab.sessionId)?.lifecycle_state
    return lifecycleState === 'stopped' || lifecycleState === 'failed'
  })
}

export function normalizedSessionWorkspaceTree(
  workspaceTree: WorkspaceTreeNode[],
): WorkspaceTreeNode[] {
  return workspaceTree.map((workspace) => ({
    ...workspace,
    children: workspace.children.map((session) => sessionForWorkspace(session, workspace.id)),
  }))
}

export function defaultBackgroundSessionSelectionKeys(
  workspaceTree: WorkspaceTreeNode[],
  openedTabs: OpenSessionTab[],
): Set<string> {
  const openedSessionKeys = new Set(openedTabs.map(tabIdentityKey))
  return new Set(
    workspaceTree
      .flatMap((workspace) => workspace.children)
      .filter(
        (session) =>
          session.lifecycle_state === 'running' &&
          !openedSessionKeys.has(sessionIdentityKey(session.workspace_id, session.id)),
      )
      .map((session) => sessionIdentityKey(session.workspace_id, session.id)),
  )
}

export function selectedSessionWorkspaceTreeTargets(
  workspaceTree: WorkspaceTreeNode[],
  selectedSessions: SessionIdentity[],
): SessionSummary[] {
  const selectedKeys = new Set(
    selectedSessions.map((session) => sessionIdentityKey(session.workspaceId, session.sessionId)),
  )
  return workspaceTree.flatMap((workspace) =>
    workspace.children.filter(
      (session) =>
        session.lifecycle_state === 'running' &&
        selectedKeys.delete(sessionIdentityKey(session.workspace_id, session.id)),
    ),
  )
}

export async function closeSessionsSerially(
  sessions: SessionSummary[],
  closeSession: (workspaceId: string, sessionId: string) => Promise<SessionSummary>,
  applySession: (session: SessionSummary) => void,
) {
  for (const session of sessions) {
    const updated = await closeSession(session.workspace_id, session.id)
    applySession(updated)
  }
}

function tabIdentityKey(tab: SessionIdentity): string {
  return sessionIdentityKey(tab.workspaceId, tab.sessionId)
}

function sessionForWorkspace(session: SessionSummary, workspaceId: string): SessionSummary {
  return session.workspace_id ? session : { ...session, workspace_id: workspaceId }
}

export function sessionIdentityKey(workspaceId: string, sessionId: string): string {
  return JSON.stringify([workspaceId, sessionId])
}
