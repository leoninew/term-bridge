/** Pure helpers for workspace file-name search (no monaco side-effects). */

export function matchesFilePattern(
  relativePath: string,
  pattern: string,
  shouldGlob: boolean,
  globMatch: (pattern: string, path: string, options?: { ignoreCase?: boolean }) => boolean,
): boolean {
  if (!pattern) {
    return true
  }
  const path = relativePath.replace(/^\/+/, '')
  if (shouldGlob) {
    const withSlash = '/' + path
    return Boolean(
      globMatch(pattern, path, { ignoreCase: true }) ||
      globMatch(pattern, withSlash, { ignoreCase: true }),
    )
  }
  return path.toLowerCase().includes(pattern.toLowerCase())
}

export function relativeFromRoot(rootPath: string, resourcePath: string): string {
  const normalizedRoot = rootPath.replace(/\/+$/, '') || ''
  let rel = resourcePath
  if (normalizedRoot && (rel === normalizedRoot || rel.startsWith(normalizedRoot + '/'))) {
    rel = rel.slice(normalizedRoot.length)
  }
  return rel.replace(/^\/+/, '')
}

export function joinChildPath(dirPath: string, name: string): string {
  const base = dirPath.replace(/\/+$/, '') || ''
  const path = (base + '/' + name).replace(/\/+/g, '/')
  return path.startsWith('/') ? path : '/' + path
}

export type WalkEntry = [string, 'file' | 'dir']

export async function walkMatchPaths(options: {
  rootPath: string
  readDirectory: (path: string) => Promise<WalkEntry[]>
  pattern: string
  shouldGlob: boolean
  globMatch: (pattern: string, path: string, options?: { ignoreCase?: boolean }) => boolean
  maxResults: number
  maxDepth?: number
  maxWalkEntries?: number
  isCancelled?: () => boolean
}): Promise<{ paths: string[]; limitHit: boolean }> {
  const maxDepth = options.maxDepth ?? 40
  const maxWalkEntries = options.maxWalkEntries ?? 20_000
  const paths: string[] = []
  let walked = 0
  let limitHit = false

  const walk = async (dirPath: string, depth: number): Promise<void> => {
    if (options.isCancelled?.() || limitHit || depth > maxDepth || walked >= maxWalkEntries) {
      return
    }
    let entries: WalkEntry[]
    try {
      entries = await options.readDirectory(dirPath)
    } catch {
      return
    }
    for (const [name, kind] of entries) {
      if (options.isCancelled?.() || limitHit) {
        return
      }
      walked += 1
      if (walked > maxWalkEntries) {
        limitHit = true
        return
      }
      const childPath = joinChildPath(dirPath, name)
      const rel = relativeFromRoot(options.rootPath, childPath)
      if (matchesFilePattern(rel, options.pattern, options.shouldGlob, options.globMatch)) {
        paths.push(childPath)
        if (paths.length >= options.maxResults) {
          limitHit = true
          return
        }
      }
      if (kind === 'dir') {
        await walk(childPath, depth + 1)
      }
    }
  }

  await walk(options.rootPath, 0)
  return { paths, limitHit }
}
