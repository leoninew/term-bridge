import { describe, expect, it } from 'vitest'
import { FileType } from '../../gen/proto/termbridge/agent/v1/file'
import { decodeFileStat, decodeFileType } from './wire'

describe('workbench wire decode', () => {
  it('maps protojson enum names to numeric FileType', () => {
    expect(decodeFileType('FILE_TYPE_DIRECTORY')).toBe(FileType.FILE_TYPE_DIRECTORY)
    expect(decodeFileType('FILE_TYPE_FILE')).toBe(FileType.FILE_TYPE_FILE)
    expect(decodeFileType(2)).toBe(FileType.FILE_TYPE_DIRECTORY)
  })

  it('normalizes FileStat int64 strings', () => {
    const stat = decodeFileStat({
      type: 'FILE_TYPE_FILE',
      ctime: '100',
      mtime: '200',
      size: '12',
      permissions: 'FILE_PERMISSION_UNSPECIFIED',
      etag: 'rev',
    })
    expect(stat).toEqual({
      type: FileType.FILE_TYPE_FILE,
      ctime: 100,
      mtime: 200,
      size: 12,
      permissions: 0,
      etag: 'rev',
    })
  })
})
