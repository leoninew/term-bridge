export type SessionLifecycleState = 'running' | 'stopped' | 'failed'

const lifecycleStateClassNames: Record<SessionLifecycleState, string> = {
  running: 'text-[var(--color-state-running-text)]',
  stopped: 'text-[var(--color-state-stopped-text)]',
  failed: 'text-[var(--color-state-stopped-text)]',
}

export function lifecycleStateClassName(state: string): string {
  return (
    lifecycleStateClassNames[state as SessionLifecycleState] ?? 'text-[var(--color-text-muted)]'
  )
}
