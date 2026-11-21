import { describe, expect, it } from 'vitest'
import {
  closeTab,
  cycleGroup,
  cycleTab,
  emptyLayout,
  findGroup,
  focusTab,
  focusedGroup,
  groups,
  isSplit,
  newTab,
  replaceTab,
  openTab,
  parseLayout,
  singleTabLayout,
  tabCount,
  type Layout,
  type Node,
} from './layout'
import { moveTab, moveTabToPane, resizeSplit, splitAt, splitFocused } from './layout-edit'

const t = (id: string) => newTab(id)
const ids = (n: Node): unknown =>
  isSplit(n) ? { [n.split]: n.children.map(ids) } : n.tabs.map((x) => x.id).join(',')
const shape = (l: Layout) => ids(l.root)

describe('open, focus, close', () => {
  it('opens a tab in the focused group and focuses it', () => {
    const l = openTab(singleTabLayout(t('a')), t('b'))
    expect(shape(l)).toBe('a,b')
    expect(l.focus).toBe('b')
    expect(focusedGroup(l)?.active).toBe('b')
  })

  it('opens the first tab in an empty layout', () => {
    const l = openTab(emptyLayout(), t('a'))
    expect(shape(l)).toBe('a')
    expect(l.focus).toBe('a')
  })

  it('focusing a tab makes it active in its group', () => {
    const l = focusTab(openTab(singleTabLayout(t('a')), t('b')), 'a')
    expect(focusedGroup(l)?.active).toBe('a')
    expect(focusTab(l, 'nope')).toBe(l)
  })

  it('closing the active tab activates its neighbour', () => {
    let l = openTab(openTab(singleTabLayout(t('a')), t('b')), t('c'))
    l = closeTab(focusTab(l, 'b'), 'b')
    expect(shape(l)).toBe('a,c')
    expect(l.focus).toBe('c')
  })

  it('closing the last tab of the only group leaves an empty layout', () => {
    const l = closeTab(singleTabLayout(t('a')), 'a')
    expect(shape(l)).toBe('')
    expect(l.focus).toBe('')
  })

  it('closing the last tab of a pane removes the pane and collapses the split', () => {
    const l = closeTab(splitFocused(singleTabLayout(t('a')), 'row', t('b')), 'b')
    expect(shape(l)).toBe('a')
    expect(isSplit(l.root)).toBe(false)
    expect(l.focus).toBe('a')
  })

  it('closing a pane in a three-way split keeps the other two with equal sizes', () => {
    let l = singleTabLayout(t('a'))
    l = splitFocused(l, 'row', t('b'))
    l = splitFocused(l, 'column', t('c'))
    expect(shape(l)).toEqual({ row: ['a', { column: ['b', 'c'] }] })
    l = closeTab(l, 'b')
    expect(shape(l)).toEqual({ row: ['a', 'c'] })
    expect(l.focus).toBe('c')
    const root = l.root
    expect(isSplit(root) && root.sizes).toEqual([0.5, 0.5])
  })

  it('merges a same-direction split left behind by a close', () => {
    let l = singleTabLayout(t('a'))
    l = splitFocused(l, 'row', t('b'))
    l = splitFocused(l, 'column', t('c'))
    l = splitAt(l, 'c', 'right', t('d'))
    expect(shape(l)).toEqual({ row: ['a', { column: ['b', { row: ['c', 'd'] }] }] })
    l = closeTab(l, 'b')
    expect(shape(l)).toEqual({ row: ['a', 'c', 'd'] })
    const root = l.root
    expect(isSplit(root) && root.sizes.reduce((x, y) => x + y)).toBeCloseTo(1)
  })

  it('closing an unknown tab changes nothing', () => {
    const l = singleTabLayout(t('a'))
    expect(closeTab(l, 'zz')).toBe(l)
  })

  it('does not change its input', () => {
    const l = splitFocused(singleTabLayout(t('a')), 'row', t('b'))
    const before = JSON.stringify(l)
    closeTab(l, 'b')
    moveTab(l, 'a', 'b', 'bottom')
    resizeSplit(l, [], 0, 0.2)
    expect(JSON.stringify(l)).toBe(before)
  })
})

describe('splitting', () => {
  it('splits right into a row and down into a column, focusing the new tab', () => {
    const r = splitFocused(singleTabLayout(t('a')), 'row', t('b'))
    expect(shape(r)).toEqual({ row: ['a', 'b'] })
    expect(r.focus).toBe('b')
    expect(shape(splitFocused(singleTabLayout(t('a')), 'column', t('b')))).toEqual({ column: ['a', 'b'] })
  })

  it('splitting an empty layout just opens the tab', () => {
    expect(shape(splitFocused(emptyLayout(), 'row', t('a')))).toBe('a')
  })

  it('splitting the same direction twice makes one flat split of three', () => {
    let l = splitFocused(singleTabLayout(t('a')), 'row', t('b'))
    l = splitFocused(l, 'row', t('c'))
    expect(shape(l)).toEqual({ row: ['a', 'b', 'c'] })
    const root = l.root
    expect(isSplit(root) && root.sizes.reduce((x, y) => x + y)).toBeCloseTo(1)
  })
})

