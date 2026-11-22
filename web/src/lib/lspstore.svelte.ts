// Language servers as the server lists them: state, what runs, and the events that change them.
// The client itself is in a lazy chunk (lspclient.ts); this file is small and stays in the app shell.
import { restartNotice, serverFor, type LspRunning, type LspServer } from './lspmatch'

type Run = <T>(name: string, args: unknown) => Promise<T>
export type StoppedEvent = { projectId: string; folder: string; id: string; reason: string; stderr: string }
type StopListener = (e: StoppedEvent) => void

export const lspKey = (project: string, folder: string, id: string) => `${project}\n${folder}\n${id}`

class LspStore {
  servers = $state<LspServer[]>([])
  running = $state<LspRunning[]>([])
  loaded = $state(false)
  /** Servers whose bar the user closed with "Not now"; lasts until the page reloads. */
  dismissed = $state<Record<string, boolean>>({})
  /** Connection key -> the connection state whose bar the user hid. */
  shut = $state<Record<string, string>>({})
  /** The last line an install printed, by server id. */
  progress = $state<Record<string, string>>({})
  /** The last output of a server that crashed, by place key (see lspKey). */
  crashes = $state<Record<string, string>>({})
  /** The socket of each place the editor holds open, by lspKey; written by the lsp chunk. */
  conn = $state<Record<string, 'connecting' | 'open' | 'waiting' | 'failed' | 'stopped'>>({})

  private run: Run | null = null
  private notify: (text: string) => void = () => {}
  private loading: Promise<void> | null = null
  private listeners = new Set<StopListener>()

  attach(run: Run, notify: (text: string) => void) {
    this.run = run
    this.notify = notify
  }

  serverFor(path: string): LspServer | null {
    return serverFor(this.servers, path)
  }

  server(id: string): LspServer | undefined {
    return this.servers.find((s) => s.id === id)
  }

  /** Reads the list once; `force` reads it again (the dialog does). */
  load(force = false): Promise<void> {
    if (this.loaded && !force) return Promise.resolve()
    this.loading ??= this.cmd<{ servers: LspServer[]; running: LspRunning[] }>('lsp.list', {})
      .then((l) => {
        this.servers = l.servers ?? []
        this.running = l.running ?? []
        this.loaded = true
      })
      .finally(() => (this.loading = null))
    return this.loading
  }

  /** After a fresh snapshot, events may have been missed. */
  resync() {
    if (this.loaded) void this.load(true).catch(() => {})
  }

  private cmd<T>(name: string, args: unknown): Promise<T> {
    return this.run ? this.run<T>(name, args) : Promise.reject(new Error('The connection is not ready.'))
  }

  // Commands (each runs once on the server; the result and the event both update the list)

  async allow(id: string, command: string[]) {
    this.upsert(await this.cmd<LspServer>('lsp.allow', { id, command }))
  }

  async install(id: string) {
    this.progress[id] = ''
    this.upsert(await this.cmd<LspServer>('lsp.install', { id }))
  }

  async setCustom(args: { id: string; name: string; endings: string[]; languageId: string; command: string[] }) {
    this.upsert(await this.cmd<LspServer>('lsp.setCustom', args))
  }

  async removeCustom(id: string) {
    await this.cmd('lsp.removeCustom', { id })
    await this.load(true)
  }

  async stop(projectId: string, folder: string, id: string) {
    await this.cmd('lsp.stop', { projectId, folder, id })
    this.running = this.running.filter((r) => !(r.projectId === projectId && r.folder === folder && r.id === id))
  }

  // Events

  onStopped(fn: StopListener): () => void {
    this.listeners.add(fn)
    return () => this.listeners.delete(fn)
  }

  private upsert(s: LspServer) {
    this.servers = this.servers.some((x) => x.id === s.id) ? this.servers.map((x) => (x.id === s.id ? s : x)) : [...this.servers, s]
  }

  apply(kind: string, data: unknown) {
    const d = (data ?? {}) as Record<string, unknown>
    const [projectId, folder, id] = [String(d.projectId ?? ''), String(d.folder ?? ''), String(d.id ?? '')]
    switch (kind) {
      case 'lsp.changed':
        return this.upsert(d.server as LspServer)
      case 'lsp.removed':
        this.servers = this.servers.filter((s) => s.id !== id)
        return
      case 'lsp.install.progress':
        this.progress[id] = String(d.line ?? '')
        return
      case 'lsp.started':
        if (!this.running.some((r) => r.projectId === projectId && r.folder === folder && r.id === id)) {
          this.running = [...this.running, { projectId, folder, id, pid: 0, mb: 0, clients: 0, startedAt: Date.now() }]
        }
        return
      case 'lsp.stopped': {
        this.running = this.running.filter((r) => !(r.projectId === projectId && r.folder === folder && r.id === id))
        const e = { projectId, folder, id, reason: String(d.reason ?? ''), stderr: String(d.stderr ?? '') }
        if (e.reason === 'crashed') this.crashes[lspKey(projectId, folder, id)] = e.stderr
        this.listeners.forEach((l) => l(e))
        return
      }
      case 'lsp.restarted':
        this.running = this.running.filter((r) => !(r.projectId === projectId && r.folder === folder && r.id === id))
        this.notify(restartNotice(this.server(id)?.name ?? id, Number(d.mb) || 0, Number(d.limitMb) || 0))
    }
  }
}

export const lsp = new LspStore()
