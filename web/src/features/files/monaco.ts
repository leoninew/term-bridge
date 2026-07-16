import * as monaco from 'monaco-editor'
import type { RuntimeTarget } from '../runtimeTarget'
import { type AppTheme } from '../../store/theme'
import EditorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'
import JsonWorker from 'monaco-editor/esm/vs/language/json/json.worker?worker'
import CssWorker from 'monaco-editor/esm/vs/language/css/css.worker?worker'
import HtmlWorker from 'monaco-editor/esm/vs/language/html/html.worker?worker'
import TypeScriptWorker from 'monaco-editor/esm/vs/language/typescript/ts.worker?worker'

export { monaco }

const languageByExtension: Record<string, string> = {
  bash: 'shell',
  c: 'c',
  cc: 'cpp',
  css: 'css',
  go: 'go',
  h: 'cpp',
  html: 'html',
  java: 'java',
  js: 'javascript',
  json: 'json',
  jsx: 'javascript',
  md: 'markdown',
  py: 'python',
  rs: 'rust',
  sh: 'shell',
  sql: 'sql',
  ts: 'typescript',
  tsx: 'typescript',
  vue: 'html',
  xml: 'xml',
  yaml: 'yaml',
  yml: 'yaml',
}

const monacoThemes = {
  light: 'termbridge-light',
  dark: 'termbridge-dark',
} as const

let configured = false

export function configureMonaco() {
  if (configured) {
    return
  }
  configured = true
  self.MonacoEnvironment = {
    getWorker(_: string, label: string) {
      if (label === 'json') {
        return new JsonWorker()
      }
      if (label === 'css' || label === 'scss' || label === 'less') {
        return new CssWorker()
      }
      if (label === 'html' || label === 'handlebars' || label === 'razor') {
        return new HtmlWorker()
      }
      if (label === 'typescript' || label === 'javascript') {
        return new TypeScriptWorker()
      }
      return new EditorWorker()
    },
  }
  monaco.editor.defineTheme(monacoThemes.dark, {
    base: 'vs-dark',
    inherit: true,
    rules: [],
    colors: {
      'editor.background': '#090d14',
      'editorGutter.background': '#090d14',
      'editorLineNumber.foreground': '#64748b',
      'editorLineNumber.activeForeground': '#94a3b8',
    },
  })
  monaco.editor.defineTheme(monacoThemes.light, {
    base: 'vs',
    inherit: true,
    rules: [],
    colors: {
      'editor.background': '#f8fafc',
      'editorGutter.background': '#f8fafc',
      'editorLineNumber.foreground': '#64748b',
      'editorLineNumber.activeForeground': '#475569',
    },
  })
}

export function applyMonacoTheme(theme: AppTheme) {
  configureMonaco()
  monaco.editor.setTheme(monacoThemes[theme])
}

export function languageForPath(path: string): string {
  const name = path.split('/').at(-1)?.toLowerCase() ?? ''
  if (name === 'dockerfile') {
    return 'dockerfile'
  }
  if (name === 'makefile') {
    return 'makefile'
  }
  const extension = name.split('.').at(-1)
  return extension && extension !== name
    ? (languageByExtension[extension] ?? 'plaintext')
    : 'plaintext'
}

export function fileUri(
  target: RuntimeTarget,
  workspaceId: string,
  path: string,
  side = 'content',
) {
  const targetId = target.mode === 'local' ? 'local' : `cloud-${target.deviceId}`
  return monaco.Uri.from({
    scheme: 'termbridge',
    authority: encodeURIComponent(`${targetId}-${workspaceId}`),
    path: `/${encodeURIComponent(side)}/${path.split('/').map(encodeURIComponent).join('/')}`,
  })
}

export const editorOptions: monaco.editor.IStandaloneEditorConstructionOptions = {
  automaticLayout: false,
  minimap: { enabled: false },
  stickyScroll: { enabled: false },
  scrollBeyondLastLine: false,
  fontSize: 13,
  tabSize: 2,
  renderWhitespace: 'selection',
  wordWrap: 'on',
}

export const diffEditorOptions: monaco.editor.IStandaloneDiffEditorConstructionOptions = {
  automaticLayout: false,
  readOnly: true,
  originalEditable: false,
  renderSideBySide: true,
  minimap: { enabled: false },
  stickyScroll: { enabled: false },
  scrollBeyondLastLine: false,
  fontSize: 13,
  wordWrap: 'on',
}
