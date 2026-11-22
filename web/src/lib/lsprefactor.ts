// Find references and rename through the language server, across the whole project rather
// than only the files open in an editor. Part of the lazy lsp chunk.
import { LSPPlugin } from '@codemirror/lsp-client'
import type { EditorView } from '@codemirror/view'
import { currentText, projectTarget } from './edittarget'
import { planEdit, type FileEdits, type Plan } from './lspedits'
import { pathFromUri } from './lspmatch'
import { liveView } from './lspviews'
import { MAX_READ } from './refsview'
import { applyFiles, type EditResult } from './multiedit'

export interface Ref {
  path: string
  /** 1-based, as the editor counts. */
  line: number
  col: number
  /** The line's text, or '' when the file was not read. */
  text: string
}

interface Location {
  uri: string
  range: { start: { line: number; character: number } }
}


/** Waits for the start-up exchange, so the server's abilities are known; false if it takes too long. */
async function started(plugin: LSPPlugin, ms = 8000): Promise<boolean> {
  if (plugin.client.serverCapabilities) return true
  let timer: ReturnType<typeof setTimeout> | undefined
  await Promise.race([plugin.client.initializing.catch(() => {}), new Promise((r) => (timer = setTimeout(r, ms)))])
  clearTimeout(timer)
  return !!plugin.client.serverCapabilities
}

const failed = (e: unknown, what: string) => (e instanceof Error && e.message ? `${what}: ${e.message}` : what)

export async function findReferences(view: EditorView, projectId: string, projectPath: string): Promise<{ refs: Ref[] } | { error: string }> {
  const plugin = LSPPlugin.get(view)
  if (!plugin) return { error: 'No language server is running for this file.' }
  if (!(await started(plugin))) return { error: 'The language server is still starting. Try again in a moment.' }
  if (!plugin.client.serverCapabilities?.referencesProvider) return { error: 'This language server cannot find references.' }
  plugin.client.sync()
  const position = plugin.toPosition(view.state.selection.main.head)
  let locs: Location[] | null
  try {
    locs = await plugin.client.request<unknown, Location[] | null>('textDocument/references', {
      textDocument: { uri: plugin.uri },
      position,
      context: { includeDeclaration: true },
    })
  } catch (e) {
    return { error: failed(e, 'The language server could not find references') }
  }
  const refs: Ref[] = []
  for (const l of locs ?? []) {
    const path = pathFromUri(projectPath, l.uri)
    if (path !== null) refs.push({ path, line: l.range.start.line + 1, col: l.range.start.character + 1, text: '' })
  }
  refs.sort((a, b) => a.path.localeCompare(b.path) || a.line - b.line || a.col - b.col)
  await fillLines(projectId, refs)
  return { refs }
}

async function fillLines(projectId: string, refs: Ref[]) {
  const paths = [...new Set(refs.map((r) => r.path))].slice(0, MAX_READ)
  await Promise.all(
    paths.map(async (path) => {
      const text = await currentText(projectId, path).catch(() => null)
      if (text === null) return
      const lines = text.split('\n')
      for (const r of refs) if (r.path === path) r.text = (lines[r.line - 1] ?? '').replace(/\r$/, '').trim().slice(0, 200)
    }),
  )
}

/** The word a rename starts from, or why there is nothing to rename. */
export function renameTarget(view: EditorView): { name: string } | { error: string } {
  const plugin = LSPPlugin.get(view)
  if (!plugin) return { error: 'No language server is running for this file.' }
  const caps = plugin.client.serverCapabilities
  if (caps && !caps.renameProvider) return { error: 'This language server cannot rename.' }
  const word = view.state.wordAt(view.state.selection.main.head)
  if (!word) return { error: 'Put the cursor on a name to rename it.' }
  return { name: view.state.sliceDoc(word.from, word.to) }
}

/** Asks the server what a rename would change; nothing changes yet. */
export async function planRename(view: EditorView, projectPath: string, newName: string): Promise<Plan> {
  const plugin = LSPPlugin.get(view)
  if (!plugin) return { ok: false, reason: 'No language server is running for this file.' }
  if (!(await started(plugin))) return { ok: false, reason: 'The language server is still starting. Try again in a moment.' }
  if (!plugin.client.serverCapabilities?.renameProvider) return { ok: false, reason: 'This language server cannot rename.' }
  plugin.client.sync()
  try {
    const edit = await plugin.client.request<unknown, unknown>('textDocument/rename', {
      textDocument: { uri: plugin.uri },
      position: plugin.toPosition(view.state.selection.main.head),
      newName,
    })
    if (!edit) return { ok: false, reason: 'The language server found nothing to rename here.' }
    return planEdit(edit, (uri) => pathFromUri(projectPath, uri))
  } catch (e) {
    return { ok: false, reason: failed(e, 'The language server could not rename this') }
  }
}

/** Makes the planned rename: open editors change unsaved, other files are saved. */
export const applyRename = (projectId: string, plan: FileEdits[]): Promise<EditResult | { refused: string }> =>
  applyFiles(plan, projectTarget(projectId, 'rename'))

/** Which of these files are open in an editor, so a rename changes them there without saving. */
export const openPaths = (projectId: string, paths: string[]): string[] => paths.filter((p) => liveView(projectId, p))
