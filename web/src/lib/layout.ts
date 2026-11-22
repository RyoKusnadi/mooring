// Pure functions over the contract's Layout tree. Nothing here mutates its input.
// A "group" is a leaf node: a pane with a tab bar. Only the root may be an empty group.

export interface Tab {
  id: string
  kind: string
  title: string
  /** Kind 'file' only: the file's path inside the project. */
  path?: string
  /** Kinds 'diff', 'terminal', 'history' and 'commit': the place (project folder or worktree) the tab belongs to. */
  folder?: string
  /** Kind 'diff' only: the staged side of the file, not the working side. */
  staged?: boolean
  /** Kind 'commit': the commit or stash shown. Kind 'diff': the file as that commit changed it. */
  commit?: string
  /** Kinds 'terminal', 'remote' and 'rfile': the saved SSH host the tab is on. A terminal without it is local. */
  host?: string
  /** Kind 'file': shown in italics until the next single click replaces it, or until it is edited or kept. */
  preview?: boolean
  /** Kept at the start of its pane; Close others and Close to the right leave it. */
  pinned?: boolean
}
export interface Group {
  tabs: Tab[]
  active: string
}
export interface Split {
  split: 'row' | 'column'
  sizes: number[]
  children: Node[]
}
export type Node = Group | Split
export interface Layout {
  root: Node
  focus: string
}

export const MIN_SIZE = 0.1

export const isSplit = (n: Node): n is Split => 'split' in n

export function newTab(id: string, kind = 'blank', title = 'Blank', path?: string): Tab {
  return path === undefined ? { id, kind, title } : { id, kind, title, path }
}

/** The tab that edits this file, if one is open. */
export function findFileTab(layout: Layout, path: string): Tab | null {
  for (const g of groups(layout.root)) {
    const t = g.tabs.find((x) => x.kind === 'file' && x.path === path)
    if (t) return t
  }
  return null
}

/** "Blank 1", "Blank 2", ...: the next number not used by a tab in this layout. */
export function nextBlankTitle(layout: Layout | null): string {
  const used = layout ? groups(layout.root).flatMap((g) => g.tabs.map((t) => /^Blank (\d+)$/.exec(t.title)?.[1] ?? '0')) : []
  return `Blank ${Math.max(0, ...used.map(Number)) + 1}`
}

/** Paths of every file tab, once each. */
export function filePaths(layout: Layout): string[] {
  const paths = groups(layout.root).flatMap((g) => g.tabs.flatMap((t) => (t.kind === 'file' && t.path !== undefined ? [t.path] : [])))
  return [...new Set(paths)]
}

/** File tabs whose file is `path` or inside it (when `path` is a folder). */
export function fileTabsIn(layout: Layout, path: string): Tab[] {
  const inside = (p: string) => p === path || p.startsWith(path + '/')
  return groups(layout.root).flatMap((g) => g.tabs.filter((t) => t.kind === 'file' && t.path !== undefined && inside(t.path)))
}

/** After a rename or move: file tabs under `from` now point at the same place under `to`. */
export function retargetFiles(layout: Layout, from: string, to: string, title: (path: string) => string): Layout {
  const hit = new Set(fileTabsIn(layout, from).map((t) => t.id))
  if (hit.size === 0) return layout
  const fix = (t: Tab): Tab => {
    if (!hit.has(t.id)) return t
    const path = to + t.path!.slice(from.length)
    return { ...t, path, title: title(path) }
  }
  const walk = (n: Node): Node => (isSplit(n) ? { ...n, children: n.children.map(walk) } : { ...n, tabs: n.tabs.map(fix) })
  return { ...layout, root: walk(layout.root) }
}

export function singleTabLayout(tab: Tab): Layout {
  return { root: { tabs: [tab], active: tab.id }, focus: tab.id }
}

export function emptyLayout(): Layout {
  return { root: { tabs: [], active: '' }, focus: '' }
}

