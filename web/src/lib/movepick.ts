// The folders a "Move to…" dialog offers: every folder that holds a listed file, plus the folders the tree has loaded.
import { canMove } from './treedrag'

/** Folder paths ("" is the project folder) from file paths and extra known folders, sorted, without duplicates. */
export function folderList(filePaths: readonly string[], known: readonly string[] = []): string[] {
  const set = new Set<string>([''])
  const add = (dir: string) => {
    for (let at = dir.indexOf('/'); ; at = dir.indexOf('/', at + 1)) {
      set.add(at < 0 ? dir : dir.slice(0, at))
      if (at < 0) break
    }
  }
  for (const p of filePaths) {
    const i = p.lastIndexOf('/')
    if (i > 0) add(p.slice(0, i))
  }
  for (const k of known) if (k) add(k)
  return [...set].sort((a, b) => a.localeCompare(b))
}

/** The folders `from` can move into, narrowed by what the user typed. An untouched field shows them all. */
export function targets(folders: readonly string[], from: string, query: string): string[] {
  const q = query.trim().toLowerCase()
  return folders.filter((d) => canMove(from, d) && d.toLowerCase().includes(q))
}
