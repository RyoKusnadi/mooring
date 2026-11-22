import { describe, expect, it } from 'vitest'
import { getBlame, runAt, type Blame } from './blame'

const b: Blame = {
  runs: [
    { start: 1, count: 2, commit: 'a' },
    { start: 3, count: 1, commit: '' },
    { start: 4, count: 5, commit: 'b' },
  ],
  commits: {},
}

describe('runAt', () => {
  it('finds the run of a line', () => {
    expect(runAt(b, 1)?.commit).toBe('a')
    expect(runAt(b, 2)?.commit).toBe('a')
    expect(runAt(b, 3)?.commit).toBe('')
    expect(runAt(b, 8)?.commit).toBe('b')
  })
  it('is null outside the runs', () => {
    expect(runAt(b, 0)).toBeNull()
    expect(runAt(b, 9)).toBeNull()
    expect(runAt({ runs: [], commits: {} }, 1)).toBeNull()
  })
})

describe('getBlame', () => {
  it('asks for the file in the folder', async () => {
    let url = ''
    const f = async (input: RequestInfo | URL) => {
      url = String(input)
      return new Response(JSON.stringify({ runs: [], commits: {} }), { status: 200, headers: { 'Content-Type': 'application/json' } })
    }
    await getBlame('p1', 'sub', 'a b.txt', undefined, f)
    expect(url).toContain('/api/git/blame?')
    expect(url).toContain('path=a%20b.txt')
    expect(url).toContain('folder=sub')
  })
})
