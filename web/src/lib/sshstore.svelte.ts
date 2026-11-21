// SSH as the server reports it: saved hosts, live connections, questions waiting for an answer and
// port forwards. Plain state in the app shell; the screens that use it load on first use.
import { CommandError } from './events'
import { fetchHosts, type Forward, type Host, type HostDraft, type SshConn, type SshPrompt } from './sshapi'

type Run = <T>(name: string, args: unknown) => Promise<T>

export interface SshSnapshot {
  sshPrompts?: SshPrompt[]
  sshConnections?: SshConn[]
  sshForwards?: Forward[]
}
/** A connection that stopped for a reason the user has to read. */
export interface Failure {
  hostId: string
  code: string
  message: string
}

/** Codes where the user said no, so there is nothing to explain. */
const QUIET = new Set(['CANCELLED', 'HOST_KEY_REJECTED'])

class SshStore {
  hosts = $state<Host[]>([])
  canRemember = $state(false)
  loaded = $state(false)
  conns = $state<SshConn[]>([])
  prompts = $state<SshPrompt[]>([])
  forwards = $state<Forward[]>([])
  /** Why a connection ended, by host id (the last one). Dropped when the host's last tab closes. */
  closedHost = $state<Record<string, string>>({})
  /** Hosts a connect command is running for. */
  connecting = $state<Record<string, boolean>>({})
  failure = $state<Failure | null>(null)
  dialog = $state(false)

  private run: Run | null = null
  private pending = new Map<string, Promise<string>>()
  /** A password typed together with the host-key trust, kept in memory until the password question comes, then dropped. */
  private connHost = new Map<string, string>()
  private armed = new Map<string, { value: string; remember: boolean }>()

  attach(run: Run) {
    this.run = run
  }

  cmd<T>(name: string, args: unknown): Promise<T> {
    return this.run ? this.run<T>(name, args) : Promise.reject(new CommandError('CLOSED', 'The connection is not ready.'))
  }

  host(id: string): Host | undefined {
    return this.hosts.find((h) => h.id === id)
  }

  /** The live connection of a host, if there is one. */
  connFor(hostId: string): SshConn | undefined {
    return this.conns.find((c) => c.hostId === hostId && c.state === 'connected')
  }

  /** Something SSH shows on top of the page: the hosts dialog, a question, or a failure. */
  get modal(): boolean {
    return this.dialog || this.prompts.length > 0 || !!this.failure
  }

  forgetClosed(hostId: string) {
    delete this.closedHost[hostId]
  }

  hostOfConn(connId: string): Host | undefined {
    const c = this.conns.find((x) => x.id === connId)
    return c ? this.host(c.hostId) : undefined
  }

  /** The host a connection that is still signing in is for. */
  hostOfPending(connId: string): Host | undefined {
    const id = this.connHost.get(connId)
    return id ? this.host(id) : undefined
  }

  forwardsOf(connId: string): Forward[] {
    return this.forwards.filter((f) => f.conn === connId)
  }

  // Hosts

  async load() {
    const r = await fetchHosts()
    this.hosts = r.hosts ?? []
    this.canRemember = !!r.canRemember
    this.loaded = true
  }

  async saveHost(draft: HostDraft): Promise<Host> {
    // A live connection was made with the old settings; a test of the new ones needs a new one.
    const old = draft.id ? this.host(draft.id) : undefined
    const live = old ? this.connFor(old.id) : undefined
    const changed = !!old && (old.address !== draft.address || old.port !== draft.port || old.user !== draft.user || old.auth !== draft.auth || old.keyPath !== draft.keyPath)
    if (live && changed) await this.disconnect(live.id)
    const h = await this.cmd<Host>('ssh.hostSave', { host: draft })
    this.hosts = [...this.hosts.filter((x) => x.id !== h.id), h]
    return h
  }

  async removeHost(id: string) {
    await this.cmd('ssh.hostRemove', { id })
    this.hosts = this.hosts.filter((h) => h.id !== id)
  }

  // Connections

  /** Signs in to a host, or returns the connection it already has. Prompts arrive as events meanwhile. */
  connect(hostId: string, quiet = false): Promise<string> {
    const live = this.connFor(hostId)
    if (live) return Promise.resolve(live.id)
    const waiting = this.pending.get(hostId)
    if (waiting) return waiting
    const id = 'c' + crypto.randomUUID().replaceAll('-', '').slice(0, 20)
    this.connecting[hostId] = true
    this.connHost.set(id, hostId)
    const p = this.cmd<SshConn>('ssh.connect', { id, hostId })
      .then((c) => {
        this.upsertConn(c)
        return c.id
      })
      .catch((e: unknown) => {
        if (e instanceof CommandError && !QUIET.has(e.code) && !quiet) {
          this.failure = { hostId, code: e.code, message: e.message }
          // The dialog offers to forget the saved key only when the server says Mooring holds it.
          if (e.code === 'HOST_KEY_CHANGED') void this.load().catch(() => {})
        }
        throw e
      })
      .finally(() => {
        this.pending.delete(hostId)
        delete this.connecting[hostId]
        this.connHost.delete(id)
        this.armed.delete(id)
      })
    this.pending.set(hostId, p)
    return p
  }

