import { afterEach, describe, expect, it, vi } from 'vitest'
import { History } from './history.svelte'
import type { Commit } from './historyapi'

const commit = (n: number): Commit => ({ id: 'c' + n, parents: n < 9 ? ['c' + (n + 1)] : [], author: 'A', email: 'a@x', time: 1, subject: 's' + n, refs: [] })
const page = (from: number, to: number, next: string) =>
  new Response(JSON.stringify({ commits: Array.from({ length: to - from }, (_, i) => commit(from + i)), next, total: 10 }))

afterEach(() => vi.unstubAllGlobals())

describe('History', () => {
  it('reads page after page and adds lanes as they arrive', async () => {
    const calls: string[] = []
    vi.stubGlobal('fetch', async (u: string) => {
      calls.push(u)
      return u.includes('cursor=p2') ? page(5, 10, '') : page(0, 5, 'p2')
    })
    const h = new History('p', '')
    await h.start()
    expect(h.commits.length).toBe(5)
    expect(h.rows.length).toBe(5)
    await h.more()
    expect(h.commits.length).toBe(10)
    expect(h.rows.length).toBe(10)
    expect(h.next).toBe('')
    await h.more()
    expect(calls.length).toBe(2)
    expect(h.rows.every((r) => r.col === 0)).toBe(true)
  })

  it('starts again from the first page when the branches moved', async () => {
    let moved = false
    vi.stubGlobal('fetch', async (u: string) => {
      if (u.includes('cursor=p2') && !moved) {
        moved = true
        return new Response(JSON.stringify({ error: { code: 'LOG_CHANGED', message: 'moved' } }), { status: 409 })
      }
      return u.includes('cursor=p2') ? page(5, 10, '') : page(0, 5, 'p2')
    })
    const h = new History('p', '')
    await h.start()
    await h.more()
    expect(h.error).toBe('')
    expect(h.commits.length).toBeGreaterThanOrEqual(5)
    expect(h.restarted).toBe(1)
    expect(h.rows.length).toBe(h.commits.length)
  })

  it('a search draws no lanes', async () => {
    vi.stubGlobal('fetch', async () => new Response(JSON.stringify({ commits: [commit(1)], next: '', total: -1 })))
    const h = new History('p', '')
    await h.start('fix', 'message')
    expect(h.commits.length).toBe(1)
    expect(h.rows.length).toBe(0)
  })

  it('does not read again after a failed page until the user retries', async () => {
    let calls = 0
    vi.stubGlobal('fetch', async (u: string) => {
      calls++
      if (u.includes('cursor=p2')) return new Response(JSON.stringify({ error: { code: 'GIT_FAILED', message: 'boom' } }), { status: 500 })
      return page(0, 5, 'p2')
    })
    const h = new History('p', '')
    await h.start()
    await h.more()
    expect(h.error).not.toBe('')
    const n = calls
    await h.more()
    await h.more()
    expect(calls).toBe(n)
  })
})
