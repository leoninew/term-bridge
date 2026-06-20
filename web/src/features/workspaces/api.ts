import type { WorkspaceSummary, WorkspaceTreeSummary } from '../../protocol/terminal'
import { responseError } from '../sessions/api'

export async function listWorkspaces(): Promise<WorkspaceSummary[]> {
  const response = await fetch('/api/workspaces')
  if (!response.ok) {
    throw new Error(await responseError('List workspaces failed', response))
  }
  return ((await response.json()) as WorkspaceSummary[] | null) ?? []
}

export async function listWorkspaceTree(): Promise<WorkspaceTreeSummary[]> {
  const response = await fetch('/api/workspaces/tree')
  if (!response.ok) {
    throw new Error(await responseError('List workspace tree failed', response))
  }
  return ((await response.json()) as WorkspaceTreeSummary[] | null) ?? []
}

export async function updateWorkspaceOrder(workspaceIds: string[]): Promise<WorkspaceSummary[]> {
  const response = await fetch('/api/workspaces/order', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ workspace_ids: workspaceIds }),
  })
  if (!response.ok) {
    throw new Error(await responseError('Update workspace order failed', response))
  }
  return ((await response.json()) as WorkspaceSummary[] | null) ?? []
}

export async function deleteWorkspace(workspaceId: string): Promise<void> {
  const response = await fetch(`/api/workspaces/${encodeURIComponent(workspaceId)}`, {
    method: 'DELETE',
  })
  if (!response.ok) {
    throw new Error(await responseError('Remove workspace failed', response))
  }
}
