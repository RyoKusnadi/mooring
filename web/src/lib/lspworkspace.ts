// What the language client needs from the app: which editors hold which files, and how to put a
// file in front of the user. The library's own workspace class is not exported, so this is the
// whole thing: one editing tab per file, files opened with the tab and closed with it.
import { LSPPlugin, Workspace, type LSPClient, type WorkspaceFile } from '@codemirror/lsp-client'
import type { EditorView } from '@codemirror/view'

/** How the workspace reaches the rest of the app. */
export interface WorkspaceHost {
  /** The project path of a file URI, or null when it is outside the project. */
  pathOf(uri: string): string | null
  /** Opens or focuses the tab of a project file and gives back its editor once the text is in. */
  show(path: string): Promise<EditorView | null>
  /** Said when a file cannot be shown. */
  say(text: string): void
}

class OpenFile implements WorkspaceFile {
  constructor(
    readonly uri: string,
    readonly languageId: string,
    public version: number,
    public doc: WorkspaceFile['doc'],
    readonly view: EditorView,
  ) {}
  getView() {
    return this.view
  }
}

/** The client's plugin in an editor, or null once it was reconfigured away (late replies skip such editors). */
function pluginOf(view: EditorView): LSPPlugin | null {
  try {
    return LSPPlugin.get(view)
  } catch {
    return null
  }
}

type FileUpdate = ReturnType<Workspace['syncFiles']>[number]

export class MooringWorkspace extends Workspace {
  files: OpenFile[] = []
  private versions = new Map<string, number>()

  constructor(
    client: LSPClient,
    private host: WorkspaceHost,
  ) {
    super(client)
  }

  private next(uri: string): number {
    const v = (this.versions.get(uri) ?? -1) + 1
    this.versions.set(uri, v)
    return v
  }

  /** Collects what each editor changed since the last sync, like the library's own workspace does. */
  syncFiles(): readonly FileUpdate[] {
    const out: FileUpdate[] = []
    for (const file of this.files) {
      const plugin = pluginOf(file.view)
      if (!plugin || plugin.unsyncedChanges.empty) continue
      out.push({ file, changes: plugin.unsyncedChanges, prevDoc: file.doc })
      file.doc = file.view.state.doc
      file.version = this.next(file.uri)
      plugin.clear()
    }
    return out
  }

  /** An editor took a file. If another editor had it, that one is closed first: the server sees one at a time. */
  openFile(uri: string, languageId: string, view: EditorView): void {
    const had = this.getFile(uri)
    if (had) {
      this.files = this.files.filter((f) => f !== had)
      this.client.didClose(uri)
    }
    const file = new OpenFile(uri, languageId, this.next(uri), view.state.doc, view)
    this.files.push(file)
    this.client.didOpen(file)
  }

  closeFile(uri: string, view: EditorView): void {
    const file = this.getFile(uri)
    // A newer editor may already hold the file; only the one that holds it closes it.
    if (!file || file.getView() !== view) return
    this.files = this.files.filter((f) => f !== file)
    this.client.didClose(uri)
  }

  /** After a reconnect the server knows nothing: open every file again with its current text. */
  connected(): void {
    for (const file of this.files) {
      file.doc = file.view.state.doc
      file.version = this.next(file.uri)
      pluginOf(file.view)?.clear()
      this.client.didOpen(file)
    }
  }

  async displayFile(uri: string): Promise<EditorView | null> {
    const path = this.host.pathOf(uri)
    if (path === null) {
      this.host.say('That file is outside this project, so Mooring cannot open it here.')
      return null
    }
    const view = await this.host.show(path)
    if (!view) this.host.say(`Could not open ${path}.`)
    return view
  }
}
