// Makes a planned edit in several files at once (a rename now, replace across files later).
// A file open in an editor changes there, unsaved, so undo works; any other file is read,
// changed and saved against the version read, so a file changed meanwhile is not overwritten.
import { applyEdits, toChanges, type FileEdits } from './lspedits'

export interface LiveDoc {
  text(): string
  change(changes: { from: number; to: number; insert: string }[]): void
}

export interface EditTarget {
  /** The editor showing the file now, if any. */
  live(path: string): LiveDoc | null
  /** An editor tab that is not showing and holds unsaved changes: its text is not what the server saw. */
  heldUnsaved(path: string): boolean
  /** Null for a binary or too large file. */
  read(path: string): Promise<{ text: string; version: string } | null>
  write(path: string, base: string, text: string): Promise<void>
}

export interface EditResult {
  changed: string[]
  failed: { path: string; reason: string }[]
}

/** Checks first, so an edit that cannot be made whole is refused before any file changes. */
export async function applyFiles(files: FileEdits[], t: EditTarget): Promise<EditResult | { refused: string }> {
  const held = files.find((f) => !t.live(f.path) && t.heldUnsaved(f.path))
  if (held) return { refused: `${held.path} has unsaved changes in a tab that is not showing. Save it or show it, then try again.` }
  const out: EditResult = { changed: [], failed: [] }
  for (const f of files) {
    try {
      const doc = t.live(f.path)
      if (doc) {
        const text = doc.text()
        if (f.planned !== undefined && text !== f.planned) throw new Error(CHANGED)
        const changes = toChanges(text, f.edits)
        if (overlaps(changes)) throw new Error('The language server sent edits that overlap.')
        doc.change(changes)
      } else {
        const got = await t.read(f.path)
        if (!got) throw new Error('The file is binary or too large to change here.')
        if (f.planned !== undefined && got.text !== f.planned) throw new Error(CHANGED)
        const next = applyEdits(got.text, f.edits)
        if (next === null) throw new Error('The language server sent edits that overlap.')
        await t.write(f.path, got.version, next)
      }
      out.changed.push(f.path)
    } catch (e) {
      out.failed.push({ path: f.path, reason: e instanceof Error ? e.message : String(e) })
    }
  }
  return out
}

const CHANGED = 'The file changed after the preview, so it was left alone. Preview again.'

/** `changes` are last first, as toChanges returns them. */
function overlaps(changes: { from: number; to: number }[]): boolean {
  for (let i = 1; i < changes.length; i++) if (changes[i].to > changes[i - 1].from) return true
  return false
}
