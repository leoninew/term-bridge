export const DOC_ROUTE_NAME = 'docs'
export const DOC_DEFAULT_LANGUAGE = 'zh-CN' as const
export const DOC_LANGUAGES = ['zh-CN', 'en-US'] as const

export type DocLanguage = (typeof DOC_LANGUAGES)[number]
export type DocPageId =
  | 'index'
  | 'quick-start'
  | 'understand-termbridge'
  | 'start-cli-sessions'
  | 'continue-sessions'
  | 'manage-workspaces'
  | 'manage-shortcuts'
  | 'launch-from-shortcuts'

export interface DocPageDefinition {
  id: DocPageId
  path: string
  navAnchor: string
}

export const DOC_PAGES: readonly DocPageDefinition[] = [
  { id: 'index', path: 'index', navAnchor: 'index' },
  { id: 'quick-start', path: 'quick-start', navAnchor: 'quick-start' },
  { id: 'understand-termbridge', path: 'understand-termbridge', navAnchor: 'understand-termbridge' },
  { id: 'start-cli-sessions', path: 'start-cli-sessions', navAnchor: 'start-cli-sessions' },
  { id: 'continue-sessions', path: 'continue-sessions', navAnchor: 'continue-sessions' },
  { id: 'manage-workspaces', path: 'manage-workspaces', navAnchor: 'manage-workspaces' },
  { id: 'manage-shortcuts', path: 'manage-shortcuts', navAnchor: 'manage-shortcuts' },
  {
    id: 'launch-from-shortcuts',
    path: 'launch-from-shortcuts',
    navAnchor: 'launch-from-shortcuts',
  },
]

export const DOC_RELEASE_URL = 'https://github.com/leoninew/TermBridge-go/releases/latest'

export const DOC_RELEASE_ASSETS = [
  'TermBridge-windows-x64.zip',
  'TermBridge-linux-x64.zip',
  'TermBridge-macos-x64.zip',
] as const

export const DOC_SCREENSHOT_IDS = [
  'cli-ready',
  'release-assets',
  'package-folder',
  'agent-started',
  'workspace-entry',
  'create-session',
  'browser-terminal',
  'shortcut-management',
  'shortcut-command-source',
] as const

export type DocScreenshotId = (typeof DOC_SCREENSHOT_IDS)[number]

export function isDocLanguage(value: unknown): value is DocLanguage {
  return typeof value === 'string' && DOC_LANGUAGES.includes(value as DocLanguage)
}
