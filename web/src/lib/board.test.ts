import { describe, expect, it } from 'vitest'
import { boardCounts, boardGroups, groupOf, onBoard, unseenCount, type BoardChat } from './board'
import { switchLabel } from './boardfmt'

const chat = (id: string, over: Partial<BoardChat> = {}): BoardChat => ({
  id,
  projectId: 'p',
  state: 'idle',
  attention: false,
  updatedAt: 0,
  ...over,
})

describe('status board', () => {
  it('puts each chat in one group, attention first', () => {
    expect(groupOf(chat('a', { attention: true, state: 'running' }))).toBe('waiting')
    expect(groupOf(chat('a', { state: 'starting' }))).toBe('running')
    expect(groupOf(chat('a', { state: 'running', unseen: true }))).toBe('running')
    expect(groupOf(chat('a', { unseen: true }))).toBe('finished')
    expect(groupOf(chat('a', { state: 'stopped' }))).toBe('idle')
  })

  it('orders the groups and the chats in them, and leaves empty groups out', () => {
    const g = boardGroups([
      chat('i1', { updatedAt: 1 }),
      chat('w-new', { attention: true, updatedAt: 9 }),
      chat('f-old', { unseen: true, finishedAt: 2, updatedAt: 50 }),
      chat('w-old', { attention: true, updatedAt: 3 }),
      chat('f-new', { unseen: true, finishedAt: 7, updatedAt: 7 }),
      chat('i2', { updatedAt: 5 }),
    ])
    expect(g.map((x) => [x.key, x.chats.map((c) => c.id).join()])).toEqual([
      ['waiting', 'w-old,w-new'],
      ['finished', 'f-new,f-old'],
      ['idle', 'i2,i1'],
    ])
  })

  it('cuts only the idle group and says how many it hid', () => {
    const list = [...Array(5)].map((_, i) => chat(`i${i}`, { updatedAt: i }))
    list.push(...[...Array(4)].map((_, i) => chat(`r${i}`, { state: 'running' })))
    const g = boardGroups(list, 2)
    expect(g.find((x) => x.key === 'running')).toMatchObject({ hidden: 0 })
    expect(g.find((x) => x.key === 'idle')).toMatchObject({ hidden: 3 })
    expect(g.find((x) => x.key === 'idle')!.chats.map((c) => c.id)).toEqual(['i4', 'i3'])
    expect(unseenCount([chat('a', { unseen: true }), chat('b', { unseen: true, attention: true })])).toBe(1)
  })
})

describe('board list and switch', () => {
  it('leaves out chats of a project that is gone', () => {
    const list = [chat('a', { projectId: 'p' }), chat('b', { projectId: 'gone', unseen: true })]
    expect(onBoard(list, ['p']).map((c) => c.id)).toEqual(['a'])
  })
  it('counts waiting and finished, and names both', () => {
    const list = [chat('a', { attention: true }), chat('b', { unseen: true }), chat('c', { unseen: true })]
    expect(boardCounts(list)).toEqual({ waiting: 1, finished: 2 })
    expect(switchLabel({ waiting: 1, finished: 2 })).toBe('Board, 1 waiting for you, 2 finished since you looked')
    expect(switchLabel({ waiting: 0, finished: 2 })).toBe('Board, 2 finished since you looked')
    expect(switchLabel({ waiting: 0, finished: 0 })).toBeUndefined()
  })
})
