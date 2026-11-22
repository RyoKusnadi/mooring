// App state in runes: projects, the current project, a layout per project, connection status; fed by the events connection.
import { PHONE_QUERY } from './breakpoint'
import { CommandError, EventsClient, newCommandId, type SocketLike, type Status } from './events'
import { terms, type TermInfo } from './termstore.svelte'
import { chats, type Agent, type ChatInfo } from './chatstore.svelte'
import { files } from './filestore.svelte'
import { git } from './gitstore.svelte'
import { lsp } from './lspstore.svelte'
import { ssh, type SshSnapshot } from './sshstore.svelte'
import { accounts } from './accountstore.svelte'
import { tools } from './toolstore.svelte'
import { settings } from './settingsstore.svelte'
import { emptyLayout, parseLayout, type Layout } from './layout'
import { listen, onWake } from './wake'
export interface Project {
  id: string
  name: string
  path: string
  position: number
  createdAt: string | number
}
export interface FolderList { path: string; parent: string; entries: { name: string; dir: boolean; git: boolean }[] }
interface Snapshot extends SshSnapshot {
  home?: string
  projects?: Project[]
  layouts?: Record<string, unknown>
  terminals?: Record<string, TermInfo[]>
  chats?: Record<string, ChatInfo[]>
  agents?: Agent[]
}
const SAVE_DELAY_MS = 400
const CURRENT_KEY = 'mooring.project'
const MAX_CLONE_LINES = 300

function remembered(): string {
  try {
    return localStorage.getItem(CURRENT_KEY) ?? ''
  } catch {
    return ''
  }
}

const sortByPosition = (list: Project[]): Project[] => list.slice().sort((a, b) => a.position - b.position)

class AppState {
  status = $state<Status>('connecting')
  ready = $state(false)
  home = $state('')
  projects = $state<Project[]>([])
  layouts = $state<Record<string, Layout>>({})
  currentId = $state(remembered())
  narrow = $state(false)
  notice = $state('')
  /** Plain information for the status bar, such as a language server restart. Errors go to `notice`. */
  info = $state('')
  /** Progress lines of the clone that is running, keyed by its command id. */
  clone = $state<{ id: string; lines: string[] } | null>(null)

  private client: EventsClient | null = null
  private timers = new Map<string, ReturnType<typeof setTimeout>>()
  private inflight = new Map<string, number>()
  private unreadable = new Set<string>()
  private cleanup: (() => void)[] = []

  get current(): Project | null {
    return this.projects.find((p) => p.id === this.currentId) ?? null
  }

  get layout(): Layout | null {
    return this.layouts[this.currentId] ?? null
  }

  start() {
    if (this.client) return
    files.attach((n, a) => this.run(n, a))
    git.attach((n, a) => this.run(n, a))
    lsp.attach((n, a) => this.run(n, a), (text) => (this.info = text))
    ssh.attach((n, a) => this.run(n, a))
    const url = `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/events`
    this.client = new EventsClient({
      createSocket: () => new WebSocket(url) as unknown as SocketLike,
      onReset: (s) => this.reset(s as Snapshot),
      onEvent: (kind, data) => this.apply(kind, data),
      onStatus: (s) => (this.status = s),
    })
    this.client.start()
    const onVisibility = () => {
      this.client?.setHidden(document.hidden)
      if (document.hidden) this.flushAll()
    }
    const media = matchMedia(PHONE_QUERY)
    const onMedia = () => (this.narrow = media.matches)
    onMedia()
    this.cleanup = [
      listen(media, 'change', onMedia),
      listen(document, 'visibilitychange', onVisibility),
      listen(window, 'pagehide', this.flushAll),
      onWake(() => this.client?.wake()),
    ]
    onVisibility()
  }

  retryNow = () => this.client?.retryNow() // skips the wait before the next try
  stop() {
    this.flushAll()
    this.client?.stop()
    this.client = null
    this.cleanup.forEach((f) => f())
    this.cleanup = []
  }

  run<T>(name: string, args: unknown, id?: string): Promise<T> {
    if (!this.client) return Promise.reject(new CommandError('CLOSED', 'The connection is closed.'))
    return this.client.send(name, args, { id }) as Promise<T>
  }

  private reset(s: Snapshot) {
    this.home = s.home ?? ''
    this.projects = sortByPosition(s.projects ?? [])
    const next: Record<string, Layout> = {}
    for (const [id, raw] of Object.entries(s.layouts ?? {})) {
      const l = parseLayout(raw)
      if (l) next[id] = l
      else this.unreadable.add(id)
    }
    // A layout still waiting to be saved, or on its way, is newer than the snapshot.
    for (const id of Object.keys(this.layouts)) if (this.saving(id)) next[id] = this.layouts[id]
    this.layouts = next
    terms.reset(s.terminals)
    chats.reset(s.chats, s.agents)
    files.resync()
    lsp.resync()
    ssh.reset(s)
    for (const s of [tools, accounts, settings]) s.reset((n, a) => this.run(n, a))
    git.schedule(0)
    this.ready = true
    this.settleCurrent()
  }