/** Accepts what the server stored; returns null when it is not a usable layout. */
export function parseLayout(raw: unknown): Layout | null {
  const root = parseNode(isObj(raw) ? raw.root : undefined, 0)
  if (!root) return null
  const focus = isObj(raw) && typeof raw.focus === 'string' ? raw.focus : ''
  const layout = { root, focus }
  return findGroup(root, focus) ? layout : fixFocus(layout)
}

const isObj = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null

function parseNode(raw: unknown, depth: number): Node | null {
  if (!isObj(raw) || depth > 16) return null
  if (Array.isArray(raw.tabs)) {
    const tabs: Tab[] = []
    for (const t of raw.tabs) {
      if (!isObj(t) || typeof t.id !== 'string' || typeof t.kind !== 'string') return null
      const tab: Tab = { id: t.id, kind: t.kind, title: typeof t.title === 'string' ? t.title : '' }
      if (typeof t.path === 'string') tab.path = t.path
      if (typeof t.folder === 'string') tab.folder = t.folder
      if (typeof t.staged === 'boolean') tab.staged = t.staged
      if (typeof t.commit === 'string') tab.commit = t.commit
      if (typeof t.host === 'string') tab.host = t.host
      if (t.preview === true) tab.preview = true
      if (t.pinned === true) tab.pinned = true
      tabs.push(tab)
    }
    const active = tabs.some((t) => t.id === raw.active) ? String(raw.active) : (tabs[0]?.id ?? '')
    return { tabs, active }
  }
  if ((raw.split !== 'row' && raw.split !== 'column') || !Array.isArray(raw.children) || raw.children.length < 2) return null
  const children: Node[] = []
  for (const c of raw.children) {
    const n = parseNode(c, depth + 1)
    if (!n) return null
    children.push(n)
  }
  const sizes = Array.isArray(raw.sizes) && raw.sizes.length === children.length && raw.sizes.every((s) => typeof s === 'number' && s > 0)
    ? (raw.sizes as number[])
    : []
  return { split: raw.split, sizes: normalize(sizes, children.length), children }
}

export function normalize(sizes: number[], n: number): number[] {
  const total = sizes.reduce((a, b) => a + b, 0)
  if (sizes.length !== n || total <= 0) return Array<number>(n).fill(1 / n)
  return sizes.map((s) => s / total)
}

// Queries

export function groups(node: Node): Group[] {
  return isSplit(node) ? node.children.flatMap(groups) : [node]
}

export function findGroup(node: Node, tabId: string): Group | null {
  return groups(node).find((g) => g.tabs.some((t) => t.id === tabId)) ?? null
}

export function findTab(layout: Layout, tabId: string): Tab | null {
  return findGroup(layout.root, tabId)?.tabs.find((t) => t.id === tabId) ?? null
}

export function focusedGroup(layout: Layout): Group | null {
  return findGroup(layout.root, layout.focus) ?? groups(layout.root).find((g) => g.tabs.length > 0) ?? null
}

export function tabCount(layout: Layout): number {
  return groups(layout.root).reduce((n, g) => n + g.tabs.length, 0)
}

// Tree rewriting helpers (shared with layout-edit.ts)

/** Replaces the group that satisfies `match` with whatever `fn` returns (null removes it). */
export function mapGroup(node: Node, match: (g: Group) => boolean, fn: (g: Group) => Node | null): Node | null {
  if (!isSplit(node)) return match(node) ? fn(node) : node
  const children: Node[] = []
  const sizes: number[] = []
  node.children.forEach((c, i) => {
    const r = mapGroup(c, match, fn)
    if (r) {
      children.push(r)
      sizes.push(node.sizes[i] ?? 1 / node.children.length)
    }
  })
  return collapse({ ...node, children, sizes: normalize(sizes, children.length) })
}

