// Files of the current project: the folder tree, the live-change lease and the events that arrive.
// Commands go out through `run` (given by app state), so this file does not import it.
import type { Text } from '@codemirror/state'
import { canRetry, FileError, listDir, plainError, type Entry } from './filesapi'
import { isInside, joinPath, parentPath, rebase } from './paths'

export interface DirState {
  state: 'loading' | 'ready' | 'error'
  entries: Entry[]
  truncated: boolean
  total: number
  error: string
  /** False when the same request would fail the same way, so the list offers no "Try again". */
  retry?: boolean
  /** The folder asked for is not there. */
  missing?: boolean
}
export interface DiskChange {
  path: string
  version: string
  size: number
  deleted: boolean
  binary: boolean
  tooLarge: boolean
}
type Run = <T>(name: string, args: unknown) => Promise<T>
type Listener = (change: DiskChange | null) => void

const LEASE_MS = 30_000
const MAX_WATCHED = 190
const key = (project: string, path: string) => `${project}:${path}`
const windowId = crypto.randomUUID().replaceAll('-', '').slice(0, 12)

class FileStore {
  dirs = $state<Record<string, DirState>>({})
  open = $state<Record<string, boolean>>({})
  /** Tab ids with edits not saved yet. */
  dirty = $state<Record<string, boolean>>({})
  /** The tab the user asked to close while it had unsaved edits. */
  closing = $state('')
  finder = $state<'file' | 'text' | null>(null)
  /** Text the finder opens with (the palette passes a file name typed there). */
  finderQuery = $state('')
  /** Counts every file change the server reported, so a list built from disk can tell it is out of date. */
  changeTick = $state(0)
  /** What the tree is asking for: a name for a new entry or a rename, or a yes to a delete. */
  editing = $state<{ op: 'new-file' | 'new-dir' | 'rename' | 'delete' | 'delete-forever'; path: string } | null>(null)
  /** The folder "New file" and "New folder" in the section header add to. */
  target = $state('')
  /** The tree row that holds the one tab stop. */
  cursor = $state('')
  /** Counts requests to show a path in the tree, so the drawer on a phone opens. */
  revealTick = $state(0)
  /** Format commands of the editors that can format now, by tab id (set by the editor). */
  formatters = $state<Record<string, () => void>>({})
  /** Find references and rename of the editors that have a ready language server, by tab id (set by the editor). */
  refactors = $state<Record<string, { refs: () => void; rename: () => void }>>({})
  /** The blame switch of each file tab: whether it is on, and a way to flip it (set by the editor). */
  blamers = $state<Record<string, { on: boolean; toggle: () => void; openLine?: () => void }>>({})
  /** A request to show a line in an editor tab; the editor takes it once the text is loaded. */
  jump = $state<{ tabId: string; line: number } | null>(null)
  /** An editor's state while its tab is not showing, so switching tabs keeps unsaved edits and undo. */
  sessions = new Map<string, EditorSession>()

  private run: Run | null = null
  private listeners = new Set<Listener>()
  private wanted = new Map<string, { dirs: string[]; files: string[] }>()
  private timer: ReturnType<typeof setInterval> | null = null

  attach(run: Run) {
    this.run = run
  }

  // Tree

  /** True when the project's own folder is gone from the disk. */
  rootMissing(project: string): boolean {
    const d = this.dirs[key(project, '')]
    return d?.state === 'error' && !!d.missing
  }

  dir(project: string, path: string): DirState | undefined {
    return this.dirs[key(project, path)]
  }

  isOpen(project: string, path: string): boolean {
    return path === '' || !!this.open[key(project, path)]
  }

  async load(project: string, path: string) {
    const k = key(project, path)
    const had = this.dirs[k]
    if (!had) this.dirs[k] = { state: 'loading', entries: [], truncated: false, total: 0, error: '' }
    try {
      const l = await listDir(project, path)
      this.dirs[k] = { state: 'ready', entries: l.entries, truncated: l.truncated, total: l.total, error: '' }
    } catch (e) {
      // A refresh that fails keeps the list the user already sees.
      if (!had || had.state !== 'ready') this.dirs[k] = { state: 'error', entries: [], truncated: false, total: 0, error: plainError(e, 'Could not list this folder. Try again.'), retry: canRetry(e), missing: e instanceof FileError && e.code === 'NOT_FOUND' }
    }
  }

  toggle(project: string, path: string) {
    const k = key(project, path)
    this.open[k] = !this.open[k]
    if (this.open[k] && this.dirs[k]?.state !== 'ready') void this.load(project, path)
  }

  expand(project: string, path: string) {
    if (path === '' || this.open[key(project, path)]) return
    this.toggle(project, path)
  }

  /** Folders that are open and loaded, for the watch lease. The project folder is always first. */
  openDirs(project: string): string[] {
    const out = ['']
    const prefix = project + ':'
    for (const k of Object.keys(this.open)) if (this.open[k] && k.startsWith(prefix)) out.push(k.slice(prefix.length))
    return out
  }

  // Changes the user makes (each runs once on the server)

