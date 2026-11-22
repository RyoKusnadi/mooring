// Who last changed each line of a file: the reply of /api/git/blame, and lookups on it.
import type { Fetcher } from './api'
import { getJSON, qs } from './filesapi'

export interface BlameCommit {
  author: string
  /** Unix seconds. */
  time: number
  summary: string
}

/** Lines start..start+count-1 (1-based) last changed by one commit; commit "" is not committed yet. */
export interface BlameRun {
  start: number
  count: number
  commit: string
}

export interface Blame {
  runs: BlameRun[]
  commits: Record<string, BlameCommit>
}

export const getBlame = (project: string, folder: string, path: string, signal?: AbortSignal, f: Fetcher = (i, init) => fetch(i, init)) =>
  getJSON<Blame>(`/api/git/blame?${qs({ project, folder, path })}`, f, signal)

/** The run that holds line n (1-based), or null past the end. Runs are sorted and do not overlap. */
export function runAt(b: Blame, line: number): BlameRun | null {
  let lo = 0
  let hi = b.runs.length - 1
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    const r = b.runs[mid]
    if (line < r.start) hi = mid - 1
    else if (line >= r.start + r.count) lo = mid + 1
    else return r
  }
  return null
}
