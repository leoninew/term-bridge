/** Shared Tailwind class strings for /sessions workbench. */

export const sessionPanelTextActionClass =
  'inline-flex h-10 w-fit items-center gap-1.5 rounded-md px-2.5 text-sm text-[var(--color-text-muted)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)] sm:h-8 sm:px-1.5'

export const sessionMenuItemClass =
  'flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none hover:bg-[var(--color-control-hover)] focus:bg-[var(--color-control-hover)]'

export const sessionMenuItemInteractiveClass =
  'flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none data-[disabled]:cursor-not-allowed data-[disabled]:opacity-50 data-[highlighted]:bg-[var(--color-control-hover)]'

export const sessionDropdownContentClass =
  'z-50 min-w-44 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] p-1 text-sm text-[var(--color-text)] shadow-xl'

export const sessionIconButtonClass =
  'inline-flex size-8 shrink-0 items-center justify-center rounded-md text-[var(--color-text-subtle)] outline-none hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus:bg-[var(--color-control-hover)] focus:text-[var(--color-text)]'

/** Tree node hover-reveal action cluster. */
export const treeNodeActionsClass =
  'flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity duration-100 group-hover:opacity-100 group-focus-within:opacity-100 focus-within:opacity-100 max-md:opacity-100 [@media(hover:none)]:opacity-100'

/** Compact tree icon button (sidebar sessions). */
export const treeNodeActionClass =
  'inline-flex size-[18px] min-h-[18px] min-w-[18px] items-center justify-center rounded-sm border-0 bg-transparent p-0 text-[var(--color-text-muted)] outline-none hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus-visible:bg-[var(--color-control-hover)] focus-visible:text-[var(--color-text)] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-[var(--color-primary-border)] max-md:size-7 max-md:min-h-7 max-md:min-w-7'

export const sidebarStatusCardClass =
  'rounded-md border border-dashed border-[var(--color-border)] bg-[var(--color-surface-muted)] p-2 text-sm text-[var(--color-text-muted)]'

export const sidebarHeaderIconButtonClass =
  'inline-flex size-7 shrink-0 items-center justify-center rounded-md border border-[var(--color-border)] bg-[var(--color-control-bg)] text-[var(--color-text-muted)] outline-none hover:border-[var(--color-border-strong)] hover:bg-[var(--color-control-hover)] hover:text-[var(--color-text)] focus-visible:border-[var(--color-border-strong)] focus-visible:bg-[var(--color-control-hover)] focus-visible:text-[var(--color-text)]'
