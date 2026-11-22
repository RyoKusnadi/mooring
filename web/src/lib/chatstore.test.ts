import { beforeEach, describe, expect, it } from 'vitest'
import { ChatFeed } from './chatfeed.svelte'
import { chats, type ChatInfo } from './chatstore.svelte'
import type { SocketLike } from './events'

class FakeSocket implements SocketLike {
  sent: Record<string, unknown>[] = []
  onopen: (() => void) | null = null
  onmessage: ((e: { data: unknown }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  send(d: string) {
    this.sent.push(JSON.parse(d))
  }
  close() {}
  serve(f: unknown) {
    this.onmessage?.({ data: JSON.stringify(f) })
  }
}

const perm = (id: number, state: string, chosen?: string) => ({ id, kind: 'permission', at: 1, requestId: 'r' + id, state, chosen, options: [] })
const agentMsg = (id: number, text: string) => ({ id, kind: 'agent', at: 1, text })
const welcome = (epoch: string, pos: number, messages?: unknown[]) => ({ t: 'welcome', epoch, pos, snapshot: messages ? { messages, info: null } : null })
const ev = (pos: number, kind: string, data: unknown) => ({ t: 'event', pos, kind, data })

let sockets: FakeSocket[]
let feed: ChatFeed
const open = () => {
  const s = sockets[sockets.length - 1]
  s.onopen?.()
  return s
}

beforeEach(() => {
  sockets = []
  feed = new ChatFeed(() => {
    const s = new FakeSocket()
    sockets.push(s)
    return s
  })
  feed.open()
})

describe('ChatFeed', () => {
  it('shares one socket between holds; a release counts once and not after shut', () => {
    const n = sockets.length
    const row = feed.open()
    row()
    row()
    const board = feed.open()
    expect(sockets.length).toBe(n) // the first hold from beforeEach still has it open
    feed.shut()
    const view = feed.open()
    expect(sockets.length).toBe(n + 1)
    board() // from before shut: does not close the view's socket
    const other = feed.open()
    expect(sockets.length).toBe(n + 1)
    view()
    other()
    feed.open()
    expect(sockets.length).toBe(n + 2)
  })

  it('fills from the snapshot and streams text into one message', () => {
    const s = open()
    s.serve(welcome('e1', 5, [agentMsg(0, 'Hel')]))
    s.serve(ev(6, 'append', { id: 0, text: 'lo' }))
    expect(feed.msgs.get(0)?.text).toBe('Hello')
    expect(feed.count).toBe(1)
    s.serve(ev(7, 'set', { id: 1, msg: agentMsg(1, 'next') }))
    expect(feed.count).toBe(2)
  })

  it('resumes without a snapshot and keeps what it has', () => {
    let s = open()
    s.serve(welcome('e1', 3, [agentMsg(0, 'a')]))
    feed.close()
    feed.open()
    s = open()
    expect(s.sent[0]).toEqual({ t: 'hello', epoch: 'e1', pos: 3 })
    s.serve(welcome('e1', 4))
    s.serve(ev(5, 'set', { id: 1, msg: agentMsg(1, 'b') }))
    expect(feed.msgs.get(0)?.text).toBe('a')
    expect(feed.count).toBe(2)
  })

  it('replaces everything when the snapshot says reset (new epoch after a restart)', () => {
    let s = open()
    s.serve(welcome('e1', 3, [agentMsg(0, 'old'), agentMsg(1, 'older')]))
    feed.close()
    feed.open()
    s = open()
    s.serve(welcome('e2', 1, [agentMsg(0, 'fresh')]))
    expect(feed.count).toBe(1)
    expect(feed.msgs.has(1)).toBe(false)
    expect(feed.msgs.get(0)?.text).toBe('fresh')
  })

  it('reconnects on a gap in event numbers', () => {
    const s = open()
    s.serve(welcome('e1', 1, []))
    s.serve(ev(3, 'set', { id: 0, msg: agentMsg(0, 'x') }))
    expect(feed.msgs.size).toBe(0)
    expect(feed.wire).toBe('reconnecting')
  })

  it('tracks permission requests and shows one answered elsewhere', () => {
    const s = open()
    s.serve(welcome('e1', 1, [perm(0, 'pending'), perm(2, 'pending')]))
    expect(feed.pending).toEqual([0, 2])
    feed.mine.add('r0')
    s.serve(ev(2, 'set', { id: 0, msg: perm(0, 'answered', 'allow-once') }))
    s.serve(ev(3, 'set', { id: 2, msg: perm(2, 'answered', 'reject') }))
    expect(feed.pending).toEqual([])
    expect(feed.mine.has('r0')).toBe(true)
    expect(feed.mine.has('r2')).toBe(false)
  })
})

const info = (id: string, projectId: string, over: Partial<ChatInfo> = {}): ChatInfo => ({
  id, projectId, agent: 'gemini', title: id, state: 'idle', error: null, attention: false, mode: '', modes: null,
  model: '', models: null, usage: null, caps: null, messages: 0, updatedAt: 1, ...over,
})

describe('Chats store', () => {
  beforeEach(() => chats.reset({}, []))

  it('resets from a snapshot, and drops feeds of chats that are gone', () => {
    chats.reset({ p1: [info('a', 'p1')] }, [{ id: 'gemini', name: 'Gemini', installed: true, tested: false }])
    const f = chats.feed('a')
    expect(chats.feed('a')).toBe(f)
    chats.reset({ p1: [] }, [])
    expect(chats.list.a).toBeUndefined()
    expect(chats.feed('a')).not.toBe(f)
    chats.reset({}, [])
  })

  it('applies added, changed and removed events', () => {
    chats.apply('chat.added', info('a', 'p1'))
    chats.apply('chat.changed', info('a', 'p1', { attention: true }))
    expect(chats.attention('p1')).toBe(true)
    expect(chats.attention('p2')).toBe(false)
    expect(chats.attentionCount()).toBe(1)
    chats.apply('chat.removed', { id: 'a', projectId: 'p1' })
    expect(chats.inProject('p1')).toEqual([])
  })

  it('records an answer as mine, and forgets it when another device won', async () => {
    chats.reset({ p1: [info('a', 'p1')] }, [])
    const f = chats.feed('a')
    const lost = await chats.answer(async () => ({ accepted: false, chosen: 'reject' }) as never, 'a', 'r1', 'allow-once')
    expect(lost.chosen).toBe('reject')
    expect(f.mine.has('r1')).toBe(false)
    await chats.answer(async () => ({ accepted: true, chosen: 'allow-once' }) as never, 'a', 'r2', 'allow-once')
    expect(f.mine.has('r2')).toBe(true)
    await expect(chats.answer(async () => Promise.reject(new Error('x')), 'a', 'r3', 'o')).rejects.toThrow('x')
    expect(f.mine.has('r3')).toBe(false)
  })

  it('resumes a saved chat by replacing its info', async () => {
    chats.reset({ p1: [info('a', 'p1', { state: 'stopped' })] }, [])
    await chats.resume(async () => info('a', 'p1', { state: 'idle' }) as never, 'a')
    expect(chats.list.a.state).toBe('idle')
  })
})

describe('token saving', () => {
  it('starts from a summary with the chat and its context', async () => {
    const calls: unknown[] = []
    const run = ((n: string, a: unknown) => (calls.push([n, a]), Promise.resolve({ id: 'n1' }))) as never
    chats.seeds.n1 = { fromChat: 'old', text: 's', source: 'agent' }
    await chats.start(run, 'p', 'n1', 'gemini', undefined, [], { fromChat: 'old', context: 's' })
    expect(calls[0]).toEqual(['chat.start', { id: 'n1', projectId: 'p', agent: 'gemini', fromChat: 'old', context: 's' }])
    expect(chats.seeds.n1).toBeUndefined()
  })

  it('turns compaction on and off through chat.setFilters', async () => {
    const calls: unknown[] = []
    const run = ((n: string, a: unknown) => (calls.push([n, a]), Promise.resolve({ id: 'c1' }))) as never
    await chats.setCompact(run, 'c1', true)
    await chats.setCompact(run, 'c1', false)
    expect(calls).toEqual([
      ['chat.setFilters', { id: 'c1', filters: ['compact'] }],
      ['chat.setFilters', { id: 'c1', filters: [] }],
    ])
  })
  it('keeps why a start failed until the next start', async () => {
    const fail = (() => Promise.reject(new Error('8 chats are running. Close one to start another.'))) as never
    await expect(chats.start(fail, 'p1', 't1', 'gemini')).rejects.toThrow()
    expect(chats.startError['t1']).toContain('Close one')
    const ok = (() => Promise.resolve({ id: 't1' })) as never
    await chats.start(ok, 'p1', 't1', 'gemini')
    expect(chats.startError['t1']).toBeUndefined()
  })
})
