// Matching for "open file by name" and for showing where a search hit lies in its line.

/** Lower is better; null when the word is not in the path at all. */
function wordScore(path: string, base: string, w: string): number | null {
  const inBase = base.indexOf(w)
  if (inBase >= 0) return (inBase === 0 ? 0 : 1) + base.length / 1000
  const inPath = path.indexOf(w)
  if (inPath >= 0) return 10 + inPath / 1000
  // Letters in order, anywhere: "fsv" finds "FileSearchView".
  let at = -1
  let first = -1
  for (const ch of w) {
    at = path.indexOf(ch, at + 1)
    if (at < 0) return null
    if (first < 0) first = at
  }
  return 100 + (at - first)
}

/**
 * Paths that match every word of the query, best first. Among paths that match equally well, files opened
 * lately (`recent`, newest first) come first, then shorter paths. An empty query lists recent files, then
 * the rest, shorter paths first.
 */
export function rankPaths(paths: string[], query: string, limit = 50, recent: string[] = []): string[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  const lately = new Map(recent.map((p, i) => [p, i]))
  const order = (a: { path: string; score: number }, b: { path: string; score: number }) =>
    Math.floor(a.score) - Math.floor(b.score) || (lately.get(a.path) ?? Infinity) - (lately.get(b.path) ?? Infinity) || a.path.length - b.path.length || a.score - b.score || (a.path < b.path ? -1 : 1)
  const scored: { path: string; score: number }[] = []
  for (const path of paths) {
    const low = path.toLowerCase()
    const base = low.slice(low.lastIndexOf('/') + 1)
    let score = 0
    for (const w of words) {
      const s = wordScore(low, base, w)
      if (s === null) {
        score = -1
        break
      }
      score += s
    }
    if (score >= 0) scored.push({ path, score })
  }
  scored.sort(order)
  return scored.slice(0, limit).map((s) => s.path)
}

export interface Segment {
  text: string
  hit: boolean
}

/** Splits a line into plain and matched parts, from UTF-16 ranges. Bad ranges are ignored. */
export function segments(text: string, ranges: [number, number][]): Segment[] {
  const out: Segment[] = []
  let at = 0
  for (const [from, to] of [...ranges].sort((a, b) => a[0] - b[0])) {
    if (from < at || to <= from || to > text.length) continue
    if (from > at) out.push({ text: text.slice(at, from), hit: false })
    out.push({ text: text.slice(from, to), hit: true })
    at = to
  }
  if (at < text.length) out.push({ text: text.slice(at), hit: false })
  return out
}
