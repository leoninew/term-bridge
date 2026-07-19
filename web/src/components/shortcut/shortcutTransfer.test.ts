import { describe, expect, it } from 'vitest'
import {
  formatShortcutExportFilename,
  parseShortcutImportJson,
  toCreateShortcutRequest,
  toShortcutExportItem,
} from './shortcutTransfer'
import type { Shortcut } from '../../gen/proto/termbridge/agent/v1/shortcut'

describe('shortcutTransfer', () => {
  it('exports only reusable fields', () => {
    const shortcut = {
      id: 'shortcut-1',
      name: 'Shell',
      command: 'bash',
      description: ' login ',
      icon: ' terminal ',
      enabled: false,
      tags: ['dev', 'shell'],
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-02T00:00:00Z',
      last_used_at: '2026-01-03T00:00:00Z',
    } as Shortcut

    expect(toShortcutExportItem(shortcut)).toEqual({
      name: 'Shell',
      command: 'bash',
      description: 'login',
      icon: 'terminal',
      enabled: false,
      tags: ['dev', 'shell'],
    })
  })

  it('parses a valid json array and ignores id timestamps', () => {
    const result = parseShortcutImportJson(
      JSON.stringify([
        {
          id: 'old-id',
          name: ' Shell ',
          command: ' bash ',
          description: ' desc ',
          enabled: true,
          tags: [' a ', 'a', ''],
          created_at: 'x',
        },
      ]),
    )

    expect(result).toEqual({
      ok: true,
      items: [
        {
          name: 'Shell',
          command: 'bash',
          description: 'desc',
          enabled: true,
          tags: ['a'],
        },
      ],
    })
  })

  it('rejects non-array payloads and invalid items', () => {
    expect(parseShortcutImportJson('{ "items": [] }')).toEqual({
      ok: false,
      error: 'not_array',
    })
    expect(parseShortcutImportJson('not-json')).toEqual({
      ok: false,
      error: 'invalid_json',
    })
    expect(parseShortcutImportJson(JSON.stringify([{ name: '', command: 'x' }]))).toEqual({
      ok: false,
      error: 'invalid_item',
      index: 0,
    })
  })

  it('maps export items to create requests', () => {
    expect(
      toCreateShortcutRequest({
        name: 'Shell',
        command: 'bash',
        description: 'desc',
        icon: 'terminal',
        enabled: false,
        tags: ['dev'],
      }),
    ).toEqual({
      name: 'Shell',
      command: 'bash',
      description: 'desc',
      icon: 'terminal',
      enabled: false,
      tags: ['dev'],
    })
  })

  it('formats export filenames', () => {
    expect(formatShortcutExportFilename(new Date(2026, 6, 19, 8, 30, 5))).toBe(
      'termbridge-shortcuts-20260719-083005.json',
    )
  })
})
