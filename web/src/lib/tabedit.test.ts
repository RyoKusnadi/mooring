import { describe, expect, it } from 'vitest'
import { groups, type Layout, type Tab } from './layout'
import { openPreview, othersOf, patchTab, pinTab, rightOf } from './tabedit'

const t = (id: string, extra: Partial<Tab> = {}): Tab => ({ id, kind: 'file', title: id, path: id, ...extra })
const layoutOf = (...tabs: Tab[]): Layout => ({ root: { tabs, active: tabs[0]?.id ?? '' }, focus: tabs[0]?.id ?? '' })
const ids = (l: Layout) => groups(l.root).flatMap((g) => g.tabs.map((x) => x.id))

describe('preview', () => {
  it('adds a preview tab, then the next one takes its place', () => {
    let l = layoutOf(t('a'))
    l = openPreview(l, t('b'))
    expect(ids(l)).toEqual(['a', 'b'])
    expect(groups(l.root)[0].tabs[1].preview).toBe(true)
    l = openPreview(l, t('c'))
    expect(ids(l)).toEqual(['a', 'c'])
    expect(l.focus).toBe('c')
  })
  it('leaves a kept tab alone', () => {
    let l = openPreview(layoutOf(t('a')), t('b'))
    l = patchTab(l, 'b', { preview: false })
    expect(groups(l.root)[0].tabs[1].preview).toBeUndefined()
    l = openPreview(l, t('c'))
    expect(ids(l)).toEqual(['a', 'b', 'c'])
  })
})

describe('pin', () => {
  it('moves to the end of the pinned tabs and drops preview', () => {
    let l = layoutOf(t('a', { pinned: true }), t('b'), t('c', { preview: true }))
    l = pinTab(l, 'c', true)
    expect(ids(l)).toEqual(['a', 'c', 'b'])
    expect(groups(l.root)[0].tabs[1]).toMatchObject({ id: 'c', pinned: true })
    expect(groups(l.root)[0].tabs[1].preview).toBeUndefined()
    l = pinTab(l, 'c', false)
    expect(groups(l.root)[0].tabs[1].pinned).toBeUndefined()
  })
  it('lists what the close commands act on, without pinned tabs', () => {
    const l = layoutOf(t('a', { pinned: true }), t('b'), t('c'), t('d', { pinned: true }), t('e'))
    expect(othersOf(l, 'c').map((x) => x.id)).toEqual(['b', 'e'])
    expect(rightOf(l, 'c').map((x) => x.id)).toEqual(['e'])
  })
})
