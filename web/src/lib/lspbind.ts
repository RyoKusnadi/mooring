// Ties one open editor to the language server of its file: takes a share of the place's
// connection, puts the client's extension into the editor, and takes it out again.
// This is the entry of the lazy lsp chunk, loaded with the first file that has a server.
import { LSPPlugin } from '@codemirror/lsp-client'
import { getIndentUnit, indentUnit as indentUnitFacet } from '@codemirror/language'
import type { EditorView } from '@codemirror/view'
import { lsp as lspSlot } from './cmeditor'
import { fileUri, languageIdFor, splitPlace, type LspServer } from './lspmatch'
import { acquire, formatHooks } from './lspclient'
import { formatChanges } from './lspformat'
import { trackSymbols, type SymbolStep } from './lspsymbols'
import { fileActions } from './fileactions'
import { app } from './state.svelte'
import { viewKey, views, waitForView, waiters } from './lspviews'

export { restart } from './lspclient'
export { applyRename, findReferences, openPaths, planRename, renameTarget } from './lsprefactor'


export interface Binding {
  /** Moves the cursor to a symbol the editor reported. */
  go(step: SymbolStep): void
  dispose(): void
}

export type { SymbolStep }

/** `view` is the editor of file `path` (a project path) once its text is fully loaded. */
export function bindLsp(args: {
  view: EditorView
  projectId: string
  path: string
  server: LspServer
  /** Told whenever the server can (a function that formats the whole file as one edit) or cannot format. */
  onFormat?: (run: (() => boolean) | null, starting?: boolean) => void
  /** False for a file that cannot be saved; formatting is then refused. */
  writable?: () => boolean
  /** The symbols around the cursor, outermost first; empty when the server lists none. */
  onSymbols?: (steps: SymbolStep[]) => void
}): Binding {
  const { view, projectId, path, server, onFormat, writable, onSymbols } = args
  const project = app.projects.find((p) => p.id === projectId)
  if (!project) return { go() {}, dispose() {} }
  const { place } = splitPlace(path)
  const uri = fileUri(project.path, path)
  const languageId = languageIdFor(server, path)
  const conn = acquire({
    projectId,
    projectPath: project.path,
    folder: place,
    server,
    show: async (p) => {
      fileActions.openFile(p)
      return waitForView(projectId, p)
    },
    say: (text) => (app.notice = text),
  })
  const vk = viewKey(projectId, path)
  let applied = -1
  let gone = false
  const symbols = trackSymbols(
    view,
    () => {
      const plugin = LSPPlugin.get(view)
      if (!plugin) return Promise.reject(new Error('no client'))
      plugin.client.sync()
      return plugin.client.request('textDocument/documentSymbol', { textDocument: { uri: plugin.uri } })
    },
    (steps) => onSymbols?.(steps),
  )

  const apply = () => {
    if (gone || applied === conn.generation) return
    applied = conn.generation
    // The old client's plugin closes the file on its server; the new one opens it again.
    try {
      view.dispatch({ effects: lspSlot.reconfigure([conn.extensionFor(uri, languageId), symbols.extension]) })
    } catch {
      // The editor was torn down or its state replaced between the change and now.
    }
    if (conn.client) {
      views.set(vk, view)
      waiters.forEach((w) => w())
    } else views.delete(vk)
  }
  const format = () => {
    if (gone) return false
    if (conn.starting) {
      app.notice = 'The language server is still starting. Try again in a moment.'
      return false
    }
    if (!conn.canFormat) return false
    if (writable && !writable()) {
      app.notice = 'This file is read-only, so it is not formatted.'
      return false
    }
    return formatView(view, (text) => (app.notice = text))
  }
  const tellFormat = () => {
    if (!gone && (conn.canFormat || conn.starting)) {
      formatHooks.set(view, format)
      onFormat?.(format, conn.starting)
    } else {
      formatHooks.delete(view)
      onFormat?.(null)
    }
  }
  const tellSymbols = () => {
    if (gone) return
    if (conn.canSymbols) symbols.refresh()
    else onSymbols?.([])
  }
  const off = conn.onChange(() => (apply(), tellFormat(), tellSymbols()))
  const offCaps = conn.onCaps(() => (tellFormat(), tellSymbols()))
  apply()
  tellFormat()
  tellSymbols()

  return {
    go: symbols.go,
    dispose() {
      if (gone) return
      gone = true
      symbols.dispose()
      onSymbols?.([])
      off()
      offCaps()
      formatHooks.delete(view)
      onFormat?.(null)
      if (views.get(vk) === view) views.delete(vk)
      // Cleared before the tab saves its editor state, so a restored state holds no old client.
      try {
        view.dispatch({ effects: lspSlot.reconfigure([]) })
      } catch {
        // The editor is already gone.
      }
      conn.release()
    },
  }
}

/** Asks the server to format the whole file and applies its answer as one edit, or none at all. */
function formatView(view: EditorView, say: (text: string) => void): boolean {
  const plugin = LSPPlugin.get(view)
  if (!plugin) return false
  const before = view.state.doc
  plugin.client.sync()
  const options = { tabSize: getIndentUnit(view.state), insertSpaces: !view.state.facet(indentUnitFacet).includes('\t') }
  plugin.client.request<unknown, unknown>('textDocument/formatting', { options, textDocument: { uri: plugin.uri } }).then(
    (edits) => {
      if (!edits) return
      if (view.state.doc !== before) return say('The file changed while it was being formatted, so nothing was changed.')
      const changes = formatChanges(before, edits)
      if (!changes) return say('The language server sent edits outside the file, so nothing was changed.')
      if (changes.length) view.dispatch({ changes, userEvent: 'format' })
    },
    () => say('The language server could not format this file.'),
  )
  return true
}
