// Where a multi-file edit lands in this app: an editor bound to a language server changes in
// place; any other file is read and saved through the files API.
import { readFile, writeFile } from './filesapi'
import { files } from './filestore.svelte'
import { groups } from './layout'
import { liveView } from './lspviews'
import type { EditTarget } from './multiedit'
import { app } from './state.svelte'

export async function readWhole(projectId: string, path: string): Promise<{ text: string; version: string } | null> {
  let text = ''
  const first = await readFile(projectId, path, { onFirst: (c) => (text = c.content), onMore: (t) => (text += t) })
  return first.binary || first.tooLarge ? null : { text, version: first.version }
}

/** The text an edit is planned against: the editor's when one is live, else the file's. */
export async function currentText(projectId: string, path: string): Promise<string | null> {
  const v = liveView(projectId, path)
  if (v) return v.state.doc.toString()
  return (await readWhole(projectId, path))?.text ?? null
}

export function projectTarget(projectId: string, userEvent: string): EditTarget {
  return {
    live: (path) => {
      const v = liveView(projectId, path)
      if (!v) return null
      return {
        text: () => v.state.doc.toString(),
        change: (cs) => v.dispatch({ changes: [...cs].reverse(), userEvent }),
      }
    },
    heldUnsaved: (path) => {
      const layout = app.layouts[projectId]
      if (!layout) return false
      return groups(layout.root).some((g) => g.tabs.some((t) => t.kind === 'file' && t.path === path && files.dirty[t.id]))
    },
    read: (path) => readWhole(projectId, path),
    write: async (path, base, text) => void (await writeFile(projectId, path, base, text)),
  }
}
