// Turns the edits a language server answers to "format document" into editor changes. Every
// range is checked against the text it was asked about (line and character), because the
// editor library lets a character past the end of a line slide into the next line.
import type { Text } from '@codemirror/state'

export interface FormatChange {
  from: number
  to: number
  insert: string
}

interface Pos {
  line: number
  character: number
}

function offset(doc: Text, p: unknown): number | null {
  const { line, character } = (p ?? {}) as Partial<Pos>
  if (!Number.isInteger(line) || !Number.isInteger(character)) return null
  if (line! < 0 || line! >= doc.lines || character! < 0) return null
  const l = doc.line(line! + 1)
  return character! <= l.length ? l.from + character! : null
}

/** The changes for `edits` on `doc`, or null when any edit is malformed, out of range or overlaps another. */
export function formatChanges(doc: Text, edits: unknown): FormatChange[] | null {
  if (!Array.isArray(edits)) return null
  const out: FormatChange[] = []
  for (const e of edits as { range?: { start?: unknown; end?: unknown }; newText?: unknown }[]) {
    if (typeof e?.newText !== 'string') return null
    const from = offset(doc, e.range?.start)
    const to = offset(doc, e.range?.end)
    if (from === null || to === null || to < from) return null
    out.push({ from, to, insert: e.newText })
  }
  out.sort((a, b) => a.from - b.from)
  for (let i = 1; i < out.length; i++) if (out[i].from < out[i - 1].to) return null
  return out
}
