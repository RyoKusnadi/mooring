// HTTP half of the history contract:
// log, search, one commit, its file diffs, branches and stashes. Changes are commands (see gitplus).
import type { Fetcher } from './api'
import { getJSON, qs } from './filesapi'
import type { Diff, Side } from './gitapi'

export type RefKind = 'head' | 'branch' | 'remote' | 'tag'
export interface Ref {
  name: string
  kind: RefKind
}
export interface Commit {
  id: string
  parents: string[]
  author: string
  email: string
  time: number
  subject: string
  refs: Ref[]
}
export interface LogPage {
  commits: Commit[]
  next: string
  total: number
  stopped?: boolean
}
export type SearchIn = 'message' | 'author' | 'code'
export interface CommitFile {
  path: string
  origPath?: string
  change: Exclude<Side, ''>
  binary: boolean
}
export interface CommitInfo {
  commit: Commit
  message: string
  committer: string
  files: CommitFile[]
  truncated?: boolean
}
export interface Branch {
  name: string
  head: boolean
  upstream: string
  time: number
}
export interface Stash {
  id: string
  index: number
  message: string
  branch: string
  time: number
}

const browserFetch: Fetcher = (input, init) => fetch(input, init)

export interface LogQuery {
  cursor?: string
  limit?: number
  q?: string
  in?: SearchIn
}

export const getLog = (project: string, folder: string, o: LogQuery = {}, signal?: AbortSignal, f: Fetcher = browserFetch) =>
  getJSON<LogPage>(`/api/git/log?${qs({ project, folder, cursor: o.cursor || undefined, limit: o.limit, q: o.q, in: o.q ? o.in : undefined })}`, f, signal)

export const getCommit = (project: string, folder: string, id: string, signal?: AbortSignal, f: Fetcher = browserFetch) =>
  getJSON<CommitInfo>(`/api/git/commit?${qs({ project, folder, id })}`, f, signal)

export const getCommitDiff = (project: string, folder: string, commit: string, path: string, signal?: AbortSignal, f: Fetcher = browserFetch) =>
  getJSON<Diff>(`/api/git/diff?${qs({ project, folder, commit, path })}`, f, signal)

export const getBranches = async (project: string, folder: string, signal?: AbortSignal, f: Fetcher = browserFetch) => {
  const r = await getJSON<{ current: string; branches: Branch[] }>(`/api/git/branches?${qs({ project, folder })}`, f, signal)
  return { current: r.current, branches: r.branches ?? [] }
}

export const getStashes = async (project: string, folder: string, signal?: AbortSignal, f: Fetcher = browserFetch) =>
  (await getJSON<{ stashes: Stash[] }>(`/api/git/stashes?${qs({ project, folder })}`, f, signal)).stashes ?? []

export const shortId = (id: string) => id.slice(0, 7)

/** "3 minutes ago", "2 days ago", else the date. `now` is in ms. */
export function relativeTime(unix: number, now = Date.now()): string {
  const s = Math.max(0, Math.round(now / 1000 - unix))
  if (s < 45) return 'just now'
  const units: [number, string][] = [[60, 'minute'], [3600, 'hour'], [86400, 'day'], [86400 * 7, 'week'], [86400 * 30, 'month'], [86400 * 365, 'year']]
  let best: [number, string] = [1, 'second']
  for (const u of units) if (s >= u[0]) best = u
  const n = Math.floor(s / best[0])
  return `${n} ${best[1]}${n === 1 ? '' : 's'} ago`
}

export const fullTime = (unix: number) => new Date(unix * 1000).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })

/** What kind of ref a label is, in a word, so the kind never rests on colour. */
export const refWord = (k: RefKind) => ({ head: 'HEAD', branch: 'branch', remote: 'remote', tag: 'tag' })[k]

/** True when a "Name <mail>" committer is the author, ignoring how the pair is written. */
export function sameWho(committer: string, name: string, email: string): boolean {
  const m = /^(.*?)\s*<([^>]*)>\s*$/.exec(committer.trim())
  const [n, e] = m ? [m[1], m[2]] : [committer.trim(), email]
  return n.trim() === name.trim() && e.trim().toLowerCase() === email.trim().toLowerCase()
}
