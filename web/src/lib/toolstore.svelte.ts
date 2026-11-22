// Tool servers as the server reports them. Plain state in the app shell (the new chat screen
// reads it); the dialog that edits them loads on first use. Secret values pass through save
// and are never kept here: the server answers with names and a `set` flag only.
import { CommandError } from './events'
import type { Secrets, ServerDraft, ToolServer } from './toolapi'
import { fetchToolServers } from './toolapi'

type Run = <T>(name: string, args: unknown) => Promise<T>

export interface SignInStart {
  url: string
  redirect: string
}

class ToolStore {
  servers = $state<ToolServer[]>([])
  canSaveSecrets = $state(true)
  loaded = $state(false)
  dialog = $state(false)
  /** Why the last check or enable failed, by id. Held in the browser only; a check that passes clears it. */
  failures = $state<Record<string, string>>({})
  /** Ids a check is running for. */
  checking = $state<Record<string, boolean>>({})

  private run: Run | null = null

  attach(run: Run) {
    this.run = run
  }

  cmd<T>(name: string, args: unknown): Promise<T> {
    return this.run ? this.run<T>(name, args) : Promise.reject(new CommandError('CLOSED', 'The connection is not ready.'))
  }

  get enabled(): ToolServer[] {
    return this.servers.filter((s) => s.enabled)
  }

  get(id: string): ToolServer | undefined {
    return this.servers.find((s) => s.id === id)
  }

  /** Bumped by every event and command answer, so a read that started before one does not undo it. */
  private rev = 0

  async load(): Promise<void> {
    const at = this.rev
    const r = await fetchToolServers()
    if (at !== this.rev) {
      // Something newer arrived while the read was out: read again.
      return this.load()
    }
    this.servers = r.servers ?? []
    this.canSaveSecrets = r.canSaveSecrets !== false
    this.loaded = true
  }

  /** Adds or edits a server. `allow` is true only from the consent screen. */
  async save(server: ServerDraft, secrets: Secrets | undefined, allow: boolean): Promise<ToolServer> {
    const s = await this.cmd<ToolServer>('toolServer.save', { server, ...(secrets ? { secrets } : {}), ...(allow ? { allow: true } : {}) })
    this.put(s)
    delete this.failures[s.id]
    return s
  }

  async remove(id: string) {
    await this.cmd('toolServer.remove', { id })
    this.drop(id)
  }

  async enable(id: string, enabled: boolean): Promise<ToolServer> {
    try {
      const s = await this.cmd<ToolServer>('toolServer.enable', { id, enabled })
      this.put(s)
      delete this.failures[id]
      return s
    } catch (e) {
      if (e instanceof CommandError && e.code === 'CHECK_FAILED') this.failures[id] = e.message
      throw e
    }
  }

  /** Runs the health check. A failure is kept for the list and rethrown for the screen that asked.
   *  The server may answer from a tool list it still holds fresh; `fresh` makes it list again. */
  async check(id: string, fresh = false): Promise<ToolServer> {
    this.checking[id] = true
    try {
      const s = await this.cmd<ToolServer>('toolServer.check', fresh ? { id, fresh: true } : { id })
      this.put(s)
      delete this.failures[id]
      return s
    } catch (e) {
      if (e instanceof CommandError && e.code === 'CHECK_FAILED') this.failures[id] = e.message
      throw e
    } finally {
      delete this.checking[id]
    }
  }

  signIn(id: string) {
    return this.cmd<SignInStart>('toolServer.signIn', { id })
  }

  async signInFinish(id: string, url: string): Promise<ToolServer> {
    const s = await this.cmd<ToolServer>('toolServer.signInFinish', { id, url })
    this.put(s)
    return s
  }

  async signOut(id: string): Promise<ToolServer> {
    const s = await this.cmd<ToolServer>('toolServer.signOut', { id })
    this.put(s)
    return s
  }

  /** A new snapshot: the socket is live again, so commands can go and the list may have changed. */
  reset(run: Run) {
    this.attach(run)
    if (this.loaded) void this.load().catch(() => {})
  }

  apply(kind: string, data: unknown) {
    if (kind === 'toolServer.changed') this.put(data as ToolServer)
    else if (kind === 'toolServer.removed') this.drop(String((data as { id?: string })?.id ?? ''))
  }

  private put(s: ToolServer) {
    this.rev++
    const i = this.servers.findIndex((x) => x.id === s.id)
    if (i < 0) this.servers = [...this.servers, s]
    else this.servers = this.servers.map((x) => (x.id === s.id ? s : x))
  }

  private drop(id: string) {
    this.rev++
    this.servers = this.servers.filter((s) => s.id !== id)
    delete this.failures[id]
  }
}

export const tools = new ToolStore()
