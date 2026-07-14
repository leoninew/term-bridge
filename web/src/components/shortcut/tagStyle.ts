export const SHORTCUT_TAG_HUES = [199, 262, 160, 43, 350, 187] as const

export function shortcutTagHue(tag: string) {
  let hash = 0
  for (let index = 0; index < tag.length; index += 1) {
    hash = (hash * 31 + tag.charCodeAt(index)) >>> 0
  }
  return SHORTCUT_TAG_HUES[hash % SHORTCUT_TAG_HUES.length]
}

export function shortcutTagStyle(tag: string) {
  const hue = shortcutTagHue(tag)
  return {
    background: `color-mix(in srgb, hsl(${hue} 70% 48%) 16%, transparent)`,
    color: `color-mix(in srgb, hsl(${hue} 62% 42%) 78%, var(--color-text))`,
  }
}
