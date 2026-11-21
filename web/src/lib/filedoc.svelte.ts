// One open file in an editor tab: loading in chunks, saving, and what to do when the disk changes.
// Only the lazy editor view imports this, so CodeMirror stays out of the app shell.
import { EditorState, Transaction, type Text } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { lineRange } from './tokenfmt'
import { createEditor, editable, fromDisk, lang, lineEnding, readOnly, separator, type EditorHooks } from './cmeditor'
import { loadLanguage } from './cmlang'
import { files, type DiskChange, type EditorSession } from './filestore.svelte'
import { canRetry, FileError, localSource, plainError, type FileSource } from './filesapi'
import { languageFor } from './langs'
import { remoteSource } from './sshapi'

const NO_HIGHLIGHT_BYTES = 2 * 1024 * 1024
export type Status = 'loading' | 'ready' | 'binary' | 'toolarge' | 'error'
/** 'changed': the disk moved on while edits are unsaved. 'save': a save was refused. 'deleted': the file is gone. */
export type Conflict = '' | 'changed' | 'save' | 'deleted'

const quiet = [fromDisk.of(true), Transaction.addToHistory.of(false)]

export class FileDoc {
  status = $state<Status>('loading')
  error = $state('')
  /** False when reading again would fail the same way (a missing file), so no "Try again" shows. */
  retry = $state(true)
  notice = $state('')
  /** True for a few seconds after a clean editor took a newer version from disk. */
  updated = $state(false)
  conflict = $state<Conflict>('')
  writable = $state(true)
  dirty = $state(false)
  saving = $state(false)
  /** Bytes read so far and in all, while the rest of a large file arrives. */
  progress = $state<{ loaded: number; total: number } | null>(null)
  colours = $state(true)

  view: EditorView | null = null
  private version = ''
  private saved: Text | null = null
  size = 0
  /** Set by the language server binding; runs first in `detach` so the saved state holds no client. */
  unbind: (() => void) | null = null
  private gone = false
  private cancel = { aborted: false }
  private queue: (DiskChange | null)[] = []
  private off: (() => void) | null = null
  private hooks: EditorHooks | null = null
  /** Some text is selected, so lines can be sent to a chat. */
  selected = $state(false)
  /** Versions this tab saved lately, so a late echo of an earlier save is not a change on disk. */
  private mine: string[] = []

  private src: FileSource

  /** `remote` is a connection id: the file is on that host, not in the project. */
  constructor(
    project: string,
    private tabId: string,
    readonly path: string,
    private remote = '',
  ) {
    this.src = remote ? remoteSource(remote) : localSource(project)
  }

  /** The selected lines and their text, for a chat attachment; null when nothing is selected. */
  selection(): { kind: 'selection'; path: string; from: number; to: number; text: string } | null {
    const st = this.view?.state
    const r = st?.selection.main
    if (!st || !r || r.empty) return null
    const { from, to } = lineRange(st.doc, r.from, r.to)
    return { kind: 'selection', path: this.path, from, to, text: st.sliceDoc(r.from, r.to) }
  }

  /** Builds the editor in `el`, then loads the file, or restores the tab's earlier state. */
  attach(el: HTMLElement) {
    const s = files.sessions.get(this.tabId)
    // A restored editor state keeps the hooks it was built with, so they are reused and
    // pointed at this doc; fresh closures would leave edits reporting to the old one.
    const hooks = (s?.hooks as EditorHooks | undefined) ?? { onChange: () => {}, onSave: () => {} }
    hooks.onChange = (doc, user) => user && this.setDirty(!!this.saved && !doc.eq(this.saved))
    hooks.onSave = () => void this.save()
    hooks.onSelect = (st) => (this.selected = !st.selection.main.empty)
    this.hooks = hooks
    const v = createEditor(el, '', hooks)
    this.view = v
    // Disk events carry a path of the local project only, so a remote file must not listen to them.
    if (!this.remote) this.off = files.subscribe((c) => this.onDisk(c))
    if (!s) return void this.load()
    v.setState(s.state as EditorState)
    this.version = s.version
    this.saved = s.saved
    this.writable = s.writable
    this.size = s.size
    this.colours = s.size <= NO_HIGHLIGHT_BYTES
    this.setDirty(!s.saved.eq(v.state.doc))
    this.status = 'ready'
    void this.recheck()
  }

  detach() {
    this.gone = true
    this.unbind?.()
    this.unbind = null
    this.cancel.aborted = true
    this.off?.()
    if (this.view && this.status === 'ready' && !this.progress && this.saved) {
      const session: EditorSession = { state: this.view.state, hooks: this.hooks, version: this.version, saved: this.saved, writable: this.writable, size: this.size }
      files.sessions.set(this.tabId, session)
    }
    this.view?.destroy()
    this.view = null
  }

  private setDirty(on: boolean) {
    this.dirty = on
    files.setDirty(this.tabId, on)
  }

  // Loading

