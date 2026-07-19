export type TerminalKeepAliveConfig = {
  maxHotTerminals: number
  disposeDelayMs: number
}

export const defaultTerminalKeepAliveConfig: TerminalKeepAliveConfig = {
  maxHotTerminals: 4,
  disposeDelayMs: 30_000,
}

export const minHotTerminals = 1
export const maxHotTerminalsLimit = 16
export const minDisposeDelayMs = 0
export const maxDisposeDelayMs = 600_000

export type TerminalKeepAliveConfigInput = Partial<{
  maxHotTerminals: number
  disposeDelayMs: number
}>

export function clampTerminalKeepAliveConfig(
  input: TerminalKeepAliveConfigInput | null | undefined,
): TerminalKeepAliveConfig {
  return {
    maxHotTerminals: clampInt(
      input?.maxHotTerminals,
      defaultTerminalKeepAliveConfig.maxHotTerminals,
      minHotTerminals,
      maxHotTerminalsLimit,
    ),
    disposeDelayMs: clampInt(
      input?.disposeDelayMs,
      defaultTerminalKeepAliveConfig.disposeDelayMs,
      minDisposeDelayMs,
      maxDisposeDelayMs,
    ),
  }
}

function clampInt(value: unknown, fallback: number, min: number, max: number): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return fallback
  }
  const rounded = Math.trunc(value)
  if (rounded < min) {
    return min
  }
  if (rounded > max) {
    return max
  }
  return rounded
}