import type { WorkspaceSummary } from '../../protocol/terminal'

export async function listWorkspaces(): Promise<WorkspaceSummary[]> {
  const response = await fetch('/api/workspaces')
  if (!response.ok) {
    throw new Error(`list workspaces failed: ${response.status}`)
  }
  const body = (await response.json()) as { workspaces: WorkspaceSummary[] | null }
  return body.workspaces ?? []
}
