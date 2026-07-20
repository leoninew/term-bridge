import { describe, expect, it, vi } from 'vitest'
import type {
  SessionSummary,
  WorkspaceTreeNode,
} from '../../gen/proto/termbridge/agent/v1/workspace'
import type { OpenSessionTab } from '../../store/workbench'
import {
  closeSessionsSerially,
  defaultBackgroundSessionSelectionKeys,
  normalizedSessionWorkspaceTree,
  selectedSessionWorkspaceTreeTargets,
  sessionIdentityKey,
  terminalTabTargets,
} from './tabManagement'

function session(overrides: Partial<SessionSummary> = {}): SessionSummary {
  return {
    id: 'session-1',
    workspace_id: 'workspace-1',
    name: 'Shell',
    command: 'bash',
    cwd: '/work/one',
    lifecycle_state: 'running',
    attachment_state: '',
    command_source: '',
    shortcut_id_snapshot: '',
    shortcut_name_snapshot: '',
    updated_at: '2026-07-13T00:00:00Z',
    ...overrides,
  }
}

function treeNode(id: string, children: SessionSummary[]): WorkspaceTreeNode {
  return {
    id,
    name: `Workspace ${id}`,
    path: `/work/${id}`,
    updated_at: '2026-07-13T00:00:00Z',
    children,
  }
}

function tab(workspaceId: string, sessionId: string): OpenSessionTab {
  return {
    workspaceId,
    sessionId,
    historyText: '',
    historyLoaded: false,
    historyLoading: false,
    historyError: null,
  }
}

describe('tab management', () => {
  it('selects only stopped and failed opened tabs', () => {
    const stopped = session({ id: 'stopped', lifecycle_state: 'stopped' })
    const failed = session({ id: 'failed', lifecycle_state: 'failed' })
    const running = session({ id: 'running' })
    const openedTabs = [
      tab(stopped.workspace_id, stopped.id),
      tab(failed.workspace_id, failed.id),
      tab(running.workspace_id, running.id),
      tab('workspace-1', 'missing'),
    ]
    const sessions = [stopped, failed, running]

    const targets = terminalTabTargets(
      openedTabs,
      (workspaceId, sessionId) =>
        sessions.find(
          (candidate) => candidate.workspace_id === workspaceId && candidate.id === sessionId,
        ) ?? null,
    )

    expect(targets.map((item) => item.sessionId)).toEqual(['stopped', 'failed'])
  })

  it('preserves the complete workspace tree and normalizes child workspace identities', () => {
    const openedRunning = session({ id: 'same-id', workspace_id: 'workspace-1' })
    const stopped = session({
      id: 'stopped',
      workspace_id: 'workspace-1',
      lifecycle_state: 'stopped',
    })
    const failed = session({ id: 'failed', workspace_id: 'workspace-2', lifecycle_state: 'failed' })
    const missingWorkspaceId = session({ id: 'missing-workspace-id', workspace_id: '' })
    const input = [
      treeNode('workspace-1', [openedRunning, stopped, missingWorkspaceId]),
      treeNode('workspace-2', [failed]),
      treeNode('workspace-3', []),
    ]

    const targets = normalizedSessionWorkspaceTree(input)

    expect(targets).toEqual([
      treeNode('workspace-1', [
        openedRunning,
        stopped,
        session({ id: 'missing-workspace-id', workspace_id: 'workspace-1' }),
      ]),
      treeNode('workspace-2', [failed]),
      treeNode('workspace-3', []),
    ])
    expect(input[0]?.children[2]?.workspace_id).toBe('')
  })

  it('defaults only unopened running sessions to selected with workspace-scoped tab identities', () => {
    const openedRunning = session({ id: 'same-id', workspace_id: 'workspace-1' })
    const sameIdInAnotherWorkspace = session({ id: 'same-id', workspace_id: 'workspace-2' })
    const backgroundRunning = session({ id: 'background', workspace_id: 'workspace-1' })
    const stopped = session({
      id: 'stopped',
      workspace_id: 'workspace-1',
      lifecycle_state: 'stopped',
    })
    const failed = session({ id: 'failed', workspace_id: 'workspace-2', lifecycle_state: 'failed' })
    const tree = normalizedSessionWorkspaceTree([
      treeNode('workspace-1', [openedRunning, backgroundRunning, stopped]),
      treeNode('workspace-2', [sameIdInAnotherWorkspace, failed]),
    ])

    const selectedKeys = defaultBackgroundSessionSelectionKeys(tree, [
      tab('workspace-1', 'same-id'),
    ])

    expect(selectedKeys).toEqual(
      new Set([
        sessionIdentityKey(backgroundRunning.workspace_id, backgroundRunning.id),
        sessionIdentityKey(sameIdInAnotherWorkspace.workspace_id, sameIdInAnotherWorkspace.id),
      ]),
    )
  })

  it('resolves only selected running sessions from the current workspace tree', () => {
    const openedRunning = session({ id: 'opened', workspace_id: 'workspace-1' })
    const stopped = session({
      id: 'stopped',
      workspace_id: 'workspace-1',
      lifecycle_state: 'stopped',
    })
    const failed = session({ id: 'failed', workspace_id: 'workspace-2', lifecycle_state: 'failed' })
    const tree = normalizedSessionWorkspaceTree([
      treeNode('workspace-1', [openedRunning, stopped]),
      treeNode('workspace-2', [failed]),
    ])

    const targets = selectedSessionWorkspaceTreeTargets(tree, [
      { workspaceId: openedRunning.workspace_id, sessionId: openedRunning.id },
      { workspaceId: stopped.workspace_id, sessionId: stopped.id },
      { workspaceId: failed.workspace_id, sessionId: failed.id },
      { workspaceId: 'workspace-3', sessionId: 'removed' },
    ])

    expect(targets).toEqual([openedRunning])
  })

  it('closes sessions serially and applies each response before closing the next session', async () => {
    const first = session({ id: 'first' })
    const second = session({ id: 'second' })
    const events: string[] = []
    const closeSession = vi.fn(async (_workspaceId: string, sessionId: string) => {
      events.push(`close:${sessionId}`)
      return session({ id: sessionId, lifecycle_state: 'stopped' })
    })
    const applySession = vi.fn((updated: SessionSummary) => {
      events.push(`apply:${updated.id}`)
    })

    await closeSessionsSerially([first, second], closeSession, applySession)

    expect(events).toEqual(['close:first', 'apply:first', 'close:second', 'apply:second'])
    expect(closeSession).toHaveBeenNthCalledWith(1, first.workspace_id, first.id)
    expect(closeSession).toHaveBeenNthCalledWith(2, second.workspace_id, second.id)
  })

  it('stops the sequence when a close request fails', async () => {
    const first = session({ id: 'first' })
    const second = session({ id: 'second' })
    const failure = new Error('close failed')
    const closeSession = vi
      .fn<(workspaceId: string, sessionId: string) => Promise<SessionSummary>>()
      .mockRejectedValueOnce(failure)
    const applySession = vi.fn()

    await expect(closeSessionsSerially([first, second], closeSession, applySession)).rejects.toBe(
      failure,
    )

    expect(closeSession).toHaveBeenCalledTimes(1)
    expect(closeSession).toHaveBeenCalledWith(first.workspace_id, first.id)
    expect(applySession).not.toHaveBeenCalled()
  })
})
