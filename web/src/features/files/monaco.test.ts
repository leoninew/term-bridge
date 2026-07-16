import { describe, expect, it, vi } from 'vitest'

vi.mock('monaco-editor', () => ({
  Uri: { from: vi.fn((value) => value) },
  editor: {
    defineTheme: vi.fn(),
    setTheme: vi.fn(),
  },
}))
vi.mock('monaco-editor/esm/vs/editor/editor.worker?worker', () => ({ default: class {} }))
vi.mock('monaco-editor/esm/vs/language/json/json.worker?worker', () => ({ default: class {} }))
vi.mock('monaco-editor/esm/vs/language/css/css.worker?worker', () => ({ default: class {} }))
vi.mock('monaco-editor/esm/vs/language/html/html.worker?worker', () => ({ default: class {} }))
vi.mock('monaco-editor/esm/vs/language/typescript/ts.worker?worker', () => ({ default: class {} }))

import { fileUri, languageForPath } from './monaco'

describe('Monaco file helpers', () => {
  it('selects the expected language with plaintext fallback', () => {
    expect(languageForPath('cmd/server.go')).toBe('go')
    expect(languageForPath('web/src/main.ts')).toBe('typescript')
    expect(languageForPath('README.md')).toBe('markdown')
    expect(languageForPath('LICENSE')).toBe('plaintext')
  })

  it('scopes model URIs by runtime target and workspace', () => {
    expect(fileUri({ mode: 'local' }, 'workspace-a', 'notes.txt')).toMatchObject({
      scheme: 'termbridge',
      authority: 'local-workspace-a',
      path: '/content/notes.txt',
    })
    expect(
      fileUri({ mode: 'cloud', deviceId: 'device-a' }, 'workspace-a', 'notes.txt'),
    ).toMatchObject({
      authority: 'cloud-device-a-workspace-a',
    })
  })
})
