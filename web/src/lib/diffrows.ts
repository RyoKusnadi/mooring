// Turns the hunks of a diff into flat rows for the diff view: one header row per hunk, then its lines
// with the old and the new line number where they apply.
import type { Hunk, LineKind } from './gitapi'

export type DiffRow =
  | { type: 'hunk'; text: string; h: number }
  | { type: 'line'; kind: LineKind; old: number | null; now: number | null; text: string; h: number; l: number }

export function diffRows(hunks: Hunk[]): DiffRow[] {
  const rows: DiffRow[] = []
  hunks.forEach((hunk, hi) => {
    rows.push({ type: 'hunk', text: hunkText(hunk) + (hunk.header ? ', near ' + hunk.header : ''), h: hi })
    let old = hunk.oldStart
    let now = hunk.newStart
    hunk.lines.forEach(([kind, text], l) => {
      const at = { h: hi, l }
      if (kind === '+') rows.push({ type: 'line', kind, old: null, now: now++, text, ...at })
      else if (kind === '-') rows.push({ type: 'line', kind, old: old++, now: null, text, ...at })
      else if (kind === ' ') rows.push({ type: 'line', kind, old: old++, now: now++, text, ...at })
      else rows.push({ type: 'line', kind, old: null, now: null, text: 'No newline at end of file', ...at })
    })
  })
  return rows
}

function hunkText(h: Hunk): string {
  if (h.oldLines === 0) return `${h.newLines === 1 ? 'Added line' : 'Added lines'} ${range(h.newStart, h.newLines)}`
  if (h.newLines === 0) return `${h.oldLines === 1 ? 'Removed line' : 'Removed lines'} ${range(h.oldStart, h.oldLines)}`
  const [a, b] = [range(h.oldStart, h.oldLines), range(h.newStart, h.newLines)]
  return a === b ? `Lines ${a} changed` : `Lines ${a} became ${b}`
}

function range(start: number, lines: number): string {
  return lines === 1 ? String(start) : `${start} to ${start + lines - 1}`
}

/** Added and removed line counts, for the tab's summary. */
export function diffCounts(hunks: Hunk[]): { added: number; removed: number } {
  let added = 0
  let removed = 0
  for (const h of hunks) for (const [k] of h.lines) k === '+' ? added++ : k === '-' ? removed++ : 0
  return { added, removed }
}
