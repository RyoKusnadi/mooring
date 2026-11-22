// The commit list of one place: pages arrive as the user scrolls, and the graph's lanes are added
// page by page (see graph.ts). A search replaces the list with its results and draws no lanes.
import { gitError } from './gitapi'
import { addPage, newGraph, type GraphRow, type GraphState } from './graph'
import { getLog, type Commit, type LogPage, type SearchIn } from './historyapi'

export const PAGE = 200
/** After the refs move, pages are read again up to what was on screen, but never more than this many at once. */
const KEEP_PAGES = 10

export class History {
  commits = $state.raw<Commit[]>([])
  rows = $state.raw<GraphRow[]>([])
  total = $state(0)
  next = $state('')
  loading = $state(false)
  error = $state('')
  /** A search that ran out of time answers what it found so far. */
  stopped = $state(false)
  /** What is searched now, `''` for the plain log. */
  query = $state('')
  mode = $state<SearchIn>('message')
  /** Set when the refs moved and the list was read again from the first page. */
  restarted = $state(0)

  private graph: GraphState = newGraph()
  private seq = 0
  private ctl: AbortController | undefined

  constructor(
    private project: string,
    private folder: string,
  ) {}

  /** Reads the first page of the log, or of a search. */
  async start(query = '', mode: SearchIn = 'message') {
    this.query = query.trim()
    this.mode = mode
    await this.read('', false)
  }

  /** Reads the next page, when there is one and none is being read. */
  async more() {
    if (this.next && !this.loading && !this.error) await this.read(this.next, true)
  }

  stop() {
    this.seq++
    this.ctl?.abort()
  }

  private async read(cursor: string, append: boolean): Promise<void> {
    this.ctl?.abort()
    const ctl = (this.ctl = new AbortController())
    const seq = ++this.seq
    this.loading = true
    this.error = ''
    if (!append) {
      this.commits = []
      this.rows = []
      this.next = ''
      this.total = 0
      this.stopped = false
      this.graph = newGraph()
    }
    try {
      const page = await getLog(this.project, this.folder, { cursor, limit: PAGE, q: this.query || undefined, in: this.mode }, ctl.signal)
      if (seq !== this.seq) return
      this.take(page, append)
    } catch (e) {
      if (seq !== this.seq) return
      const err = gitError(e, 'Could not read the history. Try again.')
      if (err.code === 'LOG_CHANGED') return void (await this.restart())
      this.error = err.message
    } finally {
      if (seq === this.seq) this.loading = false
    }
  }

  private take(page: LogPage, append: boolean) {
    const commits = page.commits ?? []
    this.commits = append ? [...this.commits, ...commits] : commits
    if (!this.query) this.rows = append ? [...this.rows, ...addPage(this.graph, commits)] : addPage(this.graph, commits)
    this.next = page.next
    this.total = page.total
    this.stopped = !!page.stopped
  }

  /** The branches moved: read from the first page again, as far as the list was, then swap it in so the scroll position holds. */
  async restart() {
    const keep = Math.min(this.commits.length, KEEP_PAGES * PAGE)
    const seq = ++this.seq
    this.ctl?.abort()
    const ctl = (this.ctl = new AbortController())
    this.loading = true
    const graph = newGraph()
    let commits: Commit[] = []
    let rows: GraphRow[] = []
    let total = 0
    let next = ''
    try {
      do {
        const page = await getLog(this.project, this.folder, { cursor: next, limit: PAGE, q: this.query || undefined, in: this.mode }, ctl.signal)
        if (seq !== this.seq) return
        commits = commits.concat(page.commits ?? [])
        if (!this.query) rows = rows.concat(addPage(graph, page.commits ?? []))
        total = page.total
        next = page.next
      } while (next && commits.length < keep)
      this.graph = graph
      this.commits = commits
      this.rows = rows
      this.total = total
      this.next = next
      this.error = ''
      this.restarted++
    } catch (e) {
      if (seq === this.seq) this.error = gitError(e, 'Could not read the history. Try again.').message
    } finally {
      if (seq === this.seq) this.loading = false
    }
  }
}
