import { describe, expect, it } from 'vitest'
import type { ChatInfo } from './chatstore.svelte'
import { chatRows, groupCandidates, groupTitle, membersOf } from './chatgroups'

const chat = (id: string, o: Partial<ChatInfo> = {}) => ({ id, title: id, projectId: 'p', attention: false, updatedAt: 0, ...o }) as ChatInfo

describe('chatRows', () => {
  it('gathers a group at its newest member and keeps plain chats', () => {
    const rows = chatRows([chat('a', { groupId: 'g' }), chat('b'), chat('c', { groupId: 'g' })])
    expect(rows).toHaveLength(2)
    expect('group' in rows[0] && rows[0].group.members.map((m) => m.id)).toEqual(['a', 'c'])
    expect('chat' in rows[1] && rows[1].chat.id).toBe('b')
  })
  it('needs you when any member does', () => {
    const rows = chatRows([chat('a', { groupId: 'g' }), chat('c', { groupId: 'g', attention: true })])
    expect('group' in rows[0] && rows[0].group.attention).toBe(true)
  })
})

describe('groupTitle', () => {
  it('uses the title, else the member names', () => {
    expect(groupTitle([chat('a', { groupTitle: 'Fix login' }), chat('b')])).toBe('Fix login')
    expect(groupTitle([chat('a', { title: 'One' }), chat('b', { title: '' })])).toBe('Group: One, New chat')
  })
})

describe('membersOf and groupCandidates', () => {
  const all = { a: chat('a', { groupId: 'g', createdAt: 2 }), b: chat('b', { groupId: 'g', createdAt: 1 }), c: chat('c'), d: chat('d', { projectId: 'q' }), e: chat('e', { groupId: 'h' }) }
  it('lists members oldest first', () => expect(membersOf(all, 'g').map((m) => m.id)).toEqual(['b', 'a']))
  it('offers free chats of the same project only', () => expect(groupCandidates(all, all.c).map((m) => m.id)).toEqual([]))
  it('skips the chat itself and grouped chats', () => expect(groupCandidates({ ...all, f: chat('f') }, all.c).map((m) => m.id)).toEqual(['f']))
})
