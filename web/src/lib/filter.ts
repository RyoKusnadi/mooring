export interface PaletteItem {
  id: string
  label: string
  hint?: string
  run: () => void
}

// Palette matching: every typed word must appear in the label, in any order. Earlier hits rank first.
export function filterItems<T extends { label: string }>(items: T[], query: string): T[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  if (words.length === 0) return items
  const scored: { item: T; score: number }[] = []
  for (const item of items) {
    const label = item.label.toLowerCase()
    let score = 0
    let ok = true
    for (const w of words) {
      const at = label.indexOf(w)
      if (at < 0) {
        ok = false
        break
      }
      score += at === 0 || label[at - 1] === ' ' ? at : at + 20
    }
    if (ok) scored.push({ item, score })
  }
  return scored.sort((a, b) => a.score - b.score).map((s) => s.item)
}

/** Text that reads like a file name or path ("main.go", "src/lib"), so the palette offers to list files. */
export function looksLikeFile(text: string): boolean {
  const t = text.trim()
  if (t === '' || /\s/.test(t)) return false
  return /\.[A-Za-z0-9]{1,10}$/.test(t) || t.includes('/')
}
