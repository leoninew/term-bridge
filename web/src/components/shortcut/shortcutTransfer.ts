import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'
import type { CreateShortcutReq } from '../../gen/proto/termbridge/agent/v1/shortcut'

export type ShortcutExportItem = {
  name: string
  command: string
  description?: string
  icon?: string
  enabled?: boolean
  tags?: string[]
}

export type ShortcutImportResult =
  | { ok: true; items: ShortcutExportItem[] }
  | { ok: false; error: 'invalid_json' | 'not_array' | 'invalid_item'; index?: number }

function optionalTrimmedString(value: unknown): string | undefined {
  if (value == null) {
    return undefined
  }
  if (typeof value !== 'string') {
    return undefined
  }
  const text = value.trim()
  return text === '' ? undefined : text
}

function normalizeTags(value: unknown): string[] | undefined {
  if (value == null) {
    return undefined
  }
  if (!Array.isArray(value)) {
    return undefined
  }
  const tags: string[] = []
  const seen = new Set<string>()
  for (const entry of value) {
    if (typeof entry !== 'string') {
      continue
    }
    const tag = entry.trim()
    if (tag === '' || seen.has(tag)) {
      continue
    }
    seen.add(tag)
    tags.push(tag)
  }
  return tags
}

export function toShortcutExportItem(shortcut: Shortcut): ShortcutExportItem {
  const item: ShortcutExportItem = {
    name: shortcut.name,
    command: shortcut.command,
  }
  const description = optionalTrimmedString(shortcut.description)
  if (description !== undefined) {
    item.description = description
  }
  const icon = optionalTrimmedString(shortcut.icon)
  if (icon !== undefined) {
    item.icon = icon
  }
  if (shortcut.enabled !== undefined) {
    item.enabled = shortcut.enabled
  }
  if (shortcut.tags?.length) {
    item.tags = [...shortcut.tags]
  }
  return item
}

export function parseShortcutImportJson(text: string): ShortcutImportResult {
  let parsed: unknown
  try {
    parsed = JSON.parse(text)
  } catch {
    return { ok: false, error: 'invalid_json' }
  }
  if (!Array.isArray(parsed)) {
    return { ok: false, error: 'not_array' }
  }
  const items: ShortcutExportItem[] = []
  for (let index = 0; index < parsed.length; index += 1) {
    const entry = parsed[index]
    if (entry == null || typeof entry !== 'object' || Array.isArray(entry)) {
      return { ok: false, error: 'invalid_item', index }
    }
    const record = entry as Record<string, unknown>
    const name = optionalTrimmedString(record.name)
    const command = optionalTrimmedString(record.command)
    if (!name || !command) {
      return { ok: false, error: 'invalid_item', index }
    }
    const item: ShortcutExportItem = { name, command }
    const description = optionalTrimmedString(record.description)
    if (description !== undefined) {
      item.description = description
    }
    const icon = optionalTrimmedString(record.icon)
    if (icon !== undefined) {
      item.icon = icon
    }
    if (typeof record.enabled === 'boolean') {
      item.enabled = record.enabled
    }
    const tags = normalizeTags(record.tags)
    if (tags !== undefined) {
      item.tags = tags
    }
    items.push(item)
  }
  return { ok: true, items }
}

export function toCreateShortcutRequest(item: ShortcutExportItem): CreateShortcutReq {
  return {
    name: item.name,
    command: item.command,
    description: item.description,
    icon: item.icon,
    enabled: item.enabled,
    tags: item.tags ?? [],
  }
}

export function formatShortcutExportFilename(date = new Date()): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  const second = String(date.getSeconds()).padStart(2, '0')
  return `termbridge-shortcuts-${year}${month}${day}-${hour}${minute}${second}.json`
}

export function downloadJsonFile(filename: string, value: unknown) {
  const blob = new Blob([`${JSON.stringify(value, null, 2)}\n`], {
    type: 'application/json;charset=utf-8',
  })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.rel = 'noopener'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(url)
}
