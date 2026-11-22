// Git state for the project on screen: which place (the project or one of its worktrees) the Changes
// section shows, that place's status, and the commands. Commands go out through `run`, like filestore.
import { chunks } from './chunks'
import { getStatus, getWorktrees, gitCanRetry, gitError, type Status, type Worktree } from './gitapi'
import { isInside } from './paths'

type Run = <T>(name: string, args: unknown) => Promise<T>

export const QUIET_MS = 1000
/** How often status is read while a git view is on screen. File watches only see files added or
 * removed at the top of a folder, so an agent's edit deeper down is found by reading again. */
export const POLL_MS = 3000

class GitStore {
  /** Which view the sidebar's lower section shows. */
  view = $state<'files' | 'changes' | 'board'>('files')
  /** Counts requests to show Changes (shortcut, palette, chat header), so the drawer on a phone opens. */
  reveal = $state(0)
  /** Counts requests to show the board (the status bar), so the drawer on a phone opens. */
  boardTick = $state(0)
  /** The board takes focus once, when it next shows after such a request. */
  focusBoard = false
  showBoard() {
    this.view = 'board'
    this.focusBoard = true
    this.boardTick++
  }
  /** Counts requests to open the branch menu (palette: Switch branch). */
  branchRequest = $state(0)
  /** The last branch request a Changes view acted on. */
  branchHandled = 0
  project = $state('')
  /** The place: `""` for the project folder, else the folder of one of its worktrees. */
  folder = $state('')
  status = $state<Status | null>(null)
  worktrees = $state<Worktree[]>([])
  /** The status read that is running, or the last one that failed. */
  loading = $state(false)
  readError = $state('')
  readRetry = $state(true)
  /** Counts finished status reads, so open diffs read again. */
  rev = $state(0)
  /** Counts `git.changed` events, so the history view starts again when branches or commits moved. */
  changes = $state(0)
  /** The command that is running. */
  busy = $state('')
  /** `source` names the panel that raised it (branch, stash), which shows it itself; the commit box shows the rest. */
  failure = $state<{ code: string; message: string; source?: string } | null>(null)
  notice = $state('')
  message = $state('')
  /** The next commit rewrites the last one. */
  amend = $state(false)

  private run: Run | null = null
  private seq = 0
  /** A worktree being removed: its status is not read, since the folder may already be gone. */
  removing = ''
  private timer: ReturnType<typeof setTimeout> | null = null
  private reading = false
  private holds = 0
  private poll: ReturnType<typeof setInterval> | null = null
  /** Unsent commit messages of the places not shown, by `project + '\n' + folder`. */
  private drafts = new Map<string, string>()

  attach(run: Run) {
    this.run = run
  }

  /** Called when the current project changes. */
  select(project: string) {
    if (project === this.project) return
    this.seq++ // a status read for the old project must not land
    this.stash()
    this.project = project
    this.folder = ''
    this.message = this.drafts.get(this.key()) ?? ''
    this.status = null
    this.worktrees = []
    this.failure = null
    this.notice = ''
    this.readError = ''
    this.refresh()
    void this.loadWorktrees()
  }

  /** Shows the Changes view for a place. */
  show(folder = this.folder) {
    this.reveal++
    if (folder !== this.folder) this.setFolder(folder)
    else this.refresh()
    void this.loadWorktrees()
  }

  setFolder(folder: string) {
    this.stash()
    this.folder = folder
    this.message = this.drafts.get(this.key()) ?? ''
    this.amend = false
    this.status = null
    this.failure = null
    this.notice = ''
    this.refresh()
  }

  private key(project = this.project, folder = this.folder) {
    return project + '\n' + folder
  }

  private stash() {
    if (this.message.trim()) this.drafts.set(this.key(), this.message)
    else this.drafts.delete(this.key())
  }

  /** A view that shows git state holds it while mounted, so status is read every POLL_MS. Returns the release. */
  hold() {
    if (this.holds++ === 0) this.poll = setInterval(() => this.tick(), POLL_MS)
    return () => {
      if (--this.holds > 0 || !this.poll) return
      clearInterval(this.poll)
      this.poll = null
    }
  }

  private tick() {
    if ((typeof document !== 'undefined' && document.hidden) || this.busy || this.reading) return
    this.refresh(true)
  }

  /** `quiet` reads (the poll) leave `loading` alone, so the Refresh button does not flicker. */
  refresh(quiet = false) {
    if (!this.project || (this.removing && this.folder === this.removing)) return
    if (this.timer) clearTimeout(this.timer)
    this.timer = null
    const seq = ++this.seq
    const [project, folder] = [this.project, this.folder]
    this.reading = true
    if (!quiet) this.loading = true
    getStatus(project, folder).then(
      (s) => {
        if (seq !== this.seq) return
        this.reading = false
        this.status = s
        this.readError = ''
        this.loading = false
        this.rev++
      },
      (e: unknown) => {
        if (seq !== this.seq) return
        this.reading = false
        const g = gitError(e, 'Could not read the changes. Try again.')
        this.readError = g.message
        this.readRetry = gitCanRetry(g.code)
        this.loading = false
      },
    )
  }

