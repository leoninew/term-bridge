import { DOC_PAGES, type DocLanguage, type DocPageId } from './catalog'
import { extractDocHeadings, renderDocMarkdown, type DocHeading } from './markdown'

interface DocFrontmatter {
  id: DocPageId
  order: number
  title: string
  description: string
  next?: DocPageId
}

export type DocContentBlock =
  | { kind: 'html'; value: string }
  | { kind: 'screenshot'; value: string }

export interface DocPage {
  language: DocLanguage
  id: DocPageId
  order: number
  title: string
  description: string
  next?: DocPageId
  blocks: DocContentBlock[]
  headings: DocHeading[]
}

const rawDocuments = import.meta.glob('../../../../docs/user-guide/*/*.md', {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>

function parseDocument(source: string): { frontmatter: DocFrontmatter; body: string } {
  const match = /^---\n([\s\S]*?)\n---\n([\s\S]*)$/.exec(source)
  if (!match) throw new Error('Documentation source must start with frontmatter.')

  const entries = match[1].split('\n').flatMap((line) => {
    const separator = line.indexOf(':')
    if (separator < 0) return []
    const key = line.slice(0, separator).trim()
    const value = line.slice(separator + 1).trim().replace(/^"|"$/g, '')
    return [[key, value] as const]
  })
  const values = Object.fromEntries(entries)
  const id = values.id as DocPageId
  const order = Number(values.order)
  const title = values.title
  const description = values.description
  const next = values.next as DocPageId | undefined

  if (!id || !Number.isFinite(order) || !title || !description) {
    throw new Error('Documentation frontmatter must include id, order, title, and description.')
  }

  return { frontmatter: { id, order, title, description, next }, body: match[2] }
}

function toContentBlocks(body: string): DocContentBlock[] {
  const normalized = body.replace(/:::screenshot ([a-z0-9-]+):::/g, '\n:::\nscreenshot $1\n:::\n')
  const tokens = normalized.split(/(:::\s*screenshot [a-z0-9-]+\s*:::)/g)
  return tokens.reduce<DocContentBlock[]>((blocks, token) => {
    const screenshot = /^:::\s*screenshot ([a-z0-9-]+)\s*:::$/.exec(token.trim())
    if (screenshot) {
      blocks.push({ kind: 'screenshot', value: screenshot[1] })
    } else if (token.trim() !== '') {
      blocks.push({ kind: 'html', value: renderDocMarkdown(token) })
    }
    return blocks
  }, [])
}

function loadPages(language: DocLanguage): DocPage[] {
  const pages = Object.entries(rawDocuments)
    .filter(([path]) => path.includes(`/user-guide/${language}/`))
    .map(([, source]) => {
      const { frontmatter, body } = parseDocument(source)
      return {
        language,
        ...frontmatter,
        blocks: toContentBlocks(body),
        headings: extractDocHeadings(body),
      }
    })
    .sort((left, right) => left.order - right.order)

  const expectedIds = DOC_PAGES.map((page) => page.id)
  if (pages.length !== expectedIds.length || pages.some((page, index) => page.id !== expectedIds[index])) {
    throw new Error(`Documentation pages for ${language} do not match the catalog.`)
  }
  return pages
}

const pagesByLanguage = {
  'zh-CN': loadPages('zh-CN'),
  'en-US': loadPages('en-US'),
} satisfies Record<DocLanguage, DocPage[]>

export function getDocPages(language: DocLanguage): DocPage[] {
  return pagesByLanguage[language]
}
