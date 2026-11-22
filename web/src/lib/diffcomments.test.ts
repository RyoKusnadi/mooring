import { beforeEach, describe, expect, it, vi } from 'vitest'
import { commentsText, diffComments } from './diffcomments.svelte'

const c = (o: Partial<Parameters<typeof diffComments.save>[1]> = {}) => ({ path: 'src/a.ts', line: 3, to: 3, removed: false, quote: 'const a = 1', text: 'Why 1?', ...o })

beforeEach(() => {
  const data = new Map<string, string>()
  vi.stubGlobal('sessionStorage', { getItem: (k: string) => data.get(k) ?? null, setItem: (k: string, v: string) => void data.set(k, v), removeItem: (k: string) => void data.delete(k), clear: () => data.clear() })
  diffComments.clear('p')
  diffComments.clear('q')
})

describe('commentsText', () => {
  it('writes path:line, the quote in a fence, then the comment, in file and line order', () => {
    const t = commentsText([{ id: '2', ...c({ path: 'z.ts', line: 1, to: 1, quote: 'x', text: 'Two' }) }, { id: '1', ...c() }])
    expect(t).toBe('Comments on the changes:\n\nsrc/a.ts:3\n```\nconst a = 1\n```\nWhy 1?\n\nz.ts:1\n```\nx\n```\nTwo')
  })
  it('names a range and a removed line', () => {
    expect(commentsText([{ id: '1', ...c({ to: 5 }) }])).toContain('src/a.ts:3-5\n')
    expect(commentsText([{ id: '1', ...c({ removed: true }) }])).toContain('src/a.ts:3 (removed line)\n')
  })
  it('uses a longer fence when the quote has backticks', () => {
    expect(commentsText([{ id: '1', ...c({ quote: 'a ``` b' }) }])).toContain('````\na ``` b\n````')
  })
})

describe('diffComments', () => {
  it('keeps comments per project and replaces one on the same line', () => {
    diffComments.save('p', c())
    diffComments.save('p', c({ text: 'Better' }))
    diffComments.save('q', c())
    expect(diffComments.list('p').map((x) => x.text)).toEqual(['Better'])
    expect(diffComments.list('q')).toHaveLength(1)
  })
  it('survives a reload of the store state through sessionStorage and clears it', () => {
    diffComments.save('p', c())
    expect(JSON.parse(sessionStorage.getItem('mooring.diffcomments.p') ?? '[]')).toHaveLength(1)
    diffComments.clear('p')
    expect(sessionStorage.getItem('mooring.diffcomments.p')).toBeNull()
  })
  it('refuses over 64 KB with a plain error', () => {
    diffComments.save('p', c({ text: 'x'.repeat(70 * 1024) }))
    expect(diffComments.prepare('p').error).toContain('64 KB')
    diffComments.clear('p')
    expect(diffComments.prepare('p').error).toBe('')
  })
})
