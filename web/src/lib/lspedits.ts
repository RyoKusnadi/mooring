// A language server's WorkspaceEdit (from a rename) as plain text edits per file. Positions are
// LSP line and UTF-16 character, which is how JavaScript strings count too.

export interface Pos {
  line: number
  character: number
}
export interface LspTextEdit {
  range: { start: Pos; end: Pos }
  newText: string
}
interface WorkspaceEdit {
  changes?: Record<string, LspTextEdit[]>
  documentChanges?: ({ textDocument?: { uri?: string }; edits?: LspTextEdit[]; kind?: string })[]
}

export interface FileEdits {
  path: string
  edits: LspTextEdit[]
  /** The text the edits were planned on; if set, a file whose text differs now is not changed. */
  planned?: string
}

export type Plan = { ok: true; files: FileEdits[] } | { ok: false; reason: string }

/** Sorts the edit by file; refuses one that creates, moves or deletes files, or touches a file outside the project. */
export function planEdit(edit: unknown, pathOf: (uri: string) => string | null): Plan {
  const e = (edit ?? {}) as WorkspaceEdit
  const byUri = new Map<string, LspTextEdit[]>()
  const add = (uri: string, list: LspTextEdit[] | undefined) => {
    if (!list?.length) return
    byUri.set(uri, [...(byUri.get(uri) ?? []), ...list])
  }
  if (Array.isArray(e.documentChanges)) {
    for (const d of e.documentChanges) {
      if (d.kind) return { ok: false, reason: 'This change also creates, moves or deletes files, which Mooring cannot do yet.' }
      add(String(d.textDocument?.uri ?? ''), d.edits)
    }
  } else {
    for (const [uri, list] of Object.entries(e.changes ?? {})) add(uri, list)
  }
  const files: FileEdits[] = []
  for (const [uri, edits] of byUri) {
    const path = pathOf(uri)
    if (path === null) return { ok: false, reason: 'This change touches files outside the project, so Mooring will not make it.' }
    files.push({ path, edits })
  }
  files.sort((a, b) => a.path.localeCompare(b.path))
  return { ok: true, files }
}

/** The offset of an LSP position in `text`; past the end of a line or the file, it stops there. */
export function offsetOf(text: string, p: Pos): number {
  let at = 0
  for (let l = 0; l < p.line; l++) {
    const nl = text.indexOf('\n', at)
    if (nl < 0) return text.length
    at = nl + 1
  }
  const nl = text.indexOf('\n', at)
  const end = nl < 0 ? text.length : nl > at && text[nl - 1] === '\r' ? nl - 1 : nl
  return Math.min(at + Math.max(0, p.character), end)
}

/** The edits as offsets, last first, so applying them in order keeps the earlier offsets right. */
export function toChanges(text: string, edits: LspTextEdit[]): { from: number; to: number; insert: string }[] {
  return edits
    .map((e) => ({ from: offsetOf(text, e.range.start), to: offsetOf(text, e.range.end), insert: e.newText }))
    .sort((a, b) => b.from - a.from || b.to - a.to)
}

/** `text` with the edits made. Returns null when two edits overlap, which a server must not send. */
export function applyEdits(text: string, edits: LspTextEdit[]): string | null {
  const changes = toChanges(text, edits)
  let out = text
  let limit = Infinity
  for (const c of changes) {
    if (c.to > limit || c.to < c.from) return null
    out = out.slice(0, c.from) + c.insert + out.slice(c.to)
    limit = c.from
  }
  return out
}

/** How many places the edit changes, for the preview. */
export const countEdits = (files: FileEdits[]) => files.reduce((n, f) => n + f.edits.length, 0)
