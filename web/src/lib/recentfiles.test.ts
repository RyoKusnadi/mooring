import { describe, expect, it } from 'vitest'
import { noteOpened, recentFiles } from './recentfiles'

const memory = () => {
  const m = new Map<string, string>()
  return { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v), removeItem: (k: string) => void m.delete(k) }
}

describe('recentFiles', () => {
  it('keeps the newest first, once each, per project', () => {
    const s = memory()
    noteOpened('p1', 'a.ts', s)
    noteOpened('p1', 'b.ts', s)
    noteOpened('p1', 'a.ts', s)
    noteOpened('p2', 'z.ts', s)
    expect(recentFiles('p1', s)).toEqual(['a.ts', 'b.ts'])
    expect(recentFiles('p2', s)).toEqual(['z.ts'])
  })
  it('caps the list and survives bad stored data', () => {
    const s = memory()
    for (let i = 0; i < 40; i++) noteOpened('p', `f${i}`, s)
    expect(recentFiles('p', s)).toHaveLength(30)
    s.setItem('mooring.recent.q', '{oops')
    expect(recentFiles('q', s)).toEqual([])
  })
})
