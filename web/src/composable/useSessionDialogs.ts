import { proxyRefs, ref } from 'vue'
import type { SessionSummary, WorkspaceSummary } from '../protocol/terminal'

export function useSessionDialogs() {
  const editDialogOpen = ref(false)
  const deleteSessionDialogOpen = ref(false)
  const removeWorkspaceDialogOpen = ref(false)
  const selectedSession = ref<SessionSummary | null>(null)
  const selectedWorkspace = ref<WorkspaceSummary | null>(null)

  function openEditDialog(session: SessionSummary) {
    selectedSession.value = session
    editDialogOpen.value = true
  }

  function openDeleteSessionDialog(session: SessionSummary) {
    selectedSession.value = session
    deleteSessionDialogOpen.value = true
  }

  function openRemoveWorkspaceDialog(workspace: WorkspaceSummary) {
    selectedWorkspace.value = workspace
    removeWorkspaceDialogOpen.value = true
  }

  function clearSelectedSession() {
    selectedSession.value = null
  }

  function clearSelectedWorkspace() {
    selectedWorkspace.value = null
  }

  return proxyRefs({
    editDialogOpen,
    deleteSessionDialogOpen,
    removeWorkspaceDialogOpen,
    selectedSession,
    selectedWorkspace,
    openEditDialog,
    openDeleteSessionDialog,
    openRemoveWorkspaceDialog,
    clearSelectedSession,
    clearSelectedWorkspace,
  })
}
