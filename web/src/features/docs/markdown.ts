import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'
import anchor from 'markdown-it-anchor'
import attrs from 'markdown-it-attrs'

const markdown = new MarkdownIt({
  html: false,
  linkify: true,
  typographer: true,
})
  .use(attrs)
  .use(anchor, {
    slugify: (value: string) => value,
    permalink: false,
  })

export function renderDocMarkdown(source: string): string {
  return DOMPurify.sanitize(markdown.render(source), {
    ALLOWED_ATTR: ['class', 'href', 'id', 'target', 'rel'],
    ALLOW_UNKNOWN_PROTOCOLS: false,
  })
}

export interface DocHeading {
  id: string
  text: string
  level: 2 | 3
}

export function extractDocHeadings(source: string): DocHeading[] {
  return source
    .split('\n')
    .flatMap((line) => {
      const match = /^(#{2,3})\s+(.+?)\s+\{#([a-z0-9-]+)\}\s*$/.exec(line)
      if (!match) return []
      return [
        {
          level: match[1].length as 2 | 3,
          text: match[2],
          id: match[3],
        },
      ]
    })
}
