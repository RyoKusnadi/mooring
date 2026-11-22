// Text for the find references and rename dialogs.
import type { FileEdits } from './lspedits'
import { segments, type Segment } from './fuzzy'

/** Reading lines for the references list stops after this many files; the rest show without their text. */
export const MAX_READ = 30

export interface RefGroup<T> {
  path: string
  items: T[]
}

/** Rows in the order given, one group per file. */
export function groupByPath<T extends { path: string }>(refs: T[]): RefGroup<T>[] {
  const out: RefGroup<T>[] = []
  for (const r of refs) {
    const last = out[out.length - 1]
    if (last && last.path === r.path) last.items.push(r)
    else out.push({ path: r.path, items: [r] })
  }
  return out
}

/** The line split so every occurrence of the name can be marked. */
export function markName(text: string, name: string): Segment[] {
  const ranges: [number, number][] = []
  if (name) for (let i = text.indexOf(name); i >= 0; i = text.indexOf(name, i + name.length)) ranges.push([i, i + name.length])
  return segments(text, ranges)
}

export const plural = (n: number, one: string) => `${n} ${one}${n === 1 ? '' : 's'}`

export function refsSummary(refs: number, files: number): string {
  return `${plural(refs, 'reference')} in ${plural(files, 'file')}`
}

export function renameSummary(files: FileEdits[]): string {
  const places = files.reduce((n, f) => n + f.edits.length, 0)
  return `Rename in ${plural(files.length, 'file')}, ${plural(places, 'place')}`
}

/** Why a new name is not worth asking the server about, or ''. */
export function nameProblem(old: string, next: string): string {
  const n = next.trim()
  if (!n) return 'Type a new name.'
  if (n === old) return 'The new name is the same as the old one.'
  return ''
}

export function replaceSummary(files: number, places: number): string {
  return `Replace in ${plural(files, 'file')}, ${plural(places, 'place')}`
}
