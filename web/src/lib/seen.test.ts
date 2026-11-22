import { describe, expect, it } from 'vitest'
import { seenToSend, visibleChats } from './seen'
import type { Layout } from './layout'

const tab = (id: string, kind = 'chat') => ({ id, kind, title: id })

describe('visibleChats', () => {
  it('takes the selected chat tab of each pane', () => {
    const layout: Layout = {
      focus: 'a',
      root: { split: 'row', sizes: [0.5, 0.5], children: [{ tabs: [tab('a'), tab('b')], active: 'a' }, { tabs: [tab('f', 'file'), tab('c')], active: 'f' }] },
    }
    expect(visibleChats(layout)).toEqual(['a'])
  })
  it('is empty without a layout', () => expect(visibleChats(null)).toEqual([]))
})

describe('seenToSend', () => {
  const known = () => true
  it('sends a chat that just came into view', () => expect(seenToSend(new Set(), ['a'], known, () => false)).toEqual(['a']))
  it('stays quiet while nothing changed', () => expect(seenToSend(new Set(['a']), ['a'], known, () => false)).toEqual([]))
  it('sends again when a reply finished on the chat in view', () => expect(seenToSend(new Set(['a']), ['a'], known, () => true)).toEqual(['a']))
  it('waits for a chat the list does not have yet', () => expect(seenToSend(new Set(), ['a'], () => false, () => false)).toEqual([]))
})
