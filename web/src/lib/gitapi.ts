// HTTP half of the git contract: status, diff, worktrees.
// Stage, commit, push and the rest are commands on the events socket (see gitstore).
import type { Fetcher } from './api'
import { FileError, getJSON, qs } from './filesapi'
import { CommandError } from './events'

export type Side = '' | 'added' | 'modified' | 'deleted' | 'renamed' | 'typechange'

export interface Change {
  path: string
  origPath?: string
  staged: Side
  unstaged: Side
  kind: 'untracked' | 'conflicted' | ''
}
export interface Status {
  repo: boolean
  branch: string
  detached: boolean
  head: string
  upstream: string
  ahead: number
  behind: number
  files: Change[]
  stagedOutside: number
  truncated: boolean
  conflicted: boolean
  /** The last commit is on some remote branch. */
  pushed: boolean
}
export type LineKind = ' ' | '+' | '-' | '\\'
export interface Hunk {
  oldStart: number
  oldLines: number
  newStart: number
  newLines: number
  header: string
  lines: [LineKind, string][]
}
export interface Diff {
  path: string
  origPath?: string
  binary: boolean
  tooLarge: boolean
  hunks: Hunk[]
  /** Names the exact diff the lines were chosen from; sent back with the choice. */
  version?: string
}
export interface Worktree {
  folder: string
  branch: string
  head: string
  chatId?: string
  /** Commits of the branch that no remote has. */
  unpushed?: number
}

const browserFetch: Fetcher = (input, init) => fetch(input, init)

export const getStatus = (project: string, folder: string, f: Fetcher = browserFetch, signal?: AbortSignal) =>
  getJSON<Status>(`/api/git/status?${qs({ project, folder })}`, f, signal)

export const getDiff = (project: string, folder: string, path: string, staged: boolean, f: Fetcher = browserFetch, signal?: AbortSignal) =>
  getJSON<Diff>(`/api/git/diff?${qs({ project, folder, path, staged: staged ? 1 : 0 })}`, f, signal)

export async function getWorktrees(project: string, f: Fetcher = browserFetch, signal?: AbortSignal): Promise<Worktree[]> {
  return (await getJSON<{ worktrees: Worktree[] }>(`/api/git/worktrees?${qs({ project })}`, f, signal)).worktrees ?? []
}

/** Changes already staged, and changes that can be staged. A file can be in both lists. */
export const stagedOf = (s: Status) => s.files.filter((c) => c.staged !== '')
export const unstagedOf = (s: Status) => s.files.filter((c) => c.unstaged !== '' || c.kind === 'untracked' || c.kind === 'conflicted')

/** What one list says about a change, in a word, so the state never rests on colour. */
export function sideWord(c: Change, staged: boolean): string {
  if (c.kind === 'conflicted') return 'conflict'
  if (!staged && c.kind === 'untracked') return 'new'
  const s = staged ? c.staged : c.unstaged
  return { added: 'new', modified: 'modified', deleted: 'deleted', renamed: 'renamed', typechange: 'type changed', '': 'modified' }[s]
}

/** One letter for the change badge: M modified, A added, D deleted, R renamed, T type changed, ? untracked, U conflicted. */
export function sideLetter(c: Change, staged: boolean): string {
  if (c.kind === 'conflicted') return 'U'
  if (!staged && c.kind === 'untracked') return '?'
  const s = staged ? c.staged : c.unstaged
  return { added: 'A', modified: 'M', deleted: 'D', renamed: 'R', typechange: 'T', '': 'M' }[s]
}

/** Paths a stage, unstage or discard of one change must name: a staged rename carries both its paths. */
export const pathsOf = (c: Change, staged: boolean): string[] => (staged && c.staged === 'renamed' && c.origPath ? [c.path, c.origPath] : [c.path])

const MESSAGES: Record<string, string> = {
  NOT_A_REPO: 'This folder is not a git repository.',
  GIT_NOT_INSTALLED: 'Git is not installed on the computer running Mooring. Install git 2.20 or newer.',
  TIMEOUT: 'Git took too long. The change may still have happened, so refresh before trying again.',
  BUSY: 'A chat or a terminal is still using this worktree. Stop the chat or close the terminal first.',
  PULL_DIVERGED: 'Pull stopped: your branch and its upstream both have new commits. Merge or rebase in a terminal.',
  ALREADY_PUSHED: 'The last commit is already on the upstream. Changing it would need a force push, which Mooring does not do.',
  NOTHING_TO_COMMIT: 'Nothing is staged. Stage the files to include, then commit.',
  NO_IDENTITY: 'Git does not know who you are. Set user.name and user.email with git config on the computer running Mooring, then commit again.',
  NO_UPSTREAM: 'This branch has no upstream yet. Use Push and set upstream.',
  NO_REMOTE: 'There is no remote called origin to push to. Add one with git remote add origin <url>.',
  PUSH_REJECTED: 'The remote has commits you do not have. Pull first.',
  AUTH_FAILED: 'The remote wants a login that Mooring cannot ask for. Set up an SSH key or a credential helper on the computer running Mooring.',
  CONFLICTED: 'Some files have merge conflicts. Resolve and stage them first.',
  DIRTY: 'This worktree has uncommitted changes or new files. Commit or discard them first. If it holds ignored files and there is no trash, they would be lost, so it is kept. Removing it keeps the branch and its commits.',
  OUTSIDE_PROJECT: 'That path is outside the project.',
  NOT_FOUND: 'Git could not find that. Refresh and try again.',
  ALREADY_EXISTS: 'A branch with that name already exists. Pick another name.',
  DIFF_CHANGED: 'This file changed since you chose these lines, so the diff was read again. Choose the lines again.',
  LOG_CHANGED: 'The branches moved, so the history starts again from the newest commit.',
  NOTHING_TO_STASH: 'There is nothing to stash. Edit a file first, or tick untracked files to stash new files.',
}

/** Text for a failed git read or command that says what happened and what to do next. */
/** Trying again can only help when the cause may pass: a dropped connection, a busy repository, a failed git run. */
export function gitCanRetry(code: string): boolean {
  return ['NETWORK', 'UNKNOWN', 'GIT_FAILED', 'BUSY', 'CONFLICT', 'OUTCOME_UNKNOWN'].includes(code)
}

export function gitError(e: unknown, fallback = 'Git could not do that. Try again.'): { code: string; message: string } {
  if (e instanceof CommandError || e instanceof FileError) {
    const code = e.code
    if (code === 'HOOK_FAILED') return { code, message: 'A commit hook refused the commit.' + (e.message ? '\n' + e.message : '') }
    if (code === 'GIT_FAILED') return { code, message: e.message || fallback }
    if (code === 'CLOSED') return { code, message: 'The connection was closed. Reload the page.' }
    if (code === 'OUTCOME_UNKNOWN') return { code, message: 'Mooring restarted while this was running. Refresh to see whether it happened.' }
    // The server names which of a chat or a terminal blocks the worktree.
    if (code === 'BUSY' || code === 'PULL_DIVERGED' || code === 'ALREADY_PUSHED') return { code, message: e.message || MESSAGES[code] }
    return { code, message: MESSAGES[code] ?? (e.message || fallback) }
  }
  if (e instanceof TypeError) return { code: 'NETWORK', message: 'Could not reach the server. Check that it is running.' }
  return { code: 'UNKNOWN', message: fallback }
}
