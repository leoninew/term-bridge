/**
 * Titlebar left app icon (`a.window-appicon`) is VS Code chrome — already laid out.
 * Brand it as TermBridge "TB" and optionally navigate on click (back to sessions).
 * Workbench lives in an open ShadowRoot, so styles + listeners target that root.
 */

const STYLE_ATTR = 'data-termbridge-appicon-style'

const APP_ICON_CSS = `
/* Reuse native .window-appicon slot; swap product icon for TB brand mark */
.monaco-workbench .part.titlebar > .titlebar-container > .titlebar-left > .window-appicon {
  cursor: pointer;
  text-decoration: none;
}

.monaco-workbench .part.titlebar > .titlebar-container > .titlebar-left > .window-appicon:not(.codicon) {
  background-image: none !important;
  display: flex;
  align-items: center;
  justify-content: center;
}

.monaco-workbench .part.titlebar > .titlebar-container > .titlebar-left > .window-appicon:not(.codicon)::before {
  content: 'TB';
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 5px;
  background: #2563eb;
  color: #fff;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.02em;
  font-family: system-ui, 'Segoe UI', sans-serif;
  line-height: 1;
  box-shadow: 0 1px 2px rgb(0 0 0 / 0.35);
}

.monaco-workbench .part.titlebar > .titlebar-container > .titlebar-left > .window-appicon:hover:not(.codicon)::before,
.monaco-workbench .part.titlebar > .titlebar-container > .titlebar-left > .window-appicon:focus-visible:not(.codicon)::before {
  background: #3b82f6;
}
`

export function getWorkbenchShadowRoot(container: HTMLElement): ShadowRoot | null {
  const host = container.querySelector('.termbridge-code-workbench-host')
  return host instanceof HTMLElement ? host.shadowRoot : null
}

export function ensureWorkbenchAppIconStyles(shadowRoot: ShadowRoot): void {
  if (shadowRoot.querySelector(`style[${STYLE_ATTR}]`)) {
    return
  }
  const style = document.createElement('style')
  style.setAttribute(STYLE_ATTR, '')
  style.textContent = APP_ICON_CSS
  shadowRoot.appendChild(style)
}

export type WorkbenchAppIconBindOptions = {
  onNavigate: () => void
  ariaLabel: string
  /** How long to wait for titlebar to create the icon (ms). */
  timeoutMs?: number
}

/**
 * Styles + wires click on native `a.window-appicon` → onNavigate.
 * Returns a dispose function. Safe if icon never appears (e.g. some platforms).
 */
export function bindWorkbenchAppIconNavigation(
  container: HTMLElement,
  options: WorkbenchAppIconBindOptions,
): () => void {
  const shadowRoot = getWorkbenchShadowRoot(container)
  if (!shadowRoot) {
    return () => {}
  }

  ensureWorkbenchAppIconStyles(shadowRoot)

  let disposed = false
  let icon: HTMLAnchorElement | null = null
  let observer: MutationObserver | null = null
  let timer: ReturnType<typeof setTimeout> | null = null

  const onClick = (event: Event) => {
    event.preventDefault()
    event.stopPropagation()
    options.onNavigate()
  }

  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      event.stopPropagation()
      options.onNavigate()
    }
  }

  const unbindIcon = () => {
    if (!icon) {
      return
    }
    icon.removeEventListener('click', onClick)
    icon.removeEventListener('keydown', onKeyDown)
    icon.removeAttribute('aria-label')
    icon.removeAttribute('title')
    icon.removeAttribute('tabindex')
    icon.removeAttribute('role')
    icon = null
  }

  const bindIcon = (el: HTMLAnchorElement) => {
    unbindIcon()
    icon = el
    icon.setAttribute('role', 'link')
    icon.setAttribute('tabindex', '0')
    icon.setAttribute('aria-label', options.ariaLabel)
    icon.title = options.ariaLabel
    icon.addEventListener('click', onClick)
    icon.addEventListener('keydown', onKeyDown)
  }

  const tryBind = (): boolean => {
    const el = shadowRoot.querySelector('a.window-appicon')
    if (el instanceof HTMLAnchorElement) {
      bindIcon(el)
      return true
    }
    return false
  }

  if (!tryBind()) {
    observer = new MutationObserver(() => {
      if (tryBind()) {
        observer?.disconnect()
        observer = null
        if (timer) {
          clearTimeout(timer)
          timer = null
        }
      }
    })
    observer.observe(shadowRoot, { childList: true, subtree: true })
    timer = setTimeout(() => {
      observer?.disconnect()
      observer = null
      timer = null
    }, options.timeoutMs ?? 8000)
  }

  return () => {
    if (disposed) {
      return
    }
    disposed = true
    if (timer) {
      clearTimeout(timer)
      timer = null
    }
    observer?.disconnect()
    observer = null
    unbindIcon()
  }
}
