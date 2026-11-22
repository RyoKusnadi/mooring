import { beforeEach, describe, expect, it } from 'vitest'
import { loadQueue, MAX_ALL_CHARS, MAX_QUEUED, MAX_TOTAL_CHARS, saveQueue, sweepQueues, withinCaps } from './queuestore'

function memoryStorage(): Storage {
  const m = new Map<string, string>()
  return {
    get length() {
      return m.size
    },
    clear: () => m.clear(),
    getItem: (k) => m.get(k) ?? null,
    key: (i) => [...m.keys()][i] ?? null,
    removeItem: (k) => void m.delete(k),
    setItem: (k, v) => void m.set(k, v),
  }
}

describe('queued messages', () => {
  beforeEach(() => {
    Object.defineProperty(globalThis, 'localStorage', { value: memoryStorage(), configurable: true })
  })

  it('comes back as it was saved, per chat', () => {
    saveQueue('a', ['one', 'two'])
    saveQueue('b', ['other'])
    expect(loadQueue('a')).toEqual(['one', 'two'])
    expect(loadQueue('b')).toEqual(['other'])
    expect(loadQueue('c')).toEqual([])
  })

  it('forgets the entry when the queue is empty', () => {
    saveQueue('a', ['one'])
    saveQueue('a', [])
    expect(localStorage.length).toBe(0)
  })

  it('ignores stored data that is not a list of texts', () => {
    localStorage.setItem('mooring.queue.a', '{"x":1}')
    expect(loadQueue('a')).toEqual([])
    localStorage.setItem('mooring.queue.a', '{"tab":"none","at":1,"texts":[1,"ok","  ",null]}')
    expect(loadQueue('a')).toEqual(['ok'])
    localStorage.setItem('mooring.queue.a', 'not json')
    expect(loadQueue('a')).toEqual([])
  })

  it('keeps no more than the count and size caps', () => {
    expect(withinCaps(Array.from({ length: MAX_QUEUED + 3 }, (_, i) => `m${i}`))).toHaveLength(MAX_QUEUED)
    const big = 'x'.repeat(MAX_TOTAL_CHARS - 5)
    expect(withinCaps([big, 'short', 'this one does not fit'])).toEqual([big, 'short'])
  })

  it('is restored only by the tab that wrote it', () => {
    saveQueue('a', ['mine'])
    localStorage.setItem('mooring.queue.b', JSON.stringify({ tab: 'another-tab', at: 1, texts: ['theirs'] }))
    expect(loadQueue('a')).toEqual(['mine'])
    expect(loadQueue('b')).toEqual([])
  })

  it('forgets the queues of chats that are gone', () => {
    saveQueue('a', ['one'])
    saveQueue('b', ['two'])
    sweepQueues((id) => id === 'a')
    expect(loadQueue('a')).toEqual(['one'])
    expect(loadQueue('b')).toEqual([])
  })

  it('keeps all chats within one cap, dropping the oldest entry first', () => {
    const half = 'x'.repeat(MAX_TOTAL_CHARS)
    const old = (id: string, at: number) => localStorage.setItem('mooring.queue.' + id, JSON.stringify({ tab: 'none', at, texts: [half] }))
    old('first', 1)
    old('second', 2)
    old('third', 3)
    expect(MAX_TOTAL_CHARS * 4).toBeGreaterThan(MAX_ALL_CHARS)
    saveQueue('fourth', [half])
    expect(loadQueue('first')).toEqual([])
    expect(loadQueue('fourth')).toEqual([half])
  })
})
