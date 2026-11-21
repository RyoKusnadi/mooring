// "Open file by name" asks the server for every file path in the project at once
// (GET /api/files/names), and keeps the answer for a short while so reopening is instant.
const KEEP_MS = 30_000

export interface Index {
  paths: string[]
  /** False when the server stopped at its limit, so some files are not listed. */
  complete: boolean
}

const cache = new Map<string, { at: number; result: Index }>()

export function cached(project: string, now = Date.now()): Index | null {
  const c = cache.get(project)
  return c && now - c.at < KEEP_MS ? c.result : null
}

export function remember(project: string, result: Index, now = Date.now()) {
  cache.set(project, { at: now, result })
}

export function forgetIndex(project: string) {
  cache.delete(project)
}
