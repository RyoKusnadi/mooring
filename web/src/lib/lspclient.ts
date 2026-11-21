// The language server connections of this window. One socket per server and place (a project's own
// folder or one worktree), shared by every tab that edits a file of it, and kept for 30 seconds
// after the last tab goes. Each socket gets a fresh client, so nothing queued for a dead socket
// leaks into the next one. Not done this phase: find references, rename and quick fixes.
import { LSPClient, type LSPClientExtension, hoverTooltips, jumpToDefinitionKeymap, serverCompletion, serverDiagnostics, signatureHelp, signatureKeymap } from '@codemirror/lsp-client'
import { lintGutter } from '@codemirror/lint'
import type { Extension } from '@codemirror/state'
import { EditorView, keymap } from '@codemirror/view'
import { lsp, lspKey } from './lspstore.svelte'
import { fileUri, isFormatKey, pathFromUri, type LspServer } from './lspmatch'
import { sanitizeHTML } from './lspsanitize'
import { ignoreLateCompletions } from './lstale'
import { lspTheme } from './lsptheme'
import { LspWire, quietWhenGone, type WireSocket } from './lspwire'
import { MooringWorkspace, type WorkspaceHost } from './lspworkspace'

/** What formats an editor's file; bindLsp sets it while the server can format and the file can be saved. */
export const formatHooks = new WeakMap<EditorView, () => boolean>()

export const IDLE_MS = 30_000

export interface Place {
  projectId: string
  projectPath: string
  folder: string
  server: LspServer
  show: (path: string) => Promise<EditorView | null>
  say: (text: string) => void
}

export class Conn {
  client: LSPClient | null = null
  /** Counts each fresh client, so an editor knows when to take the new extension. */
  generation = 0
  readonly key: string
  private refs = 0
  private idle: ReturnType<typeof setTimeout> | null = null
  private wire: LspWire
  private listeners = new Set<() => void>()
  private capListeners = new Set<() => void>()

  constructor(
    private place: Place,
    private onGone: (c: Conn) => void,
    open: () => WireSocket,
  ) {
    ignoreLateCompletions()
    this.key = lspKey(place.projectId, place.folder, place.server.id)
    this.wire = new LspWire({
      open,
      onUp: (t) => {
        const host: WorkspaceHost = { pathOf: (uri) => pathFromUri(place.projectPath, uri), show: place.show, say: place.say }
        const client = new LSPClient({
          rootUri: fileUri(place.projectPath, place.folder),
          sanitizeHTML,
          // The library gives a request 3 seconds. A server that is still loading a big project
          // takes longer to answer `initialize`, and then never reported what it can do.
          timeout: 30_000,
          workspace: (c) => new MooringWorkspace(c, host),
          extensions: extensions(),
        })
        quietWhenGone(client, () => this.client === client)
        client.connect(t)
        this.client = client
        this.changed()
        // What the server can do is known only after it answers `initialize`.
        void client.initializing.then(
          () => this.client === client && this.capListeners.forEach((l) => l()),
          () => {},
        )
      },
      onDown: () => this.drop(),
      onGiveUp: () => {},
      onState: (s) => {
        lsp.conn[this.key] = s
        // A connection that works again shows its next failure.
        if (s === 'open') delete lsp.shut[this.key]
      },
    })
    this.wire.start()
  }

  private drop() {
    this.client?.disconnect()
    this.client = null
    this.changed()
  }

  private changed() {
    this.generation++
    this.listeners.forEach((l) => l())
  }

  onChange(fn: () => void): () => void {
    this.listeners.add(fn)
    return () => this.listeners.delete(fn)
  }

  /** True once the server said it formats whole documents. */
  get canFormat(): boolean {
    return !!this.client?.serverCapabilities?.documentFormattingProvider
  }

  /** True once the server said it lists the symbols of a document. */
  get canSymbols(): boolean {
    return !!this.client?.serverCapabilities?.documentSymbolProvider
  }

  /** True until the server has answered `initialize`: what it can do is not known yet. */
  get starting(): boolean {
    return !this.client?.serverCapabilities
  }

  /** Runs `fn` when the server's abilities become known. */
  onCaps(fn: () => void): () => void {
    this.capListeners.add(fn)
    return () => this.capListeners.delete(fn)
  }

  /** The editor extension of an open file; ties the editor to the current client. */
  extensionFor(uri: string, languageId: string): Extension {
    return this.client ? this.client.plugin(uri, languageId) : []
  }

  hold() {
    this.refs++
    if (this.idle) clearTimeout(this.idle)
    this.idle = null
  }

  release() {
    if (--this.refs > 0) return
    this.idle = setTimeout(() => this.close(), IDLE_MS)
  }

  /** The server was stopped from the dialog: do not open it again until the user asks. */
  halt() {
    this.wire.stop()
    this.drop()
  }

  restart() {
    this.wire.restart()
  }

  close() {
    if (this.idle) clearTimeout(this.idle)
    this.idle = null
    this.wire.stop()
    this.drop()
    delete lsp.conn[this.key]
    this.onGone(this)
  }
}

function extensions(): (Extension | LSPClientExtension)[] {
  return [
    serverCompletion(),
    hoverTooltips(),
    signatureHelp(),
    serverDiagnostics(),
    lintGutter(),
    lspTheme,
    keymap.of([...jumpToDefinitionKeymap, ...signatureKeymap]),
    // Matched by key code: on macOS Shift-Option-F types "Ï", which a key-name binding never sees.
    EditorView.domEventHandlers({
      keydown: (e, view) => {
        const run = isFormatKey(e) ? formatHooks.get(view) : undefined
        if (!run) return false
        e.preventDefault()
        run()
        return true
      },
    }),
  ]
}

const conns = new Map<string, Conn>()

function socketFor(place: Place): WireSocket {
  const q = new URLSearchParams({ project: place.projectId, folder: place.folder })
  const url = `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/lsp/${encodeURIComponent(place.server.id)}?${q}`
  return new WebSocket(url) as unknown as WireSocket
}

/** The connection of a place, opened if no tab has it yet. Give it back with `release`. */
export function acquire(place: Place, open: () => WireSocket = () => socketFor(place)): Conn {
  const key = lspKey(place.projectId, place.folder, place.server.id)
  let c = conns.get(key)
  if (!c) {
    c = new Conn(place, (x) => conns.get(x.key) === x && conns.delete(x.key), open)
    conns.set(key, c)
  }
  c.hold()
  return c
}

export function connFor(key: string): Conn | undefined {
  return conns.get(key)
}

/** "Start again" in the bar. */
export function restart(key: string) {
  conns.get(key)?.restart()
}

// A stopped server (the Stop button) must stay stopped; the relay would start it on the next socket.
lsp.onStopped((e) => {
  if (e.reason === 'closed') conns.get(lspKey(e.projectId, e.folder, e.id))?.halt()
})
