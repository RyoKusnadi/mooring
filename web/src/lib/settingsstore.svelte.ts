// Token-saving settings as the server holds them. Plain state in the app shell (the commit box
// and a new chat read it); the dialog that edits them loads on first use.
import { CommandError } from './events'
import { fetchSettings, type Model, type Settings } from './settingsapi'

type Run = <T>(name: string, args: unknown) => Promise<T>

export const defaults: Settings = { compactDefault: false, smallModel: {}, titleJobs: false, commitJobs: false, jobTokensPerDay: 200000, agentInstructions: true, projectInstructions: {}, checkpointsOff: {} }

class SettingsStore {
  value = $state<Settings>({ ...defaults })
  loaded = $state(false)
  dialog = $state(false)
  /** The language servers dialog, opened from Settings or the palette. */
  lsp = $state(false)
  /** Models by agent id, from `agent.models`. An empty list means the agent has no model option. */
  models = $state<Record<string, Model[]>>({})

  private run: Run | null = null
  /** Bumped by every event and answer, so a read that started before one does not undo it. */
  private rev = 0

  attach(run: Run) {
    this.run = run
  }

  cmd<T>(name: string, args: unknown): Promise<T> {
    return this.run ? this.run<T>(name, args) : Promise.reject(new CommandError('CLOSED', 'The connection is not ready.'))
  }

  /** Commit-message jobs are on and at least one agent has a small model. */
  get commitReady(): boolean {
    return this.value.commitJobs && Object.values(this.value.smallModel).some((m) => m !== '')
  }

  async load(): Promise<void> {
    const at = this.rev
    const s = await fetchSettings()
    if (at !== this.rev) return this.load()
    this.put(s)
    this.loaded = true
  }

  /** Saves the named keys only. The server answers with the whole settings. */
  async save(change: Partial<Settings>): Promise<Settings> {
    const s = await this.cmd<Settings>('settings.save', { settings: change })
    this.put(s)
    return s
  }

  /** Starts the agent on the project's account to read its model option. No prompt is sent. */
  async loadModels(agent: string, projectId: string): Promise<Model[]> {
    const r = await this.cmd<{ models: Model[] | null }>('agent.models', { agent, projectId })
    const list = r.models ?? []
    this.models = { ...this.models, [agent]: list }
    return list
  }

  reset(run: Run) {
    this.attach(run)
    if (this.loaded) void this.load().catch(() => {})
  }

  apply(kind: string, data: unknown) {
    if (kind === 'settings.changed') this.put(data as Settings)
  }

  private put(s: Settings) {
    this.rev++
    this.value = { ...defaults, ...s, smallModel: { ...(s.smallModel ?? {}) }, projectInstructions: { ...(s.projectInstructions ?? {}) }, checkpointsOff: { ...(s.checkpointsOff ?? {}) } }
  }
}

export const settings = new SettingsStore()
