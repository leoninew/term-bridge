export type LaunchMethod = 'shortcut' | 'command'

export function normalizeLaunchMethod(commandSource: string | null | undefined): LaunchMethod {
  return commandSource === 'shortcut' ? 'shortcut' : 'command'
}

export function isShortcutLaunchMethod(commandSource: string | null | undefined): boolean {
  return normalizeLaunchMethod(commandSource) === 'shortcut'
}

/** i18n key for launch method type labels (dialog.shortcut | dialog.directCommand). */
export function launchMethodLabelKey(
  commandSource: string | null | undefined,
): 'dialog.shortcut' | 'dialog.directCommand' {
  return isShortcutLaunchMethod(commandSource) ? 'dialog.shortcut' : 'dialog.directCommand'
}