describe('moving tabs', () => {
  const two = () => openTab(splitFocused(singleTabLayout(t('a')), 'row', t('b')), t('c'))

  it('moves a tab into another group and focuses it there', () => {
    const l = moveTab(two(), 'a', 'b', 'center')
    expect(shape(l)).toBe('b,c,a')
    expect(l.focus).toBe('a')
  })

  it('removes the source pane when its last tab moves away', () => {
    const l = moveTab(two(), 'a', 'b', 'center')
    expect(isSplit(l.root)).toBe(false)
  })

  it('moves a tab to the edge of another group, making a split', () => {
    const l = moveTab(two(), 'c', 'a', 'bottom')
    expect(shape(l)).toEqual({ row: [{ column: ['a', 'c'] }, 'b'] })
    expect(l.focus).toBe('c')
  })

  it('moves a tab to the edge of its own group when the group has other tabs', () => {
    const l = moveTab(two(), 'c', 'b', 'right')
    expect(shape(l)).toEqual({ row: ['a', 'b', 'c'] })
  })

  it('ignores a drop on the edge of a pane that holds only that tab', () => {
    const l = singleTabLayout(t('a'))
    expect(moveTab(l, 'a', 'a', 'left')).toBe(l)
  })

  it('reorders inside a group', () => {
    const l = openTab(openTab(singleTabLayout(t('a')), t('b')), t('c'))
    expect(shape(moveTab(l, 'c', 'a', 'center', 0))).toBe('c,a,b')
    expect(shape(moveTab(l, 'a', 'a', 'center', 5))).toBe('b,c,a')
  })

  it('moves to the next pane and wraps', () => {
    const l = moveTabToPane(focusTab(two(), 'c'), 1)
    expect(shape(l)).toEqual({ row: ['a,c', 'b'] })
    expect(shape(moveTabToPane(singleTabLayout(t('a')), 1))).toBe('a')
  })

  it('left edge puts the new pane first', () => {
    const l = moveTab(two(), 'c', 'a', 'left')
    expect(shape(l)).toEqual({ row: ['c', 'a', 'b'] })
  })
})

describe('cycling and resizing', () => {
  it('cycles tabs inside the focused group', () => {
    const l = openTab(openTab(singleTabLayout(t('a')), t('b')), t('c'))
    expect(cycleTab(l, 1).focus).toBe('a')
    expect(cycleTab(l, -1).focus).toBe('b')
  })

  it('cycles panes in reading order', () => {
    const l = splitFocused(singleTabLayout(t('a')), 'row', t('b'))
    expect(cycleGroup(l, 1).focus).toBe('a')
    expect(cycleGroup(cycleGroup(l, 1), -1).focus).toBe('b')
  })

  it('moves a divider and keeps a minimum size', () => {
    const l = splitFocused(singleTabLayout(t('a')), 'row', t('b'))
    const r = resizeSplit(l, [], 0, 0.2).root
    expect(isSplit(r) && r.sizes[0]).toBeCloseTo(0.7)
    expect(isSplit(r) && r.sizes[1]).toBeCloseTo(0.3)
    const far = resizeSplit(l, [], 0, 5).root
    expect(isSplit(far) && far.sizes[1]).toBeCloseTo(0.1)
    const near = resizeSplit(l, [], 0, -5).root
    expect(isSplit(near) && near.sizes[0]).toBeCloseTo(0.1)
  })

  it('resizes a nested split by path', () => {
    let l = splitFocused(singleTabLayout(t('a')), 'row', t('b'))
    l = splitFocused(l, 'column', t('c'))
    const r = resizeSplit(l, [1], 0, 0.25).root
    const inner = isSplit(r) ? r.children[1] : null
    expect(inner && isSplit(inner) && inner.sizes[0]).toBeCloseTo(0.75)
  })
})

describe('parseLayout', () => {
  it('accepts a stored layout and repairs focus and sizes', () => {
    const raw = {
      root: { split: 'row', sizes: [3, 1], children: [{ tabs: [{ id: 'a', kind: 'blank', title: 'Blank' }], active: 'a' }, { tabs: [{ id: 'b', kind: 'blank', title: 'Blank' }], active: 'zz' }] },
      focus: 'gone',
    }
    const l = parseLayout(raw)
    expect(l?.focus).toBe('a')
    expect(l && isSplit(l.root) && l.root.sizes).toEqual([0.75, 0.25])
    expect(l && findGroup(l.root, 'b')?.active).toBe('b')
    expect(l && tabCount(l)).toBe(2)
    expect(l && groups(l.root)).toHaveLength(2)
  })

  it('rejects things that are not layouts', () => {
    expect(parseLayout(null)).toBeNull()
    expect(parseLayout({})).toBeNull()
    expect(parseLayout({ root: { split: 'row', sizes: [], children: [] }, focus: '' })).toBeNull()
    expect(parseLayout({ root: { tabs: [{ id: 1 }], active: '' }, focus: '' })).toBeNull()
  })
})

describe('replaceTab', () => {
  it('swaps a tab in place and moves active and focus with it', () => {
    const l = { root: { tabs: [newTab('a'), newTab('b')], active: 'b' }, focus: 'b' }
    const r = replaceTab(l, 'b', newTab('c', 'terminal', 'T'))
    expect(r.focus).toBe('c')
    expect(findGroup(r.root, 'c')?.tabs.map((t) => t.id)).toEqual(['a', 'c'])
    expect(findGroup(r.root, 'c')?.active).toBe('c')
  })
  it('leaves the layout alone for an unknown tab', () => {
    const l = singleTabLayout(newTab('a'))
    expect(replaceTab(l, 'zz', newTab('c'))).toBe(l)
  })
})
