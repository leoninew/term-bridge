import { describe, expect, it } from 'vitest'
import { walkMatchPaths } from './fileSearchMatch'

function simpleGlob(pattern: string, path: string): boolean {
  // Very small subset for tests: treat * / ** as wildcards.
  const escaped = pattern
    .replace(/[.+^${}()|[\]\\]/g, '\\$&')
    .replace(/\*\*/g, '§§')
    .replace(/\*/g, '[^/]*')
    .replace(/§§/g, '.*')
  const re = new RegExp('^' + escaped + '$', 'i')
  return re.test(path.replace(/^\/+/, '')) || re.test(path)
}

describe('fileSearchMatch', () => {
  it('walks tree and matches file patterns', async () => {
    const tree: Record<string, [string, 'file' | 'dir'][]> = {
      '/': [
        ['src', 'dir'],
        ['README.md', 'file'],
      ],
      '/src': [
        ['main.go', 'file'],
        ['util.go', 'file'],
      ],
    }

    const files = await walkMatchPaths({
      rootPath: '/',
      readDirectory: async (path) => tree[path] ?? [],
      pattern: '**/main*',
      shouldGlob: true,
      globMatch: simpleGlob,
      maxResults: 100,
    })
    expect(files.paths).toContain('/src/main.go')
    expect(files.paths).not.toContain('/src/util.go')
  })

  it('matches directory-style patterns used by explorer', async () => {
    const tree: Record<string, [string, 'file' | 'dir'][]> = {
      '/': [['src', 'dir']],
      '/src': [['main.go', 'file']],
    }
    const dirs = await walkMatchPaths({
      rootPath: '/',
      readDirectory: async (path) => tree[path] ?? [],
      // Explorer uses this shape for directory discovery.
      pattern: '**/src/**',
      shouldGlob: true,
      globMatch: simpleGlob,
      maxResults: 100,
    })
    // Children under src match **/src/**; the directory itself matches via file query **/src*.
    expect(dirs.paths).toContain('/src/main.go')
  })
})