  /** Signs in to a saved host to see whether it works. The result is a line to show, never a dialog. */
  async test(hostId: string): Promise<{ ok: boolean; message: string }> {
    const h = this.host(hostId)
    const name = h?.name || 'the host'
    try {
      await this.connect(hostId, true)
      return { ok: true, message: `Connected to ${h ? `${h.user}@${h.address}` : name}. The host works.` }
    } catch (e) {
      if (e instanceof CommandError && QUIET.has(e.code)) return { ok: false, message: 'Not tested: the host key was not trusted, or the sign-in was cancelled.' }
      return { ok: false, message: e instanceof Error && e.message ? e.message : 'The connection failed. Check the address and try again.' }
    }
  }

  /** Holds a password for the question this connection is about to ask; used once, never written anywhere. */
  arm(connId: string, a: { value: string; remember: boolean }) {
    this.armed.set(connId, a)
  }

  async disconnect(connId: string) {
    await this.cmd('ssh.disconnect', { id: connId })
    this.dropConn(connId, 'You disconnected.')
  }

  async forgetKey(host: Host) {
    await this.cmd('ssh.forgetHostKey', { address: host.address, port: host.port })
  }

  /** Answers a question. The first answer from any window counts. */
  async answer(promptId: string, a: { value?: string; remember?: boolean; accept?: boolean; cancel?: boolean }) {
    try {
      await this.cmd('ssh.answer', { promptId, ...a })
    } catch (e) {
      // Answered somewhere else, or ended meanwhile: nothing left to do. Any other failure keeps the question open.
      if (!(e instanceof CommandError && e.code === 'NOT_FOUND')) throw e
    }
    this.prompts = this.prompts.filter((p) => p.promptId !== promptId)
  }

  // Forwards

  async forward(conn: string, localPort: number, remoteHost: string, remotePort: number) {
    this.upsertForward(await this.cmd<Forward>('ssh.forward', { conn, localPort, remoteHost, remotePort }))
  }

  async stopForward(id: string) {
    await this.cmd('ssh.forwardStop', { id })
    this.forwards = this.forwards.filter((f) => f.id !== id)
  }

  // Snapshot and events

  reset(s: SshSnapshot) {
    this.prompts = s.sshPrompts ?? []
    this.conns = s.sshConnections ?? []
    this.forwards = s.sshForwards ?? []
    if (this.loaded) void this.load().catch(() => {})
  }

  apply(kind: string, data: unknown) {
    const d = (data ?? {}) as Record<string, string>
    switch (kind) {
      case 'ssh.hostsChanged':
        return void this.load().catch(() => {})
      case 'ssh.connected':
        return this.upsertConn(data as SshConn)
      case 'ssh.closed':
        return this.dropConn(d.id, d.reason || 'The connection closed.')
      case 'ssh.prompt': {
        const p = data as SshPrompt
        const a = this.armed.get(p.connId)
        if (a && p.kind === 'password' && !p.retry) {
          this.armed.delete(p.connId)
          void this.answer(p.promptId, { value: a.value, remember: a.remember && this.canRemember }).catch(() => {})
          return
        }
        // Anything else (a wrong password, another question) is asked of the user, so the stored one is dropped.
        this.armed.delete(p.connId)
        this.prompts = [...this.prompts.filter((x) => x.promptId !== p.promptId), p]
        return
      }
      case 'ssh.prompt.closed':
        this.prompts = this.prompts.filter((p) => p.promptId !== d.promptId)
        return
      case 'ssh.forwarded':
        return this.upsertForward(data as Forward)
      case 'ssh.forwardStopped':
        this.forwards = this.forwards.filter((f) => f.id !== d.id)
    }
  }

  private upsertConn(c: SshConn) {
    this.conns = [...this.conns.filter((x) => x.id !== c.id), c]
  }

  private upsertForward(f: Forward) {
    this.forwards = [...this.forwards.filter((x) => x.id !== f.id), f]
  }

  private dropConn(id: string, reason: string) {
    const c = this.conns.find((x) => x.id === id)
    if (c) this.closedHost[c.hostId] = reason
    this.conns = this.conns.filter((x) => x.id !== id)
    this.forwards = this.forwards.filter((f) => f.conn !== id)
  }
}

export const ssh = new SshStore()
