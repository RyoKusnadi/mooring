// Branches, stashes and line staging: the phase 8 commands and reads, on top of gitstore's `act`.
import { chats } from './chatstore.svelte'
import { git } from './gitstore.svelte'
import { gitError } from './gitapi'
import { getBranches, getStashes, type Branch, type Stash } from './historyapi'

/** The chat that is running where a command was refused with BUSY, so the message can name it. */
export function runningChatName(folder: string): string {
  const here = git.worktrees.find((w) => w.folder === folder)
  for (const c of Object.values(chats.list)) {
    if (c.state === 'stopped' || c.projectId !== git.project) continue
    if ((here?.chatId === c.id) || (c.spec?.folder?.path ?? '') === folder) return c.title || 'New chat'
  }
  return ''
}

export function busyText(folder: string): string {
  const name = runningChatName(folder)
  const who = name ? `The chat "${name}"` : 'A chat'
  return `${who} is still running here, and its files would change under it. Stop or close that chat, then try again.`
}

class GitPlus {
  current = $state('')
  branches = $state<Branch[]>([])
  branchError = $state('')
  stashes = $state<Stash[]>([])
  stashError = $state('')
  private ctl: AbortController | undefined

  async loadBranches() {
    this.ctl?.abort()
    const ctl = (this.ctl = new AbortController())
    const [project, folder] = [git.project, git.folder]
    try {
      const r = await getBranches(project, folder, ctl.signal)
      if (ctl.signal.aborted) return
      this.current = r.current
      this.branches = r.branches
      this.branchError = ''
    } catch (e) {
      if (!ctl.signal.aborted) this.branchError = gitError(e, 'Could not read the branches. Try again.').message
    }
  }

  private stashSeq = 0

  async loadStashes() {
    const [project, folder] = [git.project, git.folder]
    if (!project) return
    const seq = ++this.stashSeq
    try {
      const list = await getStashes(project, folder)
      if (seq === this.stashSeq && project === git.project && folder === git.folder) {
        this.stashes = list
        this.stashError = ''
      }
    } catch (e) {
      if (seq === this.stashSeq && project === git.project && folder === git.folder) {
        const err = gitError(e, 'Could not read the stashes. Try again.')
        if (err.code === 'NOT_A_REPO') this.stashes = []
        else this.stashError = err.message
      }
    }
  }

  /** Tags a failure with the panel that shows it, and a refusal for a running chat names that chat. */
  private tag(source: string): void {
    if (!git.failure) return
    const message = git.failure.code === 'BUSY' ? busyText(git.folder) : git.failure.message
    git.failure = { ...git.failure, message, source }
  }

  async switchTo(name: string) {
    const r = await git.act<{ branch: string }>('git.switch', { branch: name })
    this.tag('branch')
    if (r) git.notice = `Switched to ${r.branch}.`
    void this.loadBranches()
    return !!r
  }

  async createBranch(name: string) {
    const r = await git.act<{ branch: string }>('git.branchCreate', { name })
    this.tag('branch')
    if (r) git.notice = `Made ${r.branch} and switched to it.`
    void this.loadBranches()
    return !!r
  }

  async stashSave(message: string, untracked: boolean) {
    const r = await git.act<{ id: string }>('git.stashSave', { ...(message.trim() ? { message: message.trim() } : {}), ...(untracked ? { untracked: true } : {}) })
    this.tag('stash')
    if (r) git.notice = 'Stashed your changes. They are out of the working copy until you apply the stash.'
    void this.loadStashes()
    return !!r
  }

  async stashApply(id: string, pop: boolean) {
    const r = await git.act(pop ? 'git.stashPop' : 'git.stashApply', { id })
    this.tag('stash')
    if (r) git.notice = pop ? 'Applied the stash and removed it from the list.' : 'Applied the stash. It stays in the list.'
    else if (git.failure?.code === 'CONFLICTED') {
      git.failure = {
        code: 'CONFLICTED',
        source: 'stash',
        message:
          'The stash did not apply cleanly. The files with conflicts are listed under Changes, marked conflict. ' +
          'Open each, fix it, and stage it. The stash is kept, so drop it once you are done.',
      }
    } else if (git.failure?.code === 'NOT_FOUND') git.failure = { code: 'NOT_FOUND', source: 'stash', message: 'That stash is gone. The list was read again.' }
    void this.loadStashes()
    return !!r
  }

  async stashDrop(id: string) {
    const r = await git.act('git.stashDrop', { id })
    this.tag('stash')
    if (r) git.notice = 'Dropped the stash.'
    else if (git.failure?.code === 'NOT_FOUND') git.failure = { code: 'NOT_FOUND', source: 'stash', message: 'That stash is gone. The list was read again.' }
    void this.loadStashes()
    return !!r
  }

  /** Stages (or unstages) the chosen lines of a file's diff, naming the diff they were chosen from. */
  async stageLines(folder: string, path: string, staged: boolean, version: string, lines: [number, number][]) {
    const r = await git.act('git.stageLines', { folder, path, staged, version, lines })
    return r !== undefined
  }
}

export const gitplus = new GitPlus()