  /** Reads the whole file into the editor, replacing what is there. The first chunk shows at once. */
  async load(retry = true) {
    const v = this.view
    if (!v) return
    this.cancel.aborted = true
    const signal = (this.cancel = { aborted: false })
    const stale = () => this.gone || signal.aborted
    let eol = '\n'
    try {
      await this.src.readFile(this.path, {
        signal,
        onFirst: (c) => {
          if (stale()) return
          this.version = c.version
          this.writable = c.writable
          this.size = c.size
          if (c.binary || c.tooLarge) {
            this.status = c.binary ? 'binary' : 'toolarge'
            return
          }
          eol = c.content.includes('\r\n') ? '\r\n' : c.content.includes('\r') && !c.content.includes('\n') ? '\r' : '\n'
          this.colours = c.size <= NO_HIGHLIGHT_BYTES
          v.dispatch({ effects: [separator.reconfigure(lineEnding(eol)), editable.reconfigure(readOnly(true))] })
          v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: c.content }, annotations: quiet })
          this.progress = c.done ? null : { loaded: c.next, total: c.size }
          this.status = 'ready'
          void this.applyLanguage(v)
        },
        onMore: (text, loaded, total) => {
          if (stale()) return
          const doc = v.state.doc
          const cut = eol === '\r\n' && text.startsWith('\n') && doc.sliceString(doc.length - 1) === '\r'
          v.dispatch({ changes: { from: cut ? doc.length - 1 : doc.length, insert: cut ? '\r' + text : text }, annotations: quiet })
          this.progress = loaded >= total ? null : { loaded, total }
        },
      })
    } catch (e) {
      if (stale()) return
      // The file changed between two chunks: start again from the top once.
      if (e instanceof FileError && e.code === 'FILE_CHANGED' && retry) return void this.load(false)
      if (e instanceof FileError && e.code === 'NOT_FOUND' && this.status === 'ready') return void this.markGone()
      this.progress = null
      this.status = 'error'
      this.error = e instanceof FileError ? e.message : 'The file could not be read. Check that the server is running.'
      this.retry = canRetry(e)
      return
    }
    if (stale() || this.status !== 'ready') return
    this.progress = null
    this.saved = v.state.doc
    v.dispatch({ effects: editable.reconfigure(readOnly(!this.writable)), selection: { anchor: Math.min(v.state.selection.main.anchor, v.state.doc.length) } })
    this.setDirty(false)
    this.conflict = ''
    this.notice = ''
  }

  private async applyLanguage(v: EditorView) {
    if (!this.colours) return
    const ext = await loadLanguage(languageFor(this.path))
    if (!this.gone && this.view === v) v.dispatch({ effects: lang.reconfigure(ext) })
  }

  // Saving

  async save() {
    const v = this.view
    if (!v || this.status !== 'ready' || this.progress || this.saving || !this.writable) return
    this.saving = true
    const doc = v.state.doc
    try {
      const r = await this.src.writeFile(this.path, this.version, doc.sliceString(0, doc.length, v.state.lineBreak))
      this.version = r.version
      this.mine = [r.version, ...this.mine.slice(0, 3)]
      this.size = r.size
      this.saved = doc
      this.conflict = ''
      this.notice = ''
      this.setDirty(this.view ? !this.view.state.doc.eq(doc) : false)
    } catch (e) {
      if (e instanceof FileError && e.code === 'FILE_CHANGED') this.conflict = 'save'
      else this.notice = 'Could not save. ' + plainError(e)
    } finally {
      this.saving = false
      const later = this.queue.splice(0)
      later.forEach((c) => this.onDisk(c))
    }
  }

  /** "Keep mine": reads the version now on disk and saves over it. */
  async keepMine() {
    try {
      const c = await this.src.readChunk(this.path, { limit: 4096 })
      this.version = c.version
    } catch (e) {
      if (e instanceof FileError && e.code === 'NOT_FOUND') this.version = ''
      else return void (this.notice = 'Could not check the file on disk. ' + plainError(e))
    }
    this.conflict = ''
    await this.save()
  }

  /** "Reload theirs": drops unsaved edits and reads the file again. */
  async reloadTheirs() {
    this.conflict = ''
    this.notice = ''
    await this.load()
  }

  // Changes on disk

  /** A `file.changed` event for any path, or null after missed events (then the version is read again). */
  onDisk(c: DiskChange | null) {
    if (this.gone || this.status !== 'ready') return
    if (this.saving) return void this.queue.push(c)
    if (c === null) return void this.recheck()
    if (c.path !== this.path) return
    if (c.deleted) return this.markGone()
    if (c.version === this.version || this.mine.includes(c.version)) return
    if (c.binary || c.tooLarge) return void (this.dirty ? (this.conflict = 'changed') : this.load())
    this.changed()
  }

  private changed() {
    if (this.dirty) this.conflict = 'changed'
    else {
      this.updated = true
      setTimeout(() => (this.updated = false), 5000)
      void this.load()
    }
  }

  private markGone() {
    this.version = ''
    this.conflict = 'deleted'
  }

  /** Reads the version on disk, after a missed event or when the tab comes back. */
  private async recheck() {
    try {
      const c = await this.src.readChunk(this.path, { limit: 4096 })
      if (this.gone || this.saving || c.version === this.version) return
      if (c.binary || c.tooLarge) return void (this.conflict = 'changed')
      this.changed()
    } catch (e) {
      if (e instanceof FileError && e.code === 'NOT_FOUND' && !this.gone) this.markGone()
    }
  }

  // Jumping

  /** Puts the cursor at the start of a 1-based line and scrolls it to the middle. */
  goto(line: number) {
    const v = this.view
    if (!v) return
    const at = v.state.doc.line(Math.min(Math.max(line, 1), v.state.doc.lines)).from
    v.dispatch({ selection: { anchor: at }, effects: EditorView.scrollIntoView(at, { y: 'center' }) })
    v.focus()
  }
}
