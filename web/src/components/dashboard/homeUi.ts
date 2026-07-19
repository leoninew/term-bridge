export const homePageMainClass =
  'flex flex-col overflow-y-auto px-3 py-4 text-sm sm:px-5 sm:py-6 md:px-6 md:py-8'

export const homePageContentClass =
  'mx-auto flex w-full max-w-5xl flex-1 flex-col justify-center gap-4 sm:gap-5'

export const dashboardPageMainClass =
  'flex items-start justify-center px-4 py-5 text-sm sm:items-center sm:px-6 sm:py-8 lg:px-8 2xl:px-12'

export const dashboardPageContentClass = 'mx-auto flex w-full max-w-[1200px] flex-col gap-5'

/** CTA used in home hero (cloud, wrap row). */
export const homeCtaClass =
  'button !rounded-xl h-11 justify-center gap-1.5 px-4 text-sm sm:h-9 sm:px-3'

/** CTA used in local home hero (stack full-width on mobile). */
export const homeCtaLocalClass =
  'button !rounded-xl h-11 w-full justify-center gap-1.5 px-4 text-sm sm:h-9 sm:w-auto sm:px-3'

/** Text action used in panel headers (home / dashboard). */
export const homePanelActionClass =
  'inline-flex h-10 items-center gap-1.5 rounded-md px-2.5 text-sm text-[var(--color-text-muted)] outline-none hover:text-[var(--color-text)] focus:text-[var(--color-text)] sm:h-8 sm:px-1.5'

export const homeFooterLinkClass =
  'outline-none transition-colors hover:text-[var(--color-text-muted)] focus-visible:text-[var(--color-text-muted)]'

/** Icon button in list rows. */
export const listIconActionClass =
  'inline-flex size-10 items-center justify-center rounded-md outline-none disabled:cursor-not-allowed sm:size-8'

export const listIconDangerActionClass =
  'inline-flex size-10 items-center justify-center rounded-md text-[var(--color-text-muted)] outline-none hover:bg-[var(--color-surface-muted)] hover:text-[var(--color-danger-text)] focus:bg-[var(--color-surface-muted)] focus:text-[var(--color-danger-text)] disabled:cursor-not-allowed disabled:opacity-60 sm:size-8'
