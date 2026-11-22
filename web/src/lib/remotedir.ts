// Pure helpers for the remote folder browser.
import type { Entry } from './filesapi'
import { groups, isSplit, type Layout, type Node, type Tab } from './layout'
import { baseName } from './paths'

export const MAX_UPLOAD = 10 * 1024 * 1024

/** Folders first, then names in a natural order. */
export function sortEntries(entries: Entry[]): Entry[] {
  const rank = (e: Entry) => (e.kind === 'dir' ? 0 : 1)
  return entries.slice().sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }))
}

/** The parts of a folder path as breadcrumb buttons, each with the path it opens. The root is first. */
export function crumbs(path: string, rootLabel: string): { label: string; path: string }[] {
  const out = [{ label: rootLabel, path: '' }]
  let at = ''
  for (const part of path.split('/').filter(Boolean)) {
    at = at ? `${at}/${part}` : part
    out.push({ label: part, path: at })
  }
  return out
}

/** Why a file cannot be uploaded, or ''. The server takes up to 10 MiB per file. */
export function uploadProblem(name: string, size: number): string {
  if (size > MAX_UPLOAD) return `${name} is over 10 MB. Uploads are limited to 10 MB per file.`
  return ''
}

/** Upload errors that are about this one file, in plain words. */
export function uploadMessage(name: string, code: string, message: string): string {
  if (code === 'ALREADY_EXISTS') return `${name} already exists in this folder. Rename or delete it first, then upload again.`
  return `${name}: ${message}`
}

const within = (p: string, dir: string) => p === dir || p.startsWith(dir + '/')

/** Open remote file tabs of `host` whose file is `path` or inside it. */
export function remoteTabsIn(layout: Layout, host: string, path: string): Tab[] {
  return groups(layout.root).flatMap((g) => g.tabs.filter((t) => t.kind === 'rfile' && t.host === host && t.path !== undefined && within(t.path, path)))
}

/** After a remote rename: tabs under `from` point at the same place under `to`. */
export function retargetRemote(layout: Layout, host: string, from: string, to: string): Layout {
  const hit = new Set(remoteTabsIn(layout, host, from).map((t) => t.id))
  if (hit.size === 0) return layout
  const fix = (t: Tab): Tab => {
    if (!hit.has(t.id)) return t
    const path = to + t.path!.slice(from.length)
    return { ...t, path, title: t.title.slice(0, Math.max(0, t.title.length - baseName(t.path!).length)) + baseName(path) }
  }
  const walk = (n: Node): Node => (isSplit(n) ? { ...n, children: n.children.map(walk) } : { ...n, tabs: n.tabs.map(fix) })
  return { ...layout, root: walk(layout.root) }
}
