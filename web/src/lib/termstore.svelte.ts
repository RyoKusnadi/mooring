// Terminals the server knows, and what each one's view shows. Plain state: the xterm
// code lives in termview.ts and loads on first use; it registers its views here.
import { bellCounts } from './notify'
import type { WireState } from './termwire'

export interface TermInfo {
  id: string
  projectId: string
  cols: number
  rows: number
  exited: boolean
  code: number
  /** The SSH connection a remote shell runs on. */
  ssh?: string
  /** Something in the terminal needs the user: a notification escape or a bell. */
  attention?: boolean
  /** The attention came from a bell alone. */
  bell?: boolean
}

/** What the view shows about a terminal; written by the view, read by the tab and the pane. */
export interface TermUi {
  title: string
  wire: WireState
  /** Size set by another window, when it differs from what fits here. */
  letterbox: { cols: number; rows: number } | null
  /** The on-screen Ctrl key is waiting for the next key. */
  ctrl: boolean
  /** The find bar is open. */
  find: boolean
}

export const emptyUi: TermUi = { title: '', wire: 'connecting', letterbox: null, ctrl: false, find: false }

type Run = <T>(name: string, args: unknown) => Promise<T>

/** What the shell may ask of a terminal's view (termview.ts, loaded on first use). */
export interface TermViewApi {
  dispose(): void
  press(data: string): void
  focus(): void
  selection(): string
  /** Searches the scrollback: up is older, down newer, same keeps the place. at is 0 with no hit. */
  find(query: string, dir: 'up' | 'down' | 'same'): { at: number; total: number }
  clearFind(): void
}

/** Opens file paths printed in a terminal; set by the shell (termplace.ts). */
export interface TermLinks {
  /** The project file a path in terminal `id` names, or null when it is not a link. */
  path(id: string, path: string): string | null
  open(path: string, line?: number): void
}

class Terms {
  links: TermLinks | null = null
  list = $state<Record<string, TermInfo>>({})
  ui = $state<Record<string, TermUi>>({})
  /** Opening, by tab id: the command is on its way. */
  opening = $state<Record<string, boolean>>({})
  /** Why a shell did not start, by terminal id, so the tab can say it instead of "gone". */
  openError = $state<Record<string, string>>({})
  /** The first snapshot arrived, so "not in the list" means gone. */
  known = $state(false)
  views = new Map<string, TermViewApi>()

  reset(byProject: Record<string, TermInfo[]> | undefined) {
    const next: Record<string, TermInfo> = {}
    for (const infos of Object.values(byProject ?? {})) for (const i of infos) next[i.id] = i
    this.list = next
    this.known = true
  }

  apply(kind: string, data: unknown) {
    const d = data as Partial<TermInfo> & { id: string }
    if (kind === 'terminal.opened') this.list[d.id] = d as TermInfo
    else if (kind === 'terminal.exited' && this.list[d.id]) {
      this.list[d.id].exited = true
      this.list[d.id].code = d.code ?? 0
    } else if (kind === 'terminal.attention' && this.list[d.id]) {
      const a = data as { on?: boolean; bell?: boolean }
      this.list[d.id].attention = !!a.on
      this.list[d.id].bell = !!a.on && !!a.bell
    } else if (kind === 'terminal.closed') this.forget(d.id)
  }

  /** The terminal needs the user. A bell alone only counts while the bell switch is on. */
  needs(id: string): boolean {
    const t = this.list[id]
    return !!t?.attention && (!t.bell || bellCounts())
  }

  attentionCount(): number {
    return Object.keys(this.list).filter((id) => this.needs(id)).length
  }

  patch(id: string, p: Partial<TermUi>) {
    this.ui[id] = { ...emptyUi, ...this.ui[id], ...p }
  }

  private forget(id: string) {
    delete this.list[id]
    delete this.ui[id]
    this.views.get(id)?.dispose()
    this.views.delete(id)
  }

  /** Starts a shell for a tab. The tab id is the terminal id. */
  async open(run: Run, projectId: string, id: string, folder = '', ssh = '') {
    this.opening[id] = true
    try {
      const info = await run<TermInfo>('terminal.open', { id, projectId, cols: 80, rows: 24, ...(ssh ? { ssh } : folder ? { folder } : {}) })
      this.list[id] = info
      delete this.openError[id]
    } catch (e) {
      this.openError[id] = e instanceof Error ? e.message : 'The server did not say why.'
      throw e
    } finally {
      delete this.opening[id]
    }
  }

  async close(run: Run, id: string) {
    this.forget(id)
    await run('terminal.close', { id })
  }
}

export const terms = new Terms()
