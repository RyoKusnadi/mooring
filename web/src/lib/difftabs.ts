// Diff tabs: one per file side in a place. Pure helpers over the layout, like layout.ts.
import { groups, isSplit, type Layout, type Node, type Tab } from './layout'
import { baseName } from './paths'

export const diffTitle = (path: string, staged: boolean) => `${staged ? 'Staged' : 'Diff'}: ${baseName(path)}`

export function newDiffTab(id: string, folder: string, path: string, staged: boolean): Tab {
  return { id, kind: 'diff', title: diffTitle(path, staged), path, folder, staged }
}

/** A file as one commit (or stash) changed it. */
export function newCommitDiffTab(id: string, folder: string, commit: string, path: string): Tab {
  return { id, kind: 'diff', title: `${commit.slice(0, 7)}: ${baseName(path)}`, path, folder, staged: false, commit }
}

export function findDiffTab(layout: Layout, folder: string, path: string, staged: boolean, commit = ''): Tab | null {
  for (const g of groups(layout.root)) {
    const t = g.tabs.find((x) => x.kind === 'diff' && x.folder === folder && x.path === path && !!x.staged === staged && (x.commit ?? '') === commit)
    if (t) return t
  }
  return null
}

/** The same tab showing the other side of its file. */
export function flipDiff(layout: Layout, id: string, staged: boolean): Layout {
  const fix = (t: Tab): Tab => (t.id === id && t.kind === 'diff' && t.path !== undefined ? { ...t, staged, title: diffTitle(t.path, staged) } : t)
  const walk = (n: Node): Node => (isSplit(n) ? { ...n, children: n.children.map(walk) } : { ...n, tabs: n.tabs.map(fix) })
  return { ...layout, root: walk(layout.root) }
}

/** A commit or stash on its own tab. */
export const newCommitTab = (id: string, folder: string, commit: string, title: string): Tab => ({ id, kind: 'commit', title, folder, commit })

export function findKindTab(layout: Layout, kind: string, folder: string, commit = ''): Tab | null {
  for (const g of groups(layout.root)) {
    const t = g.tabs.find((x) => x.kind === kind && (x.folder ?? '') === folder && (x.commit ?? '') === commit)
    if (t) return t
  }
  return null
}