/** A split with one child is that child; a child split in the same direction is merged in. */
export function collapse(node: Split): Node | null {
  if (node.children.length === 0) return null
  if (node.children.length === 1) return node.children[0]
  const children: Node[] = []
  const sizes: number[] = []
  node.children.forEach((c, i) => {
    if (isSplit(c) && c.split === node.split) {
      c.children.forEach((cc, j) => {
        children.push(cc)
        sizes.push(node.sizes[i] * c.sizes[j])
      })
    } else {
      children.push(c)
      sizes.push(node.sizes[i])
    }
  })
  return { ...node, children, sizes: normalize(sizes, children.length) }
}

/** Keeps `focus` on a tab that exists; if it does not, the first tab of the first group that has one. */
export function fixFocus(layout: Layout): Layout {
  if (findGroup(layout.root, layout.focus)) return layout
  const g = groups(layout.root).find((x) => x.tabs.length > 0)
  return { ...layout, focus: g?.active ?? '' }
}

// Tabs

/** Opens a tab in the focused group (or the only group) and focuses it. */
export function openTab(layout: Layout, tab: Tab): Layout {
  const target = focusedGroup(layout) ?? groups(layout.root)[0]
  const root = mapGroup(layout.root, (g) => g === target, (g) => ({ tabs: [...g.tabs, tab], active: tab.id }))
  return { root: root ?? layout.root, focus: tab.id }
}

export function focusTab(layout: Layout, tabId: string): Layout {
  if (!findGroup(layout.root, tabId)) return layout
  const root = mapGroup(layout.root, (g) => g.tabs.some((t) => t.id === tabId), (g) => ({ ...g, active: tabId }))
  return { root: root ?? layout.root, focus: tabId }
}

/** Closes a tab. A group left empty goes away (except the root), and the split around it collapses. */
export function closeTab(layout: Layout, tabId: string): Layout {
  const group = findGroup(layout.root, tabId)
  if (!group) return layout
  const idx = group.tabs.findIndex((t) => t.id === tabId)
  const tabs = group.tabs.filter((t) => t.id !== tabId)
  const active = group.active === tabId ? (tabs[Math.min(idx, tabs.length - 1)]?.id ?? '') : group.active
  const onlyGroup = !isSplit(layout.root)
  const root = mapGroup(layout.root, (g) => g === group, (g) => (tabs.length === 0 && !onlyGroup ? null : { ...g, tabs, active }))
  const next = { root: root ?? { tabs: [], active: '' }, focus: layout.focus }
  if (layout.focus !== tabId) return next
  if (tabs.length > 0) return { ...next, focus: active }
  // The pane is gone: focus the pane that took its place in reading order.
  const before = groups(layout.root)
  const rest = before.filter((g) => g !== group)
  return { ...next, focus: rest[Math.min(before.indexOf(group), rest.length - 1)]?.active ?? '' }
}

/** Swaps a tab for another in the same place, keeping its active and focus state. */
export function replaceTab(layout: Layout, oldId: string, tab: Tab): Layout {
  if (!findGroup(layout.root, oldId)) return layout
  const root = mapGroup(
    layout.root,
    (g) => g.tabs.some((t) => t.id === oldId),
    (g) => ({ tabs: g.tabs.map((t) => (t.id === oldId ? tab : t)), active: g.active === oldId ? tab.id : g.active }),
  )
  return { root: root ?? layout.root, focus: layout.focus === oldId ? tab.id : layout.focus }
}

/** Next or previous tab in the focused group, wrapping. */
export function cycleTab(layout: Layout, step: 1 | -1): Layout {
  const g = focusedGroup(layout)
  if (!g || g.tabs.length < 2) return layout
  const i = g.tabs.findIndex((t) => t.id === layout.focus)
  return focusTab(layout, g.tabs[(i + step + g.tabs.length) % g.tabs.length].id)
}

/** Next or previous pane in reading order, wrapping; focuses that pane's active tab. */
export function cycleGroup(layout: Layout, step: 1 | -1): Layout {
  const gs = groups(layout.root).filter((g) => g.tabs.length > 0)
  if (gs.length < 2) return layout
  const i = gs.findIndex((g) => g.tabs.some((t) => t.id === layout.focus))
  return focusTab(layout, gs[(i + step + gs.length) % gs.length].active)
}
