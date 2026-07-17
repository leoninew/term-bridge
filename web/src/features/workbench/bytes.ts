/** ProtoJSON encodes bytes as standard base64 strings over HTTP. */

export function bytesFromWire(value: unknown): Uint8Array {
  if (value == null) {
    return new Uint8Array()
  }
  if (value instanceof Uint8Array) {
    return value
  }
  if (typeof value === 'string') {
    if (value.length === 0) {
      return new Uint8Array()
    }
    const binary = atob(value)
    const out = new Uint8Array(binary.length)
    for (let i = 0; i < binary.length; i += 1) {
      out[i] = binary.charCodeAt(i)
    }
    return out
  }
  if (Array.isArray(value)) {
    return Uint8Array.from(value as number[])
  }
  return new Uint8Array()
}

export function bytesToWire(value: Uint8Array): string {
  if (value.length === 0) {
    return ''
  }
  let binary = ''
  const chunk = 0x8000
  for (let i = 0; i < value.length; i += chunk) {
    binary += String.fromCharCode(...value.subarray(i, i + chunk))
  }
  return btoa(binary)
}
