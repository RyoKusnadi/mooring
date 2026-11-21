// Preview, pin and the sets of tabs a context menu closes. Pure, like layout.ts.
import { findGroup, focusedGroup, mapGroup, openTab, replaceTab, type Layout, type Tab } from './layout'

/** Changes fields of one tab, keeping its place. A field set to undefined is removed. */
export function patchTab(layout: Layout, id: string, patch: Partial<Tab>): Layout {
  const root = mapGroup(
    layout.root,
    (g) => g.tabs.some((t) => t.id === id),
    (g) => ({
      ...g,
      tabs: g.tabs.map((t) => {
        if (t.id !== id) return t
        const next = { ...t, ...patch }
        for (const k of Object.keys(next) as (keyof Tab)[]) if (next[k] === undefined || next[k] === false) delete next[k]
        return next
      }),
    }),
  )
  return { ...layout, root: root ?? layout.root }
}

/** The preview file tab of the focused pane, if it has one. */
export function previewIn(layout: Layout): Tab | undefined {
  return focusedGroup(layout)?.tabs.find((t) => t.kind === 'file' && t.preview)
}

/** Opens a preview tab: it takes the place of the pane's current preview tab, or is added. */
export function openPreview(layout: Layout, tab: Tab): Layout {
  const old = previewIn(layout)
  return old ? replaceTab(layout, old.id, { ...tab, preview: true }) : openTab(layout, { ...tab, preview: true })
}

/** Pinning moves the tab to the end of the pinned tabs and makes it permanent; unpinning leaves it where it is. */
export function pinTab(layout: Layout, id: string, pinned: boolean): Layout {
  const g = findGroup(layout.root, id)
  const tab = g?.tabs.find((t) => t.id === id)
  if (!g || !tab) return layout
  if (!pinned) return patchTab(layout, id, { pinned: false })
  const rest = g.tabs.filter((t) => t.id !== id)
  const at = rest.filter((t) => t.pinned).length
  const kept: Tab = { ...tab, pinned: true }
  delete kept.preview
  const tabs = [...rest.slice(0, at), kept, ...rest.slice(at)]
  const root = mapGroup(layout.root, (x) => x === g, (x) => ({ ...x, tabs }))
  return { ...layout, root: root ?? layout.root }
}

/** The tabs "Close others" would close: the same pane, not pinned, not the one clicked. */
export function othersOf(layout: Layout, id: string): Tab[] {
  return findGroup(layout.root, id)?.tabs.filter((t) => t.id !== id && !t.pinned) ?? []
}

/** The tabs "Close to the right" would close. */
export function rightOf(layout: Layout, id: string): Tab[] {
  const tabs = findGroup(layout.root, id)?.tabs ?? []
  return tabs.slice(tabs.findIndex((t) => t.id === id) + 1).filter((t) => !t.pinned)
}