  /** Reads again once things have been quiet for a moment, so a burst of saves is one read. */
  schedule(ms = QUIET_MS) {
    if (this.timer) clearTimeout(this.timer)
    this.timer = setTimeout(() => this.refresh(), ms)
  }

  async loadWorktrees() {
    const project = this.project
    if (!project) return
    try {
      const list = await getWorktrees(project)
      if (project === this.project) this.worktrees = list
    } catch (e) {
      // Not a repository means no worktrees. Other failures keep the list the picker has; the status read reports them.
      if (gitError(e).code === 'NOT_A_REPO' && project === this.project) this.worktrees = []
    }
  }

  /** Events from the server. */
  apply(kind: string, data: unknown) {
    const d = (data ?? {}) as Record<string, unknown>
    if (kind !== 'git.changed' || String(d.projectId) !== this.project) return
    this.changes++
    if (String(d.folder ?? '') === this.folder) this.refresh()
    else this.rev++
    void this.loadWorktrees()
  }

  /** Files changed on disk: read the status again when it happened in the shown place. */
  noteFile(kind: string, data: unknown) {
    const d = (data ?? {}) as Record<string, unknown>
    if (kind === 'file.overflow') return this.schedule()
    if (String(d.projectId ?? this.project) !== this.project) return
    if ((kind === 'file.dir' || kind === 'file.changed') && isInside(String(d.path ?? ''), this.folder)) this.schedule()
  }

  // Commands (each runs once on the server)

  /** Runs a command in the shown place (or the `folder` in `args`) and reads status again. Failures land in `failure`. */
  async act<T>(name: string, args: Record<string, unknown>, reread = true): Promise<T | undefined> {
    if (!this.run) return undefined
    this.busy = name
    this.failure = null
    this.notice = ''
    try {
      return await this.run<T>(name, { projectId: this.project, folder: this.folder, ...args })
    } catch (e) {
      this.failure = gitError(e)
      return undefined
    } finally {
      this.busy = ''
      if (reread) this.refresh()
    }
  }

  /** `folder` names another place than the one shown, for a diff tab of a worktree that is not. */
  async stage(paths: string[], folder = this.folder) {
    return (await this.act('git.stage', { paths, folder })) !== undefined
  }

  async unstage(paths: string[], folder = this.folder) {
    return (await this.act('git.unstage', { paths, folder })) !== undefined
  }

  /** Returns 'trash' when the server has no trash: the caller asks again before `permanent`. */
  async discard(paths: string[], permanent = false): Promise<'done' | 'trash' | 'failed'> {
    let sent = 0
    for (const batch of chunks(paths, 1000)) {
      const r = await this.act('git.discard', { paths: batch, permanent })
      if (r === undefined) {
        if (this.failure?.code === 'TRASH_UNAVAILABLE') {
          this.failure = null
          return 'trash'
        }
        if (this.failure && sent > 0) this.failure = { ...this.failure, message: `Discarded ${sent} of ${paths.length} files, then it stopped. ${this.failure.message}` }
        return 'failed'
      }
      sent += batch.length
    }
    return 'done'
  }

  async commit() {
    const [sent, key] = [this.message, this.key()]
    const r = await this.act<{ commit: string; branch: string }>('git.commit', this.amend ? { message: sent, amend: true } : { message: sent })
    if (!r) return false
    this.amend = false
    // Text typed while the commit ran, or in another place since, is a new message: keep it.
    if (key === this.key() && this.message === sent) this.message = ''
    if (this.drafts.get(key) === sent) this.drafts.delete(key)
    this.notice = `Committed ${r.commit} on ${r.branch || 'a detached head'}.`
    return true
  }

  /** Settles one conflicted file: keep a side, or mark the edited file as done. */
  async resolve(path: string, side: 'ours' | 'theirs' | 'mark') {
    return (await this.act('git.resolve', { paths: [path], side })) !== undefined
  }

  async finish(how: 'abort' | 'continue') {
    const r = await this.act<{ operation?: string }>('git.finish', { how })
    if (!r) return false
    this.notice = how === 'abort' ? 'Stopped, and put everything back.' : r.operation ? 'Continued. The next step has conflicts to resolve.' : 'Finished.'
    return true
  }

  async push(setUpstream: boolean) {
    const r = await this.act<{ upstream: string }>('git.push', setUpstream ? { setUpstream: true } : {})
    if (!r) return false
    this.notice = `Pushed to ${r.upstream}.`
    return true
  }

  async removeWorktree() {
    const folder = this.folder
    // The place is gone once this works, so reading its status again would only answer NOT_FOUND.
    this.removing = folder
    const r = await this.act('git.worktreeRemove', { folder }, false)
    this.removing = ''
    if (r === undefined) {
      // The confirm shows why, next to the button that was pressed.
      if (this.failure) this.failure = { ...this.failure, source: 'worktree' }
      this.refresh()
      return false
    }
    if (this.folder === folder) this.setFolder('')
    void this.loadWorktrees()
    this.notice = 'Worktree removed. Its branch and commits are kept.'
    return true
  }

  /** The worktree at the shown place, if the place is one. */
  get worktree(): Worktree | undefined {
    return this.folder ? this.worktrees.find((w) => w.folder === this.folder) : undefined
  }
}

export const git = new GitStore()
