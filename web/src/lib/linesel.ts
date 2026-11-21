// Choosing lines of a diff to stage or unstage. Pure helpers over the diff's flat rows (see diffrows.ts).
// A choice is a set of row indexes; only added and removed lines can be chosen.
import type { Diff } from './gitapi'
import type { DiffRow } from './diffrows'

export const isChangeRow = (r: DiffRow | undefined): boolean => !!r && r.type === 'line' && (r.kind === '+' || r.kind === '-')

/** Row indexes of every line that can be chosen. */
export function changeRows(rows: DiffRow[]): number[] {
  const out: number[] = []
  rows.forEach((r, i) => isChangeRow(r) && out.push(i))
  return out
}

/** The changeable rows from `a` to `b`, in either order. */
export function rangeRows(rows: DiffRow[], a: number, b: number): number[] {
  const [lo, hi] = a <= b ? [a, b] : [b, a]
  const out: number[] = []
  for (let i = lo; i <= hi; i++) if (isChangeRow(rows[i])) out.push(i)
  return out
}

/** Every changeable row of the hunk that row `i` belongs to. */
export function blockRows(rows: DiffRow[], i: number): number[] {
  const h = rows[i]?.h
  if (h === undefined) return []
  const out: number[] = []
  rows.forEach((r, k) => r.h === h && isChangeRow(r) && out.push(k))
  return out
}

/** Chosen rows as the contract's `[hunk, line]` pairs, in diff order. */
export function toPairs(rows: DiffRow[], chosen: Iterable<number>): [number, number][] {
  const pairs: [number, number][] = []
  for (const i of [...chosen].sort((a, b) => a - b)) {
    const r = rows[i]
    if (r?.type === 'line' && isChangeRow(r)) pairs.push([r.h, r.l])
  }
  return pairs
}

/** The next changeable row after (dir 1) or before (dir -1) row `from`, or -1. */
export function stepRow(rows: DiffRow[], from: number, dir: 1 | -1): number {
  for (let i = from + dir; i >= 0 && i < rows.length; i += dir) if (isChangeRow(rows[i])) return i
  return -1
}

export type NoChoose = '' | 'binary' | 'large' | 'conflict' | 'rename' | 'old'

/** Why lines of this diff cannot be chosen, `''` when they can. */
export function whyNotChoose(diff: Diff, staged: boolean, conflicted: boolean): NoChoose {
  if (diff.binary) return 'binary'
  if (diff.tooLarge) return 'large'
  if (conflicted) return 'conflict'
  if (staged && diff.origPath) return 'rename'
  if (!diff.version) return 'old'
  return ''
}
