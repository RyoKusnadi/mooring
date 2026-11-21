// Accounts as the server reports them. Plain state in the app shell (a chat reads it to offer
// a switch); the dialog that edits them loads on first use. The server never sends a login
// token, so there is none here.
import { fetchAccounts, type Account } from './accountapi'
import { CommandError } from './events'

type Run = <T>(name: string, args: unknown) => Promise<T>

class AccountStore {
  accounts = $state<Account[]>([])
  /** Project id, agent, account id. */
  chosen = $state<Record<string, Record<string, string>>>({})
  loaded = $state(false)
  dialog = $state(false)

  private run: Run | null = null
  /** Bumped by every event and command answer, so a read that started before one does not undo it. */
  private rev = 0

  attach(run: Run) {
    this.run = run
  }

  cmd<T>(name: string, args: unknown): Promise<T> {
    return this.run ? this.run<T>(name, args) : Promise.reject(new CommandError('CLOSED', 'The connection is not ready.'))
  }

  /** Agents that can have more than one login. */
  get agents(): string[] {
    return [...new Set(this.accounts.map((a) => a.agent))]
  }

  has(agent: string): boolean {
    return this.accounts.some((a) => a.agent === agent)
  }

  of(agent: string): Account[] {
    return this.accounts.filter((a) => a.agent === agent)
  }

  get(id: string): Account | undefined {
    return this.accounts.find((a) => a.id === id)
  }

  /** The account a project uses for an agent: its own choice, else the agent's default. */
  forProject(projectId: string, agent: string): Account | undefined {
    const id = this.chosen[projectId]?.[agent]
    return (id && this.get(id)) || this.of(agent).find((a) => a.isDefault) || this.of(agent)[0]
  }

  async load(): Promise<void> {
    const at = this.rev
    const r = await fetchAccounts()
    if (at !== this.rev) return this.load()
    this.accounts = r.accounts ?? []
    this.chosen = r.projects ?? {}
    this.loaded = true
  }

  /** Adds an account (no id) or renames one. `dir` empty means a folder Mooring manages. */
  async save(account: { id?: string; agent: string; name: string; dir?: string }): Promise<Account> {
    const a = await this.cmd<Account>('account.save', { account })
    this.put(a)
    return a
  }

  async remove(id: string) {
    await this.cmd('account.remove', { id })
    this.drop(id)
  }

  async setDefault(id: string): Promise<Account> {
    const a = await this.cmd<Account>('account.setDefault', { id })
    // The server sends changed for the old default too; this keeps the list right until it arrives.
    this.rev++
    this.accounts = this.accounts.map((x) => (x.agent === a.agent ? { ...x, isDefault: x.id === a.id } : x))
    return a
  }

  /** Opens a terminal tab in the project where the user signs in. Returns its id. */
  async login(id: string, projectId: string): Promise<string> {
    return (await this.cmd<{ terminalId: string }>('account.login', { id, projectId })).terminalId
  }

  /** Moves a project to an account. Returns the chats that restarted. */
  async setProjectAccount(projectId: string, agent: string, accountId: string): Promise<string[]> {
    const r = await this.cmd<{ restarted?: string[] }>('project.setAccount', { projectId, agent, accountId })
    this.choose(projectId, agent, accountId)
    return r.restarted ?? []
  }

  /** A new snapshot: the socket is live again, so commands can go and the list may have changed. */
  reset(run: Run) {
    this.attach(run)
    if (this.loaded) void this.load().catch(() => {})
  }

  apply(kind: string, data: unknown) {
    if (kind === 'account.changed') this.put(data as Account)
    else if (kind === 'account.removed') this.drop(String((data as { id?: string })?.id ?? ''))
    else if (kind === 'project.accountChanged') {
      const d = data as { projectId: string; agent: string; accountId: string }
      this.choose(d.projectId, d.agent, d.accountId)
    }
  }

  private choose(projectId: string, agent: string, accountId: string) {
    this.rev++
    this.chosen = { ...this.chosen, [projectId]: { ...this.chosen[projectId], [agent]: accountId } }
  }

  private put(a: Account) {
    this.rev++
    const i = this.accounts.findIndex((x) => x.id === a.id)
    if (i < 0) this.accounts = [...this.accounts, a]
    else this.accounts = this.accounts.map((x) => (x.id === a.id ? a : x))
    // Only one account per agent is the default.
    if (a.isDefault) this.accounts = this.accounts.map((x) => (x.agent === a.agent && x.id !== a.id && x.isDefault ? { ...x, isDefault: false } : x))
  }

  private drop(id: string) {
    this.rev++
    this.accounts = this.accounts.filter((a) => a.id !== id)
    // Projects that used it go back to the agent's default; the server says so with events, and until then the choice is dropped.
    const next: Record<string, Record<string, string>> = {}
    for (const [p, m] of Object.entries(this.chosen)) next[p] = Object.fromEntries(Object.entries(m).filter(([, v]) => v !== id))
    this.chosen = next
  }
}

export const accounts = new AccountStore()
