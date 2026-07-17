import { describe, expect, it } from 'vitest'
import { bytesFromWire, bytesToWire } from './bytes'

describe('workbench bytes wire encoding', () => {
  it('round-trips utf-8 text via base64', () => {
    const original = new TextEncoder().encode('hello 你好')
    const wire = bytesToWire(original)
    expect(typeof wire).toBe('string')
    const decoded = bytesFromWire(wire)
    expect(new TextDecoder().decode(decoded)).toBe('hello 你好')
  })

  it('accepts empty and Uint8Array inputs', () => {
    expect(bytesFromWire('').length).toBe(0)
    expect(bytesFromWire(new Uint8Array([1, 2, 3]))).toEqual(new Uint8Array([1, 2, 3]))
  })
})
