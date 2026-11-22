// Files opened lately, newest first, per project. "Open file by name" ranks them before the rest.
import { browserStore } from './appearance'

const MAX = 30
const key = (project: string) => `mooring.recent.${project}`

export function recentFiles(project: string, store = browserStore()): string[] {
  try {
    const v: unknown = JSON.parse(store?.getItem(key(project)) ?? '[]')
    return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string').slice(0, MAX) : []
  } catch {
    return []
  }
}

export function noteOpened(project: string, path: string, store = browserStore()) {
  if (!project || !path) return
  const next = [path, ...recentFiles(project, store).filter((p) => p !== path)].slice(0, MAX)
  try {
    store?.setItem(key(project), JSON.stringify(next))
  } catch {
    // A full or blocked store only costs the ranking boost.
  }
}