  private apply(kind: string, data: unknown) {
    const d = (data ?? {}) as Record<string, unknown>
    if (kind.startsWith('terminal.')) return terms.apply(kind, data)
    if (kind.startsWith('chat.')) return chats.apply(kind, data)
    if (kind.startsWith('git.')) return git.apply(kind, data)
    if (kind.startsWith('lsp.')) return lsp.apply(kind, data)
    if (kind.startsWith('ssh.')) return ssh.apply(kind, data)
    if (kind.startsWith('toolServer.')) return tools.apply(kind, data)
    if (kind === 'settings.changed') return settings.apply(kind, data)
    if (kind.startsWith('account.') || kind === 'project.accountChanged') return accounts.apply(kind, data)
    if (kind.startsWith('file.')) {
      git.noteFile(kind, data)
      return files.apply(kind, data)
    }
    switch (kind) {
      case 'project.added':
      case 'project.changed':
        return this.upsert(data as Project)
      case 'project.reordered':
        return this.reorderLocal(d.ids as string[])
      case 'project.removed':
        return this.removeLocal(String(d.id))
      case 'layout.changed':
        return this.layoutChanged(String(d.projectId), d.layout)
      case 'clone.progress':
        if (this.clone && this.clone.id === d.cmd) {
          this.clone.lines = [...this.clone.lines, String(d.line)].slice(-MAX_CLONE_LINES)
        }
    }
  }

  upsert(p: Project) {
    this.projects = sortByPosition([...this.projects.filter((x) => x.id !== p.id), p])
    this.settleCurrent()
  }

  private reorderLocal(ids: string[]) {
    const rank = new Map(ids.map((id, i) => [id, i]))
    this.projects = this.projects.slice().sort((a, b) => (rank.get(a.id) ?? 1e9) - (rank.get(b.id) ?? 1e9))
  }

  private removeLocal(id: string) {
    chats.dropProject(id)
    this.projects = this.projects.filter((p) => p.id !== id)
    this.layouts = Object.fromEntries(Object.entries(this.layouts).filter(([k]) => k !== id))
    this.settleCurrent()
  }

  private layoutChanged(id: string, raw: unknown) {
    if (this.saving(id)) return
    const l = parseLayout(raw)
    if (l && JSON.stringify(l) !== JSON.stringify(this.layouts[id])) this.layouts[id] = l
  }

  select(id: string) {
    if (!this.projects.some((p) => p.id === id)) return
    this.currentId = id
    this.settleCurrent()
  }

  private settleCurrent() {
    if (!this.projects.some((p) => p.id === this.currentId)) this.currentId = this.projects[0]?.id ?? ''
    try {
      if (this.currentId) localStorage.setItem(CURRENT_KEY, this.currentId)
    } catch {
      // Private mode: the choice just is not remembered.
    }
    if (this.currentId && !this.layouts[this.currentId]) {
      // The default layout is not saved until the user edits it: saving it could
      // overwrite a layout another window is saving for a project it just added.
      this.layouts[this.currentId] = emptyLayout()
      if (this.unreadable.has(this.currentId)) this.notice = 'The saved pane layout could not be read, so the pane starts empty.'
    }
  }

  edit(fn: (l: Layout) => Layout) {
    const id = this.currentId
    const l = this.layouts[id]
    if (!l) return
    const next = fn(l)
    if (next === l) return
    this.layouts[id] = next
    this.scheduleSave(id)
  }

  private saving(id: string) {
    return this.timers.has(id) || (this.inflight.get(id) ?? 0) > 0
  }

  private scheduleSave(id: string) {
    clearTimeout(this.timers.get(id))
    this.timers.set(id, setTimeout(() => this.flush(id), SAVE_DELAY_MS))
  }

  private flush(id: string, force = false) {
    // One save per project at a time, so saves reach the server in edit order.
    if (!force && (this.inflight.get(id) ?? 0) > 0) return this.scheduleSave(id)
    clearTimeout(this.timers.get(id))
    this.timers.delete(id)
    const layout = this.layouts[id]
    if (!layout || !this.client) return
    this.inflight.set(id, (this.inflight.get(id) ?? 0) + 1)
    this.run('layout.save', { projectId: id, layout: $state.snapshot(layout) })
      .then(() => (this.notice = ''))
      .catch((e: unknown) => {
        if (e instanceof CommandError && e.code !== 'CLOSED') this.notice = 'Could not save the pane layout. ' + e.message
      })
      .finally(() => this.inflight.set(id, (this.inflight.get(id) ?? 1) - 1))
  }

  private flushAll = () => {
    for (const id of [...this.timers.keys()]) this.flush(id, true)
  }

  async cloneProject(args: { url: string; parent: string; name?: string }): Promise<Project> {
    const id = newCommandId()
    this.clone = { id, lines: [] }
    try {
      const p = await this.run<Project>('project.clone', args, id)
      this.upsert(p)
      return p
    } finally {
      this.clone = null
    }
  }

  async addProject(name: 'project.add' | 'project.create', args: Record<string, string>): Promise<Project> {
    const p = await this.run<Project>(name, args)
    this.upsert(p)
    return p
  }

  async rename(id: string, name: string) {
    this.upsert(await this.run<Project>('project.rename', { id, name }))
  }

  async remove(id: string) {
    await this.run('project.remove', { id })
    this.removeLocal(id)
  }

  /** Moves a project to `to` in the list; reverts if the server refuses. */
  async move(id: string, to: number) {
    const before = this.projects
    const from = before.findIndex((p) => p.id === id)
    if (from < 0 || to < 0 || to >= before.length || to === from) return
    const ids = before.map((p) => p.id)
    ids.splice(to, 0, ...ids.splice(from, 1))
    this.reorderLocal(ids)
    try {
      await this.run('project.reorder', { ids })
    } catch (e) {
      this.projects = before
      throw e
    }
  }
}

export const app = new AppState()
