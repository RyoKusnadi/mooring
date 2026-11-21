// Splitting, moving and resizing. Companion to layout.ts; still pure.
import {
  MIN_SIZE,
  findGroup,
  findTab,
  focusedGroup,
  groups,
  isSplit,
  mapGroup,
  normalize,
  openTab,
  type Group,
  type Layout,
  type Node,
  type Tab,
} from './layout'

export type Edge = 'left' | 'right' | 'top' | 'bottom'
export type DropTarget = Edge | 'center'

function insertBeside(root: Node, anchor: Group, edge: Edge, tab: Tab): Node {
  const split = edge === 'left' || edge === 'right' ? 'row' : 'column'
  const fresh: Group = { tabs: [tab], active: tab.id }
  const pair = edge === 'left' || edge === 'top' ? [fresh, anchor] : [anchor, fresh]
  return mapGroup(root, (g) => g === anchor, () => ({ split, sizes: [0.5, 0.5], children: pair })) ?? root
}

/** A new pane holding `tab`, beside the group that holds `anchorId`. With no tabs yet, just opens the tab. */
export function splitAt(layout: Layout, anchorId: string, edge: Edge, tab: Tab): Layout {
  const anchor = findGroup(layout.root, anchorId)
  if (!anchor) return openTab(layout, tab)
  return { root: insertBeside(layout.root, anchor, edge, tab), focus: tab.id }
}

/** Split the focused pane right (row) or down (column), with a new tab in the new pane. */
export function splitFocused(layout: Layout, dir: 'row' | 'column', tab: Tab): Layout {
  const g = focusedGroup(layout)
  return splitAt(layout, g?.active ?? '', dir === 'row' ? 'right' : 'bottom', tab)
}

function withoutTab(root: Node, from: Group, tabId: string): Node {
  const idx = from.tabs.findIndex((t) => t.id === tabId)
  const tabs = from.tabs.filter((t) => t.id !== tabId)
  const active = from.active === tabId ? (tabs[Math.min(idx, tabs.length - 1)]?.id ?? '') : from.active
  return mapGroup(root, (g) => g === from, () => (tabs.length ? { tabs, active } : null)) ?? root
}

/**
 * Moves a tab. `center` puts it into the group that holds `anchorId` (at `index`, default the end);
 * an edge splits that group and puts the tab in the new pane. The moved tab becomes active and focused.
 */
export function moveTab(layout: Layout, tabId: string, anchorId: string, where: DropTarget, index?: number): Layout {
  const from = findGroup(layout.root, tabId)
  const to = findGroup(layout.root, anchorId)
  const tab = findTab(layout, tabId)
  if (!from || !to || !tab) return layout

  if (where === 'center') {
    if (from === to) {
      const rest = to.tabs.filter((t) => t.id !== tabId)
      const at = Math.max(0, Math.min(index ?? rest.length, rest.length))
      const tabs = [...rest.slice(0, at), tab, ...rest.slice(at)]
      const root = mapGroup(layout.root, (g) => g === to, () => ({ tabs, active: tabId })) ?? layout.root
      return { root, focus: tabId }
    }
    const at = Math.max(0, Math.min(index ?? to.tabs.length, to.tabs.length))
    const tabs = [...to.tabs.slice(0, at), tab, ...to.tabs.slice(at)]
    const placed = mapGroup(layout.root, (g) => g === to, () => ({ tabs, active: tabId })) ?? layout.root
    return { root: withoutTab(placed, from, tabId), focus: tabId }
  }

  if (from === to && from.tabs.length === 1) return layout
  const placed = insertBeside(layout.root, to, where, tab)
  return { root: withoutTab(placed, from, tabId), focus: tabId }
}

/** Moves the focused tab into the next or previous pane. */
export function moveTabToPane(layout: Layout, step: 1 | -1): Layout {
  const gs = groups(layout.root).filter((g) => g.tabs.length > 0)
  const i = gs.findIndex((g) => g.tabs.some((t) => t.id === layout.focus))
  if (gs.length < 2 || i < 0) return layout
  return moveTab(layout, layout.focus, gs[(i + step + gs.length) % gs.length].active, 'center')
}

/**
 * Moves the divider after child `divider` of the split at `path` by `delta`, a fraction of the whole
 * split. The two neighbours trade space; neither goes below MIN_SIZE of the split.
 */
export function resizeSplit(layout: Layout, path: number[], divider: number, delta: number): Layout {
  const update = (node: Node, rest: number[]): Node => {
    if (!isSplit(node)) return node
    if (rest.length > 0) {
      const [i, ...tail] = rest
      return { ...node, children: node.children.map((c, k) => (k === i ? update(c, tail) : c)) }
    }
    const a = node.sizes[divider]
    const b = node.sizes[divider + 1]
    if (a === undefined || b === undefined) return node
    const na = Math.min(Math.max(a + delta, MIN_SIZE), a + b - MIN_SIZE)
    const sizes = node.sizes.slice()
    sizes[divider] = na
    sizes[divider + 1] = a + b - na
    return { ...node, sizes: normalize(sizes, sizes.length) }
  }
  return { ...layout, root: update(layout.root, path) }
}
