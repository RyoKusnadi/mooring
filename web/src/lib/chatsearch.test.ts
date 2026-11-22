import { describe, expect, it } from 'vitest'
import { byProject, pieces, searchChats, type ChatHit } from './chatsearch'

describe('chat search', () => {
  it('asks the server with the query and a limit', async () => {
    let url = ''
    const hits = await searchChats('a b&c', async (u) => {
      url = u
      return new Response(JSON.stringify({ hits: [{ chatId: 'c1' }] }))
    })
    expect(url).toBe('/api/chats/search?q=a%20b%26c&limit=50')
    expect(hits[0].chatId).toBe('c1')
  })

  it('splits a snippet on the hit marks', () => {
    expect(pieces('…the \x02zebra\x03 and \x02zeb\x03ras')).toEqual([
      { text: '…the ', hit: false },
      { text: 'zebra', hit: true },
      { text: ' and ', hit: false },
      { text: 'zeb', hit: true },
      { text: 'ras', hit: false },
    ])
    expect(pieces('')).toEqual([])
    expect(pieces('<b>x</b>')).toEqual([{ text: '<b>x</b>', hit: false }])
  })

  it('groups hits by project in the order they came', () => {
    const h = (chatId: string, projectId: string) => ({ chatId, projectId }) as ChatHit
    const g = byProject([h('1', 'b'), h('2', 'a'), h('3', 'b')])
    expect(g.map((x) => [x.projectId, x.hits.map((y) => y.chatId).join()])).toEqual([
      ['b', '1,3'],
      ['a', '2'],
    ])
  })
})