  private cmd<T>(name: string, args: unknown): Promise<T> {
    return this.run ? this.run<T>(name, args) : Promise.reject(new Error('The connection is not ready.'))
  }

  async create(project: string, dir: string, name: string, kind: 'file' | 'dir'): Promise<string> {
    const path = joinPath(dir, name)
    await this.cmd('file.create', { projectId: project, path, kind })
    this.expand(project, dir)
    await this.load(project, dir)
    return path
  }

  async rename(project: string, from: string, to: string) {
    await this.cmd('file.rename', { projectId: project, from, to })
    for (const k of Object.keys(this.open)) {
      const p = k.slice(project.length + 1)
      if (k.startsWith(project + ':') && isInside(p, from)) {
        delete this.open[k]
        this.open[key(project, rebase(p, from, to))] = true
      }
    }
    this.forgetDirs(project, from)
    await Promise.all([this.load(project, parentPath(from)), this.load(project, parentPath(to))])
  }

  async copy(project: string, from: string, to: string): Promise<{ path: string; skipped: string[]; skippedCount: number }> {
    const r = await this.cmd<{ path: string; skipped: string[]; skippedCount: number }>('file.copy', { projectId: project, from, to })
    await this.load(project, parentPath(to))
    return r
  }

  /** Returns true when the item went to the trash. Throws TRASH_UNAVAILABLE for the caller to ask. */
  async remove(project: string, path: string, permanent = false): Promise<boolean> {
    const r = await this.cmd<{ trashed: boolean }>('file.delete', { projectId: project, path, permanent })
    this.forgetDirs(project, path)
    await this.load(project, parentPath(path))
    return r.trashed
  }

  private forgetDirs(project: string, path: string) {
    for (const k of Object.keys(this.dirs)) {
      if (k.startsWith(project + ':') && isInside(k.slice(project.length + 1), path)) delete this.dirs[k]
    }
  }

  // Events from the server

  subscribe(fn: Listener): () => void {
    this.listeners.add(fn)
    return () => this.listeners.delete(fn)
  }

  apply(kind: string, data: unknown) {
    const d = (data ?? {}) as Record<string, unknown>
    if (kind === 'file.dir' || kind === 'file.changed' || kind === 'file.overflow') this.changeTick++
    if (kind === 'file.dir') {
      const project = String(d.projectId)
      const path = String(d.path ?? '')
      if (this.dirs[key(project, path)]) void this.load(project, path)
    } else if (kind === 'file.changed') {
      const c: DiskChange = { path: String(d.path), version: String(d.version ?? ''), size: Number(d.size) || 0, deleted: !!d.deleted, binary: !!d.binary, tooLarge: !!d.tooLarge }
      this.listeners.forEach((l) => l(c))
    } else if (kind === 'file.overflow') {
      this.resync()
    }
  }

  /** Events were lost (overflow, or a fresh snapshot): list every loaded folder and recheck every open file. */
  resync() {
    for (const k of Object.keys(this.dirs)) {
      const i = k.indexOf(':')
      void this.load(k.slice(0, i), k.slice(i + 1))
    }
    this.listeners.forEach((l) => l(null))
    this.renew()
  }

  // Live-change lease: one holder per project in this window, repeated every 30 s.

  /** Says which folders and files the window has open; everything else stops being watched. */
  watch(project: string, dirs: string[], files: string[]) {
    for (const p of [...this.wanted.keys()]) {
      if (p !== project) {
        this.wanted.delete(p)
        this.lease(p, [], [])
      }
    }
    if (!project) return
    const next = { dirs: dirs.slice(0, MAX_WATCHED), files: files.slice(0, MAX_WATCHED) }
    const cur = this.wanted.get(project)
    if (cur && JSON.stringify(cur) === JSON.stringify(next)) return
    this.wanted.set(project, next)
    this.lease(project, next.dirs, next.files)
    this.timer ??= setInterval(() => this.renew(), LEASE_MS)
  }

  renew() {
    for (const [p, w] of this.wanted) this.lease(p, w.dirs, w.files)
  }

  private lease(project: string, dirs: string[], files: string[]) {
    const holder = `w${windowId}-${project}`.replace(/[^A-Za-z0-9_-]/g, '').slice(0, 64)
    this.cmd('file.watch', { projectId: project, holder, dirs, files }).catch(() => {})
  }

  setDirty(tabId: string, on: boolean) {
    if (on) this.dirty[tabId] = true
    else delete this.dirty[tabId]
  }

  /** Drops everything kept for a closed tab. */
  forget(tabId: string) {
    this.sessions.delete(tabId)
    this.setDirty(tabId, false)
    if (this.closing === tabId) this.closing = ''
  }

  get anyDirty(): boolean {
    return Object.keys(this.dirty).length > 0
  }
}

export interface EditorSession {
  state: unknown
  /** The editor's change and save hooks; the next FileDoc of this tab points them at itself. */
  hooks: unknown
  version: string
  saved: Text
  writable: boolean
  size: number
}

export const files = new FileStore()
